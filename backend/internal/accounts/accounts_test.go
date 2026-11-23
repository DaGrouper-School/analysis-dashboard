package accounts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

// TestLastKnown: the search finds the end of the run of IDs SAM knows,
// whatever its length, within its bound on lookups.
func TestLastKnown(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		start, last int64 // SAM knows start..last; last < start for none
	}{
		{5, 4}, {5, 5}, {5, 6}, {5, 7}, {5, 41}, {1, 1000}, {1, 1 << 20},
	} {
		lookups := 0
		known := func(id int64) (bool, error) {
			lookups++
			return id >= tc.start && id <= tc.last, nil
		}
		last, found, err := lastKnown(tc.start, known)
		switch {
		case err != nil:
			t.Errorf("%d..%d: %v", tc.start, tc.last, err)
		case tc.last < tc.start && found:
			t.Errorf("none known: found %d", last)
		case tc.last >= tc.start && (!found || last != tc.last):
			t.Errorf("%d..%d: found %v, %d", tc.start, tc.last, found, last)
		case lookups > 2*maxDoublings+2:
			t.Errorf("%d..%d: %d lookups", tc.start, tc.last, lookups)
		}
	}
	boom := errors.New("SAM is down")
	if _, _, err := lastKnown(1, func(int64) (bool, error) { return false, boom }); !errors.Is(err, boom) {
		t.Errorf("an error asking SAM: %v", err)
	}
	if _, _, err := lastKnown(1, func(int64) (bool, error) { return true, nil }); err == nil {
		t.Error("an endless run was accepted")
	}
}

// TestAccounts: participant IDs are given in order, an account's
// registration is queued with it, the worker learns who an account is, and
// renumbering gives a new ID to an account SAM does not have yet.
func TestAccounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.DB(t)
	now := time.Now()
	a, err := Create(ctx, pool, now)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Create(ctx, pool, now)
	if err != nil {
		t.Fatal(err)
	}
	if a.ParticipantID != 1 || b.ParticipantID != 2 {
		t.Fatalf("participant IDs %d and %d", a.ParticipantID, b.ParticipantID)
	}
	var kind string
	var version int64
	if err := pool.QueryRow(ctx, `SELECT kind, version FROM sam_outbox WHERE entity = $1`, Entity(a.ID)).Scan(&kind, &version); err != nil ||
		kind != UserKind || version != 1 {
		t.Fatalf("the queued registration: %s v%d, %v", kind, version, err)
	}

	m := New(pool, nil, time.Now)
	if p, err := m.Person(ctx, a.ID); err != nil || p.Registered || p.ParticipantID != 1 {
		t.Fatalf("an account not yet in SAM: %+v, %v", p, err)
	}
	if _, err := m.Person(ctx, uuid.New()); !errors.Is(err, samaccess.ErrActorGone) {
		t.Fatalf("an account that does not exist: %v", err)
	}

	r, err := Renumber(ctx, pool, now, a.ID)
	if err != nil || r.ParticipantID != 3 {
		t.Fatalf("renumbered: %+v, %v", r, err)
	}
	if err := pool.QueryRow(ctx, `SELECT max(version) FROM sam_outbox WHERE entity = $1`, Entity(a.ID)).Scan(&version); err != nil || version != 2 {
		t.Fatalf("the registration under the new ID: v%d, %v", version, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET sam_principal_id = $2 WHERE id = $1`, b.ID, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if _, err := Renumber(ctx, pool, now, b.ID); !errors.Is(err, ErrRegistered) {
		t.Fatalf("renumbering an account SAM has: %v", err)
	}
	if p, err := m.Person(ctx, b.ID); err != nil || !p.Registered {
		t.Fatalf("a registered account: %+v, %v", p, err)
	}
}
