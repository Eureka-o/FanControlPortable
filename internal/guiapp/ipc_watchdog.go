package guiapp

import (
	"time"
)

const ipcWatchdogInterval = 5 * time.Second

func (a *App) startIPCWatchdog() {
	stopCh := a.shutdownChan()
	var ctxDone <-chan struct{}
	if a.ctx != nil {
		ctxDone = a.ctx.Done()
	}

	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				guiLogger.Errorf("IPC 看护协程发生 panic: %v", recovered)
			}
		}()
		ticker := time.NewTicker(ipcWatchdogInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ctxDone:
				return
			case <-ticker.C:
				if a.shuttingDown.Load() {
					return
				}
				if a.ctx != nil && !a.ipcClient.IsConnected() {
					a.reconnectCore()
				}
				a.maybeRunPeriodicMemoryReclaim(time.Now())
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
	a.emit("core-resynced", nil)
}
