package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
)

// Agent 工具循环：模型通过 OpenAI 兼容的 function calling 调工具查真实数据再回答。
// 权限红线：每个工具执行前必做成员校验（requireBookMember），只读用户自己的账本
// 与用户参与的共享账本；权限在工具层代码里卡死，不靠提示词。

const agentMaxToolRounds = 5

var errAgentForbidden = errors.New("无权访问该账本")

// formatCentsDecimal 把分格式化为不带货币符号的元字符串，如 13300 -> "133.00"，便于模型直接引用。
func formatCentsDecimal(cents int64) string {
	neg := ""
	if cents < 0 {
		neg = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s%d.%02d", neg, cents/100, cents%100)
}

// ---------- 工具参数解析 ----------

func toolStr(args map[string]interface{}, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func toolFloat(args map[string]interface{}, key string) (float64, bool) {
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	}
	return 0, false
}

func toolInt(args map[string]interface{}, key string) (int, bool) {
	f, ok := toolFloat(args, key)
	if !ok {
		return 0, false
	}
	return int(f), true
}

// resolveToolBookID 解析工具的 bookId 参数：
// - 为空 → 当前账本
// - 是用户账本的 ID 或名称 → 对应账本 ID
// - 其他值 → 原样返回，后续成员校验会拒绝（不泄露该账本是否存在）
func (s *AiService) resolveToolBookID(ctx context.Context, userID, currentBookID, arg string) string {
	if arg == "" {
		return currentBookID
	}
	if arg == currentBookID {
		return arg
	}
	books, err := s.bookRepo.ListBooks(ctx, userID)
	if err != nil {
		return arg
	}
	for _, b := range books {
		if b.ID == arg || b.Name == arg {
			return b.ID
		}
	}
	return arg
}

// requireBookMember 是所有工具的硬性门禁：请求者必须是该账本成员。
func (s *AiService) requireBookMember(ctx context.Context, userID, bookID string) error {
	if bookID == "" {
		return errAgentForbidden
	}
	if _, err := s.bookRepo.GetMemberRole(ctx, bookID, userID); err != nil {
		return errAgentForbidden
	}
	return nil
}

func (s *AiService) resolveCategoryID(ctx context.Context, bookID, idOrName string) (string, error) {
	if idOrName == "" {
		return "", nil
	}
	cats, err := s.catRepo.ListCategories(ctx, bookID, "")
	if err != nil {
		return "", err
	}
	for _, c := range cats {
		if c.ID == idOrName {
			return c.ID, nil
		}
	}
	for _, c := range cats {
		if c.Name == idOrName {
			return c.ID, nil
		}
	}
	for _, c := range cats {
		if strings.Contains(c.Name, idOrName) || strings.Contains(idOrName, c.Name) {
			return c.ID, nil
		}
	}
	return "", fmt.Errorf("找不到分类「%s」", idOrName)
}

func (s *AiService) resolveWalletID(ctx context.Context, bookID, idOrName string) (string, error) {
	if idOrName == "" {
		return "", nil
	}
	wallets, err := s.walletRepo.ListWallets(ctx, bookID, false)
	if err != nil {
		return "", err
	}
	for _, w := range wallets {
		if w.ID == idOrName || w.Name == idOrName {
			return w.ID, nil
		}
	}
	for _, w := range wallets {
		if strings.Contains(w.Name, idOrName) || strings.Contains(idOrName, w.Name) {
			return w.ID, nil
		}
	}
	return "", fmt.Errorf("找不到账户「%s」", idOrName)
}

func (s *AiService) resolveMemberID(ctx context.Context, bookID, idOrName string) (string, error) {
	if idOrName == "" {
		return "", nil
	}
	members, err := s.bookRepo.ListMembersForUser(ctx, bookID, "")
	if err != nil {
		return "", err
	}
	for _, m := range members {
		if m.UserID == idOrName || m.UserName == idOrName {
			return m.UserID, nil
		}
	}
	for _, m := range members {
		if strings.Contains(m.UserName, idOrName) || strings.Contains(idOrName, m.UserName) {
			return m.UserID, nil
		}
	}
	return "", fmt.Errorf("找不到成员「%s」", idOrName)
}

// resolveDateRange 解析工具的时间参数：period=month|quarter|year|custom，
// 或直接给 startDate/endDate。缺省为本月。
func resolveDateRange(args map[string]interface{}) (startDate, endDate string, err error) {
	now := time.Now()
	period := toolStr(args, "period")
	sd, ed := toolStr(args, "startDate"), toolStr(args, "endDate")
	if sd != "" && ed != "" {
		if _, e := time.Parse("2006-01-02", sd); e != nil {
			return "", "", fmt.Errorf("开始日期格式不对，应为 YYYY-MM-DD")
		}
		if _, e := time.Parse("2006-01-02", ed); e != nil {
			return "", "", fmt.Errorf("结束日期格式不对，应为 YYYY-MM-DD")
		}
		if sd > ed {
			return "", "", fmt.Errorf("开始日期不能晚于结束日期")
		}
		return sd, ed, nil
	}
	year := now.Year()
	if y, ok := toolInt(args, "year"); ok && y >= 2000 && y <= 2100 {
		year = y
	}
	switch period {
	case "year":
		return fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-12-31", year), nil
	case "quarter":
		q := (int(now.Month())-1)/3 + 1
		if v, ok := toolInt(args, "quarter"); ok && v >= 1 && v <= 4 {
			q = v
		} else if m, ok := toolInt(args, "month"); ok && m >= 1 && m <= 12 {
			q = (m-1)/3 + 1
		}
		startMonth := (q-1)*3 + 1
		start := time.Date(year, time.Month(startMonth), 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 3, -1)
		return start.Format("2006-01-02"), end.Format("2006-01-02"), nil
	case "custom":
		return "", "", fmt.Errorf("custom 周期需要同时给出 startDate 和 endDate")
	default: // month
		month := int(now.Month())
		if m, ok := toolInt(args, "month"); ok && m >= 1 && m <= 12 {
			month = m
		}
		start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 1, -1)
		return start.Format("2006-01-02"), end.Format("2006-01-02"), nil
	}
}

