package device

import (
	"fmt"
	"strings"

	"github.com/TIANLI0/THRM/internal/deviceprofileexec"
	"github.com/TIANLI0/THRM/internal/types"
)

const defaultSerialPercentProfileID = "builtin.serial.percent"

func defaultSerialPercentProfile(port string) types.DeviceProfile {
	caps := types.DeviceCapabilities{
		ProfileID:           defaultSerialPercentProfileID,
		DisplayName:         "Serial percent controller",
		Transport:           types.DeviceTransportSerial,
		SpeedUnit:           types.FanSpeedUnitPercent,
		SpeedRange:          types.DefaultPercentSpeedRange(),
		SupportsCustomSpeed: true,
	}
	return types.DeviceProfile{
		ID:          defaultSerialPercentProfileID,
		DisplayName: caps.DisplayName,
		Vendor:      "FanControl",
		Model:       "Serial/COM",
		BuiltIn:     true,
		Transport:   types.DeviceTransportSerial,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  caps.SpeedRange,
		Connection: types.DeviceConnectionSettings{
			SerialPort:           strings.TrimSpace(port),
			SerialBaudRate:       115200,
			SerialDataBits:       8,
			SerialStopBits:       1,
			SerialParity:         "none",
			SerialFrameDelimiter: `\n`,
		},
		Capabilities: caps,
	}
}

func (m *Manager) shouldUseSerialLocked() bool {
	return m.deviceTransport == types.DeviceTransportSerial
}

func (m *Manager) connectedInfoLocked() map[string]string {
	if m.deviceType == types.DeviceTransportBLE {
		return m.bleConnectedInfoLocked()
	}
	if m.deviceType == types.DeviceTransportHID {
		path := ""
		if m.flyDigiHID != nil {
			path = m.flyDigiHID.path
		}
		return m.flyDigiHIDInfoLocked(m.productID, path)
	}
	if m.deviceType == types.DeviceTransportSerial {
		return m.serialConnectedInfoLocked()
	}
	displayName := m.activeProfileDisplayNameLocked(wifiOnlyModelName)
	return map[string]string{
		"manufacturer": m.activeProfileVendorLocked(),
		"product":      displayName,
		"serial":       m.wifiEndpoint,
		"model":        displayName,
		"transport":    types.DeviceTransportWiFi,
		"endpoint":     m.wifiEndpoint,
	}
}

func (m *Manager) serialConnectedInfoLocked() map[string]string {
	profile := types.NormalizeDeviceProfile(m.activeProfile, "")
	port := strings.TrimSpace(profile.Connection.SerialPort)
	displayName := strings.TrimSpace(profile.DisplayName)
	if displayName == "" {
		displayName = "Serial device"
	}
	vendor := strings.TrimSpace(profile.Vendor)
	if vendor == "" {
		vendor = "FanControl"
	}
	model := strings.TrimSpace(profile.Model)
	if model == "" {
		model = displayName
	}
	return map[string]string{
		"manufacturer": vendor,
		"product":      displayName,
		"serial":       port,
		"model":        model,
		"transport":    types.DeviceTransportSerial,
		"endpoint":     port,
	}
}

func (m *Manager) connectSerialLocked() (bool, map[string]string) {
	if !m.shouldUseSerialLocked() {
		return false, nil
	}
	if m.serialExecutor == nil {
		executor, err := deviceprofileexec.NewSerialExecutor(m.activeProfile, m.serialDialer)
		if err != nil {
			m.logError("Serial profile executor configuration failed: %v", err)
			return false, nil
		}
		m.serialExecutor = executor
	}

	fanData, err := m.readSerialStateLocked()
	if err != nil {
		m.logError("Serial controller connection failed: %v", err)
		return false, nil
	}

	m.isConnected = true
	m.deviceType = types.DeviceTransportSerial
	m.productID = 0
	m.currentFanData.Store(fanData)
	if m.onFanDataUpdate != nil {
		go m.onFanDataUpdate(fanData)
	}

	return true, m.serialConnectedInfoLocked()
}

func (m *Manager) disconnectSerialLocked() bool {
	if m.deviceType != types.DeviceTransportSerial {
		return false
	}
	if m.serialExecutor != nil {
		if err := m.serialExecutor.Close(); err != nil {
			m.logWarn("Serial controller close failed: %v", err)
		}
	}
	m.isConnected = false
	m.deviceType = ""
	m.productID = 0
	m.currentFanData.Store(nil)
	return true
}

func (m *Manager) RefreshSerialState() bool {
	if m.GetDeviceType() != types.DeviceTransportSerial {
		return true
	}

	m.mutex.Lock()
	if !m.isConnected {
		m.mutex.Unlock()
		return false
	}
	fanData, err := m.readSerialStateLocked()
	if err != nil {
		m.mutex.Unlock()
		m.logError("Serial controller state refresh failed: %v", err)
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

func (m *Manager) readSerialStateLocked() (*types.FanData, error) {
	if m.serialExecutor == nil {
		return nil, fmt.Errorf("serial profile executor is not configured")
	}
	fanData, err := m.serialExecutor.ReadState(nil)
	if err != nil {
		return nil, err
	}
	fanData.Transport = types.DeviceTransportSerial
	fanData.SpeedUnit = types.NormalizeFanSpeedUnit(m.activeProfile.SpeedUnit)
	return fanData, nil
}

func (m *Manager) setSerialTargetSpeedLocked(speed types.FanSpeedValue) bool {
	speed = speed.Normalized()
	if speed.Unit != types.NormalizeFanSpeedUnit(m.activeProfile.SpeedUnit) {
		m.logError("Serial speed unit mismatch: got %s, profile expects %s", speed.Unit, m.activeProfile.SpeedUnit)
		return false
	}
	if m.serialExecutor == nil {
		m.logError("Serial profile executor is not configured")
		return false
	}

	next, err := m.serialExecutor.SetSpeed(nil, speed)
	if err != nil {
		m.logError("Serial profile speed command failed: %v", err)
		return false
	}
	next.Transport = types.DeviceTransportSerial
	next.SpeedUnit = speed.Unit
	if next.TargetRPM == 0 {
		next.TargetRPM = uint16(speedValueForFanData(speed))
	}
	m.currentFanData.Store(next)

	if m.onFanDataUpdate != nil {
		go m.onFanDataUpdate(next)
	}

	m.logDebug("Serial profile speed set: %s", formatSpeedValueForLog(speed))
	return true
}
