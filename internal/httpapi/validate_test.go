package httpapi

import "testing"

func TestValidPLZ(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"five digits", "10115", true},
		{"leading zero", "01067", true},
		{"all zeros", "00000", true},
		{"empty", "", false},
		{"four digits", "1011", false},
		{"six digits", "101150", false},
		{"letters", "abcde", false},
		{"mixed", "1011a", false},
		{"leading space", " 10115", false},
		{"trailing space", "10115 ", false},
		{"trailing newline", "10115\n", false},
		{"inner space", "101 15", false},
		{"arabic-indic digits", "١٢٣٤٥", false},
		{"fullwidth digits", "１２３４５", false},
		{"sign", "+1011", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := validPLZ(c.in); got != c.want {
				t.Errorf("validPLZ(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestValidEmail(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"simple", "a@b.de", true},
		{"dotted local and subdomain", "max.mustermann@mail.example.com", true},
		{"plus tag", "user+tag@example.org", true},
		{"empty", "", false},
		{"no at", "bad", false},
		{"no local part", "@b.de", false},
		{"no domain", "a@", false},
		{"double at", "a@@b.de", false},
		{"display name form", "Name <a@b.c>", false},
		{"angle brackets only", "<a@b.c>", false},
		{"leading space", " a@b.de", false},
		{"trailing space", "a@b.de ", false},
		{"two addresses", "a@b.de, c@d.de", false},
		{"inner space", "a b@c.de", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := validEmail(c.in); got != c.want {
				t.Errorf("validEmail(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestValidRadius(t *testing.T) {
	cases := []struct {
		in   int
		want bool
	}{
		{-1, false},
		{0, false},
		{1, true},
		{2, true},
		{defaultRadiusKm, true},
		{499, true},
		{500, true},
		{maxRadiusKm, true},
		{501, false},
		{100000, false},
	}
	for _, c := range cases {
		if got := validRadius(c.in); got != c.want {
			t.Errorf("validRadius(%d) = %v, want %v", c.in, got, c.want)
		}
	}
}
