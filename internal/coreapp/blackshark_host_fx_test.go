package coreapp

import (
	"testing"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func TestBlackSharkHostEffectsBroadcastOnlyOnChangeAndAtMostOncePerSecond(t *testing.T) {
	now := time.Unix(100, 0)
	previous := types.BlackSharkHostEffects{CurrentSlot: 7, AudioSyncRunning: true, AudioLevel: 2, AudioPushCount: 10}

	if !shouldBroadcastBlackSharkHostEffects(false, types.BlackSharkHostEffects{}, previous, now, time.Time{}) {
		t.Fatal("first client snapshot should be broadcast immediately")
	}
	unchanged := previous
	if shouldBroadcastBlackSharkHostEffects(true, previous, unchanged, now.Add(200*time.Millisecond), now) {
		t.Fatal("unchanged state should not be broadcast")
	}
	progress := previous
	progress.AudioPushCount++
	if shouldBroadcastBlackSharkHostEffects(true, previous, progress, now.Add(200*time.Millisecond), now) {
		t.Fatal("high-rate counters should be coalesced inside one second")
	}
	if !shouldBroadcastBlackSharkHostEffects(true, previous, progress, now.Add(time.Second), now) {
		t.Fatal("latest counters should be broadcast after one second")
	}
	transition := previous
	transition.CurrentSlot = 6
	if !shouldBroadcastBlackSharkHostEffects(true, previous, transition, now.Add(200*time.Millisecond), now) {
		t.Fatal("slot transitions should be broadcast immediately")
	}
}

func TestRemainingHostFxTickDelayPreservesCadenceWithoutNegativeTimers(t *testing.T) {
	interval := 204 * time.Millisecond
	if got := remainingHostFxTickDelay(interval, 80*time.Millisecond); got != 124*time.Millisecond {
		t.Fatalf("remaining delay = %s, want 124ms", got)
	}
	for _, elapsed := range []time.Duration{interval, interval + time.Millisecond} {
		if got := remainingHostFxTickDelay(interval, elapsed); got != 0 {
			t.Fatalf("remaining delay at elapsed %s = %s, want 0", elapsed, got)
		}
	}
}
