// Package consent holds guardian consent by link: a minor names a
// guardian, who gets a link, opens it without signing in, and gives or
// withdraws consent against the current consent text
// (docs/design/classrooms-and-permissions.md#guardian-consent). It owns
// guardian_links and consent_records. The decision reaches SAM as the
// user's consent attribute, through the accounts module's outbox kind.
package consent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/notify"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// GuardianInput is the front end's GuardianInput.
type GuardianInput struct {
	GuardianName    string `json:"guardianName"`
	GuardianContact string `json:"guardianContact"`
}

// CheckGuardian reports what is wrong with a guardian, by field, as the
// front end's validateGuardian does.
func CheckGuardian(in GuardianInput) map[string]string {
	f := map[string]string{}
	if strings.TrimSpace(in.GuardianName) == "" {
		f["guardianName"] = "Enter your guardian’s name."
	}
	switch contact := strings.TrimSpace(in.GuardianContact); {
	case contact == "":
		f["guardianContact"] = "Enter their email address or phone number."
	case !notify.ValidAddress(contact) && accounts.NormalizePhone(contact) == "":
		f["guardianContact"] = "Enter a valid email address, or a phone number with country code."
	}
	return f
}

// Contact is a guardian's contact as stored: a lower-case email, or a phone
// number in E.164; and whether it is an email.
func Contact(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if notify.ValidAddress(trimmed) {
		return strings.ToLower(trimmed), true
	}
	return accounts.NormalizePhone(trimmed), false
}

// Link is a consent link as stored.
type Link struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	GuardianName    string
	GuardianContact string
	RequestedAt     time.Time
	Voided          bool
}

// Record is one guardian decision.
type Record struct {
	Method      string
	LinkID      *uuid.UUID
	Decision    accounts.Consent
	Guardian    string
	TextVersion int
	Decimals    int
	DecidedAt   time.Time
}

// Errors of consent links.
var (
	ErrNoLink      = errors.New("no such consent link")
	ErrTextChanged = errors.New("the consent text has changed")
)

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Ask voids the user's live link and stores a new one to the guardian, and
// marks the user a minor whose consent is pending, all within tx. It
// returns the link and its token, which only it carries.
func Ask(ctx context.Context, tx db.Querier, now time.Time, user uuid.UUID, in GuardianInput) (Link, string, error) {
	contact, _ := Contact(in.GuardianContact)
	l := Link{ID: uuid.New(), UserID: user, GuardianName: strings.TrimSpace(in.GuardianName), GuardianContact: contact, RequestedAt: now}
	token := newToken()
	if _, err := tx.Exec(ctx, `UPDATE guardian_links SET voided_at = $2 WHERE user_id = $1 AND voided_at IS NULL`, user, now); err != nil {
		return l, "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO guardian_links (id, user_id, guardian_name, guardian_contact, token_hash, requested_at)
		VALUES ($1, $2, $3, $4, $5, $6)`, l.ID, user, l.GuardianName, l.GuardianContact, hashToken(token), now); err != nil {
		return l, "", err
	}
	return l, token, accounts.AwaitConsent(ctx, tx, now, user)
}

const linkColumns = `id, user_id, guardian_name, guardian_contact, requested_at, voided_at IS NOT NULL`

func scanLink(r pgx.Row) (Link, error) {
	var l Link
	err := r.Scan(&l.ID, &l.UserID, &l.GuardianName, &l.GuardianContact, &l.RequestedAt, &l.Voided)
	if errors.Is(err, pgx.ErrNoRows) {
		return l, ErrNoLink
	}
	return l, err
}

// ByToken is the live link with token, or ErrNoLink, voided ones
// included in neither.
func ByToken(ctx context.Context, q db.Querier, token string) (Link, error) {
	return scanLink(q.QueryRow(ctx, `SELECT `+linkColumns+` FROM guardian_links WHERE token_hash = $1 AND voided_at IS NULL`, hashToken(token)))
}

// LiveLink is the user's live link, or ErrNoLink.
func LiveLink(ctx context.Context, q db.Querier, user uuid.UUID) (Link, error) {
	return scanLink(q.QueryRow(ctx, `SELECT `+linkColumns+` FROM guardian_links WHERE user_id = $1 AND voided_at IS NULL`, user))
}

// LatestRecord is the user's latest decision; found is false when there
// is none. With a link, only that link's decisions count.
func LatestRecord(ctx context.Context, q db.Querier, user uuid.UUID, link *uuid.UUID) (Record, bool, error) {
	var r Record
	err := q.QueryRow(ctx, `SELECT method, link_id, decision, guardian_name, text_version, coordinate_decimals, decided_at
		FROM consent_records WHERE user_id = $1 AND ($2::uuid IS NULL OR link_id = $2)
		ORDER BY decided_at DESC, id LIMIT 1`, user, link).
		Scan(&r.Method, &r.LinkID, &r.Decision, &r.Guardian, &r.TextVersion, &r.Decimals, &r.DecidedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, false, nil
	}
	return r, err == nil, err
}

// Decimals are the decimal places a minor's positions are rounded to now,
// from the privacy setting: what a consent text promises when it is shown.
func Decimals(ctx context.Context, q db.Querier) (int, error) {
	e, err := settings.Get(ctx, q, settings.KeyPrivacy)
	if err != nil {
		return 0, err
	}
	var p settings.Privacy
	return p.MinorCoordinateDecimals, json.Unmarshal(e.Value, &p)
}

// Decide records the guardian's decision on the link with token, against
// the consent text's version they were shown, which must be current,
// promising decimals, and sets the minor's consent, which reaches SAM
// through the outbox. A decision the minor's consent already has records
// nothing.
func Decide(ctx context.Context, q db.Querier, now time.Time, token string, give bool, shown, current, decimals int) (Link, error) {
	var l Link
	err := pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		var err error
		if l, err = scanLink(tx.QueryRow(ctx, `SELECT `+linkColumns+` FROM guardian_links WHERE token_hash = $1 AND voided_at IS NULL FOR UPDATE`,
			hashToken(token))); err != nil {
			return err
		}
		if shown != current {
			return ErrTextChanged
		}
		u, err := accounts.Get(ctx, tx, l.UserID)
		if err != nil {
			return err
		}
		decision := accounts.ConsentWithdrawn
		if give {
			decision = accounts.ConsentGiven
		}
		if u.Consent != nil && *u.Consent == decision {
			return nil
		}
		if _, err := tx.Exec(ctx, `INSERT INTO consent_records (id, user_id, method, link_id, decision, guardian_name, text_version, coordinate_decimals, decided_at)
			VALUES ($1, $2, 'link', $3, $4, $5, $6, $7, $8)`, uuid.New(), l.UserID, l.ID, decision, l.GuardianName, shown, decimals, now); err != nil {
			return err
		}
		_, err = accounts.SetConsent(ctx, tx, now, l.UserID, decision)
		return err
	})
	return l, err
}
