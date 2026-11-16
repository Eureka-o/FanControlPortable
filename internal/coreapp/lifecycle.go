package coreapp

import (
	"os"
	"strings"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/autostart"
	"github.com/Eureka-o/FanControlPortable/internal/config"
	"github.com/Eureka-o/FanControlPortable/internal/curveprofiles"
	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/powernotify"
	"github.com/Eureka-o/FanControlPortable/internal/smartcontrol"
	"github.com/Eureka-o/FanControlPortable/internal/tray"
	"github.com/Eureka-o/FanControlPortable/internal/types"
	"github.com/Eureka-o/FanControlPortable/internal/version"
)

func (a *CoreApp) trayConfigSnapshot() types.AppConfig {
	if a == nil || a.configManager == nil {
		return types.AppConfig{}
	}
	revision := a.configManager.Revision()
	a.trayConfigMu.Lock()
	defer a.trayConfigMu.Unlock()
	if !a.trayConfigReady || revision != a.trayConfigRev {
		a.refreshTrayConfigLocked()
	}
	return a.trayConfig
}

func (a *CoreApp) refreshTrayConfigLocked() {
	a.trayConfig, a.trayConfigRev = a.configManager.GetWithRevision()
	a.trayConfigReady = true
	a.trayCurveOptions = buildTrayCurveOptions(a.trayConfig.FanCurveProfiles)
	a.trayActiveCurveID = a.trayConfig.ActiveFanCurveProfileID
}

func (a *CoreApp) trayCurveOptionsSnapshot() ([]tray.CurveOption, string) {
	if a == nil || a.configManager == nil {
		return nil, ""
	}
	revision := a.configManager.Revision()
	a.trayConfigMu.Lock()
	defer a.trayConfigMu.Unlock()
	if !a.trayConfigReady || revision != a.trayConfigRev {
		a.refreshTrayConfigLocked()
	}
	return a.trayCurveOptions, a.trayActiveCurveID
}

func buildTrayCurveOptions(profiles []types.FanCurveProfile) []tray.CurveOption {
	options := make([]tray.CurveOption, 0, len(profiles))
	for _, profile := range profiles {
		if profile.ID == "" {
			continue
		}
		name := strings.TrimSpace(profile.Name)
		if name == "" {
			name = "默认"
		}
		options = append(options, tray.CurveOption{ID: profile.ID, Name: name})
	}
	return options
}

