package classrooms

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/notify"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// Classroom is a Global Classroom as stored.
type Classroom struct {
	ID              uuid.UUID
	Code            string
	Name            string
	InstitutionName string
	Setup           string
	StageID         string
	RegionID        *string
	Sharing         roles.Sharing
	Terms           settings.DataTerms
	Profile         Profile
	HeadEmail       *string
	CreatedAt       time.Time
	SpaceID         *uuid.UUID
	// Groups are the role groups SAM has confirmed, by role.
	Groups map[roles.Role]uuid.UUID
	// ReleasedThrough is the release cutoff set by hand: records observed
	// up to it are published. Nil while none are.
	ReleasedThrough *time.Time
	// SAMPolicyVersion is the version of the space's data policy SAM
	// holds, once it confirms one.
	SAMPolicyVersion *int
}

// Profile is the front end's ClassroomProfile: what a classroom says about
// itself.
type Profile struct {
	AreaCode            string      `json:"areaCode"`
	Language            string      `json:"language"`
	Description         string      `json:"description"`
	Ecosystems          []string    `json:"ecosystems"`
	HostAgreementSigned bool        `json:"hostAgreementSigned"`
	Projects            []Project   `json:"projects"`
	Sessions            []Session   `json:"sessions"`
	CustodianRole       roles.Role  `json:"custodianRole"`
	Institution         Institution `json:"institution"`
}

// Project is a research project and the sites it measures.
type Project struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Question string `json:"question"`
	Sites    []Site `json:"sites"`
}

// Site is one site of a project.
type Site struct {
	Code         string   `json:"code"`
	Name         string   `json:"name"`
	Ecosystem    string   `json:"ecosystem"`
	ParameterIDs []string `json:"parameterIds"`
	Frequency    string   `json:"frequency"`
	// ReadingsPerDay is 24 hourly, 1 daily, 1/7 weekly.
	ReadingsPerDay float64 `json:"readingsPerDay"`
	YearsPlanned   float64 `json:"yearsPlanned"`
}

// Session is a session on the classroom's calendar.
type Session struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	StartsAt string `json:"startsAt"`
	Place    string `json:"place"`
}

// Institution is the host institution as the classroom presents it.
type Institution struct {
	PhotoURL *string `json:"photoUrl"`
	PhotoAlt string  `json:"photoAlt"`
	History  string  `json:"history"`
	Hub      *string `json:"hub"`
	Website  *string `json:"website"`
}

// newProfile is a new classroom's: it fills in as the classroom describes
// itself, with no host agreement yet.
func newProfile() Profile {
	return Profile{Ecosystems: []string{}, Projects: []Project{}, Sessions: []Session{}, CustodianRole: roles.Head}
}

// The setups of a classroom.
const (
	SetupSelfService   = "self_service"
	SetupAdminAssisted = "admin_assisted"
)

// codeAlphabet are the characters of a classroom code: no 0, O, 1, I or L,
// which are read for one another.
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// newCode is a code such as DGC-HG2W5K.
func newCode() string {
	b := make([]byte, 6)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(codeAlphabet))))
		b[i] = codeAlphabet[n.Int64()]
	}
	return "DGC-" + string(b)
}

// ErrNoStage means there is no stage for a new classroom to start at.
var ErrNoStage = errors.New("there is no stage for a classroom to start at: run dgs seed")

// CreateInput is the front end's CreateClassroomInput.
type CreateInput struct {
	Name            string  `json:"name"`
	InstitutionName string  `json:"institutionName"`
	HeadEmail       string  `json:"headEmail"`
	ApplicationID   *string `json:"applicationId,omitempty"`
}

// CheckCreate reports what is wrong with a new classroom, by field, as the
// front end checks it.
func CheckCreate(in CreateInput) map[string]string {
	f := checkNames(in.Name, in.InstitutionName)
	switch email := strings.TrimSpace(in.HeadEmail); {
	case email == "":
		f["headEmail"] = "Enter the institution head's email."
	case !validEmail(email):
		f["headEmail"] = "Enter a valid email address."
	}
	return f
}

