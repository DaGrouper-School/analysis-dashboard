package registry

import (
	"math"
	"testing"
)

func TestCheckValue(t *testing.T) {
	t.Parallel()
	lo, hi := 0.0, 500.0
	rain := &Parameter{Label: "Rain", ValueType: valueNumber, Unit: "mm", Min: &lo, Max: &hi}
	kind := &Parameter{Label: "Sky", ValueType: valueText, Options: []string{"clear", "cloudy"}}
	req, opt := TemplateField{Required: true}, TemplateField{}
	for _, c := range []struct {
		p     *Parameter
		f     TemplateField
		value any
		want  string
	}{
		{rain, req, 12.5, ""},
		{rain, req, nil, "Rain is required."},
		{rain, opt, nil, ""},
		{rain, opt, " ", ""},
		{rain, req, "12", "Rain must be a number."},
		{rain, req, math.Inf(1), "Rain must be a number."},
		{rain, req, 600.0, "Rain should be " + rangeText(rain) + "."},
		{kind, req, "clear", ""},
		{kind, req, 3.0, "Sky must be text."},
		{kind, req, "foggy", "Pick one of: clear, cloudy."},
	} {
		if got := checkValue(c.p, c.f, c.value); got != c.want {
			t.Errorf("%s %v: %q", c.p.Label, c.value, got)
		}
	}
}

func TestCheckEntry(t *testing.T) {
	t.Parallel()
	lo := 0.0
	v := &View{byID: map[string]*Parameter{"rain": {ID: "rain", Label: "Rain", ValueType: valueNumber, Min: &lo}}}
	tpl := Template{Label: "Rain gauge reading", Fields: []TemplateField{{ParameterID: "rain", Required: true}, {ParameterID: "gone"}}}
	p := v.checkEntry(tpl, map[string]any{"rain": 3.0, "wind": 2.0})
	if p["gone"] != "This value is no longer in the registry." || p["wind"] != "Rain gauge reading does not record wind." || p["rain"] != "" {
		t.Errorf("%v", p)
	}
	if p := v.checkEntry(Template{}, map[string]any{}); p["values"] != "Fill in at least one value." {
		t.Errorf("empty: %v", p)
	}
}
