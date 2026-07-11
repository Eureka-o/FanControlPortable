package coreapp

import (
	"context"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/smartcontrol"
	"github.com/TIANLI0/THRM/internal/temperature"
	"github.com/TIANLI0/THRM/internal/types"
)

const staleBridgeUpdateThreshold = 3

// idleTemperatureMonitorInterval 是后台空闲（无 GUI 连接且未开启智能控温）时的温度采样间隔下限。
// 此时温度读取仅用于托盘提示与历史记录，放慢采样可降低桥接传感器扫描带来的后台 CPU 占用。
const idleTemperatureMonitorInterval = 5 * time.Second

// idleMemoryReleaseCooldown 限制 GUI 断开后归还内存的最小间隔，避免频繁开关 GUI 时反复触发 GC。
const idleMemoryReleaseCooldown = 30 * time.Second

const (
	consecutiveBridgeFailureRestartThreshold = 2
	temperatureBridgeRestartCooldown         = 10 * time.Second
)

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

func (a *CoreApp) stopTemperatureMonitoring() {
	a.monitoringMutex.Lock()
	if a.monitoringDone != nil {
		a.monitoringStopping = true
		if a.monitoringCancel != nil {
			a.monitoringCancel()
		}
	}
	a.monitoringMutex.Unlock()
}

