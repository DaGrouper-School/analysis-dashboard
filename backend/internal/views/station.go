package views

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// The station page's views, from SAM's aggregates: each source's values
// by UTC day and by hour, in place of the front end's records.

// DayStat is one source's values of one parameter on one UTC day: the
// lowest, the highest and the last, with when it was observed.
type DayStat struct {
	Source, ParameterID, Day string
	Min, Max, Last           float64
	LastAt                   time.Time
}

// HourStat is one source's values of one parameter in one hour: the last,
// when it was observed, and how many there were.
type HourStat struct {
	Source, ParameterID string
	Hour, LastAt        time.Time
	Last                float64
	Count               int64
}

// StationCoreItem is the front end's StationCoreItem.
type StationCoreItem struct {
	Review  string   `json:"review"`
	Sources int      `json:"sources"`
	Spread  *float64 `json:"spread"`
}

// StationRainDay is the front end's StationRainDay.
type StationRainDay struct {
	Day       string   `json:"day"`
	Value     *float64 `json:"value"`
	Reference bool     `json:"reference"`
	Flagged   bool     `json:"flagged"`
}

// RainSource is one source's rain on the core day.
type RainSource struct {
	SourceID  string  `json:"sourceId"`
	Name      *string `json:"name"`
	Value     float64 `json:"value"`
	Reference bool    `json:"reference"`
}

// StationRain is the core detail on rain.
type StationRain struct {
	StationCoreItem
	BySource  []RainSource     `json:"bySource"`
	Tolerance float64          `json:"tolerance"`
	Week      []StationRainDay `json:"week"`
}

// StationTemperature is the core detail on air temperature.
type StationTemperature struct {
	StationCoreItem
	Low       *float64 `json:"low"`
	High      *float64 `json:"high"`
	MonthLow  *float64 `json:"monthLow"`
	MonthHigh *float64 `json:"monthHigh"`
}

// StationCore is the front end's StationCore.
type StationCore struct {
	Rain        StationRain        `json:"rain"`
	Temperature StationTemperature `json:"temperature"`
}

// daysUpTo are the count days up to and including d, oldest first.
func daysUpTo(d string, count int) []string {
	end, _ := time.Parse(day, d)
	out := make([]string, count)
	for i := range out {
		out[i] = end.AddDate(0, 0, i-(count-1)).Format(day)
	}
	return out
}

func spreadOf(values []float64) *float64 {
	if len(values) < 2 {
		return nil
	}
	s := round(slices.Max(values) - slices.Min(values))
	return &s
}

func ptr(v float64) *float64 { return &v }

// rainBySource is each source's rain on d: the highest running total of
// the first of the rain values it reported that day.
func rainBySource(days []DayStat, d string, p settings.CoreParameters) map[string]float64 {
	rank := map[string]int{}
	out := map[string]float64{}
	for _, s := range days {
		r := slices.Index(p.RainDaily, s.ParameterID)
		if s.Day != d || r < 0 {
			continue
		}
		if cur, ok := rank[s.Source]; !ok || r < cur {
			rank[s.Source], out[s.Source] = r, s.Max
		}
	}
	return out
}

// rainDayOn is d's rain as the front end's rainDayOn has it: the
// reference gauge's total when it reported, otherwise the source
// reporting latest, and whether the sources are further apart than the
// protocol allows.
func rainDayOn(days []DayStat, d string, p settings.CoreParameters) StationRainDay {
	out := StationRainDay{Day: d}
	var rain []RainDay
	values := []float64{}
	for _, s := range days {
		if s.Day == d && slices.Contains(p.RainDaily, s.ParameterID) {
			rain = append(rain, RainDay{Day: s.Day, Source: s.Source, ParameterID: s.ParameterID, Max: s.Max, LastAt: s.LastAt})
			out.Reference = out.Reference || s.ParameterID == p.RainReference
		}
	}
	for _, v := range rainBySource(days, d, p) {
		values = append(values, v)
	}
	if v, ok := rainOn(rain, d, p); ok {
		out.Value = &v
	}
	if s := spreadOf(values); s != nil && *s > p.RainTolerance {
		out.Flagged = true
	}
	return out
}

