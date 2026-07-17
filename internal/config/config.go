// Package config 提供配置管理功能
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/deviceprofiles"
	"github.com/TIANLI0/THRM/internal/types"
)

const configLearningCurveScopeSeparator = "::curve::"

// Manager 配置管理器
//
// 注意：内部所有对 config 字段的读写均通过 mu 保护，避免并发场景下的数据竞争。
// 公共方法（Get/Set/Update/Save/Load）是并发安全的；不要在持有 mu 的情况下调用
// 这些公共方法（会自死锁）；内部需要无锁版本时使用以 Locked 结尾的私有方法。
type Manager struct {
	mu         sync.RWMutex
	config     types.AppConfig
	revision   uint64
	installDir string
	logger     types.Logger
}

// NewManager 创建新的配置管理器
func NewManager(installDir string, logger types.Logger) *Manager {
	return &Manager{
		installDir: installDir,
		logger:     logger,
	}
}

// Load 加载配置
func (m *Manager) Load(isAutoStart bool) types.AppConfig {
	m.mu.Lock()
	defer m.mu.Unlock()

	defaultConfigDir := m.GetDefaultConfigDir()
	defaultConfigPath := filepath.Join(defaultConfigDir, "config.json")
	installConfigPath := filepath.Join(m.installDir, "config", "config.json")
	legacyInstallConfigPath := filepath.Join(m.installDir, "config.json")
	legacyConfigPaths := make([]string, 0)
	for _, legacyConfigDir := range m.GetLegacyConfigDirs() {
		legacyConfigPaths = append(legacyConfigPaths, filepath.Join(legacyConfigDir, "config.json"))
	}

	m.logInfo("尝试从便携目录加载配置: %s", installConfigPath)

	if m.tryLoadFromPathLocked(installConfigPath) {
		m.config.ConfigPath = installConfigPath
		m.logInfo("从便携目录加载配置成功: %s", installConfigPath)
		m.bumpRevisionLocked()
		return cloneAppConfig(m.config)
	}

	m.logInfo("从便携目录加载配置失败，尝试从用户目录迁移: %s", defaultConfigPath)

	m.logInfo("trying legacy install-root config migration: %s", legacyInstallConfigPath)
	if m.tryLoadFromPathLocked(legacyInstallConfigPath) {
		m.config.ConfigPath = installConfigPath
		m.logInfo("loaded legacy install-root config and migrated it to portable config directory: %s", legacyInstallConfigPath)
		if err := m.saveLocked(); err != nil {
			m.logError("failed to migrate legacy install-root config: %v", err)
		}
		m.bumpRevisionLocked()
		return cloneAppConfig(m.config)
	}

	if m.tryLoadFromPathLocked(defaultConfigPath) {
		m.config.ConfigPath = installConfigPath
		m.logInfo("从用户目录加载配置成功，将迁移到便携目录: %s", defaultConfigPath)
		if err := m.saveLocked(); err != nil {
			m.logError("迁移用户目录配置失败: %v", err)
		}
		m.bumpRevisionLocked()
		return cloneAppConfig(m.config)
	}

	for _, legacyConfigPath := range legacyConfigPaths {
		m.logInfo("从用户目录加载配置失败，尝试从旧目录迁移: %s", legacyConfigPath)

		if m.tryLoadFromPathLocked(legacyConfigPath) {
			m.config.ConfigPath = installConfigPath
			m.logInfo("从旧目录加载配置成功，将迁移到便携目录: %s", legacyConfigPath)
			if err := m.saveLocked(); err != nil {
				m.logError("迁移旧目录配置失败: %v", err)
			}
			m.bumpRevisionLocked()
			return cloneAppConfig(m.config)
		}
	}

	if m.tryLoadLegacyPortableSettingsLocked(isAutoStart, installConfigPath) {
		m.logInfo("从旧版便携配置迁移成功，将保存到便携配置: %s", installConfigPath)
		if err := m.saveLocked(); err != nil {
			m.logError("保存迁移后的配置失败: %v", err)
		}
		m.bumpRevisionLocked()
		return cloneAppConfig(m.config)
	}

	m.logError("所有配置目录加载失败，使用默认配置")

	m.config = types.GetDefaultConfig(isAutoStart)
	m.config.ConfigPath = installConfigPath
	if err := m.saveLocked(); err != nil {
		m.logError("保存默认配置失败: %v", err)
	}
	m.bumpRevisionLocked()

	return cloneAppConfig(m.config)
}

// tryLoadFromPathLocked 尝试从指定路径加载配置（调用方需持有 m.mu）
func (m *Manager) tryLoadFromPathLocked(configPath string) bool {
	if _, err := os.Stat(configPath); err != nil {
		m.logDebug("配置文件不存在: %s", configPath)
		return false
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		m.logError("读取配置文件失败 %s: %v", configPath, err)
		return false
	}

	var rawConfig map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		m.logError("解析配置文件元数据失败 %s: %v", configPath, err)
		return false
	}

	var config types.AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		m.logError("解析配置文件失败 %s: %v", configPath, err)
		return false
	}

	applyMissingHotkeyDefaults(&config, rawConfig)
	applyMissingDeviceDefaults(&config, rawConfig)
	applyMissingSmartControlDefaults(&config, rawConfig)
	applyMissingLegionFnQDefaults(&config, rawConfig)
	applyMissingThemeDefaults(&config, rawConfig)
	applyMissingWindowBlurDefaults(&config, rawConfig)
	applyMissingTemperatureDefaults(&config, rawConfig)
	normalizeSpeedConfig(&config)

	m.config = config
	return true
}

