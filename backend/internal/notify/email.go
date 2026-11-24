package notify

import (
	"context"
	"fmt"
	"log/slog"
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
