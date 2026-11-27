package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/theme"
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

func TestThemeAssetHandlerCacheControlByVersionQuery(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "themes")
	themeDir := filepath.Join(installDir, "xiaoba")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "hero.webp"), []byte("webp-bytes"), 0o644); err != nil {
		t.Fatalf("write asset failed: %v", err)
	}

	handler := themeAssetHandler(theme.NewManager(installDir, nil, nil))

	versionedReq := httptest.NewRequest(http.MethodGet, "/theme-assets/xiaoba/hero.webp?v=1.0.0", nil)
	versionedRec := httptest.NewRecorder()
	handler.ServeHTTP(versionedRec, versionedReq)
	if versionedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for versioned request, got %d", versionedRec.Code)
	}
	if got := versionedRec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("expected immutable cache-control for versioned request, got %q", got)
	}

	unversionedReq := httptest.NewRequest(http.MethodGet, "/theme-assets/xiaoba/hero.webp", nil)
	unversionedRec := httptest.NewRecorder()
	handler.ServeHTTP(unversionedRec, unversionedReq)
	if unversionedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for unversioned request, got %d", unversionedRec.Code)
	}
	if got := unversionedRec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("expected no-cache for unversioned request, got %q", got)
	}
}

func TestPluginAssetMiddlewareServesJSAndRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	pluginsDir := filepath.Join(root, "plugins")
	uiDir := filepath.Join(pluginsDir, "demo-plugin", "ui")
	if err := os.MkdirAll(uiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uiDir, "index.js"), []byte("console.log('ok');"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginsDir, "demo-plugin", "plugin.json"), []byte(`{"id":"demo-plugin"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	handler := customAssetMiddleware(nil, pluginsDir, nil)(next)

	req := httptest.NewRequest(http.MethodGet, "/plugin-assets/demo-plugin/ui/index.js?v=1.0.0", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "console.log('ok');" {
		t.Fatalf("unexpected plugin asset response: code=%d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("unexpected cache-control: %q", got)
	}

	badReq := httptest.NewRequest(http.MethodGet, "/plugin-assets/demo-plugin/../secret.txt", nil)
	badRec := httptest.NewRecorder()
	handler.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for traversal request, got %d", badRec.Code)
	}
}
