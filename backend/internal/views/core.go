// Package views builds the front end's view models from what SAM returns,
// carried over from the front end with its rounding and wording
// (docs/design/dashboards-and-public-site.md#view-models).
package views

import (
	"math"
	"slices"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// Review states of a classroom's data.
const (
	Released    = "released"
	InReview    = "in_review"
	NotReleased = "not_released"
)

// day is a date as the views write it.
const day = "2006-01-02"

// Latest is the newest value of one parameter from one source.
type Latest struct {
	ParameterID string
	Value       any
	ObservedAt  time.Time
	ReceivedAt  time.Time
}

// RainDay is one source's rain on one UTC day for one parameter: the
// highest of the day's running totals, and when it last reported.
type RainDay struct {
	Day         string
	Source      string
	ParameterID string
	Max         float64
	LastAt      time.Time
}

// CoreReadings is the front end's CoreReadings: the shared core readings
// of one classroom. Missing values are nil, never zero.
type CoreReadings struct {
	Day            *string    `json:"day"`
	RainToday      *float64   `json:"rainToday"`
	RainLast7Days  *float64   `json:"rainLast7Days"`
	RainDays       int        `json:"rainDays"`
	AirTemperature *float64   `json:"airTemperature"`
	LastSync       *time.Time `json:"lastSync"`
	Review         string     `json:"review"`
}

// NoCore is the core readings of a classroom with nothing to show.
func NoCore() CoreReadings { return CoreReadings{Review: NotReleased} }

func round(v float64) float64 { return math.Round(v*10) / 10 }

// Core is a classroom's core readings from what a reader may read, as the
// front end's coreReadings computes them from released data. "Today" is the
// UTC day of the newest value, so a quiet station shows its last day with
// its sync time rather than a zero. The rain of a day is the reference
// gauge's total when the gauge reported, otherwise the source reporting
// latest that day; the week sums the days with a total. A visitor reads
// released data only, so their review is released, or not released while
// there is none.
func Core(latest []Latest, rain []RainDay, p settings.CoreParameters) CoreReadings {
	if len(latest) == 0 {
		return NoCore()
	}
	newest := latest[0]
	for _, l := range latest[1:] {
		if l.ObservedAt.After(newest.ObservedAt) {
			newest = l
		}
	}
	today := newest.ObservedAt.UTC().Format(day)
	synced := newest.ReceivedAt
	out := CoreReadings{Day: &today, LastSync: &synced, Review: Released, AirTemperature: airTemperature(latest, p.AirTemperature)}
	end, _ := time.Parse(day, today)
	var sum float64
	for i := range 7 {
		d := end.AddDate(0, 0, -i).Format(day)
		v, ok := rainOn(rain, d, p)
		if !ok {
			continue
		}
		if i == 0 {
			out.RainToday = &v
		}
		sum += v
		out.RainDays++
	}
	if out.RainDays > 0 {
		total := round(sum)
		out.RainLast7Days = &total
	}
	return out
}

// airTemperature is the newest air temperature, the first of ids on a
// tie.
func airTemperature(latest []Latest, ids []string) *float64 {
	var best *Latest
	rank := func(l Latest) int { return slices.Index(ids, l.ParameterID) }
	for i, l := range latest {
		if _, ok := l.Value.(float64); !ok || rank(l) < 0 {
			continue
		}
		if best == nil || l.ObservedAt.After(best.ObservedAt) || l.ObservedAt.Equal(best.ObservedAt) && rank(l) < rank(*best) {
			best = &latest[i]
		}
	}
	if best == nil {
		return nil
	}
	v := best.Value.(float64)
	return &v
}

// rainOn is day's rain: the reference gauge's total when it reported,
// otherwise the source reporting latest that day; each source's value is
// that of the first of the rain parameters it reported.
func rainOn(rain []RainDay, d string, p settings.CoreParameters) (float64, bool) {
	type source struct {
		value     float64
		rank      int
		reference bool
		last      time.Time
	}
	by := map[string]*source{}
	var order []string
	for _, r := range rain {
		rank := slices.Index(p.RainDaily, r.ParameterID)
		if r.Day != d || rank < 0 {
			continue
		}
		s, ok := by[r.Source]
		if !ok {
			s = &source{rank: rank, value: r.Max}
			by[r.Source] = s
			order = append(order, r.Source)
		} else if rank < s.rank {
			s.rank, s.value = rank, r.Max
		}
		s.reference = s.reference || r.ParameterID == p.RainReference
		if r.LastAt.After(s.last) {
			s.last = r.LastAt
		}
	}
	if len(order) == 0 {
		return 0, false
	}
	slices.SortStableFunc(order, func(a, b string) int { return by[b].last.Compare(by[a].last) })
	shown := by[order[0]]
	for _, s := range order {
		if by[s].reference {
			shown = by[s]
			break
		}
	}
	return round(shown.value), true
}
