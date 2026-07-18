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

func TestSyncConnectedBuiltInDeviceProfileDoesNotPersistFlyDigiProfile(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.DeviceTransport = types.DeviceTransportWiFi
	cfg.WiFiCompatibilityEnabled = true
	cfg.ActiveDeviceProfileID = types.DefaultWiFiPercentProfileID
	cfg.DeviceProfiles = []types.DeviceProfile{
		types.DefaultWiFiPercentProfile("10.0.0.25"),
		types.FlyDigiBS2PROProfile(),
	}
	types.NormalizeDeviceProfileConfig(&cfg)
	app := newDeviceProfileTestApp(t, cfg)

	if app.syncConnectedBuiltInDeviceProfile(map[string]string{
		"transport": "hid",
		"model":     "BS2PRO",
		"productId": "0x1002",
	}) {
		t.Fatal("FlyDigi native runtime match should not update persistent active profile")
	}

	got := app.configManager.Get()
	if got.ActiveDeviceProfileID != types.DefaultWiFiPercentProfileID {
		t.Fatalf("active profile = %q, want %q", got.ActiveDeviceProfileID, types.DefaultWiFiPercentProfileID)
	}
	if got.DeviceTransport != types.DeviceTransportWiFi {
		t.Fatalf("device transport = %q, want wifi", got.DeviceTransport)
	}
	if _, ok := got.ActiveDeviceProfileIDsByTransport[types.DeviceTransportHID]; ok {
		t.Fatalf("HID active profile should not persist: %#v", got.ActiveDeviceProfileIDsByTransport)
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

func TestNativeConnectProfileByIDFindsBuiltInBLEProfile(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	profile, ok := nativeConnectProfileByID(cfg, types.FlyDigiBS1ProfileID)
	if !ok {
		t.Fatal("expected built-in BLE profile to be connectable by ID")
	}
	if profile.ID != types.FlyDigiBS1ProfileID || profile.Transport != types.DeviceTransportBLE {
		t.Fatalf("profile = %q/%q, want FlyDigi BS1 BLE", profile.ID, profile.Transport)
	}
}
