package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
	"com.xsguyu.accounting/backend/internal/service"
)

type AdminHandler struct {
	settingsRepo *repository.SettingsRepo
	userRepo     *repository.UserRepo
	authSvc      *service.AuthService
}

func NewAdminHandler(settingsRepo *repository.SettingsRepo, userRepo *repository.UserRepo, authSvc *service.AuthService) *AdminHandler {
	return &AdminHandler{
		settingsRepo: settingsRepo,
		userRepo:     userRepo,
		authSvc:      authSvc,
	}
}

func (h *AdminHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	uCtx := GetUserContext(r)
	if !uCtx.IsAdmin() {
		sendError(w, http.StatusForbidden, "admin permission required")
		return false
	}
	return true
}

func (h *AdminHandler) Overview(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	ov, err := h.settingsRepo.GetAdminOverview(r.Context())
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", ov)
}

func (h *AdminHandler) GetAiConfig(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	cfg, err := h.settingsRepo.GetAiConfig(r.Context())
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", cfg)
}

func (h *AdminHandler) UpdateAiConfig(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	cur, err := h.settingsRepo.GetAiConfig(r.Context())
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var req struct {
		Enabled         *bool    `json:"enabled"`
		Provider        *string  `json:"provider"`
		BaseURL         *string  `json:"base_url"`
		APIKey          *string  `json:"api_key"`
		Model           *string  `json:"model"`
		DailyLimit      *int     `json:"daily_limit"`
		Temperature     *float64 `json:"temperature"`
		OcrEnabled      *bool    `json:"ocr_enabled"`
		InsightsEnabled *bool    `json:"insights_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.Enabled != nil {
		cur.Enabled = *req.Enabled
	}
	if req.Provider != nil {
		cur.Provider = *req.Provider
	}
	if req.BaseURL != nil {
		cur.BaseURL = strings.TrimSpace(*req.BaseURL)
	}
	if req.APIKey != nil {
		cur.APIKey = strings.TrimSpace(*req.APIKey)
	}
	if req.Model != nil {
		cur.Model = strings.TrimSpace(*req.Model)
	}
	if req.DailyLimit != nil && *req.DailyLimit > 0 {
		cur.DailyLimit = *req.DailyLimit
	}
	if req.Temperature != nil {
		cur.Temperature = *req.Temperature
	}
	if req.OcrEnabled != nil {
		cur.OcrEnabled = *req.OcrEnabled
	}
	if req.InsightsEnabled != nil {
		cur.InsightsEnabled = *req.InsightsEnabled
	}

	if err := h.settingsRepo.SaveAiConfig(r.Context(), cur); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", cur)
}

func (h *AdminHandler) AIUsage(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	search := r.URL.Query().Get("search")
	totalCalls, todayCalls, totalIn, totalOut, users, logs, err := h.settingsRepo.GetAdminAIUsage(r.Context(), search)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"total_calls":         totalCalls,
		"today_calls":         todayCalls,
		"total_input_tokens":  totalIn,
		"total_output_tokens": totalOut,
		"users":               users,
		"recent_logs":         logs,
	})
}

func (h *AdminHandler) GetRegConfig(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	cfg, err := h.settingsRepo.GetRegConfig(r.Context())
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", cfg)
}

func (h *AdminHandler) UpdateRegConfig(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	cur, err := h.settingsRepo.GetRegConfig(r.Context())
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var req struct {
		AllowRegister       *bool   `json:"allow_register"`
		AllowPublicRegister *bool   `json:"allow_public_register"`
		RequireInviteCode   *bool   `json:"require_invite_code"`
		RequireEmailVerify  *bool   `json:"require_email_verify"`
		InviteCode          *string `json:"invite_code"`
		DefaultDailyAiLimit *int    `json:"default_daily_ai_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.AllowRegister != nil {
		cur.AllowRegister = *req.AllowRegister
	} else if req.AllowPublicRegister != nil {
		cur.AllowRegister = *req.AllowPublicRegister
	}
	if req.RequireInviteCode != nil {
		cur.RequireInviteCode = *req.RequireInviteCode
	}
	if req.RequireEmailVerify != nil {
		cur.RequireEmailVerify = *req.RequireEmailVerify
	}
	if req.InviteCode != nil {
		cur.InviteCode = strings.TrimSpace(*req.InviteCode)
	}
	if req.DefaultDailyAiLimit != nil && *req.DefaultDailyAiLimit > 0 {
		cur.DefaultDailyAiLimit = *req.DefaultDailyAiLimit
	}

	if err := h.settingsRepo.SaveRegConfig(r.Context(), cur); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", cur)
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	search := r.URL.Query().Get("search")
	users, err := h.settingsRepo.GetAdminUsers(r.Context(), search)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", users)
}

