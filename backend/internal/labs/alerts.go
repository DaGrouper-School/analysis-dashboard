package labs

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
)

// Alert levels, as the front end's alerts.ts has them: for each parameter
// of a device, the first layer with rules is in force, the device's own,
// then its classroom's for its sensor type, then the platform defaults.
// Rules are kept in the parameter's own unit, the unit SAM stores values
// in, and evaluated against a device's latest reading when it is read.

// The layers of alert rules.
const (
	ScopeDevice  = "device"
	ScopeType    = "type"
	ScopeDefault = "default"
)

// Rule is the front end's EffectiveAlertRule.
type Rule struct {
	ID          string     `json:"id"`
	ParameterID string     `json:"parameterId"`
	Direction   string     `json:"direction"`
	Value       float64    `json:"value"`
	Level       string     `json:"level"`
	Title       string     `json:"title"`
	Enabled     bool       `json:"enabled"`
	Source      string     `json:"source"`
	UpdatedBy   *string    `json:"updatedBy"`
	UpdatedAt   *time.Time `json:"updatedAt"`
}

// Alert is the front end's DeviceAlert.
type Alert struct {
	ParameterID string `json:"parameterId"`
	Level       string `json:"level"`
	Title       string `json:"title"`
	Message     string `json:"message"`
}

// DeviceAlerts is the front end's DeviceAlerts.
type DeviceAlerts struct {
	DeviceID      uuid.UUID `json:"deviceId"`
	DeviceName    string    `json:"deviceName"`
	DeviceType    string    `json:"deviceType"`
	ClassroomName string    `json:"classroomName"`
	Rules         []Rule    `json:"rules"`
	CanEditDevice bool      `json:"canEditDevice"`
	CanEditType   bool      `json:"canEditType"`
}

var (
	severity        = map[string]int{"warning": 2, "caution": 1}
	directionLabels = map[string]string{"above": "at or above", "below": "at or below"}
	whose           = map[string]string{ScopeDevice: "this device's", ScopeType: "the classroom's", ScopeDefault: "the default"}
)

// bySeverity orders rules most severe first and, within a level, the one
// that trips first.
func bySeverity(a, b Rule) int {
	if d := severity[b.Level] - severity[a.Level]; d != 0 {
		return d
	}
	diff := a.Value - b.Value
	if a.Direction != "above" {
		diff = b.Value - a.Value
	}
	switch {
	case diff < 0:
		return -1
	case diff > 0:
		return 1
	}
	return 0
}

// Effective are the rules in force: each layer's for the parameters no
// layer above has rules for.
func Effective(defaults, typeRules, deviceRules []Rule) []Rule {
	params := func(list []Rule) map[string]bool {
		out := map[string]bool{}
		for _, r := range list {
			out[r.ParameterID] = true
		}
		return out
	}
	fromDevice, fromType := params(deviceRules), params(typeRules)
	out := slices.Clone(deviceRules)
	for _, r := range typeRules {
		if !fromDevice[r.ParameterID] {
			out = append(out, r)
		}
	}
	for _, r := range defaults {
		if !fromDevice[r.ParameterID] && !fromType[r.ParameterID] {
			out = append(out, r)
		}
	}
	slices.SortStableFunc(out, bySeverity)
	return out
}

func crosses(r Rule, v float64) bool {
	if r.Direction == "above" {
		return v >= r.Value
	}
	return v <= r.Value
}

// AlertsFor are the alerts a reading's values trip, one per parameter,
// most severe first, each stating what was measured against which level;
// parameter looks a parameter up in the registry, nil for one it lacks.
func AlertsFor(values map[string]any, rules []Rule, parameter func(id string) *registry.Parameter) []Alert {
	type hit struct {
		rule  Rule
		alert Alert
	}
	var hits []hit
	var seen []string
	for _, r := range rules {
		if slices.Contains(seen, r.ParameterID) {
			continue
		}
		seen = append(seen, r.ParameterID)
		value, ok := values[r.ParameterID].(float64)
		if !ok {
			continue
		}
		var tripped []Rule
		for _, c := range rules {
			if c.Enabled && c.ParameterID == r.ParameterID && crosses(c, value) {
				tripped = append(tripped, c)
			}
		}
		if len(tripped) == 0 {
			continue
		}
		slices.SortStableFunc(tripped, bySeverity)
		t := tripped[0]
		label, unit := r.ParameterID, ""
		if p := parameter(r.ParameterID); p != nil {
			label, unit = p.Label, p.Unit
		}
		hits = append(hits, hit{t, Alert{ParameterID: r.ParameterID, Level: t.Level, Title: t.Title,
			Message: fmt.Sprintf("%s %s is %s %s %s level of %s.", label, registry.FormatValue(value, unit), directionLabels[t.Direction],
				whose[t.Source], t.Level, registry.FormatValue(t.Value, unit))}})
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return bySeverity(a.rule, b.rule) })
	out := make([]Alert, len(hits))
	for i, h := range hits {
		out[i] = h.alert
	}
	return out
}

