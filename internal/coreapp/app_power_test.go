package coreapp

import (
	"testing"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/bridge"
	"github.com/Eureka-o/FanControlPortable/internal/config"
	"github.com/Eureka-o/FanControlPortable/internal/device"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func TestSystemResumeDetectionThreshold(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		want     time.Duration
	}{
		{name: "uses floor for fast polling", interval: time.Second, want: 20 * time.Second},
		{name: "scales with normal polling", interval: 5 * time.Second, want: 30 * time.Second},
		{name: "caps long polling threshold", interval: 20 * time.Second, want: 45 * time.Second},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := systemResumeDetectionThreshold(test.interval); got != test.want {
				t.Fatalf("systemResumeDetectionThreshold(%v) = %v, want %v", test.interval, got, test.want)
			}
		})
	}
}

func TestShouldRecoverFromSystemResumeGap(t *testing.T) {
	tests := []struct {
		name     string
		gap      time.Duration
		interval time.Duration
		want     bool
	}{
		{name: "ignores normal short gap", gap: 10 * time.Second, interval: time.Second, want: false},
		{name: "detects floor threshold", gap: 20 * time.Second, interval: time.Second, want: true},
		{name: "requires scaled threshold", gap: 40 * time.Second, interval: 10 * time.Second, want: false},
		{name: "detects capped threshold", gap: 45 * time.Second, interval: 10 * time.Second, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldRecoverFromSystemResumeGap(test.gap, test.interval); got != test.want {
				t.Fatalf("shouldRecoverFromSystemResumeGap(%v, %v) = %v, want %v", test.gap, test.interval, got, test.want)
			}
		})
	}
}

