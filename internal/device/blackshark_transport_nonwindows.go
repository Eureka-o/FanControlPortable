//go:build !windows && !legacydevice

package device

import "github.com/Eureka-o/FanControlPortable/internal/types"

func (m *Manager) connectBlackSharkUSBLocked() (bool, map[string]string) {
	return false, nil
}

func nativeUSBDeviceInfo(profile types.DeviceProfile, path string) map[string]string {
	return map[string]string{
		"manufacturer": types.BlackSharkBRB02Vendor,
		"product":      profile.DisplayName,
		"model":        profile.Model,
		"transport":    types.DeviceTransportUSB,
		"endpoint":     path,
		"serial":       path,
		"profileId":    profile.ID,
	}
}
