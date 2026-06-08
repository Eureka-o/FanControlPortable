//go:build !legacydevice

package device

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/deviceprofileexec"
	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

const (
	wifiOnlyModelName = appmeta.DeviceModelName
	bleReservedModel  = appmeta.DeviceModelName + "（蓝牙预留）"
)

// Manager is the default FanControl device manager.
// It keeps WiFi and profile-driven serial transports in the normal build.
type Manager struct {
	isConnected     bool
	productID       uint16
	deviceType      string
	deviceTransport string
	wifiEndpoint    string
	wifiHTTPClient  *http.Client
	activeProfile   types.DeviceProfile
	wifiExecutor    *deviceprofileexec.WiFiExecutor
	bleConnector    deviceprofileexec.BLEConnector
	bleExecutor     *deviceprofileexec.BLEExecutor
	serialDialer    deviceprofileexec.SerialDialer
	serialExecutor  *deviceprofileexec.SerialExecutor
	flyDigiHID      *flyDigiHIDDevice
	mutex           sync.RWMutex
	logger          types.Logger
	currentFanData  atomic.Pointer[types.FanData]

	onFanDataUpdate func(data *types.FanData)
	onDisconnect    func()

	lightCmdBuf [65]byte

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
		activeProfile:   types.DefaultWiFiPercentProfile(types.DefaultFanDeviceIP),
		bleConnector:    deviceprofileexec.DefaultBLEConnector{},
		serialDialer:    deviceprofileexec.DefaultSerialDialer{},
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
		return true, m.connectedInfoLocked()
	}

	if m.shouldUseWiFiLocked() {
		return m.connectWiFiLocked()
	}
	if m.shouldUseSerialLocked() {
		return m.connectSerialLocked()
	}
	if m.shouldUseBLELocked() {
		return m.connectBLELocked()
	}
	if m.shouldUseLegacyHIDLocked() {
		return m.connectLegacyHIDLocked()
	}

	if !m.shouldUseWiFiLocked() {
		m.logWarn("蓝牙连接方式已预留，当前版本尚未启用 BLE 协议")
		return false, map[string]string{
			"manufacturer": "FanControl",
			"product":      "FanControl",
			"serial":       "",
			"model":        bleReservedModel,
			"transport":    m.deviceTransport,
			"endpoint":     "",
		}
	}
	return false, nil
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

	if m.deviceType == types.DeviceTransportBLE {
		m.disconnectBLELocked()
	} else if m.deviceType == types.DeviceTransportSerial {
		m.disconnectSerialLocked()
	} else if m.deviceType == types.DeviceTransportHID {
		m.disconnectFlyDigiHIDLocked()
	} else {
		m.disconnectWiFiLocked()
	}
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
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.productID
}

func (m *Manager) GetModelName() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.deviceType == types.DeviceTransportHID {
		return flyDigiHIDModelName(m.productID)
	}
	if m.deviceType == types.DeviceTransportBLE && m.activeProfile.ID == types.FlyDigiBS1ProfileID {
		return "BS1"
	}
	return m.activeProfileDisplayNameLocked(wifiOnlyModelName)
}

func (m *Manager) GetDeviceType() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.deviceType
}

func (m *Manager) IsBS1() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.deviceType == types.DeviceTransportBLE && m.activeProfile.ID == types.FlyDigiBS1ProfileID
}

func (m *Manager) GetCurrentFanData() *types.FanData {
	return m.currentFanData.Load()
}

func (m *Manager) SetPercentSpeed(percent int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected {
		return false
	}
	switch m.deviceType {
	case types.DeviceTransportWiFi:
		if !m.shouldUseWiFiLocked() {
			return false
		}
		if types.IsRPMSpeedUnit(m.activeProfile.SpeedUnit) {
			m.logWarn("percent speed command rejected because the active WiFi profile uses RPM")
			return false
		}
		return m.setWiFiSpeedLocked(types.ClampFanPercent(percent))
	case types.DeviceTransportSerial:
		if types.IsRPMSpeedUnit(m.activeProfile.SpeedUnit) {
			m.logWarn("percent speed command rejected because the active serial profile uses RPM")
			return false
		}
		return m.setSerialTargetSpeedLocked(types.NewPercentSpeed(percent))
	case types.DeviceTransportBLE:
		if types.IsRPMSpeedUnit(m.activeProfile.SpeedUnit) {
			m.logWarn("percent speed command rejected because the active BLE profile uses RPM")
			return false
		}
		return m.setBLETargetSpeedLocked(types.NewPercentSpeed(percent))
	default:
		return false
	}
}

