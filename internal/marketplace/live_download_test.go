package marketplace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/ghmirror"
)

func TestLiveMarketplaceMirrorRaceDownload(t *testing.T) {
	if os.Getenv("FANCONTROL_TEST_LIVE_MARKETPLACE") != "1" {
		t.Skip("set FANCONTROL_TEST_LIVE_MARKETPLACE=1 to run live GitHub/mirror marketplace download verification")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	view, err := FetchCatalog(
		ctx,
		DefaultCatalogURL,
		"windows-amd64",
		"3.0.0",
		InstalledPluginMap{},
		map[string]InstalledThemeMeta{},
		nil,
	)
	if err != nil {
		t.Fatalf("FetchCatalog(DefaultCatalogURL) failed: %v", err)
	}
	t.Logf("Marketplace Catalog winner=%s (%d ms), plugins=%d, themes=%d", view.ResolvedSource, view.LatencyMs, len(view.Plugins), len(view.Themes))

	root := t.TempDir()
	pluginsDir := filepath.Join(root, "plugins")
	for _, id := range []string{"rtss-osd", "omen-fan"} {
		var entry *CatalogPluginView
		for i := range view.Plugins {
			if view.Plugins[i].ID == id {
				entry = &view.Plugins[i]
				break
			}
		}
		if entry == nil {
			t.Fatalf("plugin %s missing from live catalog", id)
		}
		if !entry.Compatible {
			t.Fatalf("plugin %s incompatible: %s", id, entry.Reason)
		}

		ranked := ghmirror.RankByLatency(ctx, ghmirror.BuildCandidates(entry.DownloadURL), nil, "FanControl-Marketplace")
		for _, r := range ranked {
			t.Logf("  %s candidate: %s probed=%v latency=%d ms", id, r.Name, r.Probed, r.LatencyMs)
		}

		zipPath, actualSHA, cleanup, err := DownloadAndVerify(ctx, entry.DownloadURL, entry.SHA256, nil, nil)
		if err != nil {
			t.Fatalf("DownloadAndVerify(%s) failed: %v", id, err)
		}
		res, instErr := InstallPluginFromZip(zipPath, pluginsDir, id, "windows-amd64", "3.0.0", false)
		cleanup()
		if instErr != nil {
			t.Fatalf("InstallPluginFromZip(%s) failed: %v", id, instErr)
		}
		t.Logf("Installed %s v%s (sha256=%s, restartRequired=%v)", res.ID, res.Version, actualSHA, res.RestartRequired)
		if _, err := os.Stat(filepath.Join(pluginsDir, id, "plugin.json")); err != nil {
			t.Fatalf("extracted plugin.json missing for %s: %v", id, err)
		}
	}

	if len(view.Themes) == 0 {
		t.Fatalf("expected non-empty themes list in live catalog")
	}
	themesDir := filepath.Join(root, "themes")
	for _, wantThemeID := range []string{"cyberpunk2077", "miku-01"} {
		var targetTheme *CatalogThemeView
		for i := range view.Themes {
			if view.Themes[i].ID == wantThemeID {
				targetTheme = &view.Themes[i]
				break
			}
		}
		if targetTheme == nil {
			t.Fatalf("theme %s missing from live catalog", wantThemeID)
		}
		themeZip, themeSHA, themeCleanup, err := DownloadAndVerify(ctx, targetTheme.DownloadURL, targetTheme.SHA256, nil, nil)
		if err != nil {
			t.Fatalf("DownloadAndVerify(theme=%s) failed: %v", targetTheme.ID, err)
		}
		themeRes, themeInstErr := InstallThemeFromZip(themeZip, themesDir, targetTheme.ID)
		themeCleanup()
		if themeInstErr != nil {
			t.Fatalf("InstallThemeFromZip(%s) failed: %v", targetTheme.ID, themeInstErr)
		}
		t.Logf("Installed theme %s v%s (sha256=%s)", themeRes.ID, themeRes.Version, themeSHA)
		if _, err := os.Stat(filepath.Join(themesDir, targetTheme.ID, "theme.css")); err != nil {
			t.Fatalf("extracted theme.css missing for %s: %v", targetTheme.ID, err)
		}
	}
	if _, err := os.Stat(filepath.Join(themesDir, "miku-01", "assets", "icons", "iconoir", "graph-up.svg")); err != nil {
		t.Fatalf("extracted nested icon missing for miku-01: %v", err)
	}
}
