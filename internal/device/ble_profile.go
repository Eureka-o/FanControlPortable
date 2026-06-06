//go:build !legacydevice

package device

import (
	"fmt"
	"strings"

	"github.com/TIANLI0/THRM/internal/deviceprofileexec"
	"github.com/TIANLI0/THRM/internal/types"
)

func (m *Manager) shouldUseBLELocked() bool {
	return m.deviceTransport == types.DeviceTransportBLE
}

func (m *Manager) bleConnectedInfoLocked() map[string]string {
	profile := types.NormalizeDeviceProfile(m.activeProfile, "")
	conn := profile.Connection
	endpoint := strings.TrimSpace(conn.Endpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(conn.BLENameFilter)
	}
	if endpoint == "" {
		endpoint = strings.TrimSpace(conn.BLEServiceUUID)
	}
	displayName := strings.TrimSpace(profile.DisplayName)
	if displayName == "" {
		displayName = "BLE device"
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
		"serial":       endpoint,
		"model":        model,
		"transport":    types.DeviceTransportBLE,
		"endpoint":     endpoint,
	}
}

func (m *Manager) connectBLELocked() (bool, map[string]string) {
	if !m.shouldUseBLELocked() {
		return false, nil
	}
	if m.bleExecutor == nil {
		executor, err := deviceprofileexec.NewBLEExecutor(m.activeProfile, m.bleConnector)
		if err != nil {
			m.logError("BLE profile executor configuration failed: %v", err)
			return false, nil
		}
		m.bleExecutor = executor
	}

	fanData, err := m.readBLEStateLocked()
	if err != nil {
		m.logError("BLE controller connection failed: %v", err)
		return false, nil
	}

	m.isConnected = true
	m.deviceType = types.DeviceTransportBLE
	m.productID = 0
	m.currentFanData.Store(fanData)
	if m.onFanDataUpdate != nil {
		go m.onFanDataUpdate(fanData)
	}

	return true, m.bleConnectedInfoLocked()
}

func (m *Manager) disconnectBLELocked() bool {
	if m.deviceType != types.DeviceTransportBLE {
		return false
	}
	if m.bleExecutor != nil {
		if err := m.bleExecutor.Close(); err != nil {
			m.logWarn("BLE controller close failed: %v", err)
		}
	}
	m.isConnected = false
	m.deviceType = ""
	m.productID = 0
	m.currentFanData.Store(nil)
	return true
}

func (m *Manager) RefreshBLEState() bool {
	if m.GetDeviceType() != types.DeviceTransportBLE {
		return true
	}

	m.mutex.Lock()
	if !m.isConnected {
		m.mutex.Unlock()
		return false
	}
	fanData, err := m.readBLEStateLocked()
	if err != nil {
		m.mutex.Unlock()
		m.logError("BLE controller state refresh failed: %v", err)
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

func (m *Manager) readBLEStateLocked() (*types.FanData, error) {
	if m.bleExecutor == nil {
		return nil, fmt.Errorf("ble profile executor is not configured")
	}
	fanData, err := m.bleExecutor.Open(nil)
	if err != nil {
		return nil, err
	}
	fanData.Transport = types.DeviceTransportBLE
	fanData.SpeedUnit = types.NormalizeFanSpeedUnit(m.activeProfile.SpeedUnit)
	return fanData, nil
}

func (m *Manager) setBLETargetSpeedLocked(speed types.FanSpeedValue) bool {
	speed = speed.Normalized()
	if speed.Unit != types.NormalizeFanSpeedUnit(m.activeProfile.SpeedUnit) {
		m.logError("BLE speed unit mismatch: got %s, profile expects %s", speed.Unit, m.activeProfile.SpeedUnit)
		return false
	}
	if m.bleExecutor == nil {
		m.logError("BLE profile executor is not configured")
		return false
	}

	next, err := m.bleExecutor.SetSpeed(nil, speed)
	if err != nil {
		m.logError("BLE profile speed command failed: %v", err)
		return false
	}
	next.Transport = types.DeviceTransportBLE
	next.SpeedUnit = speed.Unit
	if next.TargetRPM == 0 {
		next.TargetRPM = uint16(speedValueForFanData(speed))
	}
	m.currentFanData.Store(next)

	if m.onFanDataUpdate != nil {
		go m.onFanDataUpdate(next)
	}

	m.logDebug("BLE profile speed set: %s", formatSpeedValueForLog(speed))
	return true
}
