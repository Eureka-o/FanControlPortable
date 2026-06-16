package coreapp

import (
	"strings"

	"github.com/TIANLI0/THRM/internal/types"
)

func connectedDeviceDisplayName(profile types.DeviceProfile, model string, settings *types.DeviceSettings, fallback string) string {
	candidates := []string{
		profile.DisplayName,
		profile.Capabilities.DisplayName,
		profile.Model,
		model,
	}
	if settings != nil {
		candidates = append(candidates, settings.Model)
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
