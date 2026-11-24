package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
	"com.xsguyu.accounting/backend/internal/service"
)

type AuthHandler struct {
	userRepo     *repository.UserRepo
	bookRepo     *repository.BookRepo
	walletRepo   *repository.WalletRepo
	settingsRepo *repository.SettingsRepo
	authSvc      *service.AuthService
}

func NewAuthHandler(
	userRepo *repository.UserRepo,
	bookRepo *repository.BookRepo,
	walletRepo *repository.WalletRepo,
	settingsRepo *repository.SettingsRepo,
	authSvc *service.AuthService,
) *AuthHandler {
	return &AuthHandler{
		userRepo:     userRepo,
		bookRepo:     bookRepo,
		walletRepo:   walletRepo,
		settingsRepo: settingsRepo,
		authSvc:      authSvc,
	}
}

func buildUserProfile(ctx context.Context, u *model.User, bookRepo *repository.BookRepo, walletRepo *repository.WalletRepo, userRepo *repository.UserRepo, settingsRepo *repository.SettingsRepo) model.UserProfile {
	books, _ := bookRepo.ListBooks(ctx, u.ID)
	var createdCount, joinedCount int
	var bookIDs []string
	defaultBookID := ""
	for _, b := range books {
		bookIDs = append(bookIDs, b.ID)
		if u.DefaultBookID != "" && b.ID == u.DefaultBookID {
			defaultBookID = b.ID
		}
		if b.OwnerID == u.ID || b.Role == "owner" {
			createdCount++
		} else {
			joinedCount++
		}
	}

	var totalWallets int
	for _, bid := range bookIDs {
		ws, _ := walletRepo.ListWallets(ctx, bid, false)
		totalWallets += len(ws)
	}
	if len(bookIDs) == 0 {
		ws, _ := walletRepo.ListWallets(ctx, "", false)
		totalWallets = len(ws)
	}

	streakDays, recordedDays, totalTx, healthScore, healthLabel, savingsRate := userRepo.ComputeUserProfileMetrics(ctx, u.ID, bookIDs)
	todayAi := settingsRepo.GetTodayAIUsageCount(ctx, u.ID)

	uType := "users"
	if u.Role == "admin" {
		uType = "admin"
	}

	return model.UserProfile{
		UserID:                 u.ID,
		Username:               u.Username,
		UserName:               u.Name,
		UserType:               uType,
		Email:                  u.Email,
		Avatar:                 u.Avatar,
		Role:                   u.Role,
		EmailBound:             u.EmailBound,
		GatewayBound:           strings.TrimSpace(u.GatewayUID) != "",
		DefaultBookID:          defaultBookID,
		DailyAiLimit:           u.DailyAiLimit,
		TodayAiCount:           todayAi,
		StreakDays:             streakDays,
		RecordedDays:           recordedDays,
		TotalTransactionsCount: totalTx,
		CreatedBooksCount:      createdCount,
		JoinedBooksCount:       joinedCount,
		TotalBooksCount:        len(books),
		TotalWalletsCount:      totalWallets,
		HealthScore:            healthScore,
		HealthLabel:            healthLabel,
		SavingsRate:            savingsRate,
	}
}

func sanitizeSuggestedUsername(rawName, fallbackID string) string {
	cand := strings.TrimSpace(rawName)
	if cand == "" {
		cand = strings.TrimSpace(fallbackID)
	}
	cand = strings.ReplaceAll(cand, "@", "_")
	cand = strings.ReplaceAll(cand, " ", "_")
	if len([]rune(cand)) < 2 {
		cand = "nas_" + cand
	}
	if len([]rune(cand)) > 24 {
		runes := []rune(cand)
		cand = string(runes[:24])
	}
	return cand
}

func (h *AuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	regCfg, _ := h.settingsRepo.GetRegConfig(r.Context())
	totalUsers, _ := h.userRepo.CountUsers(r.Context())
	registeredUsers, _ := h.userRepo.CountRegisteredUsers(r.Context())
	gwCtx := ExtractGatewayContext(r)

	allowRegister := true
	requireEmailVerify := false
	requireInviteCode := false
	if regCfg != nil {
		allowRegister = regCfg.AllowRegister
		requireEmailVerify = regCfg.RequireEmailVerify
		requireInviteCode = regCfg.RequireInviteCode
	}

	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"first_install":           registeredUsers == 0,
		"has_registered_users":    registeredUsers > 0,
		"total_users":             totalUsers,
		"gateway_header_detected": gwCtx != nil,
		"allow_register":          allowRegister,
		"require_email_verify":    requireEmailVerify,
		"require_invite_code":     requireInviteCode,
	})
}

