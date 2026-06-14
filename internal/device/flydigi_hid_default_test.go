//go:build !legacydevice

package device

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestNormalBuildFlyDigiHIDProfileUsesRPMCapabilities(t *testing.T) {
	m := NewManager(nil)
	m.ConfigureProfile(types.FlyDigiBS2PROProfile(), "")

	if m.deviceTransport != types.DeviceTransportHID {
		t.Fatalf("device transport = %q, want hid", m.deviceTransport)
	}
	if m.activeProfile.SpeedUnit != types.FanSpeedUnitRPM {
		t.Fatalf("active profile unit = %q, want rpm", m.activeProfile.SpeedUnit)
	}
	if got := m.GetModelName(); got != "飞智（FlyDigi）BS2PRO" {
		t.Fatalf("model name = %q, want FlyDigi profile name", got)
	}
	if !m.shouldUseLegacyHIDLocked() {
		t.Fatal("FlyDigi HID profile should use the HID connection path")
	}

	info := m.flyDigiHIDInfoLocked(types.FlyDigiBS2PROProductID, `\\?\hid#vid_37d7&pid_1002`)
	if info["transport"] != types.DeviceTransportHID {
		t.Fatalf("connect info transport = %q, want hid", info["transport"])
	}
	if info["productId"] != "0x1002" {
		t.Fatalf("connect info productId = %q, want 0x1002", info["productId"])
	}
}

func TestNormalBuildFlyDigiHIDRejectsCommandsWhenDisconnected(t *testing.T) {
	m := NewManager(nil)
	m.ConfigureProfile(types.FlyDigiBS2PROProfile(), "")

	if ok := m.SetPercentSpeed(50); ok {
		t.Fatal("FlyDigi HID path must reject percent commands")
	}
	if ok := m.SetTargetSpeed(1800, types.FanSpeedUnitRPM); ok {
		t.Fatal("FlyDigi HID path must reject RPM commands while disconnected")
	}
	if ok := m.SetGearLight(true); ok {
		t.Fatal("FlyDigi HID path must reject gear-light commands while disconnected")
	}
	if err := m.SetLightStrip(types.GetDefaultLightStripConfig()); err == nil {
		t.Fatal("FlyDigi HID path must reject light-strip commands while disconnected")
	}
}

func TestFlyDigiHIDProductIDsPreferSpecificProfile(t *testing.T) {
	got := flyDigiHIDProductIDsForProfile(types.FlyDigiBS3PROProfileID)
	if len(got) != 1 || got[0] != types.FlyDigiBS3PROProductID {
		t.Fatalf("BS3PRO product ids = %#v, want only 0x1004", got)
	}

	got = flyDigiHIDProductIDsForProfile(types.LegacyRPMProfileID)
	if len(got) < 4 {
		t.Fatalf("fallback product ids = %#v, want all FlyDigi HID devices", got)
	}
}

func TestActiveProfileUsesConnectedFlyDigiProductID(t *testing.T) {
	m := NewManager(nil)
	m.ConfigureProfile(types.LegacyRPMProfile(), "")
	m.deviceType = types.DeviceTransportHID
	m.productID = types.FlyDigiBS3PROProductID

	profile := m.ActiveProfile()
	if profile.ID != types.FlyDigiBS3PROProfileID {
		t.Fatalf("runtime profile = %q, want %q", profile.ID, types.FlyDigiBS3PROProfileID)
	}
	if !profile.Capabilities.SupportsSmartStartStop || !profile.Capabilities.SupportsLighting {
		t.Fatalf("runtime FlyDigi capabilities did not use whitelist: %#v", profile.Capabilities)
	}
}

func TestFlyDigiHIDPathMatchesBluetoothLEVIDPID(t *testing.T) {
	path := `\\?\hid#{00001812-0000-1000-8000-00805f9b34fb}_dev_vid&0137d7_pid&1004_rev&0110_dc7f643a1704#9&b6797ea&0&0000`
	productID, ok := flyDigiHIDProductIDFromPath(path, flyDigiHIDProductIDsForProfile(types.LegacyRPMProfileID))
	if !ok {
		t.Fatal("expected Bluetooth LE HID device path to match FlyDigi VID/PID")
	}
	if productID != types.FlyDigiBS3PROProductID {
		t.Fatalf("matched product id = 0x%04X, want 0x%04X", productID, types.FlyDigiBS3PROProductID)
	}
}

func TestPadFlyDigiHIDReportUsesOutputReportLength(t *testing.T) {
	shortReport := []byte{0x02, 0x5A, 0xA5, 0x23}
	padded := padFlyDigiHIDReport(shortReport, hidLightReportLen)
	if len(padded) != hidLightReportLen {
		t.Fatalf("padded report length = %d, want %d", len(padded), hidLightReportLen)
	}
	for i, value := range shortReport {
		if padded[i] != value {
			t.Fatalf("padded report byte %d = 0x%02X, want 0x%02X", i, padded[i], value)
		}
	}

	fullReport := make([]byte, hidLightReportLen)
	if got := padFlyDigiHIDReport(fullReport, hidLightReportLen); &got[0] != &fullReport[0] {
		t.Fatal("full-length HID reports should be reused without another allocation")
	}
}
