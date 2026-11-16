//go:build !legacydevice && windows

package device

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func TestApplyFlyDigiRealtimeRPMCommand(t *testing.T) {
	var active atomic.Bool
	applyFlyDigiRealtimeRPMCommand(&active, deviceproto.CmdEnterRealtimeRPM)
	if !active.Load() {
		t.Fatal("enter realtime command did not activate realtime state")
	}
	applyFlyDigiRealtimeRPMCommand(&active, deviceproto.CmdSetRealtimeRPM)
	if !active.Load() {
		t.Fatal("setting target RPM unexpectedly cleared realtime state")
	}
	applyFlyDigiRealtimeRPMCommand(&active, deviceproto.CmdExitRealtimeRPM)
	if active.Load() {
		t.Fatal("exit realtime command did not clear realtime state")
	}
}

func TestFlyDigiHIDReaderFailureClearsRealtimeState(t *testing.T) {
	m := &Manager{
		isConnected: true,
		flyDigiHID:  &flyDigiHIDDevice{},
	}
	m.flyDigiRealtimeRPMActive.Store(true)
	dev := m.flyDigiHID

	m.handleFlyDigiHIDReaderFailure(dev)

	if m.flyDigiRealtimeRPMActive.Load() {
		t.Fatal("reader failure kept stale realtime RPM state")
	}
	if m.flyDigiHID != nil {
		t.Fatal("reader failure kept the detached HID handle")
	}
}

func TestSyncFlyDigiRealtimeRPMStateIgnoresStaleGeneration(t *testing.T) {
	m := &Manager{}
	m.flyDigiRealtimeRPMActive.Store(true)
	m.connectionGen.Store(7)

	m.syncFlyDigiRealtimeRPMState(6, 0x00)
	if !m.flyDigiRealtimeRPMActive.Load() {
		t.Fatal("stale status frame changed realtime state")
	}

	m.syncFlyDigiRealtimeRPMState(7, 0x00)
	if m.flyDigiRealtimeRPMActive.Load() {
		t.Fatal("manual status frame did not clear realtime state")
	}

	m.syncFlyDigiRealtimeRPMState(7, 0x01)
	if !m.flyDigiRealtimeRPMActive.Load() {
		t.Fatal("realtime status frame did not restore realtime state")
	}
}

func TestFlyDigiRealtimeRPMStateRequiresCurrentGeneration(t *testing.T) {
	m := &Manager{}
	m.connectionGen.Store(12)
	m.flyDigiRealtimeRPMActive.Store(true)
	m.flyDigiRealtimeRPMStateGeneration.Store(11)

	if m.flyDigiRealtimeRPMIsActiveForCurrentConnection() {
		t.Fatal("stale realtime RPM state was accepted for the current connection")
	}

	m.flyDigiRealtimeRPMStateGeneration.Store(12)
	if !m.flyDigiRealtimeRPMIsActiveForCurrentConnection() {
		t.Fatal("current realtime RPM state was rejected")
	}
}

func TestFlyDigiRealtimeWriteFailuresTriggerRecovery(t *testing.T) {
	recovered := make(chan struct{}, 1)
	m := &Manager{
		isConnected:  true,
		deviceType:   types.DeviceTransportHID,
		onDisconnect: func() { recovered <- struct{}{} },
	}

	m.mutex.Lock()
	for i := 0; i < maxFlyDigiRealtimeWriteFailures; i++ {
		m.noteFlyDigiRealtimeWriteResultLocked(false)
	}
	m.mutex.Unlock()

	select {
	case <-recovered:
	case <-time.After(time.Second):
		t.Fatal("persistent realtime write failures did not trigger recovery")
	}
	if m.IsConnected() {
		t.Fatal("recovery left the HID manager connected")
	}
}
