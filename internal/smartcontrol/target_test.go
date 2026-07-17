package smartcontrol

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestCalculateTargetRPMIgnoresOffsetsWhenLearningDisabled(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 50},
	}
	cfg := types.SmartControlConfig{
		Learning:       false,
		MaxLearnOffset: 20,
		LearnedOffsets: []int{10, 10},
	}

	got := CalculateTargetRPM(60, curve, cfg)
	if got != 37 {
		t.Fatalf("CalculateTargetRPM() = %d, want base curve speed 37", got)
	}
}

func TestCalculateTargetRPMForCurveEmptyCurve(t *testing.T) {
	if got := CalculateTargetRPMForCurve(nil, 70); got != 0 {
		t.Fatalf("CalculateTargetRPMForCurve(nil) = %d, want 0", got)
	}
}

func TestCalculateTargetRPMForCurveClampsEndpoints(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 40, RPM: 1200},
		{Temperature: 80, RPM: 3600},
	}

	tests := []struct {
		name string
		temp int
		want int
	}{
		{name: "below first", temp: 35, want: 1200},
		{name: "at first", temp: 40, want: 1200},
		{name: "at last", temp: 80, want: 3600},
		{name: "above last", temp: 85, want: 3600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateTargetRPMForCurve(curve, tt.temp); got != tt.want {
				t.Fatalf("CalculateTargetRPMForCurve(temp=%d) = %d, want %d", tt.temp, got, tt.want)
			}
		})
	}
}

func TestCalculateTargetRPMForCurveInterpolatesAndRounds(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 1000},
		{Temperature: 70, RPM: 2001},
	}

	if got := CalculateTargetRPMForCurve(curve, 60); got != 1501 {
		t.Fatalf("CalculateTargetRPMForCurve() = %d, want rounded midpoint 1501", got)
	}
}

func TestCalculateTargetRPMForCurvePreservesRPMScaleValues(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 40, RPM: 1800},
		{Temperature: 60, RPM: 2600},
		{Temperature: 80, RPM: 4200},
	}

	if got := CalculateTargetRPMForCurve(curve, 70); got != 3400 {
		t.Fatalf("CalculateTargetRPMForCurve() = %d, want RPM-scale interpolation 3400", got)
	}
}

func TestCalculateTargetRPMAppliesOffsetsWhenLearningEnabled(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 50},
	}
	cfg := types.SmartControlConfig{
		Learning:       true,
		MaxLearnOffset: 20,
		LearnedOffsets: []int{10, 10},
	}

	got := CalculateTargetRPM(60, curve, cfg)
	if got != 42 {
		t.Fatalf("CalculateTargetRPM() = %d, want learned curve speed 42", got)
	}
}

func TestCalculateTargetRPMRespectsCoolingBias(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 50},
	}
	cfg := types.SmartControlConfig{
		Learning:       true,
		LearningBias:   types.LearningBiasCooling,
		MaxLearnOffset: 20,
		LearnedOffsets: []int{-10, -10},
	}

	got := CalculateTargetRPM(60, curve, cfg)
	if got != 37 {
		t.Fatalf("CalculateTargetRPM() = %d, want base curve speed 37", got)
	}
}

func TestCalculateTargetRPMRespectsQuietBias(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 50},
	}
	cfg := types.SmartControlConfig{
		Learning:       true,
		LearningBias:   types.LearningBiasQuiet,
		MaxLearnOffset: 20,
		LearnedOffsets: []int{10, 10},
	}

	got := CalculateTargetRPM(60, curve, cfg)
	if got != 37 {
		t.Fatalf("CalculateTargetRPM() = %d, want base curve speed 37", got)
	}
}

func TestLearnSteadyOffsetRespectsLearningBias(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 50},
	}
	prevOffsets := []int{0, 0}

	// 低于目标带的工况会要求降转速（负偏移），cooling 倾向禁止负偏移 → 不变。
	if offsets, changed := LearnSteadyOffset(1, 60, 0, false, curve, prevOffsets, types.SmartControlConfig{
		TargetTemp:     70,
		LearningBias:   types.LearningBiasCooling,
		LearnRate:      10,
		MaxLearnOffset: 20,
	}); changed || offsets[0] != 0 || offsets[1] != 0 {
		t.Fatalf("cooling bias learned offsets = %v, changed=%v; want unchanged zeros", offsets, changed)
	}

	// 高于目标温度的工况会要求加转速（正偏移），quiet 倾向禁止正偏移 → 不变。
	if offsets, changed := LearnSteadyOffset(0, 80, 0, false, curve, prevOffsets, types.SmartControlConfig{
		TargetTemp:     70,
		LearningBias:   types.LearningBiasQuiet,
		LearnRate:      10,
		MaxLearnOffset: 20,
	}); changed || offsets[0] != 0 || offsets[1] != 0 {
		t.Fatalf("quiet bias learned offsets = %v, changed=%v; want unchanged zeros", offsets, changed)
	}
}

