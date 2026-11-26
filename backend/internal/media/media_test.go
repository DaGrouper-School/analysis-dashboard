package media

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"testing"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 200, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewGray(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func dataURL(contentType string, data []byte) string {
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func TestParseImage(t *testing.T) {
	t.Parallel()
	p, j := pngBytes(t), jpegBytes(t)
	webp := append([]byte("RIFF\x10\x00\x00\x00WEBPVP8 "), make([]byte, 8)...)
	for _, c := range []struct {
		url, want string
	}{{dataURL("image/png", p), "image/png"}, {dataURL("image/jpeg", j), "image/jpeg"}, {dataURL("image/webp", webp), "image/webp"}} {
		img, err := ParseImage(c.url)
		if err != nil || img.ContentType != c.want || len(img.Data) == 0 {
			t.Errorf("%s: %+v, %v", c.want, img.ContentType, err)
		}
	}
	for name, c := range map[string]struct {
		url  string
		want error
	}{
		"not a data URL":          {"https://example.org/x.png", ErrNotImage},
		"not base64":              {"data:image/png,abc", ErrNotImage},
		"a type it does not take": {dataURL("image/gif", []byte("GIF89a")), ErrNotImage},
		"bytes of another type":   {dataURL("image/png", j), ErrNotImage},
		"text called an image":    {dataURL("image/jpeg", []byte("<svg></svg>")), ErrNotImage},
		"bad base64":              {"data:image/png;base64,***", ErrNotImage},
		"too large":               {dataURL("image/png", append(p, make([]byte, MaxImageBytes)...)), ErrTooLarge},
	} {
		if _, err := ParseImage(c.url); !errors.Is(err, c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestNameAndMemory(t *testing.T) {
	t.Parallel()
	p := pngBytes(t)
	name := Name("image/png", p)
	if name != Name("image/png", bytes.Clone(p)) || !strings.HasSuffix(name, ".png") || len(name) != 64+4 {
		t.Fatalf("a name: %s", name)
	}
	s := NewMemory()
	ctx := context.Background()
	if _, err := s.Get(ctx, name); !errors.Is(err, ErrNotFound) {
		t.Errorf("before: %v", err)
	}
	if err := s.Put(ctx, name, "image/png", p); err != nil {
		t.Fatal(err)
	}
	r, err := s.Get(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(r)
	if !bytes.Equal(got, p) {
		t.Error("the bytes back")
	}
}
