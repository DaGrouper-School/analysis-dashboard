// Package classrooms holds the classrooms and what they refer to. So far,
// the stages and regions platform admins keep
// (docs/design/registry-and-content.md#platform-configuration).
package classrooms

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// Stage is a classroom's stage on the way to operating, such as pilot.
type Stage struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	// Color is the marker and badge color, as #rgb or #rrggbb.
	Color string `json:"color"`
}

// Region groups classrooms in the directory.
type Region struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Stages are the stages, in display order.
func Stages(ctx context.Context, q db.Querier) ([]Stage, error) { return stages(ctx, q) }

// Regions are the regions, in display order.
func Regions(ctx context.Context, q db.Querier) ([]Region, error) { return regions(ctx, q) }

var hexColor = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// CheckStages reports what is wrong with a list of stages.
func CheckStages(path string, list []Stage) []string {
	p := jsoncheck.IDs(path, list, func(s Stage) string { return s.ID }, jsoncheck.Slug)
	for i, s := range list {
		if !hexColor.MatchString(s.Color) {
			p = append(p, fmt.Sprintf("%s[%d].color: %q is not a color such as #f6921e", path, i, s.Color))
		}
	}
	return append(p, jsoncheck.Blank(list, path)...)
}

// CheckRegions reports what is wrong with a list of regions.
func CheckRegions(path string, list []Region) []string {
	p := jsoncheck.IDs(path, list, func(r Region) string { return r.ID }, jsoncheck.Slug)
	return append(p, jsoncheck.Blank(list, path)...)
}

// seedFile is the stages' and regions' seed, the same as the front end's
// seed (frontend/src/lib/data/seed.ts), built into the binary.
//
//go:embed seed.json
var seedFile []byte

type seed struct {
	Stages  []Stage  `json:"stages"`
	Regions []Region `json:"regions"`
}

// SeedLists brings the stages and regions in line with the seed, as
// settings.SeedList does.
func SeedLists(ctx context.Context, q db.Querier, now time.Time) ([]settings.Seeded, error) {
	s, err := seedLists()
	if err != nil {
		return nil, err
	}
	out, err := settings.SeedList(ctx, q, now, stageList, s.Stages)
	if err != nil {
		return out, err
	}
	more, err := settings.SeedList(ctx, q, now, regionList, s.Regions)
	return append(out, more...), err
}

func seedLists() (seed, error) {
	s, problems := jsoncheck.Decode[seed](seedFile)
	if len(problems) == 0 {
		problems = append(CheckStages("stages", s.Stages), CheckRegions("regions", s.Regions)...)
		if len(s.Stages) == 0 || len(s.Regions) == 0 {
			problems = append(problems, "the seed needs a stage and a region")
		}
	}
	return s, jsoncheck.Refuse("the classrooms' seed", problems)
}

var stageList = settings.List[Stage]{
	Kind:    settings.KindStage,
	ID:      func(s Stage) string { return s.ID },
	Count:   countStages,
	Current: stage,
	Insert:  insertStage,
	Update:  updateStage,
}

var regionList = settings.List[Region]{
	Kind:    settings.KindRegion,
	ID:      func(r Region) string { return r.ID },
	Count:   countRegions,
	Current: region,
	Insert:  insertRegion,
	Update:  updateRegion,
}
