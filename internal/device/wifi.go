package device

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceprofileexec"
	"github.com/TIANLI0/THRM/internal/types"
)

const wifiRequestTimeout = 2 * time.Second

type wifiDataResponse struct {
	Speed           any `json:"speed"`
	Temperature     any `json:"temperature"`
	Power           any `json:"power"`
	WiFiControl     any `json:"wifiControl"`
	WiFiTargetSpeed any `json:"wifiTargetSpeed"`
	ControlMode     any `json:"controlMode"`
	Mode            any `json:"mode"`
	TargetSpeed     any `json:"targetSpeed"`
	FanSpeed        any `json:"fanSpeed"`
}

type wifiSetSpeedResponse struct {
	Status      string `json:"status"`
	ControlMode string `json:"controlMode"`
	Message     string `json:"message"`
}

func newWiFiHTTPClient() *http.Client {
	return &http.Client{
		Timeout: wifiRequestTimeout,
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: (&net.Dialer{
				Timeout:   wifiRequestTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   wifiRequestTimeout,
			ResponseHeaderTimeout: wifiRequestTimeout,
			IdleConnTimeout:       30 * time.Second,
		},
	}
}

func (m *Manager) Configure(transport, endpoint string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	transport = types.NormalizeDeviceTransport(transport)
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = types.DefaultFanDeviceIP
	}
	if transport != types.DeviceTransportWiFi {
		if transport == types.DeviceTransportSerial {
			m.configureProfileLocked(defaultSerialPercentProfile(endpoint), endpoint)
			return
		}
		m.deviceTransport = transport
		m.wifiEndpoint = endpoint
		m.wifiExecutor = nil
		if m.bleExecutor != nil {
			if err := m.bleExecutor.Close(); err != nil {
				m.logWarn("BLE profile executor close failed during reconfigure: %v", err)
			}
		}
		m.bleExecutor = nil
		if m.serialExecutor != nil {
			if err := m.serialExecutor.Close(); err != nil {
				m.logWarn("Serial profile executor close failed during reconfigure: %v", err)
			}
		}
		m.serialExecutor = nil
		m.activeProfile = types.LegacyRPMProfileForTransport(transport)
		if m.wifiHTTPClient == nil {
			m.wifiHTTPClient = newWiFiHTTPClient()
		}
		return
	}

	m.configureProfileLocked(types.DefaultWiFiPercentProfile(endpoint), endpoint)
}

func (m *Manager) ConfigureProfile(profile types.DeviceProfile, fallbackEndpoint string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.configureProfileLocked(profile, fallbackEndpoint)
}

func (m *Manager) configureProfileLocked(profile types.DeviceProfile, fallbackEndpoint string) {
	profile = types.NormalizeDeviceProfile(profile, fallbackEndpoint)
	m.activeProfile = profile
	m.deviceTransport = profile.Transport

	endpoint := strings.TrimSpace(profile.Connection.Endpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(fallbackEndpoint)
	}
	if endpoint == "" {
		endpoint = types.DefaultFanDeviceIP
	}
	m.wifiEndpoint = endpoint
	if m.wifiHTTPClient == nil {
		m.wifiHTTPClient = newWiFiHTTPClient()
	}
	m.wifiExecutor = nil
	if m.bleExecutor != nil {
		if err := m.bleExecutor.Close(); err != nil {
			m.logWarn("BLE profile executor close failed during reconfigure: %v", err)
		}
	}
	m.bleExecutor = nil
	if m.serialExecutor != nil {
		if err := m.serialExecutor.Close(); err != nil {
			m.logWarn("Serial profile executor close failed during reconfigure: %v", err)
		}
	}
	m.serialExecutor = nil
	if profile.Transport != types.DeviceTransportWiFi {
		if profile.Transport == types.DeviceTransportBLE {
			executor, err := deviceprofileexec.NewBLEExecutor(profile, m.bleConnector)
			if err != nil {
				m.logError("BLE profile executor configuration failed: %v", err)
				return
			}
			m.bleExecutor = executor
			return
		}
		if profile.Transport == types.DeviceTransportSerial {
			executor, err := deviceprofileexec.NewSerialExecutor(profile, m.serialDialer)
			if err != nil {
				m.logError("Serial profile executor configuration failed: %v", err)
				return
			}
			m.serialExecutor = executor
		}
		return
	}
	executor, err := deviceprofileexec.NewWiFiExecutor(profile, endpoint, m.wifiHTTPClient)
	if err != nil {
		m.logError("WiFi profile executor configuration failed: %v", err)
		return
	}
	m.wifiExecutor = executor
}

func (m *Manager) shouldUseWiFiLocked() bool {
	return m.deviceTransport == "" || m.deviceTransport == types.DeviceTransportWiFi
}

func (m *Manager) shouldUseWiFi() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.shouldUseWiFiLocked()
}

