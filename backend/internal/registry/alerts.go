package registry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// AlertRule is the front end's AlertRule: one of the platform's default
// alert levels, in the parameter's own unit.
type AlertRule struct {
	ID          string  `json:"id"`
	ParameterID string  `json:"parameterId"`
	Direction   string  `json:"direction"`
	Value       float64 `json:"value"`
	Level       string  `json:"level"`
	Title       string  `json:"title"`
	Enabled     bool    `json:"enabled"`
}

// AlertRuleInput is the front end's AlertRuleInput.
type AlertRuleInput struct {
	ParameterID string   `json:"parameterId"`
	Direction   string   `json:"direction"`
	Value       *float64 `json:"value"`
	Level       string   `json:"level"`
	Title       string   `json:"title"`
	Enabled     *bool    `json:"enabled"`
}

// AlertTitleMax is the most an alert's name holds.
const AlertTitleMax = 60

var (
	levelLabels     = map[string]string{"warning": "Warning", "caution": "Caution"}
	directionLabels = map[string]string{"above": "at or above", "below": "at or below"}
)

// checkAlert reports what is wrong with a level, by field, as the front
// end checks it: a number value of the registry, within its range.
func (v *View) checkAlert(in AlertRuleInput) map[string]string {
	f := map[string]string{}
	p := v.Parameter(in.ParameterID)
	switch {
	case p == nil:
		f["parameterId"] = "Pick a value from the registry."
	case p.ValueType != valueNumber:
		f["parameterId"] = fmt.Sprintf("%s is not a number, so it cannot have a level.", p.Label)
	}
	if directionLabels[in.Direction] == "" {
		f["direction"] = "Pick above or below."
	}
	if levelLabels[in.Level] == "" {
		f["level"] = "Pick caution or warning."
	}
	switch {
	case in.Value == nil || math.IsNaN(*in.Value) || math.IsInf(*in.Value, 0):
		f["value"] = "The level must be a number."
	case p != nil && ((p.Min != nil && *in.Value < *p.Min) || (p.Max != nil && *in.Value > *p.Max)):
		f["value"] = fmt.Sprintf("%s should be %s.", p.Label, rangeText(p))
	}
	switch title := strings.TrimSpace(in.Title); {
	case title == "":
		f["title"] = "Give the alert a short name."
	case utf8.RuneCountInString(title) > AlertTitleMax:
		f["title"] = fmt.Sprintf("Keep the name to %d characters.", AlertTitleMax)
	}
	return f
}

// formatValue is a value as the front end writes it: whole from 100, to
// one decimal below, with its unit.
func formatValue(v float64, unit string) string {
	r := math.Round(v*10) / 10
	if math.Abs(v) >= 100 {
		r = math.Round(v)
	}
	s := strconv.FormatFloat(r, 'f', -1, 64)
	if unit != "" && !strings.HasPrefix(unit, "°") {
		s += " "
	}
	return s + unit
}

func rangeText(p *Parameter) string {
	with := func(v float64) string { return formatValue(v, p.Unit) }
	switch {
	case p.Min == nil:
		return "up to " + with(*p.Max)
	case p.Max == nil:
		return "at least " + with(*p.Min)
	}
	return strconv.FormatFloat(*p.Min, 'f', -1, 64) + " to " + with(*p.Max)
}

// ruleText is a level as the front end words it, such as Warning at or
// above 8 (Very high UV).
func ruleText(r AlertRule, unit string) string {
	return fmt.Sprintf("%s %s %s (%s)", levelLabels[r.Level], directionLabels[r.Direction], formatValue(r.Value, unit), r.Title)
}

const alertColumns = `id, parameter_id, direction, value, level, title, enabled`

func scanAlert(r scanner) (a AlertRule, err error) {
	return a, r.Scan(&a.ID, &a.ParameterID, &a.Direction, &a.Value, &a.Level, &a.Title, &a.Enabled)
}

// AlertDefaults are the platform's default alert levels, in display order.
func AlertDefaults(ctx context.Context, q db.Querier) ([]AlertRule, error) {
	list, err := rows(ctx, q, `SELECT `+alertColumns+` FROM alert_defaults ORDER BY position, id`, scanAlert)
	return nonNilList(list), err
}

func (m *Module) listAlertDefaults(w http.ResponseWriter, r *http.Request) error {
	list, err := AlertDefaults(r.Context(), m.db)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, list)
	return nil
}

// answerAlerts answers with every default level, as admins' changes to
// them do.
func (m *Module) answerAlerts(r *http.Request) func(*View) (any, error) {
	return func(*View) (any, error) { return AlertDefaults(r.Context(), m.db) }
}

func (m *Module) createAlertDefault(w http.ResponseWriter, r *http.Request) error {
	return m.saveAlertDefault(w, r, "")
}

func (m *Module) updateAlertDefault(w http.ResponseWriter, r *http.Request) error {
	return m.saveAlertDefault(w, r, r.PathValue("ruleId"))
}

