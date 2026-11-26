package consent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/notify"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

func TestCheckGuardian(t *testing.T) {
	t.Parallel()
	for _, good := range []string{" Parent@Home.example ", "+251 911 234 567", "00251911234567"} {
		if f := CheckGuardian(GuardianInput{GuardianName: "Almaz", GuardianContact: good}); len(f) != 0 {
			t.Errorf("%q: %v", good, f)
		}
	}
	f := CheckGuardian(GuardianInput{GuardianName: " ", GuardianContact: "0911"})
	if f["guardianName"] != "Enter your guardian’s name." || f["guardianContact"] != "Enter a valid email address, or a phone number with country code." {
		t.Errorf("bad: %v", f)
	}
	if f := CheckGuardian(GuardianInput{}); f["guardianContact"] != "Enter their email address or phone number." {
		t.Errorf("blank: %v", f)
	}
	if c, email := Contact(" Parent@Home.example "); c != "parent@home.example" || !email {
		t.Errorf("an email: %q %v", c, email)
	}
	if c, email := Contact("+251 911 234 567"); c != "+251911234567" || email {
		t.Errorf("a phone: %q %v", c, email)
	}
}

func TestWords(t *testing.T) {
	t.Parallel()
	if got := joinNames([]string{"A", "B", "C"}, "none"); got != "A, B and C" {
		t.Errorf("three: %s", got)
	}
	if joinNames(nil, "none") != "none" || joinNames([]string{"A"}, "") != "A" {
		t.Error("none or one")
	}
	if firstName(" Sara  Tadesse ") != "Sara" || firstName("") != "Your child" {
		t.Error("first names")
	}
}

// TestNoTexts: a guardian named by phone where no SMS provider is set up
// is refused on the contact, saying to use an email.
func TestNoTexts(t *testing.T) {
	t.Parallel()
	m := New(nil, nil, time.Now, Config{})
	err := m.send(context.Background(), accounts.User{FullName: "Sara"}, Link{GuardianContact: "+251911234567"}, "t", false)
	var e *httpx.Error
	if !errors.As(err, &e) || e.Fields["guardianContact"] != msgNoTexts {
		t.Errorf("%v", err)
	}
	if err := New(nil, nil, time.Now, Config{SMS: &notify.SMSRecorder{}}).send(context.Background(), accounts.User{}, Link{}, "t", false); err != nil {
		t.Errorf("texted: %v", err)
	}
}

// TestLinks: asking voids the earlier link and makes consent pending; a
// decision against the current text is recorded once and sets consent; a
// decision against an older text is refused.
func TestLinks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.DB(t)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	if _, err := settings.SeedSettings(ctx, pool, now); err != nil {
		t.Fatal(err)
	}
	if _, err := classrooms.SeedConsent(ctx, pool, now); err != nil {
		t.Fatal(err)
	}
	user := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, full_name, created_at, updated_at) VALUES ($1, 'Sara Tadesse', now(), now())`, user); err != nil {
		t.Fatal(err)
	}
	ask := func(contact string) (Link, string) {
		t.Helper()
		var l Link
		var token string
		if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			var err error
			l, token, err = Ask(ctx, tx, now, user, GuardianInput{GuardianName: " Almaz ", GuardianContact: contact})
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return l, token
	}
	first, firstToken := ask("Almaz@Home.example")
	if first.GuardianName != "Almaz" || first.GuardianContact != "almaz@home.example" {
		t.Errorf("the link: %+v", first)
	}
	u, _ := accounts.Get(ctx, pool, user)
	if u.Minor == nil || !*u.Minor || u.Consent == nil || *u.Consent != accounts.ConsentPending {
		t.Fatalf("a minor awaiting consent: %+v", u)
	}
	current, err := classrooms.CurrentConsentVersion(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decide(ctx, pool, now, firstToken, true, current-1, current, 4); !errors.Is(err, ErrTextChanged) {
		t.Errorf("an older text: %v", err)
	}
	if _, err := Decide(ctx, pool, now, firstToken, true, current, current, 4); err != nil {
		t.Fatal(err)
	}
	if _, err := Decide(ctx, pool, now.Add(time.Hour), firstToken, true, current, current, 4); err != nil {
		t.Fatal(err)
	}
	r, found, _ := LatestRecord(ctx, pool, user, &first.ID)
	if !found || r.Decision != accounts.ConsentGiven || r.TextVersion != current || r.Decimals != 4 || !r.DecidedAt.Equal(now) {
		t.Errorf("recorded once: %+v %v", r, found)
	}
	if u, _ := accounts.Get(ctx, pool, user); *u.Consent != accounts.ConsentGiven {
		t.Errorf("consent: %s", *u.Consent)
	}

	second, secondToken := ask("+251 911 234 567")
	if _, err := ByToken(ctx, pool, firstToken); !errors.Is(err, ErrNoLink) {
		t.Errorf("the first link after asking again: %v", err)
	}
	if _, err := Decide(ctx, pool, now, firstToken, false, current, current, 4); !errors.Is(err, ErrNoLink) {
		t.Errorf("deciding on a voided link: %v", err)
	}
	if l, err := LiveLink(ctx, pool, user); err != nil || l.ID != second.ID || l.GuardianContact != "+251911234567" {
		t.Errorf("the live link: %+v %v", l, err)
	}
	if u, _ := accounts.Get(ctx, pool, user); *u.Consent != accounts.ConsentPending {
		t.Errorf("asking again leaves consent %s", *u.Consent)
	}
	if _, found, _ := LatestRecord(ctx, pool, user, &second.ID); found {
		t.Error("the new link has a decision")
	}
	if _, err := Decide(ctx, pool, now, secondToken, false, current, current, 4); err != nil {
		t.Fatal(err)
	}
	if u, _ := accounts.Get(ctx, pool, user); *u.Consent != accounts.ConsentWithdrawn {
		t.Errorf("withdrawn: %s", *u.Consent)
	}
}
