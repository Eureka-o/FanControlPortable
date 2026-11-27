package main

import (
	"bytes"
	"context"
	"embed"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/Eureka-o/FanControlPortable/internal/appmeta"
	"github.com/Eureka-o/FanControlPortable/internal/config"
	"github.com/Eureka-o/FanControlPortable/internal/guiapp"
	"github.com/Eureka-o/FanControlPortable/internal/plugins"
	"github.com/Eureka-o/FanControlPortable/internal/theme"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	debug.SetGCPercent(100)
	debug.SetMemoryLimit(80 << 20)
	if guiapp.RunRelaunchHelper() {
		return
	}
	for _, arg := range os.Args {
		if arg == "--monitor-only-session" {
			guiapp.SetMonitorOnlySession(true)
			break
		}
	}
	themeManager := newThemeManager()
	app := NewAppWithThemeManager(themeManager)
	pluginsDir := filepath.Join(config.GetInstallDir(), "plugins")
	windowOptions := guiapp.ResolveWindowsOptions()
	bgR, bgG, bgB, bgA := guiapp.WindowBackgroundColour()

	windowStartState := options.Normal
	for _, arg := range os.Args {
		if arg == "--autostart" || arg == "/autostart" || arg == "-autostart" {
			windowStartState = options.Minimised
			break
		}
	}
	if windowStartState == options.Minimised {
		app.SetInitialWindowHidden(true)
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
			Handler:    customAssetHandler(themeManager, pluginsDir, app.CachedPluginDevDirs),
			Middleware: customAssetMiddleware(themeManager, pluginsDir, app.CachedPluginDevDirs),
		},
		OnStartup: func(ctx context.Context) {
			guiapp.SetWailsContext(ctx)
			guiapp.SetWailsApp(app.App)
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

func customAssetMiddleware(themeManager *theme.Manager, pluginsDir string, devDirs func() []string) assetserver.Middleware {
	themeHandler := themeAssetHandler(themeManager)
	pluginHandler := pluginAssetHandler(pluginsDir, devDirs)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				if strings.HasPrefix(r.URL.Path, theme.AssetURLPrefix) {
					themeHandler.ServeHTTP(w, r)
					return
				}
				if strings.HasPrefix(r.URL.Path, plugins.AssetURLPrefix) {
					pluginHandler.ServeHTTP(w, r)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func customAssetHandler(themeManager *theme.Manager, pluginsDir string, devDirs func() []string) http.Handler {
	themeHandler := themeAssetHandler(themeManager)
	pluginHandler := pluginAssetHandler(pluginsDir, devDirs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, plugins.AssetURLPrefix) {
			pluginHandler.ServeHTTP(w, r)
			return
		}
		themeHandler.ServeHTTP(w, r)
	})
}

func pluginAssetHandler(pluginsDir string, devDirs func() []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, plugins.AssetURLPrefix) {
			http.NotFound(w, r)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, plugins.AssetURLPrefix)
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			http.NotFound(w, r)
			return
		}
		var devList []string
		if devDirs != nil {
			devList = devDirs()
		}
		asset, err := plugins.ReadPluginAsset(pluginsDir, devList, parts[0], parts[1])
		if err != nil {
			http.NotFound(w, r)
			return
		}

		ext := strings.ToLower(filepath.Ext(asset.Name))
		contentType := mime.TypeByExtension(ext)
		switch ext {
		case ".js", ".mjs":
			contentType = "application/javascript; charset=utf-8"
		case ".css":
			contentType = "text/css; charset=utf-8"
		case ".webp":
			if contentType == "" {
				contentType = "image/webp"
			}
		case ".svg":
			if contentType == "" {
				contentType = "image/svg+xml"
			}
		case ".json":
			if contentType == "" {
				contentType = "application/json; charset=utf-8"
			}
		}
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		if strings.TrimSpace(r.URL.Query().Get("v")) != "" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		http.ServeContent(w, r, asset.Name, asset.ModTime, bytes.NewReader(asset.Data))
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
		if strings.TrimSpace(r.URL.Query().Get("v")) != "" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		http.ServeContent(w, r, asset.Name, asset.ModTime, bytes.NewReader(asset.Data))
	})
}
