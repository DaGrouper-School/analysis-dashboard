package registry

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagrouper-school/sam/client"
)

// fakeSAM answers reads of the registry, counting them; while down it
// fails, and while held each read waits for release.
type fakeSAM struct {
	mu      sync.Mutex
	now     time.Time
	down    bool
	reads   atomic.Int32
	hold    chan struct{}
	started chan struct{}
}

func newFake() *fakeSAM { return &fakeSAM{now: time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)} }

func (f *fakeSAM) clock() time.Time { f.mu.Lock(); defer f.mu.Unlock(); return f.now }

func (f *fakeSAM) add(d time.Duration) { f.mu.Lock(); defer f.mu.Unlock(); f.now = f.now.Add(d) }

func (f *fakeSAM) setDown(down bool) { f.mu.Lock(); defer f.mu.Unlock(); f.down = down }

func (f *fakeSAM) fetch(ctx context.Context) (*SAM, error) {
	n := f.reads.Add(1)
	if f.hold != nil {
		f.started <- struct{}{}
		<-f.hold
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return nil, errors.New("SAM is down")
	}
	return &SAM{FetchedAt: f.now, Parameters: make([]client.Parameter, n)}, ctx.Err()
}

// TestCacheSharesARead: requests that miss together wait on one read.
func TestCacheSharesARead(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.hold, f.started = make(chan struct{}), make(chan struct{}, 1)
	c := NewCache(f.fetch, f.clock)
	var wg sync.WaitGroup
	got := make([]*SAM, 5)
	for i := range got {
		wg.Go(func() {
			s, err := c.Get(context.Background())
			if err != nil {
				t.Error(err)
			}
			got[i] = s
		})
	}
	<-f.started
	// Let every request reach the read under way before it ends.
	time.Sleep(50 * time.Millisecond)
	close(f.hold)
	wg.Wait()
	if n := f.reads.Load(); n != 1 {
		t.Fatalf("%d reads", n)
	}
	for _, s := range got {
		if s != got[0] {
			t.Fatal("requests got different copies")
		}
	}
}

// TestCacheRefreshes: a copy is served until RefreshInterval, or until
// Refresh, then SAM is read again.
func TestCacheRefreshes(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := NewCache(f.fetch, f.clock)
	ctx := context.Background()
	first, _ := c.Get(ctx)
	f.add(RefreshInterval - time.Second)
	if s, _ := c.Get(ctx); s != first || f.reads.Load() != 1 {
		t.Fatalf("a fresh copy was read again: %d reads", f.reads.Load())
	}
	f.add(time.Second)
	second, _ := c.Get(ctx)
	if second == first || f.reads.Load() != 2 {
		t.Fatalf("a stale copy was served: %d reads", f.reads.Load())
	}
	c.Refresh()
	if s, _ := c.Get(ctx); s == second || f.reads.Load() != 3 {
		t.Fatalf("Refresh did not read SAM again: %d reads", f.reads.Load())
	}
}

// TestCacheWhileSAMIsDown: the copy from before is served, and SAM is asked
// again only after RetryAfter; with no copy, the failure is the answer.
func TestCacheWhileSAMIsDown(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := NewCache(f.fetch, f.clock)
	ctx := context.Background()
	f.setDown(true)
	if _, err := c.Get(ctx); err == nil {
		t.Fatal("no copy and SAM down, yet no error")
	}
	if _, err := c.Get(ctx); err == nil || f.reads.Load() != 1 {
		t.Fatalf("SAM was asked again at once: %d reads, %v", f.reads.Load(), err)
	}
	f.add(RetryAfter)
	f.setDown(false)
	copy, err := c.Get(ctx)
	if err != nil || f.reads.Load() != 2 {
		t.Fatalf("after RetryAfter: %d reads, %v", f.reads.Load(), err)
	}

	f.setDown(true)
	f.add(RefreshInterval)
	if s, err := c.Get(ctx); s != copy || err != nil || f.reads.Load() != 3 {
		t.Fatalf("a failed refresh: %d reads, %v", f.reads.Load(), err)
	}
	f.add(RetryAfter - time.Second)
	if s, _ := c.Get(ctx); s != copy || f.reads.Load() != 3 {
		t.Fatalf("SAM was asked again within RetryAfter: %d reads", f.reads.Load())
	}
	f.add(time.Second)
	f.setDown(false)
	if s, _ := c.Get(ctx); s == copy || f.reads.Load() != 4 {
		t.Fatalf("SAM was not asked again after RetryAfter: %d reads", f.reads.Load())
	}
}

// TestCacheRequestGivesUp: a request that ends while the read is under way
// stops waiting; the read goes on for the others.
func TestCacheRequestGivesUp(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.hold, f.started = make(chan struct{}), make(chan struct{}, 1)
	c := NewCache(f.fetch, f.clock)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { _, err := c.Get(ctx); done <- err }()
	<-f.started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("the request: %v", err)
	}
	close(f.hold)
	if s, err := c.Get(context.Background()); err != nil || s == nil || f.reads.Load() != 1 {
		t.Fatalf("the read under way: %v, %d reads", err, f.reads.Load())
	}
}
