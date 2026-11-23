package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// How SAM reflects the module's users: each user's principal, kept by the
// user outbox kind, sent as this service by policy.

// UserKind is the outbox kind that keeps a user's SAM principal.
const UserKind = "user"

// entityPrefix starts a user's outbox entity, user:<user id>.
const entityPrefix = "user:"

// Entity is the outbox entity of a user's SAM principal.
func Entity(id uuid.UUID) string { return entityPrefix + id.String() }

// userState is the desired state of a user's principal.
type userState struct {
	ParticipantID int64 `json:"participant_id"`
}

func enqueue(ctx context.Context, tx db.Querier, now time.Time, u User) error {
	_, err := samaccess.Enqueue(ctx, tx, now, samaccess.Change{Entity: Entity(u.ID), Kind: UserKind,
		Payload: userState{ParticipantID: u.ParticipantID}})
	return err
}

// OutboxKinds is the user kind.
func (m *Module) OutboxKinds() []samaccess.Kind {
	return []samaccess.Kind{{Name: UserKind, Apply: m.applyUser}}
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

// applyUser registers the user's principal under their participant ID.
//
// The registration's idempotency key names the user and the version, so
// when an answer is lost the retry gets SAM's first answer back: the user's
// own principal, which is adopted. SAM answers 409 already_exists for a
// principal registered by any other request: after a database restore,
// either someone this database no longer knows, or this user's own
// registration whose answer is no longer replayed. Only an operator can
// tell which, so the change is held, never adopted on its own.
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
	if cur.Principal != nil {
		return nil
	}
	tenant := m.sam.Tenant()
	issuer := tenant.IssuerID()
	reg, err := a.SAM.Register(a.WithKey(ctx, "principal"), client.PrincipalInput{Type: "user",
		HomeSpaceID: tenant.RootSpaceID, IssuerID: &issuer, Subject: strconv.FormatInt(want.ParticipantID, 10)})
	if client.IsCode(err, client.CodeAlreadyExists) {
		return samaccess.Hold(fmt.Errorf("SAM already has a principal under participant ID %d. If this user was restored from a "+
			"backup and registered after it, it is theirs: `dgs accounts adopt %s`. Otherwise it is someone else's: "+
			"`dgs accounts renumber %s` (see docs/operations.md, \"After a database restore\")", want.ParticipantID, id, id))
	}
	if err != nil {
		return err
	}
	return setPrincipal(ctx, m.db, id, reg.Principal.ID, want.ParticipantID)
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
