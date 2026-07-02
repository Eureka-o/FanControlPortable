package coreapp

import (
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestShouldRefreshWiFiOverviewState(t *testing.T) {
	now := time.Unix(100, 0)
	interval := wifiOverviewForegroundRefreshInterval

	tests := []struct {
		name         string
		deviceType   string
		lastRefresh  time.Time
		readThisTick bool
		want         bool
	}{
		{
			name:       "first wifi refresh",
			deviceType: types.DeviceTransportWiFi,
			want:       true,
		},
		{
			name:        "wifi refresh after interval",
			deviceType:  types.DeviceTransportWiFi,
			lastRefresh: now.Add(-interval),
			want:        true,
		},
		{
			name:        "wifi refresh throttled",
			deviceType:  types.DeviceTransportWiFi,
			lastRefresh: now.Add(-interval + time.Millisecond),
			want:        false,
		},
		{
			name:         "skip when state was already read this tick",
			deviceType:   types.DeviceTransportWiFi,
			lastRefresh:  now.Add(-interval),
			readThisTick: true,
			want:         false,
		},
		{
			name:       "non wifi device",
			deviceType: types.DeviceTransportBLE,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRefreshWiFiOverviewState(tt.deviceType, now, tt.lastRefresh, tt.readThisTick, interval)
			if got != tt.want {
				t.Fatalf("shouldRefreshWiFiOverviewState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWiFiOverviewRefreshInterval(t *testing.T) {
	tests := []struct {
		name        string
		hasClients  bool
		autoControl bool
		want        time.Duration
	}{
		{name: "foreground keeps live overview responsive", hasClients: true, want: wifiOverviewForegroundRefreshInterval},
		{name: "background auto control keeps moderate refresh", autoControl: true, want: wifiOverviewAutoControlRefreshInterval},
		{name: "background idle slows overview refresh", want: wifiOverviewBackgroundRefreshInterval},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wifiOverviewRefreshInterval(tt.hasClients, tt.autoControl)
			if got != tt.want {
				t.Fatalf("wifiOverviewRefreshInterval() = %s, want %s", got, tt.want)
			}
		})
	}
}
