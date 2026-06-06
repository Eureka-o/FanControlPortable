package types

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/appmeta"
)

func TestDefaultWiFiPercentProfileNormalizesCapabilities(t *testing.T) {
	profile := NormalizeDeviceProfile(DefaultWiFiPercentProfile("192.168.1.50"), "")

	if profile.ID != DefaultWiFiPercentProfileID {
		t.Fatalf("profile ID = %q, want %q", profile.ID, DefaultWiFiPercentProfileID)
	}
	if profile.DisplayName != appmeta.DeviceModelName || profile.Model != appmeta.DeviceModelName {
		t.Fatalf("profile name/model = %q/%q, want %q", profile.DisplayName, profile.Model, appmeta.DeviceModelName)
	}
	if profile.Transport != DeviceTransportWiFi || profile.SpeedUnit != FanSpeedUnitPercent {
		t.Fatalf("profile transport/unit = %q/%q, want wifi/percent", profile.Transport, profile.SpeedUnit)
	}
	if profile.SpeedRange.TickScale != PercentSpeedTicksPerPercent {
		t.Fatalf("tick scale = %d, want %d", profile.SpeedRange.TickScale, PercentSpeedTicksPerPercent)
	}
	if profile.Connection.Endpoint != "192.168.1.50" {
		t.Fatalf("endpoint = %q, want 192.168.1.50", profile.Connection.Endpoint)
	}
	if profile.Connection.StateEndpoint != "/api/data" || profile.Connection.SpeedEndpoint != "/api/speed" {
		t.Fatalf("endpoints = %q/%q, want /api/data and /api/speed", profile.Connection.StateEndpoint, profile.Connection.SpeedEndpoint)
	}
	if !profile.Capabilities.SupportsSetSpeed || !profile.Capabilities.SupportsReadState {
		t.Fatalf("default WiFi profile should support read state and set speed: %#v", profile.Capabilities)
	}
}

func TestDefaultWiFiPercentTemplateProfileUsesTemplateName(t *testing.T) {
	profile := NormalizeDeviceProfile(DefaultWiFiPercentTemplateProfile("192.168.1.50"), "")

	if profile.ID != DefaultWiFiPercentTemplateProfileID {
		t.Fatalf("template profile ID = %q, want %q", profile.ID, DefaultWiFiPercentTemplateProfileID)
	}
	if profile.DisplayName != appmeta.DeviceTemplateName {
		t.Fatalf("template display name = %q, want %q", profile.DisplayName, appmeta.DeviceTemplateName)
	}
	if profile.Model == appmeta.DeviceModelName {
		t.Fatalf("template model should not use the concrete device name %q", appmeta.DeviceModelName)
	}
}

func TestNormalizeDefaultWiFiProfileUpdatesLegacyDisplayName(t *testing.T) {
	profile := DefaultWiFiPercentProfile("192.168.1.50")
	profile.DisplayName = "WiFi percent controller"
	profile.Model = "WiFi"
	profile.Capabilities.DisplayName = "WiFi percent controller"

	normalized := NormalizeDeviceProfile(profile, "")
	if normalized.DisplayName != appmeta.DeviceModelName || normalized.Model != appmeta.DeviceModelName {
		t.Fatalf("normalized name/model = %q/%q, want %q", normalized.DisplayName, normalized.Model, appmeta.DeviceModelName)
	}
	if normalized.Capabilities.DisplayName != appmeta.DeviceModelName {
		t.Fatalf("capability display name = %q, want %q", normalized.Capabilities.DisplayName, appmeta.DeviceModelName)
	}
}

func TestLegacyRPMProfileUsesRPMAndHID(t *testing.T) {
	profile := NormalizeDeviceProfile(LegacyRPMProfile(), "")

	if profile.ID != LegacyRPMProfileID {
		t.Fatalf("profile ID = %q, want %q", profile.ID, LegacyRPMProfileID)
	}
	if profile.Transport != DeviceTransportHID || profile.SpeedUnit != FanSpeedUnitRPM {
		t.Fatalf("profile transport/unit = %q/%q, want hid/rpm", profile.Transport, profile.SpeedUnit)
	}
	if profile.Capabilities.SupportsDebugFrames || profile.Capabilities.SupportsRawCommands ||
		profile.Capabilities.SupportsLighting || profile.Capabilities.SupportsPowerOnStart || profile.Capabilities.SupportsSmartStartStop {
		t.Fatalf("legacy RPM profile should not expose non-speed capabilities until whitelisted: %#v", profile.Capabilities)
	}
}

