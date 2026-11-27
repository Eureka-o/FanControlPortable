package marketplace

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/ghmirror"
	"github.com/Eureka-o/FanControlPortable/internal/plugins"
)

func buildTestZip(t *testing.T, files map[string]string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:])
}

func TestMarketplaceFetchAndSHA256AndZipSlip(t *testing.T) {
	sidecarZip, sidecarSHA := buildTestZip(t, map[string]string{
		"demo-sidecar/plugin.json": `{
			"id": "demo-sidecar",
			"name": "Demo Sidecar",
			"version": "1.2.0",
			"platform": "` + plugins.CurrentPlatform() + `",
			"protocolVersion": 1,
			"runtime": "sidecar",
			"backend": "bin/demo.exe",
			"frontend": "ui/index.js",
			"uiSlot": "sidebar-page"
		}`,
		"demo-sidecar/bin/demo.exe": "MZ",
		"demo-sidecar/ui/index.js":  "// ui",
	})
	inprocZip, inprocSHA := buildTestZip(t, map[string]string{
		"plugin.json": `{
			"id": "demo-inproc",
			"name": "Demo Inproc",
			"version": "1.0.0",
			"platform": "` + plugins.CurrentPlatform() + `",
			"protocolVersion": 1,
			"runtime": "inproc-dll",
			"backend": "bin/demo.dll"
		}`,
		"bin/demo.dll": "MZ",
	})
	themeZip, themeSHA := buildTestZip(t, map[string]string{
		"theme.json": `{"id":"aurora-dark","name":"Aurora Dark","base":"dark","version":"2.0.0","layer":"advanced"}`,
		"theme.css":  `:root[data-theme="aurora-dark"] { --primary: #38bdf8; }`,
	})

	var serverURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/catalog.json":
			cat := Catalog{
				SchemaVersion: 1,
				UpdatedAt:     "2026-11-26T00:00:00Z",
				Plugins: []PluginEntry{
					{
						ID:              "demo-sidecar",
						Name:            "Demo Sidecar",
						Version:         "1.2.0",
						Runtime:         plugins.RuntimeSidecar,
						UISlot:          plugins.UISlotSidebarPage,
						Platform:        plugins.CurrentPlatform(),
						ProtocolVersion: 1,
						DownloadURL:     serverURL + "/demo-sidecar.zip",
						SHA256:          sidecarSHA,
						Size:            int64(len(sidecarZip)),
					},
					{
						ID:              "demo-inproc",
						Name:            "Demo Inproc",
						Version:         "1.0.0",
						Runtime:         plugins.RuntimeInprocDLL,
						UISlot:          plugins.UISlotNone,
						Platform:        plugins.CurrentPlatform(),
						ProtocolVersion: 1,
						RestartRequired: true,
						DownloadURL:     serverURL + "/demo-inproc.zip",
						SHA256:          inprocSHA,
						Size:            int64(len(inprocZip)),
					},
				},
				Themes: []ThemeEntry{
					{
						ID:          "aurora-dark",
						Name:        "Aurora Dark",
						Version:     "2.0.0",
						Base:        "dark",
						Layer:       "advanced",
						DownloadURL: serverURL + "/aurora-dark.zip",
						SHA256:      themeSHA,
						Size:        int64(len(themeZip)),
					},
				},
			}
			_ = json.NewEncoder(w).Encode(cat)
		case "/demo-sidecar.zip":
			_, _ = w.Write(sidecarZip)
		case "/demo-inproc.zip":
			_, _ = w.Write(inprocZip)
		case "/aurora-dark.zip":
			_, _ = w.Write(themeZip)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	serverURL = srv.URL

	installedPlugins := InstalledPluginMap{
		"demo-sidecar": {ID: "demo-sidecar", Version: "1.0.0"},
	}
	installedThemes := map[string]InstalledThemeMeta{
		"aurora-dark": {Version: "2.0.0", Source: "install"},
	}
	view, err := FetchCatalog(context.Background(), srv.URL+"/catalog.json", plugins.CurrentPlatform(), "3.0.0", installedPlugins, installedThemes, nil)
	if err != nil {
		t.Fatalf("FetchCatalog failed: %v", err)
	}
	if len(view.Plugins) != 2 || len(view.Themes) != 1 {
		t.Fatalf("unexpected catalog counts: plugins=%d themes=%d", len(view.Plugins), len(view.Themes))
	}
	if view.Plugins[0].InstallStatus != InstallStatusUpdateAvailable {
		t.Fatalf("demo-sidecar status = %s, want update_available", view.Plugins[0].InstallStatus)
	}
	if view.Plugins[1].InstallStatus != InstallStatusNotInstalled {
		t.Fatalf("demo-inproc status = %s, want not_installed", view.Plugins[1].InstallStatus)
	}
	if view.Themes[0].InstallStatus != InstallStatusInstalled {
		t.Fatalf("aurora-dark status = %s, want installed", view.Themes[0].InstallStatus)
	}

	// Bad SHA256 rejection.
	if _, _, cleanup, err := DownloadAndVerify(context.Background(), srv.URL+"/demo-sidecar.zip", strings.Repeat("a", 64), nil, nil); err == nil {
		cleanup()
		t.Fatal("expected SHA256 mismatch failure")
	}

	pluginsDir := t.TempDir()
	themesDir := t.TempDir()
	configDir := t.TempDir()

	// Sidecar install (hot, RestartRequired=false).
	zipPath, _, cleanup, err := DownloadAndVerify(context.Background(), srv.URL+"/demo-sidecar.zip", sidecarSHA, nil, nil)
	if err != nil {
		t.Fatalf("download demo-sidecar failed: %v", err)
	}
	defer cleanup()
	res, err := InstallPluginFromZip(zipPath, pluginsDir, "demo-sidecar", plugins.CurrentPlatform(), "3.0.0", false)
	if err != nil || res.RestartRequired {
		t.Fatalf("InstallPluginFromZip sidecar = %+v, err=%v", res, err)
	}
	kind, err := DetectZipKind(zipPath)
	if err != nil || kind != "plugin" {
		t.Fatalf("DetectZipKind plugin = %q, err=%v", kind, err)
	}

	// Inproc install + update + pending-delete uninstall.
	inprocPath, _, cleanupInproc, err := DownloadAndVerify(context.Background(), srv.URL+"/demo-inproc.zip", inprocSHA, nil, nil)
	if err != nil {
		t.Fatalf("download demo-inproc failed: %v", err)
	}
	defer cleanupInproc()
	res1, err := InstallPluginFromZip(inprocPath, pluginsDir, "demo-inproc", plugins.CurrentPlatform(), "3.0.0", false)
	if err != nil || !res1.RestartRequired {
		t.Fatalf("initial inproc install = %+v, err=%v", res1, err)
	}
	res2, err := InstallPluginFromZip(inprocPath, pluginsDir, "demo-inproc", plugins.CurrentPlatform(), "3.0.0", true)
	if err != nil || !res2.RestartRequired || !plugins.HasPendingStaging(pluginsDir, "demo-inproc") {
		t.Fatalf("staged inproc update = %+v, err=%v", res2, err)
	}
	unRes, err := UninstallPlugin(pluginsDir, configDir, "demo-inproc", true, false)
	if err != nil || !unRes.RestartRequired {
		t.Fatalf("pending-delete inproc = %+v, err=%v", unRes, err)
	}
	if err := plugins.ApplyPendingStagedOps(pluginsDir, configDir, nil); err != nil {
		t.Fatalf("ApplyPendingStagedOps failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, "demo-inproc")); !os.IsNotExist(err) {
		t.Fatalf("expected demo-inproc deleted on restart apply")
	}

	// Theme install + uninstall (hot, no restart).
	tZip, _, cleanupTheme, err := DownloadAndVerify(context.Background(), srv.URL+"/aurora-dark.zip", themeSHA, nil, nil)
	if err != nil {
		t.Fatalf("download theme failed: %v", err)
	}
	defer cleanupTheme()
	tKind, err := DetectZipKind(tZip)
	if err != nil || tKind != "theme" {
		t.Fatalf("DetectZipKind theme = %q, err=%v", tKind, err)
	}
	tRes, err := InstallThemeFromZip(tZip, themesDir, "aurora-dark")
	if err != nil || tRes.RestartRequired {
		t.Fatalf("InstallThemeFromZip = %+v, err=%v", tRes, err)
	}
	if err := UninstallTheme(themesDir, "aurora-dark"); err != nil {
		t.Fatalf("UninstallTheme failed: %v", err)
	}

	// Zip Slip rejection.
	evilBytes, _ := buildTestZip(t, map[string]string{
		"../evil.txt": "bad",
	})
	evilPath := filepath.Join(t.TempDir(), "evil.zip")
	if err := os.WriteFile(evilPath, evilBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SafeExtractZip(evilPath, t.TempDir()); err == nil {
		t.Fatal("expected Zip Slip rejection")
	}
}

func TestMirrorLatencyRacingSelectsFastestAndFallsBackOnBadHash(t *testing.T) {
	pkgBytes, pkgSHA := buildTestZip(t, map[string]string{
		"plugin.json": `{
			"id": "mirror-demo",
			"name": "Mirror Demo",
			"version": "1.0.0",
			"platform": "` + plugins.CurrentPlatform() + `",
			"protocolVersion": 1,
			"frontend": "ui/index.js"
		}`,
		"ui/index.js": "// mirror demo",
	})

	catalogPayload, err := json.Marshal(Catalog{
		SchemaVersion: 1,
		UpdatedAt:     "2026-11-27T00:00:00Z",
		Plugins: []PluginEntry{{
			ID:              "mirror-demo",
			Name:            "Mirror Demo",
			Version:         "1.0.0",
			Runtime:         plugins.RuntimeFrontendOnly,
			UISlot:          "settings-panel",
			Platform:        plugins.CurrentPlatform(),
			ProtocolVersion: 1,
			DownloadURL:     "https://github.com/Eureka-o/FanControl-Plugins/releases/download/v1.0.0/mirror-demo.zip",
			SHA256:          pkgSHA,
			Size:            int64(len(pkgBytes)),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	slowOrigin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(250 * time.Millisecond):
			_, _ = w.Write(catalogPayload)
		}
	}))
	defer slowOrigin.Close()

	corruptFastMirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>corrupted fast mirror</html>"))
	}))
	defer corruptFastMirror.Close()

	healthyFastMirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(30 * time.Millisecond)
		if strings.HasSuffix(r.URL.Path, "catalog.json") {
			_, _ = w.Write(catalogPayload)
			return
		}
		_, _ = w.Write(pkgBytes)
	}))
	defer healthyFastMirror.Close()

	origBuilder := mirrorCandidateBuilder
	mirrorCandidateBuilder = func(rawURL string) []ghmirror.Candidate {
		if strings.HasSuffix(rawURL, "catalog.json") {
			return []ghmirror.Candidate{
				{Name: "origin", Label: "GitHub 源站", URL: slowOrigin.URL + "/catalog.json"},
				{Name: "bad-mirror", Label: "损坏镜像", URL: corruptFastMirror.URL + "/catalog.json"},
				{Name: "fast-mirror", Label: "高速镜像", URL: healthyFastMirror.URL + "/catalog.json"},
			}
		}
		return []ghmirror.Candidate{
			{Name: "origin", Label: "GitHub 源站", URL: slowOrigin.URL + "/mirror-demo.zip"},
			{Name: "bad-mirror", Label: "损坏镜像", URL: corruptFastMirror.URL + "/mirror-demo.zip"},
			{Name: "fast-mirror", Label: "高速镜像", URL: healthyFastMirror.URL + "/mirror-demo.zip"},
		}
	}
	t.Cleanup(func() { mirrorCandidateBuilder = origBuilder })

	view, err := FetchCatalog(
		context.Background(),
		"https://raw.githubusercontent.com/Eureka-o/FanControl-Plugins/main/catalog.json",
		plugins.CurrentPlatform(),
		"3.0.0",
		InstalledPluginMap{},
		map[string]InstalledThemeMeta{},
		nil,
	)
	if err != nil {
		t.Fatalf("FetchCatalog failed: %v", err)
	}
	if view.ResolvedSource != "高速镜像" || view.LatencyMs <= 0 {
		t.Fatalf("expected winner '高速镜像', got source=%q latency=%d", view.ResolvedSource, view.LatencyMs)
	}

	zipPath, gotSHA, cleanup, err := DownloadAndVerify(
		context.Background(),
		view.Plugins[0].DownloadURL,
		view.Plugins[0].SHA256,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("DownloadAndVerify failed: %v", err)
	}
	defer cleanup()
	if gotSHA != pkgSHA {
		t.Fatalf("SHA256 mismatch: got %q want %q", gotSHA, pkgSHA)
	}
	if _, err := os.Stat(zipPath); err != nil {
		t.Fatalf("downloaded zip missing: %v", err)
	}
}

