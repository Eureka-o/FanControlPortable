package guiapp

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/types"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	guiHiddenMemReleaseCooldown   = 30 * time.Second
	guiPeriodicMemReleaseInterval = 170 * time.Second
	guiMemReleaseBusyDefer        = 20 * time.Second
	guiUserActivityQuietWindow    = 15 * time.Second
)

func (a *App) emit(eventName string, data ...any) {
	if a == nil {
		return
	}
	if a.emitEvent != nil {
		a.emitEvent(eventName, data...)
		return
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, eventName, data...)
	}
}

// SetInitialWindowHidden sets the initial window hidden state (e.g. minimized autostart).
func (a *App) SetInitialWindowHidden(hidden bool) {
	if a != nil {
		a.windowHidden.Store(hidden)
	}
}

// SetWindowHidden updates whether the GUI window is hidden/minimized and flushes pending telemetry on restore.
func (a *App) SetWindowHidden(hidden bool) {
	if a == nil {
		return
	}
	wasHidden := a.windowHidden.Swap(hidden)
	if wasHidden && !hidden {
		a.flushPendingHiddenTelemetry()
		if a.sensorCatalogWanted.Load() && a.ipcClient != nil {
			go func() {
				if temp := a.syncSensorCatalogLease(true); temp.UpdateTime > 0 {
					a.emit(ipc.EventTemperatureUpdate, temp)
				}
			}()
		}
		return
	}
	if !wasHidden && hidden {
		a.releaseHiddenMemory()
		if a.sensorCatalogWanted.Load() && a.ipcClient != nil {
			go a.syncSensorCatalogLease(false)
		}
	}
}

func (a *App) isUpdateDownloadActive() bool {
	if a == nil {
		return false
	}
	a.updateMutex.Lock()
	defer a.updateMutex.Unlock()
	return a.updateControl != nil
}

func (a *App) isMemoryReleaseBusy(now time.Time, checkUserActivity bool) bool {
	if a == nil {
		return false
	}
	if a.shuttingDown.Load() ||
		a.memReclaimRunning.Load() ||
		a.sensorCatalogWanted.Load() ||
		a.isUpdateDownloadActive() {
		return true
	}
	if ts := a.lastImageTransferNs.Load(); ts > 0 && now.Sub(time.Unix(0, ts)) < guiUserActivityQuietWindow {
		return true
	}
	if checkUserActivity {
		if ts := a.lastUserActivityNs.Load(); ts > 0 && now.Sub(time.Unix(0, ts)) < guiUserActivityQuietWindow {
			return true
		}
	}
	return false
}

func evaluateGuiMemoryReclaim(
	now, startedAt, lastTrim, deferUntil time.Time,
	hidden, busy bool,
) (run bool, trimWorkingSet bool, nextDeferUntil time.Time) {
	ref := lastTrim
	if ref.IsZero() {
		ref = startedAt
	}
	if ref.IsZero() || now.Sub(ref) < guiPeriodicMemReleaseInterval {
		return false, false, time.Time{}
	}
	if !deferUntil.IsZero() && now.Before(deferUntil) {
		return false, false, deferUntil
	}
	if busy {
		return false, false, now.Add(guiMemReleaseBusyDefer)
	}
	return true, hidden, time.Time{}
}

func (a *App) runMemoryReclaim(trimWorkingSet bool) {
	if a == nil || !a.memReclaimRunning.CompareAndSwap(false, true) {
		return
	}
	reclaim := a.memReclaimFn
	go func() {
		defer a.memReclaimRunning.Store(false)
		if reclaim != nil {
			reclaim(trimWorkingSet)
			return
		}
		debug.FreeOSMemory()
		if trimWorkingSet {
			trimProcessWorkingSet()
		}
	}()
}

func (a *App) maybeRunPeriodicMemoryReclaim(now time.Time) {
	if a == nil || a.shuttingDown.Load() || a.startedAt.IsZero() {
		return
	}
	var lastTrim time.Time
	lastNs := a.lastIdleMemTrimNs.Load()
	if lastNs > 0 {
		lastTrim = time.Unix(0, lastNs)
	}
	var deferUntil time.Time
	if deferNs := a.memReclaimDeferUntilNs.Load(); deferNs > 0 {
		deferUntil = time.Unix(0, deferNs)
	}
	busy := a.isMemoryReleaseBusy(now, true)
	run, trimWS, nextDefer := evaluateGuiMemoryReclaim(
		now,
		a.startedAt,
		lastTrim,
		deferUntil,
		a.windowHidden.Load(),
		busy,
	)
	if !run {
		if nextDefer.IsZero() {
			a.memReclaimDeferUntilNs.Store(0)
		} else {
			a.memReclaimDeferUntilNs.Store(nextDefer.UnixNano())
		}
		return
	}
	nowNs := now.UnixNano()
	if !a.lastIdleMemTrimNs.CompareAndSwap(lastNs, nowNs) {
		return
	}
	a.memReclaimDeferUntilNs.Store(0)
	a.runMemoryReclaim(trimWS)
}

