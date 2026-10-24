//go:build !legacydevice

package device

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/appmeta"
	"github.com/Eureka-o/FanControlPortable/internal/deviceprofileexec"
	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

const (
	wifiOnlyModelName = appmeta.DeviceModelName
	bleReservedModel  = appmeta.DeviceModelName + "（蓝牙预留）"
)

// Manager is the default FanControl device manager.
// It keeps WiFi and profile-driven serial transports in the normal build.
type Manager struct {
	isConnected       bool
	productID         uint16
	deviceType        string
	deviceTransport   string
	wifiEndpoint      string
	wifiHTTPClient    *http.Client
	wifiProtocol      string
	wifiConfig        bool
	wifiHeartbeat     bool
	wifiHbFailures    int
	wifiHbStop        chan struct{}
	wifiHbDone        chan struct{}
	activeProfile     types.DeviceProfile
	wifiExecutor      *deviceprofileexec.WiFiExecutor
	bleConnector      deviceprofileexec.BLEConnector
	bleExecutor       *deviceprofileexec.BLEExecutor
	serialDialer      deviceprofileexec.SerialDialer
	serialExecutor    *deviceprofileexec.SerialExecutor
	flyDigiHID        *flyDigiHIDDevice
	flyDigiHIDStop    chan struct{}
	flyDigiHIDDone    chan struct{}
	mutex             sync.RWMutex
	operationControl  deviceOperationControl
	logger            types.Logger
	currentFanData    atomic.Pointer[types.FanData]
	writesBlocked     atomic.Bool
	connectionGen     atomic.Uint64
	lastAutoBLEScanAt time.Time
	// blackSharkImageACK 在图传期间由读循环转发设备应答(0xC4/0xC6/0xC5)。
	// 读循环是 IN 端点的唯一读者,用原子指针在互斥锁外交接,避免与 7ms 写节拍争锁。
	blackSharkImageACK atomic.Pointer[blackSharkImageACKStream]
	// blackSharkHostInfoFrame 是 coreapp 喂入的最新 0x07 参数页快照;
	// 参数刷新循环只重复发送这份快照，不改变传感器采样频率。
	blackSharkHostInfoFrame       atomic.Pointer[[]byte]
	blackSharkHostInfoRefreshStop chan struct{}

	onFanDataUpdate func(data *types.FanData)
	onDisconnect    func()

	lightCmdBuf [65]byte

	debugMutex  sync.Mutex
	queryMutex  sync.Mutex
	debugSeq    uint64
	debugFrames []types.DeviceDebugFrame
}

func NewManager(logger types.Logger) *Manager {
	return &Manager{
		logger:          logger,
		deviceTransport: "",
		wifiEndpoint:    types.DefaultFanDeviceIP,
		wifiHTTPClient:  newWiFiHTTPClient(),
		activeProfile:   types.DeviceProfile{},
		bleConnector:    deviceprofileexec.DefaultBLEConnector{},
		serialDialer:    deviceprofileexec.DefaultSerialDialer{},
	}
}

func (m *Manager) SetCallbacks(onFanDataUpdate func(data *types.FanData), onDisconnect func()) {
	m.onFanDataUpdate = onFanDataUpdate
	m.onDisconnect = onDisconnect
}

func (m *Manager) Init() error {
	return initFlyDigiHIDAPI()
}

func (m *Manager) Exit() error {
	return exitFlyDigiHIDAPI()
}

