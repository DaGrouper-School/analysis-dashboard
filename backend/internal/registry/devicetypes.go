package registry

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

// DeviceTypeInput is the front end's DeviceTypeInput.
type DeviceTypeInput struct {
	Label       string       `json:"label"`
	Description string       `json:"description"`
	Color       string       `json:"color"`
	Icon        string       `json:"icon"`
	Manual      bool         `json:"manual"`
	Values      []ValueInput `json:"values"`
}

// ValueInput is the front end's ExpectedValueInput: a value a device type
// sends, under the key its devices send it as.
type ValueInput struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Unit      string  `json:"unit"`
	ValueType string  `json:"valueType"`
	Domain    *string `json:"domain"`
	Tile      bool    `json:"tile"`
	Card      bool    `json:"card"`
}

// valueKey is a value's ID as an admin types it, as the front end checks
// it; it is the key devices send, and the parameter's ID in this tenant's
// namespace when the value is new.
var valueKey = regexp.MustCompile(`^[a-z][a-z0-9_]{1,47}$`)

// samTypes are SAM's value types for the front end's.
var samTypes = map[string]string{valueNumber: "number", valueText: "string", valueTime: "datetime"}

func checkDeviceType(in DeviceTypeInput) map[string]string {
	f := map[string]string{}
	if utf8.RuneCountInString(strings.TrimSpace(in.Label)) < 3 {
		f["label"] = "Enter a name for the device type."
	}
	if icon := strings.TrimSpace(in.Icon); icon != "" && !slices.Contains(registryIcons, icon) {
		f["icon"] = "Pick an icon."
	}
	if in.Color != "" && !hexColor.MatchString(in.Color) {
		f["color"] = "Pick a color."
	}
	if len(in.Values) == 0 {
		f["values"] = "Add at least one value the device sends."
	}
	var seen []string
	for i, v := range in.Values {
		at := "values." + strconv.Itoa(i) + "."
		id := strings.TrimSpace(v.ID)
		switch {
		case !valueKey.MatchString(id):
			f[at+"id"] = "Use lower case letters, digits and underscores, e.g. soil_ph."
		case slices.Contains(seen, id):
			f[at+"id"] = "Each value appears once."
		}
		seen = append(seen, id)
		if utf8.RuneCountInString(strings.TrimSpace(v.Label)) < 2 {
			f[at+"label"] = "Enter a label."
		}
		if samTypes[v.ValueType] == "" {
			f[at+"valueType"] = "Pick number, text or time."
		}
	}
	return f
}

// samUnit is a unit as SAM writes it: °C is degC.
func samUnit(shown string) string {
	shown = strings.TrimSpace(shown)
	for symbol, label := range unitLabels {
		if label != "" && label == shown {
			return symbol
		}
	}
	return shown
}

// typePlan is what a device type's values become: its keys or, for one
// read by hand, its template's fields, and the parameters added or edited
// on the way.
type typePlan struct {
	ids    []string
	keys   []client.DeviceKey
	fields []client.TemplateField
	put    []Own
	added  []string
	edited []string
	labels map[string]string
	tiles  []string
	cards  []string
}

// plan resolves each value: one of SAM's or this tenant's own already in
// the registry, under the ID typed or in this tenant's namespace, or a new
// one of this tenant's. A registered value keeps its unit and kind.
func (v *View) plan(in DeviceTypeInput) (typePlan, map[string]string) {
	p := typePlan{labels: map[string]string{}}
	f := map[string]string{}
	for i, val := range in.Values {
		at := "values." + strconv.Itoa(i) + "."
		key, label, unit := strings.TrimSpace(val.ID), strings.TrimSpace(val.Label), samUnit(val.Unit)
		if val.ValueType != valueNumber {
			unit = ""
		}
		id := key
		existing := v.Parameter(key)
		if existing == nil {
			id = Namespace + ":" + key
			existing = v.Parameter(id)
		}
		if existing != nil {
			v.checkRegistered(existing, val, unit, at, f)
			if o, ok := v.own(OwnParameter, id); ok && o.Parameter != nil && o.Parameter.Name != label {
				def := *o.Parameter
				def.Name = label
				p.put = append(p.put, Own{Kind: OwnParameter, ID: id, Parameter: &def})
				p.edited = append(p.edited, id)
				p.labels[id] = label
			}
		} else if def, ok := v.newParameter(val, label, unit, at, f); ok {
			p.put = append(p.put, Own{Kind: OwnParameter, ID: id, Parameter: def})
			p.added = append(p.added, id)
		}
		p.ids = append(p.ids, id)
		if val.Tile {
			p.tiles = append(p.tiles, id)
		}
		if val.Card {
			p.cards = append(p.cards, id)
		}
		samType := samTypes[val.ValueType]
		p.keys = append(p.keys, client.DeviceKey{Key: key, Name: label, Unit: unit, ValueType: samType, ParameterID: ptrTo(id)})
		field := client.TemplateField{Name: key, Label: label, Type: samType, Required: i == 0, ParameterID: ptrTo(id), Unit: unit}
		if existing != nil && len(existing.Options) > 0 {
			field.AllowedValues = existing.Options
		}
		p.fields = append(p.fields, field)
	}
	return p, f
}

