package api

import (
	"encoding/json"
	"github.com/mrasong/tink/server/internal/i18n"
	"net/http"
	"strings"

	"github.com/mrasong/tink/server/internal/auth"
	"github.com/mrasong/tink/server/internal/store"
)

// Login POST /api/v1/login
// 用 Secret Key 换取短期 httpOnly 会话 cookie；Secret Key 本身不进入浏览器存储
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONError(w, r, http.StatusMethodNotAllowed, i18n.MsgMethodNotAllowed)
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, r, http.StatusBadRequest, i18n.MsgInvalidJSON)
		return
	}

	key, err := auth.AuthenticateKey(h.store, req.Token)
	if err != nil {
		JSONError(w, r, http.StatusUnauthorized, i18n.MsgInvalidSecretKey)
		return
	}

	rawToken, err := store.NewSessionToken()
	if err != nil {
		JSONError(w, r, http.StatusInternalServerError, i18n.MsgCreateSessionFailed)
		return
	}
	hash := store.HashToken(rawToken)
	if _, err := h.store.CreateSession(hash, key.ID); err != nil {
		JSONError(w, r, http.StatusInternalServerError, i18n.MsgCreateSessionFailed)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    rawToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPSRequest(r),
		SameSite: http.SameSiteLaxMode,
		// 不设 Max-Age/Expires: 关闭浏览器即失效，服务端另有 8h 空闲/24h 绝对上限
	})

	JSONSuccess(w, http.StatusOK, map[string]any{
		"id":         key.ID,
		"name":       key.Name,
		"role":       key.Role,
		"is_master":  key.IsAdmin(),
		"enabled":    key.Enabled,
		"created_at": key.CreatedAt,
	})
}

// Logout POST /api/v1/logout 吊销当前会话并使 cookie 失效
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONError(w, r, http.StatusMethodNotAllowed, i18n.MsgMethodNotAllowed)
		return
	}
	if sess, ok := r.Context().Value(ContextKeySession).(*store.Session); ok && sess != nil {
		_ = h.store.DeleteSession(sess.TokenHash)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPSRequest(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	JSONSuccess(w, http.StatusOK, map[string]any{"logout": true})
}

// isHTTPSRequest 直连 TLS 或经可信反代 (Caddy 会带 X-Forwarded-Proto) 时给 cookie 加 Secure
func isHTTPSRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// SecurityHeadersMiddleware 基础安全响应头。
// CSP script-src 'self'：构建产物无内联脚本，注入的 <script> 将被浏览器拒绝执行
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	csp := strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data:",
		"font-src 'self' data:",
		"connect-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"object-src 'none'",
	}, "; ")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
