//go:build !legacydevice

package device

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/deviceprofileexec"
	"github.com/Eureka-o/FanControlPortable/internal/deviceprofiles"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

const idleBLEScanCooldown = 15 * time.Minute

func shouldSkipIdleBLEAutoScan(sinceLastScan time.Duration) bool {
	return sinceLastScan >= 0 && sinceLastScan < idleBLEScanCooldown
}

func nativeBLEAutoConnectCandidateKey(profile types.DeviceProfile) string {
	return strings.TrimSpace(profile.ID) + "\x1f" + strings.ToLower(strings.TrimSpace(profile.Connection.Endpoint))
}

func filterNativeAutoConnectCandidatesForBLECooldown(candidates []types.DeviceProfile, failures map[string]time.Time, now time.Time) []types.DeviceProfile {
	if len(failures) == 0 {
		return candidates
	}
	filtered := make([]types.DeviceProfile, 0, len(candidates))
	for _, profile := range candidates {
		if types.NormalizeDeviceTransport(profile.Transport) == types.DeviceTransportBLE {
			if failedAt, ok := failures[nativeBLEAutoConnectCandidateKey(profile)]; ok && shouldSkipIdleBLEAutoScan(now.Sub(failedAt)) {
				continue
			}
		}
		filtered = append(filtered, profile)
	}
	return filtered
}

func shouldProbeBlackSharkUSB(previousProfile types.DeviceProfile) bool {
	return strings.TrimSpace(previousProfile.ID) == "" || isBlackSharkProfileID(previousProfile.ID)
}

func nativeBLEAutoConnectProfileFromScan(profile types.DeviceProfile, devices []map[string]string, failures map[string]time.Time, now time.Time) (types.DeviceProfile, bool) {
	for _, device := range devices {
		candidate := profile
		candidate.Connection.Endpoint = strings.TrimSpace(device["endpoint"])
		if candidate.Connection.Endpoint == "" {
			continue
		}
		if failedAt, ok := failures[nativeBLEAutoConnectCandidateKey(candidate)]; ok && shouldSkipIdleBLEAutoScan(now.Sub(failedAt)) {
			continue
		}
		return candidate, true
	}
	return types.DeviceProfile{}, false
}

func (m *Manager) ScanNativeDevices() []map[string]string {
	return m.ScanNativeDevicesProfiles(nil)
}

func (m *Manager) ScanNativeDevicesProfiles(profiles []types.DeviceProfile) []map[string]string {
	devices := m.ScanNativeDevicesProfilesByTransport(profiles, types.DeviceTransportBLE)
	devices = append(devices, m.ScanNativeDevicesProfilesByTransport(profiles, types.DeviceTransportHID)...)
	devices = append(devices, m.ScanNativeDevicesProfilesByTransport(profiles, types.DeviceTransportUSB)...)
	return devices
}

func (m *Manager) ScanNativeDevicesProfilesByTransport(profiles []types.DeviceProfile, transport string) []map[string]string {
	return m.ScanNativeDevicesProfilesByTransportContext(context.Background(), profiles, transport)
}

