package coreapp

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/types"
	"github.com/TIANLI0/THRM/internal/version"
)

const (
	diagnosticsMaxLogFiles      = 10
	diagnosticsMaxLogFileBytes  = 5 * 1024 * 1024
	diagnosticsMaxTotalLogBytes = 24 * 1024 * 1024
)

type diagnosticsManifest struct {
	AppName        string `json:"appName"`
	Version        string `json:"version"`
	GeneratedAt    string `json:"generatedAt"`
	Protocol       string `json:"protocol"`
	OS             string `json:"os"`
	Arch           string `json:"arch"`
	InstallDir     string `json:"installDir,omitempty"`
	ConfigPath     string `json:"configPath,omitempty"`
	LogDir         string `json:"logDir,omitempty"`
	ConfigLocation string `json:"configLocation,omitempty"`
}

type diagnosticsConfigSummary struct {
	ConfigPath                        string                         `json:"configPath,omitempty"`
	DeviceTransport                   string                         `json:"deviceTransport"`
	ActiveDeviceProfileID             string                         `json:"activeDeviceProfileId,omitempty"`
	ActiveDeviceProfileIDsByTransport map[string]string              `json:"activeDeviceProfileIdsByTransport,omitempty"`
	FanControlDeviceIP                string                         `json:"fanControlDeviceIp,omitempty"`
	WiFiCompatibilityEnabled          bool                           `json:"wifiCompatibilityEnabled"`
	WiFiDynamicIPCompatibilityEnabled bool                           `json:"wifiDynamicIpCompatibilityEnabled"`
	WiFiSmartStartStopEnabled         bool                           `json:"wifiSmartStartStopEnabled"`
	WiFiSmartStartStopStandbySpeed    int                            `json:"wifiSmartStartStopStandbySpeed"`
	SerialCompatibilityEnabled        bool                           `json:"serialCompatibilityEnabled"`
	AutoControl                       bool                           `json:"autoControl"`
	CustomSpeedEnabled                bool                           `json:"customSpeedEnabled"`
	CustomSpeedRPM                    int                            `json:"customSpeedRPM"`
	TempUpdateRate                    int                            `json:"tempUpdateRate"`
	TempSampleCount                   int                            `json:"tempSampleCount"`
	TemperatureSelection              types.TemperatureSelection     `json:"temperatureSelection"`
	SmartControl                      diagnosticsSmartControlSummary `json:"smartControl"`
	FanCurvePointCount                int                            `json:"fanCurvePointCount"`
	FanCurveProfileCount              int                            `json:"fanCurveProfileCount"`
	FanCurveProfiles                  []diagnosticsFanCurveProfile   `json:"fanCurveProfiles,omitempty"`
	DeviceProfileCount                int                            `json:"deviceProfileCount"`
}

type diagnosticsSmartControlSummary struct {
	Enabled                           bool   `json:"enabled"`
	Learning                          bool   `json:"learning"`
	LearningBias                      string `json:"learningBias,omitempty"`
	FilterTransientSpike              bool   `json:"filterTransientSpike"`
	TemperatureRisePrediction         bool   `json:"temperatureRisePrediction"`
	TemperatureRisePredictionMaxBoost int    `json:"temperatureRisePredictionMaxBoost"`
	TargetTemp                        int    `json:"targetTemp"`
	Aggressiveness                    int    `json:"aggressiveness"`
	Hysteresis                        int    `json:"hysteresis"`
	MinRPMChange                      int    `json:"minRpmChange"`
	RampUpLimit                       int    `json:"rampUpLimit"`
	RampDownLimit                     int    `json:"rampDownLimit"`
	LearnRate                         int    `json:"learnRate"`
	LearnWindow                       int    `json:"learnWindow"`
	LearnDelay                        int    `json:"learnDelay"`
	LearnedOffsetsCount               int    `json:"learnedOffsetsCount"`
	LearnedOffsetsByProfileCount      int    `json:"learnedOffsetsByProfileCount,omitempty"`
}

type diagnosticsFanCurveProfile struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PointCount int    `json:"pointCount"`
}