func TestFetchCatalogDetectsThemeUpdateWithoutLocalVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cat := Catalog{
			SchemaVersion: 1,
			UpdatedAt:     "2026-11-27T00:00:00Z",
			Themes: []ThemeEntry{
				{
					ID:          "xiaoba",
					Name:        "小八",
					Version:     "1.2.0",
					Base:        "light",
					Layer:       "advanced",
					DownloadURL: "https://example.com/xiaoba-v1.2.0.zip",
					SHA256:      strings.Repeat("b", 64),
					Size:        1024,
				},
				{
					ID:          "doro",
					Name:        "Doro",
					Version:     "1.3.0",
					Base:        "light",
					Layer:       "advanced",
					DownloadURL: "https://example.com/doro-v1.3.0.zip",
					SHA256:      strings.Repeat("c", 64),
					Size:        1024,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(cat)
	}))
	defer srv.Close()

	installedThemes := map[string]InstalledThemeMeta{
		"xiaoba": {Version: "", Source: "install", Builtin: true},
		"doro":   {Version: "1.2.0", Source: "install", Builtin: true},
	}
	view, err := FetchCatalog(context.Background(), srv.URL+"/catalog.json", plugins.CurrentPlatform(), "3.0.0", InstalledPluginMap{}, installedThemes, nil)
	if err != nil {
		t.Fatalf("FetchCatalog failed: %v", err)
	}
	if len(view.Themes) != 2 {
		t.Fatalf("expected 2 themes, got %d", len(view.Themes))
	}
	for _, item := range view.Themes {
		if item.InstallStatus != InstallStatusUpdateAvailable {
			t.Fatalf("theme %s status = %s, want update_available", item.ID, item.InstallStatus)
		}
		if !item.InstalledBuiltin {
			t.Fatalf("theme %s expected InstalledBuiltin=true", item.ID)
		}
	}
}

