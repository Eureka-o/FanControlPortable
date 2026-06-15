package deviceprofileexec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

const maxSerialFrameBytes = 64 * 1024

type SerialPort interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
}

type SerialDialer interface {
	OpenSerialPort(profile types.DeviceProfile) (SerialPort, error)
}

type SerialDialerFunc func(profile types.DeviceProfile) (SerialPort, error)

func (f SerialDialerFunc) OpenSerialPort(profile types.DeviceProfile) (SerialPort, error) {
	return f(profile)
}

type SerialExecutor struct {
	profile        types.DeviceProfile
	dialer         SerialDialer
	port           SerialPort
	readCommand    types.DeviceCommandTemplate
	hasReadCommand bool
	setCommand     types.DeviceCommandTemplate
	hasSetCommand  bool
	parsers        CompiledResponseParsers
	delimiter      []byte

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
}

func NewSerialExecutor(profile types.DeviceProfile, dialer SerialDialer) (*SerialExecutor, error) {
	profile = types.NormalizeDeviceProfile(profile, "")
	if profile.Transport != types.DeviceTransportSerial {
		return nil, fmt.Errorf("serial executor requires a serial profile")
	}
	if dialer == nil {
		dialer = DefaultSerialDialer{}
	}
	parsers, err := CompileResponseParsers(profile.ResponseParsers)
	if err != nil {
		return nil, err
	}
	delimiter, err := DecodeSerialDelimiter(profile.Connection.SerialFrameDelimiter)
	if err != nil {
		return nil, err
	}
	readCommand, hasReadCommand := FindCommand(profile.Commands, "readState", "read-state", "state", "status", "read")
	setCommand, hasSetCommand := FindCommand(profile.Commands, "setSpeed", "set-speed", "speed")
	return &SerialExecutor{
		profile:         profile,
		dialer:          dialer,
		readCommand:     readCommand,
		hasReadCommand:  hasReadCommand,
		setCommand:      setCommand,
		hasSetCommand:   hasSetCommand,
		parsers:         parsers,
		delimiter:       delimiter,
		requestTimeout:  durationFromMillis(profile.Connection.RequestTimeoutMs, defaultHTTPTimeout, maxProfileHTTPTimeout),
		minSendInterval: durationFromMillis(profile.Connection.MinSendIntervalMs, defaultMinSendInterval, maxProfileSendInterval),
		maxRetries:      clampInt(profile.Connection.MaxRetries, 0, maxProfileRetryCount),
		retryBackoff:    durationFromMillis(profile.Connection.RetryBackoffMs, defaultRetryBackoff, maxProfileRetryBackoff),
		now:             time.Now,
		sleep:           sleepContext,
	}, nil
}

func (e *SerialExecutor) Profile() types.DeviceProfile {
	return e.profile
}

func (e *SerialExecutor) Open(ctx context.Context) (*types.FanData, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if err := e.ensureOpenLocked(); err != nil {
		return nil, err
	}
	if e.hasReadCommand {
		return e.readStateLocked(ctx)
	}
	state := e.syntheticStateLocked(types.FanSpeedValue{})
	e.lastState = cloneFanData(state)
	return state, nil
}

func (e *SerialExecutor) Close() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.port == nil {
		return nil
	}
	err := e.port.Close()
	e.port = nil
	return err
}

func (e *SerialExecutor) ReadState(ctx context.Context) (*types.FanData, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if err := e.ensureOpenLocked(); err != nil {
		return nil, err
	}
	if e.hasReadCommand {
		return e.readStateLocked(ctx)
	}
	if e.lastState != nil {
		return cloneFanData(e.lastState), nil
	}
	state := e.syntheticStateLocked(types.FanSpeedValue{})
	e.lastState = cloneFanData(state)
	return state, nil
}

