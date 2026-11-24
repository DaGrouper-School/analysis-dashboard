package notify

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// How the connection to an SMTP server is secured.
const (
	// SecuritySTARTTLS connects in the clear and requires STARTTLS before
	// anything is sent, usually on port 587.
	SecuritySTARTTLS = "starttls"
	// SecurityTLS connects over TLS from the start, usually on port 465.
	SecurityTLS = "tls"
	// SecurityNone sends in the clear, for a local catcher such as Mailpit;
	// only development and test allow it.
	SecurityNone = "none"
)

// smtpTimeout bounds one email, from connecting to the server's last answer.
const smtpTimeout = 30 * time.Second

// SMTPConfig is how email reaches an SMTP server.
type SMTPConfig struct {
	Host     string
	Port     int
	Security string
	// Username and Password sign in with AUTH PLAIN, which is refused over an
	// unencrypted connection to anything but this machine. Empty sends
	// without signing in.
	Username string
	Password string
	// From is the sender, such as "DaGrouper School <no-reply@dgs.example>".
	From string
	// TLS is the TLS configuration; nil verifies the server against the
	// system's roots. Tests set their own.
	TLS *tls.Config
}

// SMTP sends email through an SMTP server.
type SMTP struct {
	cfg  SMTPConfig
	from *mail.Address
}

// NewSMTP is the SMTP provider for cfg.
func NewSMTP(cfg SMTPConfig) (*SMTP, error) {
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("the sender %q is not an email address: %w", cfg.From, err)
	}
	switch cfg.Security {
	case SecuritySTARTTLS, SecurityTLS, SecurityNone:
	default:
		return nil, fmt.Errorf("the SMTP security %q is not starttls, tls or none", cfg.Security)
	}
	if cfg.Host == "" || cfg.Port < 1 || cfg.Port > 65535 {
		return nil, fmt.Errorf("the SMTP server %s:%d is not a host and port", cfg.Host, cfg.Port)
	}
	return &SMTP{cfg: cfg, from: from}, nil
}

// Send sends m: one connection, one recipient.
func (s *SMTP) Send(ctx context.Context, m Mail) error {
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return fmt.Errorf("the recipient is not an email address: %w", err)
	}
	msg, err := s.compose(to, m, time.Now())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, smtpTimeout)
	defer cancel()
	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()
	if err := deliver(c, s.from.Address, to.Address, msg); err != nil {
		return fmt.Errorf("sending email through %s: %w", s.cfg.Host, err)
	}
	return nil
}

// connect opens a session with the server, secured and signed in as
// configured. Cancelling ctx ends it.
func (s *SMTP) connect(ctx context.Context) (*smtp.Client, error) {
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	var conn net.Conn
	var err error
	if s.cfg.Security == SecurityTLS {
		conn, err = (&tls.Dialer{Config: s.tlsConfig()}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("connecting to the SMTP server: %w", err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()) })
	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("greeting the SMTP server: %w", err)
	}
	if err := s.secure(c); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

// secure starts TLS when configured to, and signs in.
func (s *SMTP) secure(c *smtp.Client) error {
	if s.cfg.Security == SecuritySTARTTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("the SMTP server does not offer STARTTLS")
		}
		if err := c.StartTLS(s.tlsConfig()); err != nil {
			return fmt.Errorf("starting TLS with the SMTP server: %w", err)
		}
	}
	if s.cfg.Username == "" {
		return nil
	}
	if ok, _ := c.Extension("AUTH"); !ok {
		return errors.New("the SMTP server does not offer AUTH")
	}
	if err := c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
		return fmt.Errorf("signing in to the SMTP server: %w", err)
	}
	return nil
}

func (s *SMTP) tlsConfig() *tls.Config {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if s.cfg.TLS != nil {
		cfg = s.cfg.TLS.Clone()
	}
	if cfg.ServerName == "" {
		cfg.ServerName = s.cfg.Host
	}
	return cfg
}

// deliver sends one message from from to to in an open session.
func deliver(c *smtp.Client, from, to string, msg []byte) error {
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// compose is m as a message: plain UTF-8 text, quoted-printable.
func (s *SMTP) compose(to *mail.Address, m Mail, now time.Time) ([]byte, error) {
	if strings.ContainsAny(m.Subject, "\r\n") {
		return nil, errors.New("an email subject may not hold a line break")
	}
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	domain := s.from.Address[strings.LastIndexByte(s.from.Address, '@')+1:]
	var b bytes.Buffer
	for _, h := range [][2]string{
		{"From", s.from.String()},
		{"To", to.String()},
		{"Subject", mime.QEncoding.Encode("utf-8", m.Subject)},
		{"Date", now.Format(time.RFC1123Z)},
		{"Message-ID", "<" + hex.EncodeToString(id) + "@" + domain + ">"},
		{"MIME-Version", "1.0"},
		{"Content-Type", "text/plain; charset=utf-8"},
		{"Content-Transfer-Encoding", "quoted-printable"},
	} {
		b.WriteString(h[0] + ": " + h[1] + "\r\n")
	}
	b.WriteString("\r\n")
	qp := quotedprintable.NewWriter(&b)
	if _, err := qp.Write([]byte(m.Text)); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