type legacyPortableSettings struct {
	FanControlEnabled                bool   `json:"FanControlEnabled"`
	FanControlDeviceIp               string `json:"FanControlDeviceIp"`
	FanControlMode                   string `json:"FanControlMode"`
	FanControlTemperatureSource      string `json:"FanControlTemperatureSource"`
	FanControlCpuTemperatureSensorId string `json:"FanControlCpuTemperatureSensorId"`
	FanControlGpuTemperatureSensorId string `json:"FanControlGpuTemperatureSensorId"`
	FanControlManualSpeed            int    `json:"FanControlManualSpeed"`
	FanControlMinimumAutoSpeed       int    `json:"FanControlMinimumAutoSpeed"`
	FanControlMinSpeedDelta          int    `json:"FanControlMinSpeedDelta"`
	FanControlCurve                  string `json:"FanControlCurve"`
	StartWithWindows                 bool   `json:"StartWithWindows"`
	StartMinimized                   bool   `json:"StartMinimized"`
	CloseToTray                      bool   `json:"CloseToTray"`
}

func (m *Manager) tryLoadLegacyPortableSettingsLocked(isAutoStart bool, targetPath string) bool {
	for _, path := range m.legacyPortableSettingsCandidates() {
		if strings.TrimSpace(path) == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var legacy legacyPortableSettings
		if err := json.Unmarshal(data, &legacy); err != nil {
			m.logError("解析旧版便携配置失败 %s: %v", path, err)
			continue
		}

		cfg := types.GetDefaultConfig(isAutoStart)
		applyLegacyPortableSettings(&cfg, legacy)
		cfg.ConfigPath = targetPath
		m.config = cfg
		m.logInfo("已读取旧版便携配置: %s", path)
		return true
	}
	return false
}

func (m *Manager) legacyPortableSettingsCandidates() []string {
	candidates := []string{}
	add := func(path string) {
		if strings.TrimSpace(path) == "" {
			return
		}
		for _, existing := range candidates {
			if strings.EqualFold(existing, path) {
				return
			}
		}
		candidates = append(candidates, path)
	}

	add(filepath.Join(m.installDir, "settings.json"))
	return candidates
}

func applyLegacyPortableSettings(cfg *types.AppConfig, legacy legacyPortableSettings) {
	if cfg == nil {
		return
	}

	cfg.DeviceTransport = types.DeviceTransportWiFi
	if ip := strings.TrimSpace(legacy.FanControlDeviceIp); ip != "" {
		cfg.FanControlDeviceIp = ip
	}

	cfg.AutoControl = legacy.FanControlEnabled && strings.EqualFold(legacy.FanControlMode, "auto")
	cfg.WindowsAutoStart = legacy.StartWithWindows
	cfg.CustomSpeedRPM = types.ClampFanPercent(legacy.FanControlManualSpeed)
	cfg.CustomSpeedEnabled = legacy.FanControlEnabled && strings.EqualFold(legacy.FanControlMode, "manual")
	if legacy.FanControlMinSpeedDelta > 0 {
		cfg.SmartControl.MinRPMChange = types.ClampFanPercent(legacy.FanControlMinSpeedDelta)
		if cfg.SmartControl.MinRPMChange == 0 {
			cfg.SmartControl.MinRPMChange = 1
		}
	}

	switch strings.ToLower(strings.TrimSpace(legacy.FanControlTemperatureSource)) {
	case types.TempSourceCPU:
		cfg.TempSource = types.TempSourceCPU
	case types.TempSourceGPU:
		cfg.TempSource = types.TempSourceGPU
	default:
		cfg.TempSource = types.TempSourceMax
	}
	if sensor := strings.TrimSpace(legacy.FanControlCpuTemperatureSensorId); sensor != "" {
		cfg.CpuSensor = sensor
	}
	if sensor := strings.TrimSpace(legacy.FanControlGpuTemperatureSensorId); sensor != "" {
		cfg.GpuSensor = sensor
	}

	if curve := parseLegacyFanCurve(legacy.FanControlCurve, legacy.FanControlMinimumAutoSpeed); len(curve) >= 2 {
		cfg.FanCurve = curve
		cfg.FanCurveProfiles = []types.FanCurveProfile{
			{ID: "default", Name: "默认", Curve: curve},
		}
		cfg.ActiveFanCurveProfileID = "default"
		cfg.SmartControl = types.GetDefaultSmartControlConfig(curve)
	}

	normalizeSpeedConfig(cfg)
}

