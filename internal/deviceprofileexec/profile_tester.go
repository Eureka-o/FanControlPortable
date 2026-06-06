package deviceprofileexec

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

const (
	ProfileTestActionConnect  = "connect"
	ProfileTestActionRead     = "readState"
	ProfileTestActionSetSpeed = "setSpeed"

	defaultProfileTestTimeout = 10 * time.Second
	maxProfileTestTimeout     = 30 * time.Second
)

type ProfileTester struct {
	FallbackEndpoint string
	HTTPClient       HTTPDoer
	SerialDialer     SerialDialer
	BLEConnector     BLEConnector
}

func (t ProfileTester) Test(ctx context.Context, params types.DeviceProfileTestParams) (types.DeviceProfileTestResult, error) {
	started := time.Now()
	profile := types.NormalizeDeviceProfile(params.Profile, t.FallbackEndpoint)
	action := normalizeProfileTestAction(params.Action)

	result := types.DeviceProfileTestResult{
		Action:              action,
		Transport:           profile.Transport,
		SpeedUnit:           types.NormalizeFanSpeedUnit(profile.SpeedUnit),
		ProfileID:           profile.ID,
		DisplayName:         profile.DisplayName,
		RequestedSpeedValue: params.SpeedValue,
	}
	defer func() {
		result.DurationMs = time.Since(started).Milliseconds()
	}()

	ctx, cancel := context.WithTimeout(ctxWithDefault(ctx), profileTestTimeout(params.TimeoutMs))
	defer cancel()

	var data *types.FanData
	var err error
	switch profile.Transport {
	case types.DeviceTransportWiFi:
		data, err = t.testWiFi(ctx, profile, action, params.SpeedValue)
	case types.DeviceTransportSerial:
		data, err = t.testSerial(ctx, profile, action, params.SpeedValue)
	case types.DeviceTransportBLE:
		data, err = t.testBLE(ctx, profile, action, params.SpeedValue)
	case types.DeviceTransportHID:
		err = fmt.Errorf("legacy HID profile testing must use the normal device connection path")
	default:
		err = fmt.Errorf("device transport %q is not supported for profile tests", profile.Transport)
	}
	result.DurationMs = time.Since(started).Milliseconds()
	if err != nil {
		return result, err
	}
	result.Connected = true
	result.FanData = data
	result.Message = profileTestMessage(action)
	return result, nil
}

func (t ProfileTester) testWiFi(ctx context.Context, profile types.DeviceProfile, action string, speedValue float64) (*types.FanData, error) {
	executor, err := NewWiFiExecutor(profile, t.FallbackEndpoint, t.HTTPClient)
	if err != nil {
		return nil, err
	}
	switch action {
	case ProfileTestActionSetSpeed:
		return executor.SetSpeed(ctx, speedFromTestValue(profile.SpeedUnit, speedValue))
	case ProfileTestActionConnect, ProfileTestActionRead:
		return executor.ReadState(ctx)
	default:
		return nil, fmt.Errorf("unsupported profile test action %q", action)
	}
}

func (t ProfileTester) testSerial(ctx context.Context, profile types.DeviceProfile, action string, speedValue float64) (*types.FanData, error) {
	executor, err := NewSerialExecutor(profile, t.SerialDialer)
	if err != nil {
		return nil, err
	}
	defer executor.Close()

	switch action {
	case ProfileTestActionConnect:
		return executor.Open(ctx)
	case ProfileTestActionRead:
		return executor.ReadState(ctx)
	case ProfileTestActionSetSpeed:
		return executor.SetSpeed(ctx, speedFromTestValue(profile.SpeedUnit, speedValue))
	default:
		return nil, fmt.Errorf("unsupported profile test action %q", action)
	}
}

func (t ProfileTester) testBLE(ctx context.Context, profile types.DeviceProfile, action string, speedValue float64) (*types.FanData, error) {
	executor, err := NewBLEExecutor(profile, t.BLEConnector)
	if err != nil {
		return nil, err
	}
	defer executor.Close()

	switch action {
	case ProfileTestActionConnect:
		return executor.Open(ctx)
	case ProfileTestActionRead:
		return executor.ReadState(ctx)
	case ProfileTestActionSetSpeed:
		return executor.SetSpeed(ctx, speedFromTestValue(profile.SpeedUnit, speedValue))
	default:
		return nil, fmt.Errorf("unsupported profile test action %q", action)
	}
}

func normalizeProfileTestAction(action string) string {
	switch normalizeKey(action) {
	case "set", "setspeed", "speed":
		return ProfileTestActionSetSpeed
	case "read", "readstate", "state", "status":
		return ProfileTestActionRead
	default:
		return ProfileTestActionConnect
	}
}

func profileTestTimeout(timeoutMs int) time.Duration {
	return durationFromMillis(timeoutMs, defaultProfileTestTimeout, maxProfileTestTimeout)
}

func speedFromTestValue(unit string, value float64) types.FanSpeedValue {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		value = 0
	}
	if types.IsRPMSpeedUnit(unit) {
		return types.NewRPMSpeed(int(math.Round(value)))
	}
	return types.NewPercentTickSpeed(types.PercentFloatToTicks(value))
}

func profileTestMessage(action string) string {
	switch strings.TrimSpace(action) {
	case ProfileTestActionSetSpeed:
		return "Set speed test completed"
	case ProfileTestActionRead:
		return "Read status test completed"
	default:
		return "Connect test completed"
	}
}
