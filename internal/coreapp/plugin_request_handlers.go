package coreapp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/ipc"
	pluginpkg "github.com/TIANLI0/THRM/internal/plugins"
	"github.com/TIANLI0/THRM/internal/types"
)

type pluginIDParams struct {
	ID       string `json:"id"`
	PluginID string `json:"pluginId"`
}

func (a *CoreApp) handlePluginIPCRequest(req ipc.Request) (ipc.Response, bool) {
	switch req.Type {
	case ipc.ReqGetAvailablePlugins:
		return a.dataResponse(a.availablePluginsWithRuntimeStatus()), true

	case ipc.ReqGetPluginStatus:
		pluginID, err := parsePluginID(req.Data)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		info, ok := a.pluginInfoWithRuntimeStatus(pluginID)
		if !ok {
			return a.errorResponse(fmt.Sprintf("plugin not found: %s", pluginID)), true
		}
		return a.dataResponse(info), true

	case ipc.ReqEnablePlugin:
		return a.setPluginEnabled(req.Data, true), true

	case ipc.ReqDisablePlugin:
		return a.setPluginEnabled(req.Data, false), true

	case ipc.ReqRefreshPluginDiscovery:
		discovered, err := pluginpkg.ScanPluginsDir(filepath.Join(config.GetInstallDir(), "plugins"))
		if err != nil {
			a.logError("plugin discovery refresh reported errors: %v", err)
		}
		a.updatePluginDiscoverySnapshot(discovered, true)
		return a.dataResponse(a.availablePluginsWithRuntimeStatus()), true

	default:
		return ipc.Response{}, false
	}
}

func (a *CoreApp) availablePluginsWithRuntimeStatus() []types.PluginInfo {
	snapshot := a.availablePluginsSnapshot()
	for i := range snapshot {
		snapshot[i] = a.applyRuntimePluginStatus(snapshot[i])
	}
	return snapshot
}

func (a *CoreApp) pluginInfoWithRuntimeStatus(pluginID string) (types.PluginInfo, bool) {
	pluginID = strings.TrimSpace(pluginID)
	for _, info := range a.availablePluginsSnapshot() {
		if info.ID == pluginID {
			return a.applyRuntimePluginStatus(info), true
		}
	}
	if plugin := a.runtimePlugin(pluginID); plugin != nil {
		status := plugin.Status()
		return types.PluginInfo{
			ID:        status.ID,
			Name:      status.Name,
			Status:    runtimePluginStatus(status),
			Installed: true,
			Supported: true,
			Running:   status.Running,
			LastError: status.LastError,
		}, true
	}
	return types.PluginInfo{}, false
}

func (a *CoreApp) applyRuntimePluginStatus(info types.PluginInfo) types.PluginInfo {
	plugin := a.runtimePlugin(info.ID)
	if plugin == nil {
		return info
	}
	status := plugin.Status()
	if status.Name != "" {
		info.Name = status.Name
	}
	info.Status = runtimePluginStatus(status)
	info.Running = status.Running
	info.LastError = status.LastError
	info.Supported = true
	return info
}

func (a *CoreApp) runtimePlugin(pluginID string) pluginpkg.Plugin {
	if a == nil || a.pluginManager == nil || strings.TrimSpace(pluginID) == "" {
		return nil
	}
	return a.pluginManager.Plugin(pluginID)
}

func runtimePluginStatus(status pluginpkg.Status) string {
	if status.LastError != "" {
		return "error"
	}
	if status.Running {
		return "running"
	}
	return "stopped"
}

func (a *CoreApp) setPluginEnabled(data []byte, enabled bool) ipc.Response {
	pluginID, err := parsePluginID(data)
	if err != nil {
		return a.errorResponse(err.Error())
	}
	if a.runtimePlugin(pluginID) == nil {
		return a.errorResponse(fmt.Sprintf("runtime plugin is not registered: %s", pluginID))
	}

	if enabled {
		if err := a.pluginManager.Start(pluginID); err != nil {
			return a.errorResponse(err.Error())
		}
	} else {
		if err := a.pluginManager.Stop(pluginID); err != nil {
			return a.errorResponse(err.Error())
		}
	}
	return a.successResponse(true)
}

func parsePluginID(data []byte) (string, error) {
	var params pluginIDParams
	if err := json.Unmarshal(data, &params); err != nil {
		return "", fmt.Errorf("parse params failed: %w", err)
	}
	pluginID := strings.TrimSpace(params.PluginID)
	if pluginID == "" {
		pluginID = strings.TrimSpace(params.ID)
	}
	if pluginID == "" {
		return "", fmt.Errorf("plugin id is required")
	}
	return pluginID, nil
}