func (m *Manager) Connect() (bool, map[string]string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.writesBlocked.Load() {
		return false, nil
	}
	if m.isConnected {
		if types.IsNativeDeviceTransport(m.deviceType) {
			return false, nil
		}
		return true, m.connectedInfoLocked()
	}
	ctx, finishOperation := m.operationControl.begin()
	defer finishOperation()
	if m.writesBlocked.Load() {
		return false, nil
	}

	if connected, info, handled := (compatibilityRuntime{}).connectLocked(ctx, m); handled {
		return connected, info
	}
	if m.shouldUseBLELocked() {
		return m.connectBLEWithContextLocked(ctx)
	}
	if m.shouldUseLegacyHIDLocked() {
		return m.connectLegacyHIDLocked()
	}
	if m.shouldUseBlackSharkUSBLocked() {
		return m.connectBlackSharkUSBLocked()
	}
	if strings.TrimSpace(m.deviceTransport) == "" {
		return false, nil
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

func (m *Manager) DisconnectIfConnectionGeneration(expected uint64) bool {
	return m.disconnectWithGeneration(false, expected)
}

func (m *Manager) DisconnectForRecovery() {
	m.disconnect(false)
}

func (m *Manager) disconnect(notify bool) {
	m.disconnectWithGeneration(notify, 0)
}

func (m *Manager) disconnectWithGeneration(notify bool, expected uint64) bool {
	m.operationControl.cancelActive()
	m.mutex.Lock()
	if expected != 0 && m.connectionGen.Load() != expected {
		m.mutex.Unlock()
		return false
	}
	m.connectionGen.Add(1)
	if !m.isConnected {
		m.mutex.Unlock()
		return false
	}

	if !(compatibilityRuntime{}).disconnectLocked(m) {
		if m.deviceType == types.DeviceTransportBLE {
			m.disconnectBLELocked()
		} else if m.deviceType == types.DeviceTransportHID || m.deviceType == types.DeviceTransportUSB {
			m.disconnectFlyDigiHIDLocked()
		} else {
			m.disconnectWiFiLocked()
		}
	}
	shouldNotify := notify && m.onDisconnect != nil
	m.mutex.Unlock()

	m.logInfo("网络风扇控制器连接已断开")
	if shouldNotify {
		m.onDisconnect()
	}
	return true
}

func (m *Manager) IsConnected() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.isConnected
}

func (m *Manager) ConnectionGeneration() uint64 {
	return m.connectionGen.Load()
}

func (m *Manager) GetProductID() uint16 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.productID
}

func (m *Manager) GetModelName() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.deviceType == types.DeviceTransportHID || m.deviceType == types.DeviceTransportUSB {
		if isBlackSharkProfileID(m.activeProfile.ID) {
			return types.BlackSharkBRB02DisplayName
		}
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

	if m.writesBlocked.Load() || !m.isConnected {
		return false
	}
	ctx, finishOperation := m.operationControl.begin()
	defer finishOperation()
	if m.writesBlocked.Load() {
		return false
	}
	if written, handled := (compatibilityRuntime{}).setPercentSpeedLocked(ctx, m, percent); handled {
		return written
	}
	if m.deviceType == types.DeviceTransportBLE {
		if types.IsRPMSpeedUnit(m.activeProfile.SpeedUnit) {
			m.logWarn("percent speed command rejected because the active BLE profile uses RPM")
			return false
		}
		return m.setBLETargetSpeedWithContextLocked(ctx, types.NewPercentSpeed(percent))
	}
	return false
}

