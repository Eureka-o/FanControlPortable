package deviceprofiles

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/TIANLI0/THRM/internal/types"
)

const (
	exportPrefix  = "FCDP1."
	exportSchema  = "fancontrol.device-profiles"
	exportVersion = 1
)

type exportPayload struct {
	Schema               string                `json:"schema"`
	Version              int                   `json:"version"`
	ActiveID             string                `json:"activeId,omitempty"`
	ActiveIDsByTransport map[string]string     `json:"activeIdsByTransport,omitempty"`
	Profiles             []types.DeviceProfile `json:"profiles"`
	ExportedAt           string                `json:"exportedAt,omitempty"`
}

func SupportedProfiles() []types.DeviceProfile {
	return []types.DeviceProfile{
		NormalizeForStorage(types.DefaultWiFiPercentTemplateProfile(types.DefaultFanDeviceIP), ""),
	}
}

func CloneProfile(profile types.DeviceProfile) types.DeviceProfile {
	profile.Commands = append([]types.DeviceCommandTemplate(nil), profile.Commands...)
	profile.ResponseParsers = append([]types.DeviceResponseParser(nil), profile.ResponseParsers...)
	profile.SpeedMap = append([]types.DeviceSpeedMapPoint(nil), profile.SpeedMap...)
	return profile
}

func CloneProfiles(profiles []types.DeviceProfile) []types.DeviceProfile {
	if len(profiles) == 0 {
		return nil
	}
	out := make([]types.DeviceProfile, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, CloneProfile(profile))
	}
	return out
}

func FindIndex(profiles []types.DeviceProfile, profileID string) int {
	profileID = strings.TrimSpace(profileID)
	for i := range profiles {
		if profiles[i].ID == profileID && profileID != "" {
			return i
		}
	}
	return -1
}

func FindFirstByTransport(profiles []types.DeviceProfile, transport string) int {
	transport = types.NormalizeDeviceTransport(transport)
	for i := range profiles {
		if strings.TrimSpace(profiles[i].ID) != "" && types.NormalizeDeviceTransport(profiles[i].Transport) == transport {
			return i
		}
	}
	return -1
}

