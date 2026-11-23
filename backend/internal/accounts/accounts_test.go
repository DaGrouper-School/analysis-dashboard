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
	a, err := Create(ctx, pool, now, NewUser{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Create(ctx, pool, now, NewUser{})
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

	m := New(pool, nil, time.Now, Config{})
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

func consent(c Consent) *Consent { return &c }

// TestSAMConsent: SAM holds given only while consent is given, and is told
// withdrawn when consent is withdrawn, or no longer given after it was.
func TestSAMConsent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		here, held, want *Consent
	}{
		{nil, nil, nil},
		{consent(ConsentPending), nil, nil},
		{consent(ConsentNotNeeded), nil, nil},
		{consent(ConsentGiven), nil, consent(ConsentGiven)},
		{consent(ConsentGiven), consent(ConsentWithdrawn), consent(ConsentGiven)},
		{consent(ConsentWithdrawn), nil, consent(ConsentWithdrawn)},
		{consent(ConsentWithdrawn), consent(ConsentGiven), consent(ConsentWithdrawn)},
		{consent(ConsentPending), consent(ConsentGiven), consent(ConsentWithdrawn)},
		{consent(ConsentPending), consent(ConsentWithdrawn), nil},
		{nil, consent(ConsentGiven), consent(ConsentWithdrawn)},
	} {
		if got := samConsent(tc.here, tc.held); !sameConsent(got, tc.want) {
			t.Errorf("here %v, SAM holds %v: %v, want %v", deref(tc.here), deref(tc.held), deref(got), deref(tc.want))
		}
	}
}

func deref(c *Consent) string {
	if c == nil {
		return "nothing"
	}
	return string(*c)
}

// TestWhatTakesAccessAway: becoming a minor and losing given consent are
// queued as taking access away; the rest are not.
func TestWhatTakesAccessAway(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.DB(t)
	now := time.Now()
	u, err := Create(ctx, pool, now, NewUser{})
	if err != nil {
		t.Fatal(err)
	}
	away := func() bool {
		t.Helper()
		var v bool
		if err := pool.QueryRow(ctx, `SELECT takes_access_away FROM sam_outbox WHERE entity = $1 ORDER BY version DESC LIMIT 1`,
			Entity(u.ID)).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, step := range []struct {
		name string
		do   func() error
		want bool
	}{
		{"a first minor state", func() error { _, err := SetMinor(ctx, pool, now, u.ID, true); return err }, false},
		{"an adult after all", func() error { _, err := SetMinor(ctx, pool, now, u.ID, false); return err }, false},
		{"a minor again", func() error { _, err := SetMinor(ctx, pool, now, u.ID, true); return err }, true},
		{"consent asked for", func() error { _, err := SetConsent(ctx, pool, now, u.ID, ConsentPending); return err }, false},
		{"consent given", func() error { _, err := SetConsent(ctx, pool, now, u.ID, ConsentGiven); return err }, false},
		{"consent withdrawn", func() error { _, err := SetConsent(ctx, pool, now, u.ID, ConsentWithdrawn); return err }, true},
		{"consent given again", func() error { _, err := SetConsent(ctx, pool, now, u.ID, ConsentGiven); return err }, false},
		{"asked again", func() error { _, err := SetConsent(ctx, pool, now, u.ID, ConsentPending); return err }, true},
	} {
		if err := step.do(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		if got := away(); got != step.want {
			t.Errorf("%s: takes access away %v, want %v", step.name, got, step.want)
		}
	}
	got, err := Get(ctx, pool, u.ID)
	if err != nil || got.Minor == nil || !*got.Minor || got.Consent == nil || *got.Consent != ConsentPending {
		t.Fatalf("the user: %+v, %v", got, err)
	}
	if _, err := SetConsent(ctx, pool, now, u.ID, "maybe"); err == nil {
		t.Fatal("an unknown consent state was accepted")
	}
}
