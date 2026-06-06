package coreapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/TIANLI0/THRM/internal/deviceprofileexec"
	"github.com/TIANLI0/THRM/internal/deviceprofiles"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

func (a *CoreApp) deviceProfilesPayloadFromConfig(cfg types.AppConfig) types.DeviceProfilesPayload {
	return types.DeviceProfilesPayload{
		Profiles:             deviceprofiles.CloneProfiles(cfg.DeviceProfiles),
		ActiveID:             cfg.ActiveDeviceProfileID,
		ActiveIDsByTransport: deviceprofiles.FilterActiveIDsByTransport(cfg.ActiveDeviceProfileIDsByTransport, cfg.DeviceProfiles),
	}
}

func (a *CoreApp) GetDeviceProfiles() types.DeviceProfilesPayload {
	cfg := a.configManager.Get()
	if types.NormalizeDeviceProfileConfig(&cfg) {
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("failed to save normalized device profiles: %v", err)
		}
	}
	return a.deviceProfilesPayloadFromConfig(cfg)
}

func (a *CoreApp) GetSupportedDeviceProfiles() []types.DeviceProfile {
	return deviceprofiles.SupportedProfiles()
}

func userDeviceProfilesFromConfig(cfg types.AppConfig) []types.DeviceProfile {
	profiles := make([]types.DeviceProfile, 0, len(cfg.DeviceProfiles))
	for _, profile := range cfg.DeviceProfiles {
		if !profile.BuiltIn {
			profiles = append(profiles, deviceprofiles.CloneProfile(profile))
		}
	}
	return profiles
}

func (a *CoreApp) GetUserDeviceProfiles() []types.DeviceProfile {
	cfg := a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)
	return userDeviceProfilesFromConfig(cfg)
}

func (a *CoreApp) SetActiveDeviceProfile(profileID string) (types.DeviceProfile, error) {
	cfg := a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)

	idx := deviceprofiles.FindIndex(cfg.DeviceProfiles, profileID)
	if idx < 0 {
		return types.DeviceProfile{}, fmt.Errorf("device profile not found")
	}

	cfg.ActiveDeviceProfileID = cfg.DeviceProfiles[idx].ID
	cfg.DeviceTransport = cfg.DeviceProfiles[idx].Transport
	if cfg.ActiveDeviceProfileIDsByTransport == nil {
		cfg.ActiveDeviceProfileIDsByTransport = map[string]string{}
	}
	cfg.ActiveDeviceProfileIDsByTransport[types.NormalizeDeviceTransport(cfg.DeviceProfiles[idx].Transport)] = cfg.DeviceProfiles[idx].ID
	if err := a.UpdateConfig(cfg); err != nil {
		return types.DeviceProfile{}, err
	}

	cfg = a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)
	idx = deviceprofiles.FindIndex(cfg.DeviceProfiles, profileID)
	if idx < 0 {
		return types.DeviceProfile{}, fmt.Errorf("device profile not found after update")
	}
	return deviceprofiles.CloneProfile(cfg.DeviceProfiles[idx]), nil
}

func (a *CoreApp) SaveDeviceProfile(params ipc.SaveDeviceProfileParams) (types.DeviceProfile, error) {
	cfg := a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)

	profile := params.Profile
	if strings.TrimSpace(profile.ID) == "" {
		profile.ID = deviceprofiles.GenerateID()
	}
	profile, err := deviceprofiles.NormalizeAndValidate(profile, cfg.FanControlDeviceIp)
	if err != nil {
		return types.DeviceProfile{}, err
	}

	idx := deviceprofiles.FindIndex(cfg.DeviceProfiles, profile.ID)
	if idx < 0 {
		cfg.DeviceProfiles = append(cfg.DeviceProfiles, profile)
	} else {
		cfg.DeviceProfiles[idx] = profile
	}

	if params.SetActive || cfg.ActiveDeviceProfileID == profile.ID {
		cfg.ActiveDeviceProfileID = profile.ID
		cfg.DeviceTransport = profile.Transport
		if cfg.ActiveDeviceProfileIDsByTransport == nil {
			cfg.ActiveDeviceProfileIDsByTransport = map[string]string{}
		}
		cfg.ActiveDeviceProfileIDsByTransport[types.NormalizeDeviceTransport(profile.Transport)] = profile.ID
	}

	if err := a.UpdateConfig(cfg); err != nil {
		return types.DeviceProfile{}, err
	}

	cfg = a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)
	idx = deviceprofiles.FindIndex(cfg.DeviceProfiles, profile.ID)
	if idx < 0 {
		return types.DeviceProfile{}, fmt.Errorf("device profile not found after save")
	}
	return deviceprofiles.CloneProfile(cfg.DeviceProfiles[idx]), nil
}

