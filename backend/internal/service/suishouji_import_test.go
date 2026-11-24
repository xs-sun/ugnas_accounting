package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"io"
)

func setupSuishoujiTest(t *testing.T) (*ImportService, *AiService, string, string) {
	t.Helper()
	ctx := context.Background()
	db, err := repository.InitDB(t.TempDir())
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	bookRepo := repository.NewBookRepo(db)
	walletRepo := repository.NewWalletRepo(db)
	catRepo := repository.NewCategoryRepo(db)
	txRepo := repository.NewTransactionRepo(db)
	budgetRepo := repository.NewBudgetRepo(db)
	userRepo := repository.NewUserRepo(db)
	settingsRepo := repository.NewSettingsRepo(db, t.TempDir())

	book := &model.Book{Name: "测试账本", OwnerID: "u1", Budget: 1000000}
	if err := bookRepo.CreateBook(ctx, book, "小禾"); err != nil {
		t.Fatalf("CreateBook: %v", err)
	}
	// CreateBook 已预置 微信支付/支付宝/招商银行卡/现金钱包 四个钱包；
	// 测试直接用预置钱包，不再建同名钱包（避免按名字查余额时撞名）。
	seeded, err := walletRepo.ListWallets(ctx, book.ID, false)
	if err != nil || len(seeded) == 0 {
		t.Fatalf("ListWallets: %v %d", err, len(seeded))
	}
	var defaultWalletID string
	for _, w := range seeded {
		if w.Name == "微信支付" {
			defaultWalletID = w.ID
		}
	}
	if defaultWalletID == "" {
		defaultWalletID = seeded[0].ID
	}

	importSvc := NewImportService(db, txRepo, catRepo, walletRepo)
	aiSvc := NewAiService(bookRepo, catRepo, walletRepo, txRepo, budgetRepo, userRepo, settingsRepo)
	return importSvc, aiSvc, book.ID, defaultWalletID
}

func TestAutoMapColumns(t *testing.T) {
	headers := []string{"日期", "收支类型", "金额", "分类", "子分类", "账户", "备注", "商家", "图片链接", "标签"}
	m := AutoMapColumns(headers)
	want := map[string]ColumnTarget{
		"日期": "date", "收支类型": "type", "金额": "amount", "分类": "category",
		"子分类": "subcategory", "账户": "wallet", "备注": "note",
		"商家": "title", "标签": "tags",
	}
	for h, w := range want {
		if m[h] != w {
			t.Errorf("header %q mapped to %q, want %q", h, m[h], w)
		}
	}
	// 未知列默认忽略，不报错。
	if m["图片链接"] != TargetIgnore {
		t.Errorf("unknown header mapped to %q, want ignore", m["图片链接"])
	}
}

func TestAutoMapColumnsEnglishAndVariants(t *testing.T) {
	headers := []string{"交易时间", "交易金额", "一级分类", "支付方式", "交易对方"}
	m := AutoMapColumns(headers)
	if m["交易时间"] != TargetDate || m["交易金额"] != TargetAmount ||
		m["一级分类"] != TargetCategory || m["支付方式"] != TargetWallet ||
		m["交易对方"] != TargetTitle {
		t.Errorf("variant headers mis-mapped: %+v", m)
	}
}

func TestParseImportDate(t *testing.T) {
	cases := []struct {
		in   string
		want string
		time string
	}{
		{"2026-11-24", "2026-11-24", "12:00"},
		{"2026/11/24", "2026-11-24", "12:00"},
		{"2026.11.24", "2026-11-24", "12:00"},
		{"2026-11-24 18:30:05", "2026-11-24", "18:30"},
		{"2026年11月24日", "2026-11-24", "12:00"},
		{"45965", "2025-11-04", "12:00"}, // Excel 序列号
	}
	for _, c := range cases {
		d, tm, err := parseImportDate(c.in)
		if err != nil {
			t.Errorf("parseImportDate(%q) error: %v", c.in, err)
			continue
		}
		if d != c.want || tm != c.time {
			t.Errorf("parseImportDate(%q) = %q %q, want %q %q", c.in, d, tm, c.want, c.time)
		}
	}
	if _, _, err := parseImportDate("昨天"); err == nil {
		t.Error("parseImportDate(\"昨天\") should fail")
	}
	if _, _, err := parseImportDate(""); err == nil {
		t.Error("parseImportDate(\"\") should fail")
	}
}

