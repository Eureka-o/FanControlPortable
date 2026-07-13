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

func TestAppendImportedProfilesPreservesExistingAndCreatesNewIDs(t *testing.T) {
	existing := []types.FanCurveProfile{{ID: "existing", Name: "Quiet", Curve: types.GetDefaultFanCurve()}}
	imported := []types.FanCurveProfile{
		{ID: "quiet", Name: "Quiet", Curve: types.GetDefaultFanCurve()},
		{ID: "gaming", Name: "Gaming", Curve: types.GetDefaultFanCurve()},
	}

	got, activeID := AppendImportedProfiles(existing, imported, "gaming")

	if len(got) != 3 {
		t.Fatalf("profile count = %d, want 3", len(got))
	}
	if got[0].ID != "existing" || got[0].Name != "Quiet" {
		t.Fatalf("existing profile changed: %+v", got[0])
	}
	if got[1].Name != "Quiet2" {
		t.Fatalf("duplicate imported name = %q, want Quiet2", got[1].Name)
	}
	if got[1].ID == "quiet" || got[2].ID == "gaming" || got[1].ID == got[2].ID {
		t.Fatalf("imported IDs were not regenerated: %q, %q", got[1].ID, got[2].ID)
	}
	if activeID != got[2].ID {
		t.Fatalf("activeID = %q, want %q", activeID, got[2].ID)
	}
}
