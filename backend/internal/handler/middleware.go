package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
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

	// Endpoints to probe on local UGOS gateway
	endpoints := []string{
		"http://127.0.0.1:9999/api/v1/user/info",
		"http://127.0.0.1:9999/api/v2/users/current",
		"http://127.0.0.1:80/api/v1/user/info",
		"http://127.0.0.1:80/api/v2/users/current",
	}

	client := &http.Client{Timeout: 800 * time.Millisecond}
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
	// Pad base64 if needed
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

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bypass auth for health check and static assets
		if r.URL.Path == "/api/v1/health" || r.URL.Path == "/health" {
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

		// 5. Fallback identification
		if userID == "" {
			if os.Getenv("STRICT_AUTH") == "1" {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(model.ApiResponse{
					Code:    401,
					Message: "unauthorized",
				})
				return
			}
			if sysUser := getFirstNonEmpty(os.Getenv("UGOS_USER"), os.Getenv("UGAPP_USER"), os.Getenv("USER"), os.Getenv("LOGNAME")); sysUser != "" && sysUser != "root" {
				userID = sysUser
				if userName == "" {
					userName = sysUser
				}
			} else {
				userID = "ugos_admin"
				if userName == "" {
					userName = "绿联管理员"
				}
			}
			if userType == "" {
				userType = "admin"
			}
		} else {
			if userName == "" {
				userName = userID
			}
			if userType == "" {
				userType = "admin"
			}
		}

		uCtx := &model.UserContext{
			ID:   userID,
			Name: userName,
			Type: userType,
		}

		ctx := context.WithValue(r.Context(), userContextKey, uCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserContext(r *http.Request) *model.UserContext {
	if val := r.Context().Value(userContextKey); val != nil {
		if u, ok := val.(*model.UserContext); ok {
			return u
		}
	}
	sysUser := getFirstNonEmpty(os.Getenv("UGOS_USER"), os.Getenv("UGAPP_USER"), os.Getenv("USER"), os.Getenv("LOGNAME"))
	if sysUser != "" && sysUser != "root" {
		return &model.UserContext{ID: sysUser, Name: sysUser, Type: "admin"}
	}
	return &model.UserContext{ID: "ugos_admin", Name: "绿联管理员", Type: "admin"}
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

