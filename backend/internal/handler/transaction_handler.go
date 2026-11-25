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
	"com.xsguyu.accounting/backend/internal/service"
	"github.com/google/uuid"
)

type TransactionHandler struct {
	txRepo     *repository.TransactionRepo
	bookRepo   *repository.BookRepo
	walletRepo *repository.WalletRepo
	dataDir    string
	hub        *service.Hub
	recorder   *service.ActivityRecorder
}

func NewTransactionHandler(txRepo *repository.TransactionRepo, bookRepo *repository.BookRepo, walletRepo *repository.WalletRepo, dataDir string, hub *service.Hub, recorder *service.ActivityRecorder) *TransactionHandler {
	return &TransactionHandler{
		txRepo:     txRepo,
		bookRepo:   bookRepo,
		walletRepo: walletRepo,
		dataDir:    dataDir,
		hub:        hub,
		recorder:   recorder,
	}
}

// txSummary 拼一条交易动态的摘要，如“记了一笔支出「午餐」¥32.00”。
func txSummary(verb, txType, title string, amount int64) string {
	label := "支出"
	switch txType {
	case "income":
		label = "收入"
	case "transfer":
		label = "转账"
	}
	if title == "" {
		title = label
	}
	if txType == "transfer" {
		return fmt.Sprintf("转账「%s」%s", title, service.FormatYuan(amount))
	}
	return fmt.Sprintf("%s%s「%s」%s", verb, label, title, service.FormatYuan(amount))
}

func txAmountPtr(amount int64) *int64 { return &amount }
func txIDPtr(id string) *string       { return &id }

