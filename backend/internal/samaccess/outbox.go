package samaccess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// Change is a change SAM must reflect: an entity's whole desired state, for
// the applier of its kind to send.
type Change struct {
	// Entity names what the change is about, such as user:<user id>.
	// It must not hold a participant ID or a row ID a restored database
	// could give out again, since idempotency keys are built from it.
	Entity string
	// Kind is the applier that sends it.
	Kind string
	// Payload is the entity's whole desired state, sent as JSON.
	Payload any
	// Actor is the account the change is sent as; nil for this service. A
	// kind declares which it takes.
	Actor *uuid.UUID
	// TakesAccessAway marks a change after which SAM should allow less than
	// before, such as consent withdrawn or a member removed. Until it is
	// delivered SAM is more permissive than this service, so a held one is
	// logged at error level and listed first.
	TakesAccessAway bool
}

// Enqueue writes c to the outbox in q's transaction, so it reaches SAM if
// and only if the change it reflects commits. It returns c's version: one
// more than the entity's last.
func Enqueue(ctx context.Context, q db.Querier, now time.Time, c Change) (int64, error) {
	if c.Entity == "" || c.Kind == "" {
		return 0, errors.New("an outbox change needs an entity and a kind")
	}
	payload, err := json.Marshal(c.Payload)
	if err != nil {
		return 0, fmt.Errorf("the payload of %s: %w", c.Entity, err)
	}
	var version int64
	err = q.QueryRow(ctx, `
		INSERT INTO sam_outbox_versions (entity, version) VALUES ($1, 1)
		ON CONFLICT (entity) DO UPDATE SET version = sam_outbox_versions.version + 1
		RETURNING version`, c.Entity).Scan(&version)
	if err != nil {
		return 0, err
	}
	_, err = q.Exec(ctx, `
		INSERT INTO sam_outbox (entity, kind, version, payload, actor_id, takes_access_away, next_attempt_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)`, c.Entity, c.Kind, version, payload, c.Actor, c.TakesAccessAway, now)
	return version, err
}

// Held is a change held for an operator.
type Held struct {
	Entity   string     `json:"entity"`
	Kind     string     `json:"kind"`
	Version  int64      `json:"version"`
	Actor    *uuid.UUID `json:"actor_id"`
	Attempts int        `json:"attempts"`
	// TakesAccessAway is whether SAM stays more permissive than this
	// service until the change is delivered.
	TakesAccessAway bool      `json:"takes_access_away"`
	Error           string    `json:"error"`
	CreatedAt       time.Time `json:"created_at"`
}

// HeldChanges lists the changes held for an operator: those that take
// access away first, then the rest, each oldest first.
func HeldChanges(ctx context.Context, q db.Querier) ([]Held, error) {
	rows, err := q.Query(ctx, `
		SELECT entity, kind, version, actor_id, attempts, takes_access_away, coalesce(last_error, ''), created_at
		FROM sam_outbox WHERE state = 'held' ORDER BY takes_access_away DESC, created_at, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Held, error) {
		var h Held
		err := r.Scan(&h.Entity, &h.Kind, &h.Version, &h.Actor, &h.Attempts, &h.TakesAccessAway, &h.Error, &h.CreatedAt)
		return h, err
	})
}

// ErrNothingHeld is Release's answer for an entity with no held change.
var ErrNothingHeld = errors.New("nothing is held for that entity")

// Release sends entity's latest held change again on the worker's next
// pass, with its attempts counted afresh.
func Release(ctx context.Context, q db.Querier, now time.Time, entity string) error {
	tag, err := q.Exec(ctx, `
		UPDATE sam_outbox SET state = 'pending', attempts = 0, next_attempt_at = $2, last_error = NULL
		WHERE id = (SELECT id FROM sam_outbox WHERE entity = $1 AND state = 'held' ORDER BY version DESC LIMIT 1)`,
		entity, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNothingHeld
	}
	return nil
}

// Waiting reports whether entity has a change waiting to be sent or held
// for an operator, so a reconciler queues no correction on top of one.
func Waiting(ctx context.Context, q db.Querier, entity string) (bool, error) {
	var waiting bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sam_outbox WHERE entity = $1 AND state IN ('pending', 'held'))`, entity).
		Scan(&waiting)
	return waiting, err
}
