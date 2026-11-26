package settings

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
)

// The settings' shapes, as the front end has them, and the rules the
// server and the pages rely on.

// CoreParameters is core_parameters: which registry values feed the shared
// core readings.
type CoreParameters struct {
	RainDaily      []string `json:"rainDaily"`
	AirTemperature []string `json:"airTemperature"`
	// RainTolerance is how far apart two rain sources may be on a day, in
	// mm, before the day goes to review.
	RainTolerance float64 `json:"rainTolerance"`
	// RainReference is the manual gauge's value, one of RainDaily.
	RainReference string `json:"rainReference"`
}

// DataTerms is default_terms: the terms a new classroom's data starts
// with.
type DataTerms struct {
	Licence      string   `json:"licence"`
	LicenceURL   *string  `json:"licenceUrl"`
	Attribution  string   `json:"attribution"`
	Restrictions []string `json:"restrictions"`
	// CoordinateDecimals rounds positions before they leave SAM; null keeps
	// them in full.
	CoordinateDecimals *int `json:"coordinateDecimals"`
}

// Privacy is privacy: the rounding of a minor's positions.
type Privacy struct {
	// MinorCoordinateDecimals rounds the positions a minor's phone sends.
	MinorCoordinateDecimals int `json:"minorCoordinateDecimals"`
}

// Future is future_observations: what becomes of a reading or observation
// dated after it is received, by more than the clock tolerance. SAM
// refuses one dated more than a day ahead whatever this says.
type Future struct {
	// Action is flag, to store it flagged, or refuse.
	Action string `json:"action"`
}

// The actions on future-dated observations.
const (
	FutureFlag   = "flag"
	FutureRefuse = "refuse"
)

func (f Future) check() []string {
	if f.Action != FutureFlag && f.Action != FutureRefuse {
		return []string{fmt.Sprintf("action: %q is not flag or refuse", f.Action)}
	}
	return nil
}

// MaxDecimals is the finest rounding a position may have: about 10 m.
const MaxDecimals = 4

// parameterID is a registry value's ID: SAM's form, with a tenant's
// namespace or without.
var parameterID = regexp.MustCompile(`^(?:[a-z][a-z0-9_]*:)?[a-z][a-z0-9_.]*$`)

func (c CoreParameters) check() []string {
	var p []string
	for _, list := range []struct {
		name string
		ids  []string
	}{{"rainDaily", c.RainDaily}, {"airTemperature", c.AirTemperature}} {
		if len(list.ids) == 0 {
			p = append(p, list.name+": empty; name at least one registry value")
		}
		p = append(p, jsoncheck.IDs(list.name, list.ids, func(s string) string { return s }, parameterID)...)
	}
	if !slices.Contains(c.RainDaily, c.RainReference) {
		p = append(p, fmt.Sprintf("rainReference: %q is not one of rainDaily", c.RainReference))
	}
	if c.RainTolerance < 0 {
		p = append(p, fmt.Sprintf("rainTolerance: %s is below zero", strconv.FormatFloat(c.RainTolerance, 'f', -1, 64)))
	}
	return p
}

func (t DataTerms) check() []string {
	var p []string
	if t.LicenceURL != nil && !jsoncheck.HTTPSURL(*t.LicenceURL) {
		p = append(p, fmt.Sprintf("licenceUrl: %q is not an https URL", *t.LicenceURL))
	}
	if d := t.CoordinateDecimals; d != nil && (*d < 0 || *d > MaxDecimals) {
		p = append(p, fmt.Sprintf("coordinateDecimals: %d is not from 0 to %d, or null for full precision", *d, MaxDecimals))
	}
	return append(p, jsoncheck.Placeholders("attribution", t.Attribution, "classroom", "institution")...)
}

func (v Privacy) check() []string {
	if d := v.MinorCoordinateDecimals; d < 0 || d > MaxDecimals {
		return []string{fmt.Sprintf("minorCoordinateDecimals: %d is not from 0 to %d", d, MaxDecimals)}
	}
	return nil
}
