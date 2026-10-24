package types

import "testing"

func TestBlackSharkScreenImageTransferUsesUSBOrWiFiMock(t *testing.T) {
	ble := BlackSharkBRB02Profile()
	if ble.Capabilities.SupportsScreen || ble.Capabilities.SupportsScreenImageTransfer {
		t.Fatal("BLE Black Shark profile must not advertise screen parameter or image transfer support")
	}

	usb := BlackSharkBRB02USBProfile()
	if usb.Transport != DeviceTransportUSB || !usb.Capabilities.SupportsScreen || !usb.Capabilities.SupportsScreenImageTransfer {
		t.Fatal("WinUSB Black Shark profile must advertise screen parameters and USB image transfer")
	}

	wifi := BlackSharkBRB02WiFiProfile()
	if wifi.Transport != DeviceTransportWiFi || !wifi.Capabilities.SupportsScreenImageTransfer {
		t.Fatal("Black Shark WiFi preview profile must advertise image transfer")
	}
}

func TestBlackSharkWiFiProfileMatchesWiredPreviewCapabilities(t *testing.T) {
	profile := BlackSharkBRB02WiFiProfile()
	if profile.ID != BlackSharkBRB02WiFiProfileID || profile.DisplayName != BlackSharkBRB02DisplayName {
		t.Fatalf("WiFi Black Shark identity = %q/%q", profile.ID, profile.DisplayName)
	}
	if profile.Transport != DeviceTransportWiFi || profile.SpeedUnit != FanSpeedUnitRPM {
		t.Fatalf("WiFi Black Shark transport/unit = %q/%q", profile.Transport, profile.SpeedUnit)
	}
	if profile.SpeedRange.Min != 0 || profile.SpeedRange.Max != 4000 {
		t.Fatalf("WiFi Black Shark speed range = %#v, want 0-4000", profile.SpeedRange)
	}
	if profile.Connection.Endpoint != "http://127.0.0.1:18080" ||
		profile.Connection.StateEndpoint != "/api/data" ||
		profile.Connection.SpeedEndpoint != "/api/speed" ||
		profile.Connection.HTTPMethod != "POST" {
		t.Fatalf("WiFi Black Shark connection = %#v", profile.Connection)
	}
	caps := profile.Capabilities
	if !caps.SupportsReadState || !caps.SupportsSetSpeed || !caps.SupportsLighting ||
		!caps.SupportsBrightness || caps.SupportsGearLight || !caps.SupportsScreen ||
		!caps.SupportsScreenImageTransfer {
		t.Fatalf("WiFi Black Shark capabilities = %#v", caps)
	}
}

func TestNormalizeBlackSharkScreenRefreshIntervalMs(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{input: -1, want: 0},
		{input: 0, want: 0},
		{input: 1, want: BlackSharkScreenRefreshMinMs},
		{input: 125, want: 125},
		{input: BlackSharkScreenRefreshMaxMs + 1, want: BlackSharkScreenRefreshMaxMs},
	}
	for _, test := range tests {
		if got := NormalizeBlackSharkScreenRefreshIntervalMs(test.input); got != test.want {
			t.Errorf("NormalizeBlackSharkScreenRefreshIntervalMs(%d) = %d, want %d", test.input, got, test.want)
		}
	}
}
