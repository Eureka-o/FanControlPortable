package bridge

import (
	"errors"
	"io"
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

type delayedWriteCloser struct {
	delay time.Duration
}

func (d *delayedWriteCloser) Write(p []byte) (int, error) {
	time.Sleep(d.delay)
	return len(p), nil
}

func (d *delayedWriteCloser) Close() error { return nil }

func TestSendCommandUnsafeSharesSingleDeadline(t *testing.T) {
	// Exit 命令超时为 2s；写入消耗 1.5s 后，读取只应再等剩余 ~0.5s，而不应重新计时 2s（总计 ~3.5s）。
	m := NewManager(testLogger{})
	m.stdin = &delayedWriteCloser{delay: 1500 * time.Millisecond}
	m.lineCh = make(chan string)

	started := time.Now()
	_, err := m.sendCommandUnsafe("Exit", "")
	elapsed := time.Since(started)

	if err == nil || !strings.Contains(err.Error(), "超时") {
		t.Fatalf("sendCommandUnsafe() err = %v, want timeout error", err)
	}
	if elapsed > 2700*time.Millisecond {
		t.Fatalf("sendCommandUnsafe() elapsed = %s, want shared <=2s deadline (got write+read double timeout)", elapsed)
	}
}

var _ io.WriteCloser = (*delayedWriteCloser)(nil)
