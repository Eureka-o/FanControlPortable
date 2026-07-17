package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/TIANLI0/THRM/internal/plugins"
	"github.com/TIANLI0/THRM/internal/theme"
)

func TestThemeAssetMiddlewareServesThemeAssetBeforeNextHandler(t *testing.T) {
	root := t.TempDir()
	themeDir := filepath.Join(root, "themes", "xiaoba-deluxe")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "hero.webp"), []byte("webp-data"), 0o644); err != nil {
		t.Fatalf("write theme asset failed: %v", err)
	}

	manager := theme.NewManager(filepath.Join(root, "themes"), nil, nil)
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fallback"))
	})

	handler := themeAssetMiddleware(manager)(next)
	req := httptest.NewRequest(http.MethodGet, "/theme-assets/xiaoba-deluxe/hero.webp", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatal("theme asset request should not reach the default asset handler")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "webp-data" {
		t.Fatalf("body = %q, want theme asset data", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/webp" {
		t.Fatalf("content type = %q, want image/webp", got)
	}
}

func TestThemeAssetMiddlewarePassesThroughOtherRequests(t *testing.T) {
	manager := theme.NewManager(t.TempDir(), nil, nil)
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	handler := themeAssetMiddleware(manager)(next)
	req := httptest.NewRequest(http.MethodGet, "/assets/index.js", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("non-theme request should reach the default asset handler")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestRuntimeAssetMiddlewareServesVersionedPluginAsset(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "plugins", "test-plugin")
	if err := os.MkdirAll(filepath.Join(pluginDir, "backend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pluginDir, "ui"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string][]byte{
		filepath.Join(pluginDir, "backend", "driver.exe"): []byte("backend"),
		filepath.Join(pluginDir, "ui", "index.js"):        []byte("window.testPlugin = true;"),
	} {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := plugins.Manifest{
		ID:              "test-plugin",
		Name:            "Test Plugin",
		Version:         "1.0.0",
		Platform:        runtime.GOOS + "-" + runtime.GOARCH,
		ProtocolVersion: plugins.ProtocolVersionV1,
		Backend:         "backend/driver.exe",
		Frontend:        "ui/index.js",
		HostAPIVersion:  plugins.HostAPIVersionV1,
		Page:            plugins.PageManifest{ID: "control", Title: "Test", Icon: "plug", Order: 500},
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, plugins.ManifestFileName), manifestData, 0o644); err != nil {
		t.Fatal(err)
	}

	catalog := plugins.NewCatalog(filepath.Join(root, "plugins"), "2.5.3")
	catalog.Refresh()
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	handler := runtimeAssetMiddleware(theme.NewManager(filepath.Join(root, "themes"), nil, nil), catalog)(next)
	req := httptest.NewRequest(http.MethodGet, "/plugin-assets/test-plugin/ui/index.js?v=1.0.0", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatal("plugin asset request should not reach the default asset handler")
	}
	if rec.Code != http.StatusOK || rec.Body.String() != "window.testPlugin = true;" {
		t.Fatalf("response = %d %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/javascript; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("cache control = %q", got)
	}
}
