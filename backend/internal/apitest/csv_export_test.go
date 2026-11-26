package apitest_test

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/labs"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// TestCSVExport: the raw table's values come from SAM's export as CSV in
// the front end's columns, each row with the classroom's data policy as
// SAM holds it and a link to who recorded it; the filters narrow it and
// name the file; a removed device's readings stay in it; only members
// export.
func TestCSVExport(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	e.RunBackground()
	ctx := context.Background()
	now := time.Now()
	if _, err := settings.SeedSettings(ctx, e.Pool, now); err != nil {
		t.Fatal(err)
	}
	if _, err := classrooms.SeedLists(ctx, e.Pool, now); err != nil {
		t.Fatal(err)
	}
	call := func(p *profile, method, path string, body, out any) (int, refusal) {
		t.Helper()
		b, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(ctx, method, e.URL+"/api"+path, strings.NewReader(string(b)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", apitest.FrontEnd)
		res, err := p.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		got, _ := io.ReadAll(res.Body)
		var no refusal
		if res.StatusCode >= 400 {
			_ = json.Unmarshal(got, &no)
		} else if out != nil {
			if err := json.Unmarshal(got, out); err != nil {
				t.Fatalf("%s %s: %s", method, path, got)
			}
		}
		return res.StatusCode, no
	}
	a := admin(t, e)
	head := signedIn(t, e, apitest.Person{Sub: "g-hana", Email: "hana@trinity.example", Name: "Hana Bekele"})
	stranger := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	if _, err := accounts.SetMinor(ctx, e.Pool, now, uuid.MustParse(head.id), false); err != nil {
		t.Fatal(err)
	}
	var c classrooms.Summary
	call(a, "POST", "/classrooms", map[string]any{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "hana@trinity.example"}, &c)
	call(head, "POST", "/invitations/by-token/"+tokenFor(t, e, "hana@trinity.example")+"/accept", nil, nil)
	var garden labs.Lab
	call(head, "POST", "/classrooms/"+c.ID+"/labs", map[string]any{"name": "Trinity garden", "placeName": "Trinity School", "country": "Ethiopia",
		"location": map[string]any{"lat": 9.03, "lng": 38.74}, "siteProfile": map[string]any{"setting": "urban", "description": "A place at the school.",
			"researchQuestions": []string{"How much rain falls?"}}}, &garden)
	var station labs.DeviceView
	call(head, "POST", "/labs/"+garden.ID.String()+"/devices", map[string]any{"name": "Pilot station", "type": "sen0658", "identifier": "pilot-01"}, &station)
	terms := map[string]any{"licence": "CC BY-NC 4.0", "licenceUrl": "https://creativecommons.org/licenses/by-nc/4.0/",
		"attribution": "=Trinity Weather, Trinity School", "restrictions": []string{"No commercial use."}, "coordinateDecimals": nil}
	sharing := map[string]any{"showPublic": false, "review": "manual", "autoReleaseDays": 2, "releasers": "head", "facilitatorsInvite": true,
		"facilitatorsManageDevices": true}
	if status, no := call(head, "PUT", "/classrooms/"+c.ID+"/sharing", map[string]any{"sharing": sharing, "terms": terms}, nil); status != 200 {
		t.Fatalf("the terms: %d %+v", status, no)
	}
	deliver(t, e, "")

	base := time.Now().Add(-72 * time.Hour).UTC().Truncate(time.Hour)
	var readings []map[string]any
	for i := range 3 {
		readings = append(readings, map[string]any{"observedAt": base.Add(time.Duration(i) * 24 * time.Hour).Format(time.RFC3339),
			"values": map[string]any{"air_temperature": 20.5 + float64(i)}, "location": nil})
	}
	if status, no := call(head, "POST", "/devices/"+station.ID.String()+"/readings", map[string]any{"credit": false, "readings": readings}, nil); status != 200 {
		t.Fatalf("readings: %d %+v", status, no)
	}
	if status, no := call(head, "POST", "/labs/"+garden.ID.String()+"/observations", map[string]any{"credit": false, "templateId": "species_sighting",
		"labId": garden.ID, "deviceId": nil, "observedAt": base.Add(12 * time.Hour).Format(time.RFC3339),
		"values": map[string]any{"species_name": "Ibis", "species_count": 2}, "location": map[string]any{"lat": 9.031234, "lng": 38.741234}}, nil); status != 200 {
		t.Fatalf("a sighting: %d %+v", status, no)
	}

	export := func(q map[string]any) (labs.CSVExport, [][]string) {
		t.Helper()
		body := map[string]any{"classroomId": c.ID}
		for k, v := range q {
			body[k] = v
		}
		var out labs.CSVExport
		if status, no := call(head, "POST", "/exports/raw.csv", body, &out); status != 200 {
			t.Fatalf("export %v: %d %+v", q, status, no)
		}
		rows, err := csv.NewReader(strings.NewReader(out.Content)).ReadAll()
		if err != nil || len(rows) != out.Rows+1 || !slices.Equal(rows[0], labs.CSVColumns) || !strings.HasSuffix(out.Content, "\r\n") {
			t.Fatalf("export %v: %+v, %v", q, out, err)
		}
		return out, rows[1:]
	}
	all, rows := export(nil)
	if all.Rows != 5 || !strings.HasPrefix(all.Filename, "dagrouper-trinity-weather-all-sources-") || !strings.HasSuffix(all.Filename, ".csv") {
		t.Fatalf("every value: %+v", all)
	}
	col := func(row []string, name string) string { return row[slices.Index(labs.CSVColumns, name)] }
	link := apitest.FrontEnd + "/contributors/" + strconv.FormatInt(head.pid, 10)
	for _, row := range rows {
		if col(row, "licence") != "CC BY-NC 4.0" || col(row, "attribution") != "'=Trinity Weather, Trinity School" ||
			col(row, "restrictions") != "No commercial use." || col(row, "classroom") != "Trinity Weather" || col(row, "site") != "Trinity garden" ||
			col(row, "participant_id") != strconv.FormatInt(head.pid, 10) || col(row, "participant_url") != link {
			t.Errorf("a row without its policy or recorder: %v", row)
		}
	}
	if first := rows[0]; col(first, "parameter_id") != "air_temperature" || col(first, "value") != "20.5" || col(first, "unit") == "" ||
		col(first, "parameter") == "air_temperature" || col(first, "source") != "Pilot station" || col(first, "source_type") != "device" {
		t.Errorf("the oldest row: %v", first)
	}

	one, _ := export(map[string]any{"sourceId": station.ID.String()})
	if one.Rows != 3 || !strings.Contains(one.Filename, "-pilot-station-") {
		t.Errorf("the station's: %+v", one)
	}
	if field, _ := export(map[string]any{"sourceId": "field", "parameterId": "species_count"}); field.Rows != 1 ||
		!strings.Contains(field.Filename, "-field-observation-") {
		t.Errorf("a field parameter: %+v", field)
	}
	if status, no := call(head, "POST", "/exports/raw.csv", map[string]any{"classroomId": c.ID, "from": "yesterday"}, nil); status != 400 || no.Fields["from"] == "" {
		t.Errorf("a bad day: %d %+v", status, no)
	}

	if status, _ := call(head, "DELETE", "/devices/"+station.ID.String(), nil, nil); status != 204 {
		t.Fatalf("removing the station: %d", status)
	}
	deliver(t, e, "")
	if again, kept := export(map[string]any{"sourceId": station.ID.String()}); again.Rows != 3 || col(kept[0], "source") != "Pilot station" {
		t.Errorf("a removed station's readings: %+v", again)
	}
	for name, p := range map[string]*profile{"a platform admin": a, "a stranger": stranger} {
		if status, _ := call(p, "POST", "/exports/raw.csv", map[string]any{"classroomId": c.ID}, nil); status != 404 {
			t.Errorf("%s: %d", name, status)
		}
	}
}
