package registry

import (
	"math"
	"testing"

	"github.com/dagrouper-school/sam/client"
)

func TestKeyValues(t *testing.T) {
	t.Parallel()
	rain, temp, wind := "precipitation_amount", "air_temperature", "wind_direction_cardinal"
	s := &SAM{
		Parameters: []client.Parameter{{ID: "soil_moisture", Name: "Soil moisture"}, {ID: rain, Unit: "mm"}, {ID: temp, Unit: "degC"}, {ID: wind}},
		Units: []client.Unit{{Symbol: "mm", Dimension: "length", Scale: 0.001}, {Symbol: "in", Dimension: "length", Scale: 0.0254},
			{Symbol: "degC", Dimension: "temperature", Scale: 1, Offset: 273.15}, {Symbol: "degF", Dimension: "temperature", Scale: 5.0 / 9, Offset: 255.372222}},
		DeviceTypes: []client.DeviceType{{ID: "gauge", Name: "rain gauge", Keys: []client.DeviceKey{
			{Key: "rain_in", Unit: "in", ParameterID: &rain}, {Key: "temp_f", Unit: "degF", ParameterID: &temp}, {Key: "dir", ParameterID: &wind}}}},
	}
	out, problems := s.keyValues("gauge", map[string]any{rain: 25.4, temp: 100.0, wind: "NE"})
	if len(problems) != 0 || math.Abs(out["rain_in"].(float64)-1) > 1e-9 || math.Abs(out["temp_f"].(float64)-212) > 1e-3 || out["dir"] != "NE" {
		t.Errorf("%v %v", out, problems)
	}
	_, problems = s.keyValues("gauge", map[string]any{"soil_moisture": 3.0})
	if problems["values.soil_moisture"] != "A rain gauge does not measure Soil moisture." {
		t.Errorf("%v", problems)
	}
	if _, problems := s.keyValues("toaster", nil); problems["values"] == "" {
		t.Errorf("an unknown type: %v", problems)
	}
}