// ---------- 工具执行 ----------

type agentTool struct {
	name        string
	description string
	parameters  map[string]interface{}
	execute     func(ctx context.Context, userID, currentBookID string, args map[string]interface{}) (interface{}, error)
}

func (s *AiService) agentTools() []agentTool {
	return []agentTool{
		{
			name:        "query_stats",
			description: "查询指定账本在一段时间内的收支统计：总额、按分类汇总、按月汇总。所有金额以分为单位返回，同时给出元字符串。只能查用户自己的账本或用户参与的共享账本。",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"bookId":       map[string]interface{}{"type": "string", "description": "账本 ID 或账本名称；缺省为当前账本"},
					"period":       map[string]interface{}{"type": "string", "enum": []string{"month", "quarter", "year", "custom"}, "description": "统计周期，缺省 month"},
					"year":         map[string]interface{}{"type": "integer", "description": "年份，如 2026，缺省当年"},
					"month":        map[string]interface{}{"type": "integer", "description": "月份 1-12，缺省当月"},
					"quarter":      map[string]interface{}{"type": "integer", "description": "季度 1-4，仅 period=quarter 时用"},
					"startDate":    map[string]interface{}{"type": "string", "description": "开始日期 YYYY-MM-DD，period=custom 时必填"},
					"endDate":      map[string]interface{}{"type": "string", "description": "结束日期 YYYY-MM-DD，period=custom 时必填"},
					"type":         map[string]interface{}{"type": "string", "enum": []string{"expense", "income"}, "description": "只统计支出或收入；缺省两者都返回"},
					"categoryId":   map[string]interface{}{"type": "string", "description": "分类 ID"},
					"categoryName": map[string]interface{}{"type": "string", "description": "分类名称，如 餐饮；与 categoryId 二选一"},
				},
			},
			execute: s.toolQueryStats,
		},
		{
			name:        "query_transactions",
			description: "查询指定账本的流水明细行。返回最近的若干笔，可按时间、分类、账户、成员、关键字过滤。最多返回 100 笔。",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"bookId":       map[string]interface{}{"type": "string", "description": "账本 ID 或账本名称；缺省为当前账本"},
					"startDate":    map[string]interface{}{"type": "string", "description": "开始日期 YYYY-MM-DD"},
					"endDate":      map[string]interface{}{"type": "string", "description": "结束日期 YYYY-MM-DD"},
					"categoryId":   map[string]interface{}{"type": "string", "description": "分类 ID"},
					"categoryName": map[string]interface{}{"type": "string", "description": "分类名称，如 餐饮"},
					"walletId":     map[string]interface{}{"type": "string", "description": "账户 ID"},
					"walletName":   map[string]interface{}{"type": "string", "description": "账户名称，如 微信支付"},
					"memberId":     map[string]interface{}{"type": "string", "description": "成员用户 ID"},
					"memberName":   map[string]interface{}{"type": "string", "description": "成员姓名，如 松"},
					"keyword":      map[string]interface{}{"type": "string", "description": "关键字，匹配标题、备注、分类、账户"},
					"limit":        map[string]interface{}{"type": "integer", "description": "返回笔数上限，缺省 20，最多 100"},
				},
			},
			execute: s.toolQueryTransactions,
		},
		{
			name:        "query_budgets",
			description: "查询指定账本当月的预算执行情况：月总预算与各分类预算、已用、剩余、百分比。",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"bookId": map[string]interface{}{"type": "string", "description": "账本 ID 或账本名称；缺省为当前账本"},
				},
			},
			execute: s.toolQueryBudgets,
		},
		{
			name:        "query_wallets",
			description: "查询指定账本各钱包的余额与总余额。",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"bookId": map[string]interface{}{"type": "string", "description": "账本 ID 或账本名称；缺省为当前账本"},
				},
			},
			execute: s.toolQueryWallets,
		},
		{
			name:        "query_members",
			description: "查询指定账本的成员及每人在一段时间内的收支合计。缺省统计本月。",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"bookId":    map[string]interface{}{"type": "string", "description": "账本 ID 或账本名称；缺省为当前账本"},
					"period":    map[string]interface{}{"type": "string", "enum": []string{"month", "quarter", "year", "custom"}, "description": "统计周期，缺省 month"},
					"year":      map[string]interface{}{"type": "integer", "description": "年份，缺省当年"},
					"month":     map[string]interface{}{"type": "integer", "description": "月份 1-12，缺省当月"},
					"quarter":   map[string]interface{}{"type": "integer", "description": "季度 1-4"},
					"startDate": map[string]interface{}{"type": "string", "description": "开始日期 YYYY-MM-DD"},
					"endDate":   map[string]interface{}{"type": "string", "description": "结束日期 YYYY-MM-DD"},
				},
			},
			execute: s.toolQueryMembers,
		},
		{
			name:        "create_draft",
			description: "用户描述了一笔消费或收入时调用，生成一张记账草稿。只生成草稿，绝不直接入账；用户确认后才会记账。",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type":         map[string]interface{}{"type": "string", "enum": []string{"expense", "income"}, "description": "支出或收入"},
					"amountYuan":   map[string]interface{}{"type": "number", "description": "金额（元），如 18.5"},
					"title":        map[string]interface{}{"type": "string", "description": "标题，如 瑞幸咖啡"},
					"categoryName": map[string]interface{}{"type": "string", "description": "分类名称，如 餐饮"},
					"subCategory":  map[string]interface{}{"type": "string", "description": "子分类名称，如 咖啡饮品"},
					"walletName":   map[string]interface{}{"type": "string", "description": "账户名称，如 微信支付"},
					"date":         map[string]interface{}{"type": "string", "description": "日期 YYYY-MM-DD，缺省今天"},
				},
				"required": []string{"type", "amountYuan"},
			},
			execute: s.toolCreateDraft,
		},
	}
}

