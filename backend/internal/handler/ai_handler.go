package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"com.xsguyu.accounting/backend/internal/service"
)

type AiHandler struct {
	aiSvc *service.AiService
}

func NewAiHandler(aiSvc *service.AiService) *AiHandler {
	return &AiHandler{aiSvc: aiSvc}
}

func (h *AiHandler) Chat(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		BookID  string `json:"book_id"`
		Message string `json:"message"`
		IsVoice bool   `json:"is_voice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		sendError(w, http.StatusBadRequest, "message cannot be blank")
		return
	}

	reply, draft, usage, err := h.aiSvc.Chat(r.Context(), uCtx.ID, req.BookID, req.Message, req.IsVoice)
	if err != nil {
		if errors.Is(err, service.ErrAiLimitExceed) {
			sendError(w, http.StatusTooManyRequests, "今日 AI 调用次数已达上限")
			return
		}
		if errors.Is(err, service.ErrAiDisabled) {
			sendError(w, http.StatusForbidden, "AI 记账服务已关闭")
			return
		}
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"reply": reply,
		"draft": draft,
		"usage": usage,
	})
}

func (h *AiHandler) OCR(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var bookID, demo, fileName string
	var fileBytes []byte

	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "multipart/form-data") {
		const maxUpload = 10 * 1024 * 1024
		r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
		if err := r.ParseMultipartForm(maxUpload); err != nil {
			sendError(w, http.StatusBadRequest, "invalid multipart form")
			return
		}
		bookID = r.FormValue("book_id")
		demo = r.FormValue("demo")
		if f, hdr, err := r.FormFile("file"); err == nil {
			defer f.Close()
			if hdr != nil {
				fileName = hdr.Filename
			}
			fileBytes, _ = io.ReadAll(f)
		}
	} else {
		var req struct {
			BookID string `json:"book_id"`
			Demo   string `json:"demo"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendError(w, http.StatusBadRequest, "invalid json")
			return
		}
		bookID = req.BookID
		demo = req.Demo
	}

	reply, draft, rawText, usage, err := h.aiSvc.OCR(r.Context(), uCtx.ID, bookID, demo, fileBytes, fileName)
	if err != nil {
		if errors.Is(err, service.ErrAiLimitExceed) {
			sendError(w, http.StatusTooManyRequests, "今日 AI 调用次数已达上限")
			return
		}
		if errors.Is(err, service.ErrAiDisabled) {
			sendError(w, http.StatusForbidden, "OCR 服务已关闭")
			return
		}
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"reply":    reply,
		"draft":    draft,
		"raw_text": rawText,
		"usage":    usage,
	})
}

func (h *AiHandler) Insights(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	q := r.URL.Query()
	bookID := q.Get("book_id")
	period := q.Get("period")
	if period == "" {
		period = "month"
	}
	txType := q.Get("type")
	if txType == "" {
		txType = "expense"
	}
	seed := 0
	if sStr := q.Get("seed"); sStr != "" {
		if s, err := strconv.Atoi(sStr); err == nil {
			seed = s
		}
	}

	res, err := h.aiSvc.Insights(r.Context(), uCtx.ID, bookID, period, txType, seed)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", res)
}

func (h *AiHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
		Model   string `json:"model"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	res := h.aiSvc.TestConnection(r.Context(), req.BaseURL, req.APIKey, req.Model)
	sendJSON(w, 0, http.StatusOK, "success", res)
}