func (a *App) releaseHiddenMemory() {
	if a == nil || a.shuttingDown.Load() {
		return
	}
	now := time.Now()
	if a.isMemoryReleaseBusy(now, false) {
		return
	}
	nowNs := now.UnixNano()
	last := a.lastIdleMemTrimNs.Load()
	if last > 0 && time.Duration(nowNs-last) < guiHiddenMemReleaseCooldown {
		return
	}
	if !a.lastIdleMemTrimNs.CompareAndSwap(last, nowNs) {
		return
	}
	a.memReclaimDeferUntilNs.Store(0)
	a.runMemoryReclaim(true)
}

func (a *App) flushPendingHiddenTelemetry() {
	if a == nil || !a.telemetryDirty.Swap(false) {
		return
	}
	a.mutex.RLock()
	hasTemp := a.currentTemp.UpdateTime > 0
	temp := a.currentTemp
	var fanData *types.FanData
	if a.latestFanData != nil {
		copied := *a.latestFanData
		fanData = &copied
	}
	var hostFx *types.BlackSharkHostEffects
	if a.latestHostEffects != nil {
		copied := *a.latestHostEffects
		hostFx = &copied
	}
	a.mutex.RUnlock()

	if hasTemp {
		a.emit("temperature-update", temp)
	}
	if fanData != nil {
		a.emit("fan-data-update", *fanData)
	}
	if hostFx != nil {
		a.emit("blackshark-host-effects", *hostFx)
	}
}

// ShowWindow 显示主窗口
func (a *App) ShowWindow() {
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
		restorePendingWindowMaximise(a.ctx)
		runtime.WindowSetAlwaysOnTop(a.ctx, false)
	}
	if a == nil {
		return
	}
	a.windowHidden.Store(false)
	a.emit("window-visibility-change", map[string]any{
		"visible":      true,
		"historyDirty": a.historyDirty.Swap(false),
	})
	a.flushPendingHiddenTelemetry()
}

// HideWindow 隐藏主窗口到托盘
func (a *App) HideWindow() {
	if a == nil {
		return
	}
	a.windowHidden.Store(true)
	a.emit("window-visibility-change", map[string]any{
		"visible": false,
	})
	a.releaseHiddenMemory()
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
	}
}

// QuitApp 完全退出应用
func (a *App) QuitApp() {
	guiLogger.Info("GUI 请求退出")
	a.signalShutdown()

	if a.ipcClient != nil {
		a.ipcClient.Close()
	}

	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// QuitAll 完全退出应用（包括核心服务）
func (a *App) QuitAll() {
	guiLogger.Info("GUI 请求完全退出（包括核心服务）")
	a.signalShutdown()

	a.sendRequest(ipc.ReqQuitApp, nil)

	if a.ipcClient != nil {
		a.ipcClient.Close()
	}

	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// InitSystemTray 初始化系统托盘（保持API兼容，实际由核心服务处理）
func (a *App) InitSystemTray() {
	// 托盘由核心服务管理，GUI 不需要处理
}

// UpdateGuiResponseTime 更新GUI响应时间（供前端调用）
func (a *App) UpdateGuiResponseTime() error {
	_, err := a.sendRequest(ipc.ReqUpdateGuiResponseTime, nil)
	return err
}

// RestartCore restarts the core and GUI together, optionally carrying a temporary monitor-only mode.
func (a *App) RestartCore(monitorOnlySession bool) error {
	SetMonitorOnlySession(monitorOnlySession)
	resp, err := a.sendRequest(ipc.ReqRestartCore, ipc.RestartCoreParams{MonitorOnlySession: monitorOnlySession})
	if err != nil {
		SetMonitorOnlySession(false)
		return err
	}
	if !resp.Success {
		SetMonitorOnlySession(false)
		return fmt.Errorf("%s", resp.Error)
	}
	if err := ScheduleGUIRestart(monitorOnlySession); err != nil {
		SetMonitorOnlySession(false)
		return err
	}
	return nil
}

// GetDebugInfo 获取调试信息
func (a *App) GetDebugInfo() map[string]any {
	resp, err := a.sendRequest(ipc.ReqGetDebugInfo, nil)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var info map[string]any
	json.Unmarshal(resp.Data, &info)
	return info
}

// SetDebugMode 设置调试模式
func (a *App) SetDebugMode(enabled bool) error {
	resp, err := a.sendRequest(ipc.ReqSetDebugMode, ipc.SetBoolParams{Enabled: enabled})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (a *App) SendDeviceDebugCommand(hexCommand string, waitMs int) (DeviceDebugCommandResult, error) {
	resp, err := a.sendRequest(ipc.ReqSendDeviceDebugCommand, ipc.DeviceDebugCommandParams{Hex: hexCommand, WaitMs: waitMs})
	if err != nil {
		return DeviceDebugCommandResult{}, err
	}
	if !resp.Success {
		return DeviceDebugCommandResult{}, fmt.Errorf("%s", resp.Error)
	}
	var result DeviceDebugCommandResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return DeviceDebugCommandResult{}, err
	}
	return result, nil
}

func (a *App) GetDeviceDebugFrames() []DeviceDebugFrame {
	resp, err := a.sendRequest(ipc.ReqGetDeviceDebugFrames, nil)
	if err != nil || !resp.Success {
		return nil
	}
	var frames []DeviceDebugFrame
	json.Unmarshal(resp.Data, &frames)
	return frames
}