func (m *Manager) SetTargetSpeed(value int, unit string) bool {
	unit = types.NormalizeFanSpeedUnit(unit)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected {
		return false
	}
	if types.IsRPMSpeedUnit(unit) {
		if m.deviceType == types.DeviceTransportHID {
			return m.setFlyDigiHIDTargetSpeedLocked(types.NewRPMSpeed(value))
		}
		if m.deviceType == types.DeviceTransportWiFi {
			if !m.shouldUseWiFiLocked() || !types.IsRPMSpeedUnit(m.activeProfile.SpeedUnit) {
				m.logWarn("default WiFi percent profile does not support direct RPM target speed: %d", value)
				return false
			}
			return m.setWiFiTargetSpeedLocked(types.NewRPMSpeed(value))
		}
		if m.deviceType == types.DeviceTransportSerial {
			return m.setSerialTargetSpeedLocked(types.NewRPMSpeed(value))
		}
		if m.deviceType == types.DeviceTransportBLE {
			return m.setBLETargetSpeedLocked(types.NewRPMSpeed(value))
		}
		return false
	}
	if m.deviceType == types.DeviceTransportWiFi {
		if !m.shouldUseWiFiLocked() {
			return false
		}
		return m.setWiFiTargetSpeedLocked(types.NewPercentTickSpeed(value))
	}
	if m.deviceType == types.DeviceTransportSerial {
		return m.setSerialTargetSpeedLocked(types.NewPercentTickSpeed(value))
	}
	if m.deviceType == types.DeviceTransportBLE {
		return m.setBLETargetSpeedLocked(types.NewPercentTickSpeed(value))
	}
	return false
}

func (m *Manager) SetFanSpeed(percent int) bool {
	return m.SetPercentSpeed(percent)
}

func (m *Manager) SetCustomFanSpeed(percent int) bool {
	return m.SetPercentSpeed(percent)
}

func (m *Manager) EnterAutoMode() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected {
		return fmt.Errorf("设备未连接")
	}
	if m.deviceType == types.DeviceTransportHID {
		return m.writeFlyDigiHIDFrameLocked(deviceproto.CmdEnterRealtimeRPM, nil, hidControlReportLen)
	}
	return nil
}

func (m *Manager) SetManualGear(gear, level string) bool {
	fanData := m.GetCurrentFanData()
	if fanData != nil && fanData.SpeedUnit == types.FanSpeedUnitPercent && fanData.TargetRPM > 0 && fanData.TargetRPM <= types.FanSpeedMaxPercent {
		return m.SetPercentSpeed(int(fanData.TargetRPM))
	}
	return m.SetPercentSpeed(50)
}

func (m *Manager) SetManualGearRPM(gear, level string, rpm int) bool {
	m.mutex.RLock()
	unit := types.NormalizeFanSpeedUnit(m.activeProfile.SpeedUnit)
	m.mutex.RUnlock()
	if m.GetDeviceType() == types.DeviceTransportHID {
		return m.setFlyDigiHIDManualGearRPM(gear, level, rpm)
	}
	if types.IsRPMSpeedUnit(unit) {
		return m.SetTargetSpeed(rpm, unit)
	}
	return m.SetPercentSpeed(rpm)
}

func (m *Manager) SetGearLight(enabled bool) bool {
	if m.GetDeviceType() == types.DeviceTransportHID {
		return m.setFlyDigiHIDGearLight(enabled)
	}
	return false
}

func (m *Manager) SetPowerOnStart(enabled bool) bool {
	if m.IsBS1() {
		return m.setFlyDigiBS1PowerOnStart(enabled)
	}
	if m.GetDeviceType() == types.DeviceTransportHID {
		return m.setFlyDigiHIDPowerOnStart(enabled)
	}
	return false
}

func (m *Manager) SetSmartStartStop(mode string) bool {
	if m.GetDeviceType() == types.DeviceTransportHID {
		return m.setFlyDigiHIDSmartStartStop(mode)
	}
	return false
}

