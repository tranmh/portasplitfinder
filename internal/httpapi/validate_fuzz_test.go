package httpapi

import (
	"strings"
	"testing"
)

// FuzzValidEmail checks that an address validEmail accepts contains an '@'
// and no control characters, so it is safe to put into a To: header.
func FuzzValidEmail(f *testing.F) {
	for _, s := range []string{
		"a@b.de", "max.mustermann@example.com", "", "@", "a@", "@b",
		"Max <max@example.com>", "a@b.de\r\nBcc: x@y.z", "\"a b\"@c.de",
		"a@[127.0.0.1]", "\xff@b.de", strings.Repeat("a", 300) + "@b.de",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if !validEmail(s) {
			return
		}
		if !strings.Contains(s, "@") {
			t.Errorf("validEmail(%q) = true without '@'", s)
		}
		for _, r := range s {
			if r < 0x20 || r == 0x7f {
				t.Fatalf("validEmail(%q) = true with control character %U", s, r)
			}
		}
	})
}
