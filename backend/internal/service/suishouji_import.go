package service

// 随手记导入：表头自适应的通用表格导入。
//
// 随手记导出文件的确切表头无法从公开资料核实（且可能随版本变化），
// 所以这里不写死列名：先解析出表头 + 原始行，再按别名字典自动映射，
// 映射不上的列交给 AI 建议，最后由用户在界面上确认。等拿到真实导出文件
// 后，只需往 columnAliases 里补别名即可校准。

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// MaxSuishoujiRows 单文件最多导入行数，超了提示分批。
const MaxSuishoujiRows = 2000

// ColumnTarget 导入目标字段；空字符串表示忽略该列。
type ColumnTarget string

const (
	TargetDate        ColumnTarget = "date"
	TargetType        ColumnTarget = "type"
	TargetAmount      ColumnTarget = "amount"
	TargetCategory    ColumnTarget = "category"
	TargetSubCategory ColumnTarget = "subcategory"
	TargetWallet      ColumnTarget = "wallet"
	TargetTitle       ColumnTarget = "title"
	TargetNote        ColumnTarget = "note"
	TargetTags        ColumnTarget = "tags"
	TargetIgnore      ColumnTarget = ""
)

// ColumnTargetLabels 目标字段的中文名（前端下拉框用）。
var ColumnTargetLabels = map[ColumnTarget]string{
	TargetDate:        "日期",
	TargetType:        "收支类型",
	TargetAmount:      "金额",
	TargetCategory:    "一级分类",
	TargetSubCategory: "二级分类",
	TargetWallet:      "账户",
	TargetTitle:       "标题/商家",
	TargetNote:        "备注",
	TargetTags:        "标签",
	TargetIgnore:      "忽略此列",
}

// AllColumnTargets 下拉框里的可选项顺序。
var AllColumnTargets = []ColumnTarget{
	TargetDate, TargetType, TargetAmount, TargetCategory, TargetSubCategory,
	TargetWallet, TargetTitle, TargetNote, TargetTags, TargetIgnore,
}

// columnAliases 表头别名 → 目标字段。匹配时先精确、再包含。
// 随手记各版本表头写法不一，这里只放通用写法；拿到真实文件后按实际表头补。
var columnAliases = map[ColumnTarget][]string{
	TargetDate:        {"日期", "交易时间", "记账日期", "时间", "交易日期", "账单日期", "date", "datetime", "交易时间戳"},
	TargetType:        {"收支类型", "类型", "收支", "借贷方向", "方向", "type", "收支类别"},
	TargetAmount:      {"金额", "收支金额", "交易金额", "amount", "金额（元）", "金额(元）", "金额(元)", "实付金额", "消费金额"},
	TargetCategory:    {"分类", "一级分类", "类别", "category", "一级类别", "消费分类"},
	TargetSubCategory: {"二级分类", "子分类", "subcategory", "二级类别", "子类别", "明细分类"},
	TargetWallet:      {"账户", "钱包", "支付方式", "账户名称", "资金账户", "account", "支付账户", "付款方式", "来源账户"},
	TargetTitle:       {"标题", "商品", "交易对方", "对方", "商家", "商户", "名称", "title", "交易名称", "商品名称", "对方名称", "交易对象"},
	TargetNote:        {"备注", "说明", "备注信息", "note", "附言", "备注说明", "交易备注"},
	TargetTags:        {"标签", "tag", "tags", "标签名称"},
}

