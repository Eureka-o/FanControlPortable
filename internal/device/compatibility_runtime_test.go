//go:build !legacydevice

package device

import (
	"context"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func compatibilitySerialTestProfile(id string, supportsSetSpeed bool) types.DeviceProfile {
	commands := []types.DeviceCommandTemplate{{Name: "readState", Command: "GET", Encoding: "ascii"}}
	if supportsSetSpeed {
		commands = append(commands, types.DeviceCommandTemplate{Name: "setSpeed", Command: "SET {{percent}} {{percentTicks}}", Encoding: "ascii"})
	}
	return types.DeviceProfile{
		ID:        id,
		Transport: types.DeviceTransportSerial,
		SpeedUnit: types.FanSpeedUnitPercent,
		Connection: types.DeviceConnectionSettings{
			SerialPort:           "COM42",
			SerialBaudRate:       115200,
			SerialFrameDelimiter: `\n`,
		},
		Commands: commands,
		ResponseParsers: []types.DeviceResponseParser{
			{Name: "current speed", Type: "regex", Expression: `speed=(\d+)`},
		},
		Capabilities: types.DeviceCapabilities{
			Transport:         types.DeviceTransportSerial,
			SpeedUnit:         types.FanSpeedUnitPercent,
			SpeedRange:        types.DefaultPercentSpeedRange(),
			SupportsReadState: true,
			SupportsSetSpeed:  supportsSetSpeed,
		},
	}
}

func TestCompatibilityRuntimeConnectsSerialAdapter(t *testing.T) {
	port := &managerFakeSerialPort{reads: [][]byte{[]byte("speed=25\n")}}
	manager := NewManager(nil)
	manager.serialDialer = managerFakeSerialDialer{port: port}
	manager.ConfigureProfile(types.DeviceProfile{
		ID:          "compat.serial",
		DisplayName: "Compatibility serial",
		Transport:   types.DeviceTransportSerial,
		SpeedUnit:   types.FanSpeedUnitPercent,
		Connection: types.DeviceConnectionSettings{
			SerialPort:           "COM42",
			SerialBaudRate:       115200,
			SerialFrameDelimiter: `\n`,
		},
		Commands: []types.DeviceCommandTemplate{
			{Name: "readState", Command: "GET", Encoding: "ascii"},
		},
		ResponseParsers: []types.DeviceResponseParser{
			{Name: "current speed", Type: "regex", Expression: `speed=(\d+)`},
		},
		Capabilities: types.DeviceCapabilities{
			Transport:         types.DeviceTransportSerial,
			SpeedUnit:         types.FanSpeedUnitPercent,
			SpeedRange:        types.DefaultPercentSpeedRange(),
			SupportsReadState: true,
		},
	}, "")

	manager.mutex.Lock()
	connected, info, handled := (compatibilityRuntime{}).connectLocked(context.Background(), manager)
	manager.mutex.Unlock()

	if !handled || !connected {
		t.Fatalf("connectLocked() handled/connected = %v/%v, want true/true", handled, connected)
	}
	if info["transport"] != types.DeviceTransportSerial || info["endpoint"] != "COM42" {
		t.Fatalf("connection info = %#v", info)
	}
}

func TestCompatibilityRuntimeSetsSerialTargetSpeed(t *testing.T) {
	port := &managerFakeSerialPort{reads: [][]byte{[]byte("speed=25\n"), []byte("speed=66\n")}}
	manager := NewManager(nil)
	manager.serialDialer = managerFakeSerialDialer{port: port}
	manager.ConfigureProfile(types.DeviceProfile{
		ID:        "compat.serial.speed",
		Transport: types.DeviceTransportSerial,
		SpeedUnit: types.FanSpeedUnitPercent,
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
	}, "")
	if connected, _ := manager.Connect(); !connected {
		t.Fatal("serial compatibility adapter did not connect")
	}

	manager.mutex.Lock()
	written, handled := (compatibilityRuntime{}).setTargetSpeedLocked(context.Background(), manager, 655, types.FanSpeedUnitPercent)
	manager.mutex.Unlock()

	if !handled || !written {
		t.Fatalf("setTargetSpeedLocked() handled/written = %v/%v, want true/true", handled, written)
	}
	if len(port.writes) != 2 || string(port.writes[1]) != "SET 66 655\n" {
		t.Fatalf("serial writes = %q", port.writes)
	}
}

func TestCompatibilityRuntimeSetsSerialPercentSpeed(t *testing.T) {
	port := &managerFakeSerialPort{reads: [][]byte{[]byte("speed=25\n"), []byte("speed=66\n")}}
	manager := NewManager(nil)
	manager.serialDialer = managerFakeSerialDialer{port: port}
	manager.ConfigureProfile(compatibilitySerialTestProfile("compat.serial.percent", true), "")
	if connected, _ := manager.Connect(); !connected {
		t.Fatal("serial compatibility adapter did not connect")
	}

	manager.mutex.Lock()
	written, handled := (compatibilityRuntime{}).setPercentSpeedLocked(context.Background(), manager, 66)
	manager.mutex.Unlock()

	if !handled || !written {
		t.Fatalf("setPercentSpeedLocked() handled/written = %v/%v, want true/true", handled, written)
	}
	if len(port.writes) != 2 || string(port.writes[1]) != "SET 66 660\n" {
		t.Fatalf("serial writes = %q", port.writes)
	}
}

func TestCompatibilityRuntimeDisconnectsSerialAdapter(t *testing.T) {
	port := &managerFakeSerialPort{reads: [][]byte{[]byte("speed=25\n")}}
	manager := NewManager(nil)
	manager.serialDialer = managerFakeSerialDialer{port: port}
	manager.Configure(types.DeviceTransportSerial, "COM42")
	if connected, _ := manager.Connect(); !connected {
		t.Fatal("serial compatibility adapter did not connect")
	}

	manager.mutex.Lock()
	handled := (compatibilityRuntime{}).disconnectLocked(manager)
	manager.mutex.Unlock()

	if !handled || manager.IsConnected() {
		t.Fatalf("disconnectLocked() handled/connected = %v/%v, want true/false", handled, manager.IsConnected())
	}
	if !port.closed {
		t.Fatal("serial compatibility adapter was not closed")
	}
}

func TestCompatibilityRuntimeRefreshesSerialAdapter(t *testing.T) {
	port := &managerFakeSerialPort{reads: [][]byte{[]byte("speed=25\n"), []byte("speed=40\n")}}
	manager := NewManager(nil)
	manager.serialDialer = managerFakeSerialDialer{port: port}
	manager.ConfigureProfile(compatibilitySerialTestProfile("compat.serial.refresh", false), "")
	if connected, _ := manager.Connect(); !connected {
		t.Fatal("serial compatibility adapter did not connect")
	}

	healthy, handled := (compatibilityRuntime{}).refresh(manager)
	if !handled || !healthy {
		t.Fatalf("refresh() handled/healthy = %v/%v, want true/true", handled, healthy)
	}
	if fanData := manager.GetCurrentFanData(); fanData == nil || fanData.CurrentRPM != 40 {
		t.Fatalf("refreshed fan data = %#v, want current speed 40", fanData)
	}
}
