package labs

import (
	"context"
	"crypto/rand"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// The raw data table as a CSV file: SAM's export, each row rewritten into
// the front end's columns with what SAM does not know (labels, source,
// site, classroom and participant link) and the policy SAM puts on it.

// CSVColumns are the export's columns, the front end's, then the
// licences the reader's grants attach.
var CSVColumns = []string{"observed_at", "received_at", "parameter_id", "parameter", "value", "unit", "source", "source_type", "site",
	"classroom", "latitude", "longitude", "location_source", "participant_id", "participant_url", "licence", "licence_url",
	"attribution", "restrictions", "licences"}

// CSVRow is one row of the export: a table row, and what goes beside it.
type CSVRow struct {
	RawRow
	Parameter, Unit string
	Classroom       string
	ParticipantURL  *string
	// Terms are the policy the row carries in SAM; the classroom's own
	// before SAM holds one.
	Terms    settings.DataTerms
	Licences []string
}

// RowWriter takes an export's rows as they are made, so the file can be
// written to a body or streamed without changing how rows are made.
type RowWriter interface {
	Write(CSVRow) error
	// Close finishes the file.
	Close() error
}

// csvFile writes CSV as the front end does: CRLF lines, quoted where a
// field holds a comma, quote or line break, and text a spreadsheet would
// run as a formula prefixed with an apostrophe.
type csvFile struct {
	w    *csv.Writer
	rows int
}

// newCSVFile writes the header to w and returns the writer for the rows.
func newCSVFile(w io.Writer) (*csvFile, error) {
	f := &csvFile{w: csv.NewWriter(w)}
	f.w.UseCRLF = true
	return f, f.w.Write(CSVColumns)
}

func (f *csvFile) Write(r CSVRow) error {
	f.rows++
	return f.w.Write([]string{stamp(r.ObservedAt), stamp(r.ReceivedAt), csvText(r.ParameterID), csvText(r.Parameter), csvValue(r.Value),
		csvText(r.Unit), csvText(r.SourceName), r.SourceKind, csvText(r.SiteName), csvText(r.Classroom), csvNumber(r.Location.Lat),
		csvNumber(r.Location.Lng), r.LocationSource, participant(r.ContributorID), optional(r.ParticipantURL), csvText(r.Terms.Licence),
		optional(r.Terms.LicenceURL), csvText(r.Terms.Attribution), csvText(strings.Join(r.Terms.Restrictions, " ")),
		csvText(strings.Join(r.Licences, "; "))})
}

func (f *csvFile) Close() error {
	f.w.Flush()
	return f.w.Error()
}

var formula = regexp.MustCompile(`^[=+\-@\t\r]`)

// csvText is text, prefixed with an apostrophe when a spreadsheet would
// run it as a formula and it is not a number.
func csvText(s string) string {
	if formula.MatchString(s) {
		if _, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err != nil {
			return "'" + s
		}
	}
	return s
}

func csvNumber(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func csvValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case float64:
		return csvNumber(x)
	case bool:
		return strconv.FormatBool(x)
	case string:
		return csvText(x)
	}
	return csvText(fmt.Sprint(v))
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func participant(id *int64) string {
	if id == nil {
		return ""
	}
	return strconv.FormatInt(*id, 10)
}

func optional(s *string) string {
	if s == nil {
		return ""
	}
	return csvText(*s)
}

// termsOf is the policy p as the front end's terms, or fallback when SAM
// holds none for the row.
func termsOf(p *client.Policy, fallback settings.DataTerms) settings.DataTerms {
	if p == nil {
		return fallback
	}
	return settings.DataTerms{Licence: p.Licence, LicenceURL: p.LicenceURL, Attribution: p.Attribution, Restrictions: p.Restrictions,
		CoordinateDecimals: fallback.CoordinateDecimals}
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// CSVFilename is the front end's name for an export: the classroom, the
// source exported or all sources, when, and the export's ID, so a new
// export never takes an earlier one's name on the user's device.
func CSVFilename(classroom string, source *string, at time.Time, exportID string) string {
	src := "all-sources"
	if source != nil && slug(*source) != "" {
		src = slug(*source)
	}
	name := slug(classroom)
	if name == "" {
		name = "classroom"
	}
	return fmt.Sprintf("dagrouper-%s-%s-%s-%s.csv", name, src, at.UTC().Format("20060102-1504"), exportID)
}

// exportID is six random lowercase letters and digits.
func exportID() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		b[i] = alphabet[n.Int64()]
	}
	return string(b)
}

