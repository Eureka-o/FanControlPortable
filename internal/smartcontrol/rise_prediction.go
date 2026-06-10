package smartcontrol

import (
	"math"

	"github.com/TIANLI0/THRM/internal/types"
)

type RisePredictionSample struct {
	ControlTemp   int
	CPUPowerWatts float64
	GPUPowerWatts float64
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
