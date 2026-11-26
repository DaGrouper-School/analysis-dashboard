// Package media keeps stored files, such as institution photos, in an
// object store, each named by its content hash, and serves them at
// /api/media/{name} (docs/design/dashboards-and-public-site.md#photos).
// The media table records what each file belongs to.
package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// MaxImageBytes bounds an image: about what the front end's largest data
// URL holds.
const MaxImageBytes = 1_500_000

// KindInstitutionPhoto is the kind of an institution's photo, as the media
// table records what each file belongs to.
const KindInstitutionPhoto = "institution_photo"

// images are the types an image may be, by its bytes, and their names'
// extensions.
var images = map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}

// Problem is what is wrong with an image, in words for the person who
// sent it.
type Problem string

func (p Problem) Error() string { return string(p) }

// Problems with an image, for the field it was sent in.
const (
	ErrNotImage Problem = "Choose a PNG, JPEG or WebP image."
	ErrTooLarge Problem = "That image is too large. Choose a smaller one."
)

// Image is an image read from a data URL.
type Image struct {
	ContentType string
	Data        []byte
}

// ParseImage reads a data URL (data:image/…;base64,…) as an image: a PNG,
// JPEG or WebP by its bytes as well as by what it says, of at most
// MaxImageBytes.
func ParseImage(dataURL string) (Image, error) {
	head, body, ok := strings.Cut(dataURL, ",")
	declared, isBase64 := strings.CutSuffix(strings.TrimPrefix(head, "data:"), ";base64")
	if !ok || !strings.HasPrefix(head, "data:") || !isBase64 || images[declared] == "" {
		return Image{}, ErrNotImage
	}
	if base64.StdEncoding.DecodedLen(len(body)) > MaxImageBytes+3 {
		return Image{}, ErrTooLarge
	}
	data, err := base64.StdEncoding.DecodeString(body)
	switch {
	case err != nil || len(data) == 0:
		return Image{}, ErrNotImage
	case len(data) > MaxImageBytes:
		return Image{}, ErrTooLarge
	case sniff(data) != declared:
		return Image{}, ErrNotImage
	}
	return Image{ContentType: declared, Data: data}, nil
}

// sniff is the type data's bytes say it is.
func sniff(data []byte) string {
	if len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "image/webp"
	}
	return http.DetectContentType(data)
}

// Name is the name a file is kept under: the hash of its bytes, with its
// type's extension, so a name never changes what it holds.
func Name(contentType string, data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) + images[contentType]
}

// ErrNotFound is a file the store does not hold.
var ErrNotFound = errors.New("media: no such file")

// Store keeps files by name.
type Store interface {
	Put(ctx context.Context, name, contentType string, data []byte) error
	// Get is the file's bytes; ErrNotFound when it holds none by name.
	Get(ctx context.Context, name string) (io.ReadCloser, error)
}

// Media keeps files in a store and records them in the database.
type Media struct {
	db    db.Querier
	store Store
	now   func() time.Time
}

// New keeps files in store, recorded in q.
func New(q db.Querier, store Store, now func() time.Time) *Media {
	return &Media{db: q, store: store, now: now}
}

// Save stores img for its owner and returns its name. Saving the same
// bytes again stores nothing new.
func (m *Media) Save(ctx context.Context, kind string, owner uuid.UUID, img Image) (string, error) {
	name := Name(img.ContentType, img.Data)
	if err := m.store.Put(ctx, name, img.ContentType, img.Data); err != nil {
		return "", err
	}
	_, err := m.db.Exec(ctx, `INSERT INTO media (name, content_type, size, kind, owner_id, created_at) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (name) DO NOTHING`, name, img.ContentType, len(img.Data), kind, owner, m.now())
	return name, err
}

// file is a recorded file's type and size, or ErrNotFound.
func (m *Media) file(ctx context.Context, name string) (string, int64, error) {
	var contentType string
	var size int64
	err := m.db.QueryRow(ctx, `SELECT content_type, size FROM media WHERE name = $1`, name).Scan(&contentType, &size)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, ErrNotFound
	}
	return contentType, size, err
}