func TestLegacyBLEProfileDoesNotInheritNonSpeedCapabilities(t *testing.T) {
	profile := NormalizeDeviceProfile(LegacyRPMProfileForTransport(DeviceTransportBLE), "")

	if profile.Transport != DeviceTransportBLE || profile.SpeedUnit != FanSpeedUnitRPM {
		t.Fatalf("profile transport/unit = %q/%q, want ble/rpm", profile.Transport, profile.SpeedUnit)
	}
	if !profile.Capabilities.SupportsReadState || !profile.Capabilities.SupportsSetSpeed {
		t.Fatalf("legacy BLE profile should keep speed-control capabilities: %#v", profile.Capabilities)
	}
	if profile.Capabilities.SupportsDebugFrames || profile.Capabilities.SupportsRawCommands ||
		profile.Capabilities.SupportsLighting || profile.Capabilities.SupportsPowerOnStart || profile.Capabilities.SupportsSmartStartStop {
		t.Fatalf("legacy BLE profile should not expose non-speed capabilities until whitelisted: %#v", profile.Capabilities)
	}
}

func TestNormalizeDeviceProfilePreservesValidPercentTickScale(t *testing.T) {
	profile := DefaultWiFiPercentProfile("192.168.1.51")
	profile.SpeedRange = DeviceSpeedRange{Min: 0, Max: 100, Step: 5, TickScale: 100}
	normalized := NormalizeDeviceProfile(profile, "")

	if normalized.SpeedRange.TickScale != 100 {
		t.Fatalf("tick scale = %d, want 100", normalized.SpeedRange.TickScale)
	}
	if normalized.SpeedRange.Step != 5 {
		t.Fatalf("step = %d, want 5", normalized.SpeedRange.Step)
	}
}

func TestNormalizeDeviceProfileConfigDerivesFromOldFields(t *testing.T) {
	cfg := &AppConfig{
		DeviceTransport:    DeviceTransportWiFi,
		FanControlDeviceIp: "10.0.0.25",
	}
	if !NormalizeDeviceProfileConfig(cfg) {
		t.Fatal("expected missing profile fields to be filled")
	}
	if cfg.ActiveDeviceProfileID != DefaultWiFiPercentProfileID {
		t.Fatalf("active profile = %q, want %q", cfg.ActiveDeviceProfileID, DefaultWiFiPercentProfileID)
	}
	active := ActiveDeviceProfile(cfg)
	if active.Connection.Endpoint != "10.0.0.25" {
		t.Fatalf("active endpoint = %q, want 10.0.0.25", active.Connection.Endpoint)
	}

	cfg = &AppConfig{DeviceTransport: DeviceTransportHID}
	NormalizeDeviceProfileConfig(cfg)
	if DeviceProfileSpeedUnit(cfg) != FanSpeedUnitRPM {
		t.Fatalf("HID legacy config unit = %q, want rpm", DeviceProfileSpeedUnit(cfg))
	}

	cfg = &AppConfig{DeviceTransport: DeviceTransportBLE}
	NormalizeDeviceProfileConfig(cfg)
	if cfg.DeviceTransport != DeviceTransportBLE {
		t.Fatalf("BLE legacy config transport = %q, want ble", cfg.DeviceTransport)
	}
	if DeviceProfileSpeedUnit(cfg) != FanSpeedUnitRPM {
		t.Fatalf("BLE legacy config unit = %q, want rpm", DeviceProfileSpeedUnit(cfg))
	}
}

