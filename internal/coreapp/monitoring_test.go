package coreapp

import (
	"context"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestTemperatureMonitoringRestartWaitsForPreviousRun(t *testing.T) {
	app := &CoreApp{ctx: context.Background()}
	oldCtx, oldDone, started := app.beginTemperatureMonitoring()
	if !started {
		t.Fatal("first monitoring run did not start")
	}

	app.stopTemperatureMonitoring()
	select {
	case <-oldCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("stop did not cancel the current monitoring run")
	}

	restarted := make(chan chan struct{}, 1)
	go func() {
		_, done, ok := app.beginTemperatureMonitoring()
		if ok {
			restarted <- done
		}
	}()
	select {
	case <-restarted:
		t.Fatal("new monitoring run started before the previous run exited")
	case <-time.After(20 * time.Millisecond):
	}

	app.finishTemperatureMonitoring(oldDone)
	select {
	case newDone := <-restarted:
		app.finishTemperatureMonitoring(newDone)
	case <-time.After(time.Second):
		t.Fatal("new monitoring run was lost after the previous run exited")
	}
}

func TestActiveTemperatureMonitorIntervalReturnsFromIdleToForeground(t *testing.T) {
	if got := activeTemperatureMonitorInterval(1, false, false); got != idleTemperatureMonitorInterval {
		t.Fatalf("idle interval = %v, want %v", got, idleTemperatureMonitorInterval)
	}
	if got := activeTemperatureMonitorInterval(1, true, false); got != time.Second {
		t.Fatalf("foreground interval = %v, want 1s", got)
	}
}

func TestSmartControlSampleContextChangesForTelemetryOrSpeedUnit(t *testing.T) {
	selection := types.TemperatureSelection{
		TempSource:     types.TempSourceCPU,
		CpuSensor:      "cpu/package",
		CpuPowerSensor: "cpu/package-power",
		GpuDevice:      types.TempDeviceAuto,
		GpuReadMode:    types.GPUReadModeAuto,
	}
	base := newSmartControlSampleContext(selection, types.FanSpeedUnitRPM)

	if base != newSmartControlSampleContext(selection, types.FanSpeedUnitRPM) {
		t.Fatal("same telemetry context must compare equal")
	}
	selection.CpuPowerSensor = "cpu/core-power"
	if base == newSmartControlSampleContext(selection, types.FanSpeedUnitRPM) {
		t.Fatal("power-sensor change must reset smart-control samples")
	}
	selection.CpuPowerSensor = "cpu/package-power"
	if base == newSmartControlSampleContext(selection, types.FanSpeedUnitPercent) {
		t.Fatal("speed-unit change must reset smart-control samples")
	}
}

func TestEffectiveTemperaturePowerRequiresFreshTelemetry(t *testing.T) {
	stale := types.TemperatureData{
		CPUTemp:       68,
		GPUTemp:       72,
		ControlTemp:   72,
		CPUPowerWatts: 45,
		GPUPowerWatts: 85,
		GPUReadState:  types.GPUReadStateActive,
	}
	if got := effectiveTemperaturePower(stale); got.CPUValid || got.GPUValid {
		t.Fatalf("stale telemetry must not provide effective power: %#v", got)
	}

	fresh := stale
	fresh.TelemetryFresh = true
	got := effectiveTemperaturePower(fresh)
	if !got.CPUValid || !got.GPUValid {
		t.Fatalf("fresh CPU/GPU power must be valid: %#v", got)
	}

	fresh.GPUReadState = types.GPUReadStateNotPolled
	got = effectiveTemperaturePower(fresh)
	if !got.CPUValid || got.GPUValid {
		t.Fatalf("not-polled GPU must not contribute effective power: %#v", got)
	}
}

func TestEffectiveTemperaturePowerIsolatesMissingCpuPowerAndBadCpuTemperature(t *testing.T) {
	base := types.TemperatureData{
		TelemetryFresh: true,
		CPUTemp:        68,
		GPUTemp:        72,
		ControlTemp:    72,
		GPUPowerWatts:  85,
		GPUReadState:   types.GPUReadStateActive,
	}
	if got := effectiveTemperaturePower(base); got.CPUValid || !got.GPUValid {
		t.Fatalf("missing CPU power must remain unknown without losing GPU power: %#v", got)
	}

	badCPU := base
	badCPU.CPUTemp = 150
	badCPU.CPUPowerWatts = 45
	if got := effectiveTemperaturePower(badCPU); got.CPUValid || !got.GPUValid {
		t.Fatalf("bad CPU temperature must isolate only CPU power: %#v", got)
	}

	badControl := badCPU
	badControl.ControlTemp = badCPU.CPUTemp
	if got := effectiveTemperaturePower(badControl); got.CPUValid || got.GPUValid {
		t.Fatalf("bad control temperature must not enter prediction or learning: %#v", got)
	}
}

func TestSmartControlTelemetryIsolationKeepsValidGPUWhenCPUIsBad(t *testing.T) {
	temp := types.TemperatureData{
		TelemetryFresh: true,
		CPUTemp:        150,
		GPUTemp:        72,
		ControlTemp:    72,
		CPUPowerWatts:  45,
		GPUPowerWatts:  85,
		GPUReadState:   types.GPUReadStateActive,
		ControlSource:  types.TempSourceMax,
	}
	if !hasUsableSmartControlTelemetry(temp) {
		t.Fatal("valid max-control GPU telemetry must remain usable when only CPU telemetry is bad")
	}
	stale := temp
	stale.TelemetryFresh = false
	if hasUsableSmartControlTelemetry(stale) {
		t.Fatal("stale telemetry must not enter prediction or learning")
	}

	power := effectiveTemperaturePower(temp)
	sample := newSmartControlRisePredictionSample(time.Now(), temp, temp.ControlTemp, power, 1800, 1700, true)
	if sample.CPUTemp != 0 || sample.CPUPowerValid {
		t.Fatalf("bad CPU telemetry must be removed from prediction sample: %#v", sample)
	}
	if sample.GPUTemp != 72 || !sample.GPUPowerValid || sample.ControlSource != types.TempSourceMax {
		t.Fatalf("valid GPU telemetry must remain intact: %#v", sample)
	}

	temp.ControlSource = types.TempSourceCPU
	temp.ControlTemp = 68
	if hasUsableSmartControlTelemetry(temp) {
		t.Fatal("bad CPU telemetry must not be usable when CPU is the control source")
	}

	temp.ControlSource = types.TempSourceMax
	temp.ControlTemp = 150
	if hasUsableSmartControlTelemetry(temp) {
		t.Fatal("abnormal control temperature must never enter prediction or learning")
	}
}

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

func TestTemperatureSafetyFallbackTransitions(t *testing.T) {
	var state temperatureSafetyFallback
	for sample := 1; sample < temperatureSafetyFallbackThreshold; sample++ {
		apply, recovered := state.observe(true, false)
		if apply || recovered {
			t.Fatalf("invalid sample %d transitioned early: apply=%v recovered=%v", sample, apply, recovered)
		}
	}
	apply, recovered := state.observe(true, false)
	if !apply || recovered {
		t.Fatalf("threshold transition = apply %v recovered %v, want true/false", apply, recovered)
	}
	state.markApplied()
	if apply, _ := state.observe(true, false); apply {
		t.Fatal("active safety fallback requested a duplicate write")
	}
	apply, recovered = state.observe(true, true)
	if apply || !recovered {
		t.Fatalf("recovery transition = apply %v recovered %v, want false/true", apply, recovered)
	}
	if apply, recovered = state.observe(false, false); apply || recovered || state.invalidSamples != 0 || state.active {
		t.Fatalf("disabled automatic control did not reset fallback state: %#v", state)
	}
}

func TestTemperatureSafetyFallbackTargetUsesCurveMaximumAndDeviceLimit(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 40, RPM: 1200}, {Temperature: 80, RPM: 4000}}
	capability := types.DecodeFlyDigiRuntimeCapabilityFromGearSettings(0x4A, nil)
	fanData := &types.FanData{
		Transport:         types.DeviceTransportHID,
		SpeedUnit:         types.FanSpeedUnitRPM,
		GearSettings:      0x4A,
		FlyDigiCapability: &capability,
	}
	if got := temperatureSafetyFallbackTarget(curve, types.FanSpeedUnitRPM, fanData); got != 3300 {
		t.Fatalf("RPM safety target = %d, want capability-limited 3300", got)
	}
	percentCurve := []types.FanCurvePoint{{Temperature: 40, RPM: 30}, {Temperature: 80, RPM: 100}}
	if got := temperatureSafetyFallbackTarget(percentCurve, types.FanSpeedUnitPercent, nil); got != 1000 {
		t.Fatalf("percent safety target = %d, want 1000 ticks", got)
	}
}
