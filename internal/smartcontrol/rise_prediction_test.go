package smartcontrol

import (
	"testing"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func TestEvaluateTemperatureRisePredictionUsesPowerRiseForLimitedRampAssist(t *testing.T) {
	cfg := types.GetDefaultSmartControlConfigForUnit(types.GetDefaultFanCurve(), types.FanSpeedUnitPercent)
	cfg.TemperatureRisePrediction = true
	start := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	result := EvaluateTemperatureRisePrediction(400, []RisePredictionSample{
		{SampledAt: start, ControlTemp: 60, CPUTemp: 60, CPUPowerWatts: 25, CPUPowerValid: true, ControlSource: "cpu", PreviousTarget: 400, ActualSpeed: 400, ActualSpeedValid: true},
		{SampledAt: start.Add(2 * time.Second), ControlTemp: 60, CPUTemp: 60, CPUPowerWatts: 60, CPUPowerValid: true, ControlSource: "cpu", PreviousTarget: 400, ActualSpeed: 400, ActualSpeedValid: true},
	}, cfg, types.FanSpeedUnitPercent)

	if result.Target != 400 || result.Boost != 0 {
		t.Fatalf("power-only prediction target/boost = %d/%d, want 400/0", result.Target, result.Boost)
	}
	if result.RampUpMultiplier <= 1 || result.RampUpMultiplier > 1.5 {
		t.Fatalf("power-only ramp multiplier = %v, want (1, 1.5]", result.RampUpMultiplier)
	}
	if !result.PowerAssisted {
		t.Fatal("power-only prediction should report power-assisted ramping")
	}
}

func TestEvaluateTemperatureRisePredictionBoostsOnlyConfirmedSustainedRise(t *testing.T) {
	cfg := types.GetDefaultSmartControlConfigForUnit(types.GetDefaultFanCurve(), types.FanSpeedUnitPercent)
	cfg.TemperatureRisePrediction = true
	cfg.TemperatureRisePredictionMaxBoost = 60
	start := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	result := EvaluateTemperatureRisePrediction(400, []RisePredictionSample{
		{SampledAt: start, ControlTemp: 50, CPUTemp: 50, CPUPowerWatts: 20, CPUPowerValid: true, ControlSource: "cpu", PreviousTarget: 400, ActualSpeed: 400, ActualSpeedValid: true},
		{SampledAt: start.Add(2 * time.Second), ControlTemp: 51, CPUTemp: 51, CPUPowerWatts: 55, CPUPowerValid: true, ControlSource: "cpu", PreviousTarget: 400, ActualSpeed: 400, ActualSpeedValid: true},
		{SampledAt: start.Add(4 * time.Second), ControlTemp: 52, CPUTemp: 52, CPUPowerWatts: 95, CPUPowerValid: true, ControlSource: "cpu", PreviousTarget: 400, ActualSpeed: 400, ActualSpeedValid: true},
		{SampledAt: start.Add(6 * time.Second), ControlTemp: 54, CPUTemp: 54, CPUPowerWatts: 130, CPUPowerValid: true, ControlSource: "cpu", PreviousTarget: 400, ActualSpeed: 400, ActualSpeedValid: true},
	}, cfg, types.FanSpeedUnitPercent)

	if result.Target <= 400 || result.Boost <= 0 || result.Boost > 60 {
		t.Fatalf("sustained-rise target/boost = %d/%d, want positive capped boost", result.Target, result.Boost)
	}
	if result.RampUpMultiplier <= predictionPowerRampMultiplier || result.RampUpMultiplier > 2 {
		t.Fatalf("sustained-rise ramp multiplier = %v, want (%v, 2]", result.RampUpMultiplier, predictionPowerRampMultiplier)
	}
}

func TestEvaluateTemperatureRisePredictionRejectsUnknownControlSource(t *testing.T) {
	cfg := types.GetDefaultSmartControlConfigForUnit(types.GetDefaultFanCurve(), types.FanSpeedUnitPercent)
	cfg.TemperatureRisePrediction = true
	start := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	result := EvaluateTemperatureRisePrediction(400, []RisePredictionSample{
		{SampledAt: start, ControlTemp: 50, CPUPowerWatts: 20, CPUPowerValid: true},
		{SampledAt: start.Add(2 * time.Second), ControlTemp: 51, CPUPowerWatts: 55, CPUPowerValid: true},
		{SampledAt: start.Add(4 * time.Second), ControlTemp: 52, CPUPowerWatts: 95, CPUPowerValid: true},
		{SampledAt: start.Add(6 * time.Second), ControlTemp: 54, CPUPowerWatts: 130, CPUPowerValid: true},
	}, cfg, types.FanSpeedUnitPercent)

	if result.Target != 400 || result.Boost != 0 || result.RampUpMultiplier != 1 {
		t.Fatalf("unknown-source prediction = %#v, want no prediction", result)
	}
}

func TestEvaluateTemperatureRisePredictionRejectsStaleOrLaggingSamples(t *testing.T) {
	cfg := types.GetDefaultSmartControlConfigForUnit(types.GetDefaultFanCurve(), types.FanSpeedUnitPercent)
	cfg.TemperatureRisePrediction = true
	start := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	for name, samples := range map[string][]RisePredictionSample{
		"time gap": {
			{SampledAt: start, ControlTemp: 50, CPUPowerWatts: 20, CPUPowerValid: true, ControlSource: "cpu"},
			{SampledAt: start.Add(2 * time.Second), ControlTemp: 51, CPUPowerWatts: 55, CPUPowerValid: true, ControlSource: "cpu"},
			{SampledAt: start.Add(12 * time.Second), ControlTemp: 52, CPUPowerWatts: 95, CPUPowerValid: true, ControlSource: "cpu"},
			{SampledAt: start.Add(14 * time.Second), ControlTemp: 54, CPUPowerWatts: 130, CPUPowerValid: true, ControlSource: "cpu"},
		},
		"fan lag": {
			{SampledAt: start, ControlTemp: 50, CPUPowerWatts: 20, CPUPowerValid: true, ControlSource: "cpu"},
			{SampledAt: start.Add(2 * time.Second), ControlTemp: 51, CPUPowerWatts: 55, CPUPowerValid: true, ControlSource: "cpu"},
			{SampledAt: start.Add(4 * time.Second), ControlTemp: 52, CPUPowerWatts: 95, CPUPowerValid: true, ControlSource: "cpu"},
			{SampledAt: start.Add(6 * time.Second), ControlTemp: 54, CPUPowerWatts: 130, CPUPowerValid: true, ControlSource: "cpu", PreviousTarget: 400, ActualSpeed: 250, ActualSpeedValid: true},
		},
	} {
		t.Run(name, func(t *testing.T) {
			result := EvaluateTemperatureRisePrediction(400, samples, cfg, types.FanSpeedUnitPercent)
			if result.Target != 400 || result.Boost != 0 || result.RampUpMultiplier != 1 {
				t.Fatalf("unsafe samples prediction = %#v, want no prediction", result)
			}
		})
	}
}

func TestEvaluateTemperatureRisePredictionUsesSelectedPowerDomain(t *testing.T) {
	cfg := types.GetDefaultSmartControlConfigForUnit(types.GetDefaultFanCurve(), types.FanSpeedUnitPercent)
	cfg.TemperatureRisePrediction = true
	start := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	cpuResult := EvaluateTemperatureRisePrediction(400, []RisePredictionSample{
		{SampledAt: start, ControlTemp: 60, CPUTemp: 60, GPUTemp: 50, CPUPowerWatts: 30, GPUPowerWatts: 20, CPUPowerValid: true, GPUPowerValid: true, ControlSource: "cpu"},
		{SampledAt: start.Add(2 * time.Second), ControlTemp: 60, CPUTemp: 60, GPUTemp: 50, CPUPowerWatts: 30, GPUPowerWatts: 70, CPUPowerValid: true, GPUPowerValid: true, ControlSource: "cpu"},
	}, cfg, types.FanSpeedUnitPercent)
	if cpuResult.RampUpMultiplier != 1 {
		t.Fatalf("GPU-only rise under CPU control = %#v, want no ramp assist", cpuResult)
	}

	maxResult := EvaluateTemperatureRisePrediction(400, []RisePredictionSample{
		{SampledAt: start, ControlTemp: 70, CPUTemp: 60, GPUTemp: 70, CPUPowerWatts: 30, GPUPowerWatts: 20, CPUPowerValid: true, GPUPowerValid: true, ControlSource: "max"},
		{SampledAt: start.Add(2 * time.Second), ControlTemp: 70, CPUTemp: 60, GPUTemp: 70, CPUPowerWatts: 30, GPUPowerWatts: 70, CPUPowerValid: true, GPUPowerValid: true, ControlSource: "max"},
	}, cfg, types.FanSpeedUnitPercent)
	if maxResult.RampUpMultiplier != predictionPowerRampMultiplier {
		t.Fatalf("GPU rise under max control = %#v, want %v ramp", maxResult, predictionPowerRampMultiplier)
	}

	switchedMaxResult := EvaluateTemperatureRisePrediction(400, []RisePredictionSample{
		{SampledAt: start, ControlTemp: 70, CPUTemp: 70, GPUTemp: 60, CPUPowerWatts: 20, GPUPowerWatts: 30, CPUPowerValid: true, GPUPowerValid: true, ControlSource: "max"},
		{SampledAt: start.Add(2 * time.Second), ControlTemp: 70, CPUTemp: 60, GPUTemp: 70, CPUPowerWatts: 30, GPUPowerWatts: 70, CPUPowerValid: true, GPUPowerValid: true, ControlSource: "max"},
	}, cfg, types.FanSpeedUnitPercent)
	if switchedMaxResult.RampUpMultiplier != 1 {
		t.Fatalf("max-domain switch = %#v, want no ramp assist", switchedMaxResult)
	}
}

func TestEvaluateTemperatureRisePredictionStillBoostsSustainedRiseWithoutPower(t *testing.T) {
	cfg := types.GetDefaultSmartControlConfigForUnit(types.GetDefaultFanCurve(), types.FanSpeedUnitPercent)
	cfg.TemperatureRisePrediction = true
	start := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	result := EvaluateTemperatureRisePrediction(400, []RisePredictionSample{
		{SampledAt: start, ControlTemp: 50, ControlSource: "cpu"},
		{SampledAt: start.Add(2 * time.Second), ControlTemp: 51, ControlSource: "cpu"},
		{SampledAt: start.Add(4 * time.Second), ControlTemp: 52, ControlSource: "cpu"},
		{SampledAt: start.Add(6 * time.Second), ControlTemp: 54, ControlSource: "cpu"},
	}, cfg, types.FanSpeedUnitPercent)
	if result.Target <= 400 || result.Boost <= 0 || result.RampUpMultiplier != predictionConfirmedRampMultiplier {
		t.Fatalf("sustained no-power prediction = %#v, want confirmed boost", result)
	}
	if result.PowerAssisted {
		t.Fatal("temperature-only boost should not report power assistance")
	}
}

func TestEvaluateTemperatureRisePredictionFallsBackToTempRiseAcrossMaxDomainSwitchAndIgnoresNoise(t *testing.T) {
	cfg := types.GetDefaultSmartControlConfigForUnit(types.GetDefaultFanCurve(), types.FanSpeedUnitPercent)
	cfg.TemperatureRisePrediction = true
	start := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	noiseResult := EvaluateTemperatureRisePrediction(400, []RisePredictionSample{
		{SampledAt: start, ControlTemp: 60, CPUTemp: 60, CPUPowerWatts: 0.05, CPUPowerValid: true, ControlSource: "cpu"},
		{SampledAt: start.Add(2 * time.Second), ControlTemp: 60, CPUTemp: 60, CPUPowerWatts: 40, CPUPowerValid: true, ControlSource: "cpu"},
	}, cfg, types.FanSpeedUnitPercent)
	if noiseResult.RampUpMultiplier != 1 || noiseResult.PowerAssisted {
		t.Fatalf("sub-threshold initial power noise must not trigger rapid power assist: %#v", noiseResult)
	}

	fallbackResult := EvaluateTemperatureRisePrediction(400, []RisePredictionSample{
		{SampledAt: start, ControlTemp: 55, CPUTemp: 55, GPUTemp: 50, GPUPowerWatts: 30, GPUPowerValid: true, ControlSource: "max"},
		{SampledAt: start.Add(2 * time.Second), ControlTemp: 56, CPUTemp: 56, GPUTemp: 54, GPUPowerWatts: 35, GPUPowerValid: true, ControlSource: "max"},
		{SampledAt: start.Add(4 * time.Second), ControlTemp: 58, CPUTemp: 56, GPUTemp: 58, GPUPowerWatts: 65, GPUPowerValid: true, ControlSource: "max"},
		{SampledAt: start.Add(6 * time.Second), ControlTemp: 60, CPUTemp: 57, GPUTemp: 60, GPUPowerWatts: 90, GPUPowerValid: true, ControlSource: "max"},
	}, cfg, types.FanSpeedUnitPercent)
	if fallbackResult.Target <= 400 || fallbackResult.Boost <= 0 || fallbackResult.RampUpMultiplier != predictionConfirmedRampMultiplier {
		t.Fatalf("max domain switch with sustained rise must fall back to temperature boost = %#v", fallbackResult)
	}
}

func TestEvaluateTemperatureRisePredictionSustainedPowerRampAndFallbacks(t *testing.T) {
	cfg := types.GetDefaultSmartControlConfigForUnit(types.GetDefaultFanCurve(), types.FanSpeedUnitPercent)
	cfg.TemperatureRisePrediction = true
	start := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	// Sustained 4-frame power ramp (>=30W total and >=6 W/s) without a single >=25W jump.
	sustainedPower := EvaluateTemperatureRisePrediction(400, []RisePredictionSample{
		{SampledAt: start, ControlTemp: 55, CPUTemp: 55, CPUPowerWatts: 25, CPUPowerValid: true, ControlSource: "cpu", PreviousTarget: 400, ActualSpeed: 400, ActualSpeedValid: true},
		{SampledAt: start.Add(2 * time.Second), ControlTemp: 55, CPUTemp: 55, CPUPowerWatts: 38, CPUPowerValid: true, ControlSource: "cpu", PreviousTarget: 400, ActualSpeed: 400, ActualSpeedValid: true},
		{SampledAt: start.Add(4 * time.Second), ControlTemp: 56, CPUTemp: 56, CPUPowerWatts: 52, CPUPowerValid: true, ControlSource: "cpu", PreviousTarget: 400, ActualSpeed: 400, ActualSpeedValid: true},
		{SampledAt: start.Add(6 * time.Second), ControlTemp: 56, CPUTemp: 56, CPUPowerWatts: 65, CPUPowerValid: true, ControlSource: "cpu", PreviousTarget: 400, ActualSpeed: 400, ActualSpeedValid: true},
	}, cfg, types.FanSpeedUnitPercent)
	if !sustainedPower.PowerAssisted || sustainedPower.RampUpMultiplier != predictionPowerRampMultiplier {
		t.Fatalf("sustained multi-frame power ramp = %#v, want PowerAssisted ramp", sustainedPower)
	}

	// ControlSource="max" where CPU dominates temperature but CPU power is unreadable -> falls back to readable GPU power.
	maxGpuFallback := EvaluateTemperatureRisePrediction(400, []RisePredictionSample{
		{SampledAt: start, ControlTemp: 62, CPUTemp: 62, GPUTemp: 55, CPUPowerValid: false, GPUPowerWatts: 25, GPUPowerValid: true, ControlSource: "max"},
		{SampledAt: start.Add(2 * time.Second), ControlTemp: 62, CPUTemp: 62, GPUTemp: 56, CPUPowerValid: false, GPUPowerWatts: 70, GPUPowerValid: true, ControlSource: "max"},
	}, cfg, types.FanSpeedUnitPercent)
	if !maxGpuFallback.PowerAssisted || maxGpuFallback.RampUpMultiplier != predictionPowerRampMultiplier {
		t.Fatalf("max mode with unreadable CPU power must fall back to GPU power ramp = %#v", maxGpuFallback)
	}

	// Pure temperature rapid rise (+3C over 4s, no power sensors) triggers pre-boost ramp assist.
	rapidTempNoPower := EvaluateTemperatureRisePrediction(400, []RisePredictionSample{
		{SampledAt: start, ControlTemp: 50, CPUTemp: 50, ControlSource: "cpu"},
		{SampledAt: start.Add(2 * time.Second), ControlTemp: 51, CPUTemp: 51, ControlSource: "cpu"},
		{SampledAt: start.Add(4 * time.Second), ControlTemp: 53, CPUTemp: 53, ControlSource: "cpu"},
	}, cfg, types.FanSpeedUnitPercent)
	if rapidTempNoPower.RampUpMultiplier != predictionPowerRampMultiplier || rapidTempNoPower.PowerAssisted {
		t.Fatalf("rapid temperature rise without power = %#v, want non-power ramp assist", rapidTempNoPower)
	}
}
