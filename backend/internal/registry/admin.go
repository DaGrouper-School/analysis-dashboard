package registry

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

// registryIcons are the front end's registry icon names
// (frontend/src/lib/utils/icons.ts).
var registryIcons = []string{
	"station", "thermometer", "droplet", "rain", "gauge", "wind", "sun", "waves", "bolt", "tree", "sprout", "soil",
	"water", "particles", "cloud", "haze", "snow", "mountain", "compass", "radar", "antenna", "satellite", "flask",
	"pin", "sensor", "tag",
}

// chartLabels are how the front end names each chart.
var chartLabels = map[string]string{
	"line":       "Line",
	"line_band":  "Line with each day’s low to high",
	"daily_bars": "Daily bars",
}

// domainCode is a domain's code as an admin types it, as the front end
// checks it; SAM's own codes are the same form.
var domainCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,15}$`)

// PlainMax is the most a value's plain sentence holds.
const PlainMax = 240

const (
	msgNoDomain    = "That domain no longer exists."
	msgNoParameter = "That value is not in the registry."
)

// DomainInput is the front end's DomainInput.
type DomainInput struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
}

func checkDomain(in DomainInput, withID bool) map[string]string {
	f := map[string]string{}
	if withID && !domainCode.MatchString(strings.ToUpper(strings.TrimSpace(in.ID))) {
		f["id"] = "Use 2 to 16 letters, digits or underscores, starting with a letter."
	}
	if utf8.RuneCountInString(strings.TrimSpace(in.Label)) < 2 {
		f["label"] = "Enter a name."
	}
	if icon := strings.TrimSpace(in.Icon); icon != "" && !slices.Contains(registryIcons, icon) {
		f["icon"] = "Pick an icon."
	}
	return f
}

func (v *View) domain(id string) *Domain {
	i := slices.IndexFunc(v.Domains, func(d Domain) bool { return d.ID == id })
	if i < 0 {
		return nil
	}
	return &v.Domains[i]
}

func (m *Module) createDomain(w http.ResponseWriter, r *http.Request) error {
	var in DomainInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := httpx.CheckFields(checkDomain(in, true)); err != nil {
		return err
	}
	code := strings.ToUpper(strings.TrimSpace(in.ID))
	row := DomainOverlay{Label: strings.TrimSpace(in.Label), Icon: orDefault(strings.TrimSpace(in.Icon), fallbackDomainIcon),
		Description: strings.TrimSpace(in.Description)}
	var id string
	return m.run(w, r, func(e *edit) error {
		var err error
		id, err = e.createDomain(code, row)
		return err
	}, func(v *View) (any, error) { return v.domain(id), nil })
}

// createDomain adds this tenant's domain <Namespace>:<code>, or shows one
// of SAM's codes again that an admin removed.
func (e *edit) createDomain(code string, row DomainOverlay) (string, error) {
	taken := func(id string) error {
		msg := fmt.Sprintf("There is already a %s domain.", id)
		return httpx.FieldError(httpx.CodeConflict, "id", msg)
	}
	if e.view.domain(code) != nil {
		return "", taken(code)
	}
	if e.view.sam.hasDomain(code) {
		row.ID = code
		if err := e.putDomain(row); err != nil {
			return "", err
		}
		return code, e.log(fmt.Sprintf("Added the %s domain (%s).", row.Label, code))
	}
	id := Namespace + ":" + code
	if e.view.domain(id) != nil {
		return "", taken(id)
	}
	row.ID = id
	if err := e.putOwn(Own{Kind: OwnDomain, ID: id, Domain: &client.DomainInput{Name: row.Label, Description: row.Description}}); err != nil {
		return "", err
	}
	if err := e.putDomain(row); err != nil {
		return "", err
	}
	return id, e.log(fmt.Sprintf("Added the %s domain (%s).", row.Label, id))
}

func (m *Module) updateDomain(w http.ResponseWriter, r *http.Request) error {
	var in DomainInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := httpx.CheckFields(checkDomain(in, false)); err != nil {
		return err
	}
	id := r.PathValue("domainId")
	return m.run(w, r, func(e *edit) error {
		d := e.view.domain(id)
		if d == nil {
			return httpx.NotFound(msgNoDomain)
		}
		row, err := e.domainRow(*d)
		if err != nil {
			return err
		}
		row.Label, row.Description = strings.TrimSpace(in.Label), strings.TrimSpace(in.Description)
		row.Icon = orDefault(strings.TrimSpace(in.Icon), row.Icon)
		if isOwn(id) {
			if err := e.putOwn(Own{Kind: OwnDomain, ID: id, Domain: &client.DomainInput{Name: row.Label, Description: row.Description}}); err != nil {
				return err
			}
		}
		if err := e.putDomain(row); err != nil {
			return err
		}
		return e.log(fmt.Sprintf("Edited the %s domain (%s).", row.Label, id))
	}, func(v *View) (any, error) { return v.domain(id), nil })
}

func (m *Module) deleteDomain(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("domainId")
	err := m.run(w, r, func(e *edit) error {
		d := e.view.domain(id)
		if d == nil {
			return httpx.NotFound(msgNoDomain)
		}
		if n := e.view.underDomain(id); n > 0 {
			these, it := fmt.Sprintf("%d values are", n), "them"
			if n == 1 {
				these, it = "1 value is", "it"
			}
			return httpx.New(httpx.CodeConflict, fmt.Sprintf("%s mapped to %s. Move %s to another domain first.", these, d.Label, it))
		}
		if isOwn(id) {
			if err := e.putOwn(Own{Kind: OwnDomain, ID: id}); err != nil {
				return err
			}
			if err := e.dropDomain(id); err != nil {
				return err
			}
		} else {
			row, err := e.domainRow(*d)
			if err != nil {
				return err
			}
			row.Hidden = true
			if err := e.putDomain(row); err != nil {
				return err
			}
		}
		return e.log(fmt.Sprintf("Removed the %s domain (%s).", d.Label, id))
	}, nil)
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// underDomain counts the values shown under a domain, or held under it in
// SAM.
func (v *View) underDomain(id string) int {
	n := 0
	for _, p := range v.Parameters {
		if (p.Domain != nil && *p.Domain == id) || v.sam.domainOf(p.ID) == id {
			n++
		}
	}
	return n
}

func (m *Module) mapParameter(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Domain *string `json:"domain"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	id := r.PathValue("parameterId")
	return m.run(w, r, func(e *edit) error {
		p := e.view.Parameter(id)
		if p == nil {
			return httpx.NotFound(msgNoParameter)
		}
		to := "no domain"
		if in.Domain != nil {
			d := e.view.domain(*in.Domain)
			if d == nil {
				return httpx.NotFound(msgNoDomain)
			}
			to = d.Label
		}
		if deref(p.Domain) == deref(in.Domain) {
			return nil
		}
		from := "no domain"
		if p.Domain != nil {
			from = e.view.domain(*p.Domain).Label
		}
		row, err := e.parameterRow(p)
		if err != nil {
			return err
		}
		row.Domain = ptrTo(deref(in.Domain))
		if err := e.putParameter(row); err != nil {
			return err
		}
		// One of this tenant's own values moved to a domain also moves in
		// SAM, where it needs one.
		if in.Domain != nil && isOwn(id) {
			if err := e.moveOwn(id, *in.Domain); err != nil {
				return err
			}
		}
		return e.log(fmt.Sprintf("Moved %s (%s) from %s to %s.", p.Label, id, from, to))
	}, func(v *View) (any, error) { return v.Parameter(id), nil })
}