func checkNames(name, institution string) map[string]string {
	f := map[string]string{}
	if utf8.RuneCountInString(strings.TrimSpace(name)) < 3 {
		f["name"] = "Enter a classroom name."
	}
	if utf8.RuneCountInString(strings.TrimSpace(institution)) < 2 {
		f["institutionName"] = "Enter the institution name."
	}
	return f
}

func validEmail(s string) bool { return notify.ValidAddress(s) }

// Kind is the outbox kind that creates a classroom in SAM.
const Kind = "classroom"

func entity(id uuid.UUID) string { return "classroom:" + id.String() }

// Create stores a new classroom set up by a platform admin, at the first
// stage, with the default sharing settings and the platform's default
// terms filled in for it, and sends it to SAM through the outbox. The head
// is invited by email once invitations are sent.
func Create(ctx context.Context, q db.Querier, now time.Time, by uuid.UUID, in CreateInput) (Classroom, error) {
	c := Classroom{ID: uuid.New(), Name: strings.TrimSpace(in.Name), InstitutionName: strings.TrimSpace(in.InstitutionName),
		Setup: SetupAdminAssisted, Sharing: roles.DefaultSharing, Profile: newProfile(), CreatedAt: now, Groups: map[roles.Role]uuid.UUID{}}
	email := strings.ToLower(strings.TrimSpace(in.HeadEmail))
	c.HeadEmail = &email
	err := pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		stages, err := Stages(ctx, tx)
		if err != nil {
			return err
		}
		if len(stages) == 0 {
			return ErrNoStage
		}
		c.StageID = stages[0].ID
		if c.Terms, err = defaultTerms(ctx, tx, c); err != nil {
			return err
		}
		for range 10 {
			c.Code = newCode()
			err = insert(ctx, tx, c, by)
			var pe *pgconn.PgError
			if !errors.As(err, &pe) || pe.Code != "23505" || pe.ConstraintName != "classrooms_code_key" {
				break
			}
		}
		if err != nil {
			return err
		}
		if _, err := samaccess.Enqueue(ctx, tx, now, samaccess.Change{Entity: entity(c.ID), Kind: Kind,
			Payload: desired{Sharing: c.Sharing, Decimals: c.Terms.CoordinateDecimals}}); err != nil {
			return err
		}
		// The terms reach SAM as the space's data policy, set as the
		// platform admin who created it until the classroom has a head.
		if by == uuid.Nil {
			return nil
		}
		return queuePolicy(ctx, tx, now, c.ID, by)
	})
	return c, err
}

// desired is what SAM should hold of a classroom: its space, its four role
// groups, and the grants they hold under its sharing settings.
type desired struct {
	Sharing roles.Sharing `json:"sharing"`
	// Decimals are the classroom's coordinate decimals, which the public
	// grants' grid follows.
	Decimals *int `json:"decimals,omitempty"`
}

// defaultTerms are the platform's default data terms, with the attribution
// filled in for c.
func defaultTerms(ctx context.Context, q db.Querier, c Classroom) (settings.DataTerms, error) {
	var t settings.DataTerms
	e, err := settings.Get(ctx, q, settings.KeyDefaultTerms)
	if err != nil {
		return t, fmt.Errorf("the default data terms: %w", err)
	}
	if err := json.Unmarshal(e.Value, &t); err != nil {
		return t, err
	}
	t.Attribution = strings.NewReplacer("{classroom}", c.Name, "{institution}", c.InstitutionName).Replace(t.Attribution)
	if t.Restrictions == nil {
		t.Restrictions = []string{}
	}
	return t, nil
}

// UpdateInput is the front end's UpdateClassroomInput.
type UpdateInput struct {
	Name            string  `json:"name"`
	InstitutionName string  `json:"institutionName"`
	History         *string `json:"history,omitempty"`
}

// MaxHistory is the most an institution's history holds.
const MaxHistory = 2000