// Station is the station page's detail on the core readings, as the front
// end's stationCore: the core day's rain by source and how far apart the
// sources are, the week's rain, and the day's and the month's air
// temperature range. days are the released values; review says where a
// set of values stands in review; d is the core day, nil with nothing
// released.
func Station(days []DayStat, d *string, review func(ids []string) string, p settings.CoreParameters, nameOf func(source string) *string) StationCore {
	out := StationCore{
		Rain:        StationRain{StationCoreItem: StationCoreItem{Review: review(p.RainDaily)}, BySource: []RainSource{}, Tolerance: p.RainTolerance, Week: []StationRainDay{}},
		Temperature: StationTemperature{StationCoreItem: StationCoreItem{Review: review(p.AirTemperature)}},
	}
	if d == nil {
		return out
	}
	reference := map[string]bool{}
	for _, s := range days {
		if s.ParameterID == p.RainReference {
			reference[s.Source] = true
		}
	}
	rain := rainBySource(days, *d, p)
	values := []float64{}
	for source, v := range rain {
		values = append(values, v)
		out.Rain.BySource = append(out.Rain.BySource, RainSource{SourceID: source, Name: nameOf(source), Value: round(v), Reference: reference[source]})
	}
	slices.SortFunc(out.Rain.BySource, func(a, b RainSource) int {
		if a.Reference != b.Reference {
			if a.Reference {
				return -1
			}
			return 1
		}
		return strings.Compare(a.SourceID, b.SourceID)
	})
	out.Rain.Sources, out.Rain.Spread = len(rain), spreadOf(values)
	for _, w := range daysUpTo(*d, 7) {
		out.Rain.Week = append(out.Rain.Week, rainDayOn(days, w, p))
	}

	latest := map[string]DayStat{}
	monthFrom := daysUpTo(*d, 31)[0]
	var low, high, monthLow, monthHigh []float64
	for _, s := range days {
		if !slices.Contains(p.AirTemperature, s.ParameterID) {
			continue
		}
		if s.Day > monthFrom && s.Day <= *d {
			monthLow, monthHigh = append(monthLow, s.Min), append(monthHigh, s.Max)
		}
		if s.Day != *d {
			continue
		}
		low, high = append(low, s.Min), append(high, s.Max)
		if cur, ok := latest[s.Source]; !ok || s.LastAt.After(cur.LastAt) {
			latest[s.Source] = s
		}
	}
	temps := []float64{}
	for _, s := range latest {
		temps = append(temps, s.Last)
	}
	out.Temperature.Sources, out.Temperature.Spread = len(temps), spreadOf(temps)
	if len(low) > 0 {
		out.Temperature.Low, out.Temperature.High = ptr(slices.Min(low)), ptr(slices.Max(high))
	}
	if len(monthLow) > 0 {
		out.Temperature.MonthLow, out.Temperature.MonthHigh = ptr(slices.Min(monthLow)), ptr(slices.Max(monthHigh))
	}
	return out
}

// LastDays are a source's last value of a parameter on each of the seven
// days up to end, oldest first; nil for a day without one.
func LastDays(days []DayStat, source, parameterID, end string) []*float64 {
	week := daysUpTo(end, 7)
	out := make([]*float64, 7)
	for _, s := range days {
		if s.Source != source || s.ParameterID != parameterID {
			continue
		}
		if i := slices.Index(week, s.Day); i >= 0 {
			out[i] = ptr(s.Last)
		}
	}
	return out
}

// Last24Hours are a source's values of the first of ids it has, one an
// hour over the 24 hours up to until, oldest first: the last in each hour,
// nil for an hour without one.
func Last24Hours(hours []HourStat, source string, ids []string, until time.Time) []*float64 {
	slots := make([]*HourStat, 24)
	for i, h := range hours {
		if h.Source != source || !slices.Contains(ids, h.ParameterID) || h.LastAt.After(until) || !h.LastAt.After(until.Add(-24*time.Hour)) {
			continue
		}
		at := 23 - int(until.Sub(h.LastAt)/time.Hour)
		if cur := slots[at]; cur == nil || h.LastAt.After(cur.LastAt) ||
			h.LastAt.Equal(cur.LastAt) && slices.Index(ids, h.ParameterID) < slices.Index(ids, cur.ParameterID) {
			slots[at] = &hours[i]
		}
	}
	out := make([]*float64, 24)
	for i, s := range slots {
		if s != nil {
			out[i] = ptr(s.Last)
		}
	}
	return out
}

// TrendPoint is a value at a time, for a trend.
type TrendPoint struct {
	At    time.Time
	Value float64
}