// saveAlertDefault adds a default level, or changes the one with id.
// Turning one off is a change with enabled false; a level stays on its
// value.
func (m *Module) saveAlertDefault(w http.ResponseWriter, r *http.Request, id string) error {
	var in AlertRuleInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	return m.run(w, r, func(e *edit) error {
		if err := httpx.CheckFields(e.view.checkAlert(in)); err != nil {
			return err
		}
		p := e.view.Parameter(in.ParameterID)
		rule := AlertRule{ID: id, ParameterID: in.ParameterID, Direction: in.Direction, Value: *in.Value, Level: in.Level,
			Title: strings.TrimSpace(in.Title), Enabled: in.Enabled == nil || *in.Enabled}
		if id == "" {
			rule.ID = newAlertID()
			if _, err := e.tx.Exec(e.ctx, `INSERT INTO alert_defaults (`+alertColumns+`, position, seeded, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, (SELECT coalesce(max(position) + 1, 0) FROM alert_defaults), false, $8)`,
				rule.ID, rule.ParameterID, rule.Direction, rule.Value, rule.Level, rule.Title, rule.Enabled, e.now); err != nil {
				return err
			}
			return e.log(fmt.Sprintf("Added the default %s level %s.", p.Label, ruleText(rule, p.Unit)))
		}
		stored, err := e.alert(id)
		if err != nil {
			return err
		}
		if stored.ParameterID != rule.ParameterID {
			return httpx.FieldError(httpx.CodeInvalidInput, "parameterId", "An alert stays on its value. Add a new one for another value.")
		}
		if _, err := e.tx.Exec(e.ctx, `UPDATE alert_defaults SET direction = $2, value = $3, level = $4, title = $5, enabled = $6,
			seeded = false, updated_at = $7 WHERE id = $1`, id, rule.Direction, rule.Value, rule.Level, rule.Title, rule.Enabled, e.now); err != nil {
			return err
		}
		before, after := ruleText(stored, p.Unit), ruleText(rule, p.Unit)
		if stored.Enabled != rule.Enabled && before == after {
			on := map[bool]string{true: "on", false: "off"}[rule.Enabled]
			return e.log(fmt.Sprintf("Turned %s the default %s level %s.", on, p.Label, before))
		}
		return e.log(fmt.Sprintf("Changed the default %s level %s to %s.", p.Label, before, after))
	}, m.answerAlerts(r))
}

func (m *Module) deleteAlertDefault(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("ruleId")
	return m.run(w, r, func(e *edit) error {
		stored, err := e.alert(id)
		if err != nil {
			return err
		}
		if _, err := e.tx.Exec(e.ctx, `DELETE FROM alert_defaults WHERE id = $1`, id); err != nil {
			return err
		}
		label, unit := stored.ParameterID, ""
		if p := e.view.Parameter(stored.ParameterID); p != nil {
			label, unit = p.Label, p.Unit
		}
		return e.log(fmt.Sprintf("Removed the default %s level %s.", label, ruleText(stored, unit)))
	}, m.answerAlerts(r))
}

// alert is the stored level with id, locked, or not found.
func (e *edit) alert(id string) (AlertRule, error) {
	a, err := scanAlert(e.tx.QueryRow(e.ctx, `SELECT `+alertColumns+` FROM alert_defaults WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return a, httpx.NotFound("That alert no longer exists.")
	}
	return a, err
}

func newAlertID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "alr_" + hex.EncodeToString(b)
}

// KindAlertDefault is what a default level is, as the seed reports it.
const KindAlertDefault = "alert default"

var alertList = settings.List[AlertRule]{
	Kind: KindAlertDefault, Unlogged: true,
	ID:    func(a AlertRule) string { return a.ID },
	Count: counter("alert_defaults"),
	Current: current("alert_defaults", alertColumns, func(r scanner, a *AlertRule) error {
		var err error
		*a, err = scanAlert(r)
		return err
	}),
	Insert: func(ctx context.Context, q db.Querier, now time.Time, a AlertRule, position int) error {
		_, err := q.Exec(ctx, `INSERT INTO alert_defaults (`+alertColumns+`, position, seeded, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, true, $9)`,
			a.ID, a.ParameterID, a.Direction, a.Value, a.Level, a.Title, a.Enabled, position, now)
		return err
	},
	Update: func(ctx context.Context, q db.Querier, now time.Time, a AlertRule, position int) error {
		_, err := q.Exec(ctx, `UPDATE alert_defaults SET parameter_id = $2, direction = $3, value = $4, level = $5, title = $6,
			enabled = $7, position = $8, updated_at = $9 WHERE id = $1 AND seeded`,
			a.ID, a.ParameterID, a.Direction, a.Value, a.Level, a.Title, a.Enabled, position, now)
		return err
	},
}

// alertID is a level's ID, such as alr_default_uv_warning.
var alertID = regexp.MustCompile(`^alr_[a-z0-9_]+$`)

// checkAlertSeed reports what is wrong with the seed's levels.
func checkAlertSeed(list []AlertRule) []string {
	p := jsoncheck.IDs("alertDefaults", list, func(a AlertRule) string { return a.ID }, alertID)
	for i, a := range list {
		at := fmt.Sprintf("alertDefaults[%d]", i)
		if directionLabels[a.Direction] == "" {
			p = append(p, fmt.Sprintf("%s.direction: %q is not above or below", at, a.Direction))
		}
		if levelLabels[a.Level] == "" {
			p = append(p, fmt.Sprintf("%s.level: %q is not caution or warning", at, a.Level))
		}
	}
	return append(p, jsoncheck.Blank(list, "alertDefaults")...)
}
