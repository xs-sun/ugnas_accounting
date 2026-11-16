package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
	"github.com/google/uuid"
)

type TransactionHandler struct {
	txRepo     *repository.TransactionRepo
	bookRepo   *repository.BookRepo
	walletRepo *repository.WalletRepo
	dataDir    string
}

func NewTransactionHandler(txRepo *repository.TransactionRepo, bookRepo *repository.BookRepo, walletRepo *repository.WalletRepo, dataDir string) *TransactionHandler {
	return &TransactionHandler{
		txRepo:     txRepo,
		bookRepo:   bookRepo,
		walletRepo: walletRepo,
		dataDir:    dataDir,
	}
}

// ListTransactions godoc
// @Summary 多维聚合筛选交易流水
// @Description 支持按账本、时间范围、账户、分类、收支类型及关键字检索交易明细（支持分页与日分组汇总）
// @Tags Transactions
// @Accept json
// @Produce json
// @Param book_id query string false "账本 ID"
// @Param start_date query string false "起始日期 (YYYY-MM-DD)"
// @Param end_date query string false "截止日期 (YYYY-MM-DD)"
// @Param wallet_id query string false "账户 ID"
// @Param category_id query string false "分类 ID"
// @Param type query string false "交易类型" Enums(expense, income, transfer)
// @Param page query int false "页码 (默认 1)"
// @Param page_size query int false "每页条数 (默认 20, 最大 100)"
// @Success 200 {object} Response{data=object} "流水列表与分页信息"
// @Router /api/v1/transactions [get]
func (h *TransactionHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	bookID := q.Get("book_id")
	startDate := q.Get("start_date")
	endDate := q.Get("end_date")
	walletID := q.Get("wallet_id")
	categoryID := q.Get("category_id")
	txType := q.Get("type")

	if startDate != "" && endDate != "" && startDate > endDate {
		sendError(w, http.StatusBadRequest, "invalid date range")
		return
	}

	page := 1
	if pStr := q.Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil {
			page = p
		}
	}
	if page < 1 {
		page = 1
	}

	pageSize := 20
	if psStr := q.Get("page_size"); psStr != "" {
		if ps, err := strconv.Atoi(psStr); err == nil {
			pageSize = ps
		}
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	items, total, err := h.txRepo.ListTransactions(r.Context(), bookID, walletID, categoryID, txType, startDate, endDate, pageSize, offset)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []model.Transaction{}
	}

	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetTransaction godoc
// @Summary 获取单笔流水详情
// @Description 获取交易详细记录、关联账户信息、分类、标签与凭证附件
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "流水 ID"
// @Success 200 {object} Response{data=model.Transaction} "流水详情"
// @Failure 404 {object} Response "流水不存在"
// @Router /api/v1/transactions/{id} [get]
func (h *TransactionHandler) GetTransaction(w http.ResponseWriter, r *http.Request, txID string) {
	tx, err := h.txRepo.GetTransaction(r.Context(), txID)
	if err != nil {
		sendError(w, http.StatusNotFound, "transaction not found")
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", tx)
}

// CreateTransaction godoc
// @Summary 新增记账流水 (支出 / 收入 / 转账)
// @Description 新增交易并自动触发原子级钱包余额扣减/增加、触发 SQLite 预聚合统计增量刷新
// @Tags Transactions
// @Accept json
// @Produce json
// @Param request body model.Transaction true "交易信息"
// @Success 201 {object} Response{data=model.Transaction} "记账成功"
// @Failure 400 {object} Response "参数不合法"
// @Failure 403 {object} Response "权限不足"
// @Router /api/v1/transactions [post]
func (h *TransactionHandler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		ID         string   `json:"id"`
		BookID     string   `json:"book_id"`
		Type       string   `json:"type"`
		Amount     *float64 `json:"amount"`
		Date       string   `json:"date"`
		WalletID   string   `json:"wallet_id"`
		ToWalletID *string  `json:"to_wallet_id"`
		CategoryID *string  `json:"category_id"`
		Note       string   `json:"note"`
		Tags       []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.Amount == nil || *req.Amount <= 0 {
		sendError(w, http.StatusBadRequest, "amount must be positive integer cents")
		return
	}
	amtCents := int64(*req.Amount)

	if req.BookID != "" {
		role, _ := h.bookRepo.GetMemberRole(r.Context(), req.BookID, uCtx.ID)
		if role == "viewer" {
			sendError(w, http.StatusForbidden, "viewers cannot create transactions")
			return
		}
	}

	if req.Type == "" {
		req.Type = "expense"
	}
	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}

	if req.Type == "transfer" {
		if req.ToWalletID == nil || *req.ToWalletID == "" {
			sendError(w, http.StatusBadRequest, "to_wallet_id required for transfer")
			return
		}
		if req.WalletID == *req.ToWalletID {
			sendError(w, http.StatusBadRequest, "cannot transfer to same wallet")
			return
		}
		w1, err1 := h.walletRepo.GetWallet(r.Context(), req.WalletID)
		w2, err2 := h.walletRepo.GetWallet(r.Context(), *req.ToWalletID)
		if err1 != nil || err2 != nil || w1.BookID != w2.BookID {
			sendError(w, http.StatusBadRequest, "wallets must belong to the same book")
			return
		}
	}

	tx := &model.Transaction{
		ID:         req.ID,
		BookID:     req.BookID,
		Type:       req.Type,
		Amount:     amtCents,
		Date:       req.Date,
		WalletID:   req.WalletID,
		ToWalletID: req.ToWalletID,
		CategoryID: req.CategoryID,
		Note:       req.Note,
		CreatedBy:  uCtx.ID,
	}

	if err := h.txRepo.CreateTransaction(r.Context(), tx, req.Tags); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	created, err := h.txRepo.GetTransaction(r.Context(), tx.ID)
	if err != nil {
		sendJSON(w, 0, http.StatusCreated, "success", tx)
		return
	}
	sendJSON(w, 0, http.StatusCreated, "success", created)
}

