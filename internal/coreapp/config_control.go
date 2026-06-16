package coreapp

import (
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/curveprofiles"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/smartcontrol"
	"github.com/TIANLI0/THRM/internal/types"
)

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

// UpdateConfig 更新配置
func (a *CoreApp) UpdateConfig(cfg types.AppConfig) error {
	a.mutex.Lock()

	oldCfg := a.configManager.Get()
	oldConnectionKey := deviceProfileConnectionKey(oldCfg)
	wasConnected := a.isConnected
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
	cfg.ManualGearLevels = cloneManualGearLevels(oldCfg.ManualGearLevels)
	cfg.LightStrip, _ = normalizeLightStripConfig(cfg.LightStrip)
	cfg.ThemeMode = types.NormalizeThemeMode(cfg.ThemeMode)
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
	prepareDeviceFanCurveStateForUpdate(&cfg, oldCfg)
	curveprofiles.NormalizeConfigForUnit(&cfg, unit)
	if idx := curveprofiles.FindIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID); idx >= 0 {
		cfg.FanCurveProfiles[idx].Curve = curveprofiles.CloneCurve(cfg.FanCurve)
	}
	cfg.SmartControl, _ = smartcontrol.NormalizeConfigForUnit(cfg.SmartControl, cfg.FanCurve, cfg.DebugMode, unit)
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
	if err := a.configManager.Update(cfg); err != nil {
		a.mutex.Unlock()
		return err
	}
	a.configureDeviceManager(cfg)
	connectionChanged := wasConnected && oldConnectionKey != deviceProfileConnectionKey(cfg)
	if connectionChanged {
		a.isConnected = false
		a.deviceSettings = nil
		a.autoReconnectSuppressed.Store(true)
	}
	a.syncManualGearLevelMemoryLocked(cfg)
	a.applyHotkeyBindings(cfg)
	a.applyPluginConfig(cfg)
	a.mutex.Unlock()

	if connectionChanged {
		a.deviceManager.DisconnectSilently()
		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
		}
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return nil
}

func (a *CoreApp) SetTemperatureHistoryEnabled(enabled bool) error {
	if err := a.tempHistory.SetEnabled(enabled); err != nil {
		return err
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
	cfg.SmartControl, _ = smartcontrol.NormalizeConfigForUnit(cfg.SmartControl, cfg.FanCurve, cfg.DebugMode, unit)
	runtimeDeviceKey := a.activeDeviceCurveScopeKey(cfg)
	storeSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey)
	storeDeviceFanCurveStateForKeyAndUnit(&cfg, runtimeDeviceKey, cfg, unit)
	if err := a.configManager.Update(cfg); err != nil {
		return err
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return nil
}

// ResetLearnedOffsets 清空学习到的曲线偏移。
func (a *CoreApp) ResetLearnedOffsets() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	cfg.SmartControl = smartcontrol.ResetLearnedState(cfg.SmartControl, cfg.FanCurve)
	resetSmartControlOffsetsForDeviceKey(&cfg, a.activeDeviceCurveScopeKey(cfg))
	if err := a.configManager.Update(cfg); err != nil {
		return err
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	a.logInfo("已重置学习偏移")
	return nil
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

	if enabled {
		a.userSetAutoControl = true
	}
	applyManualAfterDisable := !enabled && a.isConnected

	a.configManager.Set(cfg)
	err := a.configManager.Save()
	a.mutex.Unlock()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	if applyManualAfterDisable {
		if applyErr := a.applyCurrentGearSetting(); applyErr != nil {
			return applyErr
		}
	}

	return err
}

// applyCurrentGearSetting 应用当前挡位设置
func (a *CoreApp) applyCurrentGearSetting() error {
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
	if !a.deviceManager.SetManualGearRPM(setGear, level, rpm) {
		return fmt.Errorf("手动挡位下发失败")
	}
	return nil
}

// SetManualGear 设置手动挡位
func (a *CoreApp) SetManualGear(gear, level string) bool {
	cfg := a.configManager.Get()
	cfg.AutoControl = false
	cfg.CustomSpeedEnabled = false
	cfg.ManualGear = gear
	cfg.ManualLevel = level
	if cfg.ManualGearLevels == nil {
		cfg.ManualGearLevels = map[string]string{}
	}
	cfg.ManualGearLevels[gear] = normalizeManualLevel(level)
	unit := a.activeDeviceSpeedUnit(&cfg)
	types.NormalizeManualGearRPMForUnit(&cfg, unit)
	rpm := cfg.ResolveGearRPM(gear, level)
	a.configManager.Update(cfg)
	a.rememberManualGearLevel(gear, level)

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	return a.deviceManager.SetManualGearRPM(gear, level, rpm)
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
		if !a.deviceManager.SetTargetSpeed(configSpeedToTargetUnit(rpm, unit), unit) {
			return fmt.Errorf("当前设备拒绝自定义速度下发，请确认设备仍已连接并支持 %s 控制", types.FanSpeedDisplaySuffix(unit))
		}
	}

	a.mutex.Lock()
	a.configManager.Set(cfg)
	err := a.configManager.Save()
	a.mutex.Unlock()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	if applyManualAfterDisable {
		a.safeGo("applyCurrentGearSettingAfterCustomSpeed", func() {
			if err := a.applyCurrentGearSetting(); err != nil {
				a.logError("应用当前挡位设置失败: %v", err)
			}
		})
	}

	return err
}

