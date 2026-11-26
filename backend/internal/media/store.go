package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Memory is a store in memory, for tests and for development without an
// object store.
type Memory struct {
	mu    sync.Mutex
	files map[string][]byte
}

// NewMemory is an empty store in memory.
func NewMemory() *Memory { return &Memory{files: map[string][]byte{}} }

// Put keeps data under name.
func (s *Memory) Put(_ context.Context, name, _ string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[name] = bytes.Clone(data)
	return nil
}

// Get is the file under name.
func (s *Memory) Get(_ context.Context, name string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.files[name]
	if !ok {
		return nil, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// S3Config is how to reach an S3-compatible object store.
type S3Config struct {
	// Endpoint is its address, such as http://media:8333; https unless
	// it says http.
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	// Region is the bucket's region, where the store asks for one.
	Region string
}

// S3 is a store in an S3-compatible bucket.
type S3 struct {
	client *minio.Client
	bucket string
}

// NewS3 reaches the store cfg names and makes its bucket when it has none.
func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	host, secure := cfg.Endpoint, true
	if h, ok := strings.CutPrefix(host, "http://"); ok {
		host, secure = h, false
	} else if h, ok := strings.CutPrefix(host, "https://"); ok {
		host = h
	}
	client, err := minio.New(host, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: secure, Region: cfg.Region})
	if err != nil {
		return nil, fmt.Errorf("media store: %w", err)
	}
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("media store: reaching bucket %s: %w", cfg.Bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			return nil, fmt.Errorf("media store: making bucket %s: %w", cfg.Bucket, err)
		}
	}
	return &S3{client: client, bucket: cfg.Bucket}, nil
}

// Put keeps data under name. A name holds the same bytes whenever it is
// written, so writing it again changes nothing.
func (s *S3) Put(ctx context.Context, name, contentType string, data []byte) error {
	_, err := s.client.PutObject(ctx, s.bucket, name, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("media store: storing %s: %w", name, err)
	}
	return nil
}

// Get is the file under name.
func (s *S3) Get(ctx context.Context, name string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, name, minio.GetObjectOptions{})
	if err == nil {
		_, err = obj.Stat()
	}
	var resp minio.ErrorResponse
	if errors.As(err, &resp) && resp.Code == "NoSuchKey" {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("media store: reading %s: %w", name, err)
	}
	return obj, nil
}
