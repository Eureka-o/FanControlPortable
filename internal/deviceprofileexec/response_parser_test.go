package deviceprofileexec

import "testing"

func TestCompleteStateTargetDoesNotFakeCurrentFromFallback(t *testing.T) {
	state := ParsedState{}
	if !completeStateTarget(&state, 56) {
		t.Fatal("completeStateTarget() should keep a target-only state usable")
	}
	if state.HasCurrent || state.CurrentSpeed != 0 {
		t.Fatalf("current speed was faked: has=%v value=%d", state.HasCurrent, state.CurrentSpeed)
	}
	if !state.HasTarget || state.TargetSpeed != 56 {
		t.Fatalf("target speed = (%v, %d), want (true, 56)", state.HasTarget, state.TargetSpeed)
	}
}

func TestCompleteStateTargetUsesCurrentWhenTargetMissing(t *testing.T) {
	state := ParsedState{CurrentSpeed: 42, HasCurrent: true}
	if !completeStateTarget(&state, 0) {
		t.Fatal("completeStateTarget() should accept current-only state")
	}
	if !state.HasTarget || state.TargetSpeed != 42 {
		t.Fatalf("target speed = (%v, %d), want (true, 42)", state.HasTarget, state.TargetSpeed)
	}
}