// Start 启动核心服务
func (a *CoreApp) Start() error {
	a.logInfo("=== FanControl Core 启动 ===")
	a.logInfo("版本: %s", version.Get())
	a.logInfo("安装目录: %s", config.GetInstallDir())
	a.logInfo("调试模式: %v", a.debugMode)
	a.logInfo("当前工作目录: %s", config.GetCurrentWorkingDir())

	// 检测是否为自启动
	a.isAutoStartLaunch = autostart.DetectAutoStartLaunch(os.Args)
	a.logInfo("自启动模式: %v", a.isAutoStartLaunch)

	// 加载配置
	a.logInfo("开始加载配置文件")
	cfg := a.configManager.Load(a.isAutoStartLaunch)
	configChanged := false
	if normalizedLight, changed := normalizeLightStripConfig(cfg.LightStrip); changed {
		cfg.LightStrip = normalizedLight
		configChanged = true
	}
	if normalizeHotkeyConfig(&cfg) {
		configChanged = true
	}
	unit := types.DeviceProfileSpeedUnit(&cfg)
	changedCurveState := syncDeviceFanCurveStateForStartup(&cfg)
	if curveprofiles.NormalizeConfigForUnit(&cfg, unit) {
		changedCurveState = true
	}
	if storeActiveDeviceFanCurveState(&cfg) {
		changedCurveState = true
	}
	if changedCurveState {
		configChanged = true
	}
	unit = types.DeviceProfileSpeedUnit(&cfg)
	// 与其它定长点统一：用插值依据（黑鲨四档方案补两端端点 ⇒ 5..6 点）。
	// 传 cfg.FanCurve（4 点）会让 LearnedOffsets 在这里先被截成 4，
	// 随后被 syncSmartControlOffsetsForActiveProfile（按有效曲线定长）改回 6 ⇒ 长度翻转。
	if normalizedSmart, changed := smartcontrol.NormalizeConfigForUnit(cfg.SmartControl, a.smartControlCurveInterp(&cfg), cfg.DebugMode, unit); changed {
		cfg.SmartControl = normalizedSmart
		configChanged = true
	}
	if syncSmartControlOffsetsForActiveProfile(&cfg) {
		configChanged = true
	}
	if normalizeManualGearMemoryConfig(&cfg) {
		configChanged = true
	}
	if types.NormalizeManualGearRPMForUnit(&cfg, startupManualGearSpeedUnit(cfg)) {
		configChanged = true
	}
	if a.applyCachedLegionFnQSupport(&cfg) {
		configChanged = true
	}

	// 将启动迁移和系统自启动状态合并为一次完整配置写入。
	if upgraded, err := a.autostartManager.EnsureAutoStartTaskHealthy(); err != nil {
		a.logError("升级自启动任务定义失败: %v", err)
	} else if upgraded {
		a.logInfo("已升级自启动任务定义")
	}
	a.logInfo("检查Windows自启动状态")
	actualAutoStart := a.autostartManager.CheckWindowsAutoStart()
	if actualAutoStart != cfg.WindowsAutoStart {
		cfg.WindowsAutoStart = actualAutoStart
		configChanged = true
		a.logInfo("已同步Windows自启动状态: %v", actualAutoStart)
	}
	if configChanged {
		if err := a.commitConfigUpdate(cfg, nil); err != nil {
			a.logError("保存启动归一化配置失败: %v", err)
		}
	}
	a.syncManualGearLevelMemory(cfg)
	a.configureDeviceManager(cfg)
	a.refreshMonitorOnlyRuntime(cfg.MonitorOnly)
	a.logInfo("配置加载完成，配置路径: %s", cfg.ConfigPath)

	// 同步调试模式配置
	if cfg.DebugMode {
		a.debugMode = true
		if a.logger != nil {
			a.logger.SetDebugMode(true)
		}
		a.logInfo("从配置文件同步调试模式: 启用")
	}

	if a.monitorOnlyActive() {
		a.logInfo("仅监控模式已启用，跳过HID库初始化")
	} else {
		// 初始化HID
		a.logInfo("初始化HID库")
		if err := a.deviceManager.Init(); err != nil {
			a.logError("初始化HID库失败: %v", err)
			return err
		}
		a.logInfo("HID库初始化成功")
	}

	// 设置设备回调
	a.deviceManager.SetCallbacks(a.onFanDataUpdate, a.onDeviceDisconnect)

	// 启动 IPC 服务器
	a.logInfo("启动 IPC 服务器")
	a.ipcServer = ipc.NewServer(a.handleIPCRequest, a.logger)
	if err := a.ipcServer.Start(); err != nil {
		a.logError("启动 IPC 服务器失败: %v", err)
		return err
	}
	if !a.monitorOnlyActive() && !a.legionFnQSupportChecked.Load() {
		a.startLegionFnQSupportDetection()
	}

	// 初始化系统托盘
	a.logInfo("开始初始化系统托盘")
	a.initSystemTray()
	a.applyHotkeyBindings(cfg)
	if !a.monitorOnlyActive() {
		a.applyPluginConfig(cfg)
	}

	// 注册系统睡眠/唤醒通知：睡眠前主动断开设备/桥接，唤醒后恢复，避免唤醒崩溃。
	if stop, err := powernotify.RegisterSuspendResumeNotifications(a.onSystemSuspend, a.onSystemResume); err != nil {
		a.logError("注册系统电源通知失败（将退化为基于时间间隔的唤醒检测）: %v", err)
	} else {
		a.powerNotifyStop = stop
		a.logInfo("已注册系统睡眠/唤醒通知")
	}
	if stop, err := powernotify.RegisterSessionEndNotifications(a.onSystemEndSession); err != nil {
		a.logError("注册系统关机通知失败: %v", err)
	} else {
		a.sessionEndStop = stop
		a.logInfo("已注册系统关机/注销通知")
	}
	if !a.monitorOnlyActive() {
		if stop, err := powernotify.RegisterHIDInterfaceArrivalNotifications(
			types.FlyDigiHIDVendorID,
			[]uint16{
				types.FlyDigiBS2ProductID,
				types.FlyDigiBS2PROProductID,
				types.FlyDigiBS3ProductID,
				types.FlyDigiBS3PROProductID,
			},
			a.onSupportedHIDArrival,
		); err != nil {
			a.logDebug("注册飞智 HID 到达通知失败，将继续使用分阶段重连: %v", err)
		} else {
			a.hidNotifyStop = stop
			a.logInfo("已注册飞智 HID 接口到达通知")
		}
		if stop, err := powernotify.RegisterBluetoothLEInterfaceArrivalNotifications(a.onSupportedBLEArrival); err != nil {
			a.logDebug("Bluetooth LE interface arrival notifications unavailable; using reconnect backoff: %v", err)
		} else {
			a.bleNotifyStop = stop
			a.logInfo("Bluetooth LE interface arrival notifications registered")
		}
		// 黑鲨 BRB02 走 WinUSB（批量端点），不触发上面的 HID 那条；它需要 USB 插上就接管（见 onBlackSharkUSBArrival）。
		if stop, err := powernotify.RegisterUSBDeviceInterfaceArrivalNotifications(a.onBlackSharkUSBArrival); err != nil {
			a.logDebug("注册 USB 设备接口到达通知失败，黑鲨 USB 接管将只靠健康检查兜底: %v", err)
		} else {
			a.usbNotifyStop = stop
			a.logInfo("已注册 USB 设备接口到达通知")
		}
	}

	// 健康循环还负责温度监控自愈；仅监控模式下设备健康检查会直接返回。
	a.logInfo("启动健康监控")
	a.safeGo("startHealthMonitoring", func() {
		a.startHealthMonitoring()
	})

	// 情景循环只在已有规则时启动；之后新增首条规则会按需启动。
	if len(cfg.SceneRules) > 0 {
		a.startSceneLoop()
	}

	// 黑鲨主机侧灯效驱动：槽位 6「响应」/ 槽位 7「音频同步」需要主机持续喂数据，
	// 因此跟着当前生效的灯效槽位自动起停（理由见 blackSharkHostFx）。
	if !a.monitorOnlyActive() {
		a.startBlackSharkHostFx()
	}

	a.logInfo("=== FanControl Core 启动完成 ===")

	// 软件启动后立即开始温度监控（与智能控温开关解耦）
	a.safeGo("startTemperatureMonitoring@Start", func() {
		a.startTemperatureMonitoring()
	})

	// 启动连接与健康检查共用可取消的 generation 重连链路。
	if a.monitorOnlyActive() {
		a.logInfo("仅监控模式已启用，跳过启动设备搜索")
	} else {
		a.requestStartupReconnect()
	}

	return nil
}

