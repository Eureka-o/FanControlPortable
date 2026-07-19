package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func findDeviceProfileForTest(profiles []types.DeviceProfile, id string) (types.DeviceProfile, bool) {
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return types.DeviceProfile{}, false
}

func TestWriteConfigFileAtomicallyReplacesCompleteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "config.json")
	if err := writeConfigFileAtomically(path, []byte(`{"version":1,"stale":"content"}`)); err != nil {
		t.Fatalf("write initial config: %v", err)
	}
	if err := writeConfigFileAtomically(path, []byte(`{"version":2}`)); err != nil {
		t.Fatalf("replace config: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) != `{"version":2}` {
		t.Fatalf("config = %q, want complete replacement", data)
	}
	temps, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".config-*.tmp"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(temps) != 0 {
		t.Fatalf("temporary config files left behind: %v", temps)
	}
}

func TestUpdateKeepsCurrentConfigWhenPersistenceFails(t *testing.T) {
	blockedPath := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blockedPath, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("create blocked path: %v", err)
	}
	t.Setenv("USERPROFILE", blockedPath)
	t.Setenv("HOME", blockedPath)

	manager := NewManager(blockedPath, nil)
	initial := types.GetDefaultConfig(false)
	manager.Set(initial)
	before, beforeRevision := manager.GetWithRevision()

	next := before
	next.DebugMode = !before.DebugMode
	if err := manager.Update(next); err == nil {
		t.Fatal("Update() error = nil, want persistence failure")
	}

	after, afterRevision := manager.GetWithRevision()
	if after.DebugMode != before.DebugMode {
		t.Fatalf("DebugMode changed after failed update: got %v, want %v", after.DebugMode, before.DebugMode)
	}
	if afterRevision != beforeRevision {
		t.Fatalf("revision changed after failed update: got %d, want %d", afterRevision, beforeRevision)
	}
}

func TestValidateFanCurveForUnitAllowsReferenceRPMCurve(t *testing.T) {
	if err := ValidateFanCurveForUnit(types.GetDefaultRPMFanCurve(), types.FanSpeedUnitRPM); err != nil {
		t.Fatalf("ValidateFanCurveForUnit(RPM default) returned error: %v", err)
	}
}

func TestValidateFanCurveForUnitRejectsPercentOverflow(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 40, RPM: 50},
		{Temperature: 70, RPM: 101},
	}
	if err := ValidateFanCurveForUnit(curve, types.FanSpeedUnitPercent); err == nil {
		t.Fatal("expected percent curve >100 to be rejected")
	}
}

func TestNormalizeSpeedConfigKeepsRPMCurve(t *testing.T) {
	serial := types.DeviceProfile{
		ID:          "user.serial.rpm",
		DisplayName: "Serial RPM",
		Transport:   types.DeviceTransportSerial,
		SpeedUnit:   types.FanSpeedUnitRPM,
		SpeedRange:  types.DefaultRPMSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			SerialPort:     "COM9",
			SerialBaudRate: 115200,
			SerialDataBits: 8,
			SerialStopBits: 1,
			SerialParity:   "none",
		},
		Capabilities: types.DeviceCapabilities{
			Transport:         types.DeviceTransportSerial,
			SpeedUnit:         types.FanSpeedUnitRPM,
			SpeedRange:        types.DefaultRPMSpeedRange(),
			SupportsReadState: true,
			SupportsSetSpeed:  true,
		},
	}
	cfg := types.GetDefaultConfig(false)
	cfg.DeviceTransport = types.DeviceTransportSerial
	cfg.SerialCompatibilityEnabled = true
	cfg.ActiveDeviceProfileID = serial.ID
	cfg.DeviceProfiles = []types.DeviceProfile{serial}
	cfg.FanCurve = types.GetDefaultRPMFanCurve()
	cfg.FanCurveProfiles = []types.FanCurveProfile{{ID: "rpm", Name: "RPM", Curve: types.GetDefaultRPMFanCurve()}}
	cfg.ActiveFanCurveProfileID = "rpm"
	cfg.CustomSpeedRPM = 2000

	normalizeSpeedConfig(&cfg)

	if got := cfg.FanCurve[len(cfg.FanCurve)-1].RPM; got != 4000 {
		t.Fatalf("RPM curve max after normalize = %d, want 4000", got)
	}
	if cfg.CustomSpeedRPM != 2000 {
		t.Fatalf("RPM custom speed after normalize = %d, want 2000", cfg.CustomSpeedRPM)
	}
}

