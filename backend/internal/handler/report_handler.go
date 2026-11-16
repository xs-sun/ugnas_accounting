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

// MonthlyReport godoc
// @Summary 获取月度收支与分类汇总报表
// @Description 返回指定月份总收入、总支出、净结余及莫兰迪色彩分类排行榜与占比
// @Tags Reports
// @Accept json
// @Produce json
// @Param book_id query string false "账本 ID"
// @Param year query int false "年份 (默认当年)"
// @Param month query int false "月份 (默认当月 1-12)"
// @Success 200 {object} Response{data=model.MonthlyReport} "月度报表"
// @Failure 400 {object} Response "月份参数不合法"
// @Router /api/v1/reports/monthly [get]
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

// TrendReport godoc
// @Summary 获取收支走势分析
// @Description 获取最近 N 个月的双色柱状收支走势与净结余曲线
// @Tags Reports
// @Accept json
// @Produce json
// @Param book_id query string false "账本 ID"
// @Param months query int false "统计月数 (默认 6, 最大 24)"
// @Success 200 {object} Response{data=[]model.TrendItem} "走势数据"
// @Router /api/v1/reports/trend [get]
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

// DailyReport godoc
// @Summary 获取日度收支流水走势
// @Description 获取指定月份每日收入与支出统计
// @Tags Reports
// @Accept json
// @Produce json
// @Param book_id query string false "账本 ID"
// @Param year query int false "年份"
// @Param month query int false "月份"
// @Success 200 {object} Response{data=[]model.DailyReportItem} "每日收支"
// @Router /api/v1/reports/daily [get]
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
