package coreapp

import "testing"

func TestDidDeviceSwitchToManualModeRecognizesProtocolLabels(t *testing.T) {
	if !didDeviceSwitchToManualMode("auto/realtime RPM mode", "manual/fixed gear mode") {
		t.Fatal("expected protocol manual mode to be recognized")
	}
	if didDeviceSwitchToManualMode("manual/fixed gear mode", "挡位工作模式") {
		t.Fatal("manual alias to manual alias should not count as a new switch")
	}
	if didDeviceSwitchToManualMode("auto/realtime RPM mode", "hid") {
		t.Fatal("transport-only status should not count as manual mode")
	}
}