func (h *TransactionHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	bookID := q.Get("book_id")
	startDate := q.Get("start_date")
	endDate := q.Get("end_date")
	walletID := q.Get("wallet_id")
	categoryID := q.Get("category_id")
	txType := q.Get("type")
	search := q.Get("search")
	memberID := q.Get("member_id")
	if search == "" {
		search = q.Get("keyword")
	}

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
	if pageSize > 500 {
		pageSize = 500
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	items, total, err := h.txRepo.ListTransactions(r.Context(), bookID, walletID, categoryID, txType, startDate, endDate, search, pageSize, offset, memberID)
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

func (h *TransactionHandler) GetTransaction(w http.ResponseWriter, r *http.Request, txID string) {
	tx, err := h.txRepo.GetTransaction(r.Context(), txID)
	if err != nil {
		sendError(w, http.StatusNotFound, "transaction not found")
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", tx)
}

func (h *TransactionHandler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		ID            string   `json:"id"`
		BookID        string   `json:"book_id"`
		Type          string   `json:"type"`
		Amount        *float64 `json:"amount"`
		Date          string   `json:"date"`
		TxTime        string   `json:"tx_time"`
		WalletID      string   `json:"wallet_id"`
		ToWalletID    *string  `json:"to_wallet_id"`
		CategoryID    *string  `json:"category_id"`
		Title         string   `json:"title"`
		SubCategory   string   `json:"sub_category"`
		Note          string   `json:"note"`
		IncludeBudget *bool    `json:"include_budget"`
		Reimburse     bool     `json:"reimburse"`
		// MemberID 归属成员（真实或虚拟成员）；为空时默认是创建人本人。
		MemberID      string   `json:"member_id"`
		Tags          []string `json:"tags"`
		TagIDs        []string `json:"tag_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.Tags) == 0 && len(req.TagIDs) > 0 {
		req.Tags = req.TagIDs
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

	// 归属成员必须是本账本的成员（真实成员或虚拟成员），否则拒绝；
	// 账本所有者即使不在成员表里也算成员（GetMemberRole 有 owner 兜底）。
	if req.MemberID != "" && req.BookID != "" {
		if _, err := h.bookRepo.GetMemberRole(r.Context(), req.BookID, req.MemberID); err != nil {
			sendError(w, http.StatusBadRequest, "invalid member_id: not a member of this book")
			return
		}
	}

	if req.Type == "" {
		req.Type = "expense"
	}
	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}
	if req.TxTime == "" {
		req.TxTime = time.Now().Format("15:04")
	}
	incBudget := true
	if req.IncludeBudget != nil {
		incBudget = *req.IncludeBudget
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

	if req.CategoryID != nil && strings.TrimSpace(*req.CategoryID) == "" {
		req.CategoryID = nil
	}
	if req.ToWalletID != nil && strings.TrimSpace(*req.ToWalletID) == "" {
		req.ToWalletID = nil
	}

	tx := &model.Transaction{
		ID:            req.ID,
		BookID:        req.BookID,
		Type:          req.Type,
		Amount:        amtCents,
		Date:          req.Date,
		TxTime:        req.TxTime,
		WalletID:      req.WalletID,
		ToWalletID:    req.ToWalletID,
		CategoryID:    req.CategoryID,
		Title:         req.Title,
		SubCategory:   req.SubCategory,
		Note:          req.Note,
		IncludeBudget: incBudget,
		Reimburse:     req.Reimburse,
		MemberID:      req.MemberID,
		CreatedBy:     uCtx.ID,
	}

	if err := h.txRepo.CreateTransaction(r.Context(), tx, req.Tags); err != nil {
		msg := err.Error()
		if strings.HasPrefix(msg, "invalid ") || strings.Contains(msg, "constraint failed") || strings.Contains(msg, "requires ") {
			sendError(w, http.StatusBadRequest, msg)
			return
		}
		sendError(w, http.StatusInternalServerError, msg)
		return
	}
	// AI 草稿确认入账也走 CreateTransaction，这里统一写动态+广播，
	// 再检查预算（新记的支出可能让当月预算越线）。
	recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
		BookID:      req.BookID,
		UserID:      uCtx.ID,
		ActorName:   uCtx.Name,
		Action:      model.ActivityTxCreated,
		Summary:     txSummary("记了一笔", req.Type, req.Title, amtCents),
		AmountCents: txAmountPtr(amtCents),
		TxID:        txIDPtr(tx.ID),
	}, service.ChangeTransaction)
	h.recorder.CheckBudgets(r.Context(), req.BookID)

	created, err := h.txRepo.GetTransaction(r.Context(), tx.ID)
	if err != nil {
		sendJSON(w, 0, http.StatusCreated, "success", tx)
		return
	}
	sendJSON(w, 0, http.StatusCreated, "success", created)
}

func (h *TransactionHandler) UpdateTransaction(w http.ResponseWriter, r *http.Request, txID string) {
	uCtx := GetUserContext(r)
	// 先取旧记录：拿到账本 ID 才能广播；不存在时直接 404。
	old, err := h.txRepo.GetTransaction(r.Context(), txID)
	if err != nil {
		sendError(w, http.StatusNotFound, "transaction not found")
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	// 改归属成员时同样校验：必须是本账本的成员。
	if mid, ok := body["member_id"]; ok {
		if s, ok := mid.(string); ok && s != "" {
			if _, err := h.bookRepo.GetMemberRole(r.Context(), old.BookID, s); err != nil {
				sendError(w, http.StatusBadRequest, "invalid member_id: not a member of this book")
				return
			}
		}
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
	recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
		BookID:      old.BookID,
		UserID:      uCtx.ID,
		ActorName:   uCtx.Name,
		Action:      model.ActivityTxUpdated,
		Summary:     txSummary("修改了一笔", updated.Type, updated.Title, updated.Amount),
		AmountCents: txAmountPtr(updated.Amount),
		TxID:        txIDPtr(txID),
	}, service.ChangeTransaction)
	h.recorder.CheckBudgets(r.Context(), old.BookID)

	fullTx, err := h.txRepo.GetTransaction(r.Context(), txID)
	if err != nil {
		sendJSON(w, 0, http.StatusOK, "success", updated)
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", fullTx)
}

func (h *TransactionHandler) DeleteTransaction(w http.ResponseWriter, r *http.Request, txID string) {
	uCtx := GetUserContext(r)
	old, err := h.txRepo.GetTransaction(r.Context(), txID)
	if err != nil {
		sendError(w, http.StatusNotFound, "transaction not found")
		return
	}

	if err := h.txRepo.DeleteTransaction(r.Context(), txID); err != nil {
		if err == repository.ErrNotFound {
			sendError(w, http.StatusNotFound, "transaction not found")
			return
		}
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
		BookID:      old.BookID,
		UserID:      uCtx.ID,
		ActorName:   uCtx.Name,
		Action:      model.ActivityTxDeleted,
		Summary:     txSummary("删除了一笔", old.Type, old.Title, old.Amount),
		AmountCents: txAmountPtr(old.Amount),
		TxID:        txIDPtr(txID),
	}, service.ChangeTransaction)
	h.recorder.CheckBudgets(r.Context(), old.BookID)
	sendJSON(w, 0, http.StatusOK, "success", nil)
}

func (h *TransactionHandler) BatchDelete(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}
	// 先取每笔的账本（一次批量删除可能跨账本，按账本分别广播）。
	bookIDs := make(map[string]struct{})
	bookCounts := make(map[string]int)
	for _, id := range req.IDs {
		if tx, err := h.txRepo.GetTransaction(r.Context(), id); err == nil {
			bookIDs[tx.BookID] = struct{}{}
			bookCounts[tx.BookID]++
		}
	}
	deleted, err := h.txRepo.BatchDeleteTransactions(r.Context(), req.IDs)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if deleted > 0 {
		for bookID := range bookIDs {
			recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
				BookID:    bookID,
				UserID:    uCtx.ID,
				ActorName: uCtx.Name,
				Action:    model.ActivityTxDeleted,
				Summary:   fmt.Sprintf("批量删除了 %d 笔流水", bookCounts[bookID]),
			}, service.ChangeTransaction)
			h.recorder.CheckBudgets(r.Context(), bookID)
		}
	}
	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"deleted": deleted,
	})
}

func (h *TransactionHandler) UploadAttachment(w http.ResponseWriter, r *http.Request, txID string) {
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

	destPath := filepath.Join(attDir, fmt.Sprintf("%s_%s", attID, filepath.Base(fname)))
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
