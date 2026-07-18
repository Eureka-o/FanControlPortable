package smartcontrol

import (
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func observeSteadySegment(observer *StableObserver, start time.Time, power EffectivePower, curve []types.FanCurvePoint, cfg types.SmartControlConfig) SteadyResult {
	result := SteadyResult{}
	for i := 0; i < cfg.LearnWindow; i++ {
		result = observer.ObserveWithEffectivePowerAt(start.Add(time.Duration(i)*time.Second), 60, 2000, power, curve, cfg)
	}
	return result
}

func TestStableObserverLongTermLearningNeedsThreeSpacedSteadies(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 60, RPM: 2000}}
	cfg := types.SmartControlConfig{LearnWindow: 3, LearnDelay: 0, MinRPMChange: 50}
	power := EffectivePower{CPUWatts: 50, GPUWatts: 20, CPUValid: true, GPUValid: true}
	observer := NewLegacyRPMStableObserver(len(curve))
	start := time.Unix(0, 0)

	first := observeSteadySegment(observer, start, power, curve, cfg)
	second := observeSteadySegment(observer, start.Add(45*time.Second), power, curve, cfg)
	third := observeSteadySegment(observer, start.Add(90*time.Second), power, curve, cfg)

	if !first.Ready || first.LearningReady {
		t.Fatalf("first steady = %#v, want short-ready only", first)
	}
	if !second.Ready || second.LearningReady {
		t.Fatalf("second steady = %#v, want short-ready only", second)
	}
	if !third.Ready || !third.LearningReady {
		t.Fatalf("third steady = %#v, want long-term learning approval", third)
	}
	if third.MeanEffectivePower != power || third.MeanPower != 70 || !third.HavePower {
		t.Fatalf("long-term power = %#v total=%v have=%v, want preserved CPU/GPU context", third.MeanEffectivePower, third.MeanPower, third.HavePower)
	}
}

func TestStableObserverLongTermPowerContextChangeRestartsWindow(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 60, RPM: 2000}}
	cfg := types.SmartControlConfig{LearnWindow: 3, LearnDelay: 0, MinRPMChange: 50}
	firstPower := EffectivePower{CPUWatts: 50, GPUWatts: 20, CPUValid: true, GPUValid: true}
	secondPower := EffectivePower{CPUWatts: 20, GPUWatts: 50, CPUValid: true, GPUValid: true}
	observer := NewLegacyRPMStableObserver(len(curve))
	start := time.Unix(0, 0)

	observeSteadySegment(observer, start, firstPower, curve, cfg)
	observeSteadySegment(observer, start.Add(45*time.Second), firstPower, curve, cfg)
	changed := observeSteadySegment(observer, start.Add(90*time.Second), secondPower, curve, cfg)
	if changed.LearningReady {
		t.Fatalf("power composition change must not complete old window: %#v", changed)
	}

	observeSteadySegment(observer, start.Add(135*time.Second), secondPower, curve, cfg)
	ready := observeSteadySegment(observer, start.Add(180*time.Second), secondPower, curve, cfg)
	if !ready.LearningReady {
		t.Fatalf("new power context should complete after three matching steadies: %#v", ready)
	}
}

func TestStableObserverLongTermGapRestartsWindow(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 60, RPM: 2000}}
	cfg := types.SmartControlConfig{LearnWindow: 3, LearnDelay: 0, MinRPMChange: 50}
	power := EffectivePower{CPUWatts: 50, CPUValid: true}
	observer := NewLegacyRPMStableObserver(len(curve))
	start := time.Unix(0, 0)

	observeSteadySegment(observer, start, power, curve, cfg)
	gap := observeSteadySegment(observer, start.Add(3*time.Minute), power, curve, cfg)
	if gap.LearningReady {
		t.Fatalf("long gap must restart aggregate: %#v", gap)
	}

	observeSteadySegment(observer, start.Add(3*time.Minute+45*time.Second), power, curve, cfg)
	ready := observeSteadySegment(observer, start.Add(3*time.Minute+90*time.Second), power, curve, cfg)
	if !ready.LearningReady {
		t.Fatalf("three steadies after the gap should complete a new aggregate: %#v", ready)
	}
}

func TestAllowsLongTermOffsetLearningKeepsSafetyImmediate(t *testing.T) {
	cfg := types.SmartControlConfig{TargetTemp: 70, Hysteresis: 2}
	if !AllowsLongTermOffsetLearning(SteadyResult{Ready: true, MeanTemp: 71}, cfg) {
		t.Fatal("high-temperature safety learning must remain immediate")
	}
	if AllowsLongTermOffsetLearning(SteadyResult{Ready: true, MeanTemp: 64, QuietLearningReady: true}, cfg) {
		t.Fatal("quiet learning must wait for the long-term window")
	}
	if !AllowsLongTermOffsetLearning(SteadyResult{Ready: true, MeanTemp: 64, LearningReady: true, QuietLearningReady: true}, cfg) {
		t.Fatal("completed long-term quiet learning should be allowed")
	}
}
