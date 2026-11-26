package registry

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dagrouper-school/sam/client"
)

// view is the test registry with an own domain and value pending.
func view() *View {
	s := sam()
	own := []Own{
		{Kind: OwnDomain, ID: "dgs:MARINE", Domain: &client.DomainInput{Name: "Marine"}, UpdatedAt: published},
		{Kind: OwnParameter, ID: "dgs:salinity", Parameter: &client.ParameterInput{Name: "Salinity", Unit: "1", ValueType: "number", DomainID: "dgs:MARINE"}, UpdatedAt: published},
		{Kind: OwnDomain, ID: "dgs:GONE"},
	}
	v := Merge(withOwn(s, own), overlay())
	v.owns = own
	return v
}

// TestWithOwn: this tenant's entries show as this service defines them
// until SAM has them; a removed one is gone.
func TestWithOwn(t *testing.T) {
	t.Parallel()
	s := sam()
	s.Domains = append(s.Domains, client.Domain{ID: "dgs:GONE", Name: "Gone"})
	at := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	got := withOwn(s, []Own{
		{Kind: OwnParameter, ID: "air_temperature", Parameter: &client.ParameterInput{Name: "Renamed", Unit: "degC", ValueType: "number", DomainID: "AIR"}, UpdatedAt: at},
		{Kind: OwnParameter, ID: "dgs:new", Parameter: &client.ParameterInput{Name: "New", ValueType: "string", DomainID: "AIR"}, UpdatedAt: at},
		{Kind: OwnDomain, ID: "dgs:GONE"},
	})
	if len(s.Parameters) != 8 || s.Parameters[0].Name != "Air temperature" {
		t.Fatal("SAM's copy was changed")
	}
	if p := got.Parameters[0]; p.Name != "Renamed" || p.Version != 2 || !p.UpdatedAt.Equal(published) {
		t.Errorf("an entry SAM has: %+v", p)
	}
	if p := got.Parameters[len(got.Parameters)-1]; p.ID != "dgs:new" || !p.UpdatedAt.Equal(at) {
		t.Errorf("an entry SAM lacks: %+v", p)
	}
	if got.hasDomain("dgs:GONE") {
		t.Error("a removed domain shows")
	}
}

func TestCheckDeviceType(t *testing.T) {
	t.Parallel()
	got := checkDeviceType(DeviceTypeInput{Label: "ab", Icon: "rocket", Color: "red", Values: []ValueInput{
		{ID: "soil_ec", Label: "EC", ValueType: "number"},
		{ID: "soil_ec", Label: "x", ValueType: "colour"},
		{ID: "Soil EC", Label: "EC", ValueType: "text"},
	}})
	want := map[string]string{
		"label": "Enter a name for the device type.", "icon": "Pick an icon.", "color": "Pick a color.",
		"values.1.id": "Each value appears once.", "values.1.label": "Enter a label.", "values.1.valueType": "Pick number, text or time.",
		"values.2.id": "Use lower case letters, digits and underscores, e.g. soil_ph.",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: %q, want %q", k, got[k], w)
		}
	}
	if f := checkDeviceType(DeviceTypeInput{Label: "Probe"}); f["values"] == "" {
		t.Errorf("no values: %v", f)
	}
}

