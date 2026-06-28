package notify

import (
	"crypto/rand"
	"encoding/base64"
)

// NewToken returns a URL-safe, unguessable token suitable for confirm/unsub
// links (256 bits of entropy).
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// MustToken is like NewToken but panics on error (only fails if the OS RNG is
// broken). Convenient for seed code; use NewToken on request paths.
func MustToken() string {
	t, err := NewToken()
	if err != nil {
		panic(err)
	}
	return t
}
