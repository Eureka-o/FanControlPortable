package coreapp

import "github.com/TIANLI0/THRM/internal/types"

const (
	deviceRuntimeStateDisconnected = "disconnected"
	deviceRuntimeStateDiscovering  = "discovering"
	deviceRuntimeStateConnecting   = "connecting"
	deviceRuntimeStateCapabilities = "capabilities"
	deviceRuntimeStateReady        = "ready"
	deviceRuntimeStateUnavailable  = "unavailable"
)

const (
	deviceConnectionPhaseNone int32 = iota
	deviceConnectionPhaseDiscovering
	deviceConnectionPhaseConnecting
)

type deviceRuntimeStatus struct {
	State      string `json:"state"`
	CanControl bool   `json:"canControl"`
}

type deviceRuntimeStatusInput struct {
	Connected     bool
	Discovering   bool
	Connecting    bool
	Suspended     bool
	SettingsReady bool
	Capabilities  types.DeviceCapabilities
}

func resolveDeviceRuntimeStatus(input deviceRuntimeStatusInput) deviceRuntimeStatus {
	if input.Suspended {
		return deviceRuntimeStatus{State: deviceRuntimeStateUnavailable}
	}
	if !input.Connected {
		if input.Connecting {
			return deviceRuntimeStatus{State: deviceRuntimeStateConnecting}
		}
		if input.Discovering {
			return deviceRuntimeStatus{State: deviceRuntimeStateDiscovering}
		}
		return deviceRuntimeStatus{State: deviceRuntimeStateDisconnected}
	}
	if !input.Capabilities.SupportsSetSpeed {
		return deviceRuntimeStatus{State: deviceRuntimeStateUnavailable}
	}
	if !input.SettingsReady {
		return deviceRuntimeStatus{State: deviceRuntimeStateCapabilities}
	}
	return deviceRuntimeStatus{State: deviceRuntimeStateReady, CanControl: true}
}

func (a *CoreApp) deviceRuntimeStatus() deviceRuntimeStatus {
	a.mutex.RLock()
	connected := a.isConnected
	manager := a.deviceManager
	settingsReady := a.deviceSettings != nil && a.deviceSettings.Available
	a.mutex.RUnlock()
	connected = connected && manager != nil && manager.IsConnected()

	phase := a.connectionPhase.Load()
	return resolveDeviceRuntimeStatus(deviceRuntimeStatusInput{
		Connected:     connected,
		Discovering:   phase == deviceConnectionPhaseDiscovering || a.reconnectInProgress.Load(),
		Connecting:    phase == deviceConnectionPhaseConnecting,
		Suspended:     a.systemSuspended.Load(),
		SettingsReady: settingsReady,
		Capabilities:  a.activeDeviceCapabilities(),
	})
}

func automaticControlInputReady(temp types.TemperatureData) bool {
	return temp.BridgeOk && temp.TelemetryFresh && validSmartControlTemperature(temp.ControlTemp)
}
