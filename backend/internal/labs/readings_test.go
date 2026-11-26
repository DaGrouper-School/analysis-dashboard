package labs

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

func TestCheckReading(t *testing.T) {
	t.Parallel()
	good := ReadingInput{ObservedAt: "2026-11-26T09:00:00Z", Values: map[string]any{"air_temperature": 21.5}, Location: &Point{Lat: 9.03, Lng: 38.74}}
	if f := CheckReading(good); len(f) != 0 {
		t.Errorf("good: %v", f)
	}
	f := CheckReading(ReadingInput{ObservedAt: "yesterday", Location: &Point{Lat: 91, Lng: math.NaN()}})
	if f["observedAt"] != "Enter when it was measured." || f["values"] != "A reading needs at least one value." ||
		f["location"] != "Enter a valid latitude and longitude, or leave both blank." {
		t.Errorf("bad: %v", f)
	}
}

func TestCheckFuture(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		ahead  time.Duration
		action string
		want   string
	}{
		{time.Minute, settings.FutureRefuse, ""},
		{10 * time.Minute, settings.FutureFlag, ""},
		{10 * time.Minute, settings.FutureRefuse, "It is dated in the future."},
		{25 * time.Hour, settings.FutureFlag, "It is dated more than a day ahead."},
		{-48 * time.Hour, settings.FutureRefuse, ""},
	} {
		if got := CheckFuture(now.Add(c.ahead), now, c.action); got != c.want {
			t.Errorf("%v ahead, %s: %q", c.ahead, c.action, got)
		}
	}
}

func TestContributionBlock(t *testing.T) {
	t.Parallel()
	yes, no := true, false
	given, pending, withdrawn := accounts.ConsentGiven, accounts.ConsentPending, accounts.ConsentWithdrawn
	for _, c := range []struct {
		u    accounts.User
		want string
	}{
		{accounts.User{Minor: &no}, ""},
		{accounts.User{}, ""},
		{accounts.User{Minor: &yes, Consent: &given}, ""},
		{accounts.User{Minor: &yes, Consent: &pending}, msgNoConsent},
		{accounts.User{Minor: &yes}, msgNoConsent},
		{accounts.User{Minor: &yes, Consent: &withdrawn}, msgWithdrawn},
	} {
		if got := ContributionBlock(c.u); got != c.want {
			t.Errorf("%+v: %q", c.u, got)
		}
	}
}

func TestRoundAndSeq(t *testing.T) {
	t.Parallel()
	if p := Round(Point{Lat: 9.03456, Lng: -38.74521}, 2); p.Lat != 9.03 || p.Lng != -38.75 {
		t.Errorf("rounded: %+v", p)
	}
	at := time.Date(2026, 11, 26, 9, 0, 0, 123e6, time.UTC)
	if Seq(at) != at.UnixMilli() || Seq(at) != Seq(at.In(time.FixedZone("EAT", 3*3600))) {
		t.Error("a reading's sequence number is its time in milliseconds")
	}
}

func TestEntryID(t *testing.T) {
	t.Parallel()
	u, lab := uuid.New(), uuid.New()
	at := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	a := EntryID(u, "rain_gauge_reading", lab, nil, at)
	if a != EntryID(u, "rain_gauge_reading", lab, nil, at.In(time.FixedZone("EAT", 3*3600))) {
		t.Error("the same entry has another ID")
	}
	d := uuid.New()
	if a == EntryID(u, "rain_gauge_reading", lab, &d, at) || a == EntryID(uuid.New(), "rain_gauge_reading", lab, nil, at) || a == EntryID(u, "rain_gauge_reading", lab, nil, at.Add(time.Second)) {
		t.Error("another entry has the same ID")
	}
}
