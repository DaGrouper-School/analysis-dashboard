package apitest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// admin is a signed-in platform admin, made one as an operator does, so
// SAM holds them as one once the outbox delivers it; with the registry's
// seed loaded.
func admin(t *testing.T, e *apitest.Env) *profile {
	t.Helper()
	if _, err := registry.SeedOverlay(context.Background(), e.Pool, time.Now()); err != nil {
		t.Fatal(err)
	}
	p := signedIn(t, e, amara)
	if _, _, err := accounts.SetAdmin(context.Background(), e.Pool, time.Now(), settings.Operator, p.id, true); err != nil {
		t.Fatal(err)
	}
	return p
}

// send sends body to path and decodes the answer into out, or the refusal.
func send(t *testing.T, e *apitest.Env, p *profile, method, path string, body, out any) (int, refusal) {
	t.Helper()
	raw := ""
	if body != nil {
		b, _ := json.Marshal(body)
		raw = string(b)
	}
	res, got := e.DoWith(t, p.client, method, "/api/registry"+path, raw)
	var no refusal
	if res.StatusCode >= 400 {
		_ = json.Unmarshal([]byte(got), &no)
	} else if out != nil {
		if err := json.Unmarshal([]byte(got), out); err != nil {
			t.Fatalf("%s %s: %s", method, path, got)
		}
	}
	return res.StatusCode, no
}

// deliver sends the outbox to SAM until no change to an entity starting
// with prefix waits.
func deliver(t *testing.T, e *apitest.Env, prefix string) {
	t.Helper()
	w := e.Worker()
	for range 20 {
		if _, err := w.Pass(context.Background()); err != nil {
			t.Fatal(err)
		}
		var waiting int
		if err := e.Pool.QueryRow(context.Background(), `SELECT count(*) FROM sam_outbox WHERE state IN ('pending', 'held') AND starts_with(entity, $1)`, prefix).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting == 0 {
			return
		}
		// Due again at once, without moving the clock, whose tokens SAM
		// would see as issued in the future.
		if _, err := e.Pool.Exec(context.Background(), `UPDATE sam_outbox SET next_attempt_at = $1 WHERE state = 'pending'`, e.Clock.Now()); err != nil {
			t.Fatal(err)
		}
	}
	rows, _ := e.Pool.Query(context.Background(), `SELECT entity, state, coalesce(last_error, '') FROM sam_outbox WHERE state IN ('pending', 'held') AND starts_with(entity, $1)`, prefix)
	for rows.Next() {
		var entity, state, last string
		_ = rows.Scan(&entity, &state, &last)
		t.Logf("%s %s: %s", entity, state, last)
	}
	rows.Close()
	t.Fatal("the outbox did not empty")
}

