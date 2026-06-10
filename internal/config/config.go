// Package config 提供配置管理功能
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/types"
)

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
		return m.config
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
		return m.config
	}

	if m.tryLoadFromPathLocked(defaultConfigPath) {
		m.config.ConfigPath = installConfigPath
		m.logInfo("从用户目录加载配置成功，将迁移到便携目录: %s", defaultConfigPath)
		if err := m.saveLocked(); err != nil {
			m.logError("迁移用户目录配置失败: %v", err)
		}
		m.bumpRevisionLocked()
		return m.config
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
			return m.config
		}
	}

	if m.tryLoadLegacyPortableSettingsLocked(isAutoStart, installConfigPath) {
		m.logInfo("从旧版便携配置迁移成功，将保存到便携配置: %s", installConfigPath)
		if err := m.saveLocked(); err != nil {
			m.logError("保存迁移后的配置失败: %v", err)
		}
		m.bumpRevisionLocked()
		return m.config
	}

	m.logError("所有配置目录加载失败，使用默认配置")

	m.config = types.GetDefaultConfig(isAutoStart)
	m.config.ConfigPath = installConfigPath
	if err := m.saveLocked(); err != nil {
		m.logError("保存默认配置失败: %v", err)
	}
	m.bumpRevisionLocked()

	return m.config
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

	for i := 1; i < len(points); i++ {
		for j := i; j > 0 && points[j].Temperature < points[j-1].Temperature; j-- {
			points[j], points[j-1] = points[j-1], points[j]
		}
	}

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
	cfg.TempSource = types.NormalizeTempSource(cfg.TempSource)
	cfg.GpuDevice = types.NormalizeDeviceSelection(cfg.GpuDevice)
	cfg.CpuSensor = types.NormalizeSensorSelection(cfg.CpuSensor)
	cfg.GpuSensor = types.NormalizeSensorSelection(cfg.GpuSensor)
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
		cfg.WiFiCompatibilityEnabled = cfg.DeviceTransport == types.DeviceTransportWiFi || strings.TrimSpace(cfg.FanControlDeviceIp) != ""
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
	types.NormalizeDeviceProfileConfig(cfg)
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
		fallback := defaultFanCurveSpeedAt(defaultCurve, i, cfg.FanCurve[i].Temperature)
		next, changed := normalizeFanSpeedSettingForUnit(cfg.FanCurve[i].RPM, fallback, unit)
		cfg.FanCurve[i].RPM = next
		speedSanitized = speedSanitized || changed
	}
	for i := range cfg.FanCurveProfiles {
		for j := range cfg.FanCurveProfiles[i].Curve {
			fallback := defaultFanCurveSpeedAt(defaultCurve, j, cfg.FanCurveProfiles[i].Curve[j].Temperature)
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
			return clampSpeedToUnit(fallback, unit), true
		}
		return speed, false
	}
	if speed < types.FanSpeedMinPercent || speed > types.FanSpeedMaxPercent {
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
	curveLen := len(cfg.FanCurve)
	cfg.SmartControl.LearnedOffsets = make([]int, curveLen)
	cfg.SmartControl.LearnedOffsetsHeat = make([]int, curveLen)
	cfg.SmartControl.LearnedOffsetsCool = make([]int, curveLen)
	cfg.SmartControl.LearnedRateHeat = make([]int, 7)
	cfg.SmartControl.LearnedRateCool = make([]int, 7)
	cfg.SmartControl.LearnedOffsetsByProfile = nil
}

func defaultFanCurveSpeedAt(defaultCurve []types.FanCurvePoint, index int, temperature int) int {
	if index >= 0 && index < len(defaultCurve) {
		return defaultCurve[index].RPM
	}
	if len(defaultCurve) == 0 {
		return 45
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

	m.logDebug("尝试保存配置到便携目录: %s", installConfigPath)

	if err := os.MkdirAll(installConfigDir, 0755); err != nil {
		m.logError("创建便携配置目录失败: %v", err)
	} else {
		data, err := json.MarshalIndent(m.config, "", "  ")
		if err != nil {
			m.logError("序列化配置失败: %v", err)
		} else {
			if err := os.WriteFile(installConfigPath, data, 0644); err != nil {
				m.logError("保存配置到便携目录失败: %v", err)
			} else {
				m.config.ConfigPath = installConfigPath
				m.logInfo("配置保存到便携目录成功: %s", installConfigPath)
				return nil
			}
		}
	}

	defaultConfigDir := m.GetDefaultConfigDir()
	defaultConfigPath := filepath.Join(defaultConfigDir, "config.json")

	m.logInfo("保存到便携目录失败，尝试保存到用户目录: %s", defaultConfigPath)

	if err := os.MkdirAll(defaultConfigDir, 0755); err != nil {
		m.logError("创建用户配置目录失败: %v", err)
		return err
	}

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		m.logError("序列化配置失败: %v", err)
		return err
	}

	if err := os.WriteFile(defaultConfigPath, data, 0644); err != nil {
		m.logError("保存配置到用户目录失败: %v", err)
		return err
	}

	m.config.ConfigPath = defaultConfigPath
	m.logInfo("配置保存到用户目录成功: %s", defaultConfigPath)
	return nil
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
	return m.config
}

func (m *Manager) GetWithRevision() (types.AppConfig, uint64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config, m.revision
}

// Set 设置配置（线程安全）
func (m *Manager) Set(config types.AppConfig) {
	m.mu.Lock()
	m.config = config
	m.bumpRevisionLocked()
	m.mu.Unlock()
}

// Update 更新配置并保存（线程安全，原子操作）
func (m *Manager) Update(config types.AppConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	m.bumpRevisionLocked()
	return m.saveLocked()
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