func (a *CoreApp) requestStartupReconnect() {
	delay := time.Second
	if a.isAutoStartLaunch {
		delay = 3 * time.Second
		a.logInfo("自启动模式：等待设备初始化（3秒）")
	}
	a.requestReconnect("startup", []time.Duration{delay, 5 * time.Second, 10 * time.Second, 30 * time.Second})
}

func startupManualGearSpeedUnit(cfg types.AppConfig) string {
	if compatibilityConnectionEnabled(cfg) {
		return types.DeviceProfileSpeedUnit(&cfg)
	}
	return types.FanSpeedUnitRPM
}

// Stop 停止核心服务
func (a *CoreApp) Stop() {
	if !a.stopping.CompareAndSwap(false, true) {
		return
	}
	a.logInfo("核心服务正在停止...")
	a.cancelNoiseDiagnosticLease("核心服务停止")
	if a.sessionEndStop != nil {
		a.safeRun("session-end-unregister", a.sessionEndStop)
		a.sessionEndStop = nil
	}
	if a.powerNotifyStop != nil {
		a.safeRun("power-notify-unregister", a.powerNotifyStop)
		a.powerNotifyStop = nil
	}
	if a.hidNotifyStop != nil {
		a.safeRun("hid-notify-unregister", a.hidNotifyStop)
		a.hidNotifyStop = nil
	}
	if a.bleNotifyStop != nil {
		a.safeRun("ble-notify-unregister", a.bleNotifyStop)
		a.bleNotifyStop = nil
	}
	if a.usbNotifyStop != nil {
		a.safeRun("usb-notify-unregister", a.usbNotifyStop)
		a.usbNotifyStop = nil
	}
	a.stopTemperatureMonitoring()
	a.monitoringMutex.Lock()
	monitoringDone := a.monitoringDone
	a.monitoringMutex.Unlock()
	if monitoringDone != nil {
		select {
		case <-monitoringDone:
		case <-time.After(3 * time.Second):
			a.logError("等待温度监控停止超时，继续退出")
		}
	}
	a.returnFlyDigiToFirmwareOnStop()
	if a.tempHistory != nil {
		if err := a.tempHistory.Flush(); err != nil {
			a.logError("退出时保存温度历史失败: %v", err)
		}
	}
	if a.hotkeyManager != nil {
		a.hotkeyManager.Stop()
	}
	if a.pluginManager != nil {
		a.pluginManager.StopAll()
	}

	// 清理资源
	a.cleanup()

	// 停止所有监控
	a.DisconnectDevice()

	// 停止桥接程序
	a.bridgeManager.Stop()

	// 停止 IPC 服务器
	if a.ipcServer != nil {
		a.ipcServer.Stop()
	}

	// 停止托盘
	a.trayManager.Quit()

	a.logInfo("核心服务已停止")
	if a.logger != nil {
		a.logger.Close()
	}
}