func (m *Manager) SetTargetSpeed(value int, unit string) bool {
	unit = types.NormalizeFanSpeedUnit(unit)
	if types.IsRPMSpeedUnit(unit) && m.isBlackSharkWiFiConnected() {
		return m.setBlackSharkWiFiTargetRPM(context.Background(), value)
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.writesBlocked.Load() || !m.isConnected {
		return false
	}
	ctx, finishOperation := m.operationControl.begin()
	defer finishOperation()
	if m.writesBlocked.Load() {
		return false
	}
	if written, handled := (compatibilityRuntime{}).setTargetSpeedLocked(ctx, m, value, unit); handled {
		return written
	}
	if types.IsRPMSpeedUnit(unit) {
		if m.deviceType == types.DeviceTransportHID || m.deviceType == types.DeviceTransportUSB {
			if isBlackSharkProfileID(m.activeProfile.ID) {
				return m.setBlackSharkHIDTargetSpeedLocked(types.NewRPMSpeed(value))
			}
			return m.setFlyDigiHIDTargetSpeedLocked(types.NewRPMSpeed(value))
		}
		if m.deviceType == types.DeviceTransportBLE {
			return m.setBLETargetSpeedWithContextLocked(ctx, types.NewRPMSpeed(value))
		}
		return false
	}
	if m.deviceType == types.DeviceTransportBLE {
		return m.setBLETargetSpeedWithContextLocked(ctx, types.NewPercentTickSpeed(value))
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

	if m.writesBlocked.Load() {
		return fmt.Errorf("device writes are blocked during system suspend")
	}
	if !m.isConnected {
		return fmt.Errorf("设备未连接")
	}
	if m.deviceType == types.DeviceTransportHID || m.deviceType == types.DeviceTransportUSB {
		if isBlackSharkProfileID(m.activeProfile.ID) {
			return fmt.Errorf("黑鲨 HID 不支持飞智实时模式命令")
		}
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
	if m.GetDeviceType() == types.DeviceTransportHID || m.GetDeviceType() == types.DeviceTransportUSB {
		m.mutex.RLock()
		blackShark := isBlackSharkProfileID(m.activeProfile.ID)
		m.mutex.RUnlock()
		if blackShark {
			return m.SetTargetSpeed(rpm, unit)
		}
		return m.setFlyDigiHIDManualGearRPM(gear, level, rpm)
	}
	if types.IsRPMSpeedUnit(unit) {
		return m.SetTargetSpeed(rpm, unit)
	}
	return m.SetPercentSpeed(rpm)
}

func (m *Manager) SetGearLight(enabled bool) bool {
	if m.isBlackSharkWiFiConnected() {
		return m.postBlackSharkWiFiJSON(context.Background(), "/api/gear-light", map[string]any{"enabled": enabled}) == nil
	}
	if m.GetDeviceType() == types.DeviceTransportHID || m.GetDeviceType() == types.DeviceTransportUSB {
		m.mutex.RLock()
		blackShark := isBlackSharkProfileID(m.activeProfile.ID)
		m.mutex.RUnlock()
		if blackShark {
			return m.setBlackSharkHIDRGBEnabled(enabled)
		}
		return m.setFlyDigiHIDGearLight(enabled)
	}
	if m.GetDeviceType() == types.DeviceTransportBLE {
		m.mutex.RLock()
		blackShark := m.activeProfile.ID == types.BlackSharkBRB02ProfileID
		executor := m.bleExecutor
		m.mutex.RUnlock()
		if blackShark && executor != nil {
			return executor.SetBlackSharkRGBEnabled(nil, enabled) == nil
		}
	}
	return false
}

func (m *Manager) SetPowerOnStart(enabled bool) bool {
	m.mutex.Lock()
	if m.isWiFiFirmwareV2Locked() {
		ok := m.setWiFiFirmwareConfigLocked(map[string]any{"powerOnStart": enabled})
		m.mutex.Unlock()
		return ok
	}
	m.mutex.Unlock()

	if m.IsBS1() {
		return m.setFlyDigiBS1PowerOnStart(enabled)
	}
	if m.GetDeviceType() == types.DeviceTransportHID || m.GetDeviceType() == types.DeviceTransportUSB {
		return m.setFlyDigiHIDPowerOnStart(enabled)
	}
	return false
}

func (m *Manager) SetSmartStartStop(mode string) bool {
	mode = normalizeWiFiSmartStartStopMode(mode)
	m.mutex.Lock()
	if m.isWiFiFirmwareV2Locked() {
		payload := map[string]any{"smartStartStop": mode}
		if mode == "delayed" {
			payload["delaySeconds"] = 60
		}
		ok := m.setWiFiFirmwareConfigLocked(payload)
		m.mutex.Unlock()
		return ok
	}
	m.mutex.Unlock()

	if m.GetDeviceType() == types.DeviceTransportHID {
		return m.setFlyDigiHIDSmartStartStop(mode)
	}
	return false
}

func (m *Manager) SetWiFiSmartStartStopStandbySpeed(percent int) bool {
	percent = types.ClampWiFiSmartStartStopStandbyPercent(percent)
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.isWiFiFirmwareV2Locked() {
		return false
	}
	return m.setWiFiFirmwareConfigLocked(map[string]any{"standbySpeed": percent})
}

func (m *Manager) SetBrightness(percentage int) bool {
	if m.isBlackSharkWiFiConnected() {
		if percentage < 0 || percentage > 100 {
			return false
		}
		return m.postBlackSharkWiFiJSON(context.Background(), "/api/brightness", map[string]any{"brightness": percentage}) == nil
	}
	if m.GetDeviceType() == types.DeviceTransportHID || m.GetDeviceType() == types.DeviceTransportUSB {
		m.mutex.RLock()
		blackShark := isBlackSharkProfileID(m.activeProfile.ID)
		m.mutex.RUnlock()
		if blackShark {
			return false
		}
		return m.setFlyDigiHIDBrightness(percentage)
	}
	return false
}

func (m *Manager) SetLightStrip(cfg types.LightStripConfig) error {
	if m.isBlackSharkWiFiConnected() {
		return m.postBlackSharkWiFiJSON(context.Background(), "/api/lighting", cfg)
	}
	if m.GetDeviceType() == types.DeviceTransportHID || m.GetDeviceType() == types.DeviceTransportUSB {
		m.mutex.RLock()
		blackShark := isBlackSharkProfileID(m.activeProfile.ID)
		m.mutex.RUnlock()
		if blackShark {
			return m.setBlackSharkHIDLighting(cfg)
		}
		return m.setFlyDigiHIDLightStrip(cfg)
	}
	if m.GetDeviceType() == types.DeviceTransportBLE {
		m.mutex.RLock()
		blackShark := m.activeProfile.ID == types.BlackSharkBRB02ProfileID
		executor := m.bleExecutor
		m.mutex.RUnlock()
		if blackShark && executor != nil {
			return executor.SetBlackSharkLighting(nil, cfg)
		}
	}
	return fmt.Errorf("active device does not support lighting")
}

func (m *Manager) SetRGBOff() bool {
	if m.isBlackSharkWiFiConnected() {
		return m.postBlackSharkWiFiJSON(context.Background(), "/api/rgb-off", map[string]any{}) == nil
	}
	if m.GetDeviceType() == types.DeviceTransportHID || m.GetDeviceType() == types.DeviceTransportUSB {
		m.mutex.RLock()
		blackShark := isBlackSharkProfileID(m.activeProfile.ID)
		m.mutex.RUnlock()
		if blackShark {
			return m.setBlackSharkHIDRGBEnabled(false)
		}
		return m.setFlyDigiHIDRGBOff()
	}
	return false
}

const maxBlackSharkWiFiImageBytes = 4 * 1024 * 1024

// isBlackSharkWiFiConnected keeps the mock-only routes out of ordinary WiFi
// profiles and out of the disconnected state.
func (m *Manager) isBlackSharkWiFiConnected() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.isConnected && m.deviceType == types.DeviceTransportWiFi &&
		m.activeProfile.ID == types.BlackSharkBRB02WiFiProfileID
}

func (m *Manager) postBlackSharkWiFiJSON(ctx context.Context, path string, payload any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	m.mutex.RLock()
	endpoint := m.wifiEndpoint
	client := m.wifiHTTPClient
	connected := m.isConnected && m.deviceType == types.DeviceTransportWiFi &&
		m.activeProfile.ID == types.BlackSharkBRB02WiFiProfileID
	m.mutex.RUnlock()
	if !connected {
		return fmt.Errorf("黑鲨 WiFi 设备未连接")
	}
	if client == nil {
		client = newWiFiHTTPClient()
	}
	base, err := normalizeWiFiEndpoint(endpoint)
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return retryDeviceSendContext(ctx, "Black Shark WiFi command", func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			if len(message) > 0 {
				return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(message)))
			}
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil
	})
}

