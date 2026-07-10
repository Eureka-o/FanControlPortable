package coreapp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/plugins"
	"github.com/TIANLI0/THRM/internal/types"
)

const pluginHandlerTestID = "sample-device"

func TestGetAvailablePluginsUsesInMemorySnapshot(t *testing.T) {
	app := newPluginHandlerTestApp(t)
	app.availablePlugins = map[string]types.PluginInfo{
		pluginHandlerTestID: {
			ID:        pluginHandlerTestID,
			Name:      "Sample Device",
			Status:    "discovered",
			Installed: true,
			Frontend:  "ui/index.html",
		},
	}

	resp, ok := app.handlePluginIPCRequest(ipc.Request{Type: ipc.ReqGetAvailablePlugins})
	if !ok || !resp.Success {
		t.Fatalf("response = %#v ok=%v, want success", resp, ok)
	}
	var got []types.PluginInfo
	decodeResponseData(t, resp, &got)
	if len(got) != 1 || got[0].ID != pluginHandlerTestID {
		t.Fatalf("plugins = %#v, want in-memory plugin snapshot", got)
	}
	if got[0].Frontend != "ui/index.html" {
		t.Fatalf("frontend = %q, want manifest frontend", got[0].Frontend)
	}
}

func TestGetPluginStatusMissingReturnsError(t *testing.T) {
	app := newPluginHandlerTestApp(t)

	resp, ok := app.handlePluginIPCRequest(newJSONRequest(t, ipc.ReqGetPluginStatus, map[string]string{"id": "missing-plugin"}))
	if !ok {
		t.Fatal("request was not handled")
	}
	if resp.Success {
		t.Fatalf("response = %#v, want error for missing plugin", resp)
	}
	if !strings.Contains(resp.Error, "missing-plugin") {
		t.Fatalf("error = %q, want missing plugin id", resp.Error)
	}
}

func TestEnableDisableManifestPluginRequiresRuntimePlugin(t *testing.T) {
	app := newPluginHandlerTestApp(t)
	app.availablePlugins = map[string]types.PluginInfo{
		pluginHandlerTestID: {
			ID:        pluginHandlerTestID,
			Name:      "Sample Device",
			Status:    "discovered",
			Installed: true,
		},
	}

	enableResp, ok := app.handlePluginIPCRequest(newJSONRequest(t, ipc.ReqEnablePlugin, map[string]string{"id": pluginHandlerTestID}))
	if !ok {
		t.Fatal("enable request was not handled")
	}
	if enableResp.Success || !strings.Contains(enableResp.Error, "runtime plugin is not registered") {
		t.Fatalf("enable response = %#v, want clear missing runtime plugin error", enableResp)
	}

	disableResp, ok := app.handlePluginIPCRequest(newJSONRequest(t, ipc.ReqDisablePlugin, map[string]string{"pluginId": pluginHandlerTestID}))
	if !ok {
		t.Fatal("disable request was not handled")
	}
	if disableResp.Success || !strings.Contains(disableResp.Error, "runtime plugin is not registered") {
		t.Fatalf("disable response = %#v, want clear missing runtime plugin error", disableResp)
	}
}

func TestEnableDisableRegisteredRuntimePluginUsesGenericManager(t *testing.T) {
	app := newPluginHandlerTestApp(t)
	runtimePlugin := &pluginHandlerRuntimePlugin{id: pluginHandlerTestID, name: "Sample Device"}
	app.pluginManager.Register(runtimePlugin)

	enableResp, ok := app.handlePluginIPCRequest(newJSONRequest(t, ipc.ReqEnablePlugin, map[string]string{"id": pluginHandlerTestID}))
	if !ok || !enableResp.Success {
		t.Fatalf("enable response = %#v ok=%v, want success", enableResp, ok)
	}
	if runtimePlugin.starts != 1 || runtimePlugin.stops != 0 || !runtimePlugin.running {
		t.Fatalf("runtime lifecycle after enable = starts %d stops %d running %v", runtimePlugin.starts, runtimePlugin.stops, runtimePlugin.running)
	}

	disableResp, ok := app.handlePluginIPCRequest(newJSONRequest(t, ipc.ReqDisablePlugin, map[string]string{"pluginId": pluginHandlerTestID}))
	if !ok || !disableResp.Success {
		t.Fatalf("disable response = %#v ok=%v, want success", disableResp, ok)
	}
	if runtimePlugin.starts != 1 || runtimePlugin.stops != 1 || runtimePlugin.running {
		t.Fatalf("runtime lifecycle after disable = starts %d stops %d running %v", runtimePlugin.starts, runtimePlugin.stops, runtimePlugin.running)
	}
}

func TestGetPluginStatusIncludesRuntimeState(t *testing.T) {
	app := newPluginHandlerTestApp(t)
	runtimePlugin := &pluginHandlerRuntimePlugin{id: pluginHandlerTestID, name: "Sample Device", running: true}
	app.pluginManager.Register(runtimePlugin)

	resp, ok := app.handlePluginIPCRequest(newJSONRequest(t, ipc.ReqGetPluginStatus, map[string]string{"id": pluginHandlerTestID}))
	if !ok || !resp.Success {
		t.Fatalf("response = %#v ok=%v, want success", resp, ok)
	}
	var got types.PluginInfo
	decodeResponseData(t, resp, &got)
	if got.ID != pluginHandlerTestID || !got.Running || got.Status != "running" {
		t.Fatalf("plugin info = %#v, want runtime status", got)
	}
}

func newPluginHandlerTestApp(t *testing.T) *CoreApp {
	t.Helper()
	cfgManager := config.NewManager(t.TempDir(), nil)
	cfg := types.GetDefaultConfig(false)
	cfg.ConfigPath = filepath.Join(t.TempDir(), "config", "config.json")
	cfgManager.Set(cfg)
	return &CoreApp{
		configManager:    cfgManager,
		pluginManager:    plugins.NewManager(nil),
		availablePlugins: map[string]types.PluginInfo{},
	}
}

func newJSONRequest(t *testing.T, requestType ipc.RequestType, data any) ipc.Request {
	t.Helper()
	dataBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Marshal(%#v) failed: %v", data, err)
	}
	return ipc.Request{Type: requestType, Data: dataBytes}
}

func decodeResponseData(t *testing.T, resp ipc.Response, target any) {
	t.Helper()
	if err := json.Unmarshal(resp.Data, target); err != nil {
		t.Fatalf("Unmarshal response data failed: %v; raw=%s", err, string(resp.Data))
	}
}

type pluginHandlerRuntimePlugin struct {
	id      string
	name    string
	starts  int
	stops   int
	running bool
}

func (p *pluginHandlerRuntimePlugin) ID() string { return p.id }

func (p *pluginHandlerRuntimePlugin) Name() string {
	if p.name != "" {
		return p.name
	}
	return p.id
}

func (p *pluginHandlerRuntimePlugin) Start(ctx context.Context) error {
	p.starts++
	p.running = true
	return nil
}

func (p *pluginHandlerRuntimePlugin) Stop() error {
	p.stops++
	p.running = false
	return nil
}

func (p *pluginHandlerRuntimePlugin) Status() plugins.Status {
	return plugins.Status{ID: p.id, Name: p.Name(), Running: p.running}
}
