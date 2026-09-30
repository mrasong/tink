package api

import (
	"context"
	"errors"
	"github.com/mrasong/tink/server/internal/i18n"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mrasong/tink/server/internal/auth"
	"github.com/mrasong/tink/server/internal/store"
)

type contextKey string

const (
	ContextKeySecretKey contextKey = "secret_key"
	ContextKeySession   contextKey = "session"
	ContextKeyIsUDS     contextKey = "is_uds"

	// SessionCookieName 浏览器会话 cookie (httpOnly，与长期 Secret Key 分离)
	SessionCookieName = "tink_session"
	// CSRFHeaderName 会话鉴权下非 GET 请求必须携带的自定义头 (跨站无法携带)
	CSRFHeaderName = "X-Tink-CSRF"
)

// UDSMarkerMiddleware 由 unix socket 专属的 http.Server 挂载，
// 为本listener的全部请求显式注入 UDS 标记 (替代 RemoteAddr 字符串嗅探)
func UDSMarkerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), ContextKeyIsUDS, true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isUDSRequest(r *http.Request) bool {
	v, _ := r.Context().Value(ContextKeyIsUDS).(bool)
	return v
}

// AuthMiddleware 认证中间件。凭证优先级：
// 1. UDS 本机连接 -> 直接 admin；2. Authorization Bearer Secret Key (机器客户端)；
// 3. tink_session httpOnly cookie (浏览器会话，附带 CSRF 头校验)
func AuthMiddleware(s *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isUDSRequest(r) {
				udsKey := &store.SecretKey{
					ID:      "uds_admin",
					Name:    "UDS Admin",
					Role:    "admin",
					Enabled: 1,
				}
				ctx := context.WithValue(r.Context(), ContextKeySecretKey, udsKey)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// 机器通道：Bearer Secret Key
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				key, err := auth.AuthenticateKey(s, authHeader)
				if err != nil {
					JSONError(w, r, http.StatusUnauthorized, i18n.MsgInvalidSecretKey)
					return
				}
				ctx := context.WithValue(r.Context(), ContextKeySecretKey, key)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// 浏览器通道：httpOnly 会话 cookie
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				JSONError(w, r, http.StatusUnauthorized, i18n.MsgMissingAuthHeader)
				return
			}
			sess, err := s.GetSession(store.HashToken(cookie.Value))
			if err != nil {
				if !errors.Is(err, store.ErrSessionNotFound) {
					log.Printf("[Auth] session lookup error: %v", err)
				}
				JSONError(w, r, http.StatusUnauthorized, i18n.MsgInvalidSecretKey)
				return
			}
			key, err := s.GetKeyByID(sess.KeyID)
			if err != nil || key.Enabled != 1 {
				_ = s.DeleteSession(sess.TokenHash)
				JSONError(w, r, http.StatusUnauthorized, i18n.MsgInvalidSecretKey)
				return
			}

			// CSRF 防线：cookie 会话的非 GET 请求必须携带自定义头 (跨站请求无法附带)
			if !isSafeMethod(r.Method) && r.Header.Get(CSRFHeaderName) != "1" {
				JSONError(w, r, http.StatusForbidden, i18n.MsgCSRFCheckFailed)
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeySecretKey, key)
			ctx = context.WithValue(ctx, ContextKeySession, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// RequireAdminRole 中间件：限制仅 Admin 角色可访问管理接口
func RequireAdminRole() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key, ok := r.Context().Value(ContextKeySecretKey).(*store.SecretKey)
			if !ok || key == nil || key.Role != "admin" {
				JSONError(w, r, http.StatusForbidden, i18n.MsgAdminRoleRequired)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimiter 简单的滑动窗口限流器 (如 120 次/分钟)
type RateLimiter struct {
	mu     sync.Mutex
	limits map[string][]time.Time
	limit  int
	window time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limits: make(map[string][]time.Time),
		limit:  limit,
		window: window,
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// 过滤过期时间戳；若窗口内已无记录则整键删除，防止 map 无限增长
	times := rl.limits[key]
	valid := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) == 0 {
		delete(rl.limits, key)
		rl.limits[key] = []time.Time{now}
		return true
	}
	if len(valid) >= rl.limit {
		rl.limits[key] = valid
		return false
	}

	rl.limits[key] = append(valid, now)
	return true
}

// purgeStale 定期整体清理陈旧条目
func (rl *RateLimiter) purgeStale() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-rl.window)
	for key, times := range rl.limits {
		var latest time.Time
		for _, t := range times {
			if t.After(latest) {
				latest = t
			}
		}
		if latest.Before(cutoff) {
			delete(rl.limits, key)
		}
	}
}

// rateLimitKey 提取限流键：凭证 (Bearer 头 / 会话 cookie) 优先，回落到客户端 IP
func rateLimitKey(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		return "cred:" + store.HashToken(h)
	}
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		return "cred:" + store.HashToken(c.Value)
	}
	return "ip:" + clientIP(r.RemoteAddr)
}

func clientIP(remoteAddr string) string {
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}
	return remoteAddr
}

// RateLimitMiddleware 基于凭证或 IP 的常规限流 (作用于已通过鉴权的流量)
func RateLimitMiddleware(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isUDSRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			if !rl.Allow(rateLimitKey(r)) {
				JSONError(w, r, http.StatusTooManyRequests, i18n.MsgRateLimitExceeded)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// AuthFailLimiter 按 IP 统计鉴权失败次数，堵住凭证爆破时"每次尝试换一个新 key 绕过限流"的缺口
type AuthFailLimiter struct {
	mu     sync.Mutex
	fails  map[string][]time.Time
	limit  int
	window time.Duration
}

func NewAuthFailLimiter(limit int, window time.Duration) *AuthFailLimiter {
	l := &AuthFailLimiter{
		fails:  make(map[string][]time.Time),
		limit:  limit,
		window: window,
	}
	ticker := time.NewTicker(window)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			l.purgeStale()
		}
	}()
	return l
}

func (l *AuthFailLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-l.window)
	recent := l.fails[ip][:0]
	for _, t := range l.fails[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) == 0 {
		delete(l.fails, ip)
	} else {
		l.fails[ip] = recent
	}
	return len(recent) < l.limit
}

func (l *AuthFailLimiter) Record(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[ip] = append(l.fails[ip], time.Now())
}

func (l *AuthFailLimiter) purgeStale() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-l.window)
	for ip, times := range l.fails {
		var latest time.Time
		for _, t := range times {
			if t.After(latest) {
				latest = t
			}
		}
		if latest.Before(cutoff) {
			delete(l.fails, ip)
		}
	}
}

// statusRecorder 捕获下游写出的状态码
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Flush() {
	if f, ok := sr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// AuthFailLimitMiddleware 包裹鉴权环节：先查 IP 失败额度，401 计入失败。
// 放在 AuthMiddleware 外层，使未通过鉴权的请求也被限流。
func AuthFailLimitMiddleware(fl *AuthFailLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isUDSRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			ip := clientIP(r.RemoteAddr)
			if !fl.Allow(ip) {
				JSONError(w, r, http.StatusTooManyRequests, i18n.MsgTooManyFailedAttempts)
				return
			}
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			if rec.status == http.StatusUnauthorized {
				fl.Record(ip)
			}
		})
	}
}

// LoggerMiddleware 打印请求日志（遵守规范：不输出 Authorization / Token 明文）
func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		log.Printf("[HTTP] %s %s from %s in %v", r.Method, r.URL.Path, r.RemoteAddr, duration)
	})
}
