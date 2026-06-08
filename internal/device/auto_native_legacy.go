//go:build legacydevice

package device

func (m *Manager) ScanNativeDevices() []map[string]string {
	if success, info := m.Connect(); success && len(info) > 0 {
		return []map[string]string{info}
	}
	return nil
}

func (m *Manager) AutoConnectNative() (bool, map[string]string) {
	return m.Connect()
}
