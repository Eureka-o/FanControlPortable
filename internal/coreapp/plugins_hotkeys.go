package coreapp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/appmeta"
	"github.com/Eureka-o/FanControlPortable/internal/config"
	hotkeysvc "github.com/Eureka-o/FanControlPortable/internal/hotkey"
	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/plugins"
	"github.com/Eureka-o/FanControlPortable/internal/plugins/fnqpowermode"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

const rtssExternalPluginID = "rtss-osd"

func (a *CoreApp) registerPlugins() {
	if a.pluginManager == nil {
		return
	}
	if !a.legionFnQRegistered.CompareAndSwap(false, true) {
		return
	}

	a.pluginManager.Register(fnqpowermode.New(fnqpowermode.Options{
		Logger: a.logger,
		OnModeChange: func(state fnqpowermode.PowerModeState) {
			a.handleLegionPowerModeChange(state)
		},
	}))
}

func (a *CoreApp) applyCachedLegionFnQSupport(cfg *types.AppConfig) bool {
	if cfg == nil || cfg.MonitorOnly || a.monitorOnlyActive() || !cfg.LegionFnQSupport.Checked {
		return false
	}

	a.legionFnQSupportChecked.Store(true)
	a.legionFnQSupported.Store(cfg.LegionFnQSupport.Supported)
	a.logInfo("Lenovo Legion Fn+Q host support loaded from config cache: supported=%v", cfg.LegionFnQSupport.Supported)

	if cfg.LegionFnQSupport.Supported {
		a.registerPlugins()
		return false
	}

	return a.normalizeLegionFnQConfigForHost(cfg)
}

func (a *CoreApp) startLegionFnQSupportDetection() {
	if a.monitorOnlyActive() {
		return
	}
	a.safeGo("detectLegionFnQSupport", func() {
		supported, hostInfo, err := fnqpowermode.DetectSupport()
		if err != nil {
			a.logError("failed to detect Lenovo Legion Fn+Q host support: %v", err)
			return
		}

		a.cacheLegionFnQSupportResult(supported)
		a.legionFnQSupportChecked.Store(true)
		if !supported {
			a.logInfo("Lenovo Legion Fn+Q plugin skipped: unsupported host (manufacturer=%s model=%s family=%s product=%s)",
				hostInfo.Manufacturer, hostInfo.Model, hostInfo.Family, hostInfo.Product)
			a.disableLegionFnQConfigForUnsupportedHost()
			a.broadcastLegionFnQSupportUpdate(false)
			return
		}

		a.registerPlugins()
		a.legionFnQSupported.Store(true)
		a.broadcastLegionFnQSupportUpdate(true)

		a.mutex.RLock()
		cfg := a.configManager.Get()
		a.mutex.RUnlock()
		a.applyPluginConfig(cfg)
	})
}

func (a *CoreApp) cacheLegionFnQSupportResult(supported bool) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	if cfg.LegionFnQSupport.Checked && cfg.LegionFnQSupport.Supported == supported {
		return
	}

	_, err := a.commitConfigMutation(func(current *types.AppConfig) {
		current.LegionFnQSupport = types.LegionFnQSupportCache{
			Checked:   true,
			Supported: supported,
		}
	}, nil)
	if err != nil {
		a.logError("保存 Lenovo Legion Fn+Q 支持缓存失败: %v", err)
		return
	}
}

func (a *CoreApp) disableLegionFnQConfigForUnsupportedHost() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	if !a.normalizeLegionFnQConfigForHost(&cfg) {
		return
	}

	if err := a.commitConfigUpdate(cfg, nil); err != nil {
		a.logError("保存 Lenovo Legion Fn+Q 配置失败: %v", err)
	}
}

func (a *CoreApp) broadcastLegionFnQSupportUpdate(supported bool) {
	if a.ipcServer == nil {
		return
	}
	a.ipcServer.BroadcastEvent(ipc.EventLegionFnQSupportUpdate, map[string]any{
		"supported": supported,
	})
}

func (a *CoreApp) handleLegionPowerModeChange(state fnqpowermode.PowerModeState) {
	if !a.legionFnQSupported.Load() {
		return
	}

	a.logInfo("Lenovo Legion Fn+Q power mode changed: raw=%d mapped=%d mode=%s source=%s",
		state.Raw, state.Mapped, state.Mode, state.Source)

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventLegionPowerModeUpdate, state)
	}

	a.applyLegionFnQFanMapping(state)
}

