package coreapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/TIANLI0/THRM/internal/device"
	"github.com/TIANLI0/THRM/internal/deviceprofileexec"
	"github.com/TIANLI0/THRM/internal/deviceprofiles"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

func (a *CoreApp) ScanDeviceCandidates(mode string) types.DeviceScanResult {
	mode = normalizeDeviceScanMode(mode)

	cfg := a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)
	a.mutex.RLock()
	connected := a.isConnected && a.deviceManager.IsConnected()
	a.mutex.RUnlock()

	result := types.DeviceScanResult{
		Mode:          mode,
		Connected:     connected,
		WiFiEnabled:   cfg.WiFiCompatibilityEnabled,
		SerialEnabled: cfg.SerialCompatibilityEnabled,
	}

	for _, info := range a.deviceManager.ScanNativeDevicesProfiles(cfg.DeviceProfiles) {
		if candidate := nativeDeviceCandidate(info); candidate.ID != "" {
			result.Devices = append(result.Devices, candidate)
		}
	}

	if cfg.WiFiCompatibilityEnabled {
		wifiMode := types.WiFiDiscoveryModeNormal
		if mode == types.DeviceScanModeDeep {
			wifiMode = types.WiFiDiscoveryModeDeep
		}
		wifiScan := a.scanWiFiDevicesForConfig(cfg, wifiMode)
		for _, discovered := range wifiScan.Devices {
			if candidate := wifiDeviceCandidate(discovered, activeWiFiProfile(cfg)); candidate.ID != "" {
				result.Devices = append(result.Devices, candidate)
			}
		}
		result.ShowDeepScan = mode != types.DeviceScanModeDeep && len(wifiScan.Devices) == 0
		if wifiScan.Error != "" {
			result.Error = wifiScan.Error
		}
	}

	if cfg.SerialCompatibilityEnabled {
		result.Devices = append(result.Devices, serialDeviceCandidates(cfg, availableSerialPortNames())...)
	}

	return result
}

func normalizeDeviceScanMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), types.DeviceScanModeDeep) {
		return types.DeviceScanModeDeep
	}
	return types.DeviceScanModeNormal
}

func (a *CoreApp) scanWiFiDevicesForConfig(cfg types.AppConfig, mode string) types.WiFiDiscoveryResult {
	if !a.wifiScanRunning.CompareAndSwap(false, true) {
		return types.WiFiDiscoveryResult{
			Mode:  mode,
			Error: "已有 WiFi 扫描正在进行，请先等待完成",
		}
	}
	defer a.wifiScanRunning.Store(false)

	if a.wifiScanControl == nil {
		a.wifiScanControl = types.NewWiFiDiscoveryControl()
	}
	a.wifiScanControl.Reset()
	profile := activeWiFiProfile(cfg)
	params := wifiDiscoveryParamsFromProfile(profile, cfg.FanControlDeviceIp, mode)
	params.Control = a.wifiScanControl
	return device.DiscoverWiFiDevices(context.Background(), params)
}

func nativeDeviceCandidate(info map[string]string) types.DeviceCandidate {
	transport := types.NormalizeDeviceTransport(info["transport"])
	if !types.IsNativeDeviceTransport(transport) {
		return types.DeviceCandidate{}
	}
	profileID := strings.TrimSpace(info["profileId"])
	endpoint := firstNonEmptyString(info["endpoint"], info["serial"])
	name := firstNonEmptyString(info["product"], info["profileName"], info["model"], info["name"], "原生设备")
	return types.DeviceCandidate{
		ID:          "native:" + transport + ":" + firstNonEmptyString(profileID, info["productId"], endpoint, name),
		Transport:   transport,
		Name:        name,
		ProfileID:   profileID,
		Endpoint:    endpoint,
		Source:      "native",
		Connectable: true,
	}
}

func wifiDeviceCandidate(device types.WiFiDiscoveredDevice, profile types.DeviceProfile) types.DeviceCandidate {
	endpoint := strings.TrimSpace(device.Endpoint)
	if endpoint == "" {
		return types.DeviceCandidate{}
	}
	profileID := strings.TrimSpace(device.ProfileID)
	if profileID == "" {
		profileID = profile.ID
	}
	return types.DeviceCandidate{
		ID:          "wifi:" + profileID + ":" + endpoint,
		Transport:   types.DeviceTransportWiFi,
		Name:        firstNonEmptyString(device.Name, profile.DisplayName, profile.Model, "WiFi 设备"),
		ProfileID:   profileID,
		Endpoint:    endpoint,
		Source:      firstNonEmptyString(device.Source, "wifi"),
		Network:     device.Network,
		Speed:       device.Speed,
		TargetSpeed: device.TargetSpeed,
		Temperature: device.Temperature,
		LatencyMs:   device.LatencyMs,
		Connectable: true,
	}
}

