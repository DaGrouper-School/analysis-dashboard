package places

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// Google Places (the Places API, places.googleapis.com).
const (
	// GoogleURL is the Places API.
	GoogleURL = "https://places.googleapis.com/v1"
	// googleTimeout bounds each call.
	googleTimeout = 10 * time.Second
	// addressFields is what a lookup asks Google for, and pays for.
	addressFields = "addressComponents,location"
)

// Google looks places up with Google Places. Its key is sent in a request
// header, to Google only; it never reaches the browser.
type Google struct {
	key  string
	url  string
	http *http.Client
}

// NewGoogle is the Google provider with key; baseURL is the API's, or
// another for tests.
func NewGoogle(key, baseURL string) *Google {
	if baseURL == "" {
		baseURL = GoogleURL
	}
	return &Google{key: key, url: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: googleTimeout}}
}

// Autocomplete is Google's predictions for q.
func (g *Google) Autocomplete(ctx context.Context, q string) ([]Suggestion, error) {
	out := []Suggestion{}
	if len([]rune(strings.TrimSpace(q))) < minQuery {
		return out, nil
	}
	body, _ := json.Marshal(map[string]string{"input": q})
	var res struct {
		Suggestions []struct {
			PlacePrediction *struct {
				PlaceID          string `json:"placeId"`
				StructuredFormat struct {
					MainText      struct{ Text string } `json:"mainText"`
					SecondaryText struct{ Text string } `json:"secondaryText"`
				} `json:"structuredFormat"`
			} `json:"placePrediction"`
		} `json:"suggestions"`
	}
	if err := g.call(ctx, http.MethodPost, "/places:autocomplete", bytes.NewReader(body), "", &res); err != nil {
		return nil, err
	}
	for _, s := range res.Suggestions {
		if p := s.PlacePrediction; p != nil {
			out = append(out, Suggestion{PlaceID: p.PlaceID, PrimaryText: p.StructuredFormat.MainText.Text,
				SecondaryText: p.StructuredFormat.SecondaryText.Text})
		}
	}
	return out, nil
}

// googleComponent is one part of an address, as Google gives it.
type googleComponent struct {
	LongText string   `json:"longText"`
	Types    []string `json:"types"`
}

// Address is the address Google has for placeID.
func (g *Google) Address(ctx context.Context, placeID string) (Address, error) {
	var res struct {
		AddressComponents []googleComponent `json:"addressComponents"`
		Location          *struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"location"`
	}
	if err := g.call(ctx, http.MethodGet, "/places/"+url.PathEscape(placeID), http.NoBody, addressFields, &res); err != nil {
		return Address{}, err
	}
	part := func(types ...string) string {
		for _, t := range types {
			for _, c := range res.AddressComponents {
				if slices.Contains(c.Types, t) {
					return c.LongText
				}
			}
		}
		return ""
	}
	a := Address{PlaceID: placeID, Street: joinSpace(part("street_number"), part("route")), City: part("locality", "postal_town",
		"administrative_area_level_2"), State: part("administrative_area_level_1"), Zip: part("postal_code"), Country: part("country")}
	if a.Street == "" {
		a.Street = part("premise", "neighborhood", "sublocality")
	}
	if res.Location != nil {
		a.Lat, a.Lng = at(res.Location.Latitude, res.Location.Longitude)
	}
	return a, nil
}

// call sends one request to the API with the key, and decodes its answer.
func (g *Google) call(ctx context.Context, method, path string, body io.Reader, fields string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, g.url+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("X-Goog-Api-Key", g.key)
	req.Header.Set("Content-Type", "application/json")
	if fields != "" {
		req.Header.Set("X-Goog-FieldMask", fields)
	}
	res, err := g.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling the Places API: %w", err)
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound, res.StatusCode == http.StatusBadRequest && strings.HasPrefix(path, "/places/"):
		return ErrNotFound
	case res.StatusCode != http.StatusOK:
		return fmt.Errorf("the Places API answered %s", res.Status)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func joinSpace(a, b string) string { return strings.TrimSpace(a + " " + b) }
