//go:build !legacydevice

package device

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/deviceprofileexec"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func TestNativeBLEAutoScanCooldownIsCandidateScoped(t *testing.T) {
	failed := types.FlyDigiBS1Profile()
	failed.Connection.Endpoint = "AA:BB:CC:DD:EE:01"
	otherProfile := failed
	otherProfile.ID = "user.other.ble"
	otherEndpoint := failed
	otherEndpoint.Connection.Endpoint = "AA:BB:CC:DD:EE:02"
	now := time.Now()
	candidates := []types.DeviceProfile{failed, otherProfile, types.FlyDigiBS2PROProfile(), otherEndpoint, types.BlackSharkBRB02USBProfile()}
	failures := map[string]time.Time{nativeBLEAutoConnectCandidateKey(failed): now}

	got := filterNativeAutoConnectCandidatesForBLECooldown(candidates, failures, now)
	if !reflect.DeepEqual(got, candidates[1:]) {
		t.Fatalf("filtered candidates = %#v, want only the failed candidate removed", got)
	}
	got = filterNativeAutoConnectCandidatesForBLECooldown(candidates, failures, now.Add(idleBLEScanCooldown))
	if !reflect.DeepEqual(got, candidates) {
		t.Fatalf("expired cooldown filtered candidates = %#v, want all candidates", got)
	}
	failed.Connection.Endpoint = " aa:bb:cc:dd:ee:01 "
	if got := filterNativeAutoConnectCandidatesForBLECooldown([]types.DeviceProfile{failed}, failures, now); len(got) != 0 {
		t.Fatal("equivalent BLE address should retain the same cooldown")
	}
}

func TestNativeBLEScanSelectsNonCoolingDiscoveredEndpoint(t *testing.T) {
	profile := types.FlyDigiBS1Profile()
	profile.Connection.Endpoint = ""
	now := time.Now()
	first := profile
	first.Connection.Endpoint = "AA:BB:CC:DD:EE:01"
	second := profile
	second.Connection.Endpoint = "AA:BB:CC:DD:EE:02"

	selected, ok := nativeBLEAutoConnectProfileFromScan(profile, []map[string]string{
		{"endpoint": first.Connection.Endpoint},
		{"endpoint": second.Connection.Endpoint},
	}, map[string]time.Time{nativeBLEAutoConnectCandidateKey(first): now}, now)
	if !ok || selected.Connection.Endpoint != second.Connection.Endpoint {
		t.Fatalf("selected endpoint = %q, ok=%v, want non-cooling %q", selected.Connection.Endpoint, ok, second.Connection.Endpoint)
	}
}

func TestBlackSharkUSBProbeOnlyRunsWhenBlackSharkIsEligible(t *testing.T) {
	if shouldProbeBlackSharkUSB(types.FlyDigiBS1Profile()) {
		t.Fatal("non-Black-Shark auto-connect should not probe the USB transport")
	}
	if !shouldProbeBlackSharkUSB(types.DeviceProfile{}) || !shouldProbeBlackSharkUSB(types.BlackSharkBRB02Profile()) {
		t.Fatal("eligible Black-Shark profile should probe USB before BLE")
	}
}

func TestFailedNativeProbeRestoresPreviousProfile(t *testing.T) {
	for _, automatic := range []bool{false, true} {
		for _, panics := range []bool{false, true} {
			t.Run(fmt.Sprintf("automatic=%v/panic=%v", automatic, panics), func(t *testing.T) {
				m := NewManager(nil)
				previous := types.FlyDigiBS3Profile()
				m.ConfigureProfile(previous, "")
				m.bleConnector = deviceprofileexec.BLEConnectorFunc(func(context.Context, types.DeviceProfile) (deviceprofileexec.BLEClient, error) {
					if panics {
						panic("test native backend failure")
					}
					return nil, errors.New("test native backend failure")
				})
				probe := types.FlyDigiBS1Profile()
				probe.Connection.Endpoint = "AA:BB:CC:DD:EE:FF"
				var recovered any
				func() {
					defer func() { recovered = recover() }()
					var connected bool
					if automatic {
						connected, _ = m.connectNativeProfileContextIfDisconnected(context.Background(), probe)
					} else {
						connected, _ = m.ConnectNativeProfileContext(context.Background(), probe)
					}
					if connected {
						t.Fatal("failed probe unexpectedly connected")
					}
				}()
				if panics != (recovered != nil) {
					t.Fatalf("recovered = %v, want panic=%v", recovered, panics)
				}
				if got := m.ActiveProfile(); !reflect.DeepEqual(got, previous) {
					t.Fatalf("failed probe replaced previous profile: got %#v, want %#v", got, previous)
				}
				if m.IsConnected() {
					t.Fatal("failed probe marked manager as connected")
				}
			})
		}
	}
}

