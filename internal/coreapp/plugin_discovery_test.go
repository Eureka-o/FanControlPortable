package coreapp

import (
	"path/filepath"
	"testing"

	pluginpkg "github.com/TIANLI0/THRM/internal/plugins"
)

func TestAvailablePluginsSnapshotIsSortedAndCopied(t *testing.T) {
	app := &CoreApp{}
	app.updatePluginDiscoverySnapshot([]pluginpkg.DiscoveredPlugin{
		discoveredPluginForTest("z-plugin", "Z Plugin"),
		discoveredPluginForTest("alpha-device", "Alpha Device"),
	}, false)

	snapshot := app.availablePluginsSnapshot()
	if len(snapshot) != 2 {
		t.Fatalf("len(snapshot) = %d, want 2", len(snapshot))
	}
	if snapshot[0].ID != "alpha-device" || snapshot[1].ID != "z-plugin" {
		t.Fatalf("snapshot not sorted by id: %#v", snapshot)
	}

	snapshot[0].Name = "mutated"
	again := app.availablePluginsSnapshot()
	if again[0].Name != "Alpha Device" {
		t.Fatalf("snapshot mutated internal state: %#v", again[0])
	}
}

func TestPluginDiscoverySnapshotRemovesMissingPlugins(t *testing.T) {
	app := &CoreApp{}
	app.updatePluginDiscoverySnapshot([]pluginpkg.DiscoveredPlugin{
		discoveredPluginForTest("alpha-device", "Alpha Device"),
	}, false)
	app.updatePluginDiscoverySnapshot(nil, false)

	snapshot := app.availablePluginsSnapshot()
	if len(snapshot) != 0 {
		t.Fatalf("len(snapshot) = %d, want 0", len(snapshot))
	}
}

func TestPluginInfoFromDiscoveredPluginIncludesManifestFields(t *testing.T) {
	executablePath := filepath.Join(t.TempDir(), "driver.exe")
	app := &CoreApp{}
	app.updatePluginDiscoverySnapshot([]pluginpkg.DiscoveredPlugin{
		discoveredPluginForTestWithExecutablePath("sample-device", "Sample Device", executablePath),
	}, false)

	snapshot := app.availablePluginsSnapshot()
	if len(snapshot) != 1 {
		t.Fatalf("len(snapshot) = %d, want 1", len(snapshot))
	}
	if snapshot[0].ExePath != executablePath {
		t.Fatalf("snapshot[0].ExePath = %q, want %q", snapshot[0].ExePath, executablePath)
	}
	if snapshot[0].Frontend != "ui/index.html" {
		t.Fatalf("snapshot[0].Frontend = %q, want manifest frontend", snapshot[0].Frontend)
	}
}

func TestPluginDiscoveryDoesNotRegisterRuntimeFromManifest(t *testing.T) {
	app := &CoreApp{pluginManager: pluginpkg.NewManager(nil)}
	app.updatePluginDiscoverySnapshot([]pluginpkg.DiscoveredPlugin{
		discoveredPluginForTestWithExecutablePath("sample-device", "Sample Device", filepath.Join(t.TempDir(), "driver.exe")),
	}, false)

	if runtimePlugin := app.pluginManager.Plugin("sample-device"); runtimePlugin != nil {
		t.Fatalf("runtime plugin registered from manifest: %#v", runtimePlugin)
	}
}

func discoveredPluginForTest(id, name string) pluginpkg.DiscoveredPlugin {
	return discoveredPluginForTestWithExecutablePath(id, name, "")
}

func discoveredPluginForTestWithExecutablePath(id, name, executablePath string) pluginpkg.DiscoveredPlugin {
	return pluginpkg.DiscoveredPlugin{
		Manifest: pluginpkg.Manifest{
			ID:          id,
			Name:        name,
			Version:     "1.0.0",
			Type:        pluginpkg.PluginTypeDevice,
			Executable:  "driver.exe",
			Frontend:    "ui/index.html",
			Description: "sample plugin",
		},
		ExecutablePath: executablePath,
	}
}
