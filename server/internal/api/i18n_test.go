package api_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mrasong/tink/server/internal/i18n"
)

func TestErrorLocalization(t *testing.T) {
	handler, _, cleanup := setupTestServer(t)
	defer cleanup()

	doLogin := func(acceptLang string) string {
		req := httptest.NewRequest("POST", "/api/v1/login", strings.NewReader(`{"token":"wrong"}`))
		req.Header.Set("Content-Type", "application/json")
		if acceptLang != "" {
			req.Header.Set("Accept-Language", acceptLang)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Body.String()
	}

	if body := doLogin("zh-CN,zh;q=0.9,en;q=0.8"); !strings.Contains(body, "无效或已过期") {
		t.Fatalf("zh-CN request expected localized message, got: %s", body)
	}
	if body := doLogin("en-US,en;q=0.9"); !strings.Contains(body, i18n.MsgInvalidSecretKey) {
		t.Fatalf("en request expected English message, got: %s", body)
	}
	if body := doLogin(""); !strings.Contains(body, i18n.MsgInvalidSecretKey) {
		t.Fatalf("no Accept-Language should default to en, got: %s", body)
	}
	// 词典未收录的透传错误保持英文
	if body := doLogin("zh"); strings.Contains(body, `{"code":0`) {
		t.Fatalf("failed login should not return success envelope: %s", body)
	}
}
