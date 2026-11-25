package registry

import (
	"encoding/json"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/dagrouper-school/sam/client"
)

func ptr[T any](v T) *T { return &v }

var published = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

// sam is a small registry as SAM would give it.
func sam() *SAM {
	return &SAM{
		Domains: []client.Domain{
			{ID: "AIR", Name: "Air", Description: "SAM's air"},
			{ID: "SOIL", Name: "Soil", Description: "SAM's soil"},
			{ID: "NOISE", Name: "Noise"},
		},
		Parameters: []client.Parameter{
			{ID: "air_temperature", Name: "Air temperature", Definition: "Dry-bulb air temperature.", Unit: "degC",
				ValueType: "number", DomainID: "AIR", Min: ptr(-60.0), Max: ptr(60.0), Version: 2, UpdatedAt: published},
			{ID: "relative_humidity", Name: "Relative humidity", Unit: "%", ValueType: "number", DomainID: "AIR", Version: 1, UpdatedAt: published},
			{ID: "dew_point", Name: "Dew point", Unit: "degC", ValueType: "number", DomainID: "AIR", Version: 1, UpdatedAt: published},
			{ID: "soil_moisture", Name: "Soil moisture", Unit: "%", ValueType: "number", DomainID: "SOIL", Version: 1, UpdatedAt: published},
			{ID: "sound_level", Name: "Sound level", Unit: "dB", ValueType: "number", DomainID: "NOISE", Version: 1, UpdatedAt: published},
			{ID: "spawning", Name: "Spawning", ValueType: "category", DomainID: "AIR", CategoryValues: []string{"yes", "no"}, Version: 1, UpdatedAt: published},
			{ID: "rain_daily", Name: "Rain, daily", Unit: "mm", ValueType: "number", DomainID: "AIR", Version: 1, UpdatedAt: published},
			{ID: "dgs:soil_ec", Name: "Soil EC", Unit: "uS/cm", ValueType: "number", DomainID: "SOIL", Version: 1, UpdatedAt: published},
		},
		DeviceTypes: []client.DeviceType{
			{ID: "station", Name: "Station", Keys: []client.DeviceKey{
				{Key: "t", ParameterID: ptr("air_temperature")}, {Key: "rh", ParameterID: ptr("relative_humidity")},
				{Key: "spare"}, {Key: "noise", ParameterID: ptr("sound_level")},
			}},
			{ID: "probe", Name: "Probe", Description: "SAM's probe", Keys: []client.DeviceKey{{Key: "m", ParameterID: ptr("soil_moisture")}}},
			{ID: "gauge", Name: "Gauge", Manual: true},
		},
		Templates: []client.Template{
			{ID: "gauge_reading", Name: "Gauge reading", Fields: []client.TemplateField{
				{Name: "rain", ParameterID: ptr("rain_daily"), Required: true}, {Name: "where"},
			}},
			{ID: "spot_check", Name: "Spot check", Fields: []client.TemplateField{
				{Name: "t", ParameterID: ptr("air_temperature"), Required: true}, {Name: "m", ParameterID: ptr("soil_moisture")},
			}},
		},
		Units: []client.Unit{
			{Symbol: "K", Dimension: "temperature", Scale: 1},
			{Symbol: "degC", Dimension: "temperature", Scale: 1, Offset: 273.15},
			{Symbol: "degF", Dimension: "temperature", Scale: 5.0 / 9, Offset: 459.67 * 5 / 9},
			{Symbol: "%", Dimension: "percentage", Scale: 1},
			{Symbol: "mm", Dimension: "length", Scale: 0.001},
			{Symbol: "in", Dimension: "length", Scale: 0.0254},
		},
		Derivations: []client.Derivation{{ID: "dew_point", ParameterID: "dew_point", Function: "dew_point", FunctionVersion: 1,
			Inputs: []string{"air_temperature", "relative_humidity"}, Definition: client.DerivationFunction{Formula: "Magnus"}, UpdatedAt: published}},
	}
}

func overlay() Overlay {
	return Overlay{
		Domains: []DomainOverlay{
			{ID: "SOIL", Label: "Ground", Icon: "soil", Description: "Moisture."},
			{ID: "NOISE", Label: "Noise", Icon: "tag", Hidden: true},
		},
		Parameters: []ParameterOverlay{
			{ID: "soil_moisture", Label: "Soil water", Plain: "How wet the soil is.", Source: SourceDevice, Noise: ptr(1.0), Chart: ptr("line")},
			{ID: "air_temperature", Label: "Temperature", Unit: ptr("°C"), Source: SourceWeatherStation, Chart: ptr("line_band"),
				Explanation: ptr("A shaded thermometer.")},
			{ID: "spawning", Label: "Spawning seen", Source: SourceManual, Chart: ptr("line")},
			{ID: "relative_humidity", Label: "Humidity", Domain: ptr("SOIL"), Source: SourceWeatherStation},
		},
		DeviceTypes: []DeviceTypeOverlay{
			{ID: "gauge", Label: "Rain gauge", Description: "Read daily.", Color: "#00bdff", Icon: "gauge",
				Tiles: []string{"rain_daily"}, Cards: []string{}},
			{ID: "station", Label: "Weather station", Description: "A station.", Color: "#3cc04b", Icon: "station",
				Tiles: []string{"relative_humidity", "soil_moisture", "air_temperature"}, Cards: []string{"air_temperature"}},
		},
		Templates: []TemplateOverlay{
			{ID: "gauge_reading", Label: "Rain gauge reading", Description: "Once a day.", Domain: ptr("AIR"), DeviceType: ptr("gauge")},
			{ID: "spot_check", Label: "Spot check", Description: "Copy the display.", DeviceType: ptr("nowhere")},
		},
	}
}

