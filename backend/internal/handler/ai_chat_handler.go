package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
)

// AiChatHandler 管服务端的 AI 聊天记录：按用户 + 账本隔离，
// 各人只看自己的对话；图片文件存在 <dataDir>/ai-images/。
//
// 图片查看走带鉴权的 GET 接口，前端用 Bearer 取回 blob 再显示，
// 不把 token 放进 <img> 的 URL 里。
type AiChatHandler struct {
	repo     *repository.AiChatRepo
	bookRepo *repository.BookRepo
	dataDir  string
}

func NewAiChatHandler(repo *repository.AiChatRepo, bookRepo *repository.BookRepo, dataDir string) *AiChatHandler {
	return &AiChatHandler{repo: repo, bookRepo: bookRepo, dataDir: dataDir}
}

func (h *AiChatHandler) aiImagesDir() string {
	path := filepath.Join(h.dataDir, "ai-images")
	_ = os.MkdirAll(path, 0755)
	return path
}

func (h *AiChatHandler) requireBookMember(w http.ResponseWriter, r *http.Request, bookID string) bool {
	uCtx := GetUserContext(r)
	if _, err := h.bookRepo.GetMemberRole(r.Context(), bookID, uCtx.ID); err != nil {
		sendError(w, http.StatusForbidden, "not a member of this book")
		return false
	}
	return true
}

// imageFileName 只接受 ai-images 目录下的裸文件名，拒绝任何路径成分。
func imageFileName(name string) (string, bool) {
	if name == "" || name != filepath.Base(name) {
		return "", false
	}
	if strings.Contains(name, "..") {
		return "", false
	}
	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, ".jpg") && !strings.HasSuffix(lower, ".jpeg") &&
		!strings.HasSuffix(lower, ".png") && !strings.HasSuffix(lower, ".webp") {
		return "", false
	}
	return name, true
}

func (h *AiChatHandler) ListMessages(w http.ResponseWriter, r *http.Request, bookID string) {
	uCtx := GetUserContext(r)
	if !h.requireBookMember(w, r, bookID) {
		return
	}
	msgs, err := h.repo.ListMessages(r.Context(), bookID, uCtx.ID, repository.MaxAiChatMessagesPerBook)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if msgs == nil {
		msgs = []*model.AiChatMessage{}
	}
	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"messages": msgs,
	})
}

func (h *AiChatHandler) CreateMessage(w http.ResponseWriter, r *http.Request, bookID string) {
	uCtx := GetUserContext(r)
	if !h.requireBookMember(w, r, bookID) {
		return
	}
	var req struct {
		ID         string          `json:"id"`
		Role       string          `json:"role"`
		Text       string          `json:"text"`
		IsVoice    bool            `json:"is_voice"`
		ImagePath  string          `json:"image_path"`
		ImageLabel string          `json:"image_label"`
		OcrText    string          `json:"ocr_text"`
		Draft      json.RawMessage `json:"draft"`
		Saved      bool            `json:"saved"`
		SavedTxID  string          `json:"saved_tx_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		sendError(w, http.StatusBadRequest, "id required")
		return
	}
	if req.Role != "user" && req.Role != "assistant" {
		sendError(w, http.StatusBadRequest, "role must be user or assistant")
		return
	}
	if req.ImagePath != "" {
		name, ok := imageFileName(req.ImagePath)
		if !ok {
			sendError(w, http.StatusBadRequest, "invalid image_path")
			return
		}
		// 图片必须是之前经上传接口存下的文件，防止引用任意路径。
		if _, err := os.Stat(filepath.Join(h.aiImagesDir(), name)); err != nil {
			sendError(w, http.StatusBadRequest, "image not found")
			return
		}
		req.ImagePath = name
	}

	msg := &model.AiChatMessage{
		ID:         req.ID,
		BookID:     bookID,
		UserID:     uCtx.ID,
		Role:       req.Role,
		Text:       req.Text,
		IsVoice:    req.IsVoice,
		ImagePath:  req.ImagePath,
		ImageLabel: req.ImageLabel,
		OcrText:    req.OcrText,
		Draft:      req.Draft,
		Saved:      req.Saved,
		SavedTxID:  req.SavedTxID,
	}
	if err := h.repo.CreateMessage(r.Context(), msg); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			sendError(w, http.StatusConflict, "message id already exists")
			return
		}
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 超出上限时删最旧的，连同图片文件一起删。
	if pruned, err := h.repo.PruneMessages(r.Context(), bookID, uCtx.ID, repository.MaxAiChatMessagesPerBook); err == nil {
		for _, name := range pruned {
			_ = os.Remove(filepath.Join(h.aiImagesDir(), name))
		}
	}
	sendJSON(w, 0, http.StatusCreated, "success", map[string]interface{}{
		"message": msg,
	})
}

func (h *AiChatHandler) UpdateMessage(w http.ResponseWriter, r *http.Request, bookID, msgID string) {
	uCtx := GetUserContext(r)
	if !h.requireBookMember(w, r, bookID) {
		return
	}
	existing, err := h.repo.GetMessage(r.Context(), msgID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil || existing.BookID != bookID || existing.UserID != uCtx.ID {
		sendError(w, http.StatusNotFound, "message not found")
		return
	}

	var req struct {
		Text       *string         `json:"text"`
		IsVoice    *bool           `json:"is_voice"`
		ImagePath  *string         `json:"image_path"`
		ImageLabel *string         `json:"image_label"`
		OcrText    *string         `json:"ocr_text"`
		Draft      json.RawMessage `json:"draft"`
		Saved      *bool           `json:"saved"`
		SavedTxID  *string         `json:"saved_tx_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Text != nil {
		existing.Text = *req.Text
	}
	if req.IsVoice != nil {
		existing.IsVoice = *req.IsVoice
	}
	if req.ImagePath != nil {
		if *req.ImagePath != "" {
			name, ok := imageFileName(*req.ImagePath)
			if !ok {
				sendError(w, http.StatusBadRequest, "invalid image_path")
				return
			}
			if _, err := os.Stat(filepath.Join(h.aiImagesDir(), name)); err != nil {
				sendError(w, http.StatusBadRequest, "image not found")
				return
			}
			existing.ImagePath = name
		} else {
			existing.ImagePath = ""
		}
	}
	if req.ImageLabel != nil {
		existing.ImageLabel = *req.ImageLabel
	}
	if req.OcrText != nil {
		existing.OcrText = *req.OcrText
	}
	if len(req.Draft) > 0 {
		existing.Draft = req.Draft
	}
	if req.Saved != nil {
		existing.Saved = *req.Saved
	}
	if req.SavedTxID != nil {
		existing.SavedTxID = *req.SavedTxID
	}

	if err := h.repo.UpdateMessage(r.Context(), existing); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"message": existing,
	})
}

