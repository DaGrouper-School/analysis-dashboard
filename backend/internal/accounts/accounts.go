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
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// Config is how people sign in.
type Config struct {
	// GoogleClientID is this service's Google OAuth client, the audience of
	// the ID tokens it accepts. Without one, Google sign-in is refused.
	GoogleClientID string
	// GoogleKeysURL is where Google's signing keys are fetched; empty for
	// Google's own. Tests point it at a fake.
	GoogleKeysURL string
	// SessionTTL is how long an unused session lasts.
	SessionTTL time.Duration
	// SecureCookies sets Secure on the session cookie: the service is
	// reached over HTTPS.
	SecureCookies bool
	// Demo signs in the demo accounts by subject. Development and test only.
	Demo bool
}

// DefaultSessionTTL is how long an unused session lasts unless configured.
const DefaultSessionTTL = 30 * 24 * time.Hour

// Module is the accounts module.
type Module struct {
	db       db.Querier
	sam      *samaccess.Access
	now      func() time.Time
	cfg      Config
	google   *google
	guard    *Guard
	sessions *Sessions
}

// New builds the module.
func New(q db.Querier, sam *samaccess.Access, now func() time.Time, cfg Config) *Module {
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = DefaultSessionTTL
	}
	return &Module{db: q, sam: sam, now: now, cfg: cfg, google: newGoogle(cfg.GoogleClientID, cfg.GoogleKeysURL, now),
		guard: &Guard{db: q, sam: sam}, sessions: &Sessions{db: q, now: now, ttl: cfg.SessionTTL, secure: cfg.SecureCookies}}
}

// Guard is the restore guard, which the service runs at start-up.
func (m *Module) Guard() *Guard { return m.guard }

// Sessions finds who a request is from. Other modules are given it to guard
// their routes with RequireUser.
func (m *Module) Sessions() *Sessions { return m.sessions }

// Background is the restore guard, run once at start-up, and the pruning
// of ended sessions.
func (m *Module) Background() []func(context.Context) {
	return []func(context.Context){m.guard.Run, m.sessions.prune}
}

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

	// The rest of the account, as the front end's User shows it.
	AuthProvider        string
	FullName            string
	Email               *string
	EmailVerified       bool
	Phone               *string
	PhoneVerified       bool
	AvatarURL           *string
	Role                string
	PlatformAdmin       bool
	Address             []byte // JSON; nil for none
	OnboardingCompleted bool
	PublicProfile       bool
	CreatedAt           time.Time
	UpdatedAt           time.Time

	// samMinor and samConsent are what SAM last confirmed it holds about
	// them, so the user applier asserts only what differs.
	samMinor   *bool
	samConsent *Consent
}

// NewUser is what a sign-in knows about a person whose account it creates.
type NewUser struct {
	AuthProvider  string
	FullName      string
	Email         *string
	EmailVerified bool
	Phone         *string
	PhoneVerified bool
	AvatarURL     *string
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
func Create(ctx context.Context, tx db.Querier, now time.Time, n NewUser) (User, error) {
	if n.AuthProvider == "" {
		n.AuthProvider = "google"
	}
	u, err := insertUser(ctx, tx, uuid.New(), now, n)
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
