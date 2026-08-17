package coreapp

import (
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

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

type deviceRuntimeSnapshotData struct {
	Connected            bool
	Runtime              deviceRuntimeStatus
	Settings             *types.DeviceSettings
	CurrentData          *types.FanData
	Profile              types.DeviceProfile
	Capabilities         types.DeviceCapabilities
	ProductID            uint16
	Model                string
	LastSuccessfulReadAt time.Time
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
	return a.deviceRuntimeSnapshot().Runtime
}

func (a *CoreApp) connectionFlightSnapshot(runtime deviceRuntimeStatus) connectionFlightSnapshot {
	return a.connectionFlights.snapshot(connectionFlightSnapshotInput{
		State:               runtime.State,
		CanControl:          runtime.CanControl,
		ReconnectInProgress: a.reconnectInProgress.Load(),
		Suspended:           a.systemSuspended.Load(),
	})
}

// deviceRuntimeSnapshot keeps normal status reads non-blocking; diagnostics
// may opt into the configured manager profile because it runs off the control
// path and needs the same profile identity used by the connection adapter.
func (a *CoreApp) deviceRuntimeSnapshot(includeConfiguredProfile ...bool) deviceRuntimeSnapshotData {
	a.mutex.RLock()
	coreConnected := a.isConnected
	manager := a.deviceManager
	settings := a.deviceSettings
	lastSuccessfulReadAt := a.lastSuccessfulDeviceReadAt
	a.mutex.RUnlock()

	connected := coreConnected && manager != nil && manager.IsConnected()
	var profile types.DeviceProfile
	if a.configManager != nil {
		cfg := a.configManager.Get()
		profile = types.ActiveDeviceProfile(&cfg)
	}
	if manager != nil && (connected || (len(includeConfiguredProfile) > 0 && includeConfiguredProfile[0])) {
		if runtimeProfile := manager.ActiveProfile(); runtimeProfile.ID != "" {
			profile = runtimeProfile
		}
	}
	capabilities := profile.Capabilities
	snapshot := deviceRuntimeSnapshotData{
		Connected:            connected,
		Profile:              profile,
		Capabilities:         capabilities,
		LastSuccessfulReadAt: lastSuccessfulReadAt,
	}
	if connected {
		snapshot.Settings = settings
		snapshot.CurrentData = manager.GetCurrentFanData()
		snapshot.Profile = manager.ActiveProfile()
		snapshot.Capabilities = snapshot.Profile.Capabilities
		snapshot.ProductID = manager.GetProductID()
		snapshot.Model = manager.GetModelName()
	}

	phase := a.connectionPhase.Load()
	snapshot.Runtime = resolveDeviceRuntimeStatus(deviceRuntimeStatusInput{
		Connected:     connected,
		Discovering:   phase == deviceConnectionPhaseDiscovering || a.reconnectInProgress.Load(),
		Connecting:    phase == deviceConnectionPhaseConnecting,
		Suspended:     a.systemSuspended.Load(),
		SettingsReady: snapshot.Settings != nil && snapshot.Settings.Available,
		Capabilities:  snapshot.Capabilities,
	})
	return snapshot
}

func automaticControlInputReady(temp types.TemperatureData) bool {
	return temp.BridgeOk && temp.TelemetryFresh && validSmartControlTemperature(temp.ControlTemp)
}
