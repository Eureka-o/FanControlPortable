package deviceprofileexec

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

type fakeSerialDialer struct {
	port *fakeSerialPort
}

func (d fakeSerialDialer) OpenSerialPort(profile types.DeviceProfile) (SerialPort, error) {
	return d.port, nil
}

type fakeSerialPort struct {
	reads  [][]byte
	writes [][]byte
	closed bool
}

func (p *fakeSerialPort) Read(buf []byte) (int, error) {
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

func (p *fakeSerialPort) Write(buf []byte) (int, error) {
	p.writes = append(p.writes, append([]byte(nil), buf...))
	return len(buf), nil
}

func (p *fakeSerialPort) Close() error {
	p.closed = true
	return nil
}

func TestSerialExecutorSetSpeedWritesCommandAndParsesResponse(t *testing.T) {
	port := &fakeSerialPort{reads: [][]byte{[]byte("ok speed=56\n")}}
	executor, err := NewSerialExecutor(types.DeviceProfile{
		ID:          "user.serial.percent",
		DisplayName: "Serial percent",
		Transport:   types.DeviceTransportSerial,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			SerialPort:           "COM9",
			SerialBaudRate:       115200,
			SerialDataBits:       8,
			SerialStopBits:       1,
			SerialParity:         "none",
			SerialFrameDelimiter: `\n`,
		},
		Commands: []types.DeviceCommandTemplate{
			{Name: "setSpeed", Command: "SET {{percent}} {{percentTicks}}", Encoding: "ascii"},
		},
		ResponseParsers: []types.DeviceResponseParser{
			{Name: "current speed", Type: "regex", Expression: `speed=(\d+)`},
		},
		Capabilities: types.DeviceCapabilities{
			Transport:         types.DeviceTransportSerial,
			SpeedUnit:         types.FanSpeedUnitPercent,
			SpeedRange:        types.DefaultPercentSpeedRange(),
			SupportsSetSpeed:  true,
			SupportsReadState: true,
		},
	}, fakeSerialDialer{port: port})
	if err != nil {
		t.Fatalf("NewSerialExecutor() error = %v", err)
	}

	state, err := executor.SetSpeed(nil, types.NewPercentTickSpeed(555))
	if err != nil {
		t.Fatalf("SetSpeed() error = %v", err)
	}
	if len(port.writes) != 1 || string(port.writes[0]) != "SET 56 555\n" {
		t.Fatalf("writes = %q, want SET 56 555 newline", port.writes)
	}
	if state.CurrentRPM != 56 || state.TargetRPM != 56 {
		t.Fatalf("state speed = %d/%d, want 56/56", state.CurrentRPM, state.TargetRPM)
	}
	if state.Transport != types.DeviceTransportSerial || state.SpeedUnit != types.FanSpeedUnitPercent {
		t.Fatalf("state transport/unit = %q/%q", state.Transport, state.SpeedUnit)
	}
}

func TestSerialExecutorOpenReadsStateWhenReadCommandExists(t *testing.T) {
	port := &fakeSerialPort{reads: [][]byte{[]byte("rpm:1800\r\n")}}
	executor, err := NewSerialExecutor(types.DeviceProfile{
		ID:          "user.serial.rpm",
		DisplayName: "Serial RPM",
		Transport:   types.DeviceTransportSerial,
		SpeedUnit:   types.FanSpeedUnitRPM,
		SpeedRange:  types.DefaultRPMSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			SerialPort:           "COM10",
			SerialBaudRate:       115200,
			SerialFrameDelimiter: `\r\n`,
		},
		Commands: []types.DeviceCommandTemplate{
			{Name: "readState", Command: "GET", Encoding: "ascii"},
		},
		ResponseParsers: []types.DeviceResponseParser{
			{Name: "current rpm", Type: "regex", Expression: `rpm:(\d+)`},
		},
		Capabilities: types.DeviceCapabilities{
			Transport:         types.DeviceTransportSerial,
			SpeedUnit:         types.FanSpeedUnitRPM,
			SpeedRange:        types.DefaultRPMSpeedRange(),
			SupportsReadState: true,
		},
	}, fakeSerialDialer{port: port})
	if err != nil {
		t.Fatalf("NewSerialExecutor() error = %v", err)
	}

	state, err := executor.Open(nil)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if len(port.writes) != 1 || string(port.writes[0]) != "GET\r\n" {
		t.Fatalf("writes = %q, want GET CRLF", port.writes)
	}
	if state.CurrentRPM != 1800 || state.TargetRPM != 1800 || state.SpeedUnit != types.FanSpeedUnitRPM {
		t.Fatalf("state = %#v, want 1800 RPM", state)
	}
	if err := executor.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !port.closed {
		t.Fatal("expected serial port to be closed")
	}
}