func (m *Manager) connectWiFiLocked() (bool, map[string]string) {
	if !m.shouldUseWiFiLocked() {
		return false, nil
	}
	if m.wifiHTTPClient == nil {
		m.wifiHTTPClient = newWiFiHTTPClient()
	}
	if strings.TrimSpace(m.wifiEndpoint) == "" {
		m.wifiEndpoint = types.DefaultFanDeviceIP
	}

	fanData, err := m.readWiFiStateLocked()
	if err != nil {
		m.logError("WiFi 控制器连接失败: %v", err)
		return false, nil
	}

	m.isConnected = true
	m.deviceType = types.DeviceTransportWiFi
	m.productID = 0
	m.currentFanData.Store(fanData)
	if m.onFanDataUpdate != nil {
		go m.onFanDataUpdate(fanData)
	}

	displayName := m.activeProfileDisplayNameLocked(wifiOnlyModelName)
	info := map[string]string{
		"manufacturer": m.activeProfileVendorLocked(),
		"product":      displayName,
		"serial":       m.wifiEndpoint,
		"model":        displayName,
		"transport":    types.DeviceTransportWiFi,
		"endpoint":     m.wifiEndpoint,
	}
	return true, info
}

func (m *Manager) RefreshWiFiState() bool {
	if m.GetDeviceType() != types.DeviceTransportWiFi || !m.shouldUseWiFi() {
		return true
	}

	m.mutex.Lock()
	if !m.isConnected {
		m.mutex.Unlock()
		return false
	}
	fanData, err := m.readWiFiStateLocked()
	if err != nil {
		m.mutex.Unlock()
		m.logError("刷新 WiFi 控制器状态失败: %v", err)
		return false
	}
	m.currentFanData.Store(fanData)
	callback := m.onFanDataUpdate
	m.mutex.Unlock()

	if callback != nil {
		callback(fanData)
	}
	return true
}

func (m *Manager) disconnectWiFiLocked() bool {
	if m.deviceType != types.DeviceTransportWiFi {
		return false
	}
	m.isConnected = false
	m.deviceType = ""
	m.productID = 0
	m.currentFanData.Store(nil)
	return true
}

func (m *Manager) readWiFiStateLocked() (*types.FanData, error) {
	if m.wifiExecutor != nil {
		return m.wifiExecutor.ReadState(nil)
	}

	endpoint, err := normalizeWiFiEndpoint(m.wifiEndpoint)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint+"/api/data", nil)
	if err != nil {
		return nil, err
	}

	resp, err := m.wifiHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET /api/data returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, err
	}

	var data wifiDataResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return fanDataFromWiFiResponse(data), nil
}

func (m *Manager) setWiFiSpeedLocked(percent int) bool {
	speed := types.NewPercentSpeed(percent)
	if types.IsRPMSpeedUnit(m.activeProfile.SpeedUnit) {
		speed = types.NewRPMSpeed(percent)
	}
	return m.setWiFiTargetSpeedLocked(speed)
}

