//go:build legacydevice

package device

import (
	"context"

	"github.com/TIANLI0/THRM/internal/types"
)

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

func (m *Manager) AutoConnectNativeProfilesContext(ctx context.Context, profiles []types.DeviceProfile) (bool, map[string]string) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return false, nil
		}
	}
	return m.AutoConnectNativeProfiles(profiles)
}

func (m *Manager) ConnectNativeProfileContext(ctx context.Context, profile types.DeviceProfile) (bool, map[string]string) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return false, nil
		}
	}
	return m.Connect()
}
