package smartcontrol

import "github.com/TIANLI0/THRM/internal/types"

// NormalizeConfig 归一化智能控温配置。
func NormalizeConfig(cfg types.SmartControlConfig, curve []types.FanCurvePoint, debug bool) (types.SmartControlConfig, bool) {
	return NormalizeConfigForUnit(cfg, curve, debug, types.FanSpeedUnitPercent)
}

func NormalizeConfigForUnit(cfg types.SmartControlConfig, curve []types.FanCurvePoint, _ bool, unit string) (types.SmartControlConfig, bool) {
	unit = types.NormalizeFanSpeedUnit(unit)
	controlCurve := CurveForUnit(curve, unit)
	changed := false
	if types.IsPercentSpeedUnit(unit) {
		var scaled bool
		cfg, scaled = normalizeLegacyPercentTickScale(cfg)
		changed = changed || scaled
	}
	defaults := types.GetDefaultSmartControlConfigForUnit(curve, unit)

	if cfg.TargetTemp < 45 || cfg.TargetTemp > 90 {
		cfg.TargetTemp = defaults.TargetTemp
		changed = true
	}
	if cfg.Aggressiveness < 1 || cfg.Aggressiveness > 10 {
		cfg.Aggressiveness = defaults.Aggressiveness
		changed = true
	}
	if cfg.Hysteresis < 0 || cfg.Hysteresis > 8 {
		cfg.Hysteresis = defaults.Hysteresis
		changed = true
	}
	minRPMChangeMin := 1
	minRPMChangeMax := 200
	rampLimitMin := 1
	rampLimitMax := 400
	maxLearnOffsetMin := 1
	maxLearnOffsetMax := 600
	rampDownSlack := 100
	if types.IsRPMSpeedUnit(unit) {
		minRPMChangeMin = 20
		minRPMChangeMax = 400
		rampLimitMin = 50
		rampLimitMax = 1200
		maxLearnOffsetMin = 100
		maxLearnOffsetMax = 2000
		rampDownSlack = 300
	}
	if cfg.MinRPMChange < minRPMChangeMin || cfg.MinRPMChange > minRPMChangeMax {
		cfg.MinRPMChange = defaults.MinRPMChange
		changed = true
	}
	if cfg.RampUpLimit < rampLimitMin || cfg.RampUpLimit > rampLimitMax {
		cfg.RampUpLimit = defaults.RampUpLimit
		changed = true
	}
	if cfg.RampDownLimit < rampLimitMin || cfg.RampDownLimit > rampLimitMax {
		cfg.RampDownLimit = defaults.RampDownLimit
		changed = true
	}
	if cfg.LearnRate < 1 || cfg.LearnRate > 10 {
		cfg.LearnRate = defaults.LearnRate
		changed = true
	}
	if normalizedBias := types.NormalizeLearningBias(cfg.LearningBias); normalizedBias != cfg.LearningBias {
		cfg.LearningBias = normalizedBias
		changed = true
	}
	if normalizedJointBias := types.NormalizeJointBias(cfg.JointBias); normalizedJointBias != cfg.JointBias {
		cfg.JointBias = normalizedJointBias
		changed = true
	}
	if cfg.LearnWindow < 3 || cfg.LearnWindow > 24 {
		cfg.LearnWindow = defaults.LearnWindow
		changed = true
	}
	if cfg.LearnDelay < 1 || cfg.LearnDelay > 8 {
		cfg.LearnDelay = defaults.LearnDelay
		changed = true
	}
	if cfg.OverheatWeight < 1 || cfg.OverheatWeight > 12 {
		cfg.OverheatWeight = defaults.OverheatWeight
		changed = true
	}
	if cfg.RPMDeltaWeight < 1 || cfg.RPMDeltaWeight > 12 {
		cfg.RPMDeltaWeight = defaults.RPMDeltaWeight
		changed = true
	}
	if cfg.NoiseWeight < 0 || cfg.NoiseWeight > 12 {
		cfg.NoiseWeight = defaults.NoiseWeight
		changed = true
	}
	if cfg.TrendGain < 1 || cfg.TrendGain > 12 {
		cfg.TrendGain = defaults.TrendGain
		changed = true
	}
	if cfg.MaxLearnOffset < maxLearnOffsetMin || cfg.MaxLearnOffset > maxLearnOffsetMax {
		cfg.MaxLearnOffset = defaults.MaxLearnOffset
		changed = true
	}
	predictionBoostMin := 10
	predictionBoostMax := 150
	if types.IsRPMSpeedUnit(unit) {
		predictionBoostMin = 50
		predictionBoostMax = 600
	}
	if cfg.TemperatureRisePredictionMaxBoost < predictionBoostMin || cfg.TemperatureRisePredictionMaxBoost > predictionBoostMax {
		cfg.TemperatureRisePredictionMaxBoost = defaults.TemperatureRisePredictionMaxBoost
		changed = true
	}

	if len(cfg.LearnedOffsets) != len(controlCurve) {
		next := make([]int, len(controlCurve))
		copy(next, cfg.LearnedOffsets)
		cfg.LearnedOffsets = next
		changed = true
	}
	if sanitized, updated := constrainOffsetsToCurveBounds(cfg.LearnedOffsets, controlCurve, cfg.MaxLearnOffset); updated {
		cfg.LearnedOffsets = sanitized
		changed = true
	}
	if sanitized, updated := constrainOffsetsToLearningBias(cfg.LearnedOffsets, cfg.LearningBias); updated {
		cfg.LearnedOffsets = sanitized
		changed = true
	}

	if len(cfg.LearnedOffsetsHeat) != len(controlCurve) {
		next := make([]int, len(controlCurve))
		copy(next, cfg.LearnedOffsetsHeat)
		cfg.LearnedOffsetsHeat = next
		changed = true
	}
	if len(cfg.LearnedOffsetsCool) != len(controlCurve) {
		next := make([]int, len(controlCurve))
		copy(next, cfg.LearnedOffsetsCool)
		cfg.LearnedOffsetsCool = next
		changed = true
	}

	rateLen := rateBucketMax - rateBucketMin + 1
	if len(cfg.LearnedRateHeat) != rateLen {
		next := make([]int, rateLen)
		copy(next, cfg.LearnedRateHeat)
		cfg.LearnedRateHeat = next
		changed = true
	}
	if len(cfg.LearnedRateCool) != rateLen {
		next := make([]int, rateLen)
		copy(next, cfg.LearnedRateCool)
		cfg.LearnedRateCool = next
		changed = true
	}

	if cfg.RampDownLimit > cfg.RampUpLimit+rampDownSlack {
		cfg.RampDownLimit = cfg.RampUpLimit + rampDownSlack
		changed = true
	}

	return cfg, changed
}

