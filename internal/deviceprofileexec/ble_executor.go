package deviceprofileexec

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/types"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"tinygo.org/x/bluetooth"
)

const maxBLEFrameBytes = 64 * 1024

// bleDefaultWWRChar 是带响应写用的特征（黑鲨的 ae10）。
//
// 依据：官方走 ae30/ae10，而带响应写有链路层流控，不会静默堆积。
//
// 只给黑鲨档案（见 bleWWRCharShortUUIDFor）：ae10 只在黑鲨特征表里存在，给别的档案开会连带改掉
// 它们的 writeWithResponse 语义（那是共享契约）。
const bleDefaultWWRChar = "ae10"

// bleWWRCharShortUUIDFor 返回该档案要用的带响应写特征；空 = 不带响应写。
//
// 只对控制类命令生效：图传块流走 WriteRawStream（强制不带响应写），不跟随本函数。
func bleWWRCharShortUUIDFor(profile types.DeviceProfile) string {
	if profile.ID == types.BlackSharkBRB02ProfileID {
		return bleDefaultWWRChar
	}
	return ""
}

const bleWriteFailureStreakLimit = 3

func resolveBLEMinSendInterval(profileMs int) time.Duration {
	if profileMs < 0 {
		return 0
	}
	return durationFromMillis(profileMs, defaultMinSendInterval, maxProfileSendInterval)
}

const defaultBLEOperationTimeout = time.Duration(defaultBLEScanTimeoutMs) * time.Millisecond

type BLEClient interface {
	WriteBLECommand(ctx context.Context, payload []byte, withResponse bool) error
	ReadBLEFrame(ctx context.Context) ([]byte, error)
	Close() error
}

// BLEAddressProvider lets native clients expose the address used for a
// successful connection so BS1 can try direct recovery before scanning.
type BLEAddressProvider interface {
	BLEAddress() string
}

type BLENotificationConsumer interface {
	SetBLENotificationCallback(func([]byte))
}

type BLEConnector interface {
	ConnectBLEDevice(ctx context.Context, profile types.DeviceProfile) (BLEClient, error)
}

type BLEConnectorFunc func(ctx context.Context, profile types.DeviceProfile) (BLEClient, error)

func (f BLEConnectorFunc) ConnectBLEDevice(ctx context.Context, profile types.DeviceProfile) (BLEClient, error) {
	return f(ctx, profile)
}

type DefaultBLEConnector struct {
	Scanner BLEScanner
}

type BLEExecutor struct {
	profile           types.DeviceProfile
	connector         BLEConnector
	client            BLEClient
	readCommand       types.DeviceCommandTemplate
	hasReadCommand    bool
	setCommand        types.DeviceCommandTemplate
	hasSetCommand     bool
	parsers           CompiledResponseParsers
	writeWithResponse bool

	requestTimeout  time.Duration
	minSendInterval time.Duration
	maxRetries      int
	retryBackoff    time.Duration

	mutex     sync.Mutex
	sendMutex sync.Mutex
	lastSend  time.Time
	lastState *types.FanData
	now       func() time.Time
	sleep     func(context.Context, time.Duration) error

	heartbeatStop      chan struct{}
	connectionLost     func()
	notificationUpdate func(*types.FanData)
	lastAddress        string

	// writeFailureStreak 是"连续写失败"的计数，用于发现**链路已死但 client 还在**。
	//
	// GATT 写被拒是链路断掉的唯一早期信号，而 IsConnected() 只看 e.client != nil —— 它在写失败时
	// 不会被清，断链后所有操作会一直跑到各自超时才报错。黑鲨没有心跳，没有其它路径能发现链路已死。
	// 只用连续失败判定（成功即清零），否则会把健康连接误杀。
	writeFailureStreak int

	// rawFrame 是"每一帧原始通知"的旁路回调（在解析之前触发）。
	// 设备层用它把帧喂进自己的状态机（黑鲨的"发一帧等一条应答"按 cmd 配对）；
	// 用原子指针是因为触发方是通知 goroutine，而装配方在连接建立线程上 ——
	// 且 handleBlackSharkNotification 内部会取 e.mutex，这里绝不能再去争同一把锁。
	rawFrame atomic.Pointer[rawFrameSink]
}

// rawFrameSink 包一层是为了能放进 atomic.Pointer（函数类型本身不能直接原子存）。
type rawFrameSink struct{ fn func([]byte) }

func NewBLEExecutor(profile types.DeviceProfile, connector BLEConnector) (*BLEExecutor, error) {
	profile = types.NormalizeDeviceProfile(profile, "")
	if profile.Transport != types.DeviceTransportBLE {
		return nil, fmt.Errorf("ble executor requires a ble profile")
	}
	if connector == nil {
		connector = DefaultBLEConnector{}
	}
	parsers, err := CompileResponseParsers(profile.ResponseParsers)
	if err != nil {
		return nil, err
	}
	readCommand, hasReadCommand := FindCommand(profile.Commands, "readState", "read-state", "state", "status", "read")
	setCommand, hasSetCommand := FindCommand(profile.Commands, "setSpeed", "set-speed", "speed")
	return &BLEExecutor{
		profile:           profile,
		connector:         connector,
		readCommand:       readCommand,
		hasReadCommand:    hasReadCommand,
		setCommand:        setCommand,
		hasSetCommand:     hasSetCommand,
		parsers:           parsers,
		writeWithResponse: profile.Connection.BLEWriteWithResponse || bleWWRCharShortUUIDFor(profile) != "",
		requestTimeout:    durationFromMillis(profile.Connection.RequestTimeoutMs, defaultBLEOperationTimeout, maxProfileHTTPTimeout),
		minSendInterval:   resolveBLEMinSendInterval(profile.Connection.MinSendIntervalMs),
		maxRetries:        clampInt(profile.Connection.MaxRetries, 0, maxProfileRetryCount),
		retryBackoff:      durationFromMillis(profile.Connection.RetryBackoffMs, defaultRetryBackoff, maxProfileRetryBackoff),
		now:               time.Now,
		sleep:             sleepContext,
	}, nil
}

