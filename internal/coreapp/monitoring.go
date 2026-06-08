package coreapp

import (
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/smartcontrol"
	"github.com/TIANLI0/THRM/internal/temperature"
	"github.com/TIANLI0/THRM/internal/types"
)

const staleBridgeUpdateThreshold = 3

const (
	consecutiveBridgeFailureRestartThreshold = 2
	temperatureBridgeRestartCooldown         = 10 * time.Second
	wifiOverviewStateRefreshInterval         = 2 * time.Second
)

func trackBridgeTemperatureStaleness(temp types.TemperatureData, lastUpdate int64, staleCount int) (int64, int, bool) {
	if !temp.BridgeOk || temp.UpdateTime <= 0 {
		return 0, 0, false
	}
	if temp.UpdateTime != lastUpdate {
		return temp.UpdateTime, 0, false
	}
	staleCount++
	return lastUpdate, staleCount, staleCount >= staleBridgeUpdateThreshold
}

func shouldRestartTemperatureBridge(temp types.TemperatureData) bool {
	if temp.BridgeOk {
		return false
	}

	msg := strings.ToLower(strings.TrimSpace(temp.BridgeMsg))
	if msg == "" {
		return true
	}

	restartHints := []string{
		"启动桥接程序失败",
		"桥接程序通信失败",
		"桥接程序未连接",
		"连接管道失败",
		"发送命令失败",
		"读取响应失败",
		"等待桥接程序启动超时",
		"未能获取管道名称",
		"pipe",
		"broken",
		"closed",
	}
	for _, hint := range restartHints {
		if strings.Contains(msg, strings.ToLower(hint)) {
			return true
		}
	}

	// 休眠恢复后硬件监控库偶尔会返回全 0 但进程仍能响应，重启桥接可重新初始化底层传感器。
	return temp.CPUTemp == 0 && temp.GPUTemp == 0
}

func (a *CoreApp) recoverTemperatureBridge(reason string) {
	a.safeRun("temperature-bridge-recover@"+reason, func() {
		a.bridgeManager.Stop()
		if err := a.bridgeManager.EnsureRunning(); err != nil {
			a.logError("温度桥接自愈重启失败[%s]: %v", reason, err)
			return
		}
		a.logInfo("温度桥接已完成自愈重启: %s", reason)
	})
}

func compactTemperatureEventPayload(current, previous types.TemperatureData) types.TemperatureData {
	compact := current
	if reflect.DeepEqual(current.CpuSensors, previous.CpuSensors) {
		compact.CpuSensors = nil
	}
	if reflect.DeepEqual(current.GpuSensors, previous.GpuSensors) {
		compact.GpuSensors = nil
	}
	if reflect.DeepEqual(current.GpuDevices, previous.GpuDevices) {
		compact.GpuDevices = nil
	}
	return compact
}

func (a *CoreApp) stopTemperatureMonitoring() {
	if !a.monitoringTemp.Load() {
		return
	}

	select {
	case a.stopMonitoring <- true:
	default:
	}
}

