package media

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

// Path is where files are served, under the API's prefix.
const Path = "/media/"

// Routes serves the files, to anyone: a name is a hash, so a file never
// changes and is cached for good.
func (m *Media) Routes(mux *http.ServeMux, prefix string) {
	mux.Handle("GET "+prefix+Path+"{name}", httpx.Handler(m.serve))
}

func (m *Media) serve(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	name := r.PathValue("name")
	contentType, size, err := m.file(ctx, name)
	if errors.Is(err, ErrNotFound) {
		return httpx.NotFound("File not found.")
	}
	if err != nil {
		return err
	}
	body, err := m.store.Get(ctx, name)
	if errors.Is(err, ErrNotFound) {
		slog.ErrorContext(ctx, "a recorded file is missing from the media store", "name", name)
		return httpx.NotFound("File not found.")
	}
	if err != nil {
		return err
	}
	defer body.Close()
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Content-Length", strconv.FormatInt(size, 10))
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, body); err != nil {
		slog.WarnContext(ctx, "serving a file was cut short", "name", name, "err", err)
	}
	return nil
}
