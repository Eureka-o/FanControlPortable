package coreapp

import (
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestShouldRefreshWiFiOverviewState(t *testing.T) {
	now := time.Unix(100, 0)

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
			lastRefresh: now.Add(-wifiOverviewStateRefreshInterval),
			want:        true,
		},
		{
			name:        "wifi refresh throttled",
			deviceType:  types.DeviceTransportWiFi,
			lastRefresh: now.Add(-wifiOverviewStateRefreshInterval + time.Millisecond),
			want:        false,
		},
		{
			name:         "skip when state was already read this tick",
			deviceType:   types.DeviceTransportWiFi,
			lastRefresh:  now.Add(-wifiOverviewStateRefreshInterval),
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
			got := shouldRefreshWiFiOverviewState(tt.deviceType, now, tt.lastRefresh, tt.readThisTick)
			if got != tt.want {
				t.Fatalf("shouldRefreshWiFiOverviewState() = %v, want %v", got, tt.want)
			}
		})
	}
}