func (a *CoreApp) applyPluginConfig(cfg types.AppConfig) {
	if a.pluginManager == nil {
		return
	}

	if a.monitorOnlyActive() || !a.legionFnQSupported.Load() {
		return
	}

	if cfg.LegionFnQ.Enabled {
		if err := a.pluginManager.Start(fnqpowermode.PluginID); err != nil {
			a.logError("failed to start Lenovo Legion Fn+Q plugin: %v", err)
		}
		return
	}

	if err := a.pluginManager.Stop(fnqpowermode.PluginID); err != nil {
		a.logError("failed to stop Lenovo Legion Fn+Q plugin: %v", err)
	}
}

func cloneBoolMap(source map[string]bool) map[string]bool {
	if source == nil {
		return nil
	}
	out := make(map[string]bool, len(source))
	for k, v := range source {
		out[k] = v
	}
	return out
}

func normalizeUpdatedRTSSConfig(next, previous types.RTSSConfig) types.RTSSConfig {
	// Older GUI clients do not send the nested RTSS object. Preserve stored settings.
	if next == (types.RTSSConfig{}) {
		next = previous
	}
	normalized, _ := types.NormalizeRTSSConfig(next)
	return normalized
}

func (a *CoreApp) pluginEnabledMap(cfg types.AppConfig) map[string]bool {
	out := cloneBoolMap(cfg.Plugins)
	if out == nil {
		out = map[string]bool{}
	}
	rtssCfg, _ := types.NormalizeRTSSConfig(cfg.RTSS)
	if a.pluginManager != nil && a.pluginManager.HasExternal(rtssExternalPluginID) {
		if _, explicit := cfg.Plugins[rtssExternalPluginID]; !explicit {
			out[rtssExternalPluginID] = rtssCfg.Enabled
		}
	}
	out[fnqpowermode.PluginID] = cfg.LegionFnQ.Enabled && a.legionFnQSupported.Load()
	return out
}

func (a *CoreApp) buildPluginSnapshot(cfg types.AppConfig) []plugins.PluginInfo {
	if a == nil || a.pluginManager == nil {
		return []plugins.PluginInfo{}
	}
	list := a.pluginManager.Snapshot(a.pluginEnabledMap(cfg))
	hasFnQ := false
	for i := range list {
		if list[i].ID == fnqpowermode.PluginID {
			hasFnQ = true
			if !a.legionFnQSupported.Load() {
				list[i].Supported = false
				list[i].Enabled = false
				list[i].State = plugins.StateUnsupported
				list[i].Reason = "仅支持联想拯救者设备"
			}
		}
	}
	if !hasFnQ {
		supported := a.legionFnQSupported.Load()
		state := plugins.StateUnsupported
		reason := "仅支持联想拯救者设备"
		if !a.legionFnQSupportChecked.Load() && !supported {
			reason = "正在检测或当前设备不支持 Fn+Q"
		}
		if supported {
			state = plugins.StateDisabled
			reason = ""
		}
		list = append(list, plugins.PluginInfo{
			ID:           fnqpowermode.PluginID,
			Name:         fnqpowermode.PluginName,
			Kind:         plugins.KindBuiltin,
			Category:     "hardware",
			Version:      "1.0.0",
			Description:  "Listen for Lenovo Legion Fn+Q power mode changes",
			Capabilities: []string{"power-mode", "fan-takeover"},
			State:        state,
			Enabled:      supported && cfg.LegionFnQ.Enabled,
			Supported:    supported,
			Reason:       reason,
		})
	}
	return list
}

func (a *CoreApp) GetPlugins() []plugins.PluginInfo {
	if a == nil || a.configManager == nil {
		return []plugins.PluginInfo{}
	}
	return a.buildPluginSnapshot(a.configManager.Get())
}

func (a *CoreApp) RefreshPlugins() []plugins.PluginInfo {
	if a == nil || a.pluginManager == nil || a.configManager == nil {
		return []plugins.PluginInfo{}
	}
	a.pluginManager.RefreshExternal()
	cfg := a.configManager.Get()
	a.seedExternalRTSSSettingsIfMissing(cfg)
	snapshot := a.buildPluginSnapshot(cfg)
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventPluginsUpdate, snapshot)
	}
	return snapshot
}

