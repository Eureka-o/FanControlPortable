package device

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestWiFiSetFanSpeedUsesPercentProtocolAndReadsBackState(t *testing.T) {
	var postedSpeed int
	getCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/data":
			if r.Method != http.MethodGet {
				t.Fatalf("GET /api/data got method %s", r.Method)
			}
			getCount++
			speed := 30
			target := 30
			if getCount > 1 {
				speed = 55
				target = 55
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"speed":           speed,
				"temperature":     42,
				"power":           true,
				"wifiControl":     true,
				"wifiTargetSpeed": target,
				"controlMode":     "wifi",
			})
		case "/api/speed":
			if r.Method != http.MethodPost {
				t.Fatalf("POST /api/speed got method %s", r.Method)
			}
			var payload map[string]int
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode speed payload: %v", err)
			}
			postedSpeed = payload["speed"]
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status":      "success",
				"controlMode": "wifi",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	m := NewManager(nil)
	m.Configure(types.DeviceTransportWiFi, server.URL)

	connected, _ := m.Connect()
	if !connected {
		t.Fatal("expected WiFi test device to connect")
	}

	if ok := m.SetFanSpeed(55); !ok {
		t.Fatal("expected SetFanSpeed to succeed")
	}
	if postedSpeed != 55 {
		t.Fatalf("posted speed = %d, want 55", postedSpeed)
	}
	if getCount != 2 {
		t.Fatalf("GET /api/data count = %d, want 2", getCount)
	}
	fanData := m.GetCurrentFanData()
	if fanData == nil {
		t.Fatal("expected fan data")
	}
	if fanData.CurrentRPM != 55 || fanData.TargetRPM != 55 {
		t.Fatalf("fan data speed = current %d target %d, want 55/55", fanData.CurrentRPM, fanData.TargetRPM)
	}
	if fanData.SpeedUnit != types.FanSpeedUnitPercent || fanData.Transport != types.DeviceTransportWiFi {
		t.Fatalf("fan data unit/transport = %q/%q", fanData.SpeedUnit, fanData.Transport)
	}
}

func TestWiFiSetFanSpeedRejectsDeviceFailureResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/data":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"speed":           30,
				"wifiTargetSpeed": 30,
				"controlMode":     "manual",
			})
		case "/api/speed":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": "rejected",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	m := NewManager(nil)
	m.Configure(types.DeviceTransportWiFi, server.URL)
	connected, _ := m.Connect()
	if !connected {
		t.Fatal("expected WiFi test device to connect")
	}

	if ok := m.SetFanSpeed(55); ok {
		t.Fatal("expected SetFanSpeed to reject error response")
	}
	fanData := m.GetCurrentFanData()
	if fanData == nil || fanData.CurrentRPM != 30 {
		t.Fatalf("fan data changed after rejected command: %#v", fanData)
	}
}

func TestRefreshWiFiStateReadsOnlyAndUpdatesFanData(t *testing.T) {
	getCount := 0
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/data":
			if r.Method != http.MethodGet {
				t.Fatalf("GET /api/data got method %s", r.Method)
			}
			getCount++
			speed := 30
			target := 30
			if getCount > 1 {
				speed = 42
				target = 55
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"speed":           speed,
				"targetSpeed":     target,
				"wifiTargetSpeed": target,
				"wifiControl":     true,
				"controlMode":     "wifi",
			})
		case "/api/speed":
			postCount++
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status":      "success",
				"controlMode": "wifi",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	m := NewManager(nil)
	m.Configure(types.DeviceTransportWiFi, server.URL)
	connected, _ := m.Connect()
	if !connected {
		t.Fatal("expected WiFi test device to connect")
	}

	if ok := m.RefreshWiFiState(); !ok {
		t.Fatal("RefreshWiFiState() returned false")
	}

	if postCount != 0 {
		t.Fatalf("POST /api/speed count = %d, want 0", postCount)
	}
	if getCount != 2 {
		t.Fatalf("GET /api/data count = %d, want 2", getCount)
	}
	fanData := m.GetCurrentFanData()
	if fanData == nil {
		t.Fatal("expected fan data")
	}
	if fanData.CurrentRPM != 42 || fanData.TargetRPM != 55 {
		t.Fatalf("fan data speed = current %d target %d, want 42/55", fanData.CurrentRPM, fanData.TargetRPM)
	}
}