func parseLegacyFanCurve(raw string, minSpeed int) []types.FanCurvePoint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	points := []types.FanCurvePoint{}
	for _, part := range strings.Split(raw, ",") {
		pair := strings.Split(strings.TrimSpace(part), ":")
		if len(pair) != 2 {
			continue
		}
		temp, tempErr := strconv.Atoi(strings.TrimSpace(pair[0]))
		speed, speedErr := strconv.Atoi(strings.TrimSpace(pair[1]))
		if tempErr != nil || speedErr != nil {
			continue
		}
		if temp < 20 {
			temp = 20
		}
		if temp > types.FanCurveMaxTemperature {
			temp = types.FanCurveMaxTemperature
		}
		speed = types.ClampFanPercent(speed)
		if minSpeed > 0 && speed < minSpeed {
			speed = types.ClampFanPercent(minSpeed)
		}
		points = append(points, types.FanCurvePoint{Temperature: temp, RPM: speed})
	}
	if len(points) == 0 {
		return nil
	}

	points = append(points, types.FanCurvePoint{Temperature: 20, RPM: types.ClampFanPercent(max(minSpeed, 20))})
	points = append(points, types.FanCurvePoint{Temperature: types.FanCurveMaxTemperature, RPM: types.FanSpeedMaxPercent})

	sort.Slice(points, func(i, j int) bool {
		return points[i].Temperature < points[j].Temperature
	})

	merged := make([]types.FanCurvePoint, 0, len(points))
	for _, point := range points {
		if len(merged) > 0 && merged[len(merged)-1].Temperature == point.Temperature {
			merged[len(merged)-1].RPM = max(merged[len(merged)-1].RPM, point.RPM)
			continue
		}
		if len(merged) > 0 && point.RPM < merged[len(merged)-1].RPM {
			point.RPM = merged[len(merged)-1].RPM
		}
		merged = append(merged, point)
	}

	if len(merged) >= 2 {
		return merged
	}
	return nil
}

func applyMissingHotkeyDefaults(cfg *types.AppConfig, rawConfig map[string]json.RawMessage) {
	if cfg == nil {
		return
	}

	defaults := types.GetDefaultConfig(false)
	if _, ok := rawConfig["manualGearToggleHotkey"]; !ok {
		cfg.ManualGearToggleHotkey = defaults.ManualGearToggleHotkey
	}
	if _, ok := rawConfig["autoControlToggleHotkey"]; !ok {
		cfg.AutoControlToggleHotkey = defaults.AutoControlToggleHotkey
	}
	if _, ok := rawConfig["curveProfileToggleHotkey"]; !ok {
		cfg.CurveProfileToggleHotkey = defaults.CurveProfileToggleHotkey
	}
}

func applyMissingSmartControlDefaults(cfg *types.AppConfig, rawConfig map[string]json.RawMessage) {
	if cfg == nil {
		return
	}

	defaults := types.GetDefaultSmartControlConfigForUnit(cfg.FanCurve, types.DeviceProfileSpeedUnit(cfg))
	rawSmartControl, ok := rawConfig["smartControl"]
	if !ok {
		cfg.SmartControl.FilterTransientSpike = defaults.FilterTransientSpike
		cfg.SmartControl.LearningBias = defaults.LearningBias
		cfg.SmartControl.TemperatureRisePrediction = defaults.TemperatureRisePrediction
		cfg.SmartControl.TemperatureRisePredictionMaxBoost = defaults.TemperatureRisePredictionMaxBoost
		return
	}

	var smartControlConfig map[string]json.RawMessage
	if err := json.Unmarshal(rawSmartControl, &smartControlConfig); err != nil {
		return
	}

	if _, ok := smartControlConfig["filterTransientSpike"]; !ok {
		cfg.SmartControl.FilterTransientSpike = defaults.FilterTransientSpike
	}
	if _, ok := smartControlConfig["learningBias"]; !ok {
		cfg.SmartControl.LearningBias = defaults.LearningBias
	}
	if _, ok := smartControlConfig["temperatureRisePrediction"]; !ok {
		cfg.SmartControl.TemperatureRisePrediction = defaults.TemperatureRisePrediction
	}
	if _, ok := smartControlConfig["temperatureRisePredictionMaxBoost"]; !ok {
		cfg.SmartControl.TemperatureRisePredictionMaxBoost = defaults.TemperatureRisePredictionMaxBoost
	}
}

func applyMissingLegionFnQDefaults(cfg *types.AppConfig, rawConfig map[string]json.RawMessage) {
	if cfg == nil {
		return
	}
	if _, ok := rawConfig["legionFnQ"]; !ok {
		cfg.LegionFnQ = types.GetDefaultLegionFnQConfig()
		return
	}
	cfg.LegionFnQ = types.NormalizeLegionFnQConfig(cfg.LegionFnQ)
}

func applyMissingThemeDefaults(cfg *types.AppConfig, rawConfig map[string]json.RawMessage) {
	if cfg == nil {
		return
	}

	defaultThemeMode := types.ThemeModeSystem
	if _, ok := rawConfig["themeMode"]; !ok {
		cfg.ThemeMode = defaultThemeMode
		return
	}

	cfg.ThemeMode = types.NormalizeThemeMode(cfg.ThemeMode)
}

func applyMissingWindowBlurDefaults(cfg *types.AppConfig, rawConfig map[string]json.RawMessage) {
	if cfg == nil {
		return
	}
	if _, ok := rawConfig["windowBlur"]; !ok {
		cfg.WindowBlur = types.WindowBlurAcrylic
		return
	}
	cfg.WindowBlur = types.NormalizeWindowBlur(cfg.WindowBlur)
}

