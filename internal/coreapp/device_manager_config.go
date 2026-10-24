package coreapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/device"
	"github.com/Eureka-o/FanControlPortable/internal/deviceprofiles"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func (a *CoreApp) configureDeviceManager(cfg types.AppConfig) {
	if a == nil || a.deviceManager == nil {
		return
	}
	profile := types.ActiveDeviceProfile(&cfg)
	if strings.TrimSpace(profile.ID) == "" && strings.TrimSpace(profile.Transport) == "" {
		a.deviceManager.ClearProfile()
		return
	}
	prepared, err := deviceprofiles.PrepareRuntimeProfile(profile, cfg.FanControlDeviceIp)
	if err != nil {
		a.deviceManager.ClearProfile()
		a.logError("设备档案校验失败，已跳过运行时配置: %v", err)
		return
	}
	a.deviceManager.ConfigureProfile(prepared, cfg.FanControlDeviceIp)
}

func (a *CoreApp) reconcileDeviceManagerProfile(cfg types.AppConfig) bool {
	if a == nil || a.deviceManager == nil {
		return false
	}

	a.connectMutex.Lock()
	defer a.connectMutex.Unlock()

	managerConnected := a.deviceManager.IsConnected()
	if managerConnected {
		runtimeProfile := a.deviceManager.ActiveProfile()
		if types.IsNativeDeviceTransport(runtimeProfile.Transport) {
			return false
		}
		if deviceProfileConnectionKeyForProfile(runtimeProfile, "") == deviceProfileConnectionKey(cfg) {
			return false
		}
		a.deviceManager.DisconnectSilently()
	}

	a.configureDeviceManager(cfg)

	wasCoreConnected := newDeviceConnectionFlow(a).setRuntimeDisconnected("profile-changed")
	return managerConnected || wasCoreConnected
}

func deviceProfileConnectionKey(cfg types.AppConfig) string {
	types.NormalizeDeviceProfileConfig(&cfg)
	profile := types.ActiveDeviceProfile(&cfg)
	return deviceProfileConnectionKeyForProfile(profile, cfg.FanControlDeviceIp)
}

func deviceProfileConnectionKeyForProfile(profile types.DeviceProfile, fallbackEndpoint string) string {
	if strings.TrimSpace(profile.ID) == "" && strings.TrimSpace(profile.Transport) == "" {
		return ""
	}
	profile = types.NormalizeDeviceProfile(profile, fallbackEndpoint)

	parts := []string{
		strings.TrimSpace(profile.ID),
		types.NormalizeDeviceTransport(profile.Transport),
		strings.TrimSpace(profile.Connection.Endpoint),
	}
	switch profile.Transport {
	case types.DeviceTransportBLE:
		parts = append(parts,
			strings.TrimSpace(profile.Connection.BLENameFilter),
			strings.TrimSpace(profile.Connection.BLEServiceUUID),
			strings.TrimSpace(profile.Connection.BLEWriteCharacteristic),
			strings.TrimSpace(profile.Connection.BLENotifyCharacteristic),
			fmt.Sprint(profile.Connection.BLEWriteWithResponse),
		)
	case types.DeviceTransportSerial:
		parts = append(parts,
			strings.TrimSpace(profile.Connection.SerialPort),
			fmt.Sprint(profile.Connection.SerialBaudRate),
			fmt.Sprint(profile.Connection.SerialDataBits),
			fmt.Sprint(profile.Connection.SerialStopBits),
			strings.TrimSpace(profile.Connection.SerialParity),
		)
	}
	return strings.Join(parts, "\x1f")
}