func (s *AiService) toolQueryStats(ctx context.Context, userID, currentBookID string, args map[string]interface{}) (interface{}, error) {
	bookID := s.resolveToolBookID(ctx, userID, currentBookID, toolStr(args, "bookId"))
	if err := s.requireBookMember(ctx, userID, bookID); err != nil {
		return nil, err
	}
	startDate, endDate, err := resolveDateRange(args)
	if err != nil {
		return nil, err
	}
	txType := toolStr(args, "type")
	if txType != "expense" && txType != "income" {
		txType = ""
	}
	categoryID := toolStr(args, "categoryId")
	if categoryID == "" && toolStr(args, "categoryName") != "" {
		categoryID, err = s.resolveCategoryID(ctx, bookID, toolStr(args, "categoryName"))
		if err != nil {
			return nil, err
		}
	} else if categoryID != "" {
		if _, err := s.resolveCategoryID(ctx, bookID, categoryID); err != nil {
			return nil, err
		}
	}
	stats, err := s.txRepo.GetRangeStats(ctx, bookID, startDate, endDate, txType, categoryID)
	if err != nil {
		return nil, err
	}
	bookName := bookID
	if b, err := s.bookRepo.GetBook(ctx, bookID, userID); err == nil && b != nil {
		bookName = b.Name
	}
	cats := make([]map[string]interface{}, 0, len(stats.Categories))
	for _, c := range stats.Categories {
		cats = append(cats, map[string]interface{}{
			"category_id": c.CategoryID, "category_name": c.CategoryName,
			"amount_cents": c.Amount, "amount_yuan": formatCentsDecimal(c.Amount),
			"count": c.Count, "percentage": c.Percentage,
		})
	}
	monthly := make([]map[string]interface{}, 0, len(stats.Monthly))
	for _, m := range stats.Monthly {
		monthly = append(monthly, map[string]interface{}{
			"month":         m.Month,
			"expense_cents": m.Expense, "expense_yuan": formatCentsDecimal(m.Expense),
			"income_cents": m.Income, "income_yuan": formatCentsDecimal(m.Income),
		})
	}
	return map[string]interface{}{
		"book_id": bookID, "book_name": bookName,
		"start_date": startDate, "end_date": endDate,
		"total_expense_cents": stats.TotalExpense, "total_expense_yuan": formatCentsDecimal(stats.TotalExpense),
		"total_income_cents": stats.TotalIncome, "total_income_yuan": formatCentsDecimal(stats.TotalIncome),
		"net_savings_cents": stats.NetSavings, "net_savings_yuan": formatCentsDecimal(stats.NetSavings),
		"categories": cats, "monthly": monthly,
	}, nil
}