func FilterActiveIDsByTransport(activeIDs map[string]string, profiles []types.DeviceProfile) map[string]string {
	if len(activeIDs) == 0 || len(profiles) == 0 {
		return nil
	}
	out := make(map[string]string, len(activeIDs))
	for rawTransport, rawID := range activeIDs {
		transport := types.NormalizeDeviceTransport(strings.TrimSpace(rawTransport))
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		idx := FindIndex(profiles, id)
		if idx < 0 || types.NormalizeDeviceTransport(profiles[idx].Transport) != transport {
			continue
		}
		out[transport] = id
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func IsBuiltInProfileID(profileID string) bool {
	return types.IsBuiltInDeviceProfileID(profileID)
}

func GenerateID() string {
	return fmt.Sprintf("user.device.%x", time.Now().UnixNano())
}

func NormalizeForStorage(profile types.DeviceProfile, fallbackEndpoint string) types.DeviceProfile {
	profile = CloneProfile(profile)
	profile.ID = strings.TrimSpace(profile.ID)
	profile.DisplayName = truncateByRunes(strings.TrimSpace(profile.DisplayName), 64)
	profile.Vendor = truncateByRunes(strings.TrimSpace(profile.Vendor), 64)
	profile.Model = truncateByRunes(strings.TrimSpace(profile.Model), 64)
	profile.Notes = truncateByRunes(strings.TrimSpace(profile.Notes), 512)
	profile.Transport = strings.ToLower(strings.TrimSpace(profile.Transport))
	profile.SpeedUnit = strings.ToLower(strings.TrimSpace(profile.SpeedUnit))
	profile.Connection = normalizeConnection(profile.Connection)

	for i := range profile.Commands {
		profile.Commands[i].Name = truncateByRunes(strings.TrimSpace(profile.Commands[i].Name), 64)
		profile.Commands[i].Command = strings.TrimSpace(profile.Commands[i].Command)
		profile.Commands[i].Encoding = strings.ToLower(strings.TrimSpace(profile.Commands[i].Encoding))
		profile.Commands[i].Checksum = strings.ToLower(strings.TrimSpace(profile.Commands[i].Checksum))
		profile.Commands[i].Description = truncateByRunes(strings.TrimSpace(profile.Commands[i].Description), 256)
	}
	for i := range profile.ResponseParsers {
		profile.ResponseParsers[i].Name = truncateByRunes(strings.TrimSpace(profile.ResponseParsers[i].Name), 64)
		profile.ResponseParsers[i].Type = strings.ToLower(strings.TrimSpace(profile.ResponseParsers[i].Type))
		profile.ResponseParsers[i].Expression = strings.TrimSpace(profile.ResponseParsers[i].Expression)
	}

	profile = types.NormalizeDeviceProfile(profile, fallbackEndpoint)
	if IsBuiltInProfileID(profile.ID) {
		profile.BuiltIn = true
	} else {
		profile.BuiltIn = false
	}
	return profile
}

func NormalizeAndValidate(profile types.DeviceProfile, fallbackEndpoint string) (types.DeviceProfile, error) {
	rawTransport := strings.ToLower(strings.TrimSpace(profile.Transport))
	rawUnit := strings.ToLower(strings.TrimSpace(profile.SpeedUnit))
	if !isValidTransport(rawTransport) {
		return types.DeviceProfile{}, fmt.Errorf("device profile transport must be wifi, ble, serial, or hid")
	}
	if !isValidSpeedUnit(rawUnit) {
		return types.DeviceProfile{}, fmt.Errorf("device profile speed unit must be percent or rpm")
	}

	if strings.TrimSpace(profile.ID) == "" {
		return types.DeviceProfile{}, fmt.Errorf("device profile id is required")
	}
	if !isValidProfileID(profile.ID) {
		return types.DeviceProfile{}, fmt.Errorf("device profile id contains unsupported characters")
	}
	if strings.TrimSpace(profile.DisplayName) == "" {
		return types.DeviceProfile{}, fmt.Errorf("device profile display name is required")
	}
	if !profile.Capabilities.SupportsReadState && !profile.Capabilities.SupportsSetSpeed {
		return types.DeviceProfile{}, fmt.Errorf("device profile must support reading state or setting speed")
	}
	if rawTransport == types.DeviceTransportWiFi {
		if strings.TrimSpace(profile.Connection.Endpoint) == "" && strings.TrimSpace(fallbackEndpoint) == "" {
			return types.DeviceProfile{}, fmt.Errorf("wifi device profile requires an endpoint")
		}
		if profile.Capabilities.SupportsReadState && strings.TrimSpace(profile.Connection.StateEndpoint) == "" {
			return types.DeviceProfile{}, fmt.Errorf("wifi device profile requires a state endpoint")
		}
		if profile.Capabilities.SupportsSetSpeed && strings.TrimSpace(profile.Connection.SpeedEndpoint) == "" {
			return types.DeviceProfile{}, fmt.Errorf("wifi device profile requires a speed endpoint")
		}
	}

	profile = NormalizeForStorage(profile, fallbackEndpoint)
	if err := validateNormalized(profile, fallbackEndpoint); err != nil {
		return types.DeviceProfile{}, err
	}
	return profile, nil
}

func Validate(profile types.DeviceProfile) error {
	_, err := NormalizeAndValidate(profile, "")
	return err
}

func Export(activeID string, profiles []types.DeviceProfile) (string, error) {
	return ExportWithActiveIDs(activeID, nil, profiles)
}

func ExportWithActiveIDs(activeID string, activeIDsByTransport map[string]string, profiles []types.DeviceProfile) (string, error) {
	normalized, err := normalizeProfileSet(profiles, "")
	if err != nil {
		return "", err
	}
	if len(normalized) == 0 {
		return "", fmt.Errorf("device profile list is empty")
	}
	activeID = strings.TrimSpace(activeID)
	if activeID != "" && FindIndex(normalized, activeID) < 0 {
		activeID = ""
	}

	payload := exportPayload{
		Schema:               exportSchema,
		Version:              exportVersion,
		ActiveID:             activeID,
		ActiveIDsByTransport: FilterActiveIDsByTransport(activeIDsByTransport, normalized),
		Profiles:             normalized,
		ExportedAt:           time.Now().UTC().Format(time.RFC3339),
	}
	return encodePayload(payload)
}

func Import(code string) ([]types.DeviceProfile, string, error) {
	profiles, activeID, _, err := ImportWithActiveIDs(code)
	return profiles, activeID, err
}

func ImportWithActiveIDs(code string) ([]types.DeviceProfile, string, map[string]string, error) {
	payload, err := decodePayload(code)
	if err != nil {
		return nil, "", nil, err
	}
	if payload.Schema != exportSchema || payload.Version != exportVersion {
		return nil, "", nil, fmt.Errorf("unsupported device profile export schema")
	}

	profiles, err := normalizeProfileSet(payload.Profiles, "")
	if err != nil {
		return nil, "", nil, err
	}
	if len(profiles) == 0 {
		return nil, "", nil, fmt.Errorf("device profile export has no profiles")
	}

	activeID := strings.TrimSpace(payload.ActiveID)
	if activeID != "" && FindIndex(profiles, activeID) < 0 {
		activeID = ""
	}
	return profiles, activeID, FilterActiveIDsByTransport(payload.ActiveIDsByTransport, profiles), nil
}

func normalizeProfileSet(profiles []types.DeviceProfile, fallbackEndpoint string) ([]types.DeviceProfile, error) {
	seen := make(map[string]bool, len(profiles))
	normalized := make([]types.DeviceProfile, 0, len(profiles))
	for _, profile := range profiles {
		profile, err := NormalizeAndValidate(profile, fallbackEndpoint)
		if err != nil {
			return nil, err
		}
		if seen[profile.ID] {
			return nil, fmt.Errorf("duplicate device profile id: %s", profile.ID)
		}
		seen[profile.ID] = true
		normalized = append(normalized, profile)
	}
	return normalized, nil
}

func encodePayload(payload exportPayload) (string, error) {
	plain, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(plain); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}

	return exportPrefix + base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

func decodePayload(code string) (exportPayload, error) {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return exportPayload{}, fmt.Errorf("device profile import string cannot be empty")
	}

	var plain []byte
	if strings.HasPrefix(trimmed, exportPrefix) {
		raw := strings.TrimPrefix(trimmed, exportPrefix)
		compressed, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			return exportPayload{}, fmt.Errorf("device profile import string could not be decoded")
		}

		zr, err := zlib.NewReader(bytes.NewReader(compressed))
		if err != nil {
			return exportPayload{}, fmt.Errorf("device profile import string could not be decompressed")
		}
		defer zr.Close()

		plain, err = io.ReadAll(zr)
		if err != nil {
			return exportPayload{}, fmt.Errorf("device profile import data could not be read")
		}
	} else {
		plain = []byte(trimmed)
	}

	var payload exportPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return exportPayload{}, fmt.Errorf("device profile import data is not valid JSON")
	}
	return payload, nil
}

