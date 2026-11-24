package apitest_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/places"
)

// TestPlaces: signed-in people look addresses up; anyone gets the calling
// codes, pilot countries first.
func TestPlaces(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	if res, _ := e.Do(t, "GET", "/api/places/autocomplete?q=addis", ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("autocomplete without a session: %d", res.StatusCode)
	}
	if res, _ := e.Do(t, "GET", "/api/places/pl_addis", ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("an address without a session: %d", res.StatusCode)
	}
	res, raw := e.Do(t, "GET", "/api/places/dial-codes", "")
	var codes []places.DialCode
	if err := json.Unmarshal([]byte(raw), &codes); err != nil || res.StatusCode != http.StatusOK || len(codes) < 4 || codes[0].Code != "ET" {
		t.Fatalf("dial codes: %d %s", res.StatusCode, raw)
	}

	p := signedIn(t, e, amara)
	res, raw = e.DoWith(t, p.client, "GET", "/api/places/autocomplete?q=addis", "")
	var found []places.Suggestion
	if err := json.Unmarshal([]byte(raw), &found); err != nil || res.StatusCode != http.StatusOK || len(found) != 1 || found[0].PlaceID != "pl_addis" {
		t.Fatalf("autocomplete: %d %s", res.StatusCode, raw)
	}
	if _, raw := e.DoWith(t, p.client, "GET", "/api/places/autocomplete", ""); raw != "[]\n" {
		t.Fatalf("autocomplete without a query: %s", raw)
	}
	res, raw = e.DoWith(t, p.client, "GET", "/api/places/pl_addis", "")
	if res.StatusCode != http.StatusOK || !strings.Contains(raw, `"city":"Addis Ababa"`) {
		t.Fatalf("an address: %d %s", res.StatusCode, raw)
	}
	if res, _ := e.DoWith(t, p.client, "GET", "/api/places/pl_nowhere", ""); res.StatusCode != http.StatusNotFound {
		t.Fatalf("an unknown place: %d", res.StatusCode)
	}
}

// TestPlacesAreLimitedPerUser: past their lookups an hour, a person gets
// 429 with Retry-After; another person still gets theirs.
func TestPlacesAreLimitedPerUser(t *testing.T) {
	t.Parallel()
	e := apitest.NewWith(t, apitest.Options{Places: places.Config{PerUserPerHour: 3}})
	p := signedIn(t, e, amara)
	for _, path := range []string{"/api/places/autocomplete?q=addis", "/api/places/pl_addis", "/api/places/autocomplete?q=nassau"} {
		if res, _ := e.DoWith(t, p.client, "GET", path, ""); res.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d", path, res.StatusCode)
		}
	}
	res, _ := e.DoWith(t, p.client, "GET", "/api/places/autocomplete?q=kingston", "")
	if res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("the fourth lookup: %d", res.StatusCode)
	}
	retryAfter(t, res)
	other := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	if res, _ := e.DoWith(t, other.client, "GET", "/api/places/autocomplete?q=kingston", ""); res.StatusCode != http.StatusOK {
		t.Fatalf("another person's lookup: %d", res.StatusCode)
	}
}

// TestThePlacesKeyStaysOnTheServer: with Google Places, the key goes to
// Google in a header and appears in no answer this service gives.
func TestThePlacesKeyStaysOnTheServer(t *testing.T) {
	t.Parallel()
	const key = "places-secret-key"
	var mu sync.Mutex
	var seen []string
	google := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("X-Goog-Api-Key"))
		mu.Unlock()
		switch r.URL.Path {
		case "/places:autocomplete":
			_, _ = w.Write([]byte(`{"suggestions":[{"placePrediction":{"placeId":"ChIJ-bole","structuredFormat":{
				"mainText":{"text":"Bole Road"},"secondaryText":{"text":"Addis Ababa, Ethiopia"}}}}]}`))
		default:
			_, _ = w.Write([]byte(`{"addressComponents":[{"longText":"Bole Road","types":["route"]},
				{"longText":"Addis Ababa","types":["locality"]},{"longText":"Ethiopia","types":["country"]}]}`))
		}
	}))
	t.Cleanup(google.Close)
	e := apitest.NewWith(t, apitest.Options{Places: places.Config{APIKey: key, BaseURL: google.URL}})
	p := signedIn(t, e, amara)
	for _, path := range []string{"/api/places/autocomplete?q=bole", "/api/places/ChIJ-bole", "/api/places/dial-codes"} {
		res, raw := e.DoWith(t, p.client, "GET", path, "")
		headers, _ := json.Marshal(res.Header)
		if res.StatusCode != http.StatusOK || strings.Contains(raw, key) || strings.Contains(string(headers), key) {
			t.Fatalf("%s: %d, or the key in %s %s", path, res.StatusCode, raw, headers)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 || seen[0] != key || seen[1] != key {
		t.Fatalf("Google was sent %v", seen)
	}
}