func (h *AuthHandler) GatewayLogin(w http.ResponseWriter, r *http.Request) {
	gwCtx := ExtractGatewayContext(r)
	if gwCtx == nil || strings.TrimSpace(gwCtx.ID) == "" {
		sendError(w, http.StatusUnauthorized, "未检测到绿联云网关登录信息，请使用账号密码登录")
		return
	}

	existing, err := h.userRepo.GetUserByGatewayUID(r.Context(), gwCtx.ID)
	if err == nil && existing != nil {
		if !existing.IsActive {
			sendError(w, http.StatusForbidden, "account disabled")
			return
		}
		if existing.EmailBound && strings.TrimSpace(existing.PasswordHash) != "" {
			h.userRepo.TouchLastActive(r.Context(), existing.ID)
			token, err := h.authSvc.IssueToken(existing.ID, existing.Role, service.DefaultAccessTokenTTL)
			if err != nil {
				sendError(w, http.StatusInternalServerError, err.Error())
				return
			}
			refreshToken, err := h.authSvc.IssueRefreshToken(r.Context(), existing.ID, service.DefaultRefreshTokenTTL)
			if err != nil {
				sendError(w, http.StatusInternalServerError, err.Error())
				return
			}
			profile := buildUserProfile(r.Context(), existing, h.bookRepo, h.walletRepo, h.userRepo, h.settingsRepo)
			sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
				"status":             "authenticated",
				"token":              token,
				"refresh_token":      refreshToken,
				"expires_in":         int64(service.DefaultAccessTokenTTL.Seconds()),
				"refresh_expires_in": int64(service.DefaultRefreshTokenTTL.Seconds()),
				"user":               profile,
			})
			return
		}
	}

	existingUID := ""
	if existing != nil {
		existingUID = existing.ID
	}
	suggestedUsername := sanitizeSuggestedUsername(gwCtx.Name, gwCtx.ID)
	if taken, uErr := h.userRepo.GetUserByUsername(r.Context(), suggestedUsername); uErr == nil && taken != nil && taken.ID != existingUID {
		suggestedUsername = suggestedUsername + "_" + service.GenerateCode6()[:4]
	}

	ticket, err := h.authSvc.IssueGatewayBindTicket(gwCtx.ID, gwCtx.Name, gwCtx.Type, existingUID, 10*time.Minute)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"status":             "need_register",
		"bind_ticket":        ticket,
		"gateway_uid":        gwCtx.ID,
		"suggested_username": suggestedUsername,
		"suggested_name":     gwCtx.Name,
	})
}

