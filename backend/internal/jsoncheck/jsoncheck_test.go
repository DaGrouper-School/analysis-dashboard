package jsoncheck

import (
	"slices"
	"strings"
	"testing"
)

type inner struct {
	Name string `json:"name"`
}

type shape struct {
	Count  int     `json:"count"`
	Note   *string `json:"note"`
	Items  []inner `json:"items"`
	Nested inner   `json:"nested"`
}

func TestDecode(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, raw string
		want      []string
	}{
		{"valid", `{"count": 1, "note": null, "items": [], "nested": {"name": "a"}}`, nil},
		{"every unknown and missing field", `{"count": 1, "extra": 1, "more": 2, "items": [{"name": "a", "x": 1}], "nested": {}}`,
			[]string{"extra: not a field here", "more: not a field here", "items[0].x: not a field here", "nested.name: missing", "note: missing"}},
		{"null where a value belongs", `{"count": null, "note": null, "items": null, "nested": {"name": null}}`,
			[]string{"count: null is not allowed", "items: null is not allowed", "nested.name: null is not allowed"}},
		{"a value of the wrong kind", `{"count": "1", "note": null, "items": [], "nested": {"name": "a"}}`,
			[]string{"count: a string where a number belongs"}},
		{"not JSON", `{"count": 1,`, []string{"not JSON: the value ends early"}},
		{"more after the value", `{"count": 1, "note": null, "items": [], "nested": {"name": "a"}} {}`, []string{"there is more after the value"}},
		{"null", `null`, []string{"the value is null"}},
	} {
		_, got := Decode[shape]([]byte(c.raw))
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestChecks(t *testing.T) {
	t.Parallel()
	if got := Blank(struct {
		Label string `json:"label"`
		Help  string `json:"help"`
	}{" ", ""}, "q", "help"); !slices.Equal(got, []string{"q.label: blank"}) {
		t.Errorf("Blank: %q", got)
	}
	got := IDs("stages", []string{"pilot", "Pilot", "pilot"}, func(s string) string { return s }, Slug)
	if len(got) != 2 || !strings.Contains(got[0], "stages[1]") || !strings.Contains(got[1], "repeats stages[0]") {
		t.Errorf("IDs: %q", got)
	}
	if got := Placeholders("attribution", "{classroom} by {school}", "classroom"); len(got) != 1 || !strings.Contains(got[0], "{school}") {
		t.Errorf("Placeholders: %q", got)
	}
	for s, want := range map[string]bool{"https://example.org/x": true, "http://example.org": false, "/relative": false, "https://": false} {
		if HTTPSURL(s) != want {
			t.Errorf("HTTPSURL(%q) is %v", s, !want)
		}
	}
	if err := Refuse("privacy", nil); err != nil {
		t.Errorf("Refuse with no problems: %v", err)
	}
	if err := Refuse("privacy", []string{"a", "b"}); err == nil || err.Error() != "privacy is not valid:\n  - a\n  - b" {
		t.Errorf("Refuse: %v", err)
	}
}
