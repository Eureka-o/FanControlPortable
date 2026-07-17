package temperature

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

type testLogger struct{}

func (testLogger) Info(string, ...any)  {}
func (testLogger) Error(string, ...any) {}
func (testLogger) Warn(string, ...any)  {}
func (testLogger) Debug(string, ...any) {}
func (testLogger) Close()               {}
func (testLogger) CleanOldLogs()        {}
func (testLogger) SetDebugMode(bool)    {}
func (testLogger) GetLogDir() string    { return "" }

type fakeBridgeTemperatureProvider struct {
	responses []types.BridgeTemperatureData
	calls     int
}

func TestResolveControlTempFallsBackToAvailableSensor(t *testing.T) {
	if got := resolveControlTemp(0, 67, types.TempSourceCPU); got != 67 {
		t.Fatalf("CPU source fallback = %d, want 67", got)
	}
	if got := resolveControlTemp(58, 0, types.TempSourceGPU); got != 58 {
		t.Fatalf("GPU source fallback = %d, want 58", got)
	}
	if got := resolveControlTemp(0, 0, types.TempSourceGPU); got != 0 {
		t.Fatalf("empty fallback = %d, want 0", got)
	}
}

func (f *fakeBridgeTemperatureProvider) GetTemperature(types.TemperatureSelection) types.BridgeTemperatureData {
	if f.calls >= len(f.responses) {
		return types.BridgeTemperatureData{Success: false, Error: "unexpected bridge call"}
	}
	response := f.responses[f.calls]
	f.calls++
	return response
}

func TestDetectGPUVendorCachesResult(t *testing.T) {
	oldExec := execHelperCommand
	oldNow := readTimeNow
	defer func() {
		execHelperCommand = oldExec
		readTimeNow = oldNow
	}()

	now := time.Unix(1_717_000_000, 0)
	readTimeNow = func() time.Time { return now }

	calls := 0
	execHelperCommand = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
		calls++
		if timeout != helperCommandTimeout {
			t.Fatalf("unexpected timeout: %s", timeout)
		}
		if name != "nvidia-smi" {
			t.Fatalf("unexpected command: %s", name)
		}
		return []byte("NVIDIA-SMI 555.00"), nil
	}

	r := NewReader(nil, testLogger{})
	if got := r.detectGPUVendor(); got != "nvidia" {
		t.Fatalf("detectGPUVendor() = %q, want nvidia", got)
	}
	if got := r.detectGPUVendor(); got != "nvidia" {
		t.Fatalf("detectGPUVendor() cached = %q, want nvidia", got)
	}
	if calls != 1 {
		t.Fatalf("detectGPUVendor() calls = %d, want 1 with cache hit", calls)
	}

	now = now.Add(gpuVendorCacheTTL + time.Second)
	if got := r.detectGPUVendor(); got != "nvidia" {
		t.Fatalf("detectGPUVendor() after ttl = %q, want nvidia", got)
	}
	if calls != 2 {
		t.Fatalf("detectGPUVendor() calls after ttl = %d, want 2", calls)
	}
}

func TestReadWindowsCPUTempUsesTimeout(t *testing.T) {
	oldExec := execHelperCommand
	defer func() {
		execHelperCommand = oldExec
	}()

	called := false
	execHelperCommand = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
		called = true
		if timeout != helperCommandTimeout {
			t.Fatalf("unexpected timeout: %s", timeout)
		}
		if name != "wmic" {
			t.Fatalf("unexpected command: %s", name)
		}
		return nil, context.DeadlineExceeded
	}

	r := NewReader(nil, testLogger{})
	if got := r.readWindowsCPUTemp(); got != 0 {
		t.Fatalf("readWindowsCPUTemp() = %d, want 0 on timeout", got)
	}
	if !called {
		t.Fatal("readWindowsCPUTemp() did not invoke helper command")
	}
}

func TestReadUsesRecentBridgeTemperatureOnTransientFailure(t *testing.T) {
	oldNow := readTimeNow
	defer func() {
		readTimeNow = oldNow
	}()

	now := time.Unix(1_717_000_000, 0)
	readTimeNow = func() time.Time { return now }

	bridge := &fakeBridgeTemperatureProvider{
		responses: []types.BridgeTemperatureData{
			{
				Success:           true,
				CpuTemp:           61,
				GpuTemp:           54,
				CpuPowerWatts:     22.5,
				GpuPowerWatts:     31.25,
				ControlTemp:       61,
				ControlSource:     types.TempSourceCPU,
				GPUReadState:      types.GPUReadStateActive,
				SelectedGpuDevice: "gpu/nvidia/test",
			},
			{
				Success: false,
				Error:   "timeout",
			},
		},
	}

	reader := NewReader(bridge, testLogger{})
	selection := types.GetDefaultTemperatureSelection()
	selection.TempSource = types.TempSourceCPU

	first := reader.Read(selection)
	if !first.BridgeOk || first.CPUTemp != 61 || first.GPUTemp != 54 {
		t.Fatalf("first read = %+v, want successful bridge data", first)
	}

	now = now.Add(5 * time.Second)
	second := reader.Read(selection)
	if !second.BridgeOk {
		t.Fatalf("second read BridgeOk = false, want cached bridge data")
	}
	if second.BridgeMsg != "" {
		t.Fatalf("second read BridgeMsg = %q, want empty", second.BridgeMsg)
	}
	if second.CPUTemp != 61 || second.GPUTemp != 54 || second.CPUPowerWatts != 22.5 || second.GPUPowerWatts != 31.25 {
		t.Fatalf("second read = %+v, want last valid bridge values", second)
	}
}

