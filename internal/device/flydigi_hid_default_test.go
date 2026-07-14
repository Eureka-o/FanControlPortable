//go:build !legacydevice

package device

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestNormalBuildFlyDigiHIDProfileUsesRPMCapabilities(t *testing.T) {
	m := NewManager(nil)
	m.ConfigureProfile(types.FlyDigiBS2PROProfile(), "")

	if m.deviceTransport != types.DeviceTransportHID {
		t.Fatalf("device transport = %q, want hid", m.deviceTransport)
	}
	if m.activeProfile.SpeedUnit != types.FanSpeedUnitRPM {
		t.Fatalf("active profile unit = %q, want rpm", m.activeProfile.SpeedUnit)
	}
	if got := m.GetModelName(); got != "飞智（FlyDigi）BS2PRO" {
		t.Fatalf("model name = %q, want FlyDigi profile name", got)
	}
	if !m.shouldUseLegacyHIDLocked() {
		t.Fatal("FlyDigi HID profile should use the HID connection path")
	}

	info := m.flyDigiHIDInfoLocked(types.FlyDigiBS2PROProductID, `\\?\hid#vid_37d7&pid_1002`)
	if info["transport"] != types.DeviceTransportHID {
		t.Fatalf("connect info transport = %q, want hid", info["transport"])
	}
	if info["productId"] != "0x1002" {
		t.Fatalf("connect info productId = %q, want 0x1002", info["productId"])
	}
}

func TestNormalBuildFlyDigiHIDRejectsCommandsWhenDisconnected(t *testing.T) {
	m := NewManager(nil)
	m.ConfigureProfile(types.FlyDigiBS2PROProfile(), "")

	if ok := m.SetPercentSpeed(50); ok {
		t.Fatal("FlyDigi HID path must reject percent commands")
	}
	if ok := m.SetTargetSpeed(1800, types.FanSpeedUnitRPM); ok {
		t.Fatal("FlyDigi HID path must reject RPM commands while disconnected")
	}
	if ok := m.SetGearLight(true); ok {
		t.Fatal("FlyDigi HID path must reject gear-light commands while disconnected")
	}
	if err := m.SetLightStrip(types.GetDefaultLightStripConfig()); err == nil {
		t.Fatal("FlyDigi HID path must reject light-strip commands while disconnected")
	}
}

func TestFlyDigiHIDProductIDsPreferSpecificProfile(t *testing.T) {
	got := flyDigiHIDProductIDsForProfile(types.FlyDigiBS3PROProfileID)
	if len(got) != 1 || got[0] != types.FlyDigiBS3PROProductID {
		t.Fatalf("BS3PRO product ids = %#v, want only 0x1004", got)
	}

	got = flyDigiHIDProductIDsForProfile(types.LegacyRPMProfileID)
	if len(got) < 4 {
		t.Fatalf("fallback product ids = %#v, want all FlyDigi HID devices", got)
	}
}

func TestNativeAutoConnectCandidatesKeepUserNativeProfilesBeforeBuiltInFallbacks(t *testing.T) {
	userHID := types.LegacyRPMProfileForTransport(types.DeviceTransportHID)
	userHID.ID = "user.hid.rpm"
	userHID.DisplayName = "User HID RPM"
	userHID.BuiltIn = false

	userBLE := types.FlyDigiBS1Profile()
	userBLE.ID = "user.ble.rpm"
	userBLE.DisplayName = "User BLE RPM"
	userBLE.BuiltIn = false

	candidates := nativeAutoConnectCandidates([]types.DeviceProfile{
		types.DefaultWiFiPercentProfile("10.0.0.25"),
		types.FlyDigiBS1Profile(),
		userBLE,
		userHID,
	})

	gotIDs := make([]string, 0, len(candidates))
	for _, profile := range candidates {
		gotIDs = append(gotIDs, profile.ID)
	}
	wantIDs := []string{
		userHID.ID,
		types.LegacyRPMProfileID,
		userBLE.ID,
		types.FlyDigiBS1ProfileID,
	}
	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("candidate IDs = %#v, want %#v", gotIDs, wantIDs)
	}
	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Fatalf("candidate IDs = %#v, want %#v", gotIDs, wantIDs)
		}
	}
}

