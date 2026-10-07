package coreapp

import (
	"context"
	"fmt"
	"github.com/Eureka-o/FanControlPortable/internal/appmeta"
	"github.com/Eureka-o/FanControlPortable/internal/config"
	"github.com/Eureka-o/FanControlPortable/internal/curveprofiles"
	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/smartcontrol"
	"github.com/Eureka-o/FanControlPortable/internal/types"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

func (a *CoreApp) finishConfigCommit(cfg types.AppConfig, afterCommit func(types.AppConfig)) {
	if afterCommit != nil {
		afterCommit(cfg)
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
}

func (a *CoreApp) commitConfigUpdate(cfg types.AppConfig, afterCommit func(types.AppConfig)) error {
	// 自动连接 BLE 冷却时间是运行期缓存、不是用户设置：本函数用调用方的**整体快照**覆盖配置，
	// 某个调用方拿旧快照保存时就会把它清掉（下次启动又白等 10s）。这里只在快照丢了它时补成
	// 当前生效值，已有值不动 —— 写入方（MutateAndSave）带的是非零新值，不会被这里回退。
	if cfg.NativeAutoBLEScanUnix == 0 && a.configManager != nil {
		cfg.NativeAutoBLEScanUnix = a.configManager.Get().NativeAutoBLEScanUnix
	}
	if err := a.configManager.Update(cfg); err != nil {
		return err
	}
	cfg = a.configManager.Get()
	a.finishConfigCommit(cfg, afterCommit)
	return nil
}

func (a *CoreApp) commitConfigMutation(mutate func(*types.AppConfig), afterCommit func(types.AppConfig)) (types.AppConfig, error) {
	cfg, err := a.configManager.MutateAndSave(mutate)
	if err != nil {
		return cfg, err
	}
	a.finishConfigCommit(cfg, afterCommit)
	return cfg, nil
}

func (a *CoreApp) commitConfigMutationIfRevision(expected uint64, mutate func(*types.AppConfig), afterCommit func(types.AppConfig)) (types.AppConfig, uint64, bool, error) {
	cfg, revision, applied, err := a.configManager.MutateIfRevisionAndSave(expected, mutate)
	if err != nil || !applied {
		return cfg, revision, applied, err
	}
	a.finishConfigCommit(cfg, afterCommit)
	return cfg, revision, true, nil
}

func runtimeDebugInfo() map[string]any {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	toMB := func(value uint64) float64 {
		return float64(value) / (1024 * 1024)
	}

	lastGC := ""
	if mem.LastGC > 0 {
		lastGC = time.Unix(0, int64(mem.LastGC)).Format("2006-01-02 15:04:05")
	}

	return map[string]any{
		"goroutines":     runtime.NumGoroutine(),
		"allocMB":        toMB(mem.Alloc),
		"heapAllocMB":    toMB(mem.HeapAlloc),
		"heapInUseMB":    toMB(mem.HeapInuse),
		"heapIdleMB":     toMB(mem.HeapIdle),
		"heapReleasedMB": toMB(mem.HeapReleased),
		"stackInUseMB":   toMB(mem.StackInuse),
		"sysMB":          toMB(mem.Sys),
		"heapObjects":    mem.HeapObjects,
		"nextGCMB":       toMB(mem.NextGC),
		"numGC":          mem.NumGC,
		"lastGC":         lastGC,
		"gccpFraction":   mem.GCCPUFraction,
		"pauseTotalMs":   float64(mem.PauseTotalNs) / 1_000_000,
	}
}

func configSpeedToTargetUnit(speed int, unit string) int {
	if types.IsPercentSpeedUnit(unit) {
		return types.PercentToTicks(speed)
	}
	return types.ClampRPM(speed)
}

func (a *CoreApp) activeDeviceCapabilities() types.DeviceCapabilities {
	if a.deviceManager != nil && a.deviceManager.IsConnected() {
		return a.deviceManager.ActiveCapabilities()
	}
	cfg := a.configManager.Get()
	return types.ActiveDeviceProfile(&cfg).Capabilities
}

func (a *CoreApp) activeDeviceSupportsManualGears() bool {
	caps := a.activeDeviceCapabilities()
	return caps.SupportsManualGears || caps.SupportsSetSpeed || caps.SupportsCustomSpeed
}

// UpdateConfig 更新配置
func (a *CoreApp) UpdateConfig(cfg types.AppConfig) error {
	a.mutex.Lock()
	locked := true
	defer func() {
		if locked {
			a.mutex.Unlock()
		}
	}()

	oldCfg := a.configManager.Get()
	monitorOnlyChanged := oldCfg.MonitorOnly != cfg.MonitorOnly
	cfg.HistoryRetentionHours = types.NormalizeTemperatureHistoryRetentionHours(oldCfg.HistoryRetentionHours)
	oldConnectionKey := deviceProfileConnectionKey(oldCfg)
	if len(cfg.FanCurveProfiles) == 0 && len(oldCfg.FanCurveProfiles) > 0 {
		cfg.FanCurveProfiles = curveprofiles.CloneProfiles(oldCfg.FanCurveProfiles)
		cfg.ActiveFanCurveProfileID = oldCfg.ActiveFanCurveProfileID
	}
	if len(cfg.DeviceProfiles) == 0 && len(oldCfg.DeviceProfiles) > 0 {
		cfg.DeviceProfiles = oldCfg.DeviceProfiles
	}
	if cfg.ActiveDeviceProfileID == "" {
		cfg.ActiveDeviceProfileID = oldCfg.ActiveDeviceProfileID
	}
	cfg.LegionFnQSupport = oldCfg.LegionFnQSupport
	// 同 LegionFnQSupport：这是运行期缓存，不是用户设置 ⇒ 前端的配置更新不该覆盖它。
	// 漏了这行的话，用户在设置页点一下就会把 BLE 冷却状态清掉 ⇒ 下次启动又白等 10s。
	cfg.NativeAutoBLEScanUnix = oldCfg.NativeAutoBLEScanUnix
	cfg.ManualGearLevels = cloneManualGearLevels(oldCfg.ManualGearLevels)
	cfg.LightStrip, _ = normalizeLightStripConfig(cfg.LightStrip)
	cfg.ThemeMode = types.NormalizeThemeMode(cfg.ThemeMode)
	cfg.WindowBlur = types.NormalizeWindowBlur(cfg.WindowBlur)
	if cfg.DeviceTransport == "" {
		cfg.DeviceTransport = oldCfg.DeviceTransport
	}
	cfg.DeviceTransport = types.NormalizeDeviceTransport(cfg.DeviceTransport)
	if cfg.FanControlDeviceIp == "" {
		cfg.FanControlDeviceIp = oldCfg.FanControlDeviceIp
	}
	if cfg.FanControlDeviceIp == "" {
		cfg.FanControlDeviceIp = types.DefaultFanDeviceIP
	}
	cfg.WiFiSmartStartStopStandbySpeed = types.ClampWiFiSmartStartStopStandbyPercent(cfg.WiFiSmartStartStopStandbySpeed)
	types.NormalizeDeviceProfileConfig(&cfg)
	unit := a.activeDeviceSpeedUnit(&cfg)
	cfg.TempSource = types.NormalizeTempSource(cfg.TempSource)
	cfg.GpuDevice = types.NormalizeDeviceSelection(cfg.GpuDevice)
	cfg.CpuSensor = types.NormalizeSensorSelection(cfg.CpuSensor)
	cfg.GpuSensor = types.NormalizeSensorSelection(cfg.GpuSensor)
	cfg.CpuPowerSensor = types.NormalizeSensorSelection(cfg.CpuPowerSensor)
	cfg.GpuPowerSensor = types.NormalizeSensorSelection(cfg.GpuPowerSensor)
	if cfg.GpuReadMode == "" {
		if cfg.GpuLowPowerProtection {
			cfg.GpuReadMode = types.GPUReadModeAuto
		} else {
			cfg.GpuReadMode = types.GPUReadModeAlways
		}
	}
	cfg.GpuReadMode = types.NormalizeGPUReadMode(cfg.GpuReadMode)
	cfg.GpuLowPowerProtection = cfg.GpuReadMode != types.GPUReadModeAlways
	if cfg.GpuReadMode == types.GPUReadModeNever && cfg.TempSource == types.TempSourceGPU {
		cfg.TempSource = types.TempSourceCPU
	}
	types.NormalizePowerSpoofConfig(&cfg)
	prepareDeviceFanCurveStateForUpdate(&cfg, oldCfg)
	curveprofiles.NormalizeConfigForUnit(&cfg, unit)
	if idx := curveprofiles.FindIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID); idx >= 0 {
		cfg.FanCurveProfiles[idx].Curve = curveprofiles.CloneCurve(cfg.FanCurve)
	}
	// 必须传插值依据（黑鲨四档方案会补两端端点 ⇒ 5..6 点），不能传 cfg.FanCurve（4 点）：
	// 本函数内部按曲线长度给 LearnedOffsets 定长（smartcontrol 的 NormalizeConfigForUnit），
	// 传 4 会把它截成 4、丢掉 80℃ 那格的已学偏移，与 syncSmartControlOffsetsForDeviceKey 的长度约定冲突。
	cfg.SmartControl, _ = smartcontrol.NormalizeConfigForUnit(cfg.SmartControl, a.smartControlCurveForUnit(&cfg, unit), cfg.DebugMode, unit)
	prepareSmartControlOffsetsForUpdate(&cfg, oldCfg)
	runtimeDeviceKey := a.activeDeviceCurveScopeKey(cfg)
	syncSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey)
	storeDeviceFanCurveStateForKeyAndUnit(&cfg, runtimeDeviceKey, cfg, unit)
	cfg.LegionFnQ = types.NormalizeLegionFnQConfig(cfg.LegionFnQ)
	if a.legionFnQSupportChecked.Load() && !a.legionFnQSupported.Load() && (cfg.LegionFnQ.Enabled || cfg.LegionFnQ.TakeOverFan) {
		return fmt.Errorf("Lenovo Legion Fn+Q 仅支持拯救者设备")
	}
	normalizeHotkeyConfig(&cfg)
	normalizeManualGearMemoryConfig(&cfg)
	types.NormalizeManualGearRPMForUnit(&cfg, unit)
	cfg.CustomSpeedRPM = types.ClampSpeedForUnit(cfg.CustomSpeedRPM, unit)

	cfg.ConfigPath = oldCfg.ConfigPath
	configConnectionChanged := oldConnectionKey != deviceProfileConnectionKey(cfg)
	return a.commitConfigUpdate(cfg, func(committed types.AppConfig) {
		a.syncManualGearLevelMemoryLocked(committed)
		a.syncBlackSharkTempSource(committed)
		a.applyHotkeyBindings(committed)
		a.applyPluginConfig(committed)
		a.mutex.Unlock()
		locked = false

		if configConnectionChanged || a.deviceManager == nil {
			if configConnectionChanged {
				a.cancelReconnect()
			}
			if disconnected := a.reconcileDeviceManagerProfile(committed); disconnected {
				if configConnectionChanged {
					a.autoReconnectSuppressed.Store(true)
				}
				if a.ipcServer != nil {
					a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
				}
			}
		}
		if monitorOnlyChanged {
			if committed.MonitorOnly {
				a.refreshMonitorOnlyRuntime(true)
			}
			a.scheduleCoreRestart()
		}
	})
}

