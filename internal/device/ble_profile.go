//go:build !legacydevice

package device

import (
	"context"
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
		"profileId":    strings.TrimSpace(profile.ID),
	}
}

func (m *Manager) connectBLELocked() (bool, map[string]string) {
	return m.connectBLEWithContextLocked(context.Background())
}

func (m *Manager) connectBLEWithContextLocked(ctx context.Context) (bool, map[string]string) {
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
	expectedGeneration := m.connectionGen.Load()
	m.bleExecutor.SetConnectionLostCallback(func() {
		m.onBLEConnectionLost(expectedGeneration)
	})
	m.bleExecutor.SetNotificationCallback(func(data *types.FanData) {
		m.onBS1Notification(expectedGeneration, data)
	})

	fanData, err := m.readBLEStateWithContextLocked(ctx)
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

func (m *Manager) markBS1DisconnectedLocked() func() {
	if !m.isConnected || m.deviceType != types.DeviceTransportBLE || m.activeProfile.ID != types.FlyDigiBS1ProfileID {
		return nil
	}
	m.connectionGen.Add(1)
	m.disconnectBLELocked()
	return m.onDisconnect
}

func (m *Manager) onBLEConnectionLost(expectedGeneration uint64) {
	m.mutex.Lock()
	if expectedGeneration != 0 && m.connectionGen.Load() != expectedGeneration {
		m.mutex.Unlock()
		return
	}
	callback := m.markBS1DisconnectedLocked()
	m.mutex.Unlock()
	if callback != nil {
		callback()
	}
}

func (m *Manager) onBS1Notification(expectedGeneration uint64, data *types.FanData) {
	if data == nil {
		return
	}
	m.mutex.Lock()
	if expectedGeneration != 0 && m.connectionGen.Load() != expectedGeneration {
		m.mutex.Unlock()
		return
	}
	if !m.isConnected || m.deviceType != types.DeviceTransportBLE || m.activeProfile.ID != types.FlyDigiBS1ProfileID {
		m.mutex.Unlock()
		return
	}
	m.currentFanData.Store(data)
	callback := m.onFanDataUpdate
	m.mutex.Unlock()
	if callback != nil {
		callback(data)
	}
}

// refreshBLEStateWithChangeDetection 是 RefreshBLEState 的内部比较逻辑，
// 用轻量字段比较替代 reflect.DeepEqual（BLE 刷新频率高，避免每次反射开销）。
func refreshBLEStateWithChangeDetection(prev, next *types.FanData) bool {
	if prev == nil || next == nil {
		return prev != next
	}
	// 实际控制关心的字段：当前转速和目标转速。
	if prev.CurrentRPM != next.CurrentRPM {
		return true
	}
	if prev.TargetRPM != next.TargetRPM {
		return true
	}
	// FlyDigiCapability 指针比较。
	if (prev.FlyDigiCapability == nil) != (next.FlyDigiCapability == nil) {
		return true
	}
	if prev.FlyDigiCapability != nil && next.FlyDigiCapability != nil {
		if prev.FlyDigiCapability.GearSettings != next.FlyDigiCapability.GearSettings {
			return true
		}
		if prev.FlyDigiCapability.SelectedGearCode != next.FlyDigiCapability.SelectedGearCode {
			return true
		}
	}
	return false
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
	if m.activeProfile.ID == types.FlyDigiBS1ProfileID {
		if m.bleExecutor == nil || !m.bleExecutor.IsConnected() {
			m.mutex.Unlock()
			return false
		}
		// THRM treats BS1 notifications as the source of truth. A health poll
		// must not consume the next notification or start a second scan.
		fanData := m.currentFanData.Load()
		m.mutex.Unlock()
		return fanData != nil
	}
	fanData, err := m.refreshBLEStateLocked()
	if err != nil {
		m.mutex.Unlock()
		m.logError("BLE controller state refresh failed: %v", err)
		return false
	}
	var callback func(data *types.FanData)
	if refreshBLEStateWithChangeDetection(m.currentFanData.Load(), fanData) {
		m.currentFanData.Store(fanData)
		callback = m.onFanDataUpdate
	}
	m.mutex.Unlock()

	if callback != nil {
		callback(fanData)
	}
	return true
}

func (m *Manager) readBLEStateLocked() (*types.FanData, error) {
	return m.readBLEStateWithContextLocked(context.Background())
}

func (m *Manager) readBLEStateWithContextLocked(ctx context.Context) (*types.FanData, error) {
	if m.bleExecutor == nil {
		return nil, fmt.Errorf("ble profile executor is not configured")
	}
	fanData, err := m.bleExecutor.Open(ctx)
	if err != nil {
		return nil, err
	}
	fanData.Transport = types.DeviceTransportBLE
	fanData.SpeedUnit = types.NormalizeFanSpeedUnit(m.activeProfile.SpeedUnit)
	return fanData, nil
}

func (m *Manager) refreshBLEStateLocked() (*types.FanData, error) {
	if m.bleExecutor == nil {
		return nil, fmt.Errorf("ble profile executor is not configured")
	}
	fanData, err := m.bleExecutor.ReadState(nil)
	if err != nil {
		return nil, err
	}
	fanData.Transport = types.DeviceTransportBLE
	fanData.SpeedUnit = types.NormalizeFanSpeedUnit(m.activeProfile.SpeedUnit)
	return fanData, nil
}

func (m *Manager) setBLETargetSpeedLocked(speed types.FanSpeedValue) bool {
	return m.setBLETargetSpeedWithContextLocked(context.Background(), speed)
}

func (m *Manager) setBLETargetSpeedWithContextLocked(ctx context.Context, speed types.FanSpeedValue) bool {
	speed = speed.Normalized()
	if speed.Unit != types.NormalizeFanSpeedUnit(m.activeProfile.SpeedUnit) {
		m.logError("BLE speed unit mismatch: got %s, profile expects %s", speed.Unit, m.activeProfile.SpeedUnit)
		return false
	}
	if m.bleExecutor == nil {
		m.logError("BLE profile executor is not configured")
		return false
	}

	next, err := m.bleExecutor.SetSpeed(ctx, speed)
	if err != nil {
		m.logError("BLE profile speed command failed: %v", err)
		if callback := m.markBS1DisconnectedLocked(); callback != nil {
			go callback()
		}
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
