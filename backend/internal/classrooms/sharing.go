package classrooms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// Sharing and release: how a classroom shows its data, what its
// facilitators may do, its data terms, and how far its data is released
// (docs/design/dashboards-and-public-site.md#sharing-and-release).

// The outbox kinds of sharing and release, sent as the person whose
// decision each carries out.
const (
	PolicyKind  = "classroom_policy"
	ReleaseKind = "release"
)

func policyEntity(id uuid.UUID) string  { return "policy:" + id.String() }
func releaseEntity(id uuid.UUID) string { return "release:" + id.String() }

// MaxTermsDecimals bounds a classroom's coordinate decimals, as the front
// end does.
const MaxTermsDecimals = 6

// SharingInput is the front end's SharingInput.
type SharingInput struct {
	Sharing roles.Sharing      `json:"sharing"`
	Terms   settings.DataTerms `json:"terms"`
}

// CheckSharing reports what is wrong with sharing settings and terms, by
// field, as the front end's validateSharing does. The licence's address
// must be https, as SAM's data policies take.
func CheckSharing(in SharingInput) map[string]string {
	f := map[string]string{}
	s := in.Sharing
	if s.Review != roles.ReviewManual && s.Review != roles.ReviewAuto {
		f["review"] = "Pick manual or automatic review."
	}
	if s.Review == roles.ReviewAuto && (s.AutoReleaseDays < 1 || s.AutoReleaseDays > roles.MaxAutoReleaseDays) {
		f["autoReleaseDays"] = fmt.Sprintf("Choose 1 to %d days.", roles.MaxAutoReleaseDays)
	}
	if s.Releasers != roles.ReleasersHead && s.Releasers != roles.ReleasersHeadAndFacils {
		f["releasers"] = "Pick who may release data."
	}
	t := in.Terms
	if d := t.CoordinateDecimals; d != nil && (*d < 0 || *d > MaxTermsDecimals) {
		f["coordinateDecimals"] = fmt.Sprintf("Choose 0 to %d decimal places.", MaxTermsDecimals)
	}
	if strings.TrimSpace(t.Licence) == "" {
		f["licence"] = "Enter the licence."
	}
	if strings.TrimSpace(t.Attribution) == "" {
		f["attribution"] = "Enter how to credit the data."
	}
	if t.LicenceURL != nil && strings.TrimSpace(*t.LicenceURL) != "" {
		u, err := url.Parse(strings.TrimSpace(*t.LicenceURL))
		if err != nil || u.Scheme != "https" || u.Host == "" {
			f["licenceUrl"] = "Enter a web address starting https://, or leave it empty."
		}
	}
	return f
}

// cleanTerms trims the terms: an empty licence address is none, and empty
// restrictions are dropped.
func cleanTerms(t settings.DataTerms) settings.DataTerms {
	out := t
	out.Licence, out.Attribution = strings.TrimSpace(t.Licence), strings.TrimSpace(t.Attribution)
	if t.LicenceURL != nil {
		if u := strings.TrimSpace(*t.LicenceURL); u != "" {
			out.LicenceURL = &u
		} else {
			out.LicenceURL = nil
		}
	}
	out.Restrictions = []string{}
	for _, r := range t.Restrictions {
		if r = strings.TrimSpace(r); r != "" {
			out.Restrictions = append(out.Restrictions, r)
		}
	}
	return out
}

// narrows reports whether going from was to now takes anyone's access
// away: the public's, or the facilitators'.
func narrows(was, now roles.Sharing) bool {
	return was.ShowPublic && !now.ShowPublic ||
		was.Review == roles.ReviewAuto && (now.Review != roles.ReviewAuto || now.AutoReleaseDays > was.AutoReleaseDays) ||
		was.Releasers == roles.ReleasersHeadAndFacils && now.Releasers != roles.ReleasersHeadAndFacils ||
		was.FacilitatorsManageDevices && !now.FacilitatorsManageDevices ||
		was.FacilitatorsInvite && !now.FacilitatorsInvite
}

