package coreapp

import (
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/smartcontrol"
	"github.com/Eureka-o/FanControlPortable/internal/types"
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
			if got := smartcontrol.GateReason(test.autoControl, test.inputReady, test.controlReady); got != test.want {
				t.Fatalf("GateReason() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestEvaluateSmartControlTargetKeepsBaselineAndPredictionInOneDecision(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 40, RPM: 1200}, {Temperature: 80, RPM: 2400}}
	result := smartcontrol.EvaluateDecision(smartcontrol.DecisionInput{
		ControlTemp:    60,
		Curve:          curve,
		Config:         types.SmartControlConfig{},
		SpeedUnit:      types.FanSpeedUnitRPM,
		PreviousTarget: -1,
	})
	if result.CurveMin != 1200 || result.CurveMax != 2400 {
		t.Fatalf("curve bounds = (%d, %d), want (1200, 2400)", result.CurveMin, result.CurveMax)
	}
	if result.BaseTarget != 1800 || result.LearnedTarget != 1800 || result.FinalTarget != 1800 {
		t.Fatalf("target decision = %#v, want 1800 baseline/target", result)
	}
	if result.PredictionRampMultiplier != 1 {
		t.Fatalf("disabled prediction multiplier = %v, want 1", result.PredictionRampMultiplier)
	}
}

func TestEvaluateSmartControlDecisionAppliesRampAndCurveBounds(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 40, RPM: 800}, {Temperature: 80, RPM: 2200}}
	result := smartcontrol.EvaluateDecision(smartcontrol.DecisionInput{
		ControlTemp:    80,
		Curve:          curve,
		Config:         types.SmartControlConfig{RampUpLimit: 300, RampDownLimit: 200},
		SpeedUnit:      types.FanSpeedUnitRPM,
		PreviousTarget: 1000,
	})
	if result.FinalTarget != 1300 || result.RampAdjustment != -900 {
		t.Fatalf("ramped decision = %#v, want target 1300 and adjustment -900", result)
	}
}