// startTemperatureMonitoring 开始温度监控
func (a *CoreApp) startTemperatureMonitoring() {
	// CAS：原子地从 false 翻到 true，确保 Start/ConnectDevice 并发调用时只有一条循环启动。
	if !a.monitoringTemp.CompareAndSwap(false, true) {
		return
	}

	// 清理可能残留的停止信号，避免新监控循环被立即中断。
	select {
	case <-a.stopMonitoring:
	default:
	}

	// 注意：不在此处立即调用 EnterAutoMode，因为在启动时温度数据（桥接程序）可能尚未就绪。
	// 如果在温度读取成功之前切换到软件控制模式，设备将不会收到转速指令，导致风扇停转。
	// EnterAutoMode 和转速设置会在首次成功读取温度后，由 SetFanSpeed 内部统一完成。

	cfg, cfgRevision := a.configManager.GetWithRevision()
	updateInterval := temperatureMonitorInterval(cfg.TempUpdateRate)

	// 温度采样使用 EMA 平滑。
	sampleCount := max(cfg.TempSampleCount, 1)
	tempEMA := 0
	tempEMAReady := false

	rawTempHistory := make([]int, 0, 6)
	recentAvgTemps := make([]int, 0, 24)
	recentControlTemps := make([]int, 0, 24)
	initialSelection := types.TemperatureSelection{
		TempSource: cfg.TempSource,
		GpuDevice:  cfg.GpuDevice,
		CpuSensor:  cfg.CpuSensor,
		GpuSensor:  cfg.GpuSensor,
	}
	initialTemp := a.tempReader.Read(initialSelection)
	if initialTemp.ControlTemp > 0 {
		rawTempHistory = append(rawTempHistory, initialTemp.ControlTemp)
	}
	lastTargetRPM := -1
	learningDirty := false
	lastLearningSave := time.Now()
	lastMonitorTick := time.Now()
	lastBridgeUpdateTime := initialTemp.UpdateTime
	staleBridgeUpdateCount := 0
	bridgeFailureCount := 0
	lastBridgeRestart := time.Time{}
	lastWiFiOverviewRefresh := time.Time{}
	var wifiOverviewRefreshRunning atomic.Bool
	var smartCfg types.SmartControlConfig
	smartCfgRevision := cfgRevision - 1

	// 每个曲线点对应一个稳态采样桶。
	speedUnit := types.DeviceProfileSpeedUnit(&cfg)
	steadyObserver := newStableObserverForActiveUnit(len(cfg.FanCurve), speedUnit)
	timer := time.NewTimer(updateInterval)
	wifiOverviewTimer := time.NewTimer(wifiOverviewStateRefreshInterval)
	defer timer.Stop()
	defer wifiOverviewTimer.Stop()

	for a.monitoringTemp.Load() {
		select {
		case <-a.stopMonitoring:
			a.monitoringTemp.Store(false)
			return
		case <-wifiOverviewTimer.C:
			now := time.Now()
			if shouldRefreshWiFiOverviewState(a.deviceManager.GetDeviceType(), now, lastWiFiOverviewRefresh, false) {
				lastWiFiOverviewRefresh = now
				a.refreshWiFiOverviewState(&wifiOverviewRefreshRunning)
			}
			wifiOverviewTimer.Reset(wifiOverviewStateRefreshInterval)
		case <-timer.C:
			now := time.Now()
			gap := now.Sub(lastMonitorTick)
			lastMonitorTick = now
			if a.maybeRecoverFromSystemResume("temperature-monitor", gap, updateInterval) {
				timer.Reset(updateInterval)
				continue
			}

			cfg, cfgRevision = a.configManager.GetWithRevision()
			updateInterval = temperatureMonitorInterval(cfg.TempUpdateRate)
			selection := types.TemperatureSelection{
				TempSource: cfg.TempSource,
				GpuDevice:  cfg.GpuDevice,
				CpuSensor:  cfg.CpuSensor,
				GpuSensor:  cfg.GpuSensor,
			}
			temp := a.tempReader.Read(selection)
			if temp.BridgeOk {
				bridgeFailureCount = 0
				staleBridge := false
				lastBridgeUpdateTime, staleBridgeUpdateCount, staleBridge = trackBridgeTemperatureStaleness(temp, lastBridgeUpdateTime, staleBridgeUpdateCount)
				if staleBridge && time.Since(lastBridgeRestart) >= temperatureBridgeRestartCooldown {
					a.logError("温度桥接返回的 updateTime 连续 %d 次未变化，触发桥接重连自愈", staleBridgeUpdateCount+1)
					a.recoverTemperatureBridge("stale-update")
					lastBridgeRestart = time.Now()
					lastBridgeUpdateTime = 0
					staleBridgeUpdateCount = 0
				}
			} else {
				lastBridgeUpdateTime = 0
				staleBridgeUpdateCount = 0
				if shouldRestartTemperatureBridge(temp) {
					bridgeFailureCount++
					if bridgeFailureCount >= consecutiveBridgeFailureRestartThreshold && time.Since(lastBridgeRestart) >= temperatureBridgeRestartCooldown {
						a.logError("温度桥接连续 %d 次读取失败，触发桥接重连自愈: %s", bridgeFailureCount, temp.BridgeMsg)
						a.recoverTemperatureBridge("read-failure")
						lastBridgeRestart = time.Now()
						bridgeFailureCount = 0
					}
				} else {
					bridgeFailureCount = 0
				}
			}

			a.mutex.Lock()
			previousTemp := a.currentTemp
			a.currentTemp = temp
			a.mutex.Unlock()

			eventTemp := compactTemperatureEventPayload(temp, previousTemp)

			historyPoint, recorded := a.tempHistory.Add(temp, a.deviceManager.GetCurrentFanData())

			// 广播温度更新
			if a.ipcServer != nil {
				a.ipcServer.BroadcastEvent(ipc.EventTemperatureUpdate, eventTemp)
				if recorded {
					a.ipcServer.BroadcastEvent(ipc.EventTemperatureHistoryUpdate, historyPoint)
				}
			}

			if cfgRevision != smartCfgRevision {
				smartChanged := false
				speedUnit = types.DeviceProfileSpeedUnit(&cfg)
				smartCfg, smartChanged = smartcontrol.NormalizeConfigForUnit(cfg.SmartControl, cfg.FanCurve, cfg.DebugMode, speedUnit)
				smartCfgRevision = cfgRevision
				if smartChanged {
					cfg.SmartControl = smartCfg
					a.configManager.Set(cfg)
					if err := a.configManager.Save(); err != nil {
						a.logError("保存智能控温配置失败: %v", err)
					}
				}
			}

			if cfg.AutoControl && temp.ControlTemp > 0 {
				speedUnit = types.DeviceProfileSpeedUnit(&cfg)
				controlCurve := smartcontrol.CurveForUnit(cfg.FanCurve, speedUnit)
				// 采样窗口变化时重置 EMA，避免阶跃。
				newSampleCount := max(cfg.TempSampleCount, 1)
				if newSampleCount != sampleCount {
					sampleCount = newSampleCount
					tempEMAReady = false
				}

				if steadyObserver == nil || len(controlCurve) != steadyObserver.CurveLen() {
					steadyObserver = newStableObserverForActiveUnit(len(controlCurve), speedUnit)
				} else if steadyObserver.SetUnit(speedUnit) {
					lastTargetRPM = -1
				}

				sampleTemp := temp.ControlTemp
				sampleSpikeSuppressed := false
				if smartCfg.FilterTransientSpike {
					sampleTemp, sampleSpikeSuppressed = smartcontrol.FilterTransientSample(temp.ControlTemp, rawTempHistory, smartCfg.Hysteresis)
				}
				rawTempHistory = append(rawTempHistory, temp.ControlTemp)
				if len(rawTempHistory) > 6 {
					rawTempHistory = rawTempHistory[len(rawTempHistory)-6:]
				}

				if !tempEMAReady {
					tempEMA = sampleTemp
					tempEMAReady = true
				} else {
					n := sampleCount
					tempEMA = (2*sampleTemp + (n-1)*tempEMA) / (n + 1)
				}
				avgTemp := tempEMA

				recentAvgTemps = append(recentAvgTemps, avgTemp)
				if len(recentAvgTemps) > 24 {
					recentAvgTemps = recentAvgTemps[len(recentAvgTemps)-24:]
				}

				controlTemp := avgTemp
				controlSpikeSuppressed := false
				if smartCfg.FilterTransientSpike {
					controlTemp, controlSpikeSuppressed = smartcontrol.FilterTransientSpike(avgTemp, recentAvgTemps, smartCfg.TargetTemp, smartCfg.Hysteresis)
				}
				spikeSuppressed := sampleSpikeSuppressed || controlSpikeSuppressed
				recentControlTemps = append(recentControlTemps, controlTemp)
				if len(recentControlTemps) > 24 {
					recentControlTemps = recentControlTemps[len(recentControlTemps)-24:]
				}

				curveMinRPM, curveMaxRPM := smartcontrol.GetCurveRPMBounds(controlCurve)

				baseRPM := temperature.CalculateTargetRPM(controlTemp, controlCurve)
				prevTargetRPM := lastTargetRPM

				targetRPM := 0
				if types.IsPercentSpeedUnit(speedUnit) {
					targetRPM = smartcontrol.CalculatePercentTargetTicks(controlTemp, cfg.FanCurve, smartCfg)
				} else {
					targetRPM = smartcontrol.CalculateLegacyRPMTarget(controlTemp, cfg.FanCurve, smartCfg)
				}
				if targetRPM <= 0 {
					targetRPM = baseRPM
				}

				if targetRPM > 0 {
					targetRPM = min(max(targetRPM, curveMinRPM), curveMaxRPM)
				}

				if prevTargetRPM >= 0 {
					targetRPM = smartcontrol.ApplyRampLimit(targetRPM, prevTargetRPM, smartCfg.RampUpLimit, smartCfg.RampDownLimit)
					if targetRPM > 0 {
						targetRPM = min(max(targetRPM, curveMinRPM), curveMaxRPM)
					}
				}

				fanData := a.deviceManager.GetCurrentFanData()
				observedRPM := targetRPM
				if fanData != nil && fanData.CurrentRPM > 0 {
					observedRPM = fanDataSpeedForControlUnit(int(fanData.CurrentRPM), speedUnit)
				}
				if shouldSendTargetSpeed(targetRPM, prevTargetRPM, smartCfg.MinRPMChange, fanData, speedUnit) {
					if a.deviceManager.SetTargetSpeed(targetRPM, speedUnit) {
						lastTargetRPM = targetRPM
						if a.deviceManager.GetDeviceType() == types.DeviceTransportWiFi {
							lastWiFiOverviewRefresh = now
						}
					} else {
						lastTargetRPM = -1
						a.logError("智能控温速度下发失败，将在下个周期重试: %d%s", displaySpeedForLog(targetRPM, speedUnit), types.FanSpeedDisplaySuffix(speedUnit))
					}
				}

				if smartCfg.Learning && !spikeSuppressed {
					steady := steadyObserver.Observe(controlTemp, observedRPM, controlCurve, smartCfg)
					if steady.Ready && steady.BucketIdx >= 0 {
						newOffsets, changed := learnSteadyOffsetForActiveUnit(steady.BucketIdx, steady.MeanTemp, steady.LocalEff, steady.HaveEff, cfg.FanCurve, smartCfg.LearnedOffsets, smartCfg, speedUnit)
						if changed {
							smartCfg.LearnedOffsets = newOffsets
							cfg.SmartControl = smartCfg
							a.configManager.Set(cfg)
							learningDirty = true
						}
					}

					if learningDirty && time.Since(lastLearningSave) >= 25*time.Second {
						if err := a.configManager.Save(); err != nil {
							a.logError("保存学习偏移失败: %v", err)
						} else {
							lastLearningSave = time.Now()
							learningDirty = false
							if a.ipcServer != nil {
								a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
							}
						}
					}
				} else if !smartCfg.Learning {
					steadyObserver.Reset()
				}

				if baseRPM > 0 {
					a.logDebug("智能控温: 最高=%d°C 基准=%s 当前=%d°C 平均=%d°C 控制温度=%d°C 基础=%d%s 目标=%d%s", temp.MaxTemp, temp.ControlSource, temp.ControlTemp, avgTemp, controlTemp, displaySpeedForLog(baseRPM, speedUnit), types.FanSpeedDisplaySuffix(speedUnit), displaySpeedForLog(targetRPM, speedUnit), types.FanSpeedDisplaySuffix(speedUnit))
				}
			}

			if !cfg.AutoControl {
				lastTargetRPM = -1
			}

			timer.Reset(updateInterval)
		}
	}

	if learningDirty {
		if err := a.configManager.Save(); err != nil {
			a.logError("退出监控时保存学习曲线失败: %v", err)
		}
	}
}

