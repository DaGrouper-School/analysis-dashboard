package samaccess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// The worker's pace.
const (
	// PollInterval is how often the worker looks for changes due.
	PollInterval = time.Second
	// MaxBackoff is the longest wait between attempts at one change.
	MaxBackoff = 5 * time.Minute
	// ReplayWindow is how long SAM replays the answer to a request by its
	// idempotency key (SAM_IDEMPOTENCY_TTL, 24 hours by default).
	ReplayWindow = 24 * time.Hour
	// passLimit bounds how many entities one pass sends.
	passLimit = 100
	// errorLimit bounds the error kept with a row.
	errorLimit = 1000
)

// Kind is a kind of change and how it reaches SAM. The module that owns the
// entity declares it.
type Kind struct {
	Name string
	// ByPerson is whether its changes are sent as the person who made them,
	// whose account each row names; otherwise they are sent as this service
	// and name none.
	ByPerson bool
	// Apply sends a change's desired state to SAM. It returns nil once SAM
	// holds it; ErrNotYet while another entity must reach SAM first; an
	// error wrapped by Hold for what retrying cannot fix; and any other
	// error to be retried, or held as SAM's answer decides.
	Apply func(ctx context.Context, a Attempt) error
}

// Attempt is one attempt at sending a change.
type Attempt struct {
	Entity  string
	Version int64
	Payload json.RawMessage
	// SAM calls SAM as the change's actor.
	SAM *client.Client
	// Replayed is whether SAM still replays the answers to this change's
	// first attempt. When it does not, a change that must happen once, such
	// as a release, checks what SAM holds before sending again.
	Replayed bool
}

// Key is the idempotency key for one request of the change: the entity,
// what the request changes and the version, such as
// user:<id>:attr:consent:v3. It carries no attempt number, so a retry after
// a lost answer gets the first answer back.
func (a Attempt) Key(what string) string { return fmt.Sprintf("%s:%s:v%d", a.Entity, what, a.Version) }

// WithKey is ctx with Key(what) as the idempotency key of its writes.
func (a Attempt) WithKey(ctx context.Context, what string) context.Context {
	return client.WithIdempotencyKey(ctx, a.Key(what))
}

// ErrNotYet means another entity must reach SAM first, such as a
// classroom's groups before a membership. The change is retried, not held.
var ErrNotYet = errors.New("waiting for another change to reach SAM")

// Hold marks err as what retrying cannot fix: the change is held for an
// operator.
func Hold(err error) error { return holdError{err} }

type holdError struct{ err error }

func (h holdError) Error() string { return h.err.Error() }
func (h holdError) Unwrap() error { return h.err }

// Actors tells the worker who a change is sent as. The accounts module
// provides it.
type Actors interface {
	// Person is the account's SAM identity, or ErrActorGone when the
	// account no longer exists.
	Person(ctx context.Context, account uuid.UUID) (Person, error)
}

// ErrActorGone means a change's actor no longer exists. The change is held:
// it is never sent as this service instead.
var ErrActorGone = errors.New("the change's actor no longer exists")

// Worker sends the outbox to SAM.
type Worker struct {
	pool   *pgxpool.Pool
	access *Access
	kinds  map[string]Kind
	actors Actors
	now    func() time.Time
}

// NewWorker builds the worker for kinds. actors may be nil while no kind is
// sent as a person.
func NewWorker(pool *pgxpool.Pool, access *Access, kinds []Kind, actors Actors, now func() time.Time) (*Worker, error) {
	w := &Worker{pool: pool, access: access, kinds: map[string]Kind{}, actors: actors, now: now}
	for _, k := range kinds {
		if k.Name == "" || k.Apply == nil {
			return nil, errors.New("an outbox kind needs a name and an applier")
		}
		if _, dup := w.kinds[k.Name]; dup {
			return nil, fmt.Errorf("outbox kind %q is declared twice", k.Name)
		}
		w.kinds[k.Name] = k
	}
	return w, nil
}