func TestNativeAutoConnectCandidatesPreferLastRuntimeProfile(t *testing.T) {
	previous := types.FlyDigiBS1Profile()
	previous.Connection.Endpoint = "AA:BB:CC:DD:EE:FF"

	candidates := nativeAutoConnectCandidates(nil, previous)
	if len(candidates) == 0 || candidates[0].ID != types.FlyDigiBS1ProfileID {
		t.Fatalf("first candidate = %#v, want previous BS1 profile", candidates)
	}
	if candidates[0].Connection.Endpoint != previous.Connection.Endpoint {
		t.Fatalf("preferred endpoint = %q, want %q", candidates[0].Connection.Endpoint, previous.Connection.Endpoint)
	}
}

func TestNativeBLEDeviceInfoUsesMatchedUserProfile(t *testing.T) {
	userBLE := types.FlyDigiBS1Profile()
	userBLE.ID = "user.ble.rpm"
	userBLE.DisplayName = "User BLE RPM"
	userBLE.Vendor = "DIY"
	userBLE.Model = "BLE Fan"
	userBLE.BuiltIn = false

	info := nativeBLEDeviceInfo(types.BLEDeviceInfo{
		Address:                   "AA:BB:CC:00:00:07",
		Name:                      "DIY BLE Fan",
		Matched:                   true,
		MatchedProfileID:          userBLE.ID,
		MatchedProfileDisplayName: userBLE.DisplayName,
	}, []types.DeviceProfile{userBLE, types.FlyDigiBS1Profile()})

	if info["profileId"] != userBLE.ID {
		t.Fatalf("profile id = %q, want %q", info["profileId"], userBLE.ID)
	}
	if info["manufacturer"] != "DIY" || info["product"] != userBLE.DisplayName {
		t.Fatalf("scan info = %#v, want user BLE identity", info)
	}
	if info["transport"] != types.DeviceTransportBLE || info["endpoint"] != "AA:BB:CC:00:00:07" {
		t.Fatalf("scan info = %#v, want BLE address endpoint", info)
	}
}

func TestNativeHIDDeviceInfoKeepsUserProfileID(t *testing.T) {
	userHID := types.LegacyRPMProfileForTransport(types.DeviceTransportHID)
	userHID.ID = "user.hid.rpm"
	userHID.DisplayName = "User HID RPM"
	userHID.Vendor = "DIY"
	userHID.BuiltIn = false

	info := nativeHIDDeviceInfo(userHID, types.FlyDigiBS2PROProductID, `\\?\hid#vid_37d7&pid_1002`)

	if info["profileId"] != userHID.ID {
		t.Fatalf("profile id = %q, want %q", info["profileId"], userHID.ID)
	}
	if info["manufacturer"] != "DIY" || info["product"] != userHID.DisplayName {
		t.Fatalf("scan info = %#v, want user HID identity", info)
	}
}

func TestNativeHIDDeviceInfoMapsFallbackToConcreteFlyDigiProfile(t *testing.T) {
	info := nativeHIDDeviceInfo(
		types.LegacyRPMProfileForTransport(types.DeviceTransportHID),
		types.FlyDigiBS3PROProductID,
		`\\?\hid#vid_37d7&pid_1004`,
	)

	if info["profileId"] != types.FlyDigiBS3PROProfileID {
		t.Fatalf("profile id = %q, want %q", info["profileId"], types.FlyDigiBS3PROProfileID)
	}
	if info["transport"] != types.DeviceTransportHID || info["productId"] != "0x1004" {
		t.Fatalf("scan info = %#v, want HID BS3PRO identity", info)
	}
}

func TestActiveProfileUsesConnectedFlyDigiProductID(t *testing.T) {
	m := NewManager(nil)
	m.ConfigureProfile(types.LegacyRPMProfile(), "")
	m.deviceType = types.DeviceTransportHID
	m.productID = types.FlyDigiBS3PROProductID

	profile := m.ActiveProfile()
	if profile.ID != types.FlyDigiBS3PROProfileID {
		t.Fatalf("runtime profile = %q, want %q", profile.ID, types.FlyDigiBS3PROProfileID)
	}
	if !profile.Capabilities.SupportsSmartStartStop || !profile.Capabilities.SupportsLighting {
		t.Fatalf("runtime FlyDigi capabilities did not use whitelist: %#v", profile.Capabilities)
	}
}

func TestFlyDigiHIDPathMatchesBluetoothLEVIDPID(t *testing.T) {
	path := `\\?\hid#{00001812-0000-1000-8000-00805f9b34fb}_dev_vid&0137d7_pid&1004_rev&0110_dc7f643a1704#9&b6797ea&0&0000`
	productID, ok := flyDigiHIDProductIDFromPath(path, flyDigiHIDProductIDsForProfile(types.LegacyRPMProfileID))
	if !ok {
		t.Fatal("expected Bluetooth LE HID device path to match FlyDigi VID/PID")
	}
	if productID != types.FlyDigiBS3PROProductID {
		t.Fatalf("matched product id = 0x%04X, want 0x%04X", productID, types.FlyDigiBS3PROProductID)
	}
}

