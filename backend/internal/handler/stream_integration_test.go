package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// sseEvent 是流里一条事件的解析结果。
type sseEvent struct {
	event string
	data  map[string]interface{}
}

// sseClient 读一条 SSE 流，把完整事件发到 events channel；读到 EOF 或出错时关闭 done。
type sseClient struct {
	resp   *http.Response
	events chan sseEvent
	done   chan struct{}
}

func openSSE(t *testing.T, url string) *sseClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	_ = cancel // 连接随 resp.Body 关闭而结束；测试结束时显式关闭。
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build stream request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	c := &sseClient{resp: resp, events: make(chan sseEvent, 64), done: make(chan struct{})}
	go c.readLoop()
	t.Cleanup(func() {
		_ = resp.Body.Close()
	})
	return c
}

func (c *sseClient) readLoop() {
	defer close(c.done)
	reader := bufio.NewReader(c.resp.Body)
	var ev sseEvent
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			if ev.event != "" || ev.data != nil {
				c.events <- ev
			}
			ev = sseEvent{}
		case strings.HasPrefix(line, "event: "):
			ev.event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &data); err == nil {
				ev.data = data
			}
		}
	}
}

// waitEvent 等待一条 event 名匹配、且 data.change（若给出）匹配的事件。
func (c *sseClient) waitEvent(t *testing.T, wantEvent, wantChange string, timeout time.Duration) sseEvent {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-c.events:
			if ev.event != wantEvent {
				continue
			}
			if wantChange != "" && ev.data["change"] != wantChange {
				continue
			}
			return ev
		case <-deadline:
			t.Fatalf("timed out waiting for event %q change %q", wantEvent, wantChange)
			return sseEvent{}
		}
	}
}

// expectNoEvent 确认在 timeout 内没有收到匹配的事件。
func (c *sseClient) expectNoEvent(t *testing.T, wantEvent string, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-c.events:
			if ev.event == wantEvent {
				t.Fatalf("unexpected event %q: %+v", wantEvent, ev.data)
			}
		case <-deadline:
			return
		}
	}
}

func registerTestUser(t *testing.T, env *testEnv, name, email string) (userID, token string) {
	t.Helper()
	_, resp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/send-code", map[string]string{
		"email":   email,
		"purpose": "register",
	}, nil)
	code, _ := resp["data"].(map[string]interface{})["demo_code"].(string)
	rr, resp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"name":     name,
		"email":    email,
		"code":     code,
		"password": "password123",
	}, nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("register %s failed: %d %s", email, rr.Code, rr.Body.String())
	}
	data := resp["data"].(map[string]interface{})
	token, _ = data["token"].(string)
	if token == "" {
		t.Fatalf("register %s returned no token: %v", email, resp)
	}
	userID = ""
	if u, ok := data["user"].(map[string]interface{}); ok {
		userID, _ = u["user_id"].(string)
	}
	if userID == "" {
		t.Fatalf("register %s returned no user id: %v", email, resp)
	}
	return userID, token
}

func authHeaders(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func createTestBook(t *testing.T, env *testEnv, token, name string) string {
	t.Helper()
	rr, resp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books", map[string]interface{}{
		"name": name,
	}, authHeaders(token))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create book failed: %d %s", rr.Code, rr.Body.String())
	}
	id, _ := resp["data"].(map[string]interface{})["id"].(string)
	if id == "" {
		t.Fatalf("create book returned no id: %v", resp)
	}
	return id
}

func createTestWallet(t *testing.T, env *testEnv, token, bookID, name string) string {
	t.Helper()
	rr, resp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/wallets", map[string]interface{}{
		"book_id": bookID,
		"name":    name,
	}, authHeaders(token))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create wallet failed: %d %s", rr.Code, rr.Body.String())
	}
	id, _ := resp["data"].(map[string]interface{})["id"].(string)
	if id == "" {
		t.Fatalf("create wallet returned no id: %v", resp)
	}
	return id
}

func createTestTransaction(t *testing.T, env *testEnv, token, bookID, walletID string, amount int64) string {
	t.Helper()
	rr, resp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/transactions", map[string]interface{}{
		"book_id":  bookID,
		"wallet_id": walletID,
		"type":     "expense",
		"amount":   amount,
		"note":     "sse test",
	}, authHeaders(token))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create transaction failed: %d %s", rr.Code, rr.Body.String())
	}
	id, _ := resp["data"].(map[string]interface{})["id"].(string)
	if id == "" {
		t.Fatalf("create transaction returned no id: %v", resp)
	}
	return id
}

