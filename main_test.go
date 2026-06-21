package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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
