package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

// TestS3 runs against the S3-compatible store DGS_TEST_MEDIA_URL names,
// such as the compose file's test-media (make test-media), and is
// skipped without one.
func TestS3(t *testing.T) {
	t.Parallel()
	endpoint := os.Getenv("DGS_TEST_MEDIA_URL")
	if endpoint == "" {
		t.Skip("set DGS_TEST_MEDIA_URL to test the S3 store")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cfg := S3Config{Endpoint: endpoint, Bucket: "dgs-test-" + time.Now().Format("20060102150405"), AccessKey: "dgs-dev", SecretKey: "dgs-dev-secret"}
	var s *S3
	var err error
	// The store takes a few seconds to start taking requests.
	for range 30 {
		if s, err = NewS3(ctx, cfg); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewS3(ctx, cfg); err != nil {
		t.Fatalf("a bucket that exists: %v", err)
	}
	wrong := cfg
	wrong.SecretKey = "not-the-secret"
	if _, err := NewS3(ctx, wrong); err == nil {
		t.Error("a wrong key was let in")
	}
	p := pngBytes(t)
	name := Name("image/png", p)
	if _, err := s.Get(ctx, name); !errors.Is(err, ErrNotFound) {
		t.Errorf("before: %v", err)
	}
	for range 2 {
		if err := s.Put(ctx, name, "image/png", p); err != nil {
			t.Fatal(err)
		}
	}
	r, err := s.Get(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, _ := io.ReadAll(r)
	if !bytes.Equal(got, p) {
		t.Error("the bytes back")
	}
}
