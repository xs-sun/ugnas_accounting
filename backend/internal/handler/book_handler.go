package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
	"com.xsguyu.accounting/backend/internal/service"
	"github.com/google/uuid"
)

type BookHandler struct {
	bookRepo *repository.BookRepo
	userRepo *repository.UserRepo
	authSvc  *service.AuthService
	hub      *service.Hub
	recorder *service.ActivityRecorder
}

func NewBookHandler(bookRepo *repository.BookRepo, userRepo *repository.UserRepo, authSvc *service.AuthService, hub *service.Hub, recorder *service.ActivityRecorder) *BookHandler {
	return &BookHandler{
		bookRepo: bookRepo,
		userRepo: userRepo,
		authSvc:  authSvc,
		hub:      hub,
		recorder: recorder,
	}
}

func (h *BookHandler) ListBooks(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	books, err := h.bookRepo.ListBooks(r.Context(), uCtx.ID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(books) == 0 {
		if err := h.bookRepo.EnsureDefaultBookForUser(r.Context(), uCtx.ID, uCtx.Name); err == nil {
			books, _ = h.bookRepo.ListBooks(r.Context(), uCtx.ID)
		}
	}
	if books == nil {
		books = []model.Book{}
	}
	sendJSON(w, 0, http.StatusOK, "success", books)
}

func (h *BookHandler) CreateBook(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Currency    string `json:"currency"`
		Description string `json:"description"`
		Color       string `json:"color"`
		Budget      int64  `json:"budget"`
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
	if req.Color == "" {
		req.Color = "#DE8A1E"
	}
	if req.Budget <= 0 {
		req.Budget = 450000
	}

	book := &model.Book{
		ID:          req.ID,
		Name:        req.Name,
		Currency:    req.Currency,
		Description: req.Description,
		Color:       req.Color,
		Budget:      req.Budget,
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

func (h *BookHandler) GetBook(w http.ResponseWriter, r *http.Request, bookID string) {
	uCtx := GetUserContext(r)
	book, err := h.bookRepo.GetBook(r.Context(), bookID, uCtx.ID)
	if err != nil {
		sendError(w, http.StatusNotFound, "book not found")
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", book)
}

func (h *BookHandler) UpdateBook(w http.ResponseWriter, r *http.Request, bookID string) {
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Currency    *string `json:"currency"`
		Color       *string `json:"color"`
		Budget      *int64  `json:"budget"`
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
	oldName := book.Name

	if req.Name != nil {
		book.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		book.Description = *req.Description
	}
	if req.Currency != nil {
		book.Currency = *req.Currency
	}
	if req.Color != nil && strings.TrimSpace(*req.Color) != "" {
		book.Color = strings.TrimSpace(*req.Color)
	}
	if req.Budget != nil && *req.Budget >= 0 {
		book.Budget = *req.Budget
	}

	if err := h.bookRepo.UpdateBook(r.Context(), book); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	action := model.ActivityBookUpdated
	summary := fmt.Sprintf("更新了账本「%s」", book.Name)
	if req.Name != nil && book.Name != oldName {
		action = model.ActivityBookRenamed
		summary = fmt.Sprintf("把账本改名为「%s」", book.Name)
	}
	recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
		BookID:    bookID,
		UserID:    uCtx.ID,
		ActorName: uCtx.Name,
		Action:    action,
		Summary:   summary,
	}, service.ChangeBook)

	updated, _ := h.bookRepo.GetBook(r.Context(), bookID, uCtx.ID)
	sendJSON(w, 0, http.StatusOK, "success", updated)
}

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
	h.hub.Publish(bookID, service.ChangeBook, uCtx.Name)
	sendJSON(w, 0, http.StatusOK, "success", nil)
}

func (h *BookHandler) ListMembers(w http.ResponseWriter, r *http.Request, bookID string) {
	uCtx := GetUserContext(r)
	members, err := h.bookRepo.ListMembersForUser(r.Context(), bookID, uCtx.ID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if members == nil {
		members = []model.BookMember{}
	}
	sendJSON(w, 0, http.StatusOK, "success", members)
}

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
		Avatar   string `json:"avatar"`
		Color    string `json:"color"`
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
		Avatar:   req.Avatar,
		Color:    req.Color,
	}

	if err := h.bookRepo.AddMember(r.Context(), member); err != nil {
		if err == repository.ErrConflict {
			sendError(w, http.StatusConflict, "member already exists")
			return
		}
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
		BookID:    bookID,
		UserID:    uCtx.ID,
		ActorName: uCtx.Name,
		Action:    model.ActivityMemberJoined,
		Summary:   fmt.Sprintf("邀请了「%s」加入账本", member.UserName),
	}, service.ChangeMember)

	sendJSON(w, 0, http.StatusCreated, "success", map[string]interface{}{
		"book_id": bookID,
		"user_id": req.UserID,
		"role":    req.Role,
	})
}

