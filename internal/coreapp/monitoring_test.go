package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestCompactTemperatureEventPayload(t *testing.T) {
	sharedCPUSensors := []types.TemperatureSensor{{Key: "cpu-package", Name: "CPU Package", Value: 71}}
	sharedGPUSensors := []types.TemperatureSensor{{Key: "gpu-core", Name: "GPU Core", Value: 66}}
	sharedCPUPowerSensors := []types.PowerSensor{{Key: "cpu-package-power", Name: "CPU Package", Value: 45.5}}
	sharedGPUPowerSensors := []types.PowerSensor{{Key: "gpu-board-power", Name: "GPU Board", Value: 88.5}}
	sharedGPUDevices := []types.TemperatureGPUDevice{{
		Key:    "gpu0",
		Name:   "GPU 0",
		Vendor: "nvidia",
		Sensors: []types.TemperatureSensor{{
			Key:   "gpu-core",
			Name:  "GPU Core",
			Value: 66,
		}},
		PowerSensors: []types.PowerSensor{{
			Key:   "gpu-board-power",
			Name:  "GPU Board",
			Value: 88.5,
		}},
	}}

	previous := types.TemperatureData{
		CPUTemp:         70,
		CpuSensors:      sharedCPUSensors,
		GpuSensors:      sharedGPUSensors,
		CpuPowerSensors: sharedCPUPowerSensors,
		GpuPowerSensors: sharedGPUPowerSensors,
		GpuDevices:      sharedGPUDevices,
	}
	current := previous
	current.CPUTemp = 72

	compact := compactTemperatureEventPayload(current, previous)
	if compact.CpuSensors != nil {
		t.Fatal("compactTemperatureEventPayload() should strip unchanged cpuSensors")
	}
	if compact.GpuSensors != nil {
		t.Fatal("compactTemperatureEventPayload() should strip unchanged gpuSensors")
	}
	if compact.CpuPowerSensors != nil {
		t.Fatal("compactTemperatureEventPayload() should strip unchanged cpuPowerSensors")
	}
	if compact.GpuPowerSensors != nil {
		t.Fatal("compactTemperatureEventPayload() should strip unchanged gpuPowerSensors")
	}
	if compact.GpuDevices != nil {
		t.Fatal("compactTemperatureEventPayload() should strip unchanged gpuDevices")
	}

	changed := current
	changed.GpuSensors = []types.TemperatureSensor{{Key: "gpu-hotspot", Name: "GPU Hotspot", Value: 77}}
	compactChanged := compactTemperatureEventPayload(changed, previous)
	if len(compactChanged.GpuSensors) != 1 || compactChanged.GpuSensors[0].Key != "gpu-hotspot" {
		t.Fatal("compactTemperatureEventPayload() should keep changed gpuSensors")
	}

	changedPower := current
	changedPower.GpuPowerSensors = []types.PowerSensor{{Key: "gpu-chip-power", Name: "GPU Chip", Value: 76.5}}
	compactChangedPower := compactTemperatureEventPayload(changedPower, previous)
	if len(compactChangedPower.GpuPowerSensors) != 1 || compactChangedPower.GpuPowerSensors[0].Key != "gpu-chip-power" {
		t.Fatal("compactTemperatureEventPayload() should keep changed gpuPowerSensors")
	}

	cleared := current
	cleared.CpuSensors = []types.TemperatureSensor{}
	compactCleared := compactTemperatureEventPayload(cleared, previous)
	if compactCleared.CpuSensors == nil {
		t.Fatal("compactTemperatureEventPayload() should keep explicit empty cpuSensors to clear stale metadata")
	}
	if len(compactCleared.CpuSensors) != 0 {
		t.Fatalf("compactTemperatureEventPayload() kept unexpected cpuSensors length: %d", len(compactCleared.CpuSensors))
	}

	valueOnlyChanged := current
	valueOnlyChanged.CpuSensors = []types.TemperatureSensor{{Key: "cpu-package", Name: "CPU Package", Value: 73}}
	compactValueOnlyChanged := compactTemperatureEventPayload(valueOnlyChanged, previous)
	if compactValueOnlyChanged.CpuSensors != nil {
		t.Fatal("compactTemperatureEventPayload() should strip value-only sensor changes")
	}

	explicitEmptyPrevious := types.TemperatureData{CpuSensors: nil}
	explicitEmptyCurrent := types.TemperatureData{CpuSensors: []types.TemperatureSensor{}}
	compactExplicitEmpty := compactTemperatureEventPayload(explicitEmptyCurrent, explicitEmptyPrevious)
	if compactExplicitEmpty.CpuSensors == nil {
		t.Fatal("compactTemperatureEventPayload() should keep empty cpuSensors when previous was nil")
	}
}