func TestWiFiStateUsesFanSpeedAsCurrentWhenSpeedIsSetpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/data":
			if r.Method != http.MethodGet {
				t.Fatalf("GET /api/data got method %s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"speed":       75,
				"fanSpeed":    42,
				"controlMode": "manual",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	m := NewManager(nil)
	m.Configure(types.DeviceTransportWiFi, server.URL)
	connected, _ := m.Connect()
	if !connected {
		t.Fatal("expected WiFi test device to connect")
	}

	fanData := m.GetCurrentFanData()
	if fanData == nil {
		t.Fatal("expected fan data")
	}
	if fanData.CurrentRPM != 42 || fanData.TargetRPM != 75 {
		t.Fatalf("fan data speed = current %d target %d, want 42/75", fanData.CurrentRPM, fanData.TargetRPM)
	}
}

func TestWiFiConnectRejectsGenericJSONState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/data":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"temperature": 42,
				"power":       18,
				"mode":        "online",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	m := NewManager(nil)
	m.Configure(types.DeviceTransportWiFi, server.URL)

	connected, _ := m.Connect()
	if connected {
		t.Fatal("generic JSON endpoint should not connect as a WiFi fan controller")
	}
	if m.IsConnected() {
		t.Fatal("manager should stay disconnected after rejected WiFi state")
	}
}

func TestWiFiManagerRejectsNonSpeedFeatureCommandsByDefault(t *testing.T) {
	m := NewManager(nil)
	m.Configure(types.DeviceTransportWiFi, "192.168.1.50")

	if m.SetGearLight(true) {
		t.Fatal("default WiFi profile must not report gear-light support")
	}
	if m.SetPowerOnStart(true) {
		t.Fatal("default WiFi profile must not report power-on-start support")
	}
	if m.SetSmartStartStop("immediate") {
		t.Fatal("default WiFi profile must not report smart start/stop support")
	}
	if m.SetBrightness(80) {
		t.Fatal("default WiFi profile must not report lighting brightness support")
	}
	if err := m.SetLightStrip(types.GetDefaultLightStripConfig()); err == nil {
		t.Fatal("default WiFi profile must not report light-strip support")
	}
	if m.SetRGBOff() {
		t.Fatal("default WiFi profile must not report RGB support")
	}
}