func ids[T any](list []T, id func(T) string) []string {
	out := make([]string, len(list))
	for i, x := range list {
		out[i] = id(x)
	}
	return out
}

// TestMergeDomains: the overlay's order first, then SAM's; a hidden domain
// is not shown; one without a row is shown as SAM has it.
func TestMergeDomains(t *testing.T) {
	t.Parallel()
	v := Merge(sam(), overlay())
	want := []Domain{
		{ID: "SOIL", Label: "Ground", Icon: "soil", Description: "Moisture."},
		{ID: "AIR", Label: "Air", Icon: fallbackDomainIcon, Description: "SAM's air"},
	}
	if !slices.Equal(v.Domains, want) {
		t.Fatalf("domains: %+v", v.Domains)
	}
}

// TestMergeParameters: the overlay's text and display over SAM's, in the
// overlay's order; SAM's own where the overlay has none.
func TestMergeParameters(t *testing.T) {
	t.Parallel()
	v := Merge(sam(), overlay())
	got := ids(v.Parameters, func(p Parameter) string { return p.ID })
	want := []string{"soil_moisture", "air_temperature", "spawning", "relative_humidity", "dew_point", "sound_level", "rain_daily", "dgs:soil_ec"}
	if !slices.Equal(got, want) {
		t.Fatalf("order: %q", got)
	}
	temp := v.Parameter("air_temperature")
	if temp.Label != "Temperature" || temp.Unit != "°C" || temp.Definition != "Dry-bulb air temperature." || *temp.Domain != "AIR" ||
		temp.Source != SourceWeatherStation || temp.Chart != "line_band" || *temp.Min != -60 || temp.ValueType != "number" {
		t.Errorf("air temperature: %+v", temp)
	}
	if c := temp.Calculation; c.Method != "measured" || c.Formula != "A shaded thermometer." || len(c.Inputs) != 0 ||
		c.Version != "SAM air_temperature v2" || c.PublishedAt != "2026-11-01" {
		t.Errorf("air temperature's calculation: %+v", c)
	}
	if rh := v.Parameter("relative_humidity"); *rh.Domain != "SOIL" || rh.Label != "Humidity" {
		t.Errorf("a value moved to another domain: %+v", rh)
	}
	if sound := v.Parameter("sound_level"); sound.Domain != nil || sound.Source != SourceDevice || sound.Label != "Sound level" {
		t.Errorf("a value under a hidden domain, sent by a device: %+v", sound)
	}
	if rain := v.Parameter("rain_daily"); rain.Source != SourceManual || rain.Calculation.Method != "manual" ||
		rain.Calculation.Formula != sourceWording[SourceManual] {
		t.Errorf("a value no device sends: %+v", rain)
	}
	if s := v.Parameter("spawning"); s.ValueType != "text" || s.Chart != "" || !slices.Equal(s.Options, []string{"yes", "no"}) || s.AltUnits != nil {
		t.Errorf("a category: %+v", s)
	}
	if ec := v.Parameter("dgs:soil_ec"); ec.Unit != "µS/cm" || *ec.Domain != "SOIL" {
		t.Errorf("this tenant's own value: %+v", ec)
	}
	if d := v.Parameter("dew_point").Calculation; d.Method != "derived" || d.Formula != "Magnus" ||
		!slices.Equal(d.Inputs, []string{"air_temperature", "relative_humidity"}) || d.Version != "SAM dew_point v1" {
		t.Errorf("a derived value: %+v", d)
	}
}

// TestAltUnits: the catalogue's other units of the dimension, converted to
// the parameter's: value × factor + offset.
func TestAltUnits(t *testing.T) {
	t.Parallel()
	v := Merge(sam(), Overlay{})
	alt := v.Parameter("air_temperature").AltUnits
	if len(alt) != 2 || alt[0].Unit != "K" || alt[1].Unit != "°F" {
		t.Fatalf("temperature: %+v", alt)
	}
	for _, c := range []struct {
		alt      AltUnit
		in, want float64
	}{
		{alt[0], 273.15, 0},
		{alt[1], 212, 100},
		{alt[1], -40, -40},
		{v.Parameter("rain_daily").AltUnits[0], 1, 25.4},
	} {
		if got := c.in*c.alt.Factor + c.alt.Offset; math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%v %s is %v, want %v", c.in, c.alt.Unit, got, c.want)
		}
	}
	if alt := v.Parameter("soil_moisture").AltUnits; alt != nil {
		t.Errorf("a unit alone in its dimension: %+v", alt)
	}
}

