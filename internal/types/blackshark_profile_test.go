package types

import "testing"

func TestBlackSharkScreenImageTransferIsUSBOnly(t *testing.T) {
	ble := BlackSharkBRB02Profile()
	if ble.Capabilities.SupportsScreenImageTransfer {
		t.Fatal("BLE Black Shark profile must not advertise screen image transfer")
	}

	usb := BlackSharkBRB02USBProfile()
	if usb.Transport != DeviceTransportUSB || !usb.Capabilities.SupportsScreenImageTransfer {
		t.Fatal("WinUSB Black Shark profile must advertise USB image transfer")
	}
}
