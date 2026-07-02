package coreapp

import (
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

const (
	wifiOverviewForegroundRefreshInterval  = 2 * time.Second
	wifiOverviewAutoControlRefreshInterval = 5 * time.Second
	wifiOverviewBackgroundRefreshInterval  = 15 * time.Second
)

func wifiOverviewRefreshInterval(hasClients, autoControl bool) time.Duration {
	if hasClients {
		return wifiOverviewForegroundRefreshInterval
	}
	if autoControl {
		return wifiOverviewAutoControlRefreshInterval
	}
	return wifiOverviewBackgroundRefreshInterval
}

func shouldRefreshWiFiOverviewState(deviceType string, now, lastRefresh time.Time, readThisTick bool, interval time.Duration) bool {
	if deviceType != types.DeviceTransportWiFi || readThisTick {
		return false
	}
	if interval <= 0 {
		interval = wifiOverviewForegroundRefreshInterval
	}
	return lastRefresh.IsZero() || now.Sub(lastRefresh) >= interval
}

func (a *CoreApp) refreshWiFiOverviewState(refreshRunning *atomic.Bool) {
	if !refreshRunning.CompareAndSwap(false, true) {
		return
	}

	a.safeGo("refreshWiFiOverviewState", func() {
		defer refreshRunning.Store(false)
		if !a.deviceManager.RefreshWiFiState() {
			a.logDebug("实时概览刷新 WiFi 控制器状态失败，等待健康检查处理连接状态")
		}
	})
}
