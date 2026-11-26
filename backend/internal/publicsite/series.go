package publicsite

import (
	"slices"
	"strings"
	"time"
)

// The pure part of a parameter page: points from SAM, by source and
// value, into one series per source.

// Source is the front end's PublicSource: a device by its name, or the
// classroom's field observations.
type Source struct {
	ID    string  `json:"id"`
	Name  *string `json:"name"`
	Kind  string  `json:"kind"`
	Color string  `json:"color,omitempty"`
}

// Point is one value of a series.
type Point struct {
	ObservedAt time.Time `json:"observedAt"`
	Value      any       `json:"value"`
}

// Series is the front end's PublicParameterSeries.
type Series struct {
	Source      Source  `json:"source"`
	ParameterID string  `json:"parameterId"`
	Points      []Point `json:"points"`
}

// sourced is a point from SAM: the source it came from and the value it
// reports.
type sourced struct {
	source      Source
	parameterID string
	Point
}

// seriesOf is one series per source, oldest first. A source reports the
// first of ids it has values of, so a gauge and a station show the same
// rain under the values each reports, and its other values are left out.
// For rain, the first source reporting a value a person reads by hand
// (manual) is the reference every other is checked against, and comes
// first; the others come by name.
func seriesOf(points []sourced, ids []string, rain bool, manual func(parameterID string) bool) (out []Series, reference *string) {
	reports := map[string]string{}
	for _, p := range points {
		if cur, ok := reports[p.source.ID]; !ok || slices.Index(ids, p.parameterID) < slices.Index(ids, cur) {
			reports[p.source.ID] = p.parameterID
		}
	}
	bySource := map[string]*Series{}
	var order []string
	for _, p := range points {
		if reports[p.source.ID] != p.parameterID {
			continue
		}
		s, ok := bySource[p.source.ID]
		if !ok {
			s = &Series{Source: p.source, ParameterID: p.parameterID, Points: []Point{}}
			bySource[p.source.ID] = s
			order = append(order, p.source.ID)
		}
		s.Points = append(s.Points, p.Point)
	}
	for _, id := range order {
		s := bySource[id]
		slices.SortStableFunc(s.Points, func(a, b Point) int { return a.ObservedAt.Compare(b.ObservedAt) })
		out = append(out, *s)
	}
	name := func(s Series) string {
		if s.Source.Name == nil {
			return ""
		}
		return strings.ToLower(*s.Source.Name)
	}
	slices.SortFunc(out, func(a, b Series) int { return strings.Compare(name(a), name(b)) })
	if rain {
		for i, s := range out {
			if manual(s.ParameterID) {
				id := s.Source.ID
				reference = &id
				out = append([]Series{s}, append(out[:i:i], out[i+1:]...)...)
				break
			}
		}
	}
	if out == nil {
		out = []Series{}
	}
	return out, reference
}

// rainIDs are the values a page of id shows together: for one of the
// day's rain values, every one of them, id first; otherwise id alone.
func rainIDs(id string, rain []string) []string {
	if !slices.Contains(rain, id) {
		return []string{id}
	}
	out := []string{id}
	for _, r := range rain {
		if r != id {
			out = append(out, r)
		}
	}
	return out
}

// seasonDays is the longest range a parameter page charts: three months,
// long enough for the rains, as the front end's season.
const seasonDays = 91

// season is the window a page's points span: whole UTC days, the last of
// them the day of latest.
func season(latest time.Time) time.Time {
	d := latest.UTC()
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(seasonDays - 1))
}
