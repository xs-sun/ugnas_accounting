package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
)

// AI 聊天记录服务端存储：按用户 + 账本隔离，图片存服务器文件。
func TestAiChatMessagesServerStorage(t *testing.T) {
	env := setupTestEnv(t)
	ownerH := map[string]string{"Ugreen-User-ID": "u_owner", "Ugreen-User-Name": "小禾", "Ugreen-User-Type": "admin"}
	strangerH := map[string]string{"Ugreen-User-ID": "u_stranger", "Ugreen-User-Name": "路人"}

	rr, resp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/books", nil, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("list books failed: %d", rr.Code)
	}
	bookID := resp["data"].([]interface{})[0].(map[string]interface{})["id"].(string)
	base := "/api/v1/books/" + bookID

	// 非账本成员：读、写、删一律 403。
	rr, _ = doJSONReq(t, env.handler, http.MethodGet, base+"/ai-messages", nil, strangerH)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("stranger list expected 403, got %d", rr.Code)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, base+"/ai-messages", map[string]interface{}{
		"id": "u_1", "role": "user", "text": "hello",
	}, strangerH)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("stranger create expected 403, got %d", rr.Code)
	}

	// 创建两条记录：用户提问 + 带草稿（含 member_id，验证 draft 原样往返）的助手回复。
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, base+"/ai-messages", map[string]interface{}{
		"id": "u_1", "role": "user", "text": "瑞幸拿铁 18.5",
	}, ownerH)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create user msg failed: %d %s", rr.Code, rr.Body.String())
	}
	draft := map[string]interface{}{
		"type": "expense", "amount": 1850, "title": "瑞幸拿铁",
		"category_name": "餐饮", "sub_category": "咖啡饮品",
		"wallet_name": "微信支付", "date": "2026-11-26", "member_id": "u_owner",
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, base+"/ai-messages", map[string]interface{}{
		"id": "a_1", "role": "assistant", "text": "已识别", "draft": draft,
	}, ownerH)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create assistant msg failed: %d %s", rr.Code, rr.Body.String())
	}

	// 引用不存在的图片 400。
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, base+"/ai-messages", map[string]interface{}{
		"id": "u_bad", "role": "user", "text": "", "image_path": "nope.jpg",
	}, ownerH)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad image_path expected 400, got %d", rr.Code)
	}

	// 列表：按插入顺序，两条，草稿里的 member_id 还在。
	rr, resp = doJSONReq(t, env.handler, http.MethodGet, base+"/ai-messages", nil, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("list failed: %d", rr.Code)
	}
	msgs := resp["data"].(map[string]interface{})["messages"].([]interface{})
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].(map[string]interface{})["text"] != "瑞幸拿铁 18.5" {
		t.Fatalf("first message text mismatch: %+v", msgs[0])
	}
	gotDraft := msgs[1].(map[string]interface{})["draft"].(map[string]interface{})
	if gotDraft["member_id"] != "u_owner" || gotDraft["amount"] != float64(1850) {
		t.Fatalf("draft did not round-trip: %+v", gotDraft)
	}

	// 另一位账本成员（直接经 repo 写入）看不到 u_owner 的记录：按用户隔离。
	// 先让 u_other 请求一次，使其中间件建出用户行（外键要求用户存在）。
	otherH := map[string]string{"Ugreen-User-ID": "u_other", "Ugreen-User-Name": "另一位"}
	rr, _ = doJSONReq(t, env.handler, http.MethodGet, "/api/v1/books", nil, otherH)
	if rr.Code != http.StatusOK {
		t.Fatalf("other user init failed: %d", rr.Code)
	}
	otherRepo := repository.NewAiChatRepo(env.db)
	if err := otherRepo.CreateMessage(context.Background(), &model.AiChatMessage{
		ID: "u_other_1", BookID: bookID, UserID: "u_other", Role: "user", Text: "别人的话",
	}); err != nil {
		t.Fatalf("seed other user msg: %v", err)
	}
	rr, resp = doJSONReq(t, env.handler, http.MethodGet, base+"/ai-messages", nil, ownerH)
	msgs = resp["data"].(map[string]interface{})["messages"].([]interface{})
	if len(msgs) != 2 {
		t.Fatalf("owner should still see only own 2 messages, got %d", len(msgs))
	}
	// u_owner 不能改别人的记录。
	rr, _ = doJSONReq(t, env.handler, http.MethodPatch, base+"/ai-messages/u_other_1", map[string]interface{}{
		"text": "篡改",
	}, ownerH)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("patch other user's msg expected 404, got %d", rr.Code)
	}

	// 更新自己的记录：OCR 文本 + 入账状态。
	rr, _ = doJSONReq(t, env.handler, http.MethodPatch, base+"/ai-messages/a_1", map[string]interface{}{
		"ocr_text": "瑞幸咖啡 18.50", "saved": true, "saved_tx_id": "tx_1",
	}, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch failed: %d %s", rr.Code, rr.Body.String())
	}
	rr, resp = doJSONReq(t, env.handler, http.MethodGet, base+"/ai-messages", nil, ownerH)
	msgs = resp["data"].(map[string]interface{})["messages"].([]interface{})
	second := msgs[1].(map[string]interface{})
	if second["ocr_text"] != "瑞幸咖啡 18.50" || second["saved"] != true || second["saved_tx_id"] != "tx_1" {
		t.Fatalf("patch did not persist: %+v", second)
	}

	// 图片上传 + 查看 + 绑定到消息。
	var mpBuf bytes.Buffer
	mw := multipart.NewWriter(&mpBuf)
	fw, _ := mw.CreateFormFile("file", "receipt.jpg")
	_, _ = fw.Write([]byte("\xff\xd8\xff\xe0fakejpeg"))
	_ = mw.Close()
	mpReq := httptest.NewRequest(http.MethodPost, base+"/ai-images", &mpBuf)
	mpReq.Header.Set("Content-Type", mw.FormDataContentType())
	mpReq.Header.Set("Ugreen-User-ID", "u_owner")
	mpReq.Header.Set("Ugreen-User-Name", "小禾")
	mpRec := httptest.NewRecorder()
	env.handler.ServeHTTP(mpRec, mpReq)
	if mpRec.Code != http.StatusCreated {
		t.Fatalf("upload image failed: %d %s", mpRec.Code, mpRec.Body.String())
	}
	var uploadResp map[string]interface{}
	_ = json.Unmarshal(mpRec.Body.Bytes(), &uploadResp)
	imagePath := uploadResp["data"].(map[string]interface{})["image_path"].(string)
	if imagePath == "" {
		t.Fatalf("empty image_path in upload response")
	}

	getReq := httptest.NewRequest(http.MethodGet, base+"/ai-images/"+imagePath, nil)
	getReq.Header.Set("Ugreen-User-ID", "u_owner")
	getRec := httptest.NewRecorder()
	env.handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("serve image failed: %d %s", getRec.Code, getRec.Body.String())
	}
	if ct := getRec.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %q", ct)
	}
	if !bytes.Equal(getRec.Body.Bytes(), []byte("\xff\xd8\xff\xe0fakejpeg")) {
		t.Fatalf("served image bytes mismatch")
	}
	// 非成员看不到图片。
	getReq2 := httptest.NewRequest(http.MethodGet, base+"/ai-images/"+imagePath, nil)
	getReq2.Header.Set("Ugreen-User-ID", "u_stranger")
	getRec2 := httptest.NewRecorder()
	env.handler.ServeHTTP(getRec2, getReq2)
	if getRec2.Code != http.StatusForbidden {
		t.Fatalf("stranger serve image expected 403, got %d", getRec2.Code)
	}
	// 不存在的文件 404，非法扩展名 400。
	rr, _ = doJSONReq(t, env.handler, http.MethodGet, base+"/ai-images/nope.jpg", nil, ownerH)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing image expected 404, got %d", rr.Code)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodGet, base+"/ai-images/x.txt", nil, ownerH)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad extension expected 400, got %d", rr.Code)
	}

	// 把图片绑到用户消息上，再清空：记录没了，文件也删了。
	rr, _ = doJSONReq(t, env.handler, http.MethodPatch, base+"/ai-messages/u_1", map[string]interface{}{
		"image_path": imagePath, "image_label": "小票",
	}, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch image_path failed: %d %s", rr.Code, rr.Body.String())
	}
	rr, resp = doJSONReq(t, env.handler, http.MethodDelete, base+"/ai-messages", nil, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("clear failed: %d %s", rr.Code, rr.Body.String())
	}
	if int(resp["data"].(map[string]interface{})["deleted"].(float64)) != 2 {
		t.Fatalf("expected deleted=2, got %+v", resp["data"])
	}
	if _, err := os.Stat(filepath.Join(env.dataDir, "ai-images", imagePath)); !os.IsNotExist(err) {
		t.Fatalf("image file should be deleted on clear, stat err: %v", err)
	}
	rr, resp = doJSONReq(t, env.handler, http.MethodGet, base+"/ai-messages", nil, ownerH)
	msgs = resp["data"].(map[string]interface{})["messages"].([]interface{})
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after clear, got %d", len(msgs))
	}
}

