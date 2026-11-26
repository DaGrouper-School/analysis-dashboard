package views

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// "What's happening now?", device summaries and month completeness, as
// the front end's station-now and completeness compute them.

// Lookup is a registry parameter by ID, nil for one it does not hold.
type Lookup func(id string) *registry.Parameter

// Value is the newest value of one parameter from one source.
type Value struct {
	ParameterID string
	Value       any
	ObservedAt  time.Time
	Source      string
}

// NowTile is the front end's NowTile.
type NowTile struct {
	ID          string        `json:"id"`
	Label       string        `json:"label"`
	Icon        string        `json:"icon"`
	ParameterID *string       `json:"parameterId"`
	Value       any           `json:"value"`
	Unit        string        `json:"unit"`
	Caption     *string       `json:"caption"`
	ObservedAt  *time.Time    `json:"observedAt"`
	Last24Hours []*float64    `json:"last24Hours"`
	Trend       *NowTileTrend `json:"trend"`
}

// NowShown is the value a tile shows, for its last 24 hours and trend: its
// source, nil for a tile without one.
type NowShown struct {
	Tile  int
	Value Value
}

// NowTiles are "What's happening now?" from the newest values, as the
// front end's nowTiles: per tile, the first of its values with data, the
// newest across sources, or the classroom's core reading when the tile
// names one. Missing data stays nil. shown are the values tiles show, for
// their last 24 hours and trends.
func NowTiles(config []settings.NowTile, values []Value, param Lookup, core *CoreReadings) (tiles []NowTile, shown []NowShown) {
	n := nowValues{values: values, param: param}
	tiles = []NowTile{}
	for i, t := range config {
		if t.Core != nil && core != nil {
			tiles = append(tiles, n.coreTile(t, core))
			continue
		}
		tile, v := n.valueTile(t)
		if v != nil {
			shown = append(shown, NowShown{Tile: i, Value: *v})
		}
		tiles = append(tiles, tile)
	}
	return tiles, shown
}

// nowValues are the newest values tiles choose from.
type nowValues struct {
	values []Value
	param  Lookup
}

// latest is the newest value of the first of ids with one.
func (n nowValues) latest(ids []string) *Value {
	for _, id := range ids {
		var best *Value
		for i, v := range n.values {
			if v.ParameterID == id && (best == nil || v.ObservedAt.After(best.ObservedAt)) {
				best = &n.values[i]
			}
		}
		if best != nil {
			return best
		}
	}
	return nil
}

func (n nowValues) unitOf(id string) string {
	if p := n.param(id); p != nil {
		return p.Unit
	}
	return ""
}

// coreTile is a tile taken from the classroom's core readings.
func (n nowValues) coreTile(t settings.NowTile, core *CoreReadings) NowTile {
	tile := NowTile{ID: t.ID, Label: t.Label, Icon: t.Icon, Last24Hours: []*float64{}, Unit: n.unitOf(t.ParameterIDs[0])}
	value := core.RainToday
	if t.Core.Value == "airTemperature" {
		value = core.AirTemperature
	}
	if value != nil {
		tile.Value, tile.ObservedAt = *value, core.LastSync
		tile.Caption = coreCaption(t, *value, core.RainLast7Days, tile.Unit)
	}
	return tile
}

// valueTile is a tile showing the newest of its values, and that value.
func (n nowValues) valueTile(t settings.NowTile) (NowTile, *Value) {
	tile := NowTile{ID: t.ID, Label: t.Label, Icon: t.Icon, Last24Hours: []*float64{}}
	v := n.latest(t.ParameterIDs)
	id := t.ParameterIDs[0]
	if v != nil {
		id = v.ParameterID
		pid, at := v.ParameterID, v.ObservedAt
		tile.ParameterID, tile.Value, tile.ObservedAt = &pid, v.Value, &at
	}
	tile.Unit = n.unitOf(id)
	switch {
	case v != nil && t.ZeroText != "" && v.Value == 0.0:
		tile.Caption = &t.ZeroText
	case v != nil && t.Caption != nil:
		tile.Caption = n.caption(*t.Caption)
	}
	return tile, v
}

