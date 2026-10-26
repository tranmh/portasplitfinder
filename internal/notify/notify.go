// Package notify renders German notification emails and sends them over SMTP.
// It exposes a Mailer interface with an SMTP implementation (config-driven TLS)
// and a fake implementation for tests.
package notify

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
)

// Message is a rendered email ready to send.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

// Mailer sends an email message.
type Mailer interface {
	Send(msg Message) error
}

// ConfirmData is the template input for the double opt-in confirmation email.
type ConfirmData struct {
	Email      string
	ConfirmURL string
	UnsubURL   string
}

// AvailabilityData is the template input for the availability notification.
type AvailabilityData struct {
	Email       string
	ProductName string
	// Where describes where it became available (store name + city, or retailer).
	Where    string
	PriceEUR string // formatted, e.g. "499,00 €" or "Preis unbekannt"
	URL      string
	UnsubURL string
}

var confirmTmpl = template.Must(template.New("confirm").Parse(`<!doctype html>
<html lang="de"><body>
<h2>Bitte bestätige deine Anmeldung</h2>
<p>Hallo,</p>
<p>du hast dich bei <strong>PortaSplitFinder</strong> für Verfügbarkeitsbenachrichtigungen angemeldet.
Bitte bestätige deine E-Mail-Adresse, indem du auf den folgenden Link klickst:</p>
<p><a href="{{.ConfirmURL}}">Anmeldung bestätigen</a></p>
<p>Falls der Link nicht funktioniert, kopiere diese Adresse in deinen Browser:<br>{{.ConfirmURL}}</p>
<hr>
<p style="font-size:12px;color:#666">Du möchtest keine Benachrichtigungen mehr erhalten?
<a href="{{.UnsubURL}}">Hier abmelden</a>.</p>
</body></html>`))

var confirmTextTmpl = template.Must(template.New("confirmtext").Parse(
	`Bitte bestätige deine Anmeldung bei PortaSplitFinder.

Bestätigungslink: {{.ConfirmURL}}

Abmelden: {{.UnsubURL}}
`))

var availabilityTmpl = template.Must(template.New("avail").Parse(`<!doctype html>
<html lang="de"><body>
<h2>{{.ProductName}} ist wieder verfügbar!</h2>
<p>Gute Nachrichten: <strong>{{.ProductName}}</strong> ist jetzt verfügbar bei:</p>
<p><strong>{{.Where}}</strong><br>Preis: {{.PriceEUR}}</p>
<p><a href="{{.URL}}">Zum Angebot</a></p>
<hr>
<p style="font-size:12px;color:#666">Du erhältst diese E-Mail, weil du dich bei PortaSplitFinder angemeldet hast.
<a href="{{.UnsubURL}}">Hier abmelden</a>.</p>
</body></html>`))

var availabilityTextTmpl = template.Must(template.New("availtext").Parse(
	`{{.ProductName}} ist wieder verfügbar!

Verfügbar bei: {{.Where}}
Preis: {{.PriceEUR}}
Angebot: {{.URL}}

Abmelden: {{.UnsubURL}}
`))

// RenderConfirm builds the confirmation email message.
func RenderConfirm(d ConfirmData) (Message, error) {
	html, err := exec(confirmTmpl, d)
	if err != nil {
		return Message{}, err
	}
	text, err := exec(confirmTextTmpl, d)
	if err != nil {
		return Message{}, err
	}
	return Message{
		To:      d.Email,
		Subject: "Bitte bestätige deine Anmeldung bei PortaSplitFinder",
		HTML:    html,
		Text:    text,
	}, nil
}

// RenderAvailability builds the availability notification message.
func RenderAvailability(d AvailabilityData) (Message, error) {
	html, err := exec(availabilityTmpl, d)
	if err != nil {
		return Message{}, err
	}
	text, err := exec(availabilityTextTmpl, d)
	if err != nil {
		return Message{}, err
	}
	return Message{
		To:      d.Email,
		Subject: fmt.Sprintf("%s ist wieder verfügbar – PortaSplitFinder", d.ProductName),
		HTML:    html,
		Text:    text,
	}, nil
}

func exec(t *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// FormatPriceEUR formats euro cents as a German price string, or
// "Preis unbekannt" when the price is nil.
func FormatPriceEUR(cents *int) string {
	if cents == nil {
		return "Preis unbekannt"
	}
	// Format the sign separately so amounts between -1 and -99 cents keep it,
	// and use the uint64 magnitude so math.MinInt does not overflow.
	sign := ""
	mag := uint64(*cents)
	if *cents < 0 {
		sign = "-"
		mag = -mag
	}
	return fmt.Sprintf("%s%d,%02d €", sign, mag/100, mag%100)
}

// BuildURL joins a base URL and a path query for confirm/unsub links.
func BuildURL(baseURL, path, token string) string {
	base := strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s%s?token=%s", base, path, token)
}

// Paths used to build confirm/unsubscribe links.
const (
	PathConfirm = "/api/subscriptions/confirm"
	PathUnsub   = "/api/subscriptions/unsubscribe"
)
