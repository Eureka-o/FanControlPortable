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

func TestResolveDeviceRuntimeTransitionSequence(t *testing.T) {
	capabilities := types.DeviceCapabilities{SupportsSetSpeed: true}
	sequence := []struct {
		input deviceRuntimeStatusInput
		want  string
	}{
		{input: deviceRuntimeStatusInput{Discovering: true}, want: deviceRuntimeStateDiscovering},
		{input: deviceRuntimeStatusInput{Connecting: true}, want: deviceRuntimeStateConnecting},
		{input: deviceRuntimeStatusInput{Connected: true, Capabilities: capabilities}, want: deviceRuntimeStateCapabilities},
		{input: deviceRuntimeStatusInput{Connected: true, SettingsReady: true, Capabilities: capabilities}, want: deviceRuntimeStateReady},
		{input: deviceRuntimeStatusInput{Connected: true, SettingsReady: true, Suspended: true, Capabilities: capabilities}, want: deviceRuntimeStateUnavailable},
		{input: deviceRuntimeStatusInput{Discovering: true}, want: deviceRuntimeStateDiscovering},
		{input: deviceRuntimeStatusInput{}, want: deviceRuntimeStateDisconnected},
	}

	for i, step := range sequence {
		if got := resolveDeviceRuntimeStatus(step.input).State; got != step.want {
			t.Fatalf("transition %d state = %q, want %q", i, got, step.want)
		}
	}
}

func TestDeviceConnectionFlowOwnsRuntimeMirrorTransitions(t *testing.T) {
	app := &CoreApp{}
	flow := newDeviceConnectionFlow(app)
	settings := &types.DeviceSettings{Available: true, Source: types.DeviceTransportSerial}

	flow.setRuntimeConnected()
	if !app.isConnected || app.deviceSettings != nil {
		t.Fatalf("connected transition = connected %v, settings %#v", app.isConnected, app.deviceSettings)
	}

	flow.setRuntimeReady(settings)
	if !app.isConnected || app.deviceSettings != settings {
		t.Fatalf("ready transition = connected %v, settings %#v", app.isConnected, app.deviceSettings)
	}

	if wasConnected := flow.setRuntimeDisconnected(); !wasConnected {
		t.Fatal("disconnect transition did not report the previous connected state")
	}
	if app.isConnected || app.deviceSettings != nil {
		t.Fatalf("disconnected transition = connected %v, settings %#v", app.isConnected, app.deviceSettings)
	}
}

func TestDeviceRuntimeSnapshotRejectsStaleCoreConnection(t *testing.T) {
	app := newDeviceProfileTestApp(t, types.GetDefaultConfig(false))
	app.isConnected = true
	app.deviceSettings = &types.DeviceSettings{Available: true, Source: types.DeviceTransportSerial}

	snapshot := app.deviceRuntimeSnapshot()
	if snapshot.Connected {
		t.Fatal("snapshot reported connected while the hardware manager was disconnected")
	}
	if snapshot.Settings != nil || snapshot.CurrentData != nil {
		t.Fatalf("disconnected snapshot leaked correlated state: settings %#v, data %#v", snapshot.Settings, snapshot.CurrentData)
	}
	if snapshot.Runtime.State != deviceRuntimeStateDisconnected {
		t.Fatalf("runtime state = %q, want disconnected", snapshot.Runtime.State)
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
