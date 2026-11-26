package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
)

// ---------- helpers ----------

func listActivities(t *testing.T, env *testEnv, token, bookID string, limit int, before string) ([]map[string]interface{}, bool, string, int) {
	t.Helper()
	path := fmt.Sprintf("/api/v1/books/%s/activities?limit=%d", bookID, limit)
	if before != "" {
		path += "&before=" + before
	}
	headers := map[string]string{}
	if token != "" {
		headers = authHeaders(token)
	}
	rr, resp := doJSONReq(t, env.handler, http.MethodGet, path, nil, headers)
	if rr.Code != http.StatusOK {
		return nil, false, "", rr.Code
	}
	data := resp["data"].(map[string]interface{})
	rawItems, _ := data["items"].([]interface{})
	items := make([]map[string]interface{}, 0, len(rawItems))
	for _, it := range rawItems {
		if m, ok := it.(map[string]interface{}); ok {
			items = append(items, m)
		}
	}
	hasMore, _ := data["has_more"].(bool)
	nextBefore, _ := data["next_before"].(string)
	return items, hasMore, nextBefore, rr.Code
}

func countAction(items []map[string]interface{}, action string) int {
	n := 0
	for _, it := range items {
		if it["action"] == action {
			n++
		}
	}
	return n
}

func findAction(items []map[string]interface{}, action string) (map[string]interface{}, bool) {
	for _, it := range items {
		if it["action"] == action {
			return it, true
		}
	}
	return nil, false
}

func createTxFull(t *testing.T, env *testEnv, token, bookID, walletID, txType, title string, amount int64, categoryID string, includeBudget *bool) string {
	t.Helper()
	body := map[string]interface{}{
		"book_id": bookID, "wallet_id": walletID, "type": txType,
		"amount": amount, "title": title,
	}
	if categoryID != "" {
		body["category_id"] = categoryID
	}
	if includeBudget != nil {
		body["include_budget"] = *includeBudget
	}
	rr, resp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/transactions", body, authHeaders(token))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create transaction failed: %d %s", rr.Code, rr.Body.String())
	}
	id, _ := resp["data"].(map[string]interface{})["id"].(string)
	if id == "" {
		t.Fatalf("create transaction returned no id: %v", resp)
	}
	return id
}

func boolPtr(b bool) *bool { return &b }

func setTotalBudget(t *testing.T, env *testEnv, token, bookID string, budget int64) {
	t.Helper()
	rr, _ := doJSONReq(t, env.handler, http.MethodPut, "/api/v1/budgets/total", map[string]interface{}{
		"book_id": bookID, "budget": budget,
	}, authHeaders(token))
	if rr.Code != http.StatusOK {
		t.Fatalf("set total budget failed: %d %s", rr.Code, rr.Body.String())
	}
}

func setCategoryBudget(t *testing.T, env *testEnv, token, bookID, categoryID string, limit int64) {
	t.Helper()
	rr, _ := doJSONReq(t, env.handler, http.MethodPut, "/api/v1/budgets/category", map[string]interface{}{
		"book_id": bookID, "category_id": categoryID, "limit_amount": limit,
	}, authHeaders(token))
	if rr.Code != http.StatusOK {
		t.Fatalf("set category budget failed: %d %s", rr.Code, rr.Body.String())
	}
}

func topCategoryID(t *testing.T, env *testEnv, token, bookID, name string) string {
	t.Helper()
	rr, resp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/categories?book_id="+bookID+"&type=expense", nil, authHeaders(token))
	if rr.Code != http.StatusOK {
		t.Fatalf("list categories failed: %d %s", rr.Code, rr.Body.String())
	}
	data, _ := resp["data"].([]interface{})
	for _, it := range data {
		m, _ := it.(map[string]interface{})
		if m == nil {
			continue
		}
		if m["name"] == name {
			if pid, _ := m["parent_id"].(string); pid == "" {
				id, _ := m["id"].(string)
				return id
			}
			// parent_id 可能为 null
			if m["parent_id"] == nil {
				id, _ := m["id"].(string)
				return id
			}
		}
	}
	// 没有就新建一个
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/categories", map[string]interface{}{
		"book_id": bookID, "type": "expense", "name": name,
	}, authHeaders(token))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create category %s failed: %d %s", name, rr.Code, rr.Body.String())
	}
	id, _ := resp["data"].(map[string]interface{})["id"].(string)
	return id
}

