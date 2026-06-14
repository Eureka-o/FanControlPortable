package tray

import (
	"strings"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestFormatFanSpeedForTrayUsesSpeedUnit(t *testing.T) {
	tests := []struct {
		name  string
		speed uint16
		unit  string
		want  string
	}{
		{name: "percent", speed: 45, unit: types.FanSpeedUnitPercent, want: "45%"},
		{name: "rpm", speed: 1300, unit: types.FanSpeedUnitRPM, want: "1300 RPM"},
		{name: "unknown unit falls back to percent", speed: 55, unit: "", want: "55%"},
		{name: "zero means no data", speed: 0, unit: types.FanSpeedUnitRPM, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatFanSpeedForTray(tt.speed, tt.unit); got != tt.want {
				t.Fatalf("formatFanSpeedForTray() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatTrayTooltipKeepsFanSpeedEarlyAndShort(t *testing.T) {
	status := Status{
		Connected:        true,
		CPUTemp:          62,
		GPUTemp:          55,
		CPUPowerWatts:    18,
		GPUPowerWatts:    42,
		AutoControlState: true,
	}

	got := formatTrayTooltip(status, "1300 RPM")
	if !strings.Contains(got, "风扇 1300 RPM") {
		t.Fatalf("tooltip = %q, want fan speed with full RPM unit", got)
	}
	if strings.Index(got, "风扇") > strings.Index(got, "CPU") {
		t.Fatalf("tooltip = %q, want fan speed before CPU/GPU lines", got)
	}
	if runeCount := len([]rune(got)); runeCount > 64 {
		t.Fatalf("tooltip rune count = %d, want <= 64; tooltip = %q", runeCount, got)
	}
}

func TestDeviceStatusTitleUsesRuntimeDeviceName(t *testing.T) {
	if got := deviceStatusTitle("FlyDigi BS3 Pro", true); got != "FlyDigi BS3 Pro：已连接" {
		t.Fatalf("deviceStatusTitle() = %q", got)
	}
	if got := deviceStatusTitle("", false); got != "设备：未连接" {
		t.Fatalf("deviceStatusTitle(empty) = %q", got)
	}
}
