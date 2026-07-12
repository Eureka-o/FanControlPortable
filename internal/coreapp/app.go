package coreapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/autostart"
	"github.com/TIANLI0/THRM/internal/bridge"
	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/device"
	hotkeysvc "github.com/TIANLI0/THRM/internal/hotkey"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/logger"
	"github.com/TIANLI0/THRM/internal/notifier"
	"github.com/TIANLI0/THRM/internal/plugins"
	"github.com/TIANLI0/THRM/internal/temperature"
	"github.com/TIANLI0/THRM/internal/tray"
	"github.com/TIANLI0/THRM/internal/types"
)

// CoreApp 核心应用结构
type CoreApp struct {
	ctx context.Context

	deviceManager    *device.Manager
	bridgeManager    *bridge.Manager
	tempReader       *temperature.Reader
	tempHistory      *temperature.HistoryRecorder
	configManager    *config.Manager
	trayManager      *tray.Manager
	hotkeyManager    *hotkeysvc.Manager
	notifier         *notifier.Manager
	autostartManager *autostart.Manager
	pluginManager    *plugins.Manager
	logger           *logger.CustomLogger
	ipcServer        *ipc.Server
	wifiScanControl  *types.WiFiDiscoveryControl
	wifiScanRunning  atomic.Bool

	isConnected                   bool
	monitoringTemp                atomic.Bool
	monitoringMutex               sync.Mutex
	monitoringCancel              context.CancelFunc
	monitoringDone                chan struct{}
	monitoringStopping            bool
	stopping                      atomic.Bool
	currentTemp                   types.TemperatureData
	deviceSettings                *types.DeviceSettings
	lastDeviceMode                string
	userSetAutoControl            bool
	isAutoStartLaunch             bool
	debugMode                     bool
	legionFnQSupported            atomic.Bool
	legionFnQSupportChecked       atomic.Bool
	legionFnQRegistered           atomic.Bool
	reconnectInProgress           atomic.Bool
	connectMutex                  sync.Mutex
	reconnectMutex                sync.Mutex
	reconnectCancel               context.CancelFunc
	reconnectGeneration           uint64
	autoReconnectSuppressed       atomic.Bool
	hasSuccessfulConnection       atomic.Bool
	lastConnectionWasNative       atomic.Bool
	resumeRecoveryRunning         atomic.Bool
	systemSuspended               atomic.Bool
	wifiStandbyApplied            atomic.Bool
	forceNextAutoTarget           atomic.Bool
	lastResumeRecoveryUnix        int64
	lastHealthReconnectUnix       int64
	healthConsecutiveFailureCount int32

	powerNotifyStop func()

	guiLastResponse   int64
	guiMonitorEnabled bool
	healthCheckTicker *time.Ticker
	cleanupChan       chan bool
	quitChan          chan bool

	mutex                 sync.RWMutex
	manualGearLevelMemory map[string]string
}

const (
	systemResumeDetectionFloor   = 20 * time.Second
	systemResumeDetectionCeiling = 45 * time.Second
	systemResumeRecoveryCooldown = 15 * time.Second
	systemResumeReconnectDelay   = 3 * time.Second
	suspendCleanupGrace          = 2 * time.Second
	pawnIOInstallerTimeout       = 90 * time.Second
	pawnIOAlreadyExistsExitCode  = 183
	pawnIORegistryPath           = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO`
)

func systemResumeDetectionThreshold(expectedInterval time.Duration) time.Duration {
	threshold := min(max(expectedInterval*6, systemResumeDetectionFloor), systemResumeDetectionCeiling)
	return threshold
}

func shouldRecoverFromSystemResumeGap(gap, expectedInterval time.Duration) bool {
	return gap >= systemResumeDetectionThreshold(expectedInterval)
}

func copyFileIfMissing(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// NewCoreApp 创建核心应用实例。
func NewCoreApp(debugMode, isAutoStart bool, iconData []byte) *CoreApp {
	installDir := config.GetInstallDir()
	customLogger, err := logger.NewCustomLogger(debugMode, installDir)
	if err != nil {
		panic(fmt.Sprintf("初始化日志系统失败: %v", err))
	} else {
		customLogger.Info("核心服务启动")
		customLogger.Info("安装目录: %s", installDir)
		customLogger.Info("调试模式: %v", debugMode)
		customLogger.Info("自启动模式: %v", isAutoStart)
		customLogger.CleanOldLogs()
	}

	bridgeMgr := bridge.NewManager(customLogger)
	deviceMgr := device.NewManager(customLogger)
	tempReader := temperature.NewReader(bridgeMgr, customLogger)
	configMgr := config.NewManager(installDir, customLogger)
	historyPath := filepath.Join(installDir, temperature.DefaultHistoryRelativePath)
	if _, err := os.Stat(historyPath); err != nil && os.IsNotExist(err) {
		legacyHistoryPath := filepath.Join(installDir, temperature.LegacyHistoryRelativePath)
		if _, legacyErr := os.Stat(legacyHistoryPath); legacyErr == nil {
			if copyErr := copyFileIfMissing(legacyHistoryPath, historyPath); copyErr != nil {
				customLogger.Error("迁移历史数据文件失败，将继续使用旧路径: %v", copyErr)
				historyPath = legacyHistoryPath
			}
		}
	}
	tempHistory := temperature.NewHistoryRecorder(historyPath, temperature.DefaultHistoryCapacity, temperature.DefaultHistorySampleInterval, customLogger)
	trayMgr := tray.NewManager(customLogger, iconData)
	autostartMgr := autostart.NewManager(customLogger)
	pluginMgr := plugins.NewManager(customLogger)

	app := &CoreApp{
		ctx:                context.Background(),
		deviceManager:      deviceMgr,
		bridgeManager:      bridgeMgr,
		tempReader:         tempReader,
		tempHistory:        tempHistory,
		currentTemp:        types.TemperatureData{BridgeOk: true},
		configManager:      configMgr,
		trayManager:        trayMgr,
		autostartManager:   autostartMgr,
		pluginManager:      pluginMgr,
		logger:             customLogger,
		wifiScanControl:    types.NewWiFiDiscoveryControl(),
		isConnected:        false,
		lastDeviceMode:     "",
		userSetAutoControl: false,
		isAutoStartLaunch:  isAutoStart,
		debugMode:          debugMode,
		guiLastResponse:    time.Now().Unix(),
		cleanupChan:        make(chan bool, 1),
		quitChan:           make(chan bool, 1),
		guiMonitorEnabled:  true,
		manualGearLevelMemory: map[string]string{
			"静音": "中",
			"标准": "中",
			"强劲": "中",
			"超频": "中",
		},
	}
	app.notifier = notifier.NewManager(customLogger, iconData)
	app.hotkeyManager = hotkeysvc.NewManager(customLogger, app.handleHotkeyAction)

	return app
}
