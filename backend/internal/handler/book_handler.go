package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
)

type BookHandler struct {
	bookRepo *repository.BookRepo
}

func NewBookHandler(bookRepo *repository.BookRepo) *BookHandler {
	return &BookHandler{bookRepo: bookRepo}
}

// ListBooks godoc
// @Summary 获取账本列表
// @Description 获取当前登录用户有权限访问的所有账本（包含拥有者、协作者、只读成员）
// @Tags Books
// @Accept json
// @Produce json
// @Success 200 {object} Response{data=[]model.Book} "账本列表"
// @Failure 500 {object} Response "服务器内部错误"
// @Router /api/v1/books [get]
func (h *BookHandler) ListBooks(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	books, err := h.bookRepo.ListBooks(r.Context(), uCtx.ID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if books == nil {
		books = []model.Book{}
	}
	sendJSON(w, 0, http.StatusOK, "success", books)
}

// CreateBook godoc
// @Summary 创建新账本
// @Description 创建新的财务账本，并初始化当前用户为 Owner
// @Tags Books
// @Accept json
// @Produce json
// @Param request body model.Book true "账本创建参数"
// @Success 201 {object} Response{data=model.Book} "创建成功"
// @Failure 400 {object} Response "参数不合法"
// @Failure 500 {object} Response "服务器内部错误"
// @Router /api/v1/books [post]
func (h *BookHandler) CreateBook(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Currency    string `json:"currency"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		sendError(w, http.StatusBadRequest, "book name cannot be blank")
		return
	}
	if req.Currency == "" {
		req.Currency = "CNY"
	}

	book := &model.Book{
		ID:          req.ID,
		Name:        req.Name,
		Currency:    req.Currency,
		Description: req.Description,
		OwnerID:     uCtx.ID,
	}

	if err := h.bookRepo.CreateBook(r.Context(), book, uCtx.Name); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	created, err := h.bookRepo.GetBook(r.Context(), book.ID, uCtx.ID)
	if err != nil {
		book.Role = "owner"
		sendJSON(w, 0, http.StatusCreated, "success", book)
		return
	}
	sendJSON(w, 0, http.StatusCreated, "success", created)
}

// GetBook godoc
// @Summary 获取账本详情
// @Description 根据账本 ID 获取账本详细配置信息
// @Tags Books
// @Accept json
// @Produce json
// @Param id path string true "账本 ID"
// @Success 200 {object} Response{data=model.Book} "账本详情"
// @Failure 404 {object} Response "账本不存在"
// @Router /api/v1/books/{id} [get]
func (h *BookHandler) GetBook(w http.ResponseWriter, r *http.Request, bookID string) {
	uCtx := GetUserContext(r)
	book, err := h.bookRepo.GetBook(r.Context(), bookID, uCtx.ID)
	if err != nil {
		sendError(w, http.StatusNotFound, "book not found")
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", book)
}

// UpdateBook godoc
// @Summary 修改账本信息
// @Description 修改账本名称、币种或备注描述
// @Tags Books
// @Accept json
// @Produce json
// @Param id path string true "账本 ID"
// @Param request body model.Book true "修改参数"
// @Success 200 {object} Response{data=model.Book} "修改成功"
// @Failure 400 {object} Response "参数不合法"
// @Failure 404 {object} Response "账本不存在"
// @Router /api/v1/books/{id} [put]
func (h *BookHandler) UpdateBook(w http.ResponseWriter, r *http.Request, bookID string) {
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Currency    *string `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	uCtx := GetUserContext(r)
	book, err := h.bookRepo.GetBook(r.Context(), bookID, uCtx.ID)
	if err != nil {
		sendError(w, http.StatusNotFound, "book not found")
		return
	}

	if req.Name != nil {
		book.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		book.Description = *req.Description
	}
	if req.Currency != nil {
		book.Currency = *req.Currency
	}

	if err := h.bookRepo.UpdateBook(r.Context(), book); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, _ := h.bookRepo.GetBook(r.Context(), bookID, uCtx.ID)
	sendJSON(w, 0, http.StatusOK, "success", updated)
}

// DeleteBook godoc
// @Summary 安全删除账本
// @Description 物理删除账本及其下全部流水与账户（仅所有者 Owner 可执行）
// @Tags Books
// @Accept json
// @Produce json
// @Param id path string true "账本 ID"
// @Success 200 {object} Response "删除成功"
// @Failure 403 {object} Response "权限不足"
// @Failure 500 {object} Response "服务器内部错误"
// @Router /api/v1/books/{id} [delete]
func (h *BookHandler) DeleteBook(w http.ResponseWriter, r *http.Request, bookID string) {
	uCtx := GetUserContext(r)
	role, err := h.bookRepo.GetMemberRole(r.Context(), bookID, uCtx.ID)
	if err != nil || role != "owner" {
		sendError(w, http.StatusForbidden, "only owner can delete book")
		return
	}

	if err := h.bookRepo.DeleteBook(r.Context(), bookID); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", nil)
}

// ListMembers godoc
// @Summary 获取账本成员列表
// @Description 获取账本内所有成员及其角色权限
// @Tags Books
// @Accept json
// @Produce json
// @Param id path string true "账本 ID"
// @Success 200 {object} Response{data=[]model.BookMember} "成员列表"
// @Router /api/v1/books/{id}/members [get]
func (h *BookHandler) ListMembers(w http.ResponseWriter, r *http.Request, bookID string) {
	members, err := h.bookRepo.ListMembers(r.Context(), bookID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if members == nil {
		members = []model.BookMember{}
	}
	sendJSON(w, 0, http.StatusOK, "success", members)
}

// AddMember godoc
// @Summary 邀请/添加账本成员
// @Description 邀请绿联家庭成员或用户加入账本
// @Tags Books
// @Accept json
// @Produce json
// @Param id path string true "账本 ID"
// @Param request body model.BookMember true "成员信息"
// @Success 201 {object} Response "添加成功"
// @Failure 403 {object} Response "权限不足"
// @Failure 409 {object} Response "成员已存在"
// @Router /api/v1/books/{id}/members [post]
func (h *BookHandler) AddMember(w http.ResponseWriter, r *http.Request, bookID string) {
	uCtx := GetUserContext(r)
	role, err := h.bookRepo.GetMemberRole(r.Context(), bookID, uCtx.ID)
	if err != nil || role != "owner" {
		sendError(w, http.StatusForbidden, "only owner can add members")
		return
	}

	var req struct {
		UserID   string `json:"user_id"`
		UserName string `json:"user_name"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.Role == "" {
		req.Role = "editor"
	}
	if req.UserName == "" {
		req.UserName = "member"
	}

	member := &model.BookMember{
		BookID:   bookID,
		UserID:   req.UserID,
		UserName: req.UserName,
		Role:     req.Role,
	}

	if err := h.bookRepo.AddMember(r.Context(), member); err != nil {
		if err == repository.ErrConflict {
			sendError(w, http.StatusConflict, "member already exists")
			return
		}
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sendJSON(w, 0, http.StatusCreated, "success", map[string]interface{}{
		"book_id": bookID,
		"user_id": req.UserID,
		"role":    req.Role,
	})
}

// RemoveMember godoc
// @Summary 移除账本成员
// @Description 将指定成员从账本中移除（仅 Owner 可操作）
// @Tags Books
// @Accept json
// @Produce json
// @Param id path string true "账本 ID"
// @Param userId path string true "目标用户 ID"
// @Success 200 {object} Response "移除成功"
// @Failure 400 {object} Response "唯一所有者不能移除自身"
// @Failure 403 {object} Response "权限不足"
// @Router /api/v1/books/{id}/members/{userId} [delete]
func (h *BookHandler) RemoveMember(w http.ResponseWriter, r *http.Request, bookID, targetUserID string) {
	uCtx := GetUserContext(r)
	role, err := h.bookRepo.GetMemberRole(r.Context(), bookID, uCtx.ID)
	if err != nil || role != "owner" {
		sendError(w, http.StatusForbidden, "only owner can remove members")
		return
	}

	if targetUserID == uCtx.ID {
		sendError(w, http.StatusBadRequest, "sole owner cannot remove self")
		return
	}

	if err := h.bookRepo.RemoveMember(r.Context(), bookID, targetUserID); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", nil)
}