func TestParseImportAmount(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"18.5", 1850},
		{"¥32.00", 3200},
		{"￥1,234.56", 123456},
		{"(12.34)", -1234},
		{"-50", -5000},
		{"  88 ", 8800},
	}
	for _, c := range cases {
		got, err := parseImportAmount(c.in)
		if err != nil {
			t.Errorf("parseImportAmount(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseImportAmount(%q) = %d, want %d", c.in, got, c.want)
		}
	}
	if _, err := parseImportAmount("abc"); err == nil {
		t.Error("parseImportAmount(\"abc\") should fail")
	}
	if _, err := parseImportAmount(""); err == nil {
		t.Error("parseImportAmount(\"\") should fail")
	}
}

func TestParseImportType(t *testing.T) {
	if ty, _ := parseImportType("支出", 100); ty != "expense" {
		t.Errorf("支出 -> %q", ty)
	}
	if ty, _ := parseImportType("收入", 100); ty != "income" {
		t.Errorf("收入 -> %q", ty)
	}
	if _, err := parseImportType("转账", 100); err == nil {
		t.Error("转账 should fail")
	}
	if ty, _ := parseImportType("", -100); ty != "expense" {
		t.Errorf("empty type with negative amount -> %q", ty)
	}
	if _, err := parseImportType("???", 100); err == nil {
		t.Error("unknown type should fail")
	}
}

func TestParseCSVTableUTF8(t *testing.T) {
	svc, _, _, _ := setupSuishoujiTest(t)
	csvText := "\ufeff日期,收支类型,金额,分类,账户,备注\n2026-11-24,支出,18.50,餐饮,微信,瑞幸咖啡\n2026-11-23,收入,5000.00,工资,银行卡,发薪\n"
	table, err := svc.ParseSuishoujiFile([]byte(csvText), "suishouji.csv")
	if err != nil {
		t.Fatalf("ParseSuishoujiFile: %v", err)
	}
	if len(table.Headers) != 6 || table.Headers[0] != "日期" {
		t.Fatalf("headers = %v", table.Headers)
	}
	if len(table.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(table.Rows))
	}
	if table.Rows[0][5] != "瑞幸咖啡" {
		t.Errorf("row0 note = %q", table.Rows[0][5])
	}
}

func TestParseCSVTableGBK(t *testing.T) {
	svc, _, _, _ := setupSuishoujiTest(t)
	csvText := "日期,收支类型,金额,分类,账户,备注\n2026-11-24,支出,18.50,餐饮,微信,瑞幸咖啡\n"
	var buf bytes.Buffer
	w := transform.NewWriter(&buf, simplifiedchinese.GBK.NewEncoder())
	if _, err := io.WriteString(w, csvText); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	table, err := svc.ParseSuishoujiFile(buf.Bytes(), "suishouji.csv")
	if err != nil {
		t.Fatalf("ParseSuishoujiFile GBK: %v", err)
	}
	if len(table.Rows) != 1 || table.Rows[0][5] != "瑞幸咖啡" {
		t.Fatalf("GBK rows = %v", table.Rows)
	}
}

func TestParseXLSXTable(t *testing.T) {
	svc, _, _, _ := setupSuishoujiTest(t)
	f := excelize.NewFile()
	_ = f.SetSheetRow("Sheet1", "A1", &[]interface{}{"日期", "收支类型", "金额", "分类", "账户", "备注"})
	_ = f.SetSheetRow("Sheet1", "A2", &[]interface{}{"2026-11-24", "支出", 18.5, "餐饮", "微信", "瑞幸咖啡"})
	_ = f.SetSheetRow("Sheet1", "A3", &[]interface{}{"2026-11-23", "收入", 5000, "工资", "银行卡", "发薪"})
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	table, err := svc.ParseSuishoujiFile(buf.Bytes(), "suishouji.xlsx")
	if err != nil {
		t.Fatalf("ParseSuishoujiFile xlsx: %v", err)
	}
	if len(table.Headers) != 6 || len(table.Rows) != 2 {
		t.Fatalf("headers=%v rows=%d", table.Headers, len(table.Rows))
	}
	if table.Rows[0][2] != "18.5" {
		t.Errorf("amount cell = %q, want 18.5", table.Rows[0][2])
	}
}

func TestParseXLSTableFixture(t *testing.T) {
	svc, _, _, _ := setupSuishoujiTest(t)
	raw, err := os.ReadFile("testdata/table.xls")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	table, err := svc.ParseSuishoujiFile(raw, "table.xls")
	if err != nil {
		t.Fatalf("ParseSuishoujiFile xls: %v", err)
	}
	// 该 fixture 共 12 行（MaxRow=11）：第 1 行是表头，剩下 11 行数据。
	if len(table.Rows) != 11 {
		t.Fatalf("xls rows = %d, want 11", len(table.Rows))
	}
}

