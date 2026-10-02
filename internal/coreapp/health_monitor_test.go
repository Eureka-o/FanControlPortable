package coreapp

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestShouldRunHealthReconnect(t *testing.T) {
	var app CoreApp

	if !app.shouldRunHealthReconnect(time.Unix(100, 0)) {
		t.Fatal("expected first health reconnect to run")
	}

	atomic.StoreInt64(&app.lastHealthReconnectUnix, time.Unix(100, 0).Add(-healthReconnectCooldown+time.Second).UnixNano())
	if app.shouldRunHealthReconnect(time.Unix(100, 0)) {
		t.Fatal("expected reconnect cooldown to block health reconnect")
	}

	atomic.StoreInt64(&app.lastHealthReconnectUnix, time.Unix(100, 0).Add(-healthReconnectCooldown-time.Second).UnixNano())
	if !app.shouldRunHealthReconnect(time.Unix(100, 0)) {
		t.Fatal("expected health reconnect after cooldown")
	}
}

func TestShouldSkipDeviceHealthCheckDuringRecovery(t *testing.T) {
	tests := []struct {
		name                  string
		systemSuspended       bool
		resumeRecoveryRunning bool
		reconnectInProgress   bool
		wantSkip              bool
	}{
		{name: "normal", wantSkip: false},
		{name: "system suspended", systemSuspended: true, wantSkip: true},
		{name: "resume recovery", resumeRecoveryRunning: true, wantSkip: true},
		{name: "reconnect active", reconnectInProgress: true, wantSkip: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSkipDeviceHealthCheck(tt.systemSuspended, tt.resumeRecoveryRunning, tt.reconnectInProgress); got != tt.wantSkip {
				t.Fatalf("shouldSkipDeviceHealthCheck() = %v, want %v", got, tt.wantSkip)
			}
		})
	}
}
