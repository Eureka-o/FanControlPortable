package coreapp

import (
	"time"

	"github.com/TIANLI0/THRM/internal/smartcontrol"
	"github.com/TIANLI0/THRM/internal/temperature"
	"github.com/TIANLI0/THRM/internal/types"
)

type smartControlTargetInput struct {
	ControlTemp           int
	Curve                 []types.FanCurvePoint
	Config                types.SmartControlConfig
	SpeedUnit             string
	AdvancedTelemetry     bool
	RisePredictionSamples []smartcontrol.RisePredictionSample
}

type smartControlTargetResult struct {
	CurveMin       int
	CurveMax       int
	BaseTarget     int
	LearnedTarget  int
	Target         int
	RisePrediction smartcontrol.RisePredictionResult
}

type smartControlRampInput struct {
	Target               int
	PreviousTarget       int
	CurveMin             int
	CurveMax             int
	Config               types.SmartControlConfig
	PredictionMultiplier float64
}

type smartControlRampResult struct {
	Target     int
	Adjustment int
}

// evaluateSmartControlTarget owns the pure target-selection portion of the
// monitoring decision. Hardware writes and device-specific limits stay in the
// monitoring Module because they require runtime adapters.
func evaluateSmartControlTarget(input smartControlTargetInput) smartControlTargetResult {
	unit := types.NormalizeFanSpeedUnit(input.SpeedUnit)
	controlCurve := smartcontrol.CurveForUnit(input.Curve, unit)
	curveMin, curveMax := smartcontrol.GetCurveRPMBounds(controlCurve)
	baseTarget := temperature.CalculateTargetRPM(input.ControlTemp, controlCurve)

	learnedTarget := 0
	if types.IsPercentSpeedUnit(unit) {
		learnedTarget = smartcontrol.CalculatePercentTargetTicks(input.ControlTemp, input.Curve, input.Config)
	} else {
		learnedTarget = smartcontrol.CalculateLegacyRPMTarget(input.ControlTemp, input.Curve, input.Config)
	}
	target := learnedTarget
	if target <= 0 {
		target = baseTarget
	}
	if target > 0 {
		target = min(max(target, curveMin), curveMax)
	}
	learnedTarget = target

	prediction := smartcontrol.RisePredictionResult{Target: target, RampUpMultiplier: 1}
	if input.AdvancedTelemetry {
		prediction = smartcontrol.EvaluateTemperatureRisePrediction(target, input.RisePredictionSamples, input.Config, unit)
		if prediction.Target > 0 {
			target = min(max(prediction.Target, curveMin), curveMax)
		}
	}
	return smartControlTargetResult{
		CurveMin:       curveMin,
		CurveMax:       curveMax,
		BaseTarget:     baseTarget,
		LearnedTarget:  learnedTarget,
		Target:         target,
		RisePrediction: prediction,
	}
}

func applySmartControlRamp(input smartControlRampInput) smartControlRampResult {
	target := input.Target
	if input.PreviousTarget >= 0 {
		rampUpLimit := input.Config.RampUpLimit
		if rampUpLimit > 0 && input.PredictionMultiplier > 1 {
			rampUpLimit = int(float64(rampUpLimit)*input.PredictionMultiplier + 0.5)
		}
		target = smartcontrol.ApplyRampLimit(target, input.PreviousTarget, rampUpLimit, input.Config.RampDownLimit)
		if target > 0 {
			target = min(max(target, input.CurveMin), input.CurveMax)
		}
	}
	return smartControlRampResult{Target: target, Adjustment: target - input.Target}
}

type smartControlDecisionSnapshot struct {
	Timestamp            string `json:"timestamp,omitempty"`
	Active               bool   `json:"active"`
	ControlTemp          int    `json:"controlTemp,omitempty"`
	ControlSource        string `json:"controlSource,omitempty"`
	SpeedUnit            string `json:"speedUnit,omitempty"`
	BaseTarget           int    `json:"baseTarget,omitempty"`
	LearnedTarget        int    `json:"learnedTarget,omitempty"`
	LearningOffset       int    `json:"learningOffset,omitempty"`
	PowerAvailable       bool   `json:"powerAvailable"`
	PowerAssisted        bool   `json:"powerAssisted"`
	TemperatureRiseBoost int    `json:"temperatureRiseBoost,omitempty"`
	RampAdjustment       int    `json:"rampAdjustment,omitempty"`
	NoiseAdjustment      int    `json:"noiseAdjustment,omitempty"`
	HardwareAdjustment   int    `json:"hardwareAdjustment,omitempty"`
	SafetyFallback       bool   `json:"safetyFallback"`
	FinalTarget          int    `json:"finalTarget,omitempty"`
	WriteAttempted       bool   `json:"writeAttempted"`
	WriteSucceeded       bool   `json:"writeSucceeded"`
	GateReason           string `json:"gateReason,omitempty"`
	Confidence           string `json:"confidence,omitempty"`
}

func (a *CoreApp) setSmartControlDecision(snapshot smartControlDecisionSnapshot) {
	if a == nil {
		return
	}
	if snapshot.Timestamp == "" {
		snapshot.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	a.smartControlDecisionMu.Lock()
	a.smartControlDecision = snapshot
	a.smartControlDecisionMu.Unlock()
}

func (a *CoreApp) getSmartControlDecision() smartControlDecisionSnapshot {
	if a == nil {
		return smartControlDecisionSnapshot{}
	}
	a.smartControlDecisionMu.RLock()
	snapshot := a.smartControlDecision
	a.smartControlDecisionMu.RUnlock()
	return snapshot
}

func smartControlGateReason(autoControl, inputReady, controlReady bool) string {
	switch {
	case !autoControl:
		return "automatic-control-disabled"
	case !inputReady:
		return "temperature-unavailable"
	case !controlReady:
		return "device-not-ready"
	default:
		return ""
	}
}
