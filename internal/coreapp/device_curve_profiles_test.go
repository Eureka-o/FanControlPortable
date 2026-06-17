package coreapp

import (
	"reflect"
	"testing"

	"github.com/TIANLI0/THRM/internal/curveprofiles"
	"github.com/TIANLI0/THRM/internal/types"
)

func testRPMDeviceProfile() types.DeviceProfile {
	return types.DeviceProfile{
		ID:          "user.serial.rpm",
		DisplayName: "Serial RPM",
		Vendor:      "FanControl",
		Model:       "Serial RPM",
		Transport:   types.DeviceTransportSerial,
		SpeedUnit:   types.FanSpeedUnitRPM,
		SpeedRange:  types.DefaultRPMSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			SerialPort:     "COM9",
			SerialBaudRate: 115200,
			SerialDataBits: 8,
			SerialStopBits: 1,
			SerialParity:   "none",
		},
		Capabilities: types.DeviceCapabilities{
			Transport:         types.DeviceTransportSerial,
			SpeedUnit:         types.FanSpeedUnitRPM,
			SpeedRange:        types.DefaultRPMSpeedRange(),
			SupportsReadState: true,
			SupportsSetSpeed:  true,
		},
	}
}

func offsetCurveSpeeds(curve []types.FanCurvePoint, delta int) []types.FanCurvePoint {
	out := curveprofiles.CloneCurve(curve)
	for i := range out {
		next := out[i].RPM + delta
		minSpeed, maxSpeed := types.SpeedRangeForUnit(types.FanSpeedUnitRPM)
		if out[i].RPM <= types.FanSpeedMaxPercent {
			minSpeed, maxSpeed = types.SpeedRangeForUnit(types.FanSpeedUnitPercent)
		}
		if next < minSpeed {
			next = minSpeed
		}
		if next > maxSpeed {
			next = maxSpeed
		}
		if i > 0 && next < out[i-1].RPM {
			next = out[i-1].RPM
		}
		out[i].RPM = next
	}
	return out
}

func TestDeviceCurveProfilesFollowActiveDeviceProfile(t *testing.T) {
	wifi := types.DefaultWiFiPercentProfile("10.0.0.25")
	rpm := testRPMDeviceProfile()
	wifiCurve := offsetCurveSpeeds(types.GetDefaultFanCurve(), 3)

	cfg := types.GetDefaultConfig(false)
	cfg.DeviceTransport = types.DeviceTransportWiFi
	cfg.FanControlDeviceIp = "10.0.0.25"
	cfg.ActiveDeviceProfileID = wifi.ID
	cfg.DeviceProfiles = []types.DeviceProfile{wifi, rpm}
	cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
		types.DeviceTransportWiFi: wifi.ID,
	}
	cfg.FanCurve = curveprofiles.CloneCurve(wifiCurve)
	cfg.FanCurveProfiles = []types.FanCurveProfile{{
		ID:    "default",
		Name:  "WiFi",
		Curve: curveprofiles.CloneCurve(wifiCurve),
	}}
	cfg.ActiveFanCurveProfileID = "default"

	app := newDeviceProfileTestApp(t, cfg)

	if _, err := app.SetActiveDeviceProfile(rpm.ID); err != nil {
		t.Fatalf("switch to rpm profile: %v", err)
	}
	gotRPM := app.configManager.Get()
	if gotRPM.DeviceTransport != types.DeviceTransportSerial || types.DeviceProfileSpeedUnit(&gotRPM) != types.FanSpeedUnitRPM {
		t.Fatalf("active device after switch = %s/%s, want serial/rpm", gotRPM.DeviceTransport, types.DeviceProfileSpeedUnit(&gotRPM))
	}
	if reflect.DeepEqual(gotRPM.FanCurve, wifiCurve) {
		t.Fatal("rpm device reused the wifi percent curve")
	}
	if !reflect.DeepEqual(gotRPM.FanCurve, types.GetDefaultRPMFanCurve()) {
		t.Fatalf("rpm curve = %#v, want default rpm curve", gotRPM.FanCurve)
	}

	rpmCurve := offsetCurveSpeeds(types.GetDefaultRPMFanCurve(), -100)
	if err := app.SetFanCurve(rpmCurve); err != nil {
		t.Fatalf("save rpm curve: %v", err)
	}

	if _, err := app.SetActiveDeviceProfile(wifi.ID); err != nil {
		t.Fatalf("switch back to wifi profile: %v", err)
	}
	gotWiFi := app.configManager.Get()
	if !reflect.DeepEqual(gotWiFi.FanCurve, wifiCurve) {
		t.Fatalf("wifi curve = %#v, want original wifi curve %#v", gotWiFi.FanCurve, wifiCurve)
	}

	if _, err := app.SetActiveDeviceProfile(rpm.ID); err != nil {
		t.Fatalf("switch back to rpm profile: %v", err)
	}
	gotRPMAgain := app.configManager.Get()
	if !reflect.DeepEqual(gotRPMAgain.FanCurve, rpmCurve) {
		t.Fatalf("rpm curve after restore = %#v, want %#v", gotRPMAgain.FanCurve, rpmCurve)
	}
}