func (a *CoreApp) DeleteDeviceProfile(profileID string) error {
	cfg := a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)

	if len(cfg.DeviceProfiles) <= 1 {
		return fmt.Errorf("at least one device profile must be kept")
	}

	idx := deviceprofiles.FindIndex(cfg.DeviceProfiles, profileID)
	if idx < 0 {
		return fmt.Errorf("device profile not found")
	}
	if cfg.DeviceProfiles[idx].BuiltIn || deviceprofiles.IsBuiltInProfileID(cfg.DeviceProfiles[idx].ID) {
		return fmt.Errorf("built-in device profiles cannot be deleted")
	}
	deletedTransport := types.NormalizeDeviceTransport(cfg.DeviceProfiles[idx].Transport)

	cfg.DeviceProfiles = append(cfg.DeviceProfiles[:idx], cfg.DeviceProfiles[idx+1:]...)
	for transport, activeID := range cfg.ActiveDeviceProfileIDsByTransport {
		if activeID == profileID {
			delete(cfg.ActiveDeviceProfileIDsByTransport, transport)
		}
	}
	if cfg.ActiveDeviceProfileID == profileID {
		nextIdx := deviceprofiles.FindFirstByTransport(cfg.DeviceProfiles, deletedTransport)
		if nextIdx < 0 {
			nextIdx = idx
			if nextIdx >= len(cfg.DeviceProfiles) {
				nextIdx = len(cfg.DeviceProfiles) - 1
			}
		}
		cfg.ActiveDeviceProfileID = cfg.DeviceProfiles[nextIdx].ID
		cfg.DeviceTransport = cfg.DeviceProfiles[nextIdx].Transport
		if cfg.ActiveDeviceProfileIDsByTransport == nil {
			cfg.ActiveDeviceProfileIDsByTransport = map[string]string{}
		}
		cfg.ActiveDeviceProfileIDsByTransport[types.NormalizeDeviceTransport(cfg.DeviceProfiles[nextIdx].Transport)] = cfg.DeviceProfiles[nextIdx].ID
	}

	return a.UpdateConfig(cfg)
}

func (a *CoreApp) ExportDeviceProfiles() (string, error) {
	cfg := a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)
	userProfiles := userDeviceProfilesFromConfig(cfg)
	activeID := cfg.ActiveDeviceProfileID
	if deviceprofiles.FindIndex(userProfiles, activeID) < 0 {
		activeID = ""
	}
	return deviceprofiles.ExportWithActiveIDs(activeID, cfg.ActiveDeviceProfileIDsByTransport, userProfiles)
}

func (a *CoreApp) ImportDeviceProfiles(code string) error {
	imported, activeID, activeIDsByTransport, err := deviceprofiles.ImportWithActiveIDs(code)
	if err != nil {
		return err
	}

	cfg := a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)

	userImported := make([]types.DeviceProfile, 0, len(imported))
	for _, profile := range imported {
		if profile.BuiltIn || deviceprofiles.IsBuiltInProfileID(profile.ID) {
			continue
		}
		userImported = append(userImported, profile)
	}
	if len(userImported) == 0 {
		return fmt.Errorf("device import contains no user devices")
	}

	for _, profile := range userImported {
		idx := deviceprofiles.FindIndex(cfg.DeviceProfiles, profile.ID)
		if idx < 0 {
			cfg.DeviceProfiles = append(cfg.DeviceProfiles, profile)
			continue
		}
		cfg.DeviceProfiles[idx] = profile
	}

	if cfg.ActiveDeviceProfileIDsByTransport == nil {
		cfg.ActiveDeviceProfileIDsByTransport = map[string]string{}
	}
	for transport, profileID := range deviceprofiles.FilterActiveIDsByTransport(activeIDsByTransport, userImported) {
		if deviceprofiles.FindIndex(cfg.DeviceProfiles, profileID) >= 0 {
			cfg.ActiveDeviceProfileIDsByTransport[transport] = profileID
		}
	}
	if activeID != "" && deviceprofiles.FindIndex(userImported, activeID) >= 0 && deviceprofiles.FindIndex(cfg.DeviceProfiles, activeID) >= 0 {
		cfg.ActiveDeviceProfileID = activeID
		if idx := deviceprofiles.FindIndex(cfg.DeviceProfiles, activeID); idx >= 0 {
			cfg.DeviceTransport = cfg.DeviceProfiles[idx].Transport
			cfg.ActiveDeviceProfileIDsByTransport[types.NormalizeDeviceTransport(cfg.DeviceProfiles[idx].Transport)] = activeID
		}
	}

	return a.UpdateConfig(cfg)
}

func (a *CoreApp) TestDeviceProfile(params ipc.TestDeviceProfileParams) (types.DeviceProfileTestResult, error) {
	cfg := a.configManager.Get()
	profile := params.Profile
	if strings.TrimSpace(profile.ID) == "" {
		profile.ID = "test.device.profile"
	}
	if strings.TrimSpace(profile.DisplayName) == "" {
		profile.DisplayName = "Draft device profile"
	}

	profile, err := deviceprofiles.NormalizeAndValidate(profile, cfg.FanControlDeviceIp)
	if err != nil {
		return types.DeviceProfileTestResult{}, err
	}

	tester := deviceprofileexec.ProfileTester{
		FallbackEndpoint: cfg.FanControlDeviceIp,
		HTTPClient:       deviceprofileexec.NewHTTPClient(),
	}
	return tester.Test(context.Background(), types.DeviceProfileTestParams{
		Profile:    profile,
		Action:     params.Action,
		SpeedValue: params.SpeedValue,
		TimeoutMs:  params.TimeoutMs,
	})
}