// caption is a tile's line under its value, nil when its value has none.
func (n nowValues) caption(c settings.NowCaption) *string {
	extra := n.latest(c.ParameterIDs)
	if extra == nil {
		return nil
	}
	text := FormatValue(extra.Value, "", "number")
	num, isNumber := extra.Value.(float64)
	if p := n.param(extra.ParameterID); c.Format == "compass" && isNumber {
		text = CompassPoint(num)
	} else if p != nil {
		text = FormatValue(extra.Value, p.Unit, p.ValueType)
	}
	out := strings.ReplaceAll(c.Text, "{value}", text)
	return &out
}

func coreCaption(t settings.NowTile, value float64, week *float64, unit string) *string {
	var parts []string
	if value == 0 && t.ZeroText != "" {
		parts = append(parts, t.ZeroText)
	}
	if t.Core.Caption != nil && week != nil {
		parts = append(parts, strings.ReplaceAll(t.Core.Caption.Text, "{value}", FormatValue(*week, unit, "number")))
	}
	if len(parts) == 0 {
		return nil
	}
	c := strings.Join(parts, " · ")
	return &c
}

// TrendOf is the trend of a source's value over the 24 hours up to until,
// from its hourly values.
func TrendOf(hours []HourStat, source, parameterID string, until time.Time, p *registry.Parameter) *NowTileTrend {
	if p == nil {
		return nil
	}
	var points []TrendPoint
	for _, h := range hours {
		if h.Source == source && h.ParameterID == parameterID && !h.LastAt.After(until) && h.LastAt.After(until.Add(-24*time.Hour)) {
			points = append(points, TrendPoint{At: h.LastAt, Value: h.Last})
		}
	}
	return Trend(points, p.Noise, p.Unit)
}

// KeyValue is one value a device leads with.
type KeyValue struct {
	ID, Label, Unit, ValueType string
	Value                      any
	Icon                       *string
}

// KeyValues are the values a reading leads with, as the front end's
// keyValues: ids from its device type, in order, those the reading holds;
// with fallback, a reading with none of them shows what it sent. Each
// value's icon is its domain's.
func KeyValues(ids []string, values map[string]any, param Lookup, domainIcons map[string]string, fallback bool) []KeyValue {
	var chosen []string
	for _, id := range ids {
		if _, ok := values[id]; ok {
			chosen = append(chosen, id)
		}
	}
	if len(chosen) == 0 && fallback {
		for id := range values {
			chosen = append(chosen, id)
		}
		slices.Sort(chosen)
	}
	out := []KeyValue{}
	for _, id := range chosen {
		kv := KeyValue{ID: id, Label: fallbackLabel(id), ValueType: "number", Value: values[id]}
		if p := param(id); p != nil {
			kv.Label, kv.Unit, kv.ValueType = p.Label, p.Unit, p.ValueType
			if p.Domain != nil {
				if icon, ok := domainIcons[*p.Domain]; ok {
					kv.Icon = &icon
				}
			}
		}
		out = append(out, kv)
	}
	return out
}

