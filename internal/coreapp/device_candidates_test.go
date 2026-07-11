package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestSerialDeviceCandidatesRequireDetectedPort(t *testing.T) {
	serial := testSerialDeviceProfile()
	cfg := types.GetDefaultConfig(false)
	cfg.SerialCompatibilityEnabled = true
	cfg.DeviceProfiles = []types.DeviceProfile{
		types.DefaultWiFiPercentProfile("10.0.0.25"),
		serial,
	}

	if got := serialDeviceCandidates(cfg, map[string]bool{}); len(got) != 0 {
		t.Fatalf("serial candidates with no detected ports = %#v, want none", got)
	}

	got := serialDeviceCandidates(cfg, map[string]bool{"COM3": true})
	if len(got) != 1 {
		t.Fatalf("serial candidates with COM3 = %d, want 1", len(got))
	}
	if got[0].Transport != types.DeviceTransportSerial || got[0].Endpoint != "COM3" {
		t.Fatalf("serial candidate = %#v, want serial COM3", got[0])
	}
}

func TestSelectNativeAutoConnectCandidateUsesUniquePreferredProfile(t *testing.T) {
	devices := []map[string]string{
		{"profileId": types.FlyDigiBS2ProfileID, "endpoint": "hid-bs2"},
		{"profileId": types.FlyDigiBS3ProfileID, "endpoint": "hid-bs3"},
	}
	cfg := types.AppConfig{
		ActiveDeviceProfileIDsByTransport: map[string]string{
			types.DeviceTransportHID: types.FlyDigiBS3ProfileID,
		},
	}

	selected, ok := selectNativeAutoConnectCandidate(devices, cfg, types.DeviceTransportHID)
	if !ok || selected["endpoint"] != "hid-bs3" {
		t.Fatalf("selected = %#v, ok=%v, want unique preferred BS3", selected, ok)
	}

	devices = append(devices, map[string]string{"profileId": types.FlyDigiBS3ProfileID, "endpoint": "hid-bs3-second"})
	if selected, ok := selectNativeAutoConnectCandidate(devices, cfg, types.DeviceTransportHID); ok || selected != nil {
		t.Fatalf("duplicate preferred profile should stay ambiguous: selected=%#v ok=%v", selected, ok)
	}
}
