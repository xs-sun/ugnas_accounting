package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
	"com.xsguyu.accounting/backend/internal/service"
)

type contextKey string

const userContextKey contextKey = "userContext"

var (
	tokenUserCache = make(map[string]*model.UserContext)
	tokenCacheMu   sync.RWMutex
)

func fetchUserFromUGOSGateway(token string) *model.UserContext {
	if token == "" {
		return nil
	}
	tokenCacheMu.RLock()
	if u, ok := tokenUserCache[token]; ok {
		tokenCacheMu.RUnlock()
		return u
	}
	tokenCacheMu.RUnlock()

	endpoints := []string{
		"http://127.0.0.1:9999/api/v1/user/info",
		"http://127.0.0.1:9999/api/v2/users/current",
		"http://127.0.0.1:80/api/v1/user/info",
		"http://127.0.0.1:80/api/v2/users/current",
	}

	client := &http.Client{Timeout: 500 * time.Millisecond}
	for _, ep := range endpoints {
		req, err := http.NewRequest("GET", ep, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Ugreen-Ttk", token)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil || resp == nil {
			continue
		}
		if resp.StatusCode == http.StatusOK {
			var result struct {
				Code int `json:"code"`
				Data struct {
					UserID   string `json:"user_id"`
					UID      string `json:"uid"`
					ID       string `json:"id"`
					UserName string `json:"user_name"`
					Username string `json:"username"`
					Name     string `json:"name"`
					UserType string `json:"user_type"`
					Role     string `json:"role"`
					IsAdmin  bool   `json:"is_admin"`
				} `json:"data"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
				_ = resp.Body.Close()
				uid := getFirstNonEmpty(result.Data.UserID, result.Data.UID, result.Data.ID)
				uname := getFirstNonEmpty(result.Data.UserName, result.Data.Username, result.Data.Name)
				utype := getFirstNonEmpty(result.Data.UserType, result.Data.Role)
				if utype == "" && result.Data.IsAdmin {
					utype = "admin"
				}
				if uid != "" {
					uCtx := &model.UserContext{
						ID:   uid,
						Name: uname,
						Type: utype,
					}
					tokenCacheMu.Lock()
					tokenUserCache[token] = uCtx
					tokenCacheMu.Unlock()
					return uCtx
				}
			}
			_ = resp.Body.Close()
		} else {
			_ = resp.Body.Close()
		}
	}
	return nil
}

func parseJWTClaims(tokenStr string) map[string]interface{} {
	if tokenStr == "" {
		return nil
	}
	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	tokenStr = strings.TrimSpace(tokenStr)
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil
	}
	segment := parts[1]
	switch len(segment) % 4 {
	case 2:
		segment += "=="
	case 3:
		segment += "="
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payloadBytes, err = base64.URLEncoding.DecodeString(segment)
		if err != nil {
			payloadBytes, err = base64.StdEncoding.DecodeString(segment)
			if err != nil {
				return nil
			}
		}
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err == nil {
		return claims
	}
	return nil
}

func getFirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func getCookieVal(r *http.Request, name string) string {
	if c, err := r.Cookie(name); err == nil {
		return c.Value
	}
	return ""
}

func isDevFallbackAllowed() bool {
	if strict := strings.TrimSpace(os.Getenv("STRICT_AUTH")); strict == "1" {
		return false
	} else if strict == "0" {
		return true
	}
	appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	return appEnv == "development" || appEnv == "dev" || appEnv == "test"
}

func isPublicRoute(path string) bool {
	if path != "/api" && !strings.HasPrefix(path, "/api/") {
		return true
	}
	if path == "/api/v1/health" || path == "/health" {
		return true
	}
	if strings.HasPrefix(path, "/api/v1/auth/") {
		return true
	}
	// SSE 流：中间件跳过，鉴权由 StreamHandler 自己做（EventSource 发不了
	// Authorization 头，token 走 ?token= 查询参数）。
	if path == "/api/v1/stream" {
		return true
	}
	if strings.HasPrefix(path, "/api/v1/avatars/") {
		return true
	}
	if path == "/swagger" || strings.HasPrefix(path, "/swagger/") || path == "/api/swagger" || strings.HasPrefix(path, "/api/swagger/") {
		return true
	}
	return false
}

// AuthMiddleware preserves backward compatibility for callers without UserRepo/AuthService injection.
func AuthMiddleware(next http.Handler) http.Handler {
	return NewAuthMiddleware(nil, nil)(next)
}

// ExtractGatewayContext extracts UGOS Pro gateway user identity from headers, cookies, query params, JWT, or local gateway endpoints.
// Returns nil when no valid gateway user ID can be identified.
func ExtractGatewayContext(r *http.Request) *model.UserContext {
	// 1. Try Headers
	userID := getFirstNonEmpty(
		r.Header.Get("Ugreen-User-ID"),
		r.Header.Get("X-Ugreen-User-Id"),
		r.Header.Get("X-Ugreen-User-ID"),
		r.Header.Get("User-ID"),
		r.Header.Get("User-Id"),
		r.Header.Get("X-User-Id"),
		r.Header.Get("X-User-ID"),
		r.Header.Get("Ugreen-Uid"),
		r.Header.Get("Uid"),
	)
	userName := getFirstNonEmpty(
		r.Header.Get("Ugreen-User-Name"),
		r.Header.Get("X-Ugreen-User-Name"),
		r.Header.Get("User-Name"),
		r.Header.Get("User-name"),
		r.Header.Get("X-User-Name"),
		r.Header.Get("Username"),
		r.Header.Get("X-Username"),
	)
	userType := getFirstNonEmpty(
		r.Header.Get("Ugreen-User-Type"),
		r.Header.Get("X-Ugreen-User-Type"),
		r.Header.Get("User-Type"),
		r.Header.Get("User-type"),
		r.Header.Get("X-User-Type"),
		r.Header.Get("Role"),
		r.Header.Get("X-Role"),
	)

	token := getFirstNonEmpty(
		r.Header.Get("Ugreen-Ttk"),
		r.Header.Get("X-Ugreen-Ttk"),
		r.Header.Get("Authorization"),
		r.Header.Get("Token"),
		r.Header.Get("X-Token"),
	)
	if strings.Contains(token, "cat7_") {
		token = ""
	}

	// 2. Try Cookies if headers missing
	if userID == "" {
		userID = getFirstNonEmpty(
			getCookieVal(r, "Ugreen-User-ID"),
			getCookieVal(r, "user_id"),
			getCookieVal(r, "userId"),
			getCookieVal(r, "uid"),
			getCookieVal(r, "ugreen_user_id"),
		)
	}
	if userName == "" {
		userName = getFirstNonEmpty(
			getCookieVal(r, "Ugreen-User-Name"),
			getCookieVal(r, "username"),
			getCookieVal(r, "user_name"),
			getCookieVal(r, "name"),
		)
	}
	if token == "" {
		token = getFirstNonEmpty(
			getCookieVal(r, "Ugreen-Ttk"),
			getCookieVal(r, "token"),
			getCookieVal(r, "ttk"),
			getCookieVal(r, "ugreen_token"),
			getCookieVal(r, "ug_token"),
		)
	}

	// 3. Try URL query parameters
	if q := r.URL.Query(); q != nil {
		if userID == "" {
			userID = getFirstNonEmpty(
				q.Get("user_id"),
				q.Get("userId"),
				q.Get("uid"),
				q.Get("ugreen_user_id"),
			)
		}
		if userName == "" {
			userName = getFirstNonEmpty(
				q.Get("user_name"),
				q.Get("username"),
				q.Get("name"),
			)
		}
		if userType == "" {
			userType = getFirstNonEmpty(
				q.Get("user_type"),
				q.Get("userType"),
				q.Get("role"),
			)
		}
		if token == "" {
			token = getFirstNonEmpty(
				q.Get("token"),
				q.Get("ttk"),
				q.Get("ugreen_token"),
			)
		}
	}

	// 4. Decode JWT claims if token present
	if token != "" && (userID == "" || userName == "") {
		if claims := parseJWTClaims(token); claims != nil {
			if userID == "" {
				for _, k := range []string{"user_id", "userId", "uid", "sub", "id"} {
					if v, ok := claims[k].(string); ok && v != "" {
						userID = v
						break
					}
				}
			}
			if userName == "" {
				for _, k := range []string{"username", "user_name", "name", "nickname"} {
					if v, ok := claims[k].(string); ok && v != "" {
						userName = v
						break
					}
				}
			}
			if userType == "" {
				for _, k := range []string{"user_type", "userType", "role"} {
					if v, ok := claims[k].(string); ok && v != "" {
						userType = v
						break
					}
				}
				if isAdmin, ok := claims["is_admin"].(bool); ok && isAdmin && userType == "" {
					userType = "admin"
				}
			}
		}
	}

	// 4b. Query local UGOS Gateway with token if opaque session token
	if token != "" && (userID == "" || userName == "") {
		if u := fetchUserFromUGOSGateway(token); u != nil {
			if userID == "" {
				userID = u.ID
			}
			if userName == "" {
				userName = u.Name
			}
			if userType == "" {
				userType = u.Type
			}
		}
	}

	if userID == "" {
		return nil
	}
	userName = decodeGatewayDisplayName(userName)
	if userName == "" {
		userName = userID
	}
	if userType == "" {
		userType = "admin"
	}
	role := "user"
	if userType == "admin" {
		role = "admin"
	}
	return &model.UserContext{
		ID:   userID,
		Name: userName,
		Type: userType,
		Role: role,
	}
}

func decodeGatewayDisplayName(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "%") {
		return raw
	}
	if unescaped, err := url.PathUnescape(raw); err == nil && utf8.ValidString(unescaped) {
		return strings.TrimSpace(unescaped)
	}
	return raw
}

func NewAuthMiddleware(userRepo *repository.UserRepo, authSvc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isPublicRoute(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			if r.ContentLength >= 5*1024*1024 && !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_ = json.NewEncoder(w).Encode(model.ApiResponse{
					Code:    413,
					Message: "payload too large",
				})
				return
			}

			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authSvc != nil && strings.Contains(authHeader, "cat7_") {
				rawTok := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
				if strings.HasPrefix(rawTok, "cat7_") {
					claims, err := authSvc.VerifyToken(rawTok)
					if err != nil {
						sendError(w, http.StatusUnauthorized, "invalid or expired token")
						return
					}
					if userRepo != nil {
						u, err := userRepo.GetUserByID(r.Context(), claims.UserID)
						if err != nil {
							sendError(w, http.StatusUnauthorized, "user not found")
							return
						}
						if !u.IsActive {
							sendError(w, http.StatusForbidden, "account disabled")
							return
						}
						userRepo.TouchLastActive(r.Context(), u.ID)
						uType := "users"
						if u.Role == "admin" {
							uType = "admin"
						}
						uCtx := &model.UserContext{
							ID:    u.ID,
							Name:  u.Name,
							Type:  uType,
							Email: u.Email,
							Role:  u.Role,
						}
						ctx := context.WithValue(r.Context(), userContextKey, uCtx)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
			}

			uCtx := ExtractGatewayContext(r)

			// Fallback identification (development only; production returns 401 when no gateway/token auth is present)
			if uCtx == nil {
				if !isDevFallbackAllowed() {
					sendError(w, http.StatusUnauthorized, "unauthorized")
					return
				}
				var userID, userName string
				if sysUser := getFirstNonEmpty(os.Getenv("UGOS_USER"), os.Getenv("UGAPP_USER"), os.Getenv("USER"), os.Getenv("LOGNAME")); sysUser != "" && sysUser != "root" {
					userID = sysUser
					userName = sysUser
				} else {
					userID = "ugos_admin"
					userName = "绿联管理员"
				}
				uCtx = &model.UserContext{
					ID:   userID,
					Name: userName,
					Type: "admin",
					Role: "admin",
				}
			}

			if userRepo != nil {
				if dbUser, err := userRepo.UpsertUserFromContext(r.Context(), uCtx); err == nil && dbUser != nil {
					if !dbUser.IsActive {
						sendError(w, http.StatusForbidden, "account disabled")
						return
					}
					uCtx.ID = dbUser.ID
					uCtx.Name = dbUser.Name
					uCtx.Email = dbUser.Email
					uCtx.Role = dbUser.Role
					if dbUser.Role == "admin" {
						uCtx.Type = "admin"
					} else {
						uCtx.Type = "users"
					}
				}
			}

			ctx := context.WithValue(r.Context(), userContextKey, uCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserContext(r *http.Request) *model.UserContext {
	if val := r.Context().Value(userContextKey); val != nil {
		if u, ok := val.(*model.UserContext); ok {
			return u
		}
	}
	sysUser := getFirstNonEmpty(os.Getenv("UGOS_USER"), os.Getenv("UGAPP_USER"), os.Getenv("USER"), os.Getenv("LOGNAME"))
	if sysUser != "" && sysUser != "root" {
		return &model.UserContext{ID: sysUser, Name: sysUser, Type: "admin", Role: "admin"}
	}
	return &model.UserContext{ID: "ugos_admin", Name: "绿联管理员", Type: "admin", Role: "admin"}
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Token, Ugreen-User-ID, Ugreen-User-Name, Ugreen-User-Type, Ugreen-Ttk, X-Ugreen-User-Id, X-Ugreen-User-Name, X-Ugreen-User-Type, X-Ugreen-Ttk, X-User-Id, User-Id, X-User-Name, User-Name, X-User-Type, User-Type, X-Requested-With")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
