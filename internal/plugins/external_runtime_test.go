package plugins

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const externalHelperEnv = "FANCONTROL_PLUGIN_HELPER"

func TestExternalRuntimeHelperProcess(t *testing.T) {
	if os.Getenv(externalHelperEnv) != "1" {
		return
	}
	scenario := "normal"
	for index, arg := range os.Args {
		if arg == "--" && index+1 < len(os.Args) {
			scenario = os.Args[index+1]
			break
		}
	}
	if scenario == "timeout" {
		time.Sleep(10 * time.Second)
		return
	}

	encoder := json.NewEncoder(os.Stdout)
	version := "1.0.0"
	if scenario == "mismatch" {
		version = "9.9.9"
	}
	supported := scenario != "unsupported"
	hello := map[string]any{
		"type":            "hello",
		"protocolVersion": ExternalProtocolVersion,
		"pluginId":        "test-plugin",
		"version":         version,
		"capabilities":    []string{"status"},
		"supported":       supported,
	}
	if !supported {
		hello["reason"] = "unsupported test hardware"
	}
	if err := encoder.Encode(hello); err != nil {
		os.Exit(2)
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var message struct {
			Type     string          `json:"type"`
			ID       string          `json:"id"`
			Method   string          `json:"method"`
			Sequence uint64          `json:"sequence"`
			DataDir  string          `json:"dataDir"`
			Payload  json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			os.Exit(3)
		}
		switch message.Type {
		case "host-init":
			_ = encoder.Encode(map[string]any{
				"type":    "event",
				"event":   "initialized",
				"payload": map[string]any{"dataDir": message.DataDir},
			})
		case "telemetry":
			_ = encoder.Encode(map[string]any{
				"type":    "event",
				"event":   "telemetry-received",
				"payload": map[string]any{"sequence": message.Sequence},
			})
		case "request":
			response := map[string]any{
				"type":    "response",
				"id":      message.ID,
				"ok":      true,
				"payload": json.RawMessage(message.Payload),
			}
			if err := encoder.Encode(response); err != nil {
				os.Exit(4)
			}
			if message.Method == "host.stop" || message.Method == "host.prepare-suspend" {
				return
			}
		}
	}
}

func testRuntimeSpec(t *testing.T) RuntimeSpec {
	t.Helper()
	return RuntimeSpec{
		ID:              "test-plugin",
		Name:            "Test Plugin",
		Version:         "1.0.0",
		PluginDir:       t.TempDir(),
		BackendPath:     filepath.Join(t.TempDir(), "helper.exe"),
		Capabilities:    []string{"status"},
		TelemetryInputs: []string{"cpu-temp"},
	}
}

func helperCommandFactory(t *testing.T, scenario string) CommandFactory {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return func(ctx context.Context, spec RuntimeSpec) *exec.Cmd {
		cmd := exec.CommandContext(ctx, executable, "-test.run=TestExternalRuntimeHelperProcess", "--", scenario)
		cmd.Env = append(os.Environ(), externalHelperEnv+"=1")
		return cmd
	}
}

func TestProcessRuntimeHandshakeInvokeTelemetryAndStop(t *testing.T) {
	events := make(chan RuntimeEvent, 8)
	statuses := make(chan RuntimeStatus, 8)
	runtime := NewProcessRuntime(testRuntimeSpec(t), ProcessRuntimeOptions{
		DataDir:          filepath.Join(t.TempDir(), "data"),
		CommandFactory:   helperCommandFactory(t, "normal"),
		HandshakeTimeout: time.Second,
		RequestTimeout:   time.Second,
		StopTimeout:      time.Second,
		OnEvent:          func(event RuntimeEvent) { events <- event },
		OnStatus:         func(status RuntimeStatus) { statuses <- status },
	})

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if status := runtime.Status(); status.State != CatalogStateReady {
		t.Fatalf("status = %#v", status)
	}
	waitForRuntimeEvent(t, events, "initialized")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	response, err := runtime.Invoke(ctx, "echo", map[string]any{"value": 42})
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]int
	if err := json.Unmarshal(response, &decoded); err != nil || decoded["value"] != 42 {
		t.Fatalf("response = %s, err=%v", response, err)
	}

	runtime.SubmitTelemetry(TelemetrySnapshot{
		Sequence:  7,
		SampledAt: time.Now().UnixMilli(),
		Payload: TelemetryPayload{
			CPUTemp:  &TelemetryValue{Value: 61, Valid: true},
			GPUTemp:  &TelemetryValue{Value: 55, Valid: true},
			BridgeOK: true,
		},
	})
	event := waitForRuntimeEvent(t, events, "telemetry-received")
	var telemetryPayload map[string]uint64
	if err := json.Unmarshal(event.Payload, &telemetryPayload); err != nil || telemetryPayload["sequence"] != 7 {
		t.Fatalf("telemetry event = %s, err=%v", event.Payload, err)
	}

	if err := runtime.Stop("disabled"); err != nil {
		t.Fatal(err)
	}
	if status := runtime.Status(); status.State != CatalogStateDisabled {
		t.Fatalf("stopped status = %#v", status)
	}
	if len(statuses) < 2 {
		t.Fatalf("status updates = %d", len(statuses))
	}
}

