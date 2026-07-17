package guiapp

import (
	"encoding/json"
	"testing"
)

func TestDecodeDeviceConnectedPayloadPreservesWiFiRuntimeState(t *testing.T) {
	data, err := json.Marshal(map[string]any{
		"transport": "wifi",
		"endpoint":  "http://192.168.1.21",
		"currentData": map[string]any{
			"currentRpm": 42,
			"targetRpm":  55,
			"transport":  "wifi",
			"speedUnit":  "percent",
		},
		"deviceProfile": map[string]any{
			"id":        "wifi-default",
			"transport": "wifi",
		},
	})
	if err != nil {
		t.Fatalf("marshal device event: %v", err)
	}

	payload, err := decodeDeviceConnectedPayload(data)
	if err != nil {
		t.Fatalf("decodeDeviceConnectedPayload() error = %v", err)
	}
	if payload["transport"] != "wifi" || payload["endpoint"] != "http://192.168.1.21" {
		t.Fatalf("payload connection fields = %#v", payload)
	}
	currentData, ok := payload["currentData"].(map[string]any)
	if !ok {
		t.Fatalf("currentData type = %T, want map[string]any", payload["currentData"])
	}
	if currentData["transport"] != "wifi" || currentData["speedUnit"] != "percent" {
		t.Fatalf("currentData = %#v, want WiFi state", currentData)
	}
	profile, ok := payload["deviceProfile"].(map[string]any)
	if !ok || profile["transport"] != "wifi" {
		t.Fatalf("deviceProfile = %#v, want WiFi profile", payload["deviceProfile"])
	}
}

func TestDecodeDeviceConnectedPayloadRejectsInvalidJSON(t *testing.T) {
	if payload, err := decodeDeviceConnectedPayload(json.RawMessage(`{"transport":`)); err == nil || payload != nil {
		t.Fatalf("invalid payload = %#v, error = %v", payload, err)
	}
}