// checkRegistered refuses a change to a registered value's unit or kind:
// SAM keeps readings in the unit registered.
func (v *View) checkRegistered(p *Parameter, val ValueInput, unit, at string, f map[string]string) {
	if p.ValueType != val.ValueType {
		f[at+"valueType"] = fmt.Sprintf("%s is registered as %s. Add a new value for another kind.", p.Label, p.ValueType)
	} else if registered := v.sam.unitOf(p.ID); p.ValueType == valueNumber && unit != registered {
		f[at+"unit"] = fmt.Sprintf("%s is registered in %s. Add a new value for another unit.", p.Label, orDefault(unitLabel(registered), "no unit"))
	}
}

// newParameter is the definition of a value new to the registry, which
// SAM files under one of its domains.
func (v *View) newParameter(val ValueInput, label, unit, at string, f map[string]string) (*client.ParameterInput, bool) {
	switch {
	case val.Domain == nil:
		f[at+"domain"] = "Pick a domain: a new value needs one."
	case v.domain(*val.Domain) == nil:
		f[at+"domain"] = msgNoDomain
	}
	if val.ValueType == valueNumber && unit == "" {
		f[at+"unit"] = "Enter the unit."
	} else if val.ValueType != valueNumber && strings.TrimSpace(val.Unit) != "" {
		f[at+"unit"] = "Only numbers have a unit."
	}
	if len(f) > 0 {
		return nil, false
	}
	return &client.ParameterInput{Name: label, Unit: unit, ValueType: samTypes[val.ValueType], DomainID: *val.Domain}, true
}

func (s *SAM) unitOf(id string) string {
	if i := slices.IndexFunc(s.Parameters, func(p client.Parameter) bool { return p.ID == id }); i >= 0 {
		return s.Parameters[i].Unit
	}
	return ""
}

func (v *View) deviceType(id string) *DeviceType {
	i := slices.IndexFunc(v.DeviceTypes, func(t DeviceType) bool { return t.ID == id })
	if i < 0 {
		return nil
	}
	return &v.DeviceTypes[i]
}

// labelFree refuses a label another device type has.
func (v *View) labelFree(label, except string) error {
	if slices.ContainsFunc(v.DeviceTypes, func(t DeviceType) bool { return t.ID != except && strings.EqualFold(t.Label, label) }) {
		msg := fmt.Sprintf("There is already a %s device type.", label)
		return httpx.FieldError(httpx.CodeConflict, "label", msg)
	}
	return nil
}

var notSlug = regexp.MustCompile(`[^a-z0-9]+`)

// typeID is a new device type's ID in this tenant's namespace, from its
// label: Soil probe v2 is dgs:soil_probe_v2, numbered if it is taken.
func (v *View) typeID(label string) string {
	base := strings.Trim(notSlug.ReplaceAllString(strings.ToLower(label), "_"), "_")
	if len(base) > 40 {
		base = strings.TrimRight(base[:40], "_")
	}
	if base == "" || base[0] < 'a' || base[0] > 'z' {
		base = "type_" + base
	}
	taken := func(id string) bool {
		_, own := v.own(OwnDeviceType, id)
		return own || v.deviceType(id) != nil
	}
	id := Namespace + ":" + base
	for n := 2; taken(id); n++ {
		id = Namespace + ":" + base + "_" + strconv.Itoa(n)
	}
	return id
}

func (m *Module) createDeviceType(w http.ResponseWriter, r *http.Request) error {
	var in DeviceTypeInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := httpx.CheckFields(checkDeviceType(in)); err != nil {
		return err
	}
	var id string
	return m.run(w, r, func(e *edit) error {
		label := strings.TrimSpace(in.Label)
		if err := e.view.labelFree(label, ""); err != nil {
			return err
		}
		p, f := e.view.plan(in)
		if err := httpx.CheckFields(f); err != nil {
			return err
		}
		id = e.view.typeID(label)
		row := DeviceTypeOverlay{ID: id, Label: label, Description: strings.TrimSpace(in.Description),
			Color: orDefault(in.Color, fallbackTypeColor), Icon: orDefault(strings.TrimSpace(in.Icon), fallbackTypeIcon)}
		if err := e.saveType(id, in.Manual, row, p); err != nil {
			return err
		}
		values := fmt.Sprintf("%d values", len(p.ids))
		if len(p.ids) == 1 {
			values = "1 value"
		}
		summary := fmt.Sprintf("Registered the %s device type with %s", label, values)
		if len(p.added) > 0 {
			summary += " (new: " + strings.Join(p.added, ", ") + ")"
		}
		return e.log(summary + ".")
	}, func(v *View) (any, error) { return v.deviceType(id), nil })
}

