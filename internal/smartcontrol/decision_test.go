package smartcontrol

import (
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func TestEvaluateDecisionExplainsBaselineLearningPredictionAndRamp(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 40, RPM: 1200}, {Temperature: 80, RPM: 2400}}
	decision := EvaluateDecision(DecisionInput{
		ControlTemp:    60,
		ControlSource:  types.TempSourceCPU,
		Curve:          curve,
		Config:         types.SmartControlConfig{RampUpLimit: 300, RampDownLimit: 200},
		SpeedUnit:      types.FanSpeedUnitRPM,
		PreviousTarget: 1000,
	})

	if decision.CurveMin != 1200 || decision.CurveMax != 2400 {
		t.Fatalf("curve bounds = (%d, %d), want (1200, 2400)", decision.CurveMin, decision.CurveMax)
	}
	if decision.BaseTarget != 1800 || decision.LearnedTarget != 1800 || decision.LearningOffset != 0 {
		t.Fatalf("baseline decision = %#v", decision)
	}
	if decision.TemperatureRiseBoost != 0 || decision.PredictionRampMultiplier != 1 {
		t.Fatalf("disabled prediction = %#v", decision)
	}
	if decision.FinalTarget != 1300 || decision.RampAdjustment != -500 {
		t.Fatalf("ramped decision = %#v, want final 1300 and ramp -500", decision)
	}
	if !decision.Active || decision.ControlSource != types.TempSourceCPU || decision.SpeedUnit != types.FanSpeedUnitRPM {
		t.Fatalf("decision context = %#v", decision)
	}
}

func TestGateReasonPriority(t *testing.T) {
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
			if got := GateReason(test.autoControl, test.inputReady, test.controlReady); got != test.want {
				t.Fatalf("GateReason() = %q, want %q", got, test.want)
			}
		})
	}
}
