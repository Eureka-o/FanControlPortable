package coreapp

import (
	"context"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/bridge"
	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/smartcontrol"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

const temperatureSafetyFallbackThreshold = 3

type temperatureSafetyFallback struct {
	invalidSamples int
	active         bool
}

func (s *temperatureSafetyFallback) observe(autoControl, inputReady bool) (apply, recovered bool) {
	if !autoControl {
		*s = temperatureSafetyFallback{}
		return false, false
	}
	if inputReady {
		recovered = s.active
		*s = temperatureSafetyFallback{}
		return false, recovered
	}
	s.invalidSamples++
	// Keep applying the fallback while telemetry is unavailable so the target
	// can ramp toward the safe ceiling instead of jumping there in one write.
	return s.invalidSamples >= temperatureSafetyFallbackThreshold, false
}

func (s *temperatureSafetyFallback) markApplied() {
	s.active = true
}

func temperatureSafetyFallbackTarget(curve []types.FanCurvePoint, unit string, fanData *types.FanData) int {
	if len(curve) == 0 {
		return 0
	}
	_, target := smartcontrol.GetCurveRPMBounds(smartcontrol.CurveForUnit(curve, unit))
	target, _ = applyFlyDigiRuntimeCapabilityToTarget(target, fanData, unit)
	return target
}

func rampTemperatureSafetyFallbackTarget(target, previousTarget, rampUpLimit, rampDownLimit int) int {
	if target <= 0 || previousTarget < 0 {
		return target
	}
	return smartcontrol.ApplyRampLimit(target, previousTarget, rampUpLimit, rampDownLimit)
}

func fanDataTargetForControlUnit(fanData *types.FanData, unit string) int {
	if fanData == nil || types.NormalizeFanSpeedUnit(fanData.SpeedUnit) != types.NormalizeFanSpeedUnit(unit) {
		return -1
	}
	if fanData.TargetRPM > 0 {
		return fanDataSpeedForControlUnit(int(fanData.TargetRPM), unit)
	}
	if fanData.CurrentRPM > 0 {
		return fanDataSpeedForControlUnit(int(fanData.CurrentRPM), unit)
	}
	return -1
}

const staleBridgeUpdateThreshold = 3

const smartControlLearningSaveInterval = 2 * time.Minute

// idleTemperatureMonitorInterval 是后台空闲（无 GUI 连接且未开启智能控温）时的温度采样间隔下限。
// 此时温度读取仅用于托盘提示与历史记录，放慢采样可降低桥接传感器扫描带来的后台 CPU 占用。
const idleTemperatureMonitorInterval = 5 * time.Second

const monitorOnlyTemperatureInterval = 2 * time.Second

// idleMemoryReleaseCooldown 限制 GUI 断开后归还内存的最小间隔，避免频繁开关 GUI 时反复触发 GC。
const idleMemoryReleaseCooldown = 30 * time.Second

const (
	consecutiveBridgeFailureRestartThreshold = 2
	temperatureBridgeRestartCooldown         = 10 * time.Second
)

// smartControlSampleContext identifies telemetry that may safely share
// transient prediction and learning state.
type smartControlSampleContext struct {
	selection types.TemperatureSelection
	speedUnit string
}

func newSmartControlSampleContext(selection types.TemperatureSelection, speedUnit string) smartControlSampleContext {
	return smartControlSampleContext{
		selection: types.NormalizeTemperatureSelection(selection),
		speedUnit: types.NormalizeFanSpeedUnit(speedUnit),
	}
}

func (a *CoreApp) recoverTemperatureBridge(reason string) {
	if !a.temperatureBridgeRecoveryRunning.CompareAndSwap(false, true) {
		return
	}
	a.safeGo("temperature-bridge-recover@"+reason, func() {
		defer a.temperatureBridgeRecoveryRunning.Store(false)
		a.bridgeManager.Stop()
		if err := a.bridgeManager.EnsureRunning(); err != nil {
			if bridge.IsStarting(err.Error()) {
				a.logInfo("温度桥接已开始后台自愈重启: %s", reason)
				return
			}
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

func (a *CoreApp) deviceControlReady() bool {
	return a.deviceRuntimeStatus().CanControl
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
	updateInterval := monitorOnlySamplingInterval(temperatureMonitorInterval(cfg.TempUpdateRate), a.monitorOnlyActive())

	// 0x07 主机信息推送：节拍归 device 侧的推送循环（官方挂点也在"启动监控"这里）。
	// 设 1 Hz 并立刻推一帧，免得 LCD 参数页要等到第一个 tick；同值会短路，重复启动安全。
	if a.deviceManager != nil && a.deviceManager.IsBlackSharkProfileActive() {
		if err := a.deviceManager.SetBlackSharkHostInfoRefreshInterval(
			blackSharkSysInfoInterval, a.blackSharkHostInfoPushEntries); err != nil {
			a.logError("启动 0x07 主机信息推送失败: %v", err)
		} else if entries := a.blackSharkHostInfoPushEntries(); len(entries) > 0 {
			a.deviceManager.SendBlackSharkHostInfoFrame(entries)
		}
	}

	// 温度采样使用 EMA 平滑。
	sampleCount := max(cfg.TempSampleCount, 1)
	tempEMA := 0
	tempEMAReady := false

	rawTempHistory := make([]int, 0, 6)
	recentAvgTemps := make([]int, 0, 24)
	recentControlTemps := make([]int, 0, 24)
	risePredictionSamples := make([]smartcontrol.RisePredictionSample, 0, 12)
	// lastGateReason 记录上一次的闸门判定，用于"判定变化时才记一条日志"（避免刷屏）。
	lastGateReason := ""
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
	initialTemp := a.tempReader.Read(cachedSelection)
	if initialTemp.ControlTemp > 0 {
		rawTempHistory = append(rawTempHistory, initialTemp.ControlTemp)
	}
	lastTargetRPM := -1
	var safetyFallback temperatureSafetyFallback
	learningDirty := false
	defer func() {
		if !learningDirty {
			return
		}
		_, _, applied, err := a.commitConfigMutationIfRevision(cfgRevision, func(current *types.AppConfig) {
			current.SmartControl = cfg.SmartControl
		}, nil)
		if err != nil {
			a.logError("退出监控时保存学习曲线失败: %v", err)
		} else if !applied {
			a.logDebug("退出监控时配置已变化，放弃未提交的学习曲线")
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
	steadyObserver := newStableObserverForActiveUnit(smartControlCurveLen(&cfg), speedUnit)
	lastSmartSampleContext := newSmartControlSampleContext(cachedSelection, speedUnit)
	lastAutoControl := cfg.AutoControl
	lastSmartTelemetryUsable := false
	resetSmartControlSampling := func() {
		risePredictionSamples = risePredictionSamples[:0]
		steadyObserver = newStableObserverForActiveUnit(smartControlCurveLen(&cfg), speedUnit)
		lastSmartTelemetryUsable = false
	}
	timer := time.NewTimer(updateInterval)
	wifiOverviewInterval := wifiOverviewRefreshInterval(false, cfg.AutoControl)
	wifiOverviewTimer := time.NewTimer(wifiOverviewInterval)
	if a.monitorOnlyActive() {
		wifiOverviewTimer.Stop()
	}
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
				learningDirty = false
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
			// 先排掉"本机自己持设备写锁"造成的超长停摆（屏保上传/读图全程持 m.mutex）。
			// 这一支的处理与唤醒自愈的重启部分一致，但不拆连接；若不先排掉，
			// maybeRecoverFromSystemResume 会把上传进行中的这段停摆当成系统睡眠。
			if a.selfInflictedMonitorStall(gap, updateInterval) {
				a.logInfo("监控循环停摆 %s 由本机设备独占操作（屏保上传/读图）造成，跳过唤醒自愈并重宣示承载",
					gap.Round(time.Second))
				resetSmartControlSampling()
				lastTargetRPM = -1
				timer.Reset(updateInterval)
				continue
			}
			if a.maybeRecoverFromSystemResume("temperature-monitor", gap, updateInterval) {
				resetSmartControlSampling()
				lastTargetRPM = -1
				timer.Reset(updateInterval)
				continue
			}

			// revision 未变时跳过全量 AppConfig 拷贝（含 DeviceProfiles/FanCurveProfilesByDevice 等大字段），
			// 减少每 tick 产生的短命对象，降低 GC 压力。
			if newRev := a.configManager.Revision(); newRev != cfgRevision {
				learningDirty = false
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
			updateInterval = monitorOnlySamplingInterval(activeTemperatureMonitorInterval(cfg.TempUpdateRate, hasClients, cfg.AutoControl), a.monitorOnlyActive())
			// GUI 断开瞬间把会话期间膨胀的堆内存归还操作系统，降低核心常驻后台时的 RSS。
			if prevHasClients && !hasClients && now.Sub(lastMemRelease) > idleMemoryReleaseCooldown {
				lastMemRelease = now
				a.safeGo("release-idle-memory", func() { debug.FreeOSMemory() })
			}
			prevHasClients = hasClients

			temp := a.tempReader.Read(cachedSelection)
			staleBridgeTelemetry := false
			if temp.BridgeOk {
				bridgeFailureCount = 0
				lastBridgeUpdateTime, staleBridgeUpdateCount, staleBridgeTelemetry = trackBridgeTemperatureStaleness(temp, lastBridgeUpdateTime, staleBridgeUpdateCount)
				if staleBridgeTelemetry && time.Since(lastBridgeRestart) >= temperatureBridgeRestartCooldown {
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

			// 黑鲨参照源：cpu/gpu 固定，max 动态取更高项（带迟滞 + 最小间隔）。
			// 与上面那条推送同频挂在这里，理由相同：主机采样只应有一个来源。
			a.syncBlackSharkCoolingSourceForMax(cfg, temp.CPUTemp, temp.GPUTemp)

			fanData := a.deviceManager.GetCurrentFanData()
			historyPoint, recorded := a.tempHistory.Add(temp, fanData)

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
				smartCfg, smartChanged = smartcontrol.NormalizeConfigForUnit(cfg.SmartControl, a.smartControlCurveForUnit(&cfg, speedUnit), cfg.DebugMode, speedUnit)
				smartCfgRevision = cfgRevision
				if smartChanged {
					updatedCfg, updatedRevision, applied, persistErr := a.commitConfigMutationIfRevision(cfgRevision, func(current *types.AppConfig) {
						current.SmartControl = smartCfg
					}, nil)
					if persistErr != nil {
						a.logError("保存智能控温配置失败: %v", persistErr)
						cfg, cfgRevision = a.configManager.GetWithRevision()
						smartCfgRevision = cfgRevision - 1
						timer.Reset(updateInterval)
						continue
					}
					if !applied {
						cfg, cfgRevision = a.configManager.GetWithRevision()
						smartCfgRevision = cfgRevision - 1
						timer.Reset(updateInterval)
						continue
					}
					cfg = updatedCfg
					cfgRevision = updatedRevision
					smartCfg = cfg.SmartControl
					smartCfgRevision = updatedRevision
				}
			}
			advancedTelemetryUsable := !cfg.PowerSpoofEnabled && !staleBridgeTelemetry && hasUsableSmartControlTelemetry(temp)
			if !advancedTelemetryUsable && lastSmartTelemetryUsable {
				resetSmartControlSampling()
			}
			lastSmartTelemetryUsable = advancedTelemetryUsable

			inputReady := automaticControlInputReady(temp)
			controlReady := a.deviceControlReady()
			if controlReady && lastTargetRPM < 0 {
				speedUnit = a.activeDeviceSpeedUnit(&cfg)
				lastTargetRPM = fanDataTargetForControlUnit(fanData, speedUnit)
			}
			applySafetyFallback, safetyFallbackRecovered := false, false
			switch {
			case !cfg.AutoControl:
				applySafetyFallback, safetyFallbackRecovered = safetyFallback.observe(cfg.AutoControl, inputReady)
			case controlReady && !bridge.IsStarting(temp.BridgeMsg) && !staleBridgeTelemetry && temp.TelemetrySource != types.TelemetrySourceBridgeCache:
				applySafetyFallback, safetyFallbackRecovered = safetyFallback.observe(true, inputReady)
			}
			if safetyFallbackRecovered {
				a.forceNextAutoTarget.Store(true)
				// 变频下"强制下一拍"已经不下发任何东西（曲线已写进设备），
				// 遥测恢复后必须显式把该档曲线写回设备，否则设备可能还停在安全回退/固定形态。
				a.reassertBlackSharkInverterGear("温度遥测恢复")
				a.logInfo("温度遥测已恢复，退出安全转速并恢复正常自动控制")
			}
			if cfg.AutoControl && inputReady && !lastAutoControl {
				a.forceNextAutoTarget.Store(true)
				a.reassertBlackSharkInverterGear("自动控制刚生效")
			}
			safetyFallbackDecisionRecorded := false
			if applySafetyFallback && controlReady {
				if a.noiseDiagnosticLeaseActive() {
					a.cancelNoiseDiagnosticLease("温度遥测安全回退")
				}
				speedUnit = a.activeDeviceSpeedUnit(&cfg)
				target := temperatureSafetyFallbackTarget(a.smartControlCurveForUnit(&cfg, speedUnit), speedUnit, fanData)
				// 安全回退也套上升/降温限速：回退本身可能就是一大跳，限速能避免转速突变。
				target = rampTemperatureSafetyFallbackTarget(target, lastTargetRPM, smartCfg.RampUpLimit, smartCfg.RampDownLimit)
				if target > 0 {
					// 安全回退是"重要写入"：先作废还排着队的例行目标，否则那条旧目标随后会把安全值盖掉。
					if a.deviceManager.IsBlackSharkActive() {
						a.deviceManager.CancelPendingSpeedPush()
					}
					// 必须按固定形态下发（`false`）：变频形态下主机不下发转速，回退会被静默吞掉并假报成功。
					writeSucceeded := a.setTargetSpeedWithMode(target, speedUnit, false)
					gateReason := "write-failed"
					if writeSucceeded {
						gateReason = "temperature-safety-fallback"
					}
					a.setSmartControlDecision(smartcontrol.Decision{
						Active:         true,
						ControlTemp:    temp.ControlTemp,
						ControlSource:  types.NormalizeTempSource(temp.ControlSource),
						SpeedUnit:      speedUnit,
						SafetyFallback: true,
						FinalTarget:    target,
						WriteAttempted: true,
						WriteSucceeded: writeSucceeded,
						GateReason:     gateReason,
						Confidence:     "safety",
					})
					safetyFallbackDecisionRecorded = true
					if writeSucceeded {
						safetyFallback.markApplied()
						lastTargetRPM = target
						a.logError("连续 %d 次未获得可信温度，已切换到安全转速: %d%s", safetyFallback.invalidSamples, displaySpeedForLog(target, speedUnit), types.FanSpeedDisplaySuffix(speedUnit))
					} else {
						a.logError("温度失效安全转速下发失败，将在下个周期重试: %d%s", displaySpeedForLog(target, speedUnit), types.FanSpeedDisplaySuffix(speedUnit))
					}
				}
			}
			if cfg.AutoControl && inputReady && controlReady {
				speedUnit = a.activeDeviceSpeedUnit(&cfg)
				sampleContext := newSmartControlSampleContext(cachedSelection, speedUnit)
				contextChanged := !lastAutoControl || sampleContext != lastSmartSampleContext
				if contextChanged {
					unitChanged := sampleContext.speedUnit != lastSmartSampleContext.speedUnit
					resetSmartControlSampling()
					if unitChanged {
						lastTargetRPM = -1
					}
				}
				lastAutoControl = true
				lastSmartSampleContext = sampleContext
				controlCurve := a.smartControlCurveForUnit(&cfg, speedUnit)
				// 采样窗口变化时重置 EMA，避免阶跃。
				newSampleCount := max(cfg.TempSampleCount, 1)
				if newSampleCount != sampleCount {
					sampleCount = newSampleCount
					tempEMAReady = false
				}
				// 设备独占任务（屏保图传）刚结束：窗口里循环被饿住十几秒，旧 EMA 不能当"上一个采样"
				// 接着平滑（那等于把空洞折成一个采样步长）—— 与上面"窗口变化"同一处置：丢旧、重新起。
				if a.reseedTempEMA.Swap(false) {
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
				advancedSampleUsable := advancedTelemetryUsable && validSmartControlTemperature(controlTemp)
				if !advancedSampleUsable {
					if lastSmartTelemetryUsable {
						resetSmartControlSampling()
					}
					lastSmartTelemetryUsable = false
				} else if spikeSuppressed {
					risePredictionSamples = risePredictionSamples[:0]
					advancedSampleUsable = false
				} else {
					lastSmartTelemetryUsable = true
				}
				recentControlTemps = append(recentControlTemps, controlTemp)
				if len(recentControlTemps) > 24 {
					recentControlTemps = recentControlTemps[len(recentControlTemps)-24:]
				}

				prevTargetRPM := lastTargetRPM
				if a.autoPushFailed.Swap(false) && prevTargetRPM > 0 {
					// 上一拍的异步 Mission 失败了 ⇒ 把去重状态放回"未确认"，逼这一拍重试。
					lastTargetRPM = -1
					prevTargetRPM = -1
					a.logError("智能控温速度下发失败，将在下个周期重试")
				}

				actualSpeed := 0
				actualSpeedValid := false
				if fanData != nil && fanData.CurrentRPM > 0 {
					actualSpeed = fanDataSpeedForControlUnit(int(fanData.CurrentRPM), speedUnit)
					actualSpeedValid = true
				}

				effectivePower := smartcontrol.EffectivePower{}
				if advancedSampleUsable {
					effectivePower = effectiveTemperaturePower(temp)
					risePredictionSamples = append(risePredictionSamples, newSmartControlRisePredictionSample(now, temp, controlTemp, effectivePower, prevTargetRPM, actualSpeed, actualSpeedValid))
					if len(risePredictionSamples) > 12 {
						risePredictionSamples = risePredictionSamples[len(risePredictionSamples)-12:]
					}
				}
				decision := smartcontrol.EvaluateDecision(smartcontrol.DecisionInput{
					ControlTemp:           controlTemp,
					ControlSource:         temp.ControlSource,
					Curve:                 controlCurve,
					Config:                smartCfg,
					SpeedUnit:             speedUnit,
					PreviousTarget:        prevTargetRPM,
					AdvancedTelemetry:     advancedSampleUsable,
					PowerAvailable:        effectivePower.CPUValid || effectivePower.GPUValid,
					RisePredictionSamples: risePredictionSamples,
				})
				targetRPM := decision.FinalTarget
				predictionActive := decision.PredictionRampMultiplier > 1 || decision.TemperatureRiseBoost > 0

				rawAxisNoiseTarget := targetRPM
				axisNoiseAdjusted := false
				if adjustedTarget, adjusted := axisNoiseTargetForDevice(cfg, a.activeDeviceCurveScopeKey(cfg), targetRPM, prevTargetRPM, speedUnit); adjusted {
					targetRPM = min(adjustedTarget, decision.CurveMax)
					axisNoiseAdjusted = targetRPM != rawAxisNoiseTarget
				}

				targetLimited := false
				requestedTargetRPM := targetRPM
				targetRPM, targetLimited = applyFlyDigiRuntimeCapabilityToTarget(targetRPM, fanData, speedUnit)
				if targetLimited {
					a.logInfo("智能控温目标受飞智当前供电/挡位上限限制: %dRPM -> %dRPM", requestedTargetRPM, targetRPM)
				}
				decision.NoiseAdjustment = requestedTargetRPM - rawAxisNoiseTarget
				decision.HardwareAdjustment = targetRPM - requestedTargetRPM
				decision.FinalTarget = targetRPM
				observedRPM := targetRPM
				if actualSpeedValid {
					observedRPM = actualSpeed
				}
				forceSend := false
				if targetRPM > 0 {
					forceSend = a.forceNextAutoTarget.Swap(false)
				}
				noiseLeaseActive := a.noiseDiagnosticLeaseActive()
				shouldWrite := targetRPM > 0 && !noiseLeaseActive && (forceSend || shouldSendTargetSpeed(targetRPM, prevTargetRPM, smartCfg.MinRPMChange, fanData, speedUnit))
				writeSucceeded := false
				gateReason := "minimum-change"
				switch {
				case targetRPM <= 0:
					gateReason = "no-target"
				case noiseLeaseActive:
					gateReason = "noise-diagnostic-active"
				case forceSend:
					gateReason = "forced"
				case shouldWrite:
					gateReason = "target-change"
				}
				if shouldWrite {
					if a.deviceManager.IsBlackSharkActive() {
						// 黑鲨：例行下发走异步 Mission（对齐官方 StartExecuteAsynchronization），
						// 不让设备 I/O 占住采集线程。写入闭包必须调 a.setTargetSpeed ——
						// 只有核心层知道黑鲨的变频/固定分流；设备层的 SetTargetSpeed 只会写 form=0 固定值。
						// 变频下这个闭包什么都不发（曲线已写进设备、主机只推温度）；
						// 保留这条异步路径是为了固定模式的写入不阻塞采集。
						a.deviceManager.PushSpeedAsync(
							func() bool { return a.setTargetSpeed(targetRPM, speedUnit) },
							func(ok bool) {
								if !ok {
									a.autoPushFailed.Store(true)
								}
							},
						)
						lastTargetRPM = targetRPM
					} else {
						// 其它设备：保持原有同步行为不变。
						writeSucceeded = a.setTargetSpeed(targetRPM, speedUnit)
						if writeSucceeded {
							lastTargetRPM = targetRPM
							if a.deviceManager.GetDeviceType() == types.DeviceTransportWiFi {
								lastWiFiOverviewRefresh = now
							}
						} else {
							lastTargetRPM = -1
							gateReason = "write-failed"
							a.logError("智能控温速度下发失败，将在下个周期重试: %d%s", displaySpeedForLog(targetRPM, speedUnit), types.FanSpeedDisplaySuffix(speedUnit))
						}
					}
				}

				// 诊断（低频）：判定变化时记一条，用于排查黑鲨"目标在动却一直不写"。
				// 只对黑鲨打，避免给其它设备增加日志噪声。
				if gateReason != lastGateReason && a.deviceManager.IsBlackSharkActive() {
					deviceTargetRPM := 0
					if fanData != nil {
						deviceTargetRPM = int(fanData.TargetRPM)
					}
					a.logInfo("智能控温闸门: %s（目标=%d%s 上次=%d 设备目标=%d 最小变化=%d unit=%s）",
						gateReason, displaySpeedForLog(targetRPM, speedUnit), types.FanSpeedDisplaySuffix(speedUnit),
						prevTargetRPM, deviceTargetRPM, smartCfg.MinRPMChange, speedUnit)
				}
				lastGateReason = gateReason

				confidence := "basic"
				if advancedSampleUsable {
					confidence = "medium"
					if actualSpeedValid {
						confidence = "high"
					}
				}
				decision.WriteAttempted = shouldWrite
				decision.WriteSucceeded = writeSucceeded
				decision.GateReason = gateReason
				decision.Confidence = confidence
				a.setSmartControlDecision(decision)

				if smartCfg.Learning && advancedSampleUsable && !predictionActive && !targetLimited && !axisNoiseAdjusted && !a.noiseDiagnosticLeaseActive() {
					steady := steadyObserver.ObserveWithEffectivePowerAt(now, controlTemp, observedRPM, effectivePower, controlCurve, smartCfg)
					if steady.BucketIdx >= 0 && smartcontrol.AllowsLongTermOffsetLearning(steady, smartCfg) {
						scopeKey := a.activeDeviceCurveScopeKey(cfg)
						noiseGain := noiseLearningGainForDevice(cfg, scopeKey, steady.MeanRPM, speedUnit)
						newOffsets, changed := learnSteadyOffsetForActiveUnit(steady.BucketIdx, steady.MeanTemp, steady.MeanPower, steady.HavePower, steady.LocalEff, steady.HaveEff, controlCurve, smartCfg.LearnedOffsets, smartCfg, speedUnit, noiseGain)
						if changed {
							nextSmartCfg := smartCfg
							nextSmartCfg.LearnedOffsets = newOffsets
							cfg.SmartControl = nextSmartCfg
							storeSmartControlOffsetsForDeviceKey(&cfg, scopeKey)
							smartCfg = cfg.SmartControl
							smartCfgRevision = cfgRevision
							learningDirty = true
						}
					}

					if learningDirty && time.Since(lastLearningSave) >= smartControlLearningSaveInterval {
						updatedCfg, updatedRevision, applied, err := a.commitConfigMutationIfRevision(cfgRevision, func(current *types.AppConfig) {
							current.SmartControl = cfg.SmartControl
						}, nil)
						if err != nil {
							a.logError("保存学习偏移失败: %v", err)
						} else if !applied {
							cfg, cfgRevision = a.configManager.GetWithRevision()
							smartCfgRevision = cfgRevision - 1
							learningDirty = false
							resetSmartControlSampling()
							timer.Reset(updateInterval)
							continue
						} else {
							cfg = updatedCfg
							cfgRevision = updatedRevision
							smartCfg = cfg.SmartControl
							smartCfgRevision = updatedRevision
							lastLearningSave = time.Now()
							learningDirty = false
						}
					}
				} else if !smartCfg.Learning || !advancedSampleUsable || predictionActive || targetLimited {
					steadyObserver.Reset()
				}

				if decision.BaseTarget > 0 {
					a.logDebug("智能控温: 最高=%d°C 基准=%s 当前=%d°C 平均=%d°C 控制温度=%d°C 基础=%d%s 学习=%+d 温升=%+d 坡度=%+d 噪声=%+d 硬件=%+d 目标=%d%s 原因=%s", temp.MaxTemp, temp.ControlSource, temp.ControlTemp, avgTemp, controlTemp, displaySpeedForLog(decision.BaseTarget, speedUnit), types.FanSpeedDisplaySuffix(speedUnit), decision.LearningOffset, decision.TemperatureRiseBoost, decision.RampAdjustment, decision.NoiseAdjustment, decision.HardwareAdjustment, displaySpeedForLog(decision.FinalTarget, speedUnit), types.FanSpeedDisplaySuffix(speedUnit), decision.GateReason)
				}
			}

			gateReason := smartcontrol.GateReason(cfg.AutoControl, inputReady, controlReady)
			if gateReason != "" {
				if !safetyFallbackDecisionRecorded {
					a.setSmartControlDecision(smartcontrol.Decision{
						ControlTemp:   temp.ControlTemp,
						ControlSource: types.NormalizeTempSource(temp.ControlSource),
						SpeedUnit:     speedUnit,
						GateReason:    gateReason,
					})
				}
				if lastAutoControl {
					resetSmartControlSampling()
				}
				lastAutoControl = false
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

	// 监控停了就别再空推 0x07（设 0 = 停；不是黑鲨时是空操作）。
	if a.deviceManager != nil {
		if err := a.deviceManager.SetBlackSharkHostInfoRefreshInterval(0, nil); err != nil {
			a.logDebug("停止 0x07 主机信息推送失败: %v", err)
		}
	}
}

func temperatureMonitorInterval(updateRateSeconds int) time.Duration {
	if updateRateSeconds < 1 {
		updateRateSeconds = 1
	}
	return time.Duration(updateRateSeconds) * time.Second
}

func monitorOnlySamplingInterval(interval time.Duration, monitorOnly bool) time.Duration {
	if monitorOnly && interval < monitorOnlyTemperatureInterval {
		return monitorOnlyTemperatureInterval
	}
	return interval
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

func validSmartControlTemperature(temp int) bool {
	return temp > 0 && temp <= types.FanCurveMaxTemperature+15
}

// hasUsableSmartControlTelemetry gates only prediction and learning. Baseline
// automatic fan control intentionally continues to use its existing path.
func hasUsableSmartControlTelemetry(temp types.TemperatureData) bool {
	if !temp.TelemetryFresh || !validSmartControlTemperature(temp.ControlTemp) {
		return false
	}
	cpuTempValid := validSmartControlTemperature(temp.CPUTemp)
	gpuTempValid := validSmartControlTemperature(temp.GPUTemp) && temp.GPUReadState != types.GPUReadStateNotPolled
	switch types.NormalizeTempSource(temp.ControlSource) {
	case types.TempSourceCPU:
		return cpuTempValid
	case types.TempSourceGPU:
		return gpuTempValid
	default:
		return cpuTempValid || gpuTempValid
	}
}

// effectiveTemperaturePower keeps unknown power unknown. It is intentionally
// stricter than the display/control path: stale or implausible telemetry may
// still drive the existing safety controls, but must not train or predict.
func effectiveTemperaturePower(temp types.TemperatureData) smartcontrol.EffectivePower {
	if !hasUsableSmartControlTelemetry(temp) {
		return smartcontrol.EffectivePower{}
	}

	cpuTempValid := validSmartControlTemperature(temp.CPUTemp)
	gpuTempValid := validSmartControlTemperature(temp.GPUTemp) && temp.GPUReadState != types.GPUReadStateNotPolled
	switch types.NormalizeTempSource(temp.ControlSource) {
	case types.TempSourceCPU:
		if !cpuTempValid {
			return smartcontrol.EffectivePower{}
		}
	case types.TempSourceGPU:
		if !gpuTempValid {
			return smartcontrol.EffectivePower{}
		}
	default:
		if !cpuTempValid && !gpuTempValid {
			return smartcontrol.EffectivePower{}
		}
	}

	return smartcontrol.EffectivePower{
		CPUWatts: temp.CPUPowerWatts,
		GPUWatts: temp.GPUPowerWatts,
		CPUValid: cpuTempValid && temp.CPUPowerWatts > 0,
		GPUValid: gpuTempValid && temp.GPUPowerWatts > 0,
	}
}

func newSmartControlRisePredictionSample(now time.Time, temp types.TemperatureData, controlTemp int, power smartcontrol.EffectivePower, previousTarget, actualSpeed int, actualSpeedValid bool) smartcontrol.RisePredictionSample {
	cpuTemp := temp.CPUTemp
	if !validSmartControlTemperature(cpuTemp) {
		cpuTemp = 0
	}
	gpuTemp := temp.GPUTemp
	if !validSmartControlTemperature(gpuTemp) || temp.GPUReadState == types.GPUReadStateNotPolled {
		gpuTemp = 0
	}
	return smartcontrol.RisePredictionSample{
		SampledAt:        now,
		ControlTemp:      controlTemp,
		CPUTemp:          cpuTemp,
		GPUTemp:          gpuTemp,
		CPUPowerWatts:    power.CPUWatts,
		GPUPowerWatts:    power.GPUWatts,
		CPUPowerValid:    power.CPUValid,
		GPUPowerValid:    power.GPUValid,
		ControlSource:    types.NormalizeTempSource(temp.ControlSource),
		PreviousTarget:   previousTarget,
		ActualSpeed:      actualSpeed,
		ActualSpeedValid: actualSpeedValid,
	}
}

func newStableObserverForActiveUnit(curveLen int, unit string) *smartcontrol.StableObserver {
	if types.IsPercentSpeedUnit(unit) {
		return smartcontrol.NewPercentStableObserver(curveLen)
	}
	return smartcontrol.NewLegacyRPMStableObserver(curveLen)
}

func learnSteadyOffsetForActiveUnit(bucketIdx int, meanTemp int, meanPower float64, havePower bool, localEff float64, haveEff bool, curve []types.FanCurvePoint, prevOffsets []int, cfg types.SmartControlConfig, unit string, noiseGain float64) ([]int, bool) {
	return smartcontrol.LearnSteadyOffsetForUnitWithPowerAndNoiseGain(bucketIdx, meanTemp, meanPower, havePower, localEff, haveEff, smartcontrol.CurveForUnit(curve, unit), prevOffsets, cfg, unit, noiseGain)
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
