package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestSelectDynamicWiFiEndpointRequiresSingleNewEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		oldEndpoint string
		devices     []types.WiFiDiscoveredDevice
		want        string
	}{
		{
			name:        "multiple devices",
			oldEndpoint: "http://192.168.1.20",
			devices: []types.WiFiDiscoveredDevice{
				{Endpoint: "http://192.168.1.21"},
				{Endpoint: "http://192.168.1.22"},
			},
		},
		{
			name:        "unique new endpoint",
			oldEndpoint: "http://192.168.1.20",
			devices:     []types.WiFiDiscoveredDevice{{Endpoint: " http://192.168.1.21 "}},
			want:        "http://192.168.1.21",
		},
		{
			name:        "missing old endpoint",
			oldEndpoint: " ",
			devices:     []types.WiFiDiscoveredDevice{{Endpoint: "http://192.168.1.21"}},
		},
		{
			name:        "same endpoint",
			oldEndpoint: " http://192.168.1.20 ",
			devices:     []types.WiFiDiscoveredDevice{{Endpoint: "HTTP://192.168.1.20"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectDynamicWiFiEndpoint(tt.devices, tt.oldEndpoint); got != tt.want {
				t.Fatalf("selectDynamicWiFiEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}