func (e *BLEExecutor) Profile() types.DeviceProfile {
	return e.profile
}

func (e *BLEExecutor) SetConnectionLostCallback(callback func()) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.connectionLost = callback
}

func (e *BLEExecutor) SetNotificationCallback(callback func(*types.FanData)) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.notificationUpdate = callback
}

// SetRawFrameCallback 注册"收到每一帧原始通知"的旁路回调（在解析之前触发）。
//
// 让设备层把帧接进自己的状态机（黑鲨的 BLE 查询靠它，不接则只能超时）。传 nil 注销；
// 回调在通知 goroutine 上同步执行，必须非阻塞。
func (e *BLEExecutor) SetRawFrameCallback(callback func([]byte)) {
	if callback == nil {
		e.rawFrame.Store(nil)
		return
	}
	e.rawFrame.Store(&rawFrameSink{fn: callback})
}

// emitRawFrame 触发旁路回调；无注册者时是空操作。
func (e *BLEExecutor) emitRawFrame(body []byte) {
	if sink := e.rawFrame.Load(); sink != nil && sink.fn != nil {
		sink.fn(body)
	}
}

func (e *BLEExecutor) IsConnected() bool {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return e.client != nil
}

func (e *BLEExecutor) Open(ctx context.Context) (*types.FanData, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if err := e.ensureOpenLocked(ctx); err != nil {
		return nil, err
	}
	if e.profile.ID == types.FlyDigiBS1ProfileID && !e.hasReadCommand {
		state := e.syntheticStateLocked(types.NewRPMSpeed(0))
		e.lastState = cloneFanData(state)
		return state, nil
	}
	if e.profile.ID == types.BlackSharkBRB02ProfileID {
		return e.readBlackSharkStatusLocked(ctx)
	}
	if e.hasReadCommand || e.canReadDirectly() {
		return e.readStateLocked(ctx)
	}
	state := e.syntheticStateLocked(types.FanSpeedValue{})
	e.lastState = cloneFanData(state)
	return state, nil
}

func (e *BLEExecutor) Close() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.stopHeartbeatLocked()
	e.connectionLost = nil
	e.notificationUpdate = nil
	if e.client == nil {
		return nil
	}
	err := e.client.Close()
	e.client = nil
	return err
}

func (e *BLEExecutor) ReadState(ctx context.Context) (*types.FanData, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if err := e.ensureOpenLocked(ctx); err != nil {
		return nil, err
	}
	if e.profile.ID == types.FlyDigiBS1ProfileID && !e.hasReadCommand {
		return e.readFlyDigiBS1StateLocked(ctx)
	}
	if e.profile.ID == types.BlackSharkBRB02ProfileID {
		return e.readBlackSharkStatusLocked(ctx)
	}
	if e.hasReadCommand || e.canReadDirectly() {
		return e.readStateLocked(ctx)
	}
	if e.lastState != nil {
		return cloneFanData(e.lastState), nil
	}
	state := e.syntheticStateLocked(types.FanSpeedValue{})
	e.lastState = cloneFanData(state)
	return state, nil
}

