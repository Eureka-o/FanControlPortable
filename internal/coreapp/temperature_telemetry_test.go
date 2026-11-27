package coreapp

import (
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/bridge"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func TestShouldRestartTemperatureBridgeClassifiesPermanentMSRFailure(t *testing.T) {
	tests := []struct {
		name string
		temp types.TemperatureData
		want bool
	}{
		{
			name: "bridge starting",
			temp: types.TemperatureData{BridgeMsg: bridge.StartingMessage},
			want: false,
		},
		{
			name: "permanent msr failure",
			temp: types.TemperatureData{BridgeMsg: "[MSR-UNAVAILABLE] PawnIO installed but raw reads are invalid"},
			want: false,
		},
		{
			name: "pipe eof",
			temp: types.TemperatureData{BridgeMsg: "pipe EOF"},
			want: true,
		},
		{
			name: "responsive bridge returned zero sensors",
			temp: types.TemperatureData{BridgeMsg: "no temperature sensors"},
			want: true,
		},
		{
			name: "healthy bridge",
			temp: types.TemperatureData{BridgeOk: true, BridgeMsg: "pipe EOF"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRestartTemperatureBridge(tt.temp); got != tt.want {
				t.Fatalf("shouldRestartTemperatureBridge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeTemperatureHardwareMetadataKeepsCPUSelectionsDuringTransientEmptyRead(t *testing.T) {
	previous := types.TemperatureData{
		CpuSensors:      []types.TemperatureSensor{{Key: "cpu/package", Name: "CPU Package", Value: 68}},
		CpuPowerSensors: []types.PowerSensor{{Key: "cpu/package-power", Name: "CPU Package", Value: 49}},
	}

	merged := mergeTemperatureHardwareMetadata(previous, types.TemperatureData{
		CpuSensors:      []types.TemperatureSensor{},
		CpuPowerSensors: []types.PowerSensor{},
	})

	if len(merged.CpuSensors) != 1 || merged.CpuSensors[0].Key != "cpu/package" {
		t.Fatalf("CPU sensor metadata was not preserved: %#v", merged.CpuSensors)
	}
	if len(merged.CpuPowerSensors) != 1 || merged.CpuPowerSensors[0].Key != "cpu/package-power" {
		t.Fatalf("CPU power sensor metadata was not preserved: %#v", merged.CpuPowerSensors)
	}
}

func TestMergeTemperatureHardwareMetadataKeepsGPUDevicesDuringTransientEmptyRead(t *testing.T) {
	previous := types.TemperatureData{
		GpuDevices: []types.TemperatureGPUDevice{{
			Key:    "gpu/discrete",
			Name:   "Discrete GPU",
			Vendor: "nvidia",
		}},
	}

	merged := mergeTemperatureHardwareMetadata(previous, types.TemperatureData{
		GPUReadState: types.GPUReadStateActive,
		GpuDevices:   []types.TemperatureGPUDevice{},
	})

	if len(merged.GpuDevices) != 1 || merged.GpuDevices[0].Key != "gpu/discrete" {
		t.Fatalf("gpu devices = %#v, want previous gpu device metadata", merged.GpuDevices)
	}
}

func TestMergeTemperatureHardwareMetadataKeepsMetadataWhenOmitSensorListsReturnsNil(t *testing.T) {
	previous := types.TemperatureData{
		CpuModel:        "Ultra 5",
		GpuModel:        "RTX 5060",
		CpuSensors:      []types.TemperatureSensor{{Key: "cpu/pkg", Name: "CPU Package", Value: 52}},
		GpuSensors:      []types.TemperatureSensor{{Key: "gpu/core", Name: "GPU Core", Value: 44}},
		CpuPowerSensors: []types.PowerSensor{{Key: "cpu/pwr", Name: "CPU Package", Value: 22.1}},
		GpuPowerSensors: []types.PowerSensor{{Key: "gpu/pwr", Name: "GPU Power", Value: 12.3}},
		GpuDevices:      []types.TemperatureGPUDevice{{Key: "gpu0", Name: "RTX 5060", Vendor: "nvidia"}},
	}

	merged := mergeTemperatureHardwareMetadata(previous, types.TemperatureData{
		CPUTemp:         55,
		GPUTemp:         46,
		GPUReadState:    types.GPUReadStateActive,
		CpuSensors:      nil,
		GpuSensors:      nil,
		CpuPowerSensors: nil,
		GpuPowerSensors: nil,
		GpuDevices:      nil,
	})

	if merged.CpuModel != "Ultra 5" || merged.GpuModel != "RTX 5060" {
		t.Fatalf("models = %q / %q, want preserved", merged.CpuModel, merged.GpuModel)
	}
	if len(merged.CpuSensors) != 1 || len(merged.GpuSensors) != 1 ||
		len(merged.CpuPowerSensors) != 1 || len(merged.GpuPowerSensors) != 1 ||
		len(merged.GpuDevices) != 1 {
		t.Fatalf("nil metadata slices from omitSensorLists should preserve previous metadata: %#v", merged)
	}
}

func TestStripSensorListsAndClearSensorCatalogLeases(t *testing.T) {
	app := &CoreApp{}
	app.sensorCatalogActive.Store(true)
	app.gpuSensorProbeActive.Store(true)
	app.currentTemp = types.TemperatureData{
		CPUTemp:         52,
		GPUTemp:         41,
		CpuModel:        "Ultra 5",
		GpuModel:        "RTX 5060",
		CpuSensors:      []types.TemperatureSensor{{Key: "cpu/pkg", Name: "CPU Package", Value: 52}},
		GpuSensors:      []types.TemperatureSensor{{Key: "gpu/core", Name: "GPU Core", Value: 41}},
		CpuPowerSensors: []types.PowerSensor{{Key: "cpu/pwr", Name: "CPU Package", Value: 21.5}},
		GpuPowerSensors: []types.PowerSensor{{Key: "gpu/pwr", Name: "GPU Power", Value: 9.2}},
		GpuDevices:      []types.TemperatureGPUDevice{{Key: "gpu0", Name: "RTX 5060", Vendor: "nvidia"}},
	}

	app.clearSensorCatalogLeases()
	if app.isSensorCatalogActive() || app.isGpuSensorProbeActive() {
		t.Fatal("expected sensor catalog and gpu probe leases to be cleared")
	}
	if app.currentTemp.CPUTemp != 52 || app.currentTemp.CpuModel != "Ultra 5" || app.currentTemp.GpuModel != "RTX 5060" {
		t.Fatalf("scalar telemetry/models should be preserved: %+v", app.currentTemp)
	}
	if app.currentTemp.CpuSensors != nil || app.currentTemp.GpuSensors != nil ||
		app.currentTemp.CpuPowerSensors != nil || app.currentTemp.GpuPowerSensors != nil ||
		app.currentTemp.GpuDevices != nil {
		t.Fatalf("sensor slices should be nil after clearSensorCatalogLeases: %+v", app.currentTemp)
	}
}
