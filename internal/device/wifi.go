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
	CurrentSpeed    any `json:"currentSpeed"`
	CurrentRPM      any `json:"currentRpm"`
	Temperature     any `json:"temperature"`
	Power           any `json:"power"`
	WiFiControl     any `json:"wifiControl"`
	WiFiTargetSpeed any `json:"wifiTargetSpeed"`
	ControlMode     any `json:"controlMode"`
	Mode            any `json:"mode"`
	TargetSpeed     any `json:"targetSpeed"`
	TargetRPM       any `json:"targetRpm"`
	FanSpeed        any `json:"fanSpeed"`
}

type wifiSetSpeedResponse struct {
	Status      string `json:"status"`
	ControlMode string `json:"controlMode"`
	Message     string `json:"message"`
}

type wifiDeviceInfoResponse struct {
	Model        string `json:"model"`
	Firmware     string `json:"firmware"`
	Protocol     string `json:"protocol"`
	Capabilities struct {
		SmartStartStop bool `json:"smartStartStop"`
		Heartbeat      bool `json:"heartbeat"`
		PowerOnStart   bool `json:"powerOnStart"`
	} `json:"capabilities"`
}

type wifiFirmwareConfigResponse struct {
	PowerOnStart   bool   `json:"powerOnStart"`
	SmartStartStop string `json:"smartStartStop"`
	DelaySeconds   int    `json:"delaySeconds"`
	StandbySpeed   int    `json:"standbySpeed"`
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
	m.detectWiFiFirmwareLocked()

	m.isConnected = true
	m.deviceType = types.DeviceTransportWiFi
	m.productID = 0
	m.currentFanData.Store(fanData)
	if m.onFanDataUpdate != nil {
		go m.onFanDataUpdate(fanData)
	}
	m.startWiFiHeartbeatLocked()

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
	m.stopWiFiHeartbeatLocked()
	m.isConnected = false
	m.deviceType = ""
	m.productID = 0
	m.currentFanData.Store(nil)
	m.wifiProtocol = ""
	m.wifiConfig = false
	m.wifiHeartbeat = false
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
	if !looksLikeWiFiDiscoveryState(body) {
		return nil, fmt.Errorf("response does not look like a fan controller state")
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
	var next *types.FanData
	if err := retryDeviceSend("WiFi speed", func() error {
		req, err := http.NewRequest(http.MethodPost, endpoint+"/api/speed", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := m.wifiHTTPClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		if err != nil {
			return err
		}
		if err := validateWiFiSetSpeedResponse(body); err != nil {
			return err
		}

		next, err = m.readWiFiStateLocked()
		if err != nil {
			return err
		}
		next.Transport = types.DeviceTransportWiFi
		next.SpeedUnit = types.FanSpeedUnitPercent
		if next.TargetRPM == 0 {
			next.TargetRPM = uint16(percent)
		}
		return nil
	}); err != nil {
		m.logError("设置 WiFi 风扇速度失败: %v", err)
		return false
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

func (m *Manager) detectWiFiFirmwareLocked() {
	m.wifiProtocol = ""
	m.wifiConfig = false
	m.wifiHeartbeat = false
	if !supportsWiFiFirmwareV2Probe(m.activeProfile) {
		return
	}
	endpoint, err := normalizeWiFiEndpoint(m.wifiEndpoint)
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodGet, endpoint+"/api/device", nil)
	if err != nil {
		return
	}
	resp, err := m.wifiHTTPClient.Do(req)
	if err != nil {
		m.logDebug("WiFi firmware probe skipped: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return
	}
	var info wifiDeviceInfoResponse
	if err := json.Unmarshal(body, &info); err != nil {
		return
	}
	if strings.TrimSpace(info.Protocol) != "fancontrol-wifi-v2" {
		return
	}

	profile := m.activeProfile
	profile.Capabilities.SupportsPowerOnStart = info.Capabilities.PowerOnStart
	profile.Capabilities.SupportsSmartStartStop = info.Capabilities.SmartStartStop
	profile.Capabilities.SupportsSoftwareSmartStartStop = false
	m.activeProfile = profile
	m.wifiProtocol = "fancontrol-wifi-v2"
	m.wifiConfig = info.Capabilities.PowerOnStart || info.Capabilities.SmartStartStop
	m.wifiHeartbeat = info.Capabilities.Heartbeat
}

func supportsWiFiFirmwareV2Probe(profile types.DeviceProfile) bool {
	return profile.Transport == types.DeviceTransportWiFi && types.IsPercentSpeedUnit(profile.SpeedUnit)
}

func (m *Manager) isWiFiFirmwareV2Locked() bool {
	return m.isConnected &&
		m.deviceType == types.DeviceTransportWiFi &&
		m.wifiProtocol == "fancontrol-wifi-v2" &&
		m.wifiConfig
}

func (m *Manager) getWiFiFirmwareConfigLocked() (wifiFirmwareConfigResponse, error) {
	endpoint, err := normalizeWiFiEndpoint(m.wifiEndpoint)
	if err != nil {
		return wifiFirmwareConfigResponse{}, err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint+"/api/config", nil)
	if err != nil {
		return wifiFirmwareConfigResponse{}, err
	}
	resp, err := m.wifiHTTPClient.Do(req)
	if err != nil {
		return wifiFirmwareConfigResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return wifiFirmwareConfigResponse{}, fmt.Errorf("GET /api/config returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return wifiFirmwareConfigResponse{}, err
	}
	var cfg wifiFirmwareConfigResponse
	if err := json.Unmarshal(body, &cfg); err != nil {
		return wifiFirmwareConfigResponse{}, err
	}
	cfg.SmartStartStop = normalizeWiFiSmartStartStopMode(cfg.SmartStartStop)
	return cfg, nil
}

func (m *Manager) setWiFiFirmwareConfigLocked(payload map[string]any) bool {
	endpoint, err := normalizeWiFiEndpoint(m.wifiEndpoint)
	if err != nil {
		m.logError("WiFi 控制器地址无效: %v", err)
		return false
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	if err := retryDeviceSend("WiFi firmware config", func() error {
		req, err := http.NewRequest(http.MethodPost, endpoint+"/api/config", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := m.wifiHTTPClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return nil
	}); err != nil {
		m.logError("设置 WiFi 固件配置失败: %v", err)
		return false
	}
	return true
}

func normalizeWiFiSmartStartStopMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "immediate", "delayed":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return "off"
	}
}

func (m *Manager) startWiFiHeartbeatLocked() {
	if !m.wifiHeartbeat || m.wifiHbStop != nil {
		return
	}
	endpoint, err := normalizeWiFiEndpoint(m.wifiEndpoint)
	if err != nil {
		return
	}
	client := m.wifiHTTPClient
	stop := make(chan struct{})
	done := make(chan struct{})
	m.wifiHbStop = stop
	m.wifiHbDone = done
	go wifiHeartbeatLoop(client, endpoint, stop, done)
}

func (m *Manager) stopWiFiHeartbeatLocked() {
	if m.wifiHbStop == nil {
		return
	}
	close(m.wifiHbStop)
	m.wifiHbStop = nil
	m.wifiHbDone = nil
}

func wifiHeartbeatLoop(client *http.Client, endpoint string, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	postWiFiHeartbeat(client, endpoint)
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			postWiFiHeartbeat(client, endpoint)
		}
	}
}

func postWiFiHeartbeat(client *http.Client, endpoint string) {
	if client == nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, endpoint+"/api/heartbeat", bytes.NewReader([]byte("{}")))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4*1024))
}

func fanDataFromWiFiResponse(data wifiDataResponse) *types.FanData {
	speed, ok := firstIntFromAny(data.CurrentSpeed, data.CurrentRPM, data.FanSpeed, data.Speed)
	target, ok := firstIntFromAny(data.WiFiTargetSpeed, data.TargetSpeed, data.TargetRPM, data.Speed)
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

func firstIntFromAny(values ...any) (int, bool) {
	for _, value := range values {
		if parsed, ok := intFromAny(value); ok {
			return parsed, true
		}
	}
	return 0, false
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