// TestPlan: a value typed as one of SAM's IDs, or as this tenant's own
// without its namespace, names that value again; any other is new to the
// registry, in this tenant's namespace; registered values keep their unit
// and kind.
func TestPlan(t *testing.T) {
	t.Parallel()
	v := view()
	p, f := v.plan(DeviceTypeInput{Values: []ValueInput{
		{ID: "air_temperature", Label: "Temperature", Unit: "°C", ValueType: "number", Tile: true},
		{ID: "salinity", Label: "Sea salt", Unit: "1", ValueType: "number", Card: true},
		{ID: "leaf_wetness", Label: "Leaf wetness", Unit: "%", ValueType: "number", Domain: ptr("dgs:MARINE")},
		{ID: "notes", Label: "Notes", ValueType: "text", Domain: ptr("AIR")},
	}})
	if len(f) != 0 {
		t.Fatalf("problems: %v", f)
	}
	if !slices.Equal(p.ids, []string{"air_temperature", "dgs:salinity", "dgs:leaf_wetness", "dgs:notes"}) ||
		!slices.Equal(p.added, []string{"dgs:leaf_wetness", "dgs:notes"}) || !slices.Equal(p.edited, []string{"dgs:salinity"}) ||
		!slices.Equal(p.tiles, []string{"air_temperature"}) || !slices.Equal(p.cards, []string{"dgs:salinity"}) {
		t.Fatalf("plan: %+v", p)
	}
	if k := p.keys[1]; k.Key != "salinity" || *k.ParameterID != "dgs:salinity" || k.Unit != "1" || k.ValueType != "number" {
		t.Errorf("a key: %+v", k)
	}
	if k := p.keys[0]; k.Unit != "degC" {
		t.Errorf("a unit as shown: %+v", k)
	}
	if k := p.keys[3]; k.Unit != "" || k.ValueType != "string" {
		t.Errorf("a text key: %+v", k)
	}
	if fl := p.fields[0]; !fl.Required || p.fields[1].Required || fl.Name != "air_temperature" {
		t.Errorf("fields: %+v", p.fields)
	}
	if o := p.put[0]; o.ID != "dgs:salinity" || o.Parameter.Name != "Sea salt" || o.Parameter.DomainID != "dgs:MARINE" {
		t.Errorf("an edited value: %+v", o.Parameter)
	}

	_, f = v.plan(DeviceTypeInput{Values: []ValueInput{
		{ID: "air_temperature", Label: "T", Unit: "°F", ValueType: "number"},
		{ID: "spawning", Label: "S", ValueType: "number"},
		{ID: "new_one", Label: "N", Unit: "", ValueType: "number"},
		{ID: "gone", Label: "G", Unit: "x", ValueType: "text", Domain: ptr("NOISE")},
	}})
	want := map[string]string{
		"values.0.unit":      "Temperature is registered in °C. Add a new value for another unit.",
		"values.1.valueType": "Spawning seen is registered as text. Add a new value for another kind.",
		"values.2.domain":    "Pick a domain: a new value needs one.",
		"values.2.unit":      "Enter the unit.",
		"values.3.domain":    msgNoDomain,
		"values.3.unit":      "Only numbers have a unit.",
	}
	if len(f) != len(want) {
		t.Fatalf("problems: %v", f)
	}
	for k, w := range want {
		if f[k] != w {
			t.Errorf("%s: %q, want %q", k, f[k], w)
		}
	}
}

func TestTypeID(t *testing.T) {
	t.Parallel()
	v := view()
	v.DeviceTypes = append(v.DeviceTypes, DeviceType{ID: "dgs:soil_probe_v2"})
	for label, want := range map[string]string{"Soil probe v2": "dgs:soil_probe_v2_2", "  Weather!  ": "dgs:weather", "2nd probe": "dgs:type_2nd_probe"} {
		if got := v.typeID(label); got != want {
			t.Errorf("%q: %s, want %s", label, got, want)
		}
	}
}

// TestCheckExplanation: the front end's checks, and SAM's: a value SAM
// derives is derived from SAM's inputs; any other is measured or entered.
func TestCheckExplanation(t *testing.T) {
	t.Parallel()
	v := view()
	dew, temp := v.Parameter("dew_point"), v.Parameter("air_temperature")
	for _, c := range []struct {
		name string
		p    *Parameter
		in   Explanation
		want map[string]string
	}{
		{"SAM's derivation, worded", dew, Explanation{Method: "derived", Formula: "Magnus.", Inputs: []string{"relative_humidity", "air_temperature"}}, map[string]string{}},
		{"measured", temp, Explanation{Plain: "Warmth.", Method: "measured", Formula: "A thermometer.", Inputs: []string{}}, map[string]string{}},
		{"derived from other inputs", dew, Explanation{Method: "derived", Formula: "x", Inputs: []string{"air_temperature"}},
			map[string]string{"inputs": "SAM calculates it from air_temperature, relative_humidity."}},
		{"a derived value as measured", dew, Explanation{Method: "measured", Formula: "x", Inputs: []string{}},
			map[string]string{"method": "SAM calculates this value from other values."}},
		{"a measured value as derived", temp, Explanation{Method: "derived", Formula: " ", Inputs: []string{"nowhere", "air_temperature"}},
			map[string]string{"method": "SAM does not calculate this value: it is measured or entered.", "formula": "Enter the formula.", "inputs": "Not in the registry: nowhere."}},
		{"its own input", temp, Explanation{Method: "measured", Formula: "x", Inputs: []string{"air_temperature"}},
			map[string]string{"inputs": "A value cannot be its own input."}},
		{"too long", temp, Explanation{Plain: strings.Repeat("é", PlainMax+1), Method: "manual", Formula: "x"},
			map[string]string{"plain": "Keep it to 240 characters: one plain sentence."}},
	} {
		got := v.checkExplanation(c.in, c.p)
		if len(got) != len(c.want) {
			t.Errorf("%s: %v", c.name, got)
			continue
		}
		for k, w := range c.want {
			if got[k] != w {
				t.Errorf("%s %s: %q, want %q", c.name, k, got[k], w)
			}
		}
	}
}