// SuishoujiTable 解析后的原始表格：表头 + 数据行（不含表头行），单元格均为字符串。
type SuishoujiTable struct {
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

// SuishoujiPreviewRow 预览行：映射后的解析结果（含分类/钱包匹配结果）。
type SuishoujiPreviewRow struct {
	RowIndex     int    `json:"row_index"` // 从 1 开始的数据行号（不含表头）
	Date         string `json:"date"`
	TxTime       string `json:"tx_time"`
	Type         string `json:"type"`
	AmountCents  int64  `json:"amount_cents"`
	CategoryName string `json:"category_name"`
	SubCategory  string `json:"sub_category"`
	WalletName   string `json:"wallet_name"`
	Title        string `json:"title"`
	Note         string `json:"note"`
	Valid        bool   `json:"valid"`
	Error        string `json:"error,omitempty"`
}

// SuishoujiRowFailure 导入失败行的明细。
type SuishoujiRowFailure struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

// SuishoujiCommitResult 导入执行汇总。
type SuishoujiCommitResult struct {
	Inserted   int                   `json:"inserted"`
	Duplicates int                   `json:"duplicates"`
	Failed     int                   `json:"failed"`
	Failures   []SuishoujiRowFailure `json:"failures"`
}

func normalizeHeader(h string) string {
	h = strings.TrimSpace(h)
	h = strings.ReplaceAll(h, " ", "")
	h = strings.ReplaceAll(h, "　", "")
	return strings.ToLower(h)
}

// AutoMapColumns 按别名字典自动映射表头。未命中的列返回空（忽略），由 AI 建议或用户手动选。
func AutoMapColumns(headers []string) map[string]ColumnTarget {
	mapping := make(map[string]ColumnTarget, len(headers))
	used := make(map[ColumnTarget]bool)
	// 第一轮：精确匹配（归一化后相等）。
	for _, h := range headers {
		nh := normalizeHeader(h)
		if nh == "" {
			mapping[h] = TargetIgnore
			continue
		}
		for target, aliases := range columnAliases {
			for _, a := range aliases {
				if nh == normalizeHeader(a) {
					mapping[h] = target
					used[target] = true
					break
				}
			}
			if _, ok := mapping[h]; ok {
				break
			}
		}
	}
	// 第二轮：包含匹配，只给还没映射上的列，且一个目标字段只占一列。
	for _, h := range headers {
		if _, ok := mapping[h]; ok {
			continue
		}
		nh := normalizeHeader(h)
		for target, aliases := range columnAliases {
			if used[target] {
				continue
			}
			matched := false
			for _, a := range aliases {
				na := normalizeHeader(a)
				if na != "" && (strings.Contains(nh, na) || strings.Contains(na, nh)) {
					mapping[h] = target
					used[target] = true
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if _, ok := mapping[h]; !ok {
			mapping[h] = TargetIgnore
		}
	}
	return mapping
}

// DefaultWalletID 返回账本第一个可用钱包的 ID（导入的默认入账钱包）。
func (s *ImportService) DefaultWalletID(ctx context.Context, bookID string) (string, error) {
	wallets, err := s.walletRepo.ListWallets(ctx, bookID, false)
	if err != nil {
		return "", err
	}
	if len(wallets) == 0 {
		return "", errors.New("book has no wallets")
	}
	return wallets[0].ID, nil
}

// ParseSuishoujiFile 按扩展名（未知时按内容嗅探）解析上传文件。
// 支持 .xlsx / .xls / .csv；返回表头 + 原始数据行。
func (s *ImportService) ParseSuishoujiFile(rawBytes []byte, filename string) (*SuishoujiTable, error) {
	if len(rawBytes) == 0 {
		return nil, errors.New("empty file")
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch {
	case ext == ".xlsx" || isZipMagic(rawBytes):
		return parseXLSXTable(rawBytes)
	case ext == ".xls" || isOleMagic(rawBytes):
		return parseXLSTable(rawBytes)
	case ext == ".csv" || ext == ".txt" || ext == "":
		return parseCSVTable(rawBytes)
	default:
		return nil, fmt.Errorf("unsupported file type %q, only .xlsx / .xls / .csv are supported", ext)
	}
}

func isZipMagic(b []byte) bool {
	return len(b) >= 4 && b[0] == 'P' && b[1] == 'K' && b[2] == 0x03 && b[3] == 0x04
}

func isOleMagic(b []byte) bool {
	return len(b) >= 8 && b[0] == 0xD0 && b[1] == 0xCF && b[2] == 0x11 && b[3] == 0xE0 &&
		b[4] == 0xA1 && b[5] == 0xB1 && b[6] == 0x1A && b[7] == 0xE1
}

// normalizeTable 统一整理：去全空行、定表头、按表头宽度对齐各行。
func normalizeTable(rawRows [][]string) (*SuishoujiTable, error) {
	var rows [][]string
	for _, r := range rawRows {
		cleaned := make([]string, len(r))
		empty := true
		for i, c := range r {
			cleaned[i] = strings.TrimSpace(c)
			if cleaned[i] != "" {
				empty = false
			}
		}
		if !empty {
			rows = append(rows, cleaned)
		}
	}
	if len(rows) == 0 {
		return nil, errors.New("no data rows found in file")
	}
	headers := rows[0]
	// 表头去重：重名列加后缀，保证映射 key 唯一。
	seen := make(map[string]int)
	for i, h := range headers {
		if h == "" {
			headers[i] = fmt.Sprintf("列%d", i+1)
			continue
		}
		if n, ok := seen[h]; ok {
			seen[h] = n + 1
			headers[i] = fmt.Sprintf("%s(%d)", h, n+1)
		} else {
			seen[h] = 1
		}
	}
	width := len(headers)
	data := make([][]string, 0, len(rows)-1)
	for _, r := range rows[1:] {
		if len(r) < width {
			padded := make([]string, width)
			copy(padded, r)
			r = padded
		} else if len(r) > width {
			r = r[:width]
		}
		data = append(data, r)
	}
	if len(data) == 0 {
		return nil, errors.New("file has headers but no data rows")
	}
	return &SuishoujiTable{Headers: headers, Rows: data}, nil
}

func parseCSVTable(rawBytes []byte) (*SuishoujiTable, error) {
	if bytes.HasPrefix(rawBytes, []byte{0xef, 0xbb, 0xbf}) {
		rawBytes = rawBytes[3:]
	}
	var text string
	if isUTF8(rawBytes) {
		text = string(rawBytes)
	} else {
		reader := transform.NewReader(bytes.NewReader(rawBytes), simplifiedchinese.GBK.NewDecoder())
		decoded, err := io.ReadAll(reader)
		if err == nil {
			text = string(decoded)
		} else {
			text = string(rawBytes)
		}
	}
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}
	return normalizeTable(rows)
}

func parseXLSXTable(rawBytes []byte) (*SuishoujiTable, error) {
	f, err := excelize.OpenReader(bytes.NewReader(rawBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to open xlsx: %w", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("xlsx has no sheets")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("failed to read xlsx sheet: %w", err)
	}
	return normalizeTable(rows)
}

func parseXLSTable(rawBytes []byte) (tbl *SuishoujiTable, err error) {
	// extrame/xls 在稀疏行上可能 panic（Row() 对缺失行解引用），这里兜底转成错误。
	defer func() {
		if r := recover(); r != nil {
			tbl = nil
			err = fmt.Errorf("failed to parse xls: %v", r)
		}
	}()
	wb, err := xls.OpenReader(bytes.NewReader(rawBytes), "utf-8")
	if err != nil {
		return nil, fmt.Errorf("failed to open xls: %w", err)
	}
	if wb.NumSheets() == 0 {
		return nil, errors.New("xls has no sheets")
	}
	sheet := wb.GetSheet(0)
	var rows [][]string
	for i := 0; i <= int(sheet.MaxRow); i++ {
		row := sheet.Row(i)
		if row == nil {
			rows = append(rows, []string{})
			continue
		}
		last := row.LastCol()
		if last < 0 {
			last = 0
		}
		cells := make([]string, 0, last+1)
		for j := 0; j <= last; j++ {
			cells = append(cells, row.Col(j))
		}
		rows = append(rows, cells)
	}
	return normalizeTable(rows)
}

var dateLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
	"2006/01/02 15:04:05",
	"2006/01/02 15:04",
	"2006/01/02",
	"2006.01.02",
	"2006-1-2",
	"2006/1/2",
	"2006年01月02日",
	"2006年1月2日",
	"2006年01月02日 15:04:05",
	"2006年01月02日 15:04",
}

// parseImportDate 解析日期单元格，返回 YYYY-MM-DD 和 HH:MM。
// 还兼容 Excel 序列号（纯数字 20000~80000 视为 1899-12-30 起的天数）。
func parseImportDate(val string) (string, string, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return "", "", errors.New("empty date")
	}
	// 去掉毫秒部分。
	if len(val) > 19 && val[19] == '.' {
		val = val[:19]
	}
	if num, err := strconv.ParseFloat(val, 64); err == nil && num >= 20000 && num <= 80000 && !strings.ContainsAny(val, "-/.") {
		base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		d := base.AddDate(0, 0, int(num))
		return d.Format("2006-01-02"), "12:00", nil
	}
	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, val, time.Local); err == nil {
			txTime := "12:00"
			if strings.Contains(layout, "15:04") {
				txTime = t.Format("15:04")
			}
			return t.Format("2006-01-02"), txTime, nil
		}
	}
	return "", "", fmt.Errorf("unrecognized date %q", val)
}

var parenAmountRe = regexp.MustCompile(`^\((.*)\)$`)

// parseImportAmount 解析金额单元格：去 ¥/￥/逗号/元，括号表示负数。
func parseImportAmount(val string) (int64, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, errors.New("empty amount")
	}
	neg := false
	if m := parenAmountRe.FindStringSubmatch(val); m != nil {
		neg = true
		val = m[1]
	}
	val = strings.ReplaceAll(val, ",", "")
	val = strings.ReplaceAll(val, "，", "")
	val = strings.ReplaceAll(val, " ", "")
	cents := parseAmountCents(val)
	if cents == 0 {
		if f, err := strconv.ParseFloat(strings.TrimPrefix(val, "¥"), 64); err != nil || f == 0 {
			return 0, fmt.Errorf("invalid amount %q", val)
		}
	}
	if neg {
		cents = -cents
	}
	return cents, nil
}

