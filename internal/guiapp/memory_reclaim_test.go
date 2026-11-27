package guiapp

import (
	"testing"
	"time"
)

func TestEvaluateGuiMemoryReclaimScheduleAndBusyDeferral(t *testing.T) {
	startedAt := time.Unix(1_717_000_000, 0)

	// Before 170s periodic interval: skip
	if run, _, nextDefer := evaluateGuiMemoryReclaim(startedAt.Add(169*time.Second), startedAt, time.Time{}, time.Time{}, true, false); run || !nextDefer.IsZero() {
		t.Fatalf("before periodic interval = (%v, %v), want (false, zero)", run, nextDefer)
	}

	// Visible & quiet at 170s: heap-only reclaim (trimWorkingSet = false)
	dueAt := startedAt.Add(guiPeriodicMemReleaseInterval)
	run, trimWS, nextDefer := evaluateGuiMemoryReclaim(dueAt, startedAt, time.Time{}, time.Time{}, false, false)
	if !run || trimWS || !nextDefer.IsZero() {
		t.Fatalf("visible periodic reclaim = (%v, %v, %v), want (true, false, zero)", run, trimWS, nextDefer)
	}

	// Hidden & quiet at 170s: full reclaim (trimWorkingSet = true)
	run, trimWS, _ = evaluateGuiMemoryReclaim(dueAt, startedAt, time.Time{}, time.Time{}, true, false)
	if !run || !trimWS {
		t.Fatalf("hidden periodic reclaim = (%v, %v), want (true, true)", run, trimWS)
	}

	// Busy defers by 20s and respects deferral window before deadline
	run, _, deferUntil := evaluateGuiMemoryReclaim(dueAt, startedAt, time.Time{}, time.Time{}, true, true)
	if run || !deferUntil.Equal(dueAt.Add(guiMemReleaseBusyDefer)) {
		t.Fatalf("busy deferral = (%v, %v), want (false, %v)", run, deferUntil, dueAt.Add(guiMemReleaseBusyDefer))
	}
	run, _, nextDefer = evaluateGuiMemoryReclaim(dueAt.Add(5*time.Second), startedAt, time.Time{}, deferUntil, true, false)
	if run || !nextDefer.Equal(deferUntil) {
		t.Fatalf("inside defer window = (%v, %v), want (false, %v)", run, nextDefer, deferUntil)
	}
	run, trimWS, nextDefer = evaluateGuiMemoryReclaim(deferUntil, startedAt, time.Time{}, deferUntil, true, false)
	if !run || !trimWS || !nextDefer.IsZero() {
		t.Fatalf("after defer window = (%v, %v, %v), want (true, true, zero)", run, trimWS, nextDefer)
	}
}

func TestGuiHideWindowTriggersMemoryReclaimWithCooldown(t *testing.T) {
	called := make(chan bool, 4)
	app := &App{
		startedAt:    time.Now(),
		memReclaimFn: func(trimWS bool) { called <- trimWS },
	}

	app.HideWindow()
	select {
	case trimWS := <-called:
		if !trimWS {
			t.Fatal("HideWindow should reclaim with working-set trim enabled")
		}
	case <-time.After(time.Second):
		t.Fatal("HideWindow did not trigger memory reclaim")
	}

	// Subsequent hide within 30s cooldown is suppressed
	app.HideWindow()
	select {
	case unexpected := <-called:
		t.Fatalf("expected cooldown to suppress second reclaim, got trimWS=%v", unexpected)
	case <-time.After(50 * time.Millisecond):
	}

	// Active update download blocks hidden reclaim
	app2 := &App{
		startedAt:     time.Now(),
		updateControl: &updateDownloadControl{},
		memReclaimFn:  func(trimWS bool) { called <- trimWS },
	}
	app2.HideWindow()
	select {
	case unexpected := <-called:
		t.Fatalf("expected active update download to defer reclaim, got trimWS=%v", unexpected)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestGuiPeriodicReclaimDefersDuringRecentUserActivity(t *testing.T) {
	called := make(chan bool, 2)
	startedAt := time.Unix(1_717_000_000, 0)
	dueAt := startedAt.Add(guiPeriodicMemReleaseInterval)
	app := &App{
		startedAt:    startedAt,
		memReclaimFn: func(trimWS bool) { called <- trimWS },
	}
	app.lastUserActivityNs.Store(dueAt.Add(-5 * time.Second).UnixNano())

	app.maybeRunPeriodicMemoryReclaim(dueAt)
	select {
	case unexpected := <-called:
		t.Fatalf("expected recent user activity to defer reclaim, got trimWS=%v", unexpected)
	case <-time.After(50 * time.Millisecond):
	}
	if deferNs := app.memReclaimDeferUntilNs.Load(); deferNs != dueAt.Add(guiMemReleaseBusyDefer).UnixNano() {
		t.Fatalf("deferUntilNs = %d, want %d", deferNs, dueAt.Add(guiMemReleaseBusyDefer).UnixNano())
	}

	// After defer deadline and quiet window, reclaim runs
	afterQuiet := dueAt.Add(guiMemReleaseBusyDefer)
	app.maybeRunPeriodicMemoryReclaim(afterQuiet)
	select {
	case trimWS := <-called:
		if trimWS {
			t.Fatal("visible periodic reclaim should trim heap only (trimWS=false)")
		}
	case <-time.After(time.Second):
		t.Fatal("expected periodic reclaim to run after defer window and quiet interval")
	}
}
