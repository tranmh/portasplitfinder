package notify

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"
)

// receivedMail is one message accepted by fakeSMTP.
type receivedMail struct {
	from  string
	rcpts []string
	data  []byte
}

// fakeSMTP is a minimal local SMTP server for exercising SMTPMailer.
type fakeSMTP struct {
	ln         net.Listener
	advertAuth bool
	rejectRcpt bool

	mu    sync.Mutex
	mails []receivedMail
	auths []string
	wg    sync.WaitGroup
}

func startFakeSMTP(t *testing.T, advertAuth, rejectRcpt bool) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakeSMTP{ln: ln, advertAuth: advertAuth, rejectRcpt: rejectRcpt}
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			f.wg.Add(1)
			go func() {
				defer f.wg.Done()
				f.handle(c)
			}()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		f.wg.Wait()
	})
	return f
}

func (f *fakeSMTP) port() int { return f.ln.Addr().(*net.TCPAddr).Port }

func (f *fakeSMTP) received() ([]receivedMail, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]receivedMail(nil), f.mails...), append([]string(nil), f.auths...)
}

func (f *fakeSMTP) handle(c net.Conn) {
	defer c.Close()
	// Safety net so a misbehaving client can never hang the test run.
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))
	tp := textproto.NewConn(c)
	_ = tp.PrintfLine("220 fake ESMTP")
	var cur receivedMail
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		up := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(up, "EHLO"):
			if f.advertAuth {
				_ = tp.PrintfLine("250-fake")
				_ = tp.PrintfLine("250 AUTH PLAIN")
			} else {
				_ = tp.PrintfLine("250 fake")
			}
		case strings.HasPrefix(up, "HELO"):
			_ = tp.PrintfLine("250 fake")
		case strings.HasPrefix(up, "AUTH PLAIN "):
			f.mu.Lock()
			f.auths = append(f.auths, line[len("AUTH PLAIN "):])
			f.mu.Unlock()
			_ = tp.PrintfLine("235 ok")
		case strings.HasPrefix(up, "MAIL FROM:"):
			cur = receivedMail{from: trimAngles(line[len("MAIL FROM:"):])}
			_ = tp.PrintfLine("250 ok")
		case strings.HasPrefix(up, "RCPT TO:"):
			if f.rejectRcpt {
				_ = tp.PrintfLine("550 no such user")
				continue
			}
			cur.rcpts = append(cur.rcpts, trimAngles(line[len("RCPT TO:"):]))
			_ = tp.PrintfLine("250 ok")
		case up == "DATA":
			_ = tp.PrintfLine("354 go ahead")
			var lines []string
			for {
				l, err := tp.ReadLine()
				if err != nil {
					return
				}
				if l == "." {
					break
				}
				lines = append(lines, strings.TrimPrefix(l, "."))
			}
			cur.data = []byte(strings.Join(lines, "\r\n") + "\r\n")
			f.mu.Lock()
			f.mails = append(f.mails, cur)
			f.mu.Unlock()
			_ = tp.PrintfLine("250 queued")
		case up == "RSET" || up == "NOOP":
			_ = tp.PrintfLine("250 ok")
		case up == "QUIT":
			_ = tp.PrintfLine("221 bye")
			return
		default:
			_ = tp.PrintfLine("502 not implemented")
		}
	}
}

func trimAngles(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, ' '); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSuffix(strings.TrimPrefix(s, "<"), ">")
}

func testMessage() Message {
	return Message{
		To:      "kunde@example.com",
		Subject: "Hello there",
		Text:    "Plain body line 1\r\nline 2",
		HTML:    "<p>HTML body</p>",
	}
}

