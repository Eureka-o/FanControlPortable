package types

import "strings"

func BuiltInDeviceProfileByID(profileID string) (DeviceProfile, bool) {
	if profile, ok := FlyDigiProfileByID(profileID); ok {
		return profile, true
	}
	switch strings.TrimSpace(profileID) {
	case BlackSharkBRB02ProfileID:
		return BlackSharkBRB02Profile(), true
	case BlackSharkBRB02USBProfileID:
		return BlackSharkBRB02USBProfile(), true
	case DefaultWiFiPercentProfileID:
		return DefaultWiFiPercentProfile(DefaultFanDeviceIP), true
	case DefaultWiFiPercentTemplateProfileID:
		return DefaultWiFiPercentTemplateProfile(DefaultFanDeviceIP), true
	default:
		return DeviceProfile{}, false
	}
}

func BuiltInDeviceProfiles(endpoint string) []DeviceProfile {
	profiles := []DeviceProfile{DefaultWiFiPercentProfile(endpoint)}
	profiles = append(profiles, FlyDigiBuiltInProfiles()...)
	profiles = append(profiles, BlackSharkBRB02Profile())
	profiles = append(profiles, BlackSharkBRB02USBProfile())
	return profiles
}

func IsBuiltInDeviceProfileID(profileID string) bool {
	if IsFlyDigiDeviceProfileID(profileID) {
		return true
	}
	switch strings.TrimSpace(profileID) {
	case DefaultWiFiPercentProfileID,
		DefaultWiFiPercentTemplateProfileID,
		LegacyRPMProfileID,
		BlackSharkBRB02ProfileID,
		BlackSharkBRB02USBProfileID:
		return true
	default:
		return false
	}
}

func builtInDeviceProfileForTransport(endpoint, transport string) (DeviceProfile, bool) {
	switch NormalizeDeviceTransport(transport) {
	case DeviceTransportWiFi:
		return DefaultWiFiPercentProfile(endpoint), true
	case DeviceTransportBLE:
		return FlyDigiBS1Profile(), true
	case DeviceTransportHID:
		return FlyDigiBS2Profile(), true
	case DeviceTransportUSB:
		return BlackSharkBRB02USBProfile(), true
	default:
		return DeviceProfile{}, false
	}
}

// ensureBuiltInDeviceProfiles 补全并按需刷新用户配置里的内置档案。
func ensureBuiltInDeviceProfiles(cfg *AppConfig) bool {
	if cfg == nil {
		return false
	}
	changed := false
	builtIns := append(FlyDigiBuiltInProfiles(), BlackSharkBRB02Profile(), BlackSharkBRB02USBProfile())
	if cfg.WiFiCompatibilityEnabled {
		builtIns = append([]DeviceProfile{DefaultWiFiPercentProfile(cfg.FanControlDeviceIp)}, builtIns...)
	}
	for _, builtIn := range builtIns {
		builtIn = NormalizeDeviceProfile(builtIn, cfg.FanControlDeviceIp)
		idx := -1
		for i := range cfg.DeviceProfiles {
			if cfg.DeviceProfiles[i].ID == builtIn.ID {
				idx = i
				break
			}
		}
		if idx < 0 {
			cfg.DeviceProfiles = append(cfg.DeviceProfiles, builtIn)
			changed = true
			continue
		}
		if !cfg.DeviceProfiles[idx].BuiltIn {
			// 用户自造的同名档案，不碰。
			continue
		}
		if builtInProfileMetadataEqual(cfg.DeviceProfiles[idx], builtIn) {
			continue
		}
		cfg.DeviceProfiles[idx] = builtIn
		changed = true
	}
	return changed
}

// builtInProfileMetadataEqual 只比对内置元数据（不比对用户可编辑的连接/命令等字段）。
func builtInProfileMetadataEqual(stored, source DeviceProfile) bool {
	if stored.DisplayName != source.DisplayName ||
		stored.Vendor != source.Vendor ||
		stored.Model != source.Model ||
		stored.Notes != source.Notes ||
		stored.Transport != source.Transport ||
		stored.SpeedUnit != source.SpeedUnit ||
		stored.SpeedRange != source.SpeedRange ||
		stored.Capabilities != source.Capabilities {
		return false
	}
	if len(stored.DisplayFeatures) != len(source.DisplayFeatures) {
		return false
	}
	for i := range stored.DisplayFeatures {
		if stored.DisplayFeatures[i] != source.DisplayFeatures[i] {
			return false
		}
	}
	return true
}