func (h *AuthHandler) SendCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email   string `json:"email"`
		Purpose string `json:"purpose"`
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
	purpose := strings.TrimSpace(req.Purpose)
	if purpose == "" {
		purpose = "login"
	}

	code := service.GenerateCode6()
	if err := h.userRepo.CreateEmailCode(r.Context(), email, code, purpose, 10*time.Minute); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	smtpSent, smtpErr := h.authSvc.SendVerificationCodeEmail(r.Context(), email, purpose, code)
	if smtpErr != nil {
		smtpSent = false
	}

	demoCode := ""
	if !smtpSent {
		demoCode = code
	}
	resp := map[string]interface{}{
		"sent":       true,
		"expires_in": 600,
		"demo_code":  demoCode,
		"smtp_sent":  smtpSent,
	}
	sendJSON(w, 0, http.StatusOK, "success", resp)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username   string `json:"username"`
		Name       string `json:"name"`
		Email      string `json:"email"`
		Code       string `json:"code"`
		Password   string `json:"password"`
		InviteCode string `json:"invite_code"`
		Avatar     string `json:"avatar"`
		BindTicket string `json:"bind_ticket"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	var bindClaims *service.GatewayBindClaims
	if strings.TrimSpace(req.BindTicket) != "" {
		claims, err := h.authSvc.VerifyGatewayBindTicket(req.BindTicket)
		if err != nil {
			sendError(w, http.StatusBadRequest, "invalid or expired gateway authorization ticket")
			return
		}
		if boundUser, gErr := h.userRepo.GetUserByGatewayUID(r.Context(), claims.GatewayUID); gErr == nil && boundUser != nil {
			if boundUser.EmailBound && strings.TrimSpace(boundUser.PasswordHash) != "" {
				sendError(w, http.StatusConflict, "gateway account already registered")
				return
			}
			if claims.ExistingUID == "" {
				claims.ExistingUID = boundUser.ID
			}
		}
		bindClaims = claims
	}

	regCfg, err := h.settingsRepo.GetRegConfig(r.Context())
	registeredCnt, _ := h.userRepo.CountRegisteredUsers(r.Context())
	if err == nil && !regCfg.AllowRegister {
		// Allow initial setup or NAS admin gateway onboarding even when public signup is closed
		if bindClaims == nil || (registeredCnt > 0 && bindClaims.GatewayType != "admin") {
			sendError(w, http.StatusForbidden, "public registration is disabled")
			return
		}
	}

	name := strings.TrimSpace(req.Name)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if name == "" || email == "" || !strings.Contains(email, "@") {
		sendError(w, http.StatusBadRequest, "name and valid email are required")
		return
	}
	if len(req.Password) < 6 {
		sendError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	username := strings.TrimSpace(req.Username)
	if username != "" {
		if strings.Contains(username, "@") {
			sendError(w, http.StatusBadRequest, "username cannot contain @")
			return
		}
		if len([]rune(username)) < 2 || len([]rune(username)) > 32 {
			sendError(w, http.StatusBadRequest, "username must be between 2 and 32 characters")
			return
		}
		if existingByUname, uErr := h.userRepo.GetUserByUsername(r.Context(), username); uErr == nil && existingByUname != nil {
			if bindClaims == nil || existingByUname.ID != bindClaims.ExistingUID {
				sendError(w, http.StatusConflict, "username already taken")
				return
			}
		}
	} else {
		base := sanitizeSuggestedUsername(strings.SplitN(email, "@", 2)[0], "user")
		username = base
		if taken, uErr := h.userRepo.GetUserByUsername(r.Context(), username); uErr == nil && taken != nil {
			username = base + "_" + service.GenerateCode6()[:4]
		}
	}

	if bindClaims == nil && regCfg != nil && regCfg.RequireInviteCode {
		if strings.TrimSpace(req.InviteCode) != strings.TrimSpace(regCfg.InviteCode) {
			sendError(w, http.StatusForbidden, "invalid registration invite code")
			return
		}
	}

	if regCfg != nil && regCfg.RequireEmailVerify {
		ok, err := h.userRepo.VerifyAndConsumeEmailCode(r.Context(), email, req.Code, "register")
		if err != nil || !ok {
			sendError(w, http.StatusBadRequest, "invalid or expired verification code")
			return
		}
	}

	if existing, _ := h.userRepo.GetUserByEmail(r.Context(), email); existing != nil {
		if bindClaims == nil || existing.ID != bindClaims.ExistingUID {
			sendError(w, http.StatusConflict, "email already registered")
			return
		}
	}

	hash, err := h.authSvc.HashPassword(req.Password)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	avatar := strings.TrimSpace(req.Avatar)
	if avatar == "" {
		avatar = service.RandomCatAvatar()
	}

	var userID, userRole string
	if bindClaims != nil && bindClaims.ExistingUID != "" {
		if err := h.userRepo.CompleteGatewayUserRegistration(
			r.Context(), bindClaims.ExistingUID, bindClaims.GatewayUID, username, name, email, hash, avatar,
		); err != nil {
			sendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = h.bookRepo.EnsureDefaultBookForUser(r.Context(), bindClaims.ExistingUID, name)
		userID = bindClaims.ExistingUID
	} else {
		role := "user"
		if cnt, err := h.userRepo.CountUsers(r.Context()); err == nil && cnt == 0 {
			role = "admin"
		} else if registeredCnt == 0 {
			role = "admin"
		} else if bindClaims != nil && bindClaims.GatewayType == "admin" {
			role = "admin"
		}

		dailyLimit := 30
		if regCfg != nil && regCfg.DefaultDailyAiLimit > 0 {
			dailyLimit = regCfg.DefaultDailyAiLimit
		}

		gatewayUID := ""
		if bindClaims != nil {
			gatewayUID = bindClaims.GatewayUID
		}

		user := &model.User{
			Username:     username,
			Name:         name,
			Email:        email,
			PasswordHash: hash,
			Avatar:       avatar,
			Role:         role,
			EmailBound:   true,
			GatewayUID:   gatewayUID,
			DailyAiLimit: dailyLimit,
			IsActive:     true,
		}
		if err := h.userRepo.CreateUser(r.Context(), user); err != nil {
			sendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = h.bookRepo.EnsureDefaultBookForUser(r.Context(), user.ID, user.Name)
		userID = user.ID
		userRole = user.Role
	}

	createdUser, err := h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil || createdUser == nil {
		sendError(w, http.StatusInternalServerError, "failed to load registered user")
		return
	}
	userRole = createdUser.Role

	token, err := h.authSvc.IssueToken(createdUser.ID, userRole, service.DefaultAccessTokenTTL)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	refreshToken, err := h.authSvc.IssueRefreshToken(r.Context(), createdUser.ID, service.DefaultRefreshTokenTTL)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	profile := buildUserProfile(r.Context(), createdUser, h.bookRepo, h.walletRepo, h.userRepo, h.settingsRepo)

	sendJSON(w, 0, http.StatusCreated, "success", map[string]interface{}{
		"token":              token,
		"refresh_token":      refreshToken,
		"expires_in":         int64(service.DefaultAccessTokenTTL.Seconds()),
		"refresh_expires_in": int64(service.DefaultRefreshTokenTTL.Seconds()),
		"user":               profile,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Account  string `json:"account"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Code     string `json:"code"`
		Mode     string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	identifier := getFirstNonEmpty(req.Account, req.Email, req.Username)
	if identifier == "" {
		sendError(w, http.StatusBadRequest, "username or email required")
		return
	}

	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		if req.Code != "" && req.Password == "" {
			mode = "code"
		} else {
			mode = "password"
		}
	}

	user, err := h.userRepo.GetUserByLoginIdentifier(r.Context(), identifier)
	if err != nil || user == nil {
		sendError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if !user.IsActive {
		sendError(w, http.StatusForbidden, "account disabled")
		return
	}

	if mode == "code" {
		if !user.EmailBound || strings.TrimSpace(user.Email) == "" {
			sendError(w, http.StatusBadRequest, "no email bound to this account")
			return
		}
		ok, err := h.userRepo.VerifyAndConsumeEmailCode(r.Context(), user.Email, req.Code, "login")
		if err != nil || !ok {
			sendError(w, http.StatusUnauthorized, "invalid or expired verification code")
			return
		}
	} else {
		if strings.TrimSpace(user.PasswordHash) == "" {
			sendError(w, http.StatusUnauthorized, "该账号尚未设置登录密码，请使用绿联云授权登录完成账号完善")
			return
		}
		if !h.authSvc.VerifyPassword(req.Password, user.PasswordHash) {
			sendError(w, http.StatusUnauthorized, "invalid username/email or password")
			return
		}
	}

	h.userRepo.TouchLastActive(r.Context(), user.ID)
	token, err := h.authSvc.IssueToken(user.ID, user.Role, service.DefaultAccessTokenTTL)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	refreshToken, err := h.authSvc.IssueRefreshToken(r.Context(), user.ID, service.DefaultRefreshTokenTTL)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	profile := buildUserProfile(r.Context(), user, h.bookRepo, h.walletRepo, h.userRepo, h.settingsRepo)
	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"token":              token,
		"refresh_token":      refreshToken,
		"expires_in":         int64(service.DefaultAccessTokenTTL.Seconds()),
		"refresh_expires_in": int64(service.DefaultRefreshTokenTTL.Seconds()),
		"user":               profile,
	})
}