func TestProcessRuntimeRejectsMismatchedHello(t *testing.T) {
	runtime := NewProcessRuntime(testRuntimeSpec(t), ProcessRuntimeOptions{
		DataDir:          t.TempDir(),
		CommandFactory:   helperCommandFactory(t, "mismatch"),
		HandshakeTimeout: time.Second,
	})
	if err := runtime.Start(context.Background()); err == nil {
		t.Fatal("Start succeeded with mismatched hello")
	}
	if status := runtime.Status(); status.State != CatalogStateFailed {
		t.Fatalf("status = %#v", status)
	}
}

func TestProcessRuntimeReportsUnsupportedHardware(t *testing.T) {
	runtime := NewProcessRuntime(testRuntimeSpec(t), ProcessRuntimeOptions{
		DataDir:          t.TempDir(),
		CommandFactory:   helperCommandFactory(t, "unsupported"),
		HandshakeTimeout: time.Second,
		StopTimeout:      time.Second,
	})
	if err := runtime.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if status := runtime.Status(); status.State != CatalogStateUnsupported || status.LastError == "" {
		t.Fatalf("status = %#v", status)
	}
	if err := runtime.Stop("disabled"); err != nil {
		t.Fatal(err)
	}
}

func TestProcessRuntimeHandshakeTimeout(t *testing.T) {
	runtime := NewProcessRuntime(testRuntimeSpec(t), ProcessRuntimeOptions{
		DataDir:          t.TempDir(),
		CommandFactory:   helperCommandFactory(t, "timeout"),
		HandshakeTimeout: 100 * time.Millisecond,
	})
	started := time.Now()
	if err := runtime.Start(context.Background()); err == nil {
		t.Fatal("Start succeeded without hello")
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("handshake timeout took %s", elapsed)
	}
}

func TestSupervisorStartsAndStopsEnabledRuntime(t *testing.T) {
	supervisor := NewSupervisor(SupervisorOptions{
		DataRoot:         t.TempDir(),
		CommandFactory:   helperCommandFactory(t, "normal"),
		HandshakeTimeout: time.Second,
		StopTimeout:      time.Second,
	})
	supervisor.Sync([]RuntimeSpec{testRuntimeSpec(t)})
	supervisor.Start(context.Background())
	if err := supervisor.StartEnabled(map[string]bool{"test-plugin": true}); err != nil {
		t.Fatal(err)
	}
	if status, ok := supervisor.Status("test-plugin"); !ok || status.State != CatalogStateReady {
		t.Fatalf("status = %#v, ok=%v", status, ok)
	}
	if err := supervisor.StopAll("shutdown"); err != nil {
		t.Fatal(err)
	}
	if status, ok := supervisor.Status("test-plugin"); !ok || status.State != CatalogStateDisabled {
		t.Fatalf("stopped status = %#v, ok=%v", status, ok)
	}
}

func TestSupervisorRestartsSuspendedEnabledRuntime(t *testing.T) {
	supervisor := NewSupervisor(SupervisorOptions{
		DataRoot:         t.TempDir(),
		CommandFactory:   helperCommandFactory(t, "normal"),
		HandshakeTimeout: time.Second,
		StopTimeout:      time.Second,
	})
	supervisor.Sync([]RuntimeSpec{testRuntimeSpec(t)})
	supervisor.Start(context.Background())
	enabled := map[string]bool{"test-plugin": true}
	if err := supervisor.StartEnabled(enabled); err != nil {
		t.Fatal(err)
	}
	if err := supervisor.SuspendAll(); err != nil {
		t.Fatal(err)
	}
	if status, _ := supervisor.Status("test-plugin"); status.State != CatalogStateSuspended {
		t.Fatalf("suspended status = %#v", status)
	}
	if err := supervisor.ResumeEnabled(enabled); err != nil {
		t.Fatal(err)
	}
	if status, _ := supervisor.Status("test-plugin"); status.State != CatalogStateReady {
		t.Fatalf("resumed status = %#v", status)
	}
	if err := supervisor.StopAll("shutdown"); err != nil {
		t.Fatal(err)
	}
}

func TestOfferLatestTelemetryReplacesPendingValue(t *testing.T) {
	channel := make(chan TelemetrySnapshot, 1)
	offerLatestTelemetry(channel, TelemetrySnapshot{Sequence: 1})
	offerLatestTelemetry(channel, TelemetrySnapshot{Sequence: 2})
	if snapshot := <-channel; snapshot.Sequence != 2 {
		t.Fatalf("sequence = %d", snapshot.Sequence)
	}
}

func TestTelemetryFilterOnlyIncludesDeclaredInputs(t *testing.T) {
	snapshot, ok := (TelemetrySnapshot{Payload: TelemetryPayload{
		CPUTemp:       &TelemetryValue{Value: 60, Valid: true},
		GPUPowerWatts: &TelemetryValue{Value: 80, Valid: true},
	}}).Filter([]string{"cpu-temp"})
	if !ok || snapshot.Payload.CPUTemp == nil || snapshot.Payload.GPUPowerWatts != nil {
		t.Fatalf("filtered snapshot = %#v, ok=%v", snapshot, ok)
	}
}

func waitForRuntimeEvent(t *testing.T, events <-chan RuntimeEvent, name string) RuntimeEvent {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case event := <-events:
			if event.Event == name {
				return event
			}
		case <-timer.C:
			t.Fatal(fmt.Sprintf("timed out waiting for event %s", name))
		}
	}
}
