package device

import (
	"io"
	"testing"

	"github.com/TIANLI0/THRM/internal/deviceprofileexec"
	"github.com/TIANLI0/THRM/internal/types"
)

type managerFakeSerialDialer struct {
	port *managerFakeSerialPort
}

func (d managerFakeSerialDialer) OpenSerialPort(profile types.DeviceProfile) (deviceprofileexec.SerialPort, error) {
	return d.port, nil
}

type managerFakeSerialPort struct {
	reads  [][]byte
	writes [][]byte
	closed bool
}

func (p *managerFakeSerialPort) Read(buf []byte) (int, error) {
	if len(p.reads) == 0 {
		return 0, io.EOF
	}
	next := p.reads[0]
	p.reads = p.reads[1:]
	n := copy(buf, next)
	if n < len(next) {
		p.reads = append([][]byte{next[n:]}, p.reads...)
	}
	return n, nil
}

func (p *managerFakeSerialPort) Write(buf []byte) (int, error) {
	p.writes = append(p.writes, append([]byte(nil), buf...))
	return len(buf), nil
}

func (p *managerFakeSerialPort) Close() error {
	p.closed = true
	return nil
}

func TestSerialManagerProfileConnectsAndSetsPercentSpeed(t *testing.T) {
	port := &managerFakeSerialPort{
		reads: [][]byte{
			[]byte("speed=25\n"),
			[]byte("speed=66\n"),
		},
	}
	profile := types.DeviceProfile{
		ID:          "user.manager.serial.percent",
		DisplayName: "Serial percent",
		Vendor:      "DIY",
		Model:       "COM controller",
		Transport:   types.DeviceTransportSerial,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			SerialPort:           "COM42",
			SerialBaudRate:       115200,
			SerialFrameDelimiter: `\n`,
		},
		Commands: []types.DeviceCommandTemplate{
			{Name: "readState", Command: "GET", Encoding: "ascii"},
			{Name: "setSpeed", Command: "SET {{percent}} {{percentTicks}}", Encoding: "ascii"},
		},
		ResponseParsers: []types.DeviceResponseParser{
			{Name: "current speed", Type: "regex", Expression: `speed=(\d+)`},
		},
		Capabilities: types.DeviceCapabilities{
			Transport:         types.DeviceTransportSerial,
			SpeedUnit:         types.FanSpeedUnitPercent,
			SpeedRange:        types.DefaultPercentSpeedRange(),
			SupportsReadState: true,
			SupportsSetSpeed:  true,
		},
	}

	m := NewManager(nil)
	m.serialDialer = managerFakeSerialDialer{port: port}
	m.ConfigureProfile(profile, "")

	connected, info := m.Connect()
	if !connected {
		t.Fatal("expected serial profile to connect")
	}
	if info["transport"] != types.DeviceTransportSerial || info["endpoint"] != "COM42" {
		t.Fatalf("serial info = %#v, want serial COM42", info)
	}
	if m.GetDeviceType() != types.DeviceTransportSerial {
		t.Fatalf("device type = %q, want serial", m.GetDeviceType())
	}
	if len(port.writes) != 1 || string(port.writes[0]) != "GET\n" {
		t.Fatalf("connect writes = %q, want GET newline", port.writes)
	}

	if ok := m.SetTargetSpeed(655, types.FanSpeedUnitPercent); !ok {
		t.Fatal("expected serial percent target speed to be sent")
	}
	if len(port.writes) != 2 || string(port.writes[1]) != "SET 66 655\n" {
		t.Fatalf("set writes = %q, want SET 66 655 newline", port.writes)
	}
	fanData := m.GetCurrentFanData()
	if fanData == nil {
		t.Fatal("expected serial fan data")
	}
	if fanData.CurrentRPM != 66 || fanData.TargetRPM != 66 {
		t.Fatalf("fan data speed = %d/%d, want 66/66", fanData.CurrentRPM, fanData.TargetRPM)
	}
	if fanData.Transport != types.DeviceTransportSerial || fanData.SpeedUnit != types.FanSpeedUnitPercent {
		t.Fatalf("fan data transport/unit = %q/%q", fanData.Transport, fanData.SpeedUnit)
	}

	settings, err := m.QueryDeviceSettings()
	if err != nil {
		t.Fatalf("QueryDeviceSettings() error = %v", err)
	}
	if settings.Source != types.DeviceTransportSerial || settings.Model != "Serial percent" {
		t.Fatalf("settings = %#v, want serial model", settings)
	}

	m.Disconnect()
	if !port.closed {
		t.Fatal("expected serial port to close on disconnect")
	}
}
