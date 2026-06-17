package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestSmartControlOffsetsFollowActiveCurveProfile(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.FanCurveProfiles = []types.FanCurveProfile{
		{ID: "quiet", Name: "Quiet", Curve: cfg.FanCurve},
		{ID: "game", Name: "Game", Curve: cfg.FanCurve},
	}
	cfg.ActiveFanCurveProfileID = "quiet"
	cfg.SmartControl.LearnedOffsets = []int{1, 2, 3}

	if !storeSmartControlOffsetsForActiveProfile(&cfg) {
		t.Fatal("storeSmartControlOffsetsForActiveProfile() changed = false")
	}

	cfg.ActiveFanCurveProfileID = "game"
	cfg.SmartControl.LearnedOffsets = []int{9, 8, 7}
	if !storeSmartControlOffsetsForActiveProfile(&cfg) {
		t.Fatal("storeSmartControlOffsetsForActiveProfile(game) changed = false")
	}

	cfg.ActiveFanCurveProfileID = "quiet"
	cfg.SmartControl.LearnedOffsets = []int{9, 8, 7}
	if !syncSmartControlOffsetsForActiveProfile(&cfg) {
		t.Fatal("syncSmartControlOffsetsForActiveProfile() changed = false")
	}
	if got := cfg.SmartControl.LearnedOffsets; len(got) != len(cfg.FanCurve) || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("quiet learned offsets = %#v, want migrated prefix [1 2 3]", got)
	}

	cfg.ActiveFanCurveProfileID = "game"
	if !syncSmartControlOffsetsForActiveProfile(&cfg) {
		t.Fatal("syncSmartControlOffsetsForActiveProfile(game) changed = false")
	}
	if got := cfg.SmartControl.LearnedOffsets; len(got) != len(cfg.FanCurve) || got[0] != 9 || got[1] != 8 || got[2] != 7 {
		t.Fatalf("game learned offsets = %#v, want migrated prefix [9 8 7]", got)
	}
}

func TestSmartControlOffsetsStartEmptyForNewProfileAfterMigration(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.ActiveFanCurveProfileID = "old"
	cfg.SmartControl.LearnedOffsets = []int{1, 2, 3}

	if !syncSmartControlOffsetsForActiveProfile(&cfg) {
		t.Fatal("initial migration changed = false")
	}

	cfg.ActiveFanCurveProfileID = "new"
	cfg.SmartControl.LearnedOffsets = []int{1, 2, 3}
	if !syncSmartControlOffsetsForActiveProfile(&cfg) {
		t.Fatal("new profile sync changed = false")
	}
	if got := cfg.SmartControl.LearnedOffsets; len(got) != len(cfg.FanCurve) {
		t.Fatalf("new profile offsets length = %d, want %d", len(got), len(cfg.FanCurve))
	} else {
		for _, value := range got {
			if value != 0 {
				t.Fatalf("new profile offsets = %#v, want all zero", got)
			}
		}
	}
}

func TestLegacyLearningOffsetsCanMigrateAfterScopedKeysExist(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	legacyOffsets := []int{11, 22, 33}
	cfg.SmartControl.LearnedOffsetsByProfile = map[string][]int{
		cfg.ActiveFanCurveProfileID: cloneIntSlice(legacyOffsets),
	}

	wifiKey := deviceCurveScopeKey(cfg)
	if !syncSmartControlOffsetsForDeviceKey(&cfg, wifiKey) {
		t.Fatal("expected first sync to create wifi scoped learning offsets")
	}
	nativeKey := deviceCurveScopeKeyForProfile(types.FlyDigiBS3PROProfile())
	if !syncSmartControlOffsetsForDeviceKey(&cfg, nativeKey) {
		t.Fatal("expected native sync to inherit legacy learning offsets")
	}

	scopedKey := activeLearningScopeKeyForDeviceKey(&cfg, nativeKey)
	got := cfg.SmartControl.LearnedOffsetsByProfile[scopedKey]
	if len(got) < len(legacyOffsets) {
		t.Fatalf("native scoped offsets len = %d, want at least %d", len(got), len(legacyOffsets))
	}
	for i, want := range legacyOffsets {
		if got[i] != want {
			t.Fatalf("native scoped offsets[%d] = %d, want %d", i, got[i], want)
		}
	}
}
