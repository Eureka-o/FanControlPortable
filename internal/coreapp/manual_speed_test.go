package coreapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/device"
	"github.com/TIANLI0/THRM/internal/types"
)

func newManualSpeedTestApp(t *testing.T, cfg types.AppConfig) *CoreApp {
	t.Helper()
	manager := config.NewManager(t.TempDir(), nil)
	manager.Set(cfg)
	deviceManager := device.NewManager(nil)
	return &CoreApp{
		configManager: manager,
		deviceManager: deviceManager,
	}
}

func TestApplyCurrentGearSettingSendsConfiguredManualSpeedWithoutFanData(t *testing.T) {
	var postedSpeed int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/data":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"speed":           44,
				"targetSpeed":     44,
				"wifiTargetSpeed": 44,
				"wifiControl":     true,
				"controlMode":     "wifi",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/speed":
			var payload struct {
				Speed int `json:"speed"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode speed payload: %v", err)
			}
			postedSpeed = payload.Speed
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":      "success",
				"controlMode": "wifi",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := types.GetDefaultConfig(false)
	cfg.FanControlDeviceIp = server.URL
	cfg.DeviceTransport = types.DeviceTransportWiFi
	cfg.ActiveDeviceProfileID = types.DefaultWiFiPercentProfileID
	cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
		types.DeviceTransportWiFi: types.DefaultWiFiPercentProfileID,
	}
	cfg.DeviceProfiles = []types.DeviceProfile{types.DefaultWiFiPercentProfile(server.URL)}
	cfg.AutoControl = false
	cfg.CustomSpeedEnabled = false
	cfg.ManualGearRPM[cfg.ManualGear][cfg.ManualLevel] = 75
	types.NormalizeDeviceProfileConfig(&cfg)

	app := newManualSpeedTestApp(t, cfg)
	app.configureDeviceManager(cfg)
	success, _ := app.deviceManager.Connect()
	if !success {
		t.Fatal("expected WiFi test device to connect")
	}

	if err := app.applyCurrentGearSetting(); err != nil {
		t.Fatalf("applyCurrentGearSetting() error = %v", err)
	}

	if postedSpeed != 75 {
		t.Fatalf("posted speed = %d, want 75", postedSpeed)
	}
}

func TestSetAutoControlOffAppliesConfiguredManualSpeed(t *testing.T) {
	var postedSpeed int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/data":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"speed":           44,
				"targetSpeed":     44,
				"wifiTargetSpeed": 44,
				"wifiControl":     true,
				"controlMode":     "wifi",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/speed":
			var payload struct {
				Speed int `json:"speed"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode speed payload: %v", err)
			}
			postedSpeed = payload.Speed
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":      "success",
				"controlMode": "wifi",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := types.GetDefaultConfig(false)
	cfg.FanControlDeviceIp = server.URL
	cfg.DeviceTransport = types.DeviceTransportWiFi
	cfg.ActiveDeviceProfileID = types.DefaultWiFiPercentProfileID
	cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
		types.DeviceTransportWiFi: types.DefaultWiFiPercentProfileID,
	}
	cfg.DeviceProfiles = []types.DeviceProfile{types.DefaultWiFiPercentProfile(server.URL)}
	cfg.AutoControl = true
	cfg.CustomSpeedEnabled = false
	cfg.ManualGear = "强劲"
	cfg.ManualLevel = "高"
	cfg.ManualGearRPM["强劲"]["高"] = 85
	types.NormalizeDeviceProfileConfig(&cfg)

	app := newManualSpeedTestApp(t, cfg)
	app.configureDeviceManager(cfg)
	success, _ := app.deviceManager.Connect()
	if !success {
		t.Fatal("expected WiFi test device to connect")
	}
	app.isConnected = true

	if err := app.SetAutoControl(false); err != nil {
		t.Fatalf("SetAutoControl(false) error = %v", err)
	}

	got := app.configManager.Get()
	if got.AutoControl {
		t.Fatal("AutoControl remained enabled")
	}
	if postedSpeed != 85 {
		t.Fatalf("posted speed = %d, want 85", postedSpeed)
	}
}

