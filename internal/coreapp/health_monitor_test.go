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

func TestHealthTransportBackoffResetsAfterSuccess(t *testing.T) {
	var app CoreApp
	base := time.Unix(100, 0)

	app.recordHealthTransportFailure("ble", base, "read failed")
	if !app.shouldBackoffHealthTransport("ble", base.Add(time.Second)) {
		t.Fatal("expected BLE health check to enter backoff")
	}
	app.recordHealthTransportSuccess("ble", base.Add(healthTransportBackoffBase))
	if app.shouldBackoffHealthTransport("ble", base.Add(healthTransportBackoffBase).Add(time.Second)) {
		t.Fatal("expected successful BLE refresh to clear backoff")
	}
}

func TestHealthTransportBackoffEscalatesPerTransport(t *testing.T) {
	var app CoreApp
	base := time.Unix(100, 0)

	app.recordHealthTransportFailure("wifi", base, "read failed")
	app.recordHealthTransportFailure("wifi", base.Add(time.Second), "read failed")
	app.recordHealthTransportFailure("ble", base.Add(time.Second), "read failed")

	if !app.shouldBackoffHealthTransport("wifi", base.Add(31*time.Second)) {
		t.Fatal("expected WIFI backoff")
	}
	if app.shouldBackoffHealthTransport("ble", base.Add(31*time.Second)) {
		t.Fatal("BLE backoff should not inherit WIFI failure level")
	}
}

func TestHealthReconnectBackoffEscalatesAndResets(t *testing.T) {
	var app CoreApp
	base := time.Unix(100, 0)
	atomic.StoreInt64(&app.lastHealthReconnectUnix, base.UnixNano())

	app.healthReconnectFailures.Store(1)
	if app.shouldRunHealthReconnect(base.Add(healthReconnectCooldown + time.Second)) {
		t.Fatal("expected first failure backoff (3m) to keep blocking at 91s")
	}
	if !app.shouldRunHealthReconnect(base.Add(3*time.Minute + time.Second)) {
		t.Fatal("expected first failure backoff to allow reconnect after 3m")
	}

	app.healthReconnectFailures.Store(2)
	if app.shouldRunHealthReconnect(base.Add(3*time.Minute + time.Second)) {
		t.Fatal("expected repeated failure backoff (5m) to block at 3m")
	}
	if !app.shouldRunHealthReconnect(base.Add(healthReconnectCooldownMax + time.Second)) {
		t.Fatal("expected repeated failure backoff to allow reconnect after 5m")
	}

	app.healthReconnectFailures.Store(0)
	if !app.shouldRunHealthReconnect(base.Add(healthReconnectCooldown + time.Second)) {
		t.Fatal("expected reset failure backoff to return to 90s cooldown")
	}
}
