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

	app.applyCurrentGearSetting()

	if postedSpeed != 75 {
		t.Fatalf("posted speed = %d, want 75", postedSpeed)
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
