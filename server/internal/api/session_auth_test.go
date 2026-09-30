package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// loginAndCookie 调用 /api/v1/login 并返回会话 cookie
func loginAndCookie(t *testing.T, handler http.Handler, token string) *http.Cookie {
	t.Helper()
	body := `{"token":"` + token + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "tink_session" {
			return c
		}
	}
	t.Fatalf("no tink_session cookie in Set-Cookie: %v", rec.Header().Values("Set-Cookie"))
	return nil
}

func TestSessionCookieLoginFlow(t *testing.T) {
	handler, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. 登录签发 httpOnly 会话 cookie
	cookie := loginAndCookie(t, handler, "sk-tink-testtoken123")
	if !cookie.HttpOnly {
		t.Fatalf("session cookie must be HttpOnly")
	}
	if cookie.MaxAge != 0 || !cookie.Expires.IsZero() {
		t.Fatalf("session cookie should be browser-session scoped (no Max-Age/Expires)")
	}

	// 2. 带 cookie 访问 /me
	reqMe := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	reqMe.AddCookie(cookie)
	recMe := httptest.NewRecorder()
	handler.ServeHTTP(recMe, reqMe)
	if recMe.Code != http.StatusOK {
		t.Fatalf("GET /me with session cookie expected 200, got %d: %s", recMe.Code, recMe.Body.String())
	}
	var meEnv struct {
		Code int `json:"code"`
		Data struct {
			Role string `json:"role"`
		} `json:"data"`
	}
	_ = json.Unmarshal(recMe.Body.Bytes(), &meEnv)
	if meEnv.Code != 0 || meEnv.Data.Role != "admin" {
		t.Fatalf("expected admin role via session, got %v", meEnv)
	}

	// 3. cookie 会话的非 GET 请求缺少 CSRF 头 → 403
	msgBody := `{"title":"t","body":"b","devices":["dev-x"]}`
	reqNoCSRF := httptest.NewRequest(http.MethodPost, "/api/v1/messages", strings.NewReader(msgBody))
	reqNoCSRF.AddCookie(cookie)
	reqNoCSRF.Header.Set("Content-Type", "application/json")
	recNoCSRF := httptest.NewRecorder()
	handler.ServeHTTP(recNoCSRF, reqNoCSRF)
	if recNoCSRF.Code != http.StatusForbidden {
		t.Fatalf("POST without CSRF header expected 403, got %d: %s", recNoCSRF.Code, recNoCSRF.Body.String())
	}

	// 4. 带 CSRF 头的 cookie 写请求 → 通过鉴权 (设备不存在返回 400 而非 401/403)
	reqCSRF := httptest.NewRequest(http.MethodPost, "/api/v1/messages", strings.NewReader(msgBody))
	reqCSRF.AddCookie(cookie)
	reqCSRF.Header.Set("Content-Type", "application/json")
	reqCSRF.Header.Set("X-Tink-CSRF", "1")
	recCSRF := httptest.NewRecorder()
	handler.ServeHTTP(recCSRF, reqCSRF)
	if recCSRF.Code == http.StatusUnauthorized || recCSRF.Code == http.StatusForbidden {
		t.Fatalf("POST with CSRF header expected auth pass, got %d: %s", recCSRF.Code, recCSRF.Body.String())
	}

	// 5. Bearer Secret Key 机器通道不受影响
	reqBearer := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	reqBearer.Header.Set("Authorization", "Bearer sk-tink-testtoken123")
	recBearer := httptest.NewRecorder()
	handler.ServeHTTP(recBearer, reqBearer)
	if recBearer.Code != http.StatusOK {
		t.Fatalf("bearer auth expected 200, got %d", recBearer.Code)
	}

	// 6. 登出吊销会话：同一 cookie 立即失效
	reqLogout := httptest.NewRequest(http.MethodPost, "/api/v1/logout", nil)
	reqLogout.AddCookie(cookie)
	reqLogout.Header.Set("X-Tink-CSRF", "1")
	recLogout := httptest.NewRecorder()
	handler.ServeHTTP(recLogout, reqLogout)
	if recLogout.Code != http.StatusOK {
		t.Fatalf("logout expected 200, got %d: %s", recLogout.Code, recLogout.Body.String())
	}
	reqAfterLogout := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	reqAfterLogout.AddCookie(cookie)
	recAfter := httptest.NewRecorder()
	handler.ServeHTTP(recAfter, reqAfterLogout)
	if recAfter.Code != http.StatusUnauthorized {
		t.Fatalf("GET /me after logout expected 401, got %d", recAfter.Code)
	}
}

func TestLoginRejectionsAndSecurityHeaders(t *testing.T) {
	handler, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 错误 Secret Key → 401 且不下发 cookie
	req := httptest.NewRequest(http.MethodPost, "/api/v1/login", strings.NewReader(`{"token":"sk-tink-wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad key login expected 401, got %d", rec.Code)
	}
	if len(rec.Header().Values("Set-Cookie")) != 0 {
		t.Fatalf("expected no cookie on failed login, got %v", rec.Header().Values("Set-Cookie"))
	}

	// 安全响应头与 CORS 移除验证
	reqPing := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	recPing := httptest.NewRecorder()
	handler.ServeHTTP(recPing, reqPing)
	if got := recPing.Header().Get("Content-Security-Policy"); !strings.Contains(got, "script-src 'self'") {
		t.Fatalf("expected CSP with script-src 'self', got %q", got)
	}
	if recPing.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("expected nosniff header")
	}
	if recPing.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("Access-Control-Allow-Origin:* should be removed")
	}
}