func TestNormalizeSpeedConfigBackfillsOldWiFiProfile(t *testing.T) {
	cfg := types.AppConfig{
		DeviceTransport:          types.DeviceTransportWiFi,
		FanControlDeviceIp:       "10.0.0.9",
		WiFiCompatibilityEnabled: true,
		FanCurve:                 types.GetDefaultFanCurve(),
		CustomSpeedRPM:           45,
	}

	normalizeSpeedConfig(&cfg)

	if cfg.ActiveDeviceProfileID != types.DefaultWiFiPercentProfileID {
		t.Fatalf("active profile = %q, want %q", cfg.ActiveDeviceProfileID, types.DefaultWiFiPercentProfileID)
	}
	if got := types.DeviceProfileSpeedUnit(&cfg); got != types.FanSpeedUnitPercent {
		t.Fatalf("device speed unit = %q, want percent", got)
	}
	if got := types.ActiveDeviceProfile(&cfg).Connection.Endpoint; got != "10.0.0.9" {
		t.Fatalf("active endpoint = %q, want 10.0.0.9", got)
	}
}

func TestLoadUpgradeConfigPreservesWiFiIPAndCurveProfiles(t *testing.T) {
	installDir := t.TempDir()
	configDir := filepath.Join(installDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	curve := []types.FanCurvePoint{
		{Temperature: 25, RPM: 18},
		{Temperature: 50, RPM: 42},
		{Temperature: 75, RPM: 76},
		{Temperature: 100, RPM: 100},
	}
	profiles := []types.FanCurveProfile{
		{ID: "quiet", Name: "Quiet curve", Curve: curve[:3]},
		{ID: "boost", Name: "Boost curve", Curve: curve},
	}
	raw := map[string]any{
		"deviceTransport":         types.DeviceTransportWiFi,
		"fanControlDeviceIp":      "10.8.0.42",
		"fanCurve":                curve,
		"fanCurveProfiles":        profiles,
		"activeFanCurveProfileId": "boost",
		"customSpeedRPM":          37,
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := NewManager(installDir, nil).Load(false)

	if cfg.FanControlDeviceIp != "10.8.0.42" {
		t.Fatalf("FanControlDeviceIp = %q, want 10.8.0.42", cfg.FanControlDeviceIp)
	}
	if got := types.ActiveDeviceProfile(&cfg).Connection.Endpoint; got != "10.8.0.42" {
		t.Fatalf("active device endpoint = %q, want 10.8.0.42", got)
	}
	if cfg.ActiveFanCurveProfileID != "boost" {
		t.Fatalf("active fan curve profile = %q, want boost", cfg.ActiveFanCurveProfileID)
	}
	if len(cfg.FanCurve) != len(curve) || cfg.FanCurve[2].Temperature != 75 || cfg.FanCurve[2].RPM != 76 {
		t.Fatalf("fanCurve not preserved: %#v", cfg.FanCurve)
	}
	if len(cfg.FanCurveProfiles) != 2 || cfg.FanCurveProfiles[1].ID != "boost" || cfg.FanCurveProfiles[1].Curve[1].RPM != 42 {
		t.Fatalf("fanCurveProfiles not preserved: %#v", cfg.FanCurveProfiles)
	}
}

func TestLoadPreservesPersistedNativeRPMCurveBeforeCompatibilityMigration(t *testing.T) {
	installDir := t.TempDir()
	configDir := filepath.Join(installDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	curve := []types.FanCurvePoint{
		{Temperature: 30, RPM: 800},
		{Temperature: 55, RPM: 1400},
		{Temperature: 75, RPM: 2300},
		{Temperature: 95, RPM: 3600},
	}
	manualGearRPM := types.CloneDefaultRPMManualGearRPM()
	manualGearRPM["强劲"]["高"] = 4200
	smartControl := types.GetDefaultSmartControlConfigForUnit(curve, types.FanSpeedUnitRPM)
	smartControl.LearnedOffsets = []int{0, 11, 22, 33}
	smartControl.LearnedOffsetsByProfile = map[string][]int{
		"bs1-custom": {3, 5, 7, 9},
	}
	deviceKey := types.DeviceTransportBLE + "::" + types.FlyDigiBS1ProfileID
	raw := map[string]any{
		"deviceTransport":       types.DeviceTransportBLE,
		"activeDeviceProfileId": types.FlyDigiBS1ProfileID,
		"activeDeviceProfileIdsByTransport": map[string]string{
			types.DeviceTransportBLE: types.FlyDigiBS1ProfileID,
		},
		"deviceProfiles":          []types.DeviceProfile{types.DefaultWiFiPercentProfile(types.DefaultFanDeviceIP), types.FlyDigiBS1Profile()},
		"fanCurve":                curve,
		"fanCurveProfiles":        []types.FanCurveProfile{{ID: "bs1-custom", Name: "BS1", Curve: curve}},
		"activeFanCurveProfileId": "bs1-custom",
		"customSpeedRPM":          1800,
		"manualGearRpm":           manualGearRPM,
		"smartControl":            smartControl,
		"fanCurveProfilesByDevice": map[string]types.DeviceFanCurveProfilesState{
			deviceKey: {
				Profiles: []types.FanCurveProfile{{ID: "bs1-custom", Name: "BS1", Curve: curve}},
				ActiveID: "bs1-custom",
				FanCurve: curve,
			},
		},
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := NewManager(installDir, nil).Load(false)

	if cfg.DeviceTransport != types.DeviceTransportWiFi {
		t.Fatalf("device transport = %q, want compatibility wifi", cfg.DeviceTransport)
	}
	state := cfg.FanCurveProfilesByDevice[deviceKey]
	if len(state.FanCurve) != len(curve) || state.FanCurve[2].RPM != 2300 {
		t.Fatalf("native rpm curve state not preserved: %#v", state)
	}
	if got := state.ManualGearRPM["强劲"]["高"]; got != 4200 {
		t.Fatalf("native manual gear rpm not preserved: got %d, want 4200 in %#v", got, state.ManualGearRPM)
	}
	learningKey := deviceKey + "::curve::bs1-custom"
	if got := cfg.SmartControl.LearnedOffsetsByProfile[learningKey]; len(got) != 4 || got[2] != 7 {
		t.Fatalf("native learned offsets not preserved under %q: %#v", learningKey, cfg.SmartControl.LearnedOffsetsByProfile)
	}
	if len(cfg.FanCurve) > 0 && cfg.FanCurve[len(cfg.FanCurve)-1].RPM > types.FanSpeedMaxPercent {
		t.Fatalf("compatibility root curve should be normalized away from native rpm view: %#v", cfg.FanCurve)
	}
}

func TestLoadMigratesLegacyRootConfigJSONToInstallConfigDir(t *testing.T) {
	installDir := t.TempDir()

	curve := []types.FanCurvePoint{
		{Temperature: 30, RPM: 21},
		{Temperature: 55, RPM: 48},
		{Temperature: 80, RPM: 88},
	}
	raw := map[string]any{
		"deviceTransport":         types.DeviceTransportWiFi,
		"fanControlDeviceIp":      "10.9.0.55",
		"fanCurve":                curve,
		"fanCurveProfiles":        []types.FanCurveProfile{{ID: "legacy-root", Name: "Legacy root", Curve: curve}},
		"activeFanCurveProfileId": "legacy-root",
		"customSpeedRPM":          48,
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "config.json"), data, 0644); err != nil {
		t.Fatalf("write legacy root config: %v", err)
	}

	cfg := NewManager(installDir, nil).Load(false)

	if cfg.ConfigPath != filepath.Join(installDir, "config", "config.json") {
		t.Fatalf("ConfigPath = %q, want migrated install config path", cfg.ConfigPath)
	}
	if cfg.FanControlDeviceIp != "10.9.0.55" {
		t.Fatalf("FanControlDeviceIp = %q, want 10.9.0.55", cfg.FanControlDeviceIp)
	}
	if got := types.ActiveDeviceProfile(&cfg).Connection.Endpoint; got != "10.9.0.55" {
		t.Fatalf("active device endpoint = %q, want 10.9.0.55", got)
	}
	if cfg.ActiveFanCurveProfileID != "legacy-root" || len(cfg.FanCurveProfiles) != 1 || cfg.FanCurveProfiles[0].Curve[1].RPM != 48 {
		t.Fatalf("curve profile state not preserved: active=%q profiles=%#v", cfg.ActiveFanCurveProfileID, cfg.FanCurveProfiles)
	}
	if _, err := os.Stat(filepath.Join(installDir, "config", "config.json")); err != nil {
		t.Fatalf("migrated config file missing: %v", err)
	}
}

func TestLoadUpgradeConfigPreservesDeviceProfilesAndLearningState(t *testing.T) {
	installDir := t.TempDir()
	configDir := filepath.Join(installDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	curve := []types.FanCurvePoint{
		{Temperature: 30, RPM: 20},
		{Temperature: 50, RPM: 45},
		{Temperature: 70, RPM: 70},
		{Temperature: 90, RPM: 95},
	}
	wifi := types.DeviceProfile{
		ID:          "user.wifi.slim.custom",
		DisplayName: "User WiFi cooler",
		Vendor:      "DIY",
		Model:       "WiFi board",
		Transport:   types.DeviceTransportWiFi,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			Endpoint:          "10.8.0.77",
			StateEndpoint:     "/api/data",
			SpeedEndpoint:     "/api/speed",
			HTTPMethod:        "POST",
			MinSendIntervalMs: 350,
		},
		Commands: []types.DeviceCommandTemplate{
			{Name: "setSpeed", Command: `{"speed":{{percent}}}`, Encoding: "json"},
		},
		ResponseParsers: []types.DeviceResponseParser{
			{Name: "current", Type: "json_path", Expression: "$.speed"},
		},
		Capabilities: types.DeviceCapabilities{
			Transport:         types.DeviceTransportWiFi,
			SpeedUnit:         types.FanSpeedUnitPercent,
			SpeedRange:        types.DefaultPercentSpeedRange(),
			SupportsReadState: true,
			SupportsSetSpeed:  true,
		},
	}
	serial := types.DeviceProfile{
		ID:          "user.serial.loopback",
		DisplayName: "User serial cooler",
		Transport:   types.DeviceTransportSerial,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			SerialPort:           "COM42",
			SerialBaudRate:       57600,
			SerialDataBits:       8,
			SerialStopBits:       1,
			SerialParity:         "none",
			SerialFrameDelimiter: "\n",
		},
		Commands: []types.DeviceCommandTemplate{
			{Name: "setSpeed", Command: "SPD {{percent}}\n", Encoding: "ascii"},
		},
		Capabilities: types.DeviceCapabilities{
			Transport:        types.DeviceTransportSerial,
			SpeedUnit:        types.FanSpeedUnitPercent,
			SpeedRange:       types.DefaultPercentSpeedRange(),
			SupportsSetSpeed: true,
		},
	}
	cfg := types.GetDefaultConfig(false)
	cfg.DeviceTransport = types.DeviceTransportSerial
	cfg.WiFiCompatibilityEnabled = true
	cfg.SerialCompatibilityEnabled = true
	cfg.FanControlDeviceIp = "10.8.0.77"
	cfg.ActiveDeviceProfileID = serial.ID
	cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
		types.DeviceTransportWiFi:   wifi.ID,
		types.DeviceTransportSerial: serial.ID,
	}
	cfg.DeviceProfiles = []types.DeviceProfile{wifi, serial}
	cfg.FanCurve = curve
	cfg.FanCurveProfiles = []types.FanCurveProfile{
		{ID: "daily", Name: "Daily", Curve: curve[:3]},
		{ID: "gaming", Name: "Gaming", Curve: curve},
	}
	cfg.ActiveFanCurveProfileID = "gaming"
	cfg.CustomSpeedRPM = 52
	cfg.SmartControl.LearnedOffsets = []int{0, 5, 12, 18}
	cfg.SmartControl.LearnedOffsetsHeat = []int{0, 4, 10, 14}
	cfg.SmartControl.LearnedOffsetsCool = []int{0, -2, -5, -8}
	cfg.SmartControl.LearnedRateHeat = []int{1, 2, 3, 4, 5, 6, 7}
	cfg.SmartControl.LearnedRateCool = []int{-1, -2, -3, -4, -5, -6, -7}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	loaded := NewManager(installDir, nil).Load(false)

	if loaded.DeviceTransport != types.DeviceTransportSerial || loaded.ActiveDeviceProfileID != serial.ID {
		t.Fatalf("active transport/profile = %q/%q, want serial/%q", loaded.DeviceTransport, loaded.ActiveDeviceProfileID, serial.ID)
	}
	if loaded.FanControlDeviceIp != "10.8.0.77" {
		t.Fatalf("FanControlDeviceIp = %q, want 10.8.0.77", loaded.FanControlDeviceIp)
	}
	if loaded.ActiveDeviceProfileIDsByTransport[types.DeviceTransportWiFi] != wifi.ID {
		t.Fatalf("active WiFi device = %q, want %q", loaded.ActiveDeviceProfileIDsByTransport[types.DeviceTransportWiFi], wifi.ID)
	}
	if loaded.ActiveDeviceProfileIDsByTransport[types.DeviceTransportSerial] != serial.ID {
		t.Fatalf("active serial device = %q, want %q", loaded.ActiveDeviceProfileIDsByTransport[types.DeviceTransportSerial], serial.ID)
	}
	loadedWiFi, ok := findDeviceProfileForTest(loaded.DeviceProfiles, wifi.ID)
	if !ok {
		t.Fatalf("WiFi device profile %q not preserved: %#v", wifi.ID, loaded.DeviceProfiles)
	}
	loadedSerial, ok := findDeviceProfileForTest(loaded.DeviceProfiles, serial.ID)
	if !ok {
		t.Fatalf("serial device profile %q not preserved: %#v", serial.ID, loaded.DeviceProfiles)
	}
	if loadedWiFi.Connection.Endpoint != "10.8.0.77" || loadedWiFi.Connection.MinSendIntervalMs != 350 {
		t.Fatalf("WiFi profile connection not preserved: %#v", loadedWiFi.Connection)
	}
	if loadedSerial.Connection.SerialPort != "COM42" || loadedSerial.Connection.SerialBaudRate != 57600 {
		t.Fatalf("serial profile connection not preserved: %#v", loadedSerial.Connection)
	}
	for _, id := range []string{types.FlyDigiBS1ProfileID, types.FlyDigiBS2ProfileID, types.FlyDigiBS2PROProfileID, types.FlyDigiBS3ProfileID, types.FlyDigiBS3PROProfileID} {
		if _, ok := findDeviceProfileForTest(loaded.DeviceProfiles, id); !ok {
			t.Fatalf("FlyDigi built-in profile %q should be appended during upgrade: %#v", id, loaded.DeviceProfiles)
		}
	}
	if profile, ok := findDeviceProfileForTest(loaded.DeviceProfiles, types.LegacyRPMProfileID); ok {
		t.Fatalf("legacy RPM profile should not be exposed as a saved device after upgrade: %#v", profile)
	}
	if loaded.ActiveFanCurveProfileID != "gaming" || len(loaded.FanCurveProfiles) != 2 || loaded.FanCurveProfiles[1].Curve[2].RPM != 70 {
		t.Fatalf("curve profile state not preserved: active=%q profiles=%#v", loaded.ActiveFanCurveProfileID, loaded.FanCurveProfiles)
	}
	if len(loaded.SmartControl.LearnedOffsets) != 4 || loaded.SmartControl.LearnedOffsets[2] != 12 {
		t.Fatalf("learned offsets not preserved: %#v", loaded.SmartControl.LearnedOffsets)
	}
	if loaded.SmartControl.LearnedRateCool[6] != -7 {
		t.Fatalf("learned rate cool not preserved: %#v", loaded.SmartControl.LearnedRateCool)
	}
}

func TestLoadAppearanceAndPredictionDefaultsPreserveExplicitChoices(t *testing.T) {
	writeConfig := func(t *testing.T, cfg map[string]any) string {
		t.Helper()
		installDir := t.TempDir()
		configDir := filepath.Join(installDir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("mkdir config dir: %v", err)
		}
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("marshal config: %v", err)
		}
		if err := os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		return installDir
	}

	defaultMap := func(t *testing.T) map[string]any {
		t.Helper()
		data, err := json.Marshal(types.GetDefaultConfig(false))
		if err != nil {
			t.Fatalf("marshal defaults: %v", err)
		}
		var cfg map[string]any
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("unmarshal defaults: %v", err)
		}
		return cfg
	}

	t.Run("missing fields receive new defaults", func(t *testing.T) {
		cfg := defaultMap(t)
		delete(cfg, "windowBlur")
		smart := cfg["smartControl"].(map[string]any)
		delete(smart, "temperatureRisePrediction")

		loaded := NewManager(writeConfig(t, cfg), nil).Load(false)
		if loaded.WindowBlur != "acrylic" {
			t.Fatalf("WindowBlur = %q, want acrylic", loaded.WindowBlur)
		}
		if !loaded.SmartControl.TemperatureRisePrediction {
			t.Fatal("missing temperatureRisePrediction should migrate to enabled")
		}
	})

	t.Run("explicit choices survive upgrade", func(t *testing.T) {
		cfg := defaultMap(t)
		cfg["windowBlur"] = "mica"
		smart := cfg["smartControl"].(map[string]any)
		smart["temperatureRisePrediction"] = false

		loaded := NewManager(writeConfig(t, cfg), nil).Load(false)
		if loaded.WindowBlur != "mica" {
			t.Fatalf("WindowBlur = %q, want mica", loaded.WindowBlur)
		}
		if loaded.SmartControl.TemperatureRisePrediction {
			t.Fatal("explicit disabled temperatureRisePrediction should be preserved")
		}
	})
}

