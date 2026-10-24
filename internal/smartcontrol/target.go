package smartcontrol

import (
	"github.com/Eureka-o/FanControlPortable/internal/temperature"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

// CalculateTargetRPM 以基础曲线加学习偏移计算目标转速。
func CalculateTargetRPM(currentTemp int, curve []types.FanCurvePoint, cfg types.SmartControlConfig) int {
	return calculateTargetSpeed(currentTemp, curve, cfg, rawPercentUnit)
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
	effectiveCurve := EffectiveCurveForUnit(curve, cfg, unit)
	rpm := temperature.CalculateTargetRPM(currentTemp, effectiveCurve)
	if rpm <= 0 {
		return 0
	}

	leftMin, rightMax := GetCurveRPMBounds(effectiveCurve)
	return clampInt(rpm, leftMin, rightMax)
}

// EffectiveCurveForUnit 由基础曲线与已学偏移合成有效曲线，也是本仓库这套口径的唯一出口
// （黑鲨把曲线本身写进设备时也复用它，避免在 coreapp 重写一份而漂移）。
//
// 调用方必须传入已经换算成 `unit` 形态的曲线（`CalculateTargetSpeedForUnit` 经 `CurveForUnit`
// 完成换算），这里不再换算，否则 percent 路径会被放大两级。
// `unit` 只用于选偏移上限（`effectiveOffsetCapForUnit`），其中 `rawPercentUnit` 有独立档位，
// 因此调用方传入时必须原样保留，不要先经 `NormalizeFanSpeedUnit` 归一。
//
// 偏移按曲线点序逐点相加（不是按温度查表），并按「插值依据曲线」的点序分配，所以调用方必须
// 传依据曲线，否则偏移整体错位。`Learning` 关闭时偏移整体作废，开启时先按 `LearningBias` 约束。
func EffectiveCurveForUnit(curve []types.FanCurvePoint, cfg types.SmartControlConfig, unit string) []types.FanCurvePoint {
	offsets := cfg.LearnedOffsets
	if !cfg.Learning {
		offsets = nil
	} else if biased, updated := constrainOffsetsToLearningBias(offsets, cfg.LearningBias); updated {
		offsets = biased
	}
	return buildEffectiveCurve(curve, offsets, effectiveOffsetCapForUnit(cfg, unit))
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