func (e *BLEExecutor) SetSpeed(ctx context.Context, speed types.FanSpeedValue) (*types.FanData, error) {
	speed = speed.Normalized()
	if speed.Unit != types.NormalizeFanSpeedUnit(e.profile.SpeedUnit) {
		return nil, fmt.Errorf("speed unit %q does not match profile unit %q", speed.Unit, e.profile.SpeedUnit)
	}
	if e.profile.ID == types.FlyDigiBS1ProfileID {
		return e.setFlyDigiBS1Speed(ctx, speed)
	}
	if e.profile.ID == types.BlackSharkBRB02ProfileID {
		return e.setBlackSharkSpeed(ctx, speed)
	}
	if !e.hasSetCommand {
		return nil, fmt.Errorf("ble profile does not define a setSpeed command")
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()

	if err := e.ensureOpenLocked(ctx); err != nil {
		return nil, err
	}
	var state *types.FanData
	var err error
	for attempt := 0; attempt <= e.maxRetries; attempt++ {
		state, err = e.setSpeedOnceLocked(ctx, speed)
		if err == nil {
			e.lastState = cloneFanData(state)
			return state, nil
		}
		if attempt >= e.maxRetries {
			break
		}
		if sleepErr := e.sleep(ctxWithDefault(ctx), e.retryDelay(attempt)); sleepErr != nil {
			return nil, sleepErr
		}
	}
	return nil, err
}

func (e *BLEExecutor) setSpeedOnceLocked(ctx context.Context, speed types.FanSpeedValue) (*types.FanData, error) {
	vars := SpeedVarsFromValue(speed)
	payload, err := e.bleCommandBytes(e.setCommand, vars)
	if err != nil {
		return nil, err
	}
	if err := e.waitForSendSlot(ctxWithDefault(ctx)); err != nil {
		return nil, err
	}
	opCtx, cancel := e.operationContext(ctx)
	defer cancel()
	if err := e.client.WriteBLECommand(opCtx, payload, e.writeWithResponse); err != nil {
		return nil, err
	}
	if !e.shouldReadSetResponse() {
		return e.syntheticStateLocked(speed), nil
	}
	body, err := e.readFrame(ctx)
	if err != nil {
		return nil, err
	}
	return e.fanDataFromBody(body, speed)
}

func (e *BLEExecutor) setFlyDigiBS1Speed(ctx context.Context, speed types.FanSpeedValue) (*types.FanData, error) {
	if !types.IsRPMSpeedUnit(speed.Unit) {
		return nil, fmt.Errorf("FlyDigi BS1 requires RPM speed")
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()

	rpm := types.ClampRPM(speed.Value)
	if rpm > 4000 {
		rpm = 4000
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := e.ensureOpenLocked(ctx); err != nil {
			lastErr = err
		} else {
			state, err := e.setFlyDigiBS1SpeedOnceLocked(ctx, rpm)
			if err == nil {
				e.lastState = cloneFanData(state)
				return state, nil
			}
			lastErr = err
		}
		e.invalidateClientLocked()
		if err := ctxWithDefault(ctx).Err(); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (e *BLEExecutor) setFlyDigiBS1SpeedOnceLocked(ctx context.Context, rpm int) (*types.FanData, error) {
	if err := e.writeRawLocked(ctx, deviceproto.BuildFrame(deviceproto.CmdRGBEnable, 0x01)); err != nil {
		return nil, fmt.Errorf("enter BS1 dynamic RPM mode: %w", err)
	}
	if err := e.sleep(ctxWithDefault(ctx), 50*time.Millisecond); err != nil {
		return nil, err
	}
	payload := make([]byte, 2)
	binary.LittleEndian.PutUint16(payload, uint16(rpm))
	if err := e.writeRawLocked(ctx, deviceproto.BuildFrame(deviceproto.CmdSetRealtimeRPM, payload...)); err != nil {
		return nil, fmt.Errorf("set BS1 RPM: %w", err)
	}
	current := 0
	if e.lastState != nil {
		current = int(e.lastState.CurrentRPM)
	}
	state := e.stateWithValues(current, rpm, "自动模式(实时转速)")
	return state, nil
}

func (e *BLEExecutor) setBlackSharkSpeed(ctx context.Context, speed types.FanSpeedValue) (*types.FanData, error) {
	if !types.IsRPMSpeedUnit(speed.Unit) {
		return nil, fmt.Errorf("Black Shark BRB02 requires RPM speed")
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if err := e.ensureOpenLocked(ctx); err != nil {
		return nil, err
	}
	rpm := speed.Value
	if rpm < 0 {
		rpm = 0
	}
	if rpm > deviceproto.BlackSharkMaxRPM {
		rpm = deviceproto.BlackSharkMaxRPM
	}
	// 帧语义与 Manager 主路共用唯一所有者 deviceproto：sp 标定换算 + 官方 apply 仪式
	// （0x24[FF 暂存] → 0x08 00 → 0x24[真实值]）。gear/src 盲写 1/0 是本路径的已知局限：
	// 执行器侧没有 0x25 查询链路，拿不到设备当前档与参照源 —— 只有档案测试走这里。
	setpoint := deviceproto.BlackSharkSetpointForRPM(rpm)
	frames, err := deviceproto.BuildBlackSharkFixedSpeedFrames(1, uint16(setpoint), 0)
	if err != nil {
		return nil, err
	}
	for _, f := range frames {
		if err := e.writeRawLocked(ctx, deviceproto.BuildBlackSharkFrame(f.Cmd, f.Payload...)); err != nil {
			return nil, err
		}
	}
	current := 0
	if e.lastState != nil {
		current = int(e.lastState.CurrentRPM)
	}
	mode := byte(1)
	if e.lastState != nil {
		mode = e.lastState.CurrentMode
	}
	state := e.stateWithValues(current, rpm, deviceproto.BlackSharkModeName(mode))
	state.CurrentMode = mode
	e.lastState = cloneFanData(state)
	return state, nil
}

func (e *BLEExecutor) SetBlackSharkLighting(ctx context.Context, cfg types.LightStripConfig) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if err := e.ensureOpenLocked(ctx); err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Mode), "off") {
		return e.writeRawLocked(ctx, deviceproto.BuildBlackSharkSetRGBEnabled(false))
	}
	color := types.RGBColor{R: 255, G: 255, B: 255}
	if len(cfg.Colors) > 0 {
		color = cfg.Colors[0]
	}
	lighting, err := deviceproto.BuildBlackSharkSetLighting(cfg.Mode, cfg.Speed, cfg.Brightness, color.R, color.G, color.B)
	if err != nil {
		return err
	}
	if err := e.writeRawLocked(ctx, deviceproto.BuildBlackSharkSetRGBEnabled(true)); err != nil {
		return err
	}
	return e.writeRawLocked(ctx, lighting)
}

func (e *BLEExecutor) SetBlackSharkRGBEnabled(ctx context.Context, enabled bool) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if err := e.ensureOpenLocked(ctx); err != nil {
		return err
	}
	return e.writeRawLocked(ctx, deviceproto.BuildBlackSharkSetRGBEnabled(enabled))
}

func (e *BLEExecutor) readStateLocked(ctx context.Context) (*types.FanData, error) {
	if e.hasReadCommand {
		payload, err := e.bleCommandBytes(e.readCommand, SpeedVars{})
		if err != nil {
			return nil, err
		}
		opCtx, cancel := e.operationContext(ctx)
		defer cancel()
		if err := e.client.WriteBLECommand(opCtx, payload, e.writeWithResponse); err != nil {
			return nil, err
		}
	}
	body, err := e.readFrame(ctx)
	if err != nil {
		return nil, err
	}
	state, err := e.fanDataFromBody(body, types.FanSpeedValue{})
	if err != nil {
		return nil, err
	}
	e.lastState = cloneFanData(state)
	return state, nil
}

func (e *BLEExecutor) readFlyDigiBS1StateLocked(ctx context.Context) (*types.FanData, error) {
	opCtx, cancel := e.operationContext(ctx)
	defer cancel()
	for {
		if err := opCtx.Err(); err != nil {
			return nil, err
		}
		body, err := e.client.ReadBLEFrame(opCtx)
		if err != nil {
			return nil, err
		}
		if len(body) > maxBLEFrameBytes {
			return nil, fmt.Errorf("ble response exceeded %d bytes", maxBLEFrameBytes)
		}
		state, ok := parseFlyDigiBS1Notification(body)
		if !ok {
			continue
		}
		e.lastState = cloneFanData(state)
		return state, nil
	}
}

func (e *BLEExecutor) readBlackSharkStatusLocked(ctx context.Context) (*types.FanData, error) {
	if err := e.writeRawLocked(ctx, deviceproto.BuildBlackSharkGetStatus()); err != nil {
		return nil, err
	}
	opCtx, cancel := e.operationContext(ctx)
	defer cancel()
	for {
		body, err := e.client.ReadBLEFrame(opCtx)
		if err != nil {
			return nil, err
		}
		frame, ok := deviceproto.ParseBlackSharkFrame(body)
		if !ok {
			continue
		}
		if frame.Command == deviceproto.BlackSharkCmdStatusNotify {
			state, ok := e.blackSharkStateFromFrame(frame)
			if ok {
				e.lastState = cloneFanData(state)
			}
			continue
		}
		if frame.Command != deviceproto.BlackSharkCmdGetStatus {
			continue
		}
		state, ok := e.blackSharkStateFromFrame(frame)
		if !ok {
			continue
		}
		e.lastState = cloneFanData(state)
		return state, nil
	}
}

func (e *BLEExecutor) blackSharkStateFromFrame(frame deviceproto.BlackSharkFrame) (*types.FanData, bool) {
	rpm, flag, ok := deviceproto.ParseBlackSharkStatus(frame)
	if !ok {
		return nil, false
	}
	target := rpm
	if e.lastState != nil && e.lastState.TargetRPM > 0 {
		target = int(e.lastState.TargetRPM)
	}
	state := e.stateWithValues(rpm, target, deviceproto.BlackSharkModeName(flag))
	state.CurrentMode = flag
	return state, true
}

func (e *BLEExecutor) bleCommandBytes(command types.DeviceCommandTemplate, vars SpeedVars) ([]byte, error) {
	body, _, err := EncodeCommand(command, vars)
	return body, err
}

func (e *BLEExecutor) WriteRaw(ctx context.Context, payload []byte) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if err := e.ensureOpenLocked(ctx); err != nil {
		return err
	}
	return e.writeRawLocked(ctx, payload)
}

