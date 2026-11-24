// Package notify delivers messages to people: one-time codes by SMS, and
// links by SMS or email (docs/design/accounts-and-sign-in.md#delivery).
// Modules send through its interfaces; which provider stands behind them
// is the deployment's choice. Development and test use providers that
// write to the log, which no other environment may.
package notify

import (
	"context"
	"errors"
)

// SMS sends a text message to a number in E.164 form.
type SMS interface {
	Send(ctx context.Context, to, text string) error
}

// Email sends an email.
type Email interface {
	Send(ctx context.Context, m Mail) error
}

// Mail is one plain-text email to one address.
type Mail struct {
	To      string
	Subject string
	Text    string
}

// ErrNoProvider is the answer of a deployment with no provider for a kind
// of message.
var ErrNoProvider = errors.New("no provider is configured")