func TestSMTPMailer_Send_DeliversMultipartMessage(t *testing.T) {
	srv := startFakeSMTP(t, false, false)
	m := NewSMTPMailer(SMTPConfig{Host: "127.0.0.1", Port: srv.port(), From: "PortaSplitFinder <noreply@psf.example>"})

	msg := testMessage()
	if err := m.Send(msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	mails, auths := srv.received()
	if len(auths) != 0 {
		t.Fatalf("no auth expected without username, got %v", auths)
	}
	if len(mails) != 1 {
		t.Fatalf("got %d mails, want 1", len(mails))
	}
	got := mails[0]
	if got.from != "noreply@psf.example" {
		t.Fatalf("envelope MAIL FROM = %q, want bare address", got.from)
	}
	if len(got.rcpts) != 1 || got.rcpts[0] != msg.To {
		t.Fatalf("envelope RCPT = %v, want [%s]", got.rcpts, msg.To)
	}

	parsed, err := mail.ReadMessage(bytes.NewReader(got.data))
	if err != nil {
		t.Fatalf("parse message: %v\n%s", err, got.data)
	}
	h := parsed.Header
	if h.Get("From") != "PortaSplitFinder <noreply@psf.example>" {
		t.Fatalf("From header = %q", h.Get("From"))
	}
	if h.Get("To") != msg.To {
		t.Fatalf("To header = %q", h.Get("To"))
	}
	if h.Get("Subject") != msg.Subject {
		t.Fatalf("Subject header = %q", h.Get("Subject"))
	}
	if h.Get("MIME-Version") != "1.0" {
		t.Fatalf("MIME-Version = %q", h.Get("MIME-Version"))
	}
	mt, params, err := mime.ParseMediaType(h.Get("Content-Type"))
	if err != nil || mt != "multipart/alternative" || params["boundary"] == "" {
		t.Fatalf("Content-Type = %q (%v)", h.Get("Content-Type"), err)
	}

	mr := multipart.NewReader(parsed.Body, params["boundary"])
	want := []struct{ ctype, body string }{
		{"text/plain", msg.Text},
		{"text/html", msg.HTML},
	}
	for i, w := range want {
		p, err := mr.NextPart()
		if err != nil {
			t.Fatalf("part %d: %v", i, err)
		}
		pt, pp, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if pt != w.ctype || !strings.EqualFold(pp["charset"], "utf-8") {
			t.Fatalf("part %d Content-Type = %q", i, p.Header.Get("Content-Type"))
		}
		b, _ := io.ReadAll(p)
		if string(b) != w.body {
			t.Fatalf("part %d body = %q, want %q", i, b, w.body)
		}
	}
	if _, err := mr.NextPart(); err != io.EOF {
		t.Fatalf("expected exactly two parts, got err %v", err)
	}
}

func TestSMTPMailer_Send_DotStuffedLineSurvives(t *testing.T) {
	srv := startFakeSMTP(t, false, false)
	m := NewSMTPMailer(SMTPConfig{Host: "127.0.0.1", Port: srv.port(), From: "noreply@psf.example"})
	msg := testMessage()
	msg.Text = "before\r\n.\r\nafter"
	if err := m.Send(msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	mails, _ := srv.received()
	if len(mails) != 1 || !bytes.Contains(mails[0].data, []byte("before\r\n.\r\nafter")) {
		t.Fatalf("lone dot line was not transported intact: %q", mails)
	}
	if mails[0].from != "noreply@psf.example" {
		t.Fatalf("MAIL FROM = %q", mails[0].from)
	}
}

func TestSMTPMailer_Send_NonASCIISubjectIsEncoded(t *testing.T) {
	srv := startFakeSMTP(t, false, false)
	m := NewSMTPMailer(SMTPConfig{Host: "127.0.0.1", Port: srv.port(), From: "noreply@psf.example"})
	msg := testMessage()
	msg.Subject = "Bitte bestätige deine Anmeldung – PortaSplitFinder"
	if err := m.Send(msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	mails, _ := srv.received()
	if len(mails) != 1 {
		t.Fatalf("got %d mails", len(mails))
	}
	parsed, err := mail.ReadMessage(bytes.NewReader(mails[0].data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	raw := parsed.Header.Get("Subject")
	if !strings.HasPrefix(raw, "=?UTF-8?B?") || !strings.HasSuffix(raw, "?=") {
		t.Fatalf("subject not RFC 2047 encoded: %q", raw)
	}
	dec, err := new(mime.WordDecoder).DecodeHeader(raw)
	if err != nil || dec != msg.Subject {
		t.Fatalf("decoded subject = %q (%v), want %q", dec, err, msg.Subject)
	}
	// Header lines must stay 7-bit.
	head := mails[0].data[:bytes.Index(mails[0].data, []byte("\r\n\r\n"))]
	for _, c := range head {
		if c > 127 {
			t.Fatalf("non-ASCII byte in headers: %q", head)
		}
	}
}

func TestSMTPMailer_Send_PlainAuth(t *testing.T) {
	srv := startFakeSMTP(t, true, false)
	m := NewSMTPMailer(SMTPConfig{Host: "127.0.0.1", Port: srv.port(), Username: "user", Password: "pa55", From: "noreply@psf.example"})
	if err := m.Send(testMessage()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	mails, auths := srv.received()
	if len(mails) != 1 {
		t.Fatalf("got %d mails", len(mails))
	}
	if len(auths) != 1 {
		t.Fatalf("got %d AUTH commands, want 1", len(auths))
	}
	cred, err := base64.StdEncoding.DecodeString(auths[0])
	if err != nil || string(cred) != "\x00user\x00pa55" {
		t.Fatalf("AUTH PLAIN payload = %q (%v)", cred, err)
	}
}

func TestSMTPMailer_Send_Errors(t *testing.T) {
	cases := []struct {
		name       string
		advertAuth bool
		rejectRcpt bool
		cfg        func(port int) SMTPConfig
		msg        func() Message
	}{
		{
			name:       "recipient rejected",
			rejectRcpt: true,
			cfg:        func(p int) SMTPConfig { return SMTPConfig{Host: "127.0.0.1", Port: p, From: "noreply@psf.example"} },
			msg:        testMessage,
		},
		{
			name: "credentials set but server lacks AUTH",
			cfg: func(p int) SMTPConfig {
				return SMTPConfig{Host: "127.0.0.1", Port: p, Username: "u", Password: "p", From: "noreply@psf.example"}
			},
			msg: testMessage,
		},
		{
			name: "CRLF injection in recipient",
			cfg:  func(p int) SMTPConfig { return SMTPConfig{Host: "127.0.0.1", Port: p, From: "noreply@psf.example"} },
			msg: func() Message {
				m := testMessage()
				m.To = "kunde@example.com\r\nBcc: evil@example.com"
				return m
			},
		},
		{
			name: "LF injection in recipient",
			cfg:  func(p int) SMTPConfig { return SMTPConfig{Host: "127.0.0.1", Port: p, From: "noreply@psf.example"} },
			msg: func() Message {
				m := testMessage()
				m.To = "kunde@example.com\nRCPT TO:<evil@example.com>"
				return m
			},
		},
		{
			name: "CRLF injection in from",
			cfg: func(p int) SMTPConfig {
				return SMTPConfig{Host: "127.0.0.1", Port: p, From: "noreply@psf.example\r\nBcc: evil@example.com"}
			},
			msg: testMessage,
		},
		{
			name: "TLS against plain server",
			cfg: func(p int) SMTPConfig {
				return SMTPConfig{Host: "127.0.0.1", Port: p, From: "noreply@psf.example", TLS: true}
			},
			msg: testMessage,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := startFakeSMTP(t, tc.advertAuth, tc.rejectRcpt)
			m := NewSMTPMailer(tc.cfg(srv.port()))
			if err := m.Send(tc.msg()); err == nil {
				t.Fatal("expected an error")
			}
			mails, _ := srv.received()
			if len(mails) != 0 {
				t.Fatalf("no mail must be delivered, got %q", mails[0].data)
			}
		})
	}
}

func TestSMTPMailer_Send_ConnectionRefused(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	for _, useTLS := range []bool{false, true} {
		m := NewSMTPMailer(SMTPConfig{Host: "127.0.0.1", Port: port, From: "noreply@psf.example", TLS: useTLS})
		if err := m.Send(testMessage()); err == nil {
			t.Fatalf("TLS=%v: expected dial error", useTLS)
		}
	}
}

// A subject carrying CR/LF must not be able to add headers to the mail.
func TestSMTPMailer_Send_SubjectHeaderInjection(t *testing.T) {
	srv := startFakeSMTP(t, false, false)
	m := NewSMTPMailer(SMTPConfig{Host: "127.0.0.1", Port: srv.port(), From: "noreply@psf.example"})
	msg := testMessage()
	msg.Subject = "Portasplit\r\nBcc: evil@example.com"
	err := m.Send(msg)
	mails, _ := srv.received()
	if err != nil && len(mails) == 0 {
		return // refusing to send is acceptable
	}
	if len(mails) != 1 {
		t.Fatalf("got %d mails", len(mails))
	}
	parsed, perr := mail.ReadMessage(bytes.NewReader(mails[0].data))
	if perr != nil {
		t.Fatalf("parse: %v", perr)
	}
	if bcc := parsed.Header.Get("Bcc"); bcc != "" {
		// buildMIME/encodeHeader write an ASCII Subject verbatim, so CR/LF in
		// it injects extra headers such as Bcc.
		t.Fatalf("subject injected a Bcc header: %q", bcc)
	}
}

// A pure ASCII subject longer than 75 characters must survive intact, and any
// RFC 2047 encoded-word used for it must stay within the 75 character limit.
func TestSMTPMailer_Send_LongASCIISubject(t *testing.T) {
	srv := startFakeSMTP(t, false, false)
	m := NewSMTPMailer(SMTPConfig{Host: "127.0.0.1", Port: srv.port(), From: "noreply@psf.example"})
	msg := testMessage()
	msg.Subject = "PortaSplitFinder: your subscription for 73035 within 50 km is now confirmed, thanks"
	if len(msg.Subject) <= 75 {
		t.Fatalf("test subject must be longer than 75 characters, is %d", len(msg.Subject))
	}
	if err := m.Send(msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	mails, _ := srv.received()
	if len(mails) != 1 {
		t.Fatalf("got %d mails", len(mails))
	}
	parsed, err := mail.ReadMessage(bytes.NewReader(mails[0].data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	raw := parsed.Header.Get("Subject")
	for _, w := range strings.Fields(raw) {
		if strings.HasPrefix(w, "=?") && strings.HasSuffix(w, "?=") && len(w) > 75 {
			t.Fatalf("encoded-word longer than 75 characters (%d): %q", len(w), w)
		}
	}
	dec, err := new(mime.WordDecoder).DecodeHeader(raw)
	if err != nil || dec != msg.Subject {
		t.Fatalf("decoded subject = %q (%v), want %q", dec, err, msg.Subject)
	}
}

func TestExtractAddr(t *testing.T) {
	cases := []struct{ in, want string }{
		{"noreply@psf.example", "noreply@psf.example"},
		{"  noreply@psf.example  ", "noreply@psf.example"},
		{"PortaSplitFinder <noreply@psf.example>", "noreply@psf.example"},
		{"\"A <b>\" <real@psf.example>", "real@psf.example"},
		{"Name < spaced@psf.example >", "spaced@psf.example"},
		{"Broken <noreply@psf.example", "Broken <noreply@psf.example"},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := extractAddr(tc.in); got != tc.want {
				t.Fatalf("extractAddr(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestEncodeHeader(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hello", "Hello"},
		{"", ""},
		{"ä", "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte("ä")) + "?="},
		{"Preis 499 €", "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte("Preis 499 €")) + "?="},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := encodeHeader(tc.in); got != tc.want {
				t.Fatalf("encodeHeader(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
