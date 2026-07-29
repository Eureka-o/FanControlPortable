package types

import (
	"math"
	"testing"
)

func TestNormalizePowerSpoofConfig(t *testing.T) {
	cfg := AppConfig{
		PowerSpoofPercent: 2500, PowerSpoofOffsetWatts: -20000,
		CPUPowerSpoofPercent: math.NaN(), CPUPowerSpoofOffsetWatts: math.Inf(1),
		GPUPowerSpoofPercent: -1, GPUPowerSpoofOffsetWatts: 20000,
	}
	NormalizePowerSpoofConfig(&cfg)
	if cfg.PowerSpoofPercent != PowerSpoofPercentMax || cfg.PowerSpoofOffsetWatts != PowerSpoofOffsetMin {
		t.Fatalf("clamp = %v%% %+vW", cfg.PowerSpoofPercent, cfg.PowerSpoofOffsetWatts)
	}
	if cfg.CPUPowerSpoofPercent != 100 || cfg.CPUPowerSpoofOffsetWatts != 0 ||
		cfg.GPUPowerSpoofPercent != PowerSpoofPercentMin || cfg.GPUPowerSpoofOffsetWatts != PowerSpoofOffsetMax {
		t.Fatalf("device pairs not normalized: %+v", cfg)
	}
}

func TestSpoofDisplayedPower(t *testing.T) {
	if got := SpoofDisplayedPower(80, 125, -10); got != 90 {
		t.Fatalf("spoofed power = %v, want 90", got)
	}
	if got := SpoofDisplayedPower(0, 200, 10); got != 0 {
		t.Fatalf("missing power changed to %v", got)
	}
}

func TestNormalizeTemperatureSelectionNeverReadsGPU(t *testing.T) {
	selection := NormalizeTemperatureSelection(TemperatureSelection{TempSource: TempSourceGPU, GpuReadMode: GPUReadModeNever})
	if selection.GpuReadMode != GPUReadModeNever || selection.TempSource != TempSourceCPU {
		t.Fatalf("selection = %+v", selection)
	}
}
