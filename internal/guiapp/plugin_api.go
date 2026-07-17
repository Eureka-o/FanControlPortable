package guiapp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"

	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/plugins"
)

func (a *App) GetPluginSnapshot() (plugins.CatalogSnapshot, error) {
	return a.pluginSnapshotRequest(ipc.ReqGetPluginSnapshot, nil)
}

func (a *App) RefreshPlugins() (plugins.CatalogSnapshot, error) {
	return a.pluginSnapshotRequest(ipc.ReqRefreshPlugins, nil)
}

func (a *App) SetPluginEnabled(id string, enabled bool) (plugins.CatalogSnapshot, error) {
	return a.pluginSnapshotRequest(ipc.ReqSetPluginEnabled, ipc.SetPluginEnabledParams{ID: id, Enabled: enabled})
}

func (a *App) DeletePlugin(id string) (plugins.CatalogSnapshot, error) {
	return a.pluginSnapshotRequest(ipc.ReqDeletePlugin, ipc.PluginIDParams{ID: id})
}

func (a *App) ResetPlugin(id string) (plugins.CatalogSnapshot, error) {
	return a.pluginSnapshotRequest(ipc.ReqResetPlugin, ipc.PluginIDParams{ID: id})
}

func (a *App) InvokePlugin(id, method string, payload any) (any, error) {
	payloadData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化插件调用参数失败: %w", err)
	}
	resp, err := a.sendRequest(ipc.ReqInvokePlugin, ipc.InvokePluginParams{
		ID:      id,
		Method:  method,
		Payload: payloadData,
	})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	var result any
	if len(resp.Data) == 0 {
		return map[string]any{}, nil
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("解析插件调用结果失败: %w", err)
	}
	return result, nil
}

func (a *App) pluginSnapshotRequest(requestType ipc.RequestType, payload any) (plugins.CatalogSnapshot, error) {
	resp, err := a.sendRequest(requestType, payload)
	if err != nil {
		return plugins.CatalogSnapshot{}, err
	}
	if !resp.Success {
		return plugins.CatalogSnapshot{}, fmt.Errorf("%s", resp.Error)
	}
	var snapshot plugins.CatalogSnapshot
	if err := json.Unmarshal(resp.Data, &snapshot); err != nil {
		return plugins.CatalogSnapshot{}, err
	}
	if snapshot.Plugins == nil {
		snapshot.Plugins = []plugins.CatalogEntry{}
	}
	return snapshot, nil
}

func (a *App) OpenPluginsFolder() error {
	dir := filepath.Join(config.GetInstallDir(), "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建插件目录失败: %w", err)
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
		return fmt.Errorf("打开插件目录失败: %w", err)
	}
	return nil
}