// WriteRawStream 是**块流**专用写（黑鲨图传：一块一帧、整轮 ≈2096 块）。
//
// 与 WriteRaw 分开是因为它的调用者混着两类语义：控制帧该跟随 writeWithResponse，块流绝不该
// （每块都等一次 ATT 应答会污染整轮耗时与 0xC6 间隔的基线）。用专用名字而不是给调用点加 bool，
// 是为了不让"这是不是块流"这个语义外泄。
//
// 块流的流控由设备的 0xC6 逐块回答（0x00 收下 / 0x0C 拒收），不靠 ATT 应答。
func (e *BLEExecutor) WriteRawStream(ctx context.Context, payload []byte) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if err := e.ensureOpenLocked(ctx); err != nil {
		return err
	}
	return e.writeRawWithModeLocked(ctx, payload, false)
}

func (e *BLEExecutor) writeRawLocked(ctx context.Context, payload []byte) error {
	return e.writeRawWithModeLocked(ctx, payload, e.writeWithResponse)
}

// writeRawWithModeLocked 是两条原始帧写路径的公共实现；withResponse 由调用方显式给定
// （控制帧 = `e.writeWithResponse`，块流 = 恒 false，见 `WriteRawStream`）。
func (e *BLEExecutor) writeRawWithModeLocked(ctx context.Context, payload []byte, withResponse bool) error {
	if len(payload) == 0 {
		return fmt.Errorf("ble command is empty")
	}
	if err := e.waitForSendSlot(ctxWithDefault(ctx)); err != nil {
		return err
	}
	opCtx, cancel := e.operationContext(ctx)
	defer cancel()
	err := e.client.WriteBLECommand(opCtx, payload, withResponse)
	e.observeWriteResultLocked(err)
	return err
}

// observeWriteResultLocked 记一次写的结果，并在"连续失败到上限"时判定链路已死。
//
// 判定放在这里是因为这是唯一能观察到"GATT 写被拒"的位置。调用方必须已持有 e.mutex。
//
// 只在写确实失败时动计数（成功即清零）。判死两步：invalidateClientLocked（清 client ⇒
// IsConnected 转 false）与 notifyConnectionLostCallback（复位设备层状态机）。
// 判定必须幂等：client 置 nil 后，后续写会在 ensureOpenLocked 处就失败、走不到这里。
func (e *BLEExecutor) observeWriteResultLocked(err error) {
	if !e.shouldDetectWriteFailureStreak() {
		return
	}
	if err == nil {
		e.writeFailureStreak = 0
		return
	}
	e.writeFailureStreak++
	if e.writeFailureStreak < bleWriteFailureStreakLimit {
		return
	}
	e.writeFailureStreak = 0
	// 这两个动作都必须在已持锁的前提下用"不再取锁"的形态调用：
	//	invalidateClientLocked 本就是 ...Locked 命名（内部不取锁）
	//	notifyConnectionLost 会取锁 ⇒ 这里必须用 notifyConnectionLostCallback（回调在锁外）
	// 直接调 notifyConnectionLost() 会在同一把锁上二次获取 ⇒ 永久死锁（RWMutex 不可重入）。
	callback := e.connectionLost
	e.invalidateClientLocked()
	e.notifyConnectionLostCallback(callback)
}

