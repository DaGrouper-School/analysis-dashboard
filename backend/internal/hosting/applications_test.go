package hosting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

func goodApplication() Input {
	return Input{InstitutionName: " Lalibela Secondary School ", ContactName: "Meron Haile", ContactRole: "Science teacher",
		ContactEmail: " Meron@Lalibela.example ", ContactPhone: "00251 911 234 567",
		Answers: map[string]string{"storage": "yes", "lead": "no", "connection": "yes", "learners": " 40 ", "notes": " ", "unasked": "x"},
		Site:    Site{PlaceName: "Lalibela", Country: "Ethiopia", Ecosystems: []string{" highland ", ""}, Learners: "Grade 9", StudyGoals: "Rainfall"}}
}

func seeded(t *testing.T) []Question {
	t.Helper()
	list, err := seedQuestions()
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func TestCheck(t *testing.T) {
	t.Parallel()
	asked := seeded(t)
	if f := Check(goodApplication(), asked); len(f) != 0 {
		t.Fatalf("a good application: %v", f)
	}
	bad := Input{ContactEmail: "not an email", ContactPhone: "0911", Answers: map[string]string{"storage": "maybe", "learners": "-3", "lead": "yes"},
		Site: Site{Ecosystems: []string{" "}}}
	f := Check(bad, asked)
	for field, want := range map[string]string{
		"institutionName": "Enter the institution’s name.", "contactName": "Enter your name.", "contactRole": "Enter your role at the institution.",
		"contactEmail": "Enter a valid email address.", "contactPhone": "Enter a valid phone number, or leave it empty.",
		"answers.storage": "Choose yes or no.", "answers.learners": "Enter a number.", "answers.connection": "Answer this question.",
		"site.placeName": "Enter the town or area.", "site.country": "Enter the country.", "site.ecosystems": "Name at least one ecosystem.",
		"site.learners": "Say who the learners would be.", "site.studyGoals": "Say what the classroom wants to study.",
	} {
		if f[field] != want {
			t.Errorf("%s: %q", field, f[field])
		}
	}
	if _, ok := f["answers.notes"]; ok || f["answers.lead"] != "" {
		t.Errorf("an optional or good answer refused: %v", f)
	}
	for _, n := range []string{"Infinity", "NaN", "x"} {
		if checkAnswer(Question{Kind: KindNumber}, n) == "" {
			t.Errorf("%s taken as a number", n)
		}
	}
}

func TestClean(t *testing.T) {
	t.Parallel()
	c := clean(goodApplication(), seeded(t))
	if c.InstitutionName != "Lalibela Secondary School" || c.ContactEmail != "meron@lalibela.example" || c.ContactPhone != "+251911234567" ||
		len(c.Answers) != 4 || c.Answers["learners"] != "40" || c.Answers["unasked"] != "" || len(c.Site.Ecosystems) != 1 || c.Site.Ecosystems[0] != "highland" {
		t.Errorf("cleaned: %+v", c)
	}
	if clean(Input{ContactPhone: " "}, nil).ContactPhone != "" {
		t.Error("no phone")
	}
}

// TestApplications: an application keeps only the answers to the questions
// asked, each referring to its question, which keeps it once retired; it
// is found by its status link's token; it is decided once; no classroom
// is created from it before it is accepted, and a failed one leaves none.
// The integration tests create one.
func TestApplications(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.DB(t)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	if _, err := SeedQuestions(ctx, pool, now); err != nil {
		t.Fatal(err)
	}
	a, token, fields, err := Apply(ctx, pool, now, goodApplication())
	if err != nil || len(fields) != 0 || token == "" || len(a.Answers) != 4 {
		t.Fatalf("applied: %+v %v %v", a, fields, err)
	}
	if _, _, fields, _ := Apply(ctx, pool, now, Input{}); fields["contactEmail"] == "" {
		t.Errorf("a bad application stored: %v", fields)
	}
	if err := RetireQuestion(ctx, pool, now, settings.Operator, "learners"); err != nil {
		t.Fatal(err)
	}
	got, err := ByToken(ctx, pool, token)
	if err != nil || got.ID != a.ID || got.Answers["learners"] != "40" || got.Status != StatusSubmitted || got.DecidedAt != nil {
		t.Fatalf("by token: %+v %v", got, err)
	}
	if _, err := ByToken(ctx, pool, "nope"); !errors.Is(err, ErrNoApplication) {
		t.Errorf("a bad token: %v", err)
	}
	later, _, _, _ := Apply(ctx, pool, now.Add(time.Hour), goodApplication())
	if later.Answers["learners"] != "" {
		t.Errorf("a retired question answered: %+v", later.Answers)
	}
	if list, _ := Applications(ctx, pool); len(list) != 2 || list[0].ID != later.ID {
		t.Errorf("newest first: %+v", list)
	}

	by := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, created_at, updated_at) VALUES ($1, now(), now())`, by); err != nil {
		t.Fatal(err)
	}
	if err := CreateClassroom(ctx, pool, a.ID, func(pgx.Tx) (uuid.UUID, error) { return uuid.Nil, nil }); !errors.Is(err, ErrNotAccepted) {
		t.Errorf("a classroom before acceptance: %v", err)
	}
	if d, err := Decide(ctx, pool, now, a.ID, by, true); err != nil || d.Status != StatusAccepted || !d.DecidedAt.Equal(now) {
		t.Fatalf("accepted: %+v %v", d, err)
	}
	if _, err := Decide(ctx, pool, now, a.ID, by, false); !errors.Is(err, ErrDecided) {
		t.Errorf("decided twice: %v", err)
	}
	if _, err := Decide(ctx, pool, now, uuid.New(), by, true); !errors.Is(err, ErrNoApplication) {
		t.Errorf("an unknown application: %v", err)
	}
	failed := errors.New("no classroom")
	if err := CreateClassroom(ctx, pool, a.ID, func(pgx.Tx) (uuid.UUID, error) { return uuid.Nil, failed }); !errors.Is(err, failed) {
		t.Errorf("a failed classroom: %v", err)
	}
	if got, _ := Get(ctx, pool, a.ID); got.ClassroomID != nil {
		t.Errorf("a failed classroom recorded: %+v", got)
	}
}
