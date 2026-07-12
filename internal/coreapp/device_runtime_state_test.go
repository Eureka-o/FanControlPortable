package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestResolveDeviceRuntimeStatus(t *testing.T) {
	readyCapabilities := types.DeviceCapabilities{SupportsSetSpeed: true}
	cases := []struct {
		name       string
		input      deviceRuntimeStatusInput
		want       string
		canControl bool
	}{
		{name: "disconnected", want: deviceRuntimeStateDisconnected},
		{name: "discovering", input: deviceRuntimeStatusInput{Discovering: true}, want: deviceRuntimeStateDiscovering},
		{name: "connecting", input: deviceRuntimeStatusInput{Connecting: true}, want: deviceRuntimeStateConnecting},
		{name: "suspended", input: deviceRuntimeStatusInput{Connected: true, Suspended: true, Capabilities: readyCapabilities}, want: deviceRuntimeStateUnavailable},
		{name: "awaiting capabilities", input: deviceRuntimeStatusInput{Connected: true, Capabilities: readyCapabilities}, want: deviceRuntimeStateCapabilities},
		{name: "ready", input: deviceRuntimeStatusInput{Connected: true, SettingsReady: true, Capabilities: readyCapabilities}, want: deviceRuntimeStateReady, canControl: true},
		{name: "unsupported", input: deviceRuntimeStatusInput{Connected: true, SettingsReady: true}, want: deviceRuntimeStateUnavailable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveDeviceRuntimeStatus(tc.input)
			if got.State != tc.want {
				t.Fatalf("state = %q, want %q", got.State, tc.want)
			}
			if got.CanControl != tc.canControl {
				t.Fatalf("canControl = %v, want %v", got.CanControl, tc.canControl)
			}
		})
	}
}

func TestAutomaticControlInputReady(t *testing.T) {
	cases := []struct {
		name string
		temp types.TemperatureData
		want bool
	}{
		{name: "fresh bridge sample", temp: types.TemperatureData{BridgeOk: true, TelemetryFresh: true, ControlTemp: 65}, want: true},
		{name: "stale bridge sample", temp: types.TemperatureData{BridgeOk: true, ControlTemp: 65}, want: false},
		{name: "bridge failure", temp: types.TemperatureData{TelemetryFresh: true, ControlTemp: 65}, want: false},
		{name: "invalid temperature", temp: types.TemperatureData{BridgeOk: true, TelemetryFresh: true}, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := automaticControlInputReady(tc.temp); got != tc.want {
				t.Fatalf("automaticControlInputReady() = %v, want %v", got, tc.want)
			}
		})
	}
}
