package coreapp

import (
	"strings"

	"github.com/TIANLI0/THRM/internal/types"
)

const learningCurveScopeSeparator = "::curve::"

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

func cloneLearnedOffsetsMap(input map[string][]int) map[string][]int {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string][]int, len(input))
	for key, offsets := range input {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			out[trimmed] = cloneIntSlice(offsets)
		}
	}
	return out
}

func mergeLearnedOffsetsMaps(base, overlay map[string][]int) map[string][]int {
	out := cloneLearnedOffsetsMap(base)
	if len(overlay) == 0 {
		return out
	}
	if out == nil {
		out = map[string][]int{}
	}
	for key, offsets := range overlay {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			out[trimmed] = cloneIntSlice(offsets)
		}
	}
	return out
}

func activeLearningScopeKey(cfg *types.AppConfig) string {
	if cfg == nil || cfg.ActiveFanCurveProfileID == "" {
		return ""
	}
	deviceKey := deviceCurveScopeKey(*cfg)
	if deviceKey == "" {
		return cfg.ActiveFanCurveProfileID
	}
	return deviceKey + learningCurveScopeSeparator + cfg.ActiveFanCurveProfileID
}

func hasScopedLearningOffsets(offsets map[string][]int) bool {
	for key := range offsets {
		if strings.Contains(key, learningCurveScopeSeparator) {
			return true
		}
	}
	return false
}

func migrateLegacyLearningOffsetsToActiveDevice(cfg *types.AppConfig) bool {
	if cfg == nil || len(cfg.SmartControl.LearnedOffsetsByProfile) == 0 || hasScopedLearningOffsets(cfg.SmartControl.LearnedOffsetsByProfile) {
		return false
	}
	deviceKey := deviceCurveScopeKey(*cfg)
	if deviceKey == "" {
		return false
	}
	changed := false
	for profileID, offsets := range cfg.SmartControl.LearnedOffsetsByProfile {
		profileID = strings.TrimSpace(profileID)
		if profileID == "" || strings.Contains(profileID, learningCurveScopeSeparator) {
			continue
		}
		scopedKey := deviceKey + learningCurveScopeSeparator + profileID
		if _, exists := cfg.SmartControl.LearnedOffsetsByProfile[scopedKey]; exists {
			continue
		}
		cfg.SmartControl.LearnedOffsetsByProfile[scopedKey] = cloneIntSlice(offsets)
		changed = true
	}
	return changed
}

func prepareSmartControlOffsetsForUpdate(cfg *types.AppConfig, oldCfg types.AppConfig) bool {
	if cfg == nil {
		return false
	}
	changed := false
	merged := mergeLearnedOffsetsMaps(oldCfg.SmartControl.LearnedOffsetsByProfile, cfg.SmartControl.LearnedOffsetsByProfile)
	if merged == nil {
		merged = map[string][]int{}
	}
	cfg.SmartControl.LearnedOffsetsByProfile = merged
	oldKey := activeLearningScopeKey(&oldCfg)
	newKey := activeLearningScopeKey(cfg)
	if oldKey != "" && oldKey != newKey {
		cfg.SmartControl.LearnedOffsetsByProfile[oldKey] = cloneIntSlice(oldCfg.SmartControl.LearnedOffsets)
		changed = true
	}
	return changed
}

func storeSmartControlOffsetsForActiveProfile(cfg *types.AppConfig) bool {
	key := activeLearningScopeKey(cfg)
	if cfg == nil || key == "" {
		return false
	}
	ensureLearnedOffsetsByProfile(cfg)
	cfg.SmartControl.LearnedOffsetsByProfile[key] = cloneIntSlice(cfg.SmartControl.LearnedOffsets)
	return true
}

func syncSmartControlOffsetsForActiveProfile(cfg *types.AppConfig) bool {
	key := activeLearningScopeKey(cfg)
	if cfg == nil || key == "" {
		return false
	}
	ensureLearnedOffsetsByProfile(cfg)
	changed := migrateLegacyLearningOffsetsToActiveDevice(cfg)
	expectedLen := len(cfg.FanCurve)
	loaded, ok := cfg.SmartControl.LearnedOffsetsByProfile[key]
	if !ok {
		if len(cfg.SmartControl.LearnedOffsetsByProfile) == 0 && cfg.SmartControl.LearnedOffsets != nil {
			loaded = cloneIntSlice(cfg.SmartControl.LearnedOffsets)
		} else {
			loaded = make([]int, expectedLen)
		}
		loaded = resizeIntSlice(loaded, expectedLen)
		cfg.SmartControl.LearnedOffsetsByProfile[key] = loaded
		cfg.SmartControl.LearnedOffsets = cloneIntSlice(loaded)
		return true
	}
	if loaded == nil {
		loaded = make([]int, expectedLen)
		cfg.SmartControl.LearnedOffsetsByProfile[key] = loaded
		changed = true
	} else if len(loaded) != expectedLen {
		loaded = resizeIntSlice(loaded, expectedLen)
		cfg.SmartControl.LearnedOffsetsByProfile[key] = loaded
		changed = true
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
	return changed
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
	key := activeLearningScopeKey(cfg)
	if cfg == nil || key == "" {
		return
	}
	ensureLearnedOffsetsByProfile(cfg)
	cfg.SmartControl.LearnedOffsetsByProfile[key] = cloneIntSlice(cfg.SmartControl.LearnedOffsets)
}

func deleteSmartControlOffsetsForProfile(cfg *types.AppConfig, profileID string) {
	if cfg == nil || cfg.SmartControl.LearnedOffsetsByProfile == nil || profileID == "" {
		return
	}
	delete(cfg.SmartControl.LearnedOffsetsByProfile, profileID)
	suffix := learningCurveScopeSeparator + profileID
	for key := range cfg.SmartControl.LearnedOffsetsByProfile {
		if strings.HasSuffix(key, suffix) {
			delete(cfg.SmartControl.LearnedOffsetsByProfile, key)
		}
	}
}
