package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverExternalValidatesManifestAndCompatibility(t *testing.T) {
	dir := t.TempDir()

	validDir := filepath.Join(dir, "sample-ext")
	if err := os.MkdirAll(filepath.Join(validDir, "ui"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(validDir, "ui", "index.js"), []byte("// ok"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(validDir, "plugin.json"), []byte(`{
		"id": "sample-ext",
		"name": "Sample Extension",
		"version": "1.2.0",
		"platform": "windows-amd64",
		"minCoreVersion": "2.9.0",
		"protocolVersion": 1,
		"frontend": "ui/index.js",
		"hostApiVersion": 1,
		"capabilities": ["status"]
	}`), 0644); err != nil {
		t.Fatal(err)
	}

	escapeDir := filepath.Join(dir, "bad-path")
	if err := os.MkdirAll(escapeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(escapeDir, "plugin.json"), []byte(`{
		"id": "bad-path",
		"name": "Bad Path",
		"version": "1.0.0",
		"platform": "windows-amd64",
		"protocolVersion": 1,
		"frontend": "../outside.js"
	}`), 0644); err != nil {
		t.Fatal(err)
	}

	mismatchDir := filepath.Join(dir, "mismatch-dir")
	if err := os.MkdirAll(mismatchDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mismatchDir, "plugin.json"), []byte(`{
		"id": "other-id",
		"name": "Mismatch",
		"version": "1.0.0",
		"platform": "windows-amd64",
		"protocolVersion": 1,
		"frontend": "ui/index.js"
	}`), 0644); err != nil {
		t.Fatal(err)
	}

	oldCoreDir := filepath.Join(dir, "too-new")
	if err := os.MkdirAll(filepath.Join(oldCoreDir, "ui"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldCoreDir, "ui", "index.js"), []byte("// ok"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldCoreDir, "plugin.json"), []byte(`{
		"id": "too-new",
		"name": "Too New",
		"version": "1.0.0",
		"platform": "windows-amd64",
		"minCoreVersion": "4.0.0",
		"protocolVersion": 1,
		"frontend": "ui/index.js"
	}`), 0644); err != nil {
		t.Fatal(err)
	}

	discovered := DiscoverExternal(dir, "windows-amd64", "3.0.0")
	if len(discovered) != 4 {
		t.Fatalf("discovered = %d, want 4", len(discovered))
	}

	byID := map[string]PluginInfo{}
	for _, item := range discovered {
		byID[item.ID] = item
	}

	if sample := byID["sample-ext"]; sample.State != StateDiscovered || !sample.Supported {
		t.Fatalf("sample-ext unexpected state: %+v", sample)
	}
	if bad := byID["bad-path"]; bad.State != StateIncompatible || bad.Supported {
		t.Fatalf("bad-path unexpected state: %+v", bad)
	}
	if mismatch := byID["other-id"]; mismatch.State != StateIncompatible || mismatch.Supported {
		t.Fatalf("other-id unexpected state: %+v", mismatch)
	}
	if tooNew := byID["too-new"]; tooNew.State != StateIncompatible || tooNew.Supported {
		t.Fatalf("too-new unexpected state: %+v", tooNew)
	}
}

func TestDiscoverExternalHandlesMissingDir(t *testing.T) {
	discovered := DiscoverExternal(filepath.Join(t.TempDir(), "missing"), "windows-amd64", "3.0.0")
	if discovered == nil || len(discovered) != 0 {
		t.Fatalf("discovered = %#v, want empty slice", discovered)
	}
}
