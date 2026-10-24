package coreapp

import (
	"encoding/json"
	"fmt"

	"github.com/Eureka-o/FanControlPortable/internal/ipc"
)

// handleIPCRequest 处理 IPC 请求
func (a *CoreApp) handleIPCRequest(req ipc.Request) ipc.Response {
	a.logDebug("处理 IPC 请求[%s] type=%s", req.RequestID, req.Type)

	// 路由顺序即优先级：每条路由只认自己那批 req.Type，其余原样放行给后面的路由。
	// 黑鲨路由必须放在**最后**：它在入口按「当前是不是黑鲨设备」整片拦掉自己那批请求，
	// 而 Ping / 屏幕页 / 调试信息这些不归它管的类型要由后面的路由先接走 ——
	// 放前面会把它们一并截成「当前设备不是黑鲨 BRB02」（Ping 被拦时界面会一直连不上核心）。
	for _, route := range []func(ipc.Request) (ipc.Response, bool){
		a.handleNoiseDiagnosticIPCRequest,
		a.handleDeviceIPCRequest,
		a.handleConfigIPCRequest,
		a.handleControlIPCRequest,
		a.handleTemperatureIPCRequest,
		a.handleAutostartIPCRequest,
		a.handleWindowIPCRequest,
		a.handleSceneIPCRequest,
		a.handleScreenIPCRequest,
		a.handleDebugIPCRequest,
		a.handleSystemIPCRequest,
		a.handleBlackSharkIPCRequest,
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
