package coreapp

import "time"

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