func TestReadTelemetryFreshOnlyForDirectBridgeData(t *testing.T) {
	oldNow := readTimeNow
	defer func() {
		readTimeNow = oldNow
	}()

	now := time.Unix(1_717_000_000, 0)
	readTimeNow = func() time.Time { return now }

	bridge := &fakeBridgeTemperatureProvider{
		responses: []types.BridgeTemperatureData{
			{
				Success:       true,
				CpuTemp:       61,
				ControlTemp:   61,
				ControlSource: types.TempSourceCPU,
			},
			{
				Success: false,
				Error:   "timeout",
			},
		},
	}

	reader := NewReader(bridge, testLogger{})
	selection := types.GetDefaultTemperatureSelection()
	selection.TempSource = types.TempSourceCPU

	direct := reader.Read(selection)
	if !direct.TelemetryFresh {
		t.Fatal("direct bridge data TelemetryFresh = false, want true")
	}

	now = now.Add(time.Second)
	cached := reader.Read(selection)
	if cached.TelemetryFresh {
		t.Fatal("cached bridge data TelemetryFresh = true, want false")
	}
}

func TestTelemetryStateClassifiesFreshDelayedAndUnavailable(t *testing.T) {
	cases := []struct {
		name string
		temp types.TemperatureData
		want string
	}{
		{name: "fresh", temp: types.TemperatureData{BridgeOk: true, TelemetryFresh: true, ControlTemp: 60}, want: types.TelemetryStateFresh},
		{name: "delayed", temp: types.TemperatureData{BridgeOk: true, ControlTemp: 60}, want: types.TelemetryStateDelayed},
		{name: "bridge unavailable", temp: types.TemperatureData{ControlTemp: 60}, want: types.TelemetryStateUnavailable},
		{name: "invalid temperature", temp: types.TemperatureData{BridgeOk: true, TelemetryFresh: true}, want: types.TelemetryStateUnavailable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := telemetryStateFor(tc.temp); got != tc.want {
				t.Fatalf("telemetryStateFor() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadTelemetryFreshIsFalseForLocalFallback(t *testing.T) {
	oldExec := execHelperCommand
	defer func() {
		execHelperCommand = oldExec
	}()
	execHelperCommand = func(time.Duration, string, ...string) ([]byte, error) {
		return nil, errors.New("unavailable")
	}

	bridge := &fakeBridgeTemperatureProvider{
		responses: []types.BridgeTemperatureData{{
			Success:      true,
			GPUReadState: types.GPUReadStateNotPolled,
		}},
	}
	reader := NewReader(bridge, testLogger{})

	if got := reader.Read(types.GetDefaultTemperatureSelection()); got.TelemetryFresh {
		t.Fatal("local fallback TelemetryFresh = true, want false")
	}
}

func TestTelemetryFreshIsNotSerialized(t *testing.T) {
	encoded, err := json.Marshal(types.TemperatureData{TelemetryFresh: true})
	if err != nil {
		t.Fatalf("marshal TemperatureData: %v", err)
	}
	if strings.Contains(string(encoded), "telemetryFresh") {
		t.Fatalf("serialized TemperatureData = %s, must not include TelemetryFresh", encoded)
	}
}

func TestReadDoesNotUseExpiredBridgeTemperatureCache(t *testing.T) {
	oldNow := readTimeNow
	defer func() {
		readTimeNow = oldNow
	}()

	now := time.Unix(1_717_000_000, 0)
	readTimeNow = func() time.Time { return now }

	bridge := &fakeBridgeTemperatureProvider{
		responses: []types.BridgeTemperatureData{
			{
				Success:       true,
				CpuTemp:       61,
				GpuTemp:       54,
				ControlTemp:   61,
				ControlSource: types.TempSourceCPU,
			},
			{
				Success: false,
				Error:   "timeout",
			},
		},
	}

	reader := NewReader(bridge, testLogger{})
	selection := types.GetDefaultTemperatureSelection()
	selection.TempSource = types.TempSourceCPU

	first := reader.Read(selection)
	if !first.BridgeOk {
		t.Fatalf("first read BridgeOk = false, want successful bridge data")
	}

	now = now.Add(bridgeRecoveryTemperatureTTL + time.Second)
	second := reader.Read(selection)
	if second.BridgeOk {
		t.Fatalf("second read BridgeOk = true, want expired cache to expose bridge failure")
	}
	if second.BridgeMsg != "timeout" {
		t.Fatalf("second read BridgeMsg = %q, want timeout", second.BridgeMsg)
	}
	if second.TelemetryFresh {
		t.Fatal("second read TelemetryFresh = true, want false after bridge failure")
	}
}
