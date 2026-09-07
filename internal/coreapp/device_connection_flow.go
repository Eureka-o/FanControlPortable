package coreapp

import (
	"fmt"
	"strings"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/deviceprofiles"
	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

type deviceConnectionFlow struct {
	app *CoreApp
}

func newDeviceConnectionFlow(app *CoreApp) deviceConnectionFlow {
	return deviceConnectionFlow{app: app}
}

func (f deviceConnectionFlow) enterPhase(phase int32) func() {
	if f.app == nil {
		return func() {}
	}
	previous := f.app.connectionPhase.Swap(phase)
	return func() { f.app.connectionPhase.Store(previous) }
}

func (f deviceConnectionFlow) setRuntimeConnected() {
	f.app.mutex.Lock()
	f.app.isConnected = true
	f.app.deviceSettings = nil
	f.app.mutex.Unlock()
	f.app.connectionFlights.record(connectionFlightEvent{Stage: connectionFlightStageConnected})
}

func (f deviceConnectionFlow) setRuntimeReady(settings *types.DeviceSettings) {
	f.app.mutex.Lock()
	f.app.isConnected = true
	f.app.deviceSettings = settings
	f.app.mutex.Unlock()
	f.app.connectionFlights.record(connectionFlightEvent{Stage: connectionFlightStageReady})
}

func (f deviceConnectionFlow) setRuntimeDisconnected(reason string) bool {
	f.app.mutex.Lock()
	wasConnected := f.app.isConnected
	f.app.isConnected = false
	f.app.deviceSettings = nil
	f.app.lastSuccessfulDeviceReadAt = time.Time{}
	f.app.mutex.Unlock()
	f.app.connectionFlights.record(connectionFlightEvent{Stage: connectionFlightStageDisconnected, Reason: reason})
	return wasConnected
}

func (f deviceConnectionFlow) connectBestScannedDevice() bool {
	if f.app.monitorOnlyActive() {
		return false
	}
	leavePhase := f.enterPhase(deviceConnectionPhaseDiscovering)
	defer leavePhase()
	f.app.connectionFlights.record(connectionFlightEvent{
		Stage:  connectionFlightStageDiscovering,
		Reason: "auto-scan",
	})
	cfg := f.app.configManager.Get()
	selectionCfg := cfg
	types.NormalizeDeviceProfileConfig(&cfg)
	prioritizeWiFi := shouldPrioritizeWiFiConnection(cfg)
	if prioritizeWiFi {
		wifiScan := f.app.scanWiFiDevicesForConfig(cfg, types.WiFiDiscoveryModeNormal)
		if len(wifiScan.Devices) > 1 {
			f.broadcastError(fmt.Sprintf("发现多个 WiFi 设备（%d 个），请到设置页选择", len(wifiScan.Devices)))
			return false
		}
		if len(wifiScan.Devices) == 1 {
			candidate := wifiDeviceCandidate(wifiScan.Devices[0], activeWiFiProfile(cfg))
			if candidate.ID != "" {
				return f.connectScannedCandidate(candidate)
			}
		}
	}
	for _, transport := range []string{types.DeviceTransportBLE, types.DeviceTransportHID} {
		devices := f.app.deviceManager.ScanNativeDevicesProfilesByTransport(cfg.DeviceProfiles, transport)
		if len(devices) > 0 {
			selected, ok := selectNativeAutoConnectCandidate(devices, selectionCfg, transport)
			if !ok {
				f.broadcastError(fmt.Sprintf("发现多个%s设备（%d 个），请到设置页选择", strings.ToUpper(transport), len(devices)))
				return false
			}
			candidate := nativeDeviceCandidate(selected)
			if candidate.ID != "" {
				return f.connectScannedCandidate(candidate)
			}
		}
	}

	var devices []types.DeviceCandidate
	if prioritizeWiFi {
		if cfg.SerialCompatibilityEnabled {
			devices = serialDeviceCandidates(cfg, availableSerialPortNames())
		}
	} else {
		devices = f.app.scanDeviceCandidates(types.DeviceScanModeNormal, false).Devices
	}
	if len(devices) == 0 {
		f.broadcastError("未发现可连接的设备")
		return false
	}
	if len(devices) > 1 {
		f.broadcastError(fmt.Sprintf("发现多个设备（%d 个），请到设置页选择", len(devices)))
		return false
	}
	return f.connectScannedCandidate(devices[0])
}

func shouldPrioritizeWiFiConnection(cfg types.AppConfig) bool {
	return cfg.WiFiCompatibilityEnabled && cfg.WiFiConnectionPriorityEnabled
}

func selectNativeAutoConnectCandidate(devices []map[string]string, cfg types.AppConfig, transport string) (map[string]string, bool) {
	if len(devices) == 1 {
		return devices[0], true
	}
	transport = types.NormalizeDeviceTransport(transport)
	preferredIDs := []string{cfg.ActiveDeviceProfileIDsByTransport[transport], cfg.ActiveDeviceProfileID}
	for _, preferredID := range preferredIDs {
		preferredID = strings.TrimSpace(preferredID)
		if preferredID == "" {
			continue
		}
		var matched map[string]string
		ambiguous := false
		for _, device := range devices {
			if strings.TrimSpace(device["profileId"]) != preferredID {
				continue
			}
			if matched != nil {
				ambiguous = true
				break
			}
			matched = device
		}
		if matched != nil && !ambiguous {
			return matched, true
		}
	}
	return nil, false
}

func (f deviceConnectionFlow) connectScannedCandidate(device types.DeviceCandidate) bool {
	return f.connectCandidate(types.DeviceConnectRequest{
		ID:        device.ID,
		Transport: device.Transport,
		ProfileID: device.ProfileID,
		Endpoint:  device.Endpoint,
	})
}

func (f deviceConnectionFlow) connectCandidate(req types.DeviceConnectRequest) bool {
	if f.app.monitorOnlyActive() {
		return false
	}
	leavePhase := f.enterPhase(deviceConnectionPhaseConnecting)
	defer leavePhase()
	transport := candidateTransport(req.Transport)
	f.app.connectionFlights.record(connectionFlightEvent{
		Stage:     connectionFlightStageConnecting,
		Reason:    "candidate",
		Transport: transport,
		ProfileID: strings.TrimSpace(req.ProfileID),
	})
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
	if f.app.monitorOnlyActive() {
		return false
	}
	leavePhase := f.enterPhase(deviceConnectionPhaseConnecting)
	defer leavePhase()
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
	started := time.Now()
	success, deviceInfo := f.app.deviceManager.ConnectNativeProfile(profile)
	if success {
		f.app.finishSuccessfulDeviceConnection(deviceInfo, "ConnectNativeDevice")
		return true
	}
	f.app.connectionFlights.record(connectionFlightEvent{
		Stage:      connectionFlightStageError,
		Reason:     "native-connect-failed",
		Transport:  types.NormalizeDeviceTransport(profile.Transport),
		ProfileID:  profile.ID,
		DurationMs: time.Since(started).Milliseconds(),
	})
	f.broadcastError("未发现指定的原生设备")
	return false
}

func (f deviceConnectionFlow) connectCompatibilityCandidate(transport, profileID, endpoint string) bool {
	f.app.autoReconnectSuppressed.Store(false)
	started := time.Now()

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

	profile, err := deviceprofiles.PrepareRuntimeProfile(nextCfg.DeviceProfiles[idx], nextCfg.FanControlDeviceIp)
	if err != nil {
		f.broadcastError("设备档案校验失败: " + err.Error())
		return false
	}
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
		f.app.connectionFlights.record(connectionFlightEvent{
			Stage:      connectionFlightStageError,
			Reason:     "compatibility-connect-failed",
			Transport:  transport,
			ProfileID:  profile.ID,
			DurationMs: time.Since(started).Milliseconds(),
		})
		f.broadcastError("连接失败")
		return false
	}

	if err := f.app.commitConfigUpdate(nextCfg, nil); err != nil {
		f.app.logError("保存设备连接配置失败: %v", err)
		f.app.connectionFlights.record(connectionFlightEvent{
			Stage:      connectionFlightStageError,
			Reason:     "connection-config-save-failed",
			Transport:  transport,
			ProfileID:  profile.ID,
			DurationMs: time.Since(started).Milliseconds(),
		})
		f.broadcastError(err.Error())
		return false
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
	f.setRuntimeDisconnected("device-switch")
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
		profile, err := deviceprofiles.PrepareRuntimeProfile(profile, cfg.FanControlDeviceIp)
		return profile, err == nil && types.IsNativeDeviceTransport(profile.Transport)
	}
	if profile, ok := deviceprofiles.BuiltInProfileByID(profileID); ok {
		profile, err := deviceprofiles.PrepareRuntimeProfile(profile, cfg.FanControlDeviceIp)
		return profile, err == nil && types.IsNativeDeviceTransport(profile.Transport)
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