func applyMissingTemperatureDefaults(cfg *types.AppConfig, rawConfig map[string]json.RawMessage) {
	if cfg == nil {
		return
	}

	defaults := types.GetDefaultTemperatureSelection()
	if _, ok := rawConfig["tempSource"]; !ok {
		cfg.TempSource = defaults.TempSource
	}
	if _, ok := rawConfig["gpuDevice"]; !ok {
		cfg.GpuDevice = defaults.GpuDevice
	}
	if _, ok := rawConfig["cpuSensor"]; !ok {
		cfg.CpuSensor = defaults.CpuSensor
	}
	if _, ok := rawConfig["gpuSensor"]; !ok {
		cfg.GpuSensor = defaults.GpuSensor
	}
	if _, ok := rawConfig["cpuPowerSensor"]; !ok {
		cfg.CpuPowerSensor = defaults.CpuPowerSensor
	}
	if _, ok := rawConfig["gpuPowerSensor"]; !ok {
		cfg.GpuPowerSensor = defaults.GpuPowerSensor
	}
	if _, ok := rawConfig["gpuLowPowerProtection"]; !ok {
		cfg.GpuLowPowerProtection = defaults.GpuLowPowerProtection
	}
	if _, ok := rawConfig["gpuReadMode"]; !ok {
		if _, hasLegacy := rawConfig["gpuLowPowerProtection"]; hasLegacy && !cfg.GpuLowPowerProtection {
			cfg.GpuReadMode = types.GPUReadModeAlways
		} else {
			cfg.GpuReadMode = defaults.GpuReadMode
		}
	}
	cfg.TempSource = types.NormalizeTempSource(cfg.TempSource)
	cfg.GpuDevice = types.NormalizeDeviceSelection(cfg.GpuDevice)
	cfg.CpuSensor = types.NormalizeSensorSelection(cfg.CpuSensor)
	cfg.GpuSensor = types.NormalizeSensorSelection(cfg.GpuSensor)
	cfg.CpuPowerSensor = types.NormalizeSensorSelection(cfg.CpuPowerSensor)
	cfg.GpuPowerSensor = types.NormalizeSensorSelection(cfg.GpuPowerSensor)
	cfg.GpuReadMode = types.NormalizeGPUReadMode(cfg.GpuReadMode)
	cfg.GpuLowPowerProtection = cfg.GpuReadMode != types.GPUReadModeAlways
}

func applyMissingDeviceDefaults(cfg *types.AppConfig, rawConfig map[string]json.RawMessage) {
	if cfg == nil {
		return
	}

	defaults := types.GetDefaultConfig(false)
	if _, ok := rawConfig["activeDeviceProfileId"]; !ok {
		cfg.ActiveDeviceProfileID = defaults.ActiveDeviceProfileID
	}
	if _, ok := rawConfig["activeDeviceProfileIdsByTransport"]; !ok {
		cfg.ActiveDeviceProfileIDsByTransport = defaults.ActiveDeviceProfileIDsByTransport
	}
	if _, ok := rawConfig["deviceTransport"]; !ok {
		cfg.DeviceTransport = defaults.DeviceTransport
	}
	if _, ok := rawConfig["fanControlDeviceIp"]; !ok {
		cfg.FanControlDeviceIp = defaults.FanControlDeviceIp
	}
	cfg.DeviceTransport = types.NormalizeDeviceTransport(cfg.DeviceTransport)
	if strings.TrimSpace(cfg.FanControlDeviceIp) == "" {
		cfg.FanControlDeviceIp = defaults.FanControlDeviceIp
	}
	if _, ok := rawConfig["wifiCompatibilityEnabled"]; !ok {
		cfg.WiFiCompatibilityEnabled = true
	}
	if _, ok := rawConfig["wifiDynamicIpCompatibilityEnabled"]; !ok {
		cfg.WiFiDynamicIPCompatibilityEnabled = true
	}
	if _, ok := rawConfig["ignoreDeviceOnReconnect"]; !ok {
		cfg.IgnoreDeviceOnReconnect = defaults.IgnoreDeviceOnReconnect
	}
	if _, ok := rawConfig["wifiSmartStartStopStandbySpeed"]; !ok {
		cfg.WiFiSmartStartStopStandbySpeed = types.WiFiSmartStartStopStandbyMinPercent
	}
	cfg.WiFiSmartStartStopStandbySpeed = types.ClampWiFiSmartStartStopStandbyPercent(cfg.WiFiSmartStartStopStandbySpeed)
	if _, ok := rawConfig["serialCompatibilityEnabled"]; !ok {
		cfg.SerialCompatibilityEnabled = cfg.DeviceTransport == types.DeviceTransportSerial
	}
	if _, ok := rawConfig["deviceProfiles"]; !ok {
		cfg.DeviceProfiles = []types.DeviceProfile{types.DefaultWiFiPercentProfile(cfg.FanControlDeviceIp)}
	}
	preserveNativeFanCurveStateBeforeCompatibilityMigration(cfg)
	types.NormalizeDeviceProfileConfig(cfg)
}

func preserveNativeFanCurveStateBeforeCompatibilityMigration(cfg *types.AppConfig) {
	profile, ok := rawNativeActiveProfile(cfg)
	if !ok || !types.IsRPMSpeedUnit(profile.SpeedUnit) {
		return
	}
	key := deviceCurveScopeKeyForProfile(profile)
	if key == "" {
		return
	}
	if cfg.FanCurveProfilesByDevice == nil {
		cfg.FanCurveProfilesByDevice = map[string]types.DeviceFanCurveProfilesState{}
	}
	state := cfg.FanCurveProfilesByDevice[key]
	changed := false
	if len(state.Profiles) == 0 && len(state.FanCurve) == 0 {
		state.Profiles = cloneFanCurveProfiles(cfg.FanCurveProfiles)
		state.ActiveID = strings.TrimSpace(cfg.ActiveFanCurveProfileID)
		state.FanCurve = cloneFanCurve(cfg.FanCurve)
		changed = true
	}
	if len(state.ManualGearRPM) == 0 {
		state.ManualGearRPM = normalizedManualGearRPMForUnit(cfg.ManualGearRPM, profile.SpeedUnit)
		changed = true
	}
	preserveNativeLearningOffsetsBeforeCompatibilityMigration(cfg, key)
	if len(state.Profiles) == 0 && len(state.FanCurve) == 0 && len(state.ManualGearRPM) == 0 {
		return
	}
	if changed {
		cfg.FanCurveProfilesByDevice[key] = state
	}
}

