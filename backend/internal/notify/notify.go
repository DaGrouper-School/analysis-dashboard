// Package notify delivers messages to people: one-time codes by SMS, and
// links by SMS or email (docs/design/accounts-and-sign-in.md#delivery).
// Modules send through its interfaces; which provider stands behind them
// is the deployment's choice. Development and test use providers that
// write to the log, which no other environment may.
package notify

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

// SMS sends a text message to a number in E.164 form.
type SMS interface {
	Send(ctx context.Context, to, text string) error
}

// Log is the SMS provider for development and test: it writes each message
// to the log, codes included, and sends nothing.
type Log struct{}

// Send logs the message.
func (Log) Send(ctx context.Context, to, text string) error {
	slog.InfoContext(ctx, "SMS (written to the log, not sent)", "to", to, "text", text)
	return nil
}

// ErrNoProvider is the answer of a deployment with no SMS provider yet.
var ErrNoProvider = errors.New("no SMS provider is configured")

// None is the SMS provider of a deployment that has none: every message is
// refused, so a code never reaches a log outside development and test.
type None struct{}

// Send refuses the message.
func (None) Send(context.Context, string, string) error { return ErrNoProvider }

// ForEnvironment is the SMS provider for an environment with no provider
// configured: the log in development and test, and none elsewhere.
func ForEnvironment(local bool) SMS {
	if local {
		return Log{}
	}
	return None{}
}

// Recorder keeps what it is asked to send, for tests.
type Recorder struct {
	mu   sync.Mutex
	sent []Message
}

// Message is a message a Recorder kept.
type Message struct{ To, Text string }

// Send keeps the message.
func (r *Recorder) Send(_ context.Context, to, text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, Message{To: to, Text: text})
	return nil
}

// Sent is every message kept, oldest first.
func (r *Recorder) Sent() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Message(nil), r.sent...)
}

// Last is the last message kept for to, and whether there is one.
func (r *Recorder) Last(to string) (Message, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.sent) - 1; i >= 0; i-- {
		if r.sent[i].To == to {
			return r.sent[i], true
		}
	}
	return Message{}, false
}
