package coreapp

import (
	"strconv"
	"strings"

	"github.com/TIANLI0/THRM/internal/deviceprofiles"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

func connectedFlyDigiProfileID(deviceInfo map[string]string) string {
	if len(deviceInfo) == 0 {
		return ""
	}
	transport := types.NormalizeDeviceTransport(deviceInfo["transport"])
	if transport == "" {
		if strings.TrimSpace(deviceInfo["productId"]) != "" {
			transport = types.DeviceTransportHID
		} else if strings.EqualFold(strings.TrimSpace(deviceInfo["model"]), "BS1") {
			transport = types.DeviceTransportBLE
		}
	}

	if transport == types.DeviceTransportHID {
		if productID, ok := parseHexUint16(deviceInfo["productId"]); ok {
			if id := types.FlyDigiProfileIDForHIDProductID(productID); id != "" {
				return id
			}
		}
	}

	if transport == types.DeviceTransportBLE || transport == types.DeviceTransportHID {
		return types.FlyDigiProfileIDForModel(deviceInfo["model"])
	}
	return ""
}

func parseHexUint16(value string) (uint16, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	value = strings.TrimPrefix(strings.ToLower(value), "0x")
	parsed, err := strconv.ParseUint(value, 16, 16)
	if err != nil {
		return 0, false
	}
	return uint16(parsed), true
}

func (a *CoreApp) syncConnectedBuiltInDeviceProfile(deviceInfo map[string]string) bool {
	profileID := connectedFlyDigiProfileID(deviceInfo)
	if profileID == "" {
		return false
	}
	cfg := a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)
	idx := deviceprofiles.FindIndex(cfg.DeviceProfiles, profileID)
	if idx < 0 {
		return false
	}
	profile := cfg.DeviceProfiles[idx]
	transport := types.NormalizeDeviceTransport(profile.Transport)
	changed := false
	if cfg.ActiveDeviceProfileID != profile.ID {
		cfg.ActiveDeviceProfileID = profile.ID
		changed = true
	}
	if cfg.DeviceTransport != transport {
		cfg.DeviceTransport = transport
		changed = true
	}
	if cfg.ActiveDeviceProfileIDsByTransport == nil {
		cfg.ActiveDeviceProfileIDsByTransport = map[string]string{}
	}
	if cfg.ActiveDeviceProfileIDsByTransport[transport] != profile.ID {
		cfg.ActiveDeviceProfileIDsByTransport[transport] = profile.ID
		changed = true
	}
	if !changed {
		return false
	}

	a.configManager.Set(cfg)
	if err := a.configManager.Save(); err != nil {
		a.logError("failed to save auto-matched device profile %s: %v", profile.ID, err)
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return true
}
