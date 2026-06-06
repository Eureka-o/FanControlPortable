package deviceprofileexec

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestProfileTesterConnectsWiFiDraftWithoutSaving(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/state" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"speed": 44, "targetSpeed": 44})
	}))
	defer server.Close()

	result, err := (ProfileTester{}).Test(nil, types.DeviceProfileTestParams{
		Action: "connect",
		Profile: types.DeviceProfile{
			ID:          "test.wifi",
			DisplayName: "Test WiFi",
			Transport:   types.DeviceTransportWiFi,
			SpeedUnit:   types.FanSpeedUnitPercent,
			SpeedRange:  types.DefaultPercentSpeedRange(),
			Connection: types.DeviceConnectionSettings{
				Endpoint:      server.URL,
				StateEndpoint: "/state",
				SpeedEndpoint: "/speed",
				HTTPMethod:    http.MethodPost,
			},
			Capabilities: types.DefaultWiFiPercentCapabilities(),
		},
	})
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if !result.Connected || result.Action != ProfileTestActionConnect || result.Transport != types.DeviceTransportWiFi {
		t.Fatalf("result = %#v", result)
	}
	if result.FanData == nil || result.FanData.CurrentRPM != 44 || result.FanData.SpeedUnit != types.FanSpeedUnitPercent {
		t.Fatalf("fan data = %#v, want 44 percent", result.FanData)
	}
}

func TestProfileTesterReadsSerialDraftAndClosesPort(t *testing.T) {
	port := &fakeSerialPort{reads: [][]byte{[]byte("rpm=1880\n")}}
	result, err := (ProfileTester{SerialDialer: fakeSerialDialer{port: port}}).Test(nil, types.DeviceProfileTestParams{
		Action: "read",
		Profile: types.DeviceProfile{
			ID:          "test.serial",
			DisplayName: "Test serial",
			Transport:   types.DeviceTransportSerial,
			SpeedUnit:   types.FanSpeedUnitRPM,
			SpeedRange:  types.DefaultRPMSpeedRange(),
			Connection: types.DeviceConnectionSettings{
				SerialPort:           "COM3",
				SerialBaudRate:       115200,
				SerialFrameDelimiter: `\n`,
			},
			Commands: []types.DeviceCommandTemplate{
				{Name: "readState", Command: "GET", Encoding: "ascii"},
			},
			ResponseParsers: []types.DeviceResponseParser{
				{Name: "current rpm", Type: "regex", Expression: `rpm=(\d+)`},
			},
			Capabilities: types.DeviceCapabilities{
				Transport:         types.DeviceTransportSerial,
				SpeedUnit:         types.FanSpeedUnitRPM,
				SpeedRange:        types.DefaultRPMSpeedRange(),
				SupportsReadState: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if !port.closed {
		t.Fatal("expected transient serial test to close the port")
	}
	if len(port.writes) != 1 || string(port.writes[0]) != "GET\n" {
		t.Fatalf("writes = %q, want GET newline", port.writes)
	}
	if result.FanData == nil || result.FanData.CurrentRPM != 1880 || result.FanData.SpeedUnit != types.FanSpeedUnitRPM {
		t.Fatalf("fan data = %#v, want 1880 RPM", result.FanData)
	}
}

func TestProfileTesterSetsBLEPercentUsingTenths(t *testing.T) {
	client := &fakeBLEClient{}
	result, err := (ProfileTester{BLEConnector: &fakeBLEConnector{client: client}}).Test(nil, types.DeviceProfileTestParams{
		Action:     "setSpeed",
		SpeedValue: 55.5,
		Profile: types.DeviceProfile{
			ID:          "test.ble",
			DisplayName: "Test BLE",
			Transport:   types.DeviceTransportBLE,
			SpeedUnit:   types.FanSpeedUnitPercent,
			SpeedRange:  types.DefaultPercentSpeedRange(),
			Connection: types.DeviceConnectionSettings{
				BLEServiceUUID:         "fff0",
				BLEWriteCharacteristic: "fff2",
			},
			Commands: []types.DeviceCommandTemplate{
				{Name: "setSpeed", Command: "P{{percent}} T{{percentTicks}}", Encoding: "ascii"},
			},
			Capabilities: types.DeviceCapabilities{
				Transport:        types.DeviceTransportBLE,
				SpeedUnit:        types.FanSpeedUnitPercent,
				SpeedRange:       types.DefaultPercentSpeedRange(),
				SupportsSetSpeed: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if len(client.writes) != 1 || string(client.writes[0].payload) != "P56 T555" {
		t.Fatalf("writes = %#v, want P56 T555", client.writes)
	}
	if !client.closed {
		t.Fatal("expected transient BLE test to close the client")
	}
	if result.FanData == nil || result.FanData.CurrentRPM != 56 || result.FanData.SpeedUnit != types.FanSpeedUnitPercent {
		t.Fatalf("fan data = %#v, want synthetic 56 percent", result.FanData)
	}
}

func TestProfileTesterAppliesHexChecksumForSerialDraft(t *testing.T) {
	port := &fakeSerialPort{}
	result, err := (ProfileTester{SerialDialer: fakeSerialDialer{port: port}}).Test(nil, types.DeviceProfileTestParams{
		Action:     "setSpeed",
		SpeedValue: 10,
		Profile: types.DeviceProfile{
			ID:          "test.serial.checksum",
			DisplayName: "Checksum serial",
			Transport:   types.DeviceTransportSerial,
			SpeedUnit:   types.FanSpeedUnitPercent,
			SpeedRange:  types.DefaultPercentSpeedRange(),
			Connection: types.DeviceConnectionSettings{
				SerialPort:     "COM7",
				SerialBaudRate: 115200,
			},
			Commands: []types.DeviceCommandTemplate{
				{Name: "setSpeed", Command: "A5 {{percent}}", Encoding: "hex", Checksum: "xor8"},
			},
			Capabilities: types.DeviceCapabilities{
				Transport:        types.DeviceTransportSerial,
				SpeedUnit:        types.FanSpeedUnitPercent,
				SpeedRange:       types.DefaultPercentSpeedRange(),
				SupportsSetSpeed: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	want := []byte{0xa5, 0x10, 0xb5}
	if len(port.writes) != 1 || !bytes.Equal(port.writes[0], want) {
		t.Fatalf("writes = % X, want % X", port.writes, want)
	}
	if !port.closed {
		t.Fatal("expected transient serial test to close the port")
	}
	if result.FanData == nil || result.FanData.CurrentRPM != 10 || result.FanData.SpeedUnit != types.FanSpeedUnitPercent {
		t.Fatalf("fan data = %#v, want synthetic 10 percent", result.FanData)
	}
}
