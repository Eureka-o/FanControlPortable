package guiapp

import "testing"

func TestReconnectCoreStandsDownDuringShutdown(t *testing.T) {
	app := &App{}
	app.shuttingDown.Store(true)
	app.reconnectCore()
}
