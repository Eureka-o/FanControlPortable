package coreapp

import (
	"encoding/json"

	"github.com/Eureka-o/FanControlPortable/internal/ipc"
)

func (a *CoreApp) handlePluginIPCRequest(req ipc.Request) (ipc.Response, bool) {
	switch req.Type {
	case ipc.ReqGetPlugins:
		return a.dataResponse(a.GetPlugins()), true

	case ipc.ReqRefreshPlugins:
		return a.dataResponse(a.RefreshPlugins()), true

	case ipc.ReqSetPluginEnabled:
		var params ipc.SetPluginEnabledParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析插件开关参数失败: " + err.Error()), true
		}
		updated, err := a.SetPluginEnabled(params.PluginID, params.Enabled)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(updated), true

	case ipc.ReqInvokePlugin:
		var params ipc.InvokePluginParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析插件调用参数失败: " + err.Error()), true
		}
		res, err := a.InvokePlugin(params.PluginID, params.Method, params.Payload)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(res), true

	case ipc.ReqRestartPlugin:
		var params ipc.PluginIDParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析插件重启参数失败: " + err.Error()), true
		}
		updated, err := a.RestartPlugin(params.PluginID)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(updated), true

	case ipc.ReqGetPluginSettings:
		var params ipc.PluginIDParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析插件设置请求失败: " + err.Error()), true
		}
		settings, err := a.GetPluginSettings(params.PluginID)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(settings), true

	case ipc.ReqSavePluginSettings:
		var params ipc.SavePluginSettingsParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析保存插件设置参数失败: " + err.Error()), true
		}
		if err := a.SavePluginSettings(params.PluginID, params.Settings); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true

	case ipc.ReqGetPluginLogs:
		var params ipc.PluginIDParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析插件日志请求失败: " + err.Error()), true
		}
		return a.dataResponse(a.GetPluginLogs(params.PluginID)), true

	case ipc.ReqSetPluginDevPaths:
		var params ipc.SetPluginDevPathsParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析开发目录参数失败: " + err.Error()), true
		}
		updated, err := a.SetPluginDevPaths(params.Paths)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(updated), true

	case ipc.ReqMarkPluginPendingRestart:
		var params ipc.PluginIDParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析重启标记参数失败: " + err.Error()), true
		}
		return a.dataResponse(a.MarkPluginPendingRestart(params.PluginID)), true

	default:
		return ipc.Response{}, false
	}
}