func temperatureMonitorInterval(updateRateSeconds int) time.Duration {
	if updateRateSeconds < 1 {
		updateRateSeconds = 1
	}
	return time.Duration(updateRateSeconds) * time.Second
}

func shouldRefreshWiFiOverviewState(deviceType string, now, lastRefresh time.Time, readThisTick bool) bool {
	if deviceType != types.DeviceTransportWiFi || readThisTick {
		return false
	}
	return lastRefresh.IsZero() || now.Sub(lastRefresh) >= wifiOverviewStateRefreshInterval
}

func (a *CoreApp) refreshWiFiOverviewState(refreshRunning *atomic.Bool) {
	if !refreshRunning.CompareAndSwap(false, true) {
		return
	}

	a.safeGo("refreshWiFiOverviewState", func() {
		defer refreshRunning.Store(false)
		if !a.deviceManager.RefreshWiFiState() {
			a.logDebug("实时概览刷新 WiFi 控制器状态失败，等待健康检查处理连接状态")
		}
	})
}

func fanDataSpeedForControlUnit(speed int, unit string) int {
	if types.IsPercentSpeedUnit(unit) {
		return types.PercentToTicks(speed)
	}
	return speed
}

func displaySpeedForLog(speed int, unit string) int {
	if types.IsPercentSpeedUnit(unit) {
		return types.PercentTicksToIntegerPercent(speed)
	}
	return speed
}

