package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
	"com.xsguyu.accounting/backend/internal/service"
)

type ImportExportHandler struct {
	importService *service.ImportService
	exportService *service.ExportService
	hub           *service.Hub
	recorder      *service.ActivityRecorder
	aiSvc         *service.AiService
	bookRepo      *repository.BookRepo
}

func NewImportExportHandler(importService *service.ImportService, exportService *service.ExportService, hub *service.Hub, recorder *service.ActivityRecorder, aiSvc *service.AiService, bookRepo *repository.BookRepo) *ImportExportHandler {
	return &ImportExportHandler{
		importService: importService,
		exportService: exportService,
		hub:           hub,
		recorder:      recorder,
		aiSvc:         aiSvc,
		bookRepo:      bookRepo,
	}
}

// requireBookWriter 校验请求者是目标账本的成员且不是只读 viewer；否则返回 false（已写错误响应）。
func (h *ImportExportHandler) requireBookWriter(w http.ResponseWriter, r *http.Request, bookID string) bool {
	uCtx := GetUserContext(r)
	if bookID == "" {
		sendError(w, http.StatusBadRequest, "book_id required")
		return false
	}
	role, err := h.bookRepo.GetMemberRole(r.Context(), bookID, uCtx.ID)
	if err != nil || role == "" {
		sendError(w, http.StatusForbidden, "not a member of this book")
		return false
	}
	if role == "viewer" {
		sendError(w, http.StatusForbidden, "viewers cannot import")
		return false
	}
	return true
}

// ExportLedger godoc
// @Summary 导出账本流水为 CSV
// @Description 导出指定账本全部明细记录为标准 UTF-8 CSV 文件
// @Tags Import & Export
// @Accept json
// @Produce text/csv
// @Param book_id query string false "账本 ID"
// @Success 200 {file} file "CSV 数据流"
// @Router /api/v1/export/ledger [get]
func (h *ImportExportHandler) ExportLedger(w http.ResponseWriter, r *http.Request) {
	bookID := r.URL.Query().Get("book_id")
	csvBytes, err := h.exportService.ExportCSV(r.Context(), bookID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sendBytes(w, csvBytes, "text/csv; charset=utf-8", map[string]string{
		"Content-Disposition": `attachment; filename="ledger_export.csv"`,
	}, http.StatusOK)
}

// ImportWeChat godoc
// @Summary 解析微信支付 CSV 账单
// @Description 上传微信支付账单 CSV（自动识别 UTF-8 BOM 编码），智能过滤提现还款，匹配预置收支分类
// @Tags Import & Export
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "微信账单 CSV 文件"
// @Success 200 {object} Response{data=object} "解析预览结果"
// @Failure 400 {object} Response "文件格式不合法"
// @Router /api/v1/import/wechat [post]
func (h *ImportExportHandler) ImportWeChat(w http.ResponseWriter, r *http.Request) {
	const maxUploadSize = 10 * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		sendError(w, http.StatusRequestEntityTooLarge, "file too large")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		sendError(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()

	rawBytes, err := io.ReadAll(file)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "failed to read uploaded file: "+err.Error())
		return
	}

	items, err := h.importService.ParseWeChatCSV(rawBytes)
	if err != nil {
		sendError(w, http.StatusBadRequest, "failed to parse WeChat CSV: "+err.Error())
		return
	}
	if items == nil {
		items = []service.ImportedItem{}
	}

	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"items": items,
		"count": len(items),
	})
}

// ImportAlipay godoc
// @Summary 解析支付宝 CSV 账单
// @Description 上传支付宝账单 CSV（自动处理 GBK 编码转换与元转分高精度转换），自动生成防重哈希
// @Tags Import & Export
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "支付宝账单 CSV 文件"
// @Success 200 {object} Response{data=object} "解析预览结果"
// @Failure 400 {object} Response "文件格式不合法"
// @Router /api/v1/import/alipay [post]
func (h *ImportExportHandler) ImportAlipay(w http.ResponseWriter, r *http.Request) {
	const maxUploadSize = 10 * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		sendError(w, http.StatusRequestEntityTooLarge, "file too large")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		sendError(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()

	rawBytes, err := io.ReadAll(file)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "failed to read uploaded file: "+err.Error())
		return
	}

	items, err := h.importService.ParseAlipayCSV(rawBytes)
	if err != nil {
		sendError(w, http.StatusBadRequest, "failed to parse Alipay CSV: "+err.Error())
		return
	}
	if items == nil {
		items = []service.ImportedItem{}
	}

	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"items": items,
		"count": len(items),
	})
}

