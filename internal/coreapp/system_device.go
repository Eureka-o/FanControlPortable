package coreapp

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

// onShowWindowRequest 显示窗口请求回调
func (a *CoreApp) onShowWindowRequest() {
	a.logInfo("收到显示窗口请求")

	// 通知所有已连接的 GUI 客户端显示窗口
	if a.ipcServer != nil && a.ipcServer.HasClients() {
		a.ipcServer.BroadcastEvent("show-window", nil)
	} else {
		// 没有 GUI 连接，启动 GUI
		a.logInfo("没有 GUI 连接，尝试启动 GUI")
		if err := launchGUI(); err != nil {
			a.logError("启动 GUI 失败: %v", err)
		}
	}
}

// onQuitRequest 退出请求回调
func (a *CoreApp) onQuitRequest() {
	a.logInfo("收到退出请求")

	// 通知所有 GUI 客户端退出
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent("quit", nil)
	}

	// 发送退出信号
	select {
	case a.quitChan <- true:
	default:
	}
}

func didDeviceSwitchToManualMode(previousMode, currentMode string) bool {
	if !isManualDeviceWorkMode(currentMode) {
		return false
	}
	if previousMode == "" {
		return false
	}
	return !isManualDeviceWorkMode(previousMode)
}

func isManualDeviceWorkMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "manual", "manual mode", "manual/fixed gear mode", "fixed gear mode", "fixed", "gear mode",
		"手动", "手动模式", "挡位工作模式":
		return true
	default:
		return false
	}
}

// onFanDataUpdate 风扇数据更新回调
func (a *CoreApp) onFanDataUpdate(fanData *types.FanData) {
	a.mutex.Lock()
	cfg := a.configManager.Get()
	deviceSwitchedToManual := didDeviceSwitchToManualMode(a.lastDeviceMode, fanData.WorkMode)

	// 检查工作模式变化
	// 如果开启了"断连保持配置模式"，则忽略设备状态变化，避免误判
	if deviceSwitchedToManual &&
		cfg.AutoControl &&
		!a.userSetAutoControl &&
		!cfg.IgnoreDeviceOnReconnect {

		a.logInfo("检测到设备切换到挡位工作模式，自动关闭智能变频")
		cfg.AutoControl = false

		a.configManager.Set(cfg)
		a.configManager.Save()

		// 广播配置更新
		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
		}
	} else if deviceSwitchedToManual &&
		cfg.AutoControl &&
		!a.userSetAutoControl &&
		cfg.IgnoreDeviceOnReconnect {
		a.logInfo("检测到设备模式变化，但已开启断连保持配置模式，保持APP配置不变")
	}

	a.lastDeviceMode = fanData.WorkMode

	if a.userSetAutoControl {
		a.userSetAutoControl = false
	}

	a.mutex.Unlock()

	// 广播风扇数据更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventFanDataUpdate, fanData)
	}
}

// onDeviceDisconnect 设备断开回调
func (a *CoreApp) onDeviceDisconnect() {
	a.mutex.Lock()
	wasConnected := a.isConnected
	a.isConnected = false
	a.mutex.Unlock()

	if wasConnected {
		a.logInfo("设备连接已断开，将在健康检查时尝试自动重连")
	}

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}

	if a.autoReconnectSuppressed.Load() {
		a.logInfo("设备已手动断开，跳过自动重连")
		return
	}

	a.requestReconnect("device-disconnect", nil)
}

func defaultReconnectDelays() []time.Duration {
	return []time.Duration{
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
		30 * time.Second,
	}
}

func systemResumeReconnectDelays() []time.Duration {
	return []time.Duration{
		systemResumeReconnectDelay,
		8 * time.Second,
		15 * time.Second,
		20 * time.Second,
		30 * time.Second,
		30 * time.Second,
		60 * time.Second,
		60 * time.Second,
	}
}

func cloneReconnectDelays(delays []time.Duration) []time.Duration {
	if len(delays) == 0 {
		return defaultReconnectDelays()
	}
	cloned := make([]time.Duration, len(delays))
	copy(cloned, delays)
	return cloned
}

type reconnectAttemptResult struct {
	connected   bool
	deviceInfo  map[string]string
	disconnect  func()
	isConnected func() bool
}

