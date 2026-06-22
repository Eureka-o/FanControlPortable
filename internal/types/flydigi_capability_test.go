package types

import "testing"

func TestDecodeFlyDigiRuntimeCapabilityFromGearSettings(t *testing.T) {
	tests := []struct {
		name    string
		gear    uint8
		wantIdx int
		wantRPM int
	}{
		{name: "standard", gear: 0x2A, wantIdx: FlyDigiGearCodeStandard, wantRPM: 2700},
		{name: "performance", gear: 0x4C, wantIdx: FlyDigiGearCodePerformance, wantRPM: 3300},
		{name: "extreme", gear: 0x6E, wantIdx: FlyDigiGearCodeExtreme, wantRPM: 4000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecodeFlyDigiRuntimeCapabilityFromGearSettings(tt.gear, nil)
			if !got.Available {
				t.Fatalf("capability should be available: %#v", got)
			}
			if got.MaxGearIndex != tt.wantIdx || got.MaxRPM != tt.wantRPM {
				t.Fatalf("capability = index %d rpm %d, want index %d rpm %d", got.MaxGearIndex, got.MaxRPM, tt.wantIdx, tt.wantRPM)
			}
			if got.Source != "default" {
				t.Fatalf("source = %q, want default", got.Source)
			}
		})
	}
}

func TestDecodeFlyDigiRuntimeCapabilityPrefersGearRPMTable(t *testing.T) {
	table := []DeviceGearRPM{
		{Gear: FlyDigiGearCodeStandard, RPM: 2760},
		{Gear: FlyDigiGearCodePerformance, RPM: 3500},
		{Gear: FlyDigiGearCodeExtreme, RPM: 3700},
	}

	got := DecodeFlyDigiRuntimeCapabilityFromGearSettings(0x4A, table)
	if got.MaxRPM != 3500 {
		t.Fatalf("max rpm = %d, want table value 3500", got.MaxRPM)
	}
	if got.Source != "gearRpmTable" {
		t.Fatalf("source = %q, want gearRpmTable", got.Source)
	}
}

func TestFlyDigiClampRPMForCapability(t *testing.T) {
	capability := DecodeFlyDigiRuntimeCapabilityFromGearSettings(0x4A, nil)

	got, limited := FlyDigiClampRPMForCapability(4000, capability)
	if got != 3300 || !limited {
		t.Fatalf("clamp = (%d, %v), want (3300, true)", got, limited)
	}

	got, limited = FlyDigiClampRPMForCapability(3000, capability)
	if got != 3000 || limited {
		t.Fatalf("clamp = (%d, %v), want (3000, false)", got, limited)
	}
}

func TestFlyDigiIsGearAllowed(t *testing.T) {
	capability := DecodeFlyDigiRuntimeCapabilityFromGearSettings(0x4A, nil)

	if !FlyDigiIsGearAllowed("强劲", capability) {
		t.Fatal("performance gear should be allowed when max gear is performance")
	}
	if FlyDigiIsGearAllowed("超频", capability) {
		t.Fatal("extreme gear should be rejected when max gear is performance")
	}
}

func TestDecodeFlyDigiRuntimeCapabilityUnknownCodeIsNonBlocking(t *testing.T) {
	got := DecodeFlyDigiRuntimeCapabilityFromGearSettings(0xFA, nil)
	if got.Available {
		t.Fatalf("unknown max gear code should not be available: %#v", got)
	}
	rpm, limited := FlyDigiClampRPMForCapability(4200, got)
	if rpm != 4200 || limited {
		t.Fatalf("unknown capability should not clamp: (%d, %v)", rpm, limited)
	}
}
