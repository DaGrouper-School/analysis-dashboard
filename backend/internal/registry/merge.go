package registry

import (
	"fmt"
	"slices"
	"time"

	"github.com/dagrouper-school/sam/client"
)

// View is the registry the front end reads: SAM's entries as the overlay
// presents them, each list in display order.
type View struct {
	Parameters  []Parameter
	Domains     []Domain
	DeviceTypes []DeviceType
	// Templates are SAM's templates, then a plain one for each device type
	// that has none.
	Templates []Template
	byID      map[string]*Parameter
	// sam is SAM's registry it was merged from, and owns this tenant's own
	// entries in it.
	sam  *SAM
	owns []Own
}

// Parameter is the front end's Parameter.
type Parameter struct {
	ID          string       `json:"id"`
	Label       string       `json:"label"`
	Definition  string       `json:"definition"`
	Unit        string       `json:"unit"`
	Domain      *string      `json:"domain"`
	Source      string       `json:"source"`
	ValueType   string       `json:"valueType"`
	Min         *float64     `json:"min,omitempty"`
	Max         *float64     `json:"max,omitempty"`
	AltUnits    []AltUnit    `json:"altUnits,omitempty"`
	Options     []string     `json:"options,omitempty"`
	Plain       string       `json:"plain,omitempty"`
	Noise       *float64     `json:"noise,omitempty"`
	Calculation *Calculation `json:"calculation,omitempty"`
	Chart       string       `json:"chart,omitempty"`
}

// AltUnit is another unit a value may be measured in: value × Factor +
// Offset gives it in the parameter's own unit.
type AltUnit struct {
	Unit   string  `json:"unit"`
	Factor float64 `json:"factor"`
	Offset float64 `json:"offset,omitempty"`
}

// Calculation is how SAM gets a value.
type Calculation struct {
	Method      string   `json:"method"`
	Formula     string   `json:"formula"`
	Inputs      []string `json:"inputs"`
	Version     string   `json:"version"`
	PublishedAt string   `json:"publishedAt"`
}

// Domain is the front end's DomainDefinition.
type Domain struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
}

// DeviceType is the front end's DeviceTypeDefinition.
type DeviceType struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	Description      string   `json:"description"`
	Color            string   `json:"color"`
	Icon             string   `json:"icon"`
	ParameterIDs     []string `json:"parameterIds"`
	TileParameterIDs []string `json:"tileParameterIds"`
	CardParameterIDs []string `json:"cardParameterIds"`
	Manual           bool     `json:"manual,omitempty"`
}

// Template is the front end's ObservationTemplate.
type Template struct {
	ID          string          `json:"id"`
	Label       string          `json:"label"`
	Description string          `json:"description"`
	Domain      *string         `json:"domain"`
	DeviceType  *string         `json:"deviceType"`
	Fields      []TemplateField `json:"fields"`
}

// TemplateField is one field of a template.
type TemplateField struct {
	ParameterID string `json:"parameterId"`
	Required    bool   `json:"required"`
}

// Sources of a value, the front end's Parameter.source.
const (
	SourceWeatherStation = "weather_station"
	SourceDevice         = "device"
	SourceManual         = "manual"
)

// Fallbacks for an entry the overlay does not present: the front end's
// own icons and grey for anything unknown.
const (
	fallbackDomainIcon = "tag"
	fallbackTypeIcon   = "sensor"
	fallbackTypeColor  = "#9aa5ae"
)

// DevicePrefix starts the ID of the plain template listed for a device
// type that has none.
const DevicePrefix = "device:"

// sourceWording is how a value with no formula of its own is taken, by
// where it comes from.
var sourceWording = map[string]string{
	SourceWeatherStation: "Measured directly by the weather station.",
	SourceDevice:         "Measured directly by the sensor.",
	SourceManual:         "Read by a person and entered on a guided form.",
}

// unitLabels are SAM's unit symbols as people read them; a symbol not
// here is shown as it is.
var unitLabels = map[string]string{
	"degC":  "°C",
	"degF":  "°F",
	"deg":   "°",
	"ug/m3": "µg/m³",
	"kg/m3": "kg/m³",
	"W/m2":  "W/m²",
	"lx":    "lux",
	"uS/cm": "µS/cm",
	"1":     "",
	"count": "",
}

func unitLabel(symbol string) string {
	if l, ok := unitLabels[symbol]; ok {
		return l
	}
	return symbol
}

