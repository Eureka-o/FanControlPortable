package coreapp

import (
	"fmt"
	"strings"

	"github.com/TIANLI0/THRM/internal/deviceprofiles"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

type deviceConnectionFlow struct {
	app *CoreApp
}

func newDeviceConnectionFlow(app *CoreApp) deviceConnectionFlow {
	return deviceConnectionFlow{app: app}
}

func (f deviceConnectionFlow) connectBestScannedDevice() bool {
	scan := f.app.ScanDeviceCandidates(types.DeviceScanModeNormal)
	if len(scan.Devices) == 0 {
		f.broadcastError("未发现可连接的设备")
		return false
	}
	if len(scan.Devices) > 1 {
		f.broadcastError(fmt.Sprintf("发现多个设备（%d 个），请到设置页选择", len(scan.Devices)))
		return false
	}
	device := scan.Devices[0]
	return f.connectCandidate(types.DeviceConnectRequest{
		ID:        device.ID,
		Transport: device.Transport,
		ProfileID: device.ProfileID,
		Endpoint:  device.Endpoint,
	})
}

func (f deviceConnectionFlow) connectCandidate(req types.DeviceConnectRequest) bool {
	transport := candidateTransport(req.Transport)
	switch transport {
	case types.DeviceTransportWiFi, types.DeviceTransportSerial:
		return f.connectCompatibilityCandidate(transport, req.ProfileID, req.Endpoint)
	case types.DeviceTransportHID, types.DeviceTransportBLE:
		return f.connectNativeCandidate(transport, req.ProfileID, req.Endpoint)
	default:
		f.broadcastError("缺少可连接的设备信息")
		return false
	}
}

func (f deviceConnectionFlow) connectNativeDevice(profileID string) bool {
	f.app.autoReconnectSuppressed.Store(false)
	cfg := f.app.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)

	f.disconnectForSwitch()

	if profile, ok := nativeConnectProfileByID(cfg, profileID); ok {
		return f.connectNativeProfile(profile)
	}

	f.app.configureDeviceManager(cfg)
	success, deviceInfo := f.app.deviceManager.AutoConnectNativeProfiles(cfg.DeviceProfiles)
	if success {
		f.app.finishSuccessfulDeviceConnection(deviceInfo, "ConnectNativeDevice")
		return true
	}
	f.broadcastError("未发现可自动识别的原生设备")
	return false
}

func (f deviceConnectionFlow) connectNativeCandidate(transport, profileID, endpoint string) bool {
	f.app.autoReconnectSuppressed.Store(false)
	cfg := f.app.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)
	profile, ok := nativeConnectProfileByID(cfg, profileID)
	if !ok {
		return f.connectNativeDevice(profileID)
	}
	if types.NormalizeDeviceTransport(profile.Transport) != transport {
		f.broadcastError("设备模板与扫描结果不匹配")
		return false
	}
	if transport == types.DeviceTransportBLE {
		profile.Connection.Endpoint = strings.TrimSpace(endpoint)
	}
	f.disconnectForSwitch()
	return f.connectNativeProfile(profile)
}

func (f deviceConnectionFlow) connectNativeProfile(profile types.DeviceProfile) bool {
	success, deviceInfo := f.app.deviceManager.ConnectNativeProfile(profile)
	if success {
		f.app.finishSuccessfulDeviceConnection(deviceInfo, "ConnectNativeDevice")
		return true
	}
	f.broadcastError("未发现指定的原生设备")
	return false
}

