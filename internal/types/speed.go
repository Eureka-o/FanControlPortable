package types

import (
	"math"
	"strings"
)

const (
	// PercentSpeedTick is the internal precision for percent-based devices.
	// 1 tick = 0.1%, so 50% is stored as 500 ticks.
	PercentSpeedTicksPerPercent = 10
	FanSpeedMinPercentTicks     = FanSpeedMinPercent * PercentSpeedTicksPerPercent
	FanSpeedMaxPercentTicks     = FanSpeedMaxPercent * PercentSpeedTicksPerPercent
	DefaultMaxFanRPM            = 4000
	LegacyRPMManualGearMin      = 800
	LegacyRPMManualGearMax      = 4500
)

// FanSpeedValue carries an explicit speed unit while preserving the existing
// integer storage model used by older config fields.
type FanSpeedValue struct {
	Unit  string `json:"unit"`
	Value int    `json:"value"`
}

func NormalizeFanSpeedUnit(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case FanSpeedUnitRPM:
		return FanSpeedUnitRPM
	default:
		return FanSpeedUnitPercent
	}
}

func IsPercentSpeedUnit(unit string) bool {
	return NormalizeFanSpeedUnit(unit) == FanSpeedUnitPercent
}

func IsRPMSpeedUnit(unit string) bool {
	return NormalizeFanSpeedUnit(unit) == FanSpeedUnitRPM
}

func ClampRPM(rpm int) int {
	if rpm < 0 {
		return 0
	}
	return rpm
}

func ClampSpeedForUnit(value int, unit string) int {
	if IsRPMSpeedUnit(unit) {
		return ClampRPM(value)
	}
	return ClampFanPercent(value)
}

func PercentToTicks(percent int) int {
	return ClampFanPercent(percent) * PercentSpeedTicksPerPercent
}

func PercentFloatToTicks(percent float64) int {
	if math.IsNaN(percent) || math.IsInf(percent, 0) {
		return FanSpeedMinPercentTicks
	}
	return ClampPercentTicks(int(math.Round(percent * PercentSpeedTicksPerPercent)))
}

func ClampPercentTicks(ticks int) int {
	if ticks < FanSpeedMinPercentTicks {
		return FanSpeedMinPercentTicks
	}
	if ticks > FanSpeedMaxPercentTicks {
		return FanSpeedMaxPercentTicks
	}
	return ticks
}

func PercentTicksToIntegerPercent(ticks int) int {
	ticks = ClampPercentTicks(ticks)
	return (ticks + PercentSpeedTicksPerPercent/2) / PercentSpeedTicksPerPercent
}

func PercentTicksToDecimalPercent(ticks int) float64 {
	return float64(ClampPercentTicks(ticks)) / PercentSpeedTicksPerPercent
}

func NewPercentSpeed(percent int) FanSpeedValue {
	return FanSpeedValue{Unit: FanSpeedUnitPercent, Value: PercentToTicks(percent)}
}

func NewPercentTickSpeed(ticks int) FanSpeedValue {
	return FanSpeedValue{Unit: FanSpeedUnitPercent, Value: ClampPercentTicks(ticks)}
}

func NewRPMSpeed(rpm int) FanSpeedValue {
	return FanSpeedValue{Unit: FanSpeedUnitRPM, Value: ClampRPM(rpm)}
}

func (s FanSpeedValue) Normalized() FanSpeedValue {
	s.Unit = NormalizeFanSpeedUnit(s.Unit)
	if s.Unit == FanSpeedUnitRPM {
		s.Value = ClampRPM(s.Value)
	} else {
		s.Value = ClampPercentTicks(s.Value)
	}
	return s
}

func (s FanSpeedValue) IntegerPercentForSend() (int, bool) {
	if !IsPercentSpeedUnit(s.Unit) {
		return 0, false
	}
	return PercentTicksToIntegerPercent(s.Value), true
}

func SpeedRangeForUnit(unit string) (int, int) {
	if IsRPMSpeedUnit(unit) {
		return 0, DefaultMaxFanRPM
	}
	return FanSpeedMinPercent, FanSpeedMaxPercent
}

func FanSpeedDisplaySuffix(unit string) string {
	if IsRPMSpeedUnit(unit) {
		return "RPM"
	}
	return "%"
}
