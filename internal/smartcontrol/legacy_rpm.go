package smartcontrol

import "github.com/TIANLI0/THRM/internal/types"

// CalculateLegacyRPMTarget keeps the reference-style RPM path separate from
// FanControl's percent/tick control path.
func CalculateLegacyRPMTarget(currentTemp int, curve []types.FanCurvePoint, cfg types.SmartControlConfig) int {
	return calculateTargetSpeed(currentTemp, curve, cfg, types.FanSpeedUnitRPM)
}

func NewLegacyRPMStableObserver(curveLen int) *StableObserver {
	return NewStableObserverForUnit(curveLen, types.FanSpeedUnitRPM)
}

func LearnLegacyRPMSteadyOffset(
	bucketIdx int,
	steadyMeanTemp int,
	localEff float64,
	haveEff bool,
	curve []types.FanCurvePoint,
	prevOffsets []int,
	cfg types.SmartControlConfig,
) ([]int, bool) {
	return LearnSteadyOffsetForUnit(bucketIdx, steadyMeanTemp, localEff, haveEff, curve, prevOffsets, cfg, types.FanSpeedUnitRPM)
}

func LearnLegacyRPMSteadyOffsetWithPower(
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
	return LearnSteadyOffsetForUnitWithPower(bucketIdx, steadyMeanTemp, steadyMeanPower, havePower, localEff, haveEff, curve, prevOffsets, cfg, types.FanSpeedUnitRPM)
}
