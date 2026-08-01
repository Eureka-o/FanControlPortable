package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestSmartControlGateReason(t *testing.T) {
	tests := []struct {
		name         string
		autoControl  bool
		inputReady   bool
		controlReady bool
		want         string
	}{
		{name: "active", autoControl: true, inputReady: true, controlReady: true},
		{name: "automatic control disabled", inputReady: true, controlReady: true, want: "automatic-control-disabled"},
		{name: "temperature unavailable", autoControl: true, controlReady: true, want: "temperature-unavailable"},
		{name: "device not ready", autoControl: true, inputReady: true, want: "device-not-ready"},
		{name: "disabled takes priority", want: "automatic-control-disabled"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := smartControlGateReason(test.autoControl, test.inputReady, test.controlReady); got != test.want {
				t.Fatalf("smartControlGateReason() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestEvaluateSmartControlTargetKeepsBaselineAndPredictionInOneDecision(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 40, RPM: 1200}, {Temperature: 80, RPM: 2400}}
	result := evaluateSmartControlTarget(smartControlTargetInput{
		ControlTemp:       60,
		Curve:             curve,
		Config:            types.SmartControlConfig{},
		SpeedUnit:         types.FanSpeedUnitRPM,
		AdvancedTelemetry: false,
	})
	if result.CurveMin != 1200 || result.CurveMax != 2400 {
		t.Fatalf("curve bounds = (%d, %d), want (1200, 2400)", result.CurveMin, result.CurveMax)
	}
	if result.BaseTarget != 1800 || result.LearnedTarget != 1800 || result.Target != 1800 {
		t.Fatalf("target decision = %#v, want 1800 baseline/target", result)
	}
	if result.RisePrediction.RampUpMultiplier != 1 {
		t.Fatalf("disabled prediction multiplier = %v, want 1", result.RisePrediction.RampUpMultiplier)
	}
}

func TestApplySmartControlRampUsesPredictionMultiplierAndCurveBounds(t *testing.T) {
	result := applySmartControlRamp(smartControlRampInput{
		Target:               2400,
		PreviousTarget:       1000,
		CurveMin:             800,
		CurveMax:             2200,
		Config:               types.SmartControlConfig{RampUpLimit: 300, RampDownLimit: 200},
		PredictionMultiplier: 2,
	})
	if result.Target != 1600 || result.Adjustment != -800 {
		t.Fatalf("ramp result = %#v, want target 1600 and adjustment -800", result)
	}

	result = applySmartControlRamp(smartControlRampInput{
		Target:         700,
		PreviousTarget: 1000,
		CurveMin:       800,
		CurveMax:       2200,
		Config:         types.SmartControlConfig{RampDownLimit: 200},
	})
	if result.Target != 800 || result.Adjustment != 100 {
		t.Fatalf("bounded ramp result = %#v, want target 800 and adjustment 100", result)
	}
}
