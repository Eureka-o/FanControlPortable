package coreapp

import (
	"context"
	"strings"

	"github.com/Eureka-o/FanControlPortable/internal/device"
	"github.com/Eureka-o/FanControlPortable/internal/deviceprofileexec"
	"github.com/Eureka-o/FanControlPortable/internal/deviceprofiles"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func (a *CoreApp) ScanDeviceCandidates(mode string) types.DeviceScanResult {
	return a.scanDeviceCandidates(mode, true)
}

func (a *CoreApp) scanDeviceCandidates(mode string, includeNative bool) types.DeviceScanResult {
	mode = normalizeDeviceScanMode(mode)
	if a.monitorOnlyActive() {
		return types.DeviceScanResult{Mode: mode, Error: "仅监控模式已启用"}
	}

	cfg := a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)
	connected := a.deviceRuntimeSnapshot().Connected

	result := types.DeviceScanResult{
		Mode:          mode,
		Connected:     connected,
		WiFiEnabled:   cfg.WiFiCompatibilityEnabled,
		SerialEnabled: cfg.SerialCompatibilityEnabled,
	}

	if includeNative {
		for _, info := range a.deviceManager.ScanNativeDevicesProfiles(cfg.DeviceProfiles) {
			if candidate := nativeDeviceCandidate(info); candidate.ID != "" {
				result.Devices = append(result.Devices, candidate)
			}
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
	return a.scanWiFiDevicesForConfigContext(context.Background(), cfg, mode)
}

func (a *CoreApp) scanWiFiDevicesForConfigContext(ctx context.Context, cfg types.AppConfig, mode string) types.WiFiDiscoveryResult {
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
	if ctx == nil {
		ctx = context.Background()
	}
	return device.DiscoverWiFiDevices(ctx, params)
}

func (a *CoreApp) connectWiFiPriorityCandidate(ctx context.Context, cfg types.AppConfig, configRevision uint64, device types.WiFiDiscoveredDevice) (bool, bool, map[string]string) {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint := strings.TrimSpace(device.Endpoint)
	if endpoint == "" || ctx.Err() != nil || a.configManager.Revision() != configRevision {
		return false, false, nil
	}
	nextCfg := cfg
	idx := compatibilityProfileIndex(nextCfg, types.DeviceTransportWiFi, activeWiFiProfile(nextCfg).ID)
	if idx < 0 {
		nextCfg.DeviceProfiles = append(nextCfg.DeviceProfiles, types.DefaultWiFiPercentProfile(nextCfg.FanControlDeviceIp))
		idx = len(nextCfg.DeviceProfiles) - 1
	}
	profile, err := deviceprofiles.PrepareRuntimeProfile(nextCfg.DeviceProfiles[idx], nextCfg.FanControlDeviceIp)
	if err != nil {
		return false, false, nil
	}
	profile.Connection.Endpoint = endpoint
	nextCfg.DeviceProfiles[idx] = profile
	nextCfg.FanControlDeviceIp = endpoint
	nextCfg.ActiveDeviceProfileID = profile.ID
	nextCfg.DeviceTransport = types.DeviceTransportWiFi
	if nextCfg.ActiveDeviceProfileIDsByTransport == nil {
		nextCfg.ActiveDeviceProfileIDsByTransport = make(map[string]string)
	}
	nextCfg.ActiveDeviceProfileIDsByTransport[types.DeviceTransportWiFi] = profile.ID
	nextCfg.WiFiCompatibilityEnabled = true
	types.NormalizeDeviceProfileConfig(&nextCfg)

	a.configureDeviceManager(nextCfg)
	connected, info := a.deviceManager.Connect()
	if !connected {
		a.configureDeviceManager(a.configManager.Get())
		return false, false, nil
	}
	if info == nil {
		return true, true, nil
	}
	generation := a.deviceManager.ConnectionGeneration()
	rollback := func() {
		a.deviceManager.DisconnectIfConnectionGeneration(generation)
		a.configureDeviceManager(a.configManager.Get())
	}
	if ctx.Err() != nil {
		rollback()
		return false, false, nil
	}
	_, revision, applied, err := a.commitConfigMutationIfRevision(configRevision, func(current *types.AppConfig) {
		*current = nextCfg
	}, nil)
	if err != nil {
		a.logError("保存 WiFi 优先连接配置失败: %v", err)
		rollback()
		return false, false, nil
	}
	if !applied && revision != configRevision {
		a.logInfo("WiFi 优先连接期间配置已变化，取消过期连接结果")
		rollback()
		return false, false, nil
	}
	a.lastConnectionWasNative.Store(false)
	return true, false, info
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
	if a.monitorOnlyActive() {
		return false
	}
	a.cancelReconnect()
	a.connectMutex.Lock()
	defer a.connectMutex.Unlock()
	return newDeviceConnectionFlow(a).connectCandidate(req)
}

func (a *CoreApp) ConnectBestScannedDevice() bool {
	if a.monitorOnlyActive() {
		return false
	}
	a.cancelReconnect()
	a.connectMutex.Lock()
	defer a.connectMutex.Unlock()
	return newDeviceConnectionFlow(a).connectBestScannedDevice()
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
