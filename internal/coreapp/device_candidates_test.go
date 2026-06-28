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