func (s *AiService) toolQueryTransactions(ctx context.Context, userID, currentBookID string, args map[string]interface{}) (interface{}, error) {
	bookID := s.resolveToolBookID(ctx, userID, currentBookID, toolStr(args, "bookId"))
	if err := s.requireBookMember(ctx, userID, bookID); err != nil {
		return nil, err
	}
	categoryID := toolStr(args, "categoryId")
	if categoryID == "" && toolStr(args, "categoryName") != "" {
		var err error
		categoryID, err = s.resolveCategoryID(ctx, bookID, toolStr(args, "categoryName"))
		if err != nil {
			return nil, err
		}
	} else if categoryID != "" {
		if _, err := s.resolveCategoryID(ctx, bookID, categoryID); err != nil {
			return nil, err
		}
	}
	walletID := toolStr(args, "walletId")
	if walletID == "" && toolStr(args, "walletName") != "" {
		var err error
		walletID, err = s.resolveWalletID(ctx, bookID, toolStr(args, "walletName"))
		if err != nil {
			return nil, err
		}
	} else if walletID != "" {
		if _, err := s.resolveWalletID(ctx, bookID, walletID); err != nil {
			return nil, err
		}
	}
	memberID := toolStr(args, "memberId")
	if memberID == "" && toolStr(args, "memberName") != "" {
		var err error
		memberID, err = s.resolveMemberID(ctx, bookID, toolStr(args, "memberName"))
		if err != nil {
			return nil, err
		}
	} else if memberID != "" {
		if _, err := s.resolveMemberID(ctx, bookID, memberID); err != nil {
			return nil, err
		}
	}
	limit := 20
	if v, ok := toolInt(args, "limit"); ok && v > 0 {
		limit = v
	}
	if limit > 100 {
		limit = 100
	}
	items, total, err := s.txRepo.ListTransactions(ctx, bookID, walletID, categoryID, "", toolStr(args, "startDate"), toolStr(args, "endDate"), toolStr(args, "keyword"), limit, 0)
	if err != nil {
		return nil, err
	}
	memberNames := map[string]string{}
	if members, err := s.bookRepo.ListMembersForUser(ctx, bookID, ""); err == nil {
		for _, m := range members {
			memberNames[m.UserID] = m.UserName
		}
	}
	rows := make([]map[string]interface{}, 0, len(items))
	for _, tx := range items {
		if memberID != "" && tx.CreatedBy != memberID {
			continue
		}
		name := memberNames[tx.CreatedBy]
		if name == "" {
			name = tx.CreatedBy
		}
		rows = append(rows, map[string]interface{}{
			"date": tx.Date, "type": tx.Type, "title": tx.Title,
			"amount_cents": tx.Amount, "amount_yuan": formatCentsDecimal(tx.Amount),
			"category": tx.CategoryName, "sub_category": tx.SubCategory,
			"wallet": tx.WalletName, "member_name": name, "note": tx.Note,
		})
	}
	return map[string]interface{}{
		"total": total, "returned": len(rows), "transactions": rows,
	}, nil
}

