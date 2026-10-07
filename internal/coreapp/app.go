package coreapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/autostart"
	"github.com/Eureka-o/FanControlPortable/internal/bridge"
	"github.com/Eureka-o/FanControlPortable/internal/config"
	"github.com/Eureka-o/FanControlPortable/internal/device"
	hotkeysvc "github.com/Eureka-o/FanControlPortable/internal/hotkey"
	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/logger"
	"github.com/Eureka-o/FanControlPortable/internal/notifier"
	"github.com/Eureka-o/FanControlPortable/internal/plugins"
	"github.com/Eureka-o/FanControlPortable/internal/scene"
	"github.com/Eureka-o/FanControlPortable/internal/smartcontrol"
	"github.com/Eureka-o/FanControlPortable/internal/temperature"
	"github.com/Eureka-o/FanControlPortable/internal/tray"
	"github.com/Eureka-o/FanControlPortable/internal/types"
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

	isConnected                      bool
	monitoringTemp                   atomic.Bool
	monitoringMutex                  sync.Mutex
	monitoringCancel                 context.CancelFunc
	monitoringDone                   chan struct{}
	monitoringStopping               bool
	stopping                         atomic.Bool
	currentTemp                      types.TemperatureData
	deviceSettings                   *types.DeviceSettings
	lastSuccessfulDeviceReadAt       time.Time
	lastDeviceMode                   string
	lastPublishedFanData             *types.FanData
	userSetAutoControl               bool
	isAutoStartLaunch                bool
	debugMode                        bool
	monitorOnly                      atomic.Bool
	sessionMonitorOnly               atomic.Bool
	legionFnQSupported               atomic.Bool
	legionFnQSupportChecked          atomic.Bool
	legionFnQRegistered              atomic.Bool
	reconnectInProgress              atomic.Bool
	connectionPhase                  atomic.Int32
	connectMutex                     sync.Mutex
	reconnectMutex                   sync.Mutex
	reconnectCancel                  context.CancelFunc
	reconnectWake                    chan struct{}
	reconnectGeneration              uint64
	autoReconnectSuppressed          atomic.Bool
	hasSuccessfulConnection          atomic.Bool
	lastConnectionWasNative          atomic.Bool
	resumeRecoveryRunning            atomic.Bool
	temperatureBridgeRecoveryRunning atomic.Bool
	systemSuspended                  atomic.Bool
	resumeReconnectWanted            atomic.Bool
	suspendGeneration                atomic.Uint64
	wifiStandbyApplied               atomic.Bool
	forceNextAutoTarget              atomic.Bool
	// reseedTempEMA 表示"温度 EMA 需要重播种"：设备独占任务（屏保图传）把采样循环饿住十几秒，
	// 窗口后的第一拍不能拿窗口前的 EMA 当上一个采样 —— 否则等于把十几秒的空洞当成一个采样步长。
	// 由图传登记表的 done() 置位，监控循环在下一拍消费。
	reseedTempEMA atomic.Bool
	// autoPushFailed 记录"上一拍异步下发的 Mission 失败了"，下一拍据此强制重试。
	autoPushFailed atomic.Bool
	// blackSharkSourceWritten 是最近一次写下去的冷却参照源（-1 = 未知；0 = CPU；1 = GPU）。
	// 它记的是写过的值，不是设备真值（真值以 `0x23` / `0x25` 为准，见 currentConfigSourceLocked）；
	// 存在的唯一目的 = "没变就一个字节都不发"（max 模式每拍都会重新评估，不节流会一直写 0x22）。
	blackSharkSourceWritten atomic.Int32
	// blackSharkSourceLastAt 是上次切换参照源的时刻（UnixNano），用于 max 的最小间隔。
	blackSharkSourceLastAt atomic.Int64
	// bulkDeviceOpEndUnix 是最近一次「设备独占批量操作」（屏保上传 / 读图）结束的 UnixNano。
	// 这类操作会持设备写锁几十秒、把温度监控循环饿住；靠它把那段停摆归因到本机，
	// 而不是误判成系统睡眠（误判会在上传进行中拆掉连接，让整轮上传连环失败）。
	bulkDeviceOpEndUnix           atomic.Int64
	// blackSharkImageTransfers 是"正在进行的黑鲨图传"登记表（key = 传输 id）。
	// 与上游同形：coreapp 侧持有登记与取消，device 层只管帧序列与进度（见
	// docs/blackshark-brb02-access-notes.md §4.3.2）。它同时是"当前有没有设备独占任务在飞"
	// 的唯一所有者 —— 健康检查与运行态都读它，不再各自维护判据。
	blackSharkImageTransferMu sync.Mutex
	blackSharkImageTransfers  map[uint64]context.CancelFunc
	lastResumeRecoveryUnix        int64
	lastHealthReconnectUnix       int64
	healthConsecutiveFailureCount int32
	connectionFlights             *connectionFlightRecorder
	smartControlDecisionMu        sync.RWMutex
	smartControlDecision          smartcontrol.Decision

	powerNotifyStop func()
	hidNotifyStop   func()
	bleNotifyStop   func()

	guiLastResponse   int64
	guiMonitorEnabled bool
	healthCheckTicker *time.Ticker
	// 系统信息推送（0x07）：给散热器 LCD 送主机指标，1Hz 节流

	// 黑鲨主机侧灯效驱动（槽位 6「响应」→ 键鼠钩子；槽位 7「音频同步」→ 音频采集）。
	// 只在该灯效生效时运行，详见 blackSharkHostFx。
	btHostFx *blackSharkHostFx

	// 情景（按前台进程自动切换档位/灯效）
	sceneTicker *time.Ticker
	sceneEngine scene.Engine
	// sceneRevision 记住上次判定所用的配置版本号。
	// 引擎内部记的是规则索引；规则表被改后同一个索引会指向另一条规则，
	// 引擎会误判成没有变化从而不做任何事，用 revision 变化强制 Reset。
	sceneRevision uint64
	cleanupChan   chan bool
	quitChan      chan bool

	mutex                 sync.RWMutex
	manualGearLevelMemory map[string]string

	noiseDiagnosticMu    sync.Mutex
	noiseDiagnosticLease *noiseDiagnosticLease
}

