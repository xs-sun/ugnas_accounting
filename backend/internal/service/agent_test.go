package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
)

// ---------- stub 模型服务 ----------

type stubResponse struct {
	status int
	body   string
}

type scriptedStub struct {
	mu        sync.Mutex
	responses []stubResponse
	requests  []map[string]interface{}
}

func (s *scriptedStub) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req map[string]interface{}
	_ = json.Unmarshal(body, &req)
	s.mu.Lock()
	s.requests = append(s.requests, req)
	var resp stubResponse
	if len(s.responses) > 0 {
		resp = s.responses[0]
		s.responses = s.responses[1:]
	} else {
		resp = textResp("（stub 响应已耗尽）")
	}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.status)
	_, _ = w.Write([]byte(resp.body))
}

func (s *scriptedStub) hitCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

// toolResults 返回第 i 次请求里收到的 tool 结果（按顺序）。
func (s *scriptedStub) toolResults(i int) []map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i < 0 || i >= len(s.requests) {
		return nil
	}
	msgs, _ := s.requests[i]["messages"].([]interface{})
	var out []map[string]interface{}
	for _, m := range msgs {
		mm, _ := m.(map[string]interface{})
		if mm == nil || mm["role"] != "tool" {
			continue
		}
		content, _ := mm["content"].(string)
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(content), &parsed); err == nil {
			out = append(out, parsed)
		}
	}
	return out
}

func toolCallResp(toolName string, args map[string]interface{}) stubResponse {
	argBytes, _ := json.Marshal(args)
	bodyMap := map[string]interface{}{
		"choices": []map[string]interface{}{
			{"message": map[string]interface{}{
				"role":    "assistant",
				"content": "",
				"tool_calls": []map[string]interface{}{
					{"id": "call_" + toolName, "type": "function", "function": map[string]interface{}{
						"name":      toolName,
						"arguments": string(argBytes),
					}},
				},
			}},
		},
		"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5},
	}
	b, _ := json.Marshal(bodyMap)
	return stubResponse{status: 200, body: string(b)}
}

func textResp(text string) stubResponse {
	bodyMap := map[string]interface{}{
		"choices": []map[string]interface{}{
			{"message": map[string]interface{}{"role": "assistant", "content": text}},
		},
		"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5},
	}
	b, _ := json.Marshal(bodyMap)
	return stubResponse{status: 200, body: string(b)}
}

func newScriptedStubServer(t *testing.T, responses ...stubResponse) (*scriptedStub, *httptest.Server) {
	t.Helper()
	stub := &scriptedStub{responses: responses}
	srv := httptest.NewServer(http.HandlerFunc(stub.handler))
	t.Cleanup(srv.Close)
	return stub, srv
}

func saveStubAIConfig(t *testing.T, aiSvc *AiService, stubURL string) {
	t.Helper()
	cfg := &model.AiConfig{Enabled: true, Provider: "stub", BaseURL: stubURL, APIKey: "sk-test", Model: "stub-model", DailyLimit: 30, Temperature: 0}
	if err := aiSvc.settingsRepo.SaveAiConfig(context.Background(), cfg); err != nil {
		t.Fatalf("SaveAiConfig: %v", err)
	}
}

// numToFloat 统一处理直接调工具返回的 int64 与经 JSON 的 float64。
func numToFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	}
	return 0
}

// ---------- 测试数据 ----------

