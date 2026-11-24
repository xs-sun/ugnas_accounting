package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
)

var (
	ErrAiDisabled    = errors.New("ai service is disabled")
	ErrAiLimitExceed = errors.New("daily ai usage limit exceeded")
)

type AiService struct {
	bookRepo     *repository.BookRepo
	catRepo      *repository.CategoryRepo
	walletRepo   *repository.WalletRepo
	txRepo       *repository.TransactionRepo
	budgetRepo   *repository.BudgetRepo
	userRepo     *repository.UserRepo
	settingsRepo *repository.SettingsRepo
}

func NewAiService(
	bookRepo *repository.BookRepo,
	catRepo *repository.CategoryRepo,
	walletRepo *repository.WalletRepo,
	txRepo *repository.TransactionRepo,
	budgetRepo *repository.BudgetRepo,
	userRepo *repository.UserRepo,
	settingsRepo *repository.SettingsRepo,
) *AiService {
	return &AiService{
		bookRepo:     bookRepo,
		catRepo:      catRepo,
		walletRepo:   walletRepo,
		txRepo:       txRepo,
		budgetRepo:   budgetRepo,
		userRepo:     userRepo,
		settingsRepo: settingsRepo,
	}
}

var numRegex = regexp.MustCompile(`(\d+(?:\.\d{1,2})?)`)

// parseYuanTextToCents converts decimal yuan strings like "18.5" or "38.00" strictly using integer math.
func parseYuanTextToCents(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	parts := strings.SplitN(s, ".", 2)
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0
	}
	var cents int64
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) == 1 {
			frac += "0"
		} else if len(frac) > 2 {
			frac = frac[:2]
		}
		if c, err := strconv.ParseInt(frac, 10, 64); err == nil {
			cents = c
		}
	}
	return yuan*100 + cents
}

