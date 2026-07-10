package smartcontrol

import (
	"math"

	"github.com/TIANLI0/THRM/internal/temperature"
	"github.com/TIANLI0/THRM/internal/types"
)

// CalculateTargetRPM 以基础曲线加学习偏移计算目标转速。
func CalculateTargetRPM(currentTemp int, curve []types.FanCurvePoint, cfg types.SmartControlConfig) int {
	return calculateTargetSpeed(currentTemp, curve, cfg, rawPercentUnit)
}

func CalculateTargetRPMForCurve(curve []types.FanCurvePoint, temp int) int {
	if len(curve) == 0 {
		return 0
	}
	if temp <= curve[0].Temperature {
		return curve[0].RPM
	}
	last := curve[len(curve)-1]
	if temp >= last.Temperature {
		return last.RPM
	}

	for i := 1; i < len(curve); i++ {
		left := curve[i-1]
		right := curve[i]
		if temp > right.Temperature {
			continue
		}
		span := right.Temperature - left.Temperature
		if span <= 0 {
			return right.RPM
		}
		progress := float64(temp-left.Temperature) / float64(span)
		return int(math.Round(float64(left.RPM) + progress*float64(right.RPM-left.RPM)))
	}

	return last.RPM
}

func CurveForUnit(curve []types.FanCurvePoint, unit string) []types.FanCurvePoint {
	out := make([]types.FanCurvePoint, len(curve))
	copy(out, curve)
	if types.IsPercentSpeedUnit(unit) {
		for i := range out {
			out[i].RPM = types.PercentToTicks(out[i].RPM)
		}
	}
	return out
}

func CalculateTargetSpeedForUnit(currentTemp int, curve []types.FanCurvePoint, cfg types.SmartControlConfig, unit string) int {
	unit = types.NormalizeFanSpeedUnit(unit)
	curve = CurveForUnit(curve, unit)
	return calculateTargetSpeed(currentTemp, curve, cfg, unit)
}

func calculateTargetSpeed(currentTemp int, curve []types.FanCurvePoint, cfg types.SmartControlConfig, unit string) int {
	if len(curve) == 0 {
		return 0
	}

	offsets := cfg.LearnedOffsets
	if !cfg.Learning {
		offsets = nil
	} else if biased, updated := constrainOffsetsToLearningBias(offsets, cfg.LearningBias); updated {
		offsets = biased
	}
	effectiveCurve := buildEffectiveCurve(curve, offsets, effectiveOffsetCapForUnit(cfg, unit))
	rpm := temperature.CalculateTargetRPM(currentTemp, effectiveCurve)
	if rpm <= 0 {
		return 0
	}

	leftMin, rightMax := GetCurveRPMBounds(effectiveCurve)
	return clampInt(rpm, leftMin, rightMax)
}

// buildEffectiveCurve 把基础曲线与学习偏移合成有效曲线。
func buildEffectiveCurve(curve []types.FanCurvePoint, offsets []int, cap int) []types.FanCurvePoint {
	out := make([]types.FanCurvePoint, len(curve))
	leftMin, rightMax := GetCurveRPMBounds(curve)
	for i, p := range curve {
		off := 0
		if i < len(offsets) {
			off = offsets[i]
		}
		off = clampOffsetForPoint(off, p.RPM, leftMin, rightMax, cap)
		out[i] = types.FanCurvePoint{
			Temperature: p.Temperature,
			RPM:         clampInt(p.RPM+off, leftMin, rightMax),
		}
	}
	enforceNonDecreasingRPM(out)
	return out
}

// ApplyRampLimit 应用升降速限幅
func ApplyRampLimit(targetRPM, lastRPM, upLimit, downLimit int) int {
	if targetRPM > lastRPM {
		return min(lastRPM+upLimit, targetRPM)
	}
	if targetRPM < lastRPM {
		return max(lastRPM-downLimit, targetRPM)
	}
	return targetRPM
}