func TestNativeRuntimeCurveStateInheritsLegacyNativeCurve(t *testing.T) {
	bs1 := types.FlyDigiBS1Profile()
	defaultState := defaultDeviceFanCurveStateForUnit(types.FanSpeedUnitRPM)
	legacyCurve := offsetCurveSpeeds(types.GetDefaultRPMFanCurve(), -120)
	legacyState := types.DeviceFanCurveProfilesState{
		Profiles: []types.FanCurveProfile{{
			ID:    "custom-bs1",
			Name:  "BS1",
			Curve: curveprofiles.CloneCurve(legacyCurve),
		}},
		ActiveID: "custom-bs1",
		FanCurve: curveprofiles.CloneCurve(legacyCurve),
	}

	cfg := types.GetDefaultConfig(false)
	cfg.FanCurveProfilesByDevice = map[string]types.DeviceFanCurveProfilesState{
		deviceCurveScopeKeyForProfile(bs1):                                              defaultState,
		types.DeviceTransportBLE + deviceCurveScopeSeparator + types.LegacyRPMProfileID: legacyState,
	}

	changed := loadDeviceFanCurveStateForProfile(&cfg, bs1, types.FanSpeedUnitRPM, false)
	if !changed {
		t.Fatal("expected native runtime curve state to change")
	}
	if !reflect.DeepEqual(cfg.FanCurve, legacyCurve) {
		t.Fatalf("runtime curve = %#v, want legacy custom curve %#v", cfg.FanCurve, legacyCurve)
	}
	if got := cfg.FanCurveProfilesByDevice[deviceCurveScopeKeyForProfile(bs1)].FanCurve; !reflect.DeepEqual(got, legacyCurve) {
		t.Fatalf("stored bs1 curve = %#v, want %#v", got, legacyCurve)
	}
}

func TestNativeRuntimeStateInheritsLegacyRPMManualGear(t *testing.T) {
	bs3pro := types.FlyDigiBS3PROProfile()
	cfg := types.GetDefaultConfig(false)
	cfg.ManualGearRPM = types.CloneDefaultRPMManualGearRPM()
	cfg.ManualGearRPM["强劲"]["高"] = 4200

	changed := loadDeviceFanCurveStateForProfile(&cfg, bs3pro, types.FanSpeedUnitRPM, false)
	if !changed {
		t.Fatal("expected native runtime state to inherit legacy manual gear table")
	}
	if got := cfg.ManualGearRPM["强劲"]["高"]; got != 4200 {
		t.Fatalf("runtime manual gear 强劲/高 = %d, want 4200", got)
	}
	state := cfg.FanCurveProfilesByDevice[deviceCurveScopeKeyForProfile(bs3pro)]
	if got := state.ManualGearRPM["强劲"]["高"]; got != 4200 {
		t.Fatalf("stored bs3pro manual gear 强劲/高 = %d, want 4200", got)
	}
}

func TestLearningOffsetsAreScopedByDeviceAndCurveProfile(t *testing.T) {
	wifi := types.DefaultWiFiPercentProfile("10.0.0.25")
	rpm := testRPMDeviceProfile()
	wifiCurve := types.GetDefaultFanCurve()
	wifiOffsets := make([]int, len(wifiCurve))
	for i := range wifiOffsets {
		wifiOffsets[i] = i + 1
	}

	cfg := types.GetDefaultConfig(false)
	cfg.DeviceTransport = types.DeviceTransportWiFi
	cfg.FanControlDeviceIp = "10.0.0.25"
	cfg.ActiveDeviceProfileID = wifi.ID
	cfg.DeviceProfiles = []types.DeviceProfile{wifi, rpm}
	cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
		types.DeviceTransportWiFi: wifi.ID,
	}
	cfg.FanCurve = curveprofiles.CloneCurve(wifiCurve)
	cfg.FanCurveProfiles = []types.FanCurveProfile{{
		ID:    "default",
		Name:  "WiFi",
		Curve: curveprofiles.CloneCurve(wifiCurve),
	}}
	cfg.ActiveFanCurveProfileID = "default"
	cfg.SmartControl.LearnedOffsets = cloneIntSlice(wifiOffsets)

	app := newDeviceProfileTestApp(t, cfg)

	if _, err := app.SetActiveDeviceProfile(rpm.ID); err != nil {
		t.Fatalf("switch to rpm profile: %v", err)
	}
	gotRPM := app.configManager.Get()
	if len(gotRPM.SmartControl.LearnedOffsets) != len(gotRPM.FanCurve) {
		t.Fatalf("rpm offsets length = %d, want curve length %d", len(gotRPM.SmartControl.LearnedOffsets), len(gotRPM.FanCurve))
	}
	for _, value := range gotRPM.SmartControl.LearnedOffsets {
		if value != 0 {
			t.Fatalf("rpm offsets = %#v, want empty learning state for new device", gotRPM.SmartControl.LearnedOffsets)
		}
	}

	rpmOffsets := make([]int, len(gotRPM.FanCurve))
	for i := range rpmOffsets {
		rpmOffsets[i] = 100 + i
	}
	gotRPM.SmartControl.LearnedOffsets = cloneIntSlice(rpmOffsets)
	storeSmartControlOffsetsForActiveProfile(&gotRPM)
	app.configManager.Set(gotRPM)

	if _, err := app.SetActiveDeviceProfile(wifi.ID); err != nil {
		t.Fatalf("switch back to wifi profile: %v", err)
	}
	gotWiFi := app.configManager.Get()
	if !reflect.DeepEqual(gotWiFi.SmartControl.LearnedOffsets, wifiOffsets) {
		t.Fatalf("wifi offsets = %#v, want %#v", gotWiFi.SmartControl.LearnedOffsets, wifiOffsets)
	}

	if _, err := app.SetActiveDeviceProfile(rpm.ID); err != nil {
		t.Fatalf("switch back to rpm profile: %v", err)
	}
	gotRPMAgain := app.configManager.Get()
	if !reflect.DeepEqual(gotRPMAgain.SmartControl.LearnedOffsets, rpmOffsets) {
		t.Fatalf("rpm offsets = %#v, want %#v", gotRPMAgain.SmartControl.LearnedOffsets, rpmOffsets)
	}
}
