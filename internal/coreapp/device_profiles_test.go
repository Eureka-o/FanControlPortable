package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/device"
	"github.com/TIANLI0/THRM/internal/deviceprofiles"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

func testSerialDeviceProfile() types.DeviceProfile {
	return types.DeviceProfile{
		ID:          "user.serial.percent",
		DisplayName: "Serial percent",
		Transport:   types.DeviceTransportSerial,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			SerialPort:           "COM3",
			SerialBaudRate:       115200,
			SerialDataBits:       8,
			SerialStopBits:       1,
			SerialParity:         "none",
			SerialFrameDelimiter: "\n",
		},
		Capabilities: types.DeviceCapabilities{
			Transport:        types.DeviceTransportSerial,
			SpeedUnit:        types.FanSpeedUnitPercent,
			SpeedRange:       types.DefaultPercentSpeedRange(),
			SupportsSetSpeed: true,
		},
	}
}

func newDeviceProfileTestApp(t *testing.T, cfg types.AppConfig) *CoreApp {
	t.Helper()
	manager := config.NewManager(t.TempDir(), nil)
	manager.Set(cfg)
	return &CoreApp{
		configManager: manager,
		deviceManager: device.NewManager(nil),
	}
}

func TestSetActiveDeviceProfileKeepsSelectedTransport(t *testing.T) {
	serial := testSerialDeviceProfile()
	cfg := types.GetDefaultConfig(false)
	cfg.DeviceTransport = types.DeviceTransportWiFi
	cfg.WiFiCompatibilityEnabled = true
	cfg.FanControlDeviceIp = "10.0.0.25"
	cfg.ActiveDeviceProfileID = types.DefaultWiFiPercentProfileID
	cfg.DeviceProfiles = []types.DeviceProfile{
		types.DefaultWiFiPercentProfile("10.0.0.25"),
		serial,
	}
	app := newDeviceProfileTestApp(t, cfg)

	selected, err := app.SetActiveDeviceProfile(serial.ID)
	if err != nil {
		t.Fatalf("SetActiveDeviceProfile() returned error: %v", err)
	}
	if selected.ID != serial.ID {
		t.Fatalf("selected profile = %q, want %q", selected.ID, serial.ID)
	}

	got := app.configManager.Get()
	if got.ActiveDeviceProfileID != serial.ID {
		t.Fatalf("active profile = %q, want %q", got.ActiveDeviceProfileID, serial.ID)
	}
	if got.DeviceTransport != types.DeviceTransportSerial {
		t.Fatalf("device transport = %q, want serial", got.DeviceTransport)
	}
	if got.ActiveDeviceProfileIDsByTransport[types.DeviceTransportSerial] != serial.ID {
		t.Fatalf("remembered serial profile = %q, want %q", got.ActiveDeviceProfileIDsByTransport[types.DeviceTransportSerial], serial.ID)
	}
	if got.ActiveDeviceProfileIDsByTransport[types.DeviceTransportWiFi] != types.DefaultWiFiPercentProfileID {
		t.Fatalf("remembered wifi profile = %q, want %q", got.ActiveDeviceProfileIDsByTransport[types.DeviceTransportWiFi], types.DefaultWiFiPercentProfileID)
	}
}

func TestSaveDeviceProfileSetActiveKeepsSelectedTransport(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.DeviceTransport = types.DeviceTransportWiFi
	cfg.WiFiCompatibilityEnabled = true
	cfg.FanControlDeviceIp = "10.0.0.25"
	cfg.ActiveDeviceProfileID = types.DefaultWiFiPercentProfileID
	cfg.DeviceProfiles = []types.DeviceProfile{types.DefaultWiFiPercentProfile("10.0.0.25")}
	app := newDeviceProfileTestApp(t, cfg)

	serial := testSerialDeviceProfile()
	saved, err := app.SaveDeviceProfile(ipc.SaveDeviceProfileParams{
		Profile:   serial,
		SetActive: true,
	})
	if err != nil {
		t.Fatalf("SaveDeviceProfile() returned error: %v", err)
	}
	if saved.ID != serial.ID {
		t.Fatalf("saved profile = %q, want %q", saved.ID, serial.ID)
	}

	got := app.configManager.Get()
	if got.ActiveDeviceProfileID != serial.ID {
		t.Fatalf("active profile = %q, want %q", got.ActiveDeviceProfileID, serial.ID)
	}
	if got.DeviceTransport != types.DeviceTransportSerial {
		t.Fatalf("device transport = %q, want serial", got.DeviceTransport)
	}
}