func TestMergeTemperatureHardwareMetadataKeepsProfileWhenGpuNotPolled(t *testing.T) {
	previous := types.TemperatureData{
		CpuModel: "Ryzen 7",
		GpuModel: "GeForce RTX 4060",
		CpuSensors: []types.TemperatureSensor{{
			Key:   "cpu-package",
			Name:  "CPU Package",
			Value: 71,
		}},
		GpuSensors: []types.TemperatureSensor{{
			Key:   "gpu-core",
			Name:  "GPU Core",
			Value: 66,
		}},
		GpuPowerSensors: []types.PowerSensor{{
			Key:   "gpu-board-power",
			Name:  "GPU Board",
			Value: 88.5,
		}},
		GpuDevices: []types.TemperatureGPUDevice{{
			Key:    "gpu0",
			Name:   "GeForce RTX 4060",
			Vendor: "nvidia",
		}},
	}
	incoming := types.TemperatureData{
		CPUTemp:      72,
		GPUReadState: types.GPUReadStateNotPolled,
		GpuSensors:   []types.TemperatureSensor{},
		GpuDevices:   []types.TemperatureGPUDevice{},
	}

	got := mergeTemperatureHardwareMetadata(previous, incoming)
	if got.CpuModel != previous.CpuModel {
		t.Fatalf("cpu model = %q, want %q", got.CpuModel, previous.CpuModel)
	}
	if got.GpuModel != previous.GpuModel {
		t.Fatalf("gpu model = %q, want %q", got.GpuModel, previous.GpuModel)
	}
	if len(got.GpuSensors) != 1 || got.GpuSensors[0].Key != "gpu-core" {
		t.Fatalf("gpu sensors = %#v, want previous gpu sensor metadata", got.GpuSensors)
	}
	if len(got.GpuPowerSensors) != 1 || got.GpuPowerSensors[0].Key != "gpu-board-power" {
		t.Fatalf("gpu power sensors = %#v, want previous gpu power metadata", got.GpuPowerSensors)
	}
	if len(got.GpuDevices) != 1 || got.GpuDevices[0].Name != "GeForce RTX 4060" {
		t.Fatalf("gpu devices = %#v, want previous gpu device metadata", got.GpuDevices)
	}
}

func TestTrackBridgeTemperatureStaleness(t *testing.T) {
	tests := []struct {
		name           string
		temp           types.TemperatureData
		lastUpdate     int64
		staleCount     int
		wantUpdate     int64
		wantStaleCount int
		wantRestartNow bool
	}{
		{
			name:           "reset when bridge is not ok",
			temp:           types.TemperatureData{BridgeOk: false, UpdateTime: 1000},
			lastUpdate:     1000,
			staleCount:     2,
			wantUpdate:     0,
			wantStaleCount: 0,
		},
		{
			name:           "accept fresh update time",
			temp:           types.TemperatureData{BridgeOk: true, UpdateTime: 2000},
			lastUpdate:     1000,
			staleCount:     2,
			wantUpdate:     2000,
			wantStaleCount: 0,
		},
		{
			name:           "trigger restart after repeated stale update",
			temp:           types.TemperatureData{BridgeOk: true, UpdateTime: 2000},
			lastUpdate:     2000,
			staleCount:     staleBridgeUpdateThreshold - 1,
			wantUpdate:     2000,
			wantStaleCount: staleBridgeUpdateThreshold,
			wantRestartNow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUpdate, gotStaleCount, gotRestartNow := trackBridgeTemperatureStaleness(tt.temp, tt.lastUpdate, tt.staleCount)
			if gotUpdate != tt.wantUpdate {
				t.Fatalf("trackBridgeTemperatureStaleness() update = %d, want %d", gotUpdate, tt.wantUpdate)
			}
			if gotStaleCount != tt.wantStaleCount {
				t.Fatalf("trackBridgeTemperatureStaleness() staleCount = %d, want %d", gotStaleCount, tt.wantStaleCount)
			}
			if gotRestartNow != tt.wantRestartNow {
				t.Fatalf("trackBridgeTemperatureStaleness() restart = %v, want %v", gotRestartNow, tt.wantRestartNow)
			}
		})
	}
}