type diagnosticsDeviceProfileSummary struct {
	ID              string                         `json:"id"`
	DisplayName     string                         `json:"displayName"`
	Vendor          string                         `json:"vendor,omitempty"`
	Model           string                         `json:"model,omitempty"`
	BuiltIn         bool                           `json:"builtIn,omitempty"`
	Transport       string                         `json:"transport"`
	SpeedUnit       string                         `json:"speedUnit"`
	SpeedRange      types.DeviceSpeedRange         `json:"speedRange"`
	Connection      types.DeviceConnectionSettings `json:"connection,omitempty"`
	Capabilities    types.DeviceCapabilities       `json:"capabilities"`
	CommandCount    int                            `json:"commandCount,omitempty"`
	ParserCount     int                            `json:"parserCount,omitempty"`
	SpeedMapCount   int                            `json:"speedMapCount,omitempty"`
	Active          bool                           `json:"active,omitempty"`
	ActiveTransport bool                           `json:"activeTransport,omitempty"`
}

type diagnosticsRuntimeSnapshot struct {
	IsConnected             bool                  `json:"isConnected"`
	MonitoringTemp          bool                  `json:"monitoringTemp"`
	AutoReconnectSuppressed bool                  `json:"autoReconnectSuppressed"`
	ReconnectInProgress     bool                  `json:"reconnectInProgress"`
	SystemSuspended         bool                  `json:"systemSuspended"`
	LastDeviceMode          string                `json:"lastDeviceMode,omitempty"`
	CurrentTemp             types.TemperatureData `json:"currentTemp"`
	DeviceSettings          *types.DeviceSettings `json:"deviceSettings,omitempty"`
}

type diagnosticsSkippedLog struct {
	Name   string `json:"name"`
	Size   int64  `json:"size,omitempty"`
	Reason string `json:"reason"`
}

// ExportDiagnostics builds a FanControl-specific diagnostics package that can
// be attached to bug reports without requiring users to hunt for separate files.
func (a *CoreApp) ExportDiagnostics() (types.DiagnosticsBundle, error) {
	generatedAt := time.Now()
	installDir := config.GetInstallDir()
	logDir := ""
	if a.logger != nil {
		logDir = a.logger.GetLogDir()
	}

	cfg := a.configManager.Get()
	types.NormalizeDeviceProfileConfig(&cfg)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	skippedLogs := make([]diagnosticsSkippedLog, 0)

	addJSON := func(name string, value any) error {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal %s: %w", name, err)
		}
		return addZipBytes(zw, name, data)
	}

	manifest := diagnosticsManifest{
		AppName:        appmeta.AppName,
		Version:        version.Get(),
		GeneratedAt:    generatedAt.Format(time.RFC3339),
		Protocol:       appmeta.ProtocolVersion,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		InstallDir:     installDir,
		ConfigPath:     cfg.ConfigPath,
		LogDir:         logDir,
		ConfigLocation: configLocationSummary(cfg.ConfigPath, installDir),
	}

	if err := addJSON("manifest.json", manifest); err != nil {
		return types.DiagnosticsBundle{}, err
	}
	if err := addJSON("debug-info.json", a.GetDebugInfo()); err != nil {
		return types.DiagnosticsBundle{}, err
	}
	if err := addJSON("bridge-status.json", bridgeStatus(a)); err != nil {
		return types.DiagnosticsBundle{}, err
	}
	if err := addJSON("config-summary.json", buildDiagnosticsConfigSummary(cfg)); err != nil {
		return types.DiagnosticsBundle{}, err
	}
	if err := addJSON("device-profiles-summary.json", buildDiagnosticsDeviceProfileSummary(cfg)); err != nil {
		return types.DiagnosticsBundle{}, err
	}
	if err := addJSON("runtime-snapshot.json", a.diagnosticsRuntimeSnapshot()); err != nil {
		return types.DiagnosticsBundle{}, err
	}
	if a.deviceManager != nil {
		if err := addJSON("device-debug-frames.json", a.deviceManager.GetDebugFrames()); err != nil {
			return types.DiagnosticsBundle{}, err
		}
	}

	if err := addRecentLogs(zw, logDir, &skippedLogs); err != nil {
		skippedLogs = append(skippedLogs, diagnosticsSkippedLog{Name: logDir, Reason: err.Error()})
	}
	if err := addJSON("logs/skipped-log-files.json", skippedLogs); err != nil {
		return types.DiagnosticsBundle{}, err
	}

	if err := zw.Close(); err != nil {
		return types.DiagnosticsBundle{}, err
	}

	fileName := fmt.Sprintf("FanControl-diagnostics-%s.zip", generatedAt.Format("20060102-150405"))
	return types.DiagnosticsBundle{
		FileName:   fileName,
		DataBase64: base64.StdEncoding.EncodeToString(buf.Bytes()),
		SizeBytes:  int64(buf.Len()),
	}, nil
}

