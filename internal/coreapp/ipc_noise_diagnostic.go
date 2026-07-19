package coreapp

import (
	"encoding/json"

	"github.com/TIANLI0/THRM/internal/ipc"
)

func (a *CoreApp) handleNoiseDiagnosticIPCRequest(req ipc.Request) (ipc.Response, bool) {
	switch req.Type {
	case ipc.ReqBeginNoiseDiagnostic:
		var params ipc.BeginNoiseDiagnosticParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		session, err := a.BeginNoiseDiagnostic(params.Request)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(session), true

	case ipc.ReqSetNoiseDiagnosticTarget:
		var params ipc.SetNoiseDiagnosticTargetParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		result, err := a.SetNoiseDiagnosticTarget(params.SessionID, params.Value)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(result), true

	case ipc.ReqEndNoiseDiagnostic:
		var params ipc.NoiseDiagnosticSessionParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if err := a.EndNoiseDiagnostic(params.SessionID); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true

	case ipc.ReqCancelNoiseDiagnostic:
		var params ipc.NoiseDiagnosticSessionParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if err := a.CancelNoiseDiagnostic(params.SessionID); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true

	case ipc.ReqSaveNoiseDiagnosticResult:
		var params ipc.SaveNoiseDiagnosticResultParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if params.Result.DeviceKey == "" && len(params.Result.Points) == 0 {
			return a.errorResponse("缺少诊断结果"), true
		}
		if err := a.SaveNoiseDiagnosticResult(params.Result); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true
	default:
		return ipc.Response{}, false
	}
}