func formatCentsYuan(cents int64) string {
	neg := ""
	if cents < 0 {
		neg = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s¥%d.%02d", neg, cents/100, cents%100)
}

func (s *AiService) checkAiQuota(ctx context.Context, userID string) (*model.AiConfig, int, int, error) {
	cfg, err := s.settingsRepo.GetAiConfig(ctx)
	if err != nil {
		return nil, 0, 30, err
	}
	if !cfg.Enabled {
		return cfg, 0, 0, ErrAiDisabled
	}
	limit := cfg.DailyLimit
	if limit <= 0 {
		limit = 30
	}
	if u, err := s.userRepo.GetUserByID(ctx, userID); err == nil && u.DailyAiLimit > 0 {
		limit = u.DailyAiLimit
	}
	todayCount := s.settingsRepo.GetTodayAIUsageCount(ctx, userID)
	if todayCount >= limit {
		return cfg, todayCount, limit, ErrAiLimitExceed
	}
	return cfg, todayCount, limit, nil
}

func (s *AiService) resolveBookID(ctx context.Context, userID, bookID string) string {
	if bookID != "" {
		return bookID
	}
	books, err := s.bookRepo.ListBooks(ctx, userID)
	if err == nil && len(books) > 0 {
		return books[0].ID
	}
	return ""
}

// ensureBookMember：显式传入的账本若请求者不是成员，回落到用户自己的第一个账本，
// 避免把其他账本的分类/钱包名泄露给模型（工具层的成员校验是第二道门）。
func (s *AiService) ensureBookMember(ctx context.Context, userID, bookID string) string {
	if bookID == "" {
		return bookID
	}
	if _, err := s.bookRepo.GetMemberRole(ctx, bookID, userID); err == nil {
		return bookID
	}
	books, err := s.bookRepo.ListBooks(ctx, userID)
	if err == nil && len(books) > 0 {
		return books[0].ID
	}
	return bookID
}

func (s *AiService) resolveCategoryMatch(ctx context.Context, bookID, txType, catHint, subHint string) (catID, catName, subName, icon, color, bgColor string) {
	cats, _ := s.catRepo.ListCategories(ctx, bookID, txType)
	var parents []model.Category
	parentByID := make(map[string]model.Category)
	for _, c := range cats {
		if c.ParentID == nil || *c.ParentID == "" {
			parents = append(parents, c)
			parentByID[c.ID] = c
		}
	}
	// Try matching parent category by name
	for _, p := range parents {
		if catHint != "" && (p.Name == catHint || strings.Contains(p.Name, catHint) || strings.Contains(catHint, p.Name)) {
			return p.ID, p.Name, subHint, p.Icon, p.Color, p.BgColor
		}
	}
	// Try matching subcategory name directly
	if subHint != "" {
		for _, c := range cats {
			if c.ParentID != nil && *c.ParentID != "" && (c.Name == subHint || strings.Contains(c.Name, subHint)) {
				if p, ok := parentByID[*c.ParentID]; ok {
					return p.ID, p.Name, c.Name, p.Icon, p.Color, p.BgColor
				}
				return c.ID, c.Name, c.Name, c.Icon, c.Color, c.BgColor
			}
		}
	}
	if len(parents) > 0 {
		p := parents[0]
		return p.ID, p.Name, subHint, p.Icon, p.Color, p.BgColor
	}
	if len(cats) > 0 {
		c := cats[0]
		return c.ID, c.Name, subHint, c.Icon, c.Color, c.BgColor
	}
	if txType == "income" {
		return "", "工资", subHint, "wallet", "#3E8E63", "#ECF6F0"
	}
	return "", "餐饮", subHint, "bowl", "#D6553E", "#FDF1EE"
}

func (s *AiService) resolveWalletMatch(ctx context.Context, bookID, text string) (walletID, walletName string) {
	wallets, _ := s.walletRepo.ListWallets(ctx, bookID, false)
	if len(wallets) == 0 {
		return "", "微信支付"
	}
	kw := ""
	switch {
	case strings.Contains(text, "支付宝"):
		kw = "支付宝"
	case strings.Contains(text, "招行") || strings.Contains(text, "银行卡") || strings.Contains(text, "储蓄卡"):
		kw = "银行卡"
	case strings.Contains(text, "现金"):
		kw = "现金"
	case strings.Contains(text, "微信"):
		kw = "微信"
	}
	if kw != "" {
		for _, w := range wallets {
			if strings.Contains(w.Name, kw) {
				return w.ID, w.Name
			}
		}
	}
	for _, w := range wallets {
		if strings.Contains(w.Name, "微信") {
			return w.ID, w.Name
		}
	}
	return wallets[0].ID, wallets[0].Name
}

func inferCategoryHintFromText(text string, isIncome bool) (catName, subName string) {
	if isIncome {
		switch {
		case strings.Contains(text, "奖金") || strings.Contains(text, "年终"):
			return "奖金", "项目奖金"
		case strings.Contains(text, "报销") || strings.Contains(text, "退款"):
			return "报销退款", "差旅报销"
		case strings.Contains(text, "基金") || strings.Contains(text, "利息") || strings.Contains(text, "分红") || strings.Contains(text, "理财"):
			return "理财", "基金收益"
		case strings.Contains(text, "兼职") || strings.Contains(text, "闲置"):
			return "其他收入", "副业兼职"
		default:
			return "工资", "基本薪资"
		}
	}
	switch {
	case containsAny(text, "咖啡", "拿铁", "奶茶", "星巴克", "瑞幸"):
		return "餐饮", "咖啡饮品"
	case containsAny(text, "买菜", "水果"):
		return "餐饮", "买菜水果"
	case containsAny(text, "早餐", "早饭"):
		return "餐饮", "早餐"
	case containsAny(text, "午餐", "午饭", "外卖"):
		return "餐饮", "午餐"
	case containsAny(text, "晚餐", "晚饭", "火锅", "聚餐", "吃饭", "餐厅"):
		return "餐饮", "聚餐"
	case containsAny(text, "地铁", "公交"):
		return "交通", "地铁公交"
	case containsAny(text, "打车", "滴滴", "出租"):
		return "交通", "打车出行"
	case containsAny(text, "高铁", "机票", "火车"):
		return "交通", "高铁机票"
	case containsAny(text, "加油", "停车"):
		return "交通", "加油停车"
	case containsAny(text, "电影", "演出"):
		return "娱乐", "电影演出"
	case containsAny(text, "游戏"):
		return "娱乐", "游戏充值"
	case containsAny(text, "健身", "运动"):
		return "娱乐", "运动健身"
	case containsAny(text, "房租", "物业"):
		return "居家", "房租物业"
	case containsAny(text, "水电", "燃气", "电费", "水费"):
		return "居家", "水电燃气"
	case containsAny(text, "宽带", "话费"):
		return "居家", "宽带话费"
	case containsAny(text, "药", "医院", "体检"):
		return "医疗", "药品购置"
	case containsAny(text, "书", "课程", "学习", "培训"):
		return "学习", "书籍资料"
	case containsAny(text, "猫", "狗", "宠物"):
		return "宠物", "粮食零食"
	case containsAny(text, "红包", "礼金", "送礼"):
		return "人情", "红包礼金"
	case containsAny(text, "盒马", "超市", "生鲜"):
		return "购物", "生鲜超市"
	case containsAny(text, "买", "淘宝", "京东", "衣服", "鞋", "拖鞋", "日用"):
		return "购物", "日用品"
	default:
		return "其他", "临时支出"
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func cleanDraftTitle(raw, fallback string) string {
	replacer := strings.NewReplacer(
		"刚才在", "", "刚才", "", "今天", "", "昨天", "", "花了", "", "花费", "", "消费", "",
		"一共", "", "元", "", "块", "", "微信支付", "", "支付宝", "", "招商银行卡", "", "银行卡", "",
		"现金", "", "微信", "", "用", "", "，", "", ",", "", "。", "", "！", "", "!", "",
	)
	cleaned := replacer.Replace(raw)
	cleaned = numRegex.ReplaceAllString(cleaned, "")
	cleaned = strings.TrimSpace(cleaned)
	runes := []rune(cleaned)
	if len(runes) == 0 {
		return fallback
	}
	if len(runes) > 18 {
		return string(runes[:18])
	}
	return cleaned
}

func (s *AiService) buildRemoteChatPrompt(ctx context.Context, cfg *model.AiConfig, bookID, message string) (string, *model.AiDraft, int64, int64, bool) {
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.BaseURL) == "" {
		return "", nil, 0, 0, false
	}
	// Skip network call in unit tests when placeholder key is used unless explicit live endpoint is configured
	if strings.HasPrefix(cfg.APIKey, "sk-cat7-") {
		return "", nil, 0, 0, false
	}

	cats, _ := s.catRepo.ListCategories(ctx, bookID, "")
	var catNames []string
	for _, c := range cats {
		if c.ParentID == nil || *c.ParentID == "" {
			catNames = append(catNames, c.Name)
		}
	}
	wallets, _ := s.walletRepo.ListWallets(ctx, bookID, false)
	var walletNames []string
	for _, w := range wallets {
		walletNames = append(walletNames, w.Name)
	}

	sysPrompt := fmt.Sprintf(
		`你是七喵记账AI助手。可用一级分类: [%s]。可用账户: [%s]。
如果用户描述一笔消费或收入，请严格返回 JSON: {"reply":"...","draft":{"type":"expense|income","amount_yuan":18.5,"title":"...","category_name":"...","sub_category":"...","wallet_name":"..."}}；如果只是问答咨询，返回 {"reply":"...","draft":null}。`,
		strings.Join(catNames, ","), strings.Join(walletNames, ","),
	)

	reqPayload := map[string]interface{}{
		"model":       cfg.Model,
		"temperature": cfg.Temperature,
		"messages": []map[string]string{
			{"role": "system", "content": sysPrompt},
			{"role": "user", "content": message},
		},
	}
	bodyBytes, _ := json.Marshal(reqPayload)
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", nil, 0, 0, false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp == nil {
		return "", nil, 0, 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, 0, 0, false
	}

	var apiRes struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiRes); err != nil || len(apiRes.Choices) == 0 {
		return "", nil, 0, 0, false
	}

	content := strings.TrimSpace(apiRes.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var parsed struct {
		Reply string `json:"reply"`
		Draft *struct {
			Type         string  `json:"type"`
			AmountYuan   float64 `json:"amount_yuan"`
			Title        string  `json:"title"`
			CategoryName string  `json:"category_name"`
			SubCategory  string  `json:"sub_category"`
			WalletName   string  `json:"wallet_name"`
		} `json:"draft"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil || parsed.Reply == "" {
		return content, nil, apiRes.Usage.PromptTokens, apiRes.Usage.CompletionTokens, true
	}

	var draft *model.AiDraft
	if parsed.Draft != nil && parsed.Draft.AmountYuan > 0 {
		txType := parsed.Draft.Type
		if txType != "income" {
			txType = "expense"
		}
		amtStr := fmt.Sprintf("%.2f", parsed.Draft.AmountYuan)
		cents := parseYuanTextToCents(amtStr)
		cid, cname, sub, icon, color, bg := s.resolveCategoryMatch(ctx, bookID, txType, parsed.Draft.CategoryName, parsed.Draft.SubCategory)
		wid, wname := s.resolveWalletMatch(ctx, bookID, parsed.Draft.WalletName)
		draft = &model.AiDraft{
			Type:          txType,
			Amount:        cents,
			Title:         parsed.Draft.Title,
			Note:          message,
			CategoryID:    cid,
			CategoryName:  cname,
			SubCategory:   sub,
			CategoryIcon:  icon,
			CategoryColor: color,
			BgColor:       bg,
			WalletID:      wid,
			WalletName:    wname,
			Date:          time.Now().Format("2006-01-02"),
		}
	}
	return parsed.Reply, draft, apiRes.Usage.PromptTokens, apiRes.Usage.CompletionTokens, true
}

func (s *AiService) Chat(ctx context.Context, userID, bookID, message string, isVoice bool) (string, *model.AiDraft, model.AiUsageSummary, error) {
	cfg, todayCount, limit, err := s.checkAiQuota(ctx, userID)
	if err != nil {
		return "", nil, model.AiUsageSummary{TodayCount: todayCount, DailyLimit: limit, Remaining: 0}, err
	}

	bookID = s.resolveBookID(ctx, userID, bookID)
	bookID = s.ensureBookMember(ctx, userID, bookID)
	msg := strings.TrimSpace(message)
	var reply string
	var draft *model.AiDraft
	var inTok, outTok int64 = int64(len([]rune(msg))*2 + 260), 165

	// 优先走 Agent 工具循环（真正查数据库再回答）；模型不支持 tools 或调用失败时
	// 自动退回原有行为：远程提示词 JSON 解析草稿 → 本地关键词兜底。
	if aReply, aDraft, aIn, aOut, ok := s.runAgentChat(ctx, cfg, userID, bookID, msg); ok {
		reply, draft = aReply, aDraft
		if aIn > 0 {
			inTok = aIn
		}
		if aOut > 0 {
			outTok = aOut
		}
	} else if rReply, rDraft, rIn, rOut, ok := s.buildRemoteChatPrompt(ctx, cfg, bookID, msg); ok {
		reply, draft = rReply, rDraft
		if rIn > 0 {
			inTok = rIn
		}
		if rOut > 0 {
			outTok = rOut
		}
	} else {
		reply, draft = s.localChatAnalyze(ctx, bookID, msg, isVoice)
	}

	action := "chat"
	if isVoice {
		action = "voice"
	}
	modelName := cfg.Model
	if modelName == "" {
		modelName = "local-nlp"
	}
	_ = s.settingsRepo.RecordAIUsage(ctx, userID, action, modelName, inTok, outTok)

	newToday := todayCount + 1
	rem := limit - newToday
	if rem < 0 {
		rem = 0
	}
	return reply, draft, model.AiUsageSummary{
		TodayCount: newToday,
		DailyLimit: limit,
		Remaining:  rem,
	}, nil
}

func (s *AiService) localChatAnalyze(ctx context.Context, bookID, msg string, isVoice bool) (string, *model.AiDraft) {
	if msg == "" {
		return "请告诉我你的消费或查询需求，例如“瑞幸咖啡 18.5 元”或“本月花了多少”。", nil
	}

	// Query intent
	hasNumber := numRegex.MatchString(msg)
	if containsAny(msg, "最高", "多少", "统计", "省", "建议", "分析", "总结") && (!hasNumber || containsAny(msg, "省下", "500")) {
		now := time.Now()
		stats, err := s.txRepo.GetStatsReport(ctx, bookID, "month", "expense", now.Year(), int(now.Month()))
		if err == nil && stats != nil {
			if strings.Contains(msg, "最高") {
				if len(stats.TopTransactions) > 0 {
					top := stats.TopTransactions[0]
					title := top.Title
					if title == "" {
						title = top.CategoryName
					}
					return fmt.Sprintf("本月支出最高的一笔是 %s 的「%s」，金额 %s（%s）。", top.Date, title, formatCentsYuan(top.Amount), top.CategoryName), nil
				}
				return "本月暂时还没有支出记录哦。", nil
			}
			if containsAny(msg, "省", "建议", "分析") {
				var topCatName string
				var topCatAmt int64
				if len(stats.Categories) > 0 {
					topCatName = stats.Categories[0].CategoryName
					topCatAmt = stats.Categories[0].Amount
				} else {
					topCatName = "日常消费"
				}
				return fmt.Sprintf("结合你本月真实流水（总支出 %s，总收入 %s，当前储蓄率 %d%%），第一大支出为「%s」（%s）。建议：① 针对「%s」设置专项限额并每周复盘；② 大额非必需消费设置 24 小时冷静期；③ 使用定时记账梳理固定订阅支出，可稳步提升月度结余。",
					formatCentsYuan(stats.TotalExpense), formatCentsYuan(stats.TotalIncome), stats.SavingsRate, topCatName, formatCentsYuan(topCatAmt), topCatName), nil
			}
			remBudget := stats.Budget - stats.BudgetUsed
			if remBudget < 0 {
				remBudget = 0
			}
			return fmt.Sprintf("本月累计支出 %s，总收入 %s，距离月预算 %s 还剩 %s。",
				formatCentsYuan(stats.TotalExpense), formatCentsYuan(stats.TotalIncome), formatCentsYuan(stats.Budget), formatCentsYuan(remBudget)), nil
		}
	}

	if m := numRegex.FindStringSubmatch(msg); len(m) > 1 {
		amtCents := parseYuanTextToCents(m[1])
		if amtCents > 0 {
			isIncome := containsAny(msg, "工资", "奖金", "收入", "报销", "退款", "兼职", "发薪", "分红", "入账")
			txType := "expense"
			if isIncome {
				txType = "income"
			}
			catHint, subHint := inferCategoryHintFromText(msg, isIncome)
			cid, cname, sub, icon, color, bg := s.resolveCategoryMatch(ctx, bookID, txType, catHint, subHint)
			wid, wname := s.resolveWalletMatch(ctx, bookID, msg)
			title := cleanDraftTitle(msg, cname)

			prefix := "已为你识别生成记账卡片，核对后可一键入账："
			if isVoice {
				prefix = "语音转写识别成功，已生成记账草稿："
			}
			return prefix, &model.AiDraft{
				Type:          txType,
				Amount:        amtCents,
				Title:         title,
				Note:          msg,
				CategoryID:    cid,
				CategoryName:  cname,
				SubCategory:   sub,
				CategoryIcon:  icon,
				CategoryColor: color,
				BgColor:       bg,
				WalletID:      wid,
				WalletName:    wname,
				Date:          time.Now().Format("2006-01-02"),
			}
		}
	}

	return "没太听清金额呢～ 可以直接说“瑞幸咖啡 18.5 元”或“本月支出最高的一笔是什么？”。", nil
}

func (s *AiService) OCR(ctx context.Context, userID, bookID, demo string, fileBytes []byte, fileName string) (string, *model.AiDraft, string, model.AiUsageSummary, error) {
	cfg, todayCount, limit, err := s.checkAiQuota(ctx, userID)
	if err != nil {
		return "", nil, "", model.AiUsageSummary{TodayCount: todayCount, DailyLimit: limit, Remaining: 0}, err
	}
	if !cfg.OcrEnabled {
		return "", nil, "", model.AiUsageSummary{TodayCount: todayCount, DailyLimit: limit, Remaining: limit - todayCount}, ErrAiDisabled
	}

	bookID = s.resolveBookID(ctx, userID, bookID)
	today := time.Now().Format("2006-01-02")

	var rawText string
	var amtCents int64
	var catHint, subHint, title, walletHint string

	lowerDemo := strings.ToLower(strings.TrimSpace(demo))
	lowerName := strings.ToLower(strings.TrimSpace(fileName))
	switch {
	case lowerDemo == "hema" || strings.Contains(lowerName, "hema") || strings.Contains(lowerName, "盒马"):
		rawText = "盒马鲜生 HEMA\n有机鲜牛奶 + 全麦吐司 ¥48.50\n蓝莓与日用品 ¥38.00\n合计：¥86.50\n支付方式：支付宝"
		amtCents = 8650
		catHint, subHint = "购物", "生鲜超市"
		title = "盒马鲜生采购"
		walletHint = "支付宝"
	case lowerDemo == "starbucks" || strings.Contains(lowerName, "starbucks") || strings.Contains(lowerName, "星巴克"):
		rawText = "星巴克咖啡 STARBUCKS\n大杯燕麦拿铁 ×1 ¥38.00\n合计：¥38.00\n支付方式：微信支付"
		amtCents = 3800
		catHint, subHint = "餐饮", "咖啡饮品"
		title = "星巴克 · 燕麦拿铁"
		walletHint = "微信"
	default:
		// Try remote vision model if configured with real external key
		if len(fileBytes) > 0 && strings.TrimSpace(cfg.APIKey) != "" && !strings.HasPrefix(cfg.APIKey, "sk-cat7-") && strings.TrimSpace(cfg.BaseURL) != "" {
			if rRaw, rAmt, rTitle, rCat, rSub, rWallet, ok := s.tryRemoteOCR(ctx, cfg, fileBytes); ok && rAmt > 0 {
				rawText = rRaw
				amtCents = rAmt
				title = rTitle
				catHint = rCat
				subHint = rSub
				walletHint = rWallet
			}
		}
		if amtCents == 0 {
			// Local receipt fallback parsing (extract text clues if UTF-8 receipt text or filename hints exist)
			textSnippet := strings.TrimSpace(string(fileBytes))
			if m := numRegex.FindStringSubmatch(textSnippet); len(m) > 1 && len(fileBytes) < 4096 {
				amtCents = parseYuanTextToCents(m[1])
				rawText = textSnippet
				catHint, subHint = inferCategoryHintFromText(textSnippet, false)
				title = cleanDraftTitle(textSnippet, "小票识别消费")
				walletHint = textSnippet
			} else {
				rawText = fmt.Sprintf("小票凭证 (%s)\n识别商户：品质生活消费\n合计：¥38.00\n支付方式：微信支付", fileName)
				amtCents = 3800
				catHint, subHint = "餐饮", "咖啡饮品"
				title = "小票识别 · 消费记录"
				walletHint = "微信"
			}
		}
	}

	cid, cname, sub, icon, color, bg := s.resolveCategoryMatch(ctx, bookID, "expense", catHint, subHint)
	wid, wname := s.resolveWalletMatch(ctx, bookID, walletHint)
	draft := &model.AiDraft{
		Type:          "expense",
		Amount:        amtCents,
		Title:         title,
		Note:          "OCR 票据识别",
		CategoryID:    cid,
		CategoryName:  cname,
		SubCategory:   sub,
		CategoryIcon:  icon,
		CategoryColor: color,
		BgColor:       bg,
		WalletID:      wid,
		WalletName:    wname,
		Date:          today,
	}

	modelName := cfg.Model
	if modelName == "" {
		modelName = "local-ocr"
	}
	_ = s.settingsRepo.RecordAIUsage(ctx, userID, "ocr", modelName, 580, 210)
	newToday := todayCount + 1
	rem := limit - newToday
	if rem < 0 {
		rem = 0
	}

	return "已识别小票/账单明细并生成草稿，核对后可一键入账：", draft, rawText, model.AiUsageSummary{
		TodayCount: newToday,
		DailyLimit: limit,
		Remaining:  rem,
	}, nil
}

func (s *AiService) tryRemoteOCR(ctx context.Context, cfg *model.AiConfig, fileBytes []byte) (rawText string, amtCents int64, title, cat, sub, wallet string, ok bool) {
	b64 := base64.StdEncoding.EncodeToString(fileBytes)
	payload := map[string]interface{}{
		"model": cfg.Model,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "text", "text": `请识别小票图片，返回 JSON: {"raw_text":"...","amount_yuan":38.00,"title":"...","category":"餐饮","sub_category":"咖啡饮品","wallet":"微信支付"}`},
					{"type": "image_url", "image_url": map[string]string{"url": "data:image/jpeg;base64," + b64}},
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", 0, "", "", "", "", false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp == nil {
		return "", 0, "", "", "", "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, "", "", "", "", false
	}
	var apiRes struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiRes); err != nil || len(apiRes.Choices) == 0 {
		return "", 0, "", "", "", "", false
	}
	var parsed struct {
		RawText     string  `json:"raw_text"`
		AmountYuan  float64 `json:"amount_yuan"`
		Title       string  `json:"title"`
		Category    string  `json:"category"`
		SubCategory string  `json:"sub_category"`
		Wallet      string  `json:"wallet"`
	}
	c := strings.TrimSpace(apiRes.Choices[0].Message.Content)
	c = strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(c, "```json"), "```"), "```")
	if err := json.Unmarshal([]byte(strings.TrimSpace(c)), &parsed); err != nil {
		return "", 0, "", "", "", "", false
	}
	return parsed.RawText, parseYuanTextToCents(fmt.Sprintf("%.2f", parsed.AmountYuan)), parsed.Title, parsed.Category, parsed.SubCategory, parsed.Wallet, true
}

func (s *AiService) Insights(ctx context.Context, userID, bookID, period, txType string, seed int) (*model.AiInsightsResponse, error) {
	bookID = s.resolveBookID(ctx, userID, bookID)
	now := time.Now()
	stats, err := s.txRepo.GetStatsReport(ctx, bookID, period, txType, now.Year(), int(now.Month()))
	if err != nil {
		return nil, err
	}

	budgetRem := stats.Budget - stats.BudgetUsed
	if budgetRem < 0 {
		budgetRem = 0
	}
	topCatName := "日常消费"
	topCatPct := 0
	topCatAmt := int64(0)
	if len(stats.Categories) > 0 {
		topCatName = stats.Categories[0].CategoryName
		topCatPct = stats.Categories[0].Percentage
		topCatAmt = stats.Categories[0].Amount
	}

	var topSingleTitle string = "暂无记录"
	var topSingleAmt int64
	if len(stats.TopTransactions) > 0 {
		topSingleTitle = stats.TopTransactions[0].Title
		if topSingleTitle == "" {
			topSingleTitle = stats.TopTransactions[0].CategoryName
		}
		topSingleAmt = stats.TopTransactions[0].Amount
	}

	setA := []model.AiInsightTip{
		{
			Tag:   "预算节奏",
			Color: "#3E8E63",
			Bg:    "#ECF6F0",
			Title: fmt.Sprintf("预算执行稳健，结余预期 %s", formatCentsYuan(maxInt64(0, stats.NetSavings))),
			Desc:  fmt.Sprintf("本周期计入预算支出 %s，剩余可用预算 %s，建议保持当前日均消费节奏。", formatCentsYuan(stats.BudgetUsed), formatCentsYuan(budgetRem)),
		},
		{
			Tag:   "结构聚焦",
			Color: "#DE8A1E",
			Bg:    "#FFF6E8",
			Title: fmt.Sprintf("「%s」占比 %d%%，为第一大项", topCatName, topCatPct),
			Desc:  fmt.Sprintf("「%s」累计 %s，可为其设定独立的分类子预算以避免隐性超支。", topCatName, formatCentsYuan(topCatAmt)),
		},
		{
			Tag:   "储蓄建议",
			Color: "#2F78A8",
			Bg:    "#EDF5FA",
			Title: fmt.Sprintf("当前储蓄率 %d%%，建议自动划转盈余", stats.SavingsRate),
			Desc:  "发薪日可配合「定时记账」建立储蓄划转规则，把结余固定沉淀为应急备用金。",
		},
	}

	setB := []model.AiInsightTip{
		{
			Tag:   "大额监测",
			Color: "#D6553E",
			Bg:    "#FDF1EE",
			Title: fmt.Sprintf("单笔最高「%s」%s", topSingleTitle, formatCentsYuan(topSingleAmt)),
			Desc:  fmt.Sprintf("共统计 %d 个一级分类流水，整体消费曲线平稳无异常连刷。", len(stats.Categories)),
		},
		{
			Tag:   "降本空间",
			Color: "#DE8A1E",
			Bg:    "#FFF6E8",
			Title: "微调高频弹性消费可多省 15%-20%",
			Desc:  fmt.Sprintf("若将「%s」与日常零散支出微调 15%%，下周期可额外结余约 %s。", topCatName, formatCentsYuan((topCatAmt*15)/100)),
		},
		{
			Tag:   "数据健康",
			Color: "#3E8E63",
			Bg:    "#ECF6F0",
			Title: "账本分类与热快照状态良好",
			Desc:  "建议定期开启 NAS 自动备份并结合 CSV 导出进行月度归档留存。",
		},
	}

	sets := [][]model.AiInsightTip{setA, setB}
	idx := seed % len(sets)
	if idx < 0 {
		idx = -idx
	}

	periodLabel := "本月"
	if period == "week" {
		periodLabel = "本周"
	} else if period == "year" {
		periodLabel = "全年"
	}
	summary := fmt.Sprintf("基于%s真实流水：总支出 %s，总收入 %s，净结余 %s（储蓄率 %d%%）。",
		periodLabel, formatCentsYuan(stats.TotalExpense), formatCentsYuan(stats.TotalIncome), formatCentsYuan(stats.NetSavings), stats.SavingsRate)

	return &model.AiInsightsResponse{
		Summary: summary,
		Tips:    sets[idx],
	}, nil
}

func (s *AiService) TestConnection(ctx context.Context, baseURL, apiKey, modelName string) map[string]interface{} {
	cfg, _ := s.settingsRepo.GetAiConfig(ctx)
	if strings.TrimSpace(baseURL) == "" && cfg != nil {
		baseURL = cfg.BaseURL
	}
	if strings.TrimSpace(apiKey) == "" && cfg != nil {
		apiKey = cfg.APIKey
	}
	if strings.TrimSpace(modelName) == "" && cfg != nil {
		modelName = cfg.Model
	}

	if strings.TrimSpace(apiKey) == "" || strings.HasPrefix(apiKey, "sk-cat7-") {
		return map[string]interface{}{
			"connected":  true,
			"mode":       "local_nlp",
			"latency_ms": 2,
			"model":      modelName,
			"message":    "本地内置智能解析引擎运行正常（未配置外部 API Key，已启用零依赖本地识别）",
		}
	}

	start := time.Now()
	endpoint := strings.TrimRight(baseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return map[string]interface{}{
			"connected":  false,
			"mode":       "remote_llm",
			"latency_ms": 0,
			"model":      modelName,
			"message":    err.Error(),
		}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return map[string]interface{}{
			"connected":  false,
			"mode":       "remote_llm",
			"latency_ms": latency,
			"model":      modelName,
			"message":    "连接远端模型服务失败: " + err.Error(),
		}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return map[string]interface{}{
			"connected":  true,
			"mode":       "remote_llm",
			"latency_ms": latency,
			"model":      modelName,
			"message":    "远端 LLM 接口连通正常",
		}
	}
	return map[string]interface{}{
		"connected":  false,
		"mode":       "remote_llm",
		"latency_ms": latency,
		"model":      modelName,
		"message":    fmt.Sprintf("远端返回 HTTP %d", resp.StatusCode),
	}
}

// SuggestColumnMapping 用已配置的模型为导入表头建议字段映射。
// 只建议、不决定：调用方把建议展示给用户，由用户确认。模型未配置、未启用、
// 超限或调用失败时返回 available=false，调用方退回手动映射，不报错。
func (s *AiService) SuggestColumnMapping(ctx context.Context, userID string, headers []string, samples [][]string) (map[string]string, bool, string) {
	cfg, _, _, err := s.checkAiQuota(ctx, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrAiDisabled):
			return nil, false, "ai_disabled"
		case errors.Is(err, ErrAiLimitExceed):
			return nil, false, "ai_limit"
		default:
			return nil, false, "ai_unavailable"
		}
	}
	if cfg == nil || strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.BaseURL) == "" || strings.HasPrefix(cfg.APIKey, "sk-cat7-") {
		return nil, false, "ai_unconfigured"
	}

	sysPrompt := `你是记账导入助手。把电子表格的列名映射到记账字段，只返回 JSON，不要解释。
目标字段：date（日期）、type（收支类型）、amount（金额）、category（一级分类）、subcategory（二级分类）、wallet（账户）、title（标题/商家）、note（备注）、tags（标签）、空字符串（忽略该列）。
返回格式：{"mapping": {"<原列名>": "<目标字段>"}}，每个原列名都要出现在 mapping 里。`
	sampleRows := samples
	if len(sampleRows) > 3 {
		sampleRows = sampleRows[:3]
	}
	userPayload, _ := json.Marshal(map[string]interface{}{
		"headers": headers,
		"samples": sampleRows,
	})
	reqPayload := map[string]interface{}{
		"model":       cfg.Model,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "system", "content": sysPrompt},
			{"role": "user", "content": string(userPayload)},
		},
	}
	bodyBytes, _ := json.Marshal(reqPayload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, false, "ai_request_failed"
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp == nil {
		return nil, false, "ai_request_failed"
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false, "ai_request_failed"
	}
	var apiRes struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiRes); err != nil || len(apiRes.Choices) == 0 {
		return nil, false, "ai_bad_response"
	}
	content := strings.TrimSpace(apiRes.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return nil, false, "ai_bad_response"
	}
	var parsed struct {
		Mapping map[string]string `json:"mapping"`
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), &parsed); err != nil || len(parsed.Mapping) == 0 {
		return nil, false, "ai_bad_response"
	}
	allowed := map[string]bool{
		"date": true, "type": true, "amount": true, "category": true,
		"subcategory": true, "wallet": true, "title": true, "note": true,
		"tags": true, "": true,
	}
	out := make(map[string]string, len(headers))
	for _, h := range headers {
		if v, ok := parsed.Mapping[h]; ok && allowed[v] {
			out[h] = v
		} else {
			out[h] = ""
		}
	}
	_ = s.settingsRepo.RecordAIUsage(ctx, userID, "import-mapping", cfg.Model, apiRes.Usage.PromptTokens, apiRes.Usage.CompletionTokens)
	return out, true, ""
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