func TestNormalizeDeviceProfileConfigSwitchesRequestedTransport(t *testing.T) {
	cfg := &AppConfig{
		DeviceTransport:       DeviceTransportBLE,
		FanControlDeviceIp:    "10.0.0.25",
		ActiveDeviceProfileID: DefaultWiFiPercentProfileID,
		DeviceProfiles:        []DeviceProfile{DefaultWiFiPercentProfile("10.0.0.25")},
	}

	if !NormalizeDeviceProfileConfig(cfg) {
		t.Fatal("expected BLE transport request to update device profile config")
	}
	if cfg.DeviceTransport != DeviceTransportBLE {
		t.Fatalf("device transport = %q, want ble", cfg.DeviceTransport)
	}
	if cfg.ActiveDeviceProfileID != LegacyRPMProfileID {
		t.Fatalf("active profile = %q, want %q", cfg.ActiveDeviceProfileID, LegacyRPMProfileID)
	}
	active := ActiveDeviceProfile(cfg)
	if active.Transport != DeviceTransportBLE || active.SpeedUnit != FanSpeedUnitRPM {
		t.Fatalf("active profile transport/unit = %q/%q, want ble/rpm", active.Transport, active.SpeedUnit)
	}
}

func TestNormalizeDeviceProfileConfigSwitchesToExistingSerialProfile(t *testing.T) {
	serial := DeviceProfile{
		ID:          "user.serial.percent",
		DisplayName: "Serial percent",
		Transport:   DeviceTransportSerial,
		SpeedUnit:   FanSpeedUnitPercent,
		SpeedRange:  DefaultPercentSpeedRange(),
		Connection: DeviceConnectionSettings{
			SerialPort:           "COM3",
			SerialBaudRate:       115200,
			SerialDataBits:       8,
			SerialStopBits:       1,
			SerialParity:         "none",
			SerialFrameDelimiter: "\n",
		},
		Capabilities: DeviceCapabilities{
			Transport:        DeviceTransportSerial,
			SpeedUnit:        FanSpeedUnitPercent,
			SpeedRange:       DefaultPercentSpeedRange(),
			SupportsSetSpeed: true,
		},
	}
	cfg := &AppConfig{
		DeviceTransport:       DeviceTransportSerial,
		FanControlDeviceIp:    "10.0.0.25",
		ActiveDeviceProfileID: DefaultWiFiPercentProfileID,
		DeviceProfiles:        []DeviceProfile{DefaultWiFiPercentProfile("10.0.0.25"), serial},
	}

	if !NormalizeDeviceProfileConfig(cfg) {
		t.Fatal("expected serial transport request to select the serial profile")
	}
	if cfg.ActiveDeviceProfileID != serial.ID {
		t.Fatalf("active profile = %q, want %q", cfg.ActiveDeviceProfileID, serial.ID)
	}
	if cfg.DeviceTransport != DeviceTransportSerial {
		t.Fatalf("device transport = %q, want serial", cfg.DeviceTransport)
	}
}

func TestNormalizeDeviceProfileConfigPreservesActiveProfileWithinRequestedTransport(t *testing.T) {
	first := DeviceProfile{
		ID:          "user.serial.first",
		DisplayName: "First serial",
		Transport:   DeviceTransportSerial,
		SpeedUnit:   FanSpeedUnitPercent,
		SpeedRange:  DefaultPercentSpeedRange(),
		Connection: DeviceConnectionSettings{
			SerialPort:     "COM3",
			SerialBaudRate: 115200,
		},
		Capabilities: DeviceCapabilities{
			Transport:        DeviceTransportSerial,
			SpeedUnit:        FanSpeedUnitPercent,
			SpeedRange:       DefaultPercentSpeedRange(),
			SupportsSetSpeed: true,
		},
	}
	second := first
	second.ID = "user.serial.second"
	second.DisplayName = "Second serial"
	second.Connection.SerialPort = "COM9"

	cfg := &AppConfig{
		DeviceTransport:       DeviceTransportSerial,
		FanControlDeviceIp:    "10.0.0.25",
		ActiveDeviceProfileID: second.ID,
		ActiveDeviceProfileIDsByTransport: map[string]string{
			DeviceTransportSerial: second.ID,
		},
		DeviceProfiles: []DeviceProfile{
			DefaultWiFiPercentProfile("10.0.0.25"),
			first,
			second,
		},
	}

	if NormalizeDeviceProfileConfig(cfg) {
		t.Fatal("expected active serial profile to already be normalized for requested transport")
	}
	if cfg.ActiveDeviceProfileID != second.ID {
		t.Fatalf("active profile = %q, want %q", cfg.ActiveDeviceProfileID, second.ID)
	}
	if cfg.DeviceTransport != DeviceTransportSerial {
		t.Fatalf("device transport = %q, want serial", cfg.DeviceTransport)
	}
}