// policyByID indexes an export's policies by the space that sets them.
func policyByID(ps []client.Policy) map[uuid.UUID]*client.Policy {
	out := make(map[uuid.UUID]*client.Policy, len(ps))
	for i := range ps {
		out[ps[i].SpaceID] = &ps[i]
	}
	return out
}

// CSVExport is the front end's CsvExport: the whole file in the body, the
// simplest first. A streamed download can take its place through
// RowWriter.
type CSVExport struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
	Rows     int    `json:"rows"`
}

// exportBatch is how many rows are completed at a time with what this
// service kept of them.
const exportBatch = 500

// MaxCSVRows is the most values one export holds while the whole file
// goes in the body.
const MaxCSVRows = 100_000

var errTooManyRows = httpx.New(httpx.CodeInvalidInput, "This export has more than 100,000 values. Narrow the filters and export it in parts.")

func (m *Module) rawCSV(w http.ResponseWriter, r *http.Request) error {
	var in RawQuery
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	s, as, err := m.rawScope(r, in.ClassroomID)
	if err != nil {
		return err
	}
	var body strings.Builder
	file, err := newCSVFile(&body)
	if err != nil {
		return err
	}
	var source *string
	if s.classroom.SpaceID != nil {
		q, f := s.filters(in)
		if err := httpx.CheckFields(f); err != nil {
			return err
		}
		source = s.sourceName(in.SourceID)
		if err := m.exportRows(r.Context(), as, s, q, r.Header.Get("Origin"), file); err != nil {
			return err
		}
	}
	if err := file.Close(); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, CSVExport{Filename: CSVFilename(s.classroom.Name, source, m.now(), exportID()),
		Content: body.String(), Rows: file.rows})
	return nil
}

// sourceName is the name of the source a query picks, nil for all.
func (s rawScope) sourceName(id string) *string {
	if id == "" {
		return nil
	}
	name := "Field observation"
	if d, err := uuid.Parse(id); err == nil {
		name = s.devices[d].Name
	}
	return &name
}

// exportRows reads SAM's export of q as the member, under their
// export:create grant for this service, and writes each value to out as a
// row of the table, a batch at a time. Released readings of a removed
// device stay in it. site is the front end's address, for participant
// links.
func (m *Module) exportRows(ctx context.Context, as *client.Client, s rawScope, q client.ObservationQuery, site string, out RowWriter) error {
	v, err := m.reg.View(ctx)
	if err != nil {
		return err
	}
	var policies map[uuid.UUID]*client.Policy
	var batch []client.ExportedObservation
	n := 0
	p := &people{ctx: ctx, q: m.db, users: map[uuid.UUID]*accounts.User{}}
	write := func() error {
		ids := make([]uuid.UUID, len(batch))
		for i, rec := range batch {
			ids[i] = rec.ReadingID
		}
		kept, err := relayedRecords(ctx, m.db, ids)
		if err != nil {
			return err
		}
		for _, rec := range batch {
			row := CSVRow{RawRow: s.row(rec.ObservationRecord, kept, p), Parameter: rec.ParameterID, Unit: rec.Unit, Classroom: s.classroom.Name,
				Terms: s.classroom.Terms, Licences: rec.Licences}
			if par := v.Parameter(rec.ParameterID); par != nil {
				row.Parameter, row.Unit = par.Label, par.Unit
			}
			if rec.PolicySpaceID != nil {
				row.Terms = termsOf(policies[*rec.PolicySpaceID], s.classroom.Terms)
			}
			if id := row.ContributorID; id != nil && site != "" {
				link := strings.TrimRight(site, "/") + "/contributors/" + strconv.FormatInt(*id, 10)
				row.ParticipantURL = &link
			}
			if err := out.Write(row); err != nil {
				return err
			}
		}
		batch = batch[:0]
		return nil
	}
	err = as.ExportRecords(ctx, client.ExportQuery{ObservationQuery: q, Destination: roles.ExportDestination},
		func(ps []client.Policy) error {
			policies = policyByID(ps)
			return nil
		},
		func(rec client.ExportedObservation) error {
			if n++; n > MaxCSVRows {
				return errTooManyRows
			}
			if batch = append(batch, rec); len(batch) == exportBatch {
				return write()
			}
			return nil
		})
	switch {
	case errors.Is(err, errTooManyRows):
		return err
	case err != nil:
		return samaccess.Translate(ctx, err)
	}
	return write()
}
