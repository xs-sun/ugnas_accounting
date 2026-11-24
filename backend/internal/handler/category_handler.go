package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
	"com.xsguyu.accounting/backend/internal/service"
)

type CategoryHandler struct {
	catRepo  *repository.CategoryRepo
	hub      *service.Hub
	recorder *service.ActivityRecorder
}

func NewCategoryHandler(catRepo *repository.CategoryRepo, hub *service.Hub, recorder *service.ActivityRecorder) *CategoryHandler {
	return &CategoryHandler{catRepo: catRepo, hub: hub, recorder: recorder}
}

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

func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		ID        string  `json:"id"`
		BookID    string  `json:"book_id"`
		Type      string  `json:"type"`
		Name      string  `json:"name"`
		Icon      string  `json:"icon"`
		Color     string  `json:"color"`
		BgColor   string  `json:"bg_color"`
		ParentID  *string `json:"parent_id"`
		IsCustom  *bool   `json:"is_custom"`
		SortOrder int     `json:"sort_order"`
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
		req.Color = "#DE8A1E"
	}
	if req.BgColor == "" {
		req.BgColor = "#FFF6E8"
	}
	isCustom := true
	if req.IsCustom != nil {
		isCustom = *req.IsCustom
	}

	dup, err := h.catRepo.ExistsDuplicate(r.Context(), req.BookID, req.Type, req.ParentID, req.Name)
	if err == nil && dup {
		sendError(w, http.StatusConflict, "category already exists in this book")
		return
	}

	cat := &model.Category{
		ID:        req.ID,
		BookID:    req.BookID,
		Type:      req.Type,
		Name:      req.Name,
		Icon:      req.Icon,
		Color:     req.Color,
		BgColor:   req.BgColor,
		ParentID:  req.ParentID,
		IsCustom:  isCustom,
		SortOrder: req.SortOrder,
	}

	if err := h.catRepo.CreateCategory(r.Context(), cat); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
		BookID:    req.BookID,
		UserID:    uCtx.ID,
		ActorName: uCtx.Name,
		Action:    model.ActivityCategoryChanged,
		Summary:   fmt.Sprintf("新增了分类「%s」", cat.Name),
	}, service.ChangeCategory)

	created, err := h.catRepo.GetCategory(r.Context(), cat.ID)
	if err != nil {
		sendJSON(w, 0, http.StatusCreated, "success", cat)
		return
	}
	sendJSON(w, 0, http.StatusCreated, "success", created)
}

func (h *CategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request, catID string) {
	uCtx := GetUserContext(r)
	cat, err := h.catRepo.GetCategory(r.Context(), catID)
	if err != nil {
		sendError(w, http.StatusNotFound, "category not found")
		return
	}

	var req struct {
		Name      *string `json:"name"`
		Icon      *string `json:"icon"`
		Color     *string `json:"color"`
		BgColor   *string `json:"bg_color"`
		ParentID  *string `json:"parent_id"`
		SortOrder *int    `json:"sort_order"`
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
	if req.BgColor != nil {
		cat.BgColor = *req.BgColor
	}
	if req.ParentID != nil {
		cat.ParentID = req.ParentID
	}
	if req.SortOrder != nil {
		cat.SortOrder = *req.SortOrder
	}

	if err := h.catRepo.UpdateCategory(r.Context(), cat); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
		BookID:    cat.BookID,
		UserID:    uCtx.ID,
		ActorName: uCtx.Name,
		Action:    model.ActivityCategoryChanged,
		Summary:   fmt.Sprintf("更新了分类「%s」", cat.Name),
	}, service.ChangeCategory)

	updated, _ := h.catRepo.GetCategory(r.Context(), catID)
	sendJSON(w, 0, http.StatusOK, "success", updated)
}

func (h *CategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request, catID string) {
	uCtx := GetUserContext(r)
	cat, err := h.catRepo.GetCategory(r.Context(), catID)
	if err != nil {
		sendError(w, http.StatusNotFound, "category not found")
		return
	}
	if err := h.catRepo.DeleteCategory(r.Context(), catID); err != nil {
		if err == repository.ErrNotFound {
			sendError(w, http.StatusNotFound, "category not found")
			return
		}
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
		BookID:    cat.BookID,
		UserID:    uCtx.ID,
		ActorName: uCtx.Name,
		Action:    model.ActivityCategoryChanged,
		Summary:   fmt.Sprintf("删除了分类「%s」", cat.Name),
	}, service.ChangeCategory)
	sendJSON(w, 0, http.StatusOK, "success", nil)
}

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

func (h *CategoryHandler) DeleteTag(w http.ResponseWriter, r *http.Request, tagID string) {
	if err := h.catRepo.DeleteTag(r.Context(), tagID); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", nil)
}
