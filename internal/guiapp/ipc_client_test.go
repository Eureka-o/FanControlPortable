package guiapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestMergeTemperatureMetadata(t *testing.T) {
	previous := types.TemperatureData{
		CpuModel:          "Ryzen",
		GpuModel:          "RTX",
		SelectedGpuDevice: "gpu0",
		CpuSensors:        []types.TemperatureSensor{{Key: "cpu-package", Name: "CPU Package", Value: 70}},
		GpuSensors:        []types.TemperatureSensor{{Key: "gpu-core", Name: "GPU Core", Value: 66}},
		CpuPowerSensors:   []types.PowerSensor{{Key: "cpu-package-power", Name: "CPU Package", Value: 42.5}},
		GpuPowerSensors:   []types.PowerSensor{{Key: "gpu-board-power", Name: "GPU Board", Value: 88.5}},
		GpuDevices:        []types.TemperatureGPUDevice{{Key: "gpu0", Name: "GPU 0", Vendor: "nvidia"}},
	}

	incomingCompact := types.TemperatureData{CPUTemp: 72, GPUTemp: 67}
	mergedCompact := mergeTemperatureMetadata(previous, incomingCompact)
	if len(mergedCompact.CpuSensors) != 1 || mergedCompact.CpuSensors[0].Key != "cpu-package" {
		t.Fatal("mergeTemperatureMetadata() should preserve previous cpuSensors when incoming payload omits them")
	}
	if len(mergedCompact.GpuDevices) != 1 || mergedCompact.GpuDevices[0].Key != "gpu0" {
		t.Fatal("mergeTemperatureMetadata() should preserve previous gpuDevices when incoming payload omits them")
	}
	if len(mergedCompact.CpuPowerSensors) != 1 || mergedCompact.CpuPowerSensors[0].Key != "cpu-package-power" {
		t.Fatal("mergeTemperatureMetadata() should preserve previous cpuPowerSensors when incoming payload omits them")
	}
	if len(mergedCompact.GpuPowerSensors) != 1 || mergedCompact.GpuPowerSensors[0].Key != "gpu-board-power" {
		t.Fatal("mergeTemperatureMetadata() should preserve previous gpuPowerSensors when incoming payload omits them")
	}
	if mergedCompact.CpuModel != "Ryzen" || mergedCompact.GpuModel != "RTX" {
		t.Fatal("mergeTemperatureMetadata() should preserve previous model metadata when incoming payload omits it")
	}

	incomingClear := types.TemperatureData{CpuSensors: []types.TemperatureSensor{}, CpuPowerSensors: []types.PowerSensor{}, GpuDevices: []types.TemperatureGPUDevice{}}
	mergedClear := mergeTemperatureMetadata(previous, incomingClear)
	if mergedClear.CpuSensors == nil || len(mergedClear.CpuSensors) != 0 {
		t.Fatal("mergeTemperatureMetadata() should keep explicit empty cpuSensors to allow clearing metadata")
	}
	if mergedClear.CpuPowerSensors == nil || len(mergedClear.CpuPowerSensors) != 0 {
		t.Fatal("mergeTemperatureMetadata() should keep explicit empty cpuPowerSensors to allow clearing metadata")
	}
	if mergedClear.GpuDevices == nil || len(mergedClear.GpuDevices) != 0 {
		t.Fatal("mergeTemperatureMetadata() should keep explicit empty gpuDevices to allow clearing metadata")
	}
}