// Run sends changes as they fall due until ctx ends, on whichever instance
// holds the outbox lock.
func (w *Worker) Run(ctx context.Context) {
	t := time.NewTicker(PollInterval)
	defer t.Stop()
	for {
		if _, err := w.Pass(ctx); err != nil && ctx.Err() == nil {
			slog.ErrorContext(ctx, "outbox pass failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Pass sends the changes due now, if this instance holds the outbox lock,
// and reports whether it did.
func (w *Worker) Pass(ctx context.Context) (bool, error) {
	lock, err := w.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = lock.Rollback(context.WithoutCancel(ctx)) }()
	ok, err := db.TryLock(ctx, lock, db.OutboxLock)
	if err != nil || !ok {
		return false, err
	}
	due, err := w.due(ctx)
	if err != nil {
		return true, err
	}
	for _, r := range due {
		if ctx.Err() != nil {
			break
		}
		if err := w.record(ctx, r, w.attempt(ctx, r)); err != nil {
			return true, err
		}
	}
	return true, nil
}

// row is an outbox row the worker sends.
type row struct {
	id          int64
	entity      string
	kind        string
	version     int64
	payload     json.RawMessage
	actor       *uuid.UUID
	attempts    int
	firstSentAt *time.Time
	// takesAccessAway is the change's mark; see Change.TakesAccessAway.
	takesAccessAway bool
}

// due is the latest pending version of each entity whose next attempt has
// come. Older pending versions are never sent: each payload is the whole
// desired state.
func (w *Worker) due(ctx context.Context) ([]row, error) {
	rows, err := w.pool.Query(ctx, `
		SELECT id, entity, kind, version, payload, actor_id, attempts, first_sent_at, takes_access_away FROM (
			SELECT DISTINCT ON (entity) * FROM sam_outbox WHERE state = 'pending' ORDER BY entity, version DESC
		) latest
		WHERE next_attempt_at <= $1 ORDER BY next_attempt_at, id LIMIT $2`, w.now(), passLimit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(c pgx.CollectableRow) (row, error) {
		var r row
		err := c.Scan(&r.id, &r.entity, &r.kind, &r.version, &r.payload, &r.actor, &r.attempts, &r.firstSentAt, &r.takesAccessAway)
		return r, err
	})
}

// attempt sends one change and returns how it went.
func (w *Worker) attempt(ctx context.Context, r row) error {
	kind, ok := w.kinds[r.kind]
	if !ok {
		return Hold(fmt.Errorf("no applier for kind %q in this version", r.kind))
	}
	sam, err := w.actor(ctx, kind, r)
	if err != nil {
		return err
	}
	now := w.now()
	replayed := true
	if r.firstSentAt == nil {
		if _, err := w.pool.Exec(ctx, `UPDATE sam_outbox SET first_sent_at = $2 WHERE id = $1 AND first_sent_at IS NULL`, r.id, now); err != nil {
			return err
		}
	} else {
		replayed = now.Sub(*r.firstSentAt) < ReplayWindow
	}
	return kind.Apply(ctx, Attempt{Entity: r.entity, Version: r.version, Payload: r.payload, SAM: sam, Replayed: replayed})
}

// actor is the client the change is sent through: as this service, or as
// the person the row names, once their principal is registered.
func (w *Worker) actor(ctx context.Context, kind Kind, r row) (*client.Client, error) {
	switch {
	case !kind.ByPerson && r.actor == nil:
		return w.access.Service(), nil
	case !kind.ByPerson:
		return nil, Hold(fmt.Errorf("kind %q is sent as this service, but the change names an actor", kind.Name))
	case r.actor == nil:
		return nil, Hold(fmt.Errorf("kind %q is sent as a person, but the change names none", kind.Name))
	case w.actors == nil:
		return nil, Hold(errors.New("no accounts to find the change's actor in"))
	}
	p, err := w.actors.Person(ctx, *r.actor)
	if errors.Is(err, ErrActorGone) {
		return nil, Hold(err)
	}
	if err != nil {
		return nil, err
	}
	c, err := w.access.User(p)
	if errors.Is(err, ErrUnregistered) {
		return nil, ErrNotYet
	}
	return c, err
}

// outcome is what an attempt leaves the row as.
type outcome int

const (
	done outcome = iota
	retry
	hold
)

// heldCodes are SAM's answers that retrying cannot change.
var heldCodes = map[string]bool{
	client.CodeForbidden: true, client.CodeUnauthenticated: true, client.CodeInvalidRequest: true,
	"attenuation_violation": true, client.CodeIdempotencyReused: true,
}

// retriedCodes are SAM's answers that a later attempt may not get.
var retriedCodes = map[string]bool{
	client.CodeUnavailable: true, client.CodeInternal: true, client.CodeRateLimited: true,
	client.CodeIdempotencyInProgress: true,
}

// classify decides what err leaves a change as, and how long to wait at
// least before retrying.
func classify(err error) (outcome, time.Duration) {
	if err == nil {
		return done, 0
	}
	var h holdError
	if errors.As(err, &h) {
		return hold, 0
	}
	var e *client.Error
	if !errors.As(err, &e) {
		// ErrNotYet, a timeout or SAM unreachable.
		return retry, 0
	}
	switch {
	case heldCodes[e.Code]:
		return hold, 0
	case retriedCodes[e.Code]:
		return retry, e.RetryAfter
	case e.Status >= 500:
		return retry, 0
	}
	// Any other refusal is one the applier did not expect: retrying sends
	// the same request.
	return hold, 0
}

// backoff is the wait before attempt n+1: a second, doubling, at most
// MaxBackoff.
func backoff(attempts int) time.Duration {
	d := time.Second
	for range attempts - 1 {
		d *= 2
		if d >= MaxBackoff {
			return MaxBackoff
		}
	}
	return d
}

// record leaves the row as err decides. A change SAM now holds supersedes
// every older version of its entity still waiting or held.
func (w *Worker) record(ctx context.Context, r row, err error) error {
	if ctx.Err() != nil {
		// Stopping: the attempt is not counted, and the row is tried again
		// on the next pass, by this instance or another.
		return nil
	}
	now := w.now()
	out, wait := classify(err)
	switch out {
	case done:
		return db.InTx(ctx, w.pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `UPDATE sam_outbox SET state = 'done', finished_at = $2, last_error = NULL WHERE id = $1`, r.id, now); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `
				UPDATE sam_outbox SET state = 'superseded', finished_at = $3
				WHERE entity = $1 AND version < $2 AND state IN ('pending', 'held')`, r.entity, r.version, now)
			return err
		})
	case hold:
		level, msg := slog.LevelWarn, "outbox change held"
		if r.takesAccessAway {
			// SAM still allows what this service has taken away.
			level, msg = slog.LevelError, "outbox change held that takes access away: SAM allows more than this service until it is delivered"
		}
		slog.Log(ctx, level, msg, "entity", r.entity, "kind", r.kind, "version", r.version, "err", err)
		_, err := w.pool.Exec(ctx, `UPDATE sam_outbox SET state = 'held', attempts = attempts + 1, last_error = $2 WHERE id = $1`,
			r.id, clip(err.Error()))
		return err
	}
	attempts := r.attempts + 1
	wait = max(wait, backoff(attempts))
	slog.InfoContext(ctx, "outbox change retried", "entity", r.entity, "kind", r.kind, "version", r.version,
		"attempts", attempts, "wait", wait, "err", err)
	_, xerr := w.pool.Exec(ctx, `UPDATE sam_outbox SET attempts = $2, next_attempt_at = $3, last_error = $4 WHERE id = $1`,
		r.id, attempts, now.Add(wait), clip(err.Error()))
	return xerr
}

func clip(s string) string {
	if len(s) > errorLimit {
		return s[:errorLimit]
	}
	return s
}
