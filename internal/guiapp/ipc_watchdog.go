package guiapp

import (
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const ipcWatchdogInterval = 5 * time.Second

func (a *App) startIPCWatchdog() {
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				guiLogger.Errorf("IPC 看护协程发生 panic: %v", recovered)
			}
		}()
		ticker := time.NewTicker(ipcWatchdogInterval)
		defer ticker.Stop()
		for range ticker.C {
			if a.shuttingDown.Load() {
				return
			}
			if a.ctx != nil && !a.ipcClient.IsConnected() {
				a.reconnectCore()
			}
		}
	}()
}

func (a *App) reconnectCore() {
	if a.shuttingDown.Load() {
		return
	}
	if err := a.ensureIPCConnected(); err != nil {
		guiLogger.Warnf("IPC 看护重连失败: %v", err)
		a.emitCoreServiceError(err.Error())
		return
	}
	guiLogger.Info("IPC 看护重连成功，请求前端重新同步核心状态")
	a.emitCoreServiceOK()
	a.emitCoreResynced()
}

func (a *App) emitCoreResynced() {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "core-resynced", nil)
	}
}
