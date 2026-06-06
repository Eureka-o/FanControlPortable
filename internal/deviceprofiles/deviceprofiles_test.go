package deviceprofiles

import (
	"strings"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func testWiFiProfile() types.DeviceProfile {
	profile := types.DefaultWiFiPercentProfile("10.0.0.42")
	profile.ID = "user.wifi.percent"
	profile.DisplayName = "DIY WiFi percent"
	profile.BuiltIn = false
	profile.Connection.RequestTimeoutMs = 2500
	profile.Connection.MinSendIntervalMs = 150
	profile.Connection.MaxRetries = 2
	profile.Connection.RetryBackoffMs = 200
	profile.Commands = []types.DeviceCommandTemplate{{
		Name:     "set-speed",
		Command:  `{"speed": "{{percent}}"}`,
		Encoding: "json",
		Checksum: "none",
	}}
	profile.ResponseParsers = []types.DeviceResponseParser{{
		Name:       "current-speed",
		Type:       "jsonpath",
		Expression: "$.speed",
	}}
	profile.SpeedMap = []types.DeviceSpeedMapPoint{
		{PercentTicks: 0, RPM: 1200},
		{PercentTicks: 500, RPM: 2600},
		{PercentTicks: 1000, RPM: 4200},
	}
	return profile
}

func testSerialProfile() types.DeviceProfile {
	return types.DeviceProfile{
		ID:          "user.serial.percent",
		DisplayName: "DIY Serial percent",
		Transport:   types.DeviceTransportSerial,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			SerialPort:     "COM9",
			SerialBaudRate: 115200,
			SerialDataBits: 8,
			SerialStopBits: 1,
			SerialParity:   "none",
		},
		Capabilities: types.DeviceCapabilities{
			SupportsSetSpeed: true,
		},
	}
}

func TestExportImportRoundTripIncludesCompleteWiFiProfile(t *testing.T) {
	code, err := Export("user.wifi.percent", []types.DeviceProfile{testWiFiProfile()})
	if err != nil {
		t.Fatalf("Export returned error: %v", err)
	}
	if !strings.HasPrefix(code, exportPrefix) {
		t.Fatalf("export prefix = %q, want %q", code[:min(len(code), len(exportPrefix))], exportPrefix)
	}

	profiles, activeID, err := Import(code)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	if activeID != "user.wifi.percent" {
		t.Fatalf("active id = %q, want user.wifi.percent", activeID)
	}
	if len(profiles) != 1 {
		t.Fatalf("profiles length = %d, want 1", len(profiles))
	}

	profile := profiles[0]
	if profile.Connection.Endpoint != "10.0.0.42" {
		t.Fatalf("endpoint = %q, want 10.0.0.42", profile.Connection.Endpoint)
	}
	if profile.Connection.SpeedEndpoint != "/api/speed" || profile.Connection.StateEndpoint != "/api/data" {
		t.Fatalf("endpoints = %q/%q, want /api/speed and /api/data", profile.Connection.SpeedEndpoint, profile.Connection.StateEndpoint)
	}
	if profile.Connection.RequestTimeoutMs != 2500 || profile.Connection.MinSendIntervalMs != 150 || profile.Connection.MaxRetries != 2 || profile.Connection.RetryBackoffMs != 200 {
		t.Fatalf("runtime policy = timeout %d minSend %d retries %d backoff %d, want 2500/150/2/200",
			profile.Connection.RequestTimeoutMs,
			profile.Connection.MinSendIntervalMs,
			profile.Connection.MaxRetries,
			profile.Connection.RetryBackoffMs)
	}
	if got := len(profile.Commands); got != 1 {
		t.Fatalf("commands length = %d, want 1", got)
	}
	if profile.Commands[0].Checksum != "none" {
		t.Fatalf("command checksum = %q, want none", profile.Commands[0].Checksum)
	}
	if got := len(profile.ResponseParsers); got != 1 {
		t.Fatalf("parsers length = %d, want 1", got)
	}
	if got := len(profile.SpeedMap); got != 3 {
		t.Fatalf("speed map length = %d, want 3", got)
	}
	if profile.BuiltIn {
		t.Fatal("imported user profile should not become built-in")
	}
}

