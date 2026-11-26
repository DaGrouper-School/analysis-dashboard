package labs

import (
	"encoding/json"
	"testing"
)

func TestPlace(t *testing.T) {
	t.Parallel()
	l := Lab{Location: Point{Lat: 9.03, Lng: 38.74}}
	var r Reading
	place(&r, l, json.RawMessage(`{"type": "Point", "coordinates": [38.741, 9.031]}`), false)
	if r.LocationSource != FromDevice || r.Location.Lat != 9.031 || r.Location.Lng != 38.741 {
		t.Errorf("a GPS fix: %+v", r)
	}
	place(&r, l, json.RawMessage(`{"type": "Point", "coordinates": [38.74, 9.03]}`), true)
	if r.LocationSource != FromLab || r.Location != l.Location {
		t.Errorf("from the site: %+v", r)
	}
	for _, raw := range []string{`null`, `{"type": "Polygon", "coordinates": []}`, `{"type": "Point", "coordinates": [1]}`} {
		place(&r, l, json.RawMessage(raw), false)
		if r.LocationSource != FromLab {
			t.Errorf("%s: %+v", raw, r)
		}
	}
}