// TestMergeDeviceTypes: the values a type maps, in its keys' order; for a
// type with no sensor, the values of its templates; tiles and cards among
// them, in the overlay's order.
func TestMergeDeviceTypes(t *testing.T) {
	t.Parallel()
	v := Merge(sam(), overlay())
	want := []DeviceType{
		{ID: "gauge", Label: "Rain gauge", Description: "Read daily.", Color: "#00bdff", Icon: "gauge",
			ParameterIDs: []string{"rain_daily"}, TileParameterIDs: []string{"rain_daily"}, CardParameterIDs: []string{}, Manual: true},
		{ID: "station", Label: "Weather station", Description: "A station.", Color: "#3cc04b", Icon: "station",
			ParameterIDs:     []string{"air_temperature", "relative_humidity", "sound_level"},
			TileParameterIDs: []string{"relative_humidity", "air_temperature"}, CardParameterIDs: []string{"air_temperature"}},
		{ID: "probe", Label: "Probe", Description: "SAM's probe", Color: fallbackTypeColor, Icon: fallbackTypeIcon,
			ParameterIDs: []string{"soil_moisture"}, TileParameterIDs: []string{}, CardParameterIDs: []string{}},
	}
	if len(v.DeviceTypes) != len(want) {
		t.Fatalf("device types: %+v", v.DeviceTypes)
	}
	for i, w := range want {
		g := v.DeviceTypes[i]
		if g.ID != w.ID || g.Label != w.Label || g.Description != w.Description || g.Color != w.Color || g.Icon != w.Icon ||
			g.Manual != w.Manual || !slices.Equal(g.ParameterIDs, w.ParameterIDs) ||
			!slices.Equal(g.TileParameterIDs, w.TileParameterIDs) || !slices.Equal(g.CardParameterIDs, w.CardParameterIDs) {
			t.Errorf("device type %d: %+v, want %+v", i, g, w)
		}
	}
}

// TestMergeTemplates: SAM's templates as the overlay presents them, their
// fields without a value left out, then a plain one for each device type
// no template is read off.
func TestMergeTemplates(t *testing.T) {
	t.Parallel()
	v := Merge(sam(), overlay())
	got := ids(v.Templates, func(t Template) string { return t.ID })
	if !slices.Equal(got, []string{"gauge_reading", "spot_check", "device:station", "device:probe"}) {
		t.Fatalf("templates: %q", got)
	}
	gauge := v.Templates[0]
	if gauge.Label != "Rain gauge reading" || *gauge.Domain != "AIR" || *gauge.DeviceType != "gauge" ||
		!slices.Equal(gauge.Fields, []TemplateField{{ParameterID: "rain_daily", Required: true}}) {
		t.Errorf("a template read off a device type: %+v", gauge)
	}
	if spot := v.Templates[1]; spot.DeviceType != nil || spot.Domain != nil || len(spot.Fields) != 2 {
		t.Errorf("a template naming no device type there is, over two domains: %+v", spot)
	}
	probe := v.Templates[3]
	if probe.Label != "Probe reading" || probe.Description != "SAM's probe" || *probe.Domain != "SOIL" || *probe.DeviceType != "probe" ||
		!slices.Equal(probe.Fields, []TemplateField{{ParameterID: "soil_moisture", Required: true}}) {
		t.Errorf("a device type's plain template: %+v", probe)
	}
	if station := v.Templates[2]; station.Domain != nil || len(station.Fields) != 3 || !station.Fields[0].Required || station.Fields[1].Required {
		t.Errorf("a plain template over two domains: %+v", station)
	}
}

// TestMergeEmpty: SAM's registry with no overlay is shown as SAM has it.
func TestMergeEmpty(t *testing.T) {
	t.Parallel()
	v := Merge(sam(), Overlay{})
	if len(v.Domains) != 3 || len(v.Parameters) != 8 || len(v.DeviceTypes) != 3 || v.Parameter("air_temperature").Label != "Air temperature" {
		t.Fatalf("%+v", v)
	}
	if gauge := v.DeviceTypes[2]; len(gauge.ParameterIDs) != 0 {
		t.Errorf("a type with no sensor and no template: %+v", gauge)
	}
}

// TestMergeNothing: an empty registry is empty lists, never null.
func TestMergeNothing(t *testing.T) {
	t.Parallel()
	v := Merge(&SAM{Domains: []client.Domain{{ID: "AIR", Name: "Air"}}}, Overlay{Domains: []DomainOverlay{{ID: "AIR", Hidden: true}}})
	for name, list := range map[string]any{"parameters": v.Parameters, "domains": v.Domains, "device types": v.DeviceTypes, "templates": v.Templates} {
		if raw, _ := json.Marshal(list); string(raw) != "[]" {
			t.Errorf("%s: %s", name, raw)
		}
	}
}