func (m *Manager) setBlackSharkWiFiTargetRPM(ctx context.Context, value int) bool {
	if value < 0 || value > 4000 {
		return false
	}
	if err := m.postBlackSharkWiFiJSON(ctx, "/api/speed", map[string]any{"rpm": value, "targetRpm": value}); err != nil {
		m.logError("设置黑鲨 WiFi 转速失败: %v", err)
		return false
	}
	m.mutex.Lock()
	current := m.currentFanData.Load()
	next := &types.FanData{CurrentRPM: uint16(value), TargetRPM: uint16(value), Transport: types.DeviceTransportWiFi, SpeedUnit: types.FanSpeedUnitRPM}
	if current != nil {
		*next = *current
		next.TargetRPM = uint16(value)
		next.CurrentRPM = uint16(value)
		next.Transport = types.DeviceTransportWiFi
		next.SpeedUnit = types.FanSpeedUnitRPM
	}
	m.currentFanData.Store(next)
	callback := m.onFanDataUpdate
	m.mutex.Unlock()
	if callback != nil {
		go callback(next)
	}
	return true
}

func (m *Manager) SendBlackSharkWiFiHostInfoFrame() error {
	if !m.isBlackSharkWiFiConnected() {
		return fmt.Errorf("黑鲨 WiFi 设备未连接")
	}
	frame := m.blackSharkHostInfoFrame.Load()
	if frame == nil || len(*frame) == 0 {
		return fmt.Errorf("黑鲨屏幕参数尚未准备好")
	}
	return m.postBlackSharkWiFiJSON(context.Background(), "/api/screen/parameters", map[string]any{
		"frameBase64": base64.StdEncoding.EncodeToString(*frame),
	})
}

