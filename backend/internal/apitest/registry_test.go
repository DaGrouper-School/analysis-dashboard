package apitest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
)

// TestRegistryReads: signed-in people read SAM's core entries and this
// tenant's own, as the overlay presents them; another tenant's entries
// never show, and a value SAM derives explains its formula and inputs.
func TestRegistryReads(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	ctx := context.Background()
	for _, path := range []string{"parameters", "domains", "device-types", "observation-templates", "parameters/dew_point/calculation"} {
		if res, _ := e.Do(t, "GET", "/api/registry/"+path, ""); res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s without a session: %d", path, res.StatusCode)
		}
	}
	if _, err := registry.SeedOverlay(ctx, e.Pool, time.Now()); err != nil {
		t.Fatal(err)
	}
	other := e.SAM.AnotherTenant("other")
	for c, id := range map[*client.Client]string{e.SAM.Service(): "dgs:soil_ec", other: "other:soil_ec"} {
		if _, err := c.PutParameter(ctx, id, client.ParameterInput{Name: "Soil EC", Unit: "uS/cm", ValueType: "number", DomainID: "SOIL"}); err != nil {
			t.Fatalf("putting %s: %v", id, err)
		}
	}

	p := signedIn(t, e, amara)
	get := func(path string, into any) {
		t.Helper()
		res, raw := e.DoWith(t, p.client, "GET", "/api/registry/"+path, "")
		if err := json.Unmarshal([]byte(raw), into); err != nil || res.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d %s", path, res.StatusCode, raw)
		}
	}
	var params []registry.Parameter
	get("parameters", &params)
	find := func(id string) *registry.Parameter {
		for i := range params {
			if params[i].ID == id {
				return &params[i]
			}
		}
		return nil
	}
	if find("other:soil_ec") != nil {
		t.Error("another tenant's value shows")
	}
	if ec := find("dgs:soil_ec"); ec == nil || ec.Unit != "µS/cm" || *ec.Domain != "SOIL" || ec.Source != registry.SourceManual {
		t.Errorf("this tenant's own value: %+v", ec)
	}
	temp := find("air_temperature")
	if params[0].ID != "air_temperature" || temp.Unit != "°C" || temp.Chart != "line_band" || temp.Source != registry.SourceWeatherStation ||
		temp.Plain == "" || len(temp.AltUnits) != 2 {
		t.Errorf("air temperature: %+v", temp)
	}

	var domains []registry.Domain
	get("domains", &domains)
	if len(domains) != 10 || domains[0] != (registry.Domain{ID: "AIR", Label: "Air", Icon: "tree", Description: "Temperature, humidity and air quality."}) {
		t.Errorf("domains: %+v", domains)
	}
	var types []registry.DeviceType
	get("device-types", &types)
	if len(types) != 7 || types[1].ID != "manual_rain_gauge" || !types[1].Manual ||
		!slices.Equal(types[1].ParameterIDs, []string{"rain_gauge_daily_accumulation"}) {
		t.Errorf("device types: %+v", types)
	}
	var templates []registry.Template
	get("observation-templates", &templates)
	ids := make([]string, len(templates))
	for i, tpl := range templates {
		ids[i] = tpl.ID
	}
	if len(templates) != 9 || ids[0] != "rain_gauge_reading" || !slices.Contains(ids, "device:sen0658") || !slices.Contains(ids, "device:optical_rain_gauge") {
		t.Errorf("templates: %q", ids)
	}

	var calc registry.CalculationDetails
	get("parameters/dew_point/calculation", &calc)
	if calc.Method != "derived" || calc.Formula == "" || !slices.Equal(calc.Inputs, []string{"air_temperature", "relative_humidity"}) ||
		calc.InputDetails[0] != (registry.Input{ID: "air_temperature", Label: "Air temperature", Unit: "°C"}) || calc.Parameter.ID != "dew_point" {
		t.Errorf("dew point's calculation: %+v", calc)
	}
	if res, _ := e.DoWith(t, p.client, "GET", "/api/registry/parameters/other:soil_ec/calculation", ""); res.StatusCode != http.StatusNotFound {
		t.Errorf("another tenant's value's calculation: %d", res.StatusCode)
	}
}

// TestRegistryCache: SAM's registry is read once and served from the copy;
// a new entry shows once the copy is refreshed; while SAM is down the copy
// from before is served.
func TestRegistryCache(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	ctx := context.Background()
	p := signedIn(t, e, amara)
	count := func() int {
		t.Helper()
		var params []registry.Parameter
		res, raw := e.DoWith(t, p.client, "GET", "/api/registry/parameters", "")
		if err := json.Unmarshal([]byte(raw), &params); err != nil || res.StatusCode != http.StatusOK {
			t.Fatalf("parameters: %d %s", res.StatusCode, raw)
		}
		return len(params)
	}
	before := count()
	if _, err := e.SAM.Service().PutParameter(ctx, "dgs:leaf_wetness", client.ParameterInput{
		Name: "Leaf wetness", Unit: "%", ValueType: "number", DomainID: "AIR"}); err != nil {
		t.Fatal(err)
	}
	if n := count(); n != before {
		t.Fatalf("the copy was not served: %d, then %d", before, n)
	}
	e.App.Registry().Cache().Refresh()
	if n := count(); n != before+1 {
		t.Fatalf("after a refresh: %d, then %d", before, n)
	}
	e.SAM.Stop()
	e.App.Registry().Cache().Refresh()
	if n := count(); n != before+1 {
		t.Fatalf("while SAM is down: %d", n)
	}
	e.SAM.Start()
}