func TestShouldReconnectAfterResume(t *testing.T) {
	tests := []struct {
		name                    string
		proactivelySuspended    bool
		resumeReconnectWanted   bool
		autoReconnectSuppressed bool
		forceReconnect          bool
		want                    bool
	}{
		{name: "connected before suspend", proactivelySuspended: true, resumeReconnectWanted: true, want: true},
		{name: "manual disconnect before suspend", proactivelySuspended: true, autoReconnectSuppressed: true, forceReconnect: true},
		{name: "unexpected resume", forceReconnect: true, want: true},
		{name: "manual disconnect without suspend", autoReconnectSuppressed: true, forceReconnect: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldReconnectAfterResume(test.proactivelySuspended, test.resumeReconnectWanted, test.autoReconnectSuppressed, test.forceReconnect); got != test.want {
				t.Fatalf("shouldReconnectAfterResume() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestResumeReconnectWantedOnSuspend(t *testing.T) {
	tests := []struct {
		name                    string
		coreConnected           bool
		deviceConnected         bool
		reconnectInProgress     bool
		autoReconnectSuppressed bool
		want                    bool
	}{
		{name: "core connected", coreConnected: true, want: true},
		{name: "device handle open", deviceConnected: true, want: true},
		{name: "reconnect pending", reconnectInProgress: true, want: true},
		{name: "nothing active"},
		{name: "manual disconnect wins", coreConnected: true, reconnectInProgress: true, autoReconnectSuppressed: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resumeReconnectWantedOnSuspend(test.coreConnected, test.deviceConnected, test.reconnectInProgress, test.autoReconnectSuppressed); got != test.want {
				t.Fatalf("resumeReconnectWantedOnSuspend() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestSystemResumeReconnectDelaysAreStaged(t *testing.T) {
	want := []time.Duration{
		3 * time.Second,
		8 * time.Second,
		15 * time.Second,
		20 * time.Second,
		30 * time.Second,
		30 * time.Second,
		60 * time.Second,
		60 * time.Second,
	}
	got := systemResumeReconnectDelays()
	if len(got) != len(want) {
		t.Fatalf("resume reconnect delays = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("resume reconnect delays = %v, want %v", got, want)
		}
	}
}

func TestCurrentSuspendGenerationRequiresActiveSuspend(t *testing.T) {
	app := &CoreApp{}
	generation := app.suspendGeneration.Add(1)
	if app.isCurrentSuspendGeneration(generation) {
		t.Fatal("inactive suspend generation reported current")
	}
	app.systemSuspended.Store(true)
	if !app.isCurrentSuspendGeneration(generation) {
		t.Fatal("active suspend generation reported stale")
	}
	app.suspendGeneration.Add(1)
	if app.isCurrentSuspendGeneration(generation) {
		t.Fatal("stale suspend generation reported current")
	}
}

func TestShouldReturnFlyDigiToFirmwareOnStop(t *testing.T) {
	hidProfile := types.DeviceProfile{
		Transport: types.DeviceTransportHID,
		Capabilities: types.DeviceCapabilities{
			SupportsSmartStartStop: true,
		},
	}
	tests := []struct {
		name      string
		mode      string
		profile   types.DeviceProfile
		connected bool
		want      bool
	}{
		{name: "delayed FlyDigi HID", mode: "delayed", profile: hidProfile, connected: true, want: true},
		{name: "immediate FlyDigi HID", mode: "immediate", profile: hidProfile, connected: true, want: true},
		{name: "disabled smart start stop", mode: "off", profile: hidProfile, connected: true},
		{name: "disconnected", mode: "delayed", profile: hidProfile},
		{name: "BLE", mode: "delayed", profile: types.DeviceProfile{Transport: types.DeviceTransportBLE, Capabilities: hidProfile.Capabilities}, connected: true},
		{name: "USB", mode: "delayed", profile: types.DeviceProfile{Transport: types.DeviceTransportUSB, Capabilities: hidProfile.Capabilities}, connected: true},
		{name: "unsupported profile", mode: "delayed", profile: types.DeviceProfile{Transport: types.DeviceTransportHID}, connected: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := types.AppConfig{SmartStartStop: test.mode}
			if got := shouldReturnFlyDigiToFirmwareOnStop(cfg, test.profile, test.connected); got != test.want {
				t.Fatalf("shouldReturnFlyDigiToFirmwareOnStop() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestShouldSendTargetRPM(t *testing.T) {
	tests := []struct {
		name          string
		targetRPM     int
		prevTargetRPM int
		minRPMChange  int
		fanData       *types.FanData
		want          bool
	}{
		{name: "rejects zero target", targetRPM: 0, prevTargetRPM: -1, minRPMChange: 50, want: false},
		{name: "sends initial positive target", targetRPM: 1800, prevTargetRPM: -1, minRPMChange: 50, want: true},
		{name: "sends significant target change", targetRPM: 1900, prevTargetRPM: 1800, minRPMChange: 50, want: true},
		{name: "skips small target change", targetRPM: 1820, prevTargetRPM: 1800, minRPMChange: 50, want: false},
		{name: "resends when device reports zero target", targetRPM: 1800, prevTargetRPM: 1800, minRPMChange: 50, fanData: &types.FanData{TargetRPM: 0}, want: true},
		{name: "resends when device target drifts", targetRPM: 1800, prevTargetRPM: 1800, minRPMChange: 50, fanData: &types.FanData{TargetRPM: 1700}, want: true},
		{name: "keeps small device target drift", targetRPM: 1800, prevTargetRPM: 1800, minRPMChange: 50, fanData: &types.FanData{CurrentRPM: 1790, TargetRPM: 1775}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldSendTargetRPM(test.targetRPM, test.prevTargetRPM, test.minRPMChange, test.fanData); got != test.want {
				t.Fatalf("shouldSendTargetRPM() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestHandleSystemResumeUnblocksWritesUnlessMonitorOnly(t *testing.T) {
	cfgMgr := config.NewManager(t.TempDir(), nil)
	cfgMgr.Set(types.GetDefaultConfig(false))
	app := &CoreApp{
		configManager: cfgMgr,
		deviceManager: device.NewManager(nil),
		bridgeManager: bridge.NewManager(nil),
	}

	app.systemSuspended.Store(true)
	app.deviceManager.BlockWrites()
	app.handleSystemResume("test", time.Minute, false)
	if app.deviceManager.WritesBlocked() {
		t.Fatal("handleSystemResume() left writesBlocked=true when monitorOnly=false")
	}

	app.monitorOnly.Store(true)
	app.systemSuspended.Store(true)
	app.deviceManager.BlockWrites()
	app.handleSystemResume("test-monitor-only", time.Minute, false)
	if !app.deviceManager.WritesBlocked() {
		t.Fatal("handleSystemResume() unblocked writes while monitorOnly=true")
	}
}

func TestCleanupBroadcastsToAllWaiters(t *testing.T) {
	app := &CoreApp{
		cleanupChan: make(chan bool),
	}
	waiter1 := make(chan struct{})
	waiter2 := make(chan struct{})
	go func() {
		<-app.cleanupChan
		close(waiter1)
	}()
	go func() {
		<-app.cleanupChan
		close(waiter2)
	}()

	app.cleanup()
	app.cleanup()

	for idx, ch := range []chan struct{}{waiter1, waiter2} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("waiter %d did not unblock on cleanup()", idx+1)
		}
	}
}

func TestCanTickSceneBlocksDuringExclusiveStates(t *testing.T) {
	app := &CoreApp{}
	if app.canTickSceneForActive(false) {
		t.Fatal("canTickSceneForActive(false) = true")
	}
	if !app.canTickSceneForActive(true) {
		t.Fatal("canTickSceneForActive(true) = false for idle active BlackShark")
	}

	checks := []struct {
		name  string
		setup func(bool)
	}{
		{name: "systemSuspended", setup: func(v bool) { app.systemSuspended.Store(v) }},
		{name: "resumeRecoveryRunning", setup: func(v bool) { app.resumeRecoveryRunning.Store(v) }},
		{name: "reconnectInProgress", setup: func(v bool) { app.reconnectInProgress.Store(v) }},
		{
			name: "noiseDiagnosticLease",
			setup: func(v bool) {
				app.noiseDiagnosticMu.Lock()
				if v {
					app.noiseDiagnosticLease = &noiseDiagnosticLease{
						sessionID: "test",
						expiresAt: time.Now().Add(time.Minute),
					}
				} else {
					app.noiseDiagnosticLease = nil
				}
				app.noiseDiagnosticMu.Unlock()
			},
		},
	}
	for _, tc := range checks {
		tc.setup(true)
		if app.canTickSceneForActive(true) {
			t.Fatalf("canTickSceneForActive(true) = true while %s active", tc.name)
		}
		tc.setup(false)
		if !app.canTickSceneForActive(true) {
			t.Fatalf("canTickSceneForActive(true) = false after clearing %s", tc.name)
		}
	}
}
