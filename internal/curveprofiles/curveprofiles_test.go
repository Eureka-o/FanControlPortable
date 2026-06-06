package curveprofiles

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestNormalizeProfileNameReplacesCorruptedQuestionMarks(t *testing.T) {
	if got := NormalizeProfileName("????-?", "方案1"); got != "方案1" {
		t.Fatalf("NormalizeProfileName(corrupted) = %q, want 方案1", got)
	}
}

func TestNormalizeProfileNameKeepsValidChineseName(t *testing.T) {
	if got := NormalizeProfileName("升级测试", "方案1"); got != "升级测试" {
		t.Fatalf("NormalizeProfileName(valid Chinese) = %q, want 升级测试", got)
	}
}

func TestNormalizeConfigRepairsCorruptedProfileName(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.FanCurveProfiles = []types.FanCurveProfile{
		{ID: "bad", Name: "????-?", Curve: types.GetDefaultFanCurve()},
	}
	cfg.ActiveFanCurveProfileID = "bad"

	changed := NormalizeConfigForUnit(&cfg, types.FanSpeedUnitPercent)

	if !changed {
		t.Fatal("NormalizeConfigForUnit() changed = false, want true")
	}
	if cfg.FanCurveProfiles[0].Name != "方案1" {
		t.Fatalf("profile name = %q, want 方案1", cfg.FanCurveProfiles[0].Name)
	}
}
