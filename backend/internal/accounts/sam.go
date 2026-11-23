package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// How SAM reflects the module's users: each user's principal and what SAM
// must hold about them, minor and consent, kept by the user outbox kind and
// sent as this service by policy, and compared with SAM by the reconciler.

// UserKind is the outbox kind that keeps a user in SAM.
const UserKind = "user"

// entityPrefix starts a user's outbox entity, user:<user id>.
const entityPrefix = "user:"

// Entity is the outbox entity of a user in SAM.
func Entity(id uuid.UUID) string { return entityPrefix + id.String() }

// SAM's attribute names and values (SAM's docs/access.md, "Attribute
// assertions").
const (
	attrMinor   = "minor"
	attrConsent = "consent"
)

// userState is the desired state of a user in SAM.
type userState struct {
	ParticipantID int64 `json:"participant_id"`
	// Minor is whether they are a minor; nil asserts nothing yet.
	Minor *bool `json:"minor"`
	// Consent is this service's consent state.
	Consent *Consent `json:"consent"`
}

func enqueue(ctx context.Context, tx db.Querier, now time.Time, u User, takesAccessAway bool) error {
	_, err := samaccess.Enqueue(ctx, tx, now, samaccess.Change{Entity: Entity(u.ID), Kind: UserKind,
		Payload: userState{ParticipantID: u.ParticipantID, Minor: u.Minor, Consent: u.Consent}, TakesAccessAway: takesAccessAway})
	return err
}

// samConsent is the consent SAM must hold, given this service's state and
// what SAM holds now: given only when consent is given; withdrawn when it
// is withdrawn, or when SAM holds given and this service's state is no
// longer given, such as pending after a guardian is asked again; otherwise
// nothing need be asserted.
func samConsent(c *Consent, held *Consent) *Consent {
	given, withdrawn := ConsentGiven, ConsentWithdrawn
	switch {
	case c != nil && *c == ConsentGiven:
		return &given
	case c != nil && *c == ConsentWithdrawn:
		return &withdrawn
	case held != nil && *held == ConsentGiven:
		return &withdrawn
	}
	return nil
}

func sameBool(a, b *bool) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }

func sameConsent(a, b *Consent) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }

// OutboxKinds is the user kind.
func (m *Module) OutboxKinds() []samaccess.Kind {
	return []samaccess.Kind{{Name: UserKind, Apply: m.applyUser}}
}

// Checks is the reconciler's comparison of users with SAM.
func (m *Module) Checks() []samaccess.Check {
	return []samaccess.Check{{Name: "users", Run: m.reconcileUsers}}
}

// Person is the user's SAM identity, for changes sent as them. It
// implements samaccess.Actors.
func (m *Module) Person(ctx context.Context, id uuid.UUID) (samaccess.Person, error) {
	u, err := getUser(ctx, m.db, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return samaccess.Person{}, samaccess.ErrActorGone
	}
	if err != nil {
		return samaccess.Person{}, err
	}
	return samaccess.Person{ParticipantID: u.ParticipantID, Registered: u.Principal != nil}, nil
}

// applyUser brings the user's principal in SAM to the desired state:
// registered under their participant ID, then minor and consent asserted
// at the tenant root where they differ from what SAM holds. Assertions are
// append-only in SAM, so one that matches is never sent again. Each request
// has its own idempotency key, so a retry resumes where an attempt stopped.
func (m *Module) applyUser(ctx context.Context, a samaccess.Attempt) error {
	if !m.guard.Done() {
		return samaccess.ErrNotYet
	}
	var want userState
	if err := json.Unmarshal(a.Payload, &want); err != nil {
		return samaccess.Hold(fmt.Errorf("the user change: %w", err))
	}
	id, err := uuid.Parse(strings.TrimPrefix(a.Entity, entityPrefix))
	if err != nil {
		return samaccess.Hold(fmt.Errorf("the user change's entity %q: %w", a.Entity, err))
	}
	cur, err := getUser(ctx, m.db, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return samaccess.Hold(fmt.Errorf("user %s no longer exists", id))
	}
	if err != nil {
		return err
	}
	principal := cur.Principal
	if principal == nil {
		if principal, err = m.register(ctx, a, id, want.ParticipantID); err != nil {
			return err
		}
	}
	root := m.sam.Tenant().RootSpaceID
	if want.Minor != nil && !sameBool(cur.samMinor, want.Minor) {
		if _, err := a.SAM.Assert(a.WithKey(ctx, "attr:"+attrMinor), *principal, client.AssertionInput{Name: attrMinor,
			Value: strconv.FormatBool(*want.Minor), ScopeSpaceID: &root}); err != nil {
			return err
		}
		if err := setSAMMinor(ctx, m.db, id, want.Minor); err != nil {
			return err
		}
	}
	if c := samConsent(want.Consent, cur.samConsent); c != nil && !sameConsent(cur.samConsent, c) {
		if _, err := a.SAM.Assert(a.WithKey(ctx, "attr:"+attrConsent), *principal, client.AssertionInput{Name: attrConsent,
			Value: string(*c), ScopeSpaceID: &root}); err != nil {
			return err
		}
		if err := setSAMConsent(ctx, m.db, id, c); err != nil {
			return err
		}
	}
	return nil
}