// SetGearLight 设置挡位灯
func (a *CoreApp) SetGearLight(enabled bool) bool {
	if !a.activeDeviceCapabilities().AllowsGearLight() {
		return false
	}
	if !a.deviceManager.SetGearLight(enabled) {
		return false
	}

	cfg := a.configManager.Get()
	cfg.GearLight = enabled
	a.configManager.Update(cfg)

	// 广播配置更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
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

	cfg := a.configManager.Get()
	cfg.PowerOnStart = enabled
	a.configManager.Update(cfg)

	// 广播配置更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return true
}

// SetSmartStartStop 设置智能启停
func (a *CoreApp) SetSmartStartStop(mode string) bool {
	if !a.activeDeviceCapabilities().SupportsSmartStartStop {
		return false
	}
	if !a.deviceManager.SetSmartStartStop(mode) {
		return false
	}

	cfg := a.configManager.Get()
	cfg.SmartStartStop = mode
	a.configManager.Update(cfg)

	// 广播配置更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
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

	cfg := a.configManager.Get()
	cfg.Brightness = percentage
	a.configManager.Update(cfg)

	// 广播配置更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return true
}

// SetLightStrip 设置灯带
func (a *CoreApp) SetLightStrip(lightCfg types.LightStripConfig) error {
	if !a.activeDeviceCapabilities().AllowsLightStrip() {
		return fmt.Errorf("active device does not support lighting")
	}
	lightCfg, _ = normalizeLightStripConfig(lightCfg)

	cfg := a.configManager.Get()
	cfg.LightStrip = lightCfg
	a.configManager.Set(cfg)
	if err := a.configManager.Save(); err != nil {
		return err
	}

	if a.isConnected {
		if err := a.deviceManager.SetLightStrip(lightCfg); err != nil {
			return err
		}
	}

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	return nil
}

func (a *CoreApp) applyConfiguredLightStrip() error {
	if !a.activeDeviceCapabilities().AllowsLightStrip() {
		return nil
	}
	cfg := a.configManager.Get()
	lightCfg, changed := normalizeLightStripConfig(cfg.LightStrip)

	if changed {
		cfg.LightStrip = lightCfg
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("保存灯带默认配置失败: %v", err)
		}
	}

	return a.deviceManager.SetLightStrip(lightCfg)
}

func normalizeLightStripConfig(cfg types.LightStripConfig) (types.LightStripConfig, bool) {
	defaults := types.GetDefaultLightStripConfig()
	changed := false

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
	err := a.autostartManager.SetWindowsAutoStart(enable)
	if err == nil {
		cfg := a.configManager.Get()
		cfg.WindowsAutoStart = enable
		a.configManager.Update(cfg)

		// 广播配置更新
		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
		}
	}
	return err
}

// GetDebugInfo 获取调试信息
func (a *CoreApp) GetDebugInfo() map[string]any {
	info := map[string]any{
		"debugMode":               a.debugMode,
		"trayReady":               a.trayManager.IsReady(),
		"trayInitialized":         a.trayManager.IsInitialized(),
		"isConnected":             a.isConnected,
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
	return info
}

// SetDebugMode 设置调试模式
func (a *CoreApp) SetDebugMode(enabled bool) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	cfg.DebugMode = enabled
	cfg.SmartControl, _ = smartcontrol.NormalizeConfigForUnit(cfg.SmartControl, cfg.FanCurve, enabled, a.activeDeviceSpeedUnit(&cfg))
	a.debugMode = enabled

	if a.logger != nil {
		a.logger.SetDebugMode(enabled)
		if enabled {
			a.logger.Info("调试模式已开启，后续日志将包含调试级别")
		} else {
			a.logger.Info("调试模式已关闭，调试级别日志将被忽略")
		}
	}

	a.configManager.Set(cfg)
	if err := a.configManager.Save(); err != nil {
		return err
	}

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	return nil
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