// Merge builds the registry the front end reads from SAM's and the
// overlay.
func Merge(s *SAM, o Overlay) *View {
	v := &View{Domains: []Domain{}, Templates: []Template{}, byID: map[string]*Parameter{}, sam: s}
	shown := v.domains(s, o)
	v.parameters(s, o, shown)
	v.deviceTypes(s, o)
	v.templates(s, o)
	return v
}

// Parameter is the value with id, or nil.
func (v *View) Parameter(id string) *Parameter { return v.byID[id] }

// ordered is SAM's entries with the overlay's first, in its order, then
// the rest in SAM's.
func ordered[E, O any](entries []E, overlay []O, entryID func(E) string, overlayID func(O) string) []E {
	at := map[string]int{}
	for i, o := range overlay {
		at[overlayID(o)] = i
	}
	out := slices.Clone(entries)
	slices.SortStableFunc(out, func(a, b E) int {
		i, aok := at[entryID(a)]
		j, bok := at[entryID(b)]
		switch {
		case aok && bok:
			return i - j
		case aok:
			return -1
		case bok:
			return 1
		}
		return 0
	})
	return out
}

func index[O any](overlay []O, id func(O) string) map[string]O {
	m := make(map[string]O, len(overlay))
	for _, o := range overlay {
		m[id(o)] = o
	}
	return m
}

// domains adds every domain not hidden, and returns the IDs shown.
func (v *View) domains(s *SAM, o Overlay) map[string]bool {
	over := index(o.Domains, func(d DomainOverlay) string { return d.ID })
	shown := map[string]bool{}
	for _, d := range ordered(s.Domains, o.Domains, func(d client.Domain) string { return d.ID }, func(d DomainOverlay) string { return d.ID }) {
		out := Domain{ID: d.ID, Label: d.Name, Icon: fallbackDomainIcon, Description: d.Description}
		if ov, ok := over[d.ID]; ok {
			if ov.Hidden {
				continue
			}
			out = Domain{ID: d.ID, Label: ov.Label, Icon: ov.Icon, Description: ov.Description}
		}
		shown[d.ID] = true
		v.Domains = append(v.Domains, out)
	}
	return shown
}

func (v *View) parameters(s *SAM, o Overlay, shown map[string]bool) {
	over := index(o.Parameters, func(p ParameterOverlay) string { return p.ID })
	derived := index(s.Derivations, func(d client.Derivation) string { return d.ParameterID })
	sent := sentByDevices(s)
	units := index(s.Units, func(u client.Unit) string { return u.Symbol })
	list := ordered(s.Parameters, o.Parameters, func(p client.Parameter) string { return p.ID }, func(p ParameterOverlay) string { return p.ID })
	v.Parameters = make([]Parameter, 0, len(list))
	for _, p := range list {
		out := Parameter{ID: p.ID, Label: p.Name, Definition: p.Definition, Unit: unitLabel(p.Unit),
			Domain: &p.DomainID, Source: SourceManual, ValueType: valueType(p.ValueType), Min: p.Min, Max: p.Max,
			Options: p.CategoryValues}
		if sent[p.ID] {
			out.Source = SourceDevice
		}
		ov, has := over[p.ID]
		if has {
			out.Label, out.Plain, out.Source, out.Noise = ov.Label, ov.Plain, ov.Source, ov.Noise
			if ov.Definition != nil {
				out.Definition = *ov.Definition
			}
			if ov.Domain != nil {
				out.Domain = ov.Domain
			}
			if ov.Unit != nil {
				out.Unit = *ov.Unit
			}
			if ov.Chart != nil && out.ValueType == valueNumber {
				out.Chart = *ov.Chart
			}
		}
		if out.Domain != nil && !shown[*out.Domain] {
			out.Domain = nil
		}
		if out.ValueType == valueNumber {
			out.AltUnits = altUnits(units, p.Unit)
		}
		out.Calculation = calculation(p, out.Source, ov, derived)
		v.Parameters = append(v.Parameters, out)
	}
	for i := range v.Parameters {
		v.byID[v.Parameters[i].ID] = &v.Parameters[i]
	}
}

// Value types as the front end has them.
const (
	valueNumber = "number"
	valueText   = "text"
	valueTime   = "time"
)

// valueType is SAM's value type as the front end has it.
func valueType(sam string) string {
	switch sam {
	case "number", "integer":
		return valueNumber
	case "datetime":
		return valueTime
	}
	return valueText
}

