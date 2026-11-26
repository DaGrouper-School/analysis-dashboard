package apitest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/labs"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// TestOfflineSync: a batch synced 30 days late from a clock 10 minutes
// off is stored, each record with its own result and flagged; a retried
// batch is stored once; a record SAM stored whose answer was lost comes
// back as a duplicate; a record dated after the batch was sent is
// flagged; a refused record is the record's result alone.
func TestOfflineSync(t *testing.T) {
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
		raw := ""
		if body != nil {
			b, _ := json.Marshal(body)
			raw = string(b)
		}
		res, got := e.DoWith(t, p.client, method, "/api"+path, raw)
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
	a := admin(t, e)
	head := signedIn(t, e, apitest.Person{Sub: "g-hana", Email: "hana@trinity.example", Name: "Hana Bekele"})
	if _, err := accounts.SetMinor(ctx, e.Pool, now, uuid.MustParse(head.id), false); err != nil {
		t.Fatal(err)
	}
	var c classrooms.Summary
	call(a, "POST", "/classrooms", map[string]any{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "hana@trinity.example"}, &c)
	call(head, "POST", "/invitations/by-token/"+tokenFor(t, e, "hana@trinity.example")+"/accept", nil, nil)
	var lab labs.Lab
	call(head, "POST", "/classrooms/"+c.ID+"/labs", map[string]any{"name": "Trinity garden", "placeName": "Trinity School", "country": "Ethiopia",
		"location": map[string]any{"lat": 9.03, "lng": 38.74}, "siteProfile": map[string]any{"setting": "urban", "description": "A garden behind the school.",
			"researchQuestions": []string{"How much rain falls?"}}}, &lab)
	var gauge labs.DeviceView
	call(head, "POST", "/labs/"+lab.ID.String()+"/devices", map[string]any{"name": "Rain gauge", "type": "manual_rain_gauge", "identifier": ""}, &gauge)
	deliver(t, e, "")

	// The device's clock runs 10 minutes slow; the records are 30 days old.
	sentAt := time.Now().Add(-10 * time.Minute).UTC().Truncate(time.Second)
	old := sentAt.Add(-30 * 24 * time.Hour)
	rain := func(day int, amount float64) map[string]any {
		return map[string]any{"templateId": "rain_gauge_reading", "labId": lab.ID, "deviceId": gauge.ID,
			"observedAt": old.Add(time.Duration(day) * 24 * time.Hour).Format(time.RFC3339), "values": map[string]any{"rain_gauge_daily_accumulation": amount}, "location": nil}
	}
	records := []map[string]any{
		{"clientId": "c1", "credit": false, "observation": rain(0, 3)},
		{"clientId": "c2", "credit": false, "observation": rain(1, 4)},
		{"clientId": "c3", "credit": false, "observation": rain(2, 600)},
		{"clientId": "c4", "credit": false, "observation": map[string]any{"templateId": "species_sighting", "labId": uuid.New(), "deviceId": nil,
			"observedAt": old.Format(time.RFC3339), "values": map[string]any{"species_name": "Ibis", "species_count": 2}, "location": nil}},
	}
	batch := map[string]any{"sentAt": sentAt.Format(time.RFC3339), "records": records}
	var first labs.SyncAnswer
	if status, no := call(head, "POST", "/observations/batch", batch, &first); status != 200 || len(first.Results) != 4 {
		t.Fatalf("synced: %d %+v %+v", status, first, no)
	}
	if first.ClockOffsetMs < 9*60_000 || first.ClockOffsetMs > 11*60_000 {
		t.Errorf("the clock offset: %v", first.ClockOffsetMs)
	}
	outcomes := func(a labs.SyncAnswer) []string {
		out := make([]string, len(a.Results))
		for i, r := range a.Results {
			out[i] = r.ClientID + ":" + r.Outcome
		}
		return out
	}
	if got := outcomes(first); !slices.Equal(got, []string{"c1:stored", "c2:stored", "c3:rejected", "c4:rejected"}) {
		t.Fatalf("outcomes: %v", got)
	}
	r1, r3 := first.Results[0], first.Results[2]
	if r1.Observation == nil || !slices.Equal(r1.ClockFlags, []string{labs.FlagClockOff}) || r3.Error == nil ||
		r3.Fields["rain_gauge_daily_accumulation"] == "" || r3.Observation != nil {
		t.Errorf("the results: %+v", first.Results)
	}

	// Retried: the kept answers come back.
	var again labs.SyncAnswer
	call(head, "POST", "/observations/batch", batch, &again)
	if got := outcomes(again); !slices.Equal(got, []string{"c1:duplicate", "c2:duplicate", "c3:rejected", "c4:rejected"}) ||
		again.Results[0].Observation.ID != r1.Observation.ID {
		t.Errorf("retried: %v %+v", got, again.Results)
	}
	// The answer was lost: SAM answers it as a duplicate, under the same ID.
	if _, err := e.Pool.Exec(ctx, `DELETE FROM synced_records WHERE client_id = 'c1'`); err != nil {
		t.Fatal(err)
	}
	call(head, "POST", "/observations/batch", map[string]any{"sentAt": sentAt.Format(time.RFC3339), "records": records[:1]}, &again)
	if r := again.Results[0]; r.Outcome != labs.SyncDuplicate || r.Observation == nil || r.Observation.ID != r1.Observation.ID {
		t.Errorf("a lost answer: %+v", r)
	}
	var history []labs.Reading
	if call(head, "GET", "/devices/"+gauge.ID.String()+"/readings", nil, &history); len(history) != 2 {
		t.Errorf("the gauge's readings: %+v", history)
	}

	// A record dated after the batch was sent, from a good clock.
	late := rain(0, 5)
	now = time.Now().UTC().Truncate(time.Second)
	late["observedAt"] = now.Add(time.Hour).Format(time.RFC3339)
	var flagged labs.SyncAnswer
	call(head, "POST", "/observations/batch", map[string]any{"sentAt": now.Format(time.RFC3339), "records": []map[string]any{{"clientId": "c5", "credit": false, "observation": late}}}, &flagged)
	if r := flagged.Results[0]; r.Outcome != labs.SyncStored || !slices.Equal(r.ClockFlags, []string{labs.FlagAfterSent}) {
		t.Errorf("dated after sending: %+v", r)
	}

	many := make([]map[string]any, labs.SyncLimit+1)
	for i := range many {
		many[i] = map[string]any{"clientId": fmt.Sprintf("m%d", i), "credit": false, "observation": rain(0, 1)}
	}
	if status, _ := call(head, "POST", "/observations/batch", map[string]any{"sentAt": now.Format(time.RFC3339), "records": many}, nil); status != 400 {
		t.Errorf("too many records: %d", status)
	}
}
