package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifestValidatesAndTrimsFields(t *testing.T) {
	pluginDir := t.TempDir()
	writeManifest(t, pluginDir, `{
		"id": "alpha-device",
		"name": " Alpha Device ",
			"version": "1.0.0",
			"type": "device",
			"executable": "bin/alpha-driver.exe",
			"frontend": "ui/index.html",
			"minCoreVersion": "2.4.1",
			"description": "Sample device driver"
		}`)

	manifest, err := LoadManifest(pluginDir)
	if err != nil {
		t.Fatalf("LoadManifest returned error: %v", err)
	}
	if manifest.ID != "alpha-device" {
		t.Fatalf("ID = %q, want alpha-device", manifest.ID)
	}
	if manifest.Name != "Alpha Device" {
		t.Fatalf("Name = %q, want trimmed name", manifest.Name)
	}
	if manifest.Type != PluginTypeDevice {
		t.Fatalf("Type = %q, want %q", manifest.Type, PluginTypeDevice)
	}

	exePath, err := manifest.ExecutablePath(pluginDir)
	if err != nil {
		t.Fatalf("ExecutablePath returned error: %v", err)
	}
	want := filepath.Join(pluginDir, "bin", "alpha-driver.exe")
	if exePath != want {
		t.Fatalf("ExecutablePath = %q, want %q", exePath, want)
	}
	frontendPath, err := manifest.FrontendPath(pluginDir)
	if err != nil {
		t.Fatalf("FrontendPath returned error: %v", err)
	}
	wantFrontend := filepath.Join(pluginDir, "ui", "index.html")
	if frontendPath != wantFrontend {
		t.Fatalf("FrontendPath = %q, want %q", frontendPath, wantFrontend)
	}
}

func TestManifestValidateRejectsUnsafeValues(t *testing.T) {
	tests := []struct {
		name     string
		manifest Manifest
	}{
		{
			name: "uppercase id",
			manifest: Manifest{
				ID:         "Alpha-Device",
				Name:       "Alpha",
				Version:    "1.0.0",
				Type:       PluginTypeDevice,
				Executable: "alpha.exe",
			},
		},
		{
			name: "missing name",
			manifest: Manifest{
				ID:         "alpha-device",
				Version:    "1.0.0",
				Type:       PluginTypeDevice,
				Executable: "alpha.exe",
			},
		},
		{
			name: "unknown type",
			manifest: Manifest{
				ID:         "alpha-device",
				Name:       "Alpha",
				Version:    "1.0.0",
				Type:       "driver",
				Executable: "alpha.exe",
			},
		},
		{
			name: "absolute executable",
			manifest: Manifest{
				ID:         "alpha-device",
				Name:       "Alpha",
				Version:    "1.0.0",
				Type:       PluginTypeDevice,
				Executable: `C:\drivers\alpha.exe`,
			},
		},
		{
			name: "traversal executable",
			manifest: Manifest{
				ID:         "alpha-device",
				Name:       "Alpha",
				Version:    "1.0.0",
				Type:       PluginTypeDevice,
				Executable: `..\alpha.exe`,
			},
		},
		{
			name: "empty path component",
			manifest: Manifest{
				ID:         "alpha-device",
				Name:       "Alpha",
				Version:    "1.0.0",
				Type:       PluginTypeDevice,
				Executable: "bin//alpha.exe",
			},
		},
		{
			name: "absolute frontend",
			manifest: Manifest{
				ID:         "alpha-device",
				Name:       "Alpha",
				Version:    "1.0.0",
				Type:       PluginTypeDevice,
				Executable: "alpha.exe",
				Frontend:   `C:\drivers\index.html`,
			},
		},
		{
			name: "traversal frontend",
			manifest: Manifest{
				ID:         "alpha-device",
				Name:       "Alpha",
				Version:    "1.0.0",
				Type:       PluginTypeDevice,
				Executable: "alpha.exe",
				Frontend:   `..\ui\index.html`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.manifest.Validate(); err == nil {
				t.Fatal("Validate succeeded, want error")
			}
		})
	}
}

func writeManifest(t *testing.T, pluginDir string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(pluginDir, ManifestFileName), []byte(content), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}