func TestAutoConnectNativeBLEFailureDoesNotCoolOtherCandidate(t *testing.T) {
	failed := types.FlyDigiBS1Profile()
	failed.ID = "user.failed.ble"
	failed.BuiltIn = false
	failed.Connection.Endpoint = "AA:BB:CC:DD:EE:01"
	m := NewManager(nil)
	m.activeProfile = failed
	m.bleScanner = deviceprofileexec.BLEScannerFunc(func(context.Context, types.BLEScanParams) ([]types.BLEDeviceInfo, error) {
		return []types.BLEDeviceInfo{{Address: "AA:BB:CC:DD:EE:02", Name: "FlyDigi BS1"}}, nil
	})
	var attempted []string
	m.bleConnector = deviceprofileexec.BLEConnectorFunc(func(_ context.Context, profile types.DeviceProfile) (deviceprofileexec.BLEClient, error) {
		attempted = append(attempted, profile.ID)
		if profile.ID == failed.ID {
			return nil, errors.New("candidate unavailable")
		}
		return &managerFakeBLEClient{}, nil
	})
	t.Cleanup(m.DisconnectSilently)

	connected, info := m.AutoConnectNativeProfiles([]types.DeviceProfile{failed})
	if !connected || info["profileId"] != types.FlyDigiBS1ProfileID {
		t.Fatalf("AutoConnectNativeProfiles() = %v, %#v, attempted=%#v, want the other BLE candidate connected", connected, info, attempted)
	}
	if want := []string{failed.ID, types.FlyDigiBS1ProfileID}; !reflect.DeepEqual(attempted, want) {
		t.Fatalf("attempted candidates = %#v, want %#v", attempted, want)
	}
	if len(m.failedAutoBLEScans) != 1 || m.failedAutoBLEScans[nativeBLEAutoConnectCandidateKey(failed)].IsZero() {
		t.Fatalf("failed candidates = %#v, want only the failed candidate", m.failedAutoBLEScans)
	}
	failedAt := m.failedAutoBLEScans[nativeBLEAutoConnectCandidateKey(failed)]
	m.DisconnectSilently()
	m.ConfigureProfile(failed, "")
	if connected, _ := m.AutoConnectNativeProfiles([]types.DeviceProfile{failed}); !connected {
		t.Fatal("other BLE candidate should remain eligible during the failed candidate's cooldown")
	}
	if want := []string{failed.ID, types.FlyDigiBS1ProfileID, types.FlyDigiBS1ProfileID}; !reflect.DeepEqual(attempted, want) {
		t.Fatalf("attempted candidates = %#v, want the failed candidate skipped on retry", attempted)
	}
	if !m.failedAutoBLEScans[nativeBLEAutoConnectCandidateKey(failed)].Equal(failedAt) {
		t.Fatal("skipping a cooling-down candidate should not extend its cooldown")
	}
}

