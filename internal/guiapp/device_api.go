package guiapp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

// ConnectDevice 连接HID设备
func (a *App) ConnectDevice() bool {
	resp, err := a.sendRequest(ipc.ReqConnect, nil)
	if err != nil {
		guiLogger.Errorf("连接设备请求失败: %v", err)
		return false
	}
	if !resp.Success {
		guiLogger.Errorf("连接设备失败: %s", resp.Error)
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

func (a *App) AutoScanDevices() map[string]any {
	resp, err := a.sendRequest(ipc.ReqAutoScanDevices, nil)
	if err != nil {
		guiLogger.Errorf("自动扫描设备请求失败: %v", err)
		return map[string]any{"connected": false, "error": err.Error()}
	}
	if !resp.Success {
		guiLogger.Errorf("自动扫描设备失败: %s", resp.Error)
		return map[string]any{"connected": false, "error": resp.Error}
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return map[string]any{"connected": false, "error": err.Error()}
	}
	return result
}

func (a *App) ScanDeviceCandidates(mode string) types.DeviceScanResult {
	timeout := 15 * time.Second
	params := ipc.ScanDeviceCandidatesParams{Mode: mode}
	var resp *ipc.Response
	var err error
	if mode == types.DeviceScanModeDeep {
		timeout = 90 * time.Second
		resp, err = sendTemporaryIPCRequest(ipc.ReqScanDeviceCandidates, params, timeout)
	} else {
		resp, err = a.sendRequestWithTimeout(ipc.ReqScanDeviceCandidates, params, timeout)
	}
	if err != nil {
		guiLogger.Errorf("设备扫描请求失败: %v", err)
		return types.DeviceScanResult{Mode: mode, Error: err.Error()}
	}
	if !resp.Success {
		guiLogger.Errorf("设备扫描失败: %s", resp.Error)
		return types.DeviceScanResult{Mode: mode, Error: resp.Error}
	}
	var result types.DeviceScanResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return types.DeviceScanResult{Mode: mode, Error: err.Error()}
	}
	return result
}

func (a *App) ConnectDeviceCandidate(candidate types.DeviceConnectRequest) bool {
	resp, err := a.sendRequest(ipc.ReqConnectDeviceCandidate, ipc.ConnectDeviceCandidateParams{Candidate: candidate})
	if err != nil {
		guiLogger.Errorf("设备候选连接请求失败: %v", err)
		return false
	}
	if !resp.Success {
		guiLogger.Errorf("设备候选连接失败: %s", resp.Error)
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// DisconnectDevice 断开设备连接
func (a *App) ConnectNativeDevice(profileID string) bool {
	resp, err := a.sendRequest(ipc.ReqConnectNativeDevice, ipc.ConnectNativeDeviceParams{ProfileID: profileID})
	if err != nil {
		guiLogger.Errorf("原生设备连接请求失败: %v", err)
		return false
	}
	if !resp.Success {
		guiLogger.Errorf("原生设备连接失败: %s", resp.Error)
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

func (a *App) ScanWiFiDevices(mode string) types.WiFiDiscoveryResult {
	timeout := 12 * time.Second
	params := ipc.ScanWiFiDevicesParams{Mode: mode}
	var resp *ipc.Response
	var err error
	if mode == types.WiFiDiscoveryModeDeep {
		timeout = 90 * time.Second
		resp, err = sendTemporaryIPCRequest(ipc.ReqScanWiFiDevices, params, timeout)
	} else {
		resp, err = a.sendRequestWithTimeout(ipc.ReqScanWiFiDevices, params, timeout)
	}
	if err != nil {
		guiLogger.Errorf("WiFi IP scan request failed: %v", err)
		return types.WiFiDiscoveryResult{Mode: mode, Error: err.Error()}
	}
	if !resp.Success {
		guiLogger.Errorf("WiFi IP scan failed: %s", resp.Error)
		return types.WiFiDiscoveryResult{Mode: mode, Error: resp.Error}
	}
	var result types.WiFiDiscoveryResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return types.WiFiDiscoveryResult{Mode: mode, Error: err.Error()}
	}
	return result
}

func (a *App) ControlWiFiScan(action string) bool {
	resp, err := sendTemporaryIPCRequest(ipc.ReqControlWiFiScan, ipc.ControlWiFiScanParams{Action: action}, 3*time.Second)
	if err != nil {
		guiLogger.Errorf("WiFi scan control request failed: %v", err)
		return false
	}
	if !resp.Success {
		guiLogger.Errorf("WiFi scan control failed: %s", resp.Error)
		return false
	}
	return true
}

func sendTemporaryIPCRequest(reqType ipc.RequestType, data any, timeout time.Duration) (*ipc.Response, error) {
	client := ipc.NewClient(nil)
	if err := client.Connect(); err != nil {
		return nil, err
	}
	defer client.Close()
	return client.SendRequestWithTimeout(reqType, data, timeout)
}

func (a *App) DisconnectDevice() error {
	resp, err := a.sendRequest(ipc.ReqDisconnect, nil)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// GetDeviceStatus 获取设备连接状态
func (a *App) GetDeviceStatus() map[string]any {
	resp, err := a.sendRequest(ipc.ReqGetDeviceStatus, nil)
	if err != nil {
		return map[string]any{"connected": false, "error": err.Error()}
	}
	if !resp.Success {
		return map[string]any{"connected": false, "error": resp.Error}
	}
	var status map[string]any
	json.Unmarshal(resp.Data, &status)
	return status
}

// GetCurrentFanData 获取当前风扇数据
func (a *App) GetCurrentFanData() *FanData {
	resp, err := a.sendRequest(ipc.ReqGetCurrentFanData, nil)
	if err != nil {
		return nil
	}
	var fanData FanData
	if err := json.Unmarshal(resp.Data, &fanData); err != nil {
		return nil
	}
	return &fanData
}

func (a *App) RefreshDeviceSettings() (*DeviceSettings, error) {
	resp, err := a.sendRequest(ipc.ReqRefreshDeviceSettings, nil)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	var settings DeviceSettings
	if err := json.Unmarshal(resp.Data, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

// GetAvailableGears 获取可用挡位
func (a *App) GetAvailableGears() map[string][]GearCommand {
	resp, err := a.sendRequest(ipc.ReqGetAvailableGears, nil)
	if err != nil {
		return types.GearCommands
	}
	if !resp.Success {
		return types.GearCommands
	}
	var gears map[string][]GearCommand
	json.Unmarshal(resp.Data, &gears)
	return gears
}