func setupActivityEnv(t *testing.T) (*testEnv, string, string, string, string, string, string) {
	t.Helper()
	t.Setenv("STRICT_AUTH", "1")
	env := setupTestEnv(t)
	_, aliceToken := registerTestUser(t, env, "小禾", "alice-ab@cat7.app")
	bobID, bobToken := registerTestUser(t, env, "小北", "bob-ab@cat7.app")
	_, carolToken := registerTestUser(t, env, "小南", "carol-ab@cat7.app")
	bookID := createTestBook(t, env, aliceToken, "家庭账本")
	rr, _ := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/"+bookID+"/members", map[string]interface{}{
		"user_id": bobID, "role": "editor",
	}, authHeaders(aliceToken))
	if rr.Code != http.StatusCreated {
		t.Fatalf("add member failed: %d %s", rr.Code, rr.Body.String())
	}
	walletID := createTestWallet(t, env, aliceToken, bookID, "微信钱包")
	return env, aliceToken, bobToken, carolToken, bookID, walletID, bobID
}

// ---------- D1-D3/D11/D12/D15/D16：动态基本流 ----------

func TestActivityFeedBasic(t *testing.T) {
	env, aliceToken, bobToken, carolToken, bookID, walletID, _ := setupActivityEnv(t)

	// D1：A 记一笔，B 的动态里出现
	txID := createTxFull(t, env, aliceToken, bookID, walletID, "expense", "午餐", 3200, "", nil)
	items, _, _, code := listActivities(t, env, bobToken, bookID, 20, "")
	if code != http.StatusOK {
		t.Fatalf("list activities failed: %d", code)
	}
	created, ok := findAction(items, model.ActivityTxCreated)
	if !ok {
		t.Fatalf("expected transaction_created activity, got: %v", items)
	}
	if created["actor_name"] != "小禾" {
		t.Fatalf("actor_name = %v, want 小禾", created["actor_name"])
	}
	summary, _ := created["summary"].(string)
	if summary == "" {
		t.Fatalf("summary empty")
	}
	if amt, _ := created["amount_cents"].(float64); amt != 3200 {
		t.Fatalf("amount_cents = %v, want 3200", created["amount_cents"])
	}
	if created["tx_id"] != txID {
		t.Fatalf("tx_id = %v, want %s", created["tx_id"], txID)
	}

	// D2：修改
	rr, _ := doJSONReq(t, env.handler, http.MethodPut, "/api/v1/transactions/"+txID, map[string]interface{}{"amount": 5000}, authHeaders(aliceToken))
	if rr.Code != http.StatusOK {
		t.Fatalf("update tx failed: %d %s", rr.Code, rr.Body.String())
	}
	items, _, _, _ = listActivities(t, env, bobToken, bookID, 20, "")
	if _, ok := findAction(items, model.ActivityTxUpdated); !ok {
		t.Fatalf("expected transaction_updated activity")
	}

	// D3：删除
	rr, _ = doJSONReq(t, env.handler, http.MethodDelete, "/api/v1/transactions/"+txID, nil, authHeaders(aliceToken))
	if rr.Code != http.StatusOK {
		t.Fatalf("delete tx failed: %d %s", rr.Code, rr.Body.String())
	}
	items, _, _, _ = listActivities(t, env, bobToken, bookID, 20, "")
	if _, ok := findAction(items, model.ActivityTxDeleted); !ok {
		t.Fatalf("expected transaction_deleted activity")
	}

	// D11：非成员 403；D12：未登录 401
	_, _, _, code = listActivities(t, env, carolToken, bookID, 20, "")
	if code != http.StatusForbidden {
		t.Fatalf("non-member list activities = %d, want 403", code)
	}
	_, _, _, code = listActivities(t, env, "", bookID, 20, "")
	if code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list activities = %d, want 401", code)
	}

	// D16：另一账本的动态不串台
	book2ID := createTestBook(t, env, aliceToken, "旅行账本")
	items2, _, _, _ := listActivities(t, env, aliceToken, book2ID, 20, "")
	if countAction(items2, model.ActivityTxCreated) != 0 {
		t.Fatalf("book2 activities leaked book1 transactions: %v", items2)
	}
}

