package views

import (
	"testing"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// The front end's core readings tests
// (frontend/src/lib/features/devices/core-readings.spec.ts), on what a
// visitor reads: released data only.

var parameters = settings.CoreParameters{
	RainDaily:      []string{"precip_accum_local_day", "rain_gauge_daily_accumulation"},
	AirTemperature: []string{"air_temperature"},
	RainTolerance:  2,
	RainReference:  "rain_gauge_daily_accumulation",
}

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func latest(param string, v float64, observed string) Latest {
	return Latest{ParameterID: param, Value: v, ObservedAt: at(observed), ReceivedAt: at(observed).Add(30 * time.Second)}
}

func TestCoreCountsOnlyReleasedData(t *testing.T) {
	t.Parallel()
	// Released through 06:00 on the 20th: the 07:00 gauge and 08:00 station
	// readings are not among what a visitor reads.
	core := Core([]Latest{latest("air_temperature", 25, "2026-10-20T06:00:00Z"), latest("precip_accum_local_day", 3, "2026-10-20T06:00:00Z")},
		[]RainDay{
			{Day: "2026-10-20", Source: "ws", ParameterID: "precip_accum_local_day", Max: 3, LastAt: at("2026-10-20T06:00:00Z")},
			{Day: "2026-10-19", Source: "ws", ParameterID: "precip_accum_local_day", Max: 2, LastAt: at("2026-10-19T08:00:00Z")},
		}, parameters)
	if *core.Day != "2026-10-20" || *core.RainToday != 3 || *core.AirTemperature != 25 || !core.LastSync.Equal(at("2026-10-20T06:00:30Z")) ||
		core.Review != Released || *core.RainLast7Days != 5 || core.RainDays != 2 {
		t.Errorf("%+v", core)
	}
}

func TestCoreTakesTheGaugeAndSumsSevenDays(t *testing.T) {
	t.Parallel()
	// Station 4 mm, gauge 6 mm: the gauge is the reference, 6 mm today,
	// plus the station's 2 mm on the 19th; the 12th is outside the week.
	core := Core([]Latest{latest("air_temperature", 27, "2026-10-20T08:00:00Z"), latest("precip_accum_local_day", 4, "2026-10-20T08:00:00Z"),
		latest("rain_gauge_daily_accumulation", 6, "2026-10-20T07:00:00Z")},
		[]RainDay{
			{Day: "2026-10-20", Source: "ws", ParameterID: "precip_accum_local_day", Max: 4, LastAt: at("2026-10-20T08:00:00Z")},
			{Day: "2026-10-20", Source: "gauge", ParameterID: "rain_gauge_daily_accumulation", Max: 6, LastAt: at("2026-10-20T07:00:00Z")},
			{Day: "2026-10-19", Source: "ws", ParameterID: "precip_accum_local_day", Max: 2, LastAt: at("2026-10-19T08:00:00Z")},
			{Day: "2026-10-12", Source: "ws", ParameterID: "precip_accum_local_day", Max: 9, LastAt: at("2026-10-12T08:00:00Z")},
		}, parameters)
	if *core.RainToday != 6 || *core.RainLast7Days != 8 || core.RainDays != 2 || *core.AirTemperature != 27 || core.Review != Released {
		t.Errorf("%+v", core)
	}
}

func TestCoreLeavesGapsNull(t *testing.T) {
	t.Parallel()
	core := Core([]Latest{latest("air_temperature", 27, "2026-10-20T08:00:00Z")}, nil, parameters)
	if core.RainToday != nil || core.RainLast7Days != nil || core.RainDays != 0 || *core.AirTemperature != 27 {
		t.Errorf("no rain: %+v", core)
	}
	core = Core([]Latest{latest("precip_accum_local_day", 1, "2026-10-20T08:00:00Z")}, nil, parameters)
	if core.AirTemperature != nil {
		t.Errorf("no temperature: %+v", core)
	}
	if none := Core(nil, nil, parameters); none.Day != nil || none.LastSync != nil || none.Review != NotReleased {
		t.Errorf("nothing released: %+v", none)
	}
}

func TestRainWithoutAGaugeTakesTheLatestSource(t *testing.T) {
	t.Parallel()
	rain := []RainDay{
		{Day: "2026-10-20", Source: "a", ParameterID: "precip_accum_local_day", Max: 1.04, LastAt: at("2026-10-20T06:00:00Z")},
		{Day: "2026-10-20", Source: "b", ParameterID: "precip_accum_local_day", Max: 3.06, LastAt: at("2026-10-20T09:00:00Z")},
	}
	if v, ok := rainOn(rain, "2026-10-20", parameters); !ok || v != 3.1 {
		t.Errorf("the latest source, rounded: %v %v", v, ok)
	}
}
