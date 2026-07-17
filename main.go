package main

import (
	"bytes"
	"context"
	"embed"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/guiapp"
	"github.com/TIANLI0/THRM/internal/plugins"
	"github.com/TIANLI0/THRM/internal/theme"
	"github.com/TIANLI0/THRM/internal/version"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

const pluginAssetURLPrefix = "/plugin-assets/"

func main() {
	if !guiapp.EnsureCoreServiceRunning() {
		println("警告：无法启动核心服务，GUI 将以有限功能模式运行")
	}

	themeManager := newThemeManager()
	pluginCatalog := plugins.NewCatalog(filepath.Join(config.GetInstallDir(), "plugins"), version.Get())
	pluginCatalog.Refresh()
	app := NewAppWithThemeManager(themeManager)
	windowOptions := guiapp.ResolveWindowsOptions()
	bgR, bgG, bgB, bgA := guiapp.WindowBackgroundColour()

	windowStartState := options.Normal
	for _, arg := range os.Args {
		if arg == "--autostart" || arg == "/autostart" || arg == "-autostart" {
			windowStartState = options.Minimised
			break
		}
	}

	// 创建应用
	err := wails.Run(&options.App{
		Title:            appmeta.AppName,
		Width:            1024,
		Height:           768,
		Frameless:        guiapp.DefaultFrameless(),
		WindowStartState: windowStartState,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Handler:    runtimeAssetHandler(themeManager, pluginCatalog),
			Middleware: runtimeAssetMiddleware(themeManager, pluginCatalog),
		},
		OnStartup: func(ctx context.Context) {
			guiapp.SetWailsContext(ctx)
			app.Startup(ctx)
		},
		OnBeforeClose: app.OnWindowClosing,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               appmeta.GUISingleInstanceID,
			OnSecondInstanceLaunch: guiapp.OnSecondInstanceLaunch,
		},
		BackgroundColour: &options.RGBA{R: bgR, G: bgG, B: bgB, A: bgA},
		Windows:          windowOptions,
		Bind: []any{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func runtimeAssetMiddleware(themeManager *theme.Manager, pluginCatalog *plugins.Catalog) assetserver.Middleware {
	themeHandler := themeAssetHandler(themeManager)
	pluginHandler := pluginAssetHandler(pluginCatalog)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, theme.AssetURLPrefix) {
				themeHandler.ServeHTTP(w, r)
				return
			}
			if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, pluginAssetURLPrefix) {
				pluginHandler.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func runtimeAssetHandler(themeManager *theme.Manager, pluginCatalog *plugins.Catalog) http.Handler {
	themeHandler := themeAssetHandler(themeManager)
	pluginHandler := pluginAssetHandler(pluginCatalog)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, theme.AssetURLPrefix):
			themeHandler.ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, pluginAssetURLPrefix):
			pluginHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

func themeAssetMiddleware(themeManager *theme.Manager) assetserver.Middleware {
	themeHandler := themeAssetHandler(themeManager)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, theme.AssetURLPrefix) {
				themeHandler.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func themeAssetHandler(themeManager *theme.Manager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if themeManager == nil || r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, theme.AssetURLPrefix) {
			http.NotFound(w, r)
			return
		}

		rest := strings.TrimPrefix(r.URL.Path, theme.AssetURLPrefix)
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			http.NotFound(w, r)
			return
		}

		asset, err := themeManager.ReadAsset(parts[0], parts[1])
		if err != nil {
			http.NotFound(w, r)
			return
		}

		contentType := mime.TypeByExtension(filepath.Ext(asset.Name))
		if contentType == "" && strings.EqualFold(filepath.Ext(asset.Name), ".webp") {
			contentType = "image/webp"
		}
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, asset.Name, asset.ModTime, bytes.NewReader(asset.Data))
	})
}

func pluginAssetHandler(catalog *plugins.Catalog) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if catalog == nil || r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, pluginAssetURLPrefix) {
			http.NotFound(w, r)
			return
		}

		rest := strings.TrimPrefix(r.URL.Path, pluginAssetURLPrefix)
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			http.NotFound(w, r)
			return
		}

		asset, err := catalog.ReadAsset(parts[0], parts[1])
		if err != nil {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", asset.ContentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if requestedVersion := strings.TrimSpace(r.URL.Query().Get("v")); requestedVersion != "" && requestedVersion == asset.Version {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		http.ServeContent(w, r, asset.Name, asset.ModTime, bytes.NewReader(asset.Data))
	})
}
