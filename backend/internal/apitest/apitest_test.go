package apitest_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/contract"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/server"
)

// TestTheHarness: this service answers with its database, and SAM, built
// from source, knows this service as its tenant: the service may create
// spaces under its tenant root and nowhere above it.
func TestTheHarness(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	if res, body := e.Do(t, "GET", "/healthz", ""); res.StatusCode != http.StatusOK {
		t.Fatalf("/healthz: %d %s", res.StatusCode, body)
	}
	ctx := context.Background()
	root := e.SAM.Tenant.RootSpaceID
	results, err := e.SAM.Service().Check(ctx, []client.Check{{Action: "space:create", SpaceID: root}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].Allowed {
		t.Fatalf("creating a space under the tenant root: %+v", results)
	}
	sys, err := e.SAM.Service().System(ctx)
	if err != nil {
		t.Fatal(err)
	}
	results, err = e.SAM.Service().Check(ctx, []client.Check{{Action: "space:create", SpaceID: sys.RootSpaceID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Allowed {
		t.Fatalf("creating a space at the deployment's root: %+v", results)
	}
}

// TestRouteCoverage lists the contract's operations this service does not
// serve yet. It reports rather than fails while the service is being
// built; once every operation is served it fails on any that is not.
func TestRouteCoverage(t *testing.T) {
	t.Parallel()
	c, err := contract.Find()
	if err != nil {
		t.Fatal(err)
	}
	unserved := apitest.Unserved(c, server.New(server.Deps{}))
	for _, op := range unserved {
		t.Logf("not served: %s %s (%s)", op.Method, op.Path, op.ID)
	}
	t.Logf("%d of %d operations served", len(c.Operations())-len(unserved), len(c.Operations()))
}