// 超过 200 条时删最旧的，只留最新 200 条。
func TestAiChatMessagesPrune(t *testing.T) {
	env := setupTestEnv(t)
	ownerH := map[string]string{"Ugreen-User-ID": "u_owner", "Ugreen-User-Name": "小禾", "Ugreen-User-Type": "admin"}
	rr, resp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/books", nil, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("list books failed: %d", rr.Code)
	}
	bookID := resp["data"].([]interface{})[0].(map[string]interface{})["id"].(string)
	base := "/api/v1/books/" + bookID + "/ai-messages"

	for i := 0; i < 205; i++ {
		rr, _ = doJSONReq(t, env.handler, http.MethodPost, base, map[string]interface{}{
			"id":   fmt.Sprintf("m_%d", i),
			"role": "user", "text": "msg",
		}, ownerH)
		if rr.Code != http.StatusCreated {
			t.Fatalf("create %d failed: %d %s", i, rr.Code, rr.Body.String())
		}
	}
	rr, resp = doJSONReq(t, env.handler, http.MethodGet, base, nil, ownerH)
	msgs := resp["data"].(map[string]interface{})["messages"].([]interface{})
	if len(msgs) != 200 {
		t.Fatalf("expected 200 messages after prune, got %d", len(msgs))
	}
}