func (m *Manager) SetBrightness(percentage int) bool {
	if m.GetDeviceType() == types.DeviceTransportHID {
		return m.setFlyDigiHIDBrightness(percentage)
	}
	return false
}

func (m *Manager) SetLightStrip(cfg types.LightStripConfig) error {
	if m.GetDeviceType() == types.DeviceTransportHID {
		return m.setFlyDigiHIDLightStrip(cfg)
	}
	return fmt.Errorf("active device does not support lighting")
}

func (m *Manager) SetRGBOff() bool {
	if m.GetDeviceType() == types.DeviceTransportHID {
		return m.setFlyDigiHIDRGBOff()
	}
	return false
}

func (m *Manager) QueryDeviceSettings() (types.DeviceSettings, error) {
	m.mutex.RLock()
	available := m.isConnected
	source := m.deviceType
	if source == "" {
		source = m.deviceTransport
	}
	model := m.activeProfileDisplayNameLocked(wifiOnlyModelName)
	m.mutex.RUnlock()

	if source == types.DeviceTransportHID {
		return m.queryFlyDigiHIDDeviceSettings()
	}

	settings := types.DeviceSettings{
		Available: available,
		Source:    source,
		ReadAt:    time.Now().Format("2006-01-02 15:04:05"),
		Model:     model,
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

func (m *Manager) activeProfileDisplayNameLocked(fallback string) string {
	displayName := strings.TrimSpace(m.activeProfile.DisplayName)
	if displayName != "" {
		return displayName
	}
	model := strings.TrimSpace(m.activeProfile.Model)
	if model != "" {
		return model
	}
	fallback = strings.TrimSpace(fallback)
	if fallback != "" {
		return fallback
	}
	return appmeta.DeviceModelName
}

func (m *Manager) activeProfileVendorLocked() string {
	vendor := strings.TrimSpace(m.activeProfile.Vendor)
	if vendor != "" {
		return vendor
	}
	return "FanControl"
}

func DebugCommandPresets() []types.DeviceDebugCommandPreset {
	return []types.DeviceDebugCommandPreset{}
}

func (m *Manager) currentDebugSeq() uint64 {
	m.debugMutex.Lock()
	defer m.debugMutex.Unlock()
	return m.debugSeq
}

func (m *Manager) recordDebugFrame(direction, transport string, raw []byte) uint64 {
	debugFrame := newDeviceDebugFrame(direction, transport, raw)
	m.debugMutex.Lock()
	defer m.debugMutex.Unlock()
	return appendBoundedDebugFrame(&m.debugSeq, &m.debugFrames, debugFrame)
}

func (m *Manager) debugFramesAfter(seq uint64) []types.DeviceDebugFrame {
	m.debugMutex.Lock()
	defer m.debugMutex.Unlock()
	return debugFramesAfterSeq(m.debugFrames, seq)
}

func (m *Manager) GetDebugFrames() []types.DeviceDebugFrame {
	m.debugMutex.Lock()
	defer m.debugMutex.Unlock()
	return cloneDebugFrames(m.debugFrames)
}

func (m *Manager) SendDebugCommand(input string, waitMs int) (types.DeviceDebugCommandResult, error) {
	if waitMs < 0 {
		waitMs = 0
	}
	if waitMs > 5000 {
		waitMs = 5000
	}

	if m.GetDeviceType() == types.DeviceTransportHID {
		return m.sendFlyDigiHIDDebugCommand(input, waitMs)
	}

	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return types.DeviceDebugCommandResult{}, fmt.Errorf("raw debug command is empty")
	}

	raw := []byte(trimmed)
	rawHex := trimmed
	if frame, err := deviceproto.NormalizeDebugInput(trimmed); err == nil {
		raw = frame
		rawHex = deviceproto.Hex(frame)
	}

	startSeq := m.currentDebugSeq()
	m.recordDebugFrame("tx", types.DeviceTransportWiFi, raw)

	return types.DeviceDebugCommandResult{
		Transport: types.DeviceTransportWiFi,
		InputHex:  input,
		RawHex:    rawHex,
		WaitMs:    waitMs,
		Frames:    m.debugFramesAfter(startSeq),
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
