package smartcontrol

import (
	"math"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
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
}

const (
	predictionPowerRampMultiplier     = 1.5
	predictionConfirmedRampMultiplier = 2.0
	maxPredictionSampleGap            = 8 * time.Second
	minPowerRiseWatts                 = 25.0
	minPowerRiseWattsPerSecond        = 8.0
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
	if powerIsRisingQuickly(recent[len(recent)-2:]) {
		result.RampUpMultiplier = predictionPowerRampMultiplier
	}
	if tempRise, confirmed := sustainedTemperatureRise(recent); confirmed {
		powerScore := normalizedControlPowerScore(recent)
		tempScore := clampFloat(float64(tempRise)/6.0, 0, 1)
		score := math.Max(tempScore, 0.65*tempScore+0.35*powerScore)
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
	domain := controlPowerDomain(samples[0])
	if domain == "" {
		return false
	}
	for _, sample := range samples[1:] {
		if sample.ControlSource != source || controlPowerDomain(sample) != domain {
			return false
		}
	}
	return true
}

func predictionSpeedIsLagging(sample RisePredictionSample) bool {
	return sample.ActualSpeedValid && sample.PreviousTarget > 0 && sample.ActualSpeed*100 < sample.PreviousTarget*85
}

func powerIsRisingQuickly(samples []RisePredictionSample) bool {
	first, firstOK := powerForControlSource(samples[0])
	last, lastOK := powerForControlSource(samples[len(samples)-1])
	if !firstOK || !lastOK {
		return false
	}
	duration := samples[len(samples)-1].SampledAt.Sub(samples[0].SampledAt).Seconds()
	powerRise := last - first
	return duration > 0 && powerRise >= minPowerRiseWatts && powerRise/duration >= minPowerRiseWattsPerSecond
}

func normalizedControlPowerScore(samples []RisePredictionSample) float64 {
	if len(samples) < 2 {
		return 0
	}
	first, firstOK := powerForControlSource(samples[0])
	last, lastOK := powerForControlSource(samples[len(samples)-1])
	if !firstOK || !lastOK || last <= 0 {
		return 0
	}
	if first <= 0 {
		return clampFloat(last/180.0, 0, 1)
	}
	return clampFloat((last-first)/120.0, 0, 1)
}

func powerForControlSource(sample RisePredictionSample) (float64, bool) {
	switch controlPowerDomain(sample) {
	case "cpu":
		return sample.CPUPowerWatts, sample.CPUPowerValid && sample.CPUPowerWatts >= 0
	case "gpu":
		return sample.GPUPowerWatts, sample.GPUPowerValid && sample.GPUPowerWatts >= 0
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

func ApplyTemperatureRisePrediction(target int, samples []RisePredictionSample, cfg types.SmartControlConfig, unit string) (int, int) {
	if !cfg.TemperatureRisePrediction || target <= 0 || len(samples) < 4 {
		return target, 0
	}

	recent := samples[len(samples)-4:]
	tempRise := recent[len(recent)-1].ControlTemp - recent[0].ControlTemp
	if tempRise < 2 {
		return target, 0
	}

	positiveSteps := 0
	largestStep := 0
	for i := 1; i < len(recent); i++ {
		step := recent[i].ControlTemp - recent[i-1].ControlTemp
		if step > 0 {
			positiveSteps++
			if step > largestStep {
				largestStep = step
			}
		}
	}
	if positiveSteps < 2 || largestStep >= tempRise {
		return target, 0
	}

	powerScore := normalizedPowerScore(recent)
	tempScore := clampFloat(float64(tempRise)/6.0, 0, 1)
	score := 0.65*tempScore + 0.35*powerScore
	if score < 0.25 {
		return target, 0
	}

	maxBoost := cfg.TemperatureRisePredictionMaxBoost
	if maxBoost <= 0 {
		if types.IsRPMSpeedUnit(unit) {
			maxBoost = 300
		} else {
			maxBoost = 60
		}
	}
	boost := int(math.Round(float64(maxBoost) * score))
	if boost <= 0 {
		return target, 0
	}
	return target + boost, boost
}

func normalizedPowerScore(samples []RisePredictionSample) float64 {
	if len(samples) < 2 {
		return 0
	}
	first := totalPower(samples[0])
	last := totalPower(samples[len(samples)-1])
	if last <= 0 {
		return 0
	}
	if first <= 0 {
		return clampFloat(last/180.0, 0, 1)
	}
	return clampFloat((last-first)/120.0, 0, 1)
}

func totalPower(sample RisePredictionSample) float64 {
	total := 0.0
	if sample.CPUPowerWatts > 0 {
		total += sample.CPUPowerWatts
	}
	if sample.GPUPowerWatts > 0 {
		total += sample.GPUPowerWatts
	}
	return total
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
