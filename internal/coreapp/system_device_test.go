package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

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

func TestGetDeviceStatusRequiresManagerConnection(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	app := newDeviceProfileTestApp(t, cfg)
	app.mutex.Lock()
	app.isConnected = true
	app.mutex.Unlock()

	status := app.GetDeviceStatus()
	if connected, _ := status["connected"].(bool); connected {
		t.Fatalf("connected = true with disconnected manager: %#v", status)
	}
	if _, ok := status["deviceProfile"]; ok {
		t.Fatalf("disconnected status should not expose configured deviceProfile: %#v", status)
	}
	if _, ok := status["deviceCapabilities"]; ok {
		t.Fatalf("disconnected status should not expose configured capabilities: %#v", status)
	}
}
