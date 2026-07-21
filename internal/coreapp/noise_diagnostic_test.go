package coreapp

import (
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestNoiseDiagnosticLeaseActiveAndCancellationAreIdempotent(t *testing.T) {
	app := &CoreApp{}
	app.noiseDiagnosticLease = &noiseDiagnosticLease{
		sessionID: "session-1",
		expiresAt: time.Now().Add(time.Minute),
		done:      make(chan struct{}),
	}
	if !app.noiseDiagnosticLeaseActive() {
		t.Fatal("active noise diagnostic lease reported inactive")
	}
	if err := app.CancelNoiseDiagnostic("session-1"); err != nil {
		t.Fatalf("CancelNoiseDiagnostic() error = %v", err)
	}
	if app.noiseDiagnosticLeaseActive() {
		t.Fatal("noise diagnostic lease remains active after cancellation")
	}
	if err := app.CancelNoiseDiagnostic("session-1"); err != nil {
		t.Fatalf("second CancelNoiseDiagnostic() error = %v", err)
	}
}

func TestNoiseDiagnosticLeaseExpires(t *testing.T) {
	app := &CoreApp{}
	app.noiseDiagnosticLease = &noiseDiagnosticLease{
		sessionID: "expired",
		expiresAt: time.Now().Add(-time.Second),
		done:      make(chan struct{}),
	}
	if app.noiseDiagnosticLeaseActive() {
		t.Fatal("expired noise diagnostic lease reported active")
	}
	if _, err := app.noiseDiagnosticLeaseFor("expired"); err == nil {
		t.Fatal("expired lease lookup unexpectedly succeeded")
	}
}

func TestNoiseLearningUsesOnlyActiveDeviceResult(t *testing.T) {
	result := types.NoiseDiagnosticResult{
		DeviceKey:  "ble::bs1",
		Unit:       types.FanSpeedUnitRPM,
		RiseDB:     9,
		Confidence: "high",
		Points: []types.NoiseDiagnosticPoint{
			{Requested: 1000, Actual: 1000, LevelDB: -60, SpreadDB: 1, Valid: true},
			{Requested: 2000, Actual: 2000, LevelDB: -59, SpreadDB: 1, Valid: true},
			{Requested: 3000, Actual: 3000, LevelDB: -57, SpreadDB: 1, Valid: true},
			{Requested: 4000, Actual: 4000, LevelDB: -50, SpreadDB: 1, Valid: true},
		},
	}
	cfg := types.AppConfig{
		SmartControl: types.SmartControlConfig{NoiseWeight: 4},
		NoiseDiagnosticsByDevice: map[string]types.NoiseDiagnosticResult{
			result.DeviceKey: result,
		},
	}

	if gain := noiseLearningGainForDevice(cfg, result.DeviceKey, 3500, types.FanSpeedUnitRPM); gain <= 1 {
		t.Fatalf("active-device gain = %.2f, want > 1", gain)
	}
	if gain := noiseLearningGainForDevice(cfg, "hid::other", 3500, types.FanSpeedUnitRPM); gain != 1 {
		t.Fatalf("other-device gain = %.2f, want 1", gain)
	}
	if gain := noiseLearningGainForDevice(cfg, result.DeviceKey, 3500, types.FanSpeedUnitPercent); gain != 1 {
		t.Fatalf("unit-mismatched gain = %.2f, want 1", gain)
	}
}

func TestNoiseDiagnosticActualSpeedKeepsPercentInDisplayUnits(t *testing.T) {
	fanData := &types.FanData{CurrentRPM: 2400, TargetRPM: 79}
	if got := noiseDiagnosticActualSpeed(fanData, types.FanSpeedUnitPercent); got != 79 {
		t.Fatalf("percent target = %d; want 79", got)
	}
	if got := noiseDiagnosticActualSpeed(fanData, types.FanSpeedUnitRPM); got != 2400 {
		t.Fatalf("RPM actual = %d; want 2400", got)
	}
}

func TestNoiseDiagnosticSettleOutcomeKeepsLastValidRPMOnTimeout(t *testing.T) {
	actual, reason, err := noiseDiagnosticSettleOutcome(1000, 1400, 0, 0, types.FanSpeedUnitRPM)
	if err != nil || actual != 1400 || reason != "timeout-fallback" {
		t.Fatalf("outcome = actual %d, reason %q, error %v; want 1400, timeout-fallback, nil", actual, reason, err)
	}
	if _, _, err := noiseDiagnosticSettleOutcome(1000, 0, 0, 0, types.FanSpeedUnitRPM); err == nil {
		t.Fatal("missing RPM telemetry unexpectedly succeeded")
	}
	actual, reason, err = noiseDiagnosticSettleOutcome(5, 0, 0, 0, types.FanSpeedUnitPercent)
	if err != nil || actual != 5 || reason != "command-accepted" {
		t.Fatalf("percent outcome = actual %d, reason %q, error %v; want 5, command-accepted, nil", actual, reason, err)
	}
}

func TestNoiseDiagnosticRPMStabilityToleratesTelemetryJitter(t *testing.T) {
	if !noiseDiagnosticRPMIsSteady(1400, 1350) {
		t.Fatal("normal RPM telemetry jitter was treated as movement")
	}
	if noiseDiagnosticRPMIsSteady(1400, 1300) {
		t.Fatal("a changing RPM was treated as stable")
	}
}

func TestNoiseDiagnosticConnectionChangeStopsWaiting(t *testing.T) {
	if noiseDiagnosticConnectionChanged(true, 7, 7) {
		t.Fatal("unchanged live connection was rejected")
	}
	if !noiseDiagnosticConnectionChanged(false, 7, 7) {
		t.Fatal("disconnect was not detected")
	}
	if !noiseDiagnosticConnectionChanged(true, 8, 7) {
		t.Fatal("connection replacement was not detected")
	}
}

func TestNoiseDiagnosticResultNormalizationBeforePersistence(t *testing.T) {
	result, changed := types.NormalizeNoiseDiagnosticResult(types.NoiseDiagnosticResult{
		Unit: "rpm",
		Points: []types.NoiseDiagnosticPoint{
			{Requested: 2000, Actual: 2000, LevelDB: 3, SpreadDB: 1, Valid: true},
			{Requested: 1000, Actual: 1000, LevelDB: 1, SpreadDB: 1, Valid: true},
			{Requested: 0, Actual: 0, Valid: false},
		},
	})
	if !changed || len(result.Points) != 2 {
		t.Fatalf("normalized result = %#v, changed=%v", result, changed)
	}
	if result.Points[0].Actual != 1000 || result.Points[1].Actual != 2000 {
		t.Fatalf("result points not sorted: %#v", result.Points)
	}
}

func TestAxisNoiseAvoidanceUsesOnlyActiveDeviceProfile(t *testing.T) {
	profile := types.AxisNoiseProfile{
		DeviceKey: "hid::flydigi.bs3",
		Unit:      types.FanSpeedUnitRPM,
		Enabled:   true,
		Range:     types.NoiseDiagnosticRange{Unit: types.FanSpeedUnitRPM, Min: 1000, Max: 3600, Step: 100},
		Zones:     []types.AxisNoiseZone{{Min: 1900, Max: 2200, Severity: types.AxisNoiseSeverityObvious}},
	}
	cfg := types.GetDefaultConfig(false)
	cfg.AxisNoiseProfilesByDevice = map[string]types.AxisNoiseProfile{profile.DeviceKey: profile}

	adjusted, changed := axisNoiseTargetForDevice(cfg, profile.DeviceKey, 2050, -1, types.FanSpeedUnitRPM)
	if !changed || adjusted <= 2050 {
		t.Fatalf("active device target = %d, changed=%v; want upward soft avoidance", adjusted, changed)
	}
	unchanged, changed := axisNoiseTargetForDevice(cfg, "ble::flydigi.bs1", 2050, -1, types.FanSpeedUnitRPM)
	if changed || unchanged != 2050 {
		t.Fatalf("other device target = %d, changed=%v; want unchanged", unchanged, changed)
	}
}
