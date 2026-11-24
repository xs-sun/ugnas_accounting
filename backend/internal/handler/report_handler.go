package handler

import (
	"net/http"
	"strconv"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
)

type ReportHandler struct {
	txRepo *repository.TransactionRepo
}

func NewReportHandler(txRepo *repository.TransactionRepo) *ReportHandler {
	return &ReportHandler{txRepo: txRepo}
}

func (h *ReportHandler) MonthlyReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	bookID := q.Get("book_id")

	now := time.Now()
	year := now.Year()
	if yStr := q.Get("year"); yStr != "" {
		if y, err := strconv.Atoi(yStr); err == nil {
			year = y
		}
	}

	month := int(now.Month())
	if mStr := q.Get("month"); mStr != "" {
		if m, err := strconv.Atoi(mStr); err == nil {
			month = m
		} else {
			sendError(w, http.StatusBadRequest, "invalid month parameter")
			return
		}
	}

	if month < 1 || month > 12 {
		sendError(w, http.StatusBadRequest, "invalid month parameter")
		return
	}

	report, err := h.txRepo.GetMonthlyReport(r.Context(), bookID, year, month)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if report.Categories == nil {
		report.Categories = []model.CategoryReportItem{}
	}

	sendJSON(w, 0, http.StatusOK, "success", report)
}

func (h *ReportHandler) TrendReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	bookID := q.Get("book_id")

	months := 6
	if mStr := q.Get("months"); mStr != "" {
		if m, err := strconv.Atoi(mStr); err == nil {
			months = m
		}
	}
	if months < 1 {
		months = 1
	}
	if months > 24 {
		months = 24
	}

	trend, err := h.txRepo.GetTrendReport(r.Context(), bookID, months)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sendJSON(w, 0, http.StatusOK, "success", trend)
}

func (h *ReportHandler) DailyReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	bookID := q.Get("book_id")

	now := time.Now()
	year := now.Year()
	if yStr := q.Get("year"); yStr != "" {
		if y, err := strconv.Atoi(yStr); err == nil {
			year = y
		}
	}

	month := int(now.Month())
	if mStr := q.Get("month"); mStr != "" {
		if m, err := strconv.Atoi(mStr); err == nil {
			month = m
		}
	}

	daily, err := h.txRepo.GetDailyReport(r.Context(), bookID, year, month)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if daily == nil {
		daily = []model.DailyReportItem{}
	}

	sendJSON(w, 0, http.StatusOK, "success", daily)
}

func (h *ReportHandler) StatsReport(w http.ResponseWriter, r *http.Request) {
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

	now := time.Now()
	year := now.Year()
	if yStr := q.Get("year"); yStr != "" {
		if y, err := strconv.Atoi(yStr); err == nil {
			year = y
		}
	}

	month := int(now.Month())
	if mStr := q.Get("month"); mStr != "" {
		if m, err := strconv.Atoi(mStr); err == nil {
			month = m
		}
	}
	if month < 1 || month > 12 {
		sendError(w, http.StatusBadRequest, "invalid month parameter")
		return
	}

	stats, err := h.txRepo.GetStatsReport(r.Context(), bookID, period, txType, year, month)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", stats)
}
