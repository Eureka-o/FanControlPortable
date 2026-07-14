package coreapp

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/device"
	"github.com/TIANLI0/THRM/internal/types"
)

func TestManualDisconnectInvalidatesBlockedReconnectSuccess(t *testing.T) {
	app := &CoreApp{deviceManager: device.NewManager(nil)}
	ctx, cancel := context.WithCancel(context.Background())
	app.reconnectMutex.Lock()
	app.reconnectGeneration = 1
	app.reconnectCancel = cancel
	app.reconnectMutex.Unlock()

	attemptStarted := make(chan struct{})
	releaseAttempt := make(chan struct{})
	disconnected := make(chan struct{})
	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		app.runReconnectLoop(ctx, "blocked-test", []time.Duration{0}, 1, func() reconnectAttemptResult {
			close(attemptStarted)
			<-releaseAttempt
			return reconnectAttemptResult{
				connected: true,
				disconnect: func() {
					close(disconnected)
				},
			}
		})
	}()
	<-attemptStarted
	app.DisconnectDevice()
	close(releaseAttempt)

	select {
	case <-loopDone:
	case <-time.After(time.Second):
		t.Fatal("stale reconnect loop did not finish")
	}
	select {
	case <-disconnected:
	default:
		t.Fatal("stale successful connection was not silently disconnected")
	}
	if !app.autoReconnectSuppressed.Load() {
		t.Fatal("stale reconnect cleared manual suppression")
	}
	app.mutex.RLock()
	connected := app.isConnected
	app.mutex.RUnlock()
	if connected {
		t.Fatal("stale reconnect published a successful connection")
	}
}

func TestStaleReconnectCleanupCompletesBeforeNewGenerationCommit(t *testing.T) {
	app := &CoreApp{deviceManager: device.NewManager(nil)}
	app.reconnectMutex.Lock()
	app.reconnectGeneration = 2
	app.reconnectMutex.Unlock()

	cleanupStarted := make(chan struct{})
	releaseCleanup := make(chan struct{})
	var physicalConnected atomic.Bool
	oldDone := make(chan struct{})
	go func() {
		defer close(oldDone)
		app.completeReconnectAttempt(context.Background(), 1, reconnectAttemptResult{
			connected: true,
			disconnect: func() {
				close(cleanupStarted)
				<-releaseCleanup
				physicalConnected.Store(false)
			},
		}, "old")
	}()
	<-cleanupStarted

	newAttemptStarted := make(chan struct{})
	newLoopDone := make(chan struct{})
	go func() {
		defer close(newLoopDone)
		app.runReconnectLoop(context.Background(), "new", []time.Duration{0}, 2, func() reconnectAttemptResult {
			close(newAttemptStarted)
			physicalConnected.Store(true)
			return reconnectAttemptResult{
				connected:   true,
				isConnected: physicalConnected.Load,
			}
		})
	}()
	<-newAttemptStarted
	select {
	case <-newLoopDone:
		close(releaseCleanup)
		<-oldDone
		t.Fatal("new generation committed before stale cleanup completed")
	case <-time.After(25 * time.Millisecond):
	}

	close(releaseCleanup)
	select {
	case <-oldDone:
	case <-time.After(time.Second):
		t.Fatal("stale cleanup did not finish")
	}
	select {
	case <-newLoopDone:
	case <-time.After(time.Second):
		t.Fatal("new generation did not resume after stale cleanup")
	}
	if physicalConnected.Load() {
		t.Fatal("stale cleanup left the superseding connection physically connected")
	}
	app.mutex.RLock()
	connected := app.isConnected
	app.mutex.RUnlock()
	if connected {
		t.Fatal("new generation published success after stale cleanup disconnected it")
	}
}

func TestReconnectDelayCanBeCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if waitForReconnectDelay(ctx, time.Hour) {
		t.Fatal("canceled reconnect delay should stop immediately")
	}
}

func TestReconnectRequestSupersedesWaitingGeneration(t *testing.T) {
	app := &CoreApp{}
	app.requestReconnect("first", []time.Duration{time.Hour})
	app.requestReconnect("second", []time.Duration{time.Hour})

	app.reconnectMutex.Lock()
	generation := app.reconnectGeneration
	app.reconnectMutex.Unlock()
	if generation != 2 {
		t.Fatalf("reconnect generation = %d, want 2", generation)
	}

	app.cancelReconnect()
	deadline := time.Now().Add(time.Second)
	for app.reconnectInProgress.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if app.reconnectInProgress.Load() {
		t.Fatal("latest reconnect loop did not stop after cancellation")
	}
}

func TestManualDisconnectCancelsDelayedStartupReconnect(t *testing.T) {
	app := &CoreApp{deviceManager: device.NewManager(nil)}
	app.requestStartupReconnect()
	app.DisconnectDevice()

	deadline := time.Now().Add(time.Second)
	for app.reconnectInProgress.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if app.reconnectInProgress.Load() {
		t.Fatal("startup reconnect did not stop after manual disconnect")
	}
	if !app.autoReconnectSuppressed.Load() {
		t.Fatal("manual disconnect did not preserve reconnect suppression")
	}
}

func TestManualReconnectSuppressionSurvivesSystemResume(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	app := newDeviceProfileTestApp(t, cfg)
	app.autoReconnectSuppressed.Store(true)
	app.systemSuspended.Store(true)

	// Keep the resume path from starting a real temperature reader in this unit test.
	app.monitoringMutex.Lock()
	app.monitoringDone = make(chan struct{})
	app.monitoringMutex.Unlock()

	app.handleSystemResume("test", 0, true)
	if !app.autoReconnectSuppressed.Load() {
		t.Fatal("system resume cleared a user's manual disconnect suppression")
	}
	if app.reconnectInProgress.Load() {
		t.Fatal("system resume started reconnect despite manual suppression")
	}
}

