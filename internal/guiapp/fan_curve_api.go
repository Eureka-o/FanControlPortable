package guiapp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// SetFanCurve 设置风扇曲线
func (a *App) SetFanCurve(curve []FanCurvePoint) error {
	resp, err := a.sendRequest(ipc.ReqSetFanCurve, curve)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// ResetLearnedOffsets 清空学习到的曲线偏移
func (a *App) ResetLearnedOffsets() error {
	resp, err := a.sendRequest(ipc.ReqResetLearnedOffsets, nil)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// GetFanCurve 获取风扇曲线
func (a *App) GetFanCurve() []FanCurvePoint {
	resp, err := a.sendRequest(ipc.ReqGetFanCurve, nil)
	if err != nil {
		return types.GetDefaultFanCurve()
	}
	if !resp.Success {
		return types.GetDefaultFanCurve()
	}
	var curve []FanCurvePoint
	json.Unmarshal(resp.Data, &curve)
	return curve
}

// GetFanCurveProfiles 获取曲线方案列表
func (a *App) GetFanCurveProfiles() FanCurveProfilesPayload {
	resp, err := a.sendRequest(ipc.ReqGetFanCurveProfiles, nil)
	if err != nil || !resp.Success {
		cfg, cfgErr := a.GetConfig()
		if cfgErr != nil {
			guiLogger.Errorf("获取曲线配置失败: %v", cfgErr)
			return FanCurveProfilesPayload{}
		}
		return types.FanCurveProfilesPayload{
			Profiles: cfg.FanCurveProfiles,
			ActiveID: cfg.ActiveFanCurveProfileID,
		}
	}
	var payload FanCurveProfilesPayload
	json.Unmarshal(resp.Data, &payload)
	return payload
}

// SetActiveFanCurveProfile 设置当前激活曲线方案
func (a *App) SetActiveFanCurveProfile(profileID string) error {
	resp, err := a.sendRequest(ipc.ReqSetActiveFanCurveProfile, ipc.SetActiveFanCurveProfileParams{ID: profileID})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// SaveFanCurveProfile 保存曲线方案
func (a *App) SaveFanCurveProfile(profileID, name string, curve []FanCurvePoint, setActive bool) (FanCurveProfile, error) {
	resp, err := a.sendRequest(ipc.ReqSaveFanCurveProfile, ipc.SaveFanCurveProfileParams{
		ID:        profileID,
		Name:      name,
		Curve:     curve,
		SetActive: setActive,
	})
	if err != nil {
		return FanCurveProfile{}, err
	}
	if !resp.Success {
		return FanCurveProfile{}, fmt.Errorf("%s", resp.Error)
	}
	var profile FanCurveProfile
	json.Unmarshal(resp.Data, &profile)
	return profile, nil
}

// DeleteFanCurveProfile 删除曲线方案
func (a *App) DeleteFanCurveProfile(profileID string) error {
	resp, err := a.sendRequest(ipc.ReqDeleteFanCurveProfile, ipc.DeleteFanCurveProfileParams{ID: profileID})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// ExportFanCurveProfiles 导出曲线方案
func (a *App) ExportFanCurveProfiles() (string, error) {
	resp, err := a.sendRequest(ipc.ReqExportFanCurveProfiles, nil)
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

// ExportFanCurveProfilesToFile 通过原生另存为窗口导出曲线方案。
func (a *App) ExportFanCurveProfilesToFile() (string, error) {
	code, err := a.ExportFanCurveProfiles()
	if err != nil {
		return "", err
	}
	if a.ctx == nil {
		return "", fmt.Errorf("window context is not ready")
	}

	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出曲线方案",
		DefaultFilename: "FanControl-curve-profiles.fcurve",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "FanControl Curve Files (*.fcurve)", Pattern: "*.fcurve"},
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
		path += ".fcurve"
	}
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// ImportFanCurveProfiles 导入曲线方案
func (a *App) ImportFanCurveProfiles(code string) error {
	resp, err := a.sendRequest(ipc.ReqImportFanCurveProfiles, ipc.ImportFanCurveProfilesParams{Code: code})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}