func (m *Manager) ScanNativeDevicesProfilesByTransportContext(ctx context.Context, profiles []types.DeviceProfile, transport string) []map[string]string {
	candidates := nativeAutoConnectCandidates(profiles)
	switch types.NormalizeDeviceTransport(transport) {
	case types.DeviceTransportHID:
		return scanNativeHIDDevices(candidates)
	case types.DeviceTransportUSB:
		return scanNativeUSBDevices(candidates)
	case types.DeviceTransportBLE:
		devices, err := scanNativeBLEDevices(ctx, candidates)
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
// BLE is attempted before HID for automatic native-device arbitration.
// WiFi and serial remain compatibility-mode transports.
func (m *Manager) AutoConnectNative() (bool, map[string]string) {
	return m.AutoConnectNativeProfiles(nil)
}

func (m *Manager) AutoConnectNativeProfiles(profiles []types.DeviceProfile) (bool, map[string]string) {
	return m.AutoConnectNativeProfilesContext(context.Background(), profiles)
}

func (m *Manager) AutoConnectNativeProfilesContext(ctx context.Context, profiles []types.DeviceProfile) (bool, map[string]string) {
	if ctx == nil {
		ctx = context.Background()
	}
	m.autoNativeMu.Lock()
	defer m.autoNativeMu.Unlock()

	now := time.Now()
	m.mutex.Lock()
	wasConnected := m.isConnected
	previousProfile := m.activeProfile
	failures := make(map[string]time.Time, len(m.failedAutoBLEScans))
	for key, failedAt := range m.failedAutoBLEScans {
		if !shouldSkipIdleBLEAutoScan(now.Sub(failedAt)) {
			delete(m.failedAutoBLEScans, key)
			continue
		}
		failures[key] = failedAt
	}
	m.mutex.Unlock()

	if wasConnected {
		m.DisconnectSilently()
	}

	if ctx.Err() != nil {
		return false, nil
	}

	allCandidates := nativeAutoConnectCandidates(profiles, previousProfile)
	candidates := filterNativeAutoConnectCandidatesForBLECooldown(
		allCandidates,
		failures,
		now,
	)

	// Reconnecting a Black Shark cooler: probe its WinUSB path FIRST. The
	// libusb open attempt costs milliseconds, while the BLE candidates burn a
	// full 10s discovery scan each when the device is not advertising — long
	// enough to trip the GUI's 10s Connect budget, which recycles the shared
	// IPC pipe and kills any in-flight scan request. Non-blackshark reconnects
	// and fresh installs keep the established candidate order below (the USB
	// probe after the loop) so their arbitration is unchanged.
	// previousProfile 为空 (启动首次重连) 时同样前置探测: 否则黑鲨 USB 用户每次
	// 启动都要先等 10-20s 的 BLE 空扫描, 期间 GUI 手动连接会排队并超时。
	blackSharkUSBProbed := shouldProbeBlackSharkUSB(previousProfile)
	if blackSharkUSBProbed {
		if success, info := m.ConnectBlackSharkPreferredNative(ctx); success {
			return true, info
		}
		if ctx.Err() != nil {
			return false, nil
		}
	}

	for _, profile := range candidates {
		if ctx.Err() != nil {
			return false, nil
		}
		switch profile.Transport {
		case types.DeviceTransportHID:
			if success, info := m.connectNativeProfileContextIfDisconnected(ctx, profile); success {
				return true, info
			}
		case types.DeviceTransportUSB:
			if blackSharkUSBProbed && profile.ID == types.BlackSharkBRB02USBProfileID {
				continue
			}
			if success, info := m.connectNativeProfileContextIfDisconnected(ctx, profile); success {
				return true, info
			}
		case types.DeviceTransportBLE:
			key := nativeBLEAutoConnectCandidateKey(profile)
			bleProfile := profile
			if strings.TrimSpace(bleProfile.Connection.Endpoint) == "" {
				devices, err := m.scanNativeBLEDevices(ctx, []types.DeviceProfile{profile})
				if err != nil {
					m.logWarn("BLE native scan failed for %s: %v", profile.ID, err)
				}
				if len(devices) > 0 {
					var found bool
					bleProfile, found = nativeBLEAutoConnectProfileFromScan(profile, devices, failures, now)
					if !found {
						continue
					}
					key = nativeBLEAutoConnectCandidateKey(bleProfile)
				}
			}
			if strings.TrimSpace(bleProfile.Connection.Endpoint) != "" {
				if success, info := m.connectNativeProfileContextIfDisconnected(ctx, bleProfile); success {
					return true, info
				}
			}
			if ctx.Err() != nil {
				return false, nil
			}
			m.mutex.Lock()
			if m.failedAutoBLEScans == nil {
				m.failedAutoBLEScans = make(map[string]time.Time)
			}
			m.failedAutoBLEScans[key] = time.Now()
			m.mutex.Unlock()
		}
	}
	if len(candidates) < len(allCandidates) {
		m.logDebug("failed automatic BLE candidates are cooling down; other native candidates remained eligible")
	}
	return false, nil
}

// ConnectBlackSharkPreferredNative shares USB-first, paired-BLE-second arbitration
// between automatic scanning and CoreApp reconnects.
func (m *Manager) ConnectBlackSharkPreferredNative(ctx context.Context) (bool, map[string]string) {
	if ctx == nil {
		ctx = context.Background()
	}
	if connected, info := m.connectNativeProfileContextIfDisconnected(ctx, types.BlackSharkBRB02USBProfile()); connected {
		return true, info
	}
	if ctx.Err() != nil {
		return false, nil
	}
	return m.ConnectBlackSharkBLEDirect(ctx)
}

func (m *Manager) connectNativeProfileContextIfDisconnected(ctx context.Context, profile types.DeviceProfile) (bool, map[string]string) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return false, nil
		}
	}
	profile, err := deviceprofiles.PrepareRuntimeProfile(profile, "")
	if err != nil || !types.IsNativeDeviceTransport(profile.Transport) || isLegacyBlackSharkHIDProfileID(profile.ID) {
		return false, nil
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.isConnected {
		return true, m.connectedInfoLocked()
	}
	previousProfile := m.activeProfile
	previousEndpoint := m.wifiEndpoint
	var connected bool
	defer func() {
		if !connected {
			m.configureProfileLocked(previousProfile, previousEndpoint)
		}
	}()
	m.configureProfileLocked(profile, previousEndpoint)
	var info map[string]string
	switch profile.Transport {
	case types.DeviceTransportHID:
		connected, info = m.connectLegacyHIDLocked()
	case types.DeviceTransportUSB:
		connected, info = m.connectBlackSharkUSBLocked()
	case types.DeviceTransportBLE:
		connected, info = m.connectBLEWithContextLocked(ctx)
	}
	return connected, info
}

