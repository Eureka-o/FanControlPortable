package curveprofiles

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
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