// D15：B 的 SSE 流收到带动态摘要的 ledger-changed
func TestActivityStreamCarriesSummary(t *testing.T) {
	env, aliceToken, bobToken, _, bookID, walletID, _ := setupActivityEnv(t)
	srv := httptest.NewServer(env.handler)
	defer srv.Close()

	streamURL := fmt.Sprintf("%s/api/v1/stream?bookId=%s&token=%s", srv.URL, bookID, bobToken)
	c := openSSE(t, streamURL)
	c.waitEvent(t, "connected", "", 5*time.Second)

	createTxFull(t, env, aliceToken, bookID, walletID, "expense", "晚餐", 4500, "", nil)
	ev := c.waitEvent(t, "ledger-changed", "transaction", 5*time.Second)
	summary, _ := ev.data["summary"].(string)
	if summary == "" {
		t.Fatalf("ledger-changed event has no summary: %v", ev.data)
	}
}

// D13：分页无重复无遗漏
func TestActivityPagination(t *testing.T) {
	env, aliceToken, bobToken, _, bookID, walletID, _ := setupActivityEnv(t)
	for i := 0; i < 25; i++ {
		createTxFull(t, env, aliceToken, bookID, walletID, "expense", fmt.Sprintf("第%d笔", i+1), 100, "", nil)
	}
	seen := make(map[string]bool)
	before := ""
	pages := 0
	for {
		items, hasMore, nextBefore, code := listActivities(t, env, bobToken, bookID, 10, before)
		if code != http.StatusOK {
			t.Fatalf("list activities failed: %d", code)
		}
		pages++
		for _, it := range items {
			id, _ := it["id"].(string)
			if seen[id] {
				t.Fatalf("duplicate activity id %s across pages", id)
			}
			seen[id] = true
		}
		if !hasMore {
			break
		}
		if nextBefore == "" {
			t.Fatalf("has_more=true but next_before empty")
		}
		before = nextBefore
		if pages > 10 {
			t.Fatalf("too many pages")
		}
	}
	// 25 笔交易 + 1 加成员 + 1 建钱包 = 27 条
	if len(seen) != 27 {
		t.Fatalf("total activities = %d, want 27", len(seen))
	}
}

