package types

import "testing"

func TestPercentSpeedTicks(t *testing.T) {
	if got := NewPercentSpeed(50).Value; got != 500 {
		t.Fatalf("NewPercentSpeed(50).Value = %d, want 500", got)
	}
	if got := PercentFloatToTicks(50.4); got != 504 {
		t.Fatalf("PercentFloatToTicks(50.4) = %d, want 504", got)
	}
	if got := PercentTicksToIntegerPercent(504); got != 50 {
		t.Fatalf("PercentTicksToIntegerPercent(504) = %d, want 50", got)
	}
	if got := PercentTicksToIntegerPercent(505); got != 51 {
		t.Fatalf("PercentTicksToIntegerPercent(505) = %d, want 51", got)
	}
	if got := NewPercentTickSpeed(1200).Value; got != FanSpeedMaxPercentTicks {
		t.Fatalf("NewPercentTickSpeed(1200).Value = %d, want %d", got, FanSpeedMaxPercentTicks)
	}
}

func TestPercentTickSpeedSendRounding(t *testing.T) {
	speed := NewPercentTickSpeed(377)
	if got := PercentTicksToDecimalPercent(speed.Value); got != 37.7 {
		t.Fatalf("PercentTicksToDecimalPercent(377) = %v, want 37.7", got)
	}
	percent, ok := speed.IntegerPercentForSend()
	if !ok {
		t.Fatal("expected percent tick speed to support integer-percent send conversion")
	}
	if percent != 38 {
		t.Fatalf("IntegerPercentForSend() = %d, want rounded 38", percent)
	}
}

func TestRPMSpeedPassThrough(t *testing.T) {
	if got := NewRPMSpeed(1800).Value; got != 1800 {
		t.Fatalf("NewRPMSpeed(1800).Value = %d, want 1800", got)
	}
	if got := NewRPMSpeed(-1).Value; got != 0 {
		t.Fatalf("NewRPMSpeed(-1).Value = %d, want 0", got)
	}
}
