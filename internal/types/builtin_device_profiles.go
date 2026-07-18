package types

import "strings"

func BuiltInDeviceProfileByID(profileID string) (DeviceProfile, bool) {
	if profile, ok := FlyDigiProfileByID(profileID); ok {
		return profile, true
	}
	switch strings.TrimSpace(profileID) {
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
	return profiles
}

func IsBuiltInDeviceProfileID(profileID string) bool {
	if IsFlyDigiDeviceProfileID(profileID) {
		return true
	}
	switch strings.TrimSpace(profileID) {
	case DefaultWiFiPercentProfileID,
		DefaultWiFiPercentTemplateProfileID,
		LegacyRPMProfileID:
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
	default:
		return DeviceProfile{}, false
	}
}

// ensureBuiltInDeviceProfiles 补全用户配置中缺失的内置档案。
//
// 不再用 reflect.DeepEqual 做深层比较并覆盖已有档案,原因:
// 1. 内置档案的内容由源码保证,用户修改 Connection/Capability 等字段是有意行为。
// 2. reflect.DeepEqual 在 Capability 含指针时开销大,且指针稳定性不可控。
// 3. 每次 Load 配置都遍历所有内置档案做深层比较,后台常驻时增加启动延迟。
//
// 策略:仅在内置档案 ID 完全缺失时才追加;已有则保留用户版本。
func ensureBuiltInDeviceProfiles(cfg *AppConfig) bool {
	if cfg == nil {
		return false
	}
	changed := false
	builtIns := FlyDigiBuiltInProfiles()
	if cfg.WiFiCompatibilityEnabled {
		builtIns = append([]DeviceProfile{DefaultWiFiPercentProfile(cfg.FanControlDeviceIp)}, builtIns...)
	}
	for _, builtIn := range builtIns {
		builtIn = NormalizeDeviceProfile(builtIn, cfg.FanControlDeviceIp)
		found := false
		for i := range cfg.DeviceProfiles {
			if cfg.DeviceProfiles[i].ID == builtIn.ID {
				found = true
				break
			}
		}
		if !found {
			cfg.DeviceProfiles = append(cfg.DeviceProfiles, builtIn)
			changed = true
		}
	}
	return changed
}
