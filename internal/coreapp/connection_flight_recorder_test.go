package coreapp

import (
	"testing"
	"time"
)

func TestConnectionFlightRecorderKeepsBoundedChronologicalEvents(t *testing.T) {
	now := time.Date(2026, time.July, 31, 10, 0, 0, 0, time.UTC)
	recorder := newConnectionFlightRecorder(2, func() time.Time { return now })

	recorder.record(connectionFlightEvent{Stage: connectionFlightStageConnecting, Reason: "manual"})
	now = now.Add(time.Second)
	recorder.record(connectionFlightEvent{Stage: connectionFlightStageReconnecting, Reason: "device-disconnect", Attempt: 1})
	now = now.Add(time.Second)
	recorder.record(connectionFlightEvent{Stage: connectionFlightStageReady, Transport: "ble"})

	snapshot := recorder.snapshot(connectionFlightSnapshotInput{
		State:               deviceRuntimeStateReady,
		CanControl:          true,
		ReconnectInProgress: false,
	})
	if len(snapshot.Events) != 2 {
		t.Fatalf("expected 2 retained events, got %d", len(snapshot.Events))
	}
	if snapshot.Events[0].Sequence != 2 || snapshot.Events[1].Sequence != 3 {
		t.Fatalf("unexpected retained sequence: %#v", snapshot.Events)
	}
	if snapshot.Events[0].Stage != connectionFlightStageReconnecting || snapshot.Events[1].Stage != connectionFlightStageReady {
		t.Fatalf("events are not chronological: %#v", snapshot.Events)
	}
	if snapshot.State != deviceRuntimeStateReady || !snapshot.CanControl {
		t.Fatalf("unexpected runtime summary: %#v", snapshot)
	}

	snapshot.Events[0].Reason = "mutated"
	second := recorder.snapshot(connectionFlightSnapshotInput{})
	if second.Events[0].Reason == "mutated" {
		t.Fatal("snapshot exposed recorder-owned event storage")
	}
}

func TestConnectionFlightRecorderTracksSuccessfulIOWithoutAddingEvents(t *testing.T) {
	now := time.Date(2026, time.July, 31, 11, 0, 0, 0, time.UTC)
	recorder := newConnectionFlightRecorder(4, func() time.Time { return now })
	recorder.markSuccessfulRead()
	now = now.Add(2 * time.Second)
	recorder.markSuccessfulWrite()

	snapshot := recorder.snapshot(connectionFlightSnapshotInput{Suspended: true})
	if snapshot.LastSuccessfulReadAt != "2026-07-31T11:00:00Z" {
		t.Fatalf("unexpected read timestamp: %q", snapshot.LastSuccessfulReadAt)
	}
	if snapshot.LastSuccessfulWriteAt != "2026-07-31T11:00:02Z" {
		t.Fatalf("unexpected write timestamp: %q", snapshot.LastSuccessfulWriteAt)
	}
	if !snapshot.Suspended {
		t.Fatal("expected suspended runtime summary")
	}
	if len(snapshot.Events) != 0 {
		t.Fatalf("successful IO should not append flight events: %#v", snapshot.Events)
	}
}
