package guiapp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"

	"github.com/Eureka-o/FanControlPortable/internal/config"
	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/plugins"
)

func decodePluginsResponse(resp *ipc.Response) ([]plugins.PluginInfo, error) {
	if resp == nil {
		return nil, fmt.Errorf("empty ipc response")
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	var list []plugins.PluginInfo
	if err := json.Unmarshal(resp.Data, &list); err != nil {
		return nil, fmt.Errorf("decode plugins: %w", err)
	}
	if list == nil {
		return []plugins.PluginInfo{}, nil
	}
	return list, nil
}

// GetPlugins returns the unified list of built-in and discovered external plugins.
func (a *App) GetPlugins() ([]plugins.PluginInfo, error) {
	resp, err := a.sendRequest(ipc.ReqGetPlugins, nil)
	if err != nil {
		return nil, err
	}
	list, err := decodePluginsResponse(resp)
	if err == nil {
		devDirs := make([]string, 0)
		for _, item := range list {
			if item.DevMode && item.Path != "" {
				devDirs = append(devDirs, item.Path)
			}
		}
		a.SetCachedPluginDevDirs(devDirs)
	}
	return list, err
}

// RefreshPlugins rescans the plugins directory and returns the updated plugin list.
func (a *App) RefreshPlugins() ([]plugins.PluginInfo, error) {
	resp, err := a.sendRequest(ipc.ReqRefreshPlugins, nil)
	if err != nil {
		return nil, err
	}
	return decodePluginsResponse(resp)
}

// SetPluginEnabled enables or disables a built-in or external plugin.
func (a *App) SetPluginEnabled(pluginID string, enabled bool) ([]plugins.PluginInfo, error) {
	resp, err := a.sendRequest(ipc.ReqSetPluginEnabled, ipc.SetPluginEnabledParams{
		PluginID: pluginID,
		Enabled:  enabled,
	})
	if err != nil {
		return nil, err
	}
	return decodePluginsResponse(resp)
}

// InvokePlugin calls a method on a running external plugin backend.
func (a *App) InvokePlugin(pluginID, method, payloadJSON string) (string, error) {
	var raw json.RawMessage
	if trimmed := []byte(payloadJSON); len(trimmed) > 0 {
		raw = json.RawMessage(trimmed)
	} else {
		raw = json.RawMessage(`{}`)
	}
	resp, err := a.sendRequest(ipc.ReqInvokePlugin, ipc.InvokePluginParams{
		PluginID: pluginID,
		Method:   method,
		Payload:  raw,
	})
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("%s", resp.Error)
	}
	var inner json.RawMessage
	if err := json.Unmarshal(resp.Data, &inner); err == nil && len(inner) > 0 {
		return string(inner), nil
	}
	if len(resp.Data) == 0 {
		return "{}", nil
	}
	return string(resp.Data), nil
}

// RestartPlugin restarts a running sidecar plugin process.
func (a *App) RestartPlugin(pluginID string) ([]plugins.PluginInfo, error) {
	resp, err := a.sendRequest(ipc.ReqRestartPlugin, ipc.PluginIDParams{PluginID: pluginID})
	if err != nil {
		return nil, err
	}
	return decodePluginsResponse(resp)
}

// GetPluginSettings returns merged schema defaults + saved settings JSON for one plugin.
func (a *App) GetPluginSettings(pluginID string) (map[string]any, error) {
	resp, err := a.sendRequest(ipc.ReqGetPluginSettings, ipc.PluginIDParams{PluginID: pluginID})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	var out map[string]any
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return nil, fmt.Errorf("decode plugin settings: %w", err)
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

// SavePluginSettings persists plugin settings JSON and pushes settings-changed to running plugins.
func (a *App) SavePluginSettings(pluginID string, settings map[string]any) error {
	resp, err := a.sendRequest(ipc.ReqSavePluginSettings, ipc.SavePluginSettingsParams{
		PluginID: pluginID,
		Settings: settings,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// GetPluginLogs returns recent stderr/lifecycle lines for a sidecar plugin.
func (a *App) GetPluginLogs(pluginID string) ([]string, error) {
	resp, err := a.sendRequest(ipc.ReqGetPluginLogs, ipc.PluginIDParams{PluginID: pluginID})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	var lines []string
	if err := json.Unmarshal(resp.Data, &lines); err != nil {
		return nil, fmt.Errorf("decode plugin logs: %w", err)
	}
	if lines == nil {
		return []string{}, nil
	}
	return lines, nil
}

// SetPluginDevPaths saves Dev Mode local plugin directories and rescans plugins.
func (a *App) SetPluginDevPaths(paths []string) ([]plugins.PluginInfo, error) {
	a.SetCachedPluginDevDirs(paths)
	resp, err := a.sendRequest(ipc.ReqSetPluginDevPaths, ipc.SetPluginDevPathsParams{Paths: paths})
	if err != nil {
		return nil, err
	}
	return decodePluginsResponse(resp)
}

// OpenPluginsFolder opens <install-dir>/plugins in the OS file manager.
func (a *App) OpenPluginsFolder() error {
	dir := filepath.Join(config.GetInstallDir(), "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		guiLogger.Warnf("创建插件文件夹失败: %v", err)
	}

	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("打开插件文件夹失败: %w", err)
	}
	return nil
}
