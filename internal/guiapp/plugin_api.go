package guiapp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/ipc"
	pluginpkg "github.com/TIANLI0/THRM/internal/plugins"
	"github.com/TIANLI0/THRM/internal/types"
)

func (a *App) GetAvailablePlugins() []types.PluginInfo {
	resp, err := a.sendRequest(ipc.ReqGetAvailablePlugins, nil)
	if err != nil {
		guiLogger.Errorf("get available plugins failed: %v", err)
		return nil
	}
	if !resp.Success {
		guiLogger.Errorf("get available plugins failed: %s", resp.Error)
		return nil
	}
	var plugins []types.PluginInfo
	if err := json.Unmarshal(resp.Data, &plugins); err != nil {
		guiLogger.Errorf("parse available plugins failed: %v", err)
		return nil
	}
	return plugins
}

func (a *App) GetPluginStatus(pluginID string) (types.PluginInfo, error) {
	resp, err := a.sendRequest(ipc.ReqGetPluginStatus, map[string]string{"id": pluginID})
	if err != nil {
		return types.PluginInfo{}, err
	}
	if !resp.Success {
		return types.PluginInfo{}, fmt.Errorf("%s", resp.Error)
	}
	var plugin types.PluginInfo
	if err := json.Unmarshal(resp.Data, &plugin); err != nil {
		return types.PluginInfo{}, err
	}
	return plugin, nil
}

func (a *App) EnablePlugin(pluginID string) error {
	resp, err := a.sendRequest(ipc.ReqEnablePlugin, map[string]string{"id": pluginID})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (a *App) DisablePlugin(pluginID string) error {
	resp, err := a.sendRequest(ipc.ReqDisablePlugin, map[string]string{"id": pluginID})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (a *App) RefreshPluginDiscovery() []types.PluginInfo {
	resp, err := a.sendRequest(ipc.ReqRefreshPluginDiscovery, nil)
	if err != nil {
		guiLogger.Errorf("refresh plugin discovery failed: %v", err)
		return nil
	}
	if !resp.Success {
		guiLogger.Errorf("refresh plugin discovery failed: %s", resp.Error)
		return nil
	}
	var plugins []types.PluginInfo
	if err := json.Unmarshal(resp.Data, &plugins); err != nil {
		guiLogger.Errorf("parse refreshed plugin discovery failed: %v", err)
		return nil
	}
	return plugins
}

func (a *App) GetPluginFrontendAsset(pluginID string) (string, error) {
	return a.GetPluginFrontendAssetPath(pluginID, "")
}

func (a *App) GetPluginFrontendAssetPath(pluginID string, assetPath string) (string, error) {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return "", fmt.Errorf("plugin id is required")
	}

	pluginDir := filepath.Join(config.GetInstallDir(), "plugins", pluginID)
	manifest, err := pluginpkg.LoadManifest(pluginDir)
	if err != nil {
		return "", err
	}
	if manifest.ID != pluginID {
		return "", fmt.Errorf("plugin manifest id mismatch")
	}

	var frontendPath string
	if strings.TrimSpace(assetPath) == "" {
		var err error
		frontendPath, err = manifest.FrontendPath(pluginDir)
		if err != nil {
			return "", err
		}
	} else {
		frontendPath = filepath.Clean(filepath.Join(pluginDir, assetPath))
		rel, err := filepath.Rel(pluginDir, frontendPath)
		if err != nil {
			return "", err
		}
		if filepath.IsAbs(assetPath) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("plugin asset path escapes plugin directory")
		}
	}

	data, err := os.ReadFile(frontendPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) GetPluginFrontendHTML(pluginID string) (string, error) {
	return a.GetPluginFrontendAsset(pluginID)
}