// RefreshToken 使用长期 Refresh Token（180 天，滑动续期）无感换取新的 Access Token；
// 同时兼容携带尚未过期的旧 cat7_ Access Token 补领 180 天 Refresh Token，确保老版本升级无缝衔接。
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	rawRefresh := strings.TrimSpace(req.RefreshToken)
	if rawRefresh != "" {
		user, newAccess, sameRefresh, err := h.authSvc.RefreshWithToken(
			r.Context(),
			rawRefresh,
			service.DefaultAccessTokenTTL,
			service.DefaultRefreshTokenTTL,
		)
		if err != nil {
			if strings.Contains(err.Error(), "disabled") {
				sendError(w, http.StatusForbidden, "account disabled")
				return
			}
			sendError(w, http.StatusUnauthorized, "invalid or expired refresh token")
			return
		}
		h.userRepo.TouchLastActive(r.Context(), user.ID)
		profile := buildUserProfile(r.Context(), user, h.bookRepo, h.walletRepo, h.userRepo, h.settingsRepo)
		sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
			"token":              newAccess,
			"refresh_token":      sameRefresh,
			"expires_in":         int64(service.DefaultAccessTokenTTL.Seconds()),
			"refresh_expires_in": int64(service.DefaultRefreshTokenTTL.Seconds()),
			"user":               profile,
		})
		return
	}

	// 兼容老版本升级：如果客户端还没有 refresh_token，但带着未过期的 cat7_ token，为其补发 180 天 refresh_token
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authHeader, "Bearer ") || strings.HasPrefix(authHeader, "cat7_") {
		claims, err := h.authSvc.VerifyToken(authHeader)
		if err == nil && claims != nil {
			user, uErr := h.userRepo.GetUserByID(r.Context(), claims.UserID)
			if uErr == nil && user != nil {
				if !user.IsActive {
					sendError(w, http.StatusForbidden, "account disabled")
					return
				}
				newAccess, err := h.authSvc.IssueToken(user.ID, user.Role, service.DefaultAccessTokenTTL)
				if err != nil {
					sendError(w, http.StatusInternalServerError, err.Error())
					return
				}
				newRefresh, err := h.authSvc.IssueRefreshToken(r.Context(), user.ID, service.DefaultRefreshTokenTTL)
				if err != nil {
					sendError(w, http.StatusInternalServerError, err.Error())
					return
				}
				h.userRepo.TouchLastActive(r.Context(), user.ID)
				profile := buildUserProfile(r.Context(), user, h.bookRepo, h.walletRepo, h.userRepo, h.settingsRepo)
				sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
					"token":              newAccess,
					"refresh_token":      newRefresh,
					"expires_in":         int64(service.DefaultAccessTokenTTL.Seconds()),
					"refresh_expires_in": int64(service.DefaultRefreshTokenTTL.Seconds()),
					"user":               profile,
				})
				return
			}
		}
	}

	sendError(w, http.StatusUnauthorized, "refresh_token required")
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if strings.TrimSpace(req.RefreshToken) != "" {
		_ = h.authSvc.RevokeRefreshToken(r.Context(), req.RefreshToken)
	}
	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"logged_out": true,
	})
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		Code        string `json:"code"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || len(req.NewPassword) < 6 {
		sendError(w, http.StatusBadRequest, "valid email and new_password (>=6 chars) required")
		return
	}

	user, err := h.userRepo.GetUserByEmail(r.Context(), email)
	if err != nil || user == nil {
		sendError(w, http.StatusNotFound, "user not found")
		return
	}

	ok, err := h.userRepo.VerifyAndConsumeEmailCode(r.Context(), email, req.Code, "reset")
	if err == nil && !ok {
		ok, err = h.userRepo.VerifyAndConsumeEmailCode(r.Context(), email, req.Code, "forgot")
	}
	if err != nil || !ok {
		sendError(w, http.StatusBadRequest, "invalid or expired verification code")
		return
	}

	hash, err := h.authSvc.HashPassword(req.NewPassword)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.userRepo.UpdateUserPassword(r.Context(), user.ID, hash); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = h.authSvc.RevokeAllUserRefreshTokens(r.Context(), user.ID)

	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"reset": true,
	})
}