func bridgeStatus(a *CoreApp) map[string]any {
	if a.bridgeManager == nil {
		return map[string]any{"available": false}
	}
	status := a.bridgeManager.GetStatus()
	status["available"] = true
	return status
}

func configLocationSummary(configPath, installDir string) string {
	if configPath == "" {
		return ""
	}
	rel, err := filepath.Rel(installDir, configPath)
	if err == nil && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		return "portable"
	}
	return "user"
}

func buildDiagnosticsConfigSummary(cfg types.AppConfig) diagnosticsConfigSummary {
	profiles := make([]diagnosticsFanCurveProfile, 0, len(cfg.FanCurveProfiles))
	for _, profile := range cfg.FanCurveProfiles {
		profiles = append(profiles, diagnosticsFanCurveProfile{
			ID:         profile.ID,
			Name:       profile.Name,
			PointCount: len(profile.Curve),
		})
	}

	return diagnosticsConfigSummary{
		ConfigPath:                        cfg.ConfigPath,
		DeviceTransport:                   cfg.DeviceTransport,
		ActiveDeviceProfileID:             cfg.ActiveDeviceProfileID,
		ActiveDeviceProfileIDsByTransport: cloneStringMap(cfg.ActiveDeviceProfileIDsByTransport),
		FanControlDeviceIP:                cfg.FanControlDeviceIp,
		WiFiCompatibilityEnabled:          cfg.WiFiCompatibilityEnabled,
		WiFiDynamicIPCompatibilityEnabled: cfg.WiFiDynamicIPCompatibilityEnabled,
		WiFiSmartStartStopEnabled:         cfg.WiFiSmartStartStopEnabled,
		WiFiSmartStartStopStandbySpeed:    cfg.WiFiSmartStartStopStandbySpeed,
		SerialCompatibilityEnabled:        cfg.SerialCompatibilityEnabled,
		AutoControl:                       cfg.AutoControl,
		CustomSpeedEnabled:                cfg.CustomSpeedEnabled,
		CustomSpeedRPM:                    cfg.CustomSpeedRPM,
		TempUpdateRate:                    cfg.TempUpdateRate,
		TempSampleCount:                   cfg.TempSampleCount,
		TemperatureSelection: types.TemperatureSelection{
			TempSource:            cfg.TempSource,
			GpuDevice:             cfg.GpuDevice,
			CpuSensor:             cfg.CpuSensor,
			GpuSensor:             cfg.GpuSensor,
			CpuPowerSensor:        cfg.CpuPowerSensor,
			GpuPowerSensor:        cfg.GpuPowerSensor,
			GpuReadMode:           cfg.GpuReadMode,
			GpuLowPowerProtection: cfg.GpuLowPowerProtection,
		},
		SmartControl: diagnosticsSmartControlSummary{
			Enabled:                           cfg.SmartControl.Enabled,
			Learning:                          cfg.SmartControl.Learning,
			LearningBias:                      cfg.SmartControl.LearningBias,
			FilterTransientSpike:              cfg.SmartControl.FilterTransientSpike,
			TemperatureRisePrediction:         cfg.SmartControl.TemperatureRisePrediction,
			TemperatureRisePredictionMaxBoost: cfg.SmartControl.TemperatureRisePredictionMaxBoost,
			TargetTemp:                        cfg.SmartControl.TargetTemp,
			Aggressiveness:                    cfg.SmartControl.Aggressiveness,
			Hysteresis:                        cfg.SmartControl.Hysteresis,
			MinRPMChange:                      cfg.SmartControl.MinRPMChange,
			RampUpLimit:                       cfg.SmartControl.RampUpLimit,
			RampDownLimit:                     cfg.SmartControl.RampDownLimit,
			LearnRate:                         cfg.SmartControl.LearnRate,
			LearnWindow:                       cfg.SmartControl.LearnWindow,
			LearnDelay:                        cfg.SmartControl.LearnDelay,
			LearnedOffsetsCount:               len(cfg.SmartControl.LearnedOffsets),
			LearnedOffsetsByProfileCount:      len(cfg.SmartControl.LearnedOffsetsByProfile),
		},
		FanCurvePointCount:   len(cfg.FanCurve),
		FanCurveProfileCount: len(cfg.FanCurveProfiles),
		FanCurveProfiles:     profiles,
		DeviceProfileCount:   len(cfg.DeviceProfiles),
	}
}