func (m *Manager) SetBlackSharkWiFiHostInfoRefreshInterval(intervalMs int) error {
	intervalMs = types.NormalizeBlackSharkScreenRefreshIntervalMs(intervalMs)
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.isConnected || m.deviceType != types.DeviceTransportWiFi || m.activeProfile.ID != types.BlackSharkBRB02WiFiProfileID {
		return fmt.Errorf("黑鲨 WiFi 设备未连接")
	}
	if m.blackSharkHostInfoRefreshStop != nil {
		close(m.blackSharkHostInfoRefreshStop)
		m.blackSharkHostInfoRefreshStop = nil
	}
	if intervalMs == 0 {
		return nil
	}
	stop := make(chan struct{})
	m.blackSharkHostInfoRefreshStop = stop
	go m.blackSharkWiFiHostInfoRefreshLoop(time.Duration(intervalMs)*time.Millisecond, stop)
	return nil
}

func (m *Manager) blackSharkWiFiHostInfoRefreshLoop(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := m.SendBlackSharkWiFiHostInfoFrame(); err != nil {
				m.logDebug("黑鲨 WiFi 参数页刷新失败: %v", err)
			}
		}
	}
}

// SendBlackSharkWiFiMockImage uploads RGB565 bytes to the local mock device.
// It is intentionally separate from the USB/libusb image protocol.
func (m *Manager) SendBlackSharkWiFiMockImage(ctx context.Context, rgb565 []byte, progress func(stage string, completed, total int)) error {
	if len(rgb565) == 0 || len(rgb565) > maxBlackSharkWiFiImageBytes {
		return fmt.Errorf("黑鲨 WiFi 图像数据大小无效: %d", len(rgb565))
	}
	if !m.isBlackSharkWiFiConnected() {
		return fmt.Errorf("黑鲨 WiFi 设备未连接")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	total := len(rgb565)
	if progress != nil {
		progress("preparing", 0, total)
	}
	m.mutex.RLock()
	endpoint := m.wifiEndpoint
	client := m.wifiHTTPClient
	m.mutex.RUnlock()
	if client == nil {
		client = newWiFiHTTPClient()
	}
	base, err := normalizeWiFiEndpoint(endpoint)
	if err != nil {
		return err
	}
	if progress != nil {
		progress("uploading", 0, total)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/screen/image", bytes.NewReader(rgb565))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if len(message) > 0 {
			return fmt.Errorf("黑鲨 WiFi 图传 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(message)))
		}
		return fmt.Errorf("黑鲨 WiFi 图传 HTTP %d", resp.StatusCode)
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if progress != nil {
		progress("completed", total, total)
	}
	return nil
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

	if source == types.DeviceTransportHID || source == types.DeviceTransportUSB {
		m.mutex.RLock()
		blackShark := isBlackSharkProfileID(m.activeProfile.ID)
		m.mutex.RUnlock()
		if blackShark {
			return m.queryBlackSharkHIDDeviceSettings()
		}
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

	if m.GetDeviceType() == types.DeviceTransportHID || m.GetDeviceType() == types.DeviceTransportUSB {
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
