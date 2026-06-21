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
	"github.com/TIANLI0/THRM/internal/guiapp"
	"github.com/TIANLI0/THRM/internal/theme"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if !guiapp.EnsureCoreServiceRunning() {
		println("警告：无法启动核心服务，GUI 将以有限功能模式运行")
	}

	themeManager := newThemeManager()
	app := NewAppWithThemeManager(themeManager)

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
			Handler:    themeAssetHandler(themeManager),
			Middleware: themeAssetMiddleware(themeManager),
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
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.Mica,
		},
		Bind: []any{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
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
