package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestConnectedDeviceDisplayName(t *testing.T) {
	flyDigi := types.FlyDigiBS3PROProfile()
	slim := types.DefaultWiFiPercentProfile("")
	slim.Vendor = "FanControl"

	tests := []struct {
		name     string
		profile  types.DeviceProfile
		fallback string
		want     string
	}{
		{
			name:    "combines vendor and model",
			profile: types.DeviceProfile{DisplayName: "Friendly name", Vendor: "Acme", Model: "X1"},
			want:    "Acme X1",
		},
		{
			name:    "preserves canonical FlyDigi name",
			profile: flyDigi,
			want:    flyDigi.DisplayName,
		},
		{
			name:    "omits legacy vendor from Slim",
			profile: slim,
			want:    slim.Model,
		},
		{
			name:    "does not duplicate vendor already in model",
			profile: types.DeviceProfile{Vendor: "Acme", Model: "Acme X1"},
			want:    "Acme X1",
		},
		{
			name:     "keeps fallback behavior without profile metadata",
			fallback: "Unknown device",
			want:     "Unknown device",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := connectedDeviceDisplayName(tt.profile, "", nil, tt.fallback); got != tt.want {
				t.Fatalf("connectedDeviceDisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}
