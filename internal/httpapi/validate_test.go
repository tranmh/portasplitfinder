package httpapi

import (
	"math"
	"testing"
)

// B2: validPLZ accepts exactly 5 ASCII digits.
func TestValidPLZ(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"five digits", "73035", true},
		{"leading zeros", "01067", true},
		{"all zeros", "00000", true},
		{"all nines", "99999", true},
		{"empty", "", false},
		{"four digits", "7303", false},
		{"six digits", "730350", false},
		{"letter inside", "73a35", false},
		{"trailing letter", "7303x", false},
		{"leading space", " 73035", false},
		{"trailing space", "73035 ", false},
		{"trailing newline", "73035\n", false},
		{"inner space", "730 35", false},
		{"negative sign", "-7303", false},
		{"arabic-indic digits", "٧٣٠٣٥", false},
		{"fullwidth digits", "７３０３５", false},
		{"devanagari digits", "७३०३५", false},
		{"two codes", "73035,10115", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validPLZ(tc.in); got != tc.want {
				t.Fatalf("validPLZ(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// B4: validEmail accepts a single plain address and nothing else.
func TestValidEmail(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"plain address", "a@b.de", true},
		{"dotted local and subdomain", "max.mustermann@mail.example.com", true},
		{"plus tag", "user+tag@example.com", true},
		{"upper case", "USER@EXAMPLE.COM", true},
		{"empty", "", false},
		{"no at sign", "userexample.com", false},
		{"missing local part", "@example.com", false},
		{"missing domain", "user@", false},
		{"two at signs", "a@b@c.de", false},
		{"display name form", "Name <a@b.c>", false},
		{"quoted display name form", `"Max Mustermann" <max@example.com>`, false},
		{"angle brackets only", "<a@b.c>", false},
		{"leading whitespace", " a@b.de", false},
		{"trailing whitespace", "a@b.de ", false},
		{"two addresses", "a@b.de, c@d.de", false},
		{"crlf header injection", "a@b.de\r\nBcc: evil@example.com", false},
		{"inner space", "a b@c.de", false},
		{"double dot in domain", "user@example..com", false},
		{"leading dot in domain", "user@.example.com", false},
		{"trailing dot in domain", "user@example.com.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validEmail(tc.in); got != tc.want {
				t.Fatalf("validEmail(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// B3: validRadius accepts integers 1..500 inclusive.
func TestValidRadius(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want bool
	}{
		{"min int", math.MinInt, false},
		{"negative", -1, false},
		{"zero", 0, false},
		{"lower bound", 1, true},
		{"default", 50, true},
		{"just below max", 499, true},
		{"upper bound", 500, true},
		{"just above max", 501, false},
		{"max int", math.MaxInt, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validRadius(tc.in); got != tc.want {
				t.Fatalf("validRadius(%d) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// B7/B3: the default radius is 50 km and the maximum 500 km, both valid.
func TestRadiusConstants(t *testing.T) {
	if defaultRadiusKm != 50 {
		t.Fatalf("defaultRadiusKm = %d, want 50", defaultRadiusKm)
	}
	if maxRadiusKm != 500 {
		t.Fatalf("maxRadiusKm = %d, want 500", maxRadiusKm)
	}
	if !validRadius(defaultRadiusKm) || !validRadius(maxRadiusKm) {
		t.Fatal("default and max radius must themselves be valid")
	}
	if validRadius(maxRadiusKm + 1) {
		t.Fatal("radius above maxRadiusKm must be invalid")
	}
}
