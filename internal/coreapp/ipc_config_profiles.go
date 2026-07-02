package coreapp

import (
	"encoding/json"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

func (a *CoreApp) handleConfigIPCRequest(req ipc.Request) (ipc.Response, bool) {
	switch req.Type {
	case ipc.ReqGetConfig:
		cfg := a.configManager.Get()
		return a.dataResponse(cfg), true

	case ipc.ReqUpdateConfig:
		var cfg types.AppConfig
		if err := json.Unmarshal(req.Data, &cfg); err != nil {
			return a.errorResponse("解析配置失败: " + err.Error()), true
		}
		if err := a.UpdateConfig(cfg); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true

	case ipc.ReqSetFanCurve:
		var curve []types.FanCurvePoint
		if err := json.Unmarshal(req.Data, &curve); err != nil {
			return a.errorResponse("解析风扇曲线失败: " + err.Error()), true
		}
		if err := a.SetFanCurve(curve); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true

	case ipc.ReqGetFanCurve:
		curve := a.configManager.Get().FanCurve
		return a.dataResponse(curve), true

	case ipc.ReqGetDeviceProfiles:
		return a.dataResponse(a.GetDeviceProfiles()), true

	case ipc.ReqGetSupportedDeviceProfiles:
		return a.dataResponse(a.GetSupportedDeviceProfiles()), true

	case ipc.ReqGetUserDeviceProfiles:
		return a.dataResponse(a.GetUserDeviceProfiles()), true

	case ipc.ReqSetActiveDeviceProfile:
		var params ipc.SetActiveDeviceProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		profile, err := a.SetActiveDeviceProfile(params.ID)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(profile), true

	case ipc.ReqSaveDeviceProfile:
		var params ipc.SaveDeviceProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		profile, err := a.SaveDeviceProfile(params)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(profile), true

	case ipc.ReqDeleteDeviceProfile:
		var params ipc.DeleteDeviceProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if err := a.DeleteDeviceProfile(params.ID); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true

	case ipc.ReqExportDeviceProfiles:
		code, err := a.ExportDeviceProfiles()
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(code), true

	case ipc.ReqImportDeviceProfiles:
		var params ipc.ImportDeviceProfilesParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if err := a.ImportDeviceProfiles(params.Code); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true

	case ipc.ReqTestDeviceProfile:
		var params ipc.TestDeviceProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		result, err := a.TestDeviceProfile(params)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(result), true

	case ipc.ReqResetLearnedOffsets:
		if err := a.ResetLearnedOffsets(); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(map[string]bool{"ok": true}), true

	case ipc.ReqGetFanCurveProfiles:
		return a.dataResponse(a.GetFanCurveProfiles()), true

	case ipc.ReqSetActiveFanCurveProfile:
		var params ipc.SetActiveFanCurveProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		profile, err := a.SetActiveFanCurveProfile(params.ID)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(profile), true

	case ipc.ReqSaveFanCurveProfile:
		var params ipc.SaveFanCurveProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		profile, err := a.SaveFanCurveProfile(params)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(profile), true

	case ipc.ReqDeleteFanCurveProfile:
		var params ipc.DeleteFanCurveProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if err := a.DeleteFanCurveProfile(params.ID); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true

	case ipc.ReqExportFanCurveProfiles:
		code, err := a.ExportFanCurveProfiles()
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(code), true

	case ipc.ReqImportFanCurveProfiles:
		var params ipc.ImportFanCurveProfilesParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if err := a.ImportFanCurveProfiles(params.Code); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true
	default:
		return ipc.Response{}, false
	}
}
