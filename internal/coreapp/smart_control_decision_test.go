package coreapp

import "testing"

func TestSmartControlGateReason(t *testing.T) {
	tests := []struct {
		name         string
		autoControl  bool
		inputReady   bool
		controlReady bool
		want         string
	}{
		{name: "active", autoControl: true, inputReady: true, controlReady: true},
		{name: "automatic control disabled", inputReady: true, controlReady: true, want: "automatic-control-disabled"},
		{name: "temperature unavailable", autoControl: true, controlReady: true, want: "temperature-unavailable"},
		{name: "device not ready", autoControl: true, inputReady: true, want: "device-not-ready"},
		{name: "disabled takes priority", want: "automatic-control-disabled"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := smartControlGateReason(test.autoControl, test.inputReady, test.controlReady); got != test.want {
				t.Fatalf("smartControlGateReason() = %q, want %q", got, test.want)
			}
		})
	}
}
