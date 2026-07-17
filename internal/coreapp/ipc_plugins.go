package coreapp

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/plugins"
)

func (a *CoreApp) handlePluginIPCRequest(req ipc.Request) (ipc.Response, bool) {
	switch req.Type {
	case ipc.ReqGetPluginSnapshot:
		return a.dataResponse(a.currentPluginSnapshot()), true

	case ipc.ReqRefreshPlugins:
		return a.dataResponse(a.refreshPluginCatalog(true)), true

	case ipc.ReqSetPluginEnabled:
		var params ipc.SetPluginEnabledParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("无效的插件启用参数"), true
		}
		snapshot, err := a.setPluginEnabled(params.ID, params.Enabled)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(snapshot), true

	case ipc.ReqDeletePlugin:
		var params ipc.PluginIDParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("无效的插件删除参数"), true
		}
		snapshot, err := a.deletePlugin(params.ID)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(snapshot), true

	case ipc.ReqResetPlugin:
		var params ipc.PluginIDParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("无效的插件重置参数"), true
		}
		if err := a.resetPlugin(params.ID); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(a.currentPluginSnapshot()), true

	case ipc.ReqInvokePlugin:
		var params ipc.InvokePluginParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("无效的插件调用参数"), true
		}
		result, err := a.invokePlugin(params.ID, params.Method, params.Payload)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(result), true

	default:
		return ipc.Response{}, false
	}
}

func (a *CoreApp) currentPluginSnapshot() plugins.CatalogSnapshot {
	if a.pluginCatalog == nil {
		return plugins.CatalogSnapshot{Plugins: []plugins.CatalogEntry{}}
	}
	return a.pluginCatalog.Snapshot()
}

func (a *CoreApp) setPluginEnabled(id string, enabled bool) (plugins.CatalogSnapshot, error) {
	if a.pluginCatalog == nil {
		return plugins.CatalogSnapshot{}, fmt.Errorf("插件注册表未初始化")
	}
	entry, ok := a.pluginCatalog.Entry(id)
	if !ok {
		return a.currentPluginSnapshot(), fmt.Errorf("未找到插件: %s", id)
	}
	if entry.State == plugins.CatalogStateInvalid || entry.State == plugins.CatalogStateIncompatible {
		return a.currentPluginSnapshot(), fmt.Errorf("插件当前状态不允许启用或停用: %s", entry.State)
	}

	cfg := a.configManager.Get()
	if cfg.PluginEnabled == nil {
		cfg.PluginEnabled = map[string]bool{}
	}
	if enabled {
		cfg.PluginEnabled[id] = true
	} else {
		delete(cfg.PluginEnabled, id)
	}
	if err := a.configManager.Update(cfg); err != nil {
		return a.currentPluginSnapshot(), fmt.Errorf("保存插件启用状态失败: %w", err)
	}
	a.syncPluginSupervisor()
	if enabled {
		if _, err := a.pluginCatalog.SetEnabled(id, true); err != nil {
			return a.currentPluginSnapshot(), err
		}
		if a.pluginSupervisor != nil {
			a.pluginSupervisor.Start(a.ctx)
			if err := a.pluginSupervisor.Enable(id); err != nil {
				a.logError("启动插件 %s 失败: %v", id, err)
			}
		}
		return a.currentPluginSnapshot(), nil
	}

	if a.pluginSupervisor != nil {
		if err := a.pluginSupervisor.Disable(id, "disabled"); err != nil {
			a.logError("停用插件 %s 时后端恢复未确认: %v", id, err)
		}
	}
	return a.pluginCatalog.SetEnabled(id, false)
}

func (a *CoreApp) deletePlugin(id string) (plugins.CatalogSnapshot, error) {
	if a.pluginCatalog == nil {
		return plugins.CatalogSnapshot{}, fmt.Errorf("插件注册表未初始化")
	}
	entry, ok := a.pluginCatalog.Entry(id)
	if !ok {
		return a.currentPluginSnapshot(), fmt.Errorf("未找到插件: %s", id)
	}
	if entry.Enabled {
		return a.currentPluginSnapshot(), fmt.Errorf("请先停用插件再删除")
	}
	if a.pluginSupervisor != nil {
		if err := a.pluginSupervisor.Remove(id, "delete"); err != nil {
			a.logError("删除插件 %s 前停止后端失败: %v", id, err)
		}
	}
	snapshot, err := a.pluginCatalog.Delete(id)
	if err != nil {
		return a.currentPluginSnapshot(), err
	}
	cfg := a.configManager.Get()
	if cfg.PluginEnabled != nil {
		delete(cfg.PluginEnabled, id)
		if err := a.configManager.Update(cfg); err != nil {
			return snapshot, fmt.Errorf("插件已删除，但清理启用状态失败: %w", err)
		}
	}
	return snapshot, nil
}

func (a *CoreApp) resetPlugin(id string) error {
	if a.pluginCatalog == nil {
		return fmt.Errorf("插件注册表未初始化")
	}
	entry, ok := a.pluginCatalog.Entry(id)
	if !ok {
		return fmt.Errorf("未找到插件: %s", id)
	}
	if entry.Enabled {
		return fmt.Errorf("请先停用插件再重置")
	}
	dataRoot := filepath.Join(config.GetInstallDir(), "config", "plugins")
	if err := plugins.ResetPluginData(dataRoot, id); err != nil {
		return fmt.Errorf("重置插件数据失败: %w", err)
	}
	return nil
}