// parseImportType 解析收支类型列。空值时按金额正负推断：负数=支出，否则=支出。
// 转账返回错误（暂不支持导入转账，由调用方记入失败明细）。
func parseImportType(val string, amountCents int64) (string, error) {
	val = strings.TrimSpace(val)
	lower := strings.ToLower(val)
	switch {
	case strings.Contains(val, "收入") || lower == "income" || lower == "in":
		return "income", nil
	case strings.Contains(val, "支出") || lower == "expense" || lower == "out":
		return "expense", nil
	case strings.Contains(val, "转账") || lower == "transfer":
		return "", errors.New("transfer rows are not supported yet")
	case val == "":
		if amountCents < 0 {
			return "expense", nil
		}
		return "expense", nil
	default:
		return "", fmt.Errorf("unrecognized type %q", val)
	}
}

// rowValue 按映射取某目标字段在行里的原始值。
func rowValue(table *SuishoujiTable, mapping map[string]ColumnTarget, row []string, target ColumnTarget) string {
	for i, h := range table.Headers {
		if mapping[h] == target && i < len(row) {
			return strings.TrimSpace(row[i])
		}
	}
	return ""
}

// resolveImportCategory 按随手记分类名匹配七喵分类。
// 返回 categoryID（可为 nil）、展示用一级分类名、二级分类名、是否走了 fallback。
func (s *ImportService) resolveImportCategory(ctx context.Context, bookID, txType, catName, subName string) (*string, string, string, bool) {
	cats, _ := s.catRepo.ListCategories(ctx, bookID, txType)
	var parents []model.Category
	parentByID := make(map[string]model.Category)
	for _, c := range cats {
		if c.ParentID == nil || *c.ParentID == "" {
			parents = append(parents, c)
			parentByID[c.ID] = c
		}
	}
	if catName != "" {
		for i := range parents {
			if parents[i].Name == catName {
				return strPtr(parents[i].ID), parents[i].Name, subName, false
			}
		}
		// 二级分类名直接命中。
		if subName == "" {
			for _, c := range cats {
				if c.ParentID != nil && *c.ParentID != "" && c.Name == catName {
					if p, ok := parentByID[*c.ParentID]; ok {
						return strPtr(p.ID), p.Name, c.Name, false
					}
				}
			}
		}
		for i := range parents {
			if strings.Contains(parents[i].Name, catName) || strings.Contains(catName, parents[i].Name) {
				return strPtr(parents[i].ID), parents[i].Name, subName, false
			}
		}
	}
	if subName != "" {
		for _, c := range cats {
			if c.ParentID != nil && *c.ParentID != "" && c.Name == subName {
				if p, ok := parentByID[*c.ParentID]; ok {
					return strPtr(p.ID), p.Name, c.Name, false
				}
			}
		}
	}
	// Fallback：预置分类里的「其他」/「其他收入」。
	fallbackName := "其他"
	if txType == "income" {
		fallbackName = "其他收入"
	}
	for i := range parents {
		if parents[i].Name == fallbackName {
			return strPtr(parents[i].ID), parents[i].Name, subName, true
		}
	}
	if len(parents) > 0 {
		return strPtr(parents[0].ID), parents[0].Name, subName, true
	}
	return nil, fallbackName, subName, true
}

