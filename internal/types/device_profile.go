package types

import (
	"strings"

	"github.com/TIANLI0/THRM/internal/appmeta"
)

const (
	DefaultWiFiPercentProfileID         = "builtin.wifi.percent"
	DefaultWiFiPercentTemplateProfileID = "template.wifi.percent"
	LegacyRPMProfileID                  = "builtin.legacy.rpm"
)

type DeviceSpeedRange struct {
	Min       int `json:"min"`
	Max       int `json:"max"`
	Step      int `json:"step,omitempty"`
	TickScale int `json:"tickScale,omitempty"`
}

type DeviceCapabilities struct {
	ProfileID                      string           `json:"profileId,omitempty"`
	DisplayName                    string           `json:"displayName,omitempty"`
	Transport                      string           `json:"transport"`
	SpeedUnit                      string           `json:"speedUnit"`
	SpeedRange                     DeviceSpeedRange `json:"speedRange"`
	SupportsReadState              bool             `json:"supportsReadState"`
	SupportsSetSpeed               bool             `json:"supportsSetSpeed"`
	SupportsManualGears            bool             `json:"supportsManualGears"`
	SupportsCustomSpeed            bool             `json:"supportsCustomSpeed"`
	SupportsDebugFrames            bool             `json:"supportsDebugFrames"`
	SupportsRawCommands            bool             `json:"supportsRawCommands"`
	SupportsGearLight              bool             `json:"supportsGearLight"`
	SupportsLighting               bool             `json:"supportsLighting"`
	SupportsBrightness             bool             `json:"supportsBrightness"`
	SupportsScreen                 bool             `json:"supportsScreen"`
	SupportsPowerOnStart           bool             `json:"supportsPowerOnStart"`
	SupportsSmartStartStop         bool             `json:"supportsSmartStartStop"`
	SupportsSoftwareSmartStartStop bool             `json:"supportsSoftwareSmartStartStop"`
}

type DeviceProfilesPayload struct {
	Profiles             []DeviceProfile   `json:"profiles"`
	ActiveID             string            `json:"activeId"`
	ActiveIDsByTransport map[string]string `json:"activeIdsByTransport,omitempty"`
}

type DeviceProfileTestParams struct {
	Profile    DeviceProfile `json:"profile"`
	Action     string        `json:"action"`
	SpeedValue float64       `json:"speedValue,omitempty"`
	TimeoutMs  int           `json:"timeoutMs,omitempty"`
}

type DeviceProfileTestResult struct {
	Action              string   `json:"action"`
	Transport           string   `json:"transport"`
	SpeedUnit           string   `json:"speedUnit"`
	ProfileID           string   `json:"profileId,omitempty"`
	DisplayName         string   `json:"displayName,omitempty"`
	Connected           bool     `json:"connected"`
	DurationMs          int64    `json:"durationMs"`
	Message             string   `json:"message,omitempty"`
	RequestedSpeedValue float64  `json:"requestedSpeedValue,omitempty"`
	FanData             *FanData `json:"fanData,omitempty"`
}

type SerialPortInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Source      string `json:"source,omitempty"`
}

type BLEManufacturerData struct {
	CompanyID uint16 `json:"companyId"`
	DataHex   string `json:"dataHex,omitempty"`
}

type BLEDeviceInfo struct {
	Address                       string                `json:"address"`
	Name                          string                `json:"name,omitempty"`
	RSSI                          int                   `json:"rssi"`
	ServiceUUIDs                  []string              `json:"serviceUuids,omitempty"`
	ManufacturerData              []BLEManufacturerData `json:"manufacturerData,omitempty"`
	WriteCharacteristicUUIDs      []string              `json:"writeCharacteristicUuids,omitempty"`
	NotifyCharacteristicUUIDs     []string              `json:"notifyCharacteristicUuids,omitempty"`
	Matched                       bool                  `json:"matched"`
	MatchScore                    int                   `json:"matchScore,omitempty"`
	MatchReasons                  []string              `json:"matchReasons,omitempty"`
	MatchedProfileID              string                `json:"matchedProfileId,omitempty"`
	MatchedProfileDisplayName     string                `json:"matchedProfileDisplayName,omitempty"`
	SuggestedNameFilter           string                `json:"suggestedNameFilter,omitempty"`
	SuggestedServiceUUID          string                `json:"suggestedServiceUuid,omitempty"`
	SuggestedWriteCharacteristic  string                `json:"suggestedWriteCharacteristic,omitempty"`
	SuggestedNotifyCharacteristic string                `json:"suggestedNotifyCharacteristic,omitempty"`
}

