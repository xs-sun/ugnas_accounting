package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"com.xsguyu.accounting/backend/internal/service"
)

type ImportExportHandler struct {
	importService *service.ImportService
	exportService *service.ExportService
}

func NewImportExportHandler(importService *service.ImportService, exportService *service.ExportService) *ImportExportHandler {
	return &ImportExportHandler{
		importService: importService,
		exportService: exportService,
	}
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

	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"inserted": inserted,
	})
}