func TestShouldSendTargetSpeedWakesStoppedFan(t *testing.T) {
	tests := []struct {
		name    string
		fanData *types.FanData
		unit    string
	}{
		{
			name:    "rpm current zero",
			unit:    types.FanSpeedUnitRPM,
			fanData: &types.FanData{CurrentRPM: 0, TargetRPM: 1500},
		},
		{
			name:    "rpm target zero",
			unit:    types.FanSpeedUnitRPM,
			fanData: &types.FanData{CurrentRPM: 1200, TargetRPM: 0},
		},
		{
			name:    "percent current zero",
			unit:    types.FanSpeedUnitPercent,
			fanData: &types.FanData{CurrentRPM: 0, TargetRPM: 40},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !shouldSendTargetSpeed(500, 500, 20, tt.fanData, tt.unit) {
				t.Fatal("shouldSendTargetSpeed() should resend a positive target when device reports stopped/zero target")
			}
		})
	}
}

func TestShouldSendTargetSpeedIgnoresSmallStableDrift(t *testing.T) {
	fanData := &types.FanData{CurrentRPM: 1505, TargetRPM: 1504}
	if shouldSendTargetSpeed(1500, 1500, 20, fanData, types.FanSpeedUnitRPM) {
		t.Fatal("shouldSendTargetSpeed() should not resend for small stable RPM drift")
	}
}

func TestApplyFlyDigiRuntimeCapabilityToTargetClampsHIDRPM(t *testing.T) {
	capability := types.DecodeFlyDigiRuntimeCapabilityFromGearSettings(0x4A, nil)
	fanData := &types.FanData{
		Transport:         types.DeviceTransportHID,
		SpeedUnit:         types.FanSpeedUnitRPM,
		GearSettings:      0x4A,
		FlyDigiCapability: &capability,
	}

	got, limited := applyFlyDigiRuntimeCapabilityToTarget(4000, fanData, types.FanSpeedUnitRPM)
	if got != 3300 || !limited {
		t.Fatalf("applyFlyDigiRuntimeCapabilityToTarget() = (%d, %v), want (3300, true)", got, limited)
	}
}

func TestApplyFlyDigiRuntimeCapabilityToTargetLeavesNonFlyDigiPathsAlone(t *testing.T) {
	fanData := &types.FanData{Transport: types.DeviceTransportWiFi, SpeedUnit: types.FanSpeedUnitRPM}
	got, limited := applyFlyDigiRuntimeCapabilityToTarget(4000, fanData, types.FanSpeedUnitRPM)
	if got != 4000 || limited {
		t.Fatalf("WiFi target should not be limited: (%d, %v)", got, limited)
	}

	hidPercent := &types.FanData{Transport: types.DeviceTransportHID, SpeedUnit: types.FanSpeedUnitPercent, GearSettings: 0x2A}
	got, limited = applyFlyDigiRuntimeCapabilityToTarget(500, hidPercent, types.FanSpeedUnitPercent)
	if got != 500 || limited {
		t.Fatalf("percent target should not be limited: (%d, %v)", got, limited)
	}
}