func TestWiFiManagerConfigureProfileUsesCustomPercentRuntime(t *testing.T) {
	var posted struct {
		Percent int `json:"percent"`
		Ticks   int `json:"ticks"`
	}
	setSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/custom/state":
			speed := 25
			if setSeen {
				speed = 56
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"fan": map[string]any{
					"current": speed,
					"target":  speed,
				},
			})
		case "/custom/speed":
			if r.Method != http.MethodPut {
				t.Fatalf("custom speed method = %s, want PUT", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
				t.Fatalf("decode posted body: %v", err)
			}
			setSeen = true
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case "/api/data", "/api/speed":
			t.Fatalf("default WiFi endpoint should not be used: %s", r.URL.Path)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	profile := types.DeviceProfile{
		ID:          "user.manager.percent",
		DisplayName: "Manager custom percent",
		Vendor:      "DIY",
		Transport:   types.DeviceTransportWiFi,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			Endpoint:      server.URL,
			StateEndpoint: "/custom/state",
			SpeedEndpoint: "/custom/speed",
			HTTPMethod:    http.MethodPut,
		},
		Commands: []types.DeviceCommandTemplate{
			{Name: "setSpeed", Command: `{"percent":{{percent}},"ticks":{{percentTicks}}}`, Encoding: "json"},
		},
		ResponseParsers: []types.DeviceResponseParser{
			{Name: "current", Type: "json_path", Expression: "$.fan.current"},
			{Name: "target", Type: "json_path", Expression: "$.fan.target"},
		},
		Capabilities: types.DefaultWiFiPercentCapabilities(),
	}

	m := NewManager(nil)
	m.ConfigureProfile(profile, "")
	connected, info := m.Connect()
	if !connected {
		t.Fatal("expected custom WiFi profile to connect")
	}
	if info["product"] != profile.DisplayName || info["model"] != profile.DisplayName || info["manufacturer"] != profile.Vendor {
		t.Fatalf("connection info = %#v, want enabled WiFi user device name/vendor", info)
	}
	if got := m.GetModelName(); got != profile.DisplayName {
		t.Fatalf("model name = %q, want enabled WiFi user device name %q", got, profile.DisplayName)
	}
	settings, err := m.QueryDeviceSettings()
	if err != nil {
		t.Fatalf("QueryDeviceSettings() error = %v", err)
	}
	if settings.Source != types.DeviceTransportWiFi || settings.Model != profile.DisplayName {
		t.Fatalf("settings = %#v, want WiFi source and enabled user device name", settings)
	}

	if ok := m.SetTargetSpeed(555, types.FanSpeedUnitPercent); !ok {
		t.Fatal("expected SetTargetSpeed to use custom profile runtime")
	}
	if posted.Percent != 56 || posted.Ticks != 555 {
		t.Fatalf("posted body = %+v, want percent 56 ticks 555", posted)
	}
	fanData := m.GetCurrentFanData()
	if fanData == nil || fanData.CurrentRPM != 56 || fanData.TargetRPM != 56 {
		t.Fatalf("fan data = %#v, want 56/56", fanData)
	}
}

func TestWiFiManagerConfigureProfileAllowsRPMRuntime(t *testing.T) {
	var postedRPM int
	setSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rpm/state":
			rpm := 1200
			if setSeen {
				rpm = postedRPM
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"fan": map[string]any{"rpm": rpm}})
		case "/rpm/set":
			var payload map[string]int
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode rpm body: %v", err)
			}
			postedRPM = payload["rpm"]
			setSeen = true
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	m := NewManager(nil)
	m.ConfigureProfile(types.DeviceProfile{
		ID:          "user.manager.rpm",
		DisplayName: "Manager custom RPM",
		Transport:   types.DeviceTransportWiFi,
		SpeedUnit:   types.FanSpeedUnitRPM,
		SpeedRange:  types.DefaultRPMSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			Endpoint:      server.URL,
			StateEndpoint: "/rpm/state",
			SpeedEndpoint: "/rpm/set",
			HTTPMethod:    http.MethodPost,
		},
		Commands: []types.DeviceCommandTemplate{
			{Name: "setSpeed", Command: `{"rpm":{{rpm}}}`, Encoding: "json"},
		},
		ResponseParsers: []types.DeviceResponseParser{
			{Name: "current rpm", Type: "json_path", Expression: "$.fan.rpm"},
		},
		Capabilities: types.DeviceCapabilities{
			Transport:         types.DeviceTransportWiFi,
			SpeedUnit:         types.FanSpeedUnitRPM,
			SpeedRange:        types.DefaultRPMSpeedRange(),
			SupportsReadState: true,
			SupportsSetSpeed:  true,
		},
	}, "")

	connected, _ := m.Connect()
	if !connected {
		t.Fatal("expected custom RPM WiFi profile to connect")
	}
	if ok := m.SetTargetSpeed(1800, types.FanSpeedUnitRPM); !ok {
		t.Fatal("expected RPM WiFi profile target speed to be sent")
	}
	if postedRPM != 1800 {
		t.Fatalf("posted RPM = %d, want 1800", postedRPM)
	}
	fanData := m.GetCurrentFanData()
	if fanData == nil || fanData.CurrentRPM != 1800 || fanData.SpeedUnit != types.FanSpeedUnitRPM {
		t.Fatalf("fan data = %#v, want 1800 RPM", fanData)
	}
}