func setupAgentTest(t *testing.T) (*AiService, string, string, string, string) {
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

	for _, u := range []struct{ id, username, name string }{
		{"u1", "xiaohe", "小禾"},
		{"u2", "song", "松"},
		{"u3", "stranger", "陌生人"},
	} {
		if err := userRepo.CreateUser(ctx, &model.User{ID: u.id, Username: u.username, Name: u.name, IsActive: true}); err != nil {
			t.Fatalf("CreateUser %s: %v", u.id, err)
		}
	}

	bookA := &model.Book{Name: "家庭账本", OwnerID: "u1", Budget: 1000000}
	if err := bookRepo.CreateBook(ctx, bookA, "小禾"); err != nil {
		t.Fatalf("CreateBook A: %v", err)
	}
	if err := bookRepo.AddMember(ctx, &model.BookMember{BookID: bookA.ID, UserID: "u2", UserName: "松", Role: "editor"}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	bookB := &model.Book{Name: "陌生人账本", OwnerID: "u3", Budget: 500000}
	if err := bookRepo.CreateBook(ctx, bookB, "陌生人"); err != nil {
		t.Fatalf("CreateBook B: %v", err)
	}

	wallets, err := walletRepo.ListWallets(ctx, bookA.ID, false)
	if err != nil || len(wallets) == 0 {
		t.Fatalf("ListWallets A: %v %d", err, len(wallets))
	}
	walletA := wallets[0].ID
	for _, w := range wallets {
		if w.Name == "微信支付" {
			walletA = w.ID
		}
	}
	cats, _ := catRepo.ListCategories(ctx, bookA.ID, "")
	var catFoodID string
	for _, c := range cats {
		if c.Name == "餐饮" && (c.ParentID == nil || *c.ParentID == "") {
			catFoodID = c.ID
		}
	}
	if catFoodID == "" {
		t.Fatal("preset category 餐饮 not found")
	}

	now := time.Now()
	dayOfMonth := func(day int) string {
		return fmt.Sprintf("%04d-%02d-%02d", now.Year(), int(now.Month()), day)
	}
	lastMonth := now.AddDate(0, -1, 0)
	lastMonthDay := fmt.Sprintf("%04d-%02d-10", lastMonth.Year(), int(lastMonth.Month()))
	seed := []struct {
		user   string
		date   string
		amount int64
	}{
		{"u1", dayOfMonth(5), 3200},
		{"u1", dayOfMonth(10), 5800},
		{"u1", dayOfMonth(15), 1800},
		{"u2", dayOfMonth(12), 2500},
		{"u1", lastMonthDay, 20000},
	}
	for i, s := range seed {
		catID := catFoodID
		if err := txRepo.CreateTransaction(ctx, &model.Transaction{
			BookID: bookA.ID, Type: "expense", Amount: s.amount, Date: s.date,
			WalletID: walletA, CategoryID: &catID, Title: fmt.Sprintf("测试%d", i),
			CreatedBy: s.user, IncludeBudget: true,
		}, nil); err != nil {
			t.Fatalf("seed tx %d: %v", i, err)
		}
	}
	// 账本 B 放一笔特征金额，用于检测泄露：999999 分 = ¥9999.99。
	walletsB, _ := walletRepo.ListWallets(ctx, bookB.ID, false)
	catsB, _ := catRepo.ListCategories(ctx, bookB.ID, "")
	var catBID string
	for _, c := range catsB {
		if c.Name == "餐饮" && (c.ParentID == nil || *c.ParentID == "") {
			catBID = c.ID
		}
	}
	if err := txRepo.CreateTransaction(ctx, &model.Transaction{
		BookID: bookB.ID, Type: "expense", Amount: 999999, Date: dayOfMonth(8),
		WalletID: walletsB[0].ID, CategoryID: &catBID, Title: "陌生人大额",
		CreatedBy: "u3", IncludeBudget: true,
	}, nil); err != nil {
		t.Fatalf("seed bookB tx: %v", err)
	}

	aiSvc := NewAiService(bookRepo, catRepo, walletRepo, txRepo, budgetRepo, userRepo, settingsRepo)
	return aiSvc, bookA.ID, bookB.ID, walletA, catFoodID
}

// ---------- A. 单步查数 ----------

func TestAgentSingleStepQuery(t *testing.T) {
	aiSvc, bookA, _, _, _ := setupAgentTest(t)
	ctx := context.Background()
	stub, srv := newScriptedStubServer(t,
		toolCallResp("query_stats", map[string]interface{}{"period": "month", "type": "expense", "categoryName": "餐饮"}),
		textResp("本月餐饮共花了 ¥133.00。"),
	)
	saveStubAIConfig(t, aiSvc, srv.URL)

	reply, draft, usage, err := aiSvc.Chat(ctx, "u1", bookA, "这个月餐饮花了多少", false)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if draft != nil {
		t.Errorf("query should not produce a draft, got %+v", draft)
	}
	if !strings.Contains(reply, "133.00") {
		t.Errorf("reply = %q, want it to contain 133.00", reply)
	}
	if usage.TodayCount != 1 {
		t.Errorf("TodayCount = %d, want 1", usage.TodayCount)
	}
	// A2：断言第二次请求里喂给模型的工具结果就是真实数字。
	results := stub.toolResults(1)
	if len(results) == 0 {
		t.Fatalf("no tool results fed back to model")
	}
	if got, _ := results[0]["total_expense_cents"].(float64); got != 13300 {
		t.Errorf("tool result total_expense_cents = %v, want 13300", results[0]["total_expense_cents"])
	}
}

// ---------- B. 多步查数 ----------

func TestAgentMultiStepQuery(t *testing.T) {
	aiSvc, bookA, _, _, _ := setupAgentTest(t)
	ctx := context.Background()
	now := time.Now()
	lastMonth := now.AddDate(0, -1, 0)
	stub, srv := newScriptedStubServer(t,
		toolCallResp("query_stats", map[string]interface{}{"period": "month", "year": lastMonth.Year(), "month": int(lastMonth.Month()), "type": "expense", "categoryName": "餐饮"}),
		toolCallResp("query_stats", map[string]interface{}{"period": "month", "year": now.Year(), "month": int(now.Month()), "type": "expense", "categoryName": "餐饮"}),
		textResp("上个月餐饮花了 ¥200.00，比这个月多 ¥67.00。"),
	)
	saveStubAIConfig(t, aiSvc, srv.URL)

	reply, _, _, err := aiSvc.Chat(ctx, "u1", bookA, "上个月餐饮比这个月多多少", false)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !strings.Contains(reply, "200.00") || !strings.Contains(reply, "67.00") {
		t.Errorf("reply = %q, want 200.00 and 67.00", reply)
	}
	r1 := stub.toolResults(1)
	if len(r1) == 0 {
		t.Fatalf("no tool result in request 2")
	}
	if got, _ := r1[0]["total_expense_cents"].(float64); got != 20000 {
		t.Errorf("first tool result total = %v, want 20000", r1[0]["total_expense_cents"])
	}
	r2 := stub.toolResults(2)
	if len(r2) < 2 {
		t.Fatalf("want 2 tool results in request 3, got %d", len(r2))
	}
	if got, _ := r2[1]["total_expense_cents"].(float64); got != 13300 {
		t.Errorf("second tool result total = %v, want 13300", r2[1]["total_expense_cents"])
	}
}

// ---------- C. 按成员查 ----------

func TestAgentQueryMembers(t *testing.T) {
	aiSvc, bookA, _, _, _ := setupAgentTest(t)
	ctx := context.Background()

	// C2：工具直测。
	res, err := aiSvc.execTool(ctx, "u1", bookA, "query_members", map[string]interface{}{"period": "month"})
	if err != nil {
		t.Fatalf("execTool query_members: %v", err)
	}
	m, _ := res.(map[string]interface{})
	members, _ := m["members"].([]map[string]interface{})
	if len(members) != 2 {
		t.Fatalf("members = %+v, want 2", m["members"])
	}
	got := map[string]float64{}
	for _, mm := range members {
		got[mm["name"].(string)] = numToFloat(mm["expense_cents"])
	}
	if got["小禾"] != 10800 {
		t.Errorf("小禾 expense = %v, want 10800", got["小禾"])
	}
	if got["松"] != 2500 {
		t.Errorf("松 expense = %v, want 2500", got["松"])
	}

	// C1：走 Chat。
	stub, srv := newScriptedStubServer(t,
		toolCallResp("query_members", map[string]interface{}{"period": "month"}),
		textResp("松本月花了 ¥25.00。"),
	)
	saveStubAIConfig(t, aiSvc, srv.URL)
	reply, _, _, err := aiSvc.Chat(ctx, "u1", bookA, "松这个月花了多少", false)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !strings.Contains(reply, "25.00") {
		t.Errorf("reply = %q, want 25.00", reply)
	}
	_ = stub
}

// ---------- D. 记账仍出草稿、不直接入账 ----------

func TestAgentCreateDraft(t *testing.T) {
	aiSvc, bookA, _, _, _ := setupAgentTest(t)
	ctx := context.Background()
	before, total, err := aiSvc.txRepo.ListTransactions(ctx, bookA, "", "", "", "", "", "", 100, 0)
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	_ = before
	stub, srv := newScriptedStubServer(t,
		toolCallResp("create_draft", map[string]interface{}{
			"type": "expense", "amountYuan": 18.5, "title": "瑞幸咖啡",
			"categoryName": "餐饮", "subCategory": "咖啡饮品", "walletName": "微信支付",
		}),
		textResp("已生成草稿，请核对后确认入账。"),
	)
	saveStubAIConfig(t, aiSvc, srv.URL)

	_, draft, _, err := aiSvc.Chat(ctx, "u1", bookA, "瑞幸咖啡 18.5 元", false)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if draft == nil {
		t.Fatal("want a draft, got nil")
	}
	if draft.Amount != 1850 || draft.Type != "expense" || draft.CategoryName != "餐饮" {
		t.Errorf("draft = %+v, want amount 1850 expense 餐饮", draft)
	}
	_, after, err := aiSvc.txRepo.ListTransactions(ctx, bookA, "", "", "", "", "", "", 100, 0)
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if after != total {
		t.Errorf("transactions changed %d -> %d, draft must not post directly", total, after)
	}
	_ = stub
}

// ---------- E. 跨账本拒绝 ----------

func TestAgentCrossBookForbidden(t *testing.T) {
	aiSvc, bookA, bookB, _, _ := setupAgentTest(t)
	ctx := context.Background()

	// E2/E3：工具直测，五个查询工具传账本 B 都应拒绝。
	for _, name := range []string{"query_stats", "query_transactions", "query_budgets", "query_wallets", "query_members"} {
		_, err := aiSvc.execTool(ctx, "u1", bookA, name, map[string]interface{}{"bookId": bookB})
		if !errors.Is(err, errAgentForbidden) {
			t.Errorf("tool %s with other book: err = %v, want errAgentForbidden", name, err)
		}
	}

	// E1：走 Chat，模型调了账本 B 的工具 → 工具返回无权访问 → 模型拒绝回答。
	stub, srv := newScriptedStubServer(t,
		toolCallResp("query_stats", map[string]interface{}{"bookId": bookB, "period": "month"}),
		textResp("抱歉，我没有权限查看该账本。"),
	)
	saveStubAIConfig(t, aiSvc, srv.URL)
	reply, _, _, err := aiSvc.Chat(ctx, "u1", bookA, "查一下陌生人账本这个月花了多少", false)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !strings.Contains(reply, "没有权限") {
		t.Errorf("reply = %q, want a refusal", reply)
	}
	if strings.Contains(reply, "9999.99") {
		t.Errorf("reply leaks other book data: %q", reply)
	}
	results := stub.toolResults(1)
	if len(results) == 0 {
		t.Fatalf("no tool result fed back")
	}
	if !strings.Contains(fmt.Sprint(results[0]["error"]), "无权访问") {
		t.Errorf("tool result = %v, want 无权访问 error", results[0])
	}

	// E4：Chat 请求直接传账本 B 的 book_id，应回落到自己的账本。
	stub2, srv2 := newScriptedStubServer(t,
		toolCallResp("query_stats", map[string]interface{}{"period": "month"}),
		textResp("本月共花了 ¥133.00。"),
	)
	saveStubAIConfig(t, aiSvc, srv2.URL)
	reply2, _, _, err := aiSvc.Chat(ctx, "u1", bookB, "这个月花了多少", false)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !strings.Contains(reply2, "133.00") {
		t.Errorf("reply2 = %q, want own book total 133.00", reply2)
	}
	if strings.Contains(reply2, "9999.99") {
		t.Errorf("reply2 leaks other book data: %q", reply2)
	}
	_ = stub2
}

// ---------- F. 退回老行为 ----------

func TestAgentFallbackWhenToolsUnsupported(t *testing.T) {
	aiSvc, bookA, _, _, _ := setupAgentTest(t)
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"This model does not support function calling","type":"invalid_request_error"}}`))
	}))
	t.Cleanup(srv.Close)
	saveStubAIConfig(t, aiSvc, srv.URL)

	reply, _, _, err := aiSvc.Chat(ctx, "u1", bookA, "本月花了多少", false)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !strings.Contains(reply, "本月累计支出") || !strings.Contains(reply, "133.00") {
		t.Errorf("reply = %q, want local fallback template with 133.00", reply)
	}
}

func TestAgentUnconfiguredFallsBackToLocal(t *testing.T) {
	aiSvc, bookA, _, _, _ := setupAgentTest(t)
	ctx := context.Background()
	cfg := &model.AiConfig{Enabled: true, Provider: "", BaseURL: "", APIKey: "", Model: "", DailyLimit: 30}
	if err := aiSvc.settingsRepo.SaveAiConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveAiConfig: %v", err)
	}
	reply, _, _, err := aiSvc.Chat(ctx, "u1", bookA, "本月花了多少", false)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !strings.Contains(reply, "本月累计支出") {
		t.Errorf("reply = %q, want local fallback", reply)
	}
}

func TestAgentDisabledReturnsError(t *testing.T) {
	aiSvc, bookA, _, _, _ := setupAgentTest(t)
	ctx := context.Background()
	cfg := &model.AiConfig{Enabled: false, DailyLimit: 30}
	if err := aiSvc.settingsRepo.SaveAiConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveAiConfig: %v", err)
	}
	_, _, _, err := aiSvc.Chat(ctx, "u1", bookA, "本月花了多少", false)
	if !errors.Is(err, ErrAiDisabled) {
		t.Errorf("err = %v, want ErrAiDisabled", err)
	}
}

// ---------- G. 循环上限 ----------

func TestAgentLoopCap(t *testing.T) {
	aiSvc, bookA, _, _, _ := setupAgentTest(t)
	ctx := context.Background()
	var mu sync.Mutex
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(toolCallResp("query_stats", map[string]interface{}{"period": "month"}).body))
	}))
	t.Cleanup(srv.Close)
	saveStubAIConfig(t, aiSvc, srv.URL)

	reply, _, _, err := aiSvc.Chat(ctx, "u1", bookA, "这个月花了多少", false)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if hits > 6 {
		t.Errorf("model called %d times, want at most 6", hits)
	}
	if !strings.Contains(reply, "已查到") {
		t.Errorf("reply = %q, want cap summary", reply)
	}
	if !strings.Contains(reply, "13300") {
		t.Errorf("reply = %q, want real numbers from tool results", reply)
	}
}

// ---------- H. 用量只记一次 ----------

func TestAgentUsageCountedOnce(t *testing.T) {
	aiSvc, bookA, _, _, _ := setupAgentTest(t)
	ctx := context.Background()
	_, srv := newScriptedStubServer(t,
		toolCallResp("query_stats", map[string]interface{}{"period": "month"}),
		textResp("本月共花了 ¥133.00。"),
	)
	saveStubAIConfig(t, aiSvc, srv.URL)

	_, _, usage, err := aiSvc.Chat(ctx, "u1", bookA, "这个月花了多少", false)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if usage.TodayCount != 1 {
		t.Errorf("TodayCount = %d, want 1", usage.TodayCount)
	}
	totalCalls, _, totalIn, totalOut, _, _, err := aiSvc.settingsRepo.GetAdminAIUsage(ctx, "")
	if err != nil {
		t.Fatalf("GetAdminAIUsage: %v", err)
	}
	if totalCalls != 1 {
		t.Errorf("totalCalls = %d, want 1", totalCalls)
	}
	// 两轮 stub 响应各 10 prompt + 5 completion。
	if totalIn != 20 || totalOut != 10 {
		t.Errorf("tokens in=%d out=%d, want 20/10", totalIn, totalOut)
	}
}

// ---------- 工具参数解析 ----------

func TestResolveDateRange(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name      string
		args      map[string]interface{}
		wantStart string
		wantEnd   string
		wantErr   bool
	}{
		{"default month", map[string]interface{}{}, fmt.Sprintf("%04d-%02d-01", now.Year(), int(now.Month())), "", false},
		{"year", map[string]interface{}{"period": "year", "year": 2026}, "2026-01-01", "2026-12-31", false},
		{"quarter Q2", map[string]interface{}{"period": "quarter", "year": 2026, "quarter": 2}, "2026-04-01", "2026-06-30", false},
		{"custom", map[string]interface{}{"period": "custom", "startDate": "2026-10-01", "endDate": "2026-10-31"}, "2026-10-01", "2026-10-31", false},
		{"custom missing end", map[string]interface{}{"period": "custom", "startDate": "2026-10-01"}, "", "", true},
		{"start after end", map[string]interface{}{"startDate": "2026-10-31", "endDate": "2026-10-01"}, "", "", true},
	}
	for _, c := range cases {
		sd, ed, err := resolveDateRange(c.args)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: want error, got %s..%s", c.name, sd, ed)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", c.name, err)
			continue
		}
		if sd != c.wantStart {
			t.Errorf("%s: start = %s, want %s", c.name, sd, c.wantStart)
		}
		if c.wantEnd != "" && ed != c.wantEnd {
			t.Errorf("%s: end = %s, want %s", c.name, ed, c.wantEnd)
		}
	}
}