func TestStableObserverUsesConfiguredWindowAndDelay(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 60, RPM: 45}}
	observer := NewStableObserver(len(curve))
	cfg := types.SmartControlConfig{
		LearnWindow:    4,
		LearnDelay:     2,
		MinRPMChange:   2,
		TargetTemp:     68,
		MaxLearnOffset: 20,
	}

	for i := range 5 {
		if steady := observer.Observe(60, 45, curve, cfg); steady.Ready {
			t.Fatalf("sample %d unexpectedly reached steady state", i)
		}
	}
	if steady := observer.Observe(60, 45, curve, cfg); !steady.Ready {
		t.Fatalf("expected steady state after configured delay+window")
	}
}

func TestLearnSteadyOffsetHoldsInComfortBand(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 50},
	}
	cfg := types.SmartControlConfig{
		TargetTemp:     70,
		Hysteresis:     2,
		LearnRate:      10,
		MaxLearnOffset: 20,
	}
	// 舒适带为 [70-5, 70] = [65,70]，带内不应再调整（消除“无脑降温”）。
	if offsets, changed := LearnSteadyOffset(1, 68, 0, false, curve, []int{0, 0}, cfg); changed {
		t.Fatalf("in-band steady temp should not change offsets, got %v changed=%v", offsets, changed)
	}
}

func TestLearnSteadyOffsetOnlyAdjustsLocalBucket(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 50},
		{Temperature: 90, RPM: 75},
	}
	cfg := types.SmartControlConfig{
		TargetTemp:     70,
		Hysteresis:     2,
		LearnRate:      10,
		MaxLearnOffset: 20,
	}
	offsets, changed := LearnSteadyOffset(1, 82, 0, false, curve, []int{0, 0, 0}, cfg)
	if !changed {
		t.Fatalf("expected local bucket learning to change offsets")
	}
	if offsets[1] <= 0 {
		t.Fatalf("expected middle bucket offset to increase, got %v", offsets)
	}
	if offsets[0] != 0 || offsets[2] != 0 {
		t.Fatalf("expected neighboring buckets to remain unchanged, got %v", offsets)
	}
	if offsets[1] >= 8 {
		t.Fatalf("expected smoothing to keep a single-step change below the hard step cap, got %v", offsets)
	}
}

func TestLearnSteadyOffsetCoolsWhenAboveTarget(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 50},
	}
	cfg := types.SmartControlConfig{
		TargetTemp:     70,
		Hysteresis:     2,
		LearnRate:      10,
		MaxLearnOffset: 20,
	}
	offsets, changed := LearnSteadyOffset(0, 80, 0, false, curve, []int{0, 0}, cfg)
	if !changed || offsets[0] <= 0 {
		t.Fatalf("above-target steady temp should raise RPM offset, got %v changed=%v", offsets, changed)
	}
}

func TestLearnSteadyOffsetSavesNoiseWhenBelowTarget(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 50},
	}
	cfg := types.SmartControlConfig{
		TargetTemp:     70,
		Hysteresis:     2,
		LearnRate:      10,
		MaxLearnOffset: 20,
	}
	offsets, changed := LearnSteadyOffset(1, 55, 0, false, curve, []int{0, 0}, cfg)
	if !changed || offsets[1] >= 0 {
		t.Fatalf("well-below-target steady temp should lower RPM offset, got %v changed=%v", offsets, changed)
	}
}

// 冷却低效时，同样的温差应允许更大幅度的降速（更省噪音）。
func TestLearnSteadyOffsetEfficiencyScalesReduction(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 75},
	}
	cfg := types.SmartControlConfig{
		TargetTemp:     70,
		Hysteresis:     2,
		LearnRate:      6,
		MaxLearnOffset: 30,
	}
	effHigh, _ := LearnSteadyOffset(1, 55, 0.8, true, curve, []int{0, 0}, cfg)
	effLow, _ := LearnSteadyOffset(1, 55, 0.2, true, curve, []int{0, 0}, cfg)
	if !(effLow[1] < effHigh[1]) {
		t.Fatalf("lower cooling efficiency should reduce RPM more aggressively: low=%d high=%d", effLow[1], effHigh[1])
	}
}

