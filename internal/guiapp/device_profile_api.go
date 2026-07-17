package guiapp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TIANLI0/THRM/internal/ipc"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) GetDeviceProfiles() DeviceProfilesPayload {
	resp, err := a.sendRequest(ipc.ReqGetDeviceProfiles, nil)
	if err != nil || !resp.Success {
		cfg, cfgErr := a.GetConfig()
		if cfgErr != nil {
			guiLogger.Errorf("获取设备配置失败: %v", cfgErr)
			return DeviceProfilesPayload{}
		}
		return DeviceProfilesPayload{
			Profiles:             cfg.DeviceProfiles,
			ActiveID:             cfg.ActiveDeviceProfileID,
			ActiveIDsByTransport: cfg.ActiveDeviceProfileIDsByTransport,
		}
	}
	var payload DeviceProfilesPayload
	json.Unmarshal(resp.Data, &payload)
	return payload
}

func (a *App) GetSupportedDeviceProfiles() []DeviceProfile {
	resp, err := a.sendRequest(ipc.ReqGetSupportedDeviceProfiles, nil)
	if err != nil || !resp.Success {
		return nil
	}
	var profiles []DeviceProfile
	json.Unmarshal(resp.Data, &profiles)
	return profiles
}

func (a *App) GetUserDeviceProfiles() []DeviceProfile {
	resp, err := a.sendRequest(ipc.ReqGetUserDeviceProfiles, nil)
	if err != nil || !resp.Success {
		return nil
	}
	var profiles []DeviceProfile
	json.Unmarshal(resp.Data, &profiles)
	return profiles
}

func (a *App) SetActiveDeviceProfile(profileID string) (DeviceProfile, error) {
	resp, err := a.sendRequest(ipc.ReqSetActiveDeviceProfile, ipc.SetActiveDeviceProfileParams{ID: profileID})
	if err != nil {
		return DeviceProfile{}, err
	}
	if !resp.Success {
		return DeviceProfile{}, fmt.Errorf("%s", resp.Error)
	}
	var profile DeviceProfile
	json.Unmarshal(resp.Data, &profile)
	return profile, nil
}

func (a *App) SaveDeviceProfile(profile DeviceProfile, setActive bool) (DeviceProfile, error) {
	resp, err := a.sendRequest(ipc.ReqSaveDeviceProfile, ipc.SaveDeviceProfileParams{
		Profile:   profile,
		SetActive: setActive,
	})
	if err != nil {
		return DeviceProfile{}, err
	}
	if !resp.Success {
		return DeviceProfile{}, fmt.Errorf("%s", resp.Error)
	}
	var saved DeviceProfile
	json.Unmarshal(resp.Data, &saved)
	return saved, nil
}

func (a *App) DeleteDeviceProfile(profileID string) error {
	resp, err := a.sendRequest(ipc.ReqDeleteDeviceProfile, ipc.DeleteDeviceProfileParams{ID: profileID})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (a *App) ExportDeviceProfiles() (string, error) {
	resp, err := a.sendRequest(ipc.ReqExportDeviceProfiles, nil)
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("%s", resp.Error)
	}
	var code string
	json.Unmarshal(resp.Data, &code)
	return code, nil
}

func (a *App) ExportDeviceProfilesToFile() (string, error) {
	code, err := a.ExportDeviceProfiles()
	if err != nil {
		return "", err
	}
	if a.ctx == nil {
		return "", fmt.Errorf("window context is not ready")
	}

	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出设备信息",
		DefaultFilename: "FanControl-devices.fcdp",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "FanControl Device Files (*.fcdp)", Pattern: "*.fcdp"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if filepath.Ext(path) == "" {
		path += ".fcdp"
	}
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (a *App) ImportDeviceProfiles(code string) error {
	resp, err := a.sendRequest(ipc.ReqImportDeviceProfiles, ipc.ImportDeviceProfilesParams{Code: code})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (a *App) TestDeviceProfile(params DeviceProfileTestParams) (DeviceProfileTestResult, error) {
	resp, err := a.sendRequest(ipc.ReqTestDeviceProfile, ipc.TestDeviceProfileParams{
		Profile:    params.Profile,
		Action:     params.Action,
		SpeedValue: params.SpeedValue,
		TimeoutMs:  params.TimeoutMs,
	})
	if err != nil {
		return DeviceProfileTestResult{}, err
	}
	if !resp.Success {
		return DeviceProfileTestResult{}, fmt.Errorf("%s", resp.Error)
	}
	var result DeviceProfileTestResult
	json.Unmarshal(resp.Data, &result)
	return result, nil
}
