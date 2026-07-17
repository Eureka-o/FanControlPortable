package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeTestPlugin(t *testing.T, root, id string, mutate func(*Manifest)) {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(filepath.Join(dir, "backend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "ui"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(dir, "backend", "driver.exe"),
		filepath.Join(dir, "ui", "index.js"),
		filepath.Join(dir, "ui", "index.css"),
		filepath.Join(dir, "ui", "icon.png"),
	} {
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := Manifest{
		ID:              id,
		Name:            "Test Plugin",
		Version:         "1.0.0",
		Platform:        runtime.GOOS + "-" + runtime.GOARCH,
		MinCoreVersion:  "2.5.0",
		ProtocolVersion: ProtocolVersionV1,
		Backend:         "backend/driver.exe",
		Frontend:        "ui/index.js",
		Style:           "ui/index.css",
		HostAPIVersion:  HostAPIVersionV1,
		Capabilities:    []string{"status"},
		Page:            PageManifest{ID: "control", Title: "Test", Icon: "plug", IconAsset: "ui/icon.png", Order: 500},
	}
	if mutate != nil {
		mutate(&manifest)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogDiscoversValidPlugin(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "test-plugin", nil)

	snapshot := NewCatalog(root, "2.5.3").Refresh()
	if snapshot.Revision != 1 || len(snapshot.Plugins) != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	plugin := snapshot.Plugins[0]
	if plugin.State != CatalogStateDisabled || plugin.ID != "test-plugin" || plugin.Page.ID != "control" {
		t.Fatalf("plugin = %#v", plugin)
	}
	if plugin.Frontend != "ui/index.js" || plugin.Style != "ui/index.css" || plugin.Page.IconAsset != "ui/icon.png" {
		t.Fatalf("plugin assets = frontend %q, style %q, icon %q", plugin.Frontend, plugin.Style, plugin.Page.IconAsset)
	}
}

func TestCatalogReadsAllowedPluginAsset(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "test-plugin", nil)
	catalog := NewCatalog(root, "2.5.3")
	catalog.Refresh()

	asset, err := catalog.ReadAsset("test-plugin", "ui/index.js")
	if err != nil {
		t.Fatal(err)
	}
	if string(asset.Data) != "test" || asset.ContentType != "text/javascript; charset=utf-8" || asset.Version != "1.0.0" {
		t.Fatalf("asset = %#v", asset)
	}
}

func TestCatalogRejectsUnsafePluginAssets(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "test-plugin", nil)
	catalog := NewCatalog(root, "2.5.3")
	catalog.Refresh()

	for _, path := range []string{"../plugin.json", "backend/driver.exe", "plugin.json.exe"} {
		if _, err := catalog.ReadAsset("test-plugin", path); err == nil {
			t.Fatalf("ReadAsset(%q) succeeded", path)
		}
	}
}

func TestCatalogRejectsSymlinkedPluginAsset(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "test-plugin", nil)
	outside := filepath.Join(root, "outside.js")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "test-plugin", "ui", "linked.js")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	catalog := NewCatalog(root, "2.5.3")
	catalog.Refresh()

	if _, err := catalog.ReadAsset("test-plugin", "ui/linked.js"); err == nil {
		t.Fatal("ReadAsset succeeded for symlink")
	}
}

func TestCatalogExposesValidatedRuntimeSpec(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "test-plugin", nil)
	catalog := NewCatalog(root, "2.5.3")
	catalog.Refresh()

	specs := catalog.RuntimeSpecs()
	if len(specs) != 1 {
		t.Fatalf("runtime specs = %#v", specs)
	}
	if specs[0].ID != "test-plugin" || specs[0].BackendPath != filepath.Join(root, "test-plugin", "backend", "driver.exe") {
		t.Fatalf("runtime spec = %#v", specs[0])
	}
}

func TestCatalogNormalizesManifestAssetPaths(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "test-plugin", func(manifest *Manifest) {
		manifest.Backend = " backend/driver.exe "
		manifest.Frontend = " ui/index.js "
		manifest.Style = " ui/index.css "
	})
	catalog := NewCatalog(root, "2.5.3")
	snapshot := catalog.Refresh()

	plugin := snapshot.Plugins[0]
	if plugin.State != CatalogStateDisabled || plugin.Frontend != "ui/index.js" || plugin.Style != "ui/index.css" {
		t.Fatalf("plugin = %#v", plugin)
	}
	if got := catalog.RuntimeSpecs()[0].BackendPath; got != filepath.Join(root, "test-plugin", "backend", "driver.exe") {
		t.Fatalf("backend path = %q", got)
	}
}

func TestCatalogRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "test-plugin", func(manifest *Manifest) {
		manifest.Backend = "../outside.exe"
	})

	plugin := NewCatalog(root, "2.5.3").Refresh().Plugins[0]
	if plugin.State != CatalogStateInvalid {
		t.Fatalf("plugin state = %q, want invalid", plugin.State)
	}
}

func TestCatalogRejectsNonJavaScriptFrontend(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "test-plugin", func(manifest *Manifest) {
		manifest.Frontend = "ui/index.css"
	})

	plugin := NewCatalog(root, "2.5.3").Refresh().Plugins[0]
	if plugin.State != CatalogStateInvalid {
		t.Fatalf("plugin state = %q, want invalid", plugin.State)
	}
}

func TestCatalogRejectsExecutablePageIconAsset(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "test-plugin", func(manifest *Manifest) {
		manifest.Page.IconAsset = "backend/driver.exe"
	})

	plugin := NewCatalog(root, "2.5.3").Refresh().Plugins[0]
	if plugin.State != CatalogStateInvalid {
		t.Fatalf("plugin state = %q, want invalid", plugin.State)
	}
}

func TestCatalogReportsIncompatiblePlatform(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "test-plugin", func(manifest *Manifest) {
		manifest.Platform = "unsupported-arch"
	})

	plugin := NewCatalog(root, "2.5.3").Refresh().Plugins[0]
	if plugin.State != CatalogStateIncompatible {
		t.Fatalf("plugin state = %q, want incompatible", plugin.State)
	}
}

func TestCatalogRevisionChangesOnlyWhenSnapshotChanges(t *testing.T) {
	root := t.TempDir()
	catalog := NewCatalog(root, "2.5.3")
	first := catalog.Refresh()
	second := catalog.Refresh()
	if first.Revision != 1 || second.Revision != first.Revision {
		t.Fatalf("stable revisions = %d, %d", first.Revision, second.Revision)
	}

	writeTestPlugin(t, root, "test-plugin", nil)
	third := catalog.Refresh()
	if third.Revision != second.Revision+1 {
		t.Fatalf("changed revision = %d, want %d", third.Revision, second.Revision+1)
	}
}

func TestCatalogReturnsInvalidEntryForMissingManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "broken-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}

	plugin := NewCatalog(root, "2.5.3").Refresh().Plugins[0]
	if plugin.State != CatalogStateInvalid || plugin.ID != "broken-plugin" {
		t.Fatalf("plugin = %#v", plugin)
	}
}

func TestCatalogCanDeletePluginWithMismatchedManifestID(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "broken-plugin", func(manifest *Manifest) {
		manifest.ID = "different-plugin"
	})
	catalog := NewCatalog(root, "2.5.3")

	plugin := catalog.Refresh().Plugins[0]
	if plugin.State != CatalogStateInvalid || plugin.ID != "broken-plugin" {
		t.Fatalf("plugin = %#v", plugin)
	}
	if _, err := catalog.Delete(plugin.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "broken-plugin")); !os.IsNotExist(err) {
		t.Fatalf("plugin directory still exists: %v", err)
	}
}

func TestCatalogEnabledStateChangesRevision(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "test-plugin", nil)
	catalog := NewCatalog(root, "2.5.3")
	first := catalog.Refresh()

	second, err := catalog.SetEnabled("test-plugin", true)
	if err != nil {
		t.Fatal(err)
	}
	if second.Revision != first.Revision+1 || !second.Plugins[0].Enabled {
		t.Fatalf("enabled snapshot = %#v", second)
	}
	if second.Plugins[0].State != CatalogStateDiscovered {
		t.Fatalf("enabled state = %q", second.Plugins[0].State)
	}

	third, changed := catalog.SetRuntimeState("test-plugin", CatalogStateReady, "", []string{"status"})
	if !changed || third.Revision != second.Revision+1 || third.Plugins[0].State != CatalogStateReady {
		t.Fatalf("runtime snapshot = %#v, changed=%v", third, changed)
	}
}

func TestCatalogDeleteRequiresDisabledPlugin(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "test-plugin", nil)
	catalog := NewCatalog(root, "2.5.3")
	catalog.Refresh()
	if _, err := catalog.SetEnabled("test-plugin", true); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Delete("test-plugin"); err == nil {
		t.Fatal("Delete succeeded for enabled plugin")
	}
	if _, err := catalog.SetEnabled("test-plugin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Delete("test-plugin"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "test-plugin")); !os.IsNotExist(err) {
		t.Fatalf("plugin directory still exists: %v", err)
	}
}

func TestResetPluginDataOnlyRemovesRequestedDirectory(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"first", "second"} {
		if err := os.MkdirAll(filepath.Join(root, id), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := ResetPluginData(root, "first"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "first")); !os.IsNotExist(err) {
		t.Fatalf("first directory still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "second")); err != nil {
		t.Fatalf("second directory was removed: %v", err)
	}
}