// shouldDetectWriteFailureStreak 报告是否需要为这个档案启用"连续写失败 ⇒ 判链路死"。
func (e *BLEExecutor) shouldDetectWriteFailureStreak() bool {
	return e.profile.ID == types.BlackSharkBRB02ProfileID
}

func (e *BLEExecutor) shouldReadSetResponse() bool {
	return len(e.parsers.parsers) > 0
}

func (e *BLEExecutor) canReadDirectly() bool {
	return e.profile.Capabilities.SupportsReadState && strings.TrimSpace(e.profile.Connection.BLENotifyCharacteristic) != ""
}

func (e *BLEExecutor) readFrame(ctx context.Context) ([]byte, error) {
	opCtx, cancel := e.operationContext(ctx)
	defer cancel()
	body, err := e.client.ReadBLEFrame(opCtx)
	if err != nil {
		return nil, err
	}
	if len(body) > maxBLEFrameBytes {
		return nil, fmt.Errorf("ble response exceeded %d bytes", maxBLEFrameBytes)
	}
	return body, nil
}

func (e *BLEExecutor) fanDataFromBody(body []byte, fallback types.FanSpeedValue) (*types.FanData, error) {
	if e.profile.ID == types.FlyDigiBS1ProfileID {
		if fanData, ok := parseFlyDigiBS1Notification(body); ok {
			return fanData, nil
		}
	}
	state, err := e.parsers.Parse(body)
	if err != nil {
		return nil, err
	}
	fallbackValue := 0
	if fallback.Unit != "" {
		fallbackValue = speedValueForProfileState(fallback)
	}
	if !completeStateTarget(&state, fallbackValue) {
		return nil, fmt.Errorf("ble profile response did not contain speed data")
	}
	current := clampForProfile(state.CurrentSpeed, e.profile.SpeedUnit)
	target := clampForProfile(state.TargetSpeed, e.profile.SpeedUnit)
	workMode := strings.TrimSpace(state.WorkMode)
	if workMode == "" {
		workMode = "ble"
	}
	return e.stateWithValues(current, target, workMode), nil
}

func parseFlyDigiBS1Notification(body []byte) (*types.FanData, bool) {
	frame, ok := deviceproto.ParseFrame(body)
	if !ok || frame.Command != deviceproto.CmdStatusNotify || len(frame.Payload) < 7 {
		return nil, false
	}
	mode := frame.Payload[1]
	current := int(binary.LittleEndian.Uint16(frame.Payload[3:5]))
	target := int(binary.LittleEndian.Uint16(frame.Payload[5:7]))
	return &types.FanData{
		ReportID:     frame.ReportID,
		MagicSync:    0x5AA5,
		Command:      frame.Command,
		Status:       frame.Length,
		GearSettings: frame.Payload[0],
		CurrentMode:  mode,
		Reserved1:    frame.Payload[2],
		CurrentRPM:   uint16(clampUint16(current)),
		TargetRPM:    uint16(clampUint16(target)),
		WorkMode:     deviceproto.ModeName(mode),
		Transport:    types.DeviceTransportBLE,
		SpeedUnit:    types.FanSpeedUnitRPM,
	}, true
}

func (e *BLEExecutor) syntheticStateLocked(speed types.FanSpeedValue) *types.FanData {
	value := speedValueForProfileState(speed)
	if strings.TrimSpace(speed.Unit) != "" {
		current := 0
		if e.lastState != nil {
			current = int(e.lastState.CurrentRPM)
		}
		return e.stateWithValues(current, value, "ble")
	}
	return e.stateWithValues(value, value, "ble")
}

func (e *BLEExecutor) stateWithValues(current, target int, mode string) *types.FanData {
	if mode == "" {
		mode = "ble"
	}
	return &types.FanData{
		CurrentRPM: uint16(clampUint16(current)),
		TargetRPM:  uint16(clampUint16(target)),
		WorkMode:   mode,
		Transport:  types.DeviceTransportBLE,
		SpeedUnit:  types.NormalizeFanSpeedUnit(e.profile.SpeedUnit),
	}
}

func (e *BLEExecutor) ensureOpenLocked(ctx context.Context) error {
	if e.client != nil {
		return nil
	}
	connectProfile := e.profile
	cachedAddress := strings.TrimSpace(e.lastAddress)
	if cachedAddress != "" && (e.profile.ID == types.FlyDigiBS1ProfileID ||
		types.IsBlackSharkDeviceProfileID(e.profile.ID)) {
		connectProfile.Connection.Endpoint = cachedAddress
	}
	connect := func(profile types.DeviceProfile) (BLEClient, error) {
		opCtx, cancel := e.operationContext(ctx)
		defer cancel()
		return e.connector.ConnectBLEDevice(opCtx, profile)
	}
	client, err := connect(connectProfile)
	if err != nil && cachedAddress != "" {
		// Windows can assign a new address after a device reset; fall back to
		// the normal profile scan after invalidating only this runtime hint.
		e.lastAddress = ""
		client, err = connect(e.profile)
	}
	if err != nil {
		return err
	}
	e.client = client
	if addressClient, ok := client.(BLEAddressProvider); ok {
		if address := strings.TrimSpace(addressClient.BLEAddress()); address != "" {
			e.lastAddress = address
		}
	}
	if notificationClient, ok := client.(BLENotificationConsumer); ok {
		switch e.profile.ID {
		case types.FlyDigiBS1ProfileID:
			notificationClient.SetBLENotificationCallback(e.handleBS1Notification)
		case types.BlackSharkBRB02ProfileID:
			notificationClient.SetBLENotificationCallback(e.handleBlackSharkNotification)
		}
	}
	e.startBS1HeartbeatLocked()
	return nil
}