func (e *SerialExecutor) SetSpeed(ctx context.Context, speed types.FanSpeedValue) (*types.FanData, error) {
	speed = speed.Normalized()
	if speed.Unit != types.NormalizeFanSpeedUnit(e.profile.SpeedUnit) {
		return nil, fmt.Errorf("speed unit %q does not match profile unit %q", speed.Unit, e.profile.SpeedUnit)
	}
	if !e.hasSetCommand {
		return nil, fmt.Errorf("serial profile does not define a setSpeed command")
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()

	if err := e.ensureOpenLocked(); err != nil {
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
		if sleepErr := e.sleep(ctx, e.retryDelay(attempt)); sleepErr != nil {
			return nil, sleepErr
		}
	}
	return nil, err
}

func (e *SerialExecutor) setSpeedOnceLocked(ctx context.Context, speed types.FanSpeedValue) (*types.FanData, error) {
	vars := SpeedVarsFromValue(speed)
	payload, err := e.serialCommandBytes(e.setCommand, vars)
	if err != nil {
		return nil, err
	}
	if err := e.waitForSendSlot(ctx); err != nil {
		return nil, err
	}
	if err := writeAll(e.port, payload); err != nil {
		return nil, err
	}
	if !e.shouldReadResponse() {
		return e.syntheticStateLocked(speed), nil
	}
	body, err := e.readFrame(ctx)
	if err != nil {
		return nil, err
	}
	return e.fanDataFromBody(body, speed)
}

func (e *SerialExecutor) readStateLocked(ctx context.Context) (*types.FanData, error) {
	payload, err := e.serialCommandBytes(e.readCommand, SpeedVars{})
	if err != nil {
		return nil, err
	}
	if err := writeAll(e.port, payload); err != nil {
		return nil, err
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

func (e *SerialExecutor) serialCommandBytes(command types.DeviceCommandTemplate, vars SpeedVars) ([]byte, error) {
	body, _, err := EncodeCommand(command, vars)
	if err != nil {
		return nil, err
	}
	if len(e.delimiter) > 0 {
		body = append(append([]byte(nil), body...), e.delimiter...)
	}
	return body, nil
}

func (e *SerialExecutor) shouldReadResponse() bool {
	return len(e.parsers.parsers) > 0
}

func (e *SerialExecutor) readFrame(ctx context.Context) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, e.requestTimeout)
	defer cancel()

	buf := make([]byte, 256)
	var out []byte
	for len(out) < maxSerialFrameBytes {
		select {
		case <-ctx.Done():
			if len(out) > 0 {
				return out, nil
			}
			return nil, ctx.Err()
		default:
		}
		n, err := e.port.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
			if len(e.delimiter) == 0 || bytes.Contains(out, e.delimiter) {
				return trimSerialFrame(out, e.delimiter), nil
			}
			continue
		}
		if err != nil {
			if err == io.EOF && len(out) > 0 {
				return trimSerialFrame(out, e.delimiter), nil
			}
			return nil, err
		}
	}
	return nil, fmt.Errorf("serial response exceeded %d bytes", maxSerialFrameBytes)
}

func (e *SerialExecutor) fanDataFromBody(body []byte, fallback types.FanSpeedValue) (*types.FanData, error) {
	state, err := e.parsers.Parse(body)
	if err != nil {
		return nil, err
	}
	fallbackValue := 0
	if fallback.Unit != "" {
		fallbackValue = speedValueForProfileState(fallback)
	}
	if !completeStateTarget(&state, fallbackValue) {
		return nil, fmt.Errorf("serial profile response did not contain speed data")
	}
	current := clampForProfile(state.CurrentSpeed, e.profile.SpeedUnit)
	target := clampForProfile(state.TargetSpeed, e.profile.SpeedUnit)
	return e.stateWithValues(current, target, "serial"), nil
}

func (e *SerialExecutor) syntheticStateLocked(speed types.FanSpeedValue) *types.FanData {
	value := speedValueForProfileState(speed)
	if strings.TrimSpace(speed.Unit) != "" {
		current := 0
		if e.lastState != nil {
			current = int(e.lastState.CurrentRPM)
		}
		return e.stateWithValues(current, value, "serial")
	}
	return e.stateWithValues(value, value, "serial")
}

func (e *SerialExecutor) stateWithValues(current, target int, mode string) *types.FanData {
	if mode == "" {
		mode = "serial"
	}
	return &types.FanData{
		CurrentRPM: uint16(clampUint16(current)),
		TargetRPM:  uint16(clampUint16(target)),
		WorkMode:   mode,
		Transport:  types.DeviceTransportSerial,
		SpeedUnit:  types.NormalizeFanSpeedUnit(e.profile.SpeedUnit),
	}
}

func (e *SerialExecutor) ensureOpenLocked() error {
	if e.port != nil {
		return nil
	}
	port, err := e.dialer.OpenSerialPort(e.profile)
	if err != nil {
		return err
	}
	e.port = port
	return nil
}

func (e *SerialExecutor) waitForSendSlot(ctx context.Context) error {
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

func (e *SerialExecutor) retryDelay(attempt int) time.Duration {
	if e.retryBackoff <= 0 {
		return 0
	}
	return time.Duration(attempt+1) * e.retryBackoff
}

func DecodeSerialDelimiter(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	switch strings.ToLower(value) {
	case "", "none":
		return nil, nil
	case `\n`, "lf":
		return []byte{'\n'}, nil
	case `\r`, "cr":
		return []byte{'\r'}, nil
	case `\r\n`, "crlf":
		return []byte{'\r', '\n'}, nil
	case `\t`, "tab":
		return []byte{'\t'}, nil
	default:
		return []byte(value), nil
	}
}

func trimSerialFrame(frame, delimiter []byte) []byte {
	if len(delimiter) == 0 {
		return frame
	}
	if idx := bytes.Index(frame, delimiter); idx >= 0 {
		return frame[:idx]
	}
	return frame
}

func writeAll(port SerialPort, payload []byte) error {
	for len(payload) > 0 {
		n, err := port.Write(payload)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		payload = payload[n:]
	}
	return nil
}

func speedValueForProfileState(speed types.FanSpeedValue) int {
	speed = speed.Normalized()
	if types.IsRPMSpeedUnit(speed.Unit) {
		return clampUint16(speed.Value)
	}
	return types.PercentTicksToIntegerPercent(speed.Value)
}

func cloneFanData(data *types.FanData) *types.FanData {
	if data == nil {
		return nil
	}
	clone := *data
	return &clone
}
