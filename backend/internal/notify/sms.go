package notify

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// LogSMS is the SMS provider for development and test: it writes each
// message to the log, codes included, and sends nothing.
type LogSMS struct{}

// Send logs the message.
func (LogSMS) Send(ctx context.Context, to, text string) error {
	slog.InfoContext(ctx, "SMS (written to the log, not sent)", "to", to, "text", text)
	return nil
}

// NoSMS is the SMS provider of a deployment that has none: every message
// is refused, so a code never reaches a log outside development and test.
type NoSMS struct{}

// Send refuses the message.
func (NoSMS) Send(context.Context, string, string) error {
	return fmt.Errorf("sending an SMS: %w", ErrNoProvider)
}

// SMSFor is the SMS provider of an environment: the log in development and
// test, and none elsewhere.
func SMSFor(local bool) SMS {
	if local {
		return LogSMS{}
	}
	return NoSMS{}
}

// SMSRecorder keeps the texts it is asked to send, for tests.
type SMSRecorder struct {
	mu   sync.Mutex
	sent []Message
}

// Message is a text an SMSRecorder kept.
type Message struct{ To, Text string }

// Send keeps the message.
func (r *SMSRecorder) Send(_ context.Context, to, text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, Message{To: to, Text: text})
	return nil
}

// Sent is every message kept, oldest first.
func (r *SMSRecorder) Sent() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Message(nil), r.sent...)
}

// Last is the last message kept for to, and whether there is one.
func (r *SMSRecorder) Last(to string) (Message, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.sent) - 1; i >= 0; i-- {
		if r.sent[i].To == to {
			return r.sent[i], true
		}
	}
	return Message{}, false
}
