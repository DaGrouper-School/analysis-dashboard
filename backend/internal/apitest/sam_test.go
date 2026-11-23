package apitest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testenv"
)

// keys is a key file's keys: the first signs, and all are published.
func keys(t *testing.T, signing samaccess.KeyEntry, others ...samaccess.KeyEntry) *samaccess.Keys {
	t.Helper()
	signing.Signs = true
	all := append([]samaccess.KeyEntry{signing}, others...)
	for i := range others {
		all[i+1].Signs = false
	}
	raw, err := json.Marshal(map[string]any{"keys": all})
	if err != nil {
		t.Fatal(err)
	}
	k, err := samaccess.ParseKeys(raw)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func newEntry(t *testing.T) samaccess.KeyEntry {
	t.Helper()
	k, err := samaccess.NewKey(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// register makes participant id a SAM user of this service's issuer.
func register(t *testing.T, e *apitest.Env, id int64) samaccess.Person {
	t.Helper()
	tn := e.Access.Tenant()
	issuer := tn.IssuerID()
	if _, err := e.Access.Service().Register(context.Background(), client.PrincipalInput{Type: "user",
		HomeSpaceID: tn.RootSpaceID, IssuerID: &issuer, Subject: strconv.FormatInt(id, 10)}); err != nil {
		t.Fatal(err)
	}
	return samaccess.Person{ParticipantID: id, Registered: true}
}

// asks has SAM decide one action for p, which needs SAM to verify p's
// token; it answers whether the call was decided, not whether it was
// allowed.
func asks(t *testing.T, e *apitest.Env, p samaccess.Person) error {
	t.Helper()
	c, err := e.Access.User(p)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Check(context.Background(), []client.Check{{Action: "space:create", SpaceID: e.Access.Tenant().RootSpaceID}})
	return err
}

// TestTokensAcrossARotation: a key added to the key file is published
// before it signs, so when it takes over, SAM verifies its tokens from the
// set it already holds; a key that was never published is refused.
func TestTokensAcrossARotation(t *testing.T) {
	t.Parallel()
	old, next := newEntry(t), newEntry(t)
	e := apitest.NewWithKeys(t, keys(t, old, next))
	p := register(t, e, 1)
	if err := asks(t, e, p); err != nil {
		t.Fatalf("signed by the first key: %v", err)
	}

	e.Restart(keys(t, next, old))
	if err := asks(t, e, p); err != nil {
		t.Fatalf("signed by the next key, published before it signed: %v", err)
	}

	stranger := keys(t, newEntry(t))
	forged, err := samaccess.New(e.SAM.Config(optionsOf(e)), stranger, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	c, err := forged.User(p)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Check(context.Background(), []client.Check{{Action: "space:create", SpaceID: e.Access.Tenant().RootSpaceID}})
	if !client.IsCode(err, client.CodeUnauthenticated) {
		t.Fatalf("signed by a key never published: %v", err)
	}
	if got := samaccess.Translate(context.Background(), err); !httpx.IsCode(got, httpx.CodeForbidden) {
		t.Fatalf("a refused token reached the user as %v", got)
	}
}

func optionsOf(e *apitest.Env) testenv.Options {
	return testenv.Options{Issuer: e.URL, KeySetURL: e.URL + "/api" + samaccess.KeySetPath, Audience: apitest.Audience}
}

// TestRefusalsReachUsersAsRefusals: what SAM refuses a person reaches them
// as forbidden or not_found, never internal.
func TestRefusalsReachUsersAsRefusals(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	ctx := context.Background()
	p := register(t, e, 1)
	c, err := e.Access.User(p)
	if err != nil {
		t.Fatal(err)
	}
	tn := e.Access.Tenant()
	issuer := tn.IssuerID()
	_, err = c.Register(ctx, client.PrincipalInput{Type: "user", HomeSpaceID: tn.RootSpaceID, IssuerID: &issuer, Subject: "2"})
	for name, tc := range map[string]struct {
		err  error
		want []string
	}{
		"registering someone without the right": {err, []string{httpx.CodeForbidden}},
		"reading a principal that does not exist": {func() error {
			_, err := c.Principal(ctx, uuid.New())
			return err
		}(), []string{httpx.CodeForbidden, httpx.CodeNotFound}},
		"an account with no principal yet": {func() error {
			_, err := e.Access.User(samaccess.Person{ParticipantID: 3})
			return err
		}(), []string{httpx.CodeForbidden}},
	} {
		if tc.err == nil {
			t.Errorf("%s: SAM allowed it", name)
			continue
		}
		got := samaccess.Translate(ctx, tc.err)
		var he *httpx.Error
		if !errors.As(got, &he) || !slices.Contains(tc.want, he.Code) {
			t.Errorf("%s: %v reached the user as %v", name, tc.err, got)
			continue
		}
		w := httptest.NewRecorder()
		httpx.WriteError(w, httptest.NewRequest("GET", "/api/x", nil), got)
		if w.Code >= 500 {
			t.Errorf("%s: answered %d", name, w.Code)
		}
	}
}

// TestTheKeySet: the key set SAM fetches lists every published key.
func TestTheKeySet(t *testing.T) {
	t.Parallel()
	old, next := newEntry(t), newEntry(t)
	e := apitest.NewWithKeys(t, keys(t, old, next))
	res, body := e.Do(t, "GET", "/api"+samaccess.KeySetPath, "")
	var set samaccess.KeySet
	if err := json.Unmarshal([]byte(body), &set); err != nil || res.StatusCode != http.StatusOK || len(set.Keys) != 2 {
		t.Fatalf("%d %s", res.StatusCode, body)
	}
}
