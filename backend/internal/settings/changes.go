package settings

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// Actor is who made a change: a platform admin, named as they were at the
// time, or the operator or the seed.
type Actor struct {
	// UserID is the admin's account, or nil for the operator and the seed.
	UserID *uuid.UUID
	Name   string
}

// The actors that are not people.
var (
	// Operator is someone running a dgs command on the deployment.
	Operator = Actor{Name: "operator"}
	// Seed is dgs seed.
	Seed = Actor{Name: "seed"}
)

// What a change is to.
const (
	KindSetting      = "setting"
	KindStage        = "stage"
	KindRegion       = "region"
	KindHostQuestion = "host_question"
	KindConsentText  = "consent_text"
)

// Change is one change to the platform's configuration.
type Change struct {
	// Kind is what was changed, such as setting or stage; Subject is its
	// key or ID.
	Kind    string
	Subject string
	// Summary says what happened, as a platform admin reads it.
	Summary string
}

// Logged is a change as the log holds it.
type Logged struct {
	ID    int64
	At    time.Time
	Actor Actor
	Change
}

// Record writes c to the change log, as actor's. Every module that keeps
// part of the platform's configuration records its changes here, in the
// transaction that makes them.
func Record(ctx context.Context, q db.Querier, now time.Time, actor Actor, c Change) error {
	return record(ctx, q, now, actor, c)
}

// Changes are the latest limit changes, newest first.
func Changes(ctx context.Context, q db.Querier, limit int) ([]Logged, error) {
	return changes(ctx, q, limit)
}