func (a *CoreApp) returnFlyDigiToFirmwareOnStop() {
	if a.deviceManager == nil || a.configManager == nil {
		return
	}
	cfg := a.configManager.Get()
	profile := a.deviceManager.ActiveProfile()
	if !shouldReturnFlyDigiToFirmwareOnStop(cfg, profile, a.deviceManager.IsConnected()) {
		return
	}
	if a.deviceManager.ExitFlyDigiRealtimeRPM() {
		a.logInfo("飞智实时转速已退出，设备固件接管智能启停")
	}
}

func shouldReturnFlyDigiToFirmwareOnStop(cfg types.AppConfig, profile types.DeviceProfile, connected bool) bool {
	mode := strings.ToLower(strings.TrimSpace(cfg.SmartStartStop))
	if !connected || (mode != "immediate" && mode != "delayed") {
		return false
	}
	return types.NormalizeDeviceTransport(profile.Transport) == types.DeviceTransportHID && profile.Capabilities.SupportsSmartStartStop
}

func (a *CoreApp) onSystemEndSession() {
	a.logInfo("收到 Windows 会话结束通知")
	select {
	case a.quitChan <- true:
	default:
	}
}

// initSystemTray 初始化系统托盘
func (a *CoreApp) initSystemTray() {
	a.trayManager.SetCallbacks(
		a.onShowWindowRequest,
		a.onQuitRequest,
		func() bool {
			cfg := a.configManager.Get()
			newState := !cfg.AutoControl
			a.SetAutoControl(newState)
			return newState
		},
		func(profileID string) string {
			profile, err := a.SetActiveFanCurveProfile(profileID)
			if err != nil {
				a.logError("托盘设置温控曲线失败: %v", err)
				return ""
			}
			return profile.Name
		},
		func() ([]tray.CurveOption, string) {
			return a.trayCurveOptionsSnapshot()
		},
		func() tray.Status {
			a.mutex.RLock()
			defer a.mutex.RUnlock()
			cfg := a.trayConfigSnapshot()
			fanData := a.deviceManager.GetCurrentFanData()
			var currentRPM uint16
			speedUnit := a.activeDeviceSpeedUnit(&cfg)
			if fanData != nil {
				currentRPM = fanData.CurrentRPM
				if strings.TrimSpace(fanData.SpeedUnit) != "" {
					speedUnit = fanData.SpeedUnit
				}
			}
			speedUnit = types.NormalizeFanSpeedUnit(speedUnit)
			activeProfile := a.deviceManager.ActiveProfile()
			deviceName := strings.TrimSpace(activeProfile.DisplayName)
			if deviceName == "" {
				deviceName = strings.TrimSpace(activeProfile.Model)
			}
			if deviceName == "" {
				configProfile := types.ActiveDeviceProfile(&cfg)
				deviceName = strings.TrimSpace(configProfile.DisplayName)
				if deviceName == "" {
					deviceName = strings.TrimSpace(configProfile.Model)
				}
			}
			curveOptions, _ := a.trayCurveOptionsSnapshot()
			cpuPowerWatts := a.currentTemp.CPUPowerWatts
			gpuPowerWatts := a.currentTemp.GPUPowerWatts
			if cfg.PowerSpoofEnabled {
				cpuPowerWatts = types.SpoofDisplayedPower(cpuPowerWatts, cfg.CPUPowerSpoofPercent, cfg.CPUPowerSpoofOffsetWatts)
				gpuPowerWatts = types.SpoofDisplayedPower(gpuPowerWatts, cfg.GPUPowerSpoofPercent, cfg.GPUPowerSpoofOffsetWatts)
			}

			return tray.Status{
				Connected:            a.isConnected,
				MonitorOnly:          a.monitorOnlyActive(),
				DeviceName:           deviceName,
				CPUTemp:              a.currentTemp.CPUTemp,
				GPUTemp:              a.currentTemp.GPUTemp,
				CPUPowerWatts:        cpuPowerWatts,
				GPUPowerWatts:        gpuPowerWatts,
				GPUReadState:         a.currentTemp.GPUReadState,
				CurrentRPM:           currentRPM,
				SpeedUnit:            speedUnit,
				AutoControlState:     cfg.AutoControl,
				ActiveCurveProfileID: cfg.ActiveFanCurveProfileID,
				CurveProfiles:        curveOptions,
			}
		},
	)
	// 自启动场景下延时注册托盘：等待任务栏通知区域稳定后再注册，避免开机快速启动时图标丢失。
	a.trayManager.SetAutoStartLaunch(a.isAutoStartLaunch)
	a.trayManager.Init()
}

