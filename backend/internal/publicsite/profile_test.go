package publicsite

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
)

func TestShownPerson(t *testing.T) {
	t.Parallel()
	yes, no := true, false
	for _, c := range []struct {
		u    accounts.User
		want bool
	}{
		{accounts.User{PublicProfile: true, Minor: &no}, true},
		{accounts.User{PublicProfile: false, Minor: &no}, false},
		{accounts.User{PublicProfile: true, Minor: &yes}, false},
		{accounts.User{PublicProfile: true}, false},
	} {
		if got := shownPerson(c.u); got != c.want {
			t.Errorf("%+v: %v", c.u, got)
		}
	}
}

func TestUpcoming(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	got := upcoming([]classrooms.Session{{ID: "late", StartsAt: "2026-12-03T09:00:00.000Z"}, {ID: "past", StartsAt: "2026-11-20T09:00:00Z"},
		{ID: "soon", StartsAt: "2026-11-27T09:00:00+03:00"}, {ID: "bad", StartsAt: "tomorrow"}}, now)
	if len(got) != 2 || got[0].ID != "soon" || got[1].ID != "late" {
		t.Errorf("upcoming: %+v", got)
	}
}

func TestChunks(t *testing.T) {
	t.Parallel()
	ids := make([]uuid.UUID, 2*perRequest+1)
	got := chunks(ids)
	if len(got) != 3 || len(got[0]) != perRequest || len(got[2]) != 1 {
		t.Errorf("chunks: %d", len(got))
	}
	if chunks(nil) != nil {
		t.Error("no readings")
	}
}