// rememberConnectedDeviceProfile 把这次连成功的档案写回配置，供下次启动优先尝试它。
// 适用范围只有兼容模式传输（WiFi / 虚拟串口）；原生传输的字段会被规范化抹掉，不写。
func (a *CoreApp) rememberConnectedDeviceProfile(profileID, transport string) {
	if a == nil || a.configManager == nil {
		return
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return
	}
	transport = types.NormalizeDeviceTransport(transport)

	// 原生传输写了也会被规范化抹掉（见上），所以不写：
	// 省掉一次无意义的配置落盘与提交回调，也避免打出一条「已记住」的假日志。
	if !types.IsManualCompatibilityDeviceTransport(transport) {
		a.logDebug("档案 %s 属于原生传输（%s），不写入 activeDeviceProfileId：该字段的所有者是兼容模式，"+
			"原生传输写进去会被 NormalizeDeviceProfileConfig 抹掉；原生连接的「下次更快」由 BLE 冷却负责",
			profileID, transport)
		return
	}

	cur := a.configManager.Get()
	if cur.ActiveDeviceProfileID == profileID &&
		(transport == "" || cur.ActiveDeviceProfileIDsByTransport[transport] == profileID) {
		return // 已经是它了，不写盘
	}

	if _, err := a.configManager.MutateAndSave(func(cfg *types.AppConfig) {
		cfg.ActiveDeviceProfileID = profileID
		if transport != "" {
			if cfg.ActiveDeviceProfileIDsByTransport == nil {
				cfg.ActiveDeviceProfileIDsByTransport = map[string]string{}
			}
			cfg.ActiveDeviceProfileIDsByTransport[transport] = profileID
		}
	}); err != nil {
		a.logDebug("记住设备档案失败（只是下次会慢一点，不影响本次连接）: %v", err)
		return
	}
	a.logInfo("已记住本次连接的设备档案 %s（%s）—— 下次启动将优先尝试它", profileID, transport)
}

// autoConnectNativeWithBLECooldown 是自动连接原生设备的唯一入口（上层都走这里）。
// 它只做一件事但很关键：把 device 层那个 15 分钟 BLE 冷却状态在进程之间接起来。
func (a *CoreApp) autoConnectNativeWithBLECooldown(ctx context.Context, profiles []types.DeviceProfile) (bool, map[string]string) {
	if a == nil || a.deviceManager == nil {
		return false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	a.seedNativeBLEAutoScanCooldown()
	success, info := a.deviceManager.AutoConnectNativeProfilesContext(ctx, profiles)
	a.persistNativeBLEAutoScanCooldown()
	return success, info
}

// seedNativeBLEAutoScanCooldown 把配置里记着的上次 BLE 扫描时间灌给 device 层。
func (a *CoreApp) seedNativeBLEAutoScanCooldown() {
	if a == nil || a.deviceManager == nil || a.configManager == nil {
		return
	}
	unix := a.configManager.Get().NativeAutoBLEScanUnix
	if unix <= 0 {
		return
	}
	a.deviceManager.SeedLastAutoBLEScanAt(time.Unix(unix, 0))
}

// persistNativeBLEAutoScanCooldown 把 device 层最新的上次 BLE 扫描时间落盘。
func (a *CoreApp) persistNativeBLEAutoScanCooldown() {
	if a == nil || a.deviceManager == nil || a.configManager == nil {
		return
	}
	at := a.deviceManager.LastAutoBLEScanAt()
	if at.IsZero() {
		return
	}
	unix := at.Unix()
	if a.configManager.Get().NativeAutoBLEScanUnix == unix {
		return // 没变，不写盘（避免无谓 IO 与配置提交回调）
	}
	if _, err := a.configManager.MutateAndSave(func(cfg *types.AppConfig) {
		cfg.NativeAutoBLEScanUnix = unix
	}); err != nil {
		a.logDebug("记录 BLE 扫描时间失败（只是下次启动会再扫一次，不影响本次连接）: %v", err)
		return
	}
	a.logInfo("已记录自动连接 BLE 扫描时间 %s —— %s 内重启将跳过 BLE 组（省掉一次注定失败的扫描）",
		at.Format("15:04:05"), device.IdleBLEScanCooldown())
}