func (a *CoreApp) SetTemperatureHistoryEnabled(enabled bool) error {
	if err := a.tempHistory.SetEnabled(enabled); err != nil {
		return err
	}
	return nil
}

func (a *CoreApp) SetTemperatureHistoryRetentionHours(hours int) error {
	hours = types.NormalizeTemperatureHistoryRetentionHours(hours)
	previous := a.tempHistory.RetentionHours()
	if err := a.tempHistory.SetRetentionHours(hours); err != nil {
		return err
	}

	a.mutex.Lock()
	cfg := a.configManager.Get()
	cfg.HistoryRetentionHours = hours
	err := a.configManager.Update(cfg)
	committed := a.configManager.Get()
	a.mutex.Unlock()
	if err != nil {
		_ = a.tempHistory.SetRetentionHours(previous)
		return err
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, committed)
	}
	return nil
}

// SetFanCurve 设置风扇曲线
func (a *CoreApp) SetFanCurve(curve []types.FanCurvePoint) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	unit := a.activeDeviceSpeedUnit(&cfg)
	if err := config.ValidateFanCurveForUnit(curve, unit); err != nil {
		return err
	}

	curveprofiles.NormalizeConfigForUnit(&cfg, unit)
	cfg.FanCurve = curveprofiles.CloneCurve(curve)
	idx := curveprofiles.FindIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID)
	if idx >= 0 {
		cfg.FanCurveProfiles[idx].Curve = curveprofiles.CloneCurve(cfg.FanCurve)
	}
	// 同上面那处：传插值依据（黑鲨四档会补端点），否则 LearnedOffsets 长度会翻转。
	cfg.SmartControl, _ = smartcontrol.NormalizeConfigForUnit(cfg.SmartControl, a.smartControlCurveForUnit(&cfg, unit), cfg.DebugMode, unit)
	runtimeDeviceKey := a.activeDeviceCurveScopeKey(cfg)
	storeSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey)
	storeDeviceFanCurveStateForKeyAndUnit(&cfg, runtimeDeviceKey, cfg, unit)
	return a.commitConfigUpdate(cfg, nil)
}

