package coreapp

import (
	"encoding/json"

	"github.com/TIANLI0/THRM/internal/ipc"
)

func (a *CoreApp) handleDeviceIPCRequest(req ipc.Request) (ipc.Response, bool) {
	switch req.Type {
	case ipc.ReqConnect:
		success := a.ConnectDevice()
		return a.successResponse(success), true

	case ipc.ReqAutoScanDevices:
		return a.dataResponse(a.AutoScanDevices()), true

	case ipc.ReqScanDeviceCandidates:
		var params ipc.ScanDeviceCandidatesParams
		if len(req.Data) > 0 {
			if err := json.Unmarshal(req.Data, &params); err != nil {
				return a.errorResponse("解析设备扫描参数失败: " + err.Error()), true
			}
		}
		return a.dataResponse(a.ScanDeviceCandidates(params.Mode)), true

	case ipc.ReqConnectDeviceCandidate:
		var params ipc.ConnectDeviceCandidateParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析设备连接参数失败: " + err.Error()), true
		}
		success := a.ConnectDeviceCandidate(params.Candidate)
		return a.successResponse(success), true

	case ipc.ReqConnectNativeDevice:
		var params ipc.ConnectNativeDeviceParams
		if len(req.Data) > 0 {
			if err := json.Unmarshal(req.Data, &params); err != nil {
				return a.errorResponse("解析原生设备连接参数失败: " + err.Error()), true
			}
		}
		success := a.ConnectNativeDevice(params.ProfileID)
		return a.successResponse(success), true

	case ipc.ReqScanWiFiDevices:
		var params ipc.ScanWiFiDevicesParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析 WiFi 扫描参数失败: " + err.Error()), true
		}
		return a.dataResponse(a.ScanWiFiDevices(params.Mode)), true

	case ipc.ReqControlWiFiScan:
		var params ipc.ControlWiFiScanParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析 WiFi 扫描控制参数失败: " + err.Error()), true
		}
		if !a.ControlWiFiScan(params.Action) {
			return a.errorResponse("不支持的 WiFi 扫描控制动作"), true
		}
		return a.successResponse(true), true

	case ipc.ReqDisconnect:
		a.DisconnectDevice()
		return a.successResponse(true), true

	case ipc.ReqGetDeviceStatus:
		status := a.GetDeviceStatus()
		return a.dataResponse(status), true

	case ipc.ReqGetCurrentFanData:
		data := a.deviceManager.GetCurrentFanData()
		return a.dataResponse(data), true

	case ipc.ReqRefreshDeviceSettings:
		settings, err := a.RefreshDeviceSettings()
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(settings), true
	default:
		return ipc.Response{}, false
	}
}