func newStableObserverForActiveUnit(curveLen int, unit string) *smartcontrol.StableObserver {
	if types.IsPercentSpeedUnit(unit) {
		return smartcontrol.NewPercentStableObserver(curveLen)
	}
	return smartcontrol.NewLegacyRPMStableObserver(curveLen)
}

func learnSteadyOffsetForActiveUnit(bucketIdx int, meanTemp int, localEff float64, haveEff bool, curve []types.FanCurvePoint, prevOffsets []int, cfg types.SmartControlConfig, unit string) ([]int, bool) {
	if types.IsPercentSpeedUnit(unit) {
		return smartcontrol.LearnPercentSteadyOffsetTicks(bucketIdx, meanTemp, localEff, haveEff, curve, prevOffsets, cfg)
	}
	return smartcontrol.LearnLegacyRPMSteadyOffset(bucketIdx, meanTemp, localEff, haveEff, curve, prevOffsets, cfg)
}

func shouldSendTargetSpeed(targetRPM, prevTargetRPM, minRPMChange int, fanData *types.FanData, unit string) bool {
	if targetRPM <= 0 {
		return false
	}
	if prevTargetRPM < 0 {
		return true
	}
	if absRPMDelta(targetRPM, prevTargetRPM) >= minRPMChange {
		return true
	}
	if fanData == nil {
		return false
	}
	deviceTargetRPM := fanDataSpeedForControlUnit(int(fanData.TargetRPM), unit)
	return deviceTargetRPM == 0 || absRPMDelta(targetRPM, deviceTargetRPM) >= minRPMChange
}

