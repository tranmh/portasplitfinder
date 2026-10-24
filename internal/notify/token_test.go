package notify

import (
	"encoding/base64"
	"testing"
)

func isURLSafe(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func TestNewToken_FormatAndEntropy(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		tok, err := NewToken()
		if err != nil {
			t.Fatal(err)
		}
		if len(tok) != 43 {
			t.Fatalf("len(%q) = %d, want 43", tok, len(tok))
		}
		if !isURLSafe(tok) {
			t.Fatalf("token %q has characters outside the URL-safe alphabet", tok)
		}
		raw, err := base64.RawURLEncoding.DecodeString(tok)
		if err != nil {
			t.Fatalf("token %q is not raw URL base64: %v", tok, err)
		}
		if len(raw) != 32 {
			t.Fatalf("token decodes to %d bytes, want 32", len(raw))
		}
		if seen[tok] {
			t.Fatalf("duplicate token after %d calls: %q", i, tok)
		}
		seen[tok] = true
	}
}

func TestMustToken(t *testing.T) {
	a, b := MustToken(), MustToken()
	if len(a) != 43 || !isURLSafe(a) {
		t.Errorf("MustToken() = %q, want 43 URL-safe chars", a)
	}
	if a == b {
		t.Errorf("successive MustToken calls returned the same token %q", a)
	}
}
