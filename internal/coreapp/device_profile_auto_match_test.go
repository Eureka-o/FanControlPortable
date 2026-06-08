package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestConnectedFlyDigiProfileIDMatchesHIDProductID(t *testing.T) {
	id := connectedFlyDigiProfileID(map[string]string{
		"transport": "hid",
		"model":     "Unknown",
		"productId": "0x1002",
	})
	if id != types.FlyDigiBS2PROProfileID {
		t.Fatalf("matched profile = %q, want %q", id, types.FlyDigiBS2PROProfileID)
	}
}

func TestConnectedFlyDigiProfileIDMatchesBS1BLEModel(t *testing.T) {
	id := connectedFlyDigiProfileID(map[string]string{
		"transport": "ble",
		"model":     "BS1",
	})
	if id != types.FlyDigiBS1ProfileID {
		t.Fatalf("matched profile = %q, want %q", id, types.FlyDigiBS1ProfileID)
	}
}

func TestSyncConnectedBuiltInDeviceProfileDoesNotPersistFlyDigiBeta(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.DeviceTransport = types.DeviceTransportHID
	cfg.ActiveDeviceProfileID = types.LegacyRPMProfileID
	cfg.DeviceProfiles = []types.DeviceProfile{
		types.DefaultWiFiPercentProfile("10.0.0.25"),
		types.LegacyRPMProfile(),
	}
	types.NormalizeDeviceProfileConfig(&cfg)
	app := newDeviceProfileTestApp(t, cfg)

	if app.syncConnectedBuiltInDeviceProfile(map[string]string{
		"transport": "hid",
		"model":     "BS2PRO",
		"productId": "0x1002",
	}) {
		t.Fatal("hidden FlyDigi beta device should not update the active profile")
	}

	got := app.configManager.Get()
	if got.ActiveDeviceProfileID != types.LegacyRPMProfileID {
		t.Fatalf("active profile = %q, want %q", got.ActiveDeviceProfileID, types.LegacyRPMProfileID)
	}
	if got.DeviceTransport != types.DeviceTransportHID {
		t.Fatalf("device transport = %q, want hid", got.DeviceTransport)
	}
	if got.ActiveDeviceProfileIDsByTransport[types.DeviceTransportHID] != types.LegacyRPMProfileID {
		t.Fatalf("remembered HID profile = %q, want %q", got.ActiveDeviceProfileIDsByTransport[types.DeviceTransportHID], types.LegacyRPMProfileID)
	}
}

func TestSyncConnectedBuiltInDeviceProfileLeavesUnknownDeviceUnchanged(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	app := newDeviceProfileTestApp(t, cfg)

	if app.syncConnectedBuiltInDeviceProfile(map[string]string{
		"transport": "wifi",
		"model":     "Unknown",
	}) {
		t.Fatal("unexpected profile update for unknown WiFi device")
	}

	got := app.configManager.Get()
	if got.ActiveDeviceProfileID != cfg.ActiveDeviceProfileID || got.DeviceTransport != cfg.DeviceTransport {
		t.Fatalf("config changed unexpectedly: %#v", got)
	}
}