func TestPadFlyDigiHIDReportUsesOutputReportLength(t *testing.T) {
	shortReport := []byte{0x02, 0x5A, 0xA5, 0x23}
	padded := padFlyDigiHIDReport(shortReport, hidLightReportLen)
	if len(padded) != hidLightReportLen {
		t.Fatalf("padded report length = %d, want %d", len(padded), hidLightReportLen)
	}
	for i, value := range shortReport {
		if padded[i] != value {
			t.Fatalf("padded report byte %d = 0x%02X, want 0x%02X", i, padded[i], value)
		}
	}

	fullReport := make([]byte, hidLightReportLen)
	if got := padFlyDigiHIDReport(fullReport, hidLightReportLen); &got[0] != &fullReport[0] {
		t.Fatal("full-length HID reports should be reused without another allocation")
	}
}

func TestFlyDigiHIDTargetUpdateDoesNotFakeCurrentRPM(t *testing.T) {
	m := NewManager(nil)

	m.storeFlyDigiHIDFanDataLocked(2100, "自动模式(实时转速)")
	got := m.GetCurrentFanData()
	if got == nil {
		t.Fatal("expected synthetic HID fan data")
	}
	if got.CurrentRPM != 0 || got.TargetRPM != 2100 {
		t.Fatalf("initial synthetic HID speed = %d/%d, want 0/2100", got.CurrentRPM, got.TargetRPM)
	}

	m.currentFanData.Store(&types.FanData{
		CurrentRPM: 1500,
		TargetRPM:  1800,
		WorkMode:   "自动模式(实时转速)",
		Transport:  types.DeviceTransportHID,
		SpeedUnit:  types.FanSpeedUnitRPM,
	})
	m.storeFlyDigiHIDFanDataLocked(2300, "挡位工作模式")

	got = m.GetCurrentFanData()
	if got.CurrentRPM != 1500 || got.TargetRPM != 2300 {
		t.Fatalf("HID speed after target update = %d/%d, want 1500/2300", got.CurrentRPM, got.TargetRPM)
	}
	if got.WorkMode != "挡位工作模式" {
		t.Fatalf("work mode = %q, want 挡位工作模式", got.WorkMode)
	}
}

func TestFlyDigiHIDReadErrorPolicyToleratesTransientFailures(t *testing.T) {
	next, stop := flyDigiHIDReadErrorState(errFlyDigiHIDTimeout, 3)
	if next != 0 || stop {
		t.Fatalf("timeout state = (%d, %v), want reset and keep reading", next, stop)
	}

	transient := errors.New("temporary read failure")
	for i := 0; i < flyDigiHIDMaxConsecutiveReadErrors-1; i++ {
		next, stop = flyDigiHIDReadErrorState(transient, i)
		if next != i+1 || stop {
			t.Fatalf("transient state %d = (%d, %v), want (%d, false)", i, next, stop, i+1)
		}
	}

	next, stop = flyDigiHIDReadErrorState(transient, flyDigiHIDMaxConsecutiveReadErrors-1)
	if next != flyDigiHIDMaxConsecutiveReadErrors || !stop {
		t.Fatalf("threshold state = (%d, %v), want (%d, true)", next, stop, flyDigiHIDMaxConsecutiveReadErrors)
	}
}

func TestDisconnectForRecoveryDetachesStalledFlyDigiHID(t *testing.T) {
	m := NewManager(nil)
	stalled := &flyDigiHIDDevice{}
	stop := make(chan struct{})
	m.flyDigiHID = stalled
	m.flyDigiHIDStop = stop
	m.flyDigiHIDDone = make(chan struct{})
	m.isConnected = true
	m.deviceType = types.DeviceTransportHID
	m.productID = types.FlyDigiBS2PROProductID
	stalledGeneration := m.connectionGen.Add(1)

	m.DisconnectForRecovery()

	select {
	case <-stop:
	default:
		t.Fatal("recovery disconnect did not stop the stalled HID reader")
	}
	if m.flyDigiHID != nil || m.isConnected || m.deviceType != "" || m.productID != 0 {
		t.Fatalf("stalled HID state was not detached: device=%p connected=%v type=%q product=0x%04X", m.flyDigiHID, m.isConnected, m.deviceType, m.productID)
	}
	report := []byte{0x02, 0x5A, 0xA5, 0xEF, 0x00, 0x2A, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00}
	if m.handleFlyDigiHIDRX(stalledGeneration, report) {
		t.Fatal("detached HID reader generation remained active")
	}
}