func strPtr(s string) *string { return &s }

// resolveImportWallet 按账户名匹配钱包；匹配不上用默认钱包，再没有用账本第一个钱包。
func (s *ImportService) resolveImportWallet(ctx context.Context, bookID, walletName, defaultWalletID string) (string, string, error) {
	wallets, _ := s.walletRepo.ListWallets(ctx, bookID, false)
	if len(wallets) == 0 {
		return "", "", errors.New("book has no wallets")
	}
	if walletName != "" {
		for _, w := range wallets {
			if w.Name == walletName {
				return w.ID, w.Name, nil
			}
		}
		for _, w := range wallets {
			if strings.Contains(w.Name, walletName) || strings.Contains(walletName, w.Name) {
				return w.ID, w.Name, nil
			}
		}
	}
	if defaultWalletID != "" {
		for _, w := range wallets {
			if w.ID == defaultWalletID {
				return w.ID, w.Name, nil
			}
		}
	}
	return wallets[0].ID, wallets[0].Name, nil
}

// buildImportRow 把一行原始数据按映射解析成待入库的交易；返回预览行、可入库的
// model.Transaction 与匹配到的标签 ID。valid=false 时 tx 为 nil，错误写进预览行。
func (s *ImportService) buildImportRow(ctx context.Context, bookID, userID string, table *SuishoujiTable, mapping map[string]ColumnTarget, row []string, rowIndex int, defaultWalletID string) (*SuishoujiPreviewRow, *model.Transaction, []string) {
	preview := &SuishoujiPreviewRow{RowIndex: rowIndex}
	fail := func(reason string) (*SuishoujiPreviewRow, *model.Transaction, []string) {
		preview.Valid = false
		preview.Error = reason
		return preview, nil, nil
	}

	dateVal := rowValue(table, mapping, row, TargetDate)
	amountVal := rowValue(table, mapping, row, TargetAmount)
	if dateVal == "" {
		return fail("缺少日期")
	}
	if amountVal == "" {
		return fail("缺少金额")
	}
	date, txTime, err := parseImportDate(dateVal)
	if err != nil {
		return fail("日期格式无法识别：" + dateVal)
	}
	preview.Date = date
	preview.TxTime = txTime

	amountCents, err := parseImportAmount(amountVal)
	if err != nil {
		return fail("金额格式无法识别：" + amountVal)
	}
	if amountCents == 0 {
		return fail("金额为 0")
	}
	txType, err := parseImportType(rowValue(table, mapping, row, TargetType), amountCents)
	if err != nil {
		return fail(err.Error())
	}
	if amountCents < 0 {
		amountCents = -amountCents
	}
	preview.Type = txType
	preview.AmountCents = amountCents

	catName := rowValue(table, mapping, row, TargetCategory)
	subName := rowValue(table, mapping, row, TargetSubCategory)
	catID, displayCat, resolvedSub, usedFallback := s.resolveImportCategory(ctx, bookID, txType, catName, subName)
	preview.CategoryName = displayCat
	preview.SubCategory = resolvedSub

	walletName := rowValue(table, mapping, row, TargetWallet)
	walletID, resolvedWallet, err := s.resolveImportWallet(ctx, bookID, walletName, defaultWalletID)
	if err != nil {
		return fail(err.Error())
	}
	preview.WalletName = resolvedWallet

	title := rowValue(table, mapping, row, TargetTitle)
	if title == "" {
		title = displayCat
	}
	if title == "" {
		title = "随手记导入"
	}
	preview.Title = title

	note := rowValue(table, mapping, row, TargetNote)
	if usedFallback && catName != "" {
		if note != "" {
			note += "；"
		}
		note += "原分类：" + catName
	}
	preview.Note = note

	var tagIDs []string
	if tagsVal := rowValue(table, mapping, row, TargetTags); tagsVal != "" {
		tagIDs = s.matchTagIDs(ctx, bookID, tagsVal)
	}

	preview.Valid = true
	tx := &model.Transaction{
		BookID:        bookID,
		Type:          txType,
		Amount:        amountCents,
		Date:          date,
		TxTime:        txTime,
		WalletID:      walletID,
		CategoryID:    catID,
		Title:         title,
		SubCategory:   resolvedSub,
		Note:          note,
		IncludeBudget: true,
		CreatedBy:     userID,
	}
	return preview, tx, tagIDs
}

