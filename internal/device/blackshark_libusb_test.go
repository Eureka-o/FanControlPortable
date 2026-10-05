//go:build !legacydevice && windows

package device

import (
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func TestBlackSharkUSBTransportContract(t *testing.T) {
	if blackSharkUSBOutEndpoint != 0x01 || blackSharkUSBInEndpoint != 0x81 {
		t.Fatalf("unexpected endpoints: out=0x%02X in=0x%02X", blackSharkUSBOutEndpoint, blackSharkUSBInEndpoint)
	}
	if blackSharkHIDReportLen != 65 {
		t.Fatalf("unexpected report length: %d", blackSharkHIDReportLen)
	}
	if types.BlackSharkHIDVendorID != 0xE2B7 || types.BlackSharkBRB02HIDProductID != 0x7001 {
		t.Fatalf("unexpected black shark USB identity: VID=0x%04X PID=0x%04X", types.BlackSharkHIDVendorID, types.BlackSharkBRB02HIDProductID)
	}
	short := make([]byte, blackSharkHIDReportLen-1)
	if err := (&blackSharkUSBDevice{}).transfer(blackSharkUSBOutEndpoint, short, 10); err == nil {
		t.Fatal("short report was accepted")
	}
}