// stored is a device or type rule as kept.
type stored struct {
	Rule
	updatedBy *uuid.UUID
}

const ruleColumns = `id, scope, parameter_id, direction, value, level, title, enabled, updated_by, updated_at`

func scanRule(r pgx.Row) (stored, error) {
	var s stored
	var id uuid.UUID
	var at time.Time
	err := r.Scan(&id, &s.Source, &s.ParameterID, &s.Direction, &s.Value, &s.Level, &s.Title, &s.Enabled, &s.updatedBy, &at)
	s.ID, s.UpdatedAt = id.String(), &at
	return s, err
}

// layer is where a device's rules at scope are kept.
func layer(d Device, scope string) (string, []any) {
	if scope == ScopeDevice {
		return `scope = 'device' AND device_id = $1`, []any{d.ID}
	}
	return `scope = 'type' AND classroom_id = $1 AND device_type = $2`, []any{d.ClassroomID, d.Type}
}

func layerRules(ctx context.Context, q db.Querier, d Device, scope string) ([]stored, error) {
	where, args := layer(d, scope)
	rows, err := q.Query(ctx, `SELECT `+ruleColumns+` FROM alert_rules WHERE `+where+` ORDER BY created_at, id`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (stored, error) { return scanRule(r) })
}

// rulesFor are a device's rules in force, with the names of who last
// changed them, as they are named now.
func (m *Module) rulesFor(ctx context.Context, d Device, defaults []registry.AlertRule, p *people) ([]Rule, error) {
	named := func(list []stored) []Rule {
		out := make([]Rule, len(list))
		for i, s := range list {
			r := s.Rule
			name := "Unknown"
			if s.updatedBy != nil {
				if u := p.get(*s.updatedBy); u != nil {
					name = u.FullName
				}
			}
			r.UpdatedBy = &name
			out[i] = r
		}
		return out
	}
	typeRules, err := layerRules(ctx, m.db, d, ScopeType)
	if err != nil {
		return nil, err
	}
	deviceRules, err := layerRules(ctx, m.db, d, ScopeDevice)
	if err != nil {
		return nil, err
	}
	base := make([]Rule, len(defaults))
	for i, r := range defaults {
		base[i] = Rule{ID: r.ID, ParameterID: r.ParameterID, Direction: r.Direction, Value: r.Value, Level: r.Level, Title: r.Title,
			Enabled: r.Enabled, Source: ScopeDefault}
	}
	return Effective(base, named(typeRules), named(deviceRules)), nil
}

const (
	msgNoRule     = "That alert no longer exists."
	msgNoDefault  = "Platform defaults can’t be deleted. Turn one off instead."
	msgEditDevice = "Only the classroom’s institution head and facilitators can change a device’s alerts."
	msgEditType   = "Only the institution head sets alerts for every sensor of a type in the classroom."
	msgStaysOnIts = "An alert stays on its value. Add a new one for another value."
	msgNoLayer    = "Pick the device or its sensor type."
)

// alertDevice is the device in the path, its classroom and what the viewer
// may do there; one they may not see is not found.
func (m *Module) alertDevice(r *http.Request) (Device, classrooms.Classroom, roles.Permissions, error) {
	id, err := uuid.Parse(r.PathValue("deviceId"))
	if err != nil {
		return Device{}, classrooms.Classroom{}, roles.Permissions{}, httpx.NotFound(msgNoDevice)
	}
	d, err := GetDevice(r.Context(), m.db, id)
	if errors.Is(err, ErrNoDevice) || err == nil && d.Removed {
		return d, classrooms.Classroom{}, roles.Permissions{}, httpx.NotFound(msgNoDevice)
	}
	if err != nil {
		return d, classrooms.Classroom{}, roles.Permissions{}, err
	}
	c, _, can, err := m.classroom(r, d.ClassroomID.String())
	if httpx.IsCode(err, httpx.CodeNotFound) {
		err = httpx.NotFound(msgNoDevice)
	}
	return d, c, can, err
}