// CheckUpdate reports what is wrong with a change to a classroom, by field.
func CheckUpdate(in UpdateInput) map[string]string {
	f := checkNames(in.Name, in.InstitutionName)
	if in.History != nil && utf8.RuneCountInString(strings.TrimSpace(*in.History)) > MaxHistory {
		f["history"] = fmt.Sprintf("Keep the description under %d characters.", MaxHistory)
	}
	return f
}

// ErrNoClassroom means there is no such classroom.
var ErrNoClassroom = errors.New("no such classroom")

// Update changes a classroom's name, institution and history. Its code
// never changes.
func Update(ctx context.Context, q db.Querier, now time.Time, id uuid.UUID, in UpdateInput) error {
	return pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		c, err := lock(ctx, tx, id)
		if err != nil {
			return err
		}
		c.Name, c.InstitutionName = strings.TrimSpace(in.Name), strings.TrimSpace(in.InstitutionName)
		if in.History != nil {
			c.Profile.Institution.History = strings.TrimSpace(*in.History)
		}
		return save(ctx, tx, now, c)
	})
}

// configurable are the profile's fields admins configure. The photo is set
// on its own.
var configurable = []string{"areaCode", "language", "description", "ecosystems", "hostAgreementSigned", "projects", "sessions",
	"custodianRole", "institution"}

// InstitutionText is what admins configure of the institution.
type InstitutionText struct {
	History string  `json:"history"`
	Hub     *string `json:"hub"`
	Website *string `json:"website"`
}