func rawNativeActiveProfile(cfg *types.AppConfig) (types.DeviceProfile, bool) {
	if cfg == nil {
		return types.DeviceProfile{}, false
	}
	findByID := func(id string) (types.DeviceProfile, bool) {
		id = strings.TrimSpace(id)
		if id == "" {
			return types.DeviceProfile{}, false
		}
		for _, profile := range cfg.DeviceProfiles {
			if strings.TrimSpace(profile.ID) == id {
				profile = types.NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp)
				return profile, types.IsNativeDeviceTransport(profile.Transport)
			}
		}
		if profile, ok := deviceprofiles.BuiltInProfileByID(id); ok {
			profile = types.NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp)
			return profile, types.IsNativeDeviceTransport(profile.Transport)
		}
		return types.DeviceProfile{}, false
	}
	if profile, ok := findByID(cfg.ActiveDeviceProfileID); ok {
		return profile, true
	}
	for _, transport := range []string{types.DeviceTransportBLE, types.DeviceTransportHID} {
		if profile, ok := findByID(cfg.ActiveDeviceProfileIDsByTransport[transport]); ok {
			return profile, true
		}
	}
	switch types.NormalizeDeviceTransport(cfg.DeviceTransport) {
	case types.DeviceTransportBLE:
		return types.FlyDigiBS1Profile(), true
	case types.DeviceTransportHID:
		return types.LegacyRPMProfileForTransport(types.DeviceTransportHID), true
	default:
		return types.DeviceProfile{}, false
	}
}

func deviceCurveScopeKeyForProfile(profile types.DeviceProfile) string {
	transport := types.NormalizeDeviceTransport(profile.Transport)
	profileID := strings.TrimSpace(profile.ID)
	if profileID == "" {
		profileID = strings.TrimSpace(profile.Model)
	}
	if profileID == "" {
		profileID = strings.TrimSpace(profile.DisplayName)
	}
	if transport == "" || profileID == "" {
		return ""
	}
	return transport + "::" + profileID
}

func cloneFanCurve(curve []types.FanCurvePoint) []types.FanCurvePoint {
	if len(curve) == 0 {
		return nil
	}
	out := make([]types.FanCurvePoint, len(curve))
	copy(out, curve)
	return out
}

func cloneFanCurveProfiles(profiles []types.FanCurveProfile) []types.FanCurveProfile {
	if len(profiles) == 0 {
		return nil
	}
	out := make([]types.FanCurveProfile, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, types.FanCurveProfile{
			ID:    profile.ID,
			Name:  profile.Name,
			Curve: cloneFanCurve(profile.Curve),
		})
	}
	return out
}

