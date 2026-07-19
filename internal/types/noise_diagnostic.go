package types

import (
	"fmt"
	"math"
	"sort"
)

const (
	NoiseDiagnosticFlyDigiMinRPM = 1000
	NoiseDiagnosticPercentMin    = 5
	NoiseDiagnosticDefaultStep   = 1
)

type NoiseDiagnosticRange struct {
	Unit      string `json:"unit"`
	Min       int    `json:"min"`
	Max       int    `json:"max"`
	Step      int    `json:"step"`
	MinSource string `json:"minSource"`
	MaxSource string `json:"maxSource"`
}

type NoiseDiagnosticPoint struct {
	Requested int     `json:"requested"`
	Actual    int     `json:"actual"`
	LevelDB   float64 `json:"levelDb"`
	SpreadDB  float64 `json:"spreadDb"`
	Valid     bool    `json:"valid"`
}

type NoiseDiagnosticResult struct {
	DeviceKey        string                 `json:"deviceKey"`
	Unit             string                 `json:"unit"`
	Points           []NoiseDiagnosticPoint `json:"points"`
	BaselineDB       float64                `json:"baselineDb"`
	BaselineDriftDB  float64                `json:"baselineDriftDb"`
	RiseDB           float64                `json:"riseDb"`
	Knee             int                    `json:"knee"`
	SuspectedPeak    int                    `json:"suspectedPeak,omitempty"`
	Confidence       string                 `json:"confidence"`
	ConfidenceReason string                 `json:"confidenceReason"`
	Microphone       string                 `json:"microphone"`
	TestedAt         int64                  `json:"testedAt"`
}

type NoiseDiagnosticBeginRequest struct {
	DeviceKey string               `json:"deviceKey"`
	Range     NoiseDiagnosticRange `json:"range"`
}

type NoiseDiagnosticSession struct {
	SessionID      string               `json:"sessionId"`
	DeviceKey      string               `json:"deviceKey"`
	Range          NoiseDiagnosticRange `json:"range"`
	ConfigRevision uint64               `json:"configRevision"`
}

type NoiseDiagnosticTargetResult struct {
	Requested int    `json:"requested"`
	Actual    int    `json:"actual"`
	Unit      string `json:"unit"`
}

func NoiseDiagnosticRangeForProfile(profile DeviceProfile, capabilities DeviceCapabilities, fanData *FanData) (NoiseDiagnosticRange, error) {
	profile = NormalizeDeviceProfile(profile, "")
	capabilities = NormalizeDeviceCapabilities(capabilities)
	unit := NormalizeFanSpeedUnit(profile.SpeedUnit)
	if unit == FanSpeedUnitPercent && capabilities.SpeedUnit == FanSpeedUnitRPM {
		unit = FanSpeedUnitRPM
	}

	speedRange := profile.SpeedRange
	if speedRange.Max <= 0 {
		speedRange = capabilities.SpeedRange
	}
	if speedRange.Max <= 0 {
		return NoiseDiagnosticRange{}, fmt.Errorf("device profile has no usable maximum speed")
	}

	min := speedRange.Min
	minSource := "profile"
	if unit == FanSpeedUnitPercent {
		min = NoiseDiagnosticPercentMin
		minSource = "percent-diagnostic-floor"
	} else if IsFlyDigiDeviceProfileID(profile.ID) && (profile.Transport == DeviceTransportBLE || profile.Transport == DeviceTransportHID) {
		min = NoiseDiagnosticFlyDigiMinRPM
		minSource = "flydigi-diagnostic-floor"
	} else if min <= 0 {
		return NoiseDiagnosticRange{}, fmt.Errorf("device profile has no usable minimum speed")
	}

	max := speedRange.Max
	maxSource := "profile"
	if fanData != nil && fanData.FlyDigiCapability != nil && fanData.FlyDigiCapability.Available && fanData.FlyDigiCapability.MaxRPM > 0 && unit == FanSpeedUnitRPM {
		if fanData.FlyDigiCapability.MaxRPM < max {
			max = fanData.FlyDigiCapability.MaxRPM
		}
		maxSource = "runtime-capability"
	}
	if max <= min {
		return NoiseDiagnosticRange{}, fmt.Errorf("device speed range does not contain a diagnostic interval")
	}

	step := speedRange.Step
	if step <= 0 {
		step = NoiseDiagnosticDefaultStep
	}
	return NoiseDiagnosticRange{
		Unit:      unit,
		Min:       min,
		Max:       max,
		Step:      step,
		MinSource: minSource,
		MaxSource: maxSource,
	}, nil
}

func NormalizeNoiseDiagnosticRange(requested, allowed NoiseDiagnosticRange) (NoiseDiagnosticRange, error) {
	allowed.Unit = NormalizeFanSpeedUnit(allowed.Unit)
	if allowed.Min <= 0 || allowed.Max <= allowed.Min {
		return NoiseDiagnosticRange{}, fmt.Errorf("invalid diagnostic speed range")
	}
	if NormalizeFanSpeedUnit(requested.Unit) != allowed.Unit {
		return NoiseDiagnosticRange{}, fmt.Errorf("diagnostic speed unit does not match active device")
	}

	min := requested.Min
	if min < allowed.Min || min == 0 {
		min = allowed.Min
	}
	if min > allowed.Max {
		min = allowed.Max
	}
	max := requested.Max
	if max == 0 || max > allowed.Max {
		max = allowed.Max
	}
	if max < allowed.Min {
		max = allowed.Min
	}
	if max <= min {
		return NoiseDiagnosticRange{}, fmt.Errorf("diagnostic range must have a positive interval")
	}

	step := allowed.Step
	if requested.Step > 0 && requested.Step < step {
		step = requested.Step
	}
	if step <= 0 {
		step = NoiseDiagnosticDefaultStep
	}
	return NoiseDiagnosticRange{
		Unit:      allowed.Unit,
		Min:       min,
		Max:       max,
		Step:      step,
		MinSource: allowed.MinSource,
		MaxSource: allowed.MaxSource,
	}, nil
}

func NormalizeNoiseDiagnosticResult(result NoiseDiagnosticResult) (NoiseDiagnosticResult, bool) {
	result.Unit = NormalizeFanSpeedUnit(result.Unit)
	cleaned := make([]NoiseDiagnosticPoint, 0, len(result.Points))
	changed := false
	for _, point := range result.Points {
		if !point.Valid || point.Requested <= 0 || point.Actual <= 0 || !finiteNoiseValue(point.LevelDB) || !finiteNoiseValue(point.SpreadDB) {
			changed = true
			continue
		}
		cleaned = append(cleaned, point)
	}
	sort.SliceStable(cleaned, func(i, j int) bool {
		if cleaned[i].Actual == cleaned[j].Actual {
			return cleaned[i].Requested < cleaned[j].Requested
		}
		return cleaned[i].Actual < cleaned[j].Actual
	})
	if len(cleaned) != len(result.Points) {
		changed = true
	}
	result.Points = cleaned
	for _, value := range []float64{result.BaselineDB, result.BaselineDriftDB, result.RiseDB} {
		if !finiteNoiseValue(value) {
			changed = true
		}
	}
	if !finiteNoiseValue(result.BaselineDB) {
		result.BaselineDB = 0
	}
	if !finiteNoiseValue(result.BaselineDriftDB) {
		result.BaselineDriftDB = 0
	}
	if !finiteNoiseValue(result.RiseDB) {
		result.RiseDB = 0
	}
	if result.RiseDB < 0 {
		result.RiseDB = 0
		changed = true
	}
	return result, changed
}

func finiteNoiseValue(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