func TestStableObserverSkipsLearningWhenPowerIsUnstable(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 60, RPM: 45}}
	observer := NewStableObserver(len(curve))
	cfg := types.SmartControlConfig{
		LearnWindow:  3,
		LearnDelay:   0,
		MinRPMChange: 2,
	}

	observer.ObserveWithPower(60, 45, 35, true, curve, cfg)
	observer.ObserveWithPower(60, 45, 80, true, curve, cfg)
	if steady := observer.ObserveWithPower(60, 45, 120, true, curve, cfg); steady.Ready {
		t.Fatalf("unstable power window should not be learned: %#v", steady)
	}

	observer.ObserveWithPower(60, 45, 100, true, curve, cfg)
	observer.ObserveWithPower(60, 45, 104, true, curve, cfg)
	steady := observer.ObserveWithPower(60, 45, 106, true, curve, cfg)
	if !steady.Ready || !steady.HavePower {
		t.Fatalf("stable power window should be learned with power, got %#v", steady)
	}
}

func TestStableObserverUsesComparablePowerForEfficiency(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 60, RPM: 2000}}
	observer := NewLegacyRPMStableObserver(len(curve))
	cfg := types.SmartControlConfig{
		LearnWindow:  3,
		LearnDelay:   0,
		MinRPMChange: 50,
	}

	for range 3 {
		observer.ObserveWithPower(70, 1800, 50, true, curve, cfg)
	}
	for range 3 {
		observer.ObserveWithPower(58, 2400, 52, true, curve, cfg)
	}
	steady := SteadyResult{}
	for range 3 {
		steady = observer.ObserveWithPower(57, 2500, 54, true, curve, cfg)
	}
	if !steady.Ready || !steady.HaveEff {
		t.Fatalf("expected comparable-power RPM samples to estimate efficiency, got %#v", steady)
	}
	if steady.LocalEff < 0.015 || steady.LocalEff > 0.025 {
		t.Fatalf("local efficiency = %.4f, want around 0.02 C/RPM", steady.LocalEff)
	}

	for range 3 {
		observer.ObserveWithPower(90, 2000, 150, true, curve, cfg)
	}
	steady = SteadyResult{}
	for range 3 {
		steady = observer.ObserveWithPower(89, 2040, 152, true, curve, cfg)
	}
	if steady.Ready && steady.HaveEff {
		t.Fatalf("distant power samples should not reuse low-power efficiency, got %#v", steady)
	}
}

func TestLearnSteadyOffsetReliableEfficiencyIgnoresPowerGain(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 75},
	}
	cfg := types.SmartControlConfig{
		TargetTemp:     70,
		Hysteresis:     2,
		LearnRate:      6,
		MaxLearnOffset: 30,
	}

	lowPower, _ := LearnSteadyOffsetForUnitWithPower(1, 55, 30, true, 0.2, true, curve, []int{0, 0}, cfg, rawPercentUnit)
	highPower, _ := LearnSteadyOffsetForUnitWithPower(1, 55, 140, true, 0.2, true, curve, []int{0, 0}, cfg, rawPercentUnit)
	if lowPower[1] != highPower[1] {
		t.Fatalf("reliable efficiency should ignore global power gain: low=%v high=%v", lowPower, highPower)
	}
}

func TestLearnSteadyOffsetHalvesQuietStep(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 75},
	}
	cfg := types.SmartControlConfig{
		TargetTemp:     70,
		Hysteresis:     2,
		LearnRate:      6,
		MaxLearnOffset: 30,
	}

	offsets, changed := LearnSteadyOffsetForUnitWithPower(1, 55, 50, true, 0.2, true, curve, []int{0, 0}, cfg, rawPercentUnit)
	if !changed || offsets[1] != -2 {
		t.Fatalf("quiet offset = %v changed=%v, want [0 -2]/true", offsets, changed)
	}
}

func TestStableObserverQuietLearningRequiresTwoConsecutiveMatchingSteadies(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 60, RPM: 2000}}
	observer := NewLegacyRPMStableObserver(len(curve))
	cfg := types.SmartControlConfig{
		LearnWindow:  3,
		LearnDelay:   0,
		MinRPMChange: 50,
	}
	power := EffectivePower{CPUWatts: 50, GPUWatts: 20, CPUValid: true, GPUValid: true}

	first := SteadyResult{}
	for range 3 {
		first = observer.ObserveWithEffectivePower(60, 2000, power, curve, cfg)
	}
	if !first.Ready || !first.HavePower || first.QuietLearningReady {
		t.Fatalf("first reliable steady = %#v, want ready without quiet-learning approval", first)
	}

	second := SteadyResult{}
	for range 3 {
		second = observer.ObserveWithEffectivePower(60, 2000, power, curve, cfg)
	}
	if !second.Ready || !second.QuietLearningReady {
		t.Fatalf("second matching steady = %#v, want quiet-learning approval", second)
	}
}

