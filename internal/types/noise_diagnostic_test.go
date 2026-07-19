package types

import "testing"

func TestNoiseDiagnosticRangeForFlyDigiUsesRuntimeMaximum(t *testing.T) {
	profile := FlyDigiBS1Profile()
	fanData := &FanData{FlyDigiCapability: &FlyDigiRuntimeCapability{Available: true, MaxRPM: 3300}}

	got, err := NoiseDiagnosticRangeForProfile(profile, profile.Capabilities, fanData)
	if err != nil {
		t.Fatalf("NoiseDiagnosticRangeForProfile() error = %v", err)
	}
	if got.Unit != FanSpeedUnitRPM || got.Min != NoiseDiagnosticFlyDigiMinRPM || got.Max != 3300 {
		t.Fatalf("range = %#v, want rpm %d..3300", got, NoiseDiagnosticFlyDigiMinRPM)
	}
	if got.MinSource != "flydigi-diagnostic-floor" || got.MaxSource != "runtime-capability" {
		t.Fatalf("range sources = %q/%q", got.MinSource, got.MaxSource)
	}
}

func TestNoiseDiagnosticRangeForPercentUsesFivePercentFloor(t *testing.T) {
	profile := DefaultWiFiPercentProfile("http://127.0.0.1")
	got, err := NoiseDiagnosticRangeForProfile(profile, profile.Capabilities, nil)
	if err != nil {
		t.Fatalf("NoiseDiagnosticRangeForProfile() error = %v", err)
	}
	if got.Unit != FanSpeedUnitPercent || got.Min != NoiseDiagnosticPercentMin || got.Max != FanSpeedMaxPercent {
		t.Fatalf("range = %#v, want percent %d..%d", got, NoiseDiagnosticPercentMin, FanSpeedMaxPercent)
	}
}

func TestNoiseDiagnosticRangeRejectsUnknownRPMMinimum(t *testing.T) {
	profile := LegacyRPMProfileForTransport(DeviceTransportHID)
	profile.ID = "custom.rpm"
	profile.SpeedRange.Min = 0
	profile.Capabilities.SpeedRange.Min = 0
	if _, err := NoiseDiagnosticRangeForProfile(profile, profile.Capabilities, nil); err == nil {
		t.Fatal("expected unknown RPM minimum to be rejected")
	}
}

func TestNormalizeNoiseDiagnosticRangeClampsEditableBounds(t *testing.T) {
	allowed := NoiseDiagnosticRange{Unit: FanSpeedUnitRPM, Min: 1000, Max: 3600, Step: 1, MinSource: "floor", MaxSource: "cap"}
	got, err := NormalizeNoiseDiagnosticRange(NoiseDiagnosticRange{Unit: FanSpeedUnitRPM, Min: 500, Max: 5000}, allowed)
	if err != nil {
		t.Fatalf("NormalizeNoiseDiagnosticRange() error = %v", err)
	}
	if got.Min != 1000 || got.Max != 3600 {
		t.Fatalf("range = %#v, want 1000..3600", got)
	}
}

func TestNormalizeNoiseDiagnosticResultDropsInvalidPoints(t *testing.T) {
	result := NoiseDiagnosticResult{
		Unit: FanSpeedUnitRPM,
		Points: []NoiseDiagnosticPoint{
			{Requested: 2000, Actual: 2000, LevelDB: 4, SpreadDB: 1, Valid: true},
			{Requested: 1000, Actual: 1000, LevelDB: 0, SpreadDB: 1, Valid: true},
			{Requested: 1500, Actual: 1500, LevelDB: 1, SpreadDB: 0.5, Valid: false},
		},
		RiseDB: -2,
	}
	got, changed := NormalizeNoiseDiagnosticResult(result)
	if !changed || len(got.Points) != 2 || got.Points[0].Actual != 1000 || got.RiseDB != 0 {
		t.Fatalf("normalized result = %#v, changed=%v", got, changed)
	}
}
