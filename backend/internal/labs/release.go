package labs

import (
	"context"
	"net/http"
	"time"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// Release review and release: what waits before the public sees it, and
// moving the cutoff. They live here, beside the devices whose values wait;
// the cutoff and the publish are the classrooms module's.

const msgRelease = "You cannot release this classroom’s data."

// ReleaseSource is one source of values waiting for release.
type ReleaseSource struct {
	ID    string    `json:"id"`
	Name  *string   `json:"name"`
	Kind  string    `json:"kind"`
	Count int64     `json:"count"`
	First time.Time `json:"first"`
	Last  time.Time `json:"last"`
}

// ReleaseReview is the front end's ReleaseReview.
type ReleaseReview struct {
	ReleasedThrough *time.Time      `json:"releasedThrough"`
	Auto            bool            `json:"auto"`
	ShowPublic      bool            `json:"showPublic"`
	Pending         int64           `json:"pending"`
	Sources         []ReleaseSource `json:"sources"`
}

// releaser is the classroom in the path, for someone who may release its
// data.
func (m *Module) releaser(r *http.Request) (classrooms.Classroom, accounts.User, error) {
	c, _, can, err := m.classroom(r, r.PathValue("classroomId"))
	if err != nil {
		return c, accounts.User{}, err
	}
	if !can.ReleaseData {
		return c, accounts.User{}, httpx.Forbidden(msgRelease)
	}
	cur, _ := accounts.CurrentUser(r.Context())
	u, err := accounts.Get(r.Context(), m.db, cur.ID)
	return c, u, err
}

func (m *Module) releaseReview(w http.ResponseWriter, r *http.Request) error {
	c, u, err := m.releaser(r)
	if err != nil {
		return err
	}
	out, err := m.review(r.Context(), c, u)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (m *Module) releaseData(w http.ResponseWriter, r *http.Request) error {
	c, u, err := m.releaser(r)
	if err != nil {
		return err
	}
	var in struct {
		Through string `json:"through"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	through, err := time.Parse(time.RFC3339, in.Through)
	if err != nil {
		return httpx.FieldError(httpx.CodeInvalidInput, "through", "Choose when to release up to.")
	}
	if c, _, err = classrooms.Release(r.Context(), m.db, m.now(), c, u.ID, through); err != nil {
		return err
	}
	out, err := m.review(r.Context(), c, u)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// review is what waits for release in c: the values SAM holds unpublished
// observed after the cutoff, by source, read as u.
func (m *Module) review(ctx context.Context, c classrooms.Classroom, u accounts.User) (ReleaseReview, error) {
	cutoff := classrooms.ReleaseCutoff(c, m.now())
	out := ReleaseReview{ReleasedThrough: cutoff, Auto: c.Sharing.Review == roles.ReviewAuto, ShowPublic: c.Sharing.ShowPublic, Sources: []ReleaseSource{}}
	if c.SpaceID == nil {
		return out, nil
	}
	as, err := m.viewer(u)
	if err != nil {
		return out, samaccess.Translate(ctx, err)
	}
	q := client.ObservationQuery{SpaceID: *c.SpaceID, Publication: "unpublished"}
	if cutoff != nil {
		after := cutoff.Add(time.Microsecond)
		q.From = &after
	}
	agg, err := as.Aggregate(ctx, client.Aggregation{ObservationQuery: q, GroupBy: []string{"device_id"}, Functions: []string{"count"}})
	if err != nil {
		return out, samaccess.Translate(ctx, err)
	}
	names := map[uuid.UUID]Device{}
	if err := m.allDevices(ctx, c.ID, func(d Device) {
		if d.SAMDeviceID != nil {
			names[*d.SAMDeviceID] = d
		}
	}); err != nil {
		return out, err
	}
	for _, g := range agg.Groups {
		if g.Count == nil || *g.Count == 0 {
			continue
		}
		src := ReleaseSource{ID: FieldSource, Kind: FieldSource, Count: *g.Count}
		sq := q
		if g.DeviceID != nil {
			d := names[*g.DeviceID]
			name := d.Name
			src.ID, src.Kind, src.Name = d.ID.String(), "device", &name
			sq.DeviceIDs = []uuid.UUID{*g.DeviceID}
		} else {
			sq.Field = true
		}
		sum, err := as.Summarise(ctx, sq)
		if err != nil {
			return out, samaccess.Translate(ctx, err)
		}
		if sum.First != nil && sum.Last != nil {
			src.First, src.Last = *sum.First, *sum.Last
		}
		out.Pending += src.Count
		out.Sources = append(out.Sources, src)
	}
	return out, nil
}