// CommitImport godoc
// @Summary 批量确认入库账单流水
// @Description 批量将勾选确认的微信/支付宝账单条目写入数据库，并更新账户余额
// @Tags Import & Export
// @Accept json
// @Produce json
// @Param request body object true "入库请求体"
// @Success 200 {object} Response "入库成功"
// @Router /api/v1/import/commit [post]
func (h *ImportExportHandler) CommitImport(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		BookID   string                 `json:"book_id"`
		WalletID string                 `json:"wallet_id"`
		Items    []service.ImportedItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	inserted, err := h.importService.CommitImport(r.Context(), req.BookID, req.WalletID, req.Items, uCtx.ID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if inserted > 0 {
		recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
			BookID:    req.BookID,
			UserID:    uCtx.ID,
			ActorName: uCtx.Name,
			Action:    model.ActivityImportCommitted,
			Summary:   fmt.Sprintf("导入了 %d 条流水", inserted),
		}, service.ChangeTransaction)
		h.recorder.CheckBudgets(r.Context(), req.BookID)
	}

	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"inserted": inserted,
	})
}

// ParseSuishouji godoc
// @Summary 解析随手记导出的 Excel/CSV 文件
// @Description 上传随手记导出的 .xlsx/.xls/.csv 文件，表头自适应解析，返回表头、自动映射与 AI 建议映射
// @Tags Import & Export
// @Accept multipart/form-data
// @Produce json
// @Param book_id formData string true "目标账本 ID"
// @Param file formData file true "随手记导出文件"
// @Success 200 {object} Response{data=object} "解析预览结果"
// @Failure 400 {object} Response "文件格式不合法"
// @Failure 403 {object} Response "非账本成员"
// @Router /api/v1/imports/suishouji/parse [post]
func (h *ImportExportHandler) ParseSuishouji(w http.ResponseWriter, r *http.Request) {
	const maxUploadSize = 10 * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		sendError(w, http.StatusRequestEntityTooLarge, "file too large (max 10MB)")
		return
	}
	bookID := r.FormValue("book_id")
	if !h.requireBookWriter(w, r, bookID) {
		return
	}
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		sendError(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()
	rawBytes, err := io.ReadAll(file)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "failed to read uploaded file: "+err.Error())
		return
	}

	table, err := h.importService.ParseSuishoujiFile(rawBytes, fileHeader.Filename)
	if err != nil {
		sendError(w, http.StatusBadRequest, "failed to parse file: "+err.Error())
		return
	}
	if len(table.Rows) > service.MaxSuishoujiRows {
		sendError(w, http.StatusBadRequest, fmt.Sprintf("too many rows: %d (max %d per file, please split it)", len(table.Rows), service.MaxSuishoujiRows))
		return
	}

	autoMapping := service.AutoMapColumns(table.Headers)
	samples := table.Rows
	if len(samples) > 5 {
		samples = samples[:5]
	}
	aiSuggestions, aiAvailable, aiReason := map[string]string{}, false, ""
	if h.aiSvc != nil {
		uCtx := GetUserContext(r)
		if sug, ok, reason := h.aiSvc.SuggestColumnMapping(r.Context(), uCtx.ID, table.Headers, samples); ok {
			aiSuggestions, aiAvailable = sug, true
		} else {
			aiReason = reason
		}
	}

	// 默认钱包：账本第一个可用钱包。
	defaultWalletID, _ := h.importService.DefaultWalletID(r.Context(), bookID)
	uCtx := GetUserContext(r)
	preview := h.importService.BuildPreview(r.Context(), bookID, uCtx.ID, table, autoMapping, defaultWalletID, 10)

	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"headers":         table.Headers,
		"total_rows":      len(table.Rows),
		"rows":            table.Rows,
		"samples":         samples,
		"auto_mapping":    autoMapping,
		"ai_available":    aiAvailable,
		"ai_reason":       aiReason,
		"ai_suggestions":  aiSuggestions,
		"preview":         preview,
		"default_wallet_id": defaultWalletID,
	})
}