func TestSafeExtractZipHandlesWindowsBackslashDirectoryEntries(t *testing.T) {
	buildRawZip := func(entries []struct {
		name string
		body string
	}) string {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for _, entry := range entries {
			w, err := zw.CreateHeader(&zip.FileHeader{
				Name:   entry.name,
				Method: zip.Store,
			})
			if err != nil {
				t.Fatal(err)
			}
			if entry.body != "" {
				if _, err := w.Write([]byte(entry.body)); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		zipPath := filepath.Join(t.TempDir(), "win-dir.zip")
		if err := os.WriteFile(zipPath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return zipPath
	}

	// 1. Flat theme zip containing Windows-style backslash directory entries with ExternalAttrs == 0.
	themeZip := buildRawZip([]struct {
		name string
		body string
	}{
		{name: `assets\icons\`},
		{name: `assets\icons\iconoir\graph-up.svg`, body: `<svg id="ok"/>`},
		{name: `theme.json`, body: `{"id":"miku-01","name":"Miku","base":"light","version":"1.3.12","layer":"advanced"}`},
		{name: `theme.css`, body: `html[data-theme="miku-01"] { --primary: #39c5bb; }`},
	})
	themesDir := t.TempDir()
	res, err := InstallThemeFromZip(themeZip, themesDir, "miku-01")
	if err != nil {
		t.Fatalf("InstallThemeFromZip failed on backslash dir entry: %v", err)
	}
	if res.ID != "miku-01" {
		t.Fatalf("unexpected result: %+v", res)
	}
	iconsInfo, err := os.Stat(filepath.Join(themesDir, "miku-01", "assets", "icons"))
	if err != nil || !iconsInfo.IsDir() {
		t.Fatalf("expected assets/icons directory, err=%v", err)
	}
	svgBytes, err := os.ReadFile(filepath.Join(themesDir, "miku-01", "assets", "icons", "iconoir", "graph-up.svg"))
	if err != nil || string(svgBytes) != `<svg id="ok"/>` {
		t.Fatalf("extracted svg = %q, err=%v", string(svgBytes), err)
	}

	// 2. Single-root-wrapped zip whose root dir entry uses a trailing backslash.
	wrappedZip := buildRawZip([]struct {
		name string
		body string
	}{
		{name: `wrapped-theme\`},
		{name: `wrapped-theme\assets\icons\`},
		{name: `wrapped-theme\assets\icons\spark.svg`, body: `<svg/>`},
		{name: `wrapped-theme\theme.json`, body: `{"id":"wrapped-theme","name":"Wrapped","base":"light","version":"1.0.0"}`},
		{name: `wrapped-theme\theme.css`, body: `html[data-theme="wrapped-theme"] { --primary: #000; }`},
	})
	kind, err := DetectZipKind(wrappedZip)
	if err != nil || kind != "theme" {
		t.Fatalf("DetectZipKind wrapped = %q, err=%v", kind, err)
	}
	res2, err := InstallThemeFromZip(wrappedZip, themesDir, "wrapped-theme")
	if err != nil || res2.ID != "wrapped-theme" {
		t.Fatalf("InstallThemeFromZip wrapped failed: res=%+v err=%v", res2, err)
	}
	if _, err := os.Stat(filepath.Join(themesDir, "wrapped-theme", "assets", "icons", "spark.svg")); err != nil {
		t.Fatalf("expected unwrapped spark.svg, err=%v", err)
	}
}