// ResetLearnedOffsets 清空学习到的曲线偏移。
func (a *CoreApp) ResetLearnedOffsets() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	cfg.SmartControl = smartcontrol.ResetLearnedState(cfg.SmartControl, cfg.FanCurve)
	resetSmartControlOffsetsForDeviceKey(&cfg, a.activeDeviceCurveScopeKey(cfg))
	return a.commitConfigUpdate(cfg, func(types.AppConfig) {
		a.logInfo("已重置学习偏移")
	})
}

// SetAutoControl 设置智能变频
func (a *CoreApp) SetAutoControl(enabled bool) error {
	a.mutex.Lock()

	cfg := a.configManager.Get()

	if enabled && cfg.CustomSpeedEnabled {
		a.mutex.Unlock()
		return fmt.Errorf("自定义转速模式下无法开启智能变频")
	}

	cfg.AutoControl = enabled
	applyManualAfterDisable := !enabled && a.isConnected
	if applyManualAfterDisable {
		a.mutex.Unlock()
		if applyErr := a.applyCurrentGearSetting(); applyErr != nil {
			return applyErr
		}
		a.mutex.Lock()
		cfg = a.configManager.Get()
		cfg.AutoControl = enabled
	}

	err := a.commitConfigUpdate(cfg, func(types.AppConfig) {
		if enabled {
			a.userSetAutoControl = true
			a.forceNextAutoTarget.Store(true)
			// 变频下"强制下一拍"不下发任何东西，开启自动控制时必须显式补写该档曲线
			// （否则设备可能停在旧的 form=0 形态，而界面以为已在变频）。
			a.reassertBlackSharkInverterGear("开启自动控制")
		}
	})
	a.mutex.Unlock()

	return err
}

