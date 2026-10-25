package notify

import (
	"encoding/base64"
	"net"
	"net/textproto"
	"strings"
	"testing"
)

// smtpSession is what the fake SMTP server observed on one connection.
type smtpSession struct {
	cmds []string
	data string
}

// startFakeSMTP accepts a single plain-text SMTP connection on 127.0.0.1. It
// advertises no STARTTLS and no AUTH, like Mailhog. rcptReply is sent in answer
// to RCPT TO. The session is delivered on the returned channel once the client
// hangs up.
func startFakeSMTP(t *testing.T, rcptReply string) (string, int, <-chan smtpSession) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	ch := make(chan smtpSession, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			close(ch)
			return
		}
		defer conn.Close()
		tp := textproto.NewConn(conn)
		var s smtpSession
		defer func() { ch <- s }()

		_ = tp.PrintfLine("220 fake ESMTP ready")
		for {
			line, err := tp.ReadLine()
			if err != nil {
				return
			}
			s.cmds = append(s.cmds, line)
			verb := strings.ToUpper(strings.Fields(line + " x")[0])
			switch verb {
			case "EHLO":
				_ = tp.PrintfLine("250-fake")
				_ = tp.PrintfLine("250 8BITMIME")
			case "HELO", "MAIL", "RSET", "NOOP":
				_ = tp.PrintfLine("250 OK")
			case "RCPT":
				_ = tp.PrintfLine("%s", rcptReply)
			case "DATA":
				_ = tp.PrintfLine("354 go ahead")
				lines, err := tp.ReadDotLines()
				if err != nil {
					return
				}
				s.data = strings.Join(lines, "\n")
				_ = tp.PrintfLine("250 queued")
			case "QUIT":
				_ = tp.PrintfLine("221 bye")
				return
			default:
				_ = tp.PrintfLine("502 not implemented")
			}
		}
	}()
	return "127.0.0.1", ln.Addr().(*net.TCPAddr).Port, ch
}

func cmdWithPrefix(cmds []string, prefix string) string {
	for _, c := range cmds {
		if strings.HasPrefix(strings.ToUpper(c), prefix) {
			return c
		}
	}
	return ""
}