// matchTagIDs 按标签名匹配账本已有标签，返回匹配上的 ID（匹配不上的忽略）。
func (s *ImportService) matchTagIDs(ctx context.Context, bookID, tagsVal string) []string {
	tags, err := s.catRepo.ListTags(ctx, bookID)
	if err != nil || len(tags) == 0 {
		return nil
	}
	parts := strings.FieldsFunc(tagsVal, func(r rune) bool {
		return r == ',' || r == '，' || r == '、' || r == ';' || r == '；'
	})
	byName := make(map[string]string, len(tags))
	for _, t := range tags {
		byName[t.Name] = t.ID
	}
	var ids []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if id, ok := byName[p]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// BuildPreview 按映射解析前 limit 行，供前端预览（含分类/钱包匹配结果）。
func (s *ImportService) BuildPreview(ctx context.Context, bookID, userID string, table *SuishoujiTable, mapping map[string]ColumnTarget, defaultWalletID string, limit int) []SuishoujiPreviewRow {
	if limit <= 0 || limit > len(table.Rows) {
		limit = len(table.Rows)
	}
	out := make([]SuishoujiPreviewRow, 0, limit)
	for i := 0; i < limit; i++ {
		pv, _, _ := s.buildImportRow(ctx, bookID, userID, table, mapping, table.Rows[i], i+1, defaultWalletID)
		out = append(out, *pv)
	}
	return out
}

// CommitSuishouji 逐行入库：走 txRepo.CreateTransaction（正常记账路径，
// 钱包余额随之更新），非法行记入失败明细不中断整批；
// 去重规则：同一账本内同日期 + 同金额 + 同标题视为重复。
func (s *ImportService) CommitSuishouji(ctx context.Context, bookID, userID string, table *SuishoujiTable, mapping map[string]ColumnTarget, defaultWalletID string) (*SuishoujiCommitResult, error) {
	if len(table.Rows) > MaxSuishoujiRows {
		return nil, fmt.Errorf("too many rows: %d (max %d)", len(table.Rows), MaxSuishoujiRows)
	}
	result := &SuishoujiCommitResult{Failures: []SuishoujiRowFailure{}}
	seen := make(map[string]bool)
	for i, row := range table.Rows {
		rowIndex := i + 1
		pv, tx, tagIDs := s.buildImportRow(ctx, bookID, userID, table, mapping, row, rowIndex, defaultWalletID)
		if tx == nil {
			result.Failed++
			result.Failures = append(result.Failures, SuishoujiRowFailure{Row: rowIndex, Reason: pv.Error})
			continue
		}
		dedupKey := fmt.Sprintf("%s|%d|%s", tx.Date, tx.Amount, tx.Title)
		if seen[dedupKey] {
			result.Duplicates++
			continue
		}
		seen[dedupKey] = true
		var exists int
		err := s.db.QueryRowContext(ctx, `SELECT 1 FROM transactions WHERE book_id = ? AND date = ? AND amount = ? AND title = ? LIMIT 1`,
			bookID, tx.Date, tx.Amount, tx.Title).Scan(&exists)
		if err == nil && exists == 1 {
			result.Duplicates++
			continue
		}
		if err := s.txRepo.CreateTransaction(ctx, tx, tagIDs); err != nil {
			result.Failed++
			result.Failures = append(result.Failures, SuishoujiRowFailure{Row: rowIndex, Reason: err.Error()})
			continue
		}
		result.Inserted++
	}
	return result, nil
}
