//go:build !legacydevice

package device

import (
	"fmt"
	"strings"

	"github.com/TIANLI0/THRM/internal/deviceprofileexec"
	"github.com/TIANLI0/THRM/internal/deviceprofiles"
	"github.com/TIANLI0/THRM/internal/types"
)

func (m *Manager) ScanNativeDevices() []map[string]string {
	return m.ScanNativeDevicesProfiles(nil)
}

func (m *Manager) ScanNativeDevicesProfiles(profiles []types.DeviceProfile) []map[string]string {
	devices := m.ScanNativeDevicesProfilesByTransport(profiles, types.DeviceTransportHID)
	devices = append(devices, m.ScanNativeDevicesProfilesByTransport(profiles, types.DeviceTransportBLE)...)
	return devices
}

func (m *Manager) ScanNativeDevicesProfilesByTransport(profiles []types.DeviceProfile, transport string) []map[string]string {
	candidates := nativeAutoConnectCandidates(profiles)
	switch types.NormalizeDeviceTransport(transport) {
	case types.DeviceTransportHID:
		return scanNativeHIDDevices(candidates)
	case types.DeviceTransportBLE:
		devices, err := scanNativeBLEDevices(candidates)
		if err != nil {
			m.logWarn("BLE native scan failed: %v", err)
			return nil
		}
		return devices
	default:
		return nil
	}
}

// AutoConnectNative enumerates built-in FlyDigi native transports.
// It follows the reference app order: HID BS2/BS2PRO/BS3/BS3PRO first,
// then BLE BS1. WiFi and serial remain compatibility-mode transports.
func (m *Manager) AutoConnectNative() (bool, map[string]string) {
	return m.AutoConnectNativeProfiles(nil)
}

func (m *Manager) AutoConnectNativeProfiles(profiles []types.DeviceProfile) (bool, map[string]string) {
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

	for _, profile := range nativeAutoConnectCandidates(profiles, previousProfile) {
		m.configureProfileLocked(profile, previousEndpoint)
		switch profile.Transport {
		case types.DeviceTransportHID:
			if success, info := m.connectLegacyHIDLocked(); success {
				return true, info
			}
		case types.DeviceTransportBLE:
			if success, info := m.connectBLELocked(); success {
				return true, info
			}
		}
	}

	m.configureProfileLocked(previousProfile, previousEndpoint)
	return false, nil
}