func TestExportImportPreservesActiveIDsByTransport(t *testing.T) {
	wifi := testWiFiProfile()
	serial := testSerialProfile()
	code, err := ExportWithActiveIDs(serial.ID, map[string]string{
		types.DeviceTransportWiFi:   wifi.ID,
		types.DeviceTransportSerial: serial.ID,
		types.DeviceTransportBLE:    "missing",
	}, []types.DeviceProfile{wifi, serial})
	if err != nil {
		t.Fatalf("ExportWithActiveIDs returned error: %v", err)
	}

	profiles, activeID, activeIDs, err := ImportWithActiveIDs(code)
	if err != nil {
		t.Fatalf("ImportWithActiveIDs returned error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("profiles length = %d, want 2", len(profiles))
	}
	if activeID != serial.ID {
		t.Fatalf("active id = %q, want %q", activeID, serial.ID)
	}
	if activeIDs[types.DeviceTransportWiFi] != wifi.ID {
		t.Fatalf("wifi active id = %q, want %q", activeIDs[types.DeviceTransportWiFi], wifi.ID)
	}
	if activeIDs[types.DeviceTransportSerial] != serial.ID {
		t.Fatalf("serial active id = %q, want %q", activeIDs[types.DeviceTransportSerial], serial.ID)
	}
	if _, ok := activeIDs[types.DeviceTransportBLE]; ok {
		t.Fatal("missing BLE active id should not be preserved")
	}
}

func TestValidateCommandChecksumModes(t *testing.T) {
	for _, checksum := range []string{"none", "sum8", "xor8", "crc16"} {
		profile := testWiFiProfile()
		profile.Commands[0].Encoding = "hex"
		profile.Commands[0].Command = "A5 10"
		profile.Commands[0].Checksum = checksum
		if _, err := NormalizeAndValidate(profile, ""); err != nil {
			t.Fatalf("NormalizeAndValidate() with checksum %q returned error: %v", checksum, err)
		}
	}

	profile := testWiFiProfile()
	profile.Commands[0].Checksum = "md5"
	if _, err := NormalizeAndValidate(profile, ""); err == nil {
		t.Fatal("expected unsupported checksum mode to be rejected")
	}

	profile = testWiFiProfile()
	profile.Commands[0].Encoding = "json"
	profile.Commands[0].Checksum = "sum8"
	if _, err := NormalizeAndValidate(profile, ""); err == nil {
		t.Fatal("expected json checksum to be rejected")
	}
}

func TestImportRejectsWiFiProfileWithoutEndpoint(t *testing.T) {
	profile := testWiFiProfile()
	profile.Connection.Endpoint = ""
	code, err := encodePayload(exportPayload{
		Schema:   exportSchema,
		Version:  exportVersion,
		ActiveID: profile.ID,
		Profiles: []types.DeviceProfile{
			profile,
		},
	})
	if err != nil {
		t.Fatalf("encodePayload returned error: %v", err)
	}

	if _, _, err := Import(code); err == nil {
		t.Fatal("expected missing WiFi endpoint to be rejected")
	}
}

func TestValidateRejectsInvalidWiFiRuntimePolicy(t *testing.T) {
	profile := testWiFiProfile()
	profile.Connection.RequestTimeoutMs = 30001
	if _, err := NormalizeAndValidate(profile, ""); err == nil {
		t.Fatal("expected oversized request timeout to be rejected")
	}

	profile = testWiFiProfile()
	profile.Connection.MinSendIntervalMs = -1
	if _, err := NormalizeAndValidate(profile, ""); err == nil {
		t.Fatal("expected negative minimum send interval to be rejected")
	}

	profile = testWiFiProfile()
	profile.Connection.MaxRetries = 6
	if _, err := NormalizeAndValidate(profile, ""); err == nil {
		t.Fatal("expected oversized max retries to be rejected")
	}

	profile = testWiFiProfile()
	profile.Connection.RetryBackoffMs = 10001
	if _, err := NormalizeAndValidate(profile, ""); err == nil {
		t.Fatal("expected oversized retry backoff to be rejected")
	}
}

func TestValidateBLEAndSerialProfiles(t *testing.T) {
	ble := types.DeviceProfile{
		ID:          "user.ble.percent",
		DisplayName: "DIY BLE percent",
		Transport:   types.DeviceTransportBLE,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			BLEServiceUUID:         "0000180f-0000-1000-8000-00805f9b34fb",
			BLEWriteCharacteristic: "00002a19-0000-1000-8000-00805f9b34fb",
		},
		Capabilities: types.DeviceCapabilities{
			SupportsSetSpeed: true,
		},
	}
	if _, err := NormalizeAndValidate(ble, ""); err != nil {
		t.Fatalf("valid BLE profile returned error: %v", err)
	}

	ble.Connection.BLEServiceUUID = "not-a-uuid"
	if _, err := NormalizeAndValidate(ble, ""); err == nil {
		t.Fatal("expected invalid BLE UUID to be rejected")
	}

	serial := types.DeviceProfile{
		ID:          "user.serial.rpm",
		DisplayName: "DIY serial RPM",
		Transport:   types.DeviceTransportSerial,
		SpeedUnit:   types.FanSpeedUnitRPM,
		SpeedRange: types.DeviceSpeedRange{
			Min:  800,
			Max:  5000,
			Step: 1,
		},
		Connection: types.DeviceConnectionSettings{
			SerialPort:     "COM9",
			SerialBaudRate: 115200,
			SerialDataBits: 8,
			SerialStopBits: 1,
			SerialParity:   "none",
		},
		Capabilities: types.DeviceCapabilities{
			SupportsSetSpeed: true,
		},
	}
	if _, err := NormalizeAndValidate(serial, ""); err != nil {
		t.Fatalf("valid serial profile returned error: %v", err)
	}
}