// applyCurrentGearSetting 应用当前挡位设置
func (a *CoreApp) applyCurrentGearSetting() error {
	if !a.activeDeviceSupportsManualGears() {
		return fmt.Errorf("当前设备不支持手动挡位")
	}
	if a.deviceManager == nil {
		return fmt.Errorf("设备未连接")
	}
	fanData := a.deviceManager.GetCurrentFanData()
	cfg := a.configManager.Get()
	setGear := strings.TrimSpace(cfg.ManualGear)
	if _, ok := types.GearIndex(setGear); !ok {
		setGear = ""
		if fanData != nil {
			setGear = strings.TrimSpace(fanData.SetGear)
		}
	}
	if _, ok := types.GearIndex(setGear); !ok {
		return fmt.Errorf("当前设备未提供可用手动挡位")
	}
	level := a.getRememberedManualLevel(setGear, cfg.ManualLevel)
	rpm := cfg.ResolveGearRPM(setGear, level)
	unit := a.activeDeviceSpeedUnit(&cfg)
	if rpm <= 0 {
		return fmt.Errorf("当前手动挡位没有可用速度")
	}

	a.logInfo("应用当前挡位设置: %s %s (%d%s)", setGear, level, rpm, types.FanSpeedDisplaySuffix(unit))

	// 黑鲨由 SetManualGearRPM 内部路由到 SetBlackSharkFixedSpeedForGear（按档位下发固定转速），
	// 与「点一下挡位」走的是同一条路 —— 开关智能变频时的下发结果因此与手动点挡位完全一致。
	if !a.deviceManager.SetManualGearRPM(setGear, level, rpm) {
		return fmt.Errorf("手动挡位下发失败")
	}
	return nil
}

// SetManualGear 设置手动挡位
func (a *CoreApp) SetManualGear(gear, level string) bool {
	a.mutex.Lock()
	cfg := a.configManager.Get()
	wasConnected := a.isConnected || (a.deviceManager != nil && a.deviceManager.IsConnected())
	unit := a.activeDeviceSpeedUnit(&cfg)
	types.NormalizeManualGearRPMForUnit(&cfg, unit)
	rpm := cfg.ResolveGearRPM(gear, level)
	manualGearSupported := a.activeDeviceSupportsManualGears()
	a.mutex.Unlock()

	if !manualGearSupported {
		return false
	}
	if wasConnected && (a.deviceManager == nil || !a.deviceManager.SetManualGearRPM(gear, level, rpm)) {
		return false
	}

	cfg = a.configManager.Get()
	cfg.AutoControl = false
	cfg.CustomSpeedEnabled = false
	cfg.ManualGear = gear
	cfg.ManualLevel = level
	if cfg.ManualGearLevels == nil {
		cfg.ManualGearLevels = map[string]string{}
	}
	cfg.ManualGearLevels[gear] = normalizeManualLevel(level)
	unit = a.activeDeviceSpeedUnit(&cfg)
	types.NormalizeManualGearRPMForUnit(&cfg, unit)

	if err := a.commitConfigUpdate(cfg, func(types.AppConfig) {
		a.rememberManualGearLevel(gear, level)
	}); err != nil {
		a.logError("保存手动挡位配置失败: %v", err)
		return false
	}

	if !wasConnected {
		return true
	}
	return true
}

