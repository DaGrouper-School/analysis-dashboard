package labs

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
)

// The front end's alert tests (frontend/src/lib/domain/alerts.spec.ts),
// on the seeded platform defaults.

func seededDefaults(t *testing.T) []Rule {
	t.Helper()
	raw, err := os.ReadFile("../registry/seed.json")
	if err != nil {
		t.Fatal(err)
	}
	var seed struct {
		AlertDefaults []registry.AlertRule `json:"alertDefaults"`
	}
	if err := json.Unmarshal(raw, &seed); err != nil {
		t.Fatal(err)
	}
	out := make([]Rule, len(seed.AlertDefaults))
	for i, r := range seed.AlertDefaults {
		out[i] = Rule{ID: r.ID, ParameterID: r.ParameterID, Direction: r.Direction, Value: r.Value, Level: r.Level, Title: r.Title,
			Enabled: r.Enabled, Source: ScopeDefault}
	}
	return out
}

var testParameters = map[string]*registry.Parameter{
	"uv":                            {ID: "uv", Label: "UV index"},
	"soil_moisture":                 {ID: "soil_moisture", Label: "Soil moisture", Unit: "%"},
	"rain_gauge_daily_accumulation": {ID: "rain_gauge_daily_accumulation", Label: "Rain gauge", Unit: "mm"},
	"air_temperature":               {ID: "air_temperature", Label: "Air temperature", Unit: "°C"},
}

func lookup(id string) *registry.Parameter { return testParameters[id] }

func layered(scope, param string, value float64, enabled bool) Rule {
	return Rule{ID: scope + "_" + param, ParameterID: param, Direction: "above", Value: value, Level: "warning", Title: "UV", Enabled: enabled, Source: scope}
}

func TestDefaultLevels(t *testing.T) {
	t.Parallel()
	platform := Effective(seededDefaults(t), nil, nil)
	got := AlertsFor(map[string]any{"uv": 9.1}, platform, lookup)
	want := Alert{ParameterID: "uv", Level: "warning", Title: "Very high UV", Message: "UV index 9.1 is at or above the default warning level of 8."}
	if len(got) != 1 || got[0] != want {
		t.Errorf("one alert, the most severe: %+v", got)
	}
	if got := AlertsFor(map[string]any{"uv": 6.8}, platform, lookup); len(got) != 1 || got[0].Level != "caution" {
		t.Errorf("caution: %+v", got)
	}
	if got := AlertsFor(map[string]any{"uv": 3.4, "air_temperature": 27.0, "pressure_trend": "steady"}, platform, lookup); len(got) != 0 {
		t.Errorf("quiet: %+v", got)
	}
	got = AlertsFor(map[string]any{"soil_moisture": 12.0, "rain_gauge_daily_accumulation": 61.0}, platform, lookup)
	if len(got) != 2 || got[0].Title != "Heavy rain" || got[1].Title != "Dry soil" {
		t.Errorf("below as well as above, most severe first: %+v", got)
	}
}

func TestLayers(t *testing.T) {
	t.Parallel()
	defaults := seededDefaults(t)
	name := "Tesfaye G."
	typeRule := layered(ScopeType, "uv", 5, true)
	typeRule.UpdatedBy = &name
	rules := Effective(defaults, []Rule{typeRule}, nil)
	var uv []Rule
	for _, r := range rules {
		if r.ParameterID == "uv" {
			uv = append(uv, r)
		}
	}
	if len(uv) != 1 || uv[0].Value != 5 || uv[0].Source != ScopeType || *uv[0].UpdatedBy != name {
		t.Errorf("a type's levels over the defaults: %+v", uv)
	}
	if got := AlertsFor(map[string]any{"uv": 6.0}, rules, lookup); len(got) != 1 || got[0].Message != "UV index 6 is at or above the classroom's warning level of 5." {
		t.Errorf("the type's: %+v", got)
	}
	rules = Effective(defaults, []Rule{typeRule}, []Rule{layered(ScopeDevice, "uv", 7, true)})
	if got := AlertsFor(map[string]any{"uv": 6.0}, rules, lookup); len(got) != 0 {
		t.Errorf("the device's over the type's: %+v", got)
	}
	if got := AlertsFor(map[string]any{"uv": 7.5}, rules, lookup); len(got) != 1 || got[0].Message != "UV index 7.5 is at or above this device's warning level of 7." {
		t.Errorf("the device's: %+v", got)
	}
	rules = Effective(defaults, nil, []Rule{layered(ScopeDevice, "uv", 5, false)})
	if got := AlertsFor(map[string]any{"uv": 11.0}, rules, lookup); len(got) != 0 {
		t.Errorf("turned off: %+v", got)
	}
}
