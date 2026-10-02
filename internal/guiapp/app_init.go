package guiapp

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/appmeta"
	"github.com/Eureka-o/FanControlPortable/internal/config"
	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/theme"
	"github.com/Eureka-o/FanControlPortable/internal/types"
	"github.com/Eureka-o/FanControlPortable/internal/version"
)

// New 创建 GUI 应用实例
func New(themeManager *theme.Manager) *App {
	if themeManager != nil {
		themeManager.EnsureSeeded()
	}
	return &App{
		ipcClient:    ipc.NewClient(nil),
		currentTemp:  types.TemperatureData{BridgeOk: true},
		themeManager: themeManager,
	}
}

// Startup 应用启动时调用
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	if err := restoreWindowState(ctx); err != nil {
		guiLogger.Warnf("restore window state failed: %v", err)
	}

	guiLogger.Infof("=== %s GUI 启动 ===", appmeta.AppName)
	if err := cleanupStaleUpdateArtifacts(
		filepath.Join(os.TempDir(), "FanControl-update"),
		updateDownloadDirectory(config.GetInstallDir()),
		time.Now().Add(-7*24*time.Hour),
	); err != nil {
		guiLogger.Warnf("clean stale update files failed: %v", err)
	}
	a.startIPCWatchdog()

	if err := a.ensureIPCConnected(); err != nil {
		guiLogger.Error("核心服务不可用，GUI 将进入受限状态")
		a.emitCoreServiceError(err.Error())
		guiLogger.Infof("=== %s GUI 启动完成 ===", appmeta.AppName)
		return
	}

	guiLogger.Info("已连接到核心服务")
	if resp, pingErr := a.sendRequestWithTimeout(ipc.ReqPing, nil, 3*time.Second); pingErr != nil || resp == nil || !resp.Success {
		if pingErr != nil {
			guiLogger.Errorf("核心服务 Ping 失败: %v", pingErr)
			a.emitCoreServiceError(pingErr.Error())
		} else {
			guiLogger.Errorf("核心服务 Ping 返回异常: %+v", resp)
			a.emitCoreServiceError("核心服务 Ping 返回异常")
		}
	} else {
		a.emitCoreServiceOK()
	}

	guiLogger.Infof("=== %s GUI 启动完成 ===", appmeta.AppName)
}

// GetAppVersion 返回应用版本号（来自版本模块）
func (a *App) GetAppVersion() string {
	return version.Get()
}

// OnWindowClosing 窗口关闭事件处理
func (a *App) OnWindowClosing(ctx context.Context) bool {
	if err := saveCurrentWindowState(ctx); err != nil {
		guiLogger.Warnf("save window state failed: %v", err)
	}
	return false
}