func TestSerialExecutorRateLimitsSpeedSends(t *testing.T) {
	port := &fakeSerialPort{}
	executor, err := NewSerialExecutor(types.DeviceProfile{
		ID:          "user.serial.target-only",
		DisplayName: "Serial target only",
		Transport:   types.DeviceTransportSerial,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			SerialPort:        "COM11",
			MinSendIntervalMs: 100,
		},
		Commands: []types.DeviceCommandTemplate{
			{Name: "setSpeed", Command: "P{{percent}}", Encoding: "ascii"},
		},
		Capabilities: types.DeviceCapabilities{
			Transport:        types.DeviceTransportSerial,
			SpeedUnit:        types.FanSpeedUnitPercent,
			SpeedRange:       types.DefaultPercentSpeedRange(),
			SupportsSetSpeed: true,
		},
	}, fakeSerialDialer{port: port})
	if err != nil {
		t.Fatalf("NewSerialExecutor() error = %v", err)
	}
	now := time.Unix(100, 0)
	var slept time.Duration
	executor.now = func() time.Time { return now.Add(slept) }
	executor.sleep = func(_ context.Context, d time.Duration) error {
		slept += d
		return nil
	}

	if _, err := executor.SetSpeed(nil, types.NewPercentSpeed(20)); err != nil {
		t.Fatalf("first SetSpeed() error = %v", err)
	}
	if _, err := executor.SetSpeed(nil, types.NewPercentSpeed(30)); err != nil {
		t.Fatalf("second SetSpeed() error = %v", err)
	}
	if slept != 100*time.Millisecond {
		t.Fatalf("slept = %v, want 100ms", slept)
	}
}

func TestSerialExecutorSetSpeedWithoutReadbackDoesNotFakeCurrentSpeed(t *testing.T) {
	port := &fakeSerialPort{}
	executor, err := NewSerialExecutor(types.DeviceProfile{
		ID:          "user.serial.target-only",
		DisplayName: "Serial target only",
		Transport:   types.DeviceTransportSerial,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			SerialPort: "COM12",
		},
		Commands: []types.DeviceCommandTemplate{
			{Name: "setSpeed", Command: "P{{percent}}", Encoding: "ascii"},
		},
		Capabilities: types.DeviceCapabilities{
			Transport:        types.DeviceTransportSerial,
			SpeedUnit:        types.FanSpeedUnitPercent,
			SpeedRange:       types.DefaultPercentSpeedRange(),
			SupportsSetSpeed: true,
		},
	}, fakeSerialDialer{port: port})
	if err != nil {
		t.Fatalf("NewSerialExecutor() error = %v", err)
	}

	state, err := executor.SetSpeed(nil, types.NewPercentTickSpeed(555))
	if err != nil {
		t.Fatalf("SetSpeed() error = %v", err)
	}
	if state.CurrentRPM != 0 || state.TargetRPM != 56 {
		t.Fatalf("synthetic state = %d/%d, want 0/56", state.CurrentRPM, state.TargetRPM)
	}

	executor.lastState = &types.FanData{CurrentRPM: 44, TargetRPM: 56, Transport: types.DeviceTransportSerial, SpeedUnit: types.FanSpeedUnitPercent}
	state, err = executor.SetSpeed(nil, types.NewPercentTickSpeed(655))
	if err != nil {
		t.Fatalf("second SetSpeed() error = %v", err)
	}
	if state.CurrentRPM != 44 || state.TargetRPM != 66 {
		t.Fatalf("synthetic state after previous current = %d/%d, want 44/66", state.CurrentRPM, state.TargetRPM)
	}
}

func TestDecodeSerialDelimiter(t *testing.T) {
	tests := map[string]string{
		`\n`:   "\n",
		`\r`:   "\r",
		`\r\n`: "\r\n",
		"lf":   "\n",
		"crlf": "\r\n",
		";":    ";",
	}
	for input, want := range tests {
		got, err := DecodeSerialDelimiter(input)
		if err != nil {
			t.Fatalf("DecodeSerialDelimiter(%q) error = %v", input, err)
		}
		if string(got) != want {
			t.Fatalf("DecodeSerialDelimiter(%q) = %q, want %q", input, got, want)
		}
	}
}