const (
	systemResumeDetectionFloor   = 20 * time.Second
	systemResumeDetectionCeiling = 45 * time.Second
	systemResumeRecoveryCooldown = 15 * time.Second
	systemResumeReconnectDelay   = 3 * time.Second
	suspendCleanupGrace          = 2 * time.Second
	pawnIORegistryPath           = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO`
)

func systemResumeDetectionThreshold(expectedInterval time.Duration) time.Duration {
	threshold := min(max(expectedInterval*6, systemResumeDetectionFloor), systemResumeDetectionCeiling)
	return threshold
}

func shouldRecoverFromSystemResumeGap(gap, expectedInterval time.Duration) bool {
	return gap >= systemResumeDetectionThreshold(expectedInterval)
}

// noteBulkDeviceOperation 记录一次「设备独占批量操作」（屏保上传 / 读图）刚刚结束。
// 调用方是唯一知道这次操作何时收手的人；监控循环只消费这个时刻。
func (a *CoreApp) noteBulkDeviceOperation() {
	a.bulkDeviceOpEndUnix.Store(time.Now().UnixNano())
}

// selfInflictedMonitorStall 报告这段"够长到会被当成系统唤醒"的 gap，是不是本机自己造成的
// —— 屏保上传/读图全程持设备写锁（`m.mutex`），监控循环会卡在那把锁上几十秒。
//
// 必须区分：设备会在 4KB 边界停发 `0xC6`，一次上传可能要重传几次、耗时远超 20s 的
// 唤醒阈值；不排除它，`maybeRecoverFromSystemResume` 就会按"系统唤醒"处理，
// 在上传进行中把连接拆掉，随后 `0xC4`/`0xC5` 连环失败。
func (a *CoreApp) selfInflictedMonitorStall(gap, expectedInterval time.Duration) bool {
	if !shouldRecoverFromSystemResumeGap(gap, expectedInterval) {
		return false
	}
	end := a.bulkDeviceOpEndUnix.Load()
	return end != 0 && end >= time.Now().Add(-gap).UnixNano()
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
	historyRetentionHours := types.NormalizeTemperatureHistoryRetentionHours(configMgr.Get().HistoryRetentionHours)
	historyPointsPerHour := int(time.Hour / temperature.DefaultHistorySampleInterval)
	tempHistory := temperature.NewHistoryRecorder(historyPath, historyPointsPerHour*historyRetentionHours, temperature.DefaultHistorySampleInterval, customLogger)
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
		connectionFlights:  newConnectionFlightRecorder(defaultConnectionFlightCapacity, nil),
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
