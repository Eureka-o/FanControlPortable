package smartcontrol

import (
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
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
}
