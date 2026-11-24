package notify

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"io"
	"math/big"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTP is an SMTP server as far as the provider uses it, keeping what
// it is sent.
type fakeSMTP struct {
	t        *testing.T
	ln       net.Listener
	cert     tls.Certificate
	roots    *x509.CertPool
	tls      bool // TLS from the start
	starttls bool // offers STARTTLS
	user     string
	password string // with user, offers AUTH PLAIN once encrypted or on this machine

	mu   sync.Mutex
	sent []delivery
}

// delivery is one message the fake accepted.
type delivery struct {
	From, To string
	Data     string
	Secure   bool
	SignedIn bool
}

func newFakeSMTP(t *testing.T, setup func(*fakeSMTP)) *fakeSMTP {
	t.Helper()
	f := &fakeSMTP{t: t}
	f.cert, f.roots = selfSigned(t)
	if setup != nil {
		setup(f)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if f.tls {
		ln = tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{f.cert}})
	}
	f.ln = ln
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(conn)
		}
	}()
	return f
}

// config is a provider configuration reaching f.
func (f *fakeSMTP) config(security string) SMTPConfig {
	_, port, _ := net.SplitHostPort(f.ln.Addr().String())
	p, _ := strconv.Atoi(port)
	return SMTPConfig{Host: "127.0.0.1", Port: p, Security: security, From: "DaGrouper School <no-reply@dgs.example>",
		TLS: &tls.Config{RootCAs: f.roots, MinVersion: tls.VersionTLS12}}
}

func (f *fakeSMTP) deliveries() []delivery {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]delivery(nil), f.sent...)
}

func (f *fakeSMTP) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	secure := f.tls
	tp := textproto.NewConn(conn)
	var d delivery
	reply := func(code int, lines ...string) {
		for i, l := range lines {
			sep := "-"
			if i == len(lines)-1 {
				sep = " "
			}
			_ = tp.PrintfLine("%d%s%s", code, sep, l)
		}
	}
	reply(220, "fake ESMTP")
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		verb, arg, _ := strings.Cut(line, " ")
		switch strings.ToUpper(verb) {
		case "EHLO", "HELO":
			lines := []string{"fake"}
			if f.starttls && !secure {
				lines = append(lines, "STARTTLS")
			}
			if f.user != "" {
				lines = append(lines, "AUTH PLAIN")
			}
			reply(250, append(lines, "8BITMIME")...)
		case "STARTTLS":
			reply(220, "go ahead")
			tc := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{f.cert}})
			if tc.Handshake() != nil {
				return
			}
			conn, secure = tc, true
			tp = textproto.NewConn(tc)
		case "AUTH":
			raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(arg, "PLAIN "))
			if string(raw) != "\x00"+f.user+"\x00"+f.password {
				reply(535, "bad credentials")
				continue
			}
			d.SignedIn = true
			reply(235, "signed in")
		case "MAIL":
			d.From = address(strings.TrimPrefix(arg, "FROM:"))
			if f.user != "" && !d.SignedIn {
				reply(530, "sign in first")
				continue
			}
			reply(250, "ok")
		case "RCPT":
			d.To = address(strings.TrimPrefix(arg, "TO:"))
			reply(250, "ok")
		case "DATA":
			reply(354, "go ahead")
			data, err := tp.ReadDotBytes()
			if err != nil {
				return
			}
			d.Data, d.Secure = string(data), secure
			f.mu.Lock()
			f.sent = append(f.sent, d)
			f.mu.Unlock()
			reply(250, "queued")
		case "QUIT":
			reply(221, "bye")
			return
		default:
			reply(502, "not here")
		}
	}
}

// address is the path in "<a@b> PARAMS".
func address(arg string) string {
	path, _, _ := strings.Cut(arg, ">")
	return strings.TrimPrefix(path, "<")
}

func selfSigned(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "fake smtp"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)}, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, roots
}

