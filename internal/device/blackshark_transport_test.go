//go:build !legacydevice

package device

import (
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func TestBlackSharkUSBNativeDeviceInfoUsesUSBTransport(t *testing.T) {
	profile := types.BlackSharkBRB02USBProfile()
	info := nativeUSBDeviceInfo(profile, "libusb:vid_e2b7&pid_7001")
	if info["transport"] != types.DeviceTransportUSB {
		t.Fatalf("transport = %q, want %q", info["transport"], types.DeviceTransportUSB)
	}
	if info["profileId"] != types.BlackSharkBRB02USBProfileID {
		t.Fatalf("profileId = %q, want %q", info["profileId"], types.BlackSharkBRB02USBProfileID)
	}
}
