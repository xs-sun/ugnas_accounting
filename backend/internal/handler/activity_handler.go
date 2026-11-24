package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
	"com.xsguyu.accounting/backend/internal/service"
)

// recordActivity 是各 handler 共用的“写动态 + 发 SSE”入口。
// recorder 为空（旧的直接构造调用点）时退化为只发 SSE，与 SSE 接入前的行为一致；
// recorder 内部对写库失败只记日志，绝不让动态写入拖垮主流程。
func recordActivity(ctx context.Context, recorder *service.ActivityRecorder, hub *service.Hub, a model.BookActivity, change service.ChangeType) {
	if recorder != nil {
		recorder.Record(ctx, a, change)
		return
	}
	if hub != nil {
		hub.Publish(a.BookID, change, a.ActorName)
	}
}

// ActivityHandler 提供账本动态查询：GET /api/v1/books/:id/activities。
// 权限红线：只返回请求者本人的账本（含共享账本）的动态；非成员 403，未登录 401
// （未登录由中间件先拦；这里再验一次成员身份，双重保险）。
type ActivityHandler struct {
	activityRepo *repository.ActivityRepo
	bookRepo     *repository.BookRepo
}

func NewActivityHandler(activityRepo *repository.ActivityRepo, bookRepo *repository.BookRepo) *ActivityHandler {
	return &ActivityHandler{activityRepo: activityRepo, bookRepo: bookRepo}
}

func (h *ActivityHandler) ListActivities(w http.ResponseWriter, r *http.Request, bookID string) {
	uCtx := GetUserContext(r)
	if _, err := h.bookRepo.GetMemberRole(r.Context(), bookID, uCtx.ID); err != nil {
		sendError(w, http.StatusForbidden, "not a member of this book")
		return
	}

	q := r.URL.Query()
	limit := 20
	if lStr := q.Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	if limit > 100 {
		limit = 100
	}
	before := strings.TrimSpace(q.Get("before"))

	items, hasMore, err := h.activityRepo.List(r.Context(), bookID, limit, before)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []model.BookActivity{}
	}
	nextBefore := ""
	if hasMore && len(items) > 0 {
		nextBefore = items[len(items)-1].ID
	}
	sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
		"items":       items,
		"has_more":    hasMore,
		"next_before": nextBefore,
	})
}
