package labs

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
)

func TestRawFilters(t *testing.T) {
	t.Parallel()
	space, labSpace, samDevice := uuid.New(), uuid.New(), uuid.New()
	l := Lab{ID: uuid.New(), SpaceID: &labSpace}
	d := Device{ID: uuid.New(), SAMDeviceID: &samDevice}
	s := rawScope{classroom: classrooms.Classroom{SpaceID: &space}, labs: map[uuid.UUID]Lab{l.ID: l}, devices: map[uuid.UUID]Device{d.ID: d}}
	q, f := s.filters(RawQuery{LabID: l.ID.String(), SourceID: d.ID.String(), ParameterID: "uv", From: "2026-11-01", To: "2026-11-03"})
	if len(f) != 0 || q.SpaceID != labSpace || len(q.DeviceIDs) != 1 || q.DeviceIDs[0] != samDevice || q.ParameterIDs[0] != "uv" ||
		!q.From.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) || !q.To.Equal(time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("every filter: %+v %v", q, f)
	}
	if q, _ := s.filters(RawQuery{SourceID: FieldSource, ParameterID: "uv"}, "parameterId"); !q.Field || q.SpaceID != space || len(q.ParameterIDs) != 0 {
		t.Errorf("field observations, the parameter left out: %+v", q)
	}
	_, f = s.filters(RawQuery{LabID: uuid.NewString(), SourceID: uuid.NewString(), From: "1 Nov", To: "x"})
	for _, field := range []string{"labId", "sourceId", "from", "to"} {
		if f[field] == "" {
			t.Errorf("no problem for %s: %v", field, f)
		}
	}
}