// TestStreamRealtimeSync：两人同账本的端到端实时同步（T1/T2/T3/T4/T5/T8/T11/T13/T14）。
func TestStreamRealtimeSync(t *testing.T) {
	t.Setenv("STRICT_AUTH", "1")
	env := setupTestEnv(t)
	srv := httptest.NewServer(env.handler)
	defer srv.Close()

	aliceID, aliceToken := registerTestUser(t, env, "小禾", "alice@cat7.app")
	bobID, bobToken := registerTestUser(t, env, "小北", "bob@cat7.app")
	carolID, carolToken := registerTestUser(t, env, "小南", "carol@cat7.app")
	_ = carolID

	bookID := createTestBook(t, env, aliceToken, "家庭账本")
	book2ID := createTestBook(t, env, aliceToken, "旅行账本")

	// A 把 B 加进两个账本；C 不是任何账本的成员。
	for _, b := range []string{bookID, book2ID} {
		rr, _ := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/"+b+"/members", map[string]interface{}{
			"user_id": bobID,
			"role":    "editor",
		}, authHeaders(aliceToken))
		if rr.Code != http.StatusCreated {
			t.Fatalf("add member to %s failed: %d %s", b, rr.Code, rr.Body.String())
		}
	}
	_ = aliceID

	walletID := createTestWallet(t, env, aliceToken, bookID, "微信钱包")
	wallet2ID := createTestWallet(t, env, aliceToken, book2ID, "现金")

	// T14：B 连上账本 1 的流，立即收到 connected 事件，带当前 seq。
	streamURL := fmt.Sprintf("%s/api/v1/stream?bookId=%s&token=%s", srv.URL, bookID, bobToken)
	bStream := openSSE(t, streamURL)
	if bStream.resp.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d, want 200", bStream.resp.StatusCode)
	}
	if ct := bStream.resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	connected := bStream.waitEvent(t, "connected", "", 5*time.Second)
	seq0, _ := connected.data["seq"].(float64)

	// T4：非成员 C 连流 → 403。
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/stream?bookId=%s&token=%s", srv.URL, bookID, carolToken))
	if err != nil {
		t.Fatalf("non-member stream request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-member stream status = %d, want 403", resp.StatusCode)
	}

	// T5：未登录连流 → 401。
	resp, err = http.Get(fmt.Sprintf("%s/api/v1/stream?bookId=%s", srv.URL, bookID))
	if err != nil {
		t.Fatalf("anonymous stream request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous stream status = %d, want 401", resp.StatusCode)
	}

	// T8：B 再连一条账本 2 的流；账本 1 的变更只应出现在流 1。
	stream2URL := fmt.Sprintf("%s/api/v1/stream?bookId=%s&token=%s", srv.URL, book2ID, bobToken)
	bStream2 := openSSE(t, stream2URL)
	bStream2.waitEvent(t, "connected", "", 5*time.Second)

	// T1：A 在账本 1 记一笔，B 的流 1 收到 transaction 事件，seq 连续；流 2 收不到。
	txID := createTestTransaction(t, env, aliceToken, bookID, walletID, 3200)
	ev := bStream.waitEvent(t, "ledger-changed", "transaction", 5*time.Second)
	if ev.data["seq"].(float64) != seq0+1 {
		t.Fatalf("first event seq = %v, want %v", ev.data["seq"], seq0+1)
	}
	if ev.data["bookId"] != bookID {
		t.Fatalf("event bookId = %v, want %s", ev.data["bookId"], bookID)
	}
	if ev.data["actorName"] != "小禾" {
		t.Fatalf("event actorName = %v, want 小禾", ev.data["actorName"])
	}
	bStream2.expectNoEvent(t, "ledger-changed", 500*time.Millisecond)

	// T2：A 改这笔，B 收到。
	rr, _ := doJSONReq(t, env.handler, http.MethodPut, "/api/v1/transactions/"+txID, map[string]interface{}{
		"amount": 4500,
	}, authHeaders(aliceToken))
	if rr.Code != http.StatusOK {
		t.Fatalf("update transaction failed: %d %s", rr.Code, rr.Body.String())
	}
	ev = bStream.waitEvent(t, "ledger-changed", "transaction", 5*time.Second)
	if ev.data["seq"].(float64) != seq0+2 {
		t.Fatalf("update event seq = %v, want %v", ev.data["seq"], seq0+2)
	}

	// T13：钱包/分类/预算/成员变更各推一条对应 change 的事件。
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/wallets", map[string]interface{}{
		"book_id": bookID,
		"name":    "支付宝",
	}, authHeaders(aliceToken))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create wallet 2 failed: %d %s", rr.Code, rr.Body.String())
	}
	bStream.waitEvent(t, "ledger-changed", "wallet", 5*time.Second)

	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/categories", map[string]interface{}{
		"book_id": bookID,
		"name":    "测试分类",
		"type":    "expense",
	}, authHeaders(aliceToken))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create category failed: %d %s", rr.Code, rr.Body.String())
	}
	bStream.waitEvent(t, "ledger-changed", "category", 5*time.Second)

	rr, _ = doJSONReq(t, env.handler, http.MethodPut, "/api/v1/budgets/total", map[string]interface{}{
		"book_id": bookID,
		"budget":  500000,
	}, authHeaders(aliceToken))
	if rr.Code != http.StatusOK {
		t.Fatalf("update total budget failed: %d %s", rr.Code, rr.Body.String())
	}
	bStream.waitEvent(t, "ledger-changed", "budget", 5*time.Second)

	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/"+bookID+"/members", map[string]interface{}{
		"user_id": carolID,
		"role":    "viewer",
	}, authHeaders(aliceToken))
	if rr.Code != http.StatusCreated {
		t.Fatalf("add carol as member failed: %d %s", rr.Code, rr.Body.String())
	}
	bStream.waitEvent(t, "ledger-changed", "member", 5*time.Second)

	// T11：连续 20 笔，事件不丢、seq 连续。
	baseSeq := seq0 + 6 // 到此已收到 6 条 ledger-changed
	for i := 0; i < 20; i++ {
		createTestTransaction(t, env, aliceToken, bookID, walletID, int64(100+i))
	}
	for i := 0; i < 20; i++ {
		ev := bStream.waitEvent(t, "ledger-changed", "transaction", 5*time.Second)
		want := baseSeq + 1 + float64(i)
		if ev.data["seq"].(float64) != want {
			t.Fatalf("burst event %d seq = %v, want %v", i, ev.data["seq"], want)
		}
	}

	// T3：A 删一笔，再批量删两笔，B 各收到一条 transaction 事件。
	rr, _ = doJSONReq(t, env.handler, http.MethodDelete, "/api/v1/transactions/"+txID, nil, authHeaders(aliceToken))
	if rr.Code != http.StatusOK {
		t.Fatalf("delete transaction failed: %d %s", rr.Code, rr.Body.String())
	}
	bStream.waitEvent(t, "ledger-changed", "transaction", 5*time.Second)

	txA := createTestTransaction(t, env, aliceToken, bookID, walletID, 100)
	txB := createTestTransaction(t, env, aliceToken, bookID, walletID, 200)
	// 这两笔创建各推一条事件，先消费掉。
	bStream.waitEvent(t, "ledger-changed", "transaction", 5*time.Second)
	bStream.waitEvent(t, "ledger-changed", "transaction", 5*time.Second)
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/transactions/batch-delete", map[string]interface{}{
		"ids": []string{txA, txB},
	}, authHeaders(aliceToken))
	if rr.Code != http.StatusOK {
		t.Fatalf("batch delete failed: %d %s", rr.Code, rr.Body.String())
	}
	bStream.waitEvent(t, "ledger-changed", "transaction", 5*time.Second)

	// 账本 2 的流在整个过程中只收到自己的 connected，没有账本 1 的事件。
	bStream2.expectNoEvent(t, "ledger-changed", 500*time.Millisecond)

	// 账本 2 本身也能推：A 在账本 2 记一笔，流 2 收到。
	createTestTransaction(t, env, aliceToken, book2ID, wallet2ID, 9900)
	bStream2.waitEvent(t, "ledger-changed", "transaction", 5*time.Second)

	_ = io.Discard
}
