package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

// Manager owns built-in plugins plus external sidecar & in-process DLL plugins.
type Manager struct {
	logger      types.Logger
	plugins     []Plugin
	observers   []FanDataObserver
	external    []PluginInfo
	manifests   map[string]Manifest
	sidecars    map[string]*SidecarRunner
	inprocs     map[string]*InprocRunner
	pendingRst  map[string]bool
	pluginsDir  string
	configDir   string
	coreVersion string
	devDirs     []string

	onEvent        func(pluginID, event string, payload json.RawMessage)
	onSnapshotDiff func()

	mutex  sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

func NewManager(logger types.Logger) *Manager {
	return &Manager{
		logger:     logger,
		manifests:  make(map[string]Manifest),
		sidecars:   make(map[string]*SidecarRunner),
		inprocs:    make(map[string]*InprocRunner),
		pendingRst: make(map[string]bool),
	}
}

func (m *Manager) SetPluginsDir(dir, coreVersion string) {
	if m == nil {
		return
	}
	m.mutex.Lock()
	m.pluginsDir = dir
	m.coreVersion = coreVersion
	m.mutex.Unlock()
}

func (m *Manager) SetConfigDir(dir string) {
	if m == nil {
		return
	}
	m.mutex.Lock()
	m.configDir = dir
	m.mutex.Unlock()
}

func (m *Manager) SetDevPluginDirs(dirs []string) {
	if m == nil {
		return
	}
	cleaned := make([]string, 0, len(dirs))
	seen := make(map[string]bool)
	for _, dir := range dirs {
		trimmed := strings.TrimSpace(dir)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		cleaned = append(cleaned, trimmed)
	}
	m.mutex.Lock()
	m.devDirs = cleaned
	m.mutex.Unlock()
}

func (m *Manager) DevPluginDirs() []string {
	if m == nil {
		return nil
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return append([]string(nil), m.devDirs...)
}

func (m *Manager) SetCallbacks(onEvent func(pluginID, event string, payload json.RawMessage), onSnapshotDiff func()) {
	if m == nil {
		return
	}
	m.mutex.Lock()
	m.onEvent = onEvent
	m.onSnapshotDiff = onSnapshotDiff
	m.mutex.Unlock()
}

func (m *Manager) PluginsDir() string {
	if m == nil {
		return ""
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.pluginsDir
}

func (m *Manager) RefreshExternal() []PluginInfo {
	if m == nil {
		return []PluginInfo{}
	}
	m.mutex.Lock()
	dir := m.pluginsDir
	devDirs := append([]string(nil), m.devDirs...)
	coreVersion := m.coreVersion
	m.mutex.Unlock()

	discovered, manifests := DiscoverExternalWithDev(dir, devDirs, CurrentPlatform(), coreVersion)

	m.mutex.Lock()
	m.external = discovered
	m.manifests = manifests
	cloned := clonePluginInfoSlice(discovered)
	m.mutex.Unlock()
	return cloned
}

// SyncStartupExternal starts enabled sidecar plugins and loads enabled inproc-dll plugins at Core startup.
func (m *Manager) SyncStartupExternal(enabledByID map[string]bool) {
	if m == nil {
		return
	}
	m.RefreshExternal()

	m.mutex.Lock()
	items := clonePluginInfoSlice(m.external)
	manifests := make(map[string]Manifest, len(m.manifests))
	for k, v := range m.manifests {
		manifests[k] = v
	}
	m.mutex.Unlock()

	for _, info := range items {
		if !info.Supported || info.State == StateIncompatible {
			continue
		}
		if !enabledByID[info.ID] {
			continue
		}
		manifest, ok := manifests[info.ID]
		if !ok {
			continue
		}
		switch manifest.Runtime {
		case RuntimeSidecar:
			if err := m.startSidecar(info, manifest); err != nil {
				m.logError("启动外部插件 %s 失败: %v", info.ID, err)
			}
		case RuntimeInprocDLL:
			if err := m.startInprocOnStartup(info, manifest); err != nil {
				m.logError("加载进程内插件 %s 失败: %v", info.ID, err)
			}
		}
	}
}

func (m *Manager) startSidecar(info PluginInfo, manifest Manifest) error {
	m.mutex.Lock()
	existing := m.sidecars[info.ID]
	configDir := m.configDir
	coreVersion := m.coreVersion
	onEvent := m.onEvent
	onDiff := m.onSnapshotDiff
	m.mutex.Unlock()

	if existing != nil {
		return existing.Start()
	}

	dataDir, _ := PluginDataDir(configDir, info.ID)
	settingsMap, _ := LoadPluginSettings(configDir, info.ID, manifest.Settings)
	settingsBytes, _ := json.Marshal(settingsMap)

	runner := NewSidecarRunner(SidecarOptions{
		Manifest:        manifest,
		PluginDir:       info.Path,
		DataDir:         dataDir,
		CoreVersion:     coreVersion,
		InitialSettings: settingsBytes,
		Logger:          m.logger,
		OnEvent:         onEvent,
		OnStateChange:   onDiff,
	})

	m.mutex.Lock()
	if current := m.sidecars[info.ID]; current != nil {
		runner = current
	} else {
		m.sidecars[info.ID] = runner
	}
	m.mutex.Unlock()

	return runner.Start()
}

func (m *Manager) startInprocOnStartup(info PluginInfo, manifest Manifest) error {
	dllPath := filepath.Clean(filepath.Join(info.Path, filepath.FromSlash(manifest.Backend)))

	m.mutex.Lock()
	existing := m.inprocs[info.ID]
	configDir := m.configDir
	m.mutex.Unlock()

	if existing == nil {
		existing = NewInprocRunner(info.ID, dllPath)
		m.mutex.Lock()
		if cur := m.inprocs[info.ID]; cur != nil {
			existing = cur
		} else {
			m.inprocs[info.ID] = existing
		}
		m.mutex.Unlock()
	}

	settingsMap, _ := LoadPluginSettings(configDir, info.ID, manifest.Settings)
	settingsBytes, _ := json.Marshal(settingsMap)
	return existing.LoadAndStart(settingsBytes)
}

// SetExternalEnabled applies enable/disable for an external plugin:
// sidecar starts/stops live, while inproc-dll marks pendingRestart=true until Core restarts.
func (m *Manager) SetExternalEnabled(pluginID string, enabled bool) (pendingRestart bool, err error) {
	if m == nil {
		return false, fmt.Errorf("插件管理器未初始化")
	}

	m.mutex.Lock()
	var target *PluginInfo
	for i := range m.external {
		if m.external[i].ID == pluginID {
			copyItem := m.external[i]
			target = &copyItem
			break
		}
	}
	manifest, hasManifest := m.manifests[pluginID]
	inproc := m.inprocs[pluginID]
	sidecar := m.sidecars[pluginID]
	m.mutex.Unlock()

	if target == nil {
		return false, fmt.Errorf("未找到外部插件: %s", pluginID)
	}
	if target.State == StateIncompatible && enabled {
		if target.Reason != "" {
			return false, fmt.Errorf("%s", target.Reason)
		}
		return false, fmt.Errorf("插件 %s 当前不兼容", pluginID)
	}

	runtimeName := manifest.Runtime
	if runtimeName == "" {
		runtimeName = target.Runtime
	}

	switch runtimeName {
	case RuntimeInprocDLL:
		isLoaded := false
		if inproc != nil {
			_, _, isLoaded, _ = inproc.StateSnapshot()
		}
		needsRestart := (enabled && !isLoaded) || (!enabled && isLoaded)
		m.mutex.Lock()
		m.pendingRst[pluginID] = needsRestart
		m.mutex.Unlock()
		return needsRestart, nil

	case RuntimeSidecar:
		if !hasManifest {
			return false, fmt.Errorf("缺少有效的插件清单: %s", pluginID)
		}
		if enabled {
			return false, m.startSidecar(*target, manifest)
		}
		if sidecar != nil {
			return false, sidecar.Stop()
		}
		return false, nil

	default:
		return false, nil
	}
}

func (m *Manager) MarkPendingRestart(pluginID string) {
	if m == nil || strings.TrimSpace(pluginID) == "" {
		return
	}
	m.mutex.Lock()
	m.pendingRst[pluginID] = true
	m.mutex.Unlock()
}

func (m *Manager) IsInprocLoaded(pluginID string) bool {
	if m == nil {
		return false
	}
	m.mutex.Lock()
	runner := m.inprocs[pluginID]
	m.mutex.Unlock()
	if runner == nil {
		return false
	}
	_, _, loaded, _ := runner.StateSnapshot()
	return loaded
}

func (m *Manager) RestartExternal(pluginID string) error {
	if m == nil {
		return fmt.Errorf("插件管理器未初始化")
	}
	m.mutex.Lock()
	sidecar := m.sidecars[pluginID]
	manifest := m.manifests[pluginID]
	var target *PluginInfo
	for i := range m.external {
		if m.external[i].ID == pluginID {
			copyItem := m.external[i]
			target = &copyItem
			break
		}
	}
	m.mutex.Unlock()

	if target == nil {
		return fmt.Errorf("未找到插件: %s", pluginID)
	}
	if manifest.Runtime == RuntimeInprocDLL || target.Runtime == RuntimeInprocDLL {
		return fmt.Errorf("进程内插件需要重启核心服务才能生效")
	}
	if sidecar != nil {
		return sidecar.Restart()
	}
	return m.startSidecar(*target, manifest)
}

func (m *Manager) Invoke(ctx context.Context, pluginID, method string, payload json.RawMessage) (json.RawMessage, error) {
	if m == nil {
		return nil, fmt.Errorf("插件管理器未初始化")
	}
	m.mutex.Lock()
	sidecar := m.sidecars[pluginID]
	inproc := m.inprocs[pluginID]
	m.mutex.Unlock()

	if sidecar != nil {
		return sidecar.Invoke(ctx, method, payload)
	}
	if inproc != nil {
		return inproc.Invoke(method, payload)
	}
	return nil, fmt.Errorf("插件 %s 当前未运行", pluginID)
}

func (m *Manager) NotifySettingsChanged(pluginID string, values map[string]any) {
	if m == nil {
		return
	}
	m.mutex.Lock()
	sidecar := m.sidecars[pluginID]
	inproc := m.inprocs[pluginID]
	m.mutex.Unlock()
	if sidecar != nil {
		_ = sidecar.SendHostEvent("settings-changed", values)
	}
	if inproc != nil {
		payload, err := json.Marshal(values)
		if err == nil {
			go func(runner *InprocRunner, raw json.RawMessage) {
				_, _ = runner.Invoke("settings-changed", raw)
			}(inproc, payload)
		}
	}
}

// HasExternal reports whether an external plugin with the given ID is currently discovered.
func (m *Manager) HasExternal(pluginID string) bool {
	if m == nil {
		return false
	}
	id := strings.TrimSpace(pluginID)
	if id == "" {
		return false
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, ext := range m.external {
		if ext.ID == id {
			return true
		}
	}
	return false
}

// ReleaseExternalForTest unloads external runtime resources in tests so temp directories can be cleaned up cleanly.
func (m *Manager) ReleaseExternalForTest() {
	if m == nil {
		return
	}
	m.StopAll()
	m.mutex.Lock()
	inprocs := make([]*InprocRunner, 0, len(m.inprocs))
	for _, r := range m.inprocs {
		inprocs = append(inprocs, r)
	}
	m.inprocs = make(map[string]*InprocRunner)
	m.mutex.Unlock()
	for _, r := range inprocs {
		if releaser, ok := any(r).(interface{ ReleaseForTest() }); ok {
			releaser.ReleaseForTest()
		}
	}
}

func (m *Manager) SuspendAll(grace time.Duration) {
	if m == nil {
		return
	}
	m.mutex.Lock()
	runners := make([]*SidecarRunner, 0, len(m.sidecars))
	for _, r := range m.sidecars {
		runners = append(runners, r)
	}
	m.mutex.Unlock()

	var wg sync.WaitGroup
	for _, r := range runners {
		wg.Add(1)
		go func(runner *SidecarRunner) {
			defer wg.Done()
			_ = runner.Suspend(grace)
		}(r)
	}
	wg.Wait()
}

func (m *Manager) ResumeAll() {
	if m == nil {
		return
	}
	m.mutex.Lock()
	runners := make([]*SidecarRunner, 0, len(m.sidecars))
	for _, r := range m.sidecars {
		runners = append(runners, r)
	}
	m.mutex.Unlock()

	for _, r := range runners {
		r.Resume()
	}
}

func (m *Manager) Logs(pluginID string) []string {
	if m == nil {
		return nil
	}
	m.mutex.Lock()
	runner := m.sidecars[pluginID]
	m.mutex.Unlock()
	if runner == nil {
		return []string{}
	}
	return runner.Logs()
}

func (m *Manager) ExternalSchema(pluginID string) ([]SettingField, bool) {
	if m == nil {
		return nil, false
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	mf, ok := m.manifests[pluginID]
	if !ok {
		return nil, false
	}
	return cloneSettingFields(mf.Settings), true
}

func (m *Manager) Register(plugin Plugin) {
	if m == nil || plugin == nil {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.findLocked(plugin.ID()) != nil {
		return
	}
	m.plugins = append(m.plugins, plugin)
	if observer, ok := plugin.(FanDataObserver); ok {
		m.observers = append(m.observers, observer)
	}
}

func (m *Manager) StartAll(parent context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.ctx != nil {
		return nil
	}
	if parent == nil {
		parent = context.Background()
	}
	m.ctx, m.cancel = context.WithCancel(parent)

	var firstErr error
	for _, plugin := range m.plugins {
		if err := plugin.Start(m.ctx); err != nil {
			m.logError("plugin start failed: %s (%s): %v", plugin.Name(), plugin.ID(), err)
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", plugin.ID(), err)
			}
			continue
		}
		m.logInfo("plugin started: %s (%s)", plugin.Name(), plugin.ID())
	}

	return firstErr
}

func (m *Manager) Start(pluginID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.ctx == nil {
		m.ctx, m.cancel = context.WithCancel(context.Background())
	}

	plugin := m.findLocked(pluginID)
	if plugin == nil {
		return fmt.Errorf("plugin not registered: %s", pluginID)
	}
	if err := plugin.Start(m.ctx); err != nil {
		m.logError("plugin start failed: %s (%s): %v", plugin.Name(), plugin.ID(), err)
		return err
	}
	m.logInfo("plugin started: %s (%s)", plugin.Name(), plugin.ID())
	return nil
}

func (m *Manager) Stop(pluginID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	plugin := m.findLocked(pluginID)
	if plugin == nil {
		return fmt.Errorf("plugin not registered: %s", pluginID)
	}
	if err := plugin.Stop(); err != nil {
		m.logError("plugin stop failed: %s (%s): %v", plugin.Name(), plugin.ID(), err)
		return err
	}
	return nil
}

func (m *Manager) StopAll() {
	m.mutex.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	builtIns := append([]Plugin(nil), m.plugins...)
	sidecars := make([]*SidecarRunner, 0, len(m.sidecars))
	for _, r := range m.sidecars {
		sidecars = append(sidecars, r)
	}
	inprocs := make([]*InprocRunner, 0, len(m.inprocs))
	for _, r := range m.inprocs {
		inprocs = append(inprocs, r)
	}
	m.ctx = nil
	m.cancel = nil
	m.mutex.Unlock()

	for _, r := range sidecars {
		if err := r.Stop(); err != nil {
			m.logError("sidecar plugin stop failed: %v", err)
		}
	}
	for _, r := range inprocs {
		if err := r.Stop(); err != nil {
			m.logError("inproc plugin stop failed: %v", err)
		}
	}
	for _, plugin := range builtIns {
		if err := plugin.Stop(); err != nil {
			m.logError("plugin stop failed: %s (%s): %v", plugin.Name(), plugin.ID(), err)
		}
	}
}

func (m *Manager) Statuses() []Status {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	statuses := make([]Status, 0, len(m.plugins))
	for _, plugin := range m.plugins {
		statuses = append(statuses, plugin.Status())
	}
	return statuses
}

// Snapshot returns a unified view of registered built-in plugins and discovered external plugins.
func (m *Manager) Snapshot(enabledByID map[string]bool) []PluginInfo {
	if m == nil {
		return []PluginInfo{}
	}
	m.mutex.Lock()
	registered := append([]Plugin(nil), m.plugins...)
	external := clonePluginInfoSlice(m.external)
	sidecars := make(map[string]*SidecarRunner, len(m.sidecars))
	for k, v := range m.sidecars {
		sidecars[k] = v
	}
	inprocs := make(map[string]*InprocRunner, len(m.inprocs))
	for k, v := range m.inprocs {
		inprocs[k] = v
	}
	pendingRst := make(map[string]bool, len(m.pendingRst))
	for k, v := range m.pendingRst {
		pendingRst[k] = v
	}
	pluginsDir := m.pluginsDir
	m.mutex.Unlock()

	externalIDs := make(map[string]bool, len(external))
	for _, ext := range external {
		externalIDs[ext.ID] = true
	}

	result := make([]PluginInfo, 0, len(registered)+len(external))
	for _, plugin := range registered {
		if externalIDs[plugin.ID()] {
			continue
		}
		status := plugin.Status()
		desc := Descriptor{Kind: KindBuiltin, Runtime: RuntimeBuiltin}
		if provider, ok := plugin.(DescriptorProvider); ok {
			desc = provider.Descriptor()
			if desc.Kind == "" {
				desc.Kind = KindBuiltin
			}
			if desc.Runtime == "" {
				desc.Runtime = RuntimeBuiltin
			}
		}
		supported := true
		reason := ""
		if checker, ok := plugin.(AvailabilityChecker); ok {
			supported, reason = checker.Available()
		}
		enabled := status.Running
		if enabledByID != nil {
			if configured, ok := enabledByID[plugin.ID()]; ok {
				enabled = configured
			}
		}
		if !supported {
			enabled = false
		}

		state := StateDisabled
		switch {
		case !supported:
			state = StateUnsupported
		case status.Running:
			state = StateReady
		case status.LastError != "" && enabled:
			state = StateFailed
		}

		var caps []string
		if len(desc.Capabilities) > 0 {
			caps = append([]string(nil), desc.Capabilities...)
		}
		result = append(result, PluginInfo{
			ID:           plugin.ID(),
			Name:         plugin.Name(),
			Kind:         desc.Kind,
			Runtime:      desc.Runtime,
			UISlot:       desc.UISlot,
			Category:     desc.Category,
			Version:      desc.Version,
			Description:  desc.Description,
			Capabilities: caps,
			State:        state,
			Enabled:      enabled,
			Running:      status.Running,
			Supported:    supported,
			Reason:       reason,
			LastError:    status.LastError,
		})
	}

	for _, ext := range external {
		if ext.State == StateIncompatible {
			ext.Enabled = false
			result = append(result, ext)
			continue
		}

		if enabledByID != nil {
			ext.Enabled = enabledByID[ext.ID]
		}
		stagedPending := HasPendingStaging(pluginsDir, ext.ID)
		pendingRestart := pendingRst[ext.ID] || stagedPending

		switch ext.Runtime {
		case RuntimeSidecar:
			if runner := sidecars[ext.ID]; runner != nil {
				st, running, supported, reason, lastErr, caps := runner.StateSnapshot()
				ext.Running = running
				ext.Supported = supported
				ext.LastError = lastErr
				if reason != "" {
					ext.Reason = reason
				}
				if len(caps) > 0 {
					ext.Capabilities = caps
				}
				if !supported {
					ext.Enabled = false
					ext.State = StateUnsupported
				} else if st != "" && st != StateDisabled {
					ext.State = st
				} else if ext.Enabled {
					ext.State = StateStarting
				} else {
					ext.State = StateDisabled
				}
			} else if ext.Enabled {
				ext.State = StateStarting
			} else {
				ext.State = StateDiscovered
			}

		case RuntimeInprocDLL:
			ext.RestartRequired = true
			isLoaded := false
			if runner := inprocs[ext.ID]; runner != nil {
				st, running, loaded, lastErr := runner.StateSnapshot()
				isLoaded = loaded
				ext.Running = running
				ext.LastError = lastErr
				if st == StateFailed || st == StateIncompatible {
					ext.State = st
				} else if running {
					ext.State = StateReady
				}
			}
			if pendingRestart || (ext.Enabled && !isLoaded) || (!ext.Enabled && isLoaded) {
				pendingRestart = true
				ext.State = StatePendingRestart
			} else if !ext.Enabled && ext.State != StateFailed && ext.State != StateIncompatible {
				ext.State = StateDiscovered
			}

		default: // frontend-only
			if ext.Enabled {
				ext.Running = true
				ext.State = StateReady
			} else {
				ext.State = StateDiscovered
			}
		}

		ext.PendingRestart = pendingRestart
		result = append(result, ext)
	}
	return result
}

// PublishFanData dispatches fan telemetry to built-in FanDataObservers and external plugins.
func (m *Manager) PublishFanData(fanData *types.FanData) {
	if m == nil {
		return
	}
	m.mutex.Lock()
	observers := append([]FanDataObserver(nil), m.observers...)
	inprocs := make([]*InprocRunner, 0, len(m.inprocs))
	for _, r := range m.inprocs {
		inprocs = append(inprocs, r)
	}
	sidecarTargets := make([]*SidecarRunner, 0, len(m.sidecars))
	for id, r := range m.sidecars {
		if mf, ok := m.manifests[id]; ok && hasCapability(mf.Capabilities, "fan-telemetry") {
			sidecarTargets = append(sidecarTargets, r)
		}
	}
	m.mutex.Unlock()

	for _, observer := range observers {
		observer.OnFanData(fanData)
	}

	if fanData != nil {
		isPct := types.NormalizeFanSpeedUnit(fanData.SpeedUnit) == types.FanSpeedUnitPercent
		for _, r := range inprocs {
			r.OnFanData(uint32(fanData.CurrentRPM), isPct, true)
		}
		if len(sidecarTargets) > 0 {
			payload := map[string]any{
				"connected": true,
				"rpm":       fanData.CurrentRPM,
				"speedUnit": types.NormalizeFanSpeedUnit(fanData.SpeedUnit),
			}
			for _, r := range sidecarTargets {
				_ = r.SendHostEvent("fan-data", payload)
			}
		}
	}
}

// NotifyDeviceDisconnected notifies FanDataObserver plugins and external fan-telemetry subscribers.
func (m *Manager) NotifyDeviceDisconnected() {
	if m == nil {
		return
	}
	m.mutex.Lock()
	observers := append([]FanDataObserver(nil), m.observers...)
	inprocs := make([]*InprocRunner, 0, len(m.inprocs))
	for _, r := range m.inprocs {
		inprocs = append(inprocs, r)
	}
	sidecarTargets := make([]*SidecarRunner, 0, len(m.sidecars))
	for id, r := range m.sidecars {
		if mf, ok := m.manifests[id]; ok && hasCapability(mf.Capabilities, "fan-telemetry") {
			sidecarTargets = append(sidecarTargets, r)
		}
	}
	m.mutex.Unlock()

	for _, observer := range observers {
		observer.OnDeviceDisconnected()
	}
	for _, r := range inprocs {
		r.OnFanData(0, false, false)
	}
	for _, r := range sidecarTargets {
		_ = r.SendHostEvent("device-disconnected", map[string]any{"connected": false})
	}
}

// PublishSystemTelemetry dispatches CPU/GPU temperature and power telemetry to sidecar plugins
// that declare the "system-telemetry" capability.
func (m *Manager) PublishSystemTelemetry(temp *types.TemperatureData) {
	if m == nil || temp == nil {
		return
	}
	m.mutex.Lock()
	sidecarTargets := make([]*SidecarRunner, 0, len(m.sidecars))
	for id, r := range m.sidecars {
		if mf, ok := m.manifests[id]; ok && hasCapability(mf.Capabilities, "system-telemetry") {
			sidecarTargets = append(sidecarTargets, r)
		}
	}
	m.mutex.Unlock()
	if len(sidecarTargets) == 0 {
		return
	}

	gpuReadable := temp.GPUReadState == "" || temp.GPUReadState == "ok" || temp.GPUTemp > 0
	cpuPowerValid := temp.BridgeOk && len(temp.CpuPowerSensors) > 0 && isFiniteNonNegative(temp.CPUPowerWatts)
	gpuPowerValid := temp.BridgeOk && gpuReadable && len(temp.GpuPowerSensors) > 0 && isFiniteNonNegative(temp.GPUPowerWatts)
	payload := map[string]any{
		"cpuTemp":        temp.CPUTemp,
		"gpuTemp":        temp.GPUTemp,
		"cpuPowerWatts":  temp.CPUPowerWatts,
		"gpuPowerWatts":  temp.GPUPowerWatts,
		"cpuPowerValid":  cpuPowerValid,
		"gpuPowerValid":  gpuPowerValid,
		"gpuReadable":    gpuReadable,
		"cpuModel":       temp.CpuModel,
		"gpuModel":       temp.GpuModel,
		"bridgeOk":       temp.BridgeOk,
		"timestamp":      temp.UpdateTime,
	}
	for _, r := range sidecarTargets {
		_ = r.SendHostEvent("system-telemetry", payload)
	}
}

func isFiniteNonNegative(v float64) bool {
	return v >= 0 && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func hasCapability(caps []string, target string) bool {
	for _, c := range caps {
		if strings.EqualFold(strings.TrimSpace(c), target) {
			return true
		}
	}
	return false
}

func (m *Manager) findLocked(pluginID string) Plugin {
	for _, plugin := range m.plugins {
		if plugin.ID() == pluginID {
			return plugin
		}
	}
	return nil
}

func clonePluginInfoSlice(input []PluginInfo) []PluginInfo {
	if len(input) == 0 {
		return []PluginInfo{}
	}
	out := make([]PluginInfo, len(input))
	for i, item := range input {
		if len(item.Capabilities) > 0 {
			item.Capabilities = append([]string(nil), item.Capabilities...)
		}
		if len(item.Settings) > 0 {
			item.Settings = cloneSettingFields(item.Settings)
		}
		out[i] = item
	}
	return out
}

func (m *Manager) logInfo(format string, v ...any) {
	if m.logger != nil {
		m.logger.Info(format, v...)
	}
}

func (m *Manager) logError(format string, v ...any) {
	if m.logger != nil {
		m.logger.Error(format, v...)
	}
}
