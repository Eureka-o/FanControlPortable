package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
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

func TestEnsureSeededLeavesManifestlessThemeDirectoryUntouched(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "themes")
	themeDir := filepath.Join(installDir, "thrm")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(themeDir, "user.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	NewManager(installDir, nil, builtinThemeFS("thrm", "THRM", "1.0.0", "/* builtin */")).EnsureSeeded()

	if _, err := os.Stat(filepath.Join(themeDir, manifestName)); !os.IsNotExist(err) {
		t.Fatalf("manifestless theme directory was modified, err=%v", err)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "keep" {
		t.Fatalf("user marker changed: data=%q err=%v", data, err)
	}
}

func TestEnsureSeededLeavesCurrentBuiltinThemeUntouched(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install", "themes")
	writeTheme(t, installDir, "thrm", "Edited THRM", "1.0.0", "/* user edit */")
	customPath := filepath.Join(installDir, "thrm", "user-extra.txt")
	if err := os.WriteFile(customPath, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write custom file: %v", err)
	}
	manifestPath := filepath.Join(installDir, "thrm", manifestName)
	cssPath := filepath.Join(installDir, "thrm", styleName)
	oldTime := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(manifestPath, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes manifest: %v", err)
	}
	if err := os.Chtimes(cssPath, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes css: %v", err)
	}

	manager := NewManager(installDir, nil, builtinThemeFS("thrm", "THRM", "1.0.0", "/* builtin */"))
	manager.EnsureSeeded()

	css, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("read css: %v", err)
	}
	if string(css) != "/* user edit */" {
		t.Fatalf("same-version css changed: %q", css)
	}
	for _, path := range []string{manifestPath, cssPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if !info.ModTime().Equal(oldTime) {
			t.Fatalf("same-version file was rewritten: %s modtime=%v", path, info.ModTime())
		}
	}
	if _, err := os.Stat(customPath); err != nil {
		t.Fatalf("custom file should remain: %v", err)
	}
}