func (a *CoreApp) seedExternalRTSSSettingsIfMissing(cfg types.AppConfig) {
	if a == nil || a.pluginManager == nil || !a.pluginManager.HasExternal(rtssExternalPluginID) {
		return
	}
	dir, err := plugins.PluginDataDir(a.pluginConfigDir(cfg), rtssExternalPluginID)
	if err != nil {
		return
	}
	settingsPath := filepath.Join(dir, "settings.json")
	if _, statErr := os.Stat(settingsPath); statErr == nil {
		return
	}
	rtssCfg, _ := types.NormalizeRTSSConfig(cfg.RTSS)
	seed := map[string]any{
		"osdActive":        true,
		"updateIntervalMs": fmt.Sprintf("%d", rtssCfg.UpdateIntervalMS),
		"positionMode":     rtssCfg.PositionMode,
		"positionX":        rtssCfg.PositionX,
		"positionY":        rtssCfg.PositionY,
	}
	if err := plugins.SavePluginSettings(a.pluginConfigDir(cfg), rtssExternalPluginID, seed); err != nil {
		a.logError("初始化外置 RTSS 设置失败: %v", err)
	}
}

func (a *CoreApp) SetPluginEnabled(pluginID string, enabled bool) ([]plugins.PluginInfo, error) {
	if a == nil || a.configManager == nil {
		return nil, fmt.Errorf("配置管理器未初始化")
	}
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return nil, fmt.Errorf("插件 ID 不能为空")
	}

	if pluginID == fnqpowermode.PluginID && enabled && !a.legionFnQSupported.Load() {
		return nil, fmt.Errorf("Lenovo Legion Fn+Q 仅支持拯救者设备")
	}

	hasExternal := a.pluginManager != nil && a.pluginManager.HasExternal(pluginID)

	if pluginID != fnqpowermode.PluginID {
		snapshot := a.GetPlugins()
		var found *plugins.PluginInfo
		for i := range snapshot {
			if snapshot[i].ID == pluginID {
				found = &snapshot[i]
				break
			}
		}
		if found == nil {
			return nil, fmt.Errorf("未找到插件: %s", pluginID)
		}
		if found.State == plugins.StateIncompatible && enabled {
			if found.Reason != "" {
				return nil, fmt.Errorf("%s", found.Reason)
			}
			return nil, fmt.Errorf("插件 %s 当前不兼容", pluginID)
		}
	}

	committed, err := a.commitConfigMutation(func(current *types.AppConfig) {
		if current.Plugins == nil {
			current.Plugins = map[string]bool{}
		}
		current.Plugins[pluginID] = enabled
		if pluginID == fnqpowermode.PluginID {
			current.LegionFnQ.Enabled = enabled
			current.LegionFnQ = types.NormalizeLegionFnQConfig(current.LegionFnQ)
		}
	}, func(cfg types.AppConfig) {
		a.applyPluginConfig(cfg)
		if (hasExternal || pluginID != fnqpowermode.PluginID) && a.pluginManager != nil {
			if _, extErr := a.pluginManager.SetExternalEnabled(pluginID, enabled); extErr != nil {
				a.logError("应用外部插件开关失败 (%s): %v", pluginID, extErr)
			}
		}
		a.broadcastPluginsUpdate(cfg)
	})
	if err != nil {
		return nil, err
	}
	return a.buildPluginSnapshot(committed), nil
}

func (a *CoreApp) pluginConfigDir(cfg types.AppConfig) string {
	if strings.TrimSpace(cfg.ConfigPath) != "" {
		return filepath.Dir(cfg.ConfigPath)
	}
	return config.GetInstallDir()
}

