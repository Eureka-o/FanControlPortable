package curveprofiles

import (
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func TestNormalizeConfigForRPMReplacesPercentFallbackCurve(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.FanCurve = types.GetDefaultFanCurve()
	cfg.FanCurveProfiles = []types.FanCurveProfile{{
		ID:    "default",
		Name:  "default",
		Curve: types.GetDefaultFanCurve(),
	}}
	cfg.ActiveFanCurveProfileID = "default"

	changed := NormalizeConfigForUnit(&cfg, types.FanSpeedUnitRPM)
	if !changed {
		t.Fatal("NormalizeConfigForUnit() changed = false, want true")
	}

	want := types.GetDefaultRPMFanCurve()
	if len(cfg.FanCurve) != len(want) {
		t.Fatalf("fan curve length = %d, want %d", len(cfg.FanCurve), len(want))
	}
	for i := range want {
		if cfg.FanCurve[i] != want[i] {
			t.Fatalf("fan curve[%d] = %+v, want %+v", i, cfg.FanCurve[i], want[i])
		}
		if cfg.FanCurveProfiles[0].Curve[i] != want[i] {
			t.Fatalf("profile curve[%d] = %+v, want %+v", i, cfg.FanCurveProfiles[0].Curve[i], want[i])
		}
	}
}

func TestNormalizeConfigForRPMRestoresLowTemperatureRange(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.FanCurve = []types.FanCurvePoint{
		{Temperature: 30, RPM: 1000},
		{Temperature: 60, RPM: 2300},
		{Temperature: 110, RPM: 4000},
	}
	cfg.FanCurveProfiles = []types.FanCurveProfile{{ID: "default", Name: "default", Curve: cfg.FanCurve}}
	cfg.ActiveFanCurveProfileID = "default"

	NormalizeConfigForUnit(&cfg, types.FanSpeedUnitRPM)
	if len(cfg.FanCurve) < 2 || cfg.FanCurve[0].Temperature != 20 || cfg.FanCurve[1].Temperature != 25 {
		t.Fatalf("low-temperature curve points = %+v, want 20/25 degree points", cfg.FanCurve[:min(len(cfg.FanCurve), 2)])
	}
	if cfg.FanCurve[0].RPM != 1000 || cfg.FanCurve[1].RPM != 1000 {
		t.Fatalf("low-temperature RPM = %d/%d, want 1000/1000", cfg.FanCurve[0].RPM, cfg.FanCurve[1].RPM)
	}
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
