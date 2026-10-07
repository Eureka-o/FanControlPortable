package guiapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Eureka-o/FanControlPortable/internal/config"
	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/logger"
	"github.com/Eureka-o/FanControlPortable/internal/theme"
	"github.com/Eureka-o/FanControlPortable/internal/types"
	"go.uber.org/zap"
)

// App struct - GUI 应用程序结构
type App struct {
	ctx               context.Context
	ipcClient         *ipc.Client
	mutex             sync.RWMutex
	ipcReconnectMutex sync.Mutex
	updateMutex       sync.Mutex
	updateControl     *updateDownloadControl
	shuttingDown      atomic.Bool

	// 缓存的状态
	isConnected bool
	currentTemp types.TemperatureData

	// 自定义主题管理器（发现/播种/读取安装目录与用户目录下的主题）
	themeManager *theme.Manager
}

// 为了与前端 API 兼容，重新导出类型
type (
	FanCurvePoint               = types.FanCurvePoint
	FanCurveProfile             = types.FanCurveProfile
	FanCurveProfilesPayload     = types.FanCurveProfilesPayload
	DeviceProfile               = types.DeviceProfile
	DeviceProfilesPayload       = types.DeviceProfilesPayload
	DeviceProfileTestParams     = types.DeviceProfileTestParams
	DeviceProfileTestResult     = types.DeviceProfileTestResult
	SerialPortInfo              = types.SerialPortInfo
	BLEManufacturerData         = types.BLEManufacturerData
	BLEDeviceInfo               = types.BLEDeviceInfo
	BLEScanParams               = types.BLEScanParams
	BLEGATTProbeParams          = types.BLEGATTProbeParams
	BLEGATTCharacteristicInfo   = types.BLEGATTCharacteristicInfo
	BLEGATTServiceInfo          = types.BLEGATTServiceInfo
	BLEGATTProbeResult          = types.BLEGATTProbeResult
	WiFiDiscoveryParams         = types.WiFiDiscoveryParams
	WiFiDiscoveryScope          = types.WiFiDiscoveryScope
	WiFiDiscoveredDevice        = types.WiFiDiscoveredDevice
	WiFiDiscoveryResult         = types.WiFiDiscoveryResult
	FanData                     = types.FanData
	GearCommand                 = types.GearCommand
	TemperatureData             = types.TemperatureData
	TemperatureHistoryPoint     = types.TemperatureHistoryPoint
	TemperatureHistoryPayload   = types.TemperatureHistoryPayload
	BridgeTemperatureData       = types.BridgeTemperatureData
	DeviceDebugCommandResult    = types.DeviceDebugCommandResult
	DeviceDebugFrame            = types.DeviceDebugFrame
	DeviceDebugCommandPreset    = types.DeviceDebugCommandPreset
	DeviceSettings              = types.DeviceSettings
	DeviceGearRPM               = types.DeviceGearRPM
	DeviceStatusRead            = types.DeviceStatusRead
	AppConfig                   = types.AppConfig
	NoiseDiagnosticRange        = types.NoiseDiagnosticRange
	NoiseDiagnosticResult       = types.NoiseDiagnosticResult
	NoiseDiagnosticSession      = types.NoiseDiagnosticSession
	NoiseDiagnosticBeginRequest = types.NoiseDiagnosticBeginRequest
	NoiseDiagnosticTargetResult = types.NoiseDiagnosticTargetResult
)

var guiLogger *zap.SugaredLogger

// init 初始化 GUI 侧日志。
func init() {
	dir, err := guiLogDir()
	if err == nil {
		if custom, lerr := logger.NewPrefixedLogger(false, dir, "gui"); lerr == nil {
			// 用 GetDirectSugar 而不是 GetSugar：GUI 直接调 sugar.Infof，
			// 而 GetSugar 的 caller skip 是为包装方法准备的，直接用会让 caller 错一层。
			// 详见 logger.GetDirectSugar 的注释。
			guiLogger = custom.GetDirectSugar()
			return
		} else {
			err = lerr
		}
	}
	// 退回 stderr。这条 warn 在无控制台时也看不见，所以原因必须写进 err 里；
	// 而且这是"日志系统坏了"，属于必须让人尽快知道的故障。
	fallback, _ := zap.NewProduction()
	guiLogger = fallback.Sugar()
	guiLogger.Warnf("GUI 文件日志初始化失败（%v）：已退回 stderr，无控制台时 GUI 侧日志会丢失", err)
}

// guiLogDir 返回 GUI 日志应落的目录（与核心同一个 logs/ 目录，文件名不同）。
func guiLogDir() (string, error) {
	if dir := strings.TrimSpace(config.GetInstallDir()); dir != "" {
		return dir, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// ReportClientIssue 把前端上报的异常 / 导航 / 卸载事件转交给核心落日志。
func (a *App) ReportClientIssue(kind, message, detail string) {
	if a == nil {
		return
	}
	payload := ipc.ClientIssueReport{Kind: kind, Message: message, Detail: detail}
	if _, err := a.sendRequest(ipc.ReqReportClientIssue, payload); err != nil {
		// 核心不可达也要留下痕迹（GUI 日志的 caller 会指向这里）。
		guiLogger.Errorf("[前端] %s | %s | %s（上报核心失败: %v）", kind, message, detail, err)
	}
}
