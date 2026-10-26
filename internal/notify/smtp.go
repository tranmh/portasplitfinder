package notify

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

func base64Std(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// SMTPConfig configures the SMTP mailer.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// TLS toggles explicit TLS. When false (e.g. Mailhog) a plain connection is
	// used. When true a TLS connection is established (STARTTLS-free implicit TLS
	// is not used here; we dial plain then upgrade via STARTTLS if offered, else
	// fall back to a direct TLS dial).
	TLS bool
}

// SMTPMailer sends email over SMTP.
type SMTPMailer struct {
	cfg SMTPConfig
}

// NewSMTPMailer builds an SMTP mailer.
func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	return &SMTPMailer{cfg: cfg}
}

// Send delivers msg over SMTP.
func (m *SMTPMailer) Send(msg Message) error {
	addr := net.JoinHostPort(m.cfg.Host, fmt.Sprintf("%d", m.cfg.Port))
	from := m.cfg.From
	raw := buildMIME(from, msg)

	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}

	if !m.cfg.TLS {
		// Plain (dev / Mailhog). net/smtp.SendMail attempts STARTTLS if the
		// server advertises it and then auth; Mailhog accepts plain.
		return smtp.SendMail(addr, auth, extractAddr(from), []string{msg.To}, raw)
	}

	return m.sendTLS(addr, auth, extractAddr(from), msg.To, raw)
}

func (m *SMTPMailer) sendTLS(addr string, auth smtp.Auth, from, to string, raw []byte) error {
	d := &net.Dialer{Timeout: 15 * time.Second}
	conn, err := tls.DialWithDialer(d, "tcp", addr, &tls.Config{ServerName: m.cfg.Host})
	if err != nil {
		return fmt.Errorf("smtp tls dial: %w", err)
	}
	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()

	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return c.Quit()
}

// buildMIME constructs a multipart-free HTML email with a plain-text alternative
// folded as the primary body. For simplicity we send the HTML part with a
// text/html content type; the plain text is included as a fallback header
// comment is not standard, so we send a simple multipart/alternative body.
func buildMIME(from string, msg Message) []byte {
	boundary := "psf-boundary-000"
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + msg.To + "\r\n")
	b.WriteString("Subject: " + encodeHeader(msg.Subject) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
	b.WriteString("\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(msg.Text)
	b.WriteString("\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(msg.HTML)
	b.WriteString("\r\n")

	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String())
}

// encodeHeader RFC2047-encodes a header value if it contains non-ASCII or
// CR/LF, so the value can never break out of its header line.
func encodeHeader(s string) string {
	for _, r := range s {
		if r > 127 || r == '\r' || r == '\n' {
			return mimeEncode(s)
		}
	}
	return s
}

func mimeEncode(s string) string {
	// =?UTF-8?B?...?= base64 encoding for non-ASCII subjects.
	return "=?UTF-8?B?" + base64Std(s) + "?="
}

// extractAddr returns the bare address from a "Name <addr>" string.
func extractAddr(from string) string {
	if i := strings.LastIndex(from, "<"); i >= 0 {
		if j := strings.Index(from[i:], ">"); j > 0 {
			return strings.TrimSpace(from[i+1 : i+j])
		}
	}
	return strings.TrimSpace(from)
}