func TestSetAutoControlOffDoesNotPersistWhenManualApplyFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/data":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"speed":           44,
				"targetSpeed":     44,
				"wifiTargetSpeed": 44,
				"wifiControl":     true,
				"controlMode":     "wifi",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/speed":
			http.Error(w, "device rejected speed", http.StatusConflict)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := types.GetDefaultConfig(false)
	cfg.FanControlDeviceIp = server.URL
	cfg.DeviceTransport = types.DeviceTransportWiFi
	cfg.ActiveDeviceProfileID = types.DefaultWiFiPercentProfileID
	cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
		types.DeviceTransportWiFi: types.DefaultWiFiPercentProfileID,
	}
	cfg.DeviceProfiles = []types.DeviceProfile{types.DefaultWiFiPercentProfile(server.URL)}
	cfg.AutoControl = true
	cfg.CustomSpeedEnabled = false
	cfg.ManualGear = "强劲"
	cfg.ManualLevel = "高"
	cfg.ManualGearRPM["强劲"]["高"] = 85
	types.NormalizeDeviceProfileConfig(&cfg)

	app := newManualSpeedTestApp(t, cfg)
	app.configureDeviceManager(cfg)
	success, _ := app.deviceManager.Connect()
	if !success {
		t.Fatal("expected WiFi test device to connect")
	}
	app.isConnected = true

	if err := app.SetAutoControl(false); err == nil {
		t.Fatal("SetAutoControl(false) returned nil for rejected manual apply")
	}

	got := app.configManager.Get()
	if !got.AutoControl {
		t.Fatal("AutoControl was disabled even though manual apply failed")
	}
}