func shouldSendTargetRPM(targetRPM, prevTargetRPM, minRPMChange int, fanData *types.FanData) bool {
	return shouldSendTargetSpeed(targetRPM, prevTargetRPM, minRPMChange, fanData, types.FanSpeedUnitRPM)
}

func absRPMDelta(a, b int) int {
	delta := a - b
	if delta < 0 {
		return -delta
	}
	return delta
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
		a.logInfo("健康检查: 设备未连接，尝试重新连接")
		a.requestReconnect("health-check", []time.Duration{0})
	} else {
		switch a.deviceManager.GetDeviceType() {
		case types.DeviceTransportWiFi:
			if !a.deviceManager.RefreshWiFiState() {
				a.logError("健康检查: WiFi 控制器状态刷新失败，触发断开回调")
				a.onDeviceDisconnect()
				return
			}
		case types.DeviceTransportBLE:
			if !a.deviceManager.RefreshBLEState() {
				a.logError("BLE controller state refresh failed during health check")
				a.onDeviceDisconnect()
				return
			}
		case types.DeviceTransportSerial:
			if !a.deviceManager.RefreshSerialState() {
				a.logError("健康检查: 串口控制器状态刷新失败，触发断开回调")
				a.onDeviceDisconnect()
				return
			}
		}
		// 验证设备实际连接状态
		if !a.deviceManager.IsConnected() {
			a.logError("健康检查: 检测到设备状态不一致，触发断开回调")
			a.onDeviceDisconnect()
		}
	}
}
