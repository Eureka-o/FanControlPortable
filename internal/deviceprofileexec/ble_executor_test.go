package deviceprofileexec

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

type fakeBLEConnector struct {
	client  *fakeBLEClient
	profile types.DeviceProfile
}

func (c *fakeBLEConnector) ConnectBLEDevice(ctx context.Context, profile types.DeviceProfile) (BLEClient, error) {
	c.profile = profile
	return c.client, nil
}

type fakeBLEClient struct {
	reads    [][]byte
	writes   []fakeBLEWrite
	writeErr error
	closed   bool
}

type fakeBLEWrite struct {
	payload      []byte
	withResponse bool
}

func (c *fakeBLEClient) WriteBLECommand(ctx context.Context, payload []byte, withResponse bool) error {
	c.writes = append(c.writes, fakeBLEWrite{
		payload:      append([]byte(nil), payload...),
		withResponse: withResponse,
	})
	return c.writeErr
}

func (c *fakeBLEClient) ReadBLEFrame(ctx context.Context) ([]byte, error) {
	if len(c.reads) == 0 {
		return nil, context.DeadlineExceeded
	}
	next := c.reads[0]
	c.reads = c.reads[1:]
	return append([]byte(nil), next...), nil
}

func (c *fakeBLEClient) Close() error {
	c.closed = true
	return nil
}

func TestBLEExecutorOpenReadsStateAndSetSpeed(t *testing.T) {
	client := &fakeBLEClient{reads: [][]byte{
		[]byte("speed=25"),
		[]byte("speed=66"),
	}}
	connector := &fakeBLEConnector{client: client}
	executor, err := NewBLEExecutor(types.DeviceProfile{
		ID:          "user.ble.percent",
		DisplayName: "BLE percent",
		Transport:   types.DeviceTransportBLE,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
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
	}, connector)
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}

	state, err := executor.Open(nil)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if state.CurrentRPM != 25 || state.TargetRPM != 25 {
		t.Fatalf("open state = %d/%d, want 25/25", state.CurrentRPM, state.TargetRPM)
	}
	if len(client.writes) != 1 || string(client.writes[0].payload) != "READ" || !client.writes[0].withResponse {
		t.Fatalf("open writes = %#v, want READ with response", client.writes)
	}

	state, err = executor.SetSpeed(nil, types.NewPercentTickSpeed(655))
	if err != nil {
		t.Fatalf("SetSpeed() error = %v", err)
	}
	if len(client.writes) != 2 || string(client.writes[1].payload) != "SET 66 655" || !client.writes[1].withResponse {
		t.Fatalf("set writes = %#v, want SET 66 655 with response", client.writes)
	}
	if state.CurrentRPM != 66 || state.TargetRPM != 66 || state.Transport != types.DeviceTransportBLE {
		t.Fatalf("set state = %#v, want 66 BLE", state)
	}
	if connector.profile.Transport != types.DeviceTransportBLE {
		t.Fatalf("connector profile transport = %q, want ble", connector.profile.Transport)
	}
}

func TestBLEExecutorSetSpeedWithoutParserUsesSyntheticState(t *testing.T) {
	client := &fakeBLEClient{}
	executor, err := NewBLEExecutor(types.DeviceProfile{
		ID:          "user.ble.target-only",
		DisplayName: "BLE target only",
		Transport:   types.DeviceTransportBLE,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			BLEServiceUUID:         "fff0",
			BLEWriteCharacteristic: "fff2",
		},
		Commands: []types.DeviceCommandTemplate{
			{Name: "setSpeed", Command: "P{{percent}}", Encoding: "ascii"},
		},
		Capabilities: types.DeviceCapabilities{
			Transport:        types.DeviceTransportBLE,
			SpeedUnit:        types.FanSpeedUnitPercent,
			SpeedRange:       types.DefaultPercentSpeedRange(),
			SupportsSetSpeed: true,
		},
	}, &fakeBLEConnector{client: client})
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}

	state, err := executor.SetSpeed(nil, types.NewPercentTickSpeed(555))
	if err != nil {
		t.Fatalf("SetSpeed() error = %v", err)
	}
	if len(client.writes) != 1 || string(client.writes[0].payload) != "P56" || client.writes[0].withResponse {
		t.Fatalf("writes = %#v, want P56 without response", client.writes)
	}
	if state.CurrentRPM != 0 || state.TargetRPM != 56 {
		t.Fatalf("synthetic state = %d/%d, want 0/56", state.CurrentRPM, state.TargetRPM)
	}

	executor.lastState = &types.FanData{CurrentRPM: 31, TargetRPM: 56, Transport: types.DeviceTransportBLE, SpeedUnit: types.FanSpeedUnitPercent}
	state, err = executor.SetSpeed(nil, types.NewPercentTickSpeed(655))
	if err != nil {
		t.Fatalf("second SetSpeed() error = %v", err)
	}
	if state.CurrentRPM != 31 || state.TargetRPM != 66 {
		t.Fatalf("synthetic state after previous current = %d/%d, want 31/66", state.CurrentRPM, state.TargetRPM)
	}
}

