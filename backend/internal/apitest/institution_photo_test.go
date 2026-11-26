package apitest_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/media"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/publicsite"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// TestInstitutionPhoto: the institution head sets the photo, kept under
// its content hash and served at its address to anyone, for good; the
// station page shows it; an upload that is not an image, too large or
// without what it shows is refused; the photo can be removed; a stranger
// cannot set one.
func TestInstitutionPhoto(t *testing.T) {
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
	var c classrooms.Summary
	call(a, "POST", "/classrooms", map[string]any{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "hana@trinity.example"}, &c)
	call(head, "POST", "/invitations/by-token/"+tokenFor(t, e, "hana@trinity.example")+"/accept", nil, nil)
	deliver(t, e, "")

	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	photo := b.Bytes()
	image := "data:image/png;base64," + base64.StdEncoding.EncodeToString(photo)
	path := "/classrooms/" + c.ID + "/institution/photo"

	var info classrooms.Institution
	if status, no := call(head, "PUT", path, map[string]any{"image": image, "alt": " The school garden "}, &info); status != 200 {
		t.Fatalf("setting the photo: %d %+v", status, no)
	}
	want := e.URL + "/api/media/" + media.Name("image/png", photo)
	if info.PhotoURL == nil || *info.PhotoURL != want || info.PhotoAlt != "The school garden" {
		t.Fatalf("the photo: %+v, want %s", info, want)
	}

	res, err := http.Get(*info.PhotoURL)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/png" || !strings.Contains(res.Header.Get("Cache-Control"), "immutable") ||
		res.Header.Get("X-Content-Type-Options") != "nosniff" || !bytes.Equal(got, photo) {
		t.Errorf("serving the photo: %d %v", res.StatusCode, res.Header)
	}
	if res, err := http.Get(e.URL + "/api/media/" + strings.Repeat("0", 64) + ".png"); err != nil || res.StatusCode != 404 {
		t.Errorf("an unknown file: %v %v", res, err)
	}

	var station publicsite.StationPage
	if status, _ := call(head, "GET", "/public/classrooms/"+c.ID, nil, &station); status != 200 || station.Institution == nil ||
		station.Institution.PhotoURL == nil || *station.Institution.PhotoURL != want {
		t.Errorf("the station page's institution: %d %+v", status, station.Institution)
	}

	tooLarge := "data:image/png;base64," + base64.StdEncoding.EncodeToString(append(bytes.Clone(photo), make([]byte, media.MaxImageBytes)...))
	for name, c := range map[string]struct {
		body  map[string]any
		field string
	}{
		"not an image":  {map[string]any{"image": "not an image", "alt": "The garden"}, "image"},
		"a gif":         {map[string]any{"image": "data:image/gif;base64,R0lGODlhAQABAAAAACw=", "alt": "The garden"}, "image"},
		"too large":     {map[string]any{"image": tooLarge, "alt": "The garden"}, "image"},
		"what it shows": {map[string]any{"image": image, "alt": " a "}, "alt"},
	} {
		if status, no := call(head, "PUT", path, c.body, nil); status != 400 || no.Code != "invalid_input" || no.Fields[c.field] == "" {
			t.Errorf("%s: %d %+v", name, status, no)
		}
	}
	if status, _ := call(stranger, "PUT", path, map[string]any{"image": image, "alt": "The garden"}, nil); status != 404 {
		t.Errorf("a stranger: %d", status)
	}

	if status, no := call(head, "PUT", path, map[string]any{"image": nil, "alt": ""}, &info); status != 200 || info.PhotoURL != nil || info.PhotoAlt != "" {
		t.Errorf("removing the photo: %d %+v %+v", status, info, no)
	}
}