func (s *AiService) toolQueryBudgets(ctx context.Context, userID, currentBookID string, args map[string]interface{}) (interface{}, error) {
	bookID := s.resolveToolBookID(ctx, userID, currentBookID, toolStr(args, "bookId"))
	if err := s.requireBookMember(ctx, userID, bookID); err != nil {
		return nil, err
	}
	now := time.Now()
	ov, err := s.budgetRepo.GetBudgetOverview(ctx, bookID, now.Year(), int(now.Month()))
	if err != nil {
		return nil, err
	}
	cats := make([]map[string]interface{}, 0, len(ov.Categories))
	for _, c := range ov.Categories {
		cats = append(cats, map[string]interface{}{
			"category_id": c.CategoryID, "category_name": c.CategoryName,
			"limit_cents": c.LimitAmount, "limit_yuan": formatCentsDecimal(c.LimitAmount),
			"spent_cents": c.SpentAmount, "spent_yuan": formatCentsDecimal(c.SpentAmount),
			"remaining_cents": c.RemainingAmount, "remaining_yuan": formatCentsDecimal(c.RemainingAmount),
			"percentage": c.Percentage,
		})
	}
	return map[string]interface{}{
		"month":              fmt.Sprintf("%04d-%02d", ov.Year, ov.Month),
		"total_budget_cents": ov.TotalBudget, "total_budget_yuan": formatCentsDecimal(ov.TotalBudget),
		"total_spent_cents": ov.TotalSpent, "total_spent_yuan": formatCentsDecimal(ov.TotalSpent),
		"remaining_cents": ov.RemainingBudget, "remaining_yuan": formatCentsDecimal(ov.RemainingBudget),
		"percentage": ov.Percentage,
		"categories": cats,
	}, nil
}

