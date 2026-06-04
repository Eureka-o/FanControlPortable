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

	"github.com/TIANLI0/THRM/internal/appmeta"
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

	m.deviceTransport = types.NormalizeDeviceTransport(transport)
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = types.DefaultFanDeviceIP
	}
	m.wifiEndpoint = endpoint
	if m.wifiHTTPClient == nil {
		m.wifiHTTPClient = newWiFiHTTPClient()
	}
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

	info := map[string]string{
		"manufacturer": "Eureka-o",
		"product":      appmeta.DeviceModelName,
		"serial":       m.wifiEndpoint,
		"model":        appmeta.DeviceModelName,
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
