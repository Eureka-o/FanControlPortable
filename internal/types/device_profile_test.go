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
	if profile.Capabilities.SupportsSoftwareSmartStartStop {
		t.Fatalf("default WiFi profile should not expose software smart start/stop: %#v", profile.Capabilities)
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
		profile.Capabilities.SupportsGearLight || profile.Capabilities.SupportsLighting ||
		profile.Capabilities.SupportsBrightness || profile.Capabilities.SupportsScreen ||
		profile.Capabilities.SupportsPowerOnStart || profile.Capabilities.SupportsSmartStartStop ||
		profile.Capabilities.SupportsSoftwareSmartStartStop {
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
		profile.Capabilities.SupportsGearLight || profile.Capabilities.SupportsLighting ||
		profile.Capabilities.SupportsBrightness || profile.Capabilities.SupportsScreen ||
		profile.Capabilities.SupportsPowerOnStart || profile.Capabilities.SupportsSmartStartStop ||
		profile.Capabilities.SupportsSoftwareSmartStartStop {
		t.Fatalf("legacy BLE profile should not expose non-speed capabilities until whitelisted: %#v", profile.Capabilities)
	}
}

func TestBuiltInDeviceProfilesIncludeFlyDigiProfiles(t *testing.T) {
	profiles := BuiltInDeviceProfiles("10.0.0.25")
	expectedIDs := []string{
		DefaultWiFiPercentProfileID,
		FlyDigiBS1ProfileID,
		FlyDigiBS2ProfileID,
		FlyDigiBS2PROProfileID,
		FlyDigiBS3ProfileID,
		FlyDigiBS3PROProfileID,
	}
	for _, id := range expectedIDs {
		found := false
		for _, profile := range profiles {
			if profile.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("built-in profiles should include %q: %#v", id, profiles)
		}
	}
	for _, profile := range profiles {
		if profile.ID == LegacyRPMProfileID {
			t.Fatalf("legacy RPM profile should not be part of the visible built-in device library: %#v", profile)
		}
	}
}

func TestDefaultConfigDoesNotPersistWiFiWithoutCompatibility(t *testing.T) {
	cfg := GetDefaultConfig(false)

	if cfg.WiFiCompatibilityEnabled {
		t.Fatal("WiFi compatibility should be disabled by default")
	}
	if cfg.DeviceTransport != "" || cfg.ActiveDeviceProfileID != "" {
		t.Fatalf("default compatibility identity = %q/%q, want empty", cfg.DeviceTransport, cfg.ActiveDeviceProfileID)
	}
	for _, profile := range cfg.DeviceProfiles {
		if NormalizeDeviceTransport(profile.Transport) == DeviceTransportWiFi {
			t.Fatalf("default config persisted WiFi profile %#v", profile)
		}
	}
	if active := ActiveDeviceProfile(&cfg); active.ID != "" || active.Transport != "" {
		t.Fatalf("active profile = %#v, want no compatibility profile", active)
	}
}

func TestFlyDigiBuiltInProfilesDeclareExpectedCapabilities(t *testing.T) {
	bs1 := NormalizeDeviceProfile(FlyDigiBS1Profile(), "")
	if bs1.ID != FlyDigiBS1ProfileID {
		t.Fatalf("BS1 profile ID = %q, want %q", bs1.ID, FlyDigiBS1ProfileID)
	}
	if bs1.DisplayName != "飞智（FlyDigi）BS1" {
		t.Fatalf("BS1 display name = %q", bs1.DisplayName)
	}
	if bs1.Transport != DeviceTransportBLE || bs1.SpeedUnit != FanSpeedUnitRPM {
		t.Fatalf("BS1 transport/unit = %q/%q, want ble/rpm", bs1.Transport, bs1.SpeedUnit)
	}
	if bs1.Connection.BLEServiceUUID != "fff0" || bs1.Connection.BLEWriteCharacteristic != "fff2" || bs1.Connection.BLENotifyCharacteristic != "fff1" {
		t.Fatalf("BS1 BLE connection = %#v, want FFF0/FFF2/FFF1", bs1.Connection)
	}
	if !bs1.Capabilities.SupportsPowerOnStart {
		t.Fatalf("BS1 should support power-on-start: %#v", bs1.Capabilities)
	}
	if bs1.Capabilities.SupportsGearLight || bs1.Capabilities.SupportsLighting ||
		bs1.Capabilities.SupportsBrightness || bs1.Capabilities.SupportsScreen ||
		bs1.Capabilities.SupportsSmartStartStop || bs1.Capabilities.SupportsSoftwareSmartStartStop {
		t.Fatalf("BS1 should not expose lighting, screen, or smart start/stop: %#v", bs1.Capabilities)
	}

	hidProfiles := []DeviceProfile{
		FlyDigiBS2Profile(),
		FlyDigiBS2PROProfile(),
		FlyDigiBS3Profile(),
		FlyDigiBS3PROProfile(),
	}
	for _, raw := range hidProfiles {
		profile := NormalizeDeviceProfile(raw, "")
		if profile.Transport != DeviceTransportHID || profile.SpeedUnit != FanSpeedUnitRPM {
			t.Fatalf("%s transport/unit = %q/%q, want hid/rpm", profile.ID, profile.Transport, profile.SpeedUnit)
		}
		if !profile.Capabilities.SupportsSetSpeed || !profile.Capabilities.SupportsReadState ||
			!profile.Capabilities.SupportsManualGears || !profile.Capabilities.SupportsCustomSpeed {
			t.Fatalf("%s should expose speed/read/manual/custom capabilities: %#v", profile.ID, profile.Capabilities)
		}
		if !profile.Capabilities.SupportsGearLight || !profile.Capabilities.SupportsLighting ||
			!profile.Capabilities.SupportsBrightness || !profile.Capabilities.SupportsPowerOnStart ||
			!profile.Capabilities.SupportsSmartStartStop {
			t.Fatalf("%s should expose whitelisted HID device functions: %#v", profile.ID, profile.Capabilities)
		}
		if profile.Capabilities.SupportsScreen {
			t.Fatalf("%s should not expose screen support without a verified whitelist: %#v", profile.ID, profile.Capabilities)
		}
		if profile.Capabilities.SupportsSoftwareSmartStartStop {
			t.Fatalf("%s should not use WiFi software smart start/stop whitelist: %#v", profile.ID, profile.Capabilities)
		}
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
		DeviceTransport:          DeviceTransportWiFi,
		FanControlDeviceIp:       "10.0.0.25",
		WiFiCompatibilityEnabled: true,
	}
	if !NormalizeDeviceProfileConfig(cfg) {
		t.Fatal("expected missing profile fields to be filled")
	}
	if cfg.ActiveDeviceProfileID != DefaultWiFiPercentProfileID {
		t.Fatalf("active profile = %q, want %q", cfg.ActiveDeviceProfileID, DefaultWiFiPercentProfileID)
	}
	for _, id := range []string{DefaultWiFiPercentProfileID, FlyDigiBS1ProfileID, FlyDigiBS2ProfileID, FlyDigiBS2PROProfileID, FlyDigiBS3ProfileID, FlyDigiBS3PROProfileID} {
		found := false
		for _, profile := range cfg.DeviceProfiles {
			if profile.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("normalized config should contain built-in profile %q", id)
		}
	}
	for _, profile := range cfg.DeviceProfiles {
		if profile.ID == LegacyRPMProfileID {
			t.Fatalf("legacy RPM profile should not remain in normalized device profiles: %#v", profile)
		}
	}
	active := ActiveDeviceProfile(cfg)
	if active.Connection.Endpoint != "10.0.0.25" {
		t.Fatalf("active endpoint = %q, want 10.0.0.25", active.Connection.Endpoint)
	}

	cfg = &AppConfig{DeviceTransport: DeviceTransportHID}
	NormalizeDeviceProfileConfig(cfg)
	if cfg.DeviceTransport != "" {
		t.Fatalf("HID persistent config transport = %q, want empty", cfg.DeviceTransport)
	}
	if DeviceProfileSpeedUnit(cfg) != FanSpeedUnitPercent {
		t.Fatalf("HID legacy config unit = %q, want percent", DeviceProfileSpeedUnit(cfg))
	}

	cfg = &AppConfig{DeviceTransport: DeviceTransportBLE}
	NormalizeDeviceProfileConfig(cfg)
	if cfg.DeviceTransport != "" {
		t.Fatalf("BLE persistent config transport = %q, want empty", cfg.DeviceTransport)
	}
	if DeviceProfileSpeedUnit(cfg) != FanSpeedUnitPercent {
		t.Fatalf("BLE legacy config unit = %q, want percent", DeviceProfileSpeedUnit(cfg))
	}
}

func TestNormalizeDeviceProfileConfigMigratesNativeTransportToCompatibilityWiFi(t *testing.T) {
	cfg := &AppConfig{
		DeviceTransport:          DeviceTransportBLE,
		FanControlDeviceIp:       "10.0.0.25",
		WiFiCompatibilityEnabled: true,
		ActiveDeviceProfileID:    DefaultWiFiPercentProfileID,
		DeviceProfiles:           []DeviceProfile{DefaultWiFiPercentProfile("10.0.0.25")},
	}

	if !NormalizeDeviceProfileConfig(cfg) {
		t.Fatal("expected native transport request to update device profile config")
	}
	if cfg.DeviceTransport != DeviceTransportWiFi {
		t.Fatalf("device transport = %q, want wifi", cfg.DeviceTransport)
	}
	if cfg.ActiveDeviceProfileID != DefaultWiFiPercentProfileID {
		t.Fatalf("active profile = %q, want %q", cfg.ActiveDeviceProfileID, DefaultWiFiPercentProfileID)
	}
	if _, ok := cfg.ActiveDeviceProfileIDsByTransport[DeviceTransportBLE]; ok {
		t.Fatalf("BLE active id should not persist in compatibility config: %#v", cfg.ActiveDeviceProfileIDsByTransport)
	}
	active := ActiveDeviceProfile(cfg)
	if active.Transport != DeviceTransportWiFi || active.SpeedUnit != FanSpeedUnitPercent {
		t.Fatalf("active profile transport/unit = %q/%q, want wifi/percent", active.Transport, active.SpeedUnit)
	}
}

func TestNormalizeDeviceProfileConfigMigratesPersistedBS1ActiveConfig(t *testing.T) {
	cfg := &AppConfig{
		DeviceTransport:          DeviceTransportBLE,
		FanControlDeviceIp:       "192.168.137.2",
		WiFiCompatibilityEnabled: true,
		ActiveDeviceProfileID:    FlyDigiBS1ProfileID,
		ActiveDeviceProfileIDsByTransport: map[string]string{
			DeviceTransportBLE:  FlyDigiBS1ProfileID,
			DeviceTransportWiFi: DefaultWiFiPercentProfileID,
		},
		DeviceProfiles: []DeviceProfile{
			DefaultWiFiPercentProfile("192.168.137.2"),
			FlyDigiBS1Profile(),
		},
	}

	if !NormalizeDeviceProfileConfig(cfg) {
		t.Fatal("expected persisted BS1 active config to be migrated")
	}
	if cfg.DeviceTransport != DeviceTransportWiFi {
		t.Fatalf("device transport = %q, want wifi", cfg.DeviceTransport)
	}
	if cfg.ActiveDeviceProfileID != DefaultWiFiPercentProfileID {
		t.Fatalf("active profile = %q, want %q", cfg.ActiveDeviceProfileID, DefaultWiFiPercentProfileID)
	}
	if _, ok := cfg.ActiveDeviceProfileIDsByTransport[DeviceTransportBLE]; ok {
		t.Fatalf("BLE active id should be removed after migration: %#v", cfg.ActiveDeviceProfileIDsByTransport)
	}
	if cfg.ActiveDeviceProfileIDsByTransport[DeviceTransportWiFi] != DefaultWiFiPercentProfileID {
		t.Fatalf("wifi remembered profile = %q, want %q", cfg.ActiveDeviceProfileIDsByTransport[DeviceTransportWiFi], DefaultWiFiPercentProfileID)
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
		DeviceTransport:            DeviceTransportSerial,
		FanControlDeviceIp:         "10.0.0.25",
		SerialCompatibilityEnabled: true,
		ActiveDeviceProfileID:      DefaultWiFiPercentProfileID,
		DeviceProfiles:             []DeviceProfile{DefaultWiFiPercentProfile("10.0.0.25"), serial},
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
		DeviceTransport:            DeviceTransportSerial,
		FanControlDeviceIp:         "10.0.0.25",
		SerialCompatibilityEnabled: true,
		ActiveDeviceProfileID:      second.ID,
		ActiveDeviceProfileIDsByTransport: map[string]string{
			DeviceTransportSerial: second.ID,
		},
		DeviceProfiles: []DeviceProfile{
			DefaultWiFiPercentProfile("10.0.0.25"),
			first,
			second,
		},
	}

	NormalizeDeviceProfileConfig(cfg)
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
		DeviceTransport:            DeviceTransportSerial,
		FanControlDeviceIp:         "10.0.0.25",
		WiFiCompatibilityEnabled:   true,
		SerialCompatibilityEnabled: true,
		ActiveDeviceProfileID:      DefaultWiFiPercentProfileID,
		DeviceProfiles:             []DeviceProfile{DefaultWiFiPercentProfile("10.0.0.25"), first, second},
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

func TestActiveDeviceProfileIgnoresNativeTransportInPersistentConfig(t *testing.T) {
	cfg := &AppConfig{
		DeviceTransport:          DeviceTransportHID,
		FanControlDeviceIp:       "10.0.0.25",
		WiFiCompatibilityEnabled: true,
		ActiveDeviceProfileID:    DefaultWiFiPercentProfileID,
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
	if active.Transport != DeviceTransportWiFi || active.SpeedUnit != FanSpeedUnitPercent {
		t.Fatalf("active profile transport/unit = %q/%q, want wifi/percent", active.Transport, active.SpeedUnit)
	}
	if DeviceProfileSpeedUnit(cfg) != FanSpeedUnitPercent {
		t.Fatalf("configured speed unit = %q, want percent", DeviceProfileSpeedUnit(cfg))
	}
}

func TestActiveDeviceProfileFallsBackToWiFiWhenNativeProfileMissingFromConfig(t *testing.T) {
	cfg := &AppConfig{
		DeviceTransport:          DeviceTransportHID,
		FanControlDeviceIp:       "10.0.0.25",
		WiFiCompatibilityEnabled: true,
		ActiveDeviceProfileID:    DefaultWiFiPercentProfileID,
		DeviceProfiles:           []DeviceProfile{DefaultWiFiPercentProfile("10.0.0.25")},
	}

	active := ActiveDeviceProfile(cfg)
	if active.Transport != DeviceTransportWiFi || active.SpeedUnit != FanSpeedUnitPercent {
		t.Fatalf("active profile transport/unit = %q/%q, want wifi/percent", active.Transport, active.SpeedUnit)
	}
	if active.ID != DefaultWiFiPercentProfileID {
		t.Fatalf("active profile = %q, want %q", active.ID, DefaultWiFiPercentProfileID)
	}
}

func TestNormalizeDeviceProfileConfigHidesBuiltInWiFiWhenCompatibilityDisabled(t *testing.T) {
	custom := DefaultWiFiPercentProfile("10.0.0.50")
	custom.ID = "user.wifi.custom"
	custom.DisplayName = "Custom WiFi"
	custom.BuiltIn = false
	cfg := &AppConfig{
		DeviceTransport:       DeviceTransportWiFi,
		FanControlDeviceIp:    "10.0.0.25",
		ActiveDeviceProfileID: DefaultWiFiPercentProfileID,
		ActiveDeviceProfileIDsByTransport: map[string]string{
			DeviceTransportWiFi: DefaultWiFiPercentProfileID,
		},
		DeviceProfiles: []DeviceProfile{
			DefaultWiFiPercentProfile("10.0.0.25"),
			custom,
		},
	}

	NormalizeDeviceProfileConfig(cfg)
	if cfg.DeviceTransport != "" || cfg.ActiveDeviceProfileID != "" {
		t.Fatalf("disabled WiFi compatibility identity = %q/%q, want empty", cfg.DeviceTransport, cfg.ActiveDeviceProfileID)
	}
	if _, ok := cfg.ActiveDeviceProfileIDsByTransport[DeviceTransportWiFi]; ok {
		t.Fatalf("disabled WiFi active identity should be removed: %#v", cfg.ActiveDeviceProfileIDsByTransport)
	}
	foundCustom := false
	for _, profile := range cfg.DeviceProfiles {
		if profile.ID == DefaultWiFiPercentProfileID {
			t.Fatalf("built-in WiFi profile should not persist while compatibility is disabled: %#v", profile)
		}
		if profile.ID == custom.ID {
			foundCustom = true
		}
	}
	if !foundCustom {
		t.Fatal("custom WiFi profile should be preserved for upgrade safety")
	}
}

func TestNormalizeDeviceProfileConfigRestoresBuiltInWiFiWhenCompatibilityEnabled(t *testing.T) {
	cfg := GetDefaultConfig(false)
	cfg.WiFiCompatibilityEnabled = true
	cfg.DeviceTransport = DeviceTransportWiFi

	NormalizeDeviceProfileConfig(&cfg)
	if cfg.DeviceTransport != DeviceTransportWiFi || cfg.ActiveDeviceProfileID != DefaultWiFiPercentProfileID {
		t.Fatalf("enabled WiFi compatibility identity = %q/%q", cfg.DeviceTransport, cfg.ActiveDeviceProfileID)
	}
	if active := ActiveDeviceProfile(&cfg); active.ID != DefaultWiFiPercentProfileID || active.Transport != DeviceTransportWiFi {
		t.Fatalf("active WiFi profile = %#v", active)
	}
}