func fallbackLabel(id string) string {
	s := strings.ReplaceAll(id, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ReadingTiles are a device's tiles, as the front end's readingTiles: its
// key values from its newest reading, observed at at, each with its last
// 24 hours and which way it went.
func ReadingTiles(values []KeyValue, at *time.Time, icon string, hours []HourStat, source string, param Lookup) []NowTile {
	out := []NowTile{}
	for _, v := range values {
		id := v.ID
		t := NowTile{ID: v.ID, Label: v.Label, Icon: icon, ParameterID: &id, Value: v.Value, Unit: v.Unit, ObservedAt: at, Last24Hours: []*float64{}}
		if v.Icon != nil {
			t.Icon = *v.Icon
		}
		if _, ok := v.Value.(float64); ok && at != nil {
			t.Last24Hours = Last24Hours(hours, source, []string{v.ID}, *at)
			t.Trend = TrendOf(hours, source, v.ID, *at, param(v.ID))
		}
		out = append(out, t)
	}
	return out
}

// SummaryDay is one day of a device card.
type SummaryDay struct {
	Day   string   `json:"day"`
	Value *float64 `json:"value"`
	Low   *float64 `json:"low"`
	High  *float64 `json:"high"`
}

// DeviceSummaryCard is the front end's DeviceSummaryCard.
type DeviceSummaryCard struct {
	ID         string       `json:"id"`
	Label      string       `json:"label"`
	Unit       string       `json:"unit"`
	Icon       *string      `json:"icon"`
	Latest     *float64     `json:"latest"`
	ObservedAt *time.Time   `json:"observedAt"`
	Week       []SummaryDay `json:"week"`
	Chart      string       `json:"chart"`
	Review     string       `json:"review"`
	TodayLow   *float64     `json:"todayLow"`
	TodayHigh  *float64     `json:"todayHigh"`
	WeekLow    *float64     `json:"weekLow"`
	WeekHigh   *float64     `json:"weekHigh"`
}

// DeviceSummary is a device's cards over the seven days up to its newest
// reading, as the front end's deviceSummary: the newest value, each day's
// last value and range, and today's and the week's range. A device with
// no reading yet keeps its numeric cards with no data, on the seven days
// up to now. days are its own values by day.
func DeviceSummary(values []KeyValue, newest *time.Time, now time.Time, days []DayStat, source string,
	chart func(id string) string, review func(id string) string) []DeviceSummaryCard {
	today := now.UTC().Format(day)
	if newest != nil {
		today = newest.UTC().Format(day)
	}
	week := daysUpTo(today, 7)
	out := []DeviceSummaryCard{}
	for _, v := range values {
		n, isNumber := v.Value.(float64)
		if !isNumber && (newest != nil || v.ValueType != "number") {
			continue
		}
		c := DeviceSummaryCard{ID: v.ID, Label: v.Label, Unit: v.Unit, Icon: v.Icon, ObservedAt: newest, Chart: chart(v.ID), Review: review(v.ID)}
		if newest != nil {
			c.Latest = &n
		}
		var weekLow, weekHigh []float64
		for _, d := range week {
			sd := SummaryDay{Day: d}
			for _, s := range days {
				if s.Source == source && s.ParameterID == v.ID && s.Day == d {
					sd.Value, sd.Low, sd.High = ptr(s.Last), ptr(s.Min), ptr(s.Max)
					weekLow, weekHigh = append(weekLow, s.Min), append(weekHigh, s.Max)
				}
			}
			if d == today {
				c.TodayLow, c.TodayHigh = sd.Low, sd.High
			}
			c.Week = append(c.Week, sd)
		}
		if len(weekLow) > 0 {
			c.WeekLow, c.WeekHigh = ptr(slices.Min(weekLow)), ptr(slices.Max(weekHigh))
		}
		out = append(out, c)
	}
	return out
}

// ChartOf is a value's chart: its registry choice, a line for other
// numbers, and none for text.
func ChartOf(p *registry.Parameter) string {
	switch {
	case p == nil:
		return "line"
	case p.ValueType != "" && p.ValueType != "number":
		return ""
	case p.Chart != "":
		return p.Chart
	}
	return "line"
}

// CompletenessDomain is the front end's DomainCompleteness.
type CompletenessDomain struct {
	Domain   *DomainRef        `json:"domain"`
	Received int               `json:"received"`
	Expected int               `json:"expected"`
	Days     []CompletenessDay `json:"days"`
}

// DomainRef is a domain as a page names it.
type DomainRef struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
}

// CompletenessDay is one day's status: complete, partial or missing.
type CompletenessDay struct {
	Day    string `json:"day"`
	Status string `json:"status"`
}

// Completeness is "How complete is this month?".
type Completeness struct {
	Month   string               `json:"month"`
	Domains []CompletenessDomain `json:"domains"`
}

// Expectation is how often a classroom's protocol reads a value: 24 is
// hourly, 1 daily, 1/7 weekly.
type Expectation struct {
	ParameterID    string
	ReadingsPerDay float64
}

// MonthCompleteness is how complete the month of d is, as the front end's
// monthCompleteness: for each domain, the protocol's slots that have a
// reading, out of those expected from the first of the month through d,
// each counting once; and each day's status for values read daily or
// faster. hours are the values read, by hour.
func MonthCompleteness(hours []HourStat, d string, expectations []Expectation, domainOf func(parameterID string) *DomainRef) Completeness {
	month := d[:7]
	start, _ := time.Parse(day, month+"-01")
	endDay, _ := time.Parse(day, d)
	end := endDay.AddDate(0, 0, 1)
	dayCount := int(math.Round(end.Sub(start).Hours() / 24))
	rates, order := fastestRates(expectations)
	slots := filledSlots(hours, rates, start, end)
	out := Completeness{Month: month, Domains: []CompletenessDomain{}}
	for _, g := range byDomain(order, domainOf) {
		out.Domains = append(out.Domains, domainCompleteness(g.domain, g.ids, rates, slots, start, dayCount))
	}
	return out
}

// fastestRates are the fastest rate any site asks for, per value, and the
// values in the order first asked for.
func fastestRates(expectations []Expectation) (map[string]float64, []string) {
	rates := map[string]float64{}
	var order []string
	for _, e := range expectations {
		if _, ok := rates[e.ParameterID]; !ok {
			order = append(order, e.ParameterID)
		}
		rates[e.ParameterID] = math.Max(rates[e.ParameterID], e.ReadingsPerDay)
	}
	return rates, order
}

// slotOf is the slot a time counts for: an hour, a day or a week from
// start, by the rate.
func slotOf(t, start time.Time, perDay float64) int {
	size := 7 * 24 * time.Hour
	if perDay >= 1 {
		size = 24 * time.Hour / time.Duration(math.Round(perDay))
	}
	return int(t.Sub(start) / size)
}

// filledSlots are the slots of each value that have a reading from start
// to end.
func filledSlots(hours []HourStat, rates map[string]float64, start, end time.Time) map[string]map[int]bool {
	slots := map[string]map[int]bool{}
	for _, h := range hours {
		perDay, ok := rates[h.ParameterID]
		if !ok || h.Count == 0 || h.LastAt.Before(start) || !h.LastAt.Before(end) {
			continue
		}
		if slots[h.ParameterID] == nil {
			slots[h.ParameterID] = map[int]bool{}
		}
		slots[h.ParameterID][slotOf(h.LastAt, start, perDay)] = true
	}
	return slots
}

type domainGroup struct {
	domain *DomainRef
	ids    []string
}

// byDomain are the values by domain, in the order the domains first come.
func byDomain(ids []string, domainOf func(string) *DomainRef) []*domainGroup {
	var groups []*domainGroup
	byKey := map[string]*domainGroup{}
	for _, id := range ids {
		dom := domainOf(id)
		key := ""
		if dom != nil {
			key = dom.ID
		}
		g, ok := byKey[key]
		if !ok {
			g = &domainGroup{domain: dom}
			byKey[key] = g
			groups = append(groups, g)
		}
		g.ids = append(g.ids, id)
	}
	return groups
}

// domainCompleteness is one domain's slots received and expected, and
// each day's status for the values read daily or faster.
func domainCompleteness(dom *DomainRef, ids []string, rates map[string]float64, slots map[string]map[int]bool, start time.Time, dayCount int) CompletenessDomain {
	c := CompletenessDomain{Domain: dom, Days: []CompletenessDay{}}
	var daily []string
	for _, id := range ids {
		perDay := rates[id]
		total := int(math.Ceil(float64(dayCount) / 7))
		if perDay >= 1 {
			total = dayCount * int(math.Round(perDay))
			daily = append(daily, id)
		}
		c.Expected += total
		c.Received += min(len(slots[id]), total)
	}
	if len(daily) == 0 {
		return c
	}
	for i := range dayCount {
		want, got := 0, 0
		for _, id := range daily {
			perDay := int(math.Round(rates[id]))
			want += perDay
			for s := i * perDay; s < (i+1)*perDay; s++ {
				if slots[id][s] {
					got++
				}
			}
		}
		status := "complete"
		switch {
		case got == 0:
			status = "missing"
		case got < want:
			status = "partial"
		}
		c.Days = append(c.Days, CompletenessDay{Day: start.AddDate(0, 0, i).Format(day), Status: status})
	}
	return c
}
