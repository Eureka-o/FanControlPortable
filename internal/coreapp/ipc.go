package coreapp

import (
	"encoding/json"
	"fmt"

	"github.com/TIANLI0/THRM/internal/ipc"
)

// handleIPCRequest 处理 IPC 请求
func (a *CoreApp) handleIPCRequest(req ipc.Request) ipc.Response {
	a.logDebug("处理 IPC 请求[%s] type=%s", req.RequestID, req.Type)

	for _, route := range []func(ipc.Request) (ipc.Response, bool){
		a.handleDeviceIPCRequest,
		a.handleConfigIPCRequest,
		a.handleControlIPCRequest,
		a.handleTemperatureIPCRequest,
		a.handlePluginIPCRequest,
		a.handleAutostartIPCRequest,
		a.handleWindowIPCRequest,
		a.handleDebugIPCRequest,
		a.handleSystemIPCRequest,
	} {
		if resp, ok := route(req); ok {
			return resp
		}
	}

	return a.errorResponse(fmt.Sprintf("未知的请求类型: %s", req.Type))
}

// 响应辅助方法
func (a *CoreApp) successResponse(success bool) ipc.Response {
	data, _ := json.Marshal(success)
	return ipc.Response{Success: true, Data: data}
}

func (a *CoreApp) errorResponse(errMsg string) ipc.Response {
	return ipc.Response{Success: false, Error: errMsg}
}

func (a *CoreApp) dataResponse(data any) ipc.Response {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return a.errorResponse("序列化数据失败: " + err.Error())
	}
	return ipc.Response{Success: true, Data: dataBytes}
}
