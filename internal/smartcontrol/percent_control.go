package smartcontrol

import "github.com/TIANLI0/THRM/internal/types"

// CalculatePercentTargetTicks is the FanControl-owned percent control path.
// Config curves stay in 0-100 percent for compatibility; this path converts
// them to 0.1% ticks before applying learning and ramp decisions.
func CalculatePercentTargetTicks(currentTemp int, curve []types.FanCurvePoint, cfg types.SmartControlConfig) int {
	return CalculateTargetSpeedForUnit(currentTemp, curve, cfg, types.FanSpeedUnitPercent)
}

func NewPercentStableObserver(curveLen int) *StableObserver {
	return NewStableObserverForUnit(curveLen, types.FanSpeedUnitPercent)
}

func LearnPercentSteadyOffsetTicks(
	bucketIdx int,
	steadyMeanTemp int,
	localEff float64,
	haveEff bool,
	curve []types.FanCurvePoint,
	prevOffsets []int,
	cfg types.SmartControlConfig,
) ([]int, bool) {
	tickCurve := CurveForUnit(curve, types.FanSpeedUnitPercent)
	return LearnSteadyOffsetForUnit(bucketIdx, steadyMeanTemp, localEff, haveEff, tickCurve, prevOffsets, cfg, types.FanSpeedUnitPercent)
}

func LearnPercentSteadyOffsetTicksWithPower(
	bucketIdx int,
	steadyMeanTemp int,
	steadyMeanPower float64,
	havePower bool,
	localEff float64,
	haveEff bool,
	curve []types.FanCurvePoint,
	prevOffsets []int,
	cfg types.SmartControlConfig,
) ([]int, bool) {
	tickCurve := CurveForUnit(curve, types.FanSpeedUnitPercent)
	return LearnSteadyOffsetForUnitWithPower(bucketIdx, steadyMeanTemp, steadyMeanPower, havePower, localEff, haveEff, tickCurve, prevOffsets, cfg, types.FanSpeedUnitPercent)
}
