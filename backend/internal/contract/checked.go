package contract

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
)

// Checked wraps h so that every response to a request under Prefix is
// checked against the contract, and each breach is passed to report: a
// response the operation does not allow, or a request for a path the
// contract does not have. Paths under one of outside, such as the key set
// SAM fetches, lie outside the contract and pass unchecked; so do paths
// outside Prefix, such as /healthz.
func (c *Contract) Checked(h http.Handler, report func(error), outside ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if !strings.HasPrefix(path, Prefix+"/") || hasAnyPrefix(path, outside) {
			h.ServeHTTP(w, r)
			return
		}
		op := c.Match(r.Method, path)
		if op == nil {
			report(fmt.Errorf("%s %s is not in the contract", r.Method, path))
			h.ServeHTTP(w, r)
			return
		}
		rec := &recorder{ResponseWriter: w}
		h.ServeHTTP(rec, r)
		if err := op.Check(rec.code(), rec.sent, rec.body.Bytes()); err != nil {
			report(err)
		}
	})
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// recorder passes a response through while keeping a copy of it.
type recorder struct {
	http.ResponseWriter
	status int
	sent   http.Header
	body   bytes.Buffer
}

func (r *recorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
		r.sent = r.Header().Clone()
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(http.StatusOK)
	}
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the connection.
func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (r *recorder) code() int {
	if r.status == 0 {
		r.sent = r.Header().Clone()
		return http.StatusOK
	}
	return r.status
}
