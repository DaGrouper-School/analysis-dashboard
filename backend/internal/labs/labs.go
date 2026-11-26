// Package labs is the labs module: the places a classroom measures, kept
// here and created in SAM as a space under the classroom's, with a site at
// the lab (docs/design/labs-and-readings.md#labs).
package labs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// Lab is the front end's Lab.
type Lab struct {
	ID          uuid.UUID   `json:"id"`
	ClassroomID uuid.UUID   `json:"classroomId"`
	Name        string      `json:"name"`
	PlaceName   string      `json:"placeName"`
	Country     string      `json:"country"`
	Location    Point       `json:"location"`
	SiteProfile SiteProfile `json:"siteProfile"`
	CreatedAt   time.Time   `json:"createdAt"`
	// Number is the lab's in its classroom, from 1.
	Number  int        `json:"-"`
	SpaceID *uuid.UUID `json:"-"`
}

// Point is the front end's GeoPoint.
type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// SiteProfile is the front end's SiteProfile: the site's questionnaire and
// the classroom's notes on each value there, by registry ID.
type SiteProfile struct {
	Setting           string            `json:"setting"`
	Description       string            `json:"description"`
	ResearchQuestions []string          `json:"researchQuestions"`
	ParameterNotes    map[string]string `json:"parameterNotes,omitempty"`
}

// Input is the front end's CreateLabInput.
type Input struct {
	Name        string      `json:"name"`
	PlaceName   string      `json:"placeName"`
	Country     string      `json:"country"`
	Location    Point       `json:"location"`
	SiteProfile SiteProfile `json:"siteProfile"`
}

var siteSettings = []string{"coastal", "urban", "rural", "highland", "forest", "other"}

// NoteMax is the most a value's note at a site holds.
const NoteMax = 400

// parameterID is a registry ID in SAM's forms, such as pm2_5 or
// dgs:soil_ec.
var parameterID = regexp.MustCompile(`^(?:[a-z][a-z0-9]*:)?[a-z][a-z0-9_]*$`)

// Check reports what is wrong with a lab, by field, as the front end
// checks it.
func Check(in Input) map[string]string {
	f := map[string]string{}
	if utf8.RuneCountInString(strings.TrimSpace(in.Name)) < 3 {
		f["name"] = "Enter a lab name."
	}
	if strings.TrimSpace(in.PlaceName) == "" {
		f["placeName"] = "Enter where the lab is."
	}
	if strings.TrimSpace(in.Country) == "" {
		f["country"] = "Enter a country."
	}
	if p := in.Location; math.IsNaN(p.Lat) || math.IsNaN(p.Lng) || math.Abs(p.Lat) > 90 || math.Abs(p.Lng) > 180 {
		f["location"] = "Enter a valid latitude and longitude."
	}
	sp := in.SiteProfile
	if !slices.Contains(siteSettings, sp.Setting) {
		f["setting"] = "Pick the kind of place."
	}
	if utf8.RuneCountInString(strings.TrimSpace(sp.Description)) < 10 {
		f["description"] = "Describe the site in a sentence or two."
	}
	if !slices.ContainsFunc(sp.ResearchQuestions, func(q string) bool { return strings.TrimSpace(q) != "" }) {
		f["researchQuestions"] = "Add at least one research question."
	}
	for id, note := range sp.ParameterNotes {
		switch {
		case !parameterID.MatchString(id):
			f["parameterNotes"] = fmt.Sprintf("%q is not a value of the registry.", id)
		case utf8.RuneCountInString(strings.TrimSpace(note)) > NoteMax:
			f["parameterNotes"] = fmt.Sprintf("Keep the note on %s to %d characters.", id, NoteMax)
		}
	}
	return f
}

// clean is in trimmed, with blank questions and notes left out.
func clean(in Input) Input {
	out := Input{Name: strings.TrimSpace(in.Name), PlaceName: strings.TrimSpace(in.PlaceName), Country: strings.TrimSpace(in.Country),
		Location: in.Location, SiteProfile: SiteProfile{Setting: in.SiteProfile.Setting, Description: strings.TrimSpace(in.SiteProfile.Description),
			ResearchQuestions: []string{}}}
	for _, q := range in.SiteProfile.ResearchQuestions {
		if q = strings.TrimSpace(q); q != "" {
			out.SiteProfile.ResearchQuestions = append(out.SiteProfile.ResearchQuestions, q)
		}
	}
	for id, note := range in.SiteProfile.ParameterNotes {
		if note = strings.TrimSpace(note); note != "" {
			if out.SiteProfile.ParameterNotes == nil {
				out.SiteProfile.ParameterNotes = map[string]string{}
			}
			out.SiteProfile.ParameterNotes[id] = note
		}
	}
	return out
}

// SiteCode is a lab's site code: its classroom's code and its number,
// such as DGC-HG2W5K-01.
func SiteCode(classroomCode string, number int) string {
	return fmt.Sprintf("%s-%02d", classroomCode, number)
}

