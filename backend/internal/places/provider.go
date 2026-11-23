package places

import (
	"context"
	"errors"
	"strings"
)

// Provider looks places up: what a person types, and the address of a
// place they chose.
type Provider interface {
	Autocomplete(ctx context.Context, q string) ([]Suggestion, error)
	Address(ctx context.Context, placeID string) (Address, error)
}

// ErrNotFound is a provider's answer for a place it does not know.
var ErrNotFound = errors.New("no such place")

// Suggestion is the front end's PlaceSuggestion.
type Suggestion struct {
	PlaceID       string `json:"placeId"`
	PrimaryText   string `json:"primaryText"`
	SecondaryText string `json:"secondaryText"`
}

// Address is the front end's Address, with the place it came from.
type Address struct {
	Street  string   `json:"street"`
	Suite   string   `json:"suite,omitempty"`
	City    string   `json:"city"`
	State   string   `json:"state"`
	Zip     string   `json:"zip"`
	Country string   `json:"country"`
	PlaceID string   `json:"placeId,omitempty"`
	Lat     *float64 `json:"lat,omitempty"`
	Lng     *float64 `json:"lng,omitempty"`
}

// minQuery is the shortest query looked up; shorter ones match nothing.
const minQuery = 2

// Offline is the provider development and test use: a fixed list covering
// the pilot regions, the same as the front end's mock.
type Offline struct{}

func at(lat, lng float64) (*float64, *float64) { return &lat, &lng }

var offline = func() []Address {
	place := func(id, street, city, state, zip, country string, lat, lng float64) Address {
		a := Address{PlaceID: id, Street: street, City: city, State: state, Zip: zip, Country: country}
		a.Lat, a.Lng = at(lat, lng)
		return a
	}
	return []Address{
		place("pl_shashamane", "Kebele 04", "Shashamane", "Oromia", "", "Ethiopia", 7.2, 38.6),
		place("pl_addis", "Churchill Avenue", "Addis Ababa", "Addis Ababa", "1000", "Ethiopia", 9.03, 38.74),
		place("pl_hawassa", "Lake Road", "Hawassa", "Sidama", "", "Ethiopia", 7.06, 38.48),
		place("pl_nassau", "14 Bay Street", "Nassau", "New Providence", "", "Bahamas", 25.06, -77.34),
		place("pl_freeport", "East Mall Drive", "Freeport", "Grand Bahama", "", "Bahamas", 26.53, -78.7),
		place("pl_hargeisa", "Independence Road", "Hargeisa", "Maroodi Jeex", "", "Somalia", 9.56, 44.06),
		place("pl_mogadishu", "Maka Al Mukarama Road", "Mogadishu", "Banaadir", "", "Somalia", 2.05, 45.32),
		place("pl_kingston", "King Street", "Kingston", "Kingston Parish", "", "Jamaica", 17.97, -76.79),
	}
}()

// Autocomplete is the places whose street, city, region or country holds
// q, ignoring case.
func (Offline) Autocomplete(_ context.Context, q string) ([]Suggestion, error) {
	q = strings.ToLower(strings.TrimSpace(q))
	out := []Suggestion{}
	if len([]rune(q)) < minQuery {
		return out, nil
	}
	for _, p := range offline {
		for _, part := range []string{p.Street, p.City, p.State, p.Country} {
			if strings.Contains(strings.ToLower(part), q) {
				out = append(out, Suggestion{PlaceID: p.PlaceID, PrimaryText: p.Street, SecondaryText: joinNonEmpty(p.City, p.State, p.Country)})
				break
			}
		}
	}
	return out, nil
}

// Address is the listed place placeID.
func (Offline) Address(_ context.Context, placeID string) (Address, error) {
	for _, p := range offline {
		if p.PlaceID == placeID {
			return p, nil
		}
	}
	return Address{}, ErrNotFound
}

func joinNonEmpty(parts ...string) string {
	kept := parts[:0:0]
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ", ")
}