func TestParseSuishoujiFileUnsupported(t *testing.T) {
	svc, _, _, _ := setupSuishoujiTest(t)
	if _, err := svc.ParseSuishoujiFile([]byte("hello"), "note.txt"); err == nil {
		t.Error("txt file should fail (only empty .txt with no table is an error path)")
	}
	if _, err := svc.ParseSuishoujiFile([]byte{0x89, 0x50, 0x4E, 0x47}, "pic.png"); err == nil {
		t.Error("png file should fail")
	}
	if _, err := svc.ParseSuishoujiFile(nil, "empty.csv"); err == nil {
		t.Error("empty file should fail")
	}
}

func TestCommitSuishouji(t *testing.T) {
	svc, _, bookID, walletID := setupSuishoujiTest(t)
	ctx := context.Background()
	table := &SuishoujiTable{
		Headers: []string{"日期", "收支类型", "金额", "分类", "子分类", "账户", "备注", "商家"},
		Rows: [][]string{
			{"2026-11-24", "支出", "18.50", "餐饮", "午餐", "微信支付", "", "瑞幸咖啡"},
			{"2026-11-23", "收入", "5000.00", "工资", "", "招商银行卡", "发薪", ""},
			{"2026-11-22", "支出", "99.00", "火星开支", "", "支付宝", "", "朋友生日礼物"},
			{"2026-11-21", "支出", "abc", "餐饮", "", "微信支付", "", "坏数据行"},
			{"2026-11-20", "支出", "25.00", "餐饮", "", "不存在的钱包", "", "早餐"},
			{"2026-11-19", "支出", "66.00", "人情往来", "", "现金钱包", "", "随份子"},
		},
	}
	mapping := AutoMapColumns(table.Headers)
	res, err := svc.CommitSuishouji(ctx, bookID, "u1", table, mapping, walletID)
	if err != nil {
		t.Fatalf("CommitSuishouji: %v", err)
	}
	if res.Inserted != 5 {
		t.Errorf("inserted = %d, want 5", res.Inserted)
	}
	if res.Failed != 1 {
		t.Errorf("failed = %d, want 1", res.Failed)
	}
	if len(res.Failures) != 1 || res.Failures[0].Row != 4 {
		t.Errorf("failures = %+v, want row 4", res.Failures)
	}
	// 分类 fallback：火星开支匹配不上，应进预置「其他」，原分类名进备注。
	items, total, err := svc.txRepo.ListTransactions(ctx, bookID, "", "", "", "", "", "", 10, 0)
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	var gift *model.Transaction
	for i := range items {
		if items[i].Title == "朋友生日礼物" {
			gift = &items[i]
		}
	}
	if gift == nil {
		t.Fatal("gift transaction not found")
	}
	if gift.CategoryName != "其他" {
		t.Errorf("fallback category = %q, want 其他", gift.CategoryName)
	}
	if !strings.Contains(gift.Note, "原分类：火星开支") {
		t.Errorf("note = %q, want 原分类 suffix", gift.Note)
	}
	// 包含匹配：人情往来应匹配到预置分类「人情」。
	var giftMoney *model.Transaction
	for i := range items {
		if items[i].Title == "随份子" {
			giftMoney = &items[i]
		}
	}
	if giftMoney == nil || giftMoney.CategoryName != "人情" {
		t.Errorf("人情往来 category = %v, want 人情", giftMoney)
	}
	// 钱包 fallback：不存在的钱包应落入默认钱包（微信支付）。
	var breakfast *model.Transaction
	for i := range items {
		if items[i].Title == "早餐" {
			breakfast = &items[i]
		}
	}
	if breakfast == nil || breakfast.WalletID != walletID {
		t.Errorf("breakfast wallet = %v, want %s", breakfast, walletID)
	}
	// 钱包余额：微信支付：-18.5 -25；招商银行卡：+5000；支付宝：-99；现金钱包：-66。
	wallets, _ := svc.walletRepo.ListWallets(ctx, bookID, false)
	balances := map[string]int64{}
	for _, w := range wallets {
		balances[w.Name] = w.Balance
	}
	if balances["微信支付"] != -1850-2500 {
		t.Errorf("微信支付 balance = %d, want %d", balances["微信支付"], -1850-2500)
	}
	if balances["招商银行卡"] != 500000 {
		t.Errorf("招商银行卡 balance = %d, want 500000", balances["招商银行卡"])
	}
	if balances["支付宝"] != -9900 {
		t.Errorf("支付宝 balance = %d, want -9900", balances["支付宝"])
	}
	if balances["现金钱包"] != -6600 {
		t.Errorf("现金钱包 balance = %d, want -6600", balances["现金钱包"])
	}

	// 再次提交同一文件：5 条有效行判重，坏行再次失败。
	res2, err := svc.CommitSuishouji(ctx, bookID, "u1", table, mapping, walletID)
	if err != nil {
		t.Fatalf("second CommitSuishouji: %v", err)
	}
	if res2.Duplicates != 5 {
		t.Errorf("second run duplicates = %d, want 5", res2.Duplicates)
	}
	if res2.Inserted != 0 {
		t.Errorf("second run inserted = %d, want 0", res2.Inserted)
	}
}