// D14：保留上限（1000 条 / 90 天）
func TestActivityRetention(t *testing.T) {
	env, aliceToken, _, _, bookID, walletID, _ := setupActivityEnv(t)
	ctx := context.Background()
	// 1005 条近期 + 5 条 100 天前
	for i := 0; i < 1005; i++ {
		_, err := env.db.ExecContext(ctx, `
			INSERT INTO book_activities (id, book_id, user_id, actor_name, action, summary, created_at)
			VALUES (?, ?, 'u1', '测试', 'transaction_created', '旧动态', datetime('now'))
		`, fmt.Sprintf("old_%04d", i), bookID)
		if err != nil {
			t.Fatalf("insert old activity: %v", err)
		}
	}
	for i := 0; i < 5; i++ {
		_, err := env.db.ExecContext(ctx, `
			INSERT INTO book_activities (id, book_id, user_id, actor_name, action, summary, created_at)
			VALUES (?, ?, 'u1', '测试', 'transaction_created', '很旧的动态', datetime('now', '-100 days'))
		`, fmt.Sprintf("ancient_%d", i), bookID)
		if err != nil {
			t.Fatalf("insert ancient activity: %v", err)
		}
	}
	// 再记一笔触发清理
	createTxFull(t, env, aliceToken, bookID, walletID, "expense", "触发清理", 100, "", nil)
	n, err := env.activityRepo.Count(ctx, bookID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n > 1000 {
		t.Fatalf("activities after prune = %d, want <= 1000", n)
	}
	// 100 天前的 5 条必须已清
	var ancient int
	_ = env.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM book_activities WHERE book_id = ? AND summary = '很旧的动态'`, bookID).Scan(&ancient)
	if ancient != 0 {
		t.Fatalf("ancient activities not pruned: %d remain", ancient)
	}
}

// D5/D6：导入与定时入账留痕
func TestActivityImportAndRecurring(t *testing.T) {
	env, aliceToken, bobToken, _, bookID, walletID, _ := setupActivityEnv(t)
	ctx := context.Background()

	// D5：导入提交 3 条
	rr, resp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/imports/commit", map[string]interface{}{
		"book_id": bookID, "wallet_id": walletID,
		"items": []map[string]interface{}{
			{"date": time.Now().Format("2006-01-02"), "type": "expense", "amount": 1200, "counterparty": "测试店", "product": "午餐"},
			{"date": time.Now().Format("2006-01-02"), "type": "expense", "amount": 800, "counterparty": "测试店", "product": "晚餐"},
			{"date": time.Now().Format("2006-01-02"), "type": "income", "amount": 5000, "counterparty": "公司", "product": "报销"},
		},
	}, authHeaders(aliceToken))
	if rr.Code != http.StatusOK {
		t.Fatalf("import commit failed: %d %s", rr.Code, rr.Body.String())
	}
	if inserted, _ := resp["data"].(map[string]interface{})["inserted"].(float64); inserted != 3 {
		t.Fatalf("inserted = %v, want 3", inserted)
	}
	items, _, _, _ := listActivities(t, env, bobToken, bookID, 20, "")
	imp, ok := findAction(items, model.ActivityImportCommitted)
	if !ok {
		t.Fatalf("expected import_committed activity")
	}
	if s, _ := imp["summary"].(string); s == "" {
		t.Fatalf("import activity summary empty")
	}

	// D6：定时规则到点入账（next_date 用昨天：已逾期的规则立即补记，
	// 不受“当天 9 点才入账”的门限制；当天到期的规则在 9 点前不入账，
	// 见 service.TestShouldPostRule）
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/recurring", map[string]interface{}{
		"book_id": bookID, "type": "expense", "title": "房租", "amount": 300000,
		"cycle": "每月", "start_date": time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
		"next_date": time.Now().AddDate(0, 0, -1).Format("2006-01-02"), "wallet_id": walletID,
	}, authHeaders(aliceToken))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create recurring rule failed: %d %s", rr.Code, rr.Body.String())
	}
	if n := env.recurringScheduler.RunOnce(ctx); n != 1 {
		t.Fatalf("recurring RunOnce executed = %d, want 1", n)
	}
	items, _, _, _ = listActivities(t, env, bobToken, bookID, 50, "")
	posted, ok := findAction(items, model.ActivityRecurringPosted)
	if !ok {
		t.Fatalf("expected recurring_posted activity")
	}
	if posted["actor_name"] != "定时任务" {
		t.Fatalf("recurring_posted actor = %v, want 定时任务", posted["actor_name"])
	}
}

// ---------- B1-B4/B6：月总预算 80%/100% ----------

func TestBudgetAlertTotalLevels(t *testing.T) {
	env, aliceToken, bobToken, _, bookID, walletID, _ := setupActivityEnv(t)
	srv := httptest.NewServer(env.handler)
	defer srv.Close()

	setTotalBudget(t, env, aliceToken, bookID, 10000) // ¥100

	// B 的流：用来断言 budget-alert 真的推了
	c := openSSE(t, fmt.Sprintf("%s/api/v1/stream?bookId=%s&token=%s", srv.URL, bookID, bobToken))
	c.waitEvent(t, "connected", "", 5*time.Second)

	alerts := func() []map[string]interface{} {
		items, _, _, _ := listActivities(t, env, bobToken, bookID, 50, "")
		out := []map[string]interface{}{}
		for _, it := range items {
			if it["action"] == model.ActivityBudgetAlert {
				out = append(out, it)
			}
		}
		return out
	}

	// B1：花到 ¥80 → 80% 提醒一次（SSE + 动态）
	createTxFull(t, env, aliceToken, bookID, walletID, "expense", "第一笔", 8000, "", nil)
	ev := c.waitEvent(t, "budget-alert", "budget", 5*time.Second)
	if ba, ok := ev.data["budgetAlert"].(map[string]interface{}); !ok {
		t.Fatalf("budget-alert event has no budgetAlert payload: %v", ev.data)
	} else if lv, _ := ba["level"].(float64); lv != 80 {
		t.Fatalf("budget-alert level = %v, want 80", ba["level"])
	}
	if n := len(alerts()); n != 1 {
		t.Fatalf("budget_alert activities = %d, want 1", n)
	}

	// B2：再花 ¥10（累计 ¥90）→ 不再提醒
	createTxFull(t, env, aliceToken, bookID, walletID, "expense", "第二笔", 1000, "", nil)
	c.expectNoEvent(t, "budget-alert", 2*time.Second)
	if n := len(alerts()); n != 1 {
		t.Fatalf("budget_alert activities after B2 = %d, want 1", n)
	}

	// B3：再花 ¥10（累计 ¥100）→ 100% 提醒一次
	createTxFull(t, env, aliceToken, bookID, walletID, "expense", "第三笔", 1000, "", nil)
	ev = c.waitEvent(t, "budget-alert", "budget", 5*time.Second)
	if ba, ok := ev.data["budgetAlert"].(map[string]interface{}); !ok {
		t.Fatalf("budget-alert event has no budgetAlert payload: %v", ev.data)
	} else if lv, _ := ba["level"].(float64); lv != 100 {
		t.Fatalf("budget-alert level = %v, want 100", ba["level"])
	}
	if n := len(alerts()); n != 2 {
		t.Fatalf("budget_alert activities after B3 = %d, want 2", n)
	}

	// B4：再花 ¥5（累计 ¥105）→ 不再提醒
	createTxFull(t, env, aliceToken, bookID, walletID, "expense", "第四笔", 500, "", nil)
	c.expectNoEvent(t, "budget-alert", 2*time.Second)
	if n := len(alerts()); n != 2 {
		t.Fatalf("budget_alert activities after B4 = %d, want 2", n)
	}

	// B6：删掉 ¥10 的一笔（累计 ¥95）→ 不产生新提醒；再记 ¥10 回到 ¥105 时再次提醒 100%
	// 直接删“第二笔”（¥10）：先通过列表接口找到它
	rr, resp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/transactions?book_id="+bookID+"&page_size=50", nil, authHeaders(aliceToken))
	if rr.Code != http.StatusOK {
		t.Fatalf("list transactions failed: %d", rr.Code)
	}
	txItems, _ := resp["data"].(map[string]interface{})["items"].([]interface{})
	var secondTxID string
	for _, it := range txItems {
		m, _ := it.(map[string]interface{})
		if m == nil {
			continue
		}
		if amt, _ := m["amount"].(float64); amt == 1000 {
			if title, _ := m["title"].(string); title == "第二笔" {
				secondTxID, _ = m["id"].(string)
			}
		}
	}
	if secondTxID == "" {
		t.Fatalf("second tx not found")
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodDelete, "/api/v1/transactions/"+secondTxID, nil, authHeaders(aliceToken))
	if rr.Code != http.StatusOK {
		t.Fatalf("delete tx failed: %d", rr.Code)
	}
	c.expectNoEvent(t, "budget-alert", 2*time.Second)
	if n := len(alerts()); n != 2 {
		t.Fatalf("budget_alert activities after delete = %d, want 2", n)
	}
	// 再记 ¥10 回到 ¥105 → 100% 再次提醒
	createTxFull(t, env, aliceToken, bookID, walletID, "expense", "第五笔", 1000, "", nil)
	c.waitEvent(t, "budget-alert", "budget", 5*time.Second)
	if n := len(alerts()); n != 3 {
		t.Fatalf("budget_alert activities after re-cross = %d, want 3", n)
	}
}

// B5：分类预算；B7/B8：不计入预算的支出与收入不触发
func TestBudgetAlertCategoryAndExclusions(t *testing.T) {
	env, aliceToken, bobToken, _, bookID, walletID, _ := setupActivityEnv(t)
	diningID := topCategoryID(t, env, aliceToken, bookID, "餐饮")
	setCategoryBudget(t, env, aliceToken, bookID, diningID, 5000) // ¥50
	setTotalBudget(t, env, aliceToken, bookID, 100000)            // 总预算设很大，避免总预算先触发

	alerts := func() []map[string]interface{} {
		items, _, _, _ := listActivities(t, env, bobToken, bookID, 50, "")
		out := []map[string]interface{}{}
		for _, it := range items {
			if it["action"] == model.ActivityBudgetAlert {
				out = append(out, it)
			}
		}
		return out
	}

	// B7：不计入预算的 ¥200 支出不触发
	createTxFull(t, env, aliceToken, bookID, walletID, "expense", "不计入", 20000, diningID, boolPtr(false))
	if n := len(alerts()); n != 0 {
		t.Fatalf("budget_alert after excluded expense = %d, want 0", n)
	}
	// B8：收入不触发
	createTxFull(t, env, aliceToken, bookID, walletID, "income", "工资", 100000, "", nil)
	if n := len(alerts()); n != 0 {
		t.Fatalf("budget_alert after income = %d, want 0", n)
	}
	// B5：餐饮 ¥30（60%）无提醒；再记 ¥25（累计 ¥55，110%）→ 100% 提醒 1 次
	// 注意：从 0 直接到 110% 会一次越过两档，按设计只发最高档（100%）一条
	createTxFull(t, env, aliceToken, bookID, walletID, "expense", "午餐", 3000, diningID, nil)
	if n := len(alerts()); n != 0 {
		t.Fatalf("budget_alert at 60%% = %d, want 0", n)
	}
	createTxFull(t, env, aliceToken, bookID, walletID, "expense", "晚餐", 2500, diningID, nil)
	got := alerts()
	if n := len(got); n != 1 {
		t.Fatalf("budget_alert after category cross = %d, want 1", n)
	}
	if s, _ := got[0]["summary"].(string); s == "" {
		t.Fatalf("category alert summary empty")
	}
}

// B9：调高预算后占比回落，不误报；再花到 80% 才提醒
func TestBudgetAlertRaiseLimit(t *testing.T) {
	env, aliceToken, bobToken, _, bookID, walletID, _ := setupActivityEnv(t)
	setTotalBudget(t, env, aliceToken, bookID, 10000)
	createTxFull(t, env, aliceToken, bookID, walletID, "expense", "一笔", 9000, "", nil)
	items, _, _, _ := listActivities(t, env, bobToken, bookID, 50, "")
	if n := countAction(items, model.ActivityBudgetAlert); n != 1 {
		t.Fatalf("budget_alert after ¥90/¥100 = %d, want 1", n)
	}
	// 预算调到 ¥1000：占比回落到 9%，不产生新提醒
	setTotalBudget(t, env, aliceToken, bookID, 100000)
	items, _, _, _ = listActivities(t, env, bobToken, bookID, 50, "")
	if n := countAction(items, model.ActivityBudgetAlert); n != 1 {
		t.Fatalf("budget_alert after raising limit = %d, want 1", n)
	}
	// 再花 ¥710（累计 ¥800，80%）→ 提醒一次
	createTxFull(t, env, aliceToken, bookID, walletID, "expense", "又一笔", 71000, "", nil)
	items, _, _, _ = listActivities(t, env, bobToken, bookID, 50, "")
	if n := countAction(items, model.ActivityBudgetAlert); n != 2 {
		t.Fatalf("budget_alert after re-cross 80%% = %d, want 2", n)
	}
}

// B10：每日检查补发越线提醒，且不重复
func TestBudgetAlertDailyCheck(t *testing.T) {
	env, aliceToken, bobToken, _, bookID, walletID, _ := setupActivityEnv(t)
	ctx := context.Background()
	setTotalBudget(t, env, aliceToken, bookID, 10000)

	// 绕过 handler 直接写库：当月支出 ¥90，但没有任何提醒
	tx := &model.Transaction{
		BookID: bookID, Type: "expense", Amount: 9000,
		Date: time.Now().Format("2006-01-02"), WalletID: walletID,
		Title: "直接入库", IncludeBudget: true, CreatedBy: "u1",
	}
	if err := env.txRepo.CreateTransaction(ctx, tx, nil); err != nil {
		t.Fatalf("direct insert tx: %v", err)
	}
	if n := env.budgetAlertScheduler.RunOnce(ctx); n != 1 {
		t.Fatalf("daily check RunOnce alerts = %d, want 1", n)
	}
	items, _, _, _ := listActivities(t, env, bobToken, bookID, 50, "")
	if n := countAction(items, model.ActivityBudgetAlert); n != 1 {
		t.Fatalf("budget_alert after daily check = %d, want 1", n)
	}
	// 再跑一次：不重复
	if n := env.budgetAlertScheduler.RunOnce(ctx); n != 0 {
		t.Fatalf("second daily check alerts = %d, want 0", n)
	}
}

// B11：C 的账本越线，A 的流里收不到提醒
func TestBudgetAlertIsolation(t *testing.T) {
	env, aliceToken, bobToken, carolToken, bookID, walletID, _ := setupActivityEnv(t)
	srv := httptest.NewServer(env.handler)
	defer srv.Close()

	// C 自己的账本，预算 ¥100，花 ¥90
	carolBook := createTestBook(t, env, carolToken, "小南的账本")
	carolWallet := createTestWallet(t, env, carolToken, carolBook, "现金")
	setTotalBudget(t, env, carolToken, carolBook, 10000)

	// A 连自己账本的流
	c := openSSE(t, fmt.Sprintf("%s/api/v1/stream?bookId=%s&token=%s", srv.URL, bookID, aliceToken))
	c.waitEvent(t, "connected", "", 5*time.Second)
	_ = bobToken
	_ = walletID

	createTxFull(t, env, carolToken, carolBook, carolWallet, "expense", "越线了", 9000, "", nil)
	c.expectNoEvent(t, "budget-alert", 2*time.Second)
}