func (a *CoreApp) requestReconnect(reason string, retryDelays []time.Duration) {
	if a.autoReconnectSuppressed.Load() {
		a.logInfo("自动重连已被手动断开抑制，忽略请求: %s", reason)
		return
	}
	if a.systemSuspended.Load() {
		a.logDebug("系统仍处于挂起状态，忽略重连请求: %s", reason)
		return
	}

	delays := cloneReconnectDelays(retryDelays)
	ctx, cancel := context.WithCancel(context.Background())
	a.reconnectMutex.Lock()
	if a.autoReconnectSuppressed.Load() || a.systemSuspended.Load() {
		a.reconnectMutex.Unlock()
		cancel()
		return
	}
	if a.reconnectCancel != nil {
		a.reconnectCancel()
	}
	a.reconnectGeneration++
	generation := a.reconnectGeneration
	a.reconnectCancel = cancel
	a.reconnectInProgress.Store(true)
	a.reconnectMutex.Unlock()

	a.safeGo("reconnect@"+reason, func() {
		defer func() {
			cancel()
			a.reconnectMutex.Lock()
			if a.reconnectGeneration == generation {
				a.reconnectCancel = nil
				a.reconnectInProgress.Store(false)
			}
			a.reconnectMutex.Unlock()
		}()
		a.runReconnectLoop(ctx, reason, delays, generation, a.reconnectDevice)
	})
}

func (a *CoreApp) cancelReconnect() {
	a.reconnectMutex.Lock()
	if a.reconnectCancel != nil {
		a.reconnectCancel()
	}
	a.reconnectMutex.Unlock()
}

func (a *CoreApp) suppressReconnect() {
	a.reconnectMutex.Lock()
	a.autoReconnectSuppressed.Store(true)
	if a.reconnectCancel != nil {
		a.reconnectCancel()
	}
	a.reconnectMutex.Unlock()
}

// runReconnectLoop 安排设备重连
func (a *CoreApp) runReconnectLoop(
	ctx context.Context,
	reason string,
	retryDelays []time.Duration,
	generation uint64,
	attempt func() reconnectAttemptResult,
) {
	a.logInfo("启动设备重连流程: %s", reason)

	for i, delay := range retryDelays {
		if ctx.Err() != nil {
			a.logDebug("重连流程已被更新请求取消: %s", reason)
			return
		}
		if a.autoReconnectSuppressed.Load() {
			a.logInfo("自动重连已被手动断开抑制，停止重连流程: %s", reason)
			return
		}
		if a.systemSuspended.Load() {
			a.logDebug("系统进入挂起状态，停止重连流程: %s", reason)
			return
		}

		// 检查是否已经连接（可能其他途径已重连）
		a.mutex.RLock()
		connected := a.isConnected
		a.mutex.RUnlock()

		if connected {
			a.logInfo("设备已重新连接，停止重连尝试")
			return
		}

		if delay > 0 {
			a.logInfo("等待 %v 后尝试第 %d 次重连...", delay, i+1)
			if !waitForReconnectDelay(ctx, delay) {
				a.logDebug("重连等待已被更新请求取消: %s", reason)
				return
			}
		}

		if a.autoReconnectSuppressed.Load() {
			a.logInfo("自动重连已被手动断开抑制，停止重连流程: %s", reason)
			return
		}
		if a.systemSuspended.Load() {
			a.logDebug("系统进入挂起状态，停止重连流程: %s", reason)
			return
		}

		// 再次检查连接状态
		a.mutex.RLock()
		connected = a.isConnected
		a.mutex.RUnlock()

		if connected {
			a.logInfo("设备已重新连接，停止重连尝试")
			return
		}
		if ctx.Err() != nil {
			a.logDebug("重连尝试已被更新请求取消: %s", reason)
			return
		}

		a.logInfo("尝试第 %d 次重连设备...", i+1)
		result := attempt()
		current, connected := a.completeReconnectAttempt(ctx, generation, result, "reconnect@"+reason)
		if !current {
			return
		}
		if connected {
			a.logInfo("设备重连成功")

			// 如果开启了断连保持配置模式，重新应用APP配置
			cfg := a.configManager.Get()
			if cfg.IgnoreDeviceOnReconnect {
				a.logInfo("断连保持配置模式已开启，重新应用APP配置")
				a.reapplyConfigAfterReconnect()
			}

			return
		}
		a.logError("第 %d 次重连失败", i+1)
	}

	if ctx.Err() == nil {
		a.logError("所有重连尝试均失败，等待下次健康检查")
	}
}

