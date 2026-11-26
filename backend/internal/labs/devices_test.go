package labs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

func TestCheckDevice(t *testing.T) {
	t.Parallel()
	if f := CheckDevice(DeviceInput{Name: "Rain gauge", Identifier: "rg-01.a_b"}, false); len(f) != 0 {
		t.Errorf("a good device: %v", f)
	}
	if f := CheckDevice(DeviceInput{Name: "Rain gauge"}, true); len(f) != 0 {
		t.Errorf("a device read by hand needs no identifier: %v", f)
	}
	f := CheckDevice(DeviceInput{Name: " ab ", Identifier: "-bad"}, false)
	if f["name"] == "" || f["identifier"] == "" {
		t.Errorf("bad: %v", f)
	}
	if got := SourceID("DGC-HG2W5K", "RG-01"); got != "DGC-HG2W5K:RG-01" {
		t.Errorf("a source ID: %s", got)
	}
}

// TestDevices: identifiers are upper-cased and unique among a classroom's
// live devices; devices read by hand are numbered; a removed device frees
// its identifier and its retirement takes access away.
func TestDevices(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.DB(t)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	if _, err := settings.SeedSettings(ctx, pool, now); err != nil {
		t.Fatal(err)
	}
	if _, err := classrooms.SeedLists(ctx, pool, now); err != nil {
		t.Fatal(err)
	}
	by := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, created_at, updated_at) VALUES ($1, now(), now())`, by); err != nil {
		t.Fatal(err)
	}
	c, err := classrooms.Create(ctx, pool, now, uuid.Nil, classrooms.CreateInput{Name: "Trinity Weather", InstitutionName: "Trinity", HeadEmail: "h@t.example"})
	if err != nil {
		t.Fatal(err)
	}
	garden, _ := Create(ctx, pool, now, c.ID, c.Code, good())
	roof, _ := Create(ctx, pool, now, c.ID, c.Code, good())
	d, err := RegisterDevice(ctx, pool, now, garden, by, DeviceInput{Name: " Rain gauge ", Type: "dgs:rain", Identifier: " rg-01 "}, false)
	if err != nil || d.Identifier != "RG-01" || d.Name != "Rain gauge" {
		t.Fatalf("a device: %+v, %v", d, err)
	}
	if _, err := RegisterDevice(ctx, pool, now, roof, by, DeviceInput{Name: "Another", Type: "dgs:rain", Identifier: "RG-01"}, false); !errors.Is(err, ErrIdentifierTaken) {
		t.Errorf("a taken identifier in another lab: %v", err)
	}
	for i, want := range []string{"MAN-0001", "MAN-0002"} {
		m, err := RegisterDevice(ctx, pool, now, roof, by, DeviceInput{Name: "Thermometer", Type: "dgs:manual", Identifier: "ignored"}, true)
		if err != nil || m.Identifier != want {
			t.Errorf("manual device %d: %+v, %v", i, m, err)
		}
	}
	if list, _ := ClassroomDevices(ctx, pool, c.ID); len(list) != 3 {
		t.Errorf("the classroom's devices: %+v", list)
	}
	if list, _ := LabDevices(ctx, pool, garden.ID); len(list) != 1 || list[0].ID != d.ID {
		t.Errorf("the garden's devices: %+v", list)
	}

	if err := RemoveDevice(ctx, pool, now, d, by); err != nil {
		t.Fatal(err)
	}
	if got, _ := GetDevice(ctx, pool, d.ID); !got.Removed {
		t.Error("not removed")
	}
	var takesAway bool
	if err := pool.QueryRow(ctx, `SELECT takes_access_away FROM sam_outbox WHERE entity = $1 ORDER BY version DESC LIMIT 1`, deviceEntity(d.ID)).
		Scan(&takesAway); err != nil || !takesAway {
		t.Errorf("a retirement takes access away: %v, %v", takesAway, err)
	}
	if _, err := RegisterDevice(ctx, pool, now, roof, by, DeviceInput{Name: "Rain gauge", Type: "dgs:rain", Identifier: "RG-01"}, false); err != nil {
		t.Errorf("a removed device's identifier again: %v", err)
	}
	if _, err := GetDevice(ctx, pool, uuid.New()); !errors.Is(err, ErrNoDevice) {
		t.Errorf("no device: %v", err)
	}
}