// Kind is the outbox kind that creates a lab and its site in SAM.
const Kind = "lab"

func entity(id uuid.UUID) string { return "lab:" + id.String() }

// desired is what SAM should hold of a lab: a space under its classroom's,
// with a site of code at the lab's point.
type desired struct {
	Code     string `json:"code"`
	Location Point  `json:"location"`
}

// Create stores a new lab in a classroom, numbered after its others, and
// sends it to SAM.
func Create(ctx context.Context, q db.Querier, now time.Time, classroom uuid.UUID, classroomCode string, in Input) (Lab, error) {
	in = clean(in)
	l := Lab{ID: uuid.New(), ClassroomID: classroom, Name: in.Name, PlaceName: in.PlaceName, Country: in.Country, Location: in.Location,
		SiteProfile: in.SiteProfile, CreatedAt: now}
	profile, err := json.Marshal(l.SiteProfile)
	if err != nil {
		return l, err
	}
	err = pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		for range 5 {
			err := tx.QueryRow(ctx, `
				INSERT INTO labs (id, classroom_id, number, name, place_name, country, lat, lng, site_profile, created_at, updated_at)
				VALUES ($1, $2, (SELECT coalesce(max(number), 0) + 1 FROM labs WHERE classroom_id = $2), $3, $4, $5, $6, $7, $8, $9, $9)
				ON CONFLICT (classroom_id, number) DO NOTHING RETURNING number`,
				l.ID, classroom, l.Name, l.PlaceName, l.Country, l.Location.Lat, l.Location.Lng, profile, now).Scan(&l.Number)
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			return send(ctx, tx, now, l, classroomCode)
		}
		return errors.New("another lab took the number at the same time, five times")
	})
	return l, err
}

// Update changes a lab, and sends its site to SAM again when its location
// changed.
func Update(ctx context.Context, q db.Querier, now time.Time, l Lab, classroomCode string, in Input) (Lab, error) {
	in = clean(in)
	moved := in.Location != l.Location
	l.Name, l.PlaceName, l.Country, l.Location, l.SiteProfile = in.Name, in.PlaceName, in.Country, in.Location, in.SiteProfile
	profile, err := json.Marshal(l.SiteProfile)
	if err != nil {
		return l, err
	}
	err = pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE labs SET name = $2, place_name = $3, country = $4, lat = $5, lng = $6, site_profile = $7, updated_at = $8
			WHERE id = $1`, l.ID, l.Name, l.PlaceName, l.Country, l.Location.Lat, l.Location.Lng, profile, now); err != nil {
			return err
		}
		if moved {
			return send(ctx, tx, now, l, classroomCode)
		}
		return nil
	})
	return l, err
}

func send(ctx context.Context, q db.Querier, now time.Time, l Lab, classroomCode string) error {
	_, err := samaccess.Enqueue(ctx, q, now, samaccess.Change{Entity: entity(l.ID), Kind: Kind,
		Payload: desired{Code: SiteCode(classroomCode, l.Number), Location: l.Location}})
	return err
}

const labColumns = `id, classroom_id, number, name, place_name, country, lat, lng, site_profile, created_at, space_id`

func scan(r pgx.Row) (Lab, error) {
	var l Lab
	var profile []byte
	if err := r.Scan(&l.ID, &l.ClassroomID, &l.Number, &l.Name, &l.PlaceName, &l.Country, &l.Location.Lat, &l.Location.Lng, &profile,
		&l.CreatedAt, &l.SpaceID); err != nil {
		return l, err
	}
	if err := json.Unmarshal(profile, &l.SiteProfile); err != nil {
		return l, err
	}
	if l.SiteProfile.ResearchQuestions == nil {
		l.SiteProfile.ResearchQuestions = []string{}
	}
	return l, nil
}

// ErrNoLab means there is no such lab.
var ErrNoLab = errors.New("no such lab")

// Get is the lab with id, or ErrNoLab.
func Get(ctx context.Context, q db.Querier, id uuid.UUID) (Lab, error) {
	l, err := scan(q.QueryRow(ctx, `SELECT `+labColumns+` FROM labs WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return l, ErrNoLab
	}
	return l, err
}

// ForClassroom are a classroom's labs, in the order they were added.
func ForClassroom(ctx context.Context, q db.Querier, classroom uuid.UUID) ([]Lab, error) {
	rows, err := q.Query(ctx, `SELECT `+labColumns+` FROM labs WHERE classroom_id = $1 ORDER BY number`, classroom)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Lab{}
	for rows.Next() {
		l, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func confirmSpace(ctx context.Context, q db.Querier, id, space uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE labs SET space_id = $2 WHERE id = $1`, id, space)
	return err
}

func confirmSite(ctx context.Context, q db.Querier, id uuid.UUID, version int) error {
	_, err := q.Exec(ctx, `UPDATE labs SET site_version = $2 WHERE id = $1`, id, version)
	return err
}
