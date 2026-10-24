package deviceprofileexec

import (
	"context"
	"errors"
	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/types"
	"io"
	"testing"
	"time"
)

type fakeBLEConnector struct {
	client   *fakeBLEClient
	clients  []*fakeBLEClient
	profile  types.DeviceProfile
	connects int
}

func (c *fakeBLEConnector) ConnectBLEDevice(ctx context.Context, profile types.DeviceProfile) (BLEClient, error) {
	c.profile = profile
	c.connects++
	if len(c.clients) > 0 {
		client := c.clients[0]
		c.clients = c.clients[1:]
		return client, nil
	}
	return c.client, nil
}

type fakeBLEClient struct {
	reads    [][]byte
	writes   []fakeBLEWrite
	writeErr error
	closed   bool
	address  string
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

func (c *fakeBLEClient) BLEAddress() string {
	return c.address
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

func TestBLEExecutorBlackSharkSetSpeedClampsToMaxRPMAndUsesRPMModeLabel(t *testing.T) {
	client := &fakeBLEClient{}
	executor, err := NewBLEExecutor(types.BlackSharkBRB02Profile(), &fakeBLEConnector{client: client})
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}
	defer executor.Close()

	// 上限取设备可达区间的所有者，所以这里喂一个必然越界的请求值。
	state, err := executor.SetSpeed(nil, types.NewRPMSpeed(deviceproto.BlackSharkMaxRPM+260))
	if err != nil {
		t.Fatalf("SetSpeed() error = %v", err)
	}
	// 官方 apply 仪式三帧：0x24[FF 暂存] → 0x08 00 → 0x24[真实值]，sp 取标定换算后的值
	// （与 Manager 主路同源）。
	setpoint := deviceproto.BlackSharkSetpointForRPM(deviceproto.BlackSharkMaxRPM)
	staging := []byte{deviceproto.BlackSharkFixedStagingSource, deviceproto.BlackSharkFormValue, 1,
		byte(setpoint & 0xFF), byte(setpoint >> 8)}
	payload, err := deviceproto.EncodeBlackSharkFixedSpeedPayload(1, uint16(setpoint), 0)
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}
	wantStaging := deviceproto.BuildBlackSharkFrame(deviceproto.BlackSharkCmdSetSpeed, staging...)
	wantGate := deviceproto.BuildBlackSharkFrame(deviceproto.BlackSharkCmdSetSceneMode, 0x00)
	want := deviceproto.BuildBlackSharkFrame(deviceproto.BlackSharkCmdSetSpeed, payload...)
	if len(client.writes) != 3 ||
		string(client.writes[0].payload) != string(wantStaging) ||
		string(client.writes[1].payload) != string(wantGate) ||
		string(client.writes[2].payload) != string(want) {
		t.Fatalf("writes = %#v, want apply ritual for %d RPM: staging % X / gate % X / real % X",
			client.writes, deviceproto.BlackSharkMaxRPM, wantStaging, wantGate, want)
	}
	if state.TargetRPM != deviceproto.BlackSharkMaxRPM || state.WorkMode != "auto/realtime RPM mode" || state.CurrentMode != 1 {
		t.Fatalf("set state = %#v, want %d RPM auto mode", state, deviceproto.BlackSharkMaxRPM)
	}
}

func TestBLEExecutorBlackSharkStatusUsesProtocolModeFlag(t *testing.T) {
	client := &fakeBLEClient{reads: [][]byte{
		deviceproto.BuildBlackSharkFrame(deviceproto.BlackSharkCmdGetStatus, 0, 0, 0, 0xAC, 0x08),
	}}
	executor, err := NewBLEExecutor(types.BlackSharkBRB02Profile(), &fakeBLEConnector{client: client})
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}
	defer executor.Close()

	state, err := executor.ReadState(nil)
	if err != nil {
		t.Fatalf("ReadState() error = %v", err)
	}
	if state.CurrentRPM != 2220 || state.CurrentMode != 0 || state.WorkMode != "manual/fixed gear mode" {
		t.Fatalf("status state = %#v, want 2220 RPM manual mode", state)
	}
	if len(client.writes) != 1 || string(client.writes[0].payload) != string(deviceproto.BuildBlackSharkGetStatus()) {
		t.Fatalf("writes = %#v, want one get-status frame", client.writes)
	}
}

