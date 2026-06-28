package notify

import (
	"strings"
	"testing"
)

func TestNewToken_UniqueAndURLSafe(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		tok, err := NewToken()
		if err != nil {
			t.Fatal(err)
		}
		if len(tok) < 40 {
			t.Fatalf("token too short: %q", tok)
		}
		if strings.ContainsAny(tok, "+/=") {
			t.Fatalf("token not URL-safe: %q", tok)
		}
		if seen[tok] {
			t.Fatalf("duplicate token: %q", tok)
		}
		seen[tok] = true
	}
}

func TestBuildURL(t *testing.T) {
	got := BuildURL("http://localhost:8080/", PathConfirm, "abc")
	want := "http://localhost:8080/api/subscriptions/confirm?token=abc"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatPriceEUR(t *testing.T) {
	cases := []struct {
		cents *int
		want  string
	}{
		{nil, "Preis unbekannt"},
		{intp(49900), "499,00 €"},
		{intp(5), "0,05 €"},
		{intp(46500), "465,00 €"},
	}
	for _, c := range cases {
		if got := FormatPriceEUR(c.cents); got != c.want {
			t.Errorf("FormatPriceEUR(%v) = %q, want %q", c.cents, got, c.want)
		}
	}
}

func TestRenderConfirm(t *testing.T) {
	m, err := RenderConfirm(ConfirmData{
		Email:      "a@b.de",
		ConfirmURL: "http://x/confirm?token=t1",
		UnsubURL:   "http://x/unsub?token=u1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.To != "a@b.de" {
		t.Errorf("To = %q", m.To)
	}
	if !strings.Contains(m.HTML, "http://x/confirm?token=t1") {
		t.Error("confirm URL missing from HTML")
	}
	if !strings.Contains(m.HTML, "http://x/unsub?token=u1") {
		t.Error("unsub URL missing from HTML")
	}
	if !strings.Contains(m.Text, "http://x/confirm?token=t1") {
		t.Error("confirm URL missing from text")
	}
	if !strings.Contains(strings.ToLower(m.Subject), "bestätige") {
		t.Errorf("unexpected subject: %q", m.Subject)
	}
}

func TestRenderAvailability(t *testing.T) {
	m, err := RenderAvailability(AvailabilityData{
		Email:       "a@b.de",
		ProductName: "Midea Portasplit",
		Where:       "OBI Markt München-Zentrum, München",
		PriceEUR:    "509,00 €",
		URL:         "http://x/offer",
		UnsubURL:    "http://x/unsub?token=u1",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Midea Portasplit", "München", "509,00 €", "http://x/offer", "http://x/unsub?token=u1"} {
		if !strings.Contains(m.HTML, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
	if !strings.Contains(m.Subject, "Midea Portasplit") {
		t.Errorf("subject missing product: %q", m.Subject)
	}
}

func TestFakeMailer(t *testing.T) {
	f := NewFakeMailer()
	_ = f.Send(Message{To: "a@b.de", Subject: "s"})
	if f.Count() != 1 {
		t.Fatalf("count = %d", f.Count())
	}
	if f.Messages()[0].To != "a@b.de" {
		t.Error("captured message mismatch")
	}
}

func intp(i int) *int { return &i }
