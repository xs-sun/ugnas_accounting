package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
)

type WalletHandler struct {
	walletRepo *repository.WalletRepo
	bookRepo   *repository.BookRepo
}

func NewWalletHandler(walletRepo *repository.WalletRepo, bookRepo *repository.BookRepo) *WalletHandler {
	return &WalletHandler{
		walletRepo: walletRepo,
		bookRepo:   bookRepo,
	}
}

var fourDigitsRegex = regexp.MustCompile(`^\d{4}$`)

// ListWallets godoc
// @Summary 获取资产账户列表
// @Description 获取指定账本或全局的所有资产账户列表（含余额、可用额度与类型图标）
// @Tags Wallets
// @Accept json
// @Produce json
// @Param book_id query string false "账本 ID"
// @Success 200 {object} Response{data=[]model.Wallet} "账户列表"
// @Router /api/v1/wallets [get]
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

// GetSummary godoc
// @Summary 获取资产与负债概览
// @Description 获取总资产、总负债与净资产的动态核算结果
// @Tags Wallets
// @Accept json
// @Produce json
// @Param book_id query string false "账本 ID"
// @Success 200 {object} Response{data=model.WalletSummary} "资产汇总数据"
// @Router /api/v1/wallets/summary [get]
func (h *WalletHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	bookID := r.URL.Query().Get("book_id")
	summary, err := h.walletRepo.GetWalletSummary(r.Context(), bookID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", summary)
}

// CreateWallet godoc
// @Summary 创建新资产账户
// @Description 支持资金账户(cash)、信用负债(credit)、投资理财(invest)及应收应付(debt)四大核心类型
// @Tags Wallets
// @Accept json
// @Produce json
// @Param request body model.Wallet true "账户信息"
// @Success 201 {object} Response{data=model.Wallet} "创建成功"
// @Failure 400 {object} Response "参数不合法"
// @Router /api/v1/wallets [post]
func (h *WalletHandler) CreateWallet(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		ID              string `json:"id"`
		BookID          string `json:"book_id"`
		Name            string `json:"name"`
		Type            string `json:"type"`
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

	created, err := h.walletRepo.GetWallet(r.Context(), wallet.ID)
	if err != nil {
		sendJSON(w, 0, http.StatusCreated, "success", wallet)
		return
	}
	sendJSON(w, 0, http.StatusCreated, "success", created)
}

// UpdateWallet godoc
// @Summary 更新账户信息与余额
// @Description 支持修改账户名称、余额校准、信用额度、账单日、还款日及卡号后四位
// @Tags Wallets
// @Accept json
// @Produce json
// @Param id path string true "账户 ID"
// @Param request body model.Wallet true "更新内容"
// @Success 200 {object} Response{data=model.Wallet} "更新成功"
// @Failure 404 {object} Response "账户不存在"
// @Router /api/v1/wallets/{id} [put]
func (h *WalletHandler) UpdateWallet(w http.ResponseWriter, r *http.Request, walletID string) {
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

	updated, _ := h.walletRepo.GetWallet(r.Context(), walletID)
	sendJSON(w, 0, http.StatusOK, "success", updated)
}

// DeleteWallet godoc
// @Summary 删除/归档资产账户
// @Description 软删除（归档）指定资产账户，保留历史流水关联
// @Tags Wallets
// @Accept json
// @Produce json
// @Param id path string true "账户 ID"
// @Success 200 {object} Response "归档成功"
// @Failure 404 {object} Response "账户不存在"
// @Router /api/v1/wallets/{id} [delete]
func (h *WalletHandler) DeleteWallet(w http.ResponseWriter, r *http.Request, walletID string) {
	// Deleting wallet marks it as archived
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
	sendJSON(w, 0, http.StatusOK, "success", nil)
}
