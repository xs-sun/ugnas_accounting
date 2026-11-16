package handler

import (
	"net/http"

	"com.xsguyu.accounting/backend/internal/repository"
)

type UserHandler struct {
	bookRepo   *repository.BookRepo
	walletRepo *repository.WalletRepo
	txRepo     *repository.TransactionRepo
}

func NewUserHandler(bookRepo *repository.BookRepo, walletRepo *repository.WalletRepo, txRepo *repository.TransactionRepo) *UserHandler {
	return &UserHandler{
		bookRepo:   bookRepo,
		walletRepo: walletRepo,
		txRepo:     txRepo,
	}
}

// GetProfile godoc
// @Summary 获取当前登录用户信息与账本统计
// @Description 返回当前用户的 UID、昵称、身份类型以及我创建的/我参与的账本统计概览
// @Tags User
// @Accept json
// @Produce json
// @Success 200 {object} Response "用户资料与统计数据"
// @Router /api/v1/user/profile [get]
func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	books, _ := h.bookRepo.ListBooks(r.Context(), uCtx.ID)

	var createdBooksCount, joinedBooksCount int
	for _, b := range books {
		if b.OwnerID == uCtx.ID || b.Role == "owner" {
			createdBooksCount++
		} else {
			joinedBooksCount++
		}
	}

	_, totalTx, _ := h.txRepo.ListTransactions(r.Context(), "", "", "", "", "", "", 1, 0)
	wallets, _ := h.walletRepo.ListWallets(r.Context(), "", false)

	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"user_id":                  uCtx.ID,
		"user_name":                uCtx.Name,
		"user_type":                uCtx.Type,
		"created_books_count":      createdBooksCount,
		"joined_books_count":       joinedBooksCount,
		"total_books_count":        len(books),
		"total_transactions_count": totalTx,
		"total_wallets_count":      len(wallets),
	})
}
