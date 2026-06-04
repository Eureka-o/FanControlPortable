//go:build !legacydevice

package device

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/types"
)

const (
	wifiOnlyModelName = appmeta.DeviceModelName
	bleReservedModel  = appmeta.DeviceModelName + "（蓝牙预留）"
)

// Manager is the default FanControlPortable device manager.
// It keeps only the WiFi HTTP transport in the normal build.
type Manager struct {
	isConnected     bool
	productID       uint16
	deviceType      string
	deviceTransport string
	wifiEndpoint    string
	wifiHTTPClient  *http.Client
	mutex           sync.RWMutex
	logger          types.Logger
	currentFanData  atomic.Pointer[types.FanData]

	onFanDataUpdate func(data *types.FanData)
	onDisconnect    func()

	debugMutex  sync.Mutex
	debugSeq    uint64
	debugFrames []types.DeviceDebugFrame
}

func NewManager(logger types.Logger) *Manager {
	return &Manager{
		logger:          logger,
		deviceTransport: types.DeviceTransportWiFi,
		wifiEndpoint:    types.DefaultFanDeviceIP,
		wifiHTTPClient:  newWiFiHTTPClient(),
	}
}

func (m *Manager) SetCallbacks(onFanDataUpdate func(data *types.FanData), onDisconnect func()) {
	m.onFanDataUpdate = onFanDataUpdate
	m.onDisconnect = onDisconnect
}

func (m *Manager) Init() error {
	return nil
}

func (m *Manager) Exit() error {
	return nil
}

func (m *Manager) Connect() (bool, map[string]string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.isConnected {
		return true, map[string]string{
			"manufacturer": "FanControlPortable",
			"product":      "FanControlPortable",
			"serial":       m.wifiEndpoint,
			"model":        wifiOnlyModelName,
			"transport":    types.DeviceTransportWiFi,
			"endpoint":     m.wifiEndpoint,
		}
	}

	if !m.shouldUseWiFiLocked() {
		m.logWarn("蓝牙连接方式已预留，当前版本尚未启用 BLE 协议")
		return false, map[string]string{
			"manufacturer": "FanControlPortable",
			"product":      "FanControlPortable",
			"serial":       "",
			"model":        bleReservedModel,
			"transport":    types.DeviceTransportBLE,
			"endpoint":     "",
		}
	}
	return m.connectWiFiLocked()
}

func (m *Manager) Disconnect() {
	m.disconnect(true)
}

func (m *Manager) DisconnectSilently() {
	m.disconnect(false)
}

func (m *Manager) disconnect(notify bool) {
	m.mutex.Lock()
	if !m.isConnected {
		m.mutex.Unlock()
		return
	}

	m.disconnectWiFiLocked()
	shouldNotify := notify && m.onDisconnect != nil
	m.mutex.Unlock()

	m.logInfo("网络风扇控制器连接已断开")
	if shouldNotify {
		m.onDisconnect()
	}
}

func (m *Manager) IsConnected() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.isConnected
}

func (m *Manager) GetProductID() uint16 {
	return 0
}

func (m *Manager) GetModelName() string {
	return wifiOnlyModelName
}

func (m *Manager) GetDeviceType() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.deviceType
}

func (m *Manager) IsBS1() bool {
	return false
}

func (m *Manager) GetCurrentFanData() *types.FanData {
	return m.currentFanData.Load()
}

func (m *Manager) SetFanSpeed(percent int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.deviceType != types.DeviceTransportWiFi || !m.shouldUseWiFiLocked() {
		return false
	}
	return m.setWiFiSpeedLocked(types.ClampFanPercent(percent))
}

func (m *Manager) SetCustomFanSpeed(percent int) bool {
	return m.SetFanSpeed(percent)
}

func (m *Manager) EnterAutoMode() error {
	if !m.IsConnected() {
		return fmt.Errorf("设备未连接")
	}
	return nil
}

func (m *Manager) SetManualGear(gear, level string) bool {
	fanData := m.GetCurrentFanData()
	if fanData != nil && fanData.SpeedUnit == types.FanSpeedUnitPercent && fanData.TargetRPM > 0 && fanData.TargetRPM <= types.FanSpeedMaxPercent {
		return m.SetFanSpeed(int(fanData.TargetRPM))
	}
	return m.SetFanSpeed(50)
}

func (m *Manager) SetManualGearRPM(gear, level string, rpm int) bool {
	return m.SetFanSpeed(rpm)
}

func (m *Manager) SetGearLight(enabled bool) bool {
	return true
}

func (m *Manager) SetPowerOnStart(enabled bool) bool {
	return true
}

func (m *Manager) SetSmartStartStop(mode string) bool {
	return true
}

func (m *Manager) SetBrightness(percentage int) bool {
	return true
}

func (m *Manager) SetLightStrip(cfg types.LightStripConfig) error {
	return nil
}

func (m *Manager) SetRGBOff() bool {
	return true
}

func (m *Manager) QueryDeviceSettings() (types.DeviceSettings, error) {
	settings := types.DeviceSettings{
		Available: m.IsConnected(),
		Source:    types.DeviceTransportWiFi,
		ReadAt:    time.Now().Format("2006-01-02 15:04:05"),
		Model:     wifiOnlyModelName,
	}

	fanData := m.GetCurrentFanData()
	if fanData != nil {
		settings.WorkMode = fanData.WorkMode
		settings.WorkModeName = fanData.WorkMode
		settings.Status = &types.DeviceStatusRead{
			ModeName:   fanData.WorkMode,
			CurrentRPM: int(fanData.CurrentRPM),
			TargetRPM:  int(fanData.TargetRPM),
		}
	}

	if !settings.Available {
		return settings, fmt.Errorf("设备未连接")
	}
	return settings, nil
}

func DebugCommandPresets() []types.DeviceDebugCommandPreset {
	return []types.DeviceDebugCommandPreset{}
}

func (m *Manager) GetDebugFrames() []types.DeviceDebugFrame {
	m.debugMutex.Lock()
	defer m.debugMutex.Unlock()
	frames := make([]types.DeviceDebugFrame, len(m.debugFrames))
	copy(frames, m.debugFrames)
	return frames
}

func (m *Manager) SendDebugCommand(input string, waitMs int) (types.DeviceDebugCommandResult, error) {
	return types.DeviceDebugCommandResult{
		Transport: types.DeviceTransportWiFi,
		InputHex:  input,
		WaitMs:    waitMs,
		Frames:    m.GetDebugFrames(),
	}, fmt.Errorf("网络风扇控制器不支持原始协议调试命令")
}

func (m *Manager) logInfo(format string, v ...any) {
	if m.logger != nil {
		m.logger.Info(format, v...)
	}
}

func (m *Manager) logError(format string, v ...any) {
	if m.logger != nil {
		m.logger.Error(format, v...)
	}
}

func (m *Manager) logWarn(format string, v ...any) {
	if m.logger != nil {
		m.logger.Warn(format, v...)
	}
}

func (m *Manager) logDebug(format string, v ...any) {
	if m.logger != nil {
		m.logger.Debug(format, v...)
	}
}
