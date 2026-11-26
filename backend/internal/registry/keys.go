package registry

import (
	"context"
	"fmt"

	"github.com/dagrouper-school/sam/client"
)

// KeyValues are a reading's values, given by parameter in each parameter's
// own unit as the front end has them, under the device type's keys in each
// key's unit, as a device sends them to SAM. Text and category values pass
// as they are. A parameter the type has no key for, or a unit that does not
// convert, is a problem, by values.<parameter>.
func (m *Module) KeyValues(ctx context.Context, typeID string, values map[string]any) (map[string]any, map[string]string, error) {
	s, err := m.cache.Get(ctx)
	if err != nil {
		return nil, nil, err
	}
	out, problems := s.keyValues(typeID, values)
	return out, problems, nil
}

func (s *SAM) keyValues(typeID string, values map[string]any) (map[string]any, map[string]string) {
	problems := map[string]string{}
	var t *client.DeviceType
	for i := range s.DeviceTypes {
		if s.DeviceTypes[i].ID == typeID {
			t = &s.DeviceTypes[i]
		}
	}
	if t == nil {
		problems["values"] = "This device's type is not in the registry."
		return nil, problems
	}
	units := map[string]client.Unit{}
	for _, u := range s.Units {
		units[u.Symbol] = u
	}
	out := map[string]any{}
	for param, v := range values {
		key := keyFor(t, param)
		if key == nil {
			problems["values."+param] = fmt.Sprintf("A %s does not measure %s.", t.Name, s.nameOf(param))
			continue
		}
		n, isNumber := v.(float64)
		from, to := s.unitOf(param), key.Unit
		if !isNumber || from == to || from == "" || to == "" {
			out[key.Key] = v
			continue
		}
		f, okF := units[from]
		k, okK := units[to]
		if !okF || !okK || f.Dimension != k.Dimension || k.Scale == 0 {
			problems["values."+param] = fmt.Sprintf("%s cannot be sent in %s.", s.nameOf(param), to)
			continue
		}
		out[key.Key] = (n*f.Scale + f.Offset - k.Offset) / k.Scale
	}
	return out, problems
}

// keyFor is the first of t's keys that maps param, or nil.
func keyFor(t *client.DeviceType, param string) *client.DeviceKey {
	for i := range t.Keys {
		if p := t.Keys[i].ParameterID; p != nil && *p == param {
			return &t.Keys[i]
		}
	}
	return nil
}

// nameOf is the name of the parameter with id, or id when SAM has none.
func (s *SAM) nameOf(id string) string {
	for _, p := range s.Parameters {
		if p.ID == id && p.Name != "" {
			return p.Name
		}
	}
	return id
}