// SetCustomSpeed 设置自定义转速
func (a *CoreApp) SetCustomSpeed(enabled bool, rpm int) error {
	a.mutex.Lock()

	cfg := a.configManager.Get()
	unit := a.activeDeviceSpeedUnit(&cfg)
	wasConnected := a.isConnected

	if enabled {
		rpm = types.ClampSpeedForUnit(rpm, unit)
		if cfg.AutoControl {
			cfg.AutoControl = false
		}

		cfg.CustomSpeedEnabled = true
		cfg.CustomSpeedRPM = rpm
	} else {
		cfg.CustomSpeedEnabled = false
	}
	applyManualAfterDisable := !enabled && wasConnected
	a.mutex.Unlock()

	if enabled && wasConnected {
		// 用户显式设固定转速 ⇒ 固定转速意图（黑鲨走 form=0），不是变频。
		if !a.setTargetSpeedWithMode(configSpeedToTargetUnit(rpm, unit), unit, false) {
			return fmt.Errorf("当前设备拒绝自定义速度下发，请确认设备仍已连接并支持 %s 控制", types.FanSpeedDisplaySuffix(unit))
		}
	}

	a.mutex.Lock()
	err := a.commitConfigUpdate(cfg, nil)
	a.mutex.Unlock()

	if err != nil {
		return err
	}
	if applyManualAfterDisable {
		a.safeGo("applyCurrentGearSettingAfterCustomSpeed", func() {
			if err := a.applyCurrentGearSetting(); err != nil {
				a.logError("应用当前挡位设置失败: %v", err)
			}
		})
	}

	return nil
}

// SetGearLight 设置挡位灯
func (a *CoreApp) SetGearLight(enabled bool) bool {
	if !a.activeDeviceCapabilities().AllowsGearLight() {
		return false
	}
	if !a.deviceManager.SetGearLight(enabled) {
		return false
	}

	if _, err := a.commitConfigMutation(func(current *types.AppConfig) {
		current.GearLight = enabled
	}, nil); err != nil {
		a.logError("保存挡位灯配置失败: %v", err)
		return false
	}
	return true
}

// SetPowerOnStart 设置通电自启动
func (a *CoreApp) SetPowerOnStart(enabled bool) bool {
	if !a.activeDeviceCapabilities().SupportsPowerOnStart {
		return false
	}
	if !a.deviceManager.SetPowerOnStart(enabled) {
		return false
	}

	if _, err := a.commitConfigMutation(func(current *types.AppConfig) {
		current.PowerOnStart = enabled
	}, nil); err != nil {
		a.logError("保存通电自启动配置失败: %v", err)
		return false
	}
	return true
}

// SetSmartStartStop 设置智能启停
func (a *CoreApp) SetSmartStartStop(mode string) bool {
	if !a.activeDeviceCapabilities().SupportsSmartStartStop {
		return false
	}

	cfg := a.configManager.Get()
	cfg.WiFiSmartStartStopStandbySpeed = types.ClampWiFiSmartStartStopStandbyPercent(cfg.WiFiSmartStartStopStandbySpeed)
	if mode == "delayed" && types.NormalizeDeviceTransport(a.deviceManager.ActiveProfile().Transport) == types.DeviceTransportWiFi {
		_ = a.deviceManager.SetWiFiSmartStartStopStandbySpeed(cfg.WiFiSmartStartStopStandbySpeed)
	}
	if !a.deviceManager.SetSmartStartStop(mode) {
		return false
	}
	cfg.SmartStartStop = mode
	if _, err := a.commitConfigMutation(func(current *types.AppConfig) {
		current.SmartStartStop = mode
	}, nil); err != nil {
		a.logError("保存智能启停配置失败: %v", err)
		return false
	}
	return true
}

// SetWiFiSmartStartStopStandbySpeed 设置新固件心跳超时后的待机转速
func (a *CoreApp) SetWiFiSmartStartStopStandbySpeed(percent int) bool {
	if !a.activeDeviceCapabilities().SupportsSmartStartStop {
		return false
	}
	percent = types.ClampWiFiSmartStartStopStandbyPercent(percent)
	if !a.deviceManager.SetWiFiSmartStartStopStandbySpeed(percent) {
		return false
	}

	if _, err := a.commitConfigMutation(func(current *types.AppConfig) {
		current.WiFiSmartStartStopStandbySpeed = percent
	}, nil); err != nil {
		a.logError("保存 WiFi 待机转速配置失败: %v", err)
		return false
	}
	return true
}

// SetBrightness 设置亮度
func (a *CoreApp) SetBrightness(percentage int) bool {
	if !a.activeDeviceCapabilities().AllowsBrightness() {
		return false
	}
	if !a.deviceManager.SetBrightness(percentage) {
		return false
	}

	if _, err := a.commitConfigMutation(func(current *types.AppConfig) {
		current.Brightness = percentage
	}, nil); err != nil {
		a.logError("保存亮度配置失败: %v", err)
		return false
	}
	return true
}