func (m *Manager) setWiFiTargetSpeedLocked(speed types.FanSpeedValue) bool {
	speed = speed.Normalized()
	activeUnit := types.NormalizeFanSpeedUnit(m.activeProfile.SpeedUnit)
	if speed.Unit != activeUnit {
		m.logError("WiFi speed unit mismatch: got %s, profile expects %s", speed.Unit, activeUnit)
		return false
	}

	if m.wifiExecutor != nil {
		next, err := m.wifiExecutor.SetSpeed(nil, speed)
		if err != nil {
			m.logError("WiFi profile speed command failed: %v", err)
			return false
		}
		next.Transport = types.DeviceTransportWiFi
		next.SpeedUnit = speed.Unit
		if next.TargetRPM == 0 {
			next.TargetRPM = uint16(speedValueForFanData(speed))
		}
		m.currentFanData.Store(next)

		if m.onFanDataUpdate != nil {
			go m.onFanDataUpdate(next)
		}

		m.logDebug("WiFi profile speed set: %s", formatSpeedValueForLog(speed))
		return true
	}

	percent, ok := speed.IntegerPercentForSend()
	if !ok {
		m.logError("default WiFi protocol does not support RPM speed command: %d", speed.Value)
		return false
	}
	percent = types.ClampFanPercent(percent)
	endpoint, err := normalizeWiFiEndpoint(m.wifiEndpoint)
	if err != nil {
		m.logError("WiFi 控制器地址无效: %v", err)
		return false
	}

	payload, err := json.Marshal(map[string]int{"speed": percent})
	if err != nil {
		return false
	}
	req, err := http.NewRequest(http.MethodPost, endpoint+"/api/speed", bytes.NewReader(payload))
	if err != nil {
		m.logError("创建 WiFi 速度请求失败: %v", err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.wifiHTTPClient.Do(req)
	if err != nil {
		m.logError("设置 WiFi 风扇速度失败: %v", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		m.logError("设置 WiFi 风扇速度失败: HTTP %d", resp.StatusCode)
		return false
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		m.logError("读取 WiFi 速度响应失败: %v", err)
		return false
	}
	if err := validateWiFiSetSpeedResponse(body); err != nil {
		m.logError("WiFi 速度响应无效: %v", err)
		return false
	}

	next, err := m.readWiFiStateLocked()
	if err != nil {
		m.logError("速度下发后读取 WiFi 状态失败: %v", err)
		return false
	}
	next.Transport = types.DeviceTransportWiFi
	next.SpeedUnit = types.FanSpeedUnitPercent
	if next.TargetRPM == 0 {
		next.TargetRPM = uint16(percent)
	}
	m.currentFanData.Store(next)

	if m.onFanDataUpdate != nil {
		go m.onFanDataUpdate(next)
	}

	m.logDebug("已设置 WiFi 风扇速度: %d%%", percent)
	return true
}

func speedValueForFanData(speed types.FanSpeedValue) int {
	if types.IsRPMSpeedUnit(speed.Unit) {
		return clampUint16(speed.Value)
	}
	return types.PercentTicksToIntegerPercent(speed.Value)
}

func formatSpeedValueForLog(speed types.FanSpeedValue) string {
	if types.IsRPMSpeedUnit(speed.Unit) {
		return fmt.Sprintf("%d RPM", speed.Value)
	}
	return fmt.Sprintf("%.1f%%", types.PercentTicksToDecimalPercent(speed.Value))
}

func clampUint16(value int) int {
	if value < 0 {
		return 0
	}
	if value > 65535 {
		return 65535
	}
	return value
}

func normalizeWiFiEndpoint(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = types.DefaultFanDeviceIP
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("missing host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func validateWiFiSetSpeedResponse(body []byte) error {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil
	}

	var data wifiSetSpeedResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("invalid JSON response: %w", err)
	}

	status := strings.ToLower(strings.TrimSpace(data.Status))
	controlMode := strings.ToLower(strings.TrimSpace(data.ControlMode))
	if status == "" && controlMode == "" {
		return nil
	}
	if status == "success" || controlMode == "wifi" {
		return nil
	}
	if strings.TrimSpace(data.Message) != "" {
		return fmt.Errorf("%s", strings.TrimSpace(data.Message))
	}
	return fmt.Errorf("status=%q controlMode=%q", data.Status, data.ControlMode)
}

func fanDataFromWiFiResponse(data wifiDataResponse) *types.FanData {
	speed, ok := intFromAny(data.Speed)
	if !ok {
		speed, _ = intFromAny(data.FanSpeed)
	}
	target, ok := intFromAny(data.WiFiTargetSpeed)
	if !ok {
		target, ok = intFromAny(data.TargetSpeed)
	}
	if !ok {
		target = speed
	}
	speed = sanitizeWiFiPercent(speed)
	target = sanitizeWiFiPercent(target)

	controlMode := strings.ToLower(strings.TrimSpace(stringFromAny(data.ControlMode)))
	if controlMode == "" {
		controlMode = strings.ToLower(strings.TrimSpace(stringFromAny(data.Mode)))
	}
	wifiControl, _ := boolFromAny(data.WiFiControl)
	workMode := "手动"
	if wifiControl || strings.Contains(controlMode, "auto") || strings.Contains(controlMode, "wifi") || strings.Contains(controlMode, "software") {
		workMode = "软件控制"
	}

	return &types.FanData{
		CurrentRPM: uint16(speed),
		TargetRPM:  uint16(target),
		WorkMode:   workMode,
		SetGear:    "",
		MaxGear:    "",
		Transport:  types.DeviceTransportWiFi,
		SpeedUnit:  types.FanSpeedUnitPercent,
	}
}

func sanitizeWiFiPercent(value int) int {
	if value < types.FanSpeedMinPercent || value > types.FanSpeedMaxPercent {
		return 0
	}
	return value
}

func intFromAny(value any) (int, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case float64:
		return int(v + 0.5), true
	case float32:
		return int(v + 0.5), true
	case int:
		return v, true
	case int64:
		return int(v), true
	case json.Number:
		i, err := v.Int64()
		return int(i), err == nil
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return 0, false
		}
		if strings.HasSuffix(v, "%") {
			v = strings.TrimSpace(strings.TrimSuffix(v, "%"))
		}
		i, err := strconv.Atoi(v)
		return i, err == nil
	default:
		return 0, false
	}
}

func boolFromAny(value any) (bool, bool) {
	switch v := value.(type) {
	case nil:
		return false, false
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "on", "yes", "auto", "wifi", "software":
			return true, true
		case "false", "0", "off", "no", "manual":
			return false, true
		}
	case float64:
		return v != 0, true
	}
	return false, false
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}
