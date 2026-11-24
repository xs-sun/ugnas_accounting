package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"com.xsguyu.accounting/backend/internal/model"
)

// TestAgentChatViaHTTP：POST /api/v1/ai/chat 走 Agent 工具循环（H2）。
// 模型用本地 stub 模拟：第一轮返回 query_stats 的 tool_call，第二轮返回最终回答。
func TestAgentChatViaHTTP(t *testing.T) {
	env := setupTestEnv(t)
	_, token := registerTestUser(t, env, "小禾", "agent-xiaohe@test.local")
	bookID := createTestBook(t, env, token, "家庭账本")
	walletID := createTestWallet(t, env, token, bookID, "测试钱包")
	createTestTransaction(t, env, token, bookID, walletID, 3200)
	createTestTransaction(t, env, token, bookID, walletID, 5800)

	var mu sync.Mutex
	calls := 0
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		var body map[string]interface{}
		if n == 1 {
			body = map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]interface{}{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]interface{}{
							{"id": "call_stats", "type": "function", "function": map[string]interface{}{
								"name":      "query_stats",
								"arguments": `{"period":"month"}`,
							}},
						},
					}},
				},
				"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5},
			}
		} else {
			body = map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]interface{}{"role": "assistant", "content": "本月共花了 ¥90.00。"}},
				},
				"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5},
			}
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(stub.Close)

	cfg := &model.AiConfig{Enabled: true, Provider: "stub", BaseURL: stub.URL, APIKey: "sk-test", Model: "stub-model", DailyLimit: 30, Temperature: 0}
	if err := env.settingsRepo.SaveAiConfig(context.Background(), cfg); err != nil {
		t.Fatalf("SaveAiConfig: %v", err)
	}

	rr, resp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/ai/chat", map[string]interface{}{
		"book_id": bookID,
		"message": "这个月花了多少",
	}, authHeaders(token))
	if rr.Code != http.StatusOK {
		t.Fatalf("chat status = %d, body = %s", rr.Code, rr.Body.String())
	}
	data, _ := resp["data"].(map[string]interface{})
	reply, _ := data["reply"].(string)
	if !strings.Contains(reply, "90.00") {
		t.Errorf("reply = %q, want 90.00", reply)
	}
	usage, _ := data["usage"].(map[string]interface{})
	if tc, _ := usage["today_count"].(float64); tc != 1 {
		t.Errorf("today_count = %v, want 1", usage["today_count"])
	}
	mu.Lock()
	totalCalls := calls
	mu.Unlock()
	if totalCalls != 2 {
		t.Errorf("model called %d times, want 2 (one tool round + final answer)", totalCalls)
	}
}
