package labs

import (
	"strings"
	"testing"
	"time"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

func TestCSVText(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"=SUM(A1)": "'=SUM(A1)", "+1 call": "'+1 call", "@me": "'@me", "\tx": "'\tx",
		"-12.5": "-12.5", "+3": "+3", "Ibis": "Ibis", "": ""} {
		if got := csvText(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestCSVFilename(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 22, 12, 30, 0, 0, time.FixedZone("EAT", 3*3600))
	station := "Shashamane Weather Station"
	for _, c := range []struct {
		classroom string
		source    *string
		want      string
	}{
		{"Trinity Lab Classroom", &station, "dagrouper-trinity-lab-classroom-shashamane-weather-station-20261022-0930-k3f9q2.csv"},
		{"Bay Street Classroom", nil, "dagrouper-bay-street-classroom-all-sources-20261022-0930-k3f9q2.csv"},
		{"!!!", nil, "dagrouper-classroom-all-sources-20261022-0930-k3f9q2.csv"},
	} {
		if got := CSVFilename(c.classroom, c.source, at, "k3f9q2"); got != c.want {
			t.Errorf("%s: %s", c.classroom, got)
		}
	}
	if id := exportID(); len(id) != 6 || strings.Trim(id, "abcdefghijklmnopqrstuvwxyz0123456789") != "" {
		t.Errorf("an export ID: %q", id)
	}
}

func TestCSVFile(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	f, err := newCSVFile(&b)
	if err != nil {
		t.Fatal(err)
	}
	pid, link := int64(1042), "https://app.test/contributors/1042"
	row := CSVRow{RawRow: RawRow{ObservedAt: time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC), ReceivedAt: time.Date(2026, 10, 22, 9, 1, 0, 0, time.UTC),
		ParameterID: "air_temperature", Value: 21.5, SourceName: "Pilot station, roof", SourceKind: "device", SiteName: "Garden",
		Location: Point{Lat: 9.03, Lng: 38.74}, LocationSource: FromLab, ContributorID: &pid},
		Parameter: "Air temperature", Unit: "°C", Classroom: "Trinity", ParticipantURL: &link,
		Terms: settings.DataTerms{Licence: "CC BY 4.0", Attribution: "Trinity", Restrictions: []string{"No resale.", "Cite us."}}, Licences: []string{"research"}}
	if err := f.Write(row); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	want := strings.Join(CSVColumns, ",") + "\r\n" + `2026-10-22T09:00:00Z,2026-10-22T09:01:00Z,air_temperature,Air temperature,21.5,°C,"Pilot station, roof",device,Garden,Trinity,9.03,38.74,lab,1042,https://app.test/contributors/1042,CC BY 4.0,,Trinity,No resale. Cite us.,research` + "\r\n"
	if b.String() != want || f.rows != 1 {
		t.Errorf("got\n%q\nwant\n%q", b.String(), want)
	}
}

func TestTermsOf(t *testing.T) {
	t.Parallel()
	d := 2
	own := settings.DataTerms{Licence: "CC0", Attribution: "x", Restrictions: []string{}, CoordinateDecimals: &d}
	if got := termsOf(nil, own); got.Licence != "CC0" {
		t.Errorf("without SAM's: %+v", got)
	}
	got := termsOf(&client.Policy{Licence: "CC BY 4.0", Attribution: "y", Restrictions: []string{"r"}}, own)
	if got.Licence != "CC BY 4.0" || got.Attribution != "y" || got.Restrictions[0] != "r" || got.CoordinateDecimals != &d {
		t.Errorf("SAM's: %+v", got)
	}
}
