package coreapp

import (
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

// 监控/设备上报热路径的 revision 缓存快照：未变时复用缓存、变化后失效重建，
// 且读到的标志位必须与 configManager.Get() 的新克隆一致。
func TestMonitoringConfigSnapshotMatchesFreshGetAndReusesCache(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.AutoControl = true
	cfg.BlackSharkHostVariableSpeed = true
	app := newDeviceProfileTestApp(t, cfg)

	snapshot := app.monitoringConfigSnapshot()
	fresh := app.configManager.Get()
	if snapshot.AutoControl != fresh.AutoControl ||
		snapshot.CustomSpeedEnabled != fresh.CustomSpeedEnabled ||
		snapshot.BlackSharkHostVariableSpeed != fresh.BlackSharkHostVariableSpeed {
		t.Fatalf("cached snapshot flags = (auto %v, custom %v, hostSpeed %v), fresh Get = (auto %v, custom %v, hostSpeed %v)",
			snapshot.AutoControl, snapshot.CustomSpeedEnabled, snapshot.BlackSharkHostVariableSpeed,
			fresh.AutoControl, fresh.CustomSpeedEnabled, fresh.BlackSharkHostVariableSpeed)
	}
	if got, want := types.ActiveDeviceProfile(&snapshot).ID, types.ActiveDeviceProfile(&fresh).ID; got != want {
		t.Fatalf("cached active profile = %q, fresh Get profile = %q", got, want)
	}

	// revision 未变：复用同一份缓存（DeviceProfiles 底层数组不变，未重新深拷贝）。
	reused := app.monitoringConfigSnapshot()
	if &reused.DeviceProfiles[0] != &snapshot.DeviceProfiles[0] {
		t.Fatal("unchanged revision must reuse the cached snapshot instead of re-cloning")
	}

	// revision 变化（Set 总是递增）：缓存必须失效并反映新值。
	updated := fresh
	updated.AutoControl = false
	updated.CustomSpeedEnabled = true
	updated.BlackSharkHostVariableSpeed = false
	app.configManager.Set(updated)

	refreshed := app.monitoringConfigSnapshot()
	if refreshed.AutoControl || !refreshed.CustomSpeedEnabled || refreshed.BlackSharkHostVariableSpeed {
		t.Fatalf("snapshot after revision bump = (auto %v, custom %v, hostSpeed %v), want updated flags",
			refreshed.AutoControl, refreshed.CustomSpeedEnabled, refreshed.BlackSharkHostVariableSpeed)
	}
	if &refreshed.DeviceProfiles[0] == &snapshot.DeviceProfiles[0] {
		t.Fatal("revision bump must replace the cached snapshot backing store")
	}
}

func TestMonitoringConfigSnapshotWithoutConfigManager(t *testing.T) {
	app := &CoreApp{}
	if got := app.monitoringConfigSnapshot(); got.AutoControl || got.CustomSpeedEnabled {
		t.Fatalf("nil config manager snapshot = %#v, want zero config", got)
	}
}

// deviceRuntimeSnapshot 的配置档案兜底必须走 revision 缓存：revision 变化后
// 下一拍就能看到新档案（未连接时 profile 完全来自配置）。
func TestDeviceRuntimeSnapshotProfileTracksConfigRevisionWhenDisconnected(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.WiFiCompatibilityEnabled = true
	cfg.FanControlDeviceIp = "10.0.0.25"
	app := newDeviceProfileTestApp(t, cfg)

	snapshot := app.deviceRuntimeSnapshot()
	fresh := app.configManager.Get()
	if got, want := snapshot.Profile.ID, types.ActiveDeviceProfile(&fresh).ID; got != want {
		t.Fatalf("snapshot profile = %q, want configured active profile %q", got, want)
	}
	if snapshot.Profile.ID != types.DefaultWiFiPercentProfileID {
		t.Fatalf("snapshot profile = %q, want default WiFi profile %q", snapshot.Profile.ID, types.DefaultWiFiPercentProfileID)
	}

	updated := fresh
	updated.WiFiCompatibilityEnabled = false
	updated.SerialCompatibilityEnabled = false
	app.configManager.Set(updated)

	if got := app.deviceRuntimeSnapshot().Profile.ID; got != "" {
		t.Fatalf("snapshot profile after compatibility disabled = %q, want empty", got)
	}
}

// 两个热路径布尔判据（autoControlActive / blackSharkHostControlsSpeed）必须
// 与配置 revision 保持一致：Set 递增 revision 后立刻反映新值。
func TestHotPathFlagHelpersTrackConfigRevision(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.AutoControl = true
	cfg.CustomSpeedEnabled = false
	cfg.BlackSharkHostVariableSpeed = false
	app := newDeviceProfileTestApp(t, cfg)

	if !app.autoControlActive() {
		t.Fatal("autoControlActive should be true with AutoControl on and custom speed off")
	}
	if app.blackSharkHostControlsSpeed() {
		t.Fatal("blackSharkHostControlsSpeed should follow the config flag")
	}

	updated := app.configManager.Get()
	updated.CustomSpeedEnabled = true
	updated.BlackSharkHostVariableSpeed = true
	app.configManager.Set(updated)

	if app.autoControlActive() {
		t.Fatal("custom speed must disable the auto-control verdict after a revision bump")
	}
	if !app.blackSharkHostControlsSpeed() {
		t.Fatal("host-speed flag must follow the revision bump")
	}
}

func TestHotPathFlagHelpersWithoutConfigManager(t *testing.T) {
	app := &CoreApp{}
	if app.autoControlActive() || app.blackSharkHostControlsSpeed() {
		t.Fatal("nil config manager must yield false verdicts")
	}
}
