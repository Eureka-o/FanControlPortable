//go:build !legacydevice

package device

import (
	"context"
	"testing"

	"github.com/TIANLI0/THRM/internal/deviceprofileexec"
	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

func TestManagerHasNoImplicitWiFiProfile(t *testing.T) {
	m := NewManager(nil)
	if m.shouldUseWiFi() {
		t.Fatal("new manager should not use WiFi without an enabled compatibility profile")
	}
	if profile := m.ActiveProfile(); profile.ID != "" || profile.Transport != "" {
		t.Fatalf("new manager active profile = %#v, want empty", profile)
	}
	if connected, info := m.Connect(); connected || info != nil {
		t.Fatalf("empty manager Connect() = %v, %#v, want false/nil", connected, info)
	}
}

func TestManagerClearProfileRemovesWiFiFallback(t *testing.T) {
	m := NewManager(nil)
	m.ConfigureProfile(types.DefaultWiFiPercentProfile("10.0.0.25"), "")
	if !m.shouldUseWiFi() {
		t.Fatal("configured WiFi profile should enable WiFi transport")
	}

	m.ClearProfile()
	if m.shouldUseWiFi() {
		t.Fatal("cleared manager should not fall back to WiFi")
	}
	if profile := m.ActiveProfile(); profile.ID != "" || profile.Transport != "" {
		t.Fatalf("cleared manager active profile = %#v, want empty", profile)
	}
}

func TestConfigureProfileDoesNotReplaceConnectedNativeRuntime(t *testing.T) {
	m := NewManager(nil)
	m.activeProfile = types.FlyDigiBS1Profile()
	m.deviceTransport = types.DeviceTransportBLE
	m.deviceType = types.DeviceTransportBLE
	m.isConnected = true

	if m.ConfigureProfile(types.DefaultWiFiPercentProfile("10.0.0.25"), "") {
		t.Fatal("ConfigureProfile should reject reconfiguration while a device is connected")
	}
	if profile := m.ActiveProfile(); profile.ID != types.FlyDigiBS1ProfileID || profile.Transport != types.DeviceTransportBLE {
		t.Fatalf("connected native profile was replaced: %#v", profile)
	}
}

func TestAutoConnectNativeReconnectsInvalidBLEExecutor(t *testing.T) {
	profile := types.FlyDigiBS1Profile()
	client := &managerFakeBLEClient{reads: [][]byte{
		deviceproto.BuildFrame(deviceproto.CmdStatusNotify, 1, 2, 0, 0xA4, 0x06, 0x08, 0x07),
	}}
	connectCalls := 0
	connector := deviceprofileexec.BLEConnectorFunc(func(context.Context, types.DeviceProfile) (deviceprofileexec.BLEClient, error) {
		connectCalls++
		return client, nil
	})
	executor, err := deviceprofileexec.NewBLEExecutor(profile, connector)
	if err != nil {
		t.Fatalf("NewBLEExecutor() error = %v", err)
	}

	m := NewManager(nil)
	m.bleConnector = connector
	m.bleExecutor = executor
	m.activeProfile = profile
	m.deviceTransport = types.DeviceTransportBLE
	m.deviceType = types.DeviceTransportBLE
	m.isConnected = true

	connected, info := m.AutoConnectNativeProfiles(nil)
	if !connected || connectCalls != 1 {
		t.Fatalf("AutoConnectNativeProfiles() = %v, calls=%d, want a real reconnect", connected, connectCalls)
	}
	if info["transport"] != types.DeviceTransportBLE || info["profileId"] != types.FlyDigiBS1ProfileID {
		t.Fatalf("reconnected device info = %#v, want BS1/BLE", info)
	}
	if active := m.ActiveProfile(); active.ID != types.FlyDigiBS1ProfileID || active.Transport != types.DeviceTransportBLE {
		t.Fatalf("reconnected active profile = %#v, want BS1/BLE", active)
	}
	if model := m.GetModelName(); model != "BS1" {
		t.Fatalf("reconnected model = %q, want BS1", model)
	}
	m.DisconnectSilently()
}

func TestDisconnectIfConnectionGenerationDoesNotDropNewSession(t *testing.T) {
	m := NewManager(nil)
	m.activeProfile = types.FlyDigiBS1Profile()
	m.deviceTransport = types.DeviceTransportBLE
	m.deviceType = types.DeviceTransportBLE
	m.isConnected = true
	oldGeneration := m.connectionGen.Add(1)
	m.connectionGen.Add(1)

	if m.DisconnectIfConnectionGeneration(oldGeneration) {
		t.Fatal("stale connection generation unexpectedly disconnected the manager")
	}
	if !m.IsConnected() {
		t.Fatal("stale cleanup cleared the superseding connection")
	}
}