func (e *BLEExecutor) startBS1HeartbeatLocked() {
	if e.profile.ID != types.FlyDigiBS1ProfileID || e.heartbeatStop != nil {
		return
	}
	stop := make(chan struct{})
	e.heartbeatStop = stop
	go e.bs1HeartbeatLoop(stop)
}

func (e *BLEExecutor) stopHeartbeatLocked() {
	if e.heartbeatStop == nil {
		return
	}
	close(e.heartbeatStop)
	e.heartbeatStop = nil
}

func (e *BLEExecutor) bs1HeartbeatLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	commands := [][]byte{types.BS1CmdHeartbeat1, types.BS1CmdHeartbeat2}
	index := 0
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			select {
			case <-stop:
				return
			default:
			}
			if err := e.writeHeartbeat(context.Background(), commands[index%len(commands)]); err != nil {
				e.notifyConnectionLost()
				return
			}
			index++
		}
	}
}

func (e *BLEExecutor) notifyConnectionLost() {
	e.mutex.Lock()
	callback := e.connectionLost
	e.mutex.Unlock()
	e.notifyConnectionLostCallback(callback)
}

// notifyConnectionLostCallback 是"已经拿到回调"之后的收尾。
//
// 持锁路径必须走它：observeWriteResultLocked 在 writeRawLocked 持锁期间被调用，直接调
// notifyConnectionLost() 会再取一次 e.mutex ⇒ 永久死锁。回调也在锁外调用（它可能反过来调执行器）。
func (e *BLEExecutor) notifyConnectionLostCallback(callback func()) {
	if callback != nil {
		callback()
	}
}

func (e *BLEExecutor) handleBS1Notification(body []byte) {
	state, ok := parseFlyDigiBS1Notification(body)
	if !ok {
		return
	}
	e.mutex.Lock()
	e.lastState = cloneFanData(state)
	callback := e.notificationUpdate
	e.mutex.Unlock()
	if callback != nil {
		callback(state)
	}
}

func (e *BLEExecutor) handleBlackSharkNotification(body []byte) {
	// 旁路：先把原始帧交给设备层（它按 cmd 配对等应答），再走本执行器自己的状态更新。
	// 必须在 0x06 过滤之前 —— 0x25 之类的应答也是从这条通知通道来的。
	e.emitRawFrame(body)
	frame, ok := deviceproto.ParseBlackSharkFrame(body)
	if !ok || frame.Command != deviceproto.BlackSharkCmdStatusNotify {
		return
	}
	e.mutex.Lock()
	state, ok := e.blackSharkStateFromFrame(frame)
	if !ok {
		e.mutex.Unlock()
		return
	}
	e.lastState = cloneFanData(state)
	callback := e.notificationUpdate
	e.mutex.Unlock()
	if callback != nil {
		callback(state)
	}
}

func (e *BLEExecutor) writeHeartbeat(ctx context.Context, payload []byte) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if e.client == nil {
		return io.ErrClosedPipe
	}
	err := e.writeRawLocked(ctx, payload)
	if err != nil {
		e.invalidateClientLocked()
	}
	return err
}

func (e *BLEExecutor) invalidateClientLocked() {
	e.stopHeartbeatLocked()
	client := e.client
	e.client = nil
	e.lastState = nil
	if client != nil {
		_ = client.Close()
	}
}

func (e *BLEExecutor) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, e.requestTimeout)
}

func (e *BLEExecutor) waitForSendSlot(ctx context.Context) error {
	if e.minSendInterval <= 0 {
		return nil
	}
	e.sendMutex.Lock()
	defer e.sendMutex.Unlock()

	now := e.now()
	if !e.lastSend.IsZero() {
		wait := e.minSendInterval - now.Sub(e.lastSend)
		if wait > 0 {
			if err := e.sleep(ctx, wait); err != nil {
				return err
			}
			now = e.now()
		}
	}
	e.lastSend = now
	return nil
}

func (e *BLEExecutor) retryDelay(attempt int) time.Duration {
	if e.retryBackoff <= 0 {
		return 0
	}
	return time.Duration(attempt+1) * e.retryBackoff
}