func TestExportDeviceProfilesExportsOnlyUserDevices(t *testing.T) {
	serial := testSerialDeviceProfile()
	cfg := types.GetDefaultConfig(false)
	cfg.FanControlDeviceIp = "10.0.0.25"
	cfg.ActiveDeviceProfileID = serial.ID
	cfg.DeviceTransport = types.DeviceTransportSerial
	cfg.SerialCompatibilityEnabled = true
	cfg.DeviceProfiles = []types.DeviceProfile{
		types.DefaultWiFiPercentProfile("10.0.0.25"),
		serial,
	}
	app := newDeviceProfileTestApp(t, cfg)

	code, err := app.ExportDeviceProfiles()
	if err != nil {
		t.Fatalf("ExportDeviceProfiles() returned error: %v", err)
	}
	exported, activeID, err := deviceprofiles.Import(code)
	if err != nil {
		t.Fatalf("importing exported device file returned error: %v", err)
	}
	if activeID != serial.ID {
		t.Fatalf("active id = %q, want %q", activeID, serial.ID)
	}
	if len(exported) != 1 {
		t.Fatalf("exported profiles = %d, want 1 user device", len(exported))
	}
	if exported[0].ID != serial.ID {
		t.Fatalf("exported profile id = %q, want %q", exported[0].ID, serial.ID)
	}
	if exported[0].BuiltIn {
		t.Fatal("exported user device should not be marked built-in")
	}
}

func TestImportDeviceProfilesMergesUserDevicesWithoutDeletingExisting(t *testing.T) {
	existing := testSerialDeviceProfile()
	existing.ID = "user.serial.existing"
	existing.DisplayName = "Existing serial"
	incoming := testSerialDeviceProfile()
	incoming.ID = "user.serial.incoming"
	incoming.DisplayName = "Incoming serial"
	incoming.Connection.SerialPort = "COM9"

	cfg := types.GetDefaultConfig(false)
	cfg.FanControlDeviceIp = "10.0.0.25"
	cfg.ActiveDeviceProfileID = existing.ID
	cfg.DeviceTransport = types.DeviceTransportSerial
	cfg.SerialCompatibilityEnabled = true
	cfg.DeviceProfiles = []types.DeviceProfile{
		types.DefaultWiFiPercentProfile("10.0.0.25"),
		existing,
	}
	app := newDeviceProfileTestApp(t, cfg)

	code, err := deviceprofiles.Export(incoming.ID, []types.DeviceProfile{incoming})
	if err != nil {
		t.Fatalf("deviceprofiles.Export() returned error: %v", err)
	}
	if err := app.ImportDeviceProfiles(code); err != nil {
		t.Fatalf("ImportDeviceProfiles() returned error: %v", err)
	}

	got := app.configManager.Get()
	if deviceprofiles.FindIndex(got.DeviceProfiles, existing.ID) < 0 {
		t.Fatalf("existing user device %q was removed during import", existing.ID)
	}
	idx := deviceprofiles.FindIndex(got.DeviceProfiles, incoming.ID)
	if idx < 0 {
		t.Fatalf("incoming user device %q was not imported", incoming.ID)
	}
	if got.DeviceProfiles[idx].Connection.SerialPort != "COM9" {
		t.Fatalf("incoming serial port = %q, want COM9", got.DeviceProfiles[idx].Connection.SerialPort)
	}
	if got.ActiveDeviceProfileID != incoming.ID {
		t.Fatalf("active profile = %q, want imported active %q", got.ActiveDeviceProfileID, incoming.ID)
	}
	if got.ActiveDeviceProfileIDsByTransport[types.DeviceTransportSerial] != incoming.ID {
		t.Fatalf("remembered serial profile = %q, want imported active %q", got.ActiveDeviceProfileIDsByTransport[types.DeviceTransportSerial], incoming.ID)
	}
}

func TestImportDeviceProfilesRejectsTemplateOnlyFiles(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.FanControlDeviceIp = "10.0.0.25"
	cfg.DeviceProfiles = []types.DeviceProfile{types.DefaultWiFiPercentProfile("10.0.0.25")}
	app := newDeviceProfileTestApp(t, cfg)

	code, err := deviceprofiles.Export(types.DefaultWiFiPercentProfileID, []types.DeviceProfile{types.DefaultWiFiPercentProfile("10.0.0.25")})
	if err != nil {
		t.Fatalf("deviceprofiles.Export() returned error: %v", err)
	}
	if err := app.ImportDeviceProfiles(code); err == nil {
		t.Fatal("expected template-only device file to be rejected")
	}
}

