package coreapp

import (
	"context"
	"strings"

	"github.com/TIANLI0/THRM/internal/device"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

func (a *CoreApp) ScanWiFiDevices(mode string) types.WiFiDiscoveryResult {
	if !a.wifiScanRunning.CompareAndSwap(false, true) {
		return types.WiFiDiscoveryResult{
			Mode:  mode,
			Error: "已有 WiFi 扫描正在进行，请先等待完成或中止当前扫描",
		}
	}
	defer a.wifiScanRunning.Store(false)

	if a.wifiScanControl == nil {
		a.wifiScanControl = types.NewWiFiDiscoveryControl()
	}
	a.wifiScanControl.Reset()
	cfg := a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)
	profile := activeWiFiProfile(cfg)
	params := wifiDiscoveryParamsFromProfile(profile, cfg.FanControlDeviceIp, mode)
	params.Control = a.wifiScanControl
	return device.DiscoverWiFiDevices(context.Background(), params)
}

func (a *CoreApp) ControlWiFiScan(action string) bool {
	if a.wifiScanControl == nil {
		a.wifiScanControl = types.NewWiFiDiscoveryControl()
	}
	switch strings.ToLower(strings.TrimSpace(action)) {
	case types.WiFiScanControlPause:
		a.wifiScanControl.Pause()
	case types.WiFiScanControlResume:
		a.wifiScanControl.Resume()
	case types.WiFiScanControlCancel:
		a.wifiScanControl.Cancel()
	default:
		return false
	}
	return true
}

func (a *CoreApp) recoverDynamicWiFiEndpoint(cfg *types.AppConfig) bool {
	if cfg == nil || !cfg.WiFiCompatibilityEnabled {
		return false
	}
	types.NormalizeDeviceProfileConfig(cfg)
	profile := activeWiFiProfile(*cfg)
	if types.NormalizeDeviceTransport(profile.Transport) != types.DeviceTransportWiFi {
		return false
	}
	oldEndpoint := strings.TrimSpace(profile.Connection.Endpoint)
	if oldEndpoint == "" {
		oldEndpoint = strings.TrimSpace(cfg.FanControlDeviceIp)
	}
	if oldEndpoint == "" {
		return false
	}

	params := wifiDiscoveryParamsFromProfile(profile, oldEndpoint, types.WiFiDiscoveryModeDynamic)
	result := device.DiscoverWiFiDevices(context.Background(), params)
	if len(result.Devices) == 0 {
		if result.Error != "" {
			a.logDebug("dynamic WiFi endpoint scan found nothing: %s", result.Error)
		}
		return false
	}
	nextEndpoint := strings.TrimSpace(result.Devices[0].Endpoint)
	if nextEndpoint == "" || strings.EqualFold(nextEndpoint, oldEndpoint) {
		return false
	}

	updated := updateWiFiProfileEndpoint(cfg, profile.ID, nextEndpoint)
	if !updated {
		return false
	}
	cfg.FanControlDeviceIp = nextEndpoint
	cfg.WiFiCompatibilityEnabled = true
	if cfg.ActiveDeviceProfileIDsByTransport == nil {
		cfg.ActiveDeviceProfileIDsByTransport = map[string]string{}
	}
	cfg.ActiveDeviceProfileIDsByTransport[types.DeviceTransportWiFi] = profile.ID
	if err := a.configManager.Update(*cfg); err != nil {
		a.logError("failed to save dynamic WiFi endpoint %s: %v", nextEndpoint, err)
		return false
	}
	a.configureDeviceManager(*cfg)
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, *cfg)
	}
	a.logInfo("dynamic WiFi endpoint updated from %s to %s", oldEndpoint, nextEndpoint)
	return true
}

func activeWiFiProfile(cfg types.AppConfig) types.DeviceProfile {
	id := types.ActiveDeviceProfileIDForTransport(&cfg, types.DeviceTransportWiFi)
	for _, profile := range cfg.DeviceProfiles {
		if profile.ID == id && types.NormalizeDeviceTransport(profile.Transport) == types.DeviceTransportWiFi {
			return types.NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp)
		}
	}
	return types.DefaultWiFiPercentProfile(cfg.FanControlDeviceIp)
}

func wifiDiscoveryParamsFromProfile(profile types.DeviceProfile, fallbackEndpoint, mode string) types.WiFiDiscoveryParams {
	endpoint := strings.TrimSpace(profile.Connection.Endpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(fallbackEndpoint)
	}
	return types.WiFiDiscoveryParams{
		Mode:          mode,
		Endpoint:      endpoint,
		ProfileID:     profile.ID,
		ProfileName:   profile.DisplayName,
		StateEndpoint: profile.Connection.StateEndpoint,
	}
}

func updateWiFiProfileEndpoint(cfg *types.AppConfig, profileID, endpoint string) bool {
	if cfg == nil || strings.TrimSpace(profileID) == "" || strings.TrimSpace(endpoint) == "" {
		return false
	}
	for i := range cfg.DeviceProfiles {
		if cfg.DeviceProfiles[i].ID != profileID {
			continue
		}
		conn := cfg.DeviceProfiles[i].Connection
		conn.Endpoint = strings.TrimSpace(endpoint)
		if conn.StateEndpoint == "" {
			conn.StateEndpoint = "/api/data"
		}
		if conn.SpeedEndpoint == "" {
			conn.SpeedEndpoint = "/api/speed"
		}
		cfg.DeviceProfiles[i].Connection = conn
		return true
	}
	return false
}
