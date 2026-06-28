package device

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestDiscoverWiFiDevicesFindsDefaultStateEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/data" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"speed":           36,
			"fanSpeed":        36,
			"wifiTargetSpeed": 42,
			"wifiControl":     true,
			"controlMode":     "wifi",
			"temperature":     41,
		})
	}))
	defer server.Close()

	result := DiscoverWiFiDevices(context.Background(), types.WiFiDiscoveryParams{
		Mode:        types.WiFiDiscoveryModeNormal,
		Endpoint:    server.URL,
		ProfileID:   "builtin.wifi.percent",
		ProfileName: "Slim压风散热器Pro",
		TimeoutMs:   120,
	})
	if !result.Found || len(result.Devices) == 0 {
		t.Fatalf("DiscoverWiFiDevices() found no devices: %#v", result)
	}
	if result.Devices[0].Endpoint != server.URL {
		t.Fatalf("endpoint = %q, want %q", result.Devices[0].Endpoint, server.URL)
	}
	if result.Devices[0].Speed != 36 || result.Devices[0].TargetSpeed != 42 {
		t.Fatalf("speed/target = %d/%d, want 36/42", result.Devices[0].Speed, result.Devices[0].TargetSpeed)
	}
}

func TestDiscoverWiFiDevicesReportsElapsedMs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"speed":       50,
			"wifiControl": true,
		})
	}))
	defer server.Close()
	endpoint := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)

	result := DiscoverWiFiDevices(context.Background(), types.WiFiDiscoveryParams{
		Mode:      types.WiFiDiscoveryModeNormal,
		Endpoint:  endpoint,
		TimeoutMs: 120,
	})
	if result.ElapsedMs <= 0 {
		t.Fatalf("ElapsedMs = %d, want positive elapsed time", result.ElapsedMs)
	}
}

func TestDiscoverWiFiDevicesRejectsGenericJSONState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"temperature": 42,
			"power":       18,
			"mode":        "online",
		})
	}))
	defer server.Close()

	result := DiscoverWiFiDevices(context.Background(), types.WiFiDiscoveryParams{
		Mode:      types.WiFiDiscoveryModeNormal,
		Endpoint:  server.URL,
		TimeoutMs: 120,
	})
	if result.Found || len(result.Devices) != 0 {
		t.Fatalf("generic JSON should not be detected as WiFi device: %#v", result)
	}
}

func TestPausedWiFiDiscoveryCanBeCanceled(t *testing.T) {
	control := types.NewWiFiDiscoveryControl()
	control.Pause()

	done := make(chan types.WiFiDiscoveryResult, 1)
	go func() {
		done <- DiscoverWiFiDevices(context.Background(), types.WiFiDiscoveryParams{
			Mode:      types.WiFiDiscoveryModeDeep,
			Endpoint:  "192.0.2.44",
			TimeoutMs: 120,
			Control:   control,
		})
	}()

	select {
	case result := <-done:
		t.Fatalf("scan completed while paused: %#v", result)
	case <-time.After(80 * time.Millisecond):
	}

	control.Cancel()
	select {
	case result := <-done:
		if !result.Canceled {
			t.Fatalf("Canceled = false, result = %#v", result)
		}
		if result.ScannedCount != 0 {
			t.Fatalf("ScannedCount = %d, want 0 while canceled from paused state", result.ScannedCount)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("scan did not stop after cancel")
	}
}

func TestDynamicWiFiDiscoveryCandidatesOnlyUseSavedSubnet(t *testing.T) {
	candidates, scopes, err := buildWiFiDiscoveryCandidates(types.WiFiDiscoveryParams{
		Mode:     types.WiFiDiscoveryModeDynamic,
		Endpoint: "192.168.137.42:18080",
	}, types.WiFiDiscoveryModeDynamic)
	if err != nil {
		t.Fatalf("buildWiFiDiscoveryCandidates() error = %v", err)
	}
	if len(candidates) != 254 {
		t.Fatalf("candidate count = %d, want 254", len(candidates))
	}
	if len(scopes) != 1 || scopes[0].Source != "savedSubnet" || scopes[0].Network != "192.168.137.0/24" {
		t.Fatalf("scopes = %#v, want savedSubnet 192.168.137.0/24", scopes)
	}
	for _, candidate := range candidates {
		if !strings.HasPrefix(candidate.Endpoint, "http://192.168.137.") {
			t.Fatalf("candidate endpoint %q outside saved subnet", candidate.Endpoint)
		}
		if !strings.HasSuffix(candidate.Endpoint, ":18080") {
			t.Fatalf("candidate endpoint %q did not preserve port", candidate.Endpoint)
		}
	}
}

func TestDeepWiFiDiscoveryAddsCommonSubnets(t *testing.T) {
	candidates, scopes, err := buildWiFiDiscoveryCandidates(types.WiFiDiscoveryParams{
		Mode:     types.WiFiDiscoveryModeDeep,
		Endpoint: "10.8.0.20",
	}, types.WiFiDiscoveryModeDeep)
	if err != nil {
		t.Fatalf("buildWiFiDiscoveryCandidates() error = %v", err)
	}
	if len(candidates) < 60000 {
		t.Fatalf("deep scan candidate count = %d, want at least 60000", len(candidates))
	}
	required := map[string]string{
		"192.168.1.0/24":  "commonSubnet",
		"10.0.0.0/24":     "commonSubnet",
		"10.137.137.0/24": "expandedSubnet",
		"172.16.0.0/24":   "commonSubnet",
		"172.31.137.0/24": "expandedSubnet",
	}
	found := map[string]bool{}
	for _, scope := range scopes {
		if required[scope.Network] == scope.Source {
			found[scope.Network] = true
		}
	}
	for network, source := range required {
		if !found[network] {
			t.Fatalf("deep scan scopes did not include %s as %s: %#v", network, source, scopes)
		}
	}
}