// UpdateTransaction godoc
// @Summary 修改交易流水
// @Description 支持修改金额、账户、分类、日期与备注，自动逆向回滚旧账户余额并扣减新账户余额
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "流水 ID"
// @Param request body model.Transaction true "修改内容"
// @Success 200 {object} Response{data=model.Transaction} "修改成功"
// @Failure 404 {object} Response "流水不存在"
// @Router /api/v1/transactions/{id} [put]
func (h *TransactionHandler) UpdateTransaction(w http.ResponseWriter, r *http.Request, txID string) {
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	updated, err := h.txRepo.UpdateTransaction(r.Context(), txID, body)
	if err != nil {
		if err == repository.ErrNotFound {
			sendError(w, http.StatusNotFound, "transaction not found")
			return
		}
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Fetch full transaction details
	fullTx, err := h.txRepo.GetTransaction(r.Context(), txID)
	if err != nil {
		sendJSON(w, 0, http.StatusOK, "success", updated)
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", fullTx)
}

// DeleteTransaction godoc
// @Summary 删除交易流水
// @Description 软删除/硬删除流水，并自动恢复关联账户的原始资金
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "流水 ID"
// @Success 200 {object} Response "删除成功"
// @Failure 404 {object} Response "流水不存在"
// @Router /api/v1/transactions/{id} [delete]
func (h *TransactionHandler) DeleteTransaction(w http.ResponseWriter, r *http.Request, txID string) {
	if err := h.txRepo.DeleteTransaction(r.Context(), txID); err != nil {
		if err == repository.ErrNotFound {
			sendError(w, http.StatusNotFound, "transaction not found")
			return
		}
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", nil)
}

// UploadAttachment godoc
// @Summary 上传小票 / 发票凭证图片
// @Description 上传单据凭证图片文件（支持 JPG/PNG/WEBP，最大 10MB）存储至本地私有磁盘
// @Tags Transactions
// @Accept multipart/form-data
// @Produce json
// @Param id path string true "流水 ID"
// @Param file formData file true "小票图片文件"
// @Success 201 {object} Response "上传成功"
// @Failure 413 {object} Response "文件体积超限"
// @Router /api/v1/transactions/{id}/attachment [post]
func (h *TransactionHandler) UploadAttachment(w http.ResponseWriter, r *http.Request, txID string) {
	// 10MB limit
	const maxUploadSize = 10 * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		sendError(w, http.StatusRequestEntityTooLarge, "file too large")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		sendError(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()

	fname := header.Filename
	if fname == "" {
		fname = "receipt.jpg"
	}
	fnameLower := strings.ToLower(fname)
	if !strings.HasSuffix(fnameLower, ".jpg") && !strings.HasSuffix(fnameLower, ".jpeg") &&
		!strings.HasSuffix(fnameLower, ".png") && !strings.HasSuffix(fnameLower, ".webp") {
		sendError(w, http.StatusUnsupportedMediaType, "unsupported media type")
		return
	}

	attID := "att_" + uuid.NewString()[:8]
	attDir := filepath.Join(h.dataDir, "attachments")
	_ = os.MkdirAll(attDir, 0755)

	destPath := filepath.Join(attDir, fmt.Sprintf("%s_%s", attID, fname))
	out, err := os.Create(destPath)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "failed to save attachment: "+err.Error())
		return
	}
	defer out.Close()

	size, err := io.Copy(out, file)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "failed to write attachment: "+err.Error())
		return
	}

	mimeType := "image/jpeg"
	if strings.HasSuffix(fnameLower, ".png") {
		mimeType = "image/png"
	} else if strings.HasSuffix(fnameLower, ".webp") {
		mimeType = "image/webp"
	}

	if err := h.txRepo.AddAttachment(r.Context(), attID, txID, fname, destPath, size, mimeType); err != nil {
		sendError(w, http.StatusInternalServerError, "failed to record attachment: "+err.Error())
		return
	}

	sendJSON(w, 0, http.StatusCreated, "success", map[string]interface{}{
		"attachment_id": attID,
		"file_name":     fname,
	})
}
