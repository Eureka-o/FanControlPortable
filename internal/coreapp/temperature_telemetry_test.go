package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestShouldRestartTemperatureBridgeClassifiesPermanentMSRFailure(t *testing.T) {
	tests := []struct {
		name string
		temp types.TemperatureData
		want bool
	}{
		{
			name: "permanent msr failure",
			temp: types.TemperatureData{BridgeMsg: "[MSR-UNAVAILABLE] PawnIO installed but raw reads are invalid"},
			want: false,
		},
		{
			name: "pipe eof",
			temp: types.TemperatureData{BridgeMsg: "pipe EOF"},
			want: true,
		},
		{
			name: "responsive bridge returned zero sensors",
			temp: types.TemperatureData{BridgeMsg: "no temperature sensors"},
			want: true,
		},
		{
			name: "healthy bridge",
			temp: types.TemperatureData{BridgeOk: true, BridgeMsg: "pipe EOF"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRestartTemperatureBridge(tt.temp); got != tt.want {
				t.Fatalf("shouldRestartTemperatureBridge() = %v, want %v", got, tt.want)
			}
		})
	}
}
