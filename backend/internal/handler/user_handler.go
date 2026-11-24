package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
	"github.com/google/uuid"
)

type UserHandler struct {
	bookRepo     *repository.BookRepo
	walletRepo   *repository.WalletRepo
	txRepo       *repository.TransactionRepo
	userRepo     *repository.UserRepo
	settingsRepo *repository.SettingsRepo
	dataDir      string
}

func NewUserHandler(
	bookRepo *repository.BookRepo,
	walletRepo *repository.WalletRepo,
	txRepo *repository.TransactionRepo,
	userRepo *repository.UserRepo,
	settingsRepo *repository.SettingsRepo,
	dataDir string,
) *UserHandler {
	return &UserHandler{
		bookRepo:     bookRepo,
		walletRepo:   walletRepo,
		txRepo:       txRepo,
		userRepo:     userRepo,
		settingsRepo: settingsRepo,
		dataDir:      dataDir,
	}
}

func (h *UserHandler) getOrEnsureUser(r *http.Request) *model.User {
	uCtx := GetUserContext(r)
	if h.userRepo != nil {
		if u, err := h.userRepo.UpsertUserFromContext(r.Context(), uCtx); err == nil && u != nil {
			return u
		}
	}
	role := "user"
	if uCtx.IsAdmin() {
		role = "admin"
	}
	return &model.User{
		ID:           uCtx.ID,
		Name:         uCtx.Name,
		Email:        uCtx.Email,
		Avatar:       "/avatars/cat-1.jpg",
		Role:         role,
		DailyAiLimit: 30,
		IsActive:     true,
	}
}

func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	u := h.getOrEnsureUser(r)
	profile := buildUserProfile(r.Context(), u, h.bookRepo, h.walletRepo, h.userRepo, h.settingsRepo)
	sendJSON(w, 0, http.StatusOK, "success", profile)
}

func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		Name   *string `json:"name"`
		Avatar *string `json:"avatar"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.Avatar != nil && strings.HasPrefix(strings.TrimSpace(*req.Avatar), "data:image/") {
		raw := strings.TrimSpace(*req.Avatar)
		parts := strings.SplitN(raw, ",", 2)
		if len(parts) != 2 {
			sendError(w, http.StatusBadRequest, "invalid base64 image data")
			return
		}
		meta := strings.ToLower(parts[0])
		ext := ".jpg"
		if strings.Contains(meta, "image/png") {
			ext = ".png"
		} else if strings.Contains(meta, "image/webp") {
			ext = ".webp"
		}
		decoded, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			sendError(w, http.StatusBadRequest, "failed to decode base64 avatar")
			return
		}
		if len(decoded) > 5*1024*1024 {
			sendError(w, http.StatusRequestEntityTooLarge, "avatar image too large")
			return
		}
		avatarDir := filepath.Join(h.dataDir, "avatars")
		if err := os.MkdirAll(avatarDir, 0755); err != nil {
			sendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		filename := "av_" + uuid.NewString()[:12] + ext
		destPath := filepath.Join(avatarDir, filename)
		if err := os.WriteFile(destPath, decoded, 0644); err != nil {
			sendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		savedURL := "/api/v1/avatars/" + filename
		req.Avatar = &savedURL
	}

	if err := h.userRepo.UpdateUserProfile(r.Context(), uCtx.ID, req.Name, req.Avatar); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	u := h.getOrEnsureUser(r)
	profile := buildUserProfile(r.Context(), u, h.bookRepo, h.walletRepo, h.userRepo, h.settingsRepo)
	sendJSON(w, 0, http.StatusOK, "success", profile)
}

func (h *UserHandler) BindEmail(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || !strings.Contains(email, "@") {
		sendError(w, http.StatusBadRequest, "valid email required")
		return
	}

	ok, err := h.userRepo.VerifyAndConsumeEmailCode(r.Context(), email, req.Code, "bind")
	if err != nil || !ok {
		sendError(w, http.StatusBadRequest, "invalid or expired verification code")
		return
	}

	if existing, _ := h.userRepo.GetUserByEmail(r.Context(), email); existing != nil && existing.ID != uCtx.ID {
		sendError(w, http.StatusConflict, "email already bound to another user")
		return
	}

	if err := h.userRepo.BindUserEmail(r.Context(), uCtx.ID, email); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	u := h.getOrEnsureUser(r)
	profile := buildUserProfile(r.Context(), u, h.bookRepo, h.walletRepo, h.userRepo, h.settingsRepo)
	sendJSON(w, 0, http.StatusOK, "success", profile)
}

func (h *UserHandler) ServeAvatar(w http.ResponseWriter, r *http.Request, filename string) {
	if filename == "" || strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		sendError(w, http.StatusBadRequest, "invalid avatar filename")
		return
	}
	cleanName := filepath.Base(filename)
	avatarDir := filepath.Join(h.dataDir, "avatars")
	target := filepath.Join(avatarDir, cleanName)
	data, err := os.ReadFile(target)
	if err != nil {
		sendError(w, http.StatusNotFound, "avatar not found")
		return
	}

	ct := "image/jpeg"
	lower := strings.ToLower(cleanName)
	if strings.HasSuffix(lower, ".png") {
		ct = "image/png"
	} else if strings.HasSuffix(lower, ".webp") {
		ct = "image/webp"
	}
	sendBytes(w, data, ct, map[string]string{
		"Cache-Control": "public, max-age=86400",
	}, http.StatusOK)
}
