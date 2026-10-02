package bridge

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/types"
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

func TestTemperatureReadCooldownFailsFast(t *testing.T) {
	m := NewManager(testLogger{})
	m.tripTemperatureReadCooldown()

	started := time.Now()
	data := m.GetTemperature(types.TemperatureSelection{})
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("GetTemperature() blocked during cooldown for %s", elapsed)
	}
	if data.Success || !strings.Contains(data.Error, "冷却") {
		t.Fatalf("GetTemperature() during cooldown = %+v; want a fast cooldown error", data)
	}
}

func TestSuccessfulTemperatureClearsReadCooldown(t *testing.T) {
	m := NewManager(testLogger{})
	m.tripTemperatureReadCooldown()
	m.recordLastTemp(types.BridgeTemperatureData{Success: true})
	if remaining := m.temperatureReadCooldown(); remaining != 0 {
		t.Fatalf("successful temperature left %s of cooldown", remaining)
	}
}
