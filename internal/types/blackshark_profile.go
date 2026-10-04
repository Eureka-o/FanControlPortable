package types

const (
	BlackSharkBRB02ProfileID           = "builtin.blackshark.brb02.ble.rpm"
	BlackSharkBRB02HIDProfileID        = "builtin.blackshark.brb02.hid.rpm"
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

func BlackSharkBRB02HIDProfile() DeviceProfile {
	caps := blackSharkBRB02Capabilities(BlackSharkBRB02HIDProfileID, DeviceTransportHID)
	return DeviceProfile{
		ID:           BlackSharkBRB02HIDProfileID,
		DisplayName:  BlackSharkBRB02DisplayName,
		Vendor:       BlackSharkBRB02Vendor,
		Model:        BlackSharkBRB02DisplayName,
		Notes:        "Black Shark BRB02 Cooler Pro over HID; VID 0xE2B7, PID 0x7001; fixed RPM control and device-side RGB lighting.",
		BuiltIn:      true,
		Transport:    DeviceTransportHID,
		SpeedUnit:    FanSpeedUnitRPM,
		SpeedRange:   caps.SpeedRange,
		Capabilities: caps,
	}
}

func blackSharkBRB02Capabilities(profileID, transport string) DeviceCapabilities {
	return DeviceCapabilities{
		ProfileID:              profileID,
		DisplayName:            BlackSharkBRB02DisplayName,
		Transport:              transport,
		SpeedUnit:              FanSpeedUnitRPM,
		SpeedRange:             DeviceSpeedRange{Min: 0, Max: 4000, Step: 1},
		SupportsReadState:      true,
		SupportsSetSpeed:       true,
		SupportsManualGears:    false,
		SupportsCustomSpeed:    true,
		SupportsLighting:       true,
		SupportsBrightness:     true,
		SupportsGearLight:      true,
		SupportsScreen:         false,
		SupportsPowerOnStart:   false,
		SupportsSmartStartStop: false,
	}
}