func (s *AiService) toolQueryWallets(ctx context.Context, userID, currentBookID string, args map[string]interface{}) (interface{}, error) {
	bookID := s.resolveToolBookID(ctx, userID, currentBookID, toolStr(args, "bookId"))
	if err := s.requireBookMember(ctx, userID, bookID); err != nil {
		return nil, err
	}
	wallets, err := s.walletRepo.ListWallets(ctx, bookID, false)
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]interface{}, 0, len(wallets))
	var total int64
	for _, w := range wallets {
		total += w.Balance
		rows = append(rows, map[string]interface{}{
			"wallet_id": w.ID, "name": w.Name, "type": w.Type,
			"balance_cents": w.Balance, "balance_yuan": formatCentsDecimal(w.Balance),
		})
	}
	return map[string]interface{}{
		"wallets":             rows,
		"total_balance_cents": total, "total_balance_yuan": formatCentsDecimal(total),
	}, nil
}

func (s *AiService) toolQueryMembers(ctx context.Context, userID, currentBookID string, args map[string]interface{}) (interface{}, error) {
	bookID := s.resolveToolBookID(ctx, userID, currentBookID, toolStr(args, "bookId"))
	if err := s.requireBookMember(ctx, userID, bookID); err != nil {
		return nil, err
	}
	startDate, endDate, err := resolveDateRange(args)
	if err != nil {
		return nil, err
	}
	members, err := s.bookRepo.ListMembersForUser(ctx, bookID, userID)
	if err != nil {
		return nil, err
	}
	totals, err := s.txRepo.GetMemberTotals(ctx, bookID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	byUser := map[string]model.MemberTotal{}
	for _, t := range totals {
		byUser[t.UserID] = t
	}
	rows := make([]map[string]interface{}, 0, len(members))
	for _, m := range members {
		t := byUser[m.UserID]
		rows = append(rows, map[string]interface{}{
			"user_id": m.UserID, "name": m.UserName, "role": m.Role, "is_me": m.IsMe,
			"expense_cents": t.Expense, "expense_yuan": formatCentsDecimal(t.Expense),
			"income_cents": t.Income, "income_yuan": formatCentsDecimal(t.Income),
			"count": t.Count,
		})
	}
	return map[string]interface{}{
		"start_date": startDate, "end_date": endDate, "members": rows,
	}, nil
}

func (s *AiService) toolCreateDraft(ctx context.Context, userID, currentBookID string, args map[string]interface{}) (interface{}, error) {
	bookID := currentBookID
	if err := s.requireBookMember(ctx, userID, bookID); err != nil {
		return nil, err
	}
	txType := toolStr(args, "type")
	if txType != "income" {
		txType = "expense"
	}
	amountYuan, ok := toolFloat(args, "amountYuan")
	if !ok || amountYuan <= 0 {
		return nil, fmt.Errorf("记账需要给出大于 0 的金额")
	}
	amtStr := fmt.Sprintf("%.2f", amountYuan)
	cents := parseYuanTextToCents(amtStr)
	if cents <= 0 {
		return nil, fmt.Errorf("记账需要给出大于 0 的金额")
	}
	date := toolStr(args, "date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	} else if _, err := time.Parse("2006-01-02", date); err != nil {
		date = time.Now().Format("2006-01-02")
	}
	catName := toolStr(args, "categoryName")
	subName := toolStr(args, "subCategory")
	if catName == "" {
		catName, subName = inferCategoryHintFromText(toolStr(args, "title"), txType == "income")
	}
	cid, cname, sub, icon, color, bg := s.resolveCategoryMatch(ctx, bookID, txType, catName, subName)
	wid, wname := s.resolveWalletMatch(ctx, bookID, toolStr(args, "walletName"))
	title := toolStr(args, "title")
	if title == "" {
		title = cname
	}
	draft := &model.AiDraft{
		Type:          txType,
		Amount:        cents,
		Title:         title,
		Note:          "",
		CategoryID:    cid,
		CategoryName:  cname,
		SubCategory:   sub,
		CategoryIcon:  icon,
		CategoryColor: color,
		BgColor:       bg,
		WalletID:      wid,
		WalletName:    wname,
		Date:          date,
	}
	return draft, nil
}

// execTool 按名执行工具；未知工具名返回错误（模型可见，可纠正）。
func (s *AiService) execTool(ctx context.Context, userID, currentBookID, name string, args map[string]interface{}) (interface{}, error) {
	for _, t := range s.agentTools() {
		if t.name == name {
			return t.execute(ctx, userID, currentBookID, args)
		}
	}
	return nil, fmt.Errorf("没有这个工具：%s", name)
}

// ---------- Agent 循环 ----------

type agentChatMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content,omitempty"`
	ToolCalls  []agentToolCall `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type agentToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type agentChatResponse struct {
	Choices []struct {
		Message struct {
			Content   string          `json:"content"`
			ToolCalls []agentToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
}

func (s *AiService) buildAgentSystemPrompt(ctx context.Context, userID, bookID string) string {
	var sb strings.Builder
	now := time.Now()
	weekdays := []string{"日", "一", "二", "三", "四", "五", "六"}
	fmt.Fprintf(&sb, "你是七喵记账 AI 助手。今天是 %s（星期%s）。\n", now.Format("2006-01-02"), weekdays[int(now.Weekday())])
	bookName := bookID
	if b, err := s.bookRepo.GetBook(ctx, bookID, userID); err == nil && b != nil {
		bookName = b.Name
	}
	fmt.Fprintf(&sb, "当前账本是「%s」。\n", bookName)
	if cats, err := s.catRepo.ListCategories(ctx, bookID, ""); err == nil {
		subs := map[string][]string{}
		for _, c := range cats {
			if c.ParentID != nil && *c.ParentID != "" {
				subs[*c.ParentID] = append(subs[*c.ParentID], c.Name)
			}
		}
		var lines []string
		for _, c := range cats {
			if c.ParentID == nil || *c.ParentID == "" {
				lines = append(lines, c.Name+"（"+strings.Join(subs[c.ID], "、")+"）")
			}
		}
		if len(lines) > 0 {
			fmt.Fprintf(&sb, "可用分类：%s。\n", strings.Join(lines, "；"))
		}
	}
	if wallets, err := s.walletRepo.ListWallets(ctx, bookID, false); err == nil && len(wallets) > 0 {
		var names []string
		for _, w := range wallets {
			names = append(names, w.Name)
		}
		fmt.Fprintf(&sb, "可用账户：%s。\n", strings.Join(names, "、"))
	}
	if members, err := s.bookRepo.ListMembersForUser(ctx, bookID, userID); err == nil && len(members) > 0 {
		var names []string
		for _, m := range members {
			n := m.UserName
			if m.IsMe {
				n += "（我）"
			}
			names = append(names, n)
		}
		fmt.Fprintf(&sb, "账本成员：%s。\n", strings.Join(names, "、"))
	}
	sb.WriteString(`规则：
1. 凡是问钱、问账、问统计的问题，必须调工具查数据库再回答；工具返回的数字就是答案，绝不自己编数字，不知道就说不知道。
2. 只能查当前账本；用户提到别的账本时，把 bookId 设为那个账本的名称去查，工具会返回无权访问，此时简短拒绝，不要编数字，也不要再尝试其他账本。
3. 用户描述一笔消费或收入时，调 create_draft 生成草稿，回答里说明已生成草稿、请核对确认；绝不要说“已经记上了”。
4. 回答用中文，金额用元，简短直接。`)
	return sb.String()
}

func (s *AiService) callModelWithTools(ctx context.Context, cfg *model.AiConfig, messages []agentChatMessage, tools []agentTool) (*agentChatResponse, error) {
	toolDefs := make([]map[string]interface{}, 0, len(tools))
	for _, t := range tools {
		toolDefs = append(toolDefs, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.name,
				"description": t.description,
				"parameters":  t.parameters,
			},
		})
	}
	payload := map[string]interface{}{
		"model":       cfg.Model,
		"temperature": cfg.Temperature,
		"messages":    messages,
		"tools":       toolDefs,
		"tool_choice": "auto",
	}
	bodyBytes, _ := json.Marshal(payload)
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp == nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("模型返回 %d：%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var apiRes agentChatResponse
	if err := json.Unmarshal(respBody, &apiRes); err != nil || len(apiRes.Choices) == 0 {
		return nil, fmt.Errorf("模型响应解析失败")
	}
	return &apiRes, nil
}

// summarizeToolResults 在工具循环触顶时，用已查到的真实结果拼一条兜底回答，不编数字。
func summarizeToolResults(calls []string, results []interface{}) string {
	var sb strings.Builder
	sb.WriteString("这个问题需要多步查询，我先把已查到的结果告诉你：\n")
	for i, name := range calls {
		b, _ := json.Marshal(results[i])
		fmt.Fprintf(&sb, "【%s】%s\n", name, string(b))
	}
	return strings.TrimSpace(sb.String())
}

// runAgentChat 跑一轮 Agent 对话。ok=false 表示 Agent 不可用（未配置模型、
// 模型不支持 tools、调用失败等），调用方应退回原有行为。
func (s *AiService) runAgentChat(ctx context.Context, cfg *model.AiConfig, userID, bookID, message string) (reply string, draft *model.AiDraft, inTok, outTok int64, ok bool) {
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.BaseURL) == "" {
		return "", nil, 0, 0, false
	}
	if strings.HasPrefix(cfg.APIKey, "sk-cat7-") {
		return "", nil, 0, 0, false
	}
	tools := s.agentTools()
	messages := []agentChatMessage{
		{Role: "system", Content: s.buildAgentSystemPrompt(ctx, userID, bookID)},
		{Role: "user", Content: message},
	}
	var toolNames []string
	var toolResults []interface{}
	for round := 0; ; round++ {
		apiRes, err := s.callModelWithTools(ctx, cfg, messages, tools)
		if err != nil {
			return "", nil, 0, 0, false
		}
		inTok += apiRes.Usage.PromptTokens
		outTok += apiRes.Usage.CompletionTokens
		msg := apiRes.Choices[0].Message
		content := strings.TrimSpace(msg.Content)
		if len(msg.ToolCalls) == 0 {
			if content == "" && draft == nil {
				return "", nil, 0, 0, false
			}
			if content == "" && draft != nil {
				content = "已生成记账草稿，请核对后确认入账。"
			}
			return content, draft, inTok, outTok, true
		}
		if round >= agentMaxToolRounds {
			return summarizeToolResults(toolNames, toolResults), draft, inTok, outTok, true
		}
		// 把带 tool_calls 的助手消息原样追加，再逐个执行工具并把结果喂回。
		messages = append(messages, agentChatMessage{Role: "assistant", Content: content, ToolCalls: msg.ToolCalls})
		for _, tc := range msg.ToolCalls {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]interface{}{}
			}
			if args == nil {
				args = map[string]interface{}{}
			}
			result, toolErr := s.execTool(ctx, userID, bookID, tc.Function.Name, args)
			var resultJSON []byte
			if toolErr != nil {
				resultJSON, _ = json.Marshal(map[string]interface{}{"error": toolErr.Error()})
			} else {
				resultJSON, _ = json.Marshal(result)
				if tc.Function.Name == "create_draft" {
					if d, isDraft := result.(*model.AiDraft); isDraft && draft == nil {
						draft = d
					}
				}
			}
			toolNames = append(toolNames, tc.Function.Name)
			toolResults = append(toolResults, result)
			messages = append(messages, agentChatMessage{
				Role:       "tool",
				Content:    string(resultJSON),
				ToolCallID: tc.ID,
			})
		}
	}
}
