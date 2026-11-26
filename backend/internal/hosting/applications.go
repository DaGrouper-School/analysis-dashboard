package hosting

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/notify"
)

// Site is where the applicant's classroom would be.
type Site struct {
	PlaceName  string   `json:"placeName"`
	Country    string   `json:"country"`
	Ecosystems []string `json:"ecosystems"`
	Learners   string   `json:"learners"`
	StudyGoals string   `json:"studyGoals"`
}

// Input is the front end's HostApplicationInput.
type Input struct {
	InstitutionName string            `json:"institutionName"`
	ContactName     string            `json:"contactName"`
	ContactRole     string            `json:"contactRole"`
	ContactEmail    string            `json:"contactEmail"`
	ContactPhone    string            `json:"contactPhone"`
	Answers         map[string]string `json:"answers"`
	Site            Site              `json:"site"`
}

// The statuses of an application.
const (
	StatusSubmitted = "submitted"
	StatusAccepted  = "accepted"
	StatusDeclined  = "declined"
)

// Application is the front end's HostApplication.
type Application struct {
	Input
	ID          uuid.UUID  `json:"id"`
	Status      string     `json:"status"`
	SubmittedAt time.Time  `json:"submittedAt"`
	DecidedAt   *time.Time `json:"decidedAt"`
	ClassroomID *uuid.UUID `json:"classroomId"`
}

// Errors of applications.
var (
	ErrNoApplication = errors.New("no such application")
	ErrDecided       = errors.New("the application has been decided")
	ErrNotAccepted   = errors.New("the application is not accepted")
	ErrHasClassroom  = errors.New("a classroom was created from the application")
)

// Check reports what is wrong with an application, by field, as the front
// end's validateHostApplication does: contactEmail, answers.<question>,
// site.placeName and so on.
func Check(in Input, questions []Question) map[string]string {
	f := map[string]string{}
	blank := func(field, value, msg string) {
		if strings.TrimSpace(value) == "" {
			f[field] = msg
		}
	}
	blank("institutionName", in.InstitutionName, "Enter the institution’s name.")
	blank("contactName", in.ContactName, "Enter your name.")
	blank("contactRole", in.ContactRole, "Enter your role at the institution.")
	switch email := strings.TrimSpace(in.ContactEmail); {
	case email == "":
		f["contactEmail"] = "Enter your email."
	case !notify.ValidAddress(email):
		f["contactEmail"] = "Enter a valid email address."
	}
	if phone := strings.TrimSpace(in.ContactPhone); phone != "" && accounts.NormalizePhone(phone) == "" {
		f["contactPhone"] = "Enter a valid phone number, or leave it empty."
	}
	for _, q := range questions {
		if msg := checkAnswer(q, strings.TrimSpace(in.Answers[q.ID])); msg != "" {
			f["answers."+q.ID] = msg
		}
	}
	blank("site.placeName", in.Site.PlaceName, "Enter the town or area.")
	blank("site.country", in.Site.Country, "Enter the country.")
	if len(cleanList(in.Site.Ecosystems)) == 0 {
		f["site.ecosystems"] = "Name at least one ecosystem."
	}
	blank("site.learners", in.Site.Learners, "Say who the learners would be.")
	blank("site.studyGoals", in.Site.StudyGoals, "Say what the classroom wants to study.")
	return f
}

func checkAnswer(q Question, answer string) string {
	switch {
	case answer == "":
		if q.Required {
			return "Answer this question."
		}
	case q.Kind == KindYesNo && answer != "yes" && answer != "no":
		return "Choose yes or no."
	case q.Kind == KindNumber:
		if n, err := strconv.ParseFloat(answer, 64); err != nil || n < 0 || math.IsInf(n, 0) || math.IsNaN(n) {
			return "Enter a number."
		}
	}
	return ""
}

