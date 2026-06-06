//go:build !legacydevice

package device

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestNormalBuildLegacyHIDUsesExplicitStub(t *testing.T) {
	m := NewManager(nil)
	m.ConfigureProfile(types.LegacyRPMProfile(), "")

	if m.deviceTransport != types.DeviceTransportHID {
		t.Fatalf("device transport = %q, want hid", m.deviceTransport)
	}
	if m.activeProfile.SpeedUnit != types.FanSpeedUnitRPM {
		t.Fatalf("active profile unit = %q, want rpm", m.activeProfile.SpeedUnit)
	}
	if got := m.GetModelName(); got != "Legacy RPM controller" {
		t.Fatalf("model name = %q, want legacy RPM profile name", got)
	}

	connected, info := m.Connect()
	if connected {
		t.Fatal("normal build must not connect legacy HID without legacydevice tag")
	}
	if info["transport"] != types.DeviceTransportHID {
		t.Fatalf("connect info transport = %q, want hid", info["transport"])
	}
	if info["message"] != legacyHIDDisabledMessage {
		t.Fatalf("connect info message = %q, want %q", info["message"], legacyHIDDisabledMessage)
	}
	if m.GetDeviceType() != "" {
		t.Fatalf("device type = %q, want empty after failed stub connect", m.GetDeviceType())
	}

	settings, err := m.QueryDeviceSettings()
	if err == nil {
		t.Fatal("QueryDeviceSettings should report unavailable legacy HID in normal build")
	}
	if settings.Source != types.DeviceTransportHID || settings.Model != "Legacy RPM controller" {
		t.Fatalf("settings = %#v, want hid source and legacy RPM model", settings)
	}
}

func TestNormalBuildLegacyHIDRejectsPercentAndRPMCommandsWhenDisconnected(t *testing.T) {
	m := NewManager(nil)
	m.Configure(types.DeviceTransportHID, "")

	if ok := m.SetPercentSpeed(50); ok {
		t.Fatal("normal build legacy HID stub must reject percent commands")
	}
	if ok := m.SetTargetSpeed(1800, types.FanSpeedUnitRPM); ok {
		t.Fatal("normal build legacy HID stub must reject RPM commands while disconnected")
	}
}