// mayEdit refuses an edit at scope the viewer may not make.
func mayEdit(can roles.Permissions, scope string) error {
	switch {
	case scope == ScopeDevice && !can.ManageDeviceAlerts:
		return httpx.Forbidden(msgEditDevice)
	case scope == ScopeType && !can.ManageTypeAlerts:
		return httpx.Forbidden(msgEditType)
	}
	return nil
}

func (m *Module) alertsAnswer(ctx context.Context, d Device, c classrooms.Classroom, can roles.Permissions) (DeviceAlerts, error) {
	defaults, err := registry.AlertDefaults(ctx, m.db)
	if err != nil {
		return DeviceAlerts{}, err
	}
	p := &people{ctx: ctx, q: m.db, users: map[uuid.UUID]*accounts.User{}}
	rules, err := m.rulesFor(ctx, d, defaults, p)
	return DeviceAlerts{DeviceID: d.ID, DeviceName: d.Name, DeviceType: d.Type, ClassroomName: c.Name, Rules: rules,
		CanEditDevice: can.ManageDeviceAlerts, CanEditType: can.ManageTypeAlerts}, err
}

func (m *Module) listDeviceAlerts(w http.ResponseWriter, r *http.Request) error {
	d, c, can, err := m.alertDevice(r)
	if err != nil {
		return err
	}
	out, err := m.alertsAnswer(r.Context(), d, c, can)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (m *Module) createAlert(w http.ResponseWriter, r *http.Request) error {
	return m.saveAlert(w, r, "")
}

func (m *Module) updateAlert(w http.ResponseWriter, r *http.Request) error {
	return m.saveAlert(w, r, r.PathValue("ruleId"))
}

// saveAlert adds a rule at the scope for the device's parameter, or
// changes one. The first change at a layer copies the levels it replaces
// for that parameter, so changing one leaves the others as they were.
func (m *Module) saveAlert(w http.ResponseWriter, r *http.Request, ruleID string) error {
	d, c, can, err := m.alertDevice(r)
	if err != nil {
		return err
	}
	scope := r.URL.Query().Get("scope")
	if scope != ScopeDevice && scope != ScopeType {
		return httpx.FieldError(httpx.CodeInvalidInput, "scope", msgNoLayer)
	}
	if err := mayEdit(can, scope); err != nil {
		return err
	}
	var in registry.AlertRuleInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	in.Title = strings.TrimSpace(in.Title)
	v, err := m.reg.View(r.Context())
	if err != nil {
		return err
	}
	if err := httpx.CheckFields(v.CheckAlert(in)); err != nil {
		return err
	}
	cur, _ := accounts.CurrentUser(r.Context())
	if err := pgx.BeginFunc(r.Context(), m.db, func(tx pgx.Tx) error {
		return m.putRule(r.Context(), tx, d, scope, in, ruleID, cur.ID)
	}); err != nil {
		return err
	}
	out, err := m.alertsAnswer(r.Context(), d, c, can)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// putRule writes a rule at scope within tx, the layer's rules for its
// parameter first copied from the layer it replaces when it has none.
func (m *Module) putRule(ctx context.Context, tx pgx.Tx, d Device, scope string, in registry.AlertRuleInput, ruleID string, by uuid.UUID) error {
	if err := db.LockTx(ctx, tx, db.AlertRulesLock); err != nil {
		return err
	}
	now := m.now()
	copies, err := m.copyBelow(ctx, tx, d, scope, in.ParameterID, by, now)
	if err != nil {
		return err
	}
	enabled := in.Enabled == nil || *in.Enabled
	if ruleID == "" {
		return insertRule(ctx, tx, d, scope, Rule{ID: uuid.NewString(), ParameterID: in.ParameterID, Direction: in.Direction, Value: *in.Value,
			Level: in.Level, Title: in.Title, Enabled: enabled}, by, now)
	}
	if id, ok := copies[ruleID]; ok {
		ruleID = id
	}
	id, err := uuid.Parse(ruleID)
	if err != nil {
		return httpx.NotFound(msgNoRule)
	}
	where, args := layer(d, scope)
	s, err := scanRule(tx.QueryRow(ctx, `SELECT `+ruleColumns+` FROM alert_rules WHERE id = $`+fmt.Sprint(len(args)+1)+` AND `+where+` FOR UPDATE`,
		append(args, id)...))
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound(msgNoRule)
	}
	if err != nil {
		return err
	}
	if s.ParameterID != in.ParameterID {
		return httpx.FieldError(httpx.CodeInvalidInput, "parameterId", msgStaysOnIts)
	}
	_, err = tx.Exec(ctx, `UPDATE alert_rules SET direction = $2, value = $3, level = $4, title = $5, enabled = $6, updated_by = $7, updated_at = $8
		WHERE id = $1`, id, in.Direction, *in.Value, in.Level, in.Title, enabled, by, now)
	return err
}

// copyBelow copies the levels a layer replaces for a parameter, the type's
// or the defaults' for a device and the defaults' for a type, when the
// layer has none yet. It returns the copies' IDs by the IDs they were
// copied from.
func (m *Module) copyBelow(ctx context.Context, tx pgx.Tx, d Device, scope, param string, by uuid.UUID, now time.Time) (map[string]string, error) {
	copies := map[string]string{}
	mine, err := layerRules(ctx, tx, d, scope)
	if err != nil || slices.ContainsFunc(mine, func(s stored) bool { return s.ParameterID == param }) {
		return copies, err
	}
	defaults, err := registry.AlertDefaults(ctx, tx)
	if err != nil {
		return nil, err
	}
	var below []Rule
	for _, r := range defaults {
		below = append(below, Rule{ID: r.ID, ParameterID: r.ParameterID, Direction: r.Direction, Value: r.Value, Level: r.Level, Title: r.Title,
			Enabled: r.Enabled, Source: ScopeDefault})
	}
	if scope == ScopeDevice {
		typeRules, err := layerRules(ctx, tx, d, ScopeType)
		if err != nil {
			return nil, err
		}
		list := make([]Rule, len(typeRules))
		for i, s := range typeRules {
			list[i] = s.Rule
		}
		below = Effective(below, list, nil)
	}
	for _, r := range below {
		if r.ParameterID != param {
			continue
		}
		c := r
		c.ID = uuid.NewString()
		if err := insertRule(ctx, tx, d, scope, c, by, now); err != nil {
			return nil, err
		}
		copies[r.ID] = c.ID
	}
	return copies, nil
}

func insertRule(ctx context.Context, q db.Querier, d Device, scope string, r Rule, by uuid.UUID, now time.Time) error {
	var device, classroom *uuid.UUID
	var deviceType *string
	if scope == ScopeDevice {
		device = &d.ID
	} else {
		classroom, deviceType = &d.ClassroomID, &d.Type
	}
	_, err := q.Exec(ctx, `INSERT INTO alert_rules (id, scope, device_id, classroom_id, device_type, parameter_id, direction, value, level, title, enabled,
		updated_by, updated_at, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13)`,
		r.ID, scope, device, classroom, deviceType, r.ParameterID, r.Direction, r.Value, r.Level, r.Title, r.Enabled, by, now)
	return err
}

func (m *Module) deleteAlert(w http.ResponseWriter, r *http.Request) error {
	d, c, can, err := m.alertDevice(r)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(r.PathValue("ruleId"))
	if err != nil {
		return httpx.NotFound(msgNoDefault)
	}
	var found *stored
	for _, scope := range []string{ScopeDevice, ScopeType} {
		list, err := layerRules(r.Context(), m.db, d, scope)
		if err != nil {
			return err
		}
		if i := slices.IndexFunc(list, func(s stored) bool { return s.ID == id.String() }); i >= 0 {
			found = &list[i]
		}
	}
	if found == nil {
		return httpx.NotFound(msgNoDefault)
	}
	if err := mayEdit(can, found.Source); err != nil {
		return err
	}
	if _, err := m.db.Exec(r.Context(), `DELETE FROM alert_rules WHERE id = $1`, id); err != nil {
		return err
	}
	out, err := m.alertsAnswer(r.Context(), d, c, can)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// resetAlerts drops a layer's rules for a parameter, back to the next
// layer down.
func (m *Module) resetAlerts(w http.ResponseWriter, r *http.Request) error {
	d, c, can, err := m.alertDevice(r)
	if err != nil {
		return err
	}
	scope := r.URL.Query().Get("scope")
	if scope != ScopeDevice && scope != ScopeType {
		return httpx.NotFound(msgNoLayer)
	}
	if err := mayEdit(can, scope); err != nil {
		return err
	}
	where, args := layer(d, scope)
	if _, err := m.db.Exec(r.Context(), `DELETE FROM alert_rules WHERE `+where+` AND parameter_id = $`+fmt.Sprint(len(args)+1),
		append(args, r.URL.Query().Get("parameter"))...); err != nil {
		return err
	}
	out, err := m.alertsAnswer(r.Context(), d, c, can)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}