// SetLightStrip 设置灯带
func (a *CoreApp) SetLightStrip(lightCfg types.LightStripConfig) error {
	if !a.activeDeviceCapabilities().AllowsLightStrip() {
		return fmt.Errorf("active device does not support lighting")
	}
	lightCfg, _ = normalizeLightStripConfig(lightCfg)

	a.mutex.Lock()
	cfg := a.configManager.Get()
	cfg.LightStrip = lightCfg
	connected := a.isConnected
	var runtimeErr error
	saveErr := a.commitConfigUpdate(cfg, func(types.AppConfig) {
		if connected {
			runtimeErr = a.deviceManager.SetLightStrip(lightCfg)
		}
	})
	a.mutex.Unlock()

	if saveErr != nil {
		return saveErr
	}
	return runtimeErr
}

func (a *CoreApp) applyConfiguredLightStrip() error {
	if !a.activeDeviceCapabilities().AllowsLightStrip() {
		return nil
	}
	cfg := a.configManager.Get()
	lightCfg, changed := normalizeLightStripConfig(cfg.LightStrip)

	if changed {
		cfg.LightStrip = lightCfg
		if err := a.commitConfigUpdate(cfg, nil); err != nil {
			a.logError("保存灯带默认配置失败: %v", err)
		}
	}

	return a.deviceManager.SetLightStrip(lightCfg)
}

func normalizeLightStripConfig(cfg types.LightStripConfig) (types.LightStripConfig, bool) {
	defaults := types.GetDefaultLightStripConfig()
	changed := false

	originalMode := cfg.Mode
	switch strings.ToLower(strings.TrimSpace(cfg.Mode)) {
	case "blackshark_breathing":
		cfg.Mode = "breathing"
	case "blackshark_static":
		cfg.Mode = "static_single"
	case "blackshark_flashing", "blackshark_refresh", "blackshark_color_flow":
		cfg.Mode = "flowing"
	case "blackshark_response":
		cfg.Mode = "breathing"
	case "blackshark_color_cycle":
		cfg.Mode = "rotation"
	}
	if cfg.Mode != originalMode {
		changed = true
	}

	if cfg.Mode == "" {
		cfg.Mode = defaults.Mode
		changed = true
	}
	if cfg.Speed == "" {
		cfg.Speed = defaults.Speed
		changed = true
	}
	if cfg.Brightness < 0 || cfg.Brightness > 100 {
		cfg.Brightness = defaults.Brightness
		changed = true
	}
	if len(cfg.Colors) == 0 {
		cfg.Colors = defaults.Colors
		changed = true
	}

	return cfg, changed
}

// SetWindowsAutoStart 设置Windows自启动
func (a *CoreApp) SetWindowsAutoStart(enable bool) error {
	if err := a.autostartManager.SetWindowsAutoStart(enable); err != nil {
		return err
	}
	_, err := a.commitConfigMutation(func(current *types.AppConfig) {
		current.WindowsAutoStart = enable
	}, nil)
	return err
}

// GetDebugInfo 获取调试信息
func (a *CoreApp) GetDebugInfo() map[string]any {
	runtimeSnapshot := a.deviceRuntimeSnapshot()
	info := map[string]any{
		"debugMode":               a.debugMode,
		"trayReady":               a.trayManager.IsReady(),
		"trayInitialized":         a.trayManager.IsInitialized(),
		"isConnected":             runtimeSnapshot.Connected,
		"runtimeState":            runtimeSnapshot.Runtime,
		"autoReconnectSuppressed": a.autoReconnectSuppressed.Load(),
		"legionFnQSupported":      a.legionFnQSupported.Load(),
		"guiLastResponse":         time.Unix(atomic.LoadInt64(&a.guiLastResponse), 0).Format("2006-01-02 15:04:05"),
		"monitoringTemp":          a.monitoringTemp.Load(),
		"autoStartLaunch":         a.isAutoStartLaunch,
		"hasGUIClients":           a.ipcServer != nil && a.ipcServer.HasClients(),
		"pawnIOInstallerPath":     appmeta.FirstExistingPath(appmeta.PawnIOInstallerCandidates(config.GetInstallDir())),
		"runtime":                 runtimeDebugInfo(),
	}
	if a.pluginManager != nil {
		info["plugins"] = a.pluginManager.Statuses()
	}
	// 黑鲨固件版本检查结果（只读缓存，不在此处发起网络或设备查询）。
	// 结果里始终带 updateMethod="official-tool"，前端据此提示走官方工具升级。
	if status, ok := a.deviceManager.CachedBlackSharkFirmwareStatus(); ok {
		info["blackSharkFirmware"] = status
	}
	return info
}