func TestSMTPMailer_Send_Plain(t *testing.T) {
	host, port, sessions := startFakeSMTP(t, "250 OK")
	m := NewSMTPMailer(SMTPConfig{
		Host: host,
		Port: port,
		From: "PortaSplitFinder <noreply@portasplitfinder.local>",
		TLS:  false,
	})
	msg := Message{
		To:      "kunde@example.de",
		Subject: "Bitte bestätige deine Anmeldung",
		Text:    "Bestätigungslink: http://x/confirm?token=t1",
		HTML:    `<p><a href="http://x/confirm?token=t1">Anmeldung bestätigen</a></p>`,
	}
	if err := m.Send(msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	s := <-sessions

	if mail := cmdWithPrefix(s.cmds, "MAIL FROM:"); !strings.Contains(mail, "<noreply@portasplitfinder.local>") {
		t.Errorf("MAIL FROM = %q, want the bare address of SMTP_FROM", mail)
	}
	if rcpt := cmdWithPrefix(s.cmds, "RCPT TO:"); rcpt != "RCPT TO:<kunde@example.de>" {
		t.Errorf("RCPT = %q, want RCPT TO:<kunde@example.de>", rcpt)
	}
	if n := strings.Count(strings.Join(s.cmds, "\n"), "RCPT TO:"); n != 1 {
		t.Errorf("RCPT TO sent %d times, want 1", n)
	}

	head, body, ok := strings.Cut(s.data, "\n\n")
	if !ok {
		t.Fatalf("message has no header/body separator: %q", s.data)
	}
	headers := map[string]string{}
	for _, l := range strings.Split(head, "\n") {
		k, v, _ := strings.Cut(l, ": ")
		headers[k] = v
	}
	if got := headers["From"]; got != "PortaSplitFinder <noreply@portasplitfinder.local>" {
		t.Errorf("From header = %q", got)
	}
	if got := headers["To"]; got != "kunde@example.de" {
		t.Errorf("To header = %q", got)
	}
	wantSubj := "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(msg.Subject)) + "?="
	if got := headers["Subject"]; got != wantSubj {
		t.Errorf("Subject header = %q, want RFC 2047 encoded %q", got, wantSubj)
	}
	if got := headers["MIME-Version"]; got != "1.0" {
		t.Errorf("MIME-Version = %q", got)
	}
	if got := headers["Content-Type"]; !strings.HasPrefix(got, "multipart/alternative; boundary=") {
		t.Errorf("Content-Type = %q, want multipart/alternative", got)
	}
	for _, want := range []string{
		`Content-Type: text/plain; charset="utf-8"`,
		`Content-Type: text/html; charset="utf-8"`,
		msg.Text,
		msg.HTML,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
	if strings.Index(body, "text/plain") > strings.Index(body, "text/html") {
		t.Error("plain-text part should precede the HTML part")
	}
}

func TestSMTPMailer_Send_ASCIISubjectNotEncoded(t *testing.T) {
	host, port, sessions := startFakeSMTP(t, "250 OK")
	m := NewSMTPMailer(SMTPConfig{Host: host, Port: port, From: "noreply@psf.local"})
	if err := m.Send(Message{To: "a@b.de", Subject: "Hello", Text: "t", HTML: "h"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	s := <-sessions
	if mail := cmdWithPrefix(s.cmds, "MAIL FROM:"); !strings.Contains(mail, "<noreply@psf.local>") {
		t.Errorf("MAIL FROM = %q", mail)
	}
	if !strings.Contains(s.data, "\nSubject: Hello\n") {
		t.Errorf("plain ASCII subject should be sent verbatim, data=%q", s.data)
	}
}

func TestSMTPMailer_Send_RecipientRejected(t *testing.T) {
	host, port, sessions := startFakeSMTP(t, "550 mailbox unavailable")
	m := NewSMTPMailer(SMTPConfig{Host: host, Port: port, From: "noreply@psf.local"})
	err := m.Send(Message{To: "nobody@example.de", Subject: "s", Text: "t", HTML: "h"})
	if err == nil {
		t.Fatal("expected error when the server rejects the recipient")
	}
	if !strings.Contains(err.Error(), "550") {
		t.Errorf("error %q should carry the server reply", err)
	}
	if s := <-sessions; s.data != "" {
		t.Errorf("no data should be sent after a rejected RCPT, got %q", s.data)
	}
}

func freeClosedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestSMTPMailer_Send_ServerDown(t *testing.T) {
	for _, useTLS := range []bool{false, true} {
		name := "plain"
		if useTLS {
			name = "tls"
		}
		t.Run(name, func(t *testing.T) {
			m := NewSMTPMailer(SMTPConfig{Host: "127.0.0.1", Port: freeClosedPort(t), From: "noreply@psf.local", TLS: useTLS})
			if err := m.Send(Message{To: "a@b.de", Subject: "s"}); err == nil {
				t.Fatal("expected error when no SMTP server is listening")
			}
		})
	}
}

func TestSMTPMailer_Send_TLSAgainstPlainServerFails(t *testing.T) {
	host, port, _ := startFakeSMTP(t, "250 OK")
	m := NewSMTPMailer(SMTPConfig{Host: host, Port: port, From: "noreply@psf.local", TLS: true})
	err := m.Send(Message{To: "a@b.de", Subject: "s", Text: "t", HTML: "h"})
	if err == nil {
		t.Fatal("expected TLS handshake error against a plain-text server")
	}
	if !strings.Contains(err.Error(), "smtp tls dial") {
		t.Errorf("error %q should be wrapped as a TLS dial error", err)
	}
}

func TestExtractAddr(t *testing.T) {
	cases := []struct{ in, want string }{
		{"PortaSplitFinder <noreply@portasplitfinder.local>", "noreply@portasplitfinder.local"},
		{"noreply@psf.local", "noreply@psf.local"},
		{"  noreply@psf.local  ", "noreply@psf.local"},
		{"<a@b.de>", "a@b.de"},
	}
	for _, c := range cases {
		if got := extractAddr(c.in); got != c.want {
			t.Errorf("extractAddr(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