func (c DefaultBLEConnector) ConnectBLEDevice(ctx context.Context, profile types.DeviceProfile) (BLEClient, error) {
	ctx = ctxWithDefault(ctx)
	profile = types.NormalizeDeviceProfile(profile, "")
	if profile.Transport != types.DeviceTransportBLE {
		return nil, fmt.Errorf("ble connector requires a ble profile")
	}
	address, err := c.resolveAddress(ctx, profile)
	if err != nil {
		return nil, err
	}
	release, err := acquireDefaultBLEAdapter(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	device, err := adapter.Connect(address, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		_ = device.Disconnect()
		return nil, err
	}

	client, err := discoverBLEClient(device, profile)
	if err != nil {
		_ = device.Disconnect()
		return nil, err
	}
	client.address = address.String()
	return client, nil
}

func (c DefaultBLEConnector) resolveAddress(ctx context.Context, profile types.DeviceProfile) (bluetooth.Address, error) {
	endpoint := strings.TrimSpace(profile.Connection.Endpoint)
	if endpoint != "" {
		return parseBluetoothAddress(endpoint)
	}

	scanner := c.Scanner
	if scanner == nil {
		scanner = DefaultBLEScanner{}
	}
	devices, err := ScanBLEDevicesWithScanner(ctxWithDefault(ctx), scanner, types.BLEScanParams{
		TimeoutMs:                profile.Connection.RequestTimeoutMs,
		NameFilter:               profile.Connection.BLENameFilter,
		ServiceUUID:              profile.Connection.BLEServiceUUID,
		WriteCharacteristicUUID:  profile.Connection.BLEWriteCharacteristic,
		NotifyCharacteristicUUID: profile.Connection.BLENotifyCharacteristic,
		OnlyMatched:              true,
		Profiles:                 []types.DeviceProfile{profile},
	})
	if err != nil {
		return bluetooth.Address{}, err
	}
	if len(devices) == 0 {
		return bluetooth.Address{}, fmt.Errorf("no BLE device matched profile %q", profile.DisplayName)
	}
	return parseBluetoothAddress(devices[0].Address)
}

func parseBluetoothAddress(value string) (bluetooth.Address, error) {
	mac, err := bluetooth.ParseMAC(strings.TrimSpace(value))
	if err != nil {
		return bluetooth.Address{}, err
	}
	return bluetooth.Address{MACAddress: bluetooth.MACAddress{MAC: mac}}, nil
}

type realBLEClient struct {
	device               bluetooth.Device
	address              string
	writeChar            *bluetooth.DeviceCharacteristic
	writeCharWWR         *bluetooth.DeviceCharacteristic // 带响应写专用特征（见 bleWWRCharShortUUIDFor）
	wwrSoftFallback      bool                            // 档案要求带响应写特征但没枚举到（黑鲨 ae10 缺席）：withResponse 软回落为不带响应写
	readChar             *bluetooth.DeviceCharacteristic
	notifications        chan []byte
	notificationsEnabled bool
	notificationMutex    sync.RWMutex
	notificationCallback func([]byte)
}

// bleDiscoveryRetryAttempts / bleDiscoveryRetryBackoff 是"首次枚举拿不到特征"时的重试策略。
const (
	bleDiscoveryRetryAttempts = 6
	bleDiscoveryRetryBackoff  = 500 * time.Millisecond
)

// bleDiscoveryAttemptsFor 返回某档案在"首次枚举找不到特征"时的枚举次数。
func bleDiscoveryAttemptsFor(profile types.DeviceProfile) int {
	if profile.ID == types.BlackSharkBRB02ProfileID {
		return bleDiscoveryRetryAttempts
	}
	return 1
}

// discoverBLEClient 连接后枚举服务/特征并组装客户端。
func discoverBLEClient(device bluetooth.Device, profile types.DeviceProfile) (*realBLEClient, error) {
	attempts := bleDiscoveryAttemptsFor(profile)
	var (
		client *realBLEClient
		err    error
	)
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * bleDiscoveryRetryBackoff)
		}
		client, err = discoverBLEClientOnce(device, profile)
		if err == nil {
			return client, nil
		}
	}
	return nil, err
}

func discoverBLEClientOnce(device bluetooth.Device, profile types.DeviceProfile) (*realBLEClient, error) {
	serviceUUID := normalizeBLEUUID(profile.Connection.BLEServiceUUID)
	writeUUID := normalizeBLEUUID(profile.Connection.BLEWriteCharacteristic)
	notifyUUID := normalizeBLEUUID(profile.Connection.BLENotifyCharacteristic)
	if writeUUID == "" {
		return nil, fmt.Errorf("ble profile does not define a write characteristic")
	}

	services, err := discoverServicesForProfile(device, serviceUUID)
	if err != nil {
		return nil, err
	}

	var writeChar *bluetooth.DeviceCharacteristic
	var writeCharWWR *bluetooth.DeviceCharacteristic
	var wwrFallback bool
	var readChar *bluetooth.DeviceCharacteristic
	// 带响应写专用特征由档案/环境开关决定（空 = 不启用）；找不到就退回原 writeChar 的行为。
	wwrUUID := bleWWRCharShortUUIDFor(profile)
	for _, service := range services {
		if serviceUUID != "" && !bleUUIDMatches(service.UUID().String(), serviceUUID) {
			continue
		}
		chars, err := service.DiscoverCharacteristics(nil)
		if err != nil {
			continue
		}
		for i := range chars {
			characteristic := chars[i]
			charUUID := characteristic.UUID().String()
			if writeChar == nil && bleUUIDMatches(charUUID, writeUUID) {
				copyChar := characteristic
				writeChar = &copyChar
			}
			if wwrUUID != "" && writeCharWWR == nil && bleUUIDMatches(charUUID, wwrUUID) {
				copyChar := characteristic
				writeCharWWR = &copyChar
			}
			if notifyUUID != "" && readChar == nil && bleUUIDMatches(charUUID, notifyUUID) {
				copyChar := characteristic
				readChar = &copyChar
			}
		}
	}
	if writeChar == nil {
		return nil, fmt.Errorf("BLE write characteristic %q was not found", writeUUID)
	}
	if wwrUUID != "" && writeCharWWR == nil {
		wwrFallback = true
		wwrUUID = ""
	}
	if readChar == nil && notifyUUID != "" && profile.ID != types.FlyDigiBS1ProfileID {
		return nil, fmt.Errorf("BLE notify/read characteristic %q was not found", notifyUUID)
	}

	client := &realBLEClient{
		device:          device,
		writeChar:       writeChar,
		writeCharWWR:    writeCharWWR,
		wwrSoftFallback: wwrFallback,
		readChar:        readChar,
		notifications:   make(chan []byte, 8),
	}
	if readChar != nil {
		// 订阅失败必须显式失败：订阅不成的连接无法收状态帧，静默继续只会让设备"假在线"。
		var subErr error
		for attempt := 1; attempt <= 3; attempt++ {
			subErr = readChar.EnableNotifications(client.enqueueNotification)
			if subErr == nil {
				client.notificationsEnabled = true
				break
			}
			time.Sleep(400 * time.Millisecond)
		}
		if subErr != nil {
			return nil, fmt.Errorf("BLE 通知订阅失败（%s，已重试 3 次）：%w —— "+
				"设备层收帧全靠这条通知，静默放行会变成「连接成功但收不到任何应答」",
				readChar.UUID().String(), subErr)
		}
	}
	return client, nil
}

