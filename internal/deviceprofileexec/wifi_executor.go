package deviceprofileexec

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

const (
	defaultHTTPTimeout     = 2 * time.Second
	defaultMinSendInterval = 100 * time.Millisecond
	defaultRetryBackoff    = 150 * time.Millisecond
	maxHTTPBodyBytes       = 64 * 1024
	maxProfileHTTPTimeout  = 30 * time.Second
	maxProfileSendInterval = 60 * time.Second
	maxProfileRetryBackoff = 10 * time.Second
	maxProfileRetryCount   = 5
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type WiFiExecutor struct {
	profile         types.DeviceProfile
	protocol        wifiProtocol
	transport       wifiTransport
	minSendInterval time.Duration

	sendMutex sync.Mutex
	lastSend  time.Time
	now       func() time.Time
	sleep     func(context.Context, time.Duration) error
}

func NewWiFiExecutor(profile types.DeviceProfile, fallbackEndpoint string, client HTTPDoer) (*WiFiExecutor, error) {
	profile = types.NormalizeDeviceProfile(profile, fallbackEndpoint)
	if profile.Transport != types.DeviceTransportWiFi {
		return nil, fmt.Errorf("wifi executor requires a wifi profile")
	}
	baseURL, err := normalizeBaseEndpoint(profile.Connection.Endpoint)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	protocol, err := newWiFiProtocol(profile)
	if err != nil {
		return nil, err
	}
	sleep := sleepContext
	minSendInterval := durationFromMillis(profile.Connection.MinSendIntervalMs, defaultMinSendInterval, maxProfileSendInterval)
	executor := &WiFiExecutor{
		profile:         profile,
		protocol:        protocol,
		minSendInterval: minSendInterval,
		now:             time.Now,
		sleep:           sleep,
	}
	executor.transport = newHTTPWiFiTransport(profile.Connection, baseURL, client, &executor.sleep)
	return executor, nil
}

func (e *WiFiExecutor) Profile() types.DeviceProfile {
	return e.profile
}

func (e *WiFiExecutor) ReadState(ctx context.Context) (*types.FanData, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req := e.protocol.StateRequest()
	body, err := e.transport.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	return e.protocol.ParseState(body, 0)
}

func (e *WiFiExecutor) SetSpeed(ctx context.Context, speed types.FanSpeedValue) (*types.FanData, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	speed = speed.Normalized()
	if speed.Unit != types.NormalizeFanSpeedUnit(e.profile.SpeedUnit) {
		return nil, fmt.Errorf("speed unit %q does not match profile unit %q", speed.Unit, e.profile.SpeedUnit)
	}

	req, err := e.protocol.SpeedRequest(speed)
	if err != nil {
		return nil, err
	}
	if err := e.waitForSendSlot(ctx); err != nil {
		return nil, err
	}
	responseBody, err := e.transport.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := e.protocol.ValidateSetSpeedResponse(responseBody); err != nil {
		return nil, err
	}

	fanData, err := e.ReadState(ctx)
	if err != nil {
		return nil, err
	}
	if fanData.TargetRPM == 0 {
		fanData.TargetRPM = uint16(clampUint16(speedValueForProfileState(speed)))
	}
	return fanData, nil
}

func clampForProfile(value int, unit string) int {
	if types.IsRPMSpeedUnit(unit) {
		return clampUint16(types.ClampRPM(value))
	}
	return types.ClampFanPercent(value)
}

func clampUint16(value int) int {
	if value < 0 {
		return 0
	}
	if value > 65535 {
		return 65535
	}
	return value
}
