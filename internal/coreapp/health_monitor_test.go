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
