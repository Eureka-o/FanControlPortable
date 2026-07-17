package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestPluginTelemetrySnapshotPreservesValidityAndTimestamp(t *testing.T) {
	temp := types.TemperatureData{
		CPUTemp:         62,
		GPUTemp:         58,
		CPUPowerWatts:   24.5,
		GPUPowerWatts:   70,
		GPUReadState:    types.GPUReadStateNotPolled,
		CpuPowerSensors: []types.PowerSensor{{Key: "cpu", Value: 24.5}},
		GpuPowerSensors: []types.PowerSensor{{Key: "gpu", Value: 70}},
		UpdateTime:      1234,
		BridgeOk:        true,
	}
	snapshot := pluginTelemetrySnapshot(temp, 9, 9999)
	if snapshot.Sequence != 9 || snapshot.SampledAt != 1234 {
		t.Fatalf("snapshot identity = %#v", snapshot)
	}
	if snapshot.Payload.CPUTemp == nil || !snapshot.Payload.CPUTemp.Valid {
		t.Fatalf("cpu temperature = %#v", snapshot.Payload.CPUTemp)
	}
	if snapshot.Payload.GPUTemp == nil || snapshot.Payload.GPUTemp.Valid {
		t.Fatalf("gpu temperature = %#v", snapshot.Payload.GPUTemp)
	}
	if snapshot.Payload.CPUPowerWatts == nil || !snapshot.Payload.CPUPowerWatts.Valid {
		t.Fatalf("cpu power = %#v", snapshot.Payload.CPUPowerWatts)
	}
	if snapshot.Payload.GPUPowerWatts == nil || snapshot.Payload.GPUPowerWatts.Valid {
		t.Fatalf("gpu power = %#v", snapshot.Payload.GPUPowerWatts)
	}
}

func TestPluginTelemetrySnapshotUsesFallbackTimestampAndRejectsBridgeFailure(t *testing.T) {
	snapshot := pluginTelemetrySnapshot(types.TemperatureData{CPUTemp: 60, BridgeOk: false}, 1, 5678)
	if snapshot.SampledAt != 5678 {
		t.Fatalf("sampledAt = %d", snapshot.SampledAt)
	}
	if snapshot.Payload.CPUTemp == nil || snapshot.Payload.CPUTemp.Valid {
		t.Fatalf("cpu temperature = %#v", snapshot.Payload.CPUTemp)
	}
}
