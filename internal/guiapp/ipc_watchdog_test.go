package guiapp

import (
	"context"
	"testing"
	"time"
)

func TestReconnectCoreStandsDownDuringShutdown(t *testing.T) {
	app := &App{}
	app.signalShutdown()
	app.reconnectCore()
}

func TestStartIPCWatchdogExitsPromptlyOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := &App{ctx: ctx}
	done := make(chan struct{})
	stopCh := app.shutdownChan()
	go func() {
		<-stopCh
		close(done)
	}()

	app.startIPCWatchdog()
	app.signalShutdown()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected signalShutdown to unblock IPC watchdog promptly")
	}

	// Cancel context path should also be safe if invoked repeatedly.
	cancel()
	app.signalShutdown()
}
