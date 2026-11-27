package guiapp

import (
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/types"
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

func TestIPCRequestRetryableOnlyForReadOnlyRequests(t *testing.T) {
	tests := []struct {
		name  string
		type_ ipc.RequestType
		want  bool
	}{
		{name: "config read", type_: ipc.ReqGetConfig, want: true},
		{name: "temperature read", type_: ipc.ReqGetTemperature, want: true},
		{name: "diagnostics export", type_: ipc.ReqExportDiagnostics, want: true},
		{name: "config write", type_: ipc.ReqUpdateConfig},
		{name: "speed write", type_: ipc.ReqSetCustomSpeed},
		{name: "connect", type_: ipc.ReqConnect},
		{name: "debug command", type_: ipc.ReqSendDeviceDebugCommand},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ipcRequestRetryable(test.type_); got != test.want {
				t.Fatalf("ipcRequestRetryable(%q) = %v, want %v", test.type_, got, test.want)
			}
		})
	}
}

func TestWindowHiddenSuppressesTelemetryAndFlushesLatestOnRestore(t *testing.T) {
	type emittedEvent struct {
		name string
		data any
	}
	var emitted []emittedEvent
	app := &App{
		emitEvent: func(eventName string, optionalData ...any) {
			var payload any
			if len(optionalData) > 0 {
				payload = optionalData[0]
			}
			emitted = append(emitted, emittedEvent{name: eventName, data: payload})
		},
	}

	app.SetWindowHidden(true)
	app.handleCoreEvent(ipc.Event{Type: ipc.EventTemperatureUpdate, Data: []byte(`{"cpuTemp":71,"updateTime":101}`)})
	app.handleCoreEvent(ipc.Event{Type: ipc.EventTemperatureUpdate, Data: []byte(`{"cpuTemp":74,"updateTime":102}`)})
	app.handleCoreEvent(ipc.Event{Type: ipc.EventFanDataUpdate, Data: []byte(`{"currentRpm":1850}`)})
	app.handleCoreEvent(ipc.Event{Type: ipc.EventTemperatureHistoryUpdate, Data: []byte(`{"timestamp":102,"cpuTemp":74}`)})
	app.handleCoreEvent(ipc.Event{Type: ipc.EventBlackSharkHostEffects, Data: []byte(`{"mode":"steady"}`)})
	app.handleCoreEvent(ipc.Event{Type: ipc.EventHeartbeat, Data: []byte(`102`)})
	app.handleCoreEvent(ipc.Event{Type: ipc.EventHealthPing, Data: []byte(`102`)})
	if len(emitted) != 0 {
		t.Fatalf("expected telemetry/heartbeat events to be suppressed while hidden, got %+v", emitted)
	}

	// Non-telemetry lifecycle/config events must still deliver while hidden.
	app.handleCoreEvent(ipc.Event{Type: ipc.EventDeviceConnected, Data: []byte(`{"model":"BS3PRO"}`)})
	app.handleCoreEvent(ipc.Event{Type: ipc.EventConfigUpdate, Data: []byte(`{"autoControl":true}`)})
	if len(emitted) != 2 || emitted[0].name != "device-connected" || emitted[1].name != "config-update" {
		t.Fatalf("expected lifecycle/config events to pass through while hidden, got %+v", emitted)
	}

	// Restoring visibility flushes latest temperature, fan data, and host effects once.
	emitted = nil
	app.SetWindowHidden(false)
	if len(emitted) != 3 {
		t.Fatalf("expected 3 flushed telemetry events on restore, got %+v", emitted)
	}
	if emitted[0].name != "temperature-update" {
		t.Fatalf("expected first flushed event temperature-update, got %q", emitted[0].name)
	}
	if temp, ok := emitted[0].data.(types.TemperatureData); !ok || temp.CPUTemp != 74 {
		t.Fatalf("expected latest merged cpuTemp=74 on restore, got %#v", emitted[0].data)
	}
	if emitted[1].name != "fan-data-update" || emitted[2].name != "blackshark-host-effects" {
		t.Fatalf("unexpected flush order/names: %+v", emitted)
	}

	// Calling SetWindowHidden(false) again without new hidden telemetry does not re-emit.
	emitted = nil
	app.SetWindowHidden(false)
	if len(emitted) != 0 {
		t.Fatalf("expected duplicate restore not to re-flush, got %+v", emitted)
	}
}
