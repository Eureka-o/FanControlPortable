package coreapp

import (
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

const healthReconnectCooldown = 90 * time.Second

// 传输刷新连续失败阈值：超过此次数才触发断连，避免单次抖动误判。
const healthConsecutiveFailureThreshold = 3

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
		a.trayManager.IsInitialized(), a.isConnected)
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
	a.mutex.RLock()
	connected := a.isConnected
	a.mutex.RUnlock()

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

	healthOK := true
	switch a.deviceManager.GetDeviceType() {
	case types.DeviceTransportWiFi:
		if !a.deviceManager.RefreshWiFiState() {
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
		atomic.StoreInt32(&a.healthConsecutiveFailureCount, 0)
		return
	}

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

func (a *CoreApp) shouldRunHealthReconnect(now time.Time) bool {
	lastUnix := atomic.LoadInt64(&a.lastHealthReconnectUnix)
	if lastUnix == 0 {
		return true
	}
	return now.Sub(time.Unix(0, lastUnix)) >= healthReconnectCooldown
}
