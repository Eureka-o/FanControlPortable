package coreapp

import (
	"context"
	"path/filepath"
	"sort"

	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/ipc"
	pluginpkg "github.com/TIANLI0/THRM/internal/plugins"
	"github.com/TIANLI0/THRM/internal/types"
)

const pluginStatusDiscovered = "discovered"

func (a *CoreApp) startPluginDiscovery() {
	pluginsDir := filepath.Join(config.GetInstallDir(), "plugins")
	discovered, err := pluginpkg.ScanPluginsDir(pluginsDir)
	if err != nil {
		a.logError("插件目录扫描失败: %v", err)
	}
	a.updatePluginDiscoverySnapshot(discovered, true)

	ctx, cancel := context.WithCancel(a.ctx)
	a.pluginDiscoveryMutex.Lock()
	if a.pluginDiscoveryCancel != nil {
		a.pluginDiscoveryCancel()
	}
	a.pluginDiscoveryCancel = cancel
	a.pluginDiscoveryMutex.Unlock()

	a.safeGo("pluginDiscoveryWatcher", func() {
		err := pluginpkg.WatchPluginsDir(ctx, pluginpkg.DiscoveryWatcherConfig{
			PluginsDir: pluginsDir,
			OnChange: func(discovered []pluginpkg.DiscoveredPlugin) {
				a.updatePluginDiscoverySnapshot(discovered, true)
			},
			OnError: func(err error) {
				a.logError("插件目录监听失败: %v", err)
			},
		})
		if err != nil && ctx.Err() == nil {
			a.logError("插件目录监听停止: %v", err)
		}
	})
}

func (a *CoreApp) stopPluginDiscovery() {
	a.pluginDiscoveryMutex.Lock()
	cancel := a.pluginDiscoveryCancel
	a.pluginDiscoveryCancel = nil
	a.pluginDiscoveryMutex.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (a *CoreApp) updatePluginDiscoverySnapshot(discovered []pluginpkg.DiscoveredPlugin, broadcast bool) {
	next := make(map[string]types.PluginInfo, len(discovered))
	for _, plugin := range discovered {
		info := pluginInfoFromDiscovered(plugin)
		if info.ID == "" {
			continue
		}
		next[info.ID] = info
	}

	a.pluginDiscoveryMutex.Lock()
	if a.availablePlugins == nil {
		a.availablePlugins = map[string]types.PluginInfo{}
	}
	previous := a.availablePlugins
	installed := make([]types.PluginInfo, 0)
	uninstalled := make([]types.PluginInfo, 0)
	changed := make([]types.PluginInfo, 0)

	for id, info := range next {
		old, ok := previous[id]
		if !ok {
			installed = append(installed, info)
			continue
		}
		if old != info {
			changed = append(changed, info)
		}
	}
	for id, info := range previous {
		if _, ok := next[id]; !ok {
			info.Installed = false
			info.Status = "uninstalled"
			uninstalled = append(uninstalled, info)
		}
	}

	a.availablePlugins = next
	snapshot := pluginSnapshotFromMap(next)
	a.pluginDiscoveryMutex.Unlock()

	if broadcast && (len(installed) > 0 || len(uninstalled) > 0 || len(changed) > 0) {
		a.broadcastPluginDiscoveryChanges(snapshot, installed, uninstalled, changed)
	}
}

func (a *CoreApp) availablePluginsSnapshot() []types.PluginInfo {
	a.pluginDiscoveryMutex.RLock()
	defer a.pluginDiscoveryMutex.RUnlock()
	return pluginSnapshotFromMap(a.availablePlugins)
}

func (a *CoreApp) broadcastPluginDiscoveryChanges(snapshot, installed, uninstalled, changed []types.PluginInfo) {
	if a.ipcServer == nil {
		return
	}
	sortPluginInfos(installed)
	sortPluginInfos(uninstalled)
	sortPluginInfos(changed)

	for _, info := range installed {
		a.ipcServer.BroadcastEvent(ipc.EventPluginInstalled, info)
	}
	for _, info := range uninstalled {
		a.ipcServer.BroadcastEvent(ipc.EventPluginUninstalled, info)
	}
	for _, info := range changed {
		a.ipcServer.BroadcastEvent(ipc.EventPluginStatusChanged, info)
	}
	a.ipcServer.BroadcastEvent(ipc.EventPluginsDiscovered, snapshot)
}

func pluginInfoFromDiscovered(plugin pluginpkg.DiscoveredPlugin) types.PluginInfo {
	manifest := plugin.Manifest
	return types.PluginInfo{
		ID:             manifest.ID,
		Name:           manifest.Name,
		Version:        manifest.Version,
		Type:           string(manifest.Type),
		Description:    manifest.Description,
		MinCoreVersion: manifest.MinCoreVer,
		Frontend:       manifest.Frontend,
		Icon:           manifest.Icon,
		Status:         pluginStatusDiscovered,
		Installed:      true,
		ExePath:        plugin.ExecutablePath,
	}
}

func pluginSnapshotFromMap(source map[string]types.PluginInfo) []types.PluginInfo {
	if len(source) == 0 {
		return nil
	}
	snapshot := make([]types.PluginInfo, 0, len(source))
	for _, info := range source {
		snapshot = append(snapshot, info)
	}
	sortPluginInfos(snapshot)
	return snapshot
}

func sortPluginInfos(infos []types.PluginInfo) {
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].ID < infos[j].ID
	})
}
