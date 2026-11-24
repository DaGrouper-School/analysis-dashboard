package places

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOffline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	got, err := Offline{}.Autocomplete(ctx, " ADDIS ")
	if err != nil || len(got) != 1 || got[0].PlaceID != "pl_addis" || got[0].PrimaryText != "Churchill Avenue" ||
		got[0].SecondaryText != "Addis Ababa, Addis Ababa, Ethiopia" {
		t.Fatalf("addis: %+v, %v", got, err)
	}
	if got, _ := (Offline{}).Autocomplete(ctx, "ethiopia"); len(got) != 3 {
		t.Fatalf("ethiopia: %d places", len(got))
	}
	if got, _ := (Offline{}).Autocomplete(ctx, "a"); got == nil || len(got) != 0 {
		t.Fatalf("one letter: %+v", got)
	}
	a, err := Offline{}.Address(ctx, "pl_kingston")
	if err != nil || a.City != "Kingston" || a.Country != "Jamaica" || a.Lat == nil || *a.Lat != 17.97 {
		t.Fatalf("kingston: %+v, %v", a, err)
	}
	if _, err := (Offline{}).Address(ctx, "pl_nowhere"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown place: %v", err)
	}
}

// fakeGoogle is the Places API as far as the provider uses it, checking the
// key arrives in its header only.
func fakeGoogle(t *testing.T, key string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Goog-Api-Key") != key || strings.Contains(r.URL.String(), key) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/places:autocomplete":
			var in struct{ Input string }
			_ = json.NewDecoder(r.Body).Decode(&in)
			_, _ = w.Write([]byte(`{"suggestions":[{"placePrediction":{"placeId":"ChIJ-bole","structuredFormat":{
				"mainText":{"text":"Bole Road"},"secondaryText":{"text":"Addis Ababa, Ethiopia"}}}},{"queryPrediction":{}}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/places/ChIJ-bole":
			if r.Header.Get("X-Goog-FieldMask") != addressFields {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"addressComponents":[
				{"longText":"12","types":["street_number"]},
				{"longText":"Bole Road","types":["route"]},
				{"longText":"Addis Ababa","types":["locality","political"]},
				{"longText":"Addis Ababa","types":["administrative_area_level_1","political"]},
				{"longText":"1000","types":["postal_code"]},
				{"longText":"Ethiopia","types":["country","political"]}],
				"location":{"latitude":9.0,"longitude":38.79}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestGoogle: suggestions and addresses come from the Places API, the key
// sent only in its header.
func TestGoogle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	g := NewGoogle("secret-key", fakeGoogle(t, "secret-key").URL)
	got, err := g.Autocomplete(ctx, "bole")
	if err != nil || len(got) != 1 || got[0] != (Suggestion{PlaceID: "ChIJ-bole", PrimaryText: "Bole Road", SecondaryText: "Addis Ababa, Ethiopia"}) {
		t.Fatalf("suggestions %+v, %v", got, err)
	}
	a, err := g.Address(ctx, "ChIJ-bole")
	if err != nil || a.Street != "12 Bole Road" || a.City != "Addis Ababa" || a.State != "Addis Ababa" || a.Zip != "1000" ||
		a.Country != "Ethiopia" || a.PlaceID != "ChIJ-bole" || a.Lng == nil || *a.Lng != 38.79 {
		t.Fatalf("the address %+v, %v", a, err)
	}
	if _, err := g.Address(ctx, "ChIJ-nowhere"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown place: %v", err)
	}
	if _, err := NewGoogle("wrong-key", fakeGoogle(t, "secret-key").URL).Autocomplete(ctx, "bole"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("a refused key: %v", err)
	}
}