func validateNormalized(profile types.DeviceProfile, fallbackEndpoint string) error {
	if profile.SpeedRange.Max <= profile.SpeedRange.Min {
		return fmt.Errorf("device profile speed range must have max greater than min")
	}
	if profile.SpeedRange.Step < 0 || profile.SpeedRange.TickScale < 0 {
		return fmt.Errorf("device profile speed range step values cannot be negative")
	}
	if profile.SpeedUnit == types.FanSpeedUnitPercent {
		if profile.SpeedRange.Min < types.FanSpeedMinPercent || profile.SpeedRange.Max > types.FanSpeedMaxPercent {
			return fmt.Errorf("percent device profile speed range must stay within 0-100")
		}
	}
	if profile.SpeedUnit == types.FanSpeedUnitRPM && profile.SpeedRange.Max > 30000 {
		return fmt.Errorf("rpm device profile speed range is unrealistically high")
	}

	if err := validateTransportSettings(profile, fallbackEndpoint); err != nil {
		return err
	}
	if err := validateSpeedMap(profile.SpeedMap); err != nil {
		return err
	}
	if err := validateCommands(profile.Commands); err != nil {
		return err
	}
	if err := validateParsers(profile.ResponseParsers); err != nil {
		return err
	}
	return nil
}

func validateTransportSettings(profile types.DeviceProfile, fallbackEndpoint string) error {
	switch profile.Transport {
	case types.DeviceTransportWiFi:
		if strings.TrimSpace(profile.Connection.Endpoint) == "" && strings.TrimSpace(fallbackEndpoint) == "" {
			return fmt.Errorf("wifi device profile requires an endpoint")
		}
		if profile.Capabilities.SupportsReadState && strings.TrimSpace(profile.Connection.StateEndpoint) == "" {
			return fmt.Errorf("wifi device profile requires a state endpoint")
		}
		if profile.Capabilities.SupportsSetSpeed && strings.TrimSpace(profile.Connection.SpeedEndpoint) == "" {
			return fmt.Errorf("wifi device profile requires a speed endpoint")
		}
		if !isValidHTTPMethod(profile.Connection.HTTPMethod) {
			return fmt.Errorf("wifi device profile HTTP method is not supported")
		}
		if err := validateWiFiRuntimePolicy(profile.Connection); err != nil {
			return err
		}
	case types.DeviceTransportBLE:
		if !isUUIDLike(profile.Connection.BLEServiceUUID) {
			return fmt.Errorf("ble device profile requires a valid service UUID")
		}
		if profile.Capabilities.SupportsSetSpeed && !isUUIDLike(profile.Connection.BLEWriteCharacteristic) {
			return fmt.Errorf("ble device profile requires a valid write characteristic UUID")
		}
		if profile.Capabilities.SupportsReadState && strings.TrimSpace(profile.Connection.BLENotifyCharacteristic) != "" && !isUUIDLike(profile.Connection.BLENotifyCharacteristic) {
			return fmt.Errorf("ble notify characteristic UUID is invalid")
		}
	case types.DeviceTransportSerial:
		if strings.TrimSpace(profile.Connection.SerialPort) == "" {
			return fmt.Errorf("serial device profile requires a port")
		}
		if profile.Connection.SerialBaudRate <= 0 {
			return fmt.Errorf("serial device profile requires a positive baud rate")
		}
		if profile.Connection.SerialDataBits != 0 && (profile.Connection.SerialDataBits < 5 || profile.Connection.SerialDataBits > 8) {
			return fmt.Errorf("serial device profile data bits must be 5-8")
		}
		if profile.Connection.SerialStopBits != 0 && profile.Connection.SerialStopBits != 1 && profile.Connection.SerialStopBits != 2 {
			return fmt.Errorf("serial device profile stop bits must be 1 or 2")
		}
	case types.DeviceTransportHID:
		if profile.SpeedUnit != types.FanSpeedUnitRPM {
			return fmt.Errorf("hid legacy profiles must use rpm speed")
		}
	default:
		return fmt.Errorf("device profile transport is not supported")
	}
	return nil
}

