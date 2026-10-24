//go:build !legacydevice

package device

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func newBlackSharkWiFiMockManager(endpoint string, client *http.Client) *Manager {
	m := NewManager(nil)
	m.activeProfile = types.BlackSharkBRB02WiFiProfile()
	m.deviceTransport = types.DeviceTransportWiFi
	m.deviceType = types.DeviceTransportWiFi
	m.wifiEndpoint = endpoint
	m.wifiHTTPClient = client
	m.isConnected = true
	return m
}

func TestBlackSharkWiFiMockLightingCommandsUseDedicatedRoutes(t *testing.T) {
	seen := make(map[string]map[string]any)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if len(body) > 0 && json.Unmarshal(body, &payload) != nil {
			t.Fatalf("invalid payload %q", body)
		}
		seen[r.URL.Path] = payload
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	m := newBlackSharkWiFiMockManager(server.URL, server.Client())
	if !m.SetGearLight(true) || !m.SetBrightness(64) {
		t.Fatal("black shark WiFi lighting commands should succeed")
	}
	if err := m.SetLightStrip(types.LightStripConfig{Mode: "breathing", Brightness: 64}); err != nil {
		t.Fatal(err)
	}
	if !m.SetRGBOff() {
		t.Fatal("black shark WiFi RGB-off command should succeed")
	}
	if seen["/api/gear-light"]["enabled"] != true || seen["/api/brightness"]["brightness"] != float64(64) {
		t.Fatalf("lighting payloads = %#v", seen)
	}
	if seen["/api/lighting"]["mode"] != "breathing" {
		t.Fatalf("lighting payload = %#v", seen["/api/lighting"])
	}
}

func TestBlackSharkWiFiMockSpeedUsesRPMPayload(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/speed" {
			t.Fatalf("path = %s, want /api/speed", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	m := newBlackSharkWiFiMockManager(server.URL, server.Client())
	if !m.SetTargetSpeed(2800, types.FanSpeedUnitRPM) {
		t.Fatal("black shark WiFi RPM command should succeed")
	}
	if payload["rpm"] != float64(2800) || payload["targetRpm"] != float64(2800) {
		t.Fatalf("speed payload = %#v", payload)
	}
}

func TestBlackSharkWiFiMockImageUploadReportsProgress(t *testing.T) {
	var uploaded []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/screen/image" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var err error
		uploaded, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	m := newBlackSharkWiFiMockManager(server.URL, server.Client())
	input := []byte{1, 2, 3, 4}
	var stages []string
	if err := m.SendBlackSharkWiFiMockImage(nil, input, func(stage string, completed, total int) {
		stages = append(stages, stage)
	}); err != nil {
		t.Fatal(err)
	}
	if string(uploaded) != string(input) {
		t.Fatalf("uploaded = %v, want %v", uploaded, input)
	}
	if len(stages) != 3 || stages[0] != "preparing" || stages[1] != "uploading" || stages[2] != "completed" {
		t.Fatalf("progress stages = %v", stages)
	}
}

func TestBlackSharkWiFiMockScreenParametersUseSameSnapshot(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/screen/parameters" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	m := newBlackSharkWiFiMockManager(server.URL, server.Client())
	frame := []byte{0xA5, 0x10, 0x07, 0x01, 0x00, 0x2A}
	m.SetBlackSharkHostInfoFrame(frame)
	if err := m.SendBlackSharkWiFiHostInfoFrame(); err != nil {
		t.Fatal(err)
	}
	encoded, ok := payload["frameBase64"].(string)
	if !ok {
		t.Fatalf("payload = %#v, missing frameBase64", payload)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != string(frame) {
		t.Fatalf("frame = %v, want %v", decoded, frame)
	}
}

func TestDefaultWiFiDoesNotUseBlackSharkMockRoutes(t *testing.T) {
	m := NewManager(nil)
	m.Configure(types.DeviceTransportWiFi, "127.0.0.1:1")
	if m.isBlackSharkWiFiConnected() {
		t.Fatal("default WiFi profile must not be treated as Black Shark mock")
	}
}
