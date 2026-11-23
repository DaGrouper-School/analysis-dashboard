package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDecode(t *testing.T) {
	t.Parallel()
	type in struct {
		Name string `json:"name"`
	}
	cases := []struct {
		name, ct, body string
		ok             bool
	}{
		{"json", "application/json", `{"name":"a"}`, true},
		{"json with charset", "application/json; charset=utf-8", `{"name":"a"}`, true},
		{"no content type", "", `{"name":"a"}`, true},
		{"form", "application/x-www-form-urlencoded", `name=a`, false},
		{"unknown field", "application/json", `{"name":"a","x":1}`, false},
		{"trailing data", "application/json", `{"name":"a"}{}`, false},
		{"not json", "application/json", `nope`, false},
		{"too large", "application/json", `{"name":"` + strings.Repeat("a", MaxBody) + `"}`, false},
	}
	for _, c := range cases {
		r := httptest.NewRequest("POST", "/", strings.NewReader(c.body))
		if c.ct != "" {
			r.Header.Set("Content-Type", c.ct)
		}
		var v in
		err := Decode(r, &v)
		if (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok = %v", c.name, err, c.ok)
		}
		var e *Error
		if err != nil && (!errors.As(err, &e) || e.Code != CodeInvalidInput) {
			t.Errorf("%s: err = %v, want invalid_input", c.name, err)
		}
	}
}

func TestWriteError(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	WriteError(w, r, RateLimited("Wait.", 1500*time.Millisecond))
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") != "2" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("rate limited: %d %v", w.Code, w.Header())
	}
	w = httptest.NewRecorder()
	WriteError(w, r, errors.New("database on fire"))
	var body Error
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != http.StatusInternalServerError || body.Code != CodeInternal || strings.Contains(w.Body.String(), "fire") {
		t.Fatalf("internal: %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	w.Header().Set("Cache-Control", "public, max-age=60")
	WriteJSON(w, http.StatusOK, map[string]string{})
	if w.Header().Get("Cache-Control") != "public, max-age=60" {
		t.Fatalf("handler's Cache-Control replaced: %v", w.Header())
	}
}

func TestHandlerWritesFieldErrors(t *testing.T) {
	t.Parallel()
	h := Handler(func(http.ResponseWriter, *http.Request) error {
		return FieldError(CodeConflict, "email", "That email is taken.")
	})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/", nil))
	var body Error
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusConflict || body.Code != CodeConflict || body.Fields["email"] != "That email is taken." {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if !IsCode(FieldError(CodeConflict, "x", "y"), CodeConflict) || IsCode(errors.New("x"), CodeConflict) {
		t.Fatal("IsCode")
	}
	if CheckFields(nil) != nil || !IsCode(CheckFields(map[string]string{"a": "b"}), CodeInvalidInput) {
		t.Fatal("CheckFields")
	}
}
