// Package accounts is the accounts and sign-in module: the people who use
// the service, their participant IDs and their principals in SAM. It
// registers each user's principal through the outbox, guards participant
// IDs after a database restore, and tells the outbox worker who a user is
// in SAM. See docs/design/sam-integration.md#participant-ids-and-the-restore-guard.
//
// The module has the shape every module has (docs/design/README.md,
// "Modules"): service functions here, the only queries on its tables in
// store.go, and how SAM reflects its users in sam.go.
package accounts

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// Module is the accounts module.
type Module struct {
	db    db.Querier
	sam   *samaccess.Access
	now   func() time.Time
	guard *Guard
}

// New builds the module.
func New(q db.Querier, sam *samaccess.Access, now func() time.Time) *Module {
	return &Module{db: q, sam: sam, now: now, guard: &Guard{db: q, sam: sam}}
}

// Routes serves nothing yet: sign-in and the profile add the module's
// endpoints.
func (m *Module) Routes(*http.ServeMux, string) {}

// Guard is the restore guard, which the service runs at start-up.
func (m *Module) Guard() *Guard { return m.guard }

// Background is the restore guard, run once at start-up.
func (m *Module) Background() []func(context.Context) { return []func(context.Context){m.guard.Run} }

// User is a person's account.
type User struct {
	ID            uuid.UUID
	ParticipantID int64
	// Principal is the user's SAM principal, nil until registered.
	Principal *uuid.UUID
	// Minor is whether the user is a minor, nil until onboarding says.
	Minor *bool
	// Consent is their guardian's consent, nil until asked for.
	Consent *Consent

	// samMinor and samConsent are what SAM last confirmed it holds about
	// them, so the user applier asserts only what differs.
	samMinor   *bool
	samConsent *Consent
}

// Consent is a guardian's consent, as this service keeps it. Only given and
// withdrawn reach SAM; pending and not needed are this service's own.
type Consent string

// The consent states.
const (
	ConsentPending   Consent = "pending"
	ConsentGiven     Consent = "given"
	ConsentWithdrawn Consent = "withdrawn"
	ConsentNotNeeded Consent = "not_needed"
)

func (c Consent) valid() bool {
	switch c {
	case ConsentPending, ConsentGiven, ConsentWithdrawn, ConsentNotNeeded:
		return true
	}
	return false
}

// SetMinor records whether the user is a minor, in tx, and queues it for
// SAM. Onboarding sets it, false for an adult, since SAM treats a user with
// no minor assertion as a minor. A user who becomes a minor loses access:
// without consent they may no longer submit.
func SetMinor(ctx context.Context, tx db.Querier, now time.Time, id uuid.UUID, minor bool) (User, error) {
	cur, err := lockUser(ctx, tx, id)
	if err != nil {
		return User{}, err
	}
	if err := setMinor(ctx, tx, id, minor); err != nil {
		return User{}, err
	}
	away := minor && cur.Minor != nil && !*cur.Minor
	cur.Minor = &minor
	return cur, enqueue(ctx, tx, now, cur, away)
}

// SetConsent records the user's guardian's consent, in tx, and queues it
// for SAM. Consent that was given and no longer is takes access away.
func SetConsent(ctx context.Context, tx db.Querier, now time.Time, id uuid.UUID, c Consent) (User, error) {
	if !c.valid() {
		return User{}, fmt.Errorf("%q is not a consent state", c)
	}
	cur, err := lockUser(ctx, tx, id)
	if err != nil {
		return User{}, err
	}
	if err := setConsent(ctx, tx, id, c); err != nil {
		return User{}, err
	}
	away := cur.Consent != nil && *cur.Consent == ConsentGiven && c != ConsentGiven
	cur.Consent = &c
	return cur, enqueue(ctx, tx, now, cur, away)
}

// Create makes a user in tx and queues their registration in SAM. No
// request waits on SAM: until the worker registers them, they are answered
// as a person without grants.
func Create(ctx context.Context, tx db.Querier, now time.Time) (User, error) {
	u, err := insertUser(ctx, tx, uuid.New(), now)
	if err != nil {
		return User{}, err
	}
	return u, enqueue(ctx, tx, now, u, false)
}

// Get reads a user; pgx.ErrNoRows when there is none.
func Get(ctx context.Context, q db.Querier, id uuid.UUID) (User, error) { return getUser(ctx, q, id) }

// ErrRegistered is Renumber's and Adopt's answer for a user whose
// principal is already recorded: their participant ID is settled.
var ErrRegistered = errors.New("the user is registered in SAM: their participant ID is settled")

// Renumber gives a user not yet registered in SAM a new participant ID and
// queues their registration under it. It is for a user whose ID SAM
// already knows as someone else's, after a database restore.
func Renumber(ctx context.Context, q db.Querier, now time.Time, id uuid.UUID) (User, error) {
	var u User
	err := db.InTx(ctx, q, func(tx pgx.Tx) error {
		cur, err := lockUser(ctx, tx, id)
		if err != nil {
			return err
		}
		if cur.Principal != nil {
			return ErrRegistered
		}
		u = cur
		if u.ParticipantID, err = newParticipantID(ctx, tx, id); err != nil {
			return err
		}
		return enqueue(ctx, tx, now, u, false)
	})
	return u, err
}
