package apitest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/hosting"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

func hostApplication(email string) map[string]any {
	return map[string]any{"institutionName": "Lalibela Secondary School", "contactName": "Meron Haile", "contactRole": "Science teacher",
		"contactEmail": email, "contactPhone": "+251 911 234 567",
		"answers": map[string]string{"storage": "yes", "lead": "yes", "connection": "no", "learners": "40", "unasked": "x"},
		"site": map[string]any{"placeName": "Lalibela", "country": "Ethiopia", "ecosystems": []string{"highland"}, "learners": "Grade 9",
			"studyGoals": "How much rain falls"}}
}

// TestHostApplications: anyone applies without signing in and gets their
// status link by email, never in the answer; platform admins list
// applications and decide each once; a classroom is created from an
// accepted application once, with its contact invited as head; the
// applicant sees the decision on their link; applying is limited.
func TestHostApplications(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	ctx := context.Background()
	now := time.Now()
	if _, err := settings.SeedSettings(ctx, e.Pool, now); err != nil {
		t.Fatal(err)
	}
	if _, err := classrooms.SeedLists(ctx, e.Pool, now); err != nil {
		t.Fatal(err)
	}
	if _, err := hosting.SeedQuestions(ctx, e.Pool, now); err != nil {
		t.Fatal(err)
	}
	call := func(p *profile, addr, method, path string, body, out any) (int, refusal) {
		t.Helper()
		raw := ""
		if body != nil {
			b, _ := json.Marshal(body)
			raw = string(b)
		}
		client := e.NewClient()
		if p != nil {
			client = p.client
		}
		res, got := e.DoFrom(t, client, addr, method, "/api"+path, raw)
		var no refusal
		if res.StatusCode >= 400 {
			_ = json.Unmarshal([]byte(got), &no)
		} else if out != nil {
			if err := json.Unmarshal([]byte(got), out); err != nil {
				t.Fatalf("%s %s: %s", method, path, got)
			}
		}
		return res.StatusCode, no
	}
	statusToken := func(address string) string {
		t.Helper()
		sent := e.Email.Sent()
		for i := len(sent) - 1; i >= 0; i-- {
			if _, after, ok := strings.Cut(sent[i].Text, apitest.FrontEnd+"/host/status/"); ok && sent[i].To == address {
				token, _, _ := strings.Cut(after, "\n")
				return token
			}
		}
		t.Fatalf("no status link emailed to %s", address)
		return ""
	}

	var questions []hosting.Question
	if status, _ := call(nil, "", "GET", "/host/questions", nil, &questions); status != 200 || len(questions) != 5 {
		t.Fatalf("the questions: %d %+v", status, questions)
	}
	var receipt hosting.Receipt
	if status, no := call(nil, "", "POST", "/host/applications", hostApplication(" Meron@Lalibela.example "), &receipt); status != 200 ||
		receipt.Token != "" || receipt.Application.Status != hosting.StatusSubmitted || receipt.Application.ContactEmail != "meron@lalibela.example" ||
		receipt.Application.ContactPhone != "+251911234567" || len(receipt.Application.Answers) != 4 {
		t.Fatalf("applied: %d %+v %+v", status, receipt, no)
	}
	token := statusToken("meron@lalibela.example")
	bad := hostApplication("not an email")
	bad["answers"] = map[string]string{"storage": "maybe"}
	if status, no := call(nil, "", "POST", "/host/applications", bad, nil); status != 400 || no.Fields["contactEmail"] == "" ||
		no.Fields["answers.storage"] != "Choose yes or no." || no.Fields["answers.lead"] != "Answer this question." {
		t.Errorf("a bad application: %d %+v", status, no)
	}
	var seen hosting.Application
	if status, _ := call(nil, "", "GET", "/host/status/"+token, nil, &seen); status != 200 || seen.ID != receipt.Application.ID {
		t.Fatalf("the status link: %d %+v", status, seen)
	}
	if status, no := call(nil, "", "GET", "/host/status/nope", nil, nil); status != 404 || no.Message != "This status link is not valid." {
		t.Errorf("a bad status link: %d %+v", status, no)
	}

	// Platform admins decide; no one else does.
	a := admin(t, e)
	someone := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	id := receipt.Application.ID.String()
	if status, _ := call(someone, "", "GET", "/admin/host-applications", nil, nil); status != 403 {
		t.Errorf("listed by someone else: %d", status)
	}
	if status, _ := call(someone, "", "POST", "/admin/host-applications/"+id+"/decision", map[string]any{"accept": true}, nil); status != 403 {
		t.Errorf("decided by someone else: %d", status)
	}
	if status, _ := call(nil, "", "GET", "/admin/host-applications", nil, nil); status != 401 {
		t.Errorf("listed signed out: %d", status)
	}
	other := hostApplication("abebe@gondar.example")
	other["institutionName"] = "Gondar School"
	var second hosting.Receipt
	e.Clock.Add(time.Minute)
	call(nil, "", "POST", "/host/applications", other, &second)
	var list []hosting.Application
	if status, _ := call(a, "", "GET", "/admin/host-applications", nil, &list); status != 200 || len(list) != 2 || list[0].ID != second.Application.ID {
		t.Errorf("newest first: %d %+v", status, list)
	}
	create := map[string]any{"name": "Lalibela Weather", "institutionName": "Lalibela Secondary School", "headEmail": "meron@lalibela.example", "applicationId": id}
	if status, no := call(a, "", "POST", "/classrooms", create, nil); status != 409 || no.Message != "Accept the application before creating its classroom." {
		t.Errorf("a classroom before acceptance: %d %+v", status, no)
	}
	var decided hosting.Application
	if status, _ := call(a, "", "POST", "/admin/host-applications/"+id+"/decision", map[string]any{"accept": true}, &decided); status != 200 ||
		decided.Status != hosting.StatusAccepted || decided.DecidedAt == nil {
		t.Fatalf("accepted: %d %+v", status, decided)
	}
	if status, _ := call(a, "", "POST", "/admin/host-applications/"+id+"/decision", map[string]any{"accept": false}, nil); status != 409 {
		t.Errorf("decided twice: %d", status)
	}
	if status, _ := call(a, "", "POST", "/admin/host-applications/"+second.Application.ID.String()+"/decision", map[string]any{"accept": false}, nil); status != 200 {
		t.Errorf("declined: %d", status)
	}
	if status, _ := call(a, "", "POST", "/admin/host-applications/00000000-0000-0000-0000-000000000000/decision", map[string]any{"accept": true}, nil); status != 404 {
		t.Errorf("an unknown application: %d", status)
	}

	// The classroom, once; its contact is invited as head.
	var c classrooms.Summary
	if status, no := call(a, "", "POST", "/classrooms", create, &c); status != 200 {
		t.Fatalf("the classroom: %d %+v", status, no)
	}
	if status, no := call(a, "", "POST", "/classrooms", create, nil); status != 409 || no.Message != "A classroom was already created from this application." {
		t.Errorf("a second classroom: %d %+v", status, no)
	}
	create["applicationId"] = second.Application.ID.String()
	if status, _ := call(a, "", "POST", "/classrooms", create, nil); status != 409 {
		t.Errorf("a classroom from a declined application: %d", status)
	}
	create["applicationId"] = "nope"
	if status, _ := call(a, "", "POST", "/classrooms", create, nil); status != 404 {
		t.Errorf("a classroom from no application: %d", status)
	}
	if call(nil, "", "GET", "/host/status/"+token, nil, &seen); seen.ClassroomID == nil || seen.ClassroomID.String() != c.ID || seen.Status != hosting.StatusAccepted {
		t.Errorf("the applicant's view: %+v", seen)
	}
	tokenFor(t, e, "meron@lalibela.example")

	// Applying is limited by address and by email.
	for i := range hosting.AppliesPerAddress {
		if status, no := call(nil, "203.0.113.9", "POST", "/host/applications", hostApplication(fmt.Sprintf("a%d@x.example", i)), nil); status != 200 {
			t.Fatalf("application %d: %d %+v", i, status, no)
		}
	}
	if status, _ := call(nil, "203.0.113.9", "POST", "/host/applications", hostApplication("late@x.example"), nil); status != 429 {
		t.Errorf("past the address's limit: %d", status)
	}
	for i := range hosting.AppliesPerEmail {
		if status, _ := call(nil, fmt.Sprintf("198.51.100.%d", i), "POST", "/host/applications", hostApplication("same@x.example"), nil); status != 200 {
			t.Fatalf("application %d by one email: %d", i, status)
		}
	}
	if status, _ := call(nil, "198.51.100.99", "POST", "/host/applications", hostApplication("same@x.example"), nil); status != 429 {
		t.Errorf("past the email's limit: %d", status)
	}
}

// TestHostTokensInDemo: in demo mode the receipt carries the status link's
// token.
func TestHostTokensInDemo(t *testing.T) {
	t.Parallel()
	e := apitest.NewWith(t, apitest.Options{Demo: true})
	if _, err := hosting.SeedQuestions(context.Background(), e.Pool, time.Now()); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(hostApplication("meron@lalibela.example"))
	res, got := e.Do(t, "POST", "/api/host/applications", string(b))
	var receipt hosting.Receipt
	if err := json.Unmarshal([]byte(got), &receipt); err != nil || res.StatusCode != 200 || receipt.Token == "" {
		t.Fatalf("%d %s", res.StatusCode, got)
	}
	if res, _ := e.Do(t, "GET", "/api/host/status/"+receipt.Token, ""); res.StatusCode != 200 {
		t.Errorf("the demo token: %d", res.StatusCode)
	}
}