func (a *CoreApp) InvokePlugin(pluginID, method string, payload json.RawMessage) (json.RawMessage, error) {
	if a == nil || a.pluginManager == nil {
		return nil, fmt.Errorf("插件管理器未初始化")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	return a.pluginManager.Invoke(ctx, strings.TrimSpace(pluginID), strings.TrimSpace(method), payload)
}

func (a *CoreApp) RestartPlugin(pluginID string) ([]plugins.PluginInfo, error) {
	if a == nil || a.pluginManager == nil || a.configManager == nil {
		return nil, fmt.Errorf("插件管理器未初始化")
	}
	pluginID = strings.TrimSpace(pluginID)
	if err := a.pluginManager.RestartExternal(pluginID); err != nil {
		return nil, err
	}
	cfg := a.configManager.Get()
	a.broadcastPluginsUpdate(cfg)
	return a.buildPluginSnapshot(cfg), nil
}

func (a *CoreApp) GetPluginSettings(pluginID string) (map[string]any, error) {
	if a == nil || a.configManager == nil || a.pluginManager == nil {
		return nil, fmt.Errorf("插件管理器未初始化")
	}
	pluginID = strings.TrimSpace(pluginID)
	schema, _ := a.pluginManager.ExternalSchema(pluginID)
	cfg := a.configManager.Get()
	return plugins.LoadPluginSettings(a.pluginConfigDir(cfg), pluginID, schema)
}

func (a *CoreApp) SavePluginSettings(pluginID string, settings map[string]any) error {
	if a == nil || a.configManager == nil || a.pluginManager == nil {
		return fmt.Errorf("插件管理器未初始化")
	}
	pluginID = strings.TrimSpace(pluginID)
	cfg := a.configManager.Get()
	if err := plugins.SavePluginSettings(a.pluginConfigDir(cfg), pluginID, settings); err != nil {
		return err
	}
	a.pluginManager.NotifySettingsChanged(pluginID, settings)
	return nil
}

func (a *CoreApp) GetPluginLogs(pluginID string) []string {
	if a == nil || a.pluginManager == nil {
		return []string{}
	}
	return a.pluginManager.Logs(strings.TrimSpace(pluginID))
}

func (a *CoreApp) SetPluginDevPaths(paths []string) ([]plugins.PluginInfo, error) {
	if a == nil || a.configManager == nil || a.pluginManager == nil {
		return nil, fmt.Errorf("插件管理器未初始化")
	}
	cleaned := make([]string, 0, len(paths))
	seen := make(map[string]bool)
	for _, p := range paths {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		cleaned = append(cleaned, trimmed)
	}
	committed, err := a.commitConfigMutation(func(current *types.AppConfig) {
		current.PluginDevPaths = cleaned
	}, func(cfg types.AppConfig) {
		a.pluginManager.SetDevPluginDirs(cfg.PluginDevPaths)
		a.pluginManager.RefreshExternal()
		a.broadcastPluginsUpdate(cfg)
	})
	if err != nil {
		return nil, err
	}
	return a.buildPluginSnapshot(committed), nil
}

func (a *CoreApp) MarkPluginPendingRestart(pluginID string) []plugins.PluginInfo {
	if a == nil || a.pluginManager == nil || a.configManager == nil {
		return []plugins.PluginInfo{}
	}
	a.pluginManager.MarkPendingRestart(strings.TrimSpace(pluginID))
	cfg := a.configManager.Get()
	a.broadcastPluginsUpdate(cfg)
	return a.buildPluginSnapshot(cfg)
}

func (a *CoreApp) broadcastPluginsUpdate(cfg types.AppConfig) {
	if a == nil || a.ipcServer == nil || a.pluginManager == nil {
		return
	}
	a.ipcServer.BroadcastEvent(ipc.EventPluginsUpdate, a.buildPluginSnapshot(cfg))
}

func (a *CoreApp) applyLegionFnQFanMapping(state fnqpowermode.PowerModeState) {
	cfg := a.configManager.Get()
	cfg.LegionFnQ = types.NormalizeLegionFnQConfig(cfg.LegionFnQ)
	if !cfg.LegionFnQ.Enabled || !cfg.LegionFnQ.TakeOverFan {
		return
	}
	if !a.isConnected {
		a.logDebug("Lenovo Legion Fn+Q takeover skipped: device not connected")
		return
	}

	target, ok := cfg.LegionFnQ.ModeMapping[state.Mode]
	if !ok {
		a.logDebug("Lenovo Legion Fn+Q takeover skipped: no mapping for mode=%s", state.Mode)
		return
	}

	a.safeGo("applyLegionFnQFanMapping", func() {
		if ok := a.SetManualGear(target.Gear, target.Level); !ok {
			a.logError("Lenovo Legion Fn+Q takeover failed: mode=%s gear=%s level=%s", state.Mode, target.Gear, target.Level)
			return
		}
		a.logInfo("Lenovo Legion Fn+Q takeover applied: mode=%s gear=%s level=%s", state.Mode, target.Gear, target.Level)
	})
}

func (a *CoreApp) normalizeLegionFnQConfigForHost(cfg *types.AppConfig) bool {
	if cfg == nil || !a.legionFnQSupportChecked.Load() || a.legionFnQSupported.Load() {
		return false
	}

	changed := false
	if cfg.LegionFnQ.Enabled {
		cfg.LegionFnQ.Enabled = false
		changed = true
	}
	if cfg.LegionFnQ.TakeOverFan {
		cfg.LegionFnQ.TakeOverFan = false
		changed = true
	}

	return changed
}

func normalizeHotkeyConfig(cfg *types.AppConfig) bool {
	if cfg == nil {
		return false
	}

	changed := false
	if cfg.ManualGearToggleHotkey != "" {
		if _, _, err := hotkeysvc.ParseShortcut(cfg.ManualGearToggleHotkey); err != nil {
			cfg.ManualGearToggleHotkey = types.GetDefaultConfig(false).ManualGearToggleHotkey
			changed = true
		}
	}
	if cfg.AutoControlToggleHotkey != "" {
		if _, _, err := hotkeysvc.ParseShortcut(cfg.AutoControlToggleHotkey); err != nil {
			cfg.AutoControlToggleHotkey = types.GetDefaultConfig(false).AutoControlToggleHotkey
			changed = true
		}
	}
	if cfg.CurveProfileToggleHotkey != "" {
		if _, _, err := hotkeysvc.ParseShortcut(cfg.CurveProfileToggleHotkey); err != nil {
			cfg.CurveProfileToggleHotkey = types.GetDefaultConfig(false).CurveProfileToggleHotkey
			changed = true
		}
	}

	return changed
}

func normalizeManualGearMemoryConfig(cfg *types.AppConfig) bool {
	if cfg == nil {
		return false
	}

	changed := false
	if cfg.ManualGearLevels == nil {
		cfg.ManualGearLevels = map[string]string{}
		changed = true
	}

	for _, gear := range []string{"静音", "标准", "强劲", "超频"} {
		if level, ok := cfg.ManualGearLevels[gear]; !ok {
			cfg.ManualGearLevels[gear] = "中"
			changed = true
		} else {
			normalized := normalizeManualLevel(level)
			if normalized != level {
				cfg.ManualGearLevels[gear] = normalized
				changed = true
			}
		}
	}

	normalizedCurrent := normalizeManualLevel(cfg.ManualLevel)
	if normalizedCurrent != cfg.ManualLevel {
		cfg.ManualLevel = normalizedCurrent
		changed = true
	}

	if cfg.ManualGear != "" {
		if remembered, ok := cfg.ManualGearLevels[cfg.ManualGear]; !ok || remembered != normalizedCurrent {
			cfg.ManualGearLevels[cfg.ManualGear] = normalizedCurrent
			changed = true
		}
	}

	return changed
}

func (a *CoreApp) applyHotkeyBindings(cfg types.AppConfig) {
	if a.hotkeyManager == nil {
		return
	}
	if err := a.hotkeyManager.UpdateBindings(cfg.ManualGearToggleHotkey, cfg.AutoControlToggleHotkey, cfg.CurveProfileToggleHotkey); err != nil {
		a.logError("更新全局快捷键失败: %v", err)
	}
}

func (a *CoreApp) handleHotkeyAction(action hotkeysvc.Action, shortcut string) {
	a.safeGo("handleHotkeyAction", func() {
		var message string
		success := true

		switch action {
		case hotkeysvc.ActionToggleManualGear:
			msg, err := a.toggleManualGearByHotkey()
			if err != nil {
				success = false
				message = err.Error()
			} else {
				message = msg
			}
		case hotkeysvc.ActionToggleAutoMode:
			msg, err := a.toggleAutoControlByHotkey()
			if err != nil {
				success = false
				message = err.Error()
			} else {
				message = msg
			}
		case hotkeysvc.ActionToggleCurveProfile:
			msg, err := a.toggleCurveProfileByHotkey()
			if err != nil {
				success = false
				message = err.Error()
			} else {
				message = msg
			}
		default:
			success = false
			message = "未知快捷键动作"
		}

		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventHotkeyTriggered, map[string]any{
				"action":   string(action),
				"shortcut": shortcut,
				"success":  success,
				"message":  message,
			})
		}

		title := appmeta.AppName + " 快捷键"
		if !success {
			title = appmeta.AppName + " 快捷键失败"
		}
		if a.notifier != nil {
			a.notifier.Notify(title, message)
		}
	})
}