func (a *CoreApp) completeReconnectAttempt(ctx context.Context, generation uint64, result reconnectAttemptResult, caller string) (bool, bool) {
	current := false
	connected := false
	func() {
		a.reconnectMutex.Lock()
		defer a.reconnectMutex.Unlock()
		current = ctx.Err() == nil &&
			a.reconnectGeneration == generation &&
			!a.autoReconnectSuppressed.Load() &&
			!a.systemSuspended.Load()
		if current && result.connected {
			connected = result.isConnected == nil || result.isConnected()
			if connected {
				a.finishSuccessfulDeviceConnection(result.deviceInfo, caller)
			}
		} else if result.connected {
			if result.disconnect != nil {
				result.disconnect()
			}
			a.mutex.Lock()
			a.isConnected = false
			a.deviceSettings = nil
			a.mutex.Unlock()
		}
	}()
	return current, connected
}

func waitForReconnectDelay(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (a *CoreApp) reconnectDevice() reconnectAttemptResult {
	a.connectMutex.Lock()
	defer a.connectMutex.Unlock()
	a.connectionPhase.Store(deviceConnectionPhaseConnecting)
	defer a.connectionPhase.Store(deviceConnectionPhaseNone)

	cfg := a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)
	lastConnectionWasNative := a.lastConnectionWasNative.Load()
	activeProfile := a.deviceManager.ActiveProfile()
	connected, deviceInfo := reconnectTransport(
		a.hasSuccessfulConnection.Load(),
		lastConnectionWasNative,
		func() (bool, map[string]string) {
			return reconnectNativeTransport(
				lastConnectionWasNative,
				activeProfile,
				a.deviceManager.ConnectNativeProfile,
				func() (bool, map[string]string) {
					return a.deviceManager.AutoConnectNativeProfiles(cfg.DeviceProfiles)
				},
			)
		},
		func() (bool, map[string]string) {
			a.configureDeviceManager(cfg)
			return a.deviceManager.Connect()
		},
	)
	return reconnectAttemptResult{
		connected:   connected,
		deviceInfo:  deviceInfo,
		disconnect:  a.deviceManager.DisconnectSilently,
		isConnected: a.deviceManager.IsConnected,
	}
}

func reconnectNativeTransport(
	lastConnectionWasNative bool,
	activeProfile types.DeviceProfile,
	tryProfile func(types.DeviceProfile) (bool, map[string]string),
	tryAll func() (bool, map[string]string),
) (bool, map[string]string) {
	activeProfile = types.NormalizeDeviceProfile(activeProfile, "")
	if lastConnectionWasNative && types.IsNativeDeviceTransport(activeProfile.Transport) {
		return tryProfile(activeProfile)
	}
	return tryAll()
}

func reconnectTransport(
	hasSuccessfulConnection bool,
	lastConnectionWasNative bool,
	tryNative func() (bool, map[string]string),
	tryCompatibility func() (bool, map[string]string),
) (bool, map[string]string) {
	if !hasSuccessfulConnection || lastConnectionWasNative {
		connected, info := tryNative()
		if connected || lastConnectionWasNative {
			return connected, info
		}
	}
	return tryCompatibility()
}

func compatibilityConnectionEnabled(cfg types.AppConfig) bool {
	types.NormalizeDeviceProfileConfig(&cfg)
	profile := types.ActiveDeviceProfile(&cfg)
	switch types.NormalizeDeviceTransport(profile.Transport) {
	case types.DeviceTransportWiFi:
		return cfg.WiFiCompatibilityEnabled
	case types.DeviceTransportSerial:
		return cfg.SerialCompatibilityEnabled
	default:
		return false
	}
}