func TestCommitSuishoujiMissingRequiredMapping(t *testing.T) {
	svc, _, bookID, walletID := setupSuishoujiTest(t)
	ctx := context.Background()
	table := &SuishoujiTable{
		Headers: []string{"日期", "分类"},
		Rows:    [][]string{{"2026-11-24", "餐饮"}},
	}
	mapping := map[string]ColumnTarget{"日期": TargetDate, "分类": TargetCategory}
	res, err := svc.CommitSuishouji(ctx, bookID, "u1", table, mapping, walletID)
	if err != nil {
		t.Fatalf("CommitSuishouji: %v", err)
	}
	if res.Inserted != 0 || res.Failed != 1 {
		t.Errorf("inserted=%d failed=%d, want 0/1 (missing amount)", res.Inserted, res.Failed)
	}
}

func TestSuggestColumnMappingWithStub(t *testing.T) {
	_, aiSvc, _, _ := setupSuishoujiTest(t)
	ctx := context.Background()

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": `{"mapping": {"交易日期": "date", "收支": "type", "发生金额": "amount", "商户名称": "title", "神秘列": "note"}}`}},
			},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5},
		})
	}))
	defer stub.Close()

	// 把 AI 配置指向 stub 服务。
	settingsRepo := aiSvc.settingsRepo
	cfg := &model.AiConfig{Enabled: true, Provider: "stub", BaseURL: stub.URL, APIKey: "sk-test", Model: "stub-model", DailyLimit: 30, Temperature: 0}
	if err := settingsRepo.SaveAiConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveAiConfig: %v", err)
	}

	headers := []string{"交易日期", "收支", "发生金额", "商户名称", "神秘列"}
	samples := [][]string{{"2026-11-24", "支出", "18.50", "瑞幸咖啡", "x"}}
	mapping, ok, reason := aiSvc.SuggestColumnMapping(ctx, "u1", headers, samples)
	if !ok {
		t.Fatalf("SuggestColumnMapping not ok, reason=%s", reason)
	}
	if mapping["交易日期"] != "date" || mapping["收支"] != "type" ||
		mapping["发生金额"] != "amount" || mapping["商户名称"] != "title" ||
		mapping["神秘列"] != "note" {
		t.Errorf("mapping = %+v", mapping)
	}
}

func TestSuggestColumnMappingUnconfigured(t *testing.T) {
	_, aiSvc, _, _ := setupSuishoujiTest(t)
	ctx := context.Background()
	// 默认配置无 API Key → 应返回不可用，而不是报错。
	_, ok, reason := aiSvc.SuggestColumnMapping(ctx, "u1", []string{"日期"}, [][]string{{"2026-11-24"}})
	if ok {
		t.Error("unconfigured AI should return available=false")
	}
	if reason != "ai_unconfigured" && reason != "ai_disabled" {
		t.Errorf("reason = %q", reason)
	}
}

func TestSuggestColumnMappingBadStubResponse(t *testing.T) {
	_, aiSvc, _, _ := setupSuishoujiTest(t)
	ctx := context.Background()
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices": [{"message": {"content": "not json at all"}}]}`))
	}))
	defer stub.Close()
	cfg := &model.AiConfig{Enabled: true, Provider: "stub", BaseURL: stub.URL, APIKey: "sk-test", Model: "stub-model", DailyLimit: 30}
	if err := aiSvc.settingsRepo.SaveAiConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveAiConfig: %v", err)
	}
	_, ok, _ := aiSvc.SuggestColumnMapping(ctx, "u1", []string{"日期"}, [][]string{{"2026-11-24"}})
	if ok {
		t.Error("bad stub response should return available=false")
	}
}