// sentByDevices are the parameters a device type with a sensor maps.
func sentByDevices(s *SAM) map[string]bool {
	out := map[string]bool{}
	for _, t := range s.DeviceTypes {
		if t.Manual {
			continue
		}
		for _, k := range t.Keys {
			if k.ParameterID != nil {
				out[*k.ParameterID] = true
			}
		}
	}
	return out
}

// altUnits are the catalogue's other units of the dimension of symbol, each
// converted to it.
func altUnits(units map[string]client.Unit, symbol string) []AltUnit {
	own, ok := units[symbol]
	if !ok || own.Scale == 0 {
		return nil
	}
	var out []AltUnit
	for _, u := range units {
		if u.Dimension != own.Dimension || u.Symbol == own.Symbol {
			continue
		}
		// base = v × scale + offset in either unit.
		out = append(out, AltUnit{Unit: unitLabel(u.Symbol), Factor: u.Scale / own.Scale, Offset: (u.Offset - own.Offset) / own.Scale})
	}
	slices.SortFunc(out, func(a, b AltUnit) int {
		switch {
		case a.Unit < b.Unit:
			return -1
		case a.Unit > b.Unit:
			return 1
		}
		return 0
	})
	return out
}

// Methods of a calculation.
const (
	MethodMeasured = "measured"
	MethodDerived  = "derived"
	MethodManual   = "manual"
)

// calculation is how SAM gets p: its derivation's inputs, with its formula
// or an admin's wording of it; otherwise how a value from its source is
// taken, or as an admin explains it.
func calculation(p client.Parameter, source string, ov ParameterOverlay, derived map[string]client.Derivation) *Calculation {
	if d, ok := derived[p.ID]; ok {
		c := &Calculation{Method: MethodDerived, Formula: d.Definition.Formula, Inputs: nonNil(d.Inputs),
			Version: fmt.Sprintf("SAM %s v%d", d.Function, d.FunctionVersion), PublishedAt: day(d.UpdatedAt)}
		if ov.Explanation != nil {
			c.Formula = *ov.Explanation
		}
		return c
	}
	c := &Calculation{Method: MethodMeasured, Formula: sourceWording[source], Inputs: []string{},
		Version: fmt.Sprintf("SAM %s v%d", p.ID, p.Version), PublishedAt: day(p.UpdatedAt)}
	if source == SourceManual {
		c.Method = MethodManual
	}
	if ov.Method != nil {
		c.Method, c.Inputs = *ov.Method, nonNil(ov.Inputs)
	}
	if ov.Explanation != nil {
		c.Formula = *ov.Explanation
	}
	return c
}

func day(t time.Time) string { return t.UTC().Format(time.DateOnly) }

func (v *View) deviceTypes(s *SAM, o Overlay) {
	over := index(o.DeviceTypes, func(t DeviceTypeOverlay) string { return t.ID })
	read := templateFieldsByType(s, o)
	list := ordered(s.DeviceTypes, o.DeviceTypes, func(t client.DeviceType) string { return t.ID }, func(t DeviceTypeOverlay) string { return t.ID })
	v.DeviceTypes = make([]DeviceType, 0, len(list))
	for _, t := range list {
		out := DeviceType{ID: t.ID, Label: t.Name, Description: t.Description, Color: fallbackTypeColor, Icon: fallbackTypeIcon,
			ParameterIDs: []string{}, TileParameterIDs: []string{}, CardParameterIDs: []string{}, Manual: t.Manual}
		for _, k := range t.Keys {
			if k.ParameterID != nil {
				out.ParameterIDs = v.addKnown(out.ParameterIDs, *k.ParameterID)
			}
		}
		// A device with no sensor sends nothing; it is read through its
		// templates.
		if len(t.Keys) == 0 {
			for _, id := range read[t.ID] {
				out.ParameterIDs = v.addKnown(out.ParameterIDs, id)
			}
		}
		if ov, ok := over[t.ID]; ok {
			out.Label, out.Description, out.Color, out.Icon = ov.Label, ov.Description, ov.Color, ov.Icon
			out.TileParameterIDs = among(ov.Tiles, out.ParameterIDs)
			out.CardParameterIDs = among(ov.Cards, out.ParameterIDs)
		}
		v.DeviceTypes = append(v.DeviceTypes, out)
	}
}

// addKnown adds id to ids once, if it is a parameter shown.
func (v *View) addKnown(ids []string, id string) []string {
	if v.byID[id] == nil || slices.Contains(ids, id) {
		return ids
	}
	return append(ids, id)
}

