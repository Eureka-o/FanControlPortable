package curveprofiles

import (
	"strings"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestGenerateIDIncludesCollisionCounter(t *testing.T) {
	first := GenerateID()
	second := GenerateID()
	if !strings.Contains(first, "-") || !strings.Contains(second, "-") {
		t.Fatalf("generated IDs must include a collision counter: %q, %q", first, second)
	}
	if first == second {
		t.Fatalf("GenerateID() returned duplicate IDs: %q", first)
	}
}

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

func TestExportSelectedFiltersProfilesAndChoosesIncludedActiveProfile(t *testing.T) {
	profiles := []types.FanCurveProfile{
		{ID: "quiet", Name: "Quiet", Curve: types.GetDefaultFanCurve()},
		{ID: "balanced", Name: "Balanced", Curve: types.GetDefaultFanCurve()},
		{ID: "gaming", Name: "Gaming", Curve: types.GetDefaultFanCurve()},
	}

	code, err := ExportSelected("balanced", profiles, []string{"gaming", "quiet", "gaming"})
	if err != nil {
		t.Fatalf("ExportSelected() error = %v", err)
	}
	got, activeID, err := Import(code)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(got) != 2 || got[0].ID != "quiet" || got[1].ID != "gaming" {
		t.Fatalf("exported profiles = %+v, want quiet then gaming", got)
	}
	if activeID != "quiet" {
		t.Fatalf("activeID = %q, want first selected profile quiet", activeID)
	}

	code, err = ExportSelected("gaming", profiles, []string{"gaming", "quiet"})
	if err != nil {
		t.Fatalf("ExportSelected(active included) error = %v", err)
	}
	_, activeID, err = Import(code)
	if err != nil {
		t.Fatalf("Import(active included) error = %v", err)
	}
	if activeID != "gaming" {
		t.Fatalf("activeID = %q, want gaming", activeID)
	}
}

func TestExportSelectedRejectsExplicitEmptySelection(t *testing.T) {
	profiles := []types.FanCurveProfile{{ID: "quiet", Name: "Quiet", Curve: types.GetDefaultFanCurve()}}
	if _, err := ExportSelected("quiet", profiles, []string{}); err == nil {
		t.Fatal("ExportSelected() error = nil, want empty-selection error")
	}
}
