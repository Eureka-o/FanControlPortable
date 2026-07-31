package coreapp

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"testing"

	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/device"
	"github.com/TIANLI0/THRM/internal/tray"
	"github.com/TIANLI0/THRM/internal/types"
)

func TestExportDiagnosticsIncludesCachedRuntimeSnapshotsWithoutConnecting(t *testing.T) {
	configManager := config.NewManager(t.TempDir(), nil)
	profile := types.FlyDigiBS1Profile()
	cfg := types.GetDefaultConfig(false)
	cfg.DeviceTransport = types.DeviceTransportBLE
	cfg.DeviceProfiles = []types.DeviceProfile{profile}
	cfg.ActiveDeviceProfileID = profile.ID
	cfg.ActiveDeviceProfileIDsByTransport = map[string]string{types.DeviceTransportBLE: profile.ID}
	configManager.Set(cfg)

	deviceManager := device.NewManager(nil)
	if !deviceManager.ConfigureProfile(profile, "") {
		t.Fatal("ConfigureProfile() failed")
	}
	app := &CoreApp{
		configManager:     configManager,
		deviceManager:     deviceManager,
		trayManager:       tray.NewManager(nil, nil),
		connectionFlights: newConnectionFlightRecorder(defaultConnectionFlightCapacity, nil),
	}
	app.connectionFlights.record(connectionFlightEvent{
		Stage:     connectionFlightStageDisconnected,
		Reason:    "test-disconnect",
		Transport: types.DeviceTransportBLE,
		ProfileID: profile.ID,
	})
	app.setSmartControlDecision(smartControlDecisionSnapshot{
		Active:      true,
		SpeedUnit:   types.FanSpeedUnitRPM,
		FinalTarget: 2200,
		GateReason:  "target-change",
	})

	bundle, err := app.ExportDiagnostics()
	if err != nil {
		t.Fatalf("ExportDiagnostics() error = %v", err)
	}
	if deviceManager.IsConnected() {
		t.Fatal("ExportDiagnostics() connected the device manager")
	}

	files := readDiagnosticsFiles(t, bundle.DataBase64)
	for _, name := range []string{
		"connection-flight.json",
		"smart-control-decision.json",
		"runtime-device-profile.json",
	} {
		if _, ok := files[name]; !ok {
			t.Fatalf("diagnostics bundle is missing %s", name)
		}
	}

	var flight connectionFlightSnapshot
	if err := json.Unmarshal(files["connection-flight.json"], &flight); err != nil {
		t.Fatalf("decode connection flight: %v", err)
	}
	if len(flight.Events) != 1 || flight.Events[0].Reason != "test-disconnect" {
		t.Fatalf("connection flight = %+v", flight)
	}

	var decision smartControlDecisionSnapshot
	if err := json.Unmarshal(files["smart-control-decision.json"], &decision); err != nil {
		t.Fatalf("decode SmartControl decision: %v", err)
	}
	if decision.FinalTarget != 2200 || decision.GateReason != "target-change" {
		t.Fatalf("SmartControl decision = %+v", decision)
	}

	var runtimeProfile diagnosticsDeviceProfileSummary
	if err := json.Unmarshal(files["runtime-device-profile.json"], &runtimeProfile); err != nil {
		t.Fatalf("decode runtime device profile: %v", err)
	}
	if runtimeProfile.ID != profile.ID {
		t.Fatalf("runtime profile ID = %q, want %q", runtimeProfile.ID, profile.ID)
	}
}

func readDiagnosticsFiles(t *testing.T, encoded string) map[string][]byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode diagnostics bundle: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open diagnostics bundle: %v", err)
	}
	files := make(map[string][]byte, len(zr.File))
	for _, file := range zr.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}
		contents, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatalf("read %s: %v", file.Name, err)
		}
		files[file.Name] = contents
	}
	return files
}