func TestNativeBLESuccessClearsCooldown(t *testing.T) {
	for _, mode := range []string{"automatic", "explicit"} {
		t.Run(mode, func(t *testing.T) {
			profile := types.FlyDigiBS1Profile()
			profile.Connection.Endpoint = "AA:BB:CC:DD:EE:01"
			m := NewManager(nil)
			m.ConfigureProfile(profile, "")
			failedAt := time.Now()
			if mode == "automatic" {
				failedAt = failedAt.Add(-idleBLEScanCooldown)
			}
			m.failedAutoBLEScans = map[string]time.Time{nativeBLEAutoConnectCandidateKey(profile): failedAt}
			attempts := 0
			m.bleConnector = deviceprofileexec.BLEConnectorFunc(func(context.Context, types.DeviceProfile) (deviceprofileexec.BLEClient, error) {
				attempts++
				return &managerFakeBLEClient{}, nil
			})
			t.Cleanup(m.DisconnectSilently)

			var connected bool
			switch mode {
			case "explicit":
				connected, _ = m.ConnectNativeProfile(profile)
			default:
				connected, _ = m.AutoConnectNativeProfiles(nil)
			}
			if !connected || len(m.failedAutoBLEScans) != 0 {
				t.Fatalf("successful connection = %v, failed candidates = %#v, want true and cleared cooldown", connected, m.failedAutoBLEScans)
			}
			if connected, _ := m.AutoConnectNativeProfiles(nil); !connected || attempts != 2 {
				t.Fatalf("immediate automatic reconnect = %v, attempts = %d, want true and 2", connected, attempts)
			}
		})
	}
}

func TestCanceledNativeBLEAttemptDoesNotCooldownCandidate(t *testing.T) {
	profile := types.FlyDigiBS1Profile()
	m := NewManager(nil)
	m.activeProfile = profile
	m.bleScanner = deviceprofileexec.BLEScannerFunc(func(context.Context, types.BLEScanParams) ([]types.BLEDeviceInfo, error) {
		return []types.BLEDeviceInfo{{Address: "AA:BB:CC:DD:EE:03", Name: "FlyDigi BS1"}}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.bleConnector = deviceprofileexec.BLEConnectorFunc(func(context.Context, types.DeviceProfile) (deviceprofileexec.BLEClient, error) {
		cancel()
		return nil, ctx.Err()
	})

	if connected, _ := m.AutoConnectNativeProfilesContext(ctx, nil); connected {
		t.Fatal("canceled BLE attempt unexpectedly connected")
	}
	if len(m.failedAutoBLEScans) != 0 {
		t.Fatalf("canceled attempt created cooldown: %#v", m.failedAutoBLEScans)
	}
}

func TestAutoBLEDiscoveryDoesNotHoldManagerStateLock(t *testing.T) {
	profile := types.FlyDigiBS1Profile()
	m := NewManager(nil)
	m.activeProfile = profile
	started := make(chan struct{})
	m.bleScanner = deviceprofileexec.BLEScannerFunc(func(ctx context.Context, _ types.BLEScanParams) ([]types.BLEDeviceInfo, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = m.AutoConnectNativeProfilesContext(ctx, []types.DeviceProfile{profile})
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("BLE scan did not start")
	}

	configured := make(chan bool, 1)
	go func() { configured <- m.ConfigureProfile(types.DefaultWiFiPercentProfile("10.0.0.25"), "") }()
	select {
	case ok := <-configured:
		if !ok {
			t.Fatal("manager state reconfiguration was rejected while BLE discovery ran")
		}
	case <-time.After(time.Second):
		t.Fatal("manager state lock remained held for the duration of BLE discovery")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled auto-connect did not stop")
	}
}

func TestLegacyBLEScanTimestampDoesNotCooldownCandidates(t *testing.T) {
	m := NewManager(nil)
	now := time.Now()
	m.SeedLastAutoBLEScanAt(now)
	if got := m.LastAutoBLEScanAt(); !got.Equal(now) {
		t.Fatalf("LastAutoBLEScanAt() = %v, want %v", got, now)
	}
	candidates := []types.DeviceProfile{types.FlyDigiBS1Profile(), types.BlackSharkBRB02Profile()}
	if got := filterNativeAutoConnectCandidatesForBLECooldown(candidates, m.failedAutoBLEScans, now); !reflect.DeepEqual(got, candidates) {
		t.Fatalf("legacy timestamp filtered candidates = %#v, want all BLE candidates", got)
	}
}
