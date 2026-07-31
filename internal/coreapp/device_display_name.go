package coreapp

import (
	"strings"

	"github.com/TIANLI0/THRM/internal/types"
)

func connectedDeviceDisplayName(profile types.DeviceProfile, model string, settings *types.DeviceSettings, fallback string) string {
	deviceModel := strings.TrimSpace(profile.Model)
	if deviceModel == "" {
		deviceModel = strings.TrimSpace(model)
	}
	if deviceModel == "" && settings != nil {
		deviceModel = strings.TrimSpace(settings.Model)
	}

	vendor := strings.TrimSpace(profile.Vendor)
	if profile.ID == types.DefaultWiFiPercentProfileID {
		vendor = ""
	}
	if vendor != "" && deviceModel != "" {
		lowerVendor := strings.ToLower(vendor)
		if strings.Contains(strings.ToLower(deviceModel), lowerVendor) {
			return deviceModel
		}
		if displayName := strings.TrimSpace(profile.DisplayName); displayName != "" {
			if strings.EqualFold(displayName, vendor+deviceModel) {
				return displayName
			}
		}
		return vendor + " " + deviceModel
	}

	candidates := []string{
		deviceModel,
		profile.DisplayName,
		profile.Capabilities.DisplayName,
	}
	candidates = append(candidates, profile.ID, fallback)

	for _, candidate := range candidates {
		if name := strings.TrimSpace(candidate); name != "" {
			return name
		}
	}
	return ""
}

func eventPayloadString(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
