package smartcontrol

import (
	"math"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

type RisePredictionSample struct {
	SampledAt        time.Time
	ControlTemp      int
	CPUTemp          int
	GPUTemp          int
	CPUPowerWatts    float64
	GPUPowerWatts    float64
	CPUPowerValid    bool
	GPUPowerValid    bool
	ControlSource    string
	PreviousTarget   int
	ActualSpeed      int
	ActualSpeedValid bool
}

type RisePredictionResult struct {
	Target           int
	Boost            int
	RampUpMultiplier float64
	PowerAssisted    bool
}

const (
	predictionPowerRampMultiplier     = 1.5
	predictionConfirmedRampMultiplier = 2.0
	maxPredictionSampleGap            = 8 * time.Second
	minPowerRiseWatts                 = 25.0
	minPowerRiseWattsPerSecond        = 8.0
	minSustainedPowerRiseWatts        = 30.0
	minSustainedPowerRiseRate         = 6.0
	minRapidNoPowerTempRise           = 3
	minRapidNoPowerTempRate           = 0.75
)

// EvaluateTemperatureRisePrediction supplies a bounded temporary ramp assist.
// Full target boost remains reserved for a confirmed sustained temperature rise.
func EvaluateTemperatureRisePrediction(target int, samples []RisePredictionSample, cfg types.SmartControlConfig, unit string) RisePredictionResult {
	result := RisePredictionResult{Target: target, RampUpMultiplier: 1}
	if !cfg.TemperatureRisePrediction || target <= 0 || len(samples) < 2 {
		return result
	}

	recent := samples
	if len(recent) > 4 {
		recent = recent[len(recent)-4:]
	}
	if !predictionSamplesAreContinuous(recent) || !sameControlSource(recent) || predictionSpeedIsLagging(recent[len(recent)-1]) {
		return result
	}
	if recent[len(recent)-1].ControlTemp < recent[0].ControlTemp {
		return result
	}
	if powerIsRisingQuickly(recent) {
		result.RampUpMultiplier = predictionPowerRampMultiplier
		result.PowerAssisted = true
	} else if rapidTemperatureRiseWithoutPower(recent) {
		result.RampUpMultiplier = predictionPowerRampMultiplier
	}
	if tempRise, confirmed := sustainedTemperatureRise(recent); confirmed {
		powerScore := normalizedControlPowerScore(recent)
		tempScore := clampFloat(float64(tempRise)/6.0, 0, 1)
		score := math.Max(tempScore, 0.65*tempScore+0.35*powerScore)
		if powerScore <= 0 {
			duration := recent[len(recent)-1].SampledAt.Sub(recent[0].SampledAt).Seconds()
			if duration > 0 {
				rateScore := clampFloat((float64(tempRise)/duration)/1.2, 0, 1)
				score = math.Max(tempScore, 0.75*tempScore+0.25*rateScore)
			}
		}
		maxBoost := cfg.TemperatureRisePredictionMaxBoost
		if maxBoost <= 0 {
			if types.IsRPMSpeedUnit(unit) {
				maxBoost = 300
			} else {
				maxBoost = 60
			}
		}
		result.Boost = int(math.Round(float64(maxBoost) * score))
		if result.Boost > 0 {
			result.Target += result.Boost
			result.RampUpMultiplier = predictionConfirmedRampMultiplier
		}
	}
	return result
}

func sustainedTemperatureRise(samples []RisePredictionSample) (int, bool) {
	if len(samples) < 4 {
		return 0, false
	}
	tempRise := samples[len(samples)-1].ControlTemp - samples[0].ControlTemp
	if tempRise < 2 {
		return 0, false
	}
	positiveSteps := 0
	largestStep := 0
	for i := 1; i < len(samples); i++ {
		step := samples[i].ControlTemp - samples[i-1].ControlTemp
		if step > 0 {
			positiveSteps++
			if step > largestStep {
				largestStep = step
			}
		}
	}
	return tempRise, positiveSteps >= 2 && largestStep < tempRise
}

func predictionSamplesAreContinuous(samples []RisePredictionSample) bool {
	for i := 1; i < len(samples); i++ {
		previous, current := samples[i-1].SampledAt, samples[i].SampledAt
		if previous.IsZero() || current.IsZero() || !current.After(previous) || current.Sub(previous) > maxPredictionSampleGap {
			return false
		}
	}
	return true
}

func sameControlSource(samples []RisePredictionSample) bool {
	if len(samples) < 2 {
		return true
	}
	source := samples[0].ControlSource
	if controlPowerDomain(samples[0]) == "" {
		return false
	}
	for _, sample := range samples[1:] {
		if sample.ControlSource != source || controlPowerDomain(sample) == "" {
			return false
		}
	}
	return true
}

func samePowerDomain(samples []RisePredictionSample) bool {
	if len(samples) < 2 {
		return true
	}
	domain := controlPowerDomain(samples[0])
	if domain == "" {
		return false
	}
	for _, sample := range samples[1:] {
		if controlPowerDomain(sample) != domain {
			return false
		}
	}
	return true
}

func powerForExplicitDomain(sample RisePredictionSample, domain string) (float64, bool) {
	switch domain {
	case "cpu":
		return sample.CPUPowerWatts, sample.CPUPowerValid && IsValidPowerWatts(sample.CPUPowerWatts)
	case "gpu":
		return sample.GPUPowerWatts, sample.GPUPowerValid && IsValidPowerWatts(sample.GPUPowerWatts)
	default:
		return 0, false
	}
}

func predictionPowerSeriesDomain(samples []RisePredictionSample) string {
	if len(samples) < 2 {
		return ""
	}
	first := samples[0]
	last := samples[len(samples)-1]
	source := first.ControlSource
	if source == "cpu" || source == "gpu" {
		return source
	}
	if source != "max" {
		return ""
	}
	firstDom := controlPowerDomain(first)
	lastDom := controlPowerDomain(last)
	_, firstDomOK := powerForExplicitDomain(first, firstDom)
	_, lastDomOK := powerForExplicitDomain(last, lastDom)
	if firstDom == lastDom && firstDomOK && lastDomOK {
		return firstDom
	}
	if !firstDomOK || !lastDomOK {
		for _, candidate := range []string{"cpu", "gpu"} {
			_, firstCandOK := powerForExplicitDomain(first, candidate)
			_, lastCandOK := powerForExplicitDomain(last, candidate)
			if firstCandOK && lastCandOK {
				return candidate
			}
		}
	}
	return ""
}

func predictionSpeedIsLagging(sample RisePredictionSample) bool {
	return sample.ActualSpeedValid && sample.PreviousTarget > 0 && sample.ActualSpeed*100 < sample.PreviousTarget*85
}

func powerIsRisingQuickly(samples []RisePredictionSample) bool {
	if len(samples) < 2 {
		return false
	}
	tail := samples[len(samples)-2:]
	if tailDomain := predictionPowerSeriesDomain(tail); tailDomain != "" {
		first, firstOK := powerForExplicitDomain(tail[0], tailDomain)
		last, lastOK := powerForExplicitDomain(tail[1], tailDomain)
		if firstOK && lastOK {
			duration := tail[1].SampledAt.Sub(tail[0].SampledAt).Seconds()
			powerRise := last - first
			if duration > 0 && powerRise >= minPowerRiseWatts && powerRise/duration >= minPowerRiseWattsPerSecond {
				return true
			}
		}
	}
	if len(samples) < 3 {
		return false
	}
	domain := predictionPowerSeriesDomain(samples)
	if domain == "" {
		return false
	}
	first, firstOK := powerForExplicitDomain(samples[0], domain)
	prev, prevOK := powerForExplicitDomain(samples[len(samples)-2], domain)
	last, lastOK := powerForExplicitDomain(samples[len(samples)-1], domain)
	if !firstOK || !prevOK || !lastOK || last < prev-2.0 {
		return false
	}
	duration := samples[len(samples)-1].SampledAt.Sub(samples[0].SampledAt).Seconds()
	powerRise := last - first
	return duration > 0 && powerRise >= minSustainedPowerRiseWatts && powerRise/duration >= minSustainedPowerRiseRate
}

func rapidTemperatureRiseWithoutPower(samples []RisePredictionSample) bool {
	if len(samples) < 2 {
		return false
	}
	window := samples
	if len(window) > 3 {
		window = window[len(window)-3:]
	}
	for i := 1; i < len(window); i++ {
		if window[i].ControlTemp <= window[i-1].ControlTemp {
			return false
		}
	}
	tempRise := window[len(window)-1].ControlTemp - window[0].ControlTemp
	duration := window[len(window)-1].SampledAt.Sub(window[0].SampledAt).Seconds()
	return tempRise >= minRapidNoPowerTempRise && duration > 0 && float64(tempRise)/duration >= minRapidNoPowerTempRate
}

func normalizedControlPowerScore(samples []RisePredictionSample) float64 {
	if len(samples) < 2 {
		return 0
	}
	domain := predictionPowerSeriesDomain(samples)
	if domain == "" {
		return 0
	}
	first, firstOK := powerForExplicitDomain(samples[0], domain)
	last, lastOK := powerForExplicitDomain(samples[len(samples)-1], domain)
	if !firstOK || !lastOK || !IsValidPowerWatts(last) {
		return 0
	}
	if !IsValidPowerWatts(first) {
		return clampFloat(last/180.0, 0, 1)
	}
	return clampFloat((last-first)/120.0, 0, 1)
}

func powerForControlSource(sample RisePredictionSample) (float64, bool) {
	switch controlPowerDomain(sample) {
	case "cpu":
		return sample.CPUPowerWatts, sample.CPUPowerValid && IsValidPowerWatts(sample.CPUPowerWatts)
	case "gpu":
		return sample.GPUPowerWatts, sample.GPUPowerValid && IsValidPowerWatts(sample.GPUPowerWatts)
	default:
		return 0, false
	}
}

func controlPowerDomain(sample RisePredictionSample) string {
	switch sample.ControlSource {
	case "cpu", "gpu":
		return sample.ControlSource
	case "max":
		if sample.CPUTemp <= 0 && sample.GPUTemp <= 0 {
			return ""
		}
		if sample.GPUTemp > sample.CPUTemp {
			return "gpu"
		}
		return "cpu"
	default:
		return ""
	}
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
