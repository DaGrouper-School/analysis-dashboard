package logx

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestRequestIDOnEveryLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l, err := New(&buf, "json", "info")
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithRequestID(context.Background(), "abc")
	l.With("k", "v").InfoContext(ctx, "hello")
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	if line["request_id"] != "abc" || line["k"] != "v" {
		t.Fatalf("line: %v", line)
	}
}

func TestNewRefuses(t *testing.T) {
	t.Parallel()
	if _, err := New(&bytes.Buffer{}, "xml", "info"); err == nil {
		t.Error("format xml accepted")
	}
	if _, err := New(&bytes.Buffer{}, "json", "loud"); err == nil {
		t.Error("level loud accepted")
	}
	if a, b := NewRequestID(), NewRequestID(); a == b || len(a) != 16 {
		t.Errorf("request IDs %q %q", a, b)
	}
}
