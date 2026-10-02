//go:build !legacydevice

package device

import (
	"context"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

type compatibilityRuntime struct{}

func (compatibilityRuntime) connectLocked(ctx context.Context, manager *Manager) (bool, map[string]string, bool) {
	if manager.shouldUseWiFiLocked() {
		connected, info := manager.connectWiFiWithContextLocked(ctx)
		return connected, info, true
	}
	if manager.shouldUseSerialLocked() {
		connected, info := manager.connectSerialWithContextLocked(ctx)
		return connected, info, true
	}
	return false, nil, false
}

func (compatibilityRuntime) setTargetSpeedLocked(ctx context.Context, manager *Manager, value int, unit string) (bool, bool) {
	unit = types.NormalizeFanSpeedUnit(unit)
	switch manager.deviceType {
	case types.DeviceTransportWiFi:
		if !manager.shouldUseWiFiLocked() {
			return false, true
		}
		if types.IsRPMSpeedUnit(unit) {
			if !types.IsRPMSpeedUnit(manager.activeProfile.SpeedUnit) {
				manager.logWarn("default WiFi percent profile does not support direct RPM target speed: %d", value)
				return false, true
			}
			return manager.setWiFiTargetSpeedWithContextLocked(ctx, types.NewRPMSpeed(value)), true
		}
		return manager.setWiFiTargetSpeedWithContextLocked(ctx, types.NewPercentTickSpeed(value)), true
	case types.DeviceTransportSerial:
		if types.IsRPMSpeedUnit(unit) {
			return manager.setSerialTargetSpeedWithContextLocked(ctx, types.NewRPMSpeed(value)), true
		}
		return manager.setSerialTargetSpeedWithContextLocked(ctx, types.NewPercentTickSpeed(value)), true
	default:
		return false, false
	}
}

func (compatibilityRuntime) setPercentSpeedLocked(ctx context.Context, manager *Manager, percent int) (bool, bool) {
	switch manager.deviceType {
	case types.DeviceTransportWiFi:
		if !manager.shouldUseWiFiLocked() {
			return false, true
		}
		if types.IsRPMSpeedUnit(manager.activeProfile.SpeedUnit) {
			manager.logWarn("percent speed command rejected because the active WiFi profile uses RPM")
			return false, true
		}
		return manager.setWiFiSpeedWithContextLocked(ctx, types.ClampFanPercent(percent)), true
	case types.DeviceTransportSerial:
		if types.IsRPMSpeedUnit(manager.activeProfile.SpeedUnit) {
			manager.logWarn("percent speed command rejected because the active serial profile uses RPM")
			return false, true
		}
		return manager.setSerialTargetSpeedWithContextLocked(ctx, types.NewPercentSpeed(percent)), true
	default:
		return false, false
	}
}

func (compatibilityRuntime) disconnectLocked(manager *Manager) bool {
	switch manager.deviceType {
	case types.DeviceTransportWiFi:
		manager.disconnectWiFiLocked()
		return true
	case types.DeviceTransportSerial:
		manager.disconnectSerialLocked()
		return true
	default:
		return false
	}
}

func (compatibilityRuntime) refresh(manager *Manager) (bool, bool) {
	transport := manager.GetDeviceType()
	if transport != types.DeviceTransportWiFi && transport != types.DeviceTransportSerial {
		return true, false
	}

	manager.mutex.Lock()
	if !manager.isConnected {
		manager.mutex.Unlock()
		return false, true
	}
	generation := manager.connectionGen.Load()
	profileUnit := types.NormalizeFanSpeedUnit(manager.activeProfile.SpeedUnit)
	wifiExecutor := manager.wifiExecutor
	serialExecutor := manager.serialExecutor
	manager.mutex.Unlock()

	var (
		fanData *types.FanData
		err     error
	)
	if transport == types.DeviceTransportWiFi {
		if wifiExecutor == nil {
			// Legacy fallback still needs the Manager-owned endpoint/client snapshot.
			manager.mutex.Lock()
			if generation != manager.connectionGen.Load() || !manager.isConnected || manager.deviceType != transport {
				manager.mutex.Unlock()
				return true, true
			}
			fanData, err = manager.readWiFiStateLocked()
			manager.mutex.Unlock()
		} else {
			fanData, err = wifiExecutor.ReadState(context.Background())
		}
	} else {
		if serialExecutor == nil {
			manager.mutex.Lock()
			if generation != manager.connectionGen.Load() || !manager.isConnected || manager.deviceType != transport {
				manager.mutex.Unlock()
				return true, true
			}
			fanData, err = manager.readSerialStateLocked()
			manager.mutex.Unlock()
		} else {
			fanData, err = serialExecutor.ReadState(context.Background())
		}
	}
	if err != nil {
		manager.mutex.Lock()
		stale := generation != manager.connectionGen.Load() || !manager.isConnected || manager.deviceType != transport ||
			(transport == types.DeviceTransportWiFi && manager.wifiExecutor != wifiExecutor) ||
			(transport == types.DeviceTransportSerial && manager.serialExecutor != serialExecutor)
		manager.mutex.Unlock()
		if stale {
			return true, true
		}
		manager.logError("%s controller state refresh failed: %v", transport, err)
		return false, true
	}
	if fanData == nil {
		return false, true
	}
	fanData.Transport = transport
	fanData.SpeedUnit = profileUnit

	manager.mutex.Lock()
	if generation != manager.connectionGen.Load() || !manager.isConnected || manager.deviceType != transport ||
		(transport == types.DeviceTransportWiFi && manager.wifiExecutor != wifiExecutor) ||
		(transport == types.DeviceTransportSerial && manager.serialExecutor != serialExecutor) {
		manager.mutex.Unlock()
		return true, true
	}
	manager.currentFanData.Store(fanData)
	callback := manager.onFanDataUpdate
	manager.mutex.Unlock()

	if callback != nil {
		callback(fanData)
	}
	return true, true
}
