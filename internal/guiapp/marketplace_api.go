package guiapp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/config"
	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/marketplace"
	"github.com/Eureka-o/FanControlPortable/internal/plugins"
	"github.com/Eureka-o/FanControlPortable/internal/version"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const marketplaceProgressEvent = "marketplace-progress"

func (a *App) marketplaceHTTPClientFactory(timeout time.Duration) marketplace.HTTPClientFactory {
	return func(bypassProxy bool) *http.Client {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.ResponseHeaderTimeout = 20 * time.Second
		if bypassProxy {
			transport.Proxy = nil
		} else {
			enabled, server, err := readUpdateSystemProxy()
			if err != nil {
				transport.Proxy = func(*http.Request) (*url.URL, error) {
					return nil, fmt.Errorf("read Windows proxy settings: %w", err)
				}
			} else if enabled && strings.TrimSpace(server) != "" {
				transport.Proxy = func(request *http.Request) (*url.URL, error) {
					return parseWindowsProxyServer(server, request.URL.Scheme)
				}
			}
		}
		return &http.Client{
			Transport: transport,
			Timeout:   timeout,
		}
	}
}

func (a *App) emitMarketplaceProgress(evt marketplace.ProgressEvent) {
	a.emit(marketplaceProgressEvent, evt)
}

func (a *App) resolveMarketplaceDirs() (pluginsDir, themesDir, configDir string) {
	installDir := config.GetInstallDir()
	pluginsDir = filepath.Join(installDir, "plugins")
	themesDir = filepath.Join(installDir, "themes")
	if a.themeManager != nil {
		if resolved := strings.TrimSpace(a.themeManager.ResolveDir()); resolved != "" {
			themesDir = resolved
		}
	}
	configDir = installDir
	if cfg, err := a.GetConfig(); err == nil && strings.TrimSpace(cfg.ConfigPath) != "" {
		configDir = filepath.Dir(cfg.ConfigPath)
	}
	return pluginsDir, themesDir, configDir
}

