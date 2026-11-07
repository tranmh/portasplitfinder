package notify

import (
	"encoding/base64"
	"regexp"
	"testing"
)

var urlSafeTokenRe = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// B22: 32 random bytes, URL-safe base64 without padding (43 chars).
func TestNewToken_FormatAndDecoding(t *testing.T) {
	for i := 0; i < 200; i++ {
		tok, err := NewToken()
		if err != nil {
			t.Fatalf("NewToken: %v", err)
		}
		if len(tok) != 43 {
			t.Fatalf("len = %d, want 43 (%q)", len(tok), tok)
		}
		if !urlSafeTokenRe.MatchString(tok) {
			t.Fatalf("token has non URL-safe characters or padding: %q", tok)
		}
		raw, err := base64.RawURLEncoding.DecodeString(tok)
		if err != nil {
			t.Fatalf("token does not decode as raw URL base64: %v", err)
		}
		if len(raw) != 32 {
			t.Fatalf("decoded length = %d, want 32", len(raw))
		}
	}
}

func TestNewToken_Unique(t *testing.T) {
	const n = 2000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		tok, err := NewToken()
		if err != nil {
			t.Fatalf("NewToken: %v", err)
		}
		if _, dup := seen[tok]; dup {
			t.Fatalf("duplicate token after %d calls: %q", i, tok)
		}
		seen[tok] = struct{}{}
	}
}

func TestMustToken(t *testing.T) {
	a := MustToken()
	b := MustToken()
	if !urlSafeTokenRe.MatchString(a) || !urlSafeTokenRe.MatchString(b) {
		t.Fatalf("MustToken format: %q %q", a, b)
	}
	if a == b {
		t.Fatalf("MustToken returned the same token twice: %q", a)
	}
}
