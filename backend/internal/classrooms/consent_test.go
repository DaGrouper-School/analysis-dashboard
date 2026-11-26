package classrooms

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

func seedTexts(t *testing.T) ConsentTexts {
	t.Helper()
	texts, problems := jsoncheck.Decode[ConsentTexts](consentSeed)
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	return texts
}

// TestConsentSeed: the built-in first version passes its checks.
func TestConsentSeed(t *testing.T) {
	t.Parallel()
	if p := CheckConsentTexts(seedTexts(t), nil); len(p) > 0 {
		t.Fatal(p)
	}
}

func TestCheckConsentTexts(t *testing.T) {
	t.Parallel()
	en := seedTexts(t)[DefaultLanguage]
	bad := en
	bad.Title, bad.Sections = " ", nil
	bad.OfficeNote = "Record it for {school}."
	got := CheckConsentTexts(ConsentTexts{"am": en, "French": bad}, []string{"en", "so"})
	want := []string{
		"texts: there is no en text",
		"texts: there is no en text, which the current version has",
		"texts: there is no so text, which the current version has",
		`texts.French: "French" is not a language code`,
		"texts.French.sections: empty",
		"texts.French.title: blank",
		"texts.French.officeNote: {school} is not filled in",
	}
	if len(got) != len(want) {
		t.Fatalf("got %q", got)
	}
	for i, w := range want {
		if !strings.Contains(got[i], w) {
			t.Errorf("problem %d: %q lacks %q", i, got[i], w)
		}
	}
}

// TestFill: every placeholder in every part is filled in for the
// classroom.
func TestFill(t *testing.T) {
	t.Parallel()
	c := seedTexts(t)[DefaultLanguage]
	c.Choices.Give, c.Fields = "Yes for {classroom}", []string{"Name at {institution}"}
	got := Fill(c, ConsentDetails{Classroom: "Trinity Weather", Institution: "Trinity School", Decimals: 2})
	for _, f := range jsoncheck.Texts(got, "") {
		if strings.Contains(f.Text, "{") {
			t.Errorf("%s: %q", f.Path, f.Text)
		}
	}
	if !strings.HasPrefix(got.Intro, "Trinity School runs Trinity Weather,") || !strings.Contains(got.Sections[1].Body, "rounded to 2 decimal places") ||
		got.Choices.Give != "Yes for Trinity Weather" || got.Fields[0] != "Name at Trinity School" {
		t.Errorf("filled: %+v", got)
	}
	if c.Choices.Give != "Yes for {classroom}" || c.Fields[0] != "Name at {institution}" {
		t.Error("the text filled in was changed")
	}
}

// TestPublishConsent: each publication is the next version and the
// current one, logged; one that drops a language is refused; a published
// version is never changed or deleted, even by hand.
func TestPublishConsent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.DB(t)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	if _, err := CurrentConsentVersion(ctx, pool); !errors.Is(err, ErrNoConsentText) {
		t.Fatalf("before any: %v", err)
	}
	seeded, err := SeedConsent(ctx, pool, now)
	if err != nil || seeded[0].Outcome != settings.Added {
		t.Fatalf("the seed: %+v, %v", seeded, err)
	}
	if seeded, err = SeedConsent(ctx, pool, now); err != nil || seeded[0].Outcome != settings.Unchanged {
		t.Fatalf("the seed again: %+v, %v", seeded, err)
	}

	texts := seedTexts(t)
	am := texts[DefaultLanguage]
	am.Title = "የአሳዳጊ ፈቃድ"
	texts["am"] = am
	raw, _ := json.Marshal(texts)
	if v, err := PublishConsent(ctx, pool, now.Add(time.Hour), settings.Operator, raw); err != nil || v != 2 {
		t.Fatalf("a second version: %d, %v", v, err)
	}
	delete(texts, "am")
	raw, _ = json.Marshal(texts)
	var invalid *jsoncheck.Invalid
	if _, err := PublishConsent(ctx, pool, now, settings.Operator, raw); !errors.As(err, &invalid) || !strings.Contains(invalid.Problems[0], "no am text") {
		t.Fatalf("a version dropping a language: %v", err)
	}
	if _, err := PublishConsent(ctx, pool, now, settings.Operator, []byte(`{"en": {"title": "x"}}`)); !errors.As(err, &invalid) {
		t.Fatalf("a malformed version: %v", err)
	}

	versions, err := ConsentVersions(ctx, pool)
	if err != nil || len(versions) != 2 || versions[0].Version != 2 || versions[0].PublishedBy != "operator" ||
		strings.Join(versions[0].Languages, ",") != "am,en" || versions[1].PublishedBy != "seed" {
		t.Fatalf("versions: %+v, %v", versions, err)
	}
	version, c, err := CurrentConsent(ctx, pool, "am", ConsentDetails{Classroom: "C", Institution: "I", Decimals: 1})
	if err != nil || version != 2 || c.Title != "የአሳዳጊ ፈቃድ" || !strings.HasPrefix(c.Intro, "I runs C,") {
		t.Fatalf("the current text in Amharic: %d %+v, %v", version, c, err)
	}
	if _, c, _ := CurrentConsent(ctx, pool, "so", ConsentDetails{}); c.Title != seedTexts(t)[DefaultLanguage].Title {
		t.Errorf("a language the version lacks: %q", c.Title)
	}
	if seeded, _ := SeedConsent(ctx, pool, now); seeded[0].Outcome != settings.Kept {
		t.Errorf("the seed after a publication: %+v", seeded)
	}
	log, err := settings.Changes(ctx, pool, 10)
	if err != nil || len(log) != 2 || log[0].Summary != "Published consent text version 2 in am, en" || log[0].Subject != "v2" {
		t.Fatalf("the log: %+v, %v", log, err)
	}

	// Deleting the publisher's account forgets it, keeping their name.
	admin := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, created_at, updated_at) VALUES ($1, now(), now())`, admin); err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(ConsentTexts{DefaultLanguage: am, "am": am})
	if _, err := PublishConsent(ctx, pool, now, settings.Actor{UserID: &admin, Name: "Amara Okafor"}, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, admin); err != nil {
		t.Fatalf("deleting the publisher: %v", err)
	}
	if v, _, err := ConsentText(ctx, pool, 3); err != nil || v.PublishedBy != "Amara Okafor" {
		t.Errorf("after the publisher left: %+v, %v", v, err)
	}
	for _, sql := range []string{
		`UPDATE consent_texts SET published_by = NULL, published_by_name = 'someone' WHERE version = 3`,
		`UPDATE consent_texts SET content = '{}' WHERE version = 1`,
		`DELETE FROM consent_texts WHERE version = 1`,
		`TRUNCATE consent_texts`,
	} {
		if _, err := pool.Exec(ctx, sql); err == nil || !strings.Contains(err.Error(), "never changed or deleted") {
			t.Errorf("%s: %v", sql, err)
		}
	}
}