func TestDeleteDeviceProfileFallsBackWithinTransport(t *testing.T) {
	first := testSerialDeviceProfile()
	first.ID = "user.serial.first"
	first.DisplayName = "First serial"
	second := testSerialDeviceProfile()
	second.ID = "user.serial.second"
	second.DisplayName = "Second serial"
	second.Connection.SerialPort = "COM9"

	cfg := types.GetDefaultConfig(false)
	cfg.FanControlDeviceIp = "10.0.0.25"
	cfg.ActiveDeviceProfileID = second.ID
	cfg.DeviceTransport = types.DeviceTransportSerial
	cfg.SerialCompatibilityEnabled = true
	cfg.DeviceProfiles = []types.DeviceProfile{
		types.DefaultWiFiPercentProfile("10.0.0.25"),
		first,
		second,
	}
	cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
		types.DeviceTransportWiFi:   types.DefaultWiFiPercentProfileID,
		types.DeviceTransportSerial: second.ID,
	}
	app := newDeviceProfileTestApp(t, cfg)

	if err := app.DeleteDeviceProfile(second.ID); err != nil {
		t.Fatalf("DeleteDeviceProfile() returned error: %v", err)
	}

	got := app.configManager.Get()
	if got.ActiveDeviceProfileID != first.ID {
		t.Fatalf("active profile after delete = %q, want same-transport fallback %q", got.ActiveDeviceProfileID, first.ID)
	}
	if got.DeviceTransport != types.DeviceTransportSerial {
		t.Fatalf("device transport after delete = %q, want serial", got.DeviceTransport)
	}
	if got.ActiveDeviceProfileIDsByTransport[types.DeviceTransportSerial] != first.ID {
		t.Fatalf("remembered serial profile after delete = %q, want %q", got.ActiveDeviceProfileIDsByTransport[types.DeviceTransportSerial], first.ID)
	}
}

func TestDeviceProfileAPIsKeepWiFiProfilesVisibleButInactive(t *testing.T) {
	wifi := types.DefaultWiFiPercentProfile("10.0.0.25")
	wifi.ID = "user.wifi.hidden"
	wifi.DisplayName = "Hidden WiFi"
	wifi.BuiltIn = false
	serial := testSerialDeviceProfile()
	cfg := types.GetDefaultConfig(false)
	cfg.DeviceProfiles = append(cfg.DeviceProfiles, wifi, serial)
	app := newDeviceProfileTestApp(t, cfg)

	payload := app.GetDeviceProfiles()
	foundWiFi := false
	for _, profile := range payload.Profiles {
		if types.NormalizeDeviceTransport(profile.Transport) == types.DeviceTransportWiFi {
			foundWiFi = true
		}
	}
	if !foundWiFi {
		t.Fatal("disabled WiFi profile should remain visible through GetDeviceProfiles")
	}
	if payload.ActiveID != "" {
		t.Fatalf("disabled compatibility active ID = %q, want empty", payload.ActiveID)
	}
	supported := app.GetSupportedDeviceProfiles()
	if len(supported) == 0 || types.NormalizeDeviceTransport(supported[0].Transport) != types.DeviceTransportWiFi {
		t.Fatalf("WiFi device library = %#v, want visible WiFi template", supported)
	}
	foundUserWiFi := false
	for _, profile := range app.GetUserDeviceProfiles() {
		if types.NormalizeDeviceTransport(profile.Transport) == types.DeviceTransportWiFi {
			foundUserWiFi = true
		}
	}
	if !foundUserWiFi {
		t.Fatal("disabled user WiFi profile should remain visible through GetUserDeviceProfiles")
	}

	persisted := app.configManager.Get()
	if deviceprofiles.FindIndex(persisted.DeviceProfiles, wifi.ID) < 0 {
		t.Fatal("hidden custom WiFi profile should remain persisted")
	}
}

func TestSupportedWiFiProfilesAppearWhenCompatibilityEnabled(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.WiFiCompatibilityEnabled = true
	cfg.DeviceTransport = types.DeviceTransportWiFi
	types.NormalizeDeviceProfileConfig(&cfg)
	app := newDeviceProfileTestApp(t, cfg)

	if supported := app.GetSupportedDeviceProfiles(); len(supported) == 0 || supported[0].Transport != types.DeviceTransportWiFi {
		t.Fatalf("enabled WiFi templates = %#v, want WiFi template", supported)
	}
}