// TestRegistryAdmin: only platform admins change the registry; a domain and
// a device type an admin adds are this tenant's own entries, shown at once
// and sent to SAM, a new value under the admin's domain as dgs:<id> in
// dgs:<CODE>; SAM's rules are kept; every change is logged.
func TestRegistryAdmin(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	ctx := context.Background()
	someone := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	for _, c := range []struct{ method, path string }{
		{"POST", "/domains"}, {"PATCH", "/domains/AIR"}, {"DELETE", "/domains/AIR"}, {"POST", "/device-types"},
		{"PATCH", "/device-types/soil_sensor"}, {"PATCH", "/parameters/uv"}, {"PUT", "/parameters/uv/chart"},
		{"PUT", "/parameters/uv/explanation"}, {"POST", "/alert-defaults"}, {"PUT", "/alert-defaults/x"},
		{"DELETE", "/alert-defaults/x"}, {"GET", "/changes"},
	} {
		if status, _ := send(t, e, someone, c.method, c.path, map[string]any{}, nil); status != http.StatusForbidden {
			t.Errorf("%s %s by someone else: %d", c.method, c.path, status)
		}
	}
	a := admin(t, e)

	var marine registry.Domain
	if status, no := send(t, e, a, "POST", "/domains", registry.DomainInput{ID: "marine", Label: "Marine", Icon: "waves", Description: "Sea values."}, &marine); status != 200 || marine.ID != "dgs:MARINE" {
		t.Fatalf("a new domain: %d %+v %+v", status, marine, no)
	}
	if status, no := send(t, e, a, "POST", "/domains", registry.DomainInput{ID: "AIR", Label: "Air", Icon: "", Description: ""}, nil); status != 409 || no.Fields["id"] == "" {
		t.Errorf("one of SAM's codes: %d %+v", status, no)
	}

	probe := registry.DeviceTypeInput{Label: "Soil probe", Description: "A probe for salts.", Color: "#a77bf3", Values: []registry.ValueInput{
		{ID: "soil_ec", Label: "Soil EC", Unit: "µS/cm", ValueType: "number", Domain: ptr("dgs:MARINE"), Tile: true},
		{ID: "soil_moisture", Label: "Soil moisture", Unit: "%", ValueType: "number"},
	}}
	var typ registry.DeviceType
	if status, no := send(t, e, a, "POST", "/device-types", probe, &typ); status != 200 || typ.ID != "dgs:soil_probe" ||
		!slices.Equal(typ.ParameterIDs, []string{"dgs:soil_ec", "soil_moisture"}) || !slices.Equal(typ.TileParameterIDs, []string{"dgs:soil_ec"}) {
		t.Fatalf("a new device type: %d %+v %+v", status, typ, no)
	}
	var params []registry.Parameter
	send(t, e, a, "GET", "/parameters", nil, &params)
	if i := slices.IndexFunc(params, func(p registry.Parameter) bool { return p.ID == "dgs:soil_ec" }); i < 0 || *params[i].Domain != "dgs:MARINE" || params[i].Unit != "µS/cm" {
		t.Fatalf("the new value before SAM has it: %+v", params)
	}

	deliver(t, e, "registry:")
	sam := e.SAM.Service()
	if d, err := sam.Domain(ctx, "dgs:MARINE"); err != nil || d.Name != "Marine" {
		t.Fatalf("the domain in SAM: %+v, %v", d, err)
	}
	if p, err := sam.Parameter(ctx, "dgs:soil_ec"); err != nil || p.DomainID != "dgs:MARINE" || p.Unit != "uS/cm" {
		t.Fatalf("the value in SAM: %+v, %v", p, err)
	}
	if dt, err := sam.DeviceType(ctx, "dgs:soil_probe"); err != nil || len(dt.Keys) != 2 || dt.Keys[0].Key != "soil_ec" || *dt.Keys[0].ParameterID != "dgs:soil_ec" {
		t.Fatalf("the device type in SAM: %+v, %v", dt, err)
	}

	// SAM's rules.
	changed := probe
	changed.Values = slices.Clone(probe.Values)
	changed.Values[0].Unit = "mS/cm"
	if status, no := send(t, e, a, "PATCH", "/device-types/dgs:soil_probe", changed, nil); status != 400 || no.Fields["values.0.unit"] == "" {
		t.Errorf("a registered unit changed: %d %+v", status, no)
	}
	changed.Values[0] = registry.ValueInput{ID: "soil_ph", Label: "Soil pH", ValueType: "number", Unit: "1"}
	if status, no := send(t, e, a, "PATCH", "/device-types/dgs:soil_probe", changed, nil); status != 400 || no.Fields["values.0.domain"] == "" {
		t.Errorf("a new value with no domain: %d %+v", status, no)
	}
	core := registry.DeviceTypeInput{Label: "Soil sensor", Description: "In the ground.", Color: "#f6921e",
		Values: []registry.ValueInput{{ID: "soil_moisture", Label: "Soil moisture", Unit: "%", ValueType: "number"}}}
	if status, _ := send(t, e, a, "PATCH", "/device-types/soil_sensor", core, nil); status != 409 {
		t.Errorf("a core type's values: %d", status)
	}
	core.Values = append(core.Values, registry.ValueInput{ID: "soil_temperature", Label: "Soil temperature", Unit: "°C", ValueType: "number", Card: true})
	if status, no := send(t, e, a, "PATCH", "/device-types/soil_sensor", core, &typ); status != 200 || typ.Description != "In the ground." ||
		!slices.Equal(typ.CardParameterIDs, []string{"soil_temperature"}) {
		t.Errorf("a core type's presentation: %d %+v %+v", status, typ, no)
	}

	// Values: placement, charts and explanations.
	var p registry.Parameter
	if status, _ := send(t, e, a, "PATCH", "/parameters/soil_moisture", map[string]any{"domain": nil}, &p); status != 200 || p.Domain != nil {
		t.Errorf("a value taken out of its domain: %d %+v", status, p)
	}
	if status, _ := send(t, e, a, "PUT", "/parameters/uv/chart", map[string]any{"chart": "line_band"}, &p); status != 200 || p.Chart != "line_band" {
		t.Errorf("a chart: %d %+v", status, p)
	}
	if status, no := send(t, e, a, "PUT", "/parameters/species_name/chart", map[string]any{"chart": "line"}, nil); status != 400 || no.Fields["chart"] == "" {
		t.Errorf("a chart for text: %d %+v", status, no)
	}
	explain := registry.Explanation{Plain: "How wet it feels.", Method: "measured", Formula: "Read off a dial.", Inputs: []string{}}
	if status, no := send(t, e, a, "PUT", "/parameters/dew_point/explanation", explain, nil); status != 400 || no.Fields["method"] == "" {
		t.Errorf("a value SAM derives, explained as measured: %d %+v", status, no)
	}
	if status, _ := send(t, e, a, "PUT", "/parameters/air_temperature/explanation", explain, &p); status != 200 || p.Plain != "How wet it feels." ||
		p.Calculation.Formula != "Read off a dial." {
		t.Errorf("an explanation: %d %+v", status, p)
	}

	// A domain is removed once no value is under it.
	if status, _ := send(t, e, a, "DELETE", "/domains/dgs:MARINE", nil, nil); status != 409 {
		t.Errorf("a domain with a value: %d", status)
	}
	if status, _ := send(t, e, a, "PATCH", "/parameters/dgs:soil_ec", map[string]any{"domain": "SOIL"}, &p); status != 200 || *p.Domain != "SOIL" {
		t.Fatalf("moving the value: %d %+v", status, p)
	}
	if status, _ := send(t, e, a, "DELETE", "/domains/dgs:MARINE", nil, nil); status != 204 {
		t.Fatalf("an empty domain: %d", status)
	}
	renamed := probe
	renamed.Values = slices.Clone(probe.Values)
	renamed.Values[0].Label, renamed.Values[0].Domain = "Salts", nil
	if status, no := send(t, e, a, "PATCH", "/device-types/dgs:soil_probe", renamed, &typ); status != 200 {
		t.Fatalf("renaming a value: %d %+v", status, no)
	}
	send(t, e, a, "GET", "/parameters", nil, &params)
	if i := slices.IndexFunc(params, func(p registry.Parameter) bool { return p.ID == "dgs:soil_ec" }); i < 0 || params[i].Label != "Salts" {
		t.Errorf("a renamed value with an overlay row: %+v", params[i])
	}
	deliver(t, e, "registry:")
	if p, err := sam.Parameter(ctx, "dgs:soil_ec"); err != nil || p.DomainID != "SOIL" {
		t.Errorf("the moved value in SAM: %+v, %v", p, err)
	}
	if _, err := sam.Domain(ctx, "dgs:MARINE"); err == nil {
		t.Error("the removed domain is still in SAM")
	}

	var changes []registry.Change
	send(t, e, a, "GET", "/changes", nil, &changes)
	if len(changes) != 9 || changes[0].ByName != "Amara Okafor" || changes[0].Summary != "Updated the Soil probe device type: edited dgs:soil_ec." ||
		changes[1].Summary != "Removed the Marine domain (dgs:MARINE)." ||
		changes[len(changes)-1].Summary != "Added the Marine domain (dgs:MARINE)." {
		t.Errorf("the log: %+v", changes)
	}
	found := slices.ContainsFunc(changes, func(c registry.Change) bool {
		return c.Summary == "Registered the Soil probe device type with 2 values (new: dgs:soil_ec)."
	})
	if !found {
		t.Errorf("the device type's entry: %+v", changes)
	}
}