func cloneManualGearRPM(input map[string]map[string]int) map[string]map[string]int {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]map[string]int, len(input))
	for gear, levels := range input {
		gear = strings.TrimSpace(gear)
		if gear == "" || len(levels) == 0 {
			continue
		}
		inner := make(map[string]int, len(levels))
		for level, rpm := range levels {
			if level = strings.TrimSpace(level); level != "" {
				inner[level] = rpm
			}
		}
		if len(inner) > 0 {
			out[gear] = inner
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizedManualGearRPMForUnit(input map[string]map[string]int, unit string) map[string]map[string]int {
	tmp := types.AppConfig{ManualGearRPM: cloneManualGearRPM(input)}
	types.NormalizeManualGearRPMForUnit(&tmp, unit)
	return cloneManualGearRPM(tmp.ManualGearRPM)
}

func cloneIntSlice(input []int) []int {
	if len(input) == 0 {
		return nil
	}
	out := make([]int, len(input))
	copy(out, input)
	return out
}

func cloneStringMapExact(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneBoolMapExact(input map[string]bool) map[string]bool {
	if input == nil {
		return nil
	}
	out := make(map[string]bool, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneNestedIntMapExact(input map[string]map[string]int) map[string]map[string]int {
	if input == nil {
		return nil
	}
	out := make(map[string]map[string]int, len(input))
	for key, values := range input {
		if values == nil {
			out[key] = nil
			continue
		}
		inner := make(map[string]int, len(values))
		for innerKey, value := range values {
			inner[innerKey] = value
		}
		out[key] = inner
	}
	return out
}

func cloneIntSliceExact(input []int) []int {
	if input == nil {
		return nil
	}
	return append([]int{}, input...)
}

func cloneFanCurveExact(input []types.FanCurvePoint) []types.FanCurvePoint {
	if input == nil {
		return nil
	}
	return append([]types.FanCurvePoint{}, input...)
}

func cloneFanCurveProfilesExact(input []types.FanCurveProfile) []types.FanCurveProfile {
	if input == nil {
		return nil
	}
	out := make([]types.FanCurveProfile, len(input))
	for index, profile := range input {
		profile.Curve = cloneFanCurveExact(profile.Curve)
		out[index] = profile
	}
	return out
}

func cloneDeviceProfilesExact(input []types.DeviceProfile) []types.DeviceProfile {
	if input == nil {
		return nil
	}
	out := make([]types.DeviceProfile, len(input))
	for index, profile := range input {
		out[index] = deviceprofiles.CloneProfile(profile)
	}
	return out
}

func cloneDeviceCurveStatesExact(input map[string]types.DeviceFanCurveProfilesState) map[string]types.DeviceFanCurveProfilesState {
	if input == nil {
		return nil
	}
	out := make(map[string]types.DeviceFanCurveProfilesState, len(input))
	for key, state := range input {
		state.Profiles = cloneFanCurveProfilesExact(state.Profiles)
		state.FanCurve = cloneFanCurveExact(state.FanCurve)
		state.ManualGearRPM = cloneNestedIntMapExact(state.ManualGearRPM)
		out[key] = state
	}
	return out
}

func cloneLearnedOffsetsExact(input map[string][]int) map[string][]int {
	if input == nil {
		return nil
	}
	out := make(map[string][]int, len(input))
	for key, offsets := range input {
		out[key] = cloneIntSliceExact(offsets)
	}
	return out
}

func cloneSmartControlConfig(input types.SmartControlConfig) types.SmartControlConfig {
	input.LearnedOffsets = cloneIntSliceExact(input.LearnedOffsets)
	input.LearnedOffsetsHeat = cloneIntSliceExact(input.LearnedOffsetsHeat)
	input.LearnedOffsetsCool = cloneIntSliceExact(input.LearnedOffsetsCool)
	input.LearnedRateHeat = cloneIntSliceExact(input.LearnedRateHeat)
	input.LearnedRateCool = cloneIntSliceExact(input.LearnedRateCool)
	input.LearnedOffsetsByProfile = cloneLearnedOffsetsExact(input.LearnedOffsetsByProfile)
	return input
}

func cloneAppConfig(input types.AppConfig) types.AppConfig {
	input.LegionFnQ.ModeMapping = func() map[string]types.FanGearTarget {
		if input.LegionFnQ.ModeMapping == nil {
			return nil
		}
		out := make(map[string]types.FanGearTarget, len(input.LegionFnQ.ModeMapping))
		for key, value := range input.LegionFnQ.ModeMapping {
			out[key] = value
		}
		return out
	}()
	input.PluginEnabled = cloneBoolMapExact(input.PluginEnabled)
	input.ActiveDeviceProfileIDsByTransport = cloneStringMapExact(input.ActiveDeviceProfileIDsByTransport)
	input.DeviceProfiles = cloneDeviceProfilesExact(input.DeviceProfiles)
	input.ManualGearLevels = cloneStringMapExact(input.ManualGearLevels)
	input.ManualGearRPM = cloneNestedIntMapExact(input.ManualGearRPM)
	input.FanCurve = cloneFanCurveExact(input.FanCurve)
	input.FanCurveProfiles = cloneFanCurveProfilesExact(input.FanCurveProfiles)
	input.FanCurveProfilesByDevice = cloneDeviceCurveStatesExact(input.FanCurveProfilesByDevice)
	input.SmartControl = cloneSmartControlConfig(input.SmartControl)
	if input.LightStrip.Colors != nil {
		input.LightStrip.Colors = append([]types.RGBColor{}, input.LightStrip.Colors...)
	}
	return input
}

func preserveNativeLearningOffsetsBeforeCompatibilityMigration(cfg *types.AppConfig, deviceKey string) bool {
	if cfg == nil || strings.TrimSpace(deviceKey) == "" || strings.TrimSpace(cfg.ActiveFanCurveProfileID) == "" {
		return false
	}
	deviceKey = strings.TrimSpace(deviceKey)
	profileID := strings.TrimSpace(cfg.ActiveFanCurveProfileID)
	scopedKey := deviceKey + configLearningCurveScopeSeparator + profileID
	if cfg.SmartControl.LearnedOffsetsByProfile == nil {
		cfg.SmartControl.LearnedOffsetsByProfile = map[string][]int{}
	}
	if _, ok := cfg.SmartControl.LearnedOffsetsByProfile[scopedKey]; ok {
		return false
	}
	if offsets, ok := cfg.SmartControl.LearnedOffsetsByProfile[profileID]; ok && len(offsets) > 0 {
		cfg.SmartControl.LearnedOffsetsByProfile[scopedKey] = cloneIntSlice(offsets)
		return true
	}
	if len(cfg.SmartControl.LearnedOffsets) > 0 {
		cfg.SmartControl.LearnedOffsetsByProfile[scopedKey] = cloneIntSlice(cfg.SmartControl.LearnedOffsets)
		return true
	}
	return false
}

func normalizeSpeedConfig(cfg *types.AppConfig) {
	if cfg == nil {
		return
	}
	types.NormalizeDeviceProfileConfig(cfg)
	unit := types.DeviceProfileSpeedUnit(cfg)
	cfg.DeviceTransport = types.NormalizeDeviceTransport(cfg.DeviceTransport)
	cfg.FanControlDeviceIp = strings.TrimSpace(cfg.FanControlDeviceIp)
	if cfg.FanControlDeviceIp == "" {
		cfg.FanControlDeviceIp = types.DefaultFanDeviceIP
	}
	defaultCurve := defaultFanCurveForUnit(unit)
	speedSanitized := false
	for i := range cfg.FanCurve {
		fallback := defaultFanCurveSpeedAt(defaultCurve, i, cfg.FanCurve[i].Temperature, unit)
		next, changed := normalizeFanSpeedSettingForUnit(cfg.FanCurve[i].RPM, fallback, unit)
		cfg.FanCurve[i].RPM = next
		speedSanitized = speedSanitized || changed
	}
	for i := range cfg.FanCurveProfiles {
		for j := range cfg.FanCurveProfiles[i].Curve {
			fallback := defaultFanCurveSpeedAt(defaultCurve, j, cfg.FanCurveProfiles[i].Curve[j].Temperature, unit)
			next, changed := normalizeFanSpeedSettingForUnit(cfg.FanCurveProfiles[i].Curve[j].RPM, fallback, unit)
			cfg.FanCurveProfiles[i].Curve[j].RPM = next
			speedSanitized = speedSanitized || changed
		}
	}
	nextCustomSpeed, customSpeedChanged := normalizeFanSpeedSettingForUnit(cfg.CustomSpeedRPM, defaultCustomSpeedForUnit(unit), unit)
	cfg.CustomSpeedRPM = nextCustomSpeed
	speedSanitized = speedSanitized || customSpeedChanged
	if speedSanitized {
		resetLearnedPercentOffsets(cfg)
	}
}

func normalizeFanSpeedSetting(speed, fallback int) (int, bool) {
	return normalizeFanSpeedSettingForUnit(speed, fallback, types.FanSpeedUnitPercent)
}

func normalizeFanSpeedSettingForUnit(speed, fallback int, unit string) (int, bool) {
	if types.IsRPMSpeedUnit(unit) {
		minSpeed, maxSpeed := types.SpeedRangeForUnit(unit)
		if speed < minSpeed || speed > maxSpeed {
			// 越界时 fallback 必须是该单位的默认值，而不是其他单位的值。
			return clampSpeedToUnit(fallback, unit), true
		}
		return speed, false
	}
	if speed < types.FanSpeedMinPercent || speed > types.FanSpeedMaxPercent {
		// percent 单位越界时 fallback 也用 percent 默认值，避免 RPM 默认值(如 2000)
		// 被当作 percent 值错误的 clamp 后落到极小值区间。
		return types.ClampFanPercent(fallback), true
	}
	return speed, false
}

func defaultFanCurveForUnit(unit string) []types.FanCurvePoint {
	if types.IsRPMSpeedUnit(unit) {
		return types.GetDefaultRPMFanCurve()
	}
	return types.GetDefaultFanCurve()
}

func defaultCustomSpeedForUnit(unit string) int {
	if types.IsRPMSpeedUnit(unit) {
		return 2000
	}
	return 45
}

func clampSpeedToUnit(speed int, unit string) int {
	if types.IsRPMSpeedUnit(unit) {
		minSpeed, maxSpeed := types.SpeedRangeForUnit(unit)
		if speed < minSpeed {
			return minSpeed
		}
		if speed > maxSpeed {
			return maxSpeed
		}
		return speed
	}
	return types.ClampFanPercent(speed)
}

func resetLearnedPercentOffsets(cfg *types.AppConfig) {
	if cfg == nil {
		return
	}
	scopedLearning := cloneScopedLearningOffsets(cfg.SmartControl.LearnedOffsetsByProfile)
	curveLen := len(cfg.FanCurve)
	cfg.SmartControl.LearnedOffsets = make([]int, curveLen)
	cfg.SmartControl.LearnedOffsetsHeat = make([]int, curveLen)
	cfg.SmartControl.LearnedOffsetsCool = make([]int, curveLen)
	cfg.SmartControl.LearnedRateHeat = make([]int, 7)
	cfg.SmartControl.LearnedRateCool = make([]int, 7)
	cfg.SmartControl.LearnedOffsetsByProfile = scopedLearning
}

func cloneScopedLearningOffsets(input map[string][]int) map[string][]int {
	if len(input) == 0 {
		return nil
	}
	out := map[string][]int{}
	for key, offsets := range input {
		key = strings.TrimSpace(key)
		if key == "" || !strings.Contains(key, configLearningCurveScopeSeparator) {
			continue
		}
		out[key] = cloneIntSlice(offsets)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func defaultFanCurveSpeedAt(defaultCurve []types.FanCurvePoint, index int, temperature int, unit string) int {
	if len(defaultCurve) == 0 {
		return defaultCustomSpeedForUnit(unit)
	}
	if index >= 0 && index < len(defaultCurve) {
		return defaultCurve[index].RPM
	}
	best := defaultCurve[0]
	bestDistance := absInt(temperature - best.Temperature)
	for _, point := range defaultCurve[1:] {
		if distance := absInt(temperature - point.Temperature); distance < bestDistance {
			best = point
			bestDistance = distance
		}
	}
	return best.RPM
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// Save 保存配置（线程安全）
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveLocked()
}

// saveLocked 保存配置（调用方需持有 m.mu 写锁）
func (m *Manager) saveLocked() error {
	installConfigDir := filepath.Join(m.installDir, "config")
	installConfigPath := filepath.Join(installConfigDir, "config.json")
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		m.logError("序列化配置失败: %v", err)
		return err
	}

	m.logDebug("尝试保存配置到便携目录: %s", installConfigPath)
	if err := writeConfigFileAtomically(installConfigPath, data); err != nil {
		m.logError("保存配置到便携目录失败: %v", err)
	} else {
		m.config.ConfigPath = installConfigPath
		m.logInfo("配置保存到便携目录成功: %s", installConfigPath)
		return nil
	}

	defaultConfigDir := m.GetDefaultConfigDir()
	defaultConfigPath := filepath.Join(defaultConfigDir, "config.json")

	m.logInfo("保存到便携目录失败，尝试保存到用户目录: %s", defaultConfigPath)

	if err := writeConfigFileAtomically(defaultConfigPath, data); err != nil {
		m.logError("保存配置到用户目录失败: %v", err)
		return err
	}

	m.config.ConfigPath = defaultConfigPath
	m.logInfo("配置保存到用户目录成功: %s", defaultConfigPath)
	return nil
}

func writeConfigFileAtomically(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	file, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)

	if err := file.Chmod(0644); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

// GetDefaultConfigDir 获取默认配置目录
func (m *Manager) GetDefaultConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		m.logError("获取用户主目录失败: %v", err)
		return filepath.Join(m.installDir, "config")
	}
	return appmeta.UserConfigDir(homeDir)
}

// GetLegacyConfigDir 获取旧版本配置目录
func (m *Manager) GetLegacyConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(m.installDir, "config")
	}
	return appmeta.LegacyUserConfigDir(homeDir)
}

// GetLegacyConfigDirs 获取可迁移的旧版本配置目录
func (m *Manager) GetLegacyConfigDirs() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return []string{filepath.Join(m.installDir, "config")}
	}
	return appmeta.LegacyUserConfigDirs(homeDir)
}

