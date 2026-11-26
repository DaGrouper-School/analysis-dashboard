package hosting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

// TestQuestionsByAdmins: an admin adds a question at a place and retires
// one, which applicants are no longer asked and which no longer changes;
// admins still see it.
func TestQuestionsByAdmins(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.DB(t)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	if _, err := SeedQuestions(ctx, pool, now); err != nil {
		t.Fatal(err)
	}
	added, err := PutQuestion(ctx, pool, now, settings.Operator, "power", []byte(`{"label": "Is there power?", "help": "", "kind": "yes_no", "required": true, "position": 1}`))
	if err != nil || !added {
		t.Fatalf("a question added: %v", err)
	}
	if err := RetireQuestion(ctx, pool, now, settings.Operator, "notes"); err != nil {
		t.Fatal(err)
	}
	if err := RetireQuestion(ctx, pool, now.Add(time.Hour), settings.Operator, "notes"); err != nil {
		t.Fatalf("retired twice: %v", err)
	}
	if err := RetireQuestion(ctx, pool, now, settings.Operator, "nowhere"); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unknown question: %v", err)
	}
	if _, err := PutQuestion(ctx, pool, now, settings.Operator, "notes", []byte(`{"label": "Notes", "help": "", "kind": "text", "required": false, "position": 0}`)); !errors.Is(err, ErrRetired) {
		t.Errorf("a retired question changed: %v", err)
	}
	asked, _ := Questions(ctx, pool)
	all, _ := AllQuestions(ctx, pool)
	if len(asked) != 5 || asked[1].ID != "power" || len(all) != 6 || all[5].ID != "notes" || all[5].RetiredAt == nil || !all[5].RetiredAt.Equal(now) {
		t.Fatalf("asked %+v, kept %+v", asked, all)
	}
	var invalid *jsoncheck.Invalid
	_, err = PutQuestion(ctx, pool, now, settings.Operator, "2x", []byte(`{"label": "", "help": "", "kind": "choice", "required": true, "position": 0}`))
	if !errors.As(err, &invalid) {
		t.Fatalf("a bad question: %v", err)
	}
	if f := invalid.Fields(); f["id"] == "" || f["kind"] == "" || f["label"] != "blank" || f["help"] != "" {
		t.Errorf("fields: %v", f)
	}
	log, _ := settings.Changes(ctx, pool, 2)
	if log[0].Summary != "Retired host question notes" || log[1].Summary != "Added host question power" {
		t.Errorf("the log: %+v", log)
	}
}
