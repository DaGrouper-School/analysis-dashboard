package apitest_test

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
)

// consentTexts is the built-in first version's words, as an admin would
// send them.
func consentTexts(t *testing.T) classrooms.ConsentTexts {
	t.Helper()
	raw, err := os.ReadFile("../classrooms/consent.json")
	if err != nil {
		t.Fatal(err)
	}
	var texts classrooms.ConsentTexts
	if err := json.Unmarshal(raw, &texts); err != nil {
		t.Fatal(err)
	}
	return texts
}

// TestConsentTexts: only platform admins list, read and publish the
// consent text's versions; each publication is the next, current version
// in every language; one that breaks the checks is refused, field by
// field.
func TestConsentTexts(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	someone := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	for _, c := range []struct{ method, path string }{{"GET", "/admin/consent-texts"}, {"GET", "/admin/consent-texts/1"}, {"POST", "/admin/consent-texts"}} {
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
		res, got := e.DoWith(t, a.client, method, "/api/admin/consent-texts"+path, raw)
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
	var versions []classrooms.ConsentVersion
	if status, _ := do("GET", "", nil, &versions); status != 200 || len(versions) != 0 {
		t.Fatalf("before any: %d %+v", status, versions)
	}
	texts := consentTexts(t)
	var v classrooms.ConsentTextView
	if status, no := do("POST", "", texts, &v); status != 200 || v.Version != 1 || v.PublishedBy != "Amara Okafor" || v.Texts["en"].Title == "" {
		t.Fatalf("the first version: %d %+v %+v", status, v, no)
	}
	am := texts["en"]
	am.Title = "የአሳዳጊ ፈቃድ"
	texts["am"] = am
	if status, no := do("POST", "", texts, &v); status != 200 || v.Version != 2 || len(v.Languages) != 2 {
		t.Fatalf("a second version: %d %+v %+v", status, v, no)
	}
	bad := texts["en"]
	bad.Title, bad.OfficeNote = "", "For {school}."
	status, no := do("POST", "", classrooms.ConsentTexts{"en": bad}, nil)
	if status != 400 || no.Fields["texts"] == "" || no.Fields["texts.en.title"] != "blank" || no.Fields["texts.en.officeNote"] == "" {
		t.Errorf("a version dropping a language, with a blank title and an unknown placeholder: %d %+v", status, no)
	}
	if status, _ := do("GET", "", nil, &versions); status != 200 || len(versions) != 2 || versions[0].Version != 2 {
		t.Errorf("the versions: %+v", versions)
	}
	var first classrooms.ConsentTextView
	if status, _ := do("GET", "/1", nil, &first); status != 200 || first.Version != 1 || len(first.Texts) != 1 {
		t.Errorf("the first version: %d %+v", status, first)
	}
	for _, path := range []string{"/3", "/0", "/one"} {
		if status, _ := do("GET", path, nil, nil); status != 404 {
			t.Errorf("%s: %d", path, status)
		}
	}
}