func (a *CoreApp) toggleCurveProfileByHotkey() (string, error) {
	profile, err := a.CycleFanCurveProfile()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("温控曲线已切换: %s", profile.Name), nil
}

func (a *CoreApp) toggleAutoControlByHotkey() (string, error) {
	cfg := a.configManager.Get()
	target := !cfg.AutoControl
	if err := a.SetAutoControl(target); err != nil {
		return "", err
	}
	if target {
		return "智能变频已开启", nil
	}
	return "智能变频已关闭", nil
}

func (a *CoreApp) toggleManualGearByHotkey() (string, error) {
	cfg := a.configManager.Get()

	if cfg.AutoControl {
		if err := a.SetAutoControl(false); err != nil {
			return "", fmt.Errorf("切换到手动模式失败: %w", err)
		}
	}

	nextGear, nextLevel := a.getNextManualGearWithMemory(cfg.ManualGear, cfg.ManualLevel)
	if ok := a.SetManualGear(nextGear, nextLevel); !ok {
		return "", fmt.Errorf("应用手动挡位失败")
	}

	rpm := cfg.ResolveGearRPM(nextGear, nextLevel)
	if rpm > 0 {
		return fmt.Sprintf("手动挡位: %s %s (%d%%)", nextGear, nextLevel, rpm), nil
	}
	return fmt.Sprintf("手动挡位: %s %s", nextGear, nextLevel), nil
}

