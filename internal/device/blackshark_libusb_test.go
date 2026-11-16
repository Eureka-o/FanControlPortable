//go:build !legacydevice && windows

package device

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/types"
	"golang.org/x/sys/windows"
)

func TestBlackSharkLibUSBMissingDLL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "libusb-1.0.dll")
	dll := windows.NewLazyDLL(path)
	previousDLL, previousInit := libusbDLL, libusbInitProc
	libusbDLL, libusbInitProc = dll, dll.NewProc("libusb_init")
	t.Cleanup(func() {
		libusbDLL, libusbInitProc = previousDLL, previousInit
	})
	dev, err := openBlackSharkUSBDevice()
	if dev != nil || err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("missing DLL error must identify %q, got %v", path, err)
	}
}

func TestBlackSharkLibUSBMissingSymbol(t *testing.T) {
	dll := windows.NewLazySystemDLL("kernel32.dll")
	const missing = "libusb_bulk_transfer"
	err := loadBlackSharkLibUSB(dll, dll.NewProc("GetCurrentProcessId"), dll.NewProc(missing))
	if err == nil || !strings.Contains(err.Error(), dll.Name) || !strings.Contains(err.Error(), missing) {
		t.Fatalf("missing symbol error must identify DLL and symbol, got %v", err)
	}
}

func TestBlackSharkLibUSBSymbolsAvailable(t *testing.T) {
	dll := windows.NewLazySystemDLL("kernel32.dll")
	if err := loadBlackSharkLibUSB(dll, dll.NewProc("GetCurrentProcessId"), dll.NewProc("GetLastError")); err != nil {
		t.Fatalf("available library symbols failed validation: %v", err)
	}
}

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
