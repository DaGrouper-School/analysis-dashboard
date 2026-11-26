package notify

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// LogEmail is the email provider for development and test: it writes each
// email to the log, links included, and sends nothing.
type LogEmail struct{}

// Send logs the email.
func (LogEmail) Send(ctx context.Context, m Mail) error {
	slog.InfoContext(ctx, "email (written to the log, not sent)", "to", m.To, "subject", m.Subject, "text", m.Text)
	return nil
}

// EmailFor is the email provider of an environment: the SMTP server when
// one is configured, otherwise the log in development and test. Elsewhere
// a server is required.
func EmailFor(local bool, cfg SMTPConfig) (Email, error) {
	switch {
	case cfg.Host != "":
		return NewSMTP(cfg)
	case local:
		return LogEmail{}, nil
	}
	return nil, fmt.Errorf("sending email: %w", ErrNoProvider)
}

// EmailRecorder keeps the emails it is asked to send, for tests.
type EmailRecorder struct {
	mu   sync.Mutex
	sent []Mail
}

// Send keeps the email.
func (r *EmailRecorder) Send(_ context.Context, m Mail) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, m)
	return nil
}

// Sent are the emails kept, oldest first.
func (r *EmailRecorder) Sent() []Mail {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Mail(nil), r.sent...)
}