func TestEnsureSeededLeavesOlderBuiltinThemeUntouched(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install", "themes")
	writeTheme(t, installDir, "dune", "Dune", "1.0.0", "/* old */")
	extraPath := filepath.Join(installDir, "dune", "user-extra.txt")
	if err := os.WriteFile(extraPath, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write extra file: %v", err)
	}
	builtin := fstest.MapFS{
		"dune/" + manifestName: {Data: []byte(`{"id":"dune","name":"Dune","base":"light","version":"1.1.0"}`)},
		"dune/" + styleName:    {Data: []byte("/* new */")},
		"dune/new-asset.txt":   {Data: []byte("new")},
	}

	NewManager(installDir, nil, builtin).EnsureSeeded()

	css, err := os.ReadFile(filepath.Join(installDir, "dune", styleName))
	if err != nil || string(css) != "/* old */" {
		t.Fatalf("existing builtin theme changed: css=%q err=%v", css, err)
	}
	if _, err := os.Stat(extraPath); err != nil {
		t.Fatalf("extra file should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "dune", "new-asset.txt")); !os.IsNotExist(err) {
		t.Fatalf("existing theme should not receive new files, err=%v", err)
	}
}

func TestEnsureSeededDoesNotTouchNonBuiltinTheme(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install", "themes")
	writeTheme(t, installDir, "my-custom", "My Custom", "9.0.0", "/* custom */")
	customPath := filepath.Join(installDir, "my-custom", styleName)
	oldTime := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(customPath, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes custom theme: %v", err)
	}

	NewManager(installDir, nil, builtinThemeFS("thrm", "THRM", "1.0.0", "/* builtin */")).EnsureSeeded()

	data, err := os.ReadFile(customPath)
	if err != nil || string(data) != "/* custom */" {
		t.Fatalf("custom theme changed: data=%q err=%v", data, err)
	}
	info, err := os.Stat(customPath)
	if err != nil || !info.ModTime().Equal(oldTime) {
		t.Fatalf("custom theme was rewritten: modtime=%v err=%v", info.ModTime(), err)
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

func TestParseMetaDefaultsMissingLayerToBasic(t *testing.T) {
	meta, ok := parseMeta([]byte(`{"id":"plain","name":"Plain","base":"light"}`), "plain")
	if !ok {
		t.Fatal("parseMeta failed")
	}
	if meta.Layer != LayerBasic {
		t.Fatalf("Layer = %q, want %q", meta.Layer, LayerBasic)
	}
}

func TestParseMetaKeepsAdvancedLayer(t *testing.T) {
	meta, ok := parseMeta([]byte(`{"id":"deluxe","name":"Deluxe","base":"dark","layer":"advanced"}`), "deluxe")
	if !ok {
		t.Fatal("parseMeta failed")
	}
	if meta.Layer != LayerAdvanced {
		t.Fatalf("Layer = %q, want %q", meta.Layer, LayerAdvanced)
	}
}

func TestParseMetaAcceptsLegacyInterfaceAlias(t *testing.T) {
	meta, ok := parseMeta([]byte(`{"id":"compat","name":"Compat","base":"light","interface":"advanced"}`), "compat")
	if !ok {
		t.Fatal("parseMeta failed")
	}
	if meta.Layer != LayerAdvanced {
		t.Fatalf("Layer = %q, want %q", meta.Layer, LayerAdvanced)
	}
}

func TestParseMetaNormalizesInvalidLayerToBasic(t *testing.T) {
	meta, ok := parseMeta([]byte(`{"id":"odd","name":"Odd","base":"light","layer":"full"}`), "odd")
	if !ok {
		t.Fatal("parseMeta failed")
	}
	if meta.Layer != LayerBasic {
		t.Fatalf("Layer = %q, want %q", meta.Layer, LayerBasic)
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

func TestReadCSSRewritesRelativeThemeAssets(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install", "themes")
	writeTheme(t, installDir, "xiaoba", "Xiaoba", "1.0.0", `a{background:url("hero.webp")} b{background:url(data:image/png;base64,aaa)} c{background:url("/absolute.webp")}`)
	writeThemeAsset(t, installDir, "xiaoba", "hero.webp", []byte("webp"))

	manager := NewManager(installDir, nil, nil)
	css, err := manager.ReadCSS("xiaoba")
	if err != nil {
		t.Fatalf("ReadCSS failed: %v", err)
	}
	// 相对资产路径改写为 /theme-assets/ 路由 URL，不再内联为 data: URL。
	if !strings.Contains(css, `url("/theme-assets/xiaoba/hero.webp")`) {
		t.Fatalf("expected relative asset URL to be rewritten to route URL, got %q", css)
	}
	// data: 和绝对路径 URL 保持不变。
	if !strings.Contains(css, "data:image/png") || !strings.Contains(css, `url("/absolute.webp")`) {
		t.Fatalf("absolute/data URLs should stay unchanged, got %q", css)
	}
}

func TestReadCSSRewritesThemeAssetRouteURLs(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install", "themes")
	writeTheme(t, installDir, "xiaoba", "Xiaoba", "1.0.0", `a{background:url("/theme-assets/xiaoba/decorations/star.svg")} b{background:url("/theme-assets/other/star.svg")}`)
	writeThemeAsset(t, installDir, "xiaoba", "decorations/star.svg", []byte("<svg/>"))

	manager := NewManager(installDir, nil, nil)
	css, err := manager.ReadCSS("xiaoba")
	if err != nil {
		t.Fatalf("ReadCSS failed: %v", err)
	}
	// 同主题的 /theme-assets/ URL 重新生成后应与原 URL 等效（不内联为 data:）。
	if !strings.Contains(css, `url("/theme-assets/xiaoba/decorations/star.svg")`) {
		t.Fatalf("same-theme asset route URL should remain as route URL, got %q", css)
	}
	// 其他主题的 URL 不处理，保持原样。
	if !strings.Contains(css, `url("/theme-assets/other/star.svg")`) {
		t.Fatalf("other theme asset route should stay unchanged, got %q", css)
	}
}

func TestEnsureSeededCopiesBuiltinThemeAssets(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install", "themes")
	manager := NewManager(installDir, nil, fstest.MapFS{
		"xiaoba/" + manifestName: {Data: []byte(`{"id":"xiaoba","name":"Xiaoba","base":"light","version":"1.0.0"}`)},
		"xiaoba/" + styleName:    {Data: []byte(`a{background:url("hero.webp")} b{background:url("textures/noise.webp")}`)},
		"xiaoba/hero.webp":       {Data: []byte("webp")},
		"xiaoba/textures/noise.webp": {
			Data: []byte("noise"),
		},
	})

	manager.EnsureSeeded()

	asset, err := manager.ReadAsset("xiaoba", "hero.webp")
	if err != nil {
		t.Fatalf("ReadAsset failed: %v", err)
	}
	if string(asset.Data) != "webp" {
		t.Fatalf("unexpected asset data %q", string(asset.Data))
	}
	nestedAsset, err := manager.ReadAsset("xiaoba", "textures/noise.webp")
	if err != nil {
		t.Fatalf("ReadAsset for nested asset failed: %v", err)
	}
	if string(nestedAsset.Data) != "noise" {
		t.Fatalf("unexpected nested asset data %q", string(nestedAsset.Data))
	}
}

func TestReadAssetRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install", "themes")
	writeTheme(t, installDir, "xiaoba", "Xiaoba", "1.0.0", "/* css */")
	manager := NewManager(installDir, nil, nil)

	if _, err := manager.ReadAsset("xiaoba", "../theme.css"); err == nil {
		t.Fatal("expected traversal asset path to be rejected")
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

func writeThemeAsset(t *testing.T, baseDir, id, assetPath string, data []byte) {
	t.Helper()
	target := filepath.Join(baseDir, id, filepath.FromSlash(assetPath))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("MkdirAll asset dir failed: %v", err)
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		t.Fatalf("write asset failed: %v", err)
	}
}