func TestFlyDigiHIDConnectionGenerationRejectsStaleReaderData(t *testing.T) {
	m := NewManager(nil)
	stale := &flyDigiHIDDevice{}
	current := &flyDigiHIDDevice{}
	m.flyDigiHID = stale
	staleGeneration := m.connectionGen.Add(1)
	m.flyDigiHID = current
	currentGeneration := m.connectionGen.Add(1)

	report := []byte{0x02, 0x5A, 0xA5, 0xEF, 0x00, 0x2A, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00}
	binary.LittleEndian.PutUint16(report[8:10], 1560)
	binary.LittleEndian.PutUint16(report[10:12], 2200)

	if m.handleFlyDigiHIDRX(staleGeneration, report) {
		t.Fatal("stale HID reader data was accepted")
	}
	if got := m.GetCurrentFanData(); got != nil {
		t.Fatalf("stale HID reader overwrote fan data: %#v", got)
	}
	if !m.handleFlyDigiHIDRX(currentGeneration, report) {
		t.Fatal("current HID reader data was rejected")
	}
	if got := m.GetCurrentFanData(); got == nil || got.CurrentRPM != 1560 || got.TargetRPM != 2200 {
		t.Fatalf("current HID reader data = %#v, want 1560/2200 RPM", got)
	}
	if got := m.ConnectionGeneration(); got != currentGeneration {
		t.Fatalf("connection generation = %d, want %d", got, currentGeneration)
	}
}

func TestFlyDigiHIDReadyFrameDoesNotWaitForManagerLock(t *testing.T) {
	m := NewManager(nil)
	dev := &flyDigiHIDDevice{}
	m.flyDigiHID = dev
	generation := m.connectionGen.Add(1)
	report := []byte{0x02, 0x5A, 0xA5, 0xEF, 0x00, 0x2A, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00}
	binary.LittleEndian.PutUint16(report[8:10], 1560)

	m.mutex.Lock()
	result := make(chan bool, 1)
	go func() {
		result <- m.handleFlyDigiHIDRX(generation, report)
	}()
	select {
	case accepted := <-result:
		m.mutex.Unlock()
		if !accepted {
			t.Fatal("current HID ready frame was rejected")
		}
	case <-time.After(100 * time.Millisecond):
		m.mutex.Unlock()
		<-result
		t.Fatal("HID ready frame waited for the manager lock")
	}
}

func TestWaitForFlyDigiHIDReady(t *testing.T) {
	ready := make(chan struct{})
	close(ready)
	if !waitForFlyDigiHIDReady(ready, time.Second) {
		t.Fatal("ready status frame should complete HID connection")
	}

	if waitForFlyDigiHIDReady(make(chan struct{}), time.Millisecond) {
		t.Fatal("missing status frame should time out HID connection")
	}
}

func TestFlyDigiQueryResponseFramesFiltersUnrelatedTraffic(t *testing.T) {
	frames := []types.DeviceDebugFrame{
		{Direction: "tx", ChecksumOK: true, Command: "0x23"},
		{Direction: "rx", ChecksumOK: false, Command: "0x23"},
		{Direction: "rx", ChecksumOK: true, Command: "0xEF"},
		{Direction: "rx", ChecksumOK: true, Command: "0x23"},
	}
	got := flyDigiQueryResponseFrames(0x23, frames)
	if len(got) != 1 || got[0].Command != "0x23" {
		t.Fatalf("matched frames = %#v, want one valid 0x23 response", got)
	}
}

func TestParseFlyDigiFanDataFallsBackToReferenceOffsets(t *testing.T) {
	report := []byte{
		0x02, 0x5A, 0xA5, 0xEF,
		0x00, // deliberately invalid protocol length; reference-style offsets still carry status.
		0x2A, 0x01, 0x00,
		0x00, 0x00,
		0x00, 0x00,
	}
	binary.LittleEndian.PutUint16(report[8:10], 1560)
	binary.LittleEndian.PutUint16(report[10:12], 2200)

	fanData := parseFlyDigiFanData(report)
	if fanData == nil {
		t.Fatal("expected reference-offset FlyDigi status frame to be parsed")
	}
	if fanData.CurrentRPM != 1560 || fanData.TargetRPM != 2200 {
		t.Fatalf("rpm = %d/%d, want 1560/2200", fanData.CurrentRPM, fanData.TargetRPM)
	}
}