func TestManagerConfigSnapshotsDoNotShareMutableState(t *testing.T) {
	manager := NewManager(t.TempDir(), nil)
	cfg := types.GetDefaultConfig(false)
	cfg.ActiveDeviceProfileIDsByTransport[types.DeviceTransportWiFi] = "before"
	cfg.ManualGearRPM["test"] = map[string]int{"low": 1111}
	cfg.FanCurve[0].RPM = 21
	cfg.FanCurveProfiles[0].Curve = append([]types.FanCurvePoint{}, cfg.FanCurveProfiles[0].Curve...)
	cfg.FanCurveProfiles[0].Curve[0].RPM = 22
	cfg.SmartControl.LearnedOffsets[0] = 23
	cfg.SmartControl.LearnedOffsetsByProfile = map[string][]int{"profile": {24}}
	cfg.AxisNoiseProfilesByDevice = map[string]types.AxisNoiseProfile{
		"hid::flydigi.bs3": {DeviceKey: "hid::flydigi.bs3", Points: []types.AxisNoisePoint{{Actual: 2000, Severity: types.AxisNoiseSeverityMild}}},
	}
	cfg.LightStrip.Colors[0].R = 25
	cfg.LegionFnQ.ModeMapping["Quiet"] = types.FanGearTarget{Gear: "quiet", Level: "low"}
	manager.Set(cfg)

	cfg.ActiveDeviceProfileIDsByTransport[types.DeviceTransportWiFi] = "mutated-input"
	cfg.ManualGearRPM["test"]["low"] = 9000
	cfg.FanCurve[0].RPM = 90
	cfg.FanCurveProfiles[0].Curve[0].RPM = 91
	cfg.SmartControl.LearnedOffsets[0] = 92
	cfg.SmartControl.LearnedOffsetsByProfile["profile"][0] = 93
	cfg.AxisNoiseProfilesByDevice["hid::flydigi.bs3"].Points[0].Actual = 9000
	cfg.LightStrip.Colors[0].R = 94
	cfg.LegionFnQ.ModeMapping["Quiet"] = types.FanGearTarget{Gear: "mutated", Level: "high"}

	first := manager.Get()
	if first.ActiveDeviceProfileIDsByTransport[types.DeviceTransportWiFi] != "before" ||
		first.ManualGearRPM["test"]["low"] != 1111 ||
		first.FanCurve[0].RPM != 21 ||
		first.FanCurveProfiles[0].Curve[0].RPM != 22 ||
		first.SmartControl.LearnedOffsets[0] != 23 ||
		first.SmartControl.LearnedOffsetsByProfile["profile"][0] != 24 ||
		first.AxisNoiseProfilesByDevice["hid::flydigi.bs3"].Points[0].Actual != 2000 ||
		first.LightStrip.Colors[0].R != 25 ||
		first.LegionFnQ.ModeMapping["Quiet"].Gear != "quiet" {
		t.Fatalf("manager retained mutable input references: %#v", first)
	}

	first.ActiveDeviceProfileIDsByTransport[types.DeviceTransportWiFi] = "mutated-output"
	first.ManualGearRPM["test"]["low"] = 8000
	first.FanCurve[0].RPM = 80
	first.FanCurveProfiles[0].Curve[0].RPM = 81
	first.SmartControl.LearnedOffsets[0] = 82
	first.SmartControl.LearnedOffsetsByProfile["profile"][0] = 83
	first.AxisNoiseProfilesByDevice["hid::flydigi.bs3"].Points[0].Actual = 8000
	first.LightStrip.Colors[0].R = 84
	first.LegionFnQ.ModeMapping["Quiet"] = types.FanGearTarget{Gear: "output", Level: "high"}

	second := manager.Get()
	if second.ActiveDeviceProfileIDsByTransport[types.DeviceTransportWiFi] != "before" ||
		second.ManualGearRPM["test"]["low"] != 1111 ||
		second.FanCurve[0].RPM != 21 ||
		second.FanCurveProfiles[0].Curve[0].RPM != 22 ||
		second.SmartControl.LearnedOffsets[0] != 23 ||
		second.SmartControl.LearnedOffsetsByProfile["profile"][0] != 24 ||
		second.AxisNoiseProfilesByDevice["hid::flydigi.bs3"].Points[0].Actual != 2000 ||
		second.LightStrip.Colors[0].R != 25 ||
		second.LegionFnQ.ModeMapping["Quiet"].Gear != "quiet" {
		t.Fatalf("manager exposed mutable snapshot references: %#v", second)
	}
}

