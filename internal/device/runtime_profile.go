package device

import (
	"strings"

	"github.com/TIANLI0/THRM/internal/types"
)

// ActiveProfile returns the runtime device profile currently held by the
// manager. Hidden FlyDigi profiles are allowed here because this value is not
// persisted into the user's device library.
func (m *Manager) ActiveProfile() types.DeviceProfile {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.activeProfileLocked()
}

func (m *Manager) ActiveCapabilities() types.DeviceCapabilities {
	return m.ActiveProfile().Capabilities
}

func (m *Manager) activeProfileLocked() types.DeviceProfile {
	if m.deviceType == types.DeviceTransportHID && m.productID != 0 {
		if profile, ok := types.FlyDigiProfileForHIDProductID(m.productID); ok {
			return types.NormalizeDeviceProfile(profile, m.wifiEndpoint)
		}
	}
	if strings.TrimSpace(m.activeProfile.ID) == "" &&
		strings.TrimSpace(m.activeProfile.DisplayName) == "" &&
		strings.TrimSpace(m.activeProfile.Transport) == "" {
		return types.DeviceProfile{}
	}
	return types.NormalizeDeviceProfile(m.activeProfile, m.wifiEndpoint)
}
