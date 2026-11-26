package classrooms

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// ConsentContent is the front end's ConsentSheetContent: the words of the
// guardian consent sheet, with {classroom}, {institution} and {decimals}
// where each classroom's details go.
type ConsentContent struct {
	Title    string           `json:"title"`
	Intro    string           `json:"intro"`
	Sections []ConsentSection `json:"sections"`
	Choices  struct {
		Give     string `json:"give"`
		Withdraw string `json:"withdraw"`
	} `json:"choices"`
	ContributorField string   `json:"contributorField"`
	Fields           []string `json:"fields"`
	OfficeNote       string   `json:"officeNote"`
}

// ConsentSection is one section of the sheet.
type ConsentSection struct {
	Heading string `json:"heading"`
	Body    string `json:"body"`
}

// ConsentTexts are one version's words, by language.
type ConsentTexts map[string]ConsentContent

// DefaultLanguage is the language every version has, which a reader whose
// language a version lacks is shown.
const DefaultLanguage = "en"

// The placeholders a consent text may hold, filled in for each classroom.
var consentPlaceholders = []string{"classroom", "institution", "decimals"}

// languageCode is a language as the front end names it, such as en or am.
var languageCode = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z]{2})?$`)

// CheckConsentTexts reports what is wrong with a version's words, given
// the languages of the current one, which it keeps: every language, the
// default among them, has every part, and only the placeholders filled in.
func CheckConsentTexts(texts ConsentTexts, current []string) []string {
	var p []string
	if _, ok := texts[DefaultLanguage]; !ok {
		p = append(p, fmt.Sprintf("texts: there is no %s text", DefaultLanguage))
	}
	for _, lang := range current {
		if _, ok := texts[lang]; !ok {
			p = append(p, fmt.Sprintf("texts: there is no %s text, which the current version has", lang))
		}
	}
	for _, lang := range sortedLanguages(texts) {
		c, at := texts[lang], "texts."+lang
		if !languageCode.MatchString(lang) {
			p = append(p, fmt.Sprintf("%s: %q is not a language code such as en or am", at, lang))
		}
		if len(c.Sections) == 0 {
			p = append(p, at+".sections: empty")
		}
		p = append(p, jsoncheck.Blank(c, at)...)
		p = append(p, jsoncheck.Placeholders(at, c, consentPlaceholders...)...)
	}
	return p
}

func sortedLanguages(texts ConsentTexts) []string {
	out := make([]string, 0, len(texts))
	for lang := range texts {
		out = append(out, lang)
	}
	slices.Sort(out)
	return out
}

// ConsentDetails are what a classroom fills a consent text in with.
type ConsentDetails struct {
	Classroom, Institution string
	// Decimals are the decimal places a minor's positions are rounded to.
	Decimals int
}

// Fill is c with every placeholder replaced by the classroom's details.
func Fill(c ConsentContent, d ConsentDetails) ConsentContent {
	r := strings.NewReplacer("{classroom}", d.Classroom, "{institution}", d.Institution, "{decimals}", strconv.Itoa(d.Decimals))
	out := c
	out.Title, out.Intro = r.Replace(c.Title), r.Replace(c.Intro)
	out.Sections = make([]ConsentSection, len(c.Sections))
	for i, s := range c.Sections {
		out.Sections[i] = ConsentSection{Heading: r.Replace(s.Heading), Body: r.Replace(s.Body)}
	}
	out.Choices.Give, out.Choices.Withdraw = r.Replace(c.Choices.Give), r.Replace(c.Choices.Withdraw)
	out.ContributorField, out.OfficeNote = r.Replace(c.ContributorField), r.Replace(c.OfficeNote)
	out.Fields = make([]string, len(c.Fields))
	for i, f := range c.Fields {
		out.Fields[i] = r.Replace(f)
	}
	return out
}

// ConsentVersion is a published version.
type ConsentVersion struct {
	Version     int       `json:"version"`
	PublishedAt time.Time `json:"publishedAt"`
	PublishedBy string    `json:"publishedBy"`
	Languages   []string  `json:"languages"`
}

// ErrNoConsentText means no version is published yet.
var ErrNoConsentText = errors.New("no consent text is published")

// ErrConsentRace means another version was published at the same moment.
var ErrConsentRace = errors.New("another consent text was published at the same time")

// ConsentVersions are the published versions, newest first.
func ConsentVersions(ctx context.Context, q db.Querier) ([]ConsentVersion, error) {
	rows, err := q.Query(ctx, `
		SELECT version, min(published_at), min(published_by_name), array_agg(language ORDER BY language)
		FROM consent_texts GROUP BY version ORDER BY version DESC`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (v ConsentVersion, err error) {
		return v, r.Scan(&v.Version, &v.PublishedAt, &v.PublishedBy, &v.Languages)
	})
}

// ConsentText is one version's words in every language, or
// ErrNoConsentText.
func ConsentText(ctx context.Context, q db.Querier, version int) (ConsentVersion, ConsentTexts, error) {
	rows, err := q.Query(ctx, `SELECT language, content, published_at, published_by_name FROM consent_texts WHERE version = $1 ORDER BY language`, version)
	if err != nil {
		return ConsentVersion{}, nil, err
	}
	defer rows.Close()
	v, texts := ConsentVersion{Version: version, Languages: []string{}}, ConsentTexts{}
	for rows.Next() {
		var lang string
		var raw []byte
		if err := rows.Scan(&lang, &raw, &v.PublishedAt, &v.PublishedBy); err != nil {
			return v, nil, err
		}
		var c ConsentContent
		if err := json.Unmarshal(raw, &c); err != nil {
			return v, nil, fmt.Errorf("consent text %d in %s: %w", version, lang, err)
		}
		texts[lang], v.Languages = c, append(v.Languages, lang)
	}
	if err := rows.Err(); err != nil {
		return v, nil, err
	}
	if len(texts) == 0 {
		return v, nil, ErrNoConsentText
	}
	return v, texts, nil
}

// CurrentConsentVersion is the latest version's number, or
// ErrNoConsentText.
func CurrentConsentVersion(ctx context.Context, q db.Querier) (int, error) {
	var v *int
	if err := q.QueryRow(ctx, `SELECT max(version) FROM consent_texts`).Scan(&v); err != nil {
		return 0, err
	}
	if v == nil {
		return 0, ErrNoConsentText
	}
	return *v, nil
}

// CurrentConsent is the current version filled in for a classroom in
// language, or in the default language when the version lacks it.
func CurrentConsent(ctx context.Context, q db.Querier, language string, d ConsentDetails) (int, ConsentContent, error) {
	version, err := CurrentConsentVersion(ctx, q)
	if err != nil {
		return 0, ConsentContent{}, err
	}
	_, texts, err := ConsentText(ctx, q, version)
	if err != nil {
		return 0, ConsentContent{}, err
	}
	c, ok := texts[language]
	if !ok {
		c = texts[DefaultLanguage]
	}
	return version, Fill(c, d), nil
}

// PublishConsent checks a version's words and publishes them as the next
// version, logging it, all in one transaction. It returns the version.
func PublishConsent(ctx context.Context, q db.Querier, now time.Time, by settings.Actor, raw []byte) (int, error) {
	texts, problems := jsoncheck.Decode[ConsentTexts](raw)
	if len(problems) > 0 {
		return 0, jsoncheck.Refuse("the consent text", problems)
	}
	var version int
	err := pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		current, err := CurrentConsentVersion(ctx, tx)
		var languages []string
		switch {
		case errors.Is(err, ErrNoConsentText):
		case err != nil:
			return err
		default:
			v, _, err := ConsentText(ctx, tx, current)
			if err != nil {
				return err
			}
			languages = v.Languages
		}
		if err := jsoncheck.Refuse("the consent text", CheckConsentTexts(texts, languages)); err != nil {
			return err
		}
		version = current + 1
		for _, lang := range sortedLanguages(texts) {
			content, _ := json.Marshal(texts[lang])
			if _, err := tx.Exec(ctx, `
				INSERT INTO consent_texts (version, language, content, published_at, published_by, published_by_name)
				VALUES ($1, $2, $3, $4, $5, $6)`, version, lang, content, now, by.UserID, by.Name); err != nil {
				return err
			}
		}
		return settings.Record(ctx, tx, now, by, settings.Change{Kind: settings.KindConsentText, Subject: "v" + strconv.Itoa(version),
			Summary: fmt.Sprintf("Published consent text version %d in %s", version, strings.Join(sortedLanguages(texts), ", "))})
	})
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return 0, ErrConsentRace
	}
	return version, err
}

// consentSeed is the first version's words, from the front end's seed
// (frontend/src/lib/data/seed.ts), built into the binary.
//
//go:embed consent.json
var consentSeed []byte

// SeedConsent publishes the first version while none is published, as the
// seed's. One published since is kept.
func SeedConsent(ctx context.Context, q db.Querier, now time.Time) ([]settings.Seeded, error) {
	what := "consent text"
	versions, err := ConsentVersions(ctx, q)
	switch {
	case err != nil:
		return nil, err
	case len(versions) == 1 && versions[0].PublishedBy == settings.Seed.Name:
		return []settings.Seeded{{What: what, Outcome: settings.Unchanged}}, nil
	case len(versions) > 0:
		return []settings.Seeded{{What: what, Outcome: settings.Kept}}, nil
	}
	if _, err := PublishConsent(ctx, q, now, settings.Seed, consentSeed); err != nil {
		return nil, fmt.Errorf("the consent text's seed: %w", err)
	}
	return []settings.Seeded{{What: what, Outcome: settings.Added}}, nil
}
