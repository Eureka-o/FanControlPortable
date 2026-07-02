package coreapp

import (
	"encoding/json"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

func (a *CoreApp) handleControlIPCRequest(req ipc.Request) (ipc.Response, bool) {
	switch req.Type {
	case ipc.ReqSetAutoControl:
		var params ipc.SetAutoControlParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if err := a.SetAutoControl(params.Enabled); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true

	case ipc.ReqSetManualGear:
		var params ipc.SetManualGearParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		success := a.SetManualGear(params.Gear, params.Level)
		return a.successResponse(success), true

	case ipc.ReqGetAvailableGears:
		gears := types.GearCommands
		return a.dataResponse(gears), true

	case ipc.ReqSetCustomSpeed:
		var params ipc.SetCustomSpeedParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if err := a.SetCustomSpeed(params.Enabled, params.RPM); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true

	case ipc.ReqSetGearLight:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		success := a.SetGearLight(params.Enabled)
		return a.successResponse(success), true

	case ipc.ReqSetPowerOnStart:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		success := a.SetPowerOnStart(params.Enabled)
		return a.successResponse(success), true

	case ipc.ReqSetSmartStartStop:
		var params ipc.SetStringParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		success := a.SetSmartStartStop(params.Value)
		return a.successResponse(success), true

	case ipc.ReqSetBrightness:
		var params ipc.SetIntParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		success := a.SetBrightness(params.Value)
		return a.successResponse(success), true

	case ipc.ReqSetLightStrip:
		var params ipc.SetLightStripParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if err := a.SetLightStrip(params.Config); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true
	default:
		return ipc.Response{}, false
	}
}
