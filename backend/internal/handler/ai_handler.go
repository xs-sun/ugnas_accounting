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

func (h *AiHandler) Capabilities(w http.ResponseWriter, r *http.Request) {
	caps := h.aiSvc.GetCapabilities(r.Context())
	sendJSON(w, 0, http.StatusOK, "success", caps)
}

func (h *AiHandler) Transcribe(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	const maxAudioUpload = 15 * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxAudioUpload)
	if err := r.ParseMultipartForm(maxAudioUpload); err != nil {
		sendError(w, http.StatusBadRequest, "音频文件格式无效或超过 15MB 上限")
		return
	}

	f, hdr, err := r.FormFile("file")
	if err != nil {
		sendError(w, http.StatusBadRequest, "请上传有效录音文件")
		return
	}
	defer f.Close()

	fileName := "voice.webm"
	if hdr != nil && strings.TrimSpace(hdr.Filename) != "" {
		fileName = hdr.Filename
	}
	audioBytes, err := io.ReadAll(io.LimitReader(f, maxAudioUpload))
	if err != nil {
		sendError(w, http.StatusBadRequest, "读取录音数据失败")
		return
	}

	text, usage, err := h.aiSvc.Transcribe(r.Context(), uCtx.ID, audioBytes, fileName)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAiLimitExceed):
			sendError(w, http.StatusTooManyRequests, "今日 AI 调用次数已达上限")
		case errors.Is(err, service.ErrAiDisabled):
			sendError(w, http.StatusForbidden, "AI 记账服务已关闭")
		case errors.Is(err, service.ErrSttNotConfigured):
			sendError(w, http.StatusServiceUnavailable, "尚未配置语音转写服务（STT / Whisper），请在管理后台填写支持音频转写的 API 地址与密钥")
		default:
			sendError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	rawText := text
	polishFlag := r.FormValue("polish") == "1" || r.URL.Query().Get("polish") == "1"
	if polishFlag && strings.TrimSpace(text) != "" {
		if pText, pUsage, pErr := h.aiSvc.PolishVoiceText(r.Context(), uCtx.ID, text); pErr == nil && strings.TrimSpace(pText) != "" {
			text = pText
			usage = pUsage
		}
	}

	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"text":     text,
		"raw_text": rawText,
		"usage":    usage,
	})
}

func (h *AiHandler) Polish(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		sendError(w, http.StatusBadRequest, "text cannot be blank")
		return
	}
	polished, usage, err := h.aiSvc.PolishVoiceText(r.Context(), uCtx.ID, req.Text)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAiLimitExceed):
			sendError(w, http.StatusTooManyRequests, "今日 AI 调用次数已达上限")
		case errors.Is(err, service.ErrAiDisabled):
			sendError(w, http.StatusForbidden, "AI 记账服务已关闭")
		default:
			sendError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"text":  polished,
		"usage": usage,
	})
}