// SetSharing stores a classroom's sharing settings and terms as by chose
// them, and sends SAM what changes: the role and public grants through
// the classroom kind, as this service, a change that takes access away
// queued as one; and the terms as the space's data policy, as by.
func SetSharing(ctx context.Context, q db.Querier, now time.Time, c Classroom, by uuid.UUID, in SharingInput) (Classroom, error) {
	terms := cleanTerms(in.Terms)
	err := pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		sharing, err := json.Marshal(in.Sharing)
		if err != nil {
			return err
		}
		rawTerms, err := json.Marshal(terms)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE classrooms SET sharing = $2, terms = $3 WHERE id = $1`, c.ID, sharing, rawTerms); err != nil {
			return err
		}
		if _, err := samaccess.Enqueue(ctx, tx, now, samaccess.Change{Entity: entity(c.ID), Kind: Kind,
			Payload: desired{Sharing: in.Sharing, Decimals: terms.CoordinateDecimals}, TakesAccessAway: narrows(c.Sharing, in.Sharing)}); err != nil {
			return err
		}
		if !sameTerms(c.Terms, terms) {
			return queuePolicy(ctx, tx, now, c.ID, by)
		}
		return nil
	})
	c.Sharing, c.Terms = in.Sharing, terms
	return c, err
}

func sameTerms(a, b settings.DataTerms) bool {
	ra, _ := json.Marshal(a)
	rb, _ := json.Marshal(b)
	return string(ra) == string(rb)
}

// queuePolicy queues the classroom's terms for SAM as its space's data
// policy, set as by.
func queuePolicy(ctx context.Context, q db.Querier, now time.Time, classroom, by uuid.UUID) error {
	_, err := samaccess.Enqueue(ctx, q, now, samaccess.Change{Entity: policyEntity(classroom), Kind: PolicyKind, Actor: &by, Payload: struct{}{}})
	return err
}

// ReleaseCutoff is how far a classroom's data is released at now: the
// cutoff set by hand, or with automatic review the later of it and the
// review period ago. Nil while nothing is.
func ReleaseCutoff(c Classroom, now time.Time) *time.Time {
	if c.Sharing.Review != roles.ReviewAuto {
		return c.ReleasedThrough
	}
	aged := now.Add(-time.Duration(c.Sharing.AutoReleaseDays) * 24 * time.Hour)
	if c.ReleasedThrough != nil && c.ReleasedThrough.After(aged) {
		return c.ReleasedThrough
	}
	return &aged
}

// release is what SAM is to publish: the classroom's records observed
// through Through.
type release struct {
	Through time.Time `json:"through"`
}

// Release moves the classroom's cutoff to through, taken as now when it
// is later, and queues SAM's publish up to it as by, who may release. A
// cutoff is never moved back: one at or before the current changes and
// queues nothing. It reports whether a publish was queued.
func Release(ctx context.Context, q db.Querier, now time.Time, c Classroom, by uuid.UUID, through time.Time) (Classroom, bool, error) {
	if through.After(now) {
		through = now
	}
	through = through.UTC().Round(time.Microsecond)
	if c.ReleasedThrough != nil && !through.After(*c.ReleasedThrough) {
		return c, false, nil
	}
	err := pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE classrooms SET released_through = $2 WHERE id = $1 AND (released_through IS NULL OR released_through < $2)`, c.ID, through)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		_, err = samaccess.Enqueue(ctx, tx, now, samaccess.Change{Entity: releaseEntity(c.ID), Kind: ReleaseKind, Actor: &by, Payload: release{Through: through}})
		return err
	})
	c.ReleasedThrough = &through
	return c, err == nil, err
}

// actorReady reports whether SAM holds the change's person with the rights
// they act on in classroom: their membership, or the platform admins'.
func (m *Module) actorReady(ctx context.Context, classroom uuid.UUID, a samaccess.Attempt) (bool, error) {
	if a.Actor == nil {
		return false, samaccess.Hold(errors.New("the change names no person"))
	}
	if in, err := InSAM(ctx, m.db, classroom, *a.Actor); err != nil || in {
		return in, err
	}
	u, err := accounts.Get(ctx, m.db, *a.Actor)
	return u.SAMAdmin, err
}

// classroomOf is the classroom an entity names, after prefix.
func (m *Module) classroomOf(ctx context.Context, entity, prefix string) (Classroom, error) {
	id, err := uuid.Parse(strings.TrimPrefix(entity, prefix))
	if err != nil {
		return Classroom{}, samaccess.Hold(err)
	}
	c, err := Get(ctx, m.db, id)
	if errors.Is(err, ErrNoClassroom) {
		return c, samaccess.Hold(err)
	}
	return c, err
}

