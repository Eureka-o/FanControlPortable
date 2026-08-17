package smartcontrol

import (
	"github.com/Eureka-o/FanControlPortable/internal/temperature"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

// Decision is the authoritative explanation of one SmartControl cycle.
// Runtime adapters may add noise/hardware adjustments and write status to the
// value returned by EvaluateDecision, but callers should publish this value
// rather than reconstructing the decision from intermediate targets.
type Decision struct {
	Timestamp                string  `json:"timestamp,omitempty"`
	Active                   bool    `json:"active"`
	ControlTemp              int     `json:"controlTemp,omitempty"`
	ControlSource            string  `json:"controlSource,omitempty"`
	SpeedUnit                string  `json:"speedUnit,omitempty"`
	CurveMin                 int     `json:"curveMin,omitempty"`
	CurveMax                 int     `json:"curveMax,omitempty"`
	BaseTarget               int     `json:"baseTarget,omitempty"`
	LearnedTarget            int     `json:"learnedTarget,omitempty"`
	LearningOffset           int     `json:"learningOffset,omitempty"`
	PowerAvailable           bool    `json:"powerAvailable"`
	PowerAssisted            bool    `json:"powerAssisted"`
	TemperatureRiseBoost     int     `json:"temperatureRiseBoost,omitempty"`
	PredictionRampMultiplier float64 `json:"predictionRampMultiplier,omitempty"`
	RampAdjustment           int     `json:"rampAdjustment,omitempty"`
	NoiseAdjustment          int     `json:"noiseAdjustment,omitempty"`
	HardwareAdjustment       int     `json:"hardwareAdjustment,omitempty"`
	SafetyFallback           bool    `json:"safetyFallback"`
	FinalTarget              int     `json:"finalTarget,omitempty"`
	WriteAttempted           bool    `json:"writeAttempted"`
	WriteSucceeded           bool    `json:"writeSucceeded"`
	GateReason               string  `json:"gateReason,omitempty"`
	Confidence               string  `json:"confidence,omitempty"`
}

type DecisionInput struct {
	ControlTemp           int
	ControlSource         string
	Curve                 []types.FanCurvePoint
	Config                types.SmartControlConfig
	SpeedUnit             string
	PreviousTarget        int
	AdvancedTelemetry     bool
	PowerAvailable        bool
	RisePredictionSamples []RisePredictionSample
}

// EvaluateDecision owns all device-independent target selection. Device
// adapters only need to apply their real runtime limits to FinalTarget.
func EvaluateDecision(input DecisionInput) Decision {
	unit := types.NormalizeFanSpeedUnit(input.SpeedUnit)
	controlCurve := CurveForUnit(input.Curve, unit)
	curveMin, curveMax := GetCurveRPMBounds(controlCurve)
	baseTarget := temperature.CalculateTargetRPM(input.ControlTemp, controlCurve)

	learnedTarget := CalculateTargetSpeedForUnit(input.ControlTemp, input.Curve, input.Config, unit)
	if learnedTarget <= 0 {
		learnedTarget = baseTarget
	}
	if learnedTarget > 0 {
		learnedTarget = clampInt(learnedTarget, curveMin, curveMax)
	}

	prediction := RisePredictionResult{Target: learnedTarget, RampUpMultiplier: 1}
	if input.AdvancedTelemetry {
		prediction = EvaluateTemperatureRisePrediction(learnedTarget, input.RisePredictionSamples, input.Config, unit)
	}
	predictedTarget := prediction.Target
	if predictedTarget <= 0 {
		predictedTarget = learnedTarget
	}
	if predictedTarget > 0 {
		predictedTarget = clampInt(predictedTarget, curveMin, curveMax)
	}

	finalTarget := predictedTarget
	if input.PreviousTarget >= 0 {
		rampUpLimit := input.Config.RampUpLimit
		if rampUpLimit > 0 && prediction.RampUpMultiplier > 1 {
			rampUpLimit = int(float64(rampUpLimit)*prediction.RampUpMultiplier + 0.5)
		}
		finalTarget = ApplyRampLimit(finalTarget, input.PreviousTarget, rampUpLimit, input.Config.RampDownLimit)
		if finalTarget > 0 {
			finalTarget = clampInt(finalTarget, curveMin, curveMax)
		}
	}

	return Decision{
		Active:                   true,
		ControlTemp:              input.ControlTemp,
		ControlSource:            types.NormalizeTempSource(input.ControlSource),
		SpeedUnit:                unit,
		CurveMin:                 curveMin,
		CurveMax:                 curveMax,
		BaseTarget:               baseTarget,
		LearnedTarget:            learnedTarget,
		LearningOffset:           learnedTarget - baseTarget,
		PowerAvailable:           input.PowerAvailable,
		PowerAssisted:            prediction.PowerAssisted,
		TemperatureRiseBoost:     predictedTarget - learnedTarget,
		PredictionRampMultiplier: prediction.RampUpMultiplier,
		RampAdjustment:           finalTarget - predictedTarget,
		FinalTarget:              finalTarget,
	}
}

func GateReason(autoControl, inputReady, controlReady bool) string {
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