func serialDeviceCandidates(cfg types.AppConfig, availablePorts map[string]bool) []types.DeviceCandidate {
	candidates := make([]types.DeviceCandidate, 0)
	for _, profile := range cfg.DeviceProfiles {
		if types.NormalizeDeviceTransport(profile.Transport) != types.DeviceTransportSerial {
			continue
		}
		profile = types.NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp)
		port := strings.TrimSpace(profile.Connection.SerialPort)
		if profile.ID == "" || port == "" {
			continue
		}
		if !availablePorts[strings.ToUpper(port)] {
			continue
		}
		candidates = append(candidates, types.DeviceCandidate{
			ID:          "serial:" + profile.ID + ":" + port,
			Transport:   types.DeviceTransportSerial,
			Name:        firstNonEmptyString(profile.DisplayName, profile.Model, "串口设备"),
			ProfileID:   profile.ID,
			Endpoint:    port,
			Source:      "saved",
			Connectable: true,
		})
	}
	return candidates
}

func availableSerialPortNames() map[string]bool {
	ports, err := deviceprofileexec.ListSerialPorts()
	out := make(map[string]bool, len(ports))
	if err != nil {
		return out
	}
	for _, port := range ports {
		name := strings.ToUpper(strings.TrimSpace(port.Name))
		if name != "" {
			out[name] = true
		}
	}
	return out
}

func (a *CoreApp) ConnectDeviceCandidate(req types.DeviceConnectRequest) bool {
	transport := candidateTransport(req.Transport)
	switch transport {
	case types.DeviceTransportWiFi, types.DeviceTransportSerial:
		return a.connectCompatibilityCandidate(transport, req.ProfileID, req.Endpoint)
	case types.DeviceTransportHID, types.DeviceTransportBLE:
		return a.ConnectNativeDevice(req.ProfileID)
	default:
		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventDeviceError, "缺少可连接的设备信息")
		}
		return false
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

func (a *CoreApp) connectCompatibilityCandidate(transport, profileID, endpoint string) bool {
	a.autoReconnectSuppressed.Store(false)

	oldCfg := a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&oldCfg)
	nextCfg := oldCfg

	idx := compatibilityProfileIndex(nextCfg, transport, profileID)
	if idx < 0 && transport == types.DeviceTransportWiFi {
		nextCfg.DeviceProfiles = append(nextCfg.DeviceProfiles, types.DefaultWiFiPercentProfile(nextCfg.FanControlDeviceIp))
		idx = len(nextCfg.DeviceProfiles) - 1
	}
	if idx < 0 {
		a.broadcastDeviceError("未找到可连接的兼容设备")
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

	a.disconnectForCandidateSwitch()
	a.configureDeviceManager(nextCfg)
	success, deviceInfo := a.deviceManager.Connect()
	if !success {
		a.configureDeviceManager(oldCfg)
		a.broadcastDeviceError("连接失败")
		return false
	}

	if err := a.configManager.Update(nextCfg); err != nil {
		a.logError("保存设备连接配置失败: %v", err)
		a.broadcastDeviceError(err.Error())
		return false
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, nextCfg)
	}
	a.lastConnectionWasNative.Store(false)
	a.finishSuccessfulDeviceConnection(deviceInfo, "ConnectDeviceCandidate")
	return true
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

func (a *CoreApp) disconnectForCandidateSwitch() {
	if !a.deviceManager.IsConnected() {
		return
	}
	a.deviceManager.DisconnectSilently()
	a.mutex.Lock()
	a.isConnected = false
	a.deviceSettings = nil
	a.mutex.Unlock()
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}
}

func (a *CoreApp) ConnectBestScannedDevice() bool {
	scan := a.ScanDeviceCandidates(types.DeviceScanModeNormal)
	if len(scan.Devices) == 0 {
		a.broadcastDeviceError("未发现可连接的设备")
		return false
	}
	if len(scan.Devices) > 1 {
		a.broadcastDeviceError(fmt.Sprintf("发现多个设备（%d 个），请到设置页选择", len(scan.Devices)))
		return false
	}
	device := scan.Devices[0]
	return a.ConnectDeviceCandidate(types.DeviceConnectRequest{
		ID:        device.ID,
		Transport: device.Transport,
		ProfileID: device.ProfileID,
		Endpoint:  device.Endpoint,
	})
}

func (a *CoreApp) broadcastDeviceError(message string) {
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceError, message)
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