// FetchMarketplaceCatalog downloads and evaluates catalog.json from the official GitHub repo or custom URL/path.
func (a *App) FetchMarketplaceCatalog(catalogURL string) (*marketplace.CatalogView, error) {
	resolvedURL := strings.TrimSpace(catalogURL)
	if resolvedURL == "" {
		if cfg, err := a.GetConfig(); err == nil && strings.TrimSpace(cfg.PluginCatalogURL) != "" {
			resolvedURL = cfg.PluginCatalogURL
		}
	}

	installedPlugins := marketplace.InstalledPluginMap{}
	if list, err := a.GetPlugins(); err == nil {
		for _, item := range list {
			if item.Kind == plugins.KindBuiltin {
				continue
			}
			installedPlugins[item.ID] = item
		}
	}

	installedThemes := map[string]marketplace.InstalledThemeMeta{}
	for _, t := range a.ListThemes() {
		installedThemes[t.ID] = marketplace.InstalledThemeMeta{
			Version: t.Version,
			Source:  t.Source,
			Builtin: t.Builtin,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return marketplace.FetchCatalog(
		ctx,
		resolvedURL,
		plugins.CurrentPlatform(),
		version.Get(),
		installedPlugins,
		installedThemes,
		a.marketplaceHTTPClientFactory(20*time.Second),
	)
}

// InstallMarketplacePlugin downloads, verifies SHA256, and installs or stages a plugin package.
func (a *App) InstallMarketplacePlugin(pluginID, downloadURL, expectedSHA256 string) (*marketplace.InstallResult, error) {
	pluginsDir, _, _ := a.resolveMarketplaceDirs()

	isRunningSidecar := false
	isLoadedInproc := false
	wasEnabled := false
	if currentList, err := a.GetPlugins(); err == nil {
		for _, item := range currentList {
			if item.ID == pluginID {
				wasEnabled = item.Enabled
				if item.Runtime == plugins.RuntimeSidecar && item.Running {
					isRunningSidecar = true
				}
				if item.Runtime == plugins.RuntimeInprocDLL && (item.Running || item.State == plugins.StateReady) {
					isLoadedInproc = true
				}
				break
			}
		}
	}

	a.emitMarketplaceProgress(marketplace.ProgressEvent{
		Kind:    "plugin",
		ID:      pluginID,
		Stage:   "downloading",
		Percent: 0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	zipPath, _, cleanup, err := marketplace.DownloadAndVerify(
		ctx,
		downloadURL,
		expectedSHA256,
		a.marketplaceHTTPClientFactory(3*time.Minute),
		func(received, total int64) {
			pct := 0
			if total > 0 {
				pct = int(received * 90 / total)
			}
			a.emitMarketplaceProgress(marketplace.ProgressEvent{
				Kind:     "plugin",
				ID:       pluginID,
				Stage:    "downloading",
				Percent:  pct,
				Received: received,
				Total:    total,
			})
		},
	)
	if err != nil {
		a.emitMarketplaceProgress(marketplace.ProgressEvent{
			Kind:    "plugin",
			ID:      pluginID,
			Stage:   "error",
			Percent: -1,
			Message: err.Error(),
		})
		return nil, err
	}
	defer cleanup()

	a.emitMarketplaceProgress(marketplace.ProgressEvent{
		Kind:    "plugin",
		ID:      pluginID,
		Stage:   "extracting",
		Percent: 95,
	})

	if isRunningSidecar {
		_, _ = a.SetPluginEnabled(pluginID, false)
	}

	result, err := marketplace.InstallPluginFromZip(
		zipPath,
		pluginsDir,
		pluginID,
		plugins.CurrentPlatform(),
		version.Get(),
		isLoadedInproc,
	)
	if err != nil {
		a.emitMarketplaceProgress(marketplace.ProgressEvent{
			Kind:    "plugin",
			ID:      pluginID,
			Stage:   "error",
			Percent: -1,
			Message: err.Error(),
		})
		return nil, err
	}

	_, _ = a.RefreshPlugins()
	if result.RestartRequired {
		_, _ = a.sendRequest(ipc.ReqMarkPluginPendingRestart, ipc.PluginIDParams{PluginID: result.ID})
	} else if isRunningSidecar && wasEnabled {
		_, _ = a.SetPluginEnabled(result.ID, true)
	}

	a.emitMarketplaceProgress(marketplace.ProgressEvent{
		Kind:    "plugin",
		ID:      result.ID,
		Stage:   "done",
		Percent: 100,
		Message: result.Message,
	})
	return result, nil
}

// UninstallPlugin removes a sidecar/frontend plugin or schedules an inproc DLL for removal on Core restart.
func (a *App) UninstallPlugin(pluginID string, removeData bool) (*marketplace.InstallResult, error) {
	pluginsDir, _, configDir := a.resolveMarketplaceDirs()

	isLoadedInproc := false
	isSidecar := false
	if currentList, err := a.GetPlugins(); err == nil {
		for _, item := range currentList {
			if item.ID == pluginID {
				isLoadedInproc = item.Runtime == plugins.RuntimeInprocDLL && (item.Running || item.State == plugins.StateReady)
				isSidecar = item.Runtime == plugins.RuntimeSidecar
				break
			}
		}
	}
	if isSidecar {
		_, _ = a.SetPluginEnabled(pluginID, false)
	}

	result, err := marketplace.UninstallPlugin(pluginsDir, configDir, pluginID, isLoadedInproc, removeData)
	if err != nil {
		return nil, err
	}
	if result.RestartRequired {
		_, _ = a.sendRequest(ipc.ReqMarkPluginPendingRestart, ipc.PluginIDParams{PluginID: pluginID})
	} else {
		_, _ = a.RefreshPlugins()
	}
	return result, nil
}

// InstallMarketplaceTheme downloads, verifies SHA256, and hot-installs a theme package without restart.
func (a *App) InstallMarketplaceTheme(themeID, downloadURL, expectedSHA256 string) (*marketplace.InstallResult, error) {
	_, themesDir, _ := a.resolveMarketplaceDirs()
	a.emitMarketplaceProgress(marketplace.ProgressEvent{
		Kind:    "theme",
		ID:      themeID,
		Stage:   "downloading",
		Percent: 0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	zipPath, _, cleanup, err := marketplace.DownloadAndVerify(
		ctx,
		downloadURL,
		expectedSHA256,
		a.marketplaceHTTPClientFactory(2*time.Minute),
		func(received, total int64) {
			pct := 0
			if total > 0 {
				pct = int(received * 90 / total)
			}
			a.emitMarketplaceProgress(marketplace.ProgressEvent{
				Kind:     "theme",
				ID:       themeID,
				Stage:    "downloading",
				Percent:  pct,
				Received: received,
				Total:    total,
			})
		},
	)
	if err != nil {
		a.emitMarketplaceProgress(marketplace.ProgressEvent{
			Kind:    "theme",
			ID:      themeID,
			Stage:   "error",
			Percent: -1,
			Message: err.Error(),
		})
		return nil, err
	}
	defer cleanup()

	res, err := marketplace.InstallThemeFromZip(zipPath, themesDir, themeID)
	if err != nil {
		a.emitMarketplaceProgress(marketplace.ProgressEvent{
			Kind:    "theme",
			ID:      themeID,
			Stage:   "error",
			Percent: -1,
			Message: err.Error(),
		})
		return nil, err
	}
	a.emitMarketplaceProgress(marketplace.ProgressEvent{
		Kind:    "theme",
		ID:      res.ID,
		Stage:   "done",
		Percent: 100,
		Message: res.Message,
	})
	return res, nil
}

// UninstallTheme removes an installed non-builtin theme from the themes directory.
func (a *App) UninstallTheme(themeID string) error {
	if a.themeManager != nil && a.themeManager.IsBuiltin(themeID) {
		return fmt.Errorf("内置基础主题不支持卸载: %s", themeID)
	}
	_, themesDir, _ := a.resolveMarketplaceDirs()
	return marketplace.UninstallTheme(themesDir, themeID)
}

// ImportMarketplacePackage opens a local .zip file dialog (or uses zipPath directly when provided)
// and installs either a plugin or theme package cleanly.
func (a *App) ImportMarketplacePackage(zipPath string) (*marketplace.InstallResult, error) {
	selected := strings.TrimSpace(zipPath)
	if selected == "" {
		if a.ctx == nil {
			return nil, fmt.Errorf("窗口上下文尚未就绪")
		}
		chosen, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
			Title: "选择插件或主题安装包 (.zip)",
			Filters: []runtime.FileFilter{
				{DisplayName: "FanControl 扩展/主题包 (*.zip)", Pattern: "*.zip"},
			},
		})
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(chosen) == "" {
			return nil, nil
		}
		selected = chosen
	}

	kind, err := marketplace.DetectZipKind(selected)
	if err != nil {
		return nil, err
	}
	if kind == "theme" {
		_, themesDir, _ := a.resolveMarketplaceDirs()
		return marketplace.InstallThemeFromZip(selected, themesDir, "")
	}

	pluginsDir, _, _ := a.resolveMarketplaceDirs()
	res, err := marketplace.InstallPluginFromZip(selected, pluginsDir, "", plugins.CurrentPlatform(), version.Get(), false)
	if err != nil {
		return nil, err
	}
	_, _ = a.RefreshPlugins()
	if res.RestartRequired {
		_, _ = a.sendRequest(ipc.ReqMarkPluginPendingRestart, ipc.PluginIDParams{PluginID: res.ID})
	}
	return res, nil
}
