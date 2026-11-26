package views

import (
	"slices"
	"testing"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// The front end's specs, ported: core-readings, station-now, indicators
// and completeness, on the same history as daily and hourly values.

var coreParams = settings.CoreParameters{RainDaily: []string{"precip_accum_local_day", "rain_gauge_daily_accumulation"},
	AirTemperature: []string{"air_temperature"}, RainTolerance: 2, RainReference: "rain_gauge_daily_accumulation"}

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func dayStat(source, param, at string, values ...float64) DayStat {
	t := ts(at)
	return DayStat{Source: source, ParameterID: param, Day: t.Format(day), Min: slices.Min(values), Max: slices.Max(values), Last: values[len(values)-1], LastAt: t}
}

func TestStation(t *testing.T) {
	t.Parallel()
	// Released through 07:00: the station's 06:00 total and the gauge's 07:00 one.
	released := []DayStat{
		dayStat("ws", "precip_accum_local_day", "2026-10-20T06:00:00Z", 3),
		dayStat("ws", "air_temperature", "2026-10-20T06:00:00Z", 25),
		dayStat("gauge", "rain_gauge_daily_accumulation", "2026-10-20T07:00:00Z", 6),
		dayStat("ws", "precip_accum_local_day", "2026-10-19T08:00:00Z", 2),
		dayStat("ws", "air_temperature", "2026-10-19T08:00:00Z", 24),
		dayStat("ws", "precip_accum_local_day", "2026-10-12T08:00:00Z", 9),
	}
	d := "2026-10-20"
	inReview := func([]string) string { return InReview }
	core := Station(released, &d, inReview, coreParams, func(string) *string { return nil })
	if r := core.Rain; r.Review != InReview || r.Sources != 2 || r.Spread == nil || *r.Spread != 3 {
		t.Errorf("rain: %+v", r)
	}
	var got [][3]any
	for _, s := range core.Rain.BySource {
		got = append(got, [3]any{s.SourceID, s.Value, s.Reference})
	}
	if !slices.Equal(got, [][3]any{{"gauge", 6.0, true}, {"ws", 3.0, false}}) {
		t.Errorf("by source, the reference first: %v", got)
	}
	if w := core.Rain.Week; len(w) != 7 || w[6].Day != d || w[6].Value == nil || *w[6].Value != 6 || !w[6].Reference || !w[6].Flagged {
		t.Errorf("the week: %+v", w)
	}
	if tc := core.Temperature; tc.Review != InReview || tc.Sources != 1 || tc.Spread != nil || *tc.Low != 25 || *tc.High != 25 || *tc.MonthLow != 24 {
		t.Errorf("temperature: %+v", tc)
	}
	none := Station(nil, nil, func([]string) string { return NotReleased }, coreParams, func(string) *string { return nil })
	if none.Rain.Review != NotReleased || none.Rain.Sources != 0 || none.Rain.Spread != nil || len(none.Rain.Week) != 0 {
		t.Errorf("before a release: %+v", none.Rain)
	}
}

func TestCompassAndFormat(t *testing.T) {
	t.Parallel()
	for deg, want := range map[float64]string{0: "north", 112: "east-southeast", 359: "north", -90: "west"} {
		if got := CompassPoint(deg); got != want {
			t.Errorf("%v: %s", deg, got)
		}
	}
	for _, c := range []struct {
		v    any
		unit string
		want string
	}{{21.46, "°C", "21.5°C"}, {1013.4, "hPa", "1013 hPa"}, {3.0, "m/s", "3 m/s"}, {"Ibis", "", "Ibis"}, {nil, "", "—"}} {
		if got := FormatValue(c.v, c.unit, "number"); got != c.want {
			t.Errorf("%v %s: %q", c.v, c.unit, got)
		}
	}
}

func lookup(ps ...registry.Parameter) Lookup {
	return func(id string) *registry.Parameter {
		for i := range ps {
			if ps[i].ID == id {
				return &ps[i]
			}
		}
		return nil
	}
}

func TestNowTiles(t *testing.T) {
	t.Parallel()
	p := func(id, unit string) registry.Parameter {
		return registry.Parameter{ID: id, Unit: unit, ValueType: "number"}
	}
	config := []settings.NowTile{
		{ID: "wind", Label: "Wind", Icon: "wind", ParameterIDs: []string{"wind_avg"},
			Caption: &settings.NowCaption{ParameterIDs: []string{"wind_direction"}, Text: "From the {value}", Format: "compass"}},
		{ID: "rain", Label: "Rain today", Icon: "rain", ParameterIDs: []string{"rain_a", "rain_b"}, ZeroText: "No rain today"},
		{ID: "pressure", Label: "Pressure", Icon: "pressure", ParameterIDs: []string{"pressure"}},
	}
	v := func(id string, value float64, at string) Value {
		return Value{ParameterID: id, Value: value, ObservedAt: ts(at), Source: "ws"}
	}
	tiles, shown := NowTiles(config, []Value{v("wind_avg", 2, "2026-10-20T05:00:00Z"), v("wind_avg", 3.2, "2026-10-20T06:00:00Z"),
		v("wind_direction", 90, "2026-10-20T06:00:00Z"), v("rain_b", 0, "2026-10-20T06:00:00Z")},
		lookup(p("wind_avg", "m/s"), p("wind_direction", "°"), p("rain_a", "mm"), p("rain_b", "mm"), p("pressure", "hPa")), nil)
	if w := tiles[0]; w.Value != 3.2 || w.Unit != "m/s" || w.Caption == nil || *w.Caption != "From the east" {
		t.Errorf("wind: %+v", w)
	}
	if r := tiles[1]; r.ParameterID == nil || *r.ParameterID != "rain_b" || r.Value != 0.0 || *r.Caption != "No rain today" {
		t.Errorf("rain: %+v", r)
	}
	if pr := tiles[2]; pr.Value != nil || pr.Unit != "hPa" || pr.Caption != nil || pr.ObservedAt != nil {
		t.Errorf("pressure: %+v", pr)
	}
	if len(shown) != 2 || shown[0].Tile != 0 || shown[1].Tile != 1 {
		t.Errorf("shown: %+v", shown)
	}
}

func hour(source, param, at string, value float64) HourStat {
	t := ts(at)
	return HourStat{Source: source, ParameterID: param, Hour: t.Truncate(time.Hour), LastAt: t, Last: value, Count: 1}
}

func TestLast24HoursAndTiles(t *testing.T) {
	t.Parallel()
	hours := []HourStat{hour("ws", "t", "2026-10-20T06:00:00Z", 20), hour("ws", "t", "2026-10-20T05:10:00Z", 18),
		hour("ws", "t", "2026-10-20T04:50:00Z", 19), hour("ws", "t", "2026-10-19T06:00:00Z", 5)}
	got := Last24Hours(hours, "ws", []string{"t"}, ts("2026-10-20T06:00:00Z"))
	if len(got) != 24 || *got[23] != 20 || *got[22] != 19 || got[0] != nil {
		t.Errorf("the last 24 hours: %v", got)
	}

	noiseSoil, noisePH := 0.5, 0.2
	at := ts("2026-10-20T06:00:00Z")
	device := []HourStat{hour("soil1", "soil_moisture", "2026-10-20T06:00:00Z", 31), hour("soil1", "soil_moisture", "2026-10-20T05:00:00Z", 30),
		hour("soil1", "ph", "2026-10-20T06:00:00Z", 6.5), hour("soil1", "ph", "2026-10-20T05:00:00Z", 6.4)}
	icon := "droplet"
	tiles := ReadingTiles([]KeyValue{{ID: "soil_moisture", Label: "Soil moisture", Unit: "%", ValueType: "number", Value: 31.0, Icon: &icon},
		{ID: "ph", Label: "pH", ValueType: "number", Value: 6.5}}, &at, "soil", device, "soil1",
		lookup(registry.Parameter{ID: "soil_moisture", Unit: "%", Noise: &noiseSoil}, registry.Parameter{ID: "ph", Noise: &noisePH}))
	if tiles[0].Icon != "droplet" || tiles[1].Icon != "soil" || *tiles[0].Last24Hours[22] != 30 || *tiles[0].Last24Hours[23] != 31 {
		t.Errorf("the tiles: %+v", tiles)
	}
	if tr := tiles[0].Trend; tr == nil || *tr != (NowTileTrend{Direction: "rising", Label: "Rising", Text: "Rising, up 1 % over 1 hour"}) {
		t.Errorf("soil's trend: %+v", tr)
	}
	if tr := tiles[1].Trend; tr == nil || *tr != (NowTileTrend{Direction: "steady", Label: "Steady", Text: "Steady over 1 hour"}) {
		t.Errorf("pH's trend: %+v", tr)
	}
}

func TestTrend(t *testing.T) {
	t.Parallel()
	series := func(values ...float64) []TrendPoint {
		var out []TrendPoint
		for i, v := range values {
			out = append(out, TrendPoint{At: time.Date(2026, 10, 1+i, 0, 0, 0, 0, time.UTC), Value: v})
		}
		return out
	}
	noise := 0.3
	rising := series(20, 21, 22, 23)
	slices.Reverse(rising)
	if got := Trend(rising, &noise, "°C"); got == nil || got.Text != "Rising, up 3°C over 3 days" {
		t.Errorf("rising: %+v", got)
	}
	if got := Trend(series(24, 22, 20), &noise, "°C"); got == nil || got.Text != "Falling, down 4°C over 2 days" {
		t.Errorf("falling: %+v", got)
	}
	if got := Trend(series(20, 20.2, 19.9, 20.1), &noise, "°C"); got == nil || got.Direction != "steady" || got.Text != "Steady over 3 days" {
		t.Errorf("steady: %+v", got)
	}
	if Trend(series(20), &noise, "°C") != nil {
		t.Error("one reading")
	}
	for d, want := range map[time.Duration]string{24 * time.Hour: "1 day", 5 * time.Hour: "5 hours", 40 * time.Minute: "40 minutes"} {
		if got := spanText(d); got != want {
			t.Errorf("%v: %s", d, got)
		}
	}
}

func TestMonthCompleteness(t *testing.T) {
	t.Parallel()
	soil := &DomainRef{ID: "SOIL", Label: "Soil", Icon: "soil"}
	domainOf := func(id string) *DomainRef {
		if len(id) >= 4 && id[:4] == "soil" {
			return soil
		}
		return nil
	}
	h := func(at, param string) HourStat { return hour("", param, at, 1) }
	got := MonthCompleteness([]HourStat{h("2026-10-01T08:00:00Z", "rain"), h("2026-10-01T18:00:00Z", "rain"), h("2026-10-03T08:00:00Z", "rain"),
		h("2026-09-30T08:00:00Z", "rain"), h("2026-10-05T08:00:00Z", "rain")}, "2026-10-04", []Expectation{{"rain", 1}}, domainOf)
	if got.Month != "2026-10" || len(got.Domains) != 1 {
		t.Fatalf("%+v", got)
	}
	var statuses []string
	for _, d := range got.Domains[0].Days {
		statuses = append(statuses, d.Status)
	}
	if r := got.Domains[0]; r.Domain != nil || r.Received != 2 || r.Expected != 4 || !slices.Equal(statuses, []string{"complete", "missing", "complete", "missing"}) {
		t.Errorf("rain: %+v", r)
	}
	partial := MonthCompleteness([]HourStat{h("2026-10-01T08:00:00Z", "soil_moisture")}, "2026-10-01",
		[]Expectation{{"soil_moisture", 1}, {"soil_temperature", 1}}, domainOf).Domains[0]
	if partial.Received != 1 || partial.Expected != 2 || partial.Days[0].Status != "partial" {
		t.Errorf("partial: %+v", partial)
	}
	weekly := MonthCompleteness([]HourStat{h("2026-10-02T08:00:00Z", "soil_moisture")}, "2026-10-10", []Expectation{{"soil_moisture", 1.0 / 7}}, domainOf).Domains[0]
	if weekly.Received != 1 || weekly.Expected != 2 || len(weekly.Days) != 0 {
		t.Errorf("weekly: %+v", weekly)
	}
	fastest := MonthCompleteness([]HourStat{h("2026-10-01T08:00:00Z", "rain")}, "2026-10-01", []Expectation{{"rain", 1}, {"rain", 24}}, domainOf).Domains[0]
	if fastest.Received != 1 || fastest.Expected != 24 || fastest.Days[0].Status != "partial" {
		t.Errorf("the fastest rate: %+v", fastest)
	}
}

func TestDeviceSummary(t *testing.T) {
	t.Parallel()
	newest := ts("2026-10-20T06:00:00Z")
	days := []DayStat{dayStat("st", "air_temperature", "2026-10-20T06:00:00Z", 18, 22, 21), dayStat("st", "air_temperature", "2026-10-18T20:00:00Z", 15, 25, 17)}
	cards := DeviceSummary([]KeyValue{{ID: "air_temperature", Label: "Air temperature", Unit: "°C", ValueType: "number", Value: 21.0},
		{ID: "note", Label: "Note", ValueType: "text", Value: "cloudy"}}, &newest, newest, days, "st",
		func(string) string { return "line_band" }, func(string) string { return Released })
	if len(cards) != 1 {
		t.Fatalf("numbers only: %+v", cards)
	}
	c := cards[0]
	if *c.Latest != 21 || *c.TodayLow != 18 || *c.TodayHigh != 22 || *c.WeekLow != 15 || *c.WeekHigh != 25 || len(c.Week) != 7 ||
		c.Week[4].Value == nil || *c.Week[4].Value != 17 || c.Week[5].Value != nil || c.Chart != "line_band" || c.Review != Released {
		t.Errorf("the card: %+v", c)
	}
	empty := DeviceSummary([]KeyValue{{ID: "air_temperature", Label: "Air temperature", ValueType: "number", Value: nil}}, nil, newest, nil, "st",
		func(string) string { return "line" }, func(string) string { return NotReleased })
	if len(empty) != 1 || empty[0].Latest != nil || len(empty[0].Week) != 7 {
		t.Errorf("no reading yet: %+v", empty)
	}
}