// Get 获取当前配置（线程安全，返回拷贝）
func (m *Manager) Get() types.AppConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneAppConfig(m.config)
}

func (m *Manager) GetWithRevision() (types.AppConfig, uint64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneAppConfig(m.config), m.revision
}

// Revision 返回当前配置版本号，不拷贝配置内容。
// 供监控循环在 revision 未变时跳过全量 AppConfig 拷贝，降低 GC 压力。
func (m *Manager) Revision() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.revision
}

// Set 设置配置（线程安全）
func (m *Manager) Set(config types.AppConfig) {
	config = cloneAppConfig(config)
	m.mu.Lock()
	m.config = config
	m.bumpRevisionLocked()
	m.mu.Unlock()
}

// Update 更新配置并保存（线程安全，原子操作）
func (m *Manager) Update(config types.AppConfig) error {
	config = cloneAppConfig(config)
	m.mu.Lock()
	defer m.mu.Unlock()
	previous := m.config
	m.config = config
	if err := m.saveLocked(); err != nil {
		m.config = previous
		return err
	}
	m.bumpRevisionLocked()
	return nil
}

// MutateIfRevision atomically applies a narrow in-memory change when the
// caller still holds the current snapshot. Persistence remains explicit via Save.
func (m *Manager) MutateIfRevision(expected uint64, mutate func(*types.AppConfig)) (types.AppConfig, uint64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mutate == nil || m.revision != expected {
		return cloneAppConfig(m.config), m.revision, false
	}
	next := cloneAppConfig(m.config)
	mutate(&next)
	m.config = cloneAppConfig(next)
	m.bumpRevisionLocked()
	return cloneAppConfig(m.config), m.revision, true
}