// startTemperatureMonitoring 开始温度监控
func (a *CoreApp) startTemperatureMonitoring() {
	ctx, done, started := a.beginTemperatureMonitoring()
	if !started {
		return
	}
	defer a.finishTemperatureMonitoring(done)

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
	risePredictionSamples := make([]smartcontrol.RisePredictionSample, 0, 12)
	// selectionRevision 跟踪上次构建 TemperatureSelection 时的 cfg 版本，
	// revision 未变时复用 cachedSelection，避免每 tick 重建结构体。
	cachedSelection := types.TemperatureSelection{
		TempSource:            cfg.TempSource,
		GpuDevice:             cfg.GpuDevice,
		CpuSensor:             cfg.CpuSensor,
		GpuSensor:             cfg.GpuSensor,
		CpuPowerSensor:        cfg.CpuPowerSensor,
		GpuPowerSensor:        cfg.GpuPowerSensor,
		GpuReadMode:           cfg.GpuReadMode,
		GpuLowPowerProtection: cfg.GpuLowPowerProtection,
	}
	selectionRevision := cfgRevision
	initialSelection := cachedSelection
	initialTemp := a.tempReader.Read(initialSelection)
	if initialTemp.ControlTemp > 0 {
		rawTempHistory = append(rawTempHistory, initialTemp.ControlTemp)
	}
	lastTargetRPM := -1
	learningDirty := false
	defer func() {
		if learningDirty {
			if err := a.configManager.Save(); err != nil {
				a.logError("退出监控时保存学习曲线失败: %v", err)
			}
		}
	}()
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
	speedUnit := a.activeDeviceSpeedUnit(&cfg)
	steadyObserver := newStableObserverForActiveUnit(len(cfg.FanCurve), speedUnit)
	timer := time.NewTimer(updateInterval)
	wifiOverviewInterval := wifiOverviewRefreshInterval(false, cfg.AutoControl)
	wifiOverviewTimer := time.NewTimer(wifiOverviewInterval)
	defer timer.Stop()
	defer wifiOverviewTimer.Stop()

	prevHasClients := a.ipcServer != nil && a.ipcServer.HasClients()
	var lastMemRelease time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case <-wifiOverviewTimer.C:
			now := time.Now()
			// revision 门控：与主 tick 保持一致，未变时复用已缓存的 cfg。
			// selection 重建由主 tick 的独立检查统一处理，此处只更新 cfg。
			if newRev := a.configManager.Revision(); newRev != cfgRevision {
				cfg, cfgRevision = a.configManager.GetWithRevision()
			}
			hasClientsForOverview := a.ipcServer != nil && a.ipcServer.HasClients()
			deviceType := a.deviceManager.GetDeviceType()
			wifiOverviewInterval = wifiOverviewRefreshInterval(hasClientsForOverview, cfg.AutoControl)
			if deviceType != types.DeviceTransportWiFi {
				wifiOverviewTimer.Reset(wifiOverviewBackgroundRefreshInterval)
				continue
			}
			if shouldRefreshWiFiOverviewState(deviceType, now, lastWiFiOverviewRefresh, false, wifiOverviewInterval) {
				lastWiFiOverviewRefresh = now
				a.refreshWiFiOverviewState(&wifiOverviewRefreshRunning)
			}
			wifiOverviewTimer.Reset(wifiOverviewInterval)
		case <-timer.C:
			now := time.Now()
			gap := now.Sub(lastMonitorTick)
			lastMonitorTick = now
			if a.maybeRecoverFromSystemResume("temperature-monitor", gap, updateInterval) {
				timer.Reset(updateInterval)
				continue
			}

			// revision 未变时跳过全量 AppConfig 拷贝（含 DeviceProfiles/FanCurveProfilesByDevice 等大字段），
			// 减少每 tick 产生的短命对象，降低 GC 压力。
			if newRev := a.configManager.Revision(); newRev != cfgRevision {
				cfg, cfgRevision = a.configManager.GetWithRevision()
			}
			// TemperatureSelection 与 cfg 同步，独立于 cfg 读取检查。
			// wifiOverviewTimer 分支可能已更新 cfgRevision，此处统一重建，
			// 避免因分支顺序不同导致 selection 滞后一个 tick。
			if cfgRevision != selectionRevision {
				cachedSelection = types.TemperatureSelection{
					TempSource:            cfg.TempSource,
					GpuDevice:             cfg.GpuDevice,
					CpuSensor:             cfg.CpuSensor,
					GpuSensor:             cfg.GpuSensor,
					CpuPowerSensor:        cfg.CpuPowerSensor,
					GpuPowerSensor:        cfg.GpuPowerSensor,
					GpuReadMode:           cfg.GpuReadMode,
					GpuLowPowerProtection: cfg.GpuLowPowerProtection,
				}
				selectionRevision = cfgRevision
			}

			// 后台空闲（无 GUI 连接且未开启智能控温）时放慢采样；智能控温或前台打开时保持原频率。
			hasClients := a.ipcServer != nil && a.ipcServer.HasClients()
			updateInterval = activeTemperatureMonitorInterval(cfg.TempUpdateRate, hasClients, cfg.AutoControl)
			// GUI 断开瞬间把会话期间膨胀的堆内存归还操作系统，降低核心常驻后台时的 RSS。
			if prevHasClients && !hasClients && now.Sub(lastMemRelease) > idleMemoryReleaseCooldown {
				lastMemRelease = now
				a.safeGo("release-idle-memory", func() { debug.FreeOSMemory() })
			}
			prevHasClients = hasClients

			temp := a.tempReader.Read(cachedSelection)
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
			temp = mergeTemperatureHardwareMetadata(previousTemp, temp)
			a.currentTemp = temp
			a.mutex.Unlock()

			historyPoint, recorded := a.tempHistory.Add(temp, a.deviceManager.GetCurrentFanData())

			// 广播温度更新（无 GUI 客户端时跳过差分与序列化，降低后台每 tick 开销）
			if hasClients {
				eventTemp := compactTemperatureEventPayload(temp, previousTemp)
				a.ipcServer.BroadcastEvent(ipc.EventTemperatureUpdate, eventTemp)
				if recorded {
					a.ipcServer.BroadcastEvent(ipc.EventTemperatureHistoryUpdate, historyPoint)
				}
			}

			if cfgRevision != smartCfgRevision {
				smartChanged := false
				speedUnit = a.activeDeviceSpeedUnit(&cfg)
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
				speedUnit = a.activeDeviceSpeedUnit(&cfg)
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

				gpuPowerForPrediction := temp.GPUPowerWatts
				if temp.GPUReadState == types.GPUReadStateNotPolled {
					gpuPowerForPrediction = 0
				}
				risePredictionSamples = append(risePredictionSamples, smartcontrol.RisePredictionSample{
					ControlTemp:   controlTemp,
					CPUPowerWatts: temp.CPUPowerWatts,
					GPUPowerWatts: gpuPowerForPrediction,
				})
				if len(risePredictionSamples) > 12 {
					risePredictionSamples = risePredictionSamples[len(risePredictionSamples)-12:]
				}
				risePredictionBoosted := false
				if targetRPM > 0 && !spikeSuppressed {
					predictedTarget, boost := smartcontrol.ApplyTemperatureRisePrediction(targetRPM, risePredictionSamples, smartCfg, speedUnit)
					if boost > 0 {
						targetRPM = min(max(predictedTarget, curveMinRPM), curveMaxRPM)
						risePredictionBoosted = true
					}
				}

				if prevTargetRPM >= 0 {
					targetRPM = smartcontrol.ApplyRampLimit(targetRPM, prevTargetRPM, smartCfg.RampUpLimit, smartCfg.RampDownLimit)
					if targetRPM > 0 {
						targetRPM = min(max(targetRPM, curveMinRPM), curveMaxRPM)
					}
				}

				fanData := a.deviceManager.GetCurrentFanData()
				targetLimited := false
				requestedTargetRPM := targetRPM
				targetRPM, targetLimited = applyFlyDigiRuntimeCapabilityToTarget(targetRPM, fanData, speedUnit)
				if targetLimited {
					a.logInfo("智能控温目标受飞智当前供电/挡位上限限制: %dRPM -> %dRPM", requestedTargetRPM, targetRPM)
				}
				observedRPM := targetRPM
				if fanData != nil && fanData.CurrentRPM > 0 {
					observedRPM = fanDataSpeedForControlUnit(int(fanData.CurrentRPM), speedUnit)
				}
				forceSend := false
				if targetRPM > 0 {
					forceSend = a.forceNextAutoTarget.Swap(false)
				}
				if targetRPM > 0 && (forceSend || shouldSendTargetSpeed(targetRPM, prevTargetRPM, smartCfg.MinRPMChange, fanData, speedUnit)) {
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

				if smartCfg.Learning && !spikeSuppressed && !risePredictionBoosted && !targetLimited {
					powerWatts, havePower := totalTemperaturePowerWatts(temp)
					steady := steadyObserver.ObserveWithPower(controlTemp, observedRPM, powerWatts, havePower, controlCurve, smartCfg)
					if steady.Ready && steady.BucketIdx >= 0 {
						newOffsets, changed := learnSteadyOffsetForActiveUnit(steady.BucketIdx, steady.MeanTemp, steady.MeanPower, steady.HavePower, steady.LocalEff, steady.HaveEff, cfg.FanCurve, smartCfg.LearnedOffsets, smartCfg, speedUnit)
						if changed {
							smartCfg.LearnedOffsets = newOffsets
							cfg.SmartControl = smartCfg
							storeSmartControlOffsetsForDeviceKey(&cfg, a.activeDeviceCurveScopeKey(cfg))
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
				} else if !smartCfg.Learning || risePredictionBoosted || targetLimited {
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

}

func (a *CoreApp) beginTemperatureMonitoring() (context.Context, chan struct{}, bool) {
	for {
		a.monitoringMutex.Lock()
		if a.monitoringDone == nil {
			parent := a.ctx
			if parent == nil {
				parent = context.Background()
			}
			ctx, cancel := context.WithCancel(parent)
			done := make(chan struct{})
			a.monitoringCancel = cancel
			a.monitoringDone = done
			a.monitoringStopping = false
			a.monitoringTemp.Store(true)
			a.monitoringMutex.Unlock()
			return ctx, done, true
		}
		if !a.monitoringStopping {
			a.monitoringMutex.Unlock()
			return nil, nil, false
		}
		previousDone := a.monitoringDone
		a.monitoringMutex.Unlock()
		<-previousDone
	}
}

func (a *CoreApp) finishTemperatureMonitoring(done chan struct{}) {
	a.monitoringMutex.Lock()
	if a.monitoringDone == done {
		a.monitoringCancel = nil
		a.monitoringDone = nil
		a.monitoringStopping = false
		a.monitoringTemp.Store(false)
		close(done)
	}
	a.monitoringMutex.Unlock()
}

func temperatureMonitorInterval(updateRateSeconds int) time.Duration {
	if updateRateSeconds < 1 {
		updateRateSeconds = 1
	}
	return time.Duration(updateRateSeconds) * time.Second
}

func activeTemperatureMonitorInterval(updateRateSeconds int, hasClients, autoControl bool) time.Duration {
	interval := temperatureMonitorInterval(updateRateSeconds)
	if !hasClients && !autoControl && idleTemperatureMonitorInterval > interval {
		return idleTemperatureMonitorInterval
	}
	return interval
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

func totalTemperaturePowerWatts(temp types.TemperatureData) (float64, bool) {
	total := 0.0
	if temp.CPUPowerWatts > 0 {
		total += temp.CPUPowerWatts
	}
	if temp.GPUReadState != types.GPUReadStateNotPolled && temp.GPUPowerWatts > 0 {
		total += temp.GPUPowerWatts
	}
	return total, total > 0
}

func newStableObserverForActiveUnit(curveLen int, unit string) *smartcontrol.StableObserver {
	if types.IsPercentSpeedUnit(unit) {
		return smartcontrol.NewPercentStableObserver(curveLen)
	}
	return smartcontrol.NewLegacyRPMStableObserver(curveLen)
}

func learnSteadyOffsetForActiveUnit(bucketIdx int, meanTemp int, meanPower float64, havePower bool, localEff float64, haveEff bool, curve []types.FanCurvePoint, prevOffsets []int, cfg types.SmartControlConfig, unit string) ([]int, bool) {
	if types.IsPercentSpeedUnit(unit) {
		return smartcontrol.LearnPercentSteadyOffsetTicksWithPower(bucketIdx, meanTemp, meanPower, havePower, localEff, haveEff, curve, prevOffsets, cfg)
	}
	return smartcontrol.LearnLegacyRPMSteadyOffsetWithPower(bucketIdx, meanTemp, meanPower, havePower, localEff, haveEff, curve, prevOffsets, cfg)
}

func applyFlyDigiRuntimeCapabilityToTarget(targetRPM int, fanData *types.FanData, unit string) (int, bool) {
	if targetRPM <= 0 || !types.IsRPMSpeedUnit(unit) || fanData == nil || fanData.Transport != types.DeviceTransportHID {
		return targetRPM, false
	}
	capability := types.DecodeFlyDigiRuntimeCapability(fanData, nil)
	if fanData.FlyDigiCapability != nil {
		capability = *fanData.FlyDigiCapability
	}
	return types.FlyDigiClampRPMForCapability(targetRPM, capability)
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
	deviceCurrentRPM := fanDataSpeedForControlUnit(int(fanData.CurrentRPM), unit)
	if deviceCurrentRPM == 0 {
		return true
	}
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