func (f deviceConnectionFlow) connectCompatibilityCandidate(transport, profileID, endpoint string) bool {
	f.app.autoReconnectSuppressed.Store(false)

	oldCfg := f.app.configManager.Get()
	types.NormalizeDeviceProfileConfig(&oldCfg)
	nextCfg := oldCfg

	idx := compatibilityProfileIndex(nextCfg, transport, profileID)
	if idx < 0 && transport == types.DeviceTransportWiFi {
		nextCfg.DeviceProfiles = append(nextCfg.DeviceProfiles, types.DefaultWiFiPercentProfile(nextCfg.FanControlDeviceIp))
		idx = len(nextCfg.DeviceProfiles) - 1
	}
	if idx < 0 {
		f.broadcastError("未找到可连接的兼容设备")
		return false
	}

	profile := types.NormalizeDeviceProfile(nextCfg.DeviceProfiles[idx], nextCfg.FanControlDeviceIp)
	endpoint = strings.TrimSpace(endpoint)
	if endpoint != "" {
		switch transport {
		case types.DeviceTransportWiFi:
			profile.Connection.Endpoint = endpoint
			nextCfg.FanControlDeviceIp = endpoint
		case types.DeviceTransportSerial:
			profile.Connection.SerialPort = endpoint
		}
	}
	nextCfg.DeviceProfiles[idx] = profile
	nextCfg.ActiveDeviceProfileID = profile.ID
	nextCfg.DeviceTransport = transport
	if nextCfg.ActiveDeviceProfileIDsByTransport == nil {
		nextCfg.ActiveDeviceProfileIDsByTransport = map[string]string{}
	}
	nextCfg.ActiveDeviceProfileIDsByTransport[transport] = profile.ID
	if transport == types.DeviceTransportWiFi {
		nextCfg.WiFiCompatibilityEnabled = true
	} else {
		nextCfg.SerialCompatibilityEnabled = true
	}
	types.NormalizeDeviceProfileConfig(&nextCfg)

	f.disconnectForSwitch()
	f.app.configureDeviceManager(nextCfg)
	success, deviceInfo := f.app.deviceManager.Connect()
	if !success {
		f.app.configureDeviceManager(oldCfg)
		f.broadcastError("连接失败")
		return false
	}

	if err := f.app.configManager.Update(nextCfg); err != nil {
		f.app.logError("保存设备连接配置失败: %v", err)
		f.broadcastError(err.Error())
		return false
	}
	if f.app.ipcServer != nil {
		f.app.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, nextCfg)
	}
	f.app.lastConnectionWasNative.Store(false)
	f.app.finishSuccessfulDeviceConnection(deviceInfo, "ConnectDeviceCandidate")
	return true
}

func (f deviceConnectionFlow) disconnectForSwitch() {
	if !f.app.deviceManager.IsConnected() {
		return
	}
	f.app.deviceManager.DisconnectSilently()
	f.app.mutex.Lock()
	f.app.isConnected = false
	f.app.deviceSettings = nil
	f.app.mutex.Unlock()
	if f.app.ipcServer != nil {
		f.app.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}
}

func (f deviceConnectionFlow) broadcastError(message string) {
	if f.app.ipcServer != nil {
		f.app.ipcServer.BroadcastEvent(ipc.EventDeviceError, message)
	}
}

func candidateTransport(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case types.DeviceTransportWiFi:
		return types.DeviceTransportWiFi
	case types.DeviceTransportSerial:
		return types.DeviceTransportSerial
	case types.DeviceTransportHID:
		return types.DeviceTransportHID
	case types.DeviceTransportBLE:
		return types.DeviceTransportBLE
	default:
		return ""
	}
}

func nativeConnectProfileByID(cfg types.AppConfig, profileID string) (types.DeviceProfile, bool) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return types.DeviceProfile{}, false
	}
	for _, profile := range cfg.DeviceProfiles {
		if strings.TrimSpace(profile.ID) != profileID {
			continue
		}
		profile = types.NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp)
		return profile, types.IsNativeDeviceTransport(profile.Transport)
	}
	if profile, ok := deviceprofiles.BuiltInProfileByID(profileID); ok {
		profile = types.NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp)
		return profile, types.IsNativeDeviceTransport(profile.Transport)
	}
	return types.DeviceProfile{}, false
}

func compatibilityProfileIndex(cfg types.AppConfig, transport, profileID string) int {
	if strings.TrimSpace(profileID) != "" {
		if idx := deviceprofiles.FindIndex(cfg.DeviceProfiles, profileID); idx >= 0 {
			if types.NormalizeDeviceTransport(cfg.DeviceProfiles[idx].Transport) == transport {
				return idx
			}
		}
	}
	activeID := types.ActiveDeviceProfileIDForTransport(&cfg, transport)
	if idx := deviceprofiles.FindIndex(cfg.DeviceProfiles, activeID); idx >= 0 {
		return idx
	}
	for i := range cfg.DeviceProfiles {
		if types.NormalizeDeviceTransport(cfg.DeviceProfiles[i].Transport) == transport {
			return i
		}
	}
	return -1
}
