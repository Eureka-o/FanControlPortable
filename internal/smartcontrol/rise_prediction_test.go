package smartcontrol

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestApplyTemperatureRisePredictionDisabled(t *testing.T) {
	cfg := types.GetDefaultSmartControlConfigForUnit(types.GetDefaultFanCurve(), types.FanSpeedUnitPercent)
	cfg.TemperatureRisePrediction = false
	got, boost := ApplyTemperatureRisePrediction(400, []RisePredictionSample{
		{ControlTemp: 50, CPUPowerWatts: 20},
		{ControlTemp: 51, CPUPowerWatts: 40},
		{ControlTemp: 52, CPUPowerWatts: 60},
		{ControlTemp: 54, CPUPowerWatts: 80},
	}, cfg, types.FanSpeedUnitPercent)
	if got != 400 || boost != 0 {
		t.Fatalf("disabled prediction = target %d boost %d, want 400/0", got, boost)
	}
}

func TestApplyTemperatureRisePredictionBoostsSustainedRise(t *testing.T) {
	cfg := types.GetDefaultSmartControlConfigForUnit(types.GetDefaultFanCurve(), types.FanSpeedUnitPercent)
	cfg.TemperatureRisePrediction = true
	cfg.TemperatureRisePredictionMaxBoost = 60
	got, boost := ApplyTemperatureRisePrediction(400, []RisePredictionSample{
		{ControlTemp: 50, CPUPowerWatts: 20},
		{ControlTemp: 51, CPUPowerWatts: 55},
		{ControlTemp: 52, CPUPowerWatts: 95},
		{ControlTemp: 54, CPUPowerWatts: 130},
	}, cfg, types.FanSpeedUnitPercent)
	if got <= 400 || boost <= 0 || boost > 60 {
		t.Fatalf("prediction target/boost = %d/%d, want positive capped boost", got, boost)
	}
}

func TestApplyTemperatureRisePredictionIgnoresSingleSpike(t *testing.T) {
	cfg := types.GetDefaultSmartControlConfigForUnit(types.GetDefaultFanCurve(), types.FanSpeedUnitPercent)
	cfg.TemperatureRisePrediction = true
	got, boost := ApplyTemperatureRisePrediction(400, []RisePredictionSample{
		{ControlTemp: 50, CPUPowerWatts: 20},
		{ControlTemp: 50, CPUPowerWatts: 20},
		{ControlTemp: 50, CPUPowerWatts: 20},
		{ControlTemp: 58, CPUPowerWatts: 20},
	}, cfg, types.FanSpeedUnitPercent)
	if got != 400 || boost != 0 {
		t.Fatalf("single spike prediction = target %d boost %d, want 400/0", got, boost)
	}
}