func TestReconnectTransportChoiceRetriesLateNativeBeforeCompatibility(t *testing.T) {
	nativeReady := false
	nativeCalls := 0
	compatibilityCalls := 0
	tryNative := func() (bool, map[string]string) {
		nativeCalls++
		return nativeReady, map[string]string{"transport": types.DeviceTransportHID}
	}
	tryCompatibility := func() (bool, map[string]string) {
		compatibilityCalls++
		return false, nil
	}

	if connected, _ := reconnectTransport(false, false, tryNative, tryCompatibility); connected {
		t.Fatal("first reconnect unexpectedly succeeded")
	}
	nativeReady = true
	if connected, info := reconnectTransport(false, false, tryNative, tryCompatibility); !connected || info["transport"] != types.DeviceTransportHID {
		t.Fatalf("late native reconnect = connected %v, info %#v", connected, info)
	}
	if nativeCalls != 2 {
		t.Fatalf("native attempts = %d, want 2", nativeCalls)
	}
	if compatibilityCalls != 1 {
		t.Fatalf("compatibility attempts = %d, want 1 because native hit short-circuits it", compatibilityCalls)
	}
}

func TestReconnectTransportUsesCompatibilityFastPathAfterCompatibilitySuccess(t *testing.T) {
	nativeCalls := 0
	compatibilityCalls := 0
	connected, _ := reconnectTransport(
		true,
		false,
		func() (bool, map[string]string) {
			nativeCalls++
			return false, nil
		},
		func() (bool, map[string]string) {
			compatibilityCalls++
			return true, nil
		},
	)
	if !connected || nativeCalls != 0 || compatibilityCalls != 1 {
		t.Fatalf("connected=%v nativeCalls=%d compatibilityCalls=%d, want compatibility-only reconnect", connected, nativeCalls, compatibilityCalls)
	}
}

func TestReconnectNativeTransportKeepsLastSuccessfulProfile(t *testing.T) {
	profile := types.FlyDigiBS2PROProfile()
	profileCalls := 0
	autoCalls := 0
	connected, info := reconnectNativeTransport(
		true,
		profile,
		func(got types.DeviceProfile) (bool, map[string]string) {
			profileCalls++
			if got.ID != profile.ID {
				t.Fatalf("reconnect profile = %q, want %q", got.ID, profile.ID)
			}
			return true, map[string]string{"transport": got.Transport}
		},
		func() (bool, map[string]string) {
			autoCalls++
			return false, nil
		},
	)
	if !connected || info["transport"] != types.DeviceTransportHID || profileCalls != 1 || autoCalls != 0 {
		t.Fatalf("connected=%v info=%#v profileCalls=%d autoCalls=%d", connected, info, profileCalls, autoCalls)
	}
}

func TestSupportedHIDArrivalReconnectPolicy(t *testing.T) {
	if !shouldReconnectOnHIDArrival(false, false, false, false, false) {
		t.Fatal("available HID arrival should trigger reconnect")
	}
	for _, test := range []struct {
		name       string
		stopping   bool
		suspended  bool
		suppressed bool
		core       bool
		manager    bool
	}{
		{name: "stopping", stopping: true},
		{name: "suspended", suspended: true},
		{name: "manual disconnect", suppressed: true},
		{name: "core connected", core: true},
		{name: "manager connected", manager: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if shouldReconnectOnHIDArrival(test.stopping, test.suspended, test.suppressed, test.core, test.manager) {
				t.Fatal("HID arrival should not trigger reconnect")
			}
		})
	}
}

func TestReapplyConfigAfterReconnectKeepsNativeRuntimeProfile(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.AutoControl = true
	app := newDeviceProfileTestApp(t, cfg)
	app.deviceManager.ConfigureProfile(types.FlyDigiBS1Profile(), "")

	app.reapplyConfigAfterReconnect()

	if got := app.deviceManager.ActiveProfile().ID; got != types.FlyDigiBS1ProfileID {
		t.Fatalf("active profile = %q, want %q", got, types.FlyDigiBS1ProfileID)
	}
}

func TestDidDeviceSwitchToManualModeRecognizesProtocolLabels(t *testing.T) {
	if !didDeviceSwitchToManualMode("auto/realtime RPM mode", "manual/fixed gear mode") {
		t.Fatal("expected protocol manual mode to be recognized")
	}
	if didDeviceSwitchToManualMode("manual/fixed gear mode", "挡位工作模式") {
		t.Fatal("manual alias to manual alias should not count as a new switch")
	}
	if didDeviceSwitchToManualMode("auto/realtime RPM mode", "hid") {
		t.Fatal("transport-only status should not count as manual mode")
	}
}

func TestGetDeviceStatusRequiresManagerConnection(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	app := newDeviceProfileTestApp(t, cfg)
	app.mutex.Lock()
	app.isConnected = true
	app.mutex.Unlock()

	status := app.GetDeviceStatus()
	if connected, _ := status["connected"].(bool); connected {
		t.Fatalf("connected = true with disconnected manager: %#v", status)
	}
	if _, ok := status["deviceProfile"]; ok {
		t.Fatalf("disconnected status should not expose configured deviceProfile: %#v", status)
	}
	if _, ok := status["deviceCapabilities"]; ok {
		t.Fatalf("disconnected status should not expose configured capabilities: %#v", status)
	}
}
