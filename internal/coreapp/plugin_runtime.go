package coreapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/plugins"
	"github.com/TIANLI0/THRM/internal/types"
)

func (a *CoreApp) syncPluginSupervisor() {
	if a.pluginCatalog == nil || a.pluginSupervisor == nil {
		return
	}
	a.pluginSupervisor.Sync(a.pluginCatalog.RuntimeSpecs())
}

func (a *CoreApp) startEnabledPluginRuntimes(snapshot plugins.CatalogSnapshot) {
	if a.pluginSupervisor == nil {
		return
	}
	a.pluginSupervisor.Start(a.ctx)
	if err := a.pluginSupervisor.StartEnabled(enabledPluginsFromSnapshot(snapshot)); err != nil {
		a.logError("启动外部插件后端失败: %v", err)
	}
}

func enabledPluginsFromSnapshot(snapshot plugins.CatalogSnapshot) map[string]bool {
	enabled := make(map[string]bool)
	for _, plugin := range snapshot.Plugins {
		if plugin.Enabled && plugin.State != plugins.CatalogStateInvalid && plugin.State != plugins.CatalogStateIncompatible {
			enabled[plugin.ID] = true
		}
	}
	return enabled
}

func (a *CoreApp) refreshPluginCatalog(startEnabled bool) plugins.CatalogSnapshot {
	if a.pluginCatalog == nil {
		return plugins.CatalogSnapshot{Plugins: []plugins.CatalogEntry{}}
	}
	snapshot := a.pluginCatalog.Refresh()
	a.syncPluginSupervisor()
	snapshot = a.pluginCatalog.Snapshot()
	if startEnabled && a.pluginSupervisor != nil {
		a.safeGo("start-enabled-external-plugins", func() {
			a.startEnabledPluginRuntimes(snapshot)
		})
	}
	return snapshot
}

func (a *CoreApp) handlePluginRuntimeStatus(status plugins.RuntimeStatus) {
	if a.pluginCatalog == nil {
		return
	}
	snapshot, changed := a.pluginCatalog.SetRuntimeState(status.ID, status.State, status.LastError, status.Capabilities)
	if !changed {
		return
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventPluginStatusUpdate, snapshot)
	}
}

func (a *CoreApp) handlePluginRuntimeEvent(event plugins.RuntimeEvent) {
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventPluginEvent, event)
	}
}

func (a *CoreApp) invokePlugin(id, method string, payload json.RawMessage) (json.RawMessage, error) {
	if a.pluginSupervisor == nil {
		return nil, fmt.Errorf("插件运行时未初始化")
	}
	ctx, cancel := contextWithPluginTimeout(a.ctx)
	defer cancel()
	result, err := a.pluginSupervisor.Invoke(ctx, id, method, payload)
	if errors.Is(err, context.DeadlineExceeded) {
		return nil, fmt.Errorf("插件调用超时，结果未知；请刷新插件状态后再决定是否重试")
	}
	return result, err
}

func contextWithPluginTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, 5*time.Second)
}

func (a *CoreApp) submitPluginTelemetry(temp types.TemperatureData) {
	if a.pluginSupervisor == nil || !a.pluginSupervisor.HasTelemetryTargets() {
		return
	}
	snapshot := pluginTelemetrySnapshot(temp, a.pluginTelemetrySequence.Add(1), time.Now().UnixMilli())
	a.pluginSupervisor.SubmitTelemetry(snapshot)
}

func pluginTelemetrySnapshot(temp types.TemperatureData, sequence uint64, fallbackSampledAt int64) plugins.TelemetrySnapshot {
	sampledAt := temp.UpdateTime
	if sampledAt <= 0 {
		sampledAt = fallbackSampledAt
	}
	gpuReadable := temp.GPUReadState != types.GPUReadStateNotPolled &&
		temp.GPUReadState != types.GPUReadStateUnavailable &&
		temp.GPUReadState != types.GPUReadStateError
	return plugins.TelemetrySnapshot{
		Sequence:  sequence,
		SampledAt: sampledAt,
		Payload: plugins.TelemetryPayload{
			CPUTemp: &plugins.TelemetryValue{
				Value: float64(temp.CPUTemp),
				Valid: temp.BridgeOk && temp.CPUTemp > 0,
			},
			GPUTemp: &plugins.TelemetryValue{
				Value: float64(temp.GPUTemp),
				Valid: temp.BridgeOk && gpuReadable && temp.GPUTemp > 0,
			},
			CPUPowerWatts: &plugins.TelemetryValue{
				Value: temp.CPUPowerWatts,
				Valid: temp.BridgeOk && len(temp.CpuPowerSensors) > 0 && finiteNonNegative(temp.CPUPowerWatts),
			},
			GPUPowerWatts: &plugins.TelemetryValue{
				Value: temp.GPUPowerWatts,
				Valid: temp.BridgeOk && gpuReadable && len(temp.GpuPowerSensors) > 0 && finiteNonNegative(temp.GPUPowerWatts),
			},
			BridgeOK: temp.BridgeOk,
		},
	}
}

func finiteNonNegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
