package labs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// OutboxKinds are the lab kind, sent as this service, and the device kind,
// sent as the person who registered or removed the device.
func (m *Module) OutboxKinds() []samaccess.Kind {
	return []samaccess.Kind{{Name: Kind, Apply: m.applyLab}, {Name: DeviceKind, ByPerson: true, Apply: m.applyDevice}}
}

// applyLab brings a lab in SAM to its desired state: a space under its
// classroom's, once SAM has that, and a site of the lab's code at its
// point. A site that differs is replaced, naming the version SAM holds.
// Each step is recorded once SAM confirms it.
func (m *Module) applyLab(ctx context.Context, a samaccess.Attempt) error {
	var want desired
	if err := json.Unmarshal(a.Payload, &want); err != nil {
		return samaccess.Hold(err)
	}
	id, err := uuid.Parse(strings.TrimPrefix(a.Entity, "lab:"))
	if err != nil {
		return samaccess.Hold(err)
	}
	l, err := Get(ctx, m.db, id)
	if errors.Is(err, ErrNoLab) {
		return samaccess.Hold(err)
	}
	if err != nil {
		return err
	}
	c, err := classrooms.Get(ctx, m.db, l.ClassroomID)
	if err != nil {
		return err
	}
	if c.SpaceID == nil {
		return samaccess.ErrNotYet
	}
	if l.SpaceID == nil {
		space, err := a.SAM.CreateSpace(a.WithKey(ctx, "space"), client.SpaceInput{ParentID: *c.SpaceID})
		if err != nil {
			return err
		}
		if err := confirmSpace(ctx, m.db, id, space.ID); err != nil {
			return err
		}
		l.SpaceID = &space.ID
	}
	geometry := point(want.Location)
	held, err := a.SAM.Site(ctx, *l.SpaceID)
	version := 0
	var e *client.Error
	switch {
	case errors.As(err, &e) && e.Status == 404:
	case err != nil:
		return err
	case held.Code == want.Code && sameGeometry(held.Geometry, geometry):
		return confirmSite(ctx, m.db, id, held.Version)
	default:
		version = held.Version
	}
	site, err := a.SAM.PutSite(a.WithKey(ctx, "site"), *l.SpaceID, client.SiteInput{Code: want.Code, Geometry: geometry, Version: version})
	if err != nil {
		return err
	}
	return confirmSite(ctx, m.db, id, site.Version)
}

// point is p as a GeoJSON Point, longitude first.
func point(p Point) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"type":"Point","coordinates":[%v,%v]}`, p.Lng, p.Lat))
}

func sameGeometry(a, b json.RawMessage) bool {
	var x, y struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	}
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil || x.Type != y.Type || len(x.Coordinates) != len(y.Coordinates) {
		return false
	}
	for i := range x.Coordinates {
		if x.Coordinates[i] != y.Coordinates[i] {
			return false
		}
	}
	return true
}
