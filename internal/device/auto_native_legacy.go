//go:build legacydevice

package device

import "github.com/TIANLI0/THRM/internal/types"

func (m *Manager) ScanNativeDevices() []map[string]string {
	return m.ScanNativeDevicesProfiles(nil)
}

func (m *Manager) ScanNativeDevicesProfiles(_ []types.DeviceProfile) []map[string]string {
	if success, info := m.Connect(); success && len(info) > 0 {
		return []map[string]string{info}
	}
	return nil
}

func (m *Manager) AutoConnectNative() (bool, map[string]string) {
	return m.Connect()
}

func (m *Manager) AutoConnectNativeProfiles(_ []types.DeviceProfile) (bool, map[string]string) {
	return m.Connect()
}
