package accounts

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// The guard's bounds.
const (
	// maxDoublings bounds the search: it finds a run of up to 2^40 IDs
	// SAM knows above the sequence, with at most about 80 lookups.
	maxDoublings = 40
	// guardRetry and guardMaxRetry pace the guard while SAM cannot be
	// asked.
	guardRetry    = time.Second
	guardMaxRetry = time.Minute
)

// Guard moves the participant ID sequence past every ID SAM already knows,
// so a database restored from a backup never gives a new account the ID of
// a person SAM knows from before the restore. Registrations wait until it
// has run.
type Guard struct {
	db   db.Querier
	sam  *samaccess.Access
	done atomic.Bool
}

// Done reports whether the guard has run since start-up.
func (g *Guard) Done() bool { return g.done.Load() }

// Run checks until it succeeds or ctx ends, retrying while SAM cannot be
// asked. The service serves meanwhile.
func (g *Guard) Run(ctx context.Context) {
	wait := guardRetry
	for {
		err := g.Check(ctx)
		if err == nil || ctx.Err() != nil {
			return
		}
		slog.WarnContext(ctx, "participant ID guard: SAM could not be asked; registrations wait", "err", err, "retry_in", wait)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(2*wait, guardMaxRetry)
	}
}

// Check looks the IDs from the sequence's next one up in SAM and moves the
// sequence past the last it knows.
func (g *Guard) Check(ctx context.Context) error {
	next, err := nextParticipantID(ctx, g.db)
	if err != nil {
		return err
	}
	tenant := g.sam.Tenant()
	issuer := tenant.IssuerID()
	known := func(id int64) (bool, error) {
		_, err := g.sam.Service().FindUser(ctx, issuer, strconv.FormatInt(id, 10))
		if client.IsCode(err, client.CodeNotFound) {
			return false, nil
		}
		return err == nil, err
	}
	last, found, err := lastKnown(next, known)
	if err != nil {
		return err
	}
	if found {
		if err := moveParticipantIDsPast(ctx, g.db, last); err != nil {
			return err
		}
		slog.WarnContext(ctx, "participant ID guard moved the sequence: SAM knows IDs this database had not given out, "+
			"as after a database restore", "from", next, "to", last+1)
	}
	g.done.Store(true)
	return nil
}

// lastKnown is the last ID of the run SAM knows from start upward, and
// whether SAM knows start at all. IDs are given in order, so the ones
// registered above the sequence form a run: lastKnown finds its end by
// doubling the step until an ID is unknown, then halving between the last
// known and the first unknown.
func lastKnown(start int64, known func(int64) (bool, error)) (int64, bool, error) {
	ok, err := known(start)
	if err != nil || !ok {
		return 0, false, err
	}
	lo, hi := start, int64(0)
	for step := int64(1); hi == 0; step *= 2 {
		if step > 1<<maxDoublings {
			return 0, false, fmt.Errorf("SAM knows more than 2^%d participant IDs above %d", maxDoublings, start)
		}
		ok, err := known(start + step)
		if err != nil {
			return 0, false, err
		}
		if ok {
			lo = start + step
		} else {
			hi = start + step
		}
	}
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		ok, err := known(mid)
		if err != nil {
			return 0, false, err
		}
		if ok {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo, true, nil
}
