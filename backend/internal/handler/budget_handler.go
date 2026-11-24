package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"com.xsguyu.accounting/backend/internal/repository"
)

type BudgetHandler struct {
	budgetRepo *repository.BudgetRepo
	bookRepo   *repository.BookRepo
}

func NewBudgetHandler(budgetRepo *repository.BudgetRepo, bookRepo *repository.BookRepo) *BudgetHandler {
	return &BudgetHandler{
		budgetRepo: budgetRepo,
		bookRepo:   bookRepo,
	}
}

func (h *BudgetHandler) GetBudgets(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	q := r.URL.Query()
	bookID := strings.TrimSpace(q.Get("book_id"))
	if bookID == "" {
		books, err := h.bookRepo.ListBooks(r.Context(), uCtx.ID)
		if err == nil && len(books) > 0 {
			bookID = books[0].ID
		}
	}

	now := time.Now()
	year := now.Year()
	if yStr := q.Get("year"); yStr != "" {
		if y, err := strconv.Atoi(yStr); err == nil && y > 0 {
			year = y
		}
	}
	month := int(now.Month())
	if mStr := q.Get("month"); mStr != "" {
		if m, err := strconv.Atoi(mStr); err == nil && m >= 1 && m <= 12 {
			month = m
		}
	}

	ov, err := h.budgetRepo.GetBudgetOverview(r.Context(), bookID, year, month)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", ov)
}

func (h *BudgetHandler) UpdateTotalBudget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BookID string `json:"book_id"`
		Budget int64  `json:"budget"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.BookID) == "" || req.Budget < 0 {
		sendError(w, http.StatusBadRequest, "book_id and non-negative budget required")
		return
	}

	if err := h.budgetRepo.UpdateTotalBudget(r.Context(), req.BookID, req.Budget); err != nil {
		if err == repository.ErrNotFound {
			sendError(w, http.StatusNotFound, "book not found")
			return
		}
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	now := time.Now()
	ov, err := h.budgetRepo.GetBudgetOverview(r.Context(), req.BookID, now.Year(), int(now.Month()))
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", ov)
}

func (h *BudgetHandler) UpdateCategoryBudget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BookID      string `json:"book_id"`
		CategoryID  string `json:"category_id"`
		LimitAmount int64  `json:"limit_amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.BookID) == "" || strings.TrimSpace(req.CategoryID) == "" || req.LimitAmount < 0 {
		sendError(w, http.StatusBadRequest, "book_id, category_id and non-negative limit_amount required")
		return
	}

	if err := h.budgetRepo.UpsertCategoryBudget(r.Context(), req.BookID, req.CategoryID, req.LimitAmount); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	now := time.Now()
	ov, err := h.budgetRepo.GetBudgetOverview(r.Context(), req.BookID, now.Year(), int(now.Month()))
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", ov)
}
