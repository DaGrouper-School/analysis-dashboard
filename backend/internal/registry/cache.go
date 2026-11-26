package registry

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// RefreshInterval is how long a copy of SAM's registry is served before it
// is read again. The registry changes rarely, and this tenant's own changes
// refresh it when they reach SAM.
const RefreshInterval = 10 * time.Minute

// RetryAfter is how long a failed read of SAM's registry is not repeated,
// while the copy from before is served.
const RetryAfter = 10 * time.Second

// fetchTimeout bounds one read of SAM's registry, which requests share.
const fetchTimeout = 30 * time.Second

// Cache holds the last copy of SAM's registry. Concurrent misses share one
// read; when SAM cannot be reached, the copy from before is served and SAM
// is asked again after RetryAfter.
type Cache struct {
	fetch func(context.Context) (*SAM, error)
	now   func() time.Time

	mu       sync.Mutex
	copy     *SAM
	stale    bool
	failedAt time.Time
	// reading is closed when the read under way ends.
	reading chan struct{}
	readErr error
}

// NewCache is a cache that reads SAM's registry with fetch.
func NewCache(fetch func(context.Context) (*SAM, error), now func() time.Time) *Cache {
	return &Cache{fetch: fetch, now: now}
}

// Get is SAM's registry: the copy held while it is fresh, a new read
// otherwise, or the copy held when that read fails.
func (c *Cache) Get(ctx context.Context) (*SAM, error) {
	c.mu.Lock()
	if s := c.usable(); s != nil {
		c.mu.Unlock()
		return s, nil
	}
	if c.copy == nil && c.recentlyFailed() {
		err := c.readErr
		c.mu.Unlock()
		return nil, err
	}
	if c.reading == nil {
		c.reading = make(chan struct{})
		go c.read(context.WithoutCancel(ctx))
	}
	reading := c.reading
	c.mu.Unlock()

	select {
	case <-reading:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.copy != nil {
		return c.copy, nil
	}
	return nil, c.readErr
}

// usable is the copy to serve without reading SAM, or nil. The caller
// holds mu.
func (c *Cache) usable() *SAM {
	if c.copy == nil {
		return nil
	}
	now := c.now()
	if !c.stale && now.Sub(c.copy.FetchedAt) < RefreshInterval {
		return c.copy
	}
	if c.recentlyFailed() {
		return c.copy
	}
	return nil
}

// recentlyFailed reports whether the last read failed less than RetryAfter
// ago. The caller holds mu.
func (c *Cache) recentlyFailed() bool {
	return !c.failedAt.IsZero() && c.now().Sub(c.failedAt) < RetryAfter
}

// read reads SAM's registry once for every request waiting on it.
func (c *Cache) read(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	s, err := c.fetch(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.readErr = err
	if err != nil {
		c.failedAt = c.now()
		slog.WarnContext(ctx, "reading SAM's registry failed; serving the copy from before", "err", err, "copy", c.copy != nil)
	} else {
		c.copy, c.stale, c.failedAt = s, false, time.Time{}
	}
	close(c.reading)
	c.reading = nil
}

// Refresh marks the copy stale, so the next request reads SAM again, as
// after one of this tenant's entries reaches SAM.
func (c *Cache) Refresh() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stale = true
	c.failedAt = time.Time{}
}
