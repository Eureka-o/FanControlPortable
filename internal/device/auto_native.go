//go:build !legacydevice

package device

import (
	"fmt"

	"github.com/TIANLI0/THRM/internal/types"
)

func (m *Manager) ScanNativeDevices() []map[string]string {
	candidates := scanFlyDigiHIDDevices(flyDigiHIDProductIDsForProfile(types.LegacyRPMProfileID))
	devices := make([]map[string]string, 0, len(candidates))
	seen := map[uint16]bool{}
	for _, candidate := range candidates {
		if candidate.productID == 0 || seen[candidate.productID] {
			continue
		}
		seen[candidate.productID] = true
		model := flyDigiHIDModelName(candidate.productID)
		profileID := types.FlyDigiProfileIDForHIDProductID(candidate.productID)
		devices = append(devices, map[string]string{
			"manufacturer": "飞智（FlyDigi）",
			"product":      "飞智（FlyDigi）" + model,
			"model":        model,
			"transport":    types.DeviceTransportHID,
			"endpoint":     candidate.path,
			"serial":       candidate.path,
			"productId":    formatHIDProductID(candidate.productID),
			"profileId":    profileID,
		})
	}
	return devices
}

// AutoConnectNative enumerates built-in FlyDigi native transports.
// It follows the reference app order: HID BS2/BS2PRO/BS3/BS3PRO first,
// then BLE BS1. WiFi and serial remain compatibility-mode transports.
func (m *Manager) AutoConnectNative() (bool, map[string]string) {
	m.mutex.RLock()
	if m.isConnected && (m.deviceType == types.DeviceTransportHID || m.deviceType == types.DeviceTransportBLE) {
		info := m.connectedInfoLocked()
		m.mutex.RUnlock()
		return true, info
	}
	wasConnected := m.isConnected
	m.mutex.RUnlock()

	if wasConnected {
		m.DisconnectSilently()
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.isConnected {
		if m.deviceType == types.DeviceTransportHID || m.deviceType == types.DeviceTransportBLE {
			return true, m.connectedInfoLocked()
		}
		return false, nil
	}

	previousProfile := m.activeProfile
	previousEndpoint := m.wifiEndpoint

	m.configureProfileLocked(types.LegacyRPMProfileForTransport(types.DeviceTransportHID), previousEndpoint)
	if success, info := m.connectLegacyHIDLocked(); success {
		return true, info
	}

	m.configureProfileLocked(types.FlyDigiBS1Profile(), previousEndpoint)
	if success, info := m.connectBLELocked(); success {
		return true, info
	}

	m.configureProfileLocked(previousProfile, previousEndpoint)
	return false, nil
}

func formatHIDProductID(productID uint16) string {
	return fmt.Sprintf("0x%04X", productID)
}