// among is the IDs of want in of, in want's order.
func among(want, of []string) []string {
	out := []string{}
	for _, id := range want {
		if slices.Contains(of, id) && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// templateFieldsByType are, for each device type, the parameters of the
// templates the overlay reads off it.
func templateFieldsByType(s *SAM, o Overlay) map[string][]string {
	over := index(o.Templates, func(t TemplateOverlay) string { return t.ID })
	out := map[string][]string{}
	for _, t := range s.Templates {
		ov, ok := over[t.ID]
		if !ok || ov.DeviceType == nil {
			continue
		}
		for _, f := range t.Fields {
			if f.ParameterID != nil {
				out[*ov.DeviceType] = append(out[*ov.DeviceType], *f.ParameterID)
			}
		}
	}
	return out
}

func (v *View) templates(s *SAM, o Overlay) {
	over := index(o.Templates, func(t TemplateOverlay) string { return t.ID })
	types := index(v.DeviceTypes, func(t DeviceType) string { return t.ID })
	covered := map[string]bool{}
	for _, t := range ordered(s.Templates, o.Templates, func(t client.Template) string { return t.ID }, func(t TemplateOverlay) string { return t.ID }) {
		out := Template{ID: t.ID, Label: t.Name, Description: t.Description, Fields: []TemplateField{}}
		for _, f := range t.Fields {
			if f.ParameterID != nil && v.byID[*f.ParameterID] != nil {
				out.Fields = append(out.Fields, TemplateField{ParameterID: *f.ParameterID, Required: f.Required})
			}
		}
		out.Domain = v.commonDomain(out.Fields)
		if ov, ok := over[t.ID]; ok {
			out.Label, out.Description = ov.Label, ov.Description
			if ov.Domain != nil && v.domainShown(*ov.Domain) {
				out.Domain = ov.Domain
			}
			if _, ok := types[deref(ov.DeviceType)]; ok {
				out.DeviceType = ov.DeviceType
				covered[*ov.DeviceType] = true
			}
		}
		v.Templates = append(v.Templates, out)
	}
	for _, t := range v.DeviceTypes {
		if covered[t.ID] || len(t.ParameterIDs) == 0 {
			continue
		}
		fields := make([]TemplateField, len(t.ParameterIDs))
		for i, id := range t.ParameterIDs {
			fields[i] = TemplateField{ParameterID: id}
		}
		// At least one value is needed; the first one the device sends
		// stands for the reading.
		fields[0].Required = true
		id := t.ID
		v.Templates = append(v.Templates, Template{ID: DevicePrefix + t.ID, Label: t.Label + " reading", Description: t.Description,
			Domain: v.commonDomain(fields), DeviceType: &id, Fields: fields})
	}
}

// commonDomain is the domain every field's value is shown under, or nil.
func (v *View) commonDomain(fields []TemplateField) *string {
	var common *string
	for i, f := range fields {
		d := v.byID[f.ParameterID].Domain
		if d == nil || (i > 0 && (common == nil || *common != *d)) {
			return nil
		}
		common = d
	}
	return common
}

func (v *View) domainShown(id string) bool {
	return slices.ContainsFunc(v.Domains, func(d Domain) bool { return d.ID == id })
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// own is this tenant's own entry of kind with id.
func (v *View) own(kind, id string) (Own, bool) {
	i := slices.IndexFunc(v.owns, func(o Own) bool { return o.Kind == kind && o.ID == id })
	if i < 0 {
		return Own{}, false
	}
	return v.owns[i], true
}

// hasDomain reports whether SAM's registry has the domain id.
func (s *SAM) hasDomain(id string) bool {
	return slices.ContainsFunc(s.Domains, func(d client.Domain) bool { return d.ID == id })
}

// domainOf is the domain SAM holds a parameter under, or "".
func (s *SAM) domainOf(id string) string {
	if i := slices.IndexFunc(s.Parameters, func(p client.Parameter) bool { return p.ID == id }); i >= 0 {
		return s.Parameters[i].DomainID
	}
	return ""
}

// derivation is how SAM derives a parameter, or nil.
func (s *SAM) derivation(id string) *client.Derivation {
	if i := slices.IndexFunc(s.Derivations, func(d client.Derivation) bool { return d.ParameterID == id }); i >= 0 {
		return &s.Derivations[i]
	}
	return nil
}