func TestStableObserverQuietLearningAllowsTwoConsecutiveNoPowerSteadies(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 60, RPM: 2000}}
	observer := NewLegacyRPMStableObserver(len(curve))
	cfg := types.SmartControlConfig{
		LearnWindow:  3,
		LearnDelay:   0,
		MinRPMChange: 50,
	}

	first := SteadyResult{}
	for range 3 {
		first = observer.Observe(60, 2000, curve, cfg)
	}
	if !first.Ready || first.HavePower || first.QuietLearningReady {
		t.Fatalf("first no-power steady = %#v, want ready without quiet-learning approval", first)
	}

	second := SteadyResult{}
	for range 3 {
		second = observer.Observe(60, 2000, curve, cfg)
	}
	if !second.Ready || second.HavePower || !second.QuietLearningReady {
		t.Fatalf("second no-power steady = %#v, want quiet-learning approval", second)
	}
}

func TestAllowsSteadyOffsetLearningRequiresConfirmationOnlyForQuietAdjustment(t *testing.T) {
	cfg := types.SmartControlConfig{TargetTemp: 70, Hysteresis: 2}

	if !AllowsSteadyOffsetLearning(SteadyResult{Ready: true, MeanTemp: 71}, cfg) {
		t.Fatal("high-temperature safety learning must not wait for quiet confirmation")
	}
	if AllowsSteadyOffsetLearning(SteadyResult{Ready: true, MeanTemp: 64}, cfg) {
		t.Fatal("first quiet learning sample must wait for confirmation")
	}
	if !AllowsSteadyOffsetLearning(SteadyResult{Ready: true, MeanTemp: 64, QuietLearningReady: true}, cfg) {
		t.Fatal("confirmed quiet learning sample must be allowed")
	}
}

func TestStableObserverKnownPowerDoesNotReuseUnknownHistory(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 60, RPM: 2000}}
	observer := NewLegacyRPMStableObserver(len(curve))
	cfg := types.SmartControlConfig{
		LearnWindow:  3,
		LearnDelay:   0,
		MinRPMChange: 50,
	}

	for range 3 {
		observer.ObserveWithEffectivePower(70, 1800, EffectivePower{}, curve, cfg)
	}

	steady := SteadyResult{}
	for range 3 {
		steady = observer.ObserveWithEffectivePower(58, 2400, EffectivePower{
			CPUWatts: 50,
			CPUValid: true,
		}, curve, cfg)
	}
	if !steady.Ready || !steady.HavePower {
		t.Fatalf("known-power sample should be steady with power, got %#v", steady)
	}
	if steady.HaveEff {
		t.Fatalf("known-power sample must not reuse unknown-power history: %#v", steady)
	}
}

func TestStableObserverUnknownPowerDoesNotReplaceKnownContext(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 60, RPM: 2000}}
	observer := NewLegacyRPMStableObserver(len(curve))
	cfg := types.SmartControlConfig{LearnWindow: 3, LearnDelay: 0, MinRPMChange: 50}
	known := EffectivePower{CPUWatts: 50, CPUValid: true}

	for range 3 {
		observer.ObserveWithEffectivePower(70, 1800, known, curve, cfg)
	}
	for range 3 {
		observer.ObserveWithEffectivePower(60, 2400, known, curve, cfg)
	}
	for range 3 {
		observer.ObserveWithEffectivePower(70, 1800, EffectivePower{}, curve, cfg)
	}

	knownPoints := 0
	for _, point := range observer.history[0] {
		if point.power.CPUValid {
			knownPoints++
		}
	}
	if knownPoints < 2 {
		t.Fatalf("unknown-power steady must not replace comparable known history: %#v", observer.history[0])
	}
}

func TestStableObserverKnownPowerDoesNotMixSwappedComponents(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 60, RPM: 2000}}
	observer := NewLegacyRPMStableObserver(len(curve))
	cfg := types.SmartControlConfig{
		LearnWindow:  3,
		LearnDelay:   0,
		MinRPMChange: 50,
	}
	firstContext := EffectivePower{CPUWatts: 50, GPUWatts: 10, CPUValid: true, GPUValid: true}

	for range 3 {
		observer.ObserveWithEffectivePower(70, 1800, firstContext, curve, cfg)
	}
	for range 3 {
		observer.ObserveWithEffectivePower(58, 2400, firstContext, curve, cfg)
	}

	steady := SteadyResult{}
	for range 3 {
		steady = observer.ObserveWithEffectivePower(57, 2500, EffectivePower{
			CPUWatts: 10,
			GPUWatts: 50,
			CPUValid: true,
			GPUValid: true,
		}, curve, cfg)
	}
	if !steady.Ready || !steady.HavePower {
		t.Fatalf("swapped-component sample should still be a valid steady sample, got %#v", steady)
	}
	if steady.HaveEff {
		t.Fatalf("same total power with swapped CPU/GPU components must not reuse efficiency: %#v", steady)
	}
}
