package auth_test

import (
	"strings"
	"testing"

	"github.com/mrasong/tink/server/internal/auth"
)

func TestGenerateTokenFormat(t *testing.T) {
	tok, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prefix := "sk-tink-"
	if !strings.HasPrefix(tok, prefix) {
		t.Fatalf("expected token to start with %q, got %q", prefix, tok)
	}

	body := strings.TrimPrefix(tok, prefix)
	if len(body) != 64 {
		t.Fatalf("expected random string length 64, got %d (%s)", len(body), body)
	}

	const allowed = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for _, ch := range body {
		if !strings.ContainsRune(allowed, ch) {
			t.Fatalf("invalid character %q in token body %s", ch, body)
		}
	}
}
