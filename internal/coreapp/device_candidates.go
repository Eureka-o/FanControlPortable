package coreapp

import (
	"context"
	"strings"

	"github.com/TIANLI0/THRM/internal/device"
	"github.com/TIANLI0/THRM/internal/deviceprofileexec"
	"github.com/TIANLI0/THRM/internal/types"
)

func (a *CoreApp) ScanDeviceCandidates(mode string) types.DeviceScanResult {
	return a.scanDeviceCandidates(mode, true)
}

func (a *CoreApp) scanDeviceCandidates(mode string, includeNative bool) types.DeviceScanResult {
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
	a.cancelReconnect()
	a.connectMutex.Lock()
	defer a.connectMutex.Unlock()
	return newDeviceConnectionFlow(a).connectCandidate(req)
}

func (a *CoreApp) ConnectBestScannedDevice() bool {
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