func TestCheckAlert(t *testing.T) {
	t.Parallel()
	v := view()
	if f := v.checkAlert(AlertRuleInput{ParameterID: "air_temperature", Direction: "above", Value: ptr(30.0), Level: "caution", Title: "Hot"}); len(f) != 0 {
		t.Fatalf("a good level: %v", f)
	}
	got := v.checkAlert(AlertRuleInput{ParameterID: "spawning", Direction: "over", Level: "danger", Title: " "})
	want := map[string]string{"parameterId": "Spawning seen is not a number, so it cannot have a level.", "direction": "Pick above or below.",
		"level": "Pick caution or warning.", "value": "The level must be a number.", "title": "Give the alert a short name."}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: %q, want %q", k, got[k], w)
		}
	}
	if f := v.checkAlert(AlertRuleInput{ParameterID: "air_temperature", Direction: "above", Value: ptr(61.0), Level: "warning", Title: "x"}); f["value"] != "Temperature should be -60 to 60°C." {
		t.Errorf("out of range: %v", f)
	}
}

func TestRuleText(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		r    AlertRule
		unit string
		want string
	}{
		{AlertRule{Level: "warning", Direction: "above", Value: 8, Title: "Very high UV"}, "", "Warning at or above 8 (Very high UV)"},
		{AlertRule{Level: "caution", Direction: "below", Value: 15.04, Title: "Dry soil"}, "%", "Caution at or below 15 % (Dry soil)"},
		{AlertRule{Level: "caution", Direction: "above", Value: 32.25, Title: "Hot"}, "°C", "Caution at or above 32.3°C (Hot)"},
		{AlertRule{Level: "warning", Direction: "above", Value: 1012.6, Title: "High"}, "hPa", "Warning at or above 1013 hPa (High)"},
	} {
		if got := ruleText(c.r, c.unit); got != c.want {
			t.Errorf("%q, want %q", got, c.want)
		}
	}
}

// TestNeeds: an entry waits for this tenant's own entries it names, never
// for SAM's.
func TestNeeds(t *testing.T) {
	t.Parallel()
	typ := Own{Kind: OwnDeviceType, Type: &client.DeviceTypeInput{Keys: []client.DeviceKey{
		{Key: "a", ParameterID: ptr("dgs:a")}, {Key: "b", ParameterID: ptr("air_temperature")}, {Key: "c"},
	}}}
	if got := typ.needs(); !slices.Equal(got, []string{"registry:parameter:dgs:a"}) {
		t.Errorf("a device type: %q", got)
	}
	param := Own{Kind: OwnParameter, Parameter: &client.ParameterInput{DomainID: "dgs:MARINE"}}
	if got := param.needs(); !slices.Equal(got, []string{"registry:domain:dgs:MARINE"}) {
		t.Errorf("a value: %q", got)
	}
	if got := (Own{Kind: OwnParameter, Parameter: &client.ParameterInput{DomainID: "AIR"}}).needs(); len(got) != 0 {
		t.Errorf("a value under one of SAM's domains: %q", got)
	}
}