// read is a delivered message's subject and decoded text.
func read(t *testing.T, d delivery) (*mail.Message, string, string) {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(d.Data))
	if err != nil {
		t.Fatal(err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil {
		t.Fatal(err)
	}
	text, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if err != nil {
		t.Fatal(err)
	}
	return msg, subject, string(text)
}

func send(t *testing.T, cfg SMTPConfig, m Mail) error {
	t.Helper()
	s, err := NewSMTP(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s.Send(context.Background(), m)
}

var welcome = Mail{To: "Amara Bekele <amara@example.org>", Subject: "Welcome, Amara — እንኳን ደህና መጡ",
	Text: "Hello Amara,\n\nYour account is ready: https://dgs.example/me\n.\nA line that is longer than seventy-six characters, so quoted-printable has to wrap it.\n"}

// TestSMTPWithSTARTTLS: the provider starts TLS, signs in, and sends one
// UTF-8 message whose subject and text arrive as written.
func TestSMTPWithSTARTTLS(t *testing.T) {
	t.Parallel()
	f := newFakeSMTP(t, func(f *fakeSMTP) { f.starttls, f.user, f.password = true, "dgs", "secret" })
	cfg := f.config(SecuritySTARTTLS)
	cfg.Username, cfg.Password = "dgs", "secret"
	if err := send(t, cfg, welcome); err != nil {
		t.Fatal(err)
	}
	sent := f.deliveries()
	if len(sent) != 1 || sent[0].From != "no-reply@dgs.example" || sent[0].To != "amara@example.org" || !sent[0].Secure || !sent[0].SignedIn {
		t.Fatalf("delivered %+v", sent)
	}
	msg, subject, text := read(t, sent[0])
	if subject != welcome.Subject || strings.ReplaceAll(text, "\r\n", "\n") != welcome.Text {
		t.Fatalf("subject %q, text %q", subject, text)
	}
	if from, _ := msg.Header.AddressList("From"); len(from) != 1 || from[0].Name != "DaGrouper School" {
		t.Fatalf("from %v", msg.Header.Get("From"))
	}
	if !strings.HasSuffix(msg.Header.Get("Message-ID"), "@dgs.example>") || msg.Header.Get("Date") == "" ||
		msg.Header.Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("headers %v", msg.Header)
	}
}

// TestSMTPWithTLS: the provider can connect over TLS from the start.
func TestSMTPWithTLS(t *testing.T) {
	t.Parallel()
	f := newFakeSMTP(t, func(f *fakeSMTP) { f.tls, f.user, f.password = true, "dgs", "secret" })
	cfg := f.config(SecurityTLS)
	cfg.Username, cfg.Password = "dgs", "secret"
	if err := send(t, cfg, welcome); err != nil {
		t.Fatal(err)
	}
	if sent := f.deliveries(); len(sent) != 1 || !sent[0].Secure || !sent[0].SignedIn {
		t.Fatalf("delivered %+v", sent)
	}
}

// TestSMTPInTheClear: with security none, as for a local catcher, the
// message is sent without TLS.
func TestSMTPInTheClear(t *testing.T) {
	t.Parallel()
	f := newFakeSMTP(t, nil)
	if err := send(t, f.config(SecurityNone), welcome); err != nil {
		t.Fatal(err)
	}
	if sent := f.deliveries(); len(sent) != 1 || sent[0].Secure {
		t.Fatalf("delivered %+v", sent)
	}
}

// TestSMTPRefuses: nothing is sent without the encryption asked for, with
// wrong credentials, to a bad address or with a header broken across lines.
func TestSMTPRefuses(t *testing.T) {
	t.Parallel()
	plain := newFakeSMTP(t, nil)
	if err := send(t, plain.config(SecuritySTARTTLS), welcome); err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("a server without STARTTLS: %v", err)
	}
	if err := send(t, plain.config(SecurityTLS), welcome); err == nil {
		t.Fatal("TLS to a server in the clear was not refused")
	}
	if len(plain.deliveries()) != 0 {
		t.Fatalf("delivered %+v", plain.deliveries())
	}

	f := newFakeSMTP(t, func(f *fakeSMTP) { f.starttls, f.user, f.password = true, "dgs", "secret" })
	cfg := f.config(SecuritySTARTTLS)
	cfg.Username, cfg.Password = "dgs", "wrong"
	if err := send(t, cfg, welcome); err == nil || !strings.Contains(err.Error(), "signing in") {
		t.Fatalf("wrong credentials: %v", err)
	}
	cfg.Password = "secret"
	untrusted := cfg
	untrusted.TLS = nil
	if err := send(t, untrusted, welcome); err == nil || !strings.Contains(err.Error(), "TLS") {
		t.Fatalf("an untrusted certificate: %v", err)
	}
	for _, m := range []Mail{
		{To: "not an address", Subject: "Hi", Text: "x"},
		{To: "amara@example.org", Subject: "Hi\r\nBcc: someone@example.org", Text: "x"},
	} {
		if err := send(t, cfg, m); err == nil {
			t.Fatalf("%+v was sent", m)
		}
	}
	if len(f.deliveries()) != 0 {
		t.Fatalf("delivered %+v", f.deliveries())
	}
}

// TestSMTPGivesUp: a server that never answers ends with the context.
func TestSMTPGivesUp(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		<-done
		_ = conn.Close()
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	p, _ := strconv.Atoi(port)
	s, err := NewSMTP(SMTPConfig{Host: "127.0.0.1", Port: p, Security: SecurityNone, From: "no-reply@dgs.example"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := s.Send(ctx, welcome); err == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("a silent server: %v after %s", err, time.Since(start))
	}
}

func TestNewSMTPRefuses(t *testing.T) {
	t.Parallel()
	ok := SMTPConfig{Host: "smtp.example", Port: 587, Security: SecuritySTARTTLS, From: "no-reply@dgs.example"}
	if _, err := NewSMTP(ok); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*SMTPConfig){
		"no sender":    func(c *SMTPConfig) { c.From = "" },
		"bad security": func(c *SMTPConfig) { c.Security = "ssl" },
		"no port":      func(c *SMTPConfig) { c.Port = 0 },
	} {
		cfg := ok
		change(&cfg)
		if _, err := NewSMTP(cfg); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestEmailFor(t *testing.T) {
	t.Parallel()
	if e, err := EmailFor(true, SMTPConfig{}); err != nil || e != (LogEmail{}) {
		t.Fatalf("development without a server: %T %v", e, err)
	}
	if _, err := EmailFor(false, SMTPConfig{}); !errors.Is(err, ErrNoProvider) {
		t.Fatalf("production without a server: %v", err)
	}
	e, err := EmailFor(false, SMTPConfig{Host: "smtp.example", Port: 587, Security: SecuritySTARTTLS, From: "no-reply@dgs.example"})
	if _, ok := e.(*SMTP); !ok || err != nil {
		t.Fatalf("with a server: %T %v", e, err)
	}
	if SMSFor(true) != (LogSMS{}) || SMSFor(false) != (NoSMS{}) {
		t.Fatal("SMSFor")
	}
	if err := (NoSMS{}).Send(context.Background(), "+251911000000", "x"); !errors.Is(err, ErrNoProvider) {
		t.Fatalf("NoSMS: %v", err)
	}
}
