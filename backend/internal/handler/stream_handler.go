package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
	"com.xsguyu.accounting/backend/internal/service"
)

// StreamHandler 提供 SSE 实时同步：GET /api/v1/stream?bookId=...&token=...
//
// 鉴权说明：EventSource 发不了 Authorization 自定义头，所以 token 允许放在
// ?token= 查询参数里（与 Authorization 头等价；本服务无访问日志中间件，
// token 不会落盘）。网关用户仍可走已有的 UGOS 身份头/cookie。
// 本路由在中间件里被列为公开路由（见 isPublicRoute），鉴权完全由本 handler
// 自己做：无有效身份 -> 401；非该账本成员 -> 403。开发 fallback 在这里
// 不生效，没有凭证就是 401。
type StreamHandler struct {
	hub      *service.Hub
	bookRepo *repository.BookRepo
	userRepo *repository.UserRepo
	authSvc  *service.AuthService
}

func NewStreamHandler(hub *service.Hub, bookRepo *repository.BookRepo, userRepo *repository.UserRepo, authSvc *service.AuthService) *StreamHandler {
	return &StreamHandler{
		hub:      hub,
		bookRepo: bookRepo,
		userRepo: userRepo,
		authSvc:  authSvc,
	}
}

// resolveStreamUser 按顺序解析流请求的身份：
//  1. Authorization: Bearer cat7_... 头
//  2. ?token=cat7_... 查询参数
//  3. UGOS 网关身份（头/cookie/URL 参数，与中间件同一套 ExtractGatewayContext）
func (h *StreamHandler) resolveStreamUser(r *http.Request) *model.UserContext {
	verifyToken := func(raw string) *model.UserContext {
		raw = strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
		if !strings.HasPrefix(raw, "cat7_") || h.authSvc == nil {
			return nil
		}
		claims, err := h.authSvc.VerifyToken(raw)
		if err != nil {
			return nil
		}
		if h.userRepo == nil {
			return nil
		}
		u, err := h.userRepo.GetUserByID(r.Context(), claims.UserID)
		if err != nil || u == nil || !u.IsActive {
			return nil
		}
		uType := "users"
		if u.Role == "admin" {
			uType = "admin"
		}
		return &model.UserContext{
			ID:    u.ID,
			Name:  u.Name,
			Type:  uType,
			Email: u.Email,
			Role:  u.Role,
		}
	}

	if u := verifyToken(r.Header.Get("Authorization")); u != nil {
		return u
	}
	if u := verifyToken(r.URL.Query().Get("token")); u != nil {
		return u
	}

	if gwCtx := ExtractGatewayContext(r); gwCtx != nil {
		if h.userRepo != nil {
			// 与中间件一致：把网关身份映射到库里的用户 ID，成员关系是按这个 ID 存的。
			if dbUser, err := h.userRepo.UpsertUserFromContext(r.Context(), gwCtx); err == nil && dbUser != nil {
				if !dbUser.IsActive {
					return nil
				}
				gwCtx.ID = dbUser.ID
				gwCtx.Name = dbUser.Name
				gwCtx.Email = dbUser.Email
				gwCtx.Role = dbUser.Role
			}
		}
		return gwCtx
	}
	return nil
}

func writeSSEEvent(rw *bufio.ReadWriter, ev service.LedgerEvent) error {
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(rw, "id: %d\nevent: %s\ndata: %s\n\n", ev.Seq, ev.Type, payload); err != nil {
		return err
	}
	return rw.Flush()
}

// Stream 建立一条 SSE 长连接。关键点：http.Server 配了 WriteTimeout: 30s，
// 普通 ResponseWriter 写会在 30 秒被掐断，所以这里 hijack 连接后自己写，
// 劫持后服务端不再对该连接施加任何超时；再配合 20 秒心跳 + 前端重连兜底。
func (h *StreamHandler) Stream(w http.ResponseWriter, r *http.Request) {
	bookID := strings.TrimSpace(r.URL.Query().Get("bookId"))
	if bookID == "" {
		bookID = strings.TrimSpace(r.URL.Query().Get("book_id"))
	}
	if bookID == "" {
		sendError(w, http.StatusBadRequest, "bookId required")
		return
	}

	uCtx := h.resolveStreamUser(r)
	if uCtx == nil {
		sendError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if _, err := h.bookRepo.GetMemberRole(r.Context(), bookID, uCtx.ID); err != nil {
		sendError(w, http.StatusForbidden, "not a member of this book")
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		sendError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return
	}
	defer conn.Close()

	// CORS 头是中间件在普通 ResponseWriter 上设置的，劫持后不会自动发出，这里补一份。
	if _, err := fmt.Fprintf(rw, "HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nCache-Control: no-cache\r\nConnection: keep-alive\r\nX-Accel-Buffering: no\r\nAccess-Control-Allow-Origin: *\r\n\r\n"); err != nil {
		return
	}
	if err := rw.Flush(); err != nil {
		return
	}

	ch, unsub := h.hub.Subscribe(bookID)
	defer unsub()

	// 初始事件：带当前序号，客户端据此知道自己连上时账本的版本；重连后也会重新发一次。
	if err := writeSSEEvent(rw, service.LedgerEvent{
		Type:   "connected",
		BookID: bookID,
		Seq:    h.hub.Seq(bookID),
		At:     time.Now(),
	}); err != nil {
		return
	}

	// 断连检测：客户端只发最初的 GET，之后不再发数据；读到任何错误（对端关闭/RST）
	// 即视为断开。读操作走劫持后返回的 bufio，避免丢掉已缓冲的数据。
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, err := rw.ReadByte(); err != nil {
				return
			}
		}
	}()

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-done:
			return
		case <-r.Context().Done():
			return
		case ev := <-ch:
			if err := writeSSEEvent(rw, ev); err != nil {
				return
			}
		case <-heartbeat.C:
			if _, err := rw.WriteString(": ping\n\n"); err != nil {
				return
			}
			if err := rw.Flush(); err != nil {
				return
			}
		}
	}
}