func shouldTryDynamicWiFiCompatibility(cfg types.AppConfig) bool {
	if !cfg.WiFiCompatibilityEnabled {
		return false
	}
	types.NormalizeDeviceProfileConfig(&cfg)
	return types.NormalizeDeviceTransport(types.ActiveDeviceProfile(&cfg).Transport) == types.DeviceTransportWiFi
}

func (a *CoreApp) maybeRecoverFromSystemResume(source string, gap, expectedInterval time.Duration) bool {
	if !shouldRecoverFromSystemResumeGap(gap, expectedInterval) {
		return false
	}
	a.triggerResumeRecovery(source, gap, false)
	return true
}

// onSystemSuspend 收到系统挂起（睡眠/休眠）通知时调用。
func (a *CoreApp) onSystemSuspend() {
	if !a.systemSuspended.CompareAndSwap(false, true) {
		return
	}
	generation := a.suspendGeneration.Add(1)
	a.deviceManager.BlockWrites()
	start := time.Now()
	a.logInfo("收到系统挂起通知：提前停止监控并断开设备/桥接，避免唤醒后失效句柄导致崩溃")

	a.mutex.RLock()
	coreConnected := a.isConnected
	a.mutex.RUnlock()
	a.resumeReconnectWanted.Store(resumeReconnectWantedOnSuspend(
		coreConnected,
		a.deviceManager.IsConnected(),
		a.reconnectInProgress.Load(),
		a.autoReconnectSuppressed.Load(),
	))
	a.cancelReconnect()
	a.autoReconnectSuppressed.Store(true)
	a.mutex.Lock()
	a.isConnected = false
	a.deviceSettings = nil
	a.mutex.Unlock()
	a.stopTemperatureMonitoring()

	done := make(chan struct{})
	a.safeGo("suspend-cleanup", func() {
		defer close(done)
		if !a.isCurrentSuspendGeneration(generation) {
			return
		}

		a.safeRun("suspend-device-disconnect", func() {
			a.deviceManager.DisconnectSilently()
		})
		if !a.isCurrentSuspendGeneration(generation) {
			return
		}

		a.safeRun("suspend-bridge-stop", func() {
			a.bridgeManager.Stop()
		})
		if !a.isCurrentSuspendGeneration(generation) {
			return
		}

		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
		}
		a.logInfo("挂起前清理完成，耗时 %s", time.Since(start).Round(time.Millisecond))
	})

	select {
	case <-done:
	case <-time.After(suspendCleanupGrace):
		a.logError("挂起前清理超过 %s 仍未完成，转入后台继续执行，避免阻塞系统电源回调", suspendCleanupGrace)
	}
}

func resumeReconnectWantedOnSuspend(coreConnected, deviceConnected, reconnectInProgress, autoReconnectSuppressed bool) bool {
	if autoReconnectSuppressed {
		return false
	}
	return coreConnected || deviceConnected || reconnectInProgress
}

func shouldReconnectAfterResume(proactivelySuspended, resumeReconnectWanted, autoReconnectSuppressed, forceReconnect bool) bool {
	if proactivelySuspended {
		return resumeReconnectWanted
	}
	return forceReconnect && !autoReconnectSuppressed
}

func (a *CoreApp) isCurrentSuspendGeneration(generation uint64) bool {
	return a.systemSuspended.Load() && a.suspendGeneration.Load() == generation && !a.stopping.Load()
}

// onSystemResume 收到系统唤醒通知时调用，触发设备与监控的恢复。
func (a *CoreApp) onSystemResume() {
	a.logInfo("收到系统唤醒通知")
	a.triggerResumeRecovery("power-event", 0, true)
}

func shouldReconnectOnHIDArrival(stopping, suspended, suppressed, coreConnected, managerConnected bool) bool {
	return !stopping && !suspended && !suppressed && !coreConnected && !managerConnected
}

func (a *CoreApp) onSupportedHIDArrival(devicePath string) {
	a.mutex.RLock()
	coreConnected := a.isConnected
	a.mutex.RUnlock()
	if !shouldReconnectOnHIDArrival(
		a.stopping.Load(),
		a.systemSuspended.Load(),
		a.autoReconnectSuppressed.Load(),
		coreConnected,
		a.deviceManager.IsConnected(),
	) {
		return
	}

	a.logInfo("检测到飞智 HID 接口已就绪，立即触发重连: %s", devicePath)
	a.requestReconnect("hid-interface-arrival", []time.Duration{0})
}

