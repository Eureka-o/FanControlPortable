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
