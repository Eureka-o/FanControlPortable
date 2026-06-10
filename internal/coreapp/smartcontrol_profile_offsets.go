package coreapp

import "github.com/TIANLI0/THRM/internal/types"

func cloneIntSlice(input []int) []int {
	if len(input) == 0 {
		return nil
	}
	out := make([]int, len(input))
	copy(out, input)
	return out
}

func ensureLearnedOffsetsByProfile(cfg *types.AppConfig) {
	if cfg == nil || cfg.SmartControl.LearnedOffsetsByProfile != nil {
		return
	}
	cfg.SmartControl.LearnedOffsetsByProfile = map[string][]int{}
}

func storeSmartControlOffsetsForActiveProfile(cfg *types.AppConfig) bool {
	if cfg == nil || cfg.ActiveFanCurveProfileID == "" {
		return false
	}
	ensureLearnedOffsetsByProfile(cfg)
	cfg.SmartControl.LearnedOffsetsByProfile[cfg.ActiveFanCurveProfileID] = cloneIntSlice(cfg.SmartControl.LearnedOffsets)
	return true
}

func syncSmartControlOffsetsForActiveProfile(cfg *types.AppConfig) bool {
	if cfg == nil || cfg.ActiveFanCurveProfileID == "" {
		return false
	}
	ensureLearnedOffsetsByProfile(cfg)
	activeID := cfg.ActiveFanCurveProfileID
	expectedLen := len(cfg.FanCurve)
	loaded, ok := cfg.SmartControl.LearnedOffsetsByProfile[activeID]
	if !ok {
		if len(cfg.SmartControl.LearnedOffsetsByProfile) == 0 && cfg.SmartControl.LearnedOffsets != nil {
			loaded = cloneIntSlice(cfg.SmartControl.LearnedOffsets)
		} else {
			loaded = make([]int, expectedLen)
		}
		loaded = resizeIntSlice(loaded, expectedLen)
		cfg.SmartControl.LearnedOffsetsByProfile[activeID] = loaded
		cfg.SmartControl.LearnedOffsets = cloneIntSlice(loaded)
		return true
	}
	if loaded == nil {
		loaded = make([]int, expectedLen)
		cfg.SmartControl.LearnedOffsetsByProfile[activeID] = loaded
	} else if len(loaded) != expectedLen {
		loaded = resizeIntSlice(loaded, expectedLen)
		cfg.SmartControl.LearnedOffsetsByProfile[activeID] = loaded
	}
	if len(cfg.SmartControl.LearnedOffsets) != len(loaded) {
		cfg.SmartControl.LearnedOffsets = cloneIntSlice(loaded)
		return true
	}
	for i := range loaded {
		if cfg.SmartControl.LearnedOffsets[i] != loaded[i] {
			cfg.SmartControl.LearnedOffsets = cloneIntSlice(loaded)
			return true
		}
	}
	return false
}

func resizeIntSlice(input []int, size int) []int {
	if size <= 0 {
		return nil
	}
	out := make([]int, size)
	copy(out, input)
	return out
}

func resetSmartControlOffsetsForActiveProfile(cfg *types.AppConfig) {
	if cfg == nil || cfg.ActiveFanCurveProfileID == "" {
		return
	}
	ensureLearnedOffsetsByProfile(cfg)
	cfg.SmartControl.LearnedOffsetsByProfile[cfg.ActiveFanCurveProfileID] = cloneIntSlice(cfg.SmartControl.LearnedOffsets)
}

func deleteSmartControlOffsetsForProfile(cfg *types.AppConfig, profileID string) {
	if cfg == nil || cfg.SmartControl.LearnedOffsetsByProfile == nil || profileID == "" {
		return
	}
	delete(cfg.SmartControl.LearnedOffsetsByProfile, profileID)
}
