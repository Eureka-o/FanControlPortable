package types

import (
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/appmeta"
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
		BlackSharkBRB02ProfileID,
		BlackSharkBRB02USBProfileID,
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

func TestBlackSharkBRB02ProfileDeclaresNativeLighting(t *testing.T) {
	profile := NormalizeDeviceProfile(BlackSharkBRB02Profile(), "")
	if profile.DisplayName != BlackSharkBRB02DisplayName || profile.Vendor != BlackSharkBRB02Vendor {
		t.Fatalf("Black Shark identity = %q/%q", profile.DisplayName, profile.Vendor)
	}
	// 量程必须等于所有者给的值：档案里不许再写一份数（写成 0..4000 会与标定表和界面预设自相矛盾）。
	if profile.Model != BlackSharkBRB02DisplayName ||
		profile.SpeedRange.Min != BlackSharkHIDMinRPM || profile.SpeedRange.Max != BlackSharkHIDMaxRPM ||
		profile.Capabilities.SpeedRange.Min != BlackSharkHIDMinRPM || profile.Capabilities.SpeedRange.Max != BlackSharkHIDMaxRPM {
		t.Fatalf("Black Shark public model/speed range = %q/%#v/%#v", profile.Model, profile.SpeedRange, profile.Capabilities.SpeedRange)
	}
	if profile.Connection.BLENameFilter != "BS BRB02 Cooler Pro" || profile.Connection.BLEWriteCharacteristic != "ae41" || profile.Connection.BLENotifyCharacteristic != "ae04" {
		t.Fatalf("Black Shark BLE connection = %#v", profile.Connection)
	}
	// 灯效 / 亮度 / 挡位灯一律不声明：声明了会把上游那套通用面板点亮成黑鲨的第二份界面，
	// 黑鲨这些功能由本工具自己的页面提供（灯效页）。声明为 false 也让灯带配置下发与
	// 挡位灯管理对黑鲨整体关闭。
	if profile.Capabilities.SupportsLighting || profile.Capabilities.SupportsBrightness ||
		profile.Capabilities.SupportsGearLight || profile.Capabilities.SupportsManualGears {
		t.Fatalf("Black Shark capabilities = %#v", profile.Capabilities)
	}
}

func TestBlackSharkBuiltInLookupAndTransportFiltering(t *testing.T) {
	if BlackSharkBRB02HIDVendorID != 0xE2B7 || BlackSharkBRB02HIDProductID != 0x7001 {
		t.Fatalf("Black Shark HID identifiers = %04X:%04X, want E2B7:7001", BlackSharkBRB02HIDVendorID, BlackSharkBRB02HIDProductID)
	}
	for _, test := range []struct {
		id        string
		transport string
	}{
		{BlackSharkBRB02ProfileID, DeviceTransportBLE},
		{BlackSharkBRB02USBProfileID, DeviceTransportUSB},
	} {
		profile, ok := BuiltInDeviceProfileByID(test.id)
		if !ok || profile.ID != test.id || profile.Transport != test.transport {
			t.Fatalf("lookup %q = %#v/%v, want transport %q", test.id, profile, ok, test.transport)
		}
		if !IsBuiltInDeviceProfileID(test.id) {
			t.Fatalf("%q should be recognized as built-in", test.id)
		}
	}
	profiles := BuiltInDeviceProfiles("")
	for _, transport := range []string{DeviceTransportBLE, DeviceTransportUSB} {
		found := false
		for _, profile := range profiles {
			if profile.ID == BlackSharkBRB02ProfileID && transport == DeviceTransportBLE ||
				profile.ID == BlackSharkBRB02USBProfileID && transport == DeviceTransportUSB {
				if NormalizeDeviceTransport(profile.Transport) == transport {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("built-in Black Shark profile missing for transport %q", transport)
		}
	}
}

func TestEnsureBuiltInDeviceProfilesSeedsBlackSharkTransportsOnce(t *testing.T) {
	cfg := &AppConfig{}
	if !ensureBuiltInDeviceProfiles(cfg) {
		t.Fatal("expected missing built-in profiles to be seeded")
	}
	if ensureBuiltInDeviceProfiles(cfg) {
		t.Fatal("seeding an already-normalized profile list should not change config")
	}
	for _, id := range []string{BlackSharkBRB02ProfileID, BlackSharkBRB02USBProfileID} {
		count := 0
		for _, profile := range cfg.DeviceProfiles {
			if profile.ID == id {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("seeded profile %q count = %d, want exactly one", id, count)
		}
	}
}

func TestBlackSharkHIDProfileIsNotBuiltIn(t *testing.T) {
	const legacyID = "builtin.blackshark.brb02.hid.rpm"
	if _, ok := BuiltInDeviceProfileByID(legacyID); ok {
		t.Fatal("legacy Black Shark HID profile must not be registered")
	}
	if IsBuiltInDeviceProfileID(legacyID) {
		t.Fatal("legacy Black Shark HID profile must not be recognized as built-in")
	}
	for _, profile := range BuiltInDeviceProfiles("") {
		if profile.ID == legacyID {
			t.Fatal("legacy Black Shark HID profile must not be seeded")
		}
	}
}

func TestNormalizeDeviceProfileConfigDropsLegacyBlackSharkHIDProfile(t *testing.T) {
	cfg := &AppConfig{DeviceProfiles: []DeviceProfile{{ID: "builtin.blackshark.brb02.hid.rpm", DisplayName: "legacy", Transport: DeviceTransportHID, SpeedUnit: FanSpeedUnitRPM}}}
	if !NormalizeDeviceProfileConfig(cfg) {
		t.Fatal("expected legacy Black Shark HID profile cleanup")
	}
	for _, profile := range cfg.DeviceProfiles {
		if profile.ID == "builtin.blackshark.brb02.hid.rpm" {
			t.Fatal("legacy Black Shark HID profile survived normalization")
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

// 黑鲨：卡片标签与能力位。

// 卡片标签必须如实列出黑鲨在专属面板里已有的能力，顺序也参与断言。
func TestBlackSharkCardShowsItsRealFeatures(t *testing.T) {
	profile := BlackSharkFengShenProProfile()
	want := []string{
		DeviceDisplayFeatureReadState,
		DeviceDisplayFeatureSetSpeed,
		DeviceDisplayFeatureManualGears,
		DeviceDisplayFeatureCustomSpeed,
		DeviceDisplayFeatureLighting,
		DeviceDisplayFeatureBrightness,
		DeviceDisplayFeaturePowerOnStart,
		DeviceDisplayFeatureSmartStartStop,
	}
	if len(profile.DisplayFeatures) != len(want) {
		t.Fatalf("黑鲨卡片标签 %d 个，应为 %d 个：%v",
			len(profile.DisplayFeatures), len(want), profile.DisplayFeatures)
	}
	for i, id := range want {
		if profile.DisplayFeatures[i] != id {
			t.Fatalf("第 %d 个标签 = %q，应为 %q（顺序也参与断言，避免顺手漏掉一项）",
				i, profile.DisplayFeatures[i], id)
		}
		if !IsKnownDeviceDisplayFeature(id) {
			t.Fatalf("标签 %q 不在已知集合里 —— 前端会静默不显示，等于少个标签", id)
		}
	}
}

// 黑鲨的能力位必须保持"通用界面不可用"：一旦置真，核心会改走飞智那条协议路径。
func TestBlackSharkCapabilitiesStayOffSoCoreNeverSendsFlyDigiFrames(t *testing.T) {
	caps := BlackSharkFengShenProProfile().Capabilities
	for name, on := range map[string]bool{
		"SupportsLighting":       caps.SupportsLighting,
		"SupportsBrightness":     caps.SupportsBrightness,
		"SupportsGearLight":      caps.SupportsGearLight,
		"SupportsScreen":         caps.SupportsScreen,
		"SupportsPowerOnStart":   caps.SupportsPowerOnStart,
		"SupportsSmartStartStop": caps.SupportsSmartStartStop,
		"SupportsManualGears":    caps.SupportsManualGears,
	} {
		if on {
			t.Fatalf("黑鲨的 %s 被点成 true —— 它会让核心走飞智那条协议路径。"+
				"卡片标签请改用 DisplayFeatures", name)
		}
	}
	if caps.AllowsLightStrip() {
		t.Fatal("AllowsLightStrip() 为真 ⇒ 核心会向黑鲨下发飞智灯带指令")
	}
	if caps.AllowsGearLight() {
		t.Fatal("AllowsGearLight() 为真 ⇒ 核心会向黑鲨下发飞智挡位灯指令")
	}
	if !caps.SupportsReadState || !caps.SupportsSetSpeed || !caps.SupportsCustomSpeed {
		t.Fatalf("黑鲨的速度能力不该被削：%#v", caps)
	}
}

// 内置档案必须能按需刷新：旧配置存的是过期副本，源码改了要跟得上。
func TestEnsureBuiltInDeviceProfilesRefreshesStaleBuiltInMetadata(t *testing.T) {
	cfg := GetDefaultConfig(false)
	// 先让配置落到稳定态，再单独验"刷新"这一条契约。
	// 不要拿 NormalizeDeviceProfileConfig 的返回值当判据：它还管着别的事。
	for i := 0; i < 4 && NormalizeDeviceProfileConfig(&cfg); i++ {
	}
	var idx = -1
	for i := range cfg.DeviceProfiles {
		// 黑鲨内置档案由 BLE / USB 两条注册（见 builtin_device_profiles.go），这里找 USB 那条。
		if cfg.DeviceProfiles[i].ID == BlackSharkBRB02USBProfileID {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("稳定态下仍没有黑鲨档案")
	}
	if ensureBuiltInDeviceProfiles(&cfg) {
		t.Fatal("刚刚稳定下来的配置不该再被判为需要补全")
	}

	// 造一份"老版本"：内置档案在，但元数据与源码不一致
	cfg.DeviceProfiles[idx].DisplayFeatures = nil
	cfg.DeviceProfiles[idx].DisplayName = "旧名字"
	cfg.DeviceProfiles[idx].BuiltIn = true

	if !ensureBuiltInDeviceProfiles(&cfg) {
		t.Fatal("存的是陈旧内置档案，应当报告 changed=true")
	}
	fresh := cfg.DeviceProfiles[idx]
	if len(fresh.DisplayFeatures) == 0 {
		t.Fatal("陈旧的内置档案没有被刷新（DisplayFeatures 仍为空）")
	}
	// 刷新源是 USB 档案（见 builtin_device_profiles.go），所以这里比的是它的显示名；
	// 上一步已证明"旧名字"确实被刷成了源的值。
	if fresh.DisplayName != BlackSharkBRB02DisplayName {
		t.Fatalf("DisplayName 没被刷新: %q", fresh.DisplayName)
	}

	// 再跑一次：已经一致了，就不该再报 changed（否则每次启动都写一遍配置）
	if ensureBuiltInDeviceProfiles(&cfg) {
		t.Fatal("元数据已经一致，不该再报告 changed —— 那会导致每次启动都写配置文件")
	}
}

// 用户自造的同名档案不能被内置版本覆盖掉。
func TestEnsureBuiltInDeviceProfilesLeavesUserProfileAlone(t *testing.T) {
	cfg := GetDefaultConfig(false)
	cfg.DeviceProfiles = append(cfg.DeviceProfiles, DeviceProfile{
		ID:           BlackSharkFengShenProProfileID,
		DisplayName:  "我自己改的",
		Transport:    DeviceTransportHID,
		SpeedUnit:    FanSpeedUnitRPM,
		Capabilities: DeviceCapabilities{SupportsSetSpeed: true},
		BuiltIn:      false, // 用户自造
	})
	NormalizeDeviceProfileConfig(&cfg)
	for i := range cfg.DeviceProfiles {
		p := cfg.DeviceProfiles[i]
		if p.ID == BlackSharkFengShenProProfileID {
			if p.DisplayName != "我自己改的" {
				t.Fatalf("用户自造的同名档案被内置版本覆盖了: %q", p.DisplayName)
			}
			return
		}
	}
	t.Fatal("用户档案不见了")
}