func (m *Manager) ConnectNativeProfile(profile types.DeviceProfile) (bool, map[string]string) {
	profile = types.NormalizeDeviceProfile(profile, "")
	if !types.IsNativeDeviceTransport(profile.Transport) {
		return false, nil
	}

	m.mutex.RLock()
	wasConnected := m.isConnected
	m.mutex.RUnlock()

	if wasConnected {
		m.DisconnectSilently()
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	previousProfile := m.activeProfile
	previousEndpoint := m.wifiEndpoint
	m.configureProfileLocked(profile, previousEndpoint)
	switch profile.Transport {
	case types.DeviceTransportHID:
		if success, info := m.connectLegacyHIDLocked(); success {
			return true, info
		}
	case types.DeviceTransportBLE:
		if success, info := m.connectBLELocked(); success {
			return true, info
		}
	}
	m.configureProfileLocked(previousProfile, previousEndpoint)
	return false, nil
}

func nativeAutoConnectCandidates(profiles []types.DeviceProfile, preferred ...types.DeviceProfile) []types.DeviceProfile {
	seen := map[string]bool{}
	preferredProfiles := make([]types.DeviceProfile, 0, len(preferred))
	hidProfiles := make([]types.DeviceProfile, 0)
	bleProfiles := make([]types.DeviceProfile, 0)

	add := func(target *[]types.DeviceProfile, profile types.DeviceProfile) {
		profile = types.NormalizeDeviceProfile(profile, "")
		if !types.IsNativeDeviceTransport(profile.Transport) {
			return
		}
		key := profile.ID
		if key == "" {
			key = profile.Transport + ":" + profile.DisplayName + ":" + profile.Connection.Endpoint + ":" + profile.Connection.BLEServiceUUID
		}
		if seen[key] {
			return
		}
		seen[key] = true
		*target = append(*target, profile)
	}

	for _, profile := range preferred {
		add(&preferredProfiles, profile)
	}
	for _, profile := range profiles {
		if profile.BuiltIn && deviceprofiles.IsBuiltInProfileID(profile.ID) {
			continue
		}
		if types.NormalizeDeviceTransport(profile.Transport) == types.DeviceTransportHID {
			add(&hidProfiles, profile)
		} else {
			add(&bleProfiles, profile)
		}
	}
	add(&hidProfiles, types.LegacyRPMProfileForTransport(types.DeviceTransportHID))
	add(&bleProfiles, types.FlyDigiBS1Profile())

	return append(preferredProfiles, append(hidProfiles, bleProfiles...)...)
}

func scanNativeHIDDevices(profiles []types.DeviceProfile) []map[string]string {
	devices := make([]map[string]string, 0)
	seenPaths := map[string]bool{}
	for _, profile := range profiles {
		profile = types.NormalizeDeviceProfile(profile, "")
		if profile.Transport != types.DeviceTransportHID {
			continue
		}
		for _, candidate := range scanFlyDigiHIDDevices(flyDigiHIDProductIDsForProfile(profile.ID)) {
			path := strings.TrimSpace(candidate.path)
			if path == "" || seenPaths[path] {
				continue
			}
			seenPaths[path] = true
			devices = append(devices, nativeHIDDeviceInfo(profile, candidate.productID, path))
		}
	}
	return devices
}

func scanNativeBLEDevices(profiles []types.DeviceProfile) ([]map[string]string, error) {
	bleProfiles := make([]types.DeviceProfile, 0)
	for _, profile := range profiles {
		profile = types.NormalizeDeviceProfile(profile, "")
		if profile.Transport == types.DeviceTransportBLE {
			bleProfiles = append(bleProfiles, profile)
		}
	}
	if len(bleProfiles) == 0 {
		return nil, nil
	}
	bleDevices, err := deviceprofileexec.ScanBLEDevices(types.BLEScanParams{
		OnlyMatched: true,
		Profiles:    bleProfiles,
	})
	if err != nil {
		return nil, err
	}
	devices := make([]map[string]string, 0, len(bleDevices))
	for _, device := range bleDevices {
		if !device.Matched {
			continue
		}
		devices = append(devices, nativeBLEDeviceInfo(device, bleProfiles))
	}
	return devices, nil
}

func nativeHIDDeviceInfo(profile types.DeviceProfile, productID uint16, path string) map[string]string {
	model := flyDigiHIDModelName(productID)
	profileID := profile.ID
	if profileID == types.LegacyRPMProfileID {
		if id := types.FlyDigiProfileIDForHIDProductID(productID); id != "" {
			profileID = id
		}
	}
	product := nativeProfileDisplayName(profile, model)
	if profile.ID == types.LegacyRPMProfileID && model != "" {
		product = "FlyDigi " + model
	}
	manufacturer := strings.TrimSpace(profile.Vendor)
	if manufacturer == "" {
		manufacturer = "FlyDigi"
	}
	return map[string]string{
		"manufacturer": manufacturer,
		"product":      product,
		"model":        model,
		"transport":    types.DeviceTransportHID,
		"endpoint":     path,
		"serial":       path,
		"productId":    formatHIDProductID(productID),
		"profileId":    profileID,
	}
}

func nativeBLEDeviceInfo(device types.BLEDeviceInfo, profiles []types.DeviceProfile) map[string]string {
	profile, _ := nativeProfileByID(profiles, device.MatchedProfileID)
	displayName := nativeProfileDisplayName(profile, device.Name)
	manufacturer := strings.TrimSpace(profile.Vendor)
	if manufacturer == "" {
		manufacturer = "BLE"
	}
	model := strings.TrimSpace(profile.Model)
	if model == "" {
		model = strings.TrimSpace(device.Name)
	}
	info := map[string]string{
		"manufacturer": manufacturer,
		"product":      displayName,
		"model":        model,
		"transport":    types.DeviceTransportBLE,
		"endpoint":     strings.TrimSpace(device.Address),
		"serial":       strings.TrimSpace(device.Address),
		"profileId":    strings.TrimSpace(device.MatchedProfileID),
	}
	if strings.TrimSpace(device.Name) != "" {
		info["name"] = strings.TrimSpace(device.Name)
	}
	if device.MatchedProfileDisplayName != "" {
		info["profileName"] = device.MatchedProfileDisplayName
	}
	return info
}

func nativeProfileByID(profiles []types.DeviceProfile, id string) (types.DeviceProfile, bool) {
	id = strings.TrimSpace(id)
	for _, profile := range profiles {
		if strings.TrimSpace(profile.ID) == id && id != "" {
			return types.NormalizeDeviceProfile(profile, ""), true
		}
	}
	return types.DeviceProfile{}, false
}

func nativeProfileDisplayName(profile types.DeviceProfile, fallback string) string {
	if displayName := strings.TrimSpace(profile.DisplayName); displayName != "" {
		return displayName
	}
	if model := strings.TrimSpace(profile.Model); model != "" {
		return model
	}
	if fallback = strings.TrimSpace(fallback); fallback != "" {
		return fallback
	}
	return strings.TrimSpace(profile.ID)
}

func formatHIDProductID(productID uint16) string {
	return fmt.Sprintf("0x%04X", productID)
}
