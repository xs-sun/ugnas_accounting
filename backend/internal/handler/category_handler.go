package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
)

type CategoryHandler struct {
	catRepo *repository.CategoryRepo
}

func NewCategoryHandler(catRepo *repository.CategoryRepo) *CategoryHandler {
	return &CategoryHandler{catRepo: catRepo}
}

// ListCategories godoc
// @Summary 获取收支两级分类列表
// @Description 支持按账本 ID 与分类类型（支出 expense / 收入 income）获取两级分类树及莫兰迪色彩配置
// @Tags Categories & Tags
// @Accept json
// @Produce json
// @Param book_id query string false "账本 ID"
// @Param type query string false "分类类型" Enums(expense, income)
// @Success 200 {object} Response{data=[]model.Category} "分类列表"
// @Router /api/v1/categories [get]
func (h *CategoryHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	bookID := r.URL.Query().Get("book_id")
	catType := r.URL.Query().Get("type")
	cats, err := h.catRepo.ListCategories(r.Context(), bookID, catType)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cats == nil {
		cats = []model.Category{}
	}
	sendJSON(w, 0, http.StatusOK, "success", cats)
}

// CreateCategory godoc
// @Summary 创建自定义分类
// @Description 支持创建一级分类或指定 parent_id 创建二级子分类
// @Tags Categories & Tags
// @Accept json
// @Produce json
// @Param request body model.Category true "分类信息"
// @Success 201 {object} Response{data=model.Category} "创建成功"
// @Failure 400 {object} Response "参数不合法"
// @Failure 409 {object} Response "同名分类已存在"
// @Router /api/v1/categories [post]
func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string  `json:"id"`
		BookID   string  `json:"book_id"`
		Type     string  `json:"type"`
		Name     string  `json:"name"`
		Icon     string  `json:"icon"`
		Color    string  `json:"color"`
		ParentID *string `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		sendError(w, http.StatusBadRequest, "category name required")
		return
	}
	if req.Type == "" {
		req.Type = "expense"
	}
	if req.Icon == "" {
		req.Icon = "tag"
	}
	if req.Color == "" {
		req.Color = "#0E8C7F"
	}

	// Check for duplicate name in book
	existing, err := h.catRepo.ListCategories(r.Context(), req.BookID, "")
	if err == nil {
		for _, c := range existing {
			if c.Name == req.Name {
				sendError(w, http.StatusConflict, "category already exists in this book")
				return
			}
		}
	}

	cat := &model.Category{
		ID:       req.ID,
		BookID:   req.BookID,
		Type:     req.Type,
		Name:     req.Name,
		Icon:     req.Icon,
		Color:    req.Color,
		ParentID: req.ParentID,
	}

	if err := h.catRepo.CreateCategory(r.Context(), cat); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	created, err := h.catRepo.GetCategory(r.Context(), cat.ID)
	if err != nil {
		sendJSON(w, 0, http.StatusCreated, "success", cat)
		return
	}
	sendJSON(w, 0, http.StatusCreated, "success", created)
}

// UpdateCategory godoc
// @Summary 修改分类信息
// @Description 修改分类名称、图标、色彩或从属关系
// @Tags Categories & Tags
// @Accept json
// @Produce json
// @Param id path string true "分类 ID"
// @Param request body model.Category true "修改参数"
// @Success 200 {object} Response{data=model.Category} "修改成功"
// @Failure 404 {object} Response "分类不存在"
// @Router /api/v1/categories/{id} [put]
func (h *CategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request, catID string) {
	cat, err := h.catRepo.GetCategory(r.Context(), catID)
	if err != nil {
		sendError(w, http.StatusNotFound, "category not found")
		return
	}

	var req struct {
		Name     *string `json:"name"`
		Icon     *string `json:"icon"`
		Color    *string `json:"color"`
		ParentID *string `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.Name != nil {
		cat.Name = strings.TrimSpace(*req.Name)
	}
	if req.Icon != nil {
		cat.Icon = *req.Icon
	}
	if req.Color != nil {
		cat.Color = *req.Color
	}
	if req.ParentID != nil {
		cat.ParentID = req.ParentID
	}

	if err := h.catRepo.UpdateCategory(r.Context(), cat); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, _ := h.catRepo.GetCategory(r.Context(), catID)
	sendJSON(w, 0, http.StatusOK, "success", updated)
}

// DeleteCategory godoc
// @Summary 删除分类
// @Description 删除指定分类（其下的交易记录自动解除关联分类或转移）
// @Tags Categories & Tags
// @Accept json
// @Produce json
// @Param id path string true "分类 ID"
// @Success 200 {object} Response "删除成功"
// @Router /api/v1/categories/{id} [delete]
func (h *CategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request, catID string) {
	if err := h.catRepo.DeleteCategory(r.Context(), catID); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", nil)
}

// ListTags godoc
// @Summary 获取标签列表
// @Description 获取正交标签体系列表（如：报销、项目、专项活动等）
// @Tags Categories & Tags
// @Accept json
// @Produce json
// @Param book_id query string false "账本 ID"
// @Success 200 {object} Response{data=[]model.Tag} "标签列表"
// @Router /api/v1/tags [get]
func (h *CategoryHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	bookID := r.URL.Query().Get("book_id")
	tags, err := h.catRepo.ListTags(r.Context(), bookID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tags == nil {
		tags = []model.Tag{}
	}
	sendJSON(w, 0, http.StatusOK, "success", tags)
}

// CreateTag godoc
// @Summary 创建新标签
// @Description 创建用于跨分类打标的正交标签
// @Tags Categories & Tags
// @Accept json
// @Produce json
// @Param request body model.Tag true "标签信息"
// @Success 201 {object} Response{data=model.Tag} "创建成功"
// @Router /api/v1/tags [post]
func (h *CategoryHandler) CreateTag(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"id"`
		BookID string `json:"book_id"`
		Name   string `json:"name"`
		Color  string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		sendError(w, http.StatusBadRequest, "tag name required")
		return
	}
	if req.Color == "" {
		req.Color = "#5D6470"
	}

	tag := &model.Tag{
		ID:     req.ID,
		BookID: req.BookID,
		Name:   req.Name,
		Color:  req.Color,
	}

	if err := h.catRepo.CreateTag(r.Context(), tag); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sendJSON(w, 0, http.StatusCreated, "success", tag)
}

// DeleteTag godoc
// @Summary 删除标签
// @Description 移除指定标签
// @Tags Categories & Tags
// @Accept json
// @Produce json
// @Param id path string true "标签 ID"
// @Success 200 {object} Response "删除成功"
// @Router /api/v1/tags/{id} [delete]
func (h *CategoryHandler) DeleteTag(w http.ResponseWriter, r *http.Request, tagID string) {
	if err := h.catRepo.DeleteTag(r.Context(), tagID); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", nil)
}
