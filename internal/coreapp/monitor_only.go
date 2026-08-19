package coreapp

import (
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/ipc"
)

const monitorOnlyRestartDelay = 180 * time.Millisecond

func (a *CoreApp) monitorOnlyActive() bool {
	return a != nil && a.monitorOnly.Load()
}

// SetSessionMonitorOnly carries the temporary homepage choice across the core
// restart without changing the persisted configuration.
func (a *CoreApp) SetSessionMonitorOnly(enabled bool) {
	if a == nil {
		return
	}
	a.sessionMonitorOnly.Store(enabled)
	if a.configManager != nil {
		a.refreshMonitorOnlyRuntime(a.configManager.Get().MonitorOnly)
	}
}

func (a *CoreApp) refreshMonitorOnlyRuntime(persisted bool) {
	if a == nil {
		return
	}
	active := persisted || a.sessionMonitorOnly.Load()
	previous := a.monitorOnly.Swap(active)
	if previous == active {
		if active && a.deviceManager != nil {
			a.deviceManager.BlockWrites()
		}
		return
	}

	if active {
		a.logInfo("仅监控模式已启用：停止设备搜索、连接和硬件写入")
		a.cancelReconnect()
		if a.wifiScanControl != nil {
			a.wifiScanControl.Cancel()
		}
		if a.deviceManager != nil {
			a.deviceManager.BlockWrites()
		}
		if a.deviceManager != nil && a.deviceManager.IsConnected() {
			a.deviceManager.DisconnectSilently()
			newDeviceConnectionFlow(a).setRuntimeDisconnected("monitor-only")
			if a.ipcServer != nil {
				a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
			}
		}
		return
	}

	a.logInfo("仅监控模式已关闭：恢复设备搜索和连接")
	if a.deviceManager != nil {
		a.deviceManager.UnblockWrites()
	}
	if !a.systemSuspended.Load() {
		a.requestReconnect("monitor-only-disabled", []time.Duration{0})
	}
}

func (a *CoreApp) scheduleCoreRestart() {
	if a == nil {
		return
	}
	a.safeGo("restart-core", func() {
		time.Sleep(monitorOnlyRestartDelay)
		select {
		case a.quitChan <- true:
		default:
		}
	})
}

func (a *CoreApp) restartCore(params ipc.RestartCoreParams) bool {
	if params.MonitorOnlySession {
		a.SetSessionMonitorOnly(true)
	}
	a.scheduleCoreRestart()
	return true
}