func TestManagerMutateIfRevisionRejectsStaleConfigAndPreservesNewerFields(t *testing.T) {
	manager := NewManager(t.TempDir(), nil)
	cfg := types.GetDefaultConfig(false)
	manager.Set(cfg)
	stale, staleRevision := manager.GetWithRevision()

	newer := stale
	newer.ThemeMode = "new-theme"
	newer.CpuSensor = "new-sensor"
	manager.Set(newer)

	_, _, applied := manager.MutateIfRevision(staleRevision, func(current *types.AppConfig) {
		current.SmartControl.LearnedOffsets[0] = 99
	})
	if applied {
		t.Fatal("stale config mutation was applied")
	}
	afterStale := manager.Get()
	if afterStale.ThemeMode != "new-theme" || afterStale.CpuSensor != "new-sensor" || afterStale.SmartControl.LearnedOffsets[0] == 99 {
		t.Fatalf("stale mutation overwrote newer config: %#v", afterStale)
	}

	_, currentRevision := manager.GetWithRevision()
	updated, nextRevision, applied := manager.MutateIfRevision(currentRevision, func(current *types.AppConfig) {
		current.SmartControl.LearnedOffsets[0] = 77
	})
	if !applied || nextRevision <= currentRevision {
		t.Fatalf("current mutation applied=%v revision=%d, want revision > %d", applied, nextRevision, currentRevision)
	}
	if updated.ThemeMode != "new-theme" || updated.CpuSensor != "new-sensor" || updated.SmartControl.LearnedOffsets[0] != 77 {
		t.Fatalf("atomic mutation did not preserve unrelated fields: %#v", updated)
	}
}