func (m *Manager) ConnectNativeProfile(profile types.DeviceProfile) (bool, map[string]string) {
	return m.ConnectNativeProfileContext(context.Background(), profile)
}

func (m *Manager) ConnectNativeProfileContext(ctx context.Context, profile types.DeviceProfile) (connected bool, info map[string]string) {
	if ctx == nil {
		ctx = context.Background()
	}
	var err error
	profile, err = deviceprofiles.PrepareRuntimeProfile(profile, "")
	if err != nil {
		m.logWarn("native device profile rejected: %v", err)
		return false, nil
	}
	if isLegacyBlackSharkHIDProfileID(profile.ID) {
		m.logWarn("legacy Black Shark HID profile is no longer supported")
		return false, nil
	}
	if !types.IsNativeDeviceTransport(profile.Transport) {
		return false, nil
	}
	if profile.Transport == types.DeviceTransportBLE && strings.TrimSpace(profile.Connection.Endpoint) == "" {
		devices, err := m.scanNativeBLEDevices(ctx, []types.DeviceProfile{profile})
		if err != nil || len(devices) == 0 {
			return false, nil
		}
		profile.Connection.Endpoint = strings.TrimSpace(devices[0]["endpoint"])
		if profile.Connection.Endpoint == "" {
			return false, nil
		}
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
	if ctx.Err() != nil {
		return false, nil
	}
	defer func() {
		if !connected {
			m.configureProfileLocked(previousProfile, previousEndpoint)
		}
	}()
	m.configureProfileLocked(profile, previousEndpoint)
	switch profile.Transport {
	case types.DeviceTransportHID:
		if success, info := m.connectLegacyHIDLocked(); success {
			return true, info
		}
	case types.DeviceTransportUSB:
		if success, info := m.connectBlackSharkUSBLocked(); success {
			return true, info
		}
	case types.DeviceTransportBLE:
		if success, info := m.connectBLEWithContextLocked(ctx); success {
			return true, info
		}
	}
	return false, nil
}

func nativeAutoConnectCandidates(profiles []types.DeviceProfile, preferred ...types.DeviceProfile) []types.DeviceProfile {
	seen := map[string]bool{}
	preferredBLEProfiles := make([]types.DeviceProfile, 0, len(preferred))
	preferredUSBProfiles := make([]types.DeviceProfile, 0, len(preferred))
	preferredHIDProfiles := make([]types.DeviceProfile, 0, len(preferred))
	usbProfiles := make([]types.DeviceProfile, 0)
	hidProfiles := make([]types.DeviceProfile, 0)
	bleProfiles := make([]types.DeviceProfile, 0)

	add := func(target *[]types.DeviceProfile, profile types.DeviceProfile) {
		if isLegacyBlackSharkHIDProfileID(profile.ID) {
			return
		}
		var err error
		profile, err = deviceprofiles.PrepareRuntimeProfile(profile, "")
		if err != nil {
			return
		}
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
		switch types.NormalizeDeviceTransport(profile.Transport) {
		case types.DeviceTransportBLE:
			add(&preferredBLEProfiles, profile)
		case types.DeviceTransportUSB:
			add(&preferredUSBProfiles, profile)
		default:
			add(&preferredHIDProfiles, profile)
		}
	}
	for _, profile := range profiles {
		if profile.BuiltIn && deviceprofiles.IsBuiltInProfileID(profile.ID) {
			continue
		}
		switch types.NormalizeDeviceTransport(profile.Transport) {
		case types.DeviceTransportHID:
			add(&hidProfiles, profile)
		case types.DeviceTransportUSB:
			add(&usbProfiles, profile)
		default:
			add(&bleProfiles, profile)
		}
	}
	add(&usbProfiles, types.BlackSharkBRB02USBProfile())
	add(&hidProfiles, types.LegacyRPMProfileForTransport(types.DeviceTransportHID))
	add(&bleProfiles, types.FlyDigiBS1Profile())
	add(&bleProfiles, types.BlackSharkBRB02Profile())

	// Keep the previous profile preferred only inside its transport group.
	result := make([]types.DeviceProfile, 0, len(preferred)+len(profiles)+5)
	result = append(result, preferredBLEProfiles...)
	result = append(result, bleProfiles...)
	result = append(result, preferredUSBProfiles...)
	result = append(result, usbProfiles...)
	result = append(result, preferredHIDProfiles...)
	return append(result, hidProfiles...)
}

func scanNativeHIDDevices(profiles []types.DeviceProfile) []map[string]string {
	devices := make([]map[string]string, 0)
	seenPaths := map[string]bool{}
	for _, profile := range profiles {
		if isLegacyBlackSharkHIDProfileID(profile.ID) {
			continue
		}
		profile = types.NormalizeDeviceProfile(profile, "")
		if profile.Transport != types.DeviceTransportHID {
			continue
		}
		candidates := scanFlyDigiHIDDevices(flyDigiHIDProductIDsForProfile(profile.ID))
		for _, candidate := range candidates {
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

func scanNativeUSBDevices(profiles []types.DeviceProfile) []map[string]string {
	matched := false
	for _, profile := range profiles {
		profile = types.NormalizeDeviceProfile(profile, "")
		if profile.Transport != types.DeviceTransportUSB || profile.ID != types.BlackSharkBRB02USBProfileID {
			continue
		}
		matched = true
		path, err := scanBlackSharkUSBDevice()
		if err != nil {
			return nil
		}
		return []map[string]string{nativeUSBDeviceInfo(profile, path)}
	}
	if !matched {
		profile := types.BlackSharkBRB02USBProfile()
		path, err := scanBlackSharkUSBDevice()
		if err != nil {
			return nil
		}
		return []map[string]string{nativeUSBDeviceInfo(profile, path)}
	}
	return nil
}

func nativeUSBDeviceInfo(profile types.DeviceProfile, path string) map[string]string {
	return map[string]string{
		"manufacturer": types.BlackSharkBRB02Vendor,
		"product":      profile.DisplayName,
		"model":        profile.Model,
		"transport":    types.DeviceTransportUSB,
		"endpoint":     path,
		"serial":       path,
		"productId":    fmt.Sprintf("0x%04X", types.BlackSharkBRB02HIDProductID),
		"profileId":    profile.ID,
	}
}

func scanNativeBLEDevices(ctx context.Context, profiles []types.DeviceProfile) ([]map[string]string, error) {
	return scanNativeBLEDevicesWithScanner(ctx, profiles, deviceprofileexec.DefaultBLEScanner{})
}

func (m *Manager) scanNativeBLEDevices(ctx context.Context, profiles []types.DeviceProfile) ([]map[string]string, error) {
	scanner := m.bleScanner
	if scanner == nil {
		scanner = deviceprofileexec.DefaultBLEScanner{}
	}
	return scanNativeBLEDevicesWithScanner(ctx, profiles, scanner)
}

func scanNativeBLEDevicesWithScanner(ctx context.Context, profiles []types.DeviceProfile, scanner deviceprofileexec.BLEScanner) ([]map[string]string, error) {
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
	bleDevices, err := deviceprofileexec.ScanBLEDevicesWithScanner(ctx, scanner, types.BLEScanParams{
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

// IdleBLEScanCooldown is retained for callers that display the legacy setting.
func IdleBLEScanCooldown() time.Duration { return idleBLEScanCooldown }

// SeedLastAutoBLEScanAt accepts the legacy global timestamp but it no longer
// participates in per-profile BLE failure backoff.
func (m *Manager) SeedLastAutoBLEScanAt(at time.Time) {
	if m == nil || at.IsZero() {
		return
	}
	m.mutex.Lock()
	if at.After(m.lastAutoBLEScanAt) {
		m.lastAutoBLEScanAt = at
	}
	m.mutex.Unlock()
}

func (m *Manager) LastAutoBLEScanAt() time.Time {
	if m == nil {
		return time.Time{}
	}
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.lastAutoBLEScanAt
}
