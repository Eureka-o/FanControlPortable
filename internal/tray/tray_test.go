package tray

import (
	"math"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/types"
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

func TestStatusDoesNotCarrySmartControlDetails(t *testing.T) {
	if _, ok := reflect.TypeOf(Status{}).FieldByName("SmartControlDecision"); ok {
		t.Fatal("tray status should not carry SmartControl detail data")
	}
}

func TestSystrayInstanceBudget(t *testing.T) {
	m := NewManager(nil, nil)
	atomic.StoreInt32(&m.instanceCount, maxSystrayInstances-1)
	if m.systrayBudgetExhausted() {
		t.Fatal("budget should allow the final instance")
	}
	atomic.StoreInt32(&m.instanceCount, maxSystrayInstances)
	if !m.systrayBudgetExhausted() {
		t.Fatal("budget should stop an unbounded rebuild loop")
	}
}

func TestTrayReannounceRetryLimit(t *testing.T) {
	if trayShellRestartReannounceAttempts != 3 {
		t.Fatalf("tray reannounce attempts = %d, want 3", trayShellRestartReannounceAttempts)
	}
	if !shouldRetryTrayReannounce(0) || !shouldRetryTrayReannounce(1) {
		t.Fatal("expected the first two reannounce attempts to retry")
	}
	if shouldRetryTrayReannounce(2) || shouldRetryTrayReannounce(99) {
		t.Fatal("reannounce retries must stop at the bounded attempt limit")
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

func TestMonitorOnlyTrayTooltipKeepsTelemetryWithoutFanControls(t *testing.T) {
	status := Status{
		MonitorOnly:   true,
		CPUTemp:       62,
		GPUTemp:       55,
		CPUPowerWatts: 18,
		GPUPowerWatts: 42,
	}

	got := formatTrayTooltip(status, "1300 RPM")
	if !strings.Contains(got, "CPU 62°C") || !strings.Contains(got, "GPU 55°C") {
		t.Fatalf("tooltip = %q, want CPU/GPU telemetry", got)
	}
	if strings.Contains(got, "设备未连接") || strings.Contains(got, "风扇 1300 RPM") {
		t.Fatalf("tooltip = %q, want monitor-only telemetry without device controls", got)
	}
}

func TestDeviceStatusTitleUsesRuntimeDeviceName(t *testing.T) {
	if got := deviceStatusTitle("FlyDigi BS3 Pro", true, false); got != "FlyDigi BS3 Pro：已连接" {
		t.Fatalf("deviceStatusTitle() = %q", got)
	}
	if got := deviceStatusTitle("", false, false); got != "设备：未连接" {
		t.Fatalf("deviceStatusTitle(empty) = %q", got)
	}
	if got := deviceStatusTitle("", false, true); got != "仅监控模式：运行中" {
		t.Fatalf("deviceStatusTitle(monitor-only) = %q", got)
	}
}

func TestTrayCurveSignatureTracksOrderAndLabels(t *testing.T) {
	first := trayCurveSignature([]CurveOption{{ID: "quiet", Name: "Quiet"}, {ID: "turbo", Name: "Turbo"}})
	if first == trayCurveSignature([]CurveOption{{ID: "turbo", Name: "Turbo"}, {ID: "quiet", Name: "Quiet"}}) {
		t.Fatal("curve signature must change when menu order changes")
	}
	if first == trayCurveSignature([]CurveOption{{ID: "quiet", Name: "Silent"}, {ID: "turbo", Name: "Turbo"}}) {
		t.Fatal("curve signature must change when a menu label changes")
	}
	if first != trayCurveSignature([]CurveOption{{ID: "quiet", Name: "Quiet"}, {ID: "turbo", Name: "Turbo"}}) {
		t.Fatal("curve signature should be stable for identical options")
	}
}

func TestTrayStatusSnapshotKeyDetectsDisplayChanges(t *testing.T) {
	base := Status{
		Connected:            true,
		DeviceName:           " FlyDigi BS3 Pro ",
		CPUTemp:              60,
		GPUTemp:              52,
		CPUPowerWatts:        9.12,
		GPUPowerWatts:        25.3,
		GPUReadState:         types.GPUReadStateActive,
		CurrentRPM:           1400,
		SpeedUnit:            "RPM",
		AutoControlState:     true,
		ActiveCurveProfileID: "quiet",
		CurveProfiles:        []CurveOption{{ID: "quiet", Name: "Quiet"}},
	}
	sig := trayCurveSignature(base.CurveProfiles)
	baseKey := makeTrayStatusSnapshotKey(base, sig)

	// Sub-display-precision power jitter (<0.05W below 10W, <0.5W above 10W) and
	// whitespace/case differences must keep the same key.
	sameDisplay := base
	sameDisplay.DeviceName = "FlyDigi BS3 Pro"
	sameDisplay.CPUPowerWatts = 9.14
	sameDisplay.GPUPowerWatts = 25.1
	sameDisplay.SpeedUnit = types.FanSpeedUnitRPM
	if got := makeTrayStatusSnapshotKey(sameDisplay, sig); got != baseKey {
		t.Fatalf("equivalent display state produced different key: %+v vs %+v", got, baseKey)
	}

	// Display-visible change in rounded wattage or temperature must invalidate the key.
	powerChanged := base
	powerChanged.CPUPowerWatts = 9.16
	if got := makeTrayStatusSnapshotKey(powerChanged, sig); got == baseKey {
		t.Fatal("rounded wattage change did not change snapshot key")
	}

	// Non-polled GPU ignores stale GPU numbers.
	notPolledA := base
	notPolledA.GPUReadState = types.GPUReadStateNotPolled
	notPolledA.GPUTemp = 48
	notPolledA.GPUPowerWatts = 12
	notPolledB := notPolledA
	notPolledB.GPUTemp = 55
	notPolledB.GPUPowerWatts = 20
	if makeTrayStatusSnapshotKey(notPolledA, sig) != makeTrayStatusSnapshotKey(notPolledB, sig) {
		t.Fatal("notPolled GPU state should ignore stale GPU temperature/power differences")
	}
}

func TestFormatPowerWattsHidesSubThresholdNoise(t *testing.T) {
	for _, watts := range []float64{0, -1, 0.02, 0.049, math.NaN()} {
		if got := formatPowerWatts(watts); got != "-- W" {
			t.Fatalf("formatPowerWatts(%v) = %q, want \"-- W\"", watts, got)
		}
	}
	if got := formatPowerWatts(0.06); got != "0.1 W" {
		t.Fatalf("formatPowerWatts(0.06) = %q, want \"0.1 W\"", got)
	}
}