func (a *CoreApp) getNextManualGearWithMemory(currentGear, currentLevel string) (string, string) {
	sequence := []string{"静音", "标准", "强劲", "超频"}
	nextIndex := 0

	for i, gear := range sequence {
		if gear == currentGear {
			nextIndex = (i + 1) % len(sequence)
			break
		}
	}

	a.rememberManualGearLevel(currentGear, currentLevel)
	fallbackLevel := normalizeManualLevel(currentLevel)
	level := a.getRememberedManualLevel(sequence[nextIndex], fallbackLevel)

	return sequence[nextIndex], level
}

func normalizeManualLevel(level string) string {
	if level == "低" || level == "中" || level == "高" {
		return level
	}
	return "中"
}

func cloneManualGearLevels(source map[string]string) map[string]string {
	cloned := map[string]string{}
	for _, gear := range []string{"静音", "标准", "强劲", "超频"} {
		if source == nil {
			cloned[gear] = "中"
			continue
		}
		cloned[gear] = normalizeManualLevel(source[gear])
	}
	return cloned
}

func (a *CoreApp) syncManualGearLevelMemory(cfg types.AppConfig) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.syncManualGearLevelMemoryLocked(cfg)
}

func (a *CoreApp) syncManualGearLevelMemoryLocked(cfg types.AppConfig) {

	if a.manualGearLevelMemory == nil {
		a.manualGearLevelMemory = map[string]string{}
	}

	defaultLevel := normalizeManualLevel(cfg.ManualLevel)
	for _, gear := range []string{"静音", "标准", "强劲", "超频"} {
		if fromCfg, ok := cfg.ManualGearLevels[gear]; ok {
			a.manualGearLevelMemory[gear] = normalizeManualLevel(fromCfg)
			continue
		}
		a.manualGearLevelMemory[gear] = defaultLevel
	}

	a.manualGearLevelMemory[cfg.ManualGear] = normalizeManualLevel(cfg.ManualLevel)
}

func (a *CoreApp) rememberManualGearLevel(gear, level string) {
	if gear != "静音" && gear != "标准" && gear != "强劲" && gear != "超频" {
		return
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.manualGearLevelMemory == nil {
		a.manualGearLevelMemory = map[string]string{}
	}
	a.manualGearLevelMemory[gear] = normalizeManualLevel(level)
}

func (a *CoreApp) getRememberedManualLevel(gear, fallback string) string {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	if a.manualGearLevelMemory == nil {
		return normalizeManualLevel(fallback)
	}
	if level, ok := a.manualGearLevelMemory[gear]; ok {
		return normalizeManualLevel(level)
	}
	return normalizeManualLevel(fallback)
}