// saveType stores one of this tenant's device types, the values it adds or
// edits and, for one read by hand, its template, and sends them to SAM:
// SAM has no keys for a device with no sensor, so its values are the
// template's fields.
func (e *edit) saveType(id string, manual bool, row DeviceTypeOverlay, p typePlan) error {
	row.Tiles, row.Cards = nonNil(p.tiles), nonNil(p.cards)
	def := &client.DeviceTypeInput{Name: row.Label, Description: row.Description, Manual: manual, Keys: p.keys}
	if manual {
		def.Keys = []client.DeviceKey{}
	}
	for _, o := range append(p.put, Own{Kind: OwnDeviceType, ID: id, Type: def}) {
		if err := e.putOwn(o); err != nil {
			return err
		}
	}
	// A value renamed here shows its new name, whatever its overlay said.
	for _, vid := range p.edited {
		if err := e.relabel(vid, p.labels[vid]); err != nil {
			return err
		}
	}
	if manual {
		reading := row.Label + " reading"
		if err := e.putOwn(Own{Kind: OwnTemplate, ID: id, Template: &client.TemplateInput{Name: reading, Description: row.Description, Fields: p.fields}}); err != nil {
			return err
		}
		if err := e.putTemplate(TemplateOverlay{ID: id, Label: reading, Description: row.Description, DeviceType: ptrTo(id)}); err != nil {
			return err
		}
	}
	return e.putDeviceType(row)
}

func (m *Module) updateDeviceType(w http.ResponseWriter, r *http.Request) error {
	var in DeviceTypeInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := httpx.CheckFields(checkDeviceType(in)); err != nil {
		return err
	}
	id := r.PathValue("deviceTypeId")
	return m.run(w, r, func(e *edit) error {
		t := e.view.deviceType(id)
		if t == nil {
			return httpx.NotFound("That device type no longer exists.")
		}
		label := strings.TrimSpace(in.Label)
		if err := e.view.labelFree(label, id); err != nil {
			return err
		}
		p, f := e.view.plan(in)
		row := DeviceTypeOverlay{ID: id, Label: label, Description: strings.TrimSpace(in.Description),
			Color: orDefault(in.Color, t.Color), Icon: orDefault(strings.TrimSpace(in.Icon), t.Icon)}
		if !isOwn(id) {
			return e.presentCoreType(t, in, row, p)
		}
		return e.updateOwnType(t, in, row, p, f)
	}, func(v *View) (any, error) { return v.deviceType(id), nil })
}

// presentCoreType changes how one of SAM's device types is presented. SAM
// defines its values and whether it is read by hand.
func (e *edit) presentCoreType(t *DeviceType, in DeviceTypeInput, row DeviceTypeOverlay, p typePlan) error {
	if !sameSet(p.ids, t.ParameterIDs) || in.Manual != t.Manual {
		return httpx.New(httpx.CodeConflict, fmt.Sprintf(
			"SAM defines the values of the %s device type, and whether it is read by hand. Its label, description and presentation can change.", t.Label))
	}
	row.Tiles, row.Cards = nonNil(p.tiles), nonNil(p.cards)
	if err := e.putDeviceType(row); err != nil {
		return err
	}
	return e.log(fmt.Sprintf("Updated the %s device type.", row.Label))
}

// updateOwnType changes one of this tenant's device types, its values
// included; whether it is read by hand is fixed.
func (e *edit) updateOwnType(t *DeviceType, in DeviceTypeInput, row DeviceTypeOverlay, p typePlan, f map[string]string) error {
	if in.Manual != t.Manual {
		return httpx.New(httpx.CodeConflict, "Whether a device type is read by hand is fixed once it is registered.")
	}
	if err := httpx.CheckFields(f); err != nil {
		return err
	}
	if err := e.saveType(t.ID, in.Manual, row, p); err != nil {
		return err
	}
	var parts []string
	for _, part := range []struct {
		what string
		ids  []string
	}{{"added", p.added}, {"edited", p.edited}, {"dropped", without(t.ParameterIDs, p.ids)}} {
		if len(part.ids) > 0 {
			parts = append(parts, part.what+" "+strings.Join(part.ids, ", "))
		}
	}
	summary := fmt.Sprintf("Updated the %s device type", row.Label)
	if len(parts) > 0 {
		summary += ": " + strings.Join(parts, "; ")
	}
	return e.log(summary + ".")
}

// without is the IDs of a not in b.
func without(a, b []string) []string {
	var out []string
	for _, id := range a {
		if !slices.Contains(b, id) {
			out = append(out, id)
		}
	}
	return out
}
