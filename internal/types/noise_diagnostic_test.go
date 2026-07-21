package types

import "testing"

func TestNoiseDiagnosticRangeForFlyDigiUsesRuntimeMaximum(t *testing.T) {
	profile := FlyDigiBS1Profile()
	fanData := &FanData{FlyDigiCapability: &FlyDigiRuntimeCapability{Available: true, MaxRPM: 3300}}

	got, err := NoiseDiagnosticRangeForProfile(profile, profile.Capabilities, fanData)
	if err != nil {
		t.Fatalf("NoiseDiagnosticRangeForProfile() error = %v", err)
	}
	if got.Unit != FanSpeedUnitRPM || got.Min != NoiseDiagnosticFlyDigiMinRPM || got.Max != 3300 || got.Step != AxisNoiseRPMFineStep {
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
	allowed := NoiseDiagnosticRange{Unit: FanSpeedUnitRPM, Min: 1000, Max: 3600, Step: 100, MinSource: "floor", MaxSource: "cap"}
	got, err := NormalizeNoiseDiagnosticRange(NoiseDiagnosticRange{Unit: FanSpeedUnitRPM, Min: 500, Max: 5000, Step: 1}, allowed)
	if err != nil {
		t.Fatalf("NormalizeNoiseDiagnosticRange() error = %v", err)
	}
	if got.Min != 1000 || got.Max != 3600 {
		t.Fatalf("range = %#v, want 1000..3600", got)
	}
	if got.Step != 100 {
		t.Fatalf("range step = %d, want device minimum 100", got.Step)
	}
}

func TestNoiseDiagnosticRangeAlignsConfiguredBoundsAndKeepsRuntimeMaximum(t *testing.T) {
	profile := FlyDigiBS1Profile()
	profile.SpeedRange.Max = 3350
	profile.Capabilities.SpeedRange.Max = 3350

	configured, err := NoiseDiagnosticRangeForProfile(profile, profile.Capabilities, nil)
	if err != nil {
		t.Fatalf("configured range error = %v", err)
	}
	if configured.Max != 3300 {
		t.Fatalf("configured max = %d, want 3300", configured.Max)
	}
	higherRuntime, err := NoiseDiagnosticRangeForProfile(profile, profile.Capabilities, &FanData{
		FlyDigiCapability: &FlyDigiRuntimeCapability{Available: true, MaxRPM: 4000},
	})
	if err != nil {
		t.Fatalf("higher runtime range error = %v", err)
	}
	if higherRuntime.Max != 3300 || higherRuntime.MaxSource != "profile" {
		t.Fatalf("higher runtime range = %#v, want aligned profile max 3300", higherRuntime)
	}

	profile.SpeedRange.Max = 4000
	profile.Capabilities.SpeedRange.Max = 4000
	runtime, err := NoiseDiagnosticRangeForProfile(profile, profile.Capabilities, &FanData{
		FlyDigiCapability: &FlyDigiRuntimeCapability{Available: true, MaxRPM: 3350},
	})
	if err != nil {
		t.Fatalf("runtime range error = %v", err)
	}
	if runtime.Max != 3350 || runtime.MaxSource != "runtime-capability" {
		t.Fatalf("runtime range = %#v, want exact runtime max 3350", runtime)
	}

	aligned, err := NormalizeNoiseDiagnosticRange(
		NoiseDiagnosticRange{Unit: FanSpeedUnitRPM, Min: 1050, Max: 3350, Step: 1},
		NoiseDiagnosticRange{Unit: FanSpeedUnitRPM, Min: 1000, Max: 4000, Step: 100, MaxSource: "profile"},
	)
	if err != nil {
		t.Fatalf("aligned range error = %v", err)
	}
	if aligned.Min != 1100 || aligned.Max != 3300 {
		t.Fatalf("aligned range = %#v, want 1100..3300", aligned)
	}

	preserved, err := NormalizeNoiseDiagnosticRange(runtime, runtime)
	if err != nil {
		t.Fatalf("preserved runtime range error = %v", err)
	}
	if preserved.Max != 3350 {
		t.Fatalf("preserved runtime max = %d, want 3350", preserved.Max)
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

func TestNormalizeAxisNoiseProfileBuildsSoftAvoidanceZone(t *testing.T) {
	allowed := NoiseDiagnosticRange{Unit: FanSpeedUnitRPM, Min: 1000, Max: 3600, Step: 100}
	profile := AxisNoiseProfile{
		DeviceKey: "hid::flydigi.bs3",
		Unit:      FanSpeedUnitRPM,
		Enabled:   true,
		Range:     allowed,
		Points: []AxisNoisePoint{
			{Requested: 1500, Actual: 1500, Severity: AxisNoiseSeverityNone},
			{Requested: 2000, Actual: 2000, Severity: AxisNoiseSeverityMild},
			{Requested: 2100, Actual: 2100, Severity: AxisNoiseSeverityObvious},
			{Requested: 2600, Actual: 2600, Severity: AxisNoiseSeverityNone},
		},
	}

	got, err := NormalizeAxisNoiseProfile(profile, allowed)
	if err != nil {
		t.Fatalf("NormalizeAxisNoiseProfile() error = %v", err)
	}
	if len(got.Zones) != 1 {
		t.Fatalf("zones = %#v, want one merged zone", got.Zones)
	}
	zone := got.Zones[0]
	if zone.Min != 1900 || zone.Max != 2200 || zone.Severity != AxisNoiseSeverityObvious {
		t.Fatalf("zone = %#v, want 1900..2200 obvious", zone)
	}

	adjusted, changed := ApplyAxisNoiseAvoidance(2050, -1, FanSpeedUnitRPM, got)
	if !changed || adjusted <= 2050 || adjusted >= zone.Max {
		t.Fatalf("soft avoidance = %d, changed=%v; want a partial upward shift below %d", adjusted, changed, zone.Max)
	}
}

func TestApplyAxisNoiseAvoidancePreservesPercentTargetUnits(t *testing.T) {
	profile := AxisNoiseProfile{
		Unit:    FanSpeedUnitPercent,
		Enabled: true,
		Zones:   []AxisNoiseZone{{Min: 40, Max: 50, Severity: AxisNoiseSeverityMild}},
	}
	target := PercentToTicks(45)
	adjusted, changed := ApplyAxisNoiseAvoidance(target, -1, FanSpeedUnitPercent, profile)
	if !changed || adjusted <= target || adjusted >= PercentToTicks(50) {
		t.Fatalf("percent target = %d, changed=%v; want a partial upward shift in ticks", adjusted, changed)
	}
}
