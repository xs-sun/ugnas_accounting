package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
	"com.xsguyu.accounting/backend/internal/service"
)

type RecurringHandler struct {
	recurringRepo *repository.RecurringRepo
}

func NewRecurringHandler(recurringRepo *repository.RecurringRepo) *RecurringHandler {
	return &RecurringHandler{recurringRepo: recurringRepo}
}

func (h *RecurringHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	bookID := r.URL.Query().Get("book_id")
	rules, err := h.recurringRepo.ListRules(r.Context(), bookID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rules == nil {
		rules = []model.RecurringRule{}
	}
	sendJSON(w, 0, http.StatusOK, "success", rules)
}

func (h *RecurringHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		ID           string `json:"id"`
		BookID       string `json:"book_id"`
		Type         string `json:"type"`
		CategoryID   string `json:"category_id"`
		SubCategory  string `json:"sub_category"`
		Title        string `json:"title"`
		Amount       int64  `json:"amount"`
		Cycle        string `json:"cycle"`
		StartDate    string `json:"start_date"`
		EndRule      string `json:"end_rule"`
		NextDate     string `json:"next_date"`
		WalletID     string `json:"wallet_id"`
		ExcludeStats bool   `json:"exclude_stats"`
		IsActive     *bool  `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.BookID == "" || req.Title == "" || req.Amount <= 0 || req.WalletID == "" {
		sendError(w, http.StatusBadRequest, "book_id, title, positive amount and wallet_id are required")
		return
	}
	if req.Type == "" {
		req.Type = "expense"
	}
	if req.Cycle == "" {
		req.Cycle = "每月"
	}
	if req.StartDate == "" {
		req.StartDate = time.Now().Format("2006-01-02")
	}
	if req.NextDate == "" {
		req.NextDate = service.ComputeNextDate(req.StartDate, req.Cycle)
	}
	if req.EndRule == "" {
		req.EndRule = "不结束"
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	rr := &model.RecurringRule{
		ID:           req.ID,
		BookID:       req.BookID,
		Type:         req.Type,
		CategoryID:   req.CategoryID,
		SubCategory:  req.SubCategory,
		Title:        req.Title,
		Amount:       req.Amount,
		Cycle:        req.Cycle,
		StartDate:    req.StartDate,
		EndRule:      req.EndRule,
		NextDate:     req.NextDate,
		WalletID:     req.WalletID,
		ExcludeStats: req.ExcludeStats,
		IsActive:     isActive,
		CreatedBy:    uCtx.ID,
	}

	if err := h.recurringRepo.CreateRule(r.Context(), rr); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	created, err := h.recurringRepo.GetRule(r.Context(), rr.ID)
	if err != nil {
		sendJSON(w, 0, http.StatusCreated, "success", rr)
		return
	}
	sendJSON(w, 0, http.StatusCreated, "success", created)
}

func (h *RecurringHandler) UpdateRule(w http.ResponseWriter, r *http.Request, id string) {
	rr, err := h.recurringRepo.GetRule(r.Context(), id)
	if err != nil {
		sendError(w, http.StatusNotFound, "recurring rule not found")
		return
	}

	var req struct {
		Type         *string `json:"type"`
		CategoryID   *string `json:"category_id"`
		SubCategory  *string `json:"sub_category"`
		Title        *string `json:"title"`
		Amount       *int64  `json:"amount"`
		Cycle        *string `json:"cycle"`
		StartDate    *string `json:"start_date"`
		EndRule      *string `json:"end_rule"`
		NextDate     *string `json:"next_date"`
		WalletID     *string `json:"wallet_id"`
		ExcludeStats *bool   `json:"exclude_stats"`
		IsActive     *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.Type != nil {
		rr.Type = *req.Type
	}
	if req.CategoryID != nil {
		rr.CategoryID = *req.CategoryID
	}
	if req.SubCategory != nil {
		rr.SubCategory = *req.SubCategory
	}
	if req.Title != nil {
		rr.Title = strings.TrimSpace(*req.Title)
	}
	if req.Amount != nil && *req.Amount > 0 {
		rr.Amount = *req.Amount
	}
	if req.Cycle != nil {
		rr.Cycle = *req.Cycle
	}
	if req.StartDate != nil {
		rr.StartDate = *req.StartDate
	}
	if req.EndRule != nil {
		rr.EndRule = *req.EndRule
	}
	if req.NextDate != nil {
		rr.NextDate = *req.NextDate
	}
	if req.WalletID != nil {
		rr.WalletID = *req.WalletID
	}
	if req.ExcludeStats != nil {
		rr.ExcludeStats = *req.ExcludeStats
	}
	if req.IsActive != nil {
		rr.IsActive = *req.IsActive
	}

	if err := h.recurringRepo.UpdateRule(r.Context(), rr); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	updated, _ := h.recurringRepo.GetRule(r.Context(), id)
	sendJSON(w, 0, http.StatusOK, "success", updated)
}

func (h *RecurringHandler) DeleteRule(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.recurringRepo.DeleteRule(r.Context(), id); err != nil {
		if err == repository.ErrNotFound {
			sendError(w, http.StatusNotFound, "recurring rule not found")
			return
		}
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", nil)
}

func (h *RecurringHandler) ToggleRule(w http.ResponseWriter, r *http.Request, id string) {
	updated, err := h.recurringRepo.ToggleRule(r.Context(), id)
	if err != nil {
		if err == repository.ErrNotFound {
			sendError(w, http.StatusNotFound, "recurring rule not found")
			return
		}
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", updated)
}
