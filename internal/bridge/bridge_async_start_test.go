package bridge

import (
	"errors"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

type testLogger struct{}

func (testLogger) Info(string, ...any)  {}
func (testLogger) Error(string, ...any) {}
func (testLogger) Warn(string, ...any)  {}
func (testLogger) Debug(string, ...any) {}
func (testLogger) Close()               {}
func (testLogger) CleanOldLogs()        {}
func (testLogger) SetDebugMode(bool)    {}
func (testLogger) GetLogDir() string    { return "" }

func TestEnsureRunningStartsInBackground(t *testing.T) {
	m := NewManager(testLogger{})
	started := time.Now()
	err := m.EnsureRunning()
	if !errors.Is(err, ErrStarting) {
		t.Fatalf("EnsureRunning() error = %v; want ErrStarting", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("EnsureRunning() blocked for %s", elapsed)
	}
}

func TestGetStatusDoesNotStartBridge(t *testing.T) {
	m := NewManager(testLogger{})
	_ = m.GetStatus()
	if m.IsStarting() {
		t.Fatal("GetStatus() started the bridge")
	}
}

func TestLastSuccessfulTemperatureIsCached(t *testing.T) {
	m := NewManager(testLogger{})
	m.recordLastTemp(types.BridgeTemperatureData{Success: true})
	m.recordLastTemp(types.BridgeTemperatureData{Success: false})
	if !m.lastTemp.Success || m.lastTempAt == 0 {
		t.Fatalf("last successful temperature was not preserved: %+v", m.lastTemp)
	}
}
