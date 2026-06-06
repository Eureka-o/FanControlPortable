//go:build !legacydevice

package device

import (
	"strings"

	"github.com/TIANLI0/THRM/internal/types"
)

const legacyHIDDisabledMessage = "legacy HID/RPM driver is not enabled in this build"

func (m *Manager) shouldUseLegacyHIDLocked() bool {
	return m.deviceTransport == types.DeviceTransportHID
}

func (m *Manager) legacyHIDInfoLocked() map[string]string {
	profile := types.NormalizeDeviceProfile(m.activeProfile, "")
	displayName := strings.TrimSpace(profile.DisplayName)
	if displayName == "" {
		displayName = "Legacy RPM controller"
	}
	vendor := strings.TrimSpace(profile.Vendor)
	if vendor == "" {
		vendor = "THRM"
	}
	model := strings.TrimSpace(profile.Model)
	if model == "" {
		model = displayName
	}
	return map[string]string{
		"manufacturer": vendor,
		"product":      displayName,
		"serial":       "",
		"model":        model,
		"transport":    types.DeviceTransportHID,
		"endpoint":     "",
		"message":      legacyHIDDisabledMessage,
	}
}

func (m *Manager) connectLegacyHIDLocked() (bool, map[string]string) {
	if !m.shouldUseLegacyHIDLocked() {
		return false, nil
	}
	m.logWarn(legacyHIDDisabledMessage)
	m.isConnected = false
	m.deviceType = ""
	m.productID = 0
	m.currentFanData.Store(nil)
	return false, m.legacyHIDInfoLocked()
}