func (h *AiChatHandler) ClearMessages(w http.ResponseWriter, r *http.Request, bookID string) {
	uCtx := GetUserContext(r)
	if !h.requireBookMember(w, r, bookID) {
		return
	}
	deleted, imagePaths, err := h.repo.DeleteMessagesForUser(r.Context(), bookID, uCtx.ID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 记录清掉的同时删图片文件，不留孤儿文件占磁盘。
	for _, name := range imagePaths {
		if clean, ok := imageFileName(name); ok {
			_ = os.Remove(filepath.Join(h.aiImagesDir(), clean))
		}
	}
	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"deleted": deleted,
	})
}

func (h *AiChatHandler) UploadImage(w http.ResponseWriter, r *http.Request, bookID string) {
	if !h.requireBookMember(w, r, bookID) {
		return
	}
	const maxUploadSize = 10 * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		sendError(w, http.StatusRequestEntityTooLarge, "file too large")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		sendError(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()

	fname := header.Filename
	if fname == "" {
		fname = "receipt.jpg"
	}
	if _, ok := imageFileName(fname); !ok {
		sendError(w, http.StatusUnsupportedMediaType, "unsupported media type")
		return
	}

	stored := "aim_" + uuid.NewString()[:8] + "_" + filepath.Base(fname)
	destPath := filepath.Join(h.aiImagesDir(), stored)
	out, err := os.Create(destPath)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "failed to save image: "+err.Error())
		return
	}
	defer out.Close()
	size, err := io.Copy(out, file)
	if err != nil {
		_ = os.Remove(destPath)
		sendError(w, http.StatusInternalServerError, "failed to write image: "+err.Error())
		return
	}
	if size == 0 {
		_ = os.Remove(destPath)
		sendError(w, http.StatusBadRequest, "empty file")
		return
	}

	sendJSON(w, 0, http.StatusCreated, "success", map[string]interface{}{
		"image_path": stored,
	})
}

func (h *AiChatHandler) ServeImage(w http.ResponseWriter, r *http.Request, bookID, name string) {
	if !h.requireBookMember(w, r, bookID) {
		return
	}
	clean, ok := imageFileName(name)
	if !ok {
		sendError(w, http.StatusBadRequest, "invalid image name")
		return
	}
	// 清理后必须仍在 ai-images 目录内，防止路径穿越。
	absDir, err := filepath.Abs(h.aiImagesDir())
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	absPath, err := filepath.Abs(filepath.Join(absDir, clean))
	if err != nil || absPath != filepath.Join(absDir, clean) {
		sendError(w, http.StatusBadRequest, "invalid image name")
		return
	}
	f, err := os.Open(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			sendError(w, http.StatusNotFound, "image not found")
			return
		}
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	contentType := "image/jpeg"
	switch strings.ToLower(filepath.Ext(clean)) {
	case ".png":
		contentType = "image/png"
	case ".webp":
		contentType = "image/webp"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeContent(w, r, clean, info.ModTime(), f)
}