func discoverServicesForProfile(device bluetooth.Device, serviceUUID string) ([]bluetooth.DeviceService, error) {
	if serviceUUID == "" {
		return device.DiscoverServices(nil)
	}
	parsed, err := bluetooth.ParseUUID(serviceUUID)
	if err != nil {
		return nil, err
	}
	services, err := device.DiscoverServices([]bluetooth.UUID{parsed})
	if err == nil {
		return services, nil
	}
	return device.DiscoverServices(nil)
}

func (c *realBLEClient) WriteBLECommand(ctx context.Context, payload []byte, withResponse bool) error {
	if c.writeChar == nil {
		return fmt.Errorf("BLE write characteristic is not configured")
	}
	if len(payload) > maxBLEFrameBytes {
		return fmt.Errorf("ble command exceeded %d bytes", maxBLEFrameBytes)
	}
	if err := ctxWithDefault(ctx).Err(); err != nil {
		return err
	}
	payload = append([]byte(nil), payload...)
	var n int
	var err error
	if withResponse {
		// 带响应写优先走专用特征（黑鲨 = ae10）：ae41 属性 0x04（只支持不带响应），对它调 Write
		// 必然失败；而 ae10（0x0A）支持带响应写，只有它能给出"帧真的到达"的往返耗时。
		if c.writeCharWWR != nil {
			n, err = c.writeCharWWR.Write(payload)
			if err != nil {
				return fmt.Errorf("带响应写（%s）失败: %w", c.writeCharWWR.UUID().String(), err)
			}
			if n != len(payload) {
				return io.ErrShortWrite
			}
			return nil
		}
		if c.wwrSoftFallback {
			// 黑鲨软回落：writeChar 是 ae41，对它调 Write 必败，退回不带响应写。
			n, err = c.writeChar.WriteWithoutResponse(payload)
		} else {
			// 其余档案保留上游语义：withResponse 就对 writeChar 调 Write。
			n, err = c.writeChar.Write(payload)
		}
	} else {
		n, err = c.writeChar.WriteWithoutResponse(payload)
		if err != nil {
			// 回退写绝不能覆盖原错误。
			//
			// 原实现是 n, err = c.writeChar.Write(payload)，而 ae41 只支持 write-without-response
			// ⇒ 回退必然以 errNoWrite（"bluetooth: write not supported"）失败，于是真正的原因
			// （GATT 写被拒 = 链路已断）被丢掉，日志读起来像"这条特征不支持写"。
			if fbN, fbErr := c.writeChar.Write(payload); fbErr == nil {
				n, err = fbN, nil
			} else {
				err = fmt.Errorf("%w（回退带响应写也失败: %v）", err, fbErr)
			}
		}
	}
	if err == nil && n != len(payload) {
		return io.ErrShortWrite
	}
	return err
}

// ControlWriteMode 是**客户端如实报出**的控制写模式（供 `WriteModeSummary` 取用）。
//
// 必须由拿到特征的对象回答，而不是按"意图"猜：默认开 ae10 之后多了一条软回落路径（没枚举到
// ae10 ⇒ 退回 ae41），只看 writeWithResponse 标志会把软回落报成"带响应写"，让日志与 A/B 结论失真。
// 判定与 WriteBLECommand 同源（writeCharWWR != nil）。
func (c *realBLEClient) ControlWriteMode() string {
	if c.writeCharWWR != nil {
		return fmt.Sprintf("带响应写（特征 %s，等 ATT 应答 ⇒ 耗时为真实往返）", c.writeCharWWR.UUID().String())
	}
	if c.writeChar != nil {
		return fmt.Sprintf("不带响应写（特征 %s；带响应写特征未取得 ⇒ 回落）", c.writeChar.UUID().String())
	}
	return "写特征未配置"
}

func (c *realBLEClient) ReadBLEFrame(ctx context.Context) ([]byte, error) {
	ctx = ctxWithDefault(ctx)
	if c.notificationsEnabled {
		select {
		case frame := <-c.notifications:
			return frame, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if c.readChar == nil {
		return nil, fmt.Errorf("BLE read/notify characteristic is not configured")
	}
	done := make(chan readBLEFrameResult, 1)
	go func() {
		buf := make([]byte, maxBLEFrameBytes)
		n, err := c.readChar.Read(buf)
		if err != nil {
			done <- readBLEFrameResult{err: err}
			return
		}
		done <- readBLEFrameResult{frame: append([]byte(nil), buf[:n]...)}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-done:
		return result.frame, result.err
	}
}

func (c *realBLEClient) Close() error {
	return c.device.Disconnect()
}

func (c *realBLEClient) BLEAddress() string {
	return c.address
}

func (c *realBLEClient) SetBLENotificationCallback(callback func([]byte)) {
	c.notificationMutex.Lock()
	defer c.notificationMutex.Unlock()
	c.notificationCallback = callback
}

func (c *realBLEClient) enqueueNotification(buf []byte) {
	frame := append([]byte(nil), buf...)
	select {
	case c.notifications <- frame:
	default:
		select {
		case <-c.notifications:
		default:
		}
		c.notifications <- frame
	}
	c.notificationMutex.RLock()
	callback := c.notificationCallback
	c.notificationMutex.RUnlock()
	if callback != nil {
		callback(frame)
	}
}

type readBLEFrameResult struct {
	frame []byte
	err   error
}

func ctxWithDefault(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