func TestBLEExecutorFlyDigiBS1SetSpeedReconnectsOnceAfterWriteFailure(t *testing.T) {
	failed := &fakeBLEClient{writeErr: io.ErrClosedPipe}
	recovered := &fakeBLEClient{}
	connector := &fakeBLEConnector{clients: []*fakeBLEClient{failed, recovered}}
	executor, err := NewBLEExecutor(types.FlyDigiBS1Profile(), connector)
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}
	defer executor.Close()

	state, err := executor.SetSpeed(nil, types.NewRPMSpeed(1800))
	if err != nil {
		t.Fatalf("SetSpeed() error = %v", err)
	}
	if connector.connects != 2 || !failed.closed {
		t.Fatalf("recovery connects = %d, failed client closed = %v", connector.connects, failed.closed)
	}
	if len(recovered.writes) != 2 || state.TargetRPM != 1800 {
		t.Fatalf("recovered writes/state = %d/%#v, want two writes and 1800 RPM", len(recovered.writes), state)
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

func TestBLEExecutorBS1ReusesAddressBeforeScanning(t *testing.T) {
	first := &fakeBLEClient{address: "AA:BB:CC:DD:EE:01"}
	reconnected := &fakeBLEClient{address: "AA:BB:CC:DD:EE:01"}
	connector := &fakeBLEConnector{clients: []*fakeBLEClient{first, reconnected}}
	executor, err := NewBLEExecutor(types.FlyDigiBS1Profile(), connector)
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}

	if _, err := executor.Open(nil); err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	if err := executor.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := executor.Open(nil); err != nil {
		t.Fatalf("reconnect Open() error = %v", err)
	}
	if connector.connects != 2 || connector.profile.Connection.Endpoint != "AA:BB:CC:DD:EE:01" {
		t.Fatalf("reconnect profile = %#v, connects = %d; expected cached address attempt", connector.profile.Connection, connector.connects)
	}
}

func TestBLEExecutorBS1NotificationCallbackPublishesState(t *testing.T) {
	executor, err := NewBLEExecutor(types.FlyDigiBS1Profile(), &fakeBLEConnector{client: &fakeBLEClient{}})
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}
	defer executor.Close()
	updates := make(chan *types.FanData, 1)
	executor.SetNotificationCallback(func(data *types.FanData) { updates <- data })

	executor.handleBS1Notification(deviceproto.BuildFrame(deviceproto.CmdStatusNotify, 1, 2, 0, 0xD2, 0x04, 0x08, 0x07))
	select {
	case state := <-updates:
		if state.CurrentRPM != 1234 || state.TargetRPM != 1800 {
			t.Fatalf("notification state = %d/%d, want 1234/1800", state.CurrentRPM, state.TargetRPM)
		}
	case <-time.After(time.Second):
		t.Fatal("notification callback was not invoked")
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

func TestResolveBLEMinSendInterval(t *testing.T) {
	t.Run("档案未设置时回落到 100ms", func(t *testing.T) {
		if got := resolveBLEMinSendInterval(0); got != 100*time.Millisecond {
			t.Fatalf("默认间隔 = %v, want 100ms（这是既有行为，不许改）", got)
		}
		// 档案显式给了值就尊重档案。
		if got := resolveBLEMinSendInterval(120); got != 120*time.Millisecond {
			t.Fatalf("档案值被无视: %v", got)
		}
	})

	t.Run("档案声明负数表示显式取消", func(t *testing.T) {
		for _, v := range []int{-1, -100} {
			if got := resolveBLEMinSendInterval(v); got != 0 {
				t.Fatalf("档案值 %d 应表示不节流, got %v", v, got)
			}
		}
	})
}

func TestBLEDiscoveryAttemptsFor(t *testing.T) {
	if got := bleDiscoveryAttemptsFor(types.BlackSharkBRB02Profile()); got != bleDiscoveryRetryAttempts {
		t.Fatalf("黑鲨 BLE 档案应重试 %d 次, got %d", bleDiscoveryRetryAttempts, got)
	}
	if got := bleDiscoveryAttemptsFor(types.BlackSharkBRB02USBProfile()); got != 1 {
		t.Fatalf("黑鲨 USB 档案不该走 BLE 重试, got %d", got)
	}
	for _, profile := range []types.DeviceProfile{
		types.FlyDigiBS1Profile(),
		{ID: "some.other.ble.profile", Transport: types.DeviceTransportBLE},
	} {
		if got := bleDiscoveryAttemptsFor(profile); got != 1 {
			t.Fatalf("非黑鲨档案 %q 应保持上游的一次枚举, got %d", profile.ID, got)
		}
	}
}

func TestBLEDiscoveryRetryWindowCoversDeviceWarmup(t *testing.T) {
	var window time.Duration
	for attempt := 1; attempt < bleDiscoveryRetryAttempts; attempt++ {
		window += time.Duration(attempt) * bleDiscoveryRetryBackoff
	}
	if window < 6*time.Second {
		t.Fatalf("重试窗口 %v 覆盖不了实测的设备预热 6s", window)
	}
}

func TestBLEExecutorBlackSharkWriteFailureStreakTreatsRepeatedFailuresAsLinkDeath(t *testing.T) {
	failed := &fakeBLEClient{writeErr: io.ErrClosedPipe}
	connector := &fakeBLEConnector{client: failed}
	executor, err := NewBLEExecutor(types.BlackSharkBRB02Profile(), connector)
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}
	defer executor.Close()

	lost := 0
	executor.SetConnectionLostCallback(func() { lost++ })

	if _, err := executor.Open(nil); err == nil {
		t.Fatalf("Open() must fail while the link refuses writes")
	}
	for i := 0; i < bleWriteFailureStreakLimit; i++ {
		_, _ = executor.SetSpeed(nil, types.NewRPMSpeed(1800))
	}

	if !failed.closed {
		t.Fatalf("client must be closed after %d consecutive write failures", bleWriteFailureStreakLimit)
	}
	if lost == 0 {
		t.Fatalf("connectionLost callback must fire after the streak limit")
	}
	// 判死后计数复位 ⇒ 后续失败重新累积（这是"连续"语义，不是"全局累计"）：
	// 本轮的 failed client 被判死清掉后，ensureOpenLocked 用同一个替身重连，
	// 于是又开始积累新的 streak。这里只要求它**小于**上限（即没有在判死后继续叠加到误判）。
	if executor.writeFailureStreak >= bleWriteFailureStreakLimit {
		t.Fatalf("streak must reset on firing, got %d", executor.writeFailureStreak)
	}
}

