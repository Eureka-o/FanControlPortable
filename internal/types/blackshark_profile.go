package types

const (
	BlackSharkBRB02ProfileID           = "builtin.blackshark.brb02.ble.rpm"
	BlackSharkBRB02USBProfileID        = "builtin.blackshark.brb02.usb.rpm"
	BlackSharkBRB02DisplayName         = "黑鲨（Black Shark）风神 Pro"
	BlackSharkBRB02Vendor              = "黑鲨（Black Shark）"
	BlackSharkBRB02HIDVendorID  uint16 = 0xE2B7
	BlackSharkHIDVendorID       uint16 = BlackSharkBRB02HIDVendorID
	BlackSharkBRB02HIDProductID uint16 = 0x7001
)

func BlackSharkBRB02Profile() DeviceProfile {
	caps := blackSharkBRB02Capabilities(BlackSharkBRB02ProfileID, DeviceTransportBLE)
	return DeviceProfile{
		ID:          BlackSharkBRB02ProfileID,
		DisplayName: BlackSharkBRB02DisplayName,
		Vendor:      BlackSharkBRB02Vendor,
		// Keep the runtime model display canonical so display-side resolvers do
		// not reinsert a separator between the vendor and Chinese product name.
		Model:      BlackSharkBRB02DisplayName,
		Notes:      "Black Shark BRB02 Cooler Pro over BLE; fixed RPM control and device-side RGB lighting for firmware v3.0.3.",
		BuiltIn:    true,
		Transport:  DeviceTransportBLE,
		SpeedUnit:  FanSpeedUnitRPM,
		SpeedRange: caps.SpeedRange,
		Connection: DeviceConnectionSettings{
			BLENameFilter: "BS BRB02 Cooler Pro",
			// Write and notify characteristics live under AE40/AE30 respectively;
			// keep service discovery open so the existing client can find both.
			BLEServiceUUID:          "",
			BLEWriteCharacteristic:  "ae41",
			BLENotifyCharacteristic: "ae04",
			BLEWriteWithResponse:    false,
		},
		Capabilities: caps,
	}
}

// BlackSharkBRB02USBProfile is the WinUSB/libusb path for the BRB02. It is
// deliberately separate from the FlyDigi HID transport because the device is
// WinUSB-bound and exposes bulk endpoints rather than a hidapi interface.
func BlackSharkBRB02USBProfile() DeviceProfile {
	caps := blackSharkBRB02Capabilities(BlackSharkBRB02USBProfileID, DeviceTransportUSB)
	return DeviceProfile{
		ID:           BlackSharkBRB02USBProfileID,
		DisplayName:  BlackSharkBRB02DisplayName,
		Vendor:       BlackSharkBRB02Vendor,
		Model:        BlackSharkBRB02DisplayName,
		Notes:        "Black Shark BRB02 Cooler Pro over WinUSB/libusb; VID 0xE2B7, PID 0x7001; bulk OUT 0x01 / IN 0x81, 65-byte frames.",
		BuiltIn:      true,
		Transport:    DeviceTransportUSB,
		SpeedUnit:    FanSpeedUnitRPM,
		SpeedRange:   caps.SpeedRange,
		Capabilities: caps,
	}
}

func blackSharkBRB02Capabilities(profileID, transport string) DeviceCapabilities {
	// Image transfer is available only through the WinUSB/libusb transport.
	// The BLE captures do not establish an equivalent image transport.
	supportsScreenImageTransfer := transport == DeviceTransportUSB
	return DeviceCapabilities{
		ProfileID:                   profileID,
		DisplayName:                 BlackSharkBRB02DisplayName,
		Transport:                   transport,
		SpeedUnit:                   FanSpeedUnitRPM,
		SpeedRange:                  DeviceSpeedRange{Min: 0, Max: 4000, Step: 1},
		SupportsReadState:           true,
		SupportsSetSpeed:            true,
		SupportsManualGears:         false,
		SupportsCustomSpeed:         true,
		SupportsLighting:            true,
		SupportsBrightness:          true,
		SupportsGearLight:           true,
		SupportsScreen:              false,
		SupportsScreenImageTransfer: supportsScreenImageTransfer,
		SupportsPowerOnStart:        false,
		SupportsSmartStartStop:      false,
	}
}
