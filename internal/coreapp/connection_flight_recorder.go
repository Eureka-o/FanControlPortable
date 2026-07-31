package coreapp

import (
	"sync"
	"time"
)

const defaultConnectionFlightCapacity = 64

const (
	connectionFlightStageDiscovering  = "discovering"
	connectionFlightStageConnecting   = "connecting"
	connectionFlightStageConnected    = "connected"
	connectionFlightStageReady        = "ready"
	connectionFlightStageDisconnected = "disconnected"
	connectionFlightStageReconnecting = "reconnecting"
	connectionFlightStageSuspended    = "suspended"
	connectionFlightStageError        = "error"
)

type connectionFlightEvent struct {
	Sequence   uint64 `json:"sequence"`
	Timestamp  string `json:"timestamp"`
	Stage      string `json:"stage"`
	Reason     string `json:"reason,omitempty"`
	Transport  string `json:"transport,omitempty"`
	ProfileID  string `json:"profileId,omitempty"`
	Attempt    int    `json:"attempt,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

type connectionFlightSnapshotInput struct {
	State               string
	CanControl          bool
	ReconnectInProgress bool
	Suspended           bool
}

type connectionFlightSnapshot struct {
	State                 string                  `json:"state"`
	CanControl            bool                    `json:"canControl"`
	ReconnectInProgress   bool                    `json:"reconnectInProgress"`
	Suspended             bool                    `json:"suspended"`
	LastSuccessfulReadAt  string                  `json:"lastSuccessfulReadAt,omitempty"`
	LastSuccessfulWriteAt string                  `json:"lastSuccessfulWriteAt,omitempty"`
	Events                []connectionFlightEvent `json:"events"`
}

type connectionFlightRecorder struct {
	mu                    sync.RWMutex
	capacity              int
	now                   func() time.Time
	nextSequence          uint64
	events                []connectionFlightEvent
	lastSuccessfulReadAt  time.Time
	lastSuccessfulWriteAt time.Time
}

func newConnectionFlightRecorder(capacity int, now func() time.Time) *connectionFlightRecorder {
	if capacity <= 0 {
		capacity = defaultConnectionFlightCapacity
	}
	if now == nil {
		now = time.Now
	}
	return &connectionFlightRecorder{
		capacity: capacity,
		now:      now,
		events:   make([]connectionFlightEvent, 0, capacity),
	}
}

func (r *connectionFlightRecorder) record(event connectionFlightEvent) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextSequence++
	event.Sequence = r.nextSequence
	event.Timestamp = r.now().UTC().Format(time.RFC3339Nano)
	if event.DurationMs < 0 {
		event.DurationMs = 0
	}
	if len(r.events) == r.capacity {
		copy(r.events, r.events[1:])
		r.events[len(r.events)-1] = event
		return
	}
	r.events = append(r.events, event)
}

func (r *connectionFlightRecorder) markSuccessfulRead() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.lastSuccessfulReadAt = r.now().UTC()
	r.mu.Unlock()
}

func (r *connectionFlightRecorder) markSuccessfulWrite() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.lastSuccessfulWriteAt = r.now().UTC()
	r.mu.Unlock()
}

func (a *CoreApp) setTargetSpeed(value int, unit string) bool {
	ok := a.deviceManager.SetTargetSpeed(value, unit)
	if ok {
		a.connectionFlights.markSuccessfulWrite()
	}
	return ok
}

func (r *connectionFlightRecorder) snapshot(input connectionFlightSnapshotInput) connectionFlightSnapshot {
	if r == nil {
		return connectionFlightSnapshot{
			State:               input.State,
			CanControl:          input.CanControl,
			ReconnectInProgress: input.ReconnectInProgress,
			Suspended:           input.Suspended,
			Events:              []connectionFlightEvent{},
		}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	events := append([]connectionFlightEvent(nil), r.events...)
	return connectionFlightSnapshot{
		State:                 input.State,
		CanControl:            input.CanControl,
		ReconnectInProgress:   input.ReconnectInProgress,
		Suspended:             input.Suspended,
		LastSuccessfulReadAt:  formatConnectionFlightTime(r.lastSuccessfulReadAt),
		LastSuccessfulWriteAt: formatConnectionFlightTime(r.lastSuccessfulWriteAt),
		Events:                events,
	}
}

func formatConnectionFlightTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
