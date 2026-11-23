// Package testenv runs SAM for integration tests: built from the source of
// the version go.mod names, on a database of its own, set up with `sam
// bootstrap` and with this service onboarded as its tenant. Test code only.
package testenv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

// Module and Command are SAM's module and its command, which go.mod lists
// as a tool so its version is pinned with the rest.
const (
	Module  = "github.com/dagrouper-school/sam"
	Command = Module + "/cmd/sam"
)

// Timeouts for SAM's commands and for it to answer once started.
const (
	commandTimeout = 2 * time.Minute
	startTimeout   = 30 * time.Second
	stopTimeout    = 10 * time.Second
)

// SAM is a running SAM with this service onboarded.
type SAM struct {
	// URL is where SAM answers, such as http://127.0.0.1:41234.
	URL string
	// OperatorKey is the deployment operator's key.
	OperatorKey string
	// Tenant is what onboarding printed.
	Tenant Tenant
}

// Tenant is what `sam tenant create -json` prints about this service as a
// tenant.
type Tenant struct {
	Name        string      `json:"name"`
	RootSpaceID uuid.UUID   `json:"root_space_id"`
	ServiceID   uuid.UUID   `json:"service_id"`
	IssuerIDs   []uuid.UUID `json:"issuer_ids"`
	Namespace   string      `json:"namespace"`
	AnonymousID uuid.UUID   `json:"anonymous_id"`
	Key         struct {
		Secret string `json:"key"`
	} `json:"key"`
}

// Options are how this service is onboarded: the issuer, key set and
// audience of the user tokens it signs.
type Options struct {
	Issuer    string
	KeySetURL string
	Audience  string
}

// TenantName is the name this service is onboarded under.
const TenantName = "dgs"

// Start runs SAM until t ends. Its database is a new one on the server
// DGS_TEST_DATABASE_URL names, which must have PostGIS. Like every
// database test it is skipped under -short.
func Start(t testing.TB, o Options) *SAM {
	t.Helper()
	dsn := testutil.EmptyDB(t)
	bin := binary(t)
	env := append(os.Environ(), "SAM_DATABASE_URL="+dsn, "SAM_LOG_FORMAT=text", "SAM_LOG_LEVEL=warn")

	var boot struct {
		OperatorKey struct {
			Secret string `json:"key"`
		} `json:"operator_key"`
	}
	run(t, bin, env, &boot, "bootstrap", "-json")
	s := &SAM{OperatorKey: boot.OperatorKey.Secret}
	run(t, bin, env, &s.Tenant, "tenant", "create", "-name", TenantName, "-registry",
		"-issuer", o.Issuer, "-jwks", o.KeySetURL, "-audience", o.Audience, "-json")

	addr := freeAddr(t)
	s.URL = "http://" + addr
	serve(t, bin, append(env, "SAM_LISTEN="+addr), s.URL)
	return s
}

// Service is a client acting as this service.
func (s *SAM) Service() *client.Client {
	return client.New(s.URL, client.WithKey(s.Tenant.Key.Secret))
}

// Operator is a client acting as the deployment's operator.
func (s *SAM) Operator() *client.Client {
	return client.New(s.URL, client.WithKey(s.OperatorKey))
}

// run runs one of SAM's commands to completion and decodes what it prints
// into out.
func run(t testing.TB, bin string, env []string, out any, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("sam %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), out); err != nil {
		t.Fatalf("sam %s printed %q: %v", strings.Join(args, " "), stdout.String(), err)
	}
}

// serve starts `sam serve`, waits until it answers, and stops it when t
// ends. Its log is shown when t fails.
func serve(t testing.TB, bin string, env []string, url string) {
	t.Helper()
	cmd := exec.Command(bin, "serve")
	cmd.Env = env
	var log syncBuffer
	cmd.Stdout, cmd.Stderr = &log, &log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// exited closes when SAM has exited, with its exit status in waitErr.
	exited := make(chan struct{})
	var waitErr error
	go func() { waitErr = cmd.Wait(); close(exited) }()
	t.Cleanup(func() {
		// The exit status of a stopped SAM says nothing a test needs.
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case <-exited:
		case <-time.After(stopTimeout):
			_ = cmd.Process.Kill()
			<-exited
		}
		if t.Failed() {
			t.Logf("sam serve:\n%s", log.String())
		}
	})
	if err := waitHealthy(url, exited, &waitErr); err != nil {
		t.Fatalf("sam serve: %v\n%s", err, log.String())
	}
}

// waitHealthy waits until SAM answers at url, or exits, reporting why.
// waitErr is read only once exited has closed.
func waitHealthy(url string, exited <-chan struct{}, waitErr *error) error {
	sam := client.New(url)
	deadline := time.Now().Add(startTimeout)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := sam.Health(ctx)
		cancel()
		if err == nil {
			return nil
		}
		select {
		case <-exited:
			return fmt.Errorf("exited before answering: %w", errors.Join(*waitErr, err))
		default:
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("did not answer within %s: %w", startTimeout, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// freeAddr is a loopback address with a port nothing listens on.
func freeAddr(t testing.TB) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

var (
	buildOnce sync.Once
	built     string
	buildErr  error
)

// binary builds SAM's command once per test process, into a directory
// named after the module's version so later runs reuse it.
func binary(t testing.TB) string {
	t.Helper()
	buildOnce.Do(func() { built, buildErr = build() })
	if buildErr != nil {
		t.Fatalf("building SAM: %v", buildErr)
	}
	return built
}

func build() (string, error) {
	version, err := goOutput("list", "-m", "-f", "{{.Version}}", Module)
	if err != nil {
		return "", err
	}
	if version == "" {
		// A replace directive points at a local checkout: build afresh.
		version = fmt.Sprintf("local-%d", os.Getpid())
	}
	dir := filepath.Join(os.TempDir(), "dgs-testenv", version)
	bin := filepath.Join(dir, "sam")
	if _, err := os.Stat(bin); err == nil && !strings.HasPrefix(version, "local-") {
		return bin, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Build beside the binary and rename, so a parallel package never runs
	// a half-written file.
	tmp := fmt.Sprintf("%s.%d", bin, os.Getpid())
	if _, err := goOutput("build", "-o", tmp, Command); err != nil {
		return "", err
	}
	return bin, os.Rename(tmp, bin)
}

func goOutput(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// syncBuffer is a buffer a running command and the test can share.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