func TestBLEExecutorBlackSharkWriteSuccessResetsFailureStreak(t *testing.T) {
	// 抖动不该断链：失败 < 上限之后只要有一次成功，计数必须清零。
	// 用 limit-1 次失败 + 1 次成功交替，验证永远不会判死（否则健康连接会被误杀）。
	client := &fakeBLEClient{}
	executor, err := NewBLEExecutor(types.BlackSharkBRB02Profile(), &fakeBLEConnector{client: client})
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}
	defer executor.Close()

	lost := 0
	executor.SetConnectionLostCallback(func() { lost++ })

	for round := 0; round < 5; round++ {
		for i := 0; i < bleWriteFailureStreakLimit-1; i++ {
			client.writeErr = io.ErrClosedPipe
			_, _ = executor.SetSpeed(nil, types.NewRPMSpeed(1800))
		}
		client.writeErr = nil
		if _, err := executor.SetSpeed(nil, types.NewRPMSpeed(1800)); err != nil {
			t.Fatalf("successful write must succeed, got %v", err)
		}
	}

	if lost != 0 || client.closed {
		t.Fatalf("interleaved successes must keep the link alive (lost=%d closed=%v)", lost, client.closed)
	}
	if !executor.IsConnected() {
		t.Fatalf("IsConnected() must stay true while writes keep succeeding")
	}
}

func TestBLEExecutorFlyDigiBS1DoesNotUseWriteFailureStreakDetection(t *testing.T) {
	// 飞智 BS1 有 3s 心跳做断链发现，不该被这条黑鲨专用判据改变语义。
	// 这里只要证明"判据不作用于 BS1"即可 —— 单次写失败它会走自己的重连路径。
	if !types.IsBlackSharkDeviceProfileID(types.BlackSharkBRB02Profile().ID) {
		t.Fatalf("test assumes the BlackShark profile id is the black-shark-scoped owner")
	}
	client := &fakeBLEClient{}
	executor, err := NewBLEExecutor(types.FlyDigiBS1Profile(), &fakeBLEConnector{client: client})
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}
	defer executor.Close()
	if executor.shouldDetectWriteFailureStreak() {
		t.Fatalf("write-failure streak detection must be black-shark scoped, not applied to BS1")
	}
}

func TestBLEExecutorBlackSharkWriteFailureStreakDoesNotDeadlock(t *testing.T) {
	// 回归护栏（sync.Mutex 不可重入）：判定路径跑在 writeRawLocked **持锁期间**，
	// 若它回头再取 e.mutex（例如直接调 notifyConnectionLost），这里会永久挂死。
	client := &fakeBLEClient{writeErr: io.ErrClosedPipe}
	executor, err := NewBLEExecutor(types.BlackSharkBRB02Profile(), &fakeBLEConnector{client: client})
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}
	defer executor.Close()
	executor.SetConnectionLostCallback(func() {})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < bleWriteFailureStreakLimit; i++ {
			_, _ = executor.SetSpeed(nil, types.NewRPMSpeed(1800))
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("write-failure streak detection deadlocked (re-entrant lock?)")
	}
}