func (h *BookHandler) InviteByEmail(w http.ResponseWriter, r *http.Request, bookID string) {
	uCtx := GetUserContext(r)
	role, err := h.bookRepo.GetMemberRole(r.Context(), bookID, uCtx.ID)
	if err != nil || role != "owner" {
		sendError(w, http.StatusForbidden, "only owner can invite members")
		return
	}
	book, err := h.bookRepo.GetBook(r.Context(), bookID, uCtx.ID)
	if err != nil {
		sendError(w, http.StatusNotFound, "book not found")
		return
	}

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || !strings.Contains(email, "@") {
		sendError(w, http.StatusBadRequest, "valid email required")
		return
	}
	if req.Role == "" {
		req.Role = "editor"
	}

	member := &model.BookMember{
		BookID: bookID,
		Role:   req.Role,
		Color:  "#3E8E63",
		Status: "invited",
	}

	if targetUser, uErr := h.userRepo.GetUserByEmail(r.Context(), email); uErr == nil && targetUser != nil {
		member.UserID = targetUser.ID
		member.UserName = targetUser.Name
		member.Avatar = targetUser.Avatar
		member.Status = "active"
	} else {
		member.UserID = "inv_" + uuid.NewString()[:8]
		member.UserName = strings.SplitN(email, "@", 2)[0]
		member.Avatar = "/avatars/cat-2.jpg"
	}

	if err := h.bookRepo.AddMember(r.Context(), member); err != nil {
		if err == repository.ErrConflict {
			sendError(w, http.StatusConflict, "member already exists in book")
			return
		}
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	smtpSent := false
	if h.authSvc != nil {
		sent, err := h.authSvc.SendBookInviteEmail(r.Context(), email, uCtx.Name, book.Name, book.InviteCode)
		if err == nil {
			smtpSent = sent
		}
	}

	sendJSON(w, 0, http.StatusCreated, "success", map[string]interface{}{
		"member":      member,
		"invite_code": book.InviteCode,
		"smtp_sent":   smtpSent,
	})
}

func (h *BookHandler) AddVirtualMember(w http.ResponseWriter, r *http.Request, bookID string) {
	uCtx := GetUserContext(r)
	role, err := h.bookRepo.GetMemberRole(r.Context(), bookID, uCtx.ID)
	if err != nil || role != "owner" {
		sendError(w, http.StatusForbidden, "only owner can add virtual members")
		return
	}

	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		sendError(w, http.StatusBadRequest, "name required")
		return
	}
	if req.Color == "" {
		req.Color = "#3E8E63"
	}

	member := &model.BookMember{
		BookID:    bookID,
		UserID:    "virt_" + uuid.NewString()[:8],
		UserName:  name,
		Role:      "editor",
		Avatar:    service.RandomCatAvatar(),
		Color:     req.Color,
		IsVirtual: true,
		Status:    "active",
	}

	if err := h.bookRepo.AddMember(r.Context(), member); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
		BookID:    bookID,
		UserID:    uCtx.ID,
		ActorName: uCtx.Name,
		Action:    model.ActivityMemberJoined,
		Summary:   fmt.Sprintf("添加了虚拟成员「%s」", member.UserName),
	}, service.ChangeMember)

	sendJSON(w, 0, http.StatusCreated, "success", member)
}

func (h *BookHandler) JoinByCode(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	var req struct {
		InviteCode string `json:"invite_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid json")
		return
	}
	code := strings.ToUpper(strings.TrimSpace(req.InviteCode))
	if code == "" {
		sendError(w, http.StatusBadRequest, "invite_code required")
		return
	}

	book, err := h.bookRepo.GetBookByInviteCode(r.Context(), code)
	if err != nil {
		sendError(w, http.StatusNotFound, "invalid invite code")
		return
	}

	if _, roleErr := h.bookRepo.GetMemberRole(r.Context(), book.ID, uCtx.ID); roleErr != nil {
		member := &model.BookMember{
			BookID:   book.ID,
			UserID:   uCtx.ID,
			UserName: uCtx.Name,
			Role:     "editor",
			Status:   "active",
		}
		if err := h.bookRepo.AddMember(r.Context(), member); err != nil && err != repository.ErrConflict {
			sendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
			BookID:    book.ID,
			UserID:    uCtx.ID,
			ActorName: uCtx.Name,
			Action:    model.ActivityMemberJoined,
			Summary:   "通过邀请码加入了账本",
		}, service.ChangeMember)
	}

	fullBook, err := h.bookRepo.GetBook(r.Context(), book.ID, uCtx.ID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", fullBook)
}

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

	// 先取名字再删，动态里写得出被移除的是谁。
	targetName := targetUserID
	if members, err := h.bookRepo.ListMembers(r.Context(), bookID); err == nil {
		for _, m := range members {
			if m.UserID == targetUserID {
				targetName = m.UserName
				break
			}
		}
	}

	if err := h.bookRepo.RemoveMember(r.Context(), bookID, targetUserID); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recordActivity(r.Context(), h.recorder, h.hub, model.BookActivity{
		BookID:    bookID,
		UserID:    uCtx.ID,
		ActorName: uCtx.Name,
		Action:    model.ActivityMemberRemoved,
		Summary:   fmt.Sprintf("移除了成员「%s」", targetName),
	}, service.ChangeMember)
	sendJSON(w, 0, http.StatusOK, "success", nil)
}