// applyPolicy sets the classroom's terms as its space's data policy, as
// the person who changed them, naming the version SAM holds, once SAM
// holds the space and the person's rights. A version another set meanwhile
// is read back from SAM and named instead.
func (m *Module) applyPolicy(ctx context.Context, a samaccess.Attempt) error {
	c, err := m.classroomOf(ctx, a.Entity, "policy:")
	if err != nil {
		return err
	}
	if c.SpaceID == nil {
		return samaccess.ErrNotYet
	}
	if ok, err := m.actorReady(ctx, c.ID, a); err != nil || !ok {
		return errors.Join(samaccess.ErrNotYet, err)
	}
	version := 0
	if c.SAMPolicyVersion != nil {
		version = *c.SAMPolicyVersion
	}
	in := client.PolicyInput{Licence: c.Terms.Licence, LicenceURL: c.Terms.LicenceURL, Attribution: c.Terms.Attribution,
		Restrictions: c.Terms.Restrictions, Version: version}
	if in.Restrictions == nil {
		in.Restrictions = []string{}
	}
	p, err := a.SAM.PutPolicy(a.WithKey(ctx, fmt.Sprintf("policy:%d", version)), *c.SpaceID, in)
	var e *client.Error
	if errors.As(err, &e) && e.Code == "version_mismatch" {
		held, gerr := a.SAM.Policy(ctx, *c.SpaceID)
		if gerr != nil {
			return gerr
		}
		in.Version = 0
		if held.SpaceID == *c.SpaceID {
			in.Version = held.Version
		}
		p, err = a.SAM.PutPolicy(a.WithKey(ctx, fmt.Sprintf("policy:%d", in.Version)), *c.SpaceID, in)
	}
	if err != nil {
		return err
	}
	_, err = m.db.Exec(ctx, `UPDATE classrooms SET sam_policy_version = $2 WHERE id = $1`, c.ID, p.Version)
	return err
}

// applyRelease publishes the classroom's records observed through the
// release's cutoff, as the person releasing, once SAM holds the space and
// their membership. Its idempotency key names the release, so a retried
// attempt publishes once.
func (m *Module) applyRelease(ctx context.Context, a samaccess.Attempt) error {
	var want release
	if err := json.Unmarshal(a.Payload, &want); err != nil {
		return samaccess.Hold(err)
	}
	c, err := m.classroomOf(ctx, a.Entity, "release:")
	if err != nil {
		return err
	}
	if c.SpaceID == nil {
		return samaccess.ErrNotYet
	}
	if ok, err := m.actorReady(ctx, c.ID, a); err != nil || !ok {
		return errors.Join(samaccess.ErrNotYet, err)
	}
	out, err := a.SAM.Publish(a.WithKey(ctx, "publish:"+want.Through.Format(time.RFC3339Nano)),
		client.PublishRequest{SpaceID: *c.SpaceID, ObservedUntil: want.Through})
	if err == nil {
		slog.InfoContext(ctx, "released a classroom's data", "classroom", c.ID, "through", want.Through, "published", out.Published)
	}
	return err
}

// Checks are the reconciler's checks of classrooms: the public grants.
func (m *Module) Checks() []samaccess.Check {
	return []samaccess.Check{{Name: "public", Run: m.reconcilePublic}}
}

// reconcilePublic compares, for each classroom SAM has a space for, the
// public grants anonymous holds there with those its sharing calls for,
// and queues the classroom when they differ.
func (m *Module) reconcilePublic(ctx context.Context) error {
	if m.cfg.SAM == nil {
		return nil
	}
	rows, err := m.db.Query(ctx, `SELECT `+classroomColumns+` FROM classrooms WHERE space_id IS NOT NULL ORDER BY id`)
	if err != nil {
		return err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Classroom, error) { return scanClassroom(r) })
	if err != nil {
		return err
	}
	anonymous := m.cfg.SAM.Tenant().AnonymousID
	for _, c := range list {
		if waiting, err := samaccess.Waiting(ctx, m.db, entity(c.ID)); err != nil || waiting {
			if err != nil {
				return err
			}
			continue
		}
		page, err := m.cfg.SAM.Service().Grants(ctx, client.GrantQuery{SubjectID: &anonymous, SpaceID: c.SpaceID}, client.PageQuery{Limit: 100})
		if err != nil {
			slog.WarnContext(ctx, "reconciler could not read a classroom's public grants", "classroom", c.ID, "err", err)
			continue
		}
		var held []client.Grant
		for _, g := range page.Items {
			if g.RevokedAt == nil && slices.Contains(g.Labels, roles.PublicLabel) {
				held = append(held, g)
			}
		}
		want := roles.PublicGrants(c.Sharing, c.Terms.CoordinateDecimals)
		differs := len(want) != len(held)
		for _, g := range want {
			differs = differs || !slices.ContainsFunc(held, func(h client.Grant) bool { return slices.Contains(h.Labels, g.Name) })
		}
		if !differs {
			continue
		}
		slog.WarnContext(ctx, "reconciler found a classroom's public grants SAM holds otherwise; correcting", "classroom", c.ID)
		if _, err := samaccess.Enqueue(ctx, m.db, m.now(), samaccess.Change{Entity: entity(c.ID), Kind: Kind,
			Payload: desired{Sharing: c.Sharing, Decimals: c.Terms.CoordinateDecimals}, TakesAccessAway: len(held) > len(want)}); err != nil {
			return err
		}
	}
	return nil
}