func TestNormalizeDeviceProfileConfigUsesRememberedProfileForTransport(t *testing.T) {
	first := DeviceProfile{
		ID:          "user.serial.first",
		DisplayName: "First serial",
		Transport:   DeviceTransportSerial,
		SpeedUnit:   FanSpeedUnitPercent,
		SpeedRange:  DefaultPercentSpeedRange(),
		Connection: DeviceConnectionSettings{
			SerialPort:     "COM3",
			SerialBaudRate: 115200,
		},
		Capabilities: DeviceCapabilities{
			Transport:        DeviceTransportSerial,
			SpeedUnit:        FanSpeedUnitPercent,
			SpeedRange:       DefaultPercentSpeedRange(),
			SupportsSetSpeed: true,
		},
	}
	second := first
	second.ID = "user.serial.second"
	second.DisplayName = "Second serial"
	second.Connection.SerialPort = "COM9"

	cfg := &AppConfig{
		DeviceTransport:       DeviceTransportSerial,
		FanControlDeviceIp:    "10.0.0.25",
		ActiveDeviceProfileID: DefaultWiFiPercentProfileID,
		DeviceProfiles:        []DeviceProfile{DefaultWiFiPercentProfile("10.0.0.25"), first, second},
		ActiveDeviceProfileIDsByTransport: map[string]string{
			DeviceTransportWiFi:   DefaultWiFiPercentProfileID,
			DeviceTransportSerial: second.ID,
		},
	}

	if !NormalizeDeviceProfileConfig(cfg) {
		t.Fatal("expected serial transport request to select the remembered serial profile")
	}
	if cfg.ActiveDeviceProfileID != second.ID {
		t.Fatalf("active profile = %q, want remembered %q", cfg.ActiveDeviceProfileID, second.ID)
	}
	if cfg.ActiveDeviceProfileIDsByTransport[DeviceTransportWiFi] != DefaultWiFiPercentProfileID {
		t.Fatalf("remembered wifi profile = %q, want %q", cfg.ActiveDeviceProfileIDsByTransport[DeviceTransportWiFi], DefaultWiFiPercentProfileID)
	}
}

func TestActiveDeviceProfilePrefersRequestedTransportOverStaleGlobalActiveID(t *testing.T) {
	cfg := &AppConfig{
		DeviceTransport:       DeviceTransportHID,
		FanControlDeviceIp:    "10.0.0.25",
		ActiveDeviceProfileID: DefaultWiFiPercentProfileID,
		DeviceProfiles: []DeviceProfile{
			DefaultWiFiPercentProfile("10.0.0.25"),
			LegacyRPMProfile(),
		},
		ActiveDeviceProfileIDsByTransport: map[string]string{
			DeviceTransportWiFi: DefaultWiFiPercentProfileID,
			DeviceTransportHID:  LegacyRPMProfileID,
		},
	}

	active := ActiveDeviceProfile(cfg)
	if active.Transport != DeviceTransportHID || active.SpeedUnit != FanSpeedUnitRPM {
		t.Fatalf("active profile transport/unit = %q/%q, want hid/rpm", active.Transport, active.SpeedUnit)
	}
	if DeviceProfileSpeedUnit(cfg) != FanSpeedUnitRPM {
		t.Fatalf("configured speed unit = %q, want rpm", DeviceProfileSpeedUnit(cfg))
	}
}

func TestActiveDeviceProfileUsesRequestedBuiltInRPMProfileWhenMissingFromConfig(t *testing.T) {
	cfg := &AppConfig{
		DeviceTransport:       DeviceTransportHID,
		FanControlDeviceIp:    "10.0.0.25",
		ActiveDeviceProfileID: DefaultWiFiPercentProfileID,
		DeviceProfiles:        []DeviceProfile{DefaultWiFiPercentProfile("10.0.0.25")},
	}

	active := ActiveDeviceProfile(cfg)
	if active.Transport != DeviceTransportHID || active.SpeedUnit != FanSpeedUnitRPM {
		t.Fatalf("active profile transport/unit = %q/%q, want hid/rpm", active.Transport, active.SpeedUnit)
	}
}
