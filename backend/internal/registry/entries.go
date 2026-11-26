package registry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/dagrouper-school/sam/client"
)

// Entry is a manual observation checked against its template, ready for
// SAM: through one of SAM's templates, its values under the template's
// field names in each field's unit; or through a device type's plain
// template, which SAM does not hold, as a reading of the device.
type Entry struct {
	Template Template
	// Plain is true for a device type's plain template.
	Plain bool
	// Fields are the values by SAM's field names; nil for a plain
	// template, which is sent under the device type's keys instead.
	Fields map[string]any
	// Parameters are the template's fields' parameters, by SAM's field
	// name, to name SAM's problems by parameter.
	Parameters map[string]string
}

// ErrNoTemplate means the registry has no such template.
var ErrNoTemplate = errors.New("no such template")

// CheckEntry checks values, given by parameter in each parameter's own
// unit, against a template, as the front end's checkObservation does:
// required fields, numbers, plausible ranges and listed answers, and
// nothing outside the template. Problems are by parameter.
func (m *Module) CheckEntry(ctx context.Context, templateID string, values map[string]any) (Entry, map[string]string, error) {
	v, err := m.View(ctx)
	if err != nil {
		return Entry{}, nil, err
	}
	i := slices.IndexFunc(v.Templates, func(t Template) bool { return t.ID == templateID })
	if i < 0 {
		return Entry{}, nil, ErrNoTemplate
	}
	e := Entry{Template: v.Templates[i], Plain: strings.HasPrefix(templateID, DevicePrefix)}
	problems := v.checkEntry(e.Template, values)
	if len(problems) > 0 || e.Plain {
		return e, problems, nil
	}
	e.Fields, e.Parameters, problems = v.sam.fieldValues(templateID, values)
	return e, problems, nil
}

func (v *View) checkEntry(t Template, values map[string]any) map[string]string {
	problems := map[string]string{}
	for _, f := range t.Fields {
		p := v.byID[f.ParameterID]
		if p == nil {
			problems[f.ParameterID] = "This value is no longer in the registry."
			continue
		}
		if msg := checkValue(p, f, values[f.ParameterID]); msg != "" {
			problems[f.ParameterID] = msg
		}
	}
	for id := range values {
		if !slices.ContainsFunc(t.Fields, func(f TemplateField) bool { return f.ParameterID == id }) {
			problems[id] = fmt.Sprintf("%s does not record %s.", t.Label, id)
		}
	}
	if len(problems) == 0 && len(values) == 0 {
		problems["values"] = "Fill in at least one value."
	}
	return problems
}

func checkValue(p *Parameter, f TemplateField, value any) string {
	s, isText := value.(string)
	if value == nil || isText && strings.TrimSpace(s) == "" {
		if f.Required {
			return p.Label + " is required."
		}
		return ""
	}
	if p.ValueType == valueNumber || p.ValueType == "" {
		n, ok := value.(float64)
		if !ok || math.IsInf(n, 0) || math.IsNaN(n) {
			return p.Label + " must be a number."
		}
		if p.Min != nil && n < *p.Min || p.Max != nil && n > *p.Max {
			return fmt.Sprintf("%s should be %s.", p.Label, rangeText(p))
		}
		return ""
	}
	if !isText {
		return p.Label + " must be text."
	}
	if len(p.Options) > 0 && !slices.Contains(p.Options, s) {
		return "Pick one of: " + strings.Join(p.Options, ", ") + "."
	}
	return ""
}

// fieldValues are values by parameter under SAM's template field names, in
// each field's unit, and the fields' parameters by name.
func (s *SAM) fieldValues(templateID string, values map[string]any) (map[string]any, map[string]string, map[string]string) {
	problems := map[string]string{}
	i := slices.IndexFunc(s.Templates, func(t client.Template) bool { return t.ID == templateID })
	if i < 0 {
		problems["values"] = "This observation form is not in the registry."
		return nil, nil, problems
	}
	units := map[string]client.Unit{}
	for _, u := range s.Units {
		units[u.Symbol] = u
	}
	out, params := map[string]any{}, map[string]string{}
	for _, f := range s.Templates[i].Fields {
		if f.ParameterID == nil {
			continue
		}
		params[f.Name] = *f.ParameterID
		v, ok := values[*f.ParameterID]
		if !ok {
			continue
		}
		converted, ok := convert(units, v, s.unitOf(*f.ParameterID), f.Unit)
		if !ok {
			problems[*f.ParameterID] = fmt.Sprintf("%s cannot be sent in %s.", s.nameOf(*f.ParameterID), f.Unit)
			continue
		}
		out[f.Name] = converted
	}
	return out, params, problems
}

// convert is v, a number in from, in to; anything else as it is.
func convert(units map[string]client.Unit, v any, from, to string) (any, bool) {
	n, isNumber := v.(float64)
	if !isNumber || from == to || from == "" || to == "" {
		return v, true
	}
	f, okF := units[from]
	k, okK := units[to]
	if !okF || !okK || f.Dimension != k.Dimension || k.Scale == 0 {
		return nil, false
	}
	return (n*f.Scale + f.Offset - k.Offset) / k.Scale, true
}