// CheckBlackSharkFirmwareUpdate 重新检查黑鲨散热器固件版本。
// 只做版本比对，不下载固件、不进入 BOOT 模式、不写任何固件数据。
func (a *CoreApp) CheckBlackSharkFirmwareUpdate() types.BlackSharkFirmwareStatus {
	status := a.deviceManager.RefreshBlackSharkFirmwareStatus(context.Background())
	a.rememberBlackSharkFirmwareStatus(status)
	return status
}

// rememberBlackSharkFirmwareStatus 把一次**成功**的检查结果写进配置，供重启后直接显示。
// 失败的结果不落盘：厂商清单拿不到（网络问题）不该把上一次的好结果盖掉。
func (a *CoreApp) rememberBlackSharkFirmwareStatus(status types.BlackSharkFirmwareStatus) {
	if a == nil || a.configManager == nil {
		return
	}
	if strings.TrimSpace(status.CurrentVersion) == "" || strings.TrimSpace(status.Error) != "" {
		return
	}
	a.safeGo("rememberBlackSharkFirmwareStatus", func() {
		if _, err := a.configManager.MutateAndSave(func(cfg *types.AppConfig) {
			snapshot := status
			cfg.BlackSharkFirmwareCache = &snapshot
		}); err != nil {
			a.logDebug("缓存固件检查结果失败（只是下次要多查一次）: %v", err)
		}
	})
}

// GetBlackSharkFirmwareStatus 读取缓存的固件检查结果，不触发刷新。
// 设备侧那份缓存只在内存里、重启即失，所以再回落到配置里落盘的那份 ——
// 面板不必等用户手点一次"检查更新"才有内容。
func (a *CoreApp) GetBlackSharkFirmwareStatus() types.BlackSharkFirmwareStatus {
	if a.deviceManager != nil {
		if status, ok := a.deviceManager.CachedBlackSharkFirmwareStatus(); ok {
			return status
		}
	}
	if a.configManager != nil {
		if cached := a.configManager.Get().BlackSharkFirmwareCache; cached != nil {
			return *cached
		}
	}
	return types.BlackSharkFirmwareStatus{
		Supported:    false,
		UpdateMethod: "official-tool",
	}
}

// 黑鲨开关类能力：这些方法只在黑鲨设备连接时有效，其他设备返回 false。

// 以下方法把后端已实现的黑鲨能力包装成 CoreApp 方法，否则前端无法通过 Wails 调用。

// blackSharkRgbLighting 聚合灯效页信息：开关状态 + 当前模式 + 全部 8 个模式参数。
func (a *CoreApp) blackSharkRgbLighting() types.BlackSharkRgbLighting {
	out := a.readBlackSharkRgbLighting()
	if len(out.Modes) > 0 {
		// 读到了 ⇒ 立刻用新的覆盖上次读到的那份。
		a.rememberBlackSharkRgbCache(out)
		return out
	}
	// 没读到 ⇒ 回落到上次读到的那份，并明确标出来。
	// 只在读失败时回落：读到新数据时上面已经覆盖，绝不会让旧的盖住新的。
	if cached, at, ok := a.cachedBlackSharkRgbLighting(); ok {
		cached.FromCache = true
		cached.CachedAtUnix = at
		if cached.Error == "" {
			cached.Error = out.Error
		}
		return cached
	}
	return out
}

// rememberBlackSharkRgbCache 把刚读到的这一份异步落盘。
func (a *CoreApp) rememberBlackSharkRgbCache(snapshot types.BlackSharkRgbLighting) {
	if a == nil || a.configManager == nil {
		return
	}
	snapshot.FromCache = false
	snapshot.CachedAtUnix = 0
	at := time.Now().Unix()
	a.safeGo("rememberBlackSharkRgbCache", func() {
		if _, err := a.configManager.MutateAndSave(func(cfg *types.AppConfig) {
			copy := snapshot
			cfg.BlackSharkRgbCache = &copy
			cfg.BlackSharkRgbCacheAtUnix = at
		}); err != nil {
			a.logDebug("缓存灯效状态失败（只是下次要多读一次）: %v", err)
		}
	})
}

// blackSharkRgbCached 只返回本机缓存的那份灯效状态，一次设备 IO 都不做。
// 打开面板时先把上次读到的那份显示出来。
func (a *CoreApp) blackSharkRgbCached() types.BlackSharkRgbLighting {
	cached, at, ok := a.cachedBlackSharkRgbLighting()
	if !ok {
		return types.BlackSharkRgbLighting{
			ColorSettable: true,
			Error:         "本机还没有缓存过灯效状态，请点「读取灯效」",
		}
	}
	cached.FromCache = true
	cached.CachedAtUnix = at
	cached.Error = ""
	return cached
}