// register registers the user's principal under participant ID n.
//
// The registration's idempotency key names the user and the version, so
// when an answer is lost the retry gets SAM's first answer back: the user's
// own principal, which is adopted. SAM answers 409 already_exists for a
// principal registered by any other request: after a database restore,
// either someone this database no longer knows, or this user's own
// registration whose answer is no longer replayed. Only an operator can
// tell which, so the change is held, never adopted on its own.
func (m *Module) register(ctx context.Context, a samaccess.Attempt, id uuid.UUID, n int64) (*uuid.UUID, error) {
	tenant := m.sam.Tenant()
	issuer := tenant.IssuerID()
	reg, err := a.SAM.Register(a.WithKey(ctx, "principal"), client.PrincipalInput{Type: "user",
		HomeSpaceID: tenant.RootSpaceID, IssuerID: &issuer, Subject: strconv.FormatInt(n, 10)})
	if client.IsCode(err, client.CodeAlreadyExists) {
		return nil, samaccess.Hold(fmt.Errorf("SAM already has a principal under participant ID %d. If this user was restored from a "+
			"backup and registered after it, it is theirs: `dgs accounts adopt %s`. Otherwise it is someone else's: "+
			"`dgs accounts renumber %s` (see docs/operations.md, \"After a database restore\")", n, id, id))
	}
	if err != nil {
		return nil, err
	}
	if err := setPrincipal(ctx, m.db, id, reg.Principal.ID, n); err != nil {
		return nil, err
	}
	return &reg.Principal.ID, nil
}

// reconcilePage is how many users the reconciler compares at a time.
const reconcilePage = 100

// reconcileUsers compares every user with SAM: a user without a principal
// and with nothing queued is queued; one whose minor or consent in SAM
// differs from what this service last confirmed is corrected, by recording
// what SAM actually holds and queuing the user, so the applier asserts the
// difference. A user it cannot compare is logged and compared again on the
// next pass. Each read of a user's attributes is audited in SAM, which is
// why the pass runs on an interval rather than continually.
func (m *Module) reconcileUsers(ctx context.Context) error {
	if !m.guard.Done() {
		return nil
	}
	after := uuid.Nil
	for {
		page, err := usersAfter(ctx, m.db, after, reconcilePage)
		if err != nil {
			return err
		}
		for _, u := range page {
			if err := m.reconcileUser(ctx, u); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				slog.WarnContext(ctx, "reconciler could not compare a user with SAM", "user", u.ID, "err", err)
			}
		}
		if len(page) < reconcilePage {
			return nil
		}
		after = page[len(page)-1].ID
	}
}

func (m *Module) reconcileUser(ctx context.Context, u User) error {
	waiting, err := samaccess.Waiting(ctx, m.db, Entity(u.ID))
	if err != nil || waiting {
		return err
	}
	if u.Principal == nil {
		return m.requeue(ctx, u, false, "it has no principal and nothing queued")
	}
	root := m.sam.Tenant().RootSpaceID
	held, err := m.sam.Service().Attributes(ctx, *u.Principal, &root)
	if err != nil {
		return err
	}
	var minor *bool
	var consent *Consent
	for _, a := range held {
		switch a.Name {
		case attrMinor:
			v := a.Value == "true"
			minor = &v
		case attrConsent:
			v := Consent(a.Value)
			consent = &v
		}
	}
	minorOff := u.Minor != nil && !sameBool(minor, u.Minor)
	want := samConsent(u.Consent, consent)
	consentOff := want != nil && !sameConsent(consent, want)
	if !minorOff && !consentOff {
		return nil
	}
	if err := setSAMMinor(ctx, m.db, u.ID, minor); err != nil {
		return err
	}
	if err := setSAMConsent(ctx, m.db, u.ID, consent); err != nil {
		return err
	}
	// SAM allows more than this service while it holds consent given that
	// is no longer given, or minor false for a minor.
	away := (consent != nil && *consent == ConsentGiven && *want == ConsentWithdrawn) ||
		(minor != nil && !*minor && u.Minor != nil && *u.Minor)
	return m.requeue(ctx, u, away, "SAM holds a different minor or consent")
}

// requeue queues the user's desired state again, marked as taking access
// away when SAM now allows more than this service.
func (m *Module) requeue(ctx context.Context, u User, away bool, why string) error {
	slog.WarnContext(ctx, "reconciler queued a user for SAM", "user", u.ID, "why", why)
	return db.InTx(ctx, m.db, func(tx pgx.Tx) error {
		cur, err := lockUser(ctx, tx, u.ID)
		if err != nil {
			return err
		}
		return enqueue(ctx, tx, m.now(), cur, away)
	})
}

// ErrNotInSAM is Adopt's answer when SAM has no principal under the user's
// participant ID.
var ErrNotInSAM = errors.New("SAM has no principal under the user's participant ID")

// Adopt records as the user's own the principal SAM has under their
// participant ID, and sends their held registration again, so it completes.
// It is for a user restored from a backup whose registration reached SAM
// after the backup was taken: the registration is held because its answer
// is no longer replayed, yet the principal is theirs. An operator adopts
// only after checking that: a principal belonging to someone else must
// never be adopted. It returns the principal adopted.
func (m *Module) Adopt(ctx context.Context, id uuid.UUID) (client.Principal, error) {
	cur, err := getUser(ctx, m.db, id)
	if err != nil {
		return client.Principal{}, err
	}
	if cur.Principal != nil {
		return client.Principal{}, ErrRegistered
	}
	p, err := m.sam.Service().FindUser(ctx, m.sam.Tenant().IssuerID(), strconv.FormatInt(cur.ParticipantID, 10))
	if client.IsCode(err, client.CodeNotFound) {
		return client.Principal{}, ErrNotInSAM
	}
	if err != nil {
		return client.Principal{}, err
	}
	if err := setPrincipal(ctx, m.db, id, p.ID, cur.ParticipantID); err != nil {
		return client.Principal{}, err
	}
	if err := samaccess.Release(ctx, m.db, m.now(), Entity(id)); err != nil && !errors.Is(err, samaccess.ErrNothingHeld) {
		return client.Principal{}, err
	}
	return p, nil
}
