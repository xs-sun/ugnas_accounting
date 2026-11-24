package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
	"com.xsguyu.accounting/backend/internal/service"
)

type WalletHandler struct {
	walletRepo *repository.WalletRepo
	bookRepo   *repository.BookRepo
	hub        *service.Hub
	recorder   *service.ActivityRecorder
}

func NewWalletHandler(walletRepo *repository.WalletRepo, bookRepo *repository.BookRepo, hub *service.Hub, recorder *service.ActivityRecorder) *WalletHandler {
	return &WalletHandler{
		walletRepo: walletRepo,
		bookRepo:   bookRepo,
		hub:        hub,
		recorder:   recorder,
	}
}

var fourDigitsRegex = regexp.MustCompile(`^\d{4}$`)

func (h *WalletHandler) ListWallets(w http.ResponseWriter, r *http.Request) {
	bookID := r.URL.Query().Get("book_id")
	wallets, err := h.walletRepo.ListWallets(r.Context(), bookID, false)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if wallets == nil {
		wallets = []model.Wallet{}
	}
	sendJSON(w, 0, http.StatusOK, "success", wallets)
}

func (h *WalletHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	bookID := r.URL.Query().Get("book_id")
	summary, err := h.walletRepo.GetWalletSummary(r.Context(), bookID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", summary)
}

func (h *WalletHandler) CreateWallet(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		ID              string `json:"id"`
		BookID          string `json:"book_id"`
		Name            string `json:"name"`
		Type            string `json:"type"`
		GroupName       string `json:"group_name"`
		Color           string `json:"color"`
		Balance         int64  `json:"balance"`
		CreditLimit     int64  `json:"credit_limit"`
		BillingDay      int    `json:"billing_day"`
		RepaymentDay    int    `json:"repayment_day"`
		CardLastFour    string `json:"card_last_four"`
		ExcludeNetAsset bool   `json:"exclude_net_asset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		sendError(w, http.StatusBadRequest, "wallet name required")
		return
	}

	if req.BookID != "" {
		role, _ := h.bookRepo.GetMemberRole(r.Context(), req.BookID, uCtx.ID)
		if role == "viewer" {
			sendError(w, http.StatusForbidden, "viewers cannot create wallets")
			return
		}
	}

	if req.Type == "" {
		req.Type = "custom"
	}
	if req.BillingDay == 0 {
		req.BillingDay = 1
	}
	if req.RepaymentDay == 0 {
		req.RepaymentDay = 20
	}

	if req.BillingDay < 1 || req.BillingDay > 31 {
		sendError(w, http.StatusBadRequest, "billing_day must be between 1 and 31")
		return
	}
	if req.RepaymentDay < 1 || req.RepaymentDay > 31 {
		sendError(w, http.StatusBadRequest, "repayment_day must be between 1 and 31")
		return
	}
	if req.CardLastFour != "" && !fourDigitsRegex.MatchString(req.CardLastFour) {
		sendError(w, http.StatusBadRequest, "card_last_four must be exactly 4 digits")
		return
	}

	wallet := &model.Wallet{
		ID:              req.ID,
		BookID:          req.BookID,
		Name:            req.Name,
		Type:            req.Type,
		GroupName:       strings.TrimSpace(req.GroupName),
		Color:           strings.TrimSpace(req.Color),
		Balance:         req.Balance,
		CreditLimit:     req.CreditLimit,
		BillingDay:      req.BillingDay,
		RepaymentDay:    req.RepaymentDay,
		CardLastFour:    req.CardLastFour,
		ExcludeNetAsset: req.ExcludeNetAsset,
	}

	if err := h.walletRepo.CreateWallet(r.Context(), wallet); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
		BookID:    req.BookID,
		UserID:    uCtx.ID,
		ActorName: uCtx.Name,
		Action:    model.ActivityWalletChanged,
		Summary:   fmt.Sprintf("添加了钱包「%s」", wallet.Name),
	}, service.ChangeWallet)

	created, err := h.walletRepo.GetWallet(r.Context(), wallet.ID)
	if err != nil {
		sendJSON(w, 0, http.StatusCreated, "success", wallet)
		return
	}
	sendJSON(w, 0, http.StatusCreated, "success", created)
}

func (h *WalletHandler) UpdateWallet(w http.ResponseWriter, r *http.Request, walletID string) {
	uCtx := GetUserContext(r)
	wallet, err := h.walletRepo.GetWallet(r.Context(), walletID)
	if err != nil {
		sendError(w, http.StatusNotFound, "wallet not found")
		return
	}

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if v, ok := req["name"]; ok {
		if s, ok := v.(string); ok {
			wallet.Name = s
		}
	}
	if v, ok := req["type"]; ok {
		if s, ok := v.(string); ok {
			wallet.Type = s
		}
	}
	if v, ok := req["group_name"]; ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			wallet.GroupName = strings.TrimSpace(s)
		}
	}
	if v, ok := req["color"]; ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			wallet.Color = strings.TrimSpace(s)
		}
	}
	if v, ok := req["balance"]; ok {
		switch val := v.(type) {
		case float64:
			wallet.Balance = int64(val)
		case int64:
			wallet.Balance = val
		}
	}
	if v, ok := req["credit_limit"]; ok {
		switch val := v.(type) {
		case float64:
			wallet.CreditLimit = int64(val)
		case int64:
			wallet.CreditLimit = val
		}
	}
	if v, ok := req["billing_day"]; ok {
		if val, ok := v.(float64); ok {
			wallet.BillingDay = int(val)
		}
	}
	if v, ok := req["repayment_day"]; ok {
		if val, ok := v.(float64); ok {
			wallet.RepaymentDay = int(val)
		}
	}
	if v, ok := req["card_last_four"]; ok {
		if s, ok := v.(string); ok {
			wallet.CardLastFour = s
		}
	}
	if v, ok := req["exclude_net_asset"]; ok {
		if b, ok := v.(bool); ok {
			wallet.ExcludeNetAsset = b
		} else if num, ok := v.(float64); ok {
			wallet.ExcludeNetAsset = num == 1
		}
	}
	if v, ok := req["is_archived"]; ok {
		if b, ok := v.(bool); ok {
			wallet.IsArchived = b
		} else if num, ok := v.(float64); ok {
			wallet.IsArchived = num == 1
		}
	}

	if err := h.walletRepo.UpdateWallet(r.Context(), wallet); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
		BookID:    wallet.BookID,
		UserID:    uCtx.ID,
		ActorName: uCtx.Name,
		Action:    model.ActivityWalletChanged,
		Summary:   fmt.Sprintf("更新了钱包「%s」", wallet.Name),
	}, service.ChangeWallet)

	updated, _ := h.walletRepo.GetWallet(r.Context(), walletID)
	sendJSON(w, 0, http.StatusOK, "success", updated)
}

func (h *WalletHandler) DeleteWallet(w http.ResponseWriter, r *http.Request, walletID string) {
	uCtx := GetUserContext(r)
	wallet, err := h.walletRepo.GetWallet(r.Context(), walletID)
	if err != nil {
		sendError(w, http.StatusNotFound, "wallet not found")
		return
	}

	wallet.IsArchived = true
	if err := h.walletRepo.UpdateWallet(r.Context(), wallet); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
		BookID:    wallet.BookID,
		UserID:    uCtx.ID,
		ActorName: uCtx.Name,
		Action:    model.ActivityWalletChanged,
		Summary:   fmt.Sprintf("归档了钱包「%s」", wallet.Name),
	}, service.ChangeWallet)
	sendJSON(w, 0, http.StatusOK, "success", nil)
}
