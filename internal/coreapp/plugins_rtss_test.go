package coreapp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/config"
	"github.com/Eureka-o/FanControlPortable/internal/plugins"
	"github.com/Eureka-o/FanControlPortable/internal/plugins/fnqpowermode"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func TestNormalizeUpdatedRTSSConfig(t *testing.T) {
	previous := types.RTSSConfig{Enabled: true, UpdateIntervalMS: 1000, PositionMode: types.RTSSPositionModeAnchor}
	tests := []struct {
		name string
		next types.RTSSConfig
		want types.RTSSConfig
	}{
		{
			name: "preserves config omitted by older client",
			next: types.RTSSConfig{},
			want: previous,
		},
		{
			name: "accepts supported update",
			next: types.RTSSConfig{Enabled: false, UpdateIntervalMS: 250},
			want: types.RTSSConfig{Enabled: false, UpdateIntervalMS: 250, PositionMode: types.RTSSPositionModeAnchor},
		},
		{
			name: "normalizes invalid update",
			next: types.RTSSConfig{Enabled: true, UpdateIntervalMS: 750},
			want: types.RTSSConfig{Enabled: true, UpdateIntervalMS: 1000, PositionMode: types.RTSSPositionModeAnchor},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeUpdatedRTSSConfig(test.next, previous); got != test.want {
				t.Fatalf("normalized config = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestSeedExternalRTSSSettingsMigratesLegacyConfig(t *testing.T) {
	installDir := t.TempDir()
	rtssExtDir := filepath.Join(installDir, "plugins", rtssExternalPluginID)
	if err := os.MkdirAll(filepath.Join(rtssExtDir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rtssExtDir, "bin", "rtss-osd.dll"), []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rtssExtDir, "plugin.json"), []byte(`{
		"id": "rtss-osd",
		"name": "RTSS OSD",
		"version": "1.0.0",
		"platform": "`+plugins.CurrentPlatform()+`",
		"protocolVersion": 1,
		"runtime": "inproc-dll",
		"backend": "bin/rtss-osd.dll"
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	pluginMgr := plugins.NewManager(nil)
	pluginMgr.SetPluginsDir(filepath.Join(installDir, "plugins"), "3.0.0")
	pluginMgr.SetConfigDir(installDir)
	pluginMgr.RefreshExternal()

	cfgMgr := config.NewManager(installDir, nil)
	cfgMgr.Load(false)
	app := &CoreApp{
		configManager: cfgMgr,
		pluginManager: pluginMgr,
	}

	cfg := cfgMgr.Get()
	cfg.ConfigPath = filepath.Join(installDir, "config.json")
	cfg.RTSS = types.RTSSConfig{
		Enabled:          true,
		UpdateIntervalMS: 500,
		PositionMode:     types.RTSSPositionModeCustom,
		PositionX:        32,
		PositionY:        -18,
	}
	app.seedExternalRTSSSettingsIfMissing(cfg)

	loaded, err := plugins.LoadPluginSettings(installDir, rtssExternalPluginID, nil)
	if err != nil {
		t.Fatalf("LoadPluginSettings failed: %v", err)
	}
	if loaded["updateIntervalMs"] != "500" || loaded["positionMode"] != "custom" {
		t.Fatalf("unexpected seeded settings: %#v", loaded)
	}
}

func TestCoreAppPluginsLifecycleAndDiscovery(t *testing.T) {
	installDir := t.TempDir()
	extDir := filepath.Join(installDir, "plugins", "demo-plugin")
	if err := os.MkdirAll(filepath.Join(extDir, "ui"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "ui", "index.js"), []byte("// demo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "plugin.json"), []byte(`{
		"id": "demo-plugin",
		"name": "Demo Plugin",
		"version": "1.0.0",
		"platform": "`+plugins.CurrentPlatform()+`",
		"protocolVersion": 1,
		"frontend": "ui/index.js"
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	pluginMgr := plugins.NewManager(nil)
	pluginMgr.SetPluginsDir(filepath.Join(installDir, "plugins"), "3.0.0")
	pluginMgr.RefreshExternal()

	cfgMgr := config.NewManager(installDir, nil)
	cfgMgr.Load(false)
	app := &CoreApp{
		configManager: cfgMgr,
		pluginManager: pluginMgr,
	}
	app.legionFnQSupportChecked.Store(true)
	app.legionFnQSupported.Store(false)

	list := app.GetPlugins()
	var foundRTSS, foundFnQ, foundExt *plugins.PluginInfo
	for i := range list {
		switch list[i].ID {
		case rtssExternalPluginID:
			foundRTSS = &list[i]
		case fnqpowermode.PluginID:
			foundFnQ = &list[i]
		case "demo-plugin":
			foundExt = &list[i]
		}
	}
	if foundRTSS != nil {
		t.Fatalf("expected built-in rtss-osd to be absent by default, got %+v", foundRTSS)
	}
	if foundFnQ == nil || foundFnQ.Supported || foundFnQ.State != plugins.StateUnsupported {
		t.Fatalf("unexpected legion fnq info: %+v", foundFnQ)
	}
	if foundExt == nil || foundExt.Kind != plugins.KindExternal || !foundExt.Supported {
		t.Fatalf("unexpected external plugin info: %+v", foundExt)
	}

	if _, err := app.SetPluginEnabled(fnqpowermode.PluginID, true); err == nil {
		t.Fatal("expected unsupported legion-fnq enable to fail")
	}
	if _, err := app.SetPluginEnabled("unknown-plugin", true); err == nil {
		t.Fatal("expected unknown plugin enable to fail")
	}

	// Verify plugin settings load/save & pending-restart flow on external inproc-dll.
	dllExtDir := filepath.Join(installDir, "plugins", "inproc-demo")
	if err := os.MkdirAll(filepath.Join(dllExtDir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dllExtDir, "bin", "demo.dll"), []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dllExtDir, "plugin.json"), []byte(`{
		"id": "inproc-demo",
		"name": "Inproc Demo",
		"version": "1.0.0",
		"platform": "`+plugins.CurrentPlatform()+`",
		"protocolVersion": 1,
		"runtime": "inproc-dll",
		"backend": "bin/demo.dll",
		"settings": [{"key": "rate", "title": "Rate", "type": "number", "default": 1000}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pluginMgr.SetConfigDir(installDir)
	pluginMgr.RefreshExternal()

	settings, err := app.GetPluginSettings("inproc-demo")
	if err != nil || settings["rate"] == nil {
		t.Fatalf("GetPluginSettings failed: %v, settings=%#v", err, settings)
	}
	if err := app.SavePluginSettings("inproc-demo", map[string]any{"rate": 500}); err != nil {
		t.Fatalf("SavePluginSettings failed: %v", err)
	}
	listAfterEnable, err := app.SetPluginEnabled("inproc-demo", true)
	if err != nil {
		t.Fatalf("SetPluginEnabled inproc-demo failed: %v", err)
	}
	var foundInproc *plugins.PluginInfo
	for i := range listAfterEnable {
		if listAfterEnable[i].ID == "inproc-demo" {
			foundInproc = &listAfterEnable[i]
			break
		}
	}
	if foundInproc == nil || !foundInproc.PendingRestart || foundInproc.State != plugins.StatePendingRestart {
		t.Fatalf("expected inproc-demo pending_restart state, got %+v", foundInproc)
	}
}