// triggerResumeRecovery 以节流方式触发唤醒恢复，避免电源事件与基于时间间隔的检测重复执行。
func (a *CoreApp) triggerResumeRecovery(source string, gap time.Duration, forceReconnect bool) {
	nowUnix := time.Now().UnixNano()
	lastUnix := atomic.LoadInt64(&a.lastResumeRecoveryUnix)
	if lastUnix > 0 && time.Duration(nowUnix-lastUnix) < systemResumeRecoveryCooldown {
		a.refreshTrayAfterResume()
		return
	}
	if !a.resumeRecoveryRunning.CompareAndSwap(false, true) {
		a.refreshTrayAfterResume()
		return
	}
	atomic.StoreInt64(&a.lastResumeRecoveryUnix, nowUnix)

	a.safeGo("systemResumeRecovery@"+source, func() {
		defer a.resumeRecoveryRunning.Store(false)
		a.handleSystemResume(source, gap, forceReconnect)
	})
}

func (a *CoreApp) handleSystemResume(source string, gap time.Duration, forceReconnect bool) {
	start := time.Now()
	defer func() {
		a.logInfo("系统唤醒恢复流程结束（来源=%s，耗时=%s）", source, time.Since(start).Round(time.Millisecond))
	}()

	// 先让仍在后台阻塞的旧休眠清理失效，避免其覆盖唤醒后的新连接。
	proactivelySuspended := a.systemSuspended.Swap(false)
	resumeReconnectWanted := a.resumeReconnectWanted.Swap(false)
	if proactivelySuspended {
		a.suspendGeneration.Add(1)
	}
	a.cancelReconnect()
	forceReconnect = shouldReconnectAfterResume(
		proactivelySuspended,
		resumeReconnectWanted,
		a.autoReconnectSuppressed.Load(),
		forceReconnect,
	)

	a.logInfo("检测到系统从睡眠/休眠恢复，来源=%s，挂起时长约=%s，主动挂起=%v，开始执行连接自愈",
		source, gap.Round(time.Second), proactivelySuspended)
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventSystemResume, map[string]any{
			"timestamp": time.Now().UnixMilli(),
			"source":    source,
		})
	}

	// 唤醒后 Explorer 可能重启或通知区域被重建，主动刷新托盘图标避免图标丢失/无响应。
	a.refreshTrayAfterResume()

	wasConnected := a.deviceManager.IsConnected()
	a.mutex.RLock()
	if !wasConnected {
		wasConnected = a.isConnected
	}
	a.mutex.RUnlock()
	if (proactivelySuspended && !resumeReconnectWanted) ||
		(!proactivelySuspended && a.autoReconnectSuppressed.Load()) {
		wasConnected = false
	}

	// 桥接停止与设备断开都涉及外部进程/cgo 调用，唤醒后句柄可能失效，统一兜底防止崩溃。
	a.safeRun("resume-bridge-stop", func() {
		a.bridgeManager.Stop()
	})

	// 主动挂起时温度监控已停止，唤醒后需重新启动（与设备连接解耦）。
	if proactivelySuspended {
		if resumeReconnectWanted {
			a.autoReconnectSuppressed.Store(false)
		}
		a.safeGo("resume-temp-monitor", func() {
			a.startTemperatureMonitoring()
		})
	}

	if !wasConnected && !forceReconnect {
		a.logInfo("系统恢复时设备原本未连接，仅重置桥接状态")
		return
	}

	a.safeRun("resume-device-disconnect", func() {
		a.deviceManager.DisconnectForRecovery()
	})
	a.mutex.Lock()
	a.isConnected = false
	a.deviceSettings = nil
	a.mutex.Unlock()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}

	a.requestReconnect("system-resume", systemResumeReconnectDelays())
}

