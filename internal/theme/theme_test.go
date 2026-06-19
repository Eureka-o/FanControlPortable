package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestEnsureSeededWritesBuiltinOnlyToInstallDir(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install", "themes")
	legacyDir := filepath.Join(root, "user", ".fancontrol", "themes")
	manager := NewManager(installDir, []string{legacyDir}, builtinThemeFS("thrm", "THRM", "1.0.0", "/* builtin */"))

	manager.EnsureSeeded()

	if _, err := os.Stat(filepath.Join(installDir, "thrm", manifestName)); err != nil {
		t.Fatalf("expected builtin theme in install dir: %v", err)
	}
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Fatalf("legacy dir should not be created, got err=%v", err)
	}
}

func TestEnsureSeededInstallThemeWinsAndLegacyIsRemoved(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install", "themes")
	legacyDir := filepath.Join(root, "user", ".fancontrol", "themes")
	writeTheme(t, installDir, "xiaoba", "Install Xiaoba", "2.0.0", "/* install */")
	writeTheme(t, legacyDir, "xiaoba", "Legacy Xiaoba", "9.0.0", "/* legacy */")

	manager := NewManager(installDir, []string{legacyDir}, nil)
	manager.EnsureSeeded()

	css, err := manager.ReadCSS("xiaoba")
	if err != nil {
		t.Fatalf("ReadCSS failed: %v", err)
	}
	if !strings.Contains(css, "install") {
		t.Fatalf("install theme should win, got %q", css)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, "xiaoba")); !os.IsNotExist(err) {
		t.Fatalf("legacy duplicate should be removed, got err=%v", err)
	}
}

func TestEnsureSeededMigratesNewestLegacyTheme(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install", "themes")
	legacyOld := filepath.Join(root, "old", ".fancontrolportable", "themes")
	legacyNew := filepath.Join(root, "new", ".fancontrol", "themes")
	writeTheme(t, legacyOld, "doro", "Doro Old", "1.0.0", "/* old */")
	writeTheme(t, legacyNew, "doro", "Doro New", "1.2.0", "/* new */")

	manager := NewManager(installDir, []string{legacyOld, legacyNew, filepath.Join(root, "missing", "themes")}, nil)
	manager.EnsureSeeded()

	css, err := manager.ReadCSS("doro")
	if err != nil {
		t.Fatalf("ReadCSS failed: %v", err)
	}
	if !strings.Contains(css, "new") {
		t.Fatalf("expected newest legacy theme to migrate, got %q", css)
	}
	if _, err := os.Stat(filepath.Join(legacyOld, "doro")); !os.IsNotExist(err) {
		t.Fatalf("old legacy theme should be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(legacyNew, "doro")); !os.IsNotExist(err) {
		t.Fatalf("new legacy theme should be removed, got err=%v", err)
	}
}

func TestListPrioritizesInstallOverLegacyAndBuiltin(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install", "themes")
	legacyDir := filepath.Join(root, "user", ".fancontrol", "themes")
	writeTheme(t, installDir, "maodie", "Install Maodie", "2.0.0", "/* install */")
	writeTheme(t, legacyDir, "maodie", "Legacy Maodie", "9.0.0", "/* legacy */")

	manager := NewManager(installDir, []string{legacyDir}, builtinThemeFS("maodie", "Builtin Maodie", "1.0.0", "/* builtin */"))

	themes := manager.List()
	if len(themes) != 1 {
		t.Fatalf("expected one merged theme, got %d: %#v", len(themes), themes)
	}
	if themes[0].Name != "Install Maodie" || themes[0].Source != SourceInstall {
		t.Fatalf("install theme should be listed, got %#v", themes[0])
	}
}

func TestResolveDirReturnsInstallDirAndDoesNotCreateLegacyDir(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install", "themes")
	legacyDir := filepath.Join(root, "user", ".fancontrol", "themes")
	manager := NewManager(installDir, []string{legacyDir}, nil)

	if got := manager.ResolveDir(); got != installDir {
		t.Fatalf("ResolveDir = %q, want %q", got, installDir)
	}
	if _, err := os.Stat(installDir); err != nil {
		t.Fatalf("expected install dir to be created: %v", err)
	}
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Fatalf("legacy dir should not be created, got err=%v", err)
	}
}

func builtinThemeFS(id, name, version, css string) fstest.MapFS {
	return fstest.MapFS{
		id + "/" + manifestName: {Data: []byte(`{"id":"` + id + `","name":"` + name + `","base":"light","version":"` + version + `"}`)},
		id + "/" + styleName:    {Data: []byte(css)},
	}
}

func writeTheme(t *testing.T, baseDir, id, name, version, css string) {
	t.Helper()
	dir := filepath.Join(baseDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	manifest := `{"id":"` + id + `","name":"` + name + `","base":"light","version":"` + version + `"}`
	if err := os.WriteFile(filepath.Join(dir, manifestName), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, styleName), []byte(css), 0o644); err != nil {
		t.Fatalf("write css failed: %v", err)
	}
}
