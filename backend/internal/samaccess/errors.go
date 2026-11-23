package samaccess

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

// ErrUnregistered is returned for a person whose account has no SAM
// principal yet. It is answered as SAM would answer a person with no
// grants, without calling SAM.
var ErrUnregistered = errors.New("the account has no SAM principal yet")

// The messages shown for SAM's refusals: this service's own words. SAM's
// detail is logged, never shown.
const (
	msgForbidden   = "You don't have permission to do that."
	msgInvalid     = "Some of that isn't accepted. Check it and try again."
	msgNotFound    = "That doesn't exist, or you can't see it."
	msgConflict    = "That clashes with something that already exists or was decided."
	msgRateLimited = "Too many requests. Please wait a moment and try again."
)

// Translate turns an error from a SAM call into what the front end gets:
// a refusal becomes forbidden, not_found, conflict, invalid_input or
// rate_limited, and is never answered as a failure of this service. SAM
// being down, slow or failing, and calls this service should never have
// made, stay errors, answered as internal and logged with the request ID.
func Translate(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrUnregistered) {
		return httpx.Forbidden(msgForbidden)
	}
	var e *client.Error
	if !errors.As(err, &e) {
		return fmt.Errorf("calling SAM: %w", err)
	}
	switch e.Code {
	case client.CodeUnauthenticated:
		// A person's token or this service's key was refused: a fault in this
		// service's credentials, not the person's.
		slog.ErrorContext(ctx, "SAM refused this service's credentials", "detail", e.Detail, "sam_request", e.RequestID)
		return httpx.Forbidden(msgForbidden)
	case client.CodeIdempotencyReused, client.CodeMethodNotAllowed, client.CodeBodyTooLarge:
		return fmt.Errorf("a call SAM refuses as malformed: %w", err)
	}
	t := refusal(e)
	if t == nil {
		return fmt.Errorf("SAM failed: %w", err)
	}
	slog.InfoContext(ctx, "SAM refused", "code", e.Code, "detail", e.Detail, "sam_request", e.RequestID)
	return t
}

// refusal is the front end's error for a refusal, by SAM's status, so the
// codes an endpoint adds (such as version_mismatch) are answered as their
// status's kind; nil for a failure.
func refusal(e *client.Error) *httpx.Error {
	switch e.Status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return httpx.New(httpx.CodeInvalidInput, msgInvalid)
	case http.StatusForbidden:
		return httpx.Forbidden(msgForbidden)
	case http.StatusNotFound, http.StatusGone:
		return httpx.NotFound(msgNotFound)
	case http.StatusConflict, http.StatusPreconditionFailed:
		return httpx.New(httpx.CodeConflict, msgConflict)
	case http.StatusTooManyRequests:
		return httpx.RateLimited(msgRateLimited, e.RetryAfter)
	}
	return nil
}
