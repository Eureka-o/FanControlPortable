package deviceprofileexec

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
	"tinygo.org/x/bluetooth"
)

const maxBLEFrameBytes = 64 * 1024

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
}

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
		writeWithResponse: profile.Connection.BLEWriteWithResponse,
		requestTimeout:    durationFromMillis(profile.Connection.RequestTimeoutMs, defaultBLEOperationTimeout, maxProfileHTTPTimeout),
		minSendInterval:   durationFromMillis(profile.Connection.MinSendIntervalMs, defaultMinSendInterval, maxProfileSendInterval),
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

func (e *BLEExecutor) writeRawLocked(ctx context.Context, payload []byte) error {
	if len(payload) == 0 {
		return fmt.Errorf("ble command is empty")
	}
	if err := e.waitForSendSlot(ctxWithDefault(ctx)); err != nil {
		return err
	}
	opCtx, cancel := e.operationContext(ctx)
	defer cancel()
	return e.client.WriteBLECommand(opCtx, payload, e.writeWithResponse)
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
	if e.profile.ID == types.FlyDigiBS1ProfileID && cachedAddress != "" {
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
	if notificationClient, ok := client.(BLENotificationConsumer); ok && e.profile.ID == types.FlyDigiBS1ProfileID {
		notificationClient.SetBLENotificationCallback(e.handleBS1Notification)
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
	readChar             *bluetooth.DeviceCharacteristic
	notifications        chan []byte
	notificationsEnabled bool
	notificationMutex    sync.RWMutex
	notificationCallback func([]byte)
}

func discoverBLEClient(device bluetooth.Device, profile types.DeviceProfile) (*realBLEClient, error) {
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
	var readChar *bluetooth.DeviceCharacteristic
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
			if notifyUUID != "" && readChar == nil && bleUUIDMatches(charUUID, notifyUUID) {
				copyChar := characteristic
				readChar = &copyChar
			}
		}
	}
	if writeChar == nil {
		return nil, fmt.Errorf("BLE write characteristic %q was not found", writeUUID)
	}
	if readChar == nil && notifyUUID != "" && profile.ID != types.FlyDigiBS1ProfileID {
		return nil, fmt.Errorf("BLE notify/read characteristic %q was not found", notifyUUID)
	}

	client := &realBLEClient{
		device:        device,
		writeChar:     writeChar,
		readChar:      readChar,
		notifications: make(chan []byte, 8),
	}
	if readChar != nil {
		if err := readChar.EnableNotifications(client.enqueueNotification); err == nil {
			client.notificationsEnabled = true
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
		n, err = c.writeChar.Write(payload)
	} else {
		n, err = c.writeChar.WriteWithoutResponse(payload)
		if err != nil {
			n, err = c.writeChar.Write(payload)
		}
	}
	if err == nil && n != len(payload) {
		return io.ErrShortWrite
	}
	return err
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
