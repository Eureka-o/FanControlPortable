package coreapp

import (
	"sync/atomic"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

const (
	healthReconnectCooldown    = 90 * time.Second
	healthReconnectCooldownMax = 5 * time.Minute
)

// 传输刷新连续失败阈值：超过此次数才触发断连，避免单次抖动误判。
const healthConsecutiveFailureThreshold = 3

const (
	healthTransportBackoffBase = 30 * time.Second
	healthTransportBackoffMax  = 5 * time.Minute
)

type healthTransportState struct {
	lastSuccessfulAt    time.Time
	lastFailureAt       time.Time
	consecutiveFailures int
	backoffLevel        int
	backoffUntil        time.Time
	lastError           string
}

// startHealthMonitoring 启动健康监控
func (a *CoreApp) startHealthMonitoring() {
	a.logInfo("启动健康监控系统")

	a.healthCheckTicker = time.NewTicker(30 * time.Second)

	a.safeGo("healthMonitoringLoop", func() {
		defer a.healthCheckTicker.Stop()
		lastHealthCheck := time.Now()

		for {
			select {
			case <-a.healthCheckTicker.C:
				now := time.Now()
				gap := now.Sub(lastHealthCheck)
				lastHealthCheck = now
				if a.maybeRecoverFromSystemResume("health-monitor", gap, 30*time.Second) {
					continue
				}

				a.emitHeartbeat()
				a.performHealthCheck()
			case <-a.cleanupChan:
				a.logInfo("健康监控系统已停止")
				return
			}
		}
	})

	if a.logger != nil {
		a.safeGo("cleanOldLogs", func() {
			a.logger.CleanOldLogs()
		})
	}
}

func (a *CoreApp) emitHeartbeat() {
	if a.ipcServer != nil && a.ipcServer.HasClients() {
		a.ipcServer.BroadcastEvent(ipc.EventHeartbeat, time.Now().UnixMilli())
	}
}

// performHealthCheck 执行健康检查
func (a *CoreApp) performHealthCheck() {
	defer func() {
		if r := recover(); r != nil {
			a.logError("健康检查中发生panic: %v", r)
		}
	}()

	a.trayManager.CheckHealth()
	a.ensureTemperatureMonitoringHealthy()
	a.checkDeviceHealth()

	a.logDebug("健康检查完成 - 托盘:%v 设备连接:%v",
		a.trayManager.IsInitialized(), a.deviceRuntimeSnapshot().Connected)
}

func (a *CoreApp) ensureTemperatureMonitoringHealthy() {
	if a.systemSuspended.Load() || a.monitoringTemp.Load() {
		return
	}

	a.logError("健康检查: 温度监控未运行，尝试重新启动")
	a.safeGo("restartTemperatureMonitoring@health-check", func() {
		a.startTemperatureMonitoring()
	})
}

// checkDeviceHealth 检查设备健康状态
func (a *CoreApp) checkDeviceHealth() {
	if a.monitorOnlyActive() || shouldSkipDeviceHealthCheck(
		a.systemSuspended.Load(),
		a.resumeRecoveryRunning.Load(),
		a.reconnectInProgress.Load(),
	) {
		return
	}
	// 设备独占任务（屏幕图片传输）进行中：这段时间读不到设备是**本机自己**造成的，
	// 不能据此判定设备掉了 —— 跳过本轮，等任务结束（登记表是这件事的唯一所有者）。
	if a.blackSharkImageTransferActive() {
		a.logDebug("健康检查: 屏幕图片传输进行中，跳过本轮设备健康检查")
		return
	}
	snapshot := a.deviceRuntimeSnapshot()
	connected := snapshot.Connected

	if !connected {
		now := time.Now()
		if !a.shouldRunHealthReconnect(now) {
			a.logDebug("健康检查: 设备未连接，但重连仍在冷却期内")
			return
		}
		atomic.StoreInt64(&a.lastHealthReconnectUnix, now.UnixNano())
		a.logInfo("健康检查: 设备未连接，尝试重新连接")
		a.requestReconnect("health-check", []time.Duration{0})
		return
	}
	transport := a.deviceManager.GetDeviceType()
	if a.shouldBackoffHealthTransport(transport, time.Now()) {
		a.logDebug("健康检查: %s 传输仍在失败退避期内", transport)
		return
	}

	healthOK := true
	switch a.deviceManager.GetDeviceType() {
	case types.DeviceTransportWiFi:
		if shouldRefreshWiFiHealthState(time.Now(), snapshot.LastSuccessfulReadAt) && !a.deviceManager.RefreshWiFiState() {
			a.logDebug("健康检查: WiFi 控制器状态刷新失败")
			healthOK = false
		}
	case types.DeviceTransportBLE:
		if !a.deviceManager.RefreshBLEState() {
			a.logDebug("健康检查: BLE 状态刷新失败")
			healthOK = false
		}
	case types.DeviceTransportSerial:
		if !a.deviceManager.RefreshSerialState() {
			a.logDebug("健康检查: 串口控制器状态刷新失败")
			healthOK = false
		}
	}

	if healthOK {
		a.healthReconnectFailures.Store(0)
		a.recordHealthTransportSuccess(transport, time.Now())
		atomic.StoreInt32(&a.healthConsecutiveFailureCount, 0)
		a.maybeTakeOverBlackSharkUSB()
		return
	}
	a.recordHealthTransportFailure(transport, time.Now(), "状态刷新失败")

	// 未通过：累计失败次数，达到阈值才断连。
	count := atomic.AddInt32(&a.healthConsecutiveFailureCount, 1)
	if count < healthConsecutiveFailureThreshold {
		a.logDebug("健康检查: 设备状态不稳定（已连续 %d/%d 次失败），等待下次确认", count, healthConsecutiveFailureThreshold)
		return
	}

	// 达到阈值：重置计数并触发断连。
	atomic.StoreInt32(&a.healthConsecutiveFailureCount, 0)
	a.logError("健康检查: 设备连续 %d 次状态刷新失败，触发断开回调", healthConsecutiveFailureThreshold)
	a.deviceManager.DisconnectForRecovery()
	a.onDeviceDisconnect()
}

func shouldSkipDeviceHealthCheck(systemSuspended, resumeRecoveryRunning, reconnectInProgress bool) bool {
	return systemSuspended || resumeRecoveryRunning || reconnectInProgress
}

// blackSharkUSBTakeoverProbeInterval 是"USB 接管"轮询兜底的最小间隔。
// 健康检查本身 30 s 一拍，这里再留一档冷却，避免 USB 在但连不上时反复触发重连。
const blackSharkUSBTakeoverProbeInterval = 2 * time.Minute

// blackSharkShouldTakeOverUSB 报告"此刻该不该为了 USB 切换传输"。
//
// 只有"已连接且正走 BLE"才需要接管：没连上时该走的是启动/重连路径，而那条路本来就 USB 优先，
// 这里再插一脚只会多一次重连。
func (a *CoreApp) blackSharkShouldTakeOverUSB() bool {
	if a == nil || a.deviceManager == nil {
		return false
	}
	if a.stopping.Load() || a.systemSuspended.Load() || a.reconnectInProgress.Load() {
		return false
	}
	if !a.deviceManager.IsConnected() {
		return false
	}
	return a.deviceManager.ActiveProfile().ID == types.BlackSharkBRB02ProfileID
}

// maybeTakeOverBlackSharkUSB 是 USB 到达事件的**兜底**：通知可能没送到（或被系统丢弃），
// 且"设备一直插着、而我们在注册通知之前就连上了 BLE"只有它能覆盖。
// 主路径是 onBlackSharkUSBArrival（事件级，立即切）。
func (a *CoreApp) maybeTakeOverBlackSharkUSB() {
	if !a.blackSharkShouldTakeOverUSB() {
		return
	}
	now := time.Now()
	last := atomic.LoadInt64(&a.lastBlackSharkUSBProbeUnix)
	if last != 0 && now.Sub(time.Unix(0, last)) < blackSharkUSBTakeoverProbeInterval {
		return
	}
	atomic.StoreInt64(&a.lastBlackSharkUSBProbeUnix, now.UnixNano())
	if !a.deviceManager.BlackSharkUSBInterfacePresent() {
		return
	}
	a.logInfo("黑鲨：检测到 USB 接口，重连以切到 USB（当前走 BLE）")
	a.requestReconnect("blackshark-usb-takeover", []time.Duration{0})
}

func healthReconnectBackoff(failures int32) time.Duration {
	switch {
	case failures <= 0:
		return healthReconnectCooldown
	case failures == 1:
		return 3 * time.Minute
	default:
		return healthReconnectCooldownMax
	}
}

func (a *CoreApp) shouldRunHealthReconnect(now time.Time) bool {
	lastUnix := atomic.LoadInt64(&a.lastHealthReconnectUnix)
	if lastUnix == 0 {
		return true
	}
	return now.Sub(time.Unix(0, lastUnix)) >= healthReconnectBackoff(a.healthReconnectFailures.Load())
}

func (a *CoreApp) shouldBackoffHealthTransport(transport string, now time.Time) bool {
	a.healthTransportMu.Lock()
	defer a.healthTransportMu.Unlock()
	state, ok := a.healthTransportStates[transport]
	return ok && now.Before(state.backoffUntil)
}

func (a *CoreApp) recordHealthTransportFailure(transport string, now time.Time, reason string) {
	a.healthTransportMu.Lock()
	defer a.healthTransportMu.Unlock()
	if a.healthTransportStates == nil {
		a.healthTransportStates = make(map[string]healthTransportState)
	}
	state := a.healthTransportStates[transport]
	state.consecutiveFailures++
	state.backoffLevel = state.consecutiveFailures
	state.lastFailureAt = now
	state.lastError = reason
	backoff := healthTransportBackoffBase
	for level := 1; level < state.backoffLevel && backoff < healthTransportBackoffMax; level++ {
		backoff *= 2
	}
	if backoff > healthTransportBackoffMax {
		backoff = healthTransportBackoffMax
	}
	state.backoffUntil = now.Add(backoff)
	a.healthTransportStates[transport] = state
}

func (a *CoreApp) recordHealthTransportSuccess(transport string, now time.Time) {
	a.healthTransportMu.Lock()
	defer a.healthTransportMu.Unlock()
	if a.healthTransportStates == nil {
		a.healthTransportStates = make(map[string]healthTransportState)
	}
	state := a.healthTransportStates[transport]
	state.lastSuccessfulAt = now
	state.consecutiveFailures = 0
	state.backoffLevel = 0
	state.backoffUntil = time.Time{}
	state.lastError = ""
	a.healthTransportStates[transport] = state
}
