package plugins

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanPluginsDirReturnsValidPluginsAndReportsInvalidManifests(t *testing.T) {
	root := t.TempDir()
	createPluginManifest(t, root, "z-plugin", `{
		"id": "z-plugin",
		"name": "Z Plugin",
		"version": "1.0.0",
		"type": "background",
		"executable": "z.exe"
	}`)
	createPluginManifest(t, root, "alpha-device", `{
		"id": "alpha-device",
		"name": "Alpha Device",
		"version": "1.0.0",
		"type": "device",
		"executable": "bin/alpha-driver.exe"
	}`)
	createPluginManifest(t, root, "bad-plugin", `{
		"id": "BadPlugin",
		"name": "Bad Plugin",
		"version": "1.0.0",
		"type": "device",
		"executable": "bad.exe"
	}`)
	if err := os.WriteFile(filepath.Join(root, "not-a-plugin.txt"), []byte("ignored"), 0644); err != nil {
		t.Fatalf("write non-plugin file: %v", err)
	}

	discovered, err := ScanPluginsDir(root)
	if err == nil {
		t.Fatal("ScanPluginsDir returned nil error, want invalid manifest error")
	}
	if len(discovered) != 2 {
		t.Fatalf("len(discovered) = %d, want 2", len(discovered))
	}
	if discovered[0].Manifest.ID != "alpha-device" || discovered[1].Manifest.ID != "z-plugin" {
		t.Fatalf("plugins not sorted by id: %#v", discovered)
	}
	if discovered[0].ExecutablePath != filepath.Join(root, "alpha-device", "bin", "alpha-driver.exe") {
		t.Fatalf("ExecutablePath = %q", discovered[0].ExecutablePath)
	}
}

func TestScanPluginsDirMissingRootReturnsEmpty(t *testing.T) {
	discovered, err := ScanPluginsDir(filepath.Join(t.TempDir(), "plugins"))
	if err != nil {
		t.Fatalf("ScanPluginsDir returned error: %v", err)
	}
	if len(discovered) != 0 {
		t.Fatalf("len(discovered) = %d, want 0", len(discovered))
	}
}

func TestWatchPluginsDirDebouncesManifestChanges(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "alpha-device")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan struct{}, 1)
	changes := make(chan []DiscoveredPlugin, 4)
	errs := make(chan error, 4)
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := WatchPluginsDir(ctx, DiscoveryWatcherConfig{
			PluginsDir: root,
			Debounce:   25 * time.Millisecond,
			Ready:      ready,
			OnChange: func(discovered []DiscoveredPlugin) {
				changes <- discovered
			},
			OnError: func(err error) {
				errs <- err
			},
		})
		if err != nil {
			errs <- err
		}
	}()

	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not become ready")
	}

	writeManifest(t, pluginDir, `{
		"id": "alpha-device",
		"name": "Alpha Device",
		"version": "1.0.0",
		"type": "device",
		"executable": "alpha-driver.exe"
	}`)

	select {
	case err := <-errs:
		t.Fatalf("watcher returned error: %v", err)
	case discovered := <-changes:
		if len(discovered) != 1 {
			t.Fatalf("len(discovered) = %d, want 1", len(discovered))
		}
		if discovered[0].Manifest.ID != "alpha-device" {
			t.Fatalf("plugin id = %q, want alpha-device", discovered[0].Manifest.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for plugin discovery change")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop after cancel")
	}
}

func createPluginManifest(t *testing.T, root, dir, content string) {
	t.Helper()
	pluginDir := filepath.Join(root, dir)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	writeManifest(t, pluginDir, content)
}
