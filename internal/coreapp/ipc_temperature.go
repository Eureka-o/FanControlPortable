package coreapp

import (
	"encoding/json"

	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func (a *CoreApp) isSensorCatalogActive() bool {
	return a != nil && a.sensorCatalogActive.Load()
}

func (a *CoreApp) isGpuSensorProbeActive() bool {
	return a != nil && a.gpuSensorProbeActive.Load()
}

func (a *CoreApp) clearSensorCatalogLeases() {
	if a == nil {
		return
	}
	a.sensorCatalogActive.Store(false)
	a.gpuSensorProbeActive.Store(false)
	a.mutex.Lock()
	stripSensorLists(&a.currentTemp)
	a.mutex.Unlock()
}

func (a *CoreApp) readAndStoreImmediateTemperature(includeSensorLists, forcePollGpu bool) types.TemperatureData {
	if a == nil || a.tempReader == nil || a.configManager == nil {
		return types.TemperatureData{}
	}
	cfg := a.configManager.Get()
	selection := types.TemperatureSelection{
		TempSource:            cfg.TempSource,
		GpuDevice:             cfg.GpuDevice,
		CpuSensor:             cfg.CpuSensor,
		GpuSensor:             cfg.GpuSensor,
		CpuPowerSensor:        cfg.CpuPowerSensor,
		GpuPowerSensor:        cfg.GpuPowerSensor,
		GpuReadMode:           cfg.GpuReadMode,
		GpuLowPowerProtection: cfg.GpuLowPowerProtection,
		OmitSensorLists:       !includeSensorLists,
		ForcePollGpu:          forcePollGpu && types.NormalizeGPUReadMode(cfg.GpuReadMode) != types.GPUReadModeNever,
	}
	temp := a.tempReader.Read(selection)

	a.mutex.Lock()
	previousTemp := a.currentTemp
	temp = mergeTemperatureHardwareMetadata(previousTemp, temp)
	if !includeSensorLists {
		stripSensorLists(&temp)
	}
	a.currentTemp = temp
	a.mutex.Unlock()

	if a.ipcServer != nil && a.ipcServer.HasInteractiveClients() {
		eventTemp := compactTemperatureEventPayload(temp, previousTemp)
		a.ipcServer.BroadcastEvent(ipc.EventTemperatureUpdate, eventTemp)
	}
	return temp
}

func (a *CoreApp) handleTemperatureIPCRequest(req ipc.Request) (ipc.Response, bool) {
	switch req.Type {
	case ipc.ReqGetTemperature:
		a.mutex.RLock()
		temp := a.currentTemp
		a.mutex.RUnlock()
		return a.dataResponse(temp), true

	case ipc.ReqGetTemperatureHistory:
		if a.tempHistory == nil {
			return a.dataResponse(types.TemperatureHistoryPayload{}), true
		}
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

	case ipc.ReqSetTemperatureHistoryRetentionHours:
		var params ipc.SetIntParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if err := a.SetTemperatureHistoryRetentionHours(params.Value); err != nil {
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

	case ipc.ReqSetSensorCatalogActive:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if !params.Enabled {
			a.clearSensorCatalogLeases()
			return a.successResponse(true), true
		}
		a.sensorCatalogActive.Store(true)
		temp := a.readAndStoreImmediateTemperature(true, a.isGpuSensorProbeActive())
		return a.dataResponse(temp), true

	case ipc.ReqProbeGpuSensors:
		cfg := a.configManager.Get()
		a.sensorCatalogActive.Store(true)
		if types.NormalizeGPUReadMode(cfg.GpuReadMode) == types.GPUReadModeNever {
			a.gpuSensorProbeActive.Store(false)
			temp := a.readAndStoreImmediateTemperature(true, false)
			return a.dataResponse(temp), true
		}
		a.gpuSensorProbeActive.Store(true)
		temp := a.readAndStoreImmediateTemperature(true, true)
		return a.dataResponse(temp), true

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