func buildDiagnosticsDeviceProfileSummary(cfg types.AppConfig) []diagnosticsDeviceProfileSummary {
	activeByTransport := cloneStringMap(cfg.ActiveDeviceProfileIDsByTransport)
	out := make([]diagnosticsDeviceProfileSummary, 0, len(cfg.DeviceProfiles))
	for _, profile := range cfg.DeviceProfiles {
		profile = types.NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp)
		out = append(out, diagnosticsDeviceProfileSummary{
			ID:              profile.ID,
			DisplayName:     profile.DisplayName,
			Vendor:          profile.Vendor,
			Model:           profile.Model,
			BuiltIn:         profile.BuiltIn,
			Transport:       profile.Transport,
			SpeedUnit:       profile.SpeedUnit,
			SpeedRange:      profile.SpeedRange,
			Connection:      profile.Connection,
			Capabilities:    profile.Capabilities,
			CommandCount:    len(profile.Commands),
			ParserCount:     len(profile.ResponseParsers),
			SpeedMapCount:   len(profile.SpeedMap),
			Active:          profile.ID == cfg.ActiveDeviceProfileID,
			ActiveTransport: activeByTransport[types.NormalizeDeviceTransport(profile.Transport)] == profile.ID,
		})
	}
	return out
}

func (a *CoreApp) diagnosticsRuntimeSnapshot() diagnosticsRuntimeSnapshot {
	a.mutex.RLock()
	currentTemp := a.currentTemp
	deviceSettings := a.deviceSettings
	isConnected := a.isConnected
	lastDeviceMode := a.lastDeviceMode
	a.mutex.RUnlock()

	return diagnosticsRuntimeSnapshot{
		IsConnected:             isConnected,
		MonitoringTemp:          a.monitoringTemp.Load(),
		AutoReconnectSuppressed: a.autoReconnectSuppressed.Load(),
		ReconnectInProgress:     a.reconnectInProgress.Load(),
		SystemSuspended:         a.systemSuspended.Load(),
		LastDeviceMode:          lastDeviceMode,
		CurrentTemp:             currentTemp,
		DeviceSettings:          deviceSettings,
	}
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func addRecentLogs(zw *zip.Writer, logDir string, skipped *[]diagnosticsSkippedLog) error {
	if strings.TrimSpace(logDir) == "" {
		return nil
	}
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return err
	}

	type logFile struct {
		name string
		path string
		info os.FileInfo
	}
	files := make([]logFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".log") && !strings.HasSuffix(name, ".log.gz") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, logFile{
			name: name,
			path: filepath.Join(logDir, name),
			info: info,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].info.ModTime().After(files[j].info.ModTime())
	})

	var totalBytes int64
	for i, file := range files {
		if i >= diagnosticsMaxLogFiles {
			*skipped = append(*skipped, diagnosticsSkippedLog{Name: file.name, Size: file.info.Size(), Reason: "older than selected log window"})
			continue
		}
		if totalBytes >= diagnosticsMaxTotalLogBytes {
			*skipped = append(*skipped, diagnosticsSkippedLog{Name: file.name, Size: file.info.Size(), Reason: "diagnostics log size budget reached"})
			continue
		}

		size := file.info.Size()
		if size <= diagnosticsMaxLogFileBytes {
			data, err := os.ReadFile(file.path)
			if err != nil {
				*skipped = append(*skipped, diagnosticsSkippedLog{Name: file.name, Size: size, Reason: err.Error()})
				continue
			}
			totalBytes += int64(len(data))
			if err := addZipBytes(zw, "logs/"+file.name, data); err != nil {
				return err
			}
			continue
		}

		if strings.HasSuffix(file.name, ".gz") {
			*skipped = append(*skipped, diagnosticsSkippedLog{Name: file.name, Size: size, Reason: "compressed log is larger than per-file budget"})
			continue
		}
		data, err := readFileTail(file.path, diagnosticsMaxLogFileBytes)
		if err != nil {
			*skipped = append(*skipped, diagnosticsSkippedLog{Name: file.name, Size: size, Reason: err.Error()})
			continue
		}
		totalBytes += int64(len(data))
		if err := addZipBytes(zw, "logs/"+file.name+".tail.log", data); err != nil {
			return err
		}
	}
	return nil
}

func readFileTail(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	offset := info.Size() - maxBytes
	if offset < 0 {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(file)
}

func addZipBytes(zw *zip.Writer, name string, data []byte) error {
	header := &zip.FileHeader{
		Name:   filepath.ToSlash(name),
		Method: zip.Deflate,
	}
	header.SetModTime(time.Now())
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}
