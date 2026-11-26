// Package httpx holds the API's error format and JSON helpers.
//
// Every error the front end sees is {code, message, fields}: code is one of
// the ServiceErrorCode values in frontend/src/lib/services/types.ts, message
// is shown to the user as is, and fields holds a message per input
// (docs/development.md, "Errors").
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Error codes, as the front end knows them.
const (
	CodeUnauthenticated = "unauthenticated"
	CodeInvalidInput    = "invalid_input"
	CodeNotFound        = "not_found"
	CodeRateLimited     = "rate_limited"
	CodeCodeInvalid     = "code_invalid"
	CodeCodeExpired     = "code_expired"
	CodeConflict        = "conflict"
	CodeForbidden       = "forbidden"
	CodeInternal        = "internal"
	// CodeConsentTextChanged refuses a guardian's decision on words that
	// are no longer the current consent text.
	CodeConsentTextChanged = "consent_text_changed"
)

var statuses = map[string]int{
	CodeUnauthenticated:    http.StatusUnauthorized,
	CodeInvalidInput:       http.StatusBadRequest,
	CodeNotFound:           http.StatusNotFound,
	CodeRateLimited:        http.StatusTooManyRequests,
	CodeCodeInvalid:        http.StatusBadRequest,
	CodeCodeExpired:        http.StatusBadRequest,
	CodeConflict:           http.StatusConflict,
	CodeForbidden:          http.StatusForbidden,
	CodeConsentTextChanged: http.StatusConflict,
	CodeInternal:           http.StatusInternalServerError,
}

// Error is an error the user should see.
type Error struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	// RetryAfter, when set, is sent as the Retry-After header.
	RetryAfter time.Duration `json:"-"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// New returns an error with a code and a message for the user.
func New(code, message string) *Error { return &Error{Code: code, Message: message} }

// IsCode reports whether err is an *Error with code.
func IsCode(err error, code string) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}

// FieldError is an error about one input: the message is shown both for the
// request and next to the field.
func FieldError(code, field, message string) *Error {
	return &Error{Code: code, Message: message, Fields: map[string]string{field: message}}
}

// Invalid is invalid_input with a message per field.
func Invalid(fields map[string]string) *Error {
	return &Error{Code: CodeInvalidInput, Message: "Check the highlighted fields.", Fields: fields}
}

// CheckFields is Invalid when any field has a problem, and nil when none
// does.
func CheckFields(fields map[string]string) error {
	if len(fields) == 0 {
		return nil
	}
	return Invalid(fields)
}

// Unauthenticated asks the user to sign in.
func Unauthenticated() *Error { return New(CodeUnauthenticated, "Please sign in again.") }

// Forbidden refuses an action the user may not take.
func Forbidden(message string) *Error { return New(CodeForbidden, message) }

// NotFound says what could not be found.
func NotFound(message string) *Error { return New(CodeNotFound, message) }

// RateLimited asks the user to wait before trying again.
func RateLimited(message string, retryAfter time.Duration) *Error {
	return &Error{Code: CodeRateLimited, Message: message, RetryAfter: retryAfter}
}

// Status is the HTTP status for an error code.
func Status(code string) int {
	if s, ok := statuses[code]; ok {
		return s
	}
	return http.StatusInternalServerError
}

// WriteJSON writes v with status. Responses are not cached unless the
// handler has already set its own Cache-Control.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	if h.Get("Cache-Control") == "" {
		h.Set("Cache-Control", "no-store")
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes err in the API's error format. An error that is not an
// *Error is logged and shown as a generic failure, so no internal detail
// reaches the user.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var e *Error
	if !errors.As(err, &e) {
		slog.ErrorContext(r.Context(), "request failed", "err", err, "method", r.Method, "path", r.URL.Path)
		e = New(CodeInternal, "Something went wrong. Please try again.")
	}
	if e.RetryAfter > 0 {
		secs := int((e.RetryAfter + time.Second - 1) / time.Second)
		w.Header().Set("Retry-After", strconv.Itoa(secs))
	}
	WriteJSON(w, Status(e.Code), e)
}

// Handler is an http.HandlerFunc that returns an error.
type Handler func(w http.ResponseWriter, r *http.Request) error

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h(w, r); err != nil {
		WriteError(w, r, err)
	}
}

// MaxBody bounds request bodies.
const MaxBody = 1 << 20

// Decode reads a JSON body strictly: unknown fields and trailing data are
// refused, and so is any content type other than JSON.
func Decode(r *http.Request, v any) error { return DecodeLimit(r, v, MaxBody) }

// DecodeLimit is Decode for a body of at most limit bytes, for the few
// endpoints that take more than MaxBody, such as a photo.
func DecodeLimit(r *http.Request, v any, limit int64) error {
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
		return New(CodeInvalidInput, "Send JSON.")
	}
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, limit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return New(CodeInvalidInput, "The request could not be read.")
	}
	if dec.More() {
		return New(CodeInvalidInput, "The request could not be read.")
	}
	return nil
}