func cleanList(list []string) []string {
	out := []string{}
	for _, s := range list {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// clean keeps only what the form asks, trimmed: the email lower-cased, the
// phone in E.164, and the answers to the questions asked.
func clean(in Input, questions []Question) Input {
	out := Input{InstitutionName: strings.TrimSpace(in.InstitutionName), ContactName: strings.TrimSpace(in.ContactName),
		ContactRole: strings.TrimSpace(in.ContactRole), ContactEmail: strings.ToLower(strings.TrimSpace(in.ContactEmail)),
		ContactPhone: accounts.NormalizePhone(in.ContactPhone), Answers: map[string]string{},
		Site: Site{PlaceName: strings.TrimSpace(in.Site.PlaceName), Country: strings.TrimSpace(in.Site.Country),
			Ecosystems: cleanList(in.Site.Ecosystems), Learners: strings.TrimSpace(in.Site.Learners), StudyGoals: strings.TrimSpace(in.Site.StudyGoals)}}
	for _, q := range questions {
		if a := strings.TrimSpace(in.Answers[q.ID]); a != "" {
			out.Answers[q.ID] = a
		}
	}
	return out
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Apply stores an application, checked against the questions asked now,
// and returns it with its status link's token, which only it carries.
func Apply(ctx context.Context, q db.Querier, now time.Time, in Input) (Application, string, map[string]string, error) {
	var a Application
	token := newToken()
	var fields map[string]string
	err := pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		asked, err := questions(ctx, tx)
		if err != nil {
			return err
		}
		if fields = Check(in, asked); len(fields) > 0 {
			return nil
		}
		a = Application{Input: clean(in, asked), ID: uuid.New(), Status: StatusSubmitted, SubmittedAt: now}
		return insertApplication(ctx, tx, a, hashToken(token))
	})
	return a, token, fields, err
}

// ByToken is the application whose status link has token.
func ByToken(ctx context.Context, q db.Querier, token string) (Application, error) {
	return oneApplication(ctx, q, `token_hash = $1`, hashToken(token))
}

// Get is the application with id.
func Get(ctx context.Context, q db.Querier, id uuid.UUID) (Application, error) {
	return oneApplication(ctx, q, `id = $1`, id)
}

// Applications are every application, newest first.
func Applications(ctx context.Context, q db.Querier) ([]Application, error) {
	return applications(ctx, q, `true ORDER BY submitted_at DESC, id`)
}

// Decide accepts or declines a submitted application, as by decided.
func Decide(ctx context.Context, q db.Querier, now time.Time, id, by uuid.UUID, accept bool) (Application, error) {
	var a Application
	err := pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		var err error
		if a, err = lockApplication(ctx, tx, id); err != nil {
			return err
		}
		if a.Status != StatusSubmitted {
			return ErrDecided
		}
		a.Status, a.DecidedAt = StatusDeclined, &now
		if accept {
			a.Status = StatusAccepted
		}
		_, err = tx.Exec(ctx, `UPDATE host_applications SET status = $2, decided_at = $3, decided_by = $4 WHERE id = $1`, id, a.Status, now, by)
		return err
	})
	return a, err
}

// CreateClassroom runs create, which makes the classroom an accepted
// application is set up as, within q's transaction, and records it on the
// application. An application gets one classroom.
func CreateClassroom(ctx context.Context, q db.Querier, id uuid.UUID, create func(pgx.Tx) (uuid.UUID, error)) error {
	return pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		a, err := lockApplication(ctx, tx, id)
		switch {
		case err != nil:
			return err
		case a.ClassroomID != nil:
			return ErrHasClassroom
		case a.Status != StatusAccepted:
			return ErrNotAccepted
		}
		classroom, err := create(tx)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE host_applications SET classroom_id = $2 WHERE id = $1`, id, classroom)
		return err
	})
}

// StatusText is the email with an applicant's status link.
func StatusText(a Application, link string) (subject, text string) {
	subject = "Your application to host a Global Classroom"
	text = fmt.Sprintf(`Hello %s,

Thank you for applying for %s to host a Global Classroom. We will review your application. See where it stands at any time:

%s

Keep this link to yourself: anyone who has it can see your application.
`, a.ContactName, a.InstitutionName, link)
	return subject, text
}
