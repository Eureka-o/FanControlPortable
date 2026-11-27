//go:build windows

package marketplace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/plugins"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

type noopLogger struct{}

func (noopLogger) Debug(string, ...any) {}
func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}
func (noopLogger) Close()               {}
func (noopLogger) CleanOldLogs()        {}
func (noopLogger) SetDebugMode(bool)    {}
func (noopLogger) GetLogDir() string    { return "" }

type dummyBuiltinRTSS struct{}

func (dummyBuiltinRTSS) ID() string                     { return "rtss-osd" }
func (dummyBuiltinRTSS) Name() string                   { return "RTSS OSD" }
func (dummyBuiltinRTSS) Start(context.Context) error    { return nil }
func (dummyBuiltinRTSS) Stop() error                    { return nil }
func (dummyBuiltinRTSS) Status() plugins.Status         { return plugins.Status{ID: "rtss-osd", Name: "RTSS OSD"} }

func TestRealMarketplacePackagesEndToEnd(t *testing.T) {
	catalogPath := filepath.Clean(filepath.Join("..", "..", "..", "FanControl-Plugins", "dist", "catalog.local.json"))
	if _, err := os.Stat(catalogPath); err != nil {
		t.Skipf("local marketplace catalog not built yet (%s): %v", catalogPath, err)
	}

	view, err := FetchCatalog(
		context.Background(),
		catalogPath,
		"windows-amd64",
		"3.0.0",
		InstalledPluginMap{},
		map[string]InstalledThemeMeta{},
		nil,
	)
	if err != nil {
		t.Fatalf("FetchCatalog failed: %v", err)
	}
	if len(view.Plugins) < 2 {
		t.Fatalf("expected at least 2 catalog plugins, got %d", len(view.Plugins))
	}

	var rtssEntry, omenEntry *CatalogPluginView
	for i := range view.Plugins {
		switch view.Plugins[i].ID {
		case "rtss-osd":
			rtssEntry = &view.Plugins[i]
		case "omen-fan":
			omenEntry = &view.Plugins[i]
		}
	}
	if rtssEntry == nil || omenEntry == nil {
		t.Fatalf("missing expected plugins in catalog: %+v", view.Plugins)
	}
	if !rtssEntry.Compatible || !omenEntry.Compatible {
		t.Fatalf("expected both rtss-osd and omen-fan compatible, got rtss=%+v omen=%+v", rtssEntry, omenEntry)
	}

	root := t.TempDir()
	pluginsDir := filepath.Join(root, "plugins")
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 1. Download, verify SHA256, stage rtss-osd (inproc-dll), and apply on startup.
	rtssZip, _, cleanRtss, err := DownloadAndVerify(context.Background(), rtssEntry.DownloadURL, rtssEntry.SHA256, nil, nil)
	if err != nil {
		t.Fatalf("DownloadAndVerify rtss-osd failed: %v", err)
	}
	defer cleanRtss()

	rtssRes, err := InstallPluginFromZip(rtssZip, pluginsDir, "rtss-osd", "windows-amd64", "3.0.0", true)
	if err != nil {
		t.Fatalf("InstallPluginFromZip rtss-osd failed: %v", err)
	}
	if !rtssRes.RestartRequired {
		t.Fatalf("expected RestartRequired=true for inproc-dll install")
	}

	// 2. Download, verify SHA256, and hot-install omen-fan (sidecar).
	omenZip, _, cleanOmen, err := DownloadAndVerify(context.Background(), omenEntry.DownloadURL, omenEntry.SHA256, nil, nil)
	if err != nil {
		t.Fatalf("DownloadAndVerify omen-fan failed: %v", err)
	}
	defer cleanOmen()

	omenRes, err := InstallPluginFromZip(omenZip, pluginsDir, "omen-fan", "windows-amd64", "3.0.0", false)
	if err != nil {
		t.Fatalf("InstallPluginFromZip omen-fan failed: %v", err)
	}
	if omenRes.RestartRequired {
		t.Fatalf("expected RestartRequired=false for sidecar install")
	}

	// 3. Apply pending staging (as Core startup does), register built-in fallback, verify external shadowing.
	if err := plugins.ApplyPendingStagedOps(pluginsDir, configDir, noopLogger{}); err != nil {
		t.Fatalf("ApplyPendingStagedOps failed: %v", err)
	}

	mgr := plugins.NewManager(noopLogger{})
	t.Cleanup(func() {
		mgr.ReleaseExternalForTest()
	})
	mgr.SetPluginsDir(pluginsDir, "3.0.0")
	mgr.SetConfigDir(configDir)
	mgr.Register(dummyBuiltinRTSS{})
	mgr.SyncStartupExternal(map[string]bool{
		"rtss-osd": true,
		"omen-fan": true,
	})

	if !mgr.HasExternal("rtss-osd") {
		t.Fatalf("expected HasExternal(rtss-osd)=true")
	}
	snap := mgr.Snapshot(map[string]bool{"rtss-osd": true, "omen-fan": true})
	rtssCount := 0
	for _, item := range snap {
		if item.ID == "rtss-osd" {
			rtssCount++
			if item.Kind != plugins.KindExternal || !item.Running {
				t.Fatalf("expected running external rtss-osd shadowing builtin, got %+v", item)
			}
		}
	}
	if rtssCount != 1 {
		t.Fatalf("expected 1 shadowed rtss-osd row, got %d (%+v)", rtssCount, snap)
	}

	// 4. Test real rtss-osd DLL invoke (get-status, settings-changed, get-layout-status, create-anchor, fan-data).
	fakeRTSS := filepath.Join(root, "fake-rtss")
	overlayDir := filepath.Join(fakeRTSS, "Plugins", "Client", "Overlays")
	if err := os.MkdirAll(overlayDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgIni := filepath.Join(fakeRTSS, "Plugins", "Client", "OverlayEditor.cfg")
	if err := os.WriteFile(cfgIni, []byte("[Settings]\r\nLayout=default.ovl\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ovlPath := filepath.Join(overlayDir, "default.ovl")
	if err := os.WriteFile(ovlPath, []byte("[General]\r\nLayers=1\r\n[Layer0]\r\nName=CPU Temp\r\nText=%CPU%\r\nPositionX=0\r\nPositionY=-2\r\nSize=100\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RTSS_INSTALL_PATH", fakeRTSS)

	statusRaw, err := mgr.Invoke(context.Background(), "rtss-osd", "get-status", nil)
	if err != nil {
		t.Fatalf("rtss-osd get-status failed: %v", err)
	}
	if !strings.Contains(string(statusRaw), `"ok":true`) {
		t.Fatalf("unexpected rtss-osd status: %s", string(statusRaw))
	}

	mgr.NotifySettingsChanged("rtss-osd", map[string]any{
		"osdActive":        true,
		"updateIntervalMs": "500",
		"positionMode":     "custom",
		"positionX":        12,
		"positionY":        -8,
	})

	anchorRaw, err := mgr.Invoke(context.Background(), "rtss-osd", "create-anchor", nil)
	if err != nil {
		t.Fatalf("rtss-osd create-anchor failed: %v", err)
	}
	var anchorResp struct {
		OK          bool   `json:"ok"`
		AnchorState string `json:"anchorState"`
		BackupPath  string `json:"backupPath"`
	}
	if err := json.Unmarshal(anchorRaw, &anchorResp); err != nil {
		t.Fatalf("decode create-anchor failed: %v (%s)", err, string(anchorRaw))
	}
	if !anchorResp.OK || anchorResp.AnchorState != "confirmed" || anchorResp.BackupPath == "" {
		t.Fatalf("unexpected create-anchor result: %+v", anchorResp)
	}
	if _, err := os.Stat(anchorResp.BackupPath); err != nil {
		t.Fatalf("expected backup file at %s: %v", anchorResp.BackupPath, err)
	}
	mgr.PublishFanData(&types.FanData{CurrentRPM: 3200, SpeedUnit: types.FanSpeedUnitRPM})

	// 5. Wait for real omen-fan sidecar Ready and test system-telemetry + RPC + suspend/resume.
	deadline := time.Now().Add(5 * time.Second)
	omenReady := false
	for time.Now().Before(deadline) {
		for _, item := range mgr.Snapshot(map[string]bool{"rtss-osd": true, "omen-fan": true}) {
			if item.ID == "omen-fan" && item.State == plugins.StateReady && item.Running {
				omenReady = true
				break
			}
		}
		if omenReady {
			break
		}
		time.Sleep(40 * time.Millisecond)
	}
	if !omenReady {
		t.Fatalf("omen-fan sidecar did not become Ready in time (logs=%v)", mgr.Logs("omen-fan"))
	}

	mgr.PublishSystemTelemetry(&types.TemperatureData{
		CPUTemp:       71,
		GPUTemp:       64,
		CPUPowerWatts: 41.5,
		GPUPowerWatts: 82.0,
		CpuModel:      "Test CPU",
		GpuModel:      "Test GPU",
		BridgeOk:      true,
	})
	time.Sleep(80 * time.Millisecond)

	omenManualRaw, err := mgr.Invoke(context.Background(), "omen-fan", "set-manual-fan", json.RawMessage(`{"cpuRpm":3800,"gpuRpm":3800,"linked":true}`))
	if err != nil {
		t.Fatalf("omen-fan set-manual-fan failed: %v", err)
	}
	if !strings.Contains(string(omenManualRaw), `"cpuRpm":3800`) {
		t.Fatalf("unexpected omen-fan manual response: %s", string(omenManualRaw))
	}

	omenAutoRaw, err := mgr.Invoke(context.Background(), "omen-fan", "restore-auto", nil)
	if err != nil {
		t.Fatalf("omen-fan restore-auto failed: %v", err)
	}
	if !strings.Contains(string(omenAutoRaw), `"controlMode":"bios-auto"`) {
		t.Fatalf("unexpected omen-fan restore-auto response: %s", string(omenAutoRaw))
	}

	// Test clean unload & pending uninstall for inproc-dll + hot uninstall for sidecar.
	if _, err := mgr.SetExternalEnabled("omen-fan", false); err != nil {
		t.Fatalf("disable omen-fan failed: %v", err)
	}
	if res, err := UninstallPlugin(pluginsDir, configDir, "omen-fan", false, true); err != nil || res == nil || res.RestartRequired {
		t.Fatalf("expected clean hot uninstall of omen-fan, got res=%+v err=%v", res, err)
	}
	if res, err := UninstallPlugin(pluginsDir, configDir, "rtss-osd", true, false); err != nil || res == nil || !res.RestartRequired {
		t.Fatalf("expected staged uninstall of rtss-osd, got res=%+v err=%v", res, err)
	}
	mgr.ReleaseExternalForTest()
	if err := plugins.ApplyPendingStagedOps(pluginsDir, configDir, noopLogger{}); err != nil {
		t.Fatalf("ApplyPendingStagedOps after uninstall failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, "rtss-osd")); !os.IsNotExist(err) {
		t.Fatalf("expected rtss-osd directory removed after staged delete")
	}

	// 6. Download, verify SHA256, and install every marketplace theme (including nested asset directories like miku-01).
	if len(view.Themes) < 9 {
		t.Fatalf("expected at least 9 catalog themes, got %d", len(view.Themes))
	}
	themesDir := filepath.Join(root, "themes")
	for _, themeItem := range view.Themes {
		themeZip, _, cleanTheme, err := DownloadAndVerify(context.Background(), themeItem.DownloadURL, themeItem.SHA256, nil, nil)
		if err != nil {
			t.Fatalf("DownloadAndVerify theme %s failed: %v", themeItem.ID, err)
		}
		themeRes, instErr := InstallThemeFromZip(themeZip, themesDir, themeItem.ID)
		cleanTheme()
		if instErr != nil {
			t.Fatalf("InstallThemeFromZip theme %s failed: %v", themeItem.ID, instErr)
		}
		if themeRes.ID != themeItem.ID {
			t.Fatalf("installed theme ID = %q, want %q", themeRes.ID, themeItem.ID)
		}
		for _, requiredFile := range []string{"theme.json", "theme.css"} {
			if _, err := os.Stat(filepath.Join(themesDir, themeItem.ID, requiredFile)); err != nil {
				t.Fatalf("extracted %s missing for theme %s: %v", requiredFile, themeItem.ID, err)
			}
		}
	}
}