// cleanup 清理资源
func (a *CoreApp) cleanup() {
	if a.healthCheckTicker != nil {
		a.healthCheckTicker.Stop()
	}
	if a.sceneTicker != nil {
		a.sceneTicker.Stop()
	}
	// 必须先停主机侧灯效驱动：它会装上全局键鼠钩子并占用系统音频采集，
	// 进程退出前不卸干净，下次启动就可能装不上钩子。
	a.stopBlackSharkHostFx()

	select {
	case a.cleanupChan <- true:
	default:
	}

}

// 日志辅助方法
func (a *CoreApp) logInfo(format string, v ...any) {
	if a.logger != nil {
		a.logger.Info(format, v...)
	}
}

func (a *CoreApp) logError(format string, v ...any) {
	if a.logger != nil {
		a.logger.Error(format, v...)
	}
}

func (a *CoreApp) logDebug(format string, v ...any) {
	if a.logger != nil {
		a.logger.Debug(format, v...)
	}
}

func (a *CoreApp) safeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				CapturePanic(a, "goroutine:"+name, r)
			}
		}()

		fn()
	}()
}

// QuitChan exposes the internal quit signal for the thin cmd/core entrypoint.
func (a *CoreApp) QuitChan() <-chan bool {
	return a.quitChan
}

// LogInfo keeps logging available to the thin cmd/core entrypoint without exposing internals.
func (a *CoreApp) LogInfo(format string, v ...any) {
	a.logInfo(format, v...)
}