func (m *Manager) bumpRevisionLocked() {
	m.revision++
}

// 日志辅助方法
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

func (m *Manager) logDebug(format string, v ...any) {
	if m.logger != nil {
		m.logger.Debug(format, v...)
	}
}

// GetConfigDir 获取配置目录（保持向后兼容）
func (m *Manager) GetConfigDir() string {
	return m.GetDefaultConfigDir()
}

// GetInstallDir 获取安装目录
func GetInstallDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exePath)
}

// GetCurrentWorkingDir 获取当前工作目录
func GetCurrentWorkingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return dir
}

// ValidateFanCurve 验证风扇曲线是否有效
func ValidateFanCurve(curve []types.FanCurvePoint) error {
	return ValidateFanCurveForUnit(curve, types.FanSpeedUnitPercent)
}

func ValidateFanCurveForUnit(curve []types.FanCurvePoint, unit string) error {
	if len(curve) < 2 {
		return fmt.Errorf("风扇曲线至少需要2个点")
	}
	minSpeed, maxSpeed := types.SpeedRangeForUnit(unit)
	unitLabel := types.FanSpeedDisplaySuffix(unit)

	for i, point := range curve {
		if point.Temperature < 0 {
			return fmt.Errorf("风扇曲线第%d个点温度超出范围(最低0°C)", i+1)
		}
		if point.Temperature > types.FanCurveMaxTemperature {
			return fmt.Errorf("风扇曲线第%d个点温度超出范围(最高%d°C)", i+1, types.FanCurveMaxTemperature)
		}
		if point.RPM < minSpeed || point.RPM > maxSpeed {
			return fmt.Errorf("风扇曲线第%d个点速度超出范围(%d-%d%s)", i+1, minSpeed, maxSpeed, unitLabel)
		}
	}

	for i := 1; i < len(curve); i++ {
		if curve[i].Temperature <= curve[i-1].Temperature {
			return fmt.Errorf("风扇曲线温度点必须递增")
		}
		if curve[i].RPM < curve[i-1].RPM {
			return fmt.Errorf("风扇曲线速度点必须从左到右非递减")
		}
	}

	return nil
}