func TestSetManualGearDisablesCompetingModesAndSendsConfiguredSpeed(t *testing.T) {
	var postedSpeed int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/data":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"speed":           44,
				"targetSpeed":     44,
				"wifiTargetSpeed": 44,
				"wifiControl":     true,
				"controlMode":     "wifi",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/speed":
			var payload struct {
				Speed int `json:"speed"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode speed payload: %v", err)
			}
			postedSpeed = payload.Speed
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":      "success",
				"controlMode": "wifi",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := types.GetDefaultConfig(false)
	cfg.FanControlDeviceIp = server.URL
	cfg.DeviceTransport = types.DeviceTransportWiFi
	cfg.ActiveDeviceProfileID = types.DefaultWiFiPercentProfileID
	cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
		types.DeviceTransportWiFi: types.DefaultWiFiPercentProfileID,
	}
	cfg.DeviceProfiles = []types.DeviceProfile{types.DefaultWiFiPercentProfile(server.URL)}
	cfg.AutoControl = true
	cfg.CustomSpeedEnabled = true
	cfg.ManualGearRPM["强劲"]["高"] = 85
	types.NormalizeDeviceProfileConfig(&cfg)

	app := newManualSpeedTestApp(t, cfg)
	app.configureDeviceManager(cfg)
	success, _ := app.deviceManager.Connect()
	if !success {
		t.Fatal("expected WiFi test device to connect")
	}

	if ok := app.SetManualGear("强劲", "高"); !ok {
		t.Fatal("SetManualGear() returned false")
	}

	got := app.configManager.Get()
	if got.AutoControl {
		t.Fatal("AutoControl remained enabled after SetManualGear")
	}
	if got.CustomSpeedEnabled {
		t.Fatal("CustomSpeedEnabled remained enabled after SetManualGear")
	}
	if got.ManualGear != "强劲" || got.ManualLevel != "高" {
		t.Fatalf("manual gear = %s/%s, want 强劲/高", got.ManualGear, got.ManualLevel)
	}
	if postedSpeed != 85 {
		t.Fatalf("posted speed = %d, want 85", postedSpeed)
	}
}

func TestSetManualGearDoesNotPersistWhenConnectedDeviceRejectsCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/data":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"speed":           44,
				"targetSpeed":     44,
				"wifiTargetSpeed": 44,
				"wifiControl":     true,
				"controlMode":     "wifi",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/speed":
			http.Error(w, "device rejected speed", http.StatusConflict)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := types.GetDefaultConfig(false)
	cfg.FanControlDeviceIp = server.URL
	cfg.DeviceTransport = types.DeviceTransportWiFi
	cfg.ActiveDeviceProfileID = types.DefaultWiFiPercentProfileID
	cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
		types.DeviceTransportWiFi: types.DefaultWiFiPercentProfileID,
	}
	cfg.DeviceProfiles = []types.DeviceProfile{types.DefaultWiFiPercentProfile(server.URL)}
	cfg.AutoControl = true
	cfg.CustomSpeedEnabled = true
	cfg.ManualGear = "静音"
	cfg.ManualLevel = "低"
	cfg.ManualGearRPM["强劲"]["高"] = 85
	types.NormalizeDeviceProfileConfig(&cfg)

	app := newManualSpeedTestApp(t, cfg)
	app.configureDeviceManager(cfg)
	success, _ := app.deviceManager.Connect()
	if !success {
		t.Fatal("expected WiFi test device to connect")
	}
	app.isConnected = true

	if ok := app.SetManualGear("强劲", "高"); ok {
		t.Fatal("SetManualGear() returned true for rejected device command")
	}

	got := app.configManager.Get()
	if !got.AutoControl {
		t.Fatal("AutoControl was disabled even though manual gear was rejected")
	}
	if !got.CustomSpeedEnabled {
		t.Fatal("CustomSpeedEnabled was disabled even though manual gear was rejected")
	}
	if got.ManualGear != "静音" || got.ManualLevel != "低" {
		t.Fatalf("manual gear persisted as %s/%s, want original 静音/低", got.ManualGear, got.ManualLevel)
	}
}

func manualGearUnsupportedWiFiProfile(endpoint string) types.DeviceProfile {
	profile := types.DefaultWiFiPercentProfile(endpoint)
	profile.ID = "test.wifi.no-manual-gears"
	profile.DisplayName = "WiFi without manual gears"
	profile.BuiltIn = false
	profile.Capabilities.SupportsSetSpeed = false
	profile.Capabilities.SupportsManualGears = false
	profile.Capabilities.SupportsCustomSpeed = false
	return types.NormalizeDeviceProfile(profile, endpoint)
}

func TestSetManualGearRejectsUnsupportedActiveDevice(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	profile := manualGearUnsupportedWiFiProfile("http://127.0.0.1")
	cfg.DeviceTransport = types.DeviceTransportWiFi
	cfg.ActiveDeviceProfileID = profile.ID
	cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
		types.DeviceTransportWiFi: profile.ID,
	}
	cfg.DeviceProfiles = []types.DeviceProfile{profile}
	cfg.AutoControl = true
	cfg.CustomSpeedEnabled = true
	cfg.ManualGear = "静音"
	cfg.ManualLevel = "低"
	cfg.ManualGearRPM["强劲"]["高"] = 85
	types.NormalizeDeviceProfileConfig(&cfg)

	app := newManualSpeedTestApp(t, cfg)
	app.configureDeviceManager(cfg)

	if ok := app.SetManualGear("强劲", "高"); ok {
		t.Fatal("SetManualGear() returned true for device without manual gear support")
	}

	got := app.configManager.Get()
	if !got.AutoControl {
		t.Fatal("AutoControl was disabled even though manual gears are unsupported")
	}
	if !got.CustomSpeedEnabled {
		t.Fatal("CustomSpeedEnabled was disabled even though manual gears are unsupported")
	}
	if got.ManualGear != "静音" || got.ManualLevel != "低" {
		t.Fatalf("manual gear persisted as %s/%s, want original 静音/低", got.ManualGear, got.ManualLevel)
	}
}

func TestSetAutoControlOffRejectsUnsupportedManualFallback(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	profile := manualGearUnsupportedWiFiProfile("http://127.0.0.1")
	cfg.DeviceTransport = types.DeviceTransportWiFi
	cfg.ActiveDeviceProfileID = profile.ID
	cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
		types.DeviceTransportWiFi: profile.ID,
	}
	cfg.DeviceProfiles = []types.DeviceProfile{profile}
	cfg.AutoControl = true
	cfg.CustomSpeedEnabled = false
	cfg.ManualGear = "标准"
	cfg.ManualLevel = "中"
	types.NormalizeDeviceProfileConfig(&cfg)

	app := newManualSpeedTestApp(t, cfg)
	app.configureDeviceManager(cfg)
	app.isConnected = true

	if err := app.SetAutoControl(false); err == nil {
		t.Fatal("SetAutoControl(false) returned nil for device without manual fallback")
	}

	got := app.configManager.Get()
	if !got.AutoControl {
		t.Fatal("AutoControl was disabled even though manual fallback is unsupported")
	}
}
