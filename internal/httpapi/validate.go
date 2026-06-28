package httpapi

import (
	"net/mail"
	"regexp"
)

var plzRe = regexp.MustCompile(`^\d{5}$`)

// validPLZ reports whether s is a 5-digit German postal code.
func validPLZ(s string) bool { return plzRe.MatchString(s) }

// validEmail reports whether s parses as a single email address.
func validEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	return err == nil && addr.Address == s
}

// clampRadius validates a radius in 1..500.
func validRadius(km int) bool { return km >= 1 && km <= 500 }

const (
	defaultRadiusKm = 50
	maxRadiusKm     = 500
)