func TestBLEExecutorFlyDigiBS1SetSpeedDoesNotFakeCurrentRPM(t *testing.T) {
	client := &fakeBLEClient{}
	executor, err := NewBLEExecutor(types.FlyDigiBS1Profile(), &fakeBLEConnector{client: client})
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}
	executor.lastState = &types.FanData{CurrentRPM: 1234, TargetRPM: 1300, Transport: types.DeviceTransportBLE, SpeedUnit: types.FanSpeedUnitRPM}

	state, err := executor.SetSpeed(nil, types.NewRPMSpeed(1800))
	if err != nil {
		t.Fatalf("SetSpeed() error = %v", err)
	}
	if len(client.writes) != 2 {
		t.Fatalf("writes = %d, want enter dynamic + set rpm", len(client.writes))
	}
	if state.CurrentRPM != 1234 || state.TargetRPM != 1800 {
		t.Fatalf("BS1 state = %d/%d, want 1234/1800", state.CurrentRPM, state.TargetRPM)
	}
}

func TestBLEExecutorFlyDigiBS1HeartbeatLifecycle(t *testing.T) {
	client := &fakeBLEClient{}
	executor, err := NewBLEExecutor(types.FlyDigiBS1Profile(), &fakeBLEConnector{client: client})
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}

	state, err := executor.Open(nil)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if state.Transport != types.DeviceTransportBLE || state.SpeedUnit != types.FanSpeedUnitRPM {
		t.Fatalf("open state = %#v, want BS1 BLE RPM state", state)
	}
	if executor.heartbeatStop == nil {
		t.Fatal("BS1 heartbeat should start after opening the BLE executor")
	}

	if err := executor.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if executor.heartbeatStop != nil {
		t.Fatal("BS1 heartbeat stop channel should be cleared after Close")
	}
	if !client.closed {
		t.Fatal("BLE client should be closed")
	}
}

func TestBLEExecutorFlyDigiBS1ReadStateConsumesStatusNotification(t *testing.T) {
	client := &fakeBLEClient{reads: [][]byte{
		deviceproto.BuildFrame(deviceproto.CmdRGBStatus),
		deviceproto.BuildFrame(deviceproto.CmdStatusNotify, 1, 2, 0, 0xD2, 0x04, 0x08, 0x07),
	}}
	executor, err := NewBLEExecutor(types.FlyDigiBS1Profile(), &fakeBLEConnector{client: client})
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}
	defer executor.Close()

	state, err := executor.ReadState(nil)
	if err != nil {
		t.Fatalf("ReadState() error = %v", err)
	}
	if state.CurrentRPM != 1234 || state.TargetRPM != 1800 {
		t.Fatalf("BS1 notification state = %d/%d, want 1234/1800", state.CurrentRPM, state.TargetRPM)
	}
}

func TestBLEExecutorFlyDigiBS1HeartbeatFailureInvalidatesAndReconnects(t *testing.T) {
	failed := &fakeBLEClient{writeErr: io.ErrClosedPipe}
	connector := &fakeBLEConnector{client: failed}
	executor, err := NewBLEExecutor(types.FlyDigiBS1Profile(), connector)
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}
	executor.client = failed
	executor.heartbeatStop = make(chan struct{})

	if err := executor.writeHeartbeat(context.Background(), types.BS1CmdHeartbeat1); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("writeHeartbeat() error = %v, want closed pipe", err)
	}
	if executor.client != nil || executor.heartbeatStop != nil || !failed.closed {
		t.Fatalf("failed heartbeat left stale client: client=%v stop=%v closed=%v", executor.client, executor.heartbeatStop, failed.closed)
	}

	reconnected := &fakeBLEClient{reads: [][]byte{
		deviceproto.BuildFrame(deviceproto.CmdStatusNotify, 1, 2, 0, 0xDC, 0x05, 0x40, 0x06),
	}}
	connector.client = reconnected
	state, err := executor.ReadState(nil)
	if err != nil {
		t.Fatalf("ReadState() after heartbeat failure error = %v", err)
	}
	defer executor.Close()
	if state.CurrentRPM != 1500 || state.TargetRPM != 1600 {
		t.Fatalf("reconnected state = %d/%d, want 1500/1600", state.CurrentRPM, state.TargetRPM)
	}
}

func TestBLEExecutorRateLimitsSpeedSends(t *testing.T) {
	client := &fakeBLEClient{}
	executor, err := NewBLEExecutor(types.DeviceProfile{
		ID:          "user.ble.rate",
		DisplayName: "BLE rate",
		Transport:   types.DeviceTransportBLE,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			BLEServiceUUID:         "fff0",
			BLEWriteCharacteristic: "fff2",
			MinSendIntervalMs:      120,
		},
		Commands: []types.DeviceCommandTemplate{
			{Name: "setSpeed", Command: "P{{percent}}", Encoding: "ascii"},
		},
		Capabilities: types.DeviceCapabilities{
			Transport:        types.DeviceTransportBLE,
			SpeedUnit:        types.FanSpeedUnitPercent,
			SpeedRange:       types.DefaultPercentSpeedRange(),
			SupportsSetSpeed: true,
		},
	}, &fakeBLEConnector{client: client})
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
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
	if slept != 120*time.Millisecond {
		t.Fatalf("slept = %v, want 120ms", slept)
	}
}
