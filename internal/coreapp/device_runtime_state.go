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

// monitoringConfigSnapshot 返回设备上报 / 监控热路径共用的 revision 缓存配置快照。
//
// 与 trayConfigSnapshot（lifecycle.go）共用同一份 revision 缓存（trayConfig* +
// trayConfigMu）：configManager.Revision() 未变时直接复用缓存的 AppConfig，
// 避免每条设备上报（黑鲨 0x06 @2Hz、每条 BLE 通知、每帧飞智）和每个监控 tick
// 都做一次 cloneAppConfig 深拷贝（DeviceProfiles / FanCurveProfilesByDevice /
// SmartControl 等大字段全部重建）。configManager 的每一条写入路径
// （Load/Set/Update/MutateAndSave/MutateIfRevision*）都会递增 revision，
// 所以缓存只在配置真正变化后失效一次。
//
// 约束：返回值是浅拷贝，与缓存共享底层 slice/map —— 调用方只能读；需要写配置的
// 调用方必须在变更分支里另取 configManager.Get() 的新可写克隆
// （见 onFanDataUpdate 的设备手动模式分支）。
//
// 正确性边界：监控循环的环内学习只改它本地 cfg 副本的 cfg.SmartControl，
// 不 bump revision 也不进 manager —— 本快照的三个消费方
// （deviceRuntimeSnapshot / autoControlActive / blackSharkHostControlsSpeed）
// 只读 AutoControl、CustomSpeedEnabled、BlackSharkHostVariableSpeed 与
// ActiveDeviceProfile（仅依赖 revision 管理的 DeviceProfiles / 兼容开关 /
// 档案 ID 字段），都不读 SmartControl，因此不受该边界影响。
func (a *CoreApp) monitoringConfigSnapshot() types.AppConfig {
	if a == nil || a.configManager == nil {
		return types.AppConfig{}
	}
	revision := a.configManager.Revision()
	a.trayConfigMu.Lock()
	defer a.trayConfigMu.Unlock()
	if !a.trayConfigReady || revision != a.trayConfigRev {
		a.refreshTrayConfigLocked()
	}
	return a.trayConfig
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
		// revision 缓存快照：ActiveDeviceProfile 只读 revision 管理的字段且不改 cfg；
		// 连接态（或诊断显式要求）会用 manager.ActiveProfile() 覆盖它，这里只是兜底。
		cfg := a.monitoringConfigSnapshot()
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
