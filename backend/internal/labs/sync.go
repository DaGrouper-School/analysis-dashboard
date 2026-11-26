package labs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

// SyncLimit is the most records one batch may carry.
const SyncLimit = 100

// The outcomes of a synced record.
const (
	SyncStored    = "stored"
	SyncDuplicate = "duplicate"
	SyncRejected  = "rejected"
)

// The flags on a device clock.
const (
	FlagClockOff  = "device_clock_off"
	FlagAfterSent = "observed_after_sent"
)

// SyncRecord is the front end's SyncRecord.
type SyncRecord struct {
	ClientID    string           `json:"clientId"`
	Observation ObservationInput `json:"observation"`
	Credit      bool             `json:"credit"`
}

// SyncResult is the front end's SyncRecordResult.
type SyncResult struct {
	ClientID    string            `json:"clientId"`
	Outcome     string            `json:"outcome"`
	Observation *Observation      `json:"observation"`
	Error       *string           `json:"error"`
	Fields      map[string]string `json:"fields"`
	ClockFlags  []string          `json:"clockFlags"`
}

// SyncAnswer is the front end's SyncBatchResult.
type SyncAnswer struct {
	ReceivedAt    time.Time    `json:"receivedAt"`
	ClockOffsetMs float64      `json:"clockOffsetMs"`
	Results       []SyncResult `json:"results"`
}

// syncNamespace names the IDs of synced records in SAM.
var syncNamespace = uuid.MustParse("0c7e9b1a-3d42-4e85-b6f1-9a2c4d7e5f13")

// SyncID is a synced record's ID in SAM: a UUID v5 of the person and the
// client's ID, so SAM answers a retry as a duplicate however long after
// the first attempt it comes.
func SyncID(user uuid.UUID, clientID string) uuid.UUID {
	return uuid.NewSHA1(syncNamespace, []byte(user.String()+"|"+clientID))
}

// ClockFlags are what is wrong with a record's clock: the batch's clock off
// this service's by more than the tolerance, or unreadable; the record
// dated after the batch was sent.
func ClockFlags(clockOff bool, observed string, sentAt time.Time, sentOK bool) []string {
	flags := []string{}
	if clockOff {
		flags = append(flags, FlagClockOff)
	}
	if at, err := time.Parse(time.RFC3339, observed); err == nil && sentOK && at.After(sentAt.Add(ClockTolerance)) {
		flags = append(flags, FlagAfterSent)
	}
	return flags
}

func (m *Module) syncObservations(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		SentAt  string       `json:"sentAt"`
		Records []SyncRecord `json:"records"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if len(in.Records) > SyncLimit {
		return httpx.New(httpx.CodeInvalidInput, fmt.Sprintf("Send at most %d observations at a time.", SyncLimit))
	}
	cur, _ := accounts.CurrentUser(r.Context())
	u, err := accounts.Get(r.Context(), m.db, cur.ID)
	if err != nil {
		return err
	}
	now := m.now()
	sentAt, perr := time.Parse(time.RFC3339, in.SentAt)
	sentOK := perr == nil
	out := SyncAnswer{ReceivedAt: now, Results: make([]SyncResult, len(in.Records))}
	var sent *time.Time
	if sentOK {
		out.ClockOffsetMs = float64(now.Sub(sentAt).Milliseconds())
		sent = &sentAt
	}
	clockOff := !sentOK || math.Abs(out.ClockOffsetMs) > float64(ClockTolerance.Milliseconds())
	for i, rec := range in.Records {
		res, err := m.syncOne(r, u, rec, ClockFlags(clockOff, rec.Observation.ObservedAt, sentAt, sentOK), sent)
		if err != nil {
			return err
		}
		out.Results[i] = res
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// syncOne is one record's result: the answer kept from an earlier attempt,
// or the record checked and relayed as a manual entry is, under its sync
// ID. A refusal is the record's result; any other failure fails the batch.
func (m *Module) syncOne(r *http.Request, u accounts.User, rec SyncRecord, flags []string, sentAt *time.Time) (SyncResult, error) {
	ctx := r.Context()
	res := SyncResult{ClientID: rec.ClientID, Fields: map[string]string{}, ClockFlags: flags}
	if strings.TrimSpace(rec.ClientID) == "" {
		return rejected(res, httpx.New(httpx.CodeInvalidInput, "This record has no client ID."))
	}
	if kept, found, err := keptSync(ctx, m.db, u.ID, rec.ClientID); err != nil || found {
		kept.Fields = map[string]string{}
		return kept, err
	}
	o, held, err := m.recordFor(r, u, rec, sentAt)
	var he *httpx.Error
	if errors.As(err, &he) {
		return rejected(res, he)
	}
	if err != nil {
		return res, err
	}
	res.Outcome, res.Observation = SyncStored, &o
	if held {
		res.Outcome = SyncDuplicate
	}
	return res, keepSync(ctx, m.db, u.ID, rec.ClientID, o, flags, m.now())
}

// recordFor relays a synced record from the lab it names, as the person
// may record there.
func (m *Module) recordFor(r *http.Request, u accounts.User, rec SyncRecord, sentAt *time.Time) (Observation, bool, error) {
	ctx := r.Context()
	lid, err := uuid.Parse(rec.Observation.LabID)
	if err != nil {
		return Observation{}, false, httpx.NotFound(msgNoLab)
	}
	l, err := Get(ctx, m.db, lid)
	if errors.Is(err, ErrNoLab) {
		return Observation{}, false, httpx.NotFound(msgNoLab)
	}
	if err != nil {
		return Observation{}, false, err
	}
	if _, _, can, err := m.classroom(r, l.ClassroomID.String()); err != nil {
		if httpx.IsCode(err, httpx.CodeNotFound) {
			return Observation{}, false, httpx.NotFound(msgNoLab)
		}
		return Observation{}, false, err
	} else if !canRecord(can) {
		return Observation{}, false, httpx.Forbidden(msgCannotObserve)
	}
	if block := ContributionBlock(u); block != "" {
		return Observation{}, false, httpx.Forbidden(block)
	}
	id := SyncID(u.ID, rec.ClientID)
	return m.Record(ctx, l, u, rec.Observation, rec.Credit, FormBatch, RecordOptions{ID: &id, SentAt: sentAt})
}

func rejected(res SyncResult, e *httpx.Error) (SyncResult, error) {
	res.Outcome, res.Error = SyncRejected, &e.Message
	if e.Fields != nil {
		res.Fields = e.Fields
	}
	return res, nil
}

func keptSync(ctx context.Context, q db.Querier, user uuid.UUID, clientID string) (SyncResult, bool, error) {
	var raw []byte
	res := SyncResult{ClientID: clientID, Outcome: SyncDuplicate}
	err := q.QueryRow(ctx, `SELECT observation, clock_flags FROM synced_records WHERE user_id = $1 AND client_id = $2`, user, clientID).
		Scan(&raw, &res.ClockFlags)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, false, nil
	}
	if err != nil {
		return res, false, err
	}
	var o Observation
	if err := json.Unmarshal(raw, &o); err != nil {
		return res, false, err
	}
	res.Observation = &o
	return res, true, nil
}

func keepSync(ctx context.Context, q db.Querier, user uuid.UUID, clientID string, o Observation, flags []string, now time.Time) error {
	raw, err := json.Marshal(o)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `INSERT INTO synced_records (user_id, client_id, observation, clock_flags, synced_at) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, client_id) DO NOTHING`, user, clientID, raw, flags, now)
	return err
}
