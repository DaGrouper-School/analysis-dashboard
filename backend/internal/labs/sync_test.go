package labs

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSyncID(t *testing.T) {
	t.Parallel()
	u := uuid.New()
	first := SyncID(u, "c1")
	if first != SyncID(u, "c1") || first == SyncID(u, "c2") || first == SyncID(uuid.New(), "c1") {
		t.Error("a synced record's ID is the person's and the client's")
	}
}

func TestClockFlags(t *testing.T) {
	t.Parallel()
	sent := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	at := func(d time.Duration) string { return sent.Add(d).Format(time.RFC3339) }
	for _, c := range []struct {
		off    bool
		at     string
		sentOK bool
		want   []string
	}{
		{false, at(-time.Hour), true, []string{}},
		{true, at(-time.Hour), true, []string{FlagClockOff}},
		{false, at(time.Minute), true, []string{}},
		{false, at(time.Hour), true, []string{FlagAfterSent}},
		{true, at(time.Hour), false, []string{FlagClockOff}},
		{false, "never", true, []string{}},
	} {
		if got := ClockFlags(c.off, c.at, sent, c.sentOK); !slices.Equal(got, c.want) {
			t.Errorf("%+v: %v", c, got)
		}
	}
}