func validateSpeedMap(points []types.DeviceSpeedMapPoint) error {
	if len(points) == 0 {
		return nil
	}
	if len(points) < 2 {
		return fmt.Errorf("percent-to-rpm speed map requires at least two points")
	}
	prevPercent := -1
	prevRPM := -1
	for _, point := range points {
		if point.PercentTicks < types.FanSpeedMinPercentTicks || point.PercentTicks > types.FanSpeedMaxPercentTicks {
			return fmt.Errorf("speed map percent ticks must stay within 0-1000")
		}
		if point.RPM < 0 {
			return fmt.Errorf("speed map rpm cannot be negative")
		}
		if point.PercentTicks < prevPercent || point.RPM < prevRPM {
			return fmt.Errorf("speed map points must be sorted and non-decreasing")
		}
		prevPercent = point.PercentTicks
		prevRPM = point.RPM
	}
	return nil
}

func validateWiFiRuntimePolicy(conn types.DeviceConnectionSettings) error {
	if conn.RequestTimeoutMs < 0 || conn.RequestTimeoutMs > 30000 {
		return fmt.Errorf("wifi request timeout must be between 0 and 30000 ms")
	}
	if conn.MinSendIntervalMs < 0 || conn.MinSendIntervalMs > 60000 {
		return fmt.Errorf("wifi minimum send interval must be between 0 and 60000 ms")
	}
	if conn.MaxRetries < 0 || conn.MaxRetries > 5 {
		return fmt.Errorf("wifi max retries must be between 0 and 5")
	}
	if conn.RetryBackoffMs < 0 || conn.RetryBackoffMs > 10000 {
		return fmt.Errorf("wifi retry backoff must be between 0 and 10000 ms")
	}
	return nil
}

