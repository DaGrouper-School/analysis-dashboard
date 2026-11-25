// Package hosting holds hosting a Global Classroom. So far, the host
// application's readiness questions platform admins keep
// (docs/design/registry-and-content.md#platform-configuration).
package hosting

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

// Question is one readiness question.
type Question struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Help is shown under the question; it may be empty.
	Help     string `json:"help"`
	Kind     string `json:"kind"`
	Required bool   `json:"required"`
}

// The kinds of answer a question takes.
const (
	KindYesNo  = "yes_no"
	KindNumber = "number"
	KindText   = "text"
)

// Questions are the questions an applicant is asked, in order: every
// question not retired.
func Questions(ctx context.Context, q db.Querier) ([]Question, error) { return questions(ctx, q) }

var questionID = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// CheckQuestions reports what is wrong with a list of questions.
func CheckQuestions(path string, list []Question) []string {
	p := jsoncheck.IDs(path, list, func(q Question) string { return q.ID }, questionID)
	for i, q := range list {
		switch q.Kind {
		case KindYesNo, KindNumber, KindText:
		default:
			p = append(p, fmt.Sprintf("%s[%d].kind: %q is not yes_no, number or text", path, i, q.Kind))
		}
	}
	return append(p, jsoncheck.Blank(list, path, "help")...)
}

// seedFile is the questions' seed, the same as the front end's seed
// (frontend/src/lib/data/seed.ts), built into the binary.
//
//go:embed seed.json
var seedFile []byte

// SeedQuestions brings the questions in line with the seed, as
// settings.SeedList does.
func SeedQuestions(ctx context.Context, q db.Querier, now time.Time) ([]settings.Seeded, error) {
	list, err := seedQuestions()
	if err != nil {
		return nil, err
	}
	return settings.SeedList(ctx, q, now, questionList, list)
}

func seedQuestions() ([]Question, error) {
	list, problems := jsoncheck.Decode[[]Question](seedFile)
	if len(problems) == 0 {
		problems = CheckQuestions("questions", list)
		if len(list) == 0 {
			problems = append(problems, "the seed needs a question")
		}
	}
	return list, jsoncheck.Refuse("the host questions' seed", problems)
}

var questionList = settings.List[Question]{
	Kind:    settings.KindHostQuestion,
	ID:      func(q Question) string { return q.ID },
	Count:   countQuestions,
	Current: question,
	Insert:  insertQuestion,
	Update:  updateQuestion,
}