// refreshTrayAfterResume 在系统唤醒后刷新托盘图标。
//
// 由于唤醒后 Explorer/通知区域可能尚未完全恢复，这里立即刷新一次，并在数秒后再刷新一次，
// 以提高托盘图标恢复成功率。
func (a *CoreApp) refreshTrayAfterResume() {
	if a.trayManager == nil {
		return
	}
	a.trayManager.RefreshIcon()
	a.safeGo("resume-tray-refresh-delayed", func() {
		time.Sleep(5 * time.Second)
		a.trayManager.RefreshIcon()
	})
}

// safeRun 在当前协程内执行 fn，并捕获其 panic，避免影响调用方的后续清理流程。
func (a *CoreApp) safeRun(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			a.logError("[%s] 执行时发生panic，已恢复: %v", name, r)
		}
	}()
	fn()
}

// ConnectDevice 连接设备
func (a *CoreApp) ConnectDevice() bool {
	return a.ConnectBestScannedDevice()
}

func (a *CoreApp) AutoScanDevices() map[string]any {
	a.connectionPhase.Store(deviceConnectionPhaseDiscovering)
	defer a.connectionPhase.Store(deviceConnectionPhaseNone)
	cfg := a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)
	devices := a.deviceManager.ScanNativeDevicesProfiles(cfg.DeviceProfiles)
	result := map[string]any{
		"connected": a.deviceManager.IsConnected(),
		"devices":   devices,
		"matched":   len(devices) > 0,
	}
	if len(devices) == 1 {
		result["deviceInfo"] = devices[0]
		result["profileId"] = devices[0]["profileId"]
		result["transport"] = devices[0]["transport"]
	}
	return result
}

func (a *CoreApp) ConnectNativeDevice(profileID string) bool {
	a.cancelReconnect()
	a.connectMutex.Lock()
	defer a.connectMutex.Unlock()
	a.connectionPhase.Store(deviceConnectionPhaseConnecting)
	defer a.connectionPhase.Store(deviceConnectionPhaseNone)
	return newDeviceConnectionFlow(a).connectNativeDevice(profileID)
}

func (a *CoreApp) finishSuccessfulDeviceConnection(deviceInfo map[string]string, caller string) *types.DeviceSettings {
	a.deviceManager.UnblockWrites()
	a.syncConnectedBuiltInDeviceProfile(deviceInfo)

	a.mutex.Lock()
	a.isConnected = true
	a.mutex.Unlock()
	atomic.StoreInt64(&a.lastHealthReconnectUnix, 0)
	a.hasSuccessfulConnection.Store(true)
	a.lastConnectionWasNative.Store(types.IsNativeDeviceTransport(a.deviceManager.GetDeviceType()))

	if cfg, changed, err := a.applyConnectedRuntimeCurveState(); err != nil {
		a.logError("sync runtime device curve failed: %v", err)
	} else if changed && a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	settings, settingsErr := a.RefreshDeviceSettings()
	if settingsErr != nil {
		a.logError("读取设备设置失败: %v", settingsErr)
	}
	if deviceInfo != nil && a.ipcServer != nil {
		eventPayload := map[string]any{}
		for key, value := range deviceInfo {
			eventPayload[key] = value
		}
		runtimeProfile := a.deviceManager.ActiveProfile()
		eventPayload["deviceName"] = connectedDeviceDisplayName(runtimeProfile, eventPayloadString(eventPayload, "model"), settings, "")
		eventPayload["deviceProfile"] = runtimeProfile
		eventPayload["deviceCapabilities"] = runtimeProfile.Capabilities
		eventPayload["currentData"] = a.deviceManager.GetCurrentFanData()
		if settings != nil {
			eventPayload["deviceSettings"] = settings
		}
		a.ipcServer.BroadcastEvent(ipc.EventDeviceConnected, eventPayload)
	}

	if a.deviceManager.GetDeviceType() == types.DeviceTypeHID {
		if err := a.applyConfiguredLightStrip(); err != nil {
			a.logError("应用灯带配置失败: %v", err)
		}
	}
	a.safeGo("startTemperatureMonitoring@"+caller, func() {
		a.startTemperatureMonitoring()
	})
	return settings
}

// DisconnectDevice 断开设备连接
func (a *CoreApp) DisconnectDevice() {
	a.suppressReconnect()
	a.lastConnectionWasNative.Store(false)

	a.mutex.Lock()
	a.isConnected = false
	a.deviceSettings = nil
	a.mutex.Unlock()

	a.deviceManager.DisconnectSilently()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}
}

