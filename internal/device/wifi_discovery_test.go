package device

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	_, scopes, err := buildWiFiDiscoveryCandidates(types.WiFiDiscoveryParams{
		Mode:     types.WiFiDiscoveryModeDeep,
		Endpoint: "10.8.0.20",
	}, types.WiFiDiscoveryModeDeep)
	if err != nil {
		t.Fatalf("buildWiFiDiscoveryCandidates() error = %v", err)
	}
	foundCommon := false
	for _, scope := range scopes {
		if scope.Source == "commonSubnet" && scope.Network == "192.168.1.0/24" {
			foundCommon = true
			break
		}
	}
	if !foundCommon {
		t.Fatalf("deep scan scopes did not include 192.168.1.0/24: %#v", scopes)
	}
}