func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request, targetID string) {
	if !h.requireAdmin(w, r) {
		return
	}
	var req struct {
		Role         *string `json:"role"`
		DailyAiLimit *int    `json:"daily_ai_limit"`
		IsActive     *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	uCtx := GetUserContext(r)
	if targetID == uCtx.ID && req.IsActive != nil && !*req.IsActive {
		sendError(w, http.StatusBadRequest, "cannot disable self")
		return
	}

	if err := h.userRepo.AdminUpdateUser(r.Context(), targetID, req.Role, req.DailyAiLimit, req.IsActive); err != nil {
		if err == repository.ErrNotFound {
			sendError(w, http.StatusNotFound, "user not found")
			return
		}
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	users, _ := h.settingsRepo.GetAdminUsers(r.Context(), "")
	for _, u := range users {
		if u.ID == targetID {
			sendJSON(w, 0, http.StatusOK, "success", u)
			return
		}
	}
	sendJSON(w, 0, http.StatusOK, "success", nil)
}

func (h *AdminHandler) GetSystemConfig(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	cfg, err := h.settingsRepo.GetSystemConfig(r.Context())
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", cfg)
}

func parseFlexiblePort(raw interface{}) (int, bool) {
	switch v := raw.(type) {
	case float64:
		if int(v) > 0 {
			return int(v), true
		}
	case int:
		if v > 0 {
			return v, true
		}
	case string:
		if p, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && p > 0 {
			return p, true
		}
	}
	return 0, false
}

func (h *AdminHandler) UpdateSystemConfig(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	cur, err := h.settingsRepo.GetSystemConfig(r.Context())
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var req struct {
		AutoBackupEnabled *bool       `json:"auto_backup_enabled"`
		AutoBackup        *bool       `json:"auto_backup"`
		AutoBackupCycle   *string     `json:"auto_backup_cycle"`
		BackupCycle       *string     `json:"backup_cycle"`
		AutoBackupRetain  *int        `json:"auto_backup_retain"`
		KeepCount         *int        `json:"keep_count"`
		BackupPath        *string     `json:"backup_path"`
		SmtpHost          *string     `json:"smtp_host"`
		SmtpPort          interface{} `json:"smtp_port"`
		SmtpUser          *string     `json:"smtp_user"`
		SmtpPass          *string     `json:"smtp_pass"`
		SmtpFrom          *string     `json:"smtp_from"`
		SmtpTLS           *bool       `json:"smtp_tls"`
		SiteName          *string     `json:"site_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.AutoBackupEnabled != nil {
		cur.AutoBackupEnabled = *req.AutoBackupEnabled
	} else if req.AutoBackup != nil {
		cur.AutoBackupEnabled = *req.AutoBackup
	}
	if req.AutoBackupCycle != nil {
		cur.AutoBackupCycle = *req.AutoBackupCycle
	} else if req.BackupCycle != nil {
		cur.AutoBackupCycle = *req.BackupCycle
	}
	if req.AutoBackupRetain != nil && *req.AutoBackupRetain > 0 {
		cur.AutoBackupRetain = *req.AutoBackupRetain
	} else if req.KeepCount != nil && *req.KeepCount > 0 {
		cur.AutoBackupRetain = *req.KeepCount
	}
	if req.BackupPath != nil && strings.TrimSpace(*req.BackupPath) != "" {
		cur.BackupPath = strings.TrimSpace(*req.BackupPath)
	}
	if req.SmtpHost != nil {
		cur.SmtpHost = strings.TrimSpace(*req.SmtpHost)
	}
	if p, ok := parseFlexiblePort(req.SmtpPort); ok {
		cur.SmtpPort = p
	}
	if req.SmtpUser != nil {
		cur.SmtpUser = strings.TrimSpace(*req.SmtpUser)
	}
	if req.SmtpPass != nil {
		cur.SmtpPass = *req.SmtpPass
	}
	if req.SmtpFrom != nil {
		cur.SmtpFrom = strings.TrimSpace(*req.SmtpFrom)
	}
	if req.SmtpTLS != nil {
		cur.SmtpTLS = *req.SmtpTLS
	}
	if req.SiteName != nil && strings.TrimSpace(*req.SiteName) != "" {
		cur.SiteName = strings.TrimSpace(*req.SiteName)
	}

	if err := h.settingsRepo.SaveSystemConfig(r.Context(), cur); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", cur)
}

func (h *AdminHandler) TestSMTP(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	cur, _ := h.settingsRepo.GetSystemConfig(r.Context())
	if cur == nil {
		cur = &model.SystemConfig{}
	}

	var req struct {
		ToEmail  string      `json:"to_email"`
		Email    string      `json:"email"`
		SmtpHost string      `json:"smtp_host"`
		SmtpPort interface{} `json:"smtp_port"`
		SmtpUser string      `json:"smtp_user"`
		SmtpPass string      `json:"smtp_pass"`
		SmtpFrom string      `json:"smtp_from"`
		SmtpTLS  *bool       `json:"smtp_tls"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	testCfg := *cur
	if strings.TrimSpace(req.SmtpHost) != "" {
		testCfg.SmtpHost = strings.TrimSpace(req.SmtpHost)
	}
	if p, ok := parseFlexiblePort(req.SmtpPort); ok {
		testCfg.SmtpPort = p
	}
	if strings.TrimSpace(req.SmtpUser) != "" {
		testCfg.SmtpUser = strings.TrimSpace(req.SmtpUser)
	}
	if req.SmtpPass != "" {
		testCfg.SmtpPass = req.SmtpPass
	}
	if strings.TrimSpace(req.SmtpFrom) != "" {
		testCfg.SmtpFrom = strings.TrimSpace(req.SmtpFrom)
	}
	if req.SmtpTLS != nil {
		testCfg.SmtpTLS = *req.SmtpTLS
	}

	to := strings.TrimSpace(req.ToEmail)
	if to == "" {
		to = strings.TrimSpace(req.Email)
	}
	if to == "" {
		to = testCfg.SmtpUser
	}
	if strings.TrimSpace(testCfg.SmtpHost) == "" || strings.TrimSpace(testCfg.SmtpUser) == "" || to == "" {
		sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
			"ok":      false,
			"message": "请先填写 SMTP 服务器、发件人邮箱及收件地址",
		})
		return
	}

	port := testCfg.SmtpPort
	if port <= 0 {
		port = 465
	}
	fromAddr := strings.TrimSpace(testCfg.SmtpFrom)
	if fromAddr == "" {
		fromAddr = strings.TrimSpace(testCfg.SmtpUser)
	}
	testMail := service.RenderSMTPTestEmail(testCfg.SiteName, testCfg.SmtpHost, port, fromAddr, to)
	if err := service.SendSMTPMessage(&testCfg, to, testMail); err != nil {
		sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
			"ok":      false,
			"message": "SMTP 发送失败: " + err.Error(),
		})
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"ok":      true,
		"message": "测试邮件发送成功",
	})
}
