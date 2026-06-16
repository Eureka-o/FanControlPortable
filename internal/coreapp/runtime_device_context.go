package coreapp

import (
	"strings"

	"github.com/TIANLI0/THRM/internal/types"
)

func deviceCurveScopeKeyForProfile(profile types.DeviceProfile) string {
	transport := types.NormalizeDeviceTransport(profile.Transport)
	profileID := strings.TrimSpace(profile.ID)
	if profileID == "" {
		profileID = strings.TrimSpace(profile.Model)
	}
	if profileID == "" {
		profileID = strings.TrimSpace(profile.DisplayName)
	}
	if profileID == "" {
		profileID = transport
	}
	if profileID == "" {
		return ""
	}
	return transport + deviceCurveScopeSeparator + profileID
}

func (a *CoreApp) connectedRuntimeDeviceProfile() (types.DeviceProfile, bool) {
	if a == nil || a.deviceManager == nil || !a.deviceManager.IsConnected() {
		return types.DeviceProfile{}, false
	}
	profile := types.NormalizeDeviceProfile(a.deviceManager.ActiveProfile(), "")
	if strings.TrimSpace(profile.ID) == "" && strings.TrimSpace(profile.DisplayName) == "" {
		return types.DeviceProfile{}, false
	}
	return profile, true
}

func (a *CoreApp) activeDeviceSpeedUnit(cfg *types.AppConfig) string {
	if profile, ok := a.connectedRuntimeDeviceProfile(); ok {
		if unit := strings.TrimSpace(profile.SpeedUnit); unit != "" {
			return types.NormalizeFanSpeedUnit(unit)
		}
		if unit := strings.TrimSpace(profile.Capabilities.SpeedUnit); unit != "" {
			return types.NormalizeFanSpeedUnit(unit)
		}
	}
	if a != nil && a.deviceManager != nil && a.deviceManager.IsConnected() {
		if fanData := a.deviceManager.GetCurrentFanData(); fanData != nil {
			if unit := strings.TrimSpace(fanData.SpeedUnit); unit != "" {
				return types.NormalizeFanSpeedUnit(unit)
			}
		}
	}
	return types.DeviceProfileSpeedUnit(cfg)
}

func (a *CoreApp) activeDeviceCurveScopeKey(cfg types.AppConfig) string {
	if profile, ok := a.connectedRuntimeDeviceProfile(); ok {
		if key := deviceCurveScopeKeyForProfile(profile); key != "" {
			return key
		}
	}
	return deviceCurveScopeKey(cfg)
}