type BLEScanParams struct {
	TimeoutMs                int             `json:"timeoutMs,omitempty"`
	NameFilter               string          `json:"nameFilter,omitempty"`
	ServiceUUID              string          `json:"serviceUuid,omitempty"`
	WriteCharacteristicUUID  string          `json:"writeCharacteristicUuid,omitempty"`
	NotifyCharacteristicUUID string          `json:"notifyCharacteristicUuid,omitempty"`
	OnlyMatched              bool            `json:"onlyMatched,omitempty"`
	Profiles                 []DeviceProfile `json:"profiles,omitempty"`
}

type BLEGATTProbeParams struct {
	TimeoutMs   int           `json:"timeoutMs,omitempty"`
	Address     string        `json:"address,omitempty"`
	ServiceUUID string        `json:"serviceUuid,omitempty"`
	Profile     DeviceProfile `json:"profile"`
}

type BLEGATTCharacteristicInfo struct {
	UUID                    string   `json:"uuid"`
	Properties              []string `json:"properties,omitempty"`
	CanRead                 bool     `json:"canRead,omitempty"`
	CanWrite                bool     `json:"canWrite,omitempty"`
	CanWriteWithoutResponse bool     `json:"canWriteWithoutResponse,omitempty"`
	CanNotify               bool     `json:"canNotify,omitempty"`
	CanIndicate             bool     `json:"canIndicate,omitempty"`
	MTU                     int      `json:"mtu,omitempty"`
}

type BLEGATTServiceInfo struct {
	UUID            string                      `json:"uuid"`
	Characteristics []BLEGATTCharacteristicInfo `json:"characteristics,omitempty"`
	Error           string                      `json:"error,omitempty"`
}

type BLEGATTProbeResult struct {
	Address                       string               `json:"address,omitempty"`
	Name                          string               `json:"name,omitempty"`
	Services                      []BLEGATTServiceInfo `json:"services,omitempty"`
	SuggestedServiceUUID          string               `json:"suggestedServiceUuid,omitempty"`
	SuggestedWriteCharacteristic  string               `json:"suggestedWriteCharacteristic,omitempty"`
	SuggestedNotifyCharacteristic string               `json:"suggestedNotifyCharacteristic,omitempty"`
}

type DeviceConnectionSettings struct {
	Endpoint                string `json:"endpoint,omitempty"`
	StateEndpoint           string `json:"stateEndpoint,omitempty"`
	SpeedEndpoint           string `json:"speedEndpoint,omitempty"`
	HTTPMethod              string `json:"httpMethod,omitempty"`
	RequestTimeoutMs        int    `json:"requestTimeoutMs,omitempty"`
	MinSendIntervalMs       int    `json:"minSendIntervalMs,omitempty"`
	MaxRetries              int    `json:"maxRetries,omitempty"`
	RetryBackoffMs          int    `json:"retryBackoffMs,omitempty"`
	BLENameFilter           string `json:"bleNameFilter,omitempty"`
	BLEServiceUUID          string `json:"bleServiceUuid,omitempty"`
	BLEWriteCharacteristic  string `json:"bleWriteCharacteristic,omitempty"`
	BLENotifyCharacteristic string `json:"bleNotifyCharacteristic,omitempty"`
	BLEWriteWithResponse    bool   `json:"bleWriteWithResponse,omitempty"`
	SerialPort              string `json:"serialPort,omitempty"`
	SerialBaudRate          int    `json:"serialBaudRate,omitempty"`
	SerialDataBits          int    `json:"serialDataBits,omitempty"`
	SerialStopBits          int    `json:"serialStopBits,omitempty"`
	SerialParity            string `json:"serialParity,omitempty"`
	SerialFrameDelimiter    string `json:"serialFrameDelimiter,omitempty"`
}