func validateCommands(commands []types.DeviceCommandTemplate) error {
	for _, command := range commands {
		if command.Name == "" {
			return fmt.Errorf("device command template name is required")
		}
		if command.Command == "" {
			return fmt.Errorf("device command template payload is required")
		}
		if !isAllowed(command.Encoding, "", "json", "hex", "ascii", "raw") {
			return fmt.Errorf("device command template encoding is not supported")
		}
		if !isAllowed(command.Checksum, "", "none", "sum8", "xor8", "crc16") {
			return fmt.Errorf("device command template checksum is not supported")
		}
		encoding := command.Encoding
		if encoding == "" {
			encoding = "json"
		}
		if encoding == "json" && command.Checksum != "" && command.Checksum != "none" {
			return fmt.Errorf("device command template checksum is only supported for hex, ascii, or raw encodings")
		}
	}
	return nil
}

func validateParsers(parsers []types.DeviceResponseParser) error {
	for _, parser := range parsers {
		if parser.Name == "" {
			return fmt.Errorf("device response parser name is required")
		}
		if !isAllowed(parser.Type, "jsonpath", "json_path", "byteoffset", "byte_offset", "regex", "plain") {
			return fmt.Errorf("device response parser type is not supported")
		}
		if parser.Type != "plain" && parser.Expression == "" {
			return fmt.Errorf("device response parser expression is required")
		}
	}
	return nil
}

func normalizeConnection(conn types.DeviceConnectionSettings) types.DeviceConnectionSettings {
	conn.Endpoint = strings.TrimSpace(conn.Endpoint)
	conn.StateEndpoint = normalizeEndpointPath(conn.StateEndpoint)
	conn.SpeedEndpoint = normalizeEndpointPath(conn.SpeedEndpoint)
	conn.HTTPMethod = strings.ToUpper(strings.TrimSpace(conn.HTTPMethod))
	conn.BLENameFilter = strings.TrimSpace(conn.BLENameFilter)
	conn.BLEServiceUUID = strings.ToLower(strings.TrimSpace(conn.BLEServiceUUID))
	conn.BLEWriteCharacteristic = strings.ToLower(strings.TrimSpace(conn.BLEWriteCharacteristic))
	conn.BLENotifyCharacteristic = strings.ToLower(strings.TrimSpace(conn.BLENotifyCharacteristic))
	conn.SerialPort = strings.TrimSpace(conn.SerialPort)
	conn.SerialParity = strings.ToLower(strings.TrimSpace(conn.SerialParity))
	if conn.SerialParity == "" {
		conn.SerialParity = "none"
	}
	conn.SerialFrameDelimiter = strings.TrimSpace(conn.SerialFrameDelimiter)
	return conn
}

func normalizeEndpointPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func isValidTransport(transport string) bool {
	switch transport {
	case types.DeviceTransportWiFi, types.DeviceTransportBLE, types.DeviceTransportSerial, types.DeviceTransportHID:
		return true
	default:
		return false
	}
}

func isValidSpeedUnit(unit string) bool {
	return unit == types.FanSpeedUnitPercent || unit == types.FanSpeedUnitRPM
}

func isValidHTTPMethod(method string) bool {
	if method == "" {
		return true
	}
	return isAllowed(method, "GET", "POST", "PUT", "PATCH")
}

func isValidProfileID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_' || r == ':':
		default:
			return false
		}
	}
	return true
}

func isUUIDLike(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	hexLen := 0
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			hexLen++
		case r >= 'a' && r <= 'f':
			hexLen++
		case r >= 'A' && r <= 'F':
			hexLen++
		case r == '-':
		default:
			return false
		}
	}
	return hexLen == 4 || hexLen == 8 || hexLen == 32
}

func isAllowed(value string, allowed ...string) bool {
	for _, option := range allowed {
		if value == option {
			return true
		}
	}
	return false
}

func truncateByRunes(input string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(input) <= maxRunes {
		return input
	}
	return string([]rune(input)[:maxRunes])
}
