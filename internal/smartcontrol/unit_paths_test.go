package smartcontrol

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestCalculatePercentTargetTicksUsesTickPrecision(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 50},
	}
	cfg := types.SmartControlConfig{Learning: false, MaxLearnOffset: 200}

	if got := CalculatePercentTargetTicks(60, curve, cfg); got != 375 {
		t.Fatalf("CalculatePercentTargetTicks() = %d, want 375", got)
	}
}

func TestCalculatePercentTargetTicksAppliesSubPercentLearnedOffsets(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 50},
	}
	cfg := types.SmartControlConfig{
		Learning:       true,
		MaxLearnOffset: 200,
		LearnedOffsets: []int{2, 2},
	}

	if got := CalculatePercentTargetTicks(60, curve, cfg); got != 376 {
		t.Fatalf("CalculatePercentTargetTicks() = %d, want 376 ticks (37.6%%)", got)
	}
}

func TestCalculateLegacyRPMTargetUsesReferenceRPMUnits(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 1800},
		{Temperature: 70, RPM: 2800},
	}
	cfg := types.SmartControlConfig{Learning: false, MaxLearnOffset: 300}

	if got := CalculateLegacyRPMTarget(60, curve, cfg); got != 2300 {
		t.Fatalf("CalculateLegacyRPMTarget() = %d, want 2300", got)
	}
}

func TestLearnPercentSteadyOffsetStoresTenthsOfPercentTicks(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 50},
	}
	cfg := types.SmartControlConfig{
		TargetTemp:     70,
		Hysteresis:     2,
		LearnRate:      1,
		MaxLearnOffset: 200,
	}

	offsets, changed := LearnPercentSteadyOffsetTicks(0, 75, 0.005, true, curve, []int{0, 0}, cfg)
	if !changed {
		t.Fatal("expected percent learning to change offsets")
	}
	if offsets[0] != 25 {
		t.Fatalf("percent learned offset = %d ticks, want 25 ticks (2.5%%)", offsets[0])
	}
}

func TestLearnLegacyRPMSteadyOffsetKeepsOneRPMUnits(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 1800},
		{Temperature: 70, RPM: 2800},
	}
	cfg := types.SmartControlConfig{
		TargetTemp:     70,
		Hysteresis:     2,
		LearnRate:      1,
		MaxLearnOffset: 300,
	}

	offsets, changed := LearnLegacyRPMSteadyOffset(0, 75, 0.006, true, curve, []int{0, 0}, cfg)
	if !changed {
		t.Fatal("expected RPM learning to change offsets")
	}
	if offsets[0] != 21 {
		t.Fatalf("RPM learned offset = %d, want 21 RPM", offsets[0])
	}
}

func TestPercentStableObserverKeepsMeanSpeedInTicks(t *testing.T) {
	curve := CurveForUnit([]types.FanCurvePoint{{Temperature: 60, RPM: 50}}, types.FanSpeedUnitPercent)
	observer := NewPercentStableObserver(len(curve))
	cfg := types.SmartControlConfig{
		LearnWindow:  3,
		LearnDelay:   0,
		MinRPMChange: 20,
	}

	if steady := observer.Observe(60, 500, curve, cfg); steady.Ready {
		t.Fatal("first sample should not be steady")
	}
	if steady := observer.Observe(60, 505, curve, cfg); steady.Ready {
		t.Fatal("second sample should not be steady")
	}
	steady := observer.Observe(60, 510, curve, cfg)
	if !steady.Ready {
		t.Fatal("third stable tick sample should be ready")
	}
	if steady.MeanRPM != 505 {
		t.Fatalf("percent observer mean speed = %d, want 505 ticks", steady.MeanRPM)
	}
}

func TestLegacyRPMStableObserverKeepsMeanSpeedInRPM(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 60, RPM: 2000}}
	observer := NewLegacyRPMStableObserver(len(curve))
	cfg := types.SmartControlConfig{
		LearnWindow:  3,
		LearnDelay:   0,
		MinRPMChange: 50,
	}

	observer.Observe(60, 2000, curve, cfg)
	observer.Observe(60, 2010, curve, cfg)
	steady := observer.Observe(60, 2021, curve, cfg)
	if !steady.Ready {
		t.Fatal("third stable RPM sample should be ready")
	}
	if steady.MeanRPM != 2010 {
		t.Fatalf("RPM observer mean speed = %d, want 2010 RPM", steady.MeanRPM)
	}
}

func TestNormalizeConfigForUnitScalesLegacyPercentLearningFields(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 25},
		{Temperature: 70, RPM: 50},
	}
	cfg := types.SmartControlConfig{
		MinRPMChange:       2,
		RampUpLimit:        8,
		RampDownLimit:      6,
		MaxLearnOffset:     20,
		TargetTemp:         68,
		Aggressiveness:     5,
		Hysteresis:         2,
		LearnRate:          3,
		LearnWindow:        8,
		LearnDelay:         3,
		OverheatWeight:     8,
		RPMDeltaWeight:     5,
		NoiseWeight:        4,
		TrendGain:          5,
		LearnedOffsets:     []int{1, -2},
		LearnedOffsetsHeat: []int{3, 4},
		LearnedOffsetsCool: []int{-5, -6},
	}

	got, changed := NormalizeConfigForUnit(cfg, curve, false, types.FanSpeedUnitPercent)
	if !changed {
		t.Fatal("expected legacy percent config to be scaled")
	}
	if got.MinRPMChange != 20 || got.RampUpLimit != 80 || got.RampDownLimit != 60 || got.MaxLearnOffset != 200 {
		t.Fatalf("scaled smart fields = min %d up %d down %d offset %d", got.MinRPMChange, got.RampUpLimit, got.RampDownLimit, got.MaxLearnOffset)
	}
	if got.LearnedOffsets[0] != 10 || got.LearnedOffsets[1] != -20 {
		t.Fatalf("scaled learned offsets = %v, want [10 -20]", got.LearnedOffsets)
	}
}

func TestNormalizeConfigForUnitKeepsReferenceRPMDefaults(t *testing.T) {
	curve := types.GetDefaultRPMFanCurve()
	cfg := types.GetDefaultSmartControlConfigForUnit(curve, types.FanSpeedUnitRPM)

	got, changed := NormalizeConfigForUnit(cfg, curve, false, types.FanSpeedUnitRPM)
	if changed {
		t.Fatalf("reference RPM defaults should already be normalized: %#v", got)
	}
	if got.MinRPMChange != 50 || got.RampUpLimit != 220 || got.RampDownLimit != 160 || got.MaxLearnOffset != 300 {
		t.Fatalf("RPM defaults changed unexpectedly: %#v", got)
	}
}
