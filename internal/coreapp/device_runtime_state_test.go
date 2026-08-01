package coreapp

import (
	"testing"
	"time"

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
	app := &CoreApp{connectionFlights: newConnectionFlightRecorder(8, nil)}
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
	app.lastSuccessfulDeviceReadAt = time.Now()

	if wasConnected := flow.setRuntimeDisconnected("test"); !wasConnected {
		t.Fatal("disconnect transition did not report the previous connected state")
	}
	if app.isConnected || app.deviceSettings != nil {
		t.Fatalf("disconnected transition = connected %v, settings %#v", app.isConnected, app.deviceSettings)
	}
	if !app.lastSuccessfulDeviceReadAt.IsZero() {
		t.Fatalf("disconnect retained stale successful-read time: %v", app.lastSuccessfulDeviceReadAt)
	}

	flight := app.connectionFlights.snapshot(connectionFlightSnapshotInput{})
	wantStages := []string{
		connectionFlightStageConnected,
		connectionFlightStageReady,
		connectionFlightStageDisconnected,
	}
	if len(flight.Events) != len(wantStages) {
		t.Fatalf("flight events = %#v, want stages %#v", flight.Events, wantStages)
	}
	for i, stage := range wantStages {
		if flight.Events[i].Stage != stage {
			t.Fatalf("flight event %d stage = %q, want %q", i, flight.Events[i].Stage, stage)
		}
	}
}

func TestDeviceConnectionFlowPhaseScopeRestoresNestedPhase(t *testing.T) {
	app := &CoreApp{}
	flow := newDeviceConnectionFlow(app)

	leaveDiscovery := flow.enterPhase(deviceConnectionPhaseDiscovering)
	if got := app.connectionPhase.Load(); got != deviceConnectionPhaseDiscovering {
		t.Fatalf("discovery phase = %d, want %d", got, deviceConnectionPhaseDiscovering)
	}
	leaveConnecting := flow.enterPhase(deviceConnectionPhaseConnecting)
	if got := app.connectionPhase.Load(); got != deviceConnectionPhaseConnecting {
		t.Fatalf("connecting phase = %d, want %d", got, deviceConnectionPhaseConnecting)
	}
	leaveConnecting()
	if got := app.connectionPhase.Load(); got != deviceConnectionPhaseDiscovering {
		t.Fatalf("nested phase restore = %d, want %d", got, deviceConnectionPhaseDiscovering)
	}
	leaveDiscovery()
	if got := app.connectionPhase.Load(); got != deviceConnectionPhaseNone {
		t.Fatalf("phase restore = %d, want %d", got, deviceConnectionPhaseNone)
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

func TestDiagnosticsRuntimeSnapshotUsesAuthoritativeDeviceSnapshot(t *testing.T) {
	app := newDeviceProfileTestApp(t, types.GetDefaultConfig(false))
	app.mutex.Lock()
	app.isConnected = true
	app.deviceSettings = &types.DeviceSettings{Available: true}
	app.mutex.Unlock()

	runtime := app.diagnosticsRuntimeSnapshot()
	if runtime.IsConnected {
		t.Fatal("diagnostics reported connected while the hardware manager was disconnected")
	}
	if runtime.DeviceSettings != nil {
		t.Fatalf("diagnostics leaked stale device settings: %#v", runtime.DeviceSettings)
	}
}

func TestGetDeviceStatusIncludesConnectionFlightSnapshotWhenDisconnected(t *testing.T) {
	app := &CoreApp{connectionFlights: newConnectionFlightRecorder(4, nil)}
	app.connectionFlights.record(connectionFlightEvent{
		Stage:  connectionFlightStageDisconnected,
		Reason: "device-disconnect",
	})

	status := app.GetDeviceStatus()
	flight, ok := status["connectionFlight"].(connectionFlightSnapshot)
	if !ok {
		t.Fatalf("connectionFlight = %#v, want connectionFlightSnapshot", status["connectionFlight"])
	}
	if flight.State != deviceRuntimeStateDisconnected || len(flight.Events) != 1 {
		t.Fatalf("unexpected connection flight snapshot: %#v", flight)
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