type DeviceCommandTemplate struct {
	Name        string `json:"name"`
	Command     string `json:"command"`
	Encoding    string `json:"encoding,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
	Description string `json:"description,omitempty"`
}

type DeviceResponseParser struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Expression string `json:"expression,omitempty"`
}

type DeviceSpeedMapPoint struct {
	PercentTicks int `json:"percentTicks"`
	RPM          int `json:"rpm"`
}

type DeviceProfile struct {
	ID              string                   `json:"id"`
	DisplayName     string                   `json:"displayName"`
	Vendor          string                   `json:"vendor,omitempty"`
	Model           string                   `json:"model,omitempty"`
	Notes           string                   `json:"notes,omitempty"`
	BuiltIn         bool                     `json:"builtIn,omitempty"`
	Transport       string                   `json:"transport"`
	SpeedUnit       string                   `json:"speedUnit"`
	SpeedRange      DeviceSpeedRange         `json:"speedRange"`
	Connection      DeviceConnectionSettings `json:"connection,omitempty"`
	Commands        []DeviceCommandTemplate  `json:"commands,omitempty"`
	ResponseParsers []DeviceResponseParser   `json:"responseParsers,omitempty"`
	SpeedMap        []DeviceSpeedMapPoint    `json:"speedMap,omitempty"`
	Capabilities    DeviceCapabilities       `json:"capabilities"`
}

func DefaultPercentSpeedRange() DeviceSpeedRange {
	return DeviceSpeedRange{
		Min:       FanSpeedMinPercent,
		Max:       FanSpeedMaxPercent,
		Step:      1,
		TickScale: PercentSpeedTicksPerPercent,
	}
}

func DefaultRPMSpeedRange() DeviceSpeedRange {
	return DeviceSpeedRange{
		Min:  0,
		Max:  DefaultMaxFanRPM,
		Step: 1,
	}
}

func DefaultWiFiPercentCapabilities() DeviceCapabilities {
	return DeviceCapabilities{
		ProfileID:                      DefaultWiFiPercentProfileID,
		DisplayName:                    appmeta.DeviceTemplateName,
		Transport:                      DeviceTransportWiFi,
		SpeedUnit:                      FanSpeedUnitPercent,
		SpeedRange:                     DefaultPercentSpeedRange(),
		SupportsReadState:              true,
		SupportsSetSpeed:               true,
		SupportsManualGears:            true,
		SupportsCustomSpeed:            true,
		SupportsDebugFrames:            false,
		SupportsRawCommands:            false,
		SupportsGearLight:              false,
		SupportsLighting:               false,
		SupportsBrightness:             false,
		SupportsScreen:                 false,
		SupportsPowerOnStart:           false,
		SupportsSmartStartStop:         false,
		SupportsSoftwareSmartStartStop: true,
	}
}

func LegacyRPMCapabilities() DeviceCapabilities {
	return DeviceCapabilities{
		ProfileID:                      LegacyRPMProfileID,
		DisplayName:                    "RPM 兼容控制模板",
		Transport:                      DeviceTransportHID,
		SpeedUnit:                      FanSpeedUnitRPM,
		SpeedRange:                     DefaultRPMSpeedRange(),
		SupportsReadState:              true,
		SupportsSetSpeed:               true,
		SupportsManualGears:            true,
		SupportsCustomSpeed:            true,
		SupportsDebugFrames:            false,
		SupportsRawCommands:            false,
		SupportsGearLight:              false,
		SupportsLighting:               false,
		SupportsBrightness:             false,
		SupportsScreen:                 false,
		SupportsPowerOnStart:           false,
		SupportsSmartStartStop:         false,
		SupportsSoftwareSmartStartStop: false,
	}
}

func LegacyRPMProfile() DeviceProfile {
	return LegacyRPMProfileForTransport(DeviceTransportHID)
}

func LegacyRPMProfileForTransport(transport string) DeviceProfile {
	caps := LegacyRPMCapabilities()
	transport = NormalizeDeviceTransport(transport)
	if transport != DeviceTransportHID && transport != DeviceTransportBLE {
		transport = DeviceTransportHID
	}
	caps.Transport = transport
	return DeviceProfile{
		ID:           LegacyRPMProfileID,
		DisplayName:  caps.DisplayName,
		Vendor:       "THRM",
		Model:        "HID/BLE RPM",
		Notes:        "保留参考软件风格的 RPM 控制路径，作为旧配置和高级模板使用。",
		BuiltIn:      true,
		Transport:    transport,
		SpeedUnit:    caps.SpeedUnit,
		SpeedRange:   caps.SpeedRange,
		Capabilities: caps,
	}
}

func DefaultWiFiPercentProfile(endpoint string) DeviceProfile {
	caps := DefaultWiFiPercentCapabilities()
	caps.DisplayName = appmeta.DeviceModelName
	return DeviceProfile{
		ID:          DefaultWiFiPercentProfileID,
		DisplayName: appmeta.DeviceModelName,
		Vendor:      "FanControl",
		Model:       appmeta.DeviceModelName,
		BuiltIn:     true,
		Transport:   caps.Transport,
		SpeedUnit:   caps.SpeedUnit,
		SpeedRange:  caps.SpeedRange,
		Connection: DeviceConnectionSettings{
			Endpoint:      endpoint,
			StateEndpoint: "/api/data",
			SpeedEndpoint: "/api/speed",
			HTTPMethod:    "POST",
		},
		Capabilities: caps,
	}
}

func DefaultWiFiPercentTemplateProfile(endpoint string) DeviceProfile {
	caps := DefaultWiFiPercentCapabilities()
	caps.ProfileID = DefaultWiFiPercentTemplateProfileID
	return DeviceProfile{
		ID:          DefaultWiFiPercentTemplateProfileID,
		DisplayName: appmeta.DeviceTemplateName,
		Vendor:      "FanControl",
		Model:       "WiFi percent controller",
		Notes:       "Reusable WiFi percent-control template. Save it as a device before enabling it.",
		BuiltIn:     true,
		Transport:   caps.Transport,
		SpeedUnit:   caps.SpeedUnit,
		SpeedRange:  caps.SpeedRange,
		Connection: DeviceConnectionSettings{
			Endpoint:      endpoint,
			StateEndpoint: "/api/data",
			SpeedEndpoint: "/api/speed",
			HTTPMethod:    "POST",
		},
		Capabilities: caps,
	}
}

func NormalizeDeviceCapabilities(caps DeviceCapabilities) DeviceCapabilities {
	caps.Transport = NormalizeDeviceTransport(caps.Transport)
	caps.SpeedUnit = NormalizeFanSpeedUnit(caps.SpeedUnit)
	if caps.SpeedUnit == FanSpeedUnitRPM {
		if caps.SpeedRange.Max <= 0 {
			caps.SpeedRange = DefaultRPMSpeedRange()
		}
	} else {
		caps.SpeedRange = normalizePercentSpeedRange(caps.SpeedRange)
	}
	return caps
}

func (caps DeviceCapabilities) AllowsGearLight() bool {
	return caps.SupportsGearLight || caps.SupportsLighting
}

func (caps DeviceCapabilities) AllowsBrightness() bool {
	return caps.SupportsBrightness || caps.SupportsLighting
}

func (caps DeviceCapabilities) AllowsLightStrip() bool {
	return caps.SupportsLighting
}

func normalizePercentSpeedRange(speedRange DeviceSpeedRange) DeviceSpeedRange {
	defaultRange := DefaultPercentSpeedRange()
	if speedRange.Max <= speedRange.Min || speedRange.Min < FanSpeedMinPercent || speedRange.Max > FanSpeedMaxPercent {
		return defaultRange
	}
	if speedRange.Step <= 0 {
		speedRange.Step = defaultRange.Step
	}
	if speedRange.TickScale <= 0 {
		speedRange.TickScale = defaultRange.TickScale
	}
	return speedRange
}

func NormalizeDeviceProfile(profile DeviceProfile, fallbackEndpoint string) DeviceProfile {
	profile.ID = strings.TrimSpace(profile.ID)
	profile.DisplayName = strings.TrimSpace(profile.DisplayName)
	profile.Transport = NormalizeDeviceTransport(profile.Transport)
	profile.SpeedUnit = NormalizeFanSpeedUnit(profile.SpeedUnit)

	if profile.ID == "" {
		if profile.SpeedUnit == FanSpeedUnitRPM {
			profile.ID = LegacyRPMProfileID
		} else {
			profile.ID = DefaultWiFiPercentProfileID
		}
	}
	if profile.ID == DefaultWiFiPercentProfileID {
		profile.DisplayName = appmeta.DeviceModelName
		profile.Vendor = "FanControl"
		profile.Model = appmeta.DeviceModelName
	}
	if profile.DisplayName == "" {
		if profile.SpeedUnit == FanSpeedUnitRPM {
			profile.DisplayName = "RPM controller"
		} else {
			profile.DisplayName = "Percent controller"
		}
	}

	if profile.SpeedUnit == FanSpeedUnitRPM {
		if profile.SpeedRange.Max <= profile.SpeedRange.Min {
			profile.SpeedRange = DefaultRPMSpeedRange()
		}
	} else {
		profile.SpeedRange = normalizePercentSpeedRange(profile.SpeedRange)
	}

	if profile.Transport == DeviceTransportWiFi {
		profile.Connection.Endpoint = strings.TrimSpace(profile.Connection.Endpoint)
		if profile.Connection.Endpoint == "" {
			profile.Connection.Endpoint = strings.TrimSpace(fallbackEndpoint)
		}
		if profile.Connection.Endpoint == "" {
			profile.Connection.Endpoint = DefaultFanDeviceIP
		}
		if strings.TrimSpace(profile.Connection.StateEndpoint) == "" {
			profile.Connection.StateEndpoint = "/api/data"
		}
		if strings.TrimSpace(profile.Connection.SpeedEndpoint) == "" {
			profile.Connection.SpeedEndpoint = "/api/speed"
		}
		if strings.TrimSpace(profile.Connection.HTTPMethod) == "" {
			profile.Connection.HTTPMethod = "POST"
		}
	}

	profile.Capabilities.ProfileID = profile.ID
	profile.Capabilities.DisplayName = profile.DisplayName
	profile.Capabilities.Transport = profile.Transport
	profile.Capabilities.SpeedUnit = profile.SpeedUnit
	profile.Capabilities.SpeedRange = profile.SpeedRange
	profile.Capabilities = NormalizeDeviceCapabilities(profile.Capabilities)
	return profile
}

func DeviceProfileSpeedUnit(cfg *AppConfig) string {
	if cfg == nil {
		return FanSpeedUnitPercent
	}
	profile := ActiveDeviceProfile(cfg)
	return NormalizeFanSpeedUnit(profile.SpeedUnit)
}

func ActiveDeviceProfile(cfg *AppConfig) DeviceProfile {
	if cfg == nil {
		return DefaultWiFiPercentProfile(DefaultFanDeviceIP)
	}
	transport := NormalizeDeviceTransport(cfg.DeviceTransport)
	if !IsManualCompatibilityDeviceTransport(transport) {
		if profile, ok := activeCompatibilityProfile(cfg); ok {
			return profile
		}
		return DefaultWiFiPercentProfile(cfg.FanControlDeviceIp)
	}
	if transport != "" {
		if id := activeDeviceProfileIDFromTransportMap(cfg.DeviceProfiles, cfg.ActiveDeviceProfileIDsByTransport, transport); id != "" {
			for _, profile := range cfg.DeviceProfiles {
				if profile.ID == id {
					return NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp)
				}
			}
		}
		activeID := strings.TrimSpace(cfg.ActiveDeviceProfileID)
		if activeID != "" {
			for _, profile := range cfg.DeviceProfiles {
				if profile.ID == activeID && NormalizeDeviceTransport(profile.Transport) == transport {
					return NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp)
				}
			}
		}
		if id := firstDeviceProfileIDForTransport(cfg.DeviceProfiles, transport); id != "" {
			for _, profile := range cfg.DeviceProfiles {
				if profile.ID == id {
					return NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp)
				}
			}
		}
		switch transport {
		case DeviceTransportWiFi:
			return DefaultWiFiPercentProfile(cfg.FanControlDeviceIp)
		}
	}
	activeID := strings.TrimSpace(cfg.ActiveDeviceProfileID)
	for _, profile := range cfg.DeviceProfiles {
		if profile.ID == activeID && profile.ID != "" {
			return NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp)
		}
	}
	if len(cfg.DeviceProfiles) > 0 {
		return NormalizeDeviceProfile(cfg.DeviceProfiles[0], cfg.FanControlDeviceIp)
	}
	return DefaultWiFiPercentProfile(cfg.FanControlDeviceIp)
}

func ActiveDeviceProfileIDForTransport(cfg *AppConfig, transport string) string {
	if cfg == nil {
		return ""
	}
	transport = NormalizeDeviceTransport(transport)
	if !IsManualCompatibilityDeviceTransport(transport) {
		return ""
	}
	if id := activeDeviceProfileIDFromTransportMap(cfg.DeviceProfiles, cfg.ActiveDeviceProfileIDsByTransport, transport); id != "" {
		return id
	}
	active := ActiveDeviceProfile(cfg)
	if NormalizeDeviceTransport(active.Transport) == transport {
		return active.ID
	}
	return firstDeviceProfileIDForTransport(cfg.DeviceProfiles, transport)
}

func deviceProfileByID(profiles []DeviceProfile, id string) (DeviceProfile, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return DeviceProfile{}, false
	}
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return DeviceProfile{}, false
}

func activeCompatibilityProfile(cfg *AppConfig) (DeviceProfile, bool) {
	if cfg == nil {
		return DeviceProfile{}, false
	}
	if profile, ok := deviceProfileByID(cfg.DeviceProfiles, cfg.ActiveDeviceProfileID); ok &&
		IsManualCompatibilityDeviceTransport(profile.Transport) {
		return NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp), true
	}
	for _, transport := range []string{DeviceTransportWiFi, DeviceTransportSerial} {
		id := activeDeviceProfileIDFromTransportMap(cfg.DeviceProfiles, cfg.ActiveDeviceProfileIDsByTransport, transport)
		if profile, ok := deviceProfileByID(cfg.DeviceProfiles, id); ok {
			return NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp), true
		}
	}
	for _, transport := range []string{DeviceTransportWiFi, DeviceTransportSerial} {
		id := firstDeviceProfileIDForTransport(cfg.DeviceProfiles, transport)
		if profile, ok := deviceProfileByID(cfg.DeviceProfiles, id); ok {
			return NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp), true
		}
	}
	return DefaultWiFiPercentProfile(cfg.FanControlDeviceIp), true
}

func firstDeviceProfileIDForTransport(profiles []DeviceProfile, transport string) string {
	transport = NormalizeDeviceTransport(transport)
	if !IsManualCompatibilityDeviceTransport(transport) {
		return ""
	}
	for _, profile := range profiles {
		if profile.ID != "" && NormalizeDeviceTransport(profile.Transport) == transport {
			return profile.ID
		}
	}
	return ""
}

func activeDeviceProfileIDFromTransportMap(profiles []DeviceProfile, activeIDs map[string]string, transport string) string {
	transport = NormalizeDeviceTransport(transport)
	if !IsManualCompatibilityDeviceTransport(transport) {
		return ""
	}
	if len(activeIDs) == 0 {
		return ""
	}
	id := strings.TrimSpace(activeIDs[transport])
	if id == "" {
		return ""
	}
	for _, profile := range profiles {
		if profile.ID == id && NormalizeDeviceTransport(profile.Transport) == transport {
			return id
		}
	}
	return ""
}

func normalizeActiveDeviceProfileIDsByTransport(profiles []DeviceProfile, activeIDs map[string]string) (map[string]string, bool) {
	changed := false
	out := make(map[string]string, len(activeIDs))
	for rawTransport, rawID := range activeIDs {
		transport := NormalizeDeviceTransport(strings.TrimSpace(rawTransport))
		id := strings.TrimSpace(rawID)
		if transport != strings.TrimSpace(rawTransport) || id != rawID {
			changed = true
		}
		if !IsManualCompatibilityDeviceTransport(transport) {
			changed = true
			continue
		}
		if id == "" {
			changed = true
			continue
		}
		if activeDeviceProfileIDFromTransportMap(profiles, map[string]string{transport: id}, transport) == "" {
			changed = true
			continue
		}
		out[transport] = id
	}
	if len(activeIDs) == 0 {
		return out, false
	}
	if len(out) != len(activeIDs) {
		changed = true
	}
	return out, changed
}

func setActiveDeviceProfileIDForTransport(cfg *AppConfig, profile DeviceProfile) bool {
	if cfg == nil || strings.TrimSpace(profile.ID) == "" {
		return false
	}
	transport := NormalizeDeviceTransport(profile.Transport)
	if !IsManualCompatibilityDeviceTransport(transport) {
		return false
	}
	if cfg.ActiveDeviceProfileIDsByTransport == nil {
		cfg.ActiveDeviceProfileIDsByTransport = map[string]string{}
	}
	if cfg.ActiveDeviceProfileIDsByTransport[transport] == profile.ID {
		return false
	}
	cfg.ActiveDeviceProfileIDsByTransport[transport] = profile.ID
	return true
}

func upsertBuiltInProfileForTransport(cfg *AppConfig, transport string) (string, bool) {
	transport = NormalizeDeviceTransport(transport)
	profile, ok := builtInDeviceProfileForTransport(cfg.FanControlDeviceIp, transport)
	if !ok {
		if transport == DeviceTransportBLE || transport == DeviceTransportHID {
			profile = LegacyRPMProfileForTransport(transport)
		} else {
			return "", false
		}
	}

	for i := range cfg.DeviceProfiles {
		if cfg.DeviceProfiles[i].ID == profile.ID {
			cfg.DeviceProfiles[i] = profile
			return profile.ID, true
		}
	}
	cfg.DeviceProfiles = append(cfg.DeviceProfiles, profile)
	return profile.ID, true
}

func NormalizeDeviceProfileConfig(cfg *AppConfig) bool {
	if cfg == nil {
		return false
	}
	changed := false
	cfg.DeviceTransport = NormalizeDeviceTransport(cfg.DeviceTransport)
	requestedTransport := cfg.DeviceTransport
	cfg.FanControlDeviceIp = strings.TrimSpace(cfg.FanControlDeviceIp)
	if cfg.FanControlDeviceIp == "" {
		cfg.FanControlDeviceIp = DefaultFanDeviceIP
		changed = true
	}

	if len(cfg.DeviceProfiles) == 0 {
		profile, ok := builtInDeviceProfileForTransport(cfg.FanControlDeviceIp, cfg.DeviceTransport)
		if !ok || !IsManualCompatibilityDeviceTransport(profile.Transport) {
			profile = DefaultWiFiPercentProfile(cfg.FanControlDeviceIp)
		}
		cfg.DeviceProfiles = []DeviceProfile{profile}
		cfg.ActiveDeviceProfileID = profile.ID
		cfg.DeviceTransport = profile.Transport
		ensureBuiltInDeviceProfiles(cfg)
		setActiveDeviceProfileIDForTransport(cfg, cfg.DeviceProfiles[0])
		return true
	}

	foundActive := false
	seen := make(map[string]bool, len(cfg.DeviceProfiles))
	normalized := make([]DeviceProfile, 0, len(cfg.DeviceProfiles))
	for _, profile := range cfg.DeviceProfiles {
		profile = NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp)
		if profile.ID == LegacyRPMProfileID {
			changed = true
			continue
		}
		if seen[profile.ID] {
			changed = true
			continue
		}
		seen[profile.ID] = true
		if profile.ID == cfg.ActiveDeviceProfileID {
			foundActive = true
		}
		normalized = append(normalized, profile)
	}
	if len(normalized) == 0 {
		normalized = []DeviceProfile{DefaultWiFiPercentProfile(cfg.FanControlDeviceIp)}
		cfg.ActiveDeviceProfileID = normalized[0].ID
		changed = true
	} else if !foundActive {
		cfg.ActiveDeviceProfileID = normalized[0].ID
		changed = true
	}
	if len(normalized) != len(cfg.DeviceProfiles) {
		changed = true
	}
	cfg.DeviceProfiles = normalized
	if ensureBuiltInDeviceProfiles(cfg) {
		changed = true
	}
	activeIDs, activeIDsChanged := normalizeActiveDeviceProfileIDsByTransport(cfg.DeviceProfiles, cfg.ActiveDeviceProfileIDsByTransport)
	cfg.ActiveDeviceProfileIDsByTransport = activeIDs
	if activeIDsChanged {
		changed = true
	}

	if !IsManualCompatibilityDeviceTransport(requestedTransport) {
		active, _ := activeCompatibilityProfile(cfg)
		if cfg.ActiveDeviceProfileID != active.ID {
			cfg.ActiveDeviceProfileID = active.ID
			changed = true
		}
		if cfg.DeviceTransport != active.Transport {
			cfg.DeviceTransport = active.Transport
			changed = true
		}
		if setActiveDeviceProfileIDForTransport(cfg, active) {
			changed = true
		}
		if active.Transport == DeviceTransportWiFi && active.Connection.Endpoint != "" && cfg.FanControlDeviceIp != active.Connection.Endpoint {
			cfg.FanControlDeviceIp = active.Connection.Endpoint
			changed = true
		}
		return changed
	}

	active := ActiveDeviceProfile(cfg)
	activeExists := false
	for _, profile := range cfg.DeviceProfiles {
		if profile.ID == active.ID && profile.ID != "" {
			activeExists = true
			break
		}
	}
	if !activeExists {
		if builtInID, added := upsertBuiltInProfileForTransport(cfg, active.Transport); added {
			cfg.ActiveDeviceProfileID = builtInID
			changed = true
			active = ActiveDeviceProfile(cfg)
		}
	}
	if active.ID != "" && cfg.ActiveDeviceProfileID != active.ID {
		cfg.ActiveDeviceProfileID = active.ID
		changed = true
	}
	if setActiveDeviceProfileIDForTransport(cfg, active) {
		changed = true
	}

	if NormalizeDeviceTransport(active.Transport) != requestedTransport {
		profileIDForRequestedTransport := activeDeviceProfileIDFromTransportMap(cfg.DeviceProfiles, cfg.ActiveDeviceProfileIDsByTransport, requestedTransport)
		if profileIDForRequestedTransport == "" {
			profileIDForRequestedTransport = firstDeviceProfileIDForTransport(cfg.DeviceProfiles, requestedTransport)
		}
		if profileIDForRequestedTransport == "" {
			if builtInID, added := upsertBuiltInProfileForTransport(cfg, requestedTransport); added {
				profileIDForRequestedTransport = builtInID
				changed = true
			}
		}
		if profileIDForRequestedTransport != "" && cfg.ActiveDeviceProfileID != profileIDForRequestedTransport {
			cfg.ActiveDeviceProfileID = profileIDForRequestedTransport
			changed = true
		}
		active = ActiveDeviceProfile(cfg)
	}
	if setActiveDeviceProfileIDForTransport(cfg, active) {
		changed = true
	}

	if cfg.DeviceTransport != active.Transport {
		cfg.DeviceTransport = active.Transport
		changed = true
	}
	if active.Transport == DeviceTransportWiFi && active.Connection.Endpoint != "" && cfg.FanControlDeviceIp != active.Connection.Endpoint {
		cfg.FanControlDeviceIp = active.Connection.Endpoint
		changed = true
	}
	return changed
}
