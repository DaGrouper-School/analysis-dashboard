package apitest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/hosting"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings/settingsapi"
)

// TestConfigurationAdmin: only platform admins keep the stages, regions,
// questions and settings, each change checked, logged with their name and
// shown at once; the log is read a page at a time.
func TestConfigurationAdmin(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	ctx := context.Background()
	now := time.Now()
	if _, err := settings.SeedSettings(ctx, e.Pool, now); err != nil {
		t.Fatal(err)
	}
	if _, err := classrooms.SeedLists(ctx, e.Pool, now); err != nil {
		t.Fatal(err)
	}
	if _, err := hosting.SeedQuestions(ctx, e.Pool, now); err != nil {
		t.Fatal(err)
	}
	// The seed logged 13 changes; these 50 put them on a second page.
	for i := range settingsapi.ChangesPage {
		if err := settings.Record(ctx, e.Pool, now, settings.Operator, settings.Change{Kind: settings.KindStage, Subject: "pilot",
			Summary: "Changed stage pilot " + strconv.Itoa(i)}); err != nil {
			t.Fatal(err)
		}
	}
	someone := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	for _, c := range []struct{ method, path string }{
		{"GET", "/admin/stages"}, {"PUT", "/admin/stages/x"}, {"DELETE", "/admin/stages/x"},
		{"GET", "/admin/regions"}, {"PUT", "/admin/regions/x"}, {"DELETE", "/admin/regions/x"},
		{"GET", "/admin/host-questions"}, {"PUT", "/admin/host-questions/x"}, {"POST", "/admin/host-questions/x/retire"},
		{"GET", "/admin/settings"}, {"PUT", "/admin/settings/privacy"}, {"GET", "/admin/configuration-changes"},
	} {
		if res, _ := e.DoWith(t, someone.client, c.method, "/api"+c.path, "{}"); res.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s by someone else: %d", c.method, c.path, res.StatusCode)
		}
	}
	a := admin(t, e)
	do := func(method, path string, body, out any) (int, refusal) {
		t.Helper()
		raw := ""
		if body != nil {
			b, _ := json.Marshal(body)
			raw = string(b)
		}
		res, got := e.DoWith(t, a.client, method, "/api/admin"+path, raw)
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

	var stages []classrooms.Stage
	if status, no := do("PUT", "/stages/planning", map[string]any{"label": "Planning", "description": "Finding a place.", "color": "#a77bf3", "position": 0}, &stages); status != 200 ||
		len(stages) != 4 || stages[0].ID != "planning" {
		t.Fatalf("a stage added: %d %+v %+v", status, stages, no)
	}
	if status, no := do("PUT", "/stages/planning", map[string]any{"label": "", "description": "d", "color": "blue", "position": 0}, nil); status != 400 ||
		no.Fields["label"] != "blank" || no.Fields["color"] == "" {
		t.Errorf("a bad stage: %d %+v", status, no)
	}
	if status, _ := do("DELETE", "/stages/planning", nil, nil); status != 204 {
		t.Errorf("a stage deleted: %d", status)
	}
	if status, _ := do("DELETE", "/stages/planning", nil, nil); status != 404 {
		t.Errorf("deleted again: %d", status)
	}
	var regions []classrooms.Region
	if status, _ := do("PUT", "/regions/south_asia", map[string]any{"label": "South Asia", "position": 9}, &regions); status != 200 || regions[len(regions)-1].ID != "south_asia" {
		t.Errorf("a region added: %d %+v", status, regions)
	}
	var questions []hosting.KeptQuestion
	if status, _ := do("POST", "/host-questions/notes/retire", nil, &questions); status != 200 || questions[len(questions)-1].RetiredAt == nil {
		t.Errorf("a question retired: %d %+v", status, questions)
	}
	if status, _ := do("PUT", "/host-questions/notes", map[string]any{"label": "Notes", "help": "", "kind": "text", "required": false, "position": 0}, nil); status != 409 {
		t.Errorf("a retired question changed: %d", status)
	}
	if status, _ := do("PUT", "/host-questions/power", map[string]any{"label": "Is there power?", "help": "", "kind": "yes_no", "required": true, "position": 0}, &questions); status != 200 ||
		questions[0].ID != "power" {
		t.Errorf("a question added: %d %+v", status, questions)
	}

	var setting settingsapi.SettingView
	if status, no := do("PUT", "/settings/privacy", map[string]any{"minorCoordinateDecimals": 9}, nil); status != 400 || no.Fields["minorCoordinateDecimals"] == "" {
		t.Errorf("a bad setting: %d %+v", status, no)
	}
	if status, _ := do("PUT", "/settings/theme", map[string]any{}, nil); status != 404 {
		t.Errorf("an unknown setting: %d", status)
	}
	if status, _ := do("PUT", "/settings/privacy", map[string]any{"minorCoordinateDecimals": 1}, &setting); status != 200 || setting.Seeded ||
		string(setting.Value) != `{"minorCoordinateDecimals":1}` {
		t.Errorf("a setting: %d %+v", status, setting)
	}
	var all []settingsapi.SettingView
	if status, _ := do("GET", "/settings", nil, &all); status != 200 || len(all) != 5 || all[2].Key != "privacy" || all[3].Key != "future_observations" || all[4].Key != "now_tiles" || all[0].Seeded != true {
		t.Errorf("the settings: %d %+v", status, all)
	}

	var page settingsapi.ChangesView
	if status, _ := do("GET", "/configuration-changes", nil, &page); status != 200 || page.Items[0].Summary != "Changed privacy" ||
		page.Items[0].ByName != "Amara Okafor" || page.Next == nil || len(page.Items) != settingsapi.ChangesPage {
		t.Fatalf("the newest changes: %d %+v", status, page)
	}
	var older settingsapi.ChangesView
	if status, _ := do("GET", "/configuration-changes?before="+*page.Next, nil, &older); status != 200 || older.Next != nil ||
		len(older.Items) != 22 || older.Items[21].Subject != "core_parameters" || older.Items[0].ID != strconv.Itoa(mustAtoi(t, *page.Next)-1) {
		t.Errorf("the page before: %d %+v", status, older)
	}
	if status, _ := do("GET", "/configuration-changes?before=x", nil, nil); status != 400 {
		t.Errorf("a bad cursor: %d", status)
	}
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
