//go:build !legacydevice

package device

import (
	"context"

	"github.com/TIANLI0/THRM/internal/types"
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

	var (
		fanData *types.FanData
		err     error
	)
	if transport == types.DeviceTransportWiFi {
		fanData, err = manager.readWiFiStateLocked()
	} else {
		fanData, err = manager.readSerialStateLocked()
	}
	if err != nil {
		manager.mutex.Unlock()
		manager.logError("%s controller state refresh failed: %v", transport, err)
		return false, true
	}
	manager.currentFanData.Store(fanData)
	callback := manager.onFanDataUpdate
	manager.mutex.Unlock()

	if callback != nil {
		callback(fanData)
	}
	return true, true
}
