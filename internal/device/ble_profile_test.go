//go:build !legacydevice

package device

import (
	"context"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceprofileexec"
	"github.com/TIANLI0/THRM/internal/types"
)

type managerFakeBLEClient struct {
	reads    [][]byte
	writes   []managerFakeBLEWrite
	writeErr error
	closed   bool
}

type managerFakeBLEWrite struct {
	payload      []byte
	withResponse bool
}

func (c *managerFakeBLEClient) WriteBLECommand(ctx context.Context, payload []byte, withResponse bool) error {
	c.writes = append(c.writes, managerFakeBLEWrite{
		payload:      append([]byte(nil), payload...),
		withResponse: withResponse,
	})
	return c.writeErr
}

func (c *managerFakeBLEClient) ReadBLEFrame(ctx context.Context) ([]byte, error) {
	if len(c.reads) == 0 {
		return []byte("speed=0"), nil
	}
	next := c.reads[0]
	c.reads = c.reads[1:]
	return append([]byte(nil), next...), nil
}

func (c *managerFakeBLEClient) Close() error {
	c.closed = true
	return nil
}

func TestBLEManagerProfileConnectsAndSetsPercentSpeed(t *testing.T) {
	client := &managerFakeBLEClient{
		reads: [][]byte{
			[]byte("speed=28"),
			[]byte("speed=45"),
		},
	}
	profile := types.DeviceProfile{
		ID:          "user.manager.ble.percent",
		DisplayName: "BLE percent",
		Vendor:      "DIY",
		Model:       "BLE cooler",
		Transport:   types.DeviceTransportBLE,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			Endpoint:                "AA:BB:CC:DD:EE:01",
			BLENameFilter:           "cooler",
			BLEServiceUUID:          "fff0",
			BLEWriteCharacteristic:  "fff2",
			BLENotifyCharacteristic: "fff1",
			BLEWriteWithResponse:    true,
		},
		Commands: []types.DeviceCommandTemplate{
			{Name: "readState", Command: "READ", Encoding: "ascii"},
			{Name: "setSpeed", Command: "SET {{percent}} {{percentTicks}}", Encoding: "ascii"},
		},
		ResponseParsers: []types.DeviceResponseParser{
			{Name: "current speed", Type: "regex", Expression: `speed=(\d+)`},
		},
		Capabilities: types.DeviceCapabilities{
			Transport:         types.DeviceTransportBLE,
			SpeedUnit:         types.FanSpeedUnitPercent,
			SpeedRange:        types.DefaultPercentSpeedRange(),
			SupportsReadState: true,
			SupportsSetSpeed:  true,
		},
	}

	m := NewManager(nil)
	m.bleConnector = deviceprofileexec.BLEConnectorFunc(func(ctx context.Context, profile types.DeviceProfile) (deviceprofileexec.BLEClient, error) {
		return client, nil
	})
	m.ConfigureProfile(profile, "")

	connected, info := m.Connect()
	if !connected {
		t.Fatal("expected BLE profile to connect")
	}
	if info["transport"] != types.DeviceTransportBLE || info["endpoint"] != "AA:BB:CC:DD:EE:01" {
		t.Fatalf("BLE info = %#v, want ble endpoint", info)
	}
	if m.GetDeviceType() != types.DeviceTransportBLE {
		t.Fatalf("device type = %q, want ble", m.GetDeviceType())
	}
	if len(client.writes) != 1 || string(client.writes[0].payload) != "READ" || !client.writes[0].withResponse {
		t.Fatalf("connect writes = %#v, want READ with response", client.writes)
	}

	if ok := m.SetTargetSpeed(445, types.FanSpeedUnitPercent); !ok {
		t.Fatal("expected BLE percent target speed to be sent")
	}
	if len(client.writes) != 2 || string(client.writes[1].payload) != "SET 45 445" || !client.writes[1].withResponse {
		t.Fatalf("set writes = %#v, want SET 45 445 with response", client.writes)
	}
	fanData := m.GetCurrentFanData()
	if fanData == nil {
		t.Fatal("expected BLE fan data")
	}
	if fanData.CurrentRPM != 45 || fanData.TargetRPM != 45 {
		t.Fatalf("fan data speed = %d/%d, want 45/45", fanData.CurrentRPM, fanData.TargetRPM)
	}
	if fanData.Transport != types.DeviceTransportBLE || fanData.SpeedUnit != types.FanSpeedUnitPercent {
		t.Fatalf("fan data transport/unit = %q/%q", fanData.Transport, fanData.SpeedUnit)
	}

	settings, err := m.QueryDeviceSettings()
	if err != nil {
		t.Fatalf("QueryDeviceSettings() error = %v", err)
	}
	if settings.Source != types.DeviceTransportBLE || settings.Model != "BLE percent" {
		t.Fatalf("settings = %#v, want BLE model", settings)
	}

	m.Disconnect()
	if !client.closed {
		t.Fatal("expected BLE client to close on disconnect")
	}
}

func TestBLEManagerRefreshBS1ReadsNotifyState(t *testing.T) {
	client := &managerFakeBLEClient{}
	m := NewManager(nil)
	m.bleConnector = deviceprofileexec.BLEConnectorFunc(func(ctx context.Context, profile types.DeviceProfile) (deviceprofileexec.BLEClient, error) {
		return client, nil
	})
	m.ConfigureProfile(types.FlyDigiBS1Profile(), "")
	defer m.Disconnect()

	connected, _ := m.Connect()
	if !connected {
		t.Fatal("expected BS1 BLE profile to connect")
	}
	if ok := m.SetTargetSpeed(1800, types.FanSpeedUnitRPM); !ok {
		t.Fatal("expected BS1 RPM target speed to be sent")
	}
	before := m.GetCurrentFanData()
	if before == nil || before.TargetRPM != 1800 {
		t.Fatalf("before refresh fan data = %#v, want target 1800", before)
	}
	if !m.RefreshBLEState() {
		t.Fatal("expected BS1 BLE refresh to succeed")
	}
	after := m.GetCurrentFanData()
	if after == nil || after.CurrentRPM != 0 || after.TargetRPM != 1800 {
		t.Fatalf("after refresh fan data = %#v, want cached current/target 0/1800", after)
	}
}

func TestBLEManagerFailedBS1WriteMarksDisconnectedAndNotifiesCore(t *testing.T) {
	client := &managerFakeBLEClient{writeErr: context.Canceled}
	m := NewManager(nil)
	m.bleConnector = deviceprofileexec.BLEConnectorFunc(func(ctx context.Context, profile types.DeviceProfile) (deviceprofileexec.BLEClient, error) {
		return client, nil
	})
	disconnected := make(chan struct{}, 1)
	m.SetCallbacks(nil, func() { disconnected <- struct{}{} })
	m.ConfigureProfile(types.FlyDigiBS1Profile(), "")

	connected, _ := m.Connect()
	if !connected {
		t.Fatal("expected BS1 BLE profile to connect")
	}
	if m.SetTargetSpeed(1800, types.FanSpeedUnitRPM) {
		t.Fatal("expected failed BS1 speed write")
	}
	if m.IsConnected() || m.GetDeviceType() != "" || m.GetCurrentFanData() != nil {
		t.Fatalf("failed BS1 write left stale manager state: connected=%v type=%q data=%#v", m.IsConnected(), m.GetDeviceType(), m.GetCurrentFanData())
	}
	select {
	case <-disconnected:
	case <-time.After(time.Second):
		t.Fatal("failed BS1 write did not notify Core")
	}
}