func normalizeLegacyPercentTickScale(cfg types.SmartControlConfig) (types.SmartControlConfig, bool) {
	if cfg.MinRPMChange <= 0 || cfg.RampUpLimit <= 0 || cfg.RampDownLimit <= 0 || cfg.MaxLearnOffset <= 0 {
		return cfg, false
	}
	if cfg.RampUpLimit > 40 || cfg.RampDownLimit > 40 || cfg.MaxLearnOffset > 60 {
		return cfg, false
	}
	cfg.MinRPMChange *= types.PercentSpeedTicksPerPercent
	cfg.RampUpLimit *= types.PercentSpeedTicksPerPercent
	cfg.RampDownLimit *= types.PercentSpeedTicksPerPercent
	cfg.MaxLearnOffset *= types.PercentSpeedTicksPerPercent
	cfg.TemperatureRisePredictionMaxBoost *= types.PercentSpeedTicksPerPercent
	scaleOffsets := func(values []int) {
		for i := range values {
			values[i] *= types.PercentSpeedTicksPerPercent
		}
	}
	scaleOffsets(cfg.LearnedOffsets)
	scaleOffsets(cfg.LearnedOffsetsHeat)
	scaleOffsets(cfg.LearnedOffsetsCool)
	scaleOffsets(cfg.LearnedRateHeat)
	scaleOffsets(cfg.LearnedRateCool)
	return cfg, true
}

// BlendOffsets 保留旧接口所需的 Heat/Cool 融合行为。
func BlendOffsets(heatOffsets, coolOffsets []int) []int {
	if len(heatOffsets) == 0 && len(coolOffsets) == 0 {
		return nil
	}
	size := max(len(coolOffsets), len(heatOffsets))
	out := make([]int, size)
	for i := range size {
		h, c := 0, 0
		if i < len(heatOffsets) {
			h = heatOffsets[i]
		}
		if i < len(coolOffsets) {
			c = coolOffsets[i]
		}
		out[i] = (h + c) / 2
	}
	return out
}