var (
	areaCode  = regexp.MustCompile(`^[A-Z]{2}(-[A-Z0-9]{1,3})?$`)
	language  = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z]{2})?$`)
	projectID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

// Configure sets a classroom's stage, region and profile fields from raw,
// an object of any of stageId, regionId (null clears it) and profile, whose
// fields replace their own value only. Every value is checked before
// anything is stored, and the change is logged as by's.
func Configure(ctx context.Context, q db.Querier, now time.Time, by settings.Actor, id uuid.UUID, raw []byte) error {
	fields, problems := jsoncheck.Decode[map[string]json.RawMessage](raw)
	if err := jsoncheck.Refuse("the classroom's configuration", problems); err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		c, err := lock(ctx, tx, id)
		if err != nil {
			return err
		}
		changed, problems := configure(ctx, tx, &c, fields)
		if err := jsoncheck.Refuse("the classroom's configuration", problems); err != nil {
			return err
		}
		if len(changed) == 0 {
			return nil
		}
		if err := save(ctx, tx, now, c); err != nil {
			return err
		}
		return settings.Record(ctx, tx, now, by, settings.Change{Kind: settings.KindClassroom, Subject: c.Code,
			Summary: fmt.Sprintf("Configured %s: %s", c.Code, strings.Join(changed, ", "))})
	})
}

// configure applies fields to c, and returns what changed or the problems
// found.
func configure(ctx context.Context, q db.Querier, c *Classroom, fields map[string]json.RawMessage) ([]string, []string) {
	var changed, problems []string
	for _, name := range sortedKeys(fields) {
		raw := fields[name]
		switch name {
		case "stageId":
			id, p := decodeAs[string](name, raw)
			if p == nil {
				p = exists(ctx, q, "stages", name, id)
			}
			if problems = append(problems, p...); p == nil {
				c.StageID = id
			}
		case "regionId":
			id, p := decodeAs[*string](name, raw)
			if p == nil && id != nil {
				p = exists(ctx, q, "regions", name, *id)
			}
			if problems = append(problems, p...); p == nil {
				c.RegionID = id
			}
		case "profile":
			p := configureProfile(&c.Profile, raw)
			problems = append(problems, p...)
		default:
			problems = append(problems, name+": not a field here; use stageId, regionId or profile")
			continue
		}
		changed = append(changed, name)
	}
	return changed, problems
}

// configureProfile replaces each profile field raw names with its value.
func configureProfile(p *Profile, raw json.RawMessage) []string {
	fields, problems := decodeAs[map[string]json.RawMessage]("profile", raw)
	if len(problems) > 0 {
		return problems
	}
	for _, name := range sortedKeys(fields) {
		at, v := "profile."+name, fields[name]
		var p2 []string
		switch name {
		case "areaCode":
			p.AreaCode, p2 = setMatching(at, v, areaCode, "an area code such as ET-AA")
		case "language":
			p.Language, p2 = setMatching(at, v, language, "a language code such as en")
		case "description":
			p.Description, p2 = decodeAs[string](at, v)
		case "ecosystems":
			p.Ecosystems, p2 = decodeAs[[]string](at, v)
			p2 = append(p2, jsoncheck.Blank(p.Ecosystems, at)...)
		case "hostAgreementSigned":
			p.HostAgreementSigned, p2 = decodeAs[bool](at, v)
		case "projects":
			p.Projects, p2 = decodeAs[[]Project](at, v)
			p2 = append(p2, checkProjects(at, p.Projects)...)
		case "sessions":
			p.Sessions, p2 = decodeAs[[]Session](at, v)
			p2 = append(p2, checkSessions(at, p.Sessions)...)
		case "custodianRole":
			p.CustodianRole, p2 = decodeAs[roles.Role](at, v)
			if p2 == nil && !slices.Contains(roles.All, p.CustodianRole) {
				p2 = []string{fmt.Sprintf("%s: %q is not a role", at, p.CustodianRole)}
			}
		case "institution":
			var t InstitutionText
			if t, p2 = decodeAs[InstitutionText](at, v); p2 == nil {
				if t.Website != nil && !jsoncheck.HTTPSURL(*t.Website) {
					p2 = append(p2, fmt.Sprintf("%s.website: %q is not an https URL", at, *t.Website))
				}
				p.Institution.History, p.Institution.Hub, p.Institution.Website = t.History, t.Hub, t.Website
			}
		default:
			p2 = []string{fmt.Sprintf("%s: not a field here; use %s", at, strings.Join(configurable, ", "))}
		}
		problems = append(problems, p2...)
	}
	return problems
}

// decodeAs decodes v strictly as a T, its problems named under at.
func decodeAs[T any](at string, v json.RawMessage) (T, []string) {
	out, problems := jsoncheck.Decode[T](v)
	for i, p := range problems {
		problems[i] = under(at, p)
	}
	if len(problems) == 0 {
		return out, nil
	}
	return out, problems
}

// under is a problem from decoding a value named at: "items[0].x: bad"
// becomes "at.items[0].x: bad", and "the value is null" becomes "at: the
// value is null".
func under(at, problem string) string {
	if path, msg, ok := strings.Cut(problem, ": "); ok && !strings.Contains(path, " ") {
		if strings.HasPrefix(path, "[") {
			return at + path + ": " + msg
		}
		return at + "." + path + ": " + msg
	}
	return at + ": " + problem
}

func setMatching(at string, v json.RawMessage, re *regexp.Regexp, what string) (string, []string) {
	s, p := decodeAs[string](at, v)
	if p == nil && s != "" && !re.MatchString(s) {
		p = []string{fmt.Sprintf("%s: %q is not %s", at, s, what)}
	}
	return s, p
}

func checkProjects(at string, list []Project) []string {
	p := jsoncheck.IDs(at, list, func(x Project) string { return x.ID }, projectID)
	for i, x := range list {
		for j, s := range x.Sites {
			where := fmt.Sprintf("%s[%d].sites[%d]", at, i, j)
			if s.ReadingsPerDay <= 0 || s.YearsPlanned < 0 {
				p = append(p, where+": readingsPerDay is above zero, and yearsPlanned not below it")
			}
		}
	}
	return append(p, jsoncheck.Blank(list, at, "question")...)
}

func checkSessions(at string, list []Session) []string {
	p := jsoncheck.IDs(at, list, func(s Session) string { return s.ID }, projectID)
	for i, s := range list {
		if _, err := time.Parse(time.RFC3339, s.StartsAt); err != nil {
			p = append(p, fmt.Sprintf("%s[%d].startsAt: %q is not a time such as 2026-12-01T09:00:00Z", at, i, s.StartsAt))
		}
	}
	return append(p, jsoncheck.Blank(list, at)...)
}

func sortedKeys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
