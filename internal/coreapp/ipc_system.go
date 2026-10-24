package coreapp

import (
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/ipc"
)

func (a *CoreApp) handleAutostartIPCRequest(req ipc.Request) (ipc.Response, bool) {
	switch req.Type {
	case ipc.ReqSetWindowsAutoStart:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if err := a.SetWindowsAutoStart(params.Enabled); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true

	case ipc.ReqCheckWindowsAutoStart:
		enabled := a.CheckWindowsAutoStart()
		return a.dataResponse(enabled), true

	case ipc.ReqIsRunningAsAdmin:
		isAdmin := a.autostartManager.IsRunningAsAdmin()
		return a.dataResponse(isAdmin), true

	case ipc.ReqGetAutoStartMethod:
		method := a.autostartManager.GetAutoStartMethod()
		return a.dataResponse(method), true

	case ipc.ReqSetAutoStartWithMethod:
		var params ipc.SetAutoStartWithMethodParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if err := a.SetAutoStartWithMethod(params.Enable, params.Method); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true
	default:
		return ipc.Response{}, false
	}
}

func (a *CoreApp) handleWindowIPCRequest(req ipc.Request) (ipc.Response, bool) {
	switch req.Type {
	case ipc.ReqShowWindow:
		a.onShowWindowRequest()
		return a.successResponse(true), true

	case ipc.ReqHideWindow:
		// GUI 自己处理隐藏
		return a.successResponse(true), true

	case ipc.ReqRestartCore:
		var params ipc.RestartCoreParams
		if len(req.Data) > 0 {
			if err := json.Unmarshal(req.Data, &params); err != nil {
				return a.errorResponse("解析参数失败: " + err.Error()), true
			}
		}
		a.restartCore(params)
		return a.successResponse(true), true

	case ipc.ReqQuitApp:
		a.safeGo("onQuitRequest", func() {
			a.onQuitRequest()
		})
		return a.successResponse(true), true
	default:
		return ipc.Response{}, false
	}
}

func (a *CoreApp) handleDebugIPCRequest(req ipc.Request) (ipc.Response, bool) {
	switch req.Type {
	case ipc.ReqGetDebugInfo:
		info := a.GetDebugInfo()
		return a.dataResponse(info), true

	case ipc.ReqExportDiagnostics:
		bundle, err := a.ExportDiagnostics()
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(bundle), true

	case ipc.ReqSetDebugMode:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if err := a.SetDebugMode(params.Enabled); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true

	case ipc.ReqSendDeviceDebugCommand:
		var params ipc.DeviceDebugCommandParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		result, err := a.SendDeviceDebugCommand(params.Hex, params.WaitMs)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(result), true

	case ipc.ReqGetDeviceDebugFrames:
		return a.dataResponse(a.GetDeviceDebugFrames()), true

	case ipc.ReqUpdateGuiResponseTime:
		atomic.StoreInt64(&a.guiLastResponse, time.Now().Unix())
		return a.successResponse(true), true
	default:
		return ipc.Response{}, false
	}
}

func (a *CoreApp) handleSystemIPCRequest(req ipc.Request) (ipc.Response, bool) {
	switch req.Type {
	case ipc.ReqPing:
		return a.dataResponse("pong"), true

	case ipc.ReqIsAutoStartLaunch:
		return a.dataResponse(a.isAutoStartLaunch), true

	// 前端上报界面上发生的事（异常 / 导航 / 卸载），见 ipc.ReqReportClientIssue 的注释。
	case ipc.ReqReportClientIssue:
		var p ipc.ClientIssueReport
		if err := json.Unmarshal(req.Data, &p); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		a.logInfo("[前端] %s | %s | %s", p.Kind, p.Message, p.Detail)
		return a.dataResponse(true), true

	default:
		return ipc.Response{}, false
	}
}
