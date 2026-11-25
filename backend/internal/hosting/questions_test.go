package hosting

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

// TestSeedFile: the built-in seed passes its checks.
func TestSeedFile(t *testing.T) {
	t.Parallel()
	if list, err := seedQuestions(); err != nil || len(list) == 0 {
		t.Fatalf("%+v, %v", list, err)
	}
}

func TestCheckQuestions(t *testing.T) {
	t.Parallel()
	got := CheckQuestions("questions", []Question{
		{ID: "storage", Label: "Storage?", Help: "", Kind: KindYesNo, Required: true},
		{ID: "2bad", Label: "", Help: "", Kind: "choice"},
	})
	want := []string{`questions[1]: "2bad" is not an ID`, `questions[1].kind: "choice" is not yes_no, number or text`, "questions[1].label: blank"}
	if len(got) != len(want) {
		t.Fatalf("got %q", got)
	}
	for i, w := range want {
		if !strings.Contains(got[i], w) {
			t.Errorf("problem %d: %q lacks %q", i, got[i], w)
		}
	}
}

// TestSeedQuestions: the seed adds every question; an applicant is never
// asked a retired one, and the seed keeps it retired.
func TestSeedQuestions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.DB(t)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	seeded, err := SeedQuestions(ctx, pool, now)
	if err != nil || len(seeded) != 5 || seeded[0].Outcome != settings.Added {
		t.Fatalf("the seed: %+v, %v", seeded, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE host_questions SET retired_at = now() WHERE id = 'notes'`); err != nil {
		t.Fatal(err)
	}
	asked, err := Questions(ctx, pool)
	if err != nil || len(asked) != 4 || asked[0].ID != "storage" || asked[3].ID != "learners" || asked[3].Kind != KindNumber {
		t.Fatalf("questions: %+v, %v", asked, err)
	}
	seeded, err = SeedQuestions(ctx, pool, now.Add(time.Hour))
	if err != nil || seeded[4].What != "host_question notes" || seeded[4].Outcome != settings.Kept {
		t.Fatalf("the seed again: %+v, %v", seeded, err)
	}
	if asked, _ = Questions(ctx, pool); len(asked) != 4 {
		t.Fatalf("a retired question came back: %+v", asked)
	}
}