// moveOwn puts one of this tenant's own values under domain in SAM.
func (e *edit) moveOwn(id, domain string) error {
	o, ok := e.view.own(OwnParameter, id)
	if !ok || o.Parameter == nil {
		return nil
	}
	in := *o.Parameter
	in.DomainID = domain
	return e.putOwn(Own{Kind: OwnParameter, ID: id, Parameter: &in})
}

func (m *Module) chartParameter(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Chart string `json:"chart"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	id := r.PathValue("parameterId")
	return m.run(w, r, func(e *edit) error {
		p := e.view.Parameter(id)
		switch {
		case p == nil:
			return httpx.NotFound(msgNoParameter)
		case p.ValueType != valueNumber:
			return httpx.FieldError(httpx.CodeInvalidInput, "chart", "Only numbers are charted.")
		case chartLabels[in.Chart] == "":
			return httpx.FieldError(httpx.CodeInvalidInput, "chart", "Pick a chart.")
		case orDefault(p.Chart, "line") == in.Chart:
			return nil
		}
		row, err := e.parameterRow(p)
		if err != nil {
			return err
		}
		row.Chart = &in.Chart
		if err := e.putParameter(row); err != nil {
			return err
		}
		return e.log(fmt.Sprintf("Set the %s chart to %s.", p.Label, strings.ToLower(chartLabels[in.Chart])))
	}, func(v *View) (any, error) { return v.Parameter(id), nil })
}

// Explanation is the front end's ExplanationInput.
type Explanation struct {
	Plain   string   `json:"plain"`
	Method  string   `json:"method"`
	Formula string   `json:"formula"`
	Inputs  []string `json:"inputs"`
}

// checkExplanation reports what is wrong with an explanation of p, by
// field: the front end's checks, and SAM's. A value SAM derives is derived
// from SAM's inputs; any other is measured or entered.
func (v *View) checkExplanation(in Explanation, p *Parameter) map[string]string {
	f := map[string]string{}
	if utf8.RuneCountInString(strings.TrimSpace(in.Plain)) > PlainMax {
		f["plain"] = fmt.Sprintf("Keep it to %d characters: one plain sentence.", PlainMax)
	}
	derived := v.sam.derivation(p.ID)
	switch {
	case !slices.Contains([]string{MethodMeasured, MethodDerived, MethodManual}, in.Method):
		f["method"] = "Pick a method."
	case derived != nil && in.Method != MethodDerived:
		f["method"] = "SAM calculates this value from other values."
	case derived == nil && in.Method == MethodDerived:
		f["method"] = "SAM does not calculate this value: it is measured or entered."
	}
	if strings.TrimSpace(in.Formula) == "" {
		f["formula"] = "Say how the value is taken."
		if in.Method == MethodDerived {
			f["formula"] = "Enter the formula."
		}
	}
	var unknown []string
	for _, id := range in.Inputs {
		if v.Parameter(id) == nil {
			unknown = append(unknown, id)
		}
	}
	switch {
	case len(unknown) > 0:
		f["inputs"] = fmt.Sprintf("Not in the registry: %s.", strings.Join(unknown, ", "))
	case slices.Contains(in.Inputs, p.ID):
		f["inputs"] = "A value cannot be its own input."
	case derived != nil && in.Method == MethodDerived && !sameSet(in.Inputs, derived.Inputs):
		f["inputs"] = fmt.Sprintf("SAM calculates it from %s.", strings.Join(derived.Inputs, ", "))
	case derived == nil && len(in.Inputs) > 0:
		f["inputs"] = "A value that is measured or entered has no inputs."
	}
	return f
}

func (m *Module) explainParameter(w http.ResponseWriter, r *http.Request) error {
	var in Explanation
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	in.Inputs = dedupe(in.Inputs)
	id := r.PathValue("parameterId")
	return m.run(w, r, func(e *edit) error {
		p := e.view.Parameter(id)
		if p == nil {
			return httpx.NotFound(msgNoParameter)
		}
		if err := httpx.CheckFields(e.view.checkExplanation(in, p)); err != nil {
			return err
		}
		row, err := e.parameterRow(p)
		if err != nil {
			return err
		}
		row.Plain, row.Explanation = strings.TrimSpace(in.Plain), ptrTo(strings.TrimSpace(in.Formula))
		row.Method, row.Inputs = &in.Method, in.Inputs
		if err := e.putParameter(row); err != nil {
			return err
		}
		return e.log(fmt.Sprintf("Edited what %s means and how it is calculated.", p.Label))
	}, func(v *View) (any, error) { return v.Parameter(id), nil })
}

// dedupe is ids trimmed, blanks and repeats left out, in order.
func dedupe(ids []string) []string {
	out := []string{}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

func sameSet(a, b []string) bool {
	return len(a) == len(b) && !slices.ContainsFunc(a, func(x string) bool { return !slices.Contains(b, x) })
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func ptrTo[T any](v T) *T { return &v }
