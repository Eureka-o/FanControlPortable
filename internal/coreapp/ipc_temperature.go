package coreapp

import (
	"encoding/json"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

func (a *CoreApp) handleTemperatureIPCRequest(req ipc.Request) (ipc.Response, bool) {
	switch req.Type {
	case ipc.ReqGetTemperature:
		a.mutex.RLock()
		temp := a.currentTemp
		a.mutex.RUnlock()
		return a.dataResponse(temp), true

	case ipc.ReqGetTemperatureHistory:
		return a.dataResponse(a.tempHistory.Snapshot()), true

	case ipc.ReqSetTemperatureHistoryEnabled:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if err := a.SetTemperatureHistoryEnabled(params.Enabled); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.successResponse(true), true

	case ipc.ReqTestTemperatureReading:
		cfg := a.configManager.Get()
		temp := a.tempReader.Read(types.TemperatureSelection{
			TempSource:            cfg.TempSource,
			GpuDevice:             cfg.GpuDevice,
			CpuSensor:             cfg.CpuSensor,
			GpuSensor:             cfg.GpuSensor,
			CpuPowerSensor:        cfg.CpuPowerSensor,
			GpuPowerSensor:        cfg.GpuPowerSensor,
			GpuReadMode:           cfg.GpuReadMode,
			GpuLowPowerProtection: cfg.GpuLowPowerProtection,
		})
		return a.dataResponse(temp), true

	case ipc.ReqTestBridgeProgram:
		cfg := a.configManager.Get()
		data := a.bridgeManager.GetTemperature(types.TemperatureSelection{
			TempSource:            cfg.TempSource,
			GpuDevice:             cfg.GpuDevice,
			CpuSensor:             cfg.CpuSensor,
			GpuSensor:             cfg.GpuSensor,
			CpuPowerSensor:        cfg.CpuPowerSensor,
			GpuPowerSensor:        cfg.GpuPowerSensor,
			GpuReadMode:           cfg.GpuReadMode,
			GpuLowPowerProtection: cfg.GpuLowPowerProtection,
		})
		return a.dataResponse(data), true

	case ipc.ReqGetBridgeProgramStatus:
		status := a.bridgeManager.GetStatus()
		return a.dataResponse(status), true

	case ipc.ReqRestartPawnIO:
		result, err := a.bridgeManager.RestartPawnIO()
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(result), true

	case ipc.ReqReinstallPawnIO:
		result, err := a.ReinstallPawnIO()
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(result), true
	default:
		return ipc.Response{}, false
	}
}