// PreviewSuishouji godoc
// @Summary 按指定映射预览随手记导入前 10 行
// @Description 用户改动字段映射后，用该映射重新解析前 10 行（含分类/钱包匹配结果）
// @Tags Import & Export
// @Accept json
// @Produce json
// @Param request body object true "预览请求体"
// @Success 200 {object} Response{data=object} "预览结果"
// @Router /api/v1/imports/suishouji/preview [post]
// validateSuishoujiMapping 校验字段映射：日期和金额必须映射；
// 日期/收支类型/金额/账户这几个单值字段不能被多列同时映射（否则取值有歧义）。
func validateSuishoujiMapping(mapping map[string]service.ColumnTarget) error {
	counts := make(map[service.ColumnTarget]int)
	for _, t := range mapping {
		counts[t]++
	}
	if counts[service.TargetDate] == 0 || counts[service.TargetAmount] == 0 {
		return fmt.Errorf("mapping must include date and amount columns")
	}
	for _, t := range []service.ColumnTarget{
		service.TargetDate, service.TargetType, service.TargetAmount, service.TargetWallet,
	} {
		if counts[t] > 1 {
			return fmt.Errorf("target %q is mapped by %d columns", string(t), counts[t])
		}
	}
	return nil
}

func (h *ImportExportHandler) PreviewSuishouji(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		BookID          string                        `json:"book_id"`
		Mapping         map[string]service.ColumnTarget `json:"mapping"`
		DefaultWalletID string                        `json:"default_wallet_id"`
		Headers         []string                      `json:"headers"`
		Rows            [][]string                    `json:"rows"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !h.requireBookWriter(w, r, req.BookID) {
		return
	}
	if err := validateSuishoujiMapping(req.Mapping); err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Rows) > 10 {
		req.Rows = req.Rows[:10]
	}
	table := &service.SuishoujiTable{Headers: req.Headers, Rows: req.Rows}
	preview := h.importService.BuildPreview(r.Context(), req.BookID, uCtx.ID, table, req.Mapping, req.DefaultWalletID, 10)
	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"preview": preview,
	})
}

// CommitSuishouji godoc
// @Summary 确认导入随手记流水
// @Description 按用户确认的字段映射把解析行批量写入账本（走正常记账路径），非法行记入失败明细不中断整批
// @Tags Import & Export
// @Accept json
// @Produce json
// @Param request body object true "导入请求体"
// @Success 200 {object} Response "导入汇总"
// @Failure 400 {object} Response "请求不合法"
// @Failure 403 {object} Response "非账本成员"
// @Router /api/v1/imports/suishouji/commit [post]
func (h *ImportExportHandler) CommitSuishouji(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		BookID          string                        `json:"book_id"`
		Mapping         map[string]service.ColumnTarget `json:"mapping"`
		DefaultWalletID string                        `json:"default_wallet_id"`
		Headers         []string                      `json:"headers"`
		Rows            [][]string                    `json:"rows"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !h.requireBookWriter(w, r, req.BookID) {
		return
	}
	if len(req.Rows) == 0 {
		sendError(w, http.StatusBadRequest, "no rows to import")
		return
	}
	if len(req.Rows) > service.MaxSuishoujiRows {
		sendError(w, http.StatusBadRequest, fmt.Sprintf("too many rows: %d (max %d per file, please split it)", len(req.Rows), service.MaxSuishoujiRows))
		return
	}
	if err := validateSuishoujiMapping(req.Mapping); err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	// 映射值合法性：只接受已知目标字段。
	for _, t := range req.Mapping {
		switch t {
		case service.TargetDate, service.TargetType, service.TargetAmount,
			service.TargetCategory, service.TargetSubCategory, service.TargetWallet,
			service.TargetTitle, service.TargetNote, service.TargetTags, service.TargetIgnore:
		default:
			sendError(w, http.StatusBadRequest, fmt.Sprintf("invalid mapping target %q", string(t)))
			return
		}
	}

	table := &service.SuishoujiTable{Headers: req.Headers, Rows: req.Rows}
	result, err := h.importService.CommitSuishouji(r.Context(), req.BookID, uCtx.ID, table, req.Mapping, req.DefaultWalletID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if result.Inserted > 0 {
		recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
			BookID:    req.BookID,
			UserID:    uCtx.ID,
			ActorName: uCtx.Name,
			Action:    model.ActivityImportCommitted,
			Summary:   fmt.Sprintf("从随手记导入了 %d 条流水", result.Inserted),
		}, service.ChangeTransaction)
		h.recorder.CheckBudgets(r.Context(), req.BookID)
	}

	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"inserted":   result.Inserted,
		"duplicates": result.Duplicates,
		"failed":     result.Failed,
		"failures":   result.Failures,
	})
}