// cachedBlackSharkRgbLighting 取上次读到的那一份缓存。
func (a *CoreApp) cachedBlackSharkRgbLighting() (types.BlackSharkRgbLighting, int64, bool) {
	if a == nil || a.configManager == nil {
		return types.BlackSharkRgbLighting{}, 0, false
	}
	cfg := a.configManager.Get()
	if cfg.BlackSharkRgbCache == nil || len(cfg.BlackSharkRgbCache.Modes) == 0 {
		return types.BlackSharkRgbLighting{}, 0, false
	}
	return *cfg.BlackSharkRgbCache, cfg.BlackSharkRgbCacheAtUnix, true
}

// readBlackSharkRgbLighting 真正向设备读一遍灯效状态。
func (a *CoreApp) readBlackSharkRgbLighting() types.BlackSharkRgbLighting {
	out := types.BlackSharkRgbLighting{ColorSettable: true}
	if !a.deviceManager.IsBlackSharkActive() {
		out.Error = "当前设备不是黑鲨散热器"
		return out
	}
	out.Available = true
	out.Count = deviceproto.BlackSharkRgbEffectCount

	// 颜色下拉的选项表从 deviceproto 带出（协议事实的唯一所有者），
	// 前端只负责渲染，不再自己维护一份序号/名字。
	for _, idx := range deviceproto.BlackSharkRgbColorOptionOrder {
		out.ColorOptions = append(out.ColorOptions, types.BlackSharkRgbColorOptionView{
			Index: idx,
			Name:  deviceproto.BlackSharkRgbColorOptionNames[idx],
		})
	}

	switches := a.deviceManager.BlackSharkSwitchStates()
	out.SwitchEnabled = switches.LightingEnabled
	out.SwitchKnown = switches.LightingKnown

	if current, ok := a.deviceManager.BlackSharkCurrentRgbMode(); ok {
		out.CurrentIndex = current.Index
	}
	modes, ok := a.deviceManager.BlackSharkRgbModes()
	if !ok {
		out.Error = "读取灯效模式参数失败（设备无响应）"
		return out
	}
	for i := range modes {
		modes[i].Current = modes[i].Known && modes[i].Index == out.CurrentIndex
	}
	out.Modes = modes
	return out
}

// SetDebugMode 切换调试模式并同步配置与日志级别。
func (a *CoreApp) SetDebugMode(enabled bool) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	cfg.DebugMode = enabled
	// 同其它定长点：用插值依据，否则黑鲨下 LearnedOffsets 会被截成 4 点。
	debugUnit := a.activeDeviceSpeedUnit(&cfg)
	cfg.SmartControl, _ = smartcontrol.NormalizeConfigForUnit(cfg.SmartControl, a.smartControlCurveForUnit(&cfg, debugUnit), enabled, debugUnit)
	return a.commitConfigUpdate(cfg, func(types.AppConfig) {
		a.debugMode = enabled
		if a.logger != nil {
			a.logger.SetDebugMode(enabled)
			if enabled {
				a.logger.Info("调试模式已开启，后续日志将包含调试级别")
			} else {
				a.logger.Info("调试模式已关闭，调试级别日志将被忽略")
			}
		}
	})
}

func (a *CoreApp) SendDeviceDebugCommand(hexCommand string, waitMs int) (types.DeviceDebugCommandResult, error) {
	if !a.debugMode {
		return types.DeviceDebugCommandResult{}, fmt.Errorf("请先开启调试模式")
	}
	return a.deviceManager.SendDebugCommand(hexCommand, waitMs)
}

func (a *CoreApp) GetDeviceDebugFrames() []types.DeviceDebugFrame {
	return a.deviceManager.GetDebugFrames()
}

func (a *CoreApp) SetAutoStartWithMethod(enable bool, method string) error {
	if err := a.autostartManager.SetAutoStartWithMethod(enable, method); err != nil {
		return err
	}
	a.syncWindowsAutoStartConfig(enable)
	return nil
}

func (a *CoreApp) CheckWindowsAutoStart() bool {
	enabled := a.autostartManager.CheckWindowsAutoStart()
	a.syncWindowsAutoStartConfig(enabled)
	return enabled
}

func (a *CoreApp) syncWindowsAutoStartConfig(enabled bool) {
	cfg := a.configManager.Get()
	if cfg.WindowsAutoStart == enabled {
		return
	}
	cfg.WindowsAutoStart = enabled
	if err := a.commitConfigUpdate(cfg, nil); err != nil {
		a.logError("sync Windows auto-start config failed: %v", err)
	}
}
