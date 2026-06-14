package tray

import (
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
