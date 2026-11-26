package apitest_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/labs"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// TestDevices: a classroom's head registers a device in a lab, which SAM
// then holds in the lab's space with its type and identifier, no key and
// policy flag, registered by the head once SAM holds their membership; a
// platform admin registers a device read by hand, numbered; a taken
// identifier, an unknown type and those who do not manage devices are
// refused; removing a device retires it in SAM as the one who removed it.
func TestDevices(t *testing.T) {
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
	stranger := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	operator := signedIn(t, e, apitest.Person{Sub: "g-amina", Email: "amina@dagrouper.example", Name: "Amina Yusuf"})
	if _, _, err := accounts.SetAdmin(ctx, e.Pool, now, settings.Operator, operator.id, true); err != nil {
		t.Fatal(err)
	}
	var c classrooms.Summary
	if status, _ := call(a, "POST", "/classrooms", map[string]any{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "hana@trinity.example"}, &c); status != 200 {
		t.Fatalf("a classroom: %d", status)
	}
	if status, _ := call(head, "POST", "/invitations/by-token/"+tokenFor(t, e, "hana@trinity.example")+"/accept", nil, nil); status != 200 {
		t.Fatalf("the head joins: %d", status)
	}
	var lab labs.Lab
	if status, no := call(head, "POST", "/classrooms/"+c.ID+"/labs", map[string]any{"name": "Trinity garden", "placeName": "Trinity School", "country": "Ethiopia",
		"location": map[string]any{"lat": 9.03, "lng": 38.74}, "siteProfile": map[string]any{"setting": "urban", "description": "A garden behind the school.",
			"researchQuestions": []string{"How much rain falls?"}}}, &lab); status != 200 {
		t.Fatalf("a lab: %d %+v", status, no)
	}
	path := "/labs/" + lab.ID.String() + "/devices"

	var station labs.DeviceView
	if status, no := call(head, "POST", path, map[string]any{"name": "Pilot station", "type": "sen0658", "identifier": "pilot-01"}, &station); status != 200 ||
		station.Identifier != "PILOT-01" || station.SAMSourceID == nil || *station.SAMSourceID != *c.Code+":PILOT-01" || station.LabName != "Trinity garden" {
		t.Fatalf("a device: %d %+v %+v", status, station, no)
	}
	if status, no := call(head, "POST", path, map[string]any{"name": "Again", "type": "sen0658", "identifier": "PILOT-01"}, nil); status != 409 || no.Fields["identifier"] == "" {
		t.Errorf("a taken identifier: %d %+v", status, no)
	}
	if status, no := call(head, "POST", path, map[string]any{"name": "A toaster", "type": "toaster", "identifier": "T-1"}, nil); status != 400 || no.Fields["type"] == "" {
		t.Errorf("an unknown type: %d %+v", status, no)
	}
	if status, no := call(head, "POST", path, map[string]any{"name": "x", "type": "sen0658", "identifier": "-"}, nil); status != 400 ||
		no.Fields["name"] == "" || no.Fields["identifier"] == "" {
		t.Errorf("a bad device: %d %+v", status, no)
	}
	if status, _ := call(stranger, "POST", path, map[string]any{"name": "Mine", "type": "sen0658", "identifier": "K-1"}, nil); status != 404 {
		t.Errorf("a device in someone else's lab: %d", status)
	}
	var gauge labs.DeviceView
	if status, no := call(operator, "POST", path, map[string]any{"name": "Rain gauge", "type": "manual_rain_gauge", "identifier": ""}, &gauge); status != 200 || gauge.Identifier != "MAN-0001" {
		t.Fatalf("a device read by hand: %d %+v %+v", status, gauge, no)
	}

	// SAM holds them once it holds the lab's space and each registrant.
	deliver(t, e, "")
	svc := e.SAM.Service()
	stored, _ := labs.Get(ctx, e.Pool, lab.ID)
	for _, w := range []struct {
		name string
		d    labs.DeviceView
		by   *profile
		typ  string
	}{{"the station", station, head, "sen0658"}, {"the gauge", gauge, operator, "manual_rain_gauge"}} {
		d, _ := labs.GetDevice(ctx, e.Pool, w.d.ID)
		if d.SAMDeviceID == nil {
			t.Fatalf("%s is not in SAM", w.name)
		}
		got, err := svc.Device(ctx, *d.SAMDeviceID)
		by, _ := accounts.Get(ctx, e.Pool, uuid.MustParse(w.by.id))
		if err != nil || got.SpaceID != *stored.SpaceID || got.DeviceTypeID != w.typ || got.Identifier != w.d.Identifier || got.Policy != "flag" ||
			got.PublicKey != nil || got.RegisteredBy != *by.Principal {
			t.Errorf("%s in SAM: %+v, %v", w.name, got, err)
		}
	}

	var list []labs.DeviceView
	if status, _ := call(head, "GET", "/classrooms/"+c.ID+"/devices", nil, &list); status != 200 || len(list) != 2 {
		t.Errorf("the classroom's devices: %d %+v", status, list)
	}
	var detail labs.Detail
	if call(head, "GET", "/labs/"+lab.ID.String(), nil, &detail); len(detail.Devices) != 2 {
		t.Errorf("the lab's devices: %+v", detail.Devices)
	}
	if status, _ := call(stranger, "DELETE", "/devices/"+station.ID.String(), nil, nil); status != 404 {
		t.Errorf("removed by someone else: %d", status)
	}

	// Removed here at once, then retired in SAM, which keeps its readings.
	if status, _ := call(head, "DELETE", "/devices/"+station.ID.String(), nil, nil); status != 204 {
		t.Fatalf("removed: %d", status)
	}
	if status, _ := call(head, "DELETE", "/devices/"+station.ID.String(), nil, nil); status != 404 {
		t.Errorf("removed twice: %d", status)
	}
	if call(head, "GET", "/classrooms/"+c.ID+"/devices", nil, &list); len(list) != 1 || list[0].ID != gauge.ID {
		t.Errorf("the devices after removing one: %+v", list)
	}
	deliver(t, e, "device:")
	d, _ := labs.GetDevice(ctx, e.Pool, station.ID)
	if got, err := svc.Device(ctx, *d.SAMDeviceID); err != nil || got.RetiredAt == nil {
		t.Errorf("the station in SAM: %+v, %v", got, err)
	}
}
