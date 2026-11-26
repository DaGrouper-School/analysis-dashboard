package publicsite

import (
	"testing"
	"time"
)

func TestSeriesOf(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	name := func(s string) *string { return &s }
	station := Source{ID: "st", Name: name("Pilot station"), Kind: "device"}
	gauge := Source{ID: "ga", Name: name("Rain gauge"), Kind: "device"}
	field := Source{ID: "field", Kind: "field"}
	points := []sourced{
		{source: station, parameterID: "rain_daily", Point: Point{ObservedAt: at.Add(time.Hour), Value: 3.0}},
		{source: station, parameterID: "rain_daily", Point: Point{ObservedAt: at, Value: 1.0}},
		{source: station, parameterID: "rain_gauge", Point: Point{ObservedAt: at, Value: 9.0}},
		{source: gauge, parameterID: "rain_gauge", Point: Point{ObservedAt: at, Value: 2.0}},
		{source: field, parameterID: "rain_daily", Point: Point{ObservedAt: at, Value: 4.0}},
	}
	manual := func(id string) bool { return id == "rain_gauge" }
	got, ref := seriesOf(points, []string{"rain_daily", "rain_gauge"}, true, manual)
	if len(got) != 3 || ref == nil || *ref != "ga" || got[0].Source.ID != "ga" || got[1].Source.ID != "field" || got[2].Source.ID != "st" {
		t.Fatalf("the gauge first, then by name: %+v %v", got, ref)
	}
	if st := got[2]; st.ParameterID != "rain_daily" || len(st.Points) != 2 || st.Points[0].Value != 1.0 {
		t.Errorf("a station reports the page's value, oldest first: %+v", st)
	}
	if got, ref := seriesOf(points[:2], []string{"rain_daily"}, false, manual); len(got) != 1 || ref != nil {
		t.Errorf("not rain: %+v %v", got, ref)
	}
	if got, _ := seriesOf(nil, []string{"x"}, false, manual); got == nil || len(got) != 0 {
		t.Errorf("no points: %+v", got)
	}
}

func TestRainIDsAndSeason(t *testing.T) {
	t.Parallel()
	rain := []string{"rain_gauge", "rain_daily"}
	if got := rainIDs("rain_daily", rain); len(got) != 2 || got[0] != "rain_daily" || got[1] != "rain_gauge" {
		t.Errorf("rain: %v", got)
	}
	if got := rainIDs("air_temperature", rain); len(got) != 1 {
		t.Errorf("not rain: %v", got)
	}
	if got := season(time.Date(2026, 10, 22, 17, 0, 0, 0, time.FixedZone("EAT", 3*3600))); !got.Equal(time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("the season: %v", got)
	}
}