// NowTileTrend is the front end's NowTileTrend.
type NowTileTrend struct {
	Direction string `json:"direction"`
	Label     string `json:"label"`
	Text      string `json:"text"`
}

var trendLabels = map[string]string{"rising": "Rising", "steady": "Steady", "falling": "Falling"}

// Trend is which way a value went over points, as the front end's
// recentTrend: a least-squares line through them and its change from the
// first to the last, steady within the parameter's noise or when it rounds
// to zero. Nil with fewer than two points.
func Trend(points []TrendPoint, noise *float64, unit string) *NowTileTrend {
	if len(points) < 2 {
		return nil
	}
	points = slices.Clone(points)
	slices.SortFunc(points, func(a, b TrendPoint) int { return a.At.Compare(b.At) })
	ms := func(t time.Time) float64 { return float64(t.UnixMilli()) }
	var meanT, meanV float64
	for _, p := range points {
		meanT += ms(p.At)
		meanV += p.Value
	}
	n := float64(len(points))
	meanT, meanV = meanT/n, meanV/n
	var spread, cov float64
	for _, p := range points {
		spread += (ms(p.At) - meanT) * (ms(p.At) - meanT)
		cov += (ms(p.At) - meanT) * (p.Value - meanV)
	}
	slope := 0.0
	if spread != 0 {
		slope = cov / spread
	}
	from, to := points[0].At, points[len(points)-1].At
	change := slope * (ms(to) - ms(from))
	shown := math.Round(change*10) / 10
	if math.Abs(change) >= 100 {
		shown = math.Round(change)
	}
	limit := 0.0
	if noise != nil {
		limit = *noise
	}
	direction := "falling"
	switch {
	case math.Abs(change) <= limit || shown == 0:
		direction = "steady"
	case change > 0:
		direction = "rising"
	}
	over := "over " + spanText(to.Sub(from))
	text := "Steady " + over
	if direction != "steady" {
		way := "down"
		if direction == "rising" {
			way = "up"
		}
		text = trendLabels[direction] + ", " + way + " " + FormatValue(math.Abs(change), unit, "number") + " " + over
	}
	return &NowTileTrend{Direction: direction, Label: trendLabels[direction], Text: text}
}

// spanText is the span a trend covers: "3 days", "14 hours", "40 minutes".
func spanText(d time.Duration) string {
	for _, u := range []struct {
		size time.Duration
		name string
	}{{24 * time.Hour, "day"}, {time.Hour, "hour"}, {time.Minute, "minute"}} {
		n := int(math.Round(float64(d) / float64(u.size)))
		if n >= 1 && (d >= u.size || u.name == "minute") {
			if n == 1 {
				return "1 " + u.name
			}
			return itoa(n) + " " + u.name + "s"
		}
	}
	return "under a minute"
}

func itoa(n int) string { return strconv.Itoa(n) }

// FormatValue is a value with its unit, rounded for reading at a glance,
// as the front end's formatValue: text as it is, a whole number from 100,
// otherwise one decimal, with a space before a unit unless it starts
// with a degree sign. A time (Unix seconds) is written in UTC.
func FormatValue(v any, unit, valueType string) string {
	switch x := v.(type) {
	case nil:
		return "—"
	case string:
		return x
	case float64:
		if valueType == "time" {
			return time.Unix(int64(x), 0).UTC().Format("2 Jan 2006, 15:04 UTC")
		}
		r := math.Round(x*10) / 10
		if math.Abs(x) >= 100 {
			r = math.Round(x)
		}
		sep := " "
		if unit == "" || strings.HasPrefix(unit, "°") {
			sep = ""
		}
		return formatNumber(r) + sep + unit
	}
	return "—"
}

// formatNumber writes a number as JavaScript's String does for the
// rounded values views show.
func formatNumber(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

var compassPoints = []string{"north", "north-northeast", "northeast", "east-northeast", "east", "east-southeast", "southeast",
	"south-southeast", "south", "south-southwest", "southwest", "west-southwest", "west", "west-northwest", "northwest", "north-northwest"}

// CompassPoint is a direction in degrees as a compass point in words: 112
// is east-southeast.
func CompassPoint(degrees float64) string {
	d := math.Mod(math.Mod(degrees, 360)+360, 360)
	return compassPoints[int(math.Round(d/22.5))%16]
}
