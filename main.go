package main

import (
	"context"
	"embed"
	"os"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/guiapp"
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

	app := NewApp()

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
			Assets: assets,
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
		Windows: &windows.Options{
			// 窗口背景本身是不透明的，开启半透明会让 WebView2 表面带 alpha 通道，
			// 从而关闭 ClearType 次像素抗锯齿，在 1080p 等非整数 DPI 缩放下字体会发虚发糊。
			// 关闭半透明以恢复清晰的字体渲染。
			WindowIsTranslucent: false,
		},
		Bind: []any{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