// TestAlertDefaults: everyone signed in reads the default levels; admins
// add, change, turn off and remove them, each checked and logged.
func TestAlertDefaults(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	a := admin(t, e)
	var list []registry.AlertRule
	if status, _ := send(t, e, a, "GET", "/alert-defaults", nil, &list); status != 200 || len(list) != 8 || list[0].ID != "alr_default_uv_warning" {
		t.Fatalf("the seeded levels: %d %+v", status, list)
	}
	rule := map[string]any{"parameterId": "uv", "direction": "above", "value": 11, "level": "warning", "title": "Extreme UV", "enabled": true}
	if status, _ := send(t, e, a, "POST", "/alert-defaults", rule, &list); status != 200 || len(list) != 9 || list[8].Title != "Extreme UV" {
		t.Fatalf("a new level: %d %+v", status, list)
	}
	id := list[8].ID
	rule["enabled"] = false
	if status, _ := send(t, e, a, "PUT", "/alert-defaults/"+id, rule, &list); status != 200 || list[8].Enabled {
		t.Fatalf("turned off: %d %+v", status, list)
	}
	rule["parameterId"] = "air_temperature"
	if status, no := send(t, e, a, "PUT", "/alert-defaults/"+id, rule, nil); status != 400 || no.Fields["parameterId"] == "" {
		t.Errorf("moved to another value: %d %+v", status, no)
	}
	for name, bad := range map[string]map[string]any{
		"text":         {"parameterId": "species_name", "direction": "above", "value": 1, "level": "warning", "title": "x", "enabled": true},
		"out of range": {"parameterId": "uv", "direction": "above", "value": 99, "level": "warning", "title": "x", "enabled": true},
		"no title":     {"parameterId": "uv", "direction": "above", "value": 9, "level": "warning", "title": " ", "enabled": true},
	} {
		if status, _ := send(t, e, a, "POST", "/alert-defaults", bad, nil); status != 400 {
			t.Errorf("%s: %d", name, status)
		}
	}
	if status, _ := send(t, e, a, "DELETE", "/alert-defaults/"+id, nil, &list); status != 200 || len(list) != 8 {
		t.Fatalf("removed: %d %+v", status, list)
	}
	if status, _ := send(t, e, a, "DELETE", "/alert-defaults/"+id, nil, nil); status != 404 {
		t.Errorf("removed again: %d", status)
	}
	var changes []registry.Change
	send(t, e, a, "GET", "/changes", nil, &changes)
	want := []string{
		"Removed the default UV index level Warning at or above 11 (Extreme UV).",
		"Turned off the default UV index level Warning at or above 11 (Extreme UV).",
		"Added the default UV index level Warning at or above 11 (Extreme UV).",
	}
	if len(changes) != 3 {
		t.Fatalf("the log: %+v", changes)
	}
	for i, w := range want {
		if changes[i].Summary != w {
			t.Errorf("change %d: %q, want %q", i, changes[i].Summary, w)
		}
	}
	someone := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	if status, _ := send(t, e, someone, "GET", "/alert-defaults", nil, &list); status != 200 || len(list) != 8 {
		t.Errorf("read by someone else: %d", status)
	}
	if !strings.HasPrefix(id, "alr_") {
		t.Errorf("a level's ID: %q", id)
	}
}

func ptr[T any](v T) *T { return &v }