// reapplyConfigAfterReconnect 重连后重新应用APP配置
func (a *CoreApp) reapplyConfigAfterReconnect() {
	cfg := a.configManager.Get()

	// 重新应用智能变频配置
	if cfg.AutoControl {
		a.logInfo("重新启动智能变频")
	} else if cfg.CustomSpeedEnabled {
		// 重新应用自定义转速
		unit := a.activeDeviceSpeedUnit(&cfg)
		speed := types.ClampSpeedForUnit(cfg.CustomSpeedRPM, unit)
		a.logInfo("重新应用自定义速度: %d%s", speed, types.FanSpeedDisplaySuffix(unit))
		if !a.deviceManager.SetTargetSpeed(configSpeedToTargetUnit(speed, unit), unit) {
			a.logError("重新应用自定义转速失败")
		}
	} else {
		if err := a.applyCurrentGearSetting(); err != nil {
			a.logError("重新应用当前挡位设置失败: %v", err)
		}
	}

	// 以下功能仅旧 HID 设备支持
	if a.deviceManager.GetDeviceType() == types.DeviceTypeHID {
		// 重新应用挡位灯配置
		if cfg.GearLight && a.activeDeviceCapabilities().AllowsGearLight() {
			a.logInfo("重新开启挡位灯")
			if !a.deviceManager.SetGearLight(true) {
				a.logError("重新开启挡位灯失败")
			}
		}

		if err := a.applyConfiguredLightStrip(); err != nil {
			a.logError("重连后重新应用灯带配置失败: %v", err)
		}
	}

	// 重新应用通电自启动配置（BS1 和 BS2/BS2PRO 都支持）
	if cfg.PowerOnStart && a.activeDeviceCapabilities().SupportsPowerOnStart {
		a.logInfo("重新开启通电自启动")
		if !a.deviceManager.SetPowerOnStart(true) {
			a.logError("重新开启通电自启动失败")
		}
	}
	if cfg.SmartStartStop != "" && cfg.SmartStartStop != "off" && a.activeDeviceCapabilities().SupportsSmartStartStop {
		a.logInfo("重新应用智能启停: %s", cfg.SmartStartStop)
		if !a.deviceManager.SetSmartStartStop(cfg.SmartStartStop) {
			a.logError("重新应用智能启停失败")
		}
	}
}

// GetDeviceStatus 获取设备状态
func (a *CoreApp) GetDeviceStatus() map[string]any {
	a.mutex.RLock()
	connected := a.isConnected
	manager := a.deviceManager
	settings := a.deviceSettings
	currentTemp := a.currentTemp
	monitoring := a.monitoringTemp.Load()
	a.mutex.RUnlock()
	connected = connected && manager != nil && manager.IsConnected()

	runtime := a.deviceRuntimeStatus()
	if !connected {
		return map[string]any{
			"connected":   false,
			"monitoring":  monitoring,
			"currentData": nil,
			"temperature": currentTemp,
			"runtime":     runtime,
		}
	}

	productID := manager.GetProductID()
	productIDHex := ""
	if productID != 0 {
		productIDHex = fmt.Sprintf("0x%04X", productID)
	}

	model := manager.GetModelName()
	profile := manager.ActiveProfile()

	return map[string]any{
		"connected":          true,
		"monitoring":         monitoring,
		"currentData":        manager.GetCurrentFanData(),
		"temperature":        currentTemp,
		"productId":          productIDHex,
		"model":              model,
		"deviceName":         connectedDeviceDisplayName(profile, model, settings, ""),
		"deviceProfile":      profile,
		"deviceCapabilities": profile.Capabilities,
		"deviceSettings":     settings,
		"runtime":            runtime,
	}
}

func (a *CoreApp) RefreshDeviceSettings() (*types.DeviceSettings, error) {
	settings, err := a.deviceManager.QueryDeviceSettings()
	if err != nil && !settings.Available {
		return nil, err
	}

	a.mutex.Lock()
	a.deviceSettings = &settings
	a.mutex.Unlock()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceSettingsUpdate, settings)
	}
	return &settings, err
}
