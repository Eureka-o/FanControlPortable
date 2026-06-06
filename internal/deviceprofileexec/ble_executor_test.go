package deviceprofileexec

import (
	"context"
	"testing"
	"time"

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
	reads  [][]byte
	writes []fakeBLEWrite
	closed bool
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
	return nil
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
	if state.CurrentRPM != 56 || state.TargetRPM != 56 {
		t.Fatalf("synthetic state = %d/%d, want 56/56", state.CurrentRPM, state.TargetRPM)
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
