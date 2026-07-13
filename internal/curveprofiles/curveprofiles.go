package curveprofiles

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	cfgpkg "github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/types"
)

const exportPrefix = "B2C1."

var generatedIDCounter uint64

type exportPayload struct {
	V        int                     `json:"v"`
	Active   string                  `json:"a"`
	Profiles []types.FanCurveProfile `json:"p"`
}

func CloneCurve(curve []types.FanCurvePoint) []types.FanCurvePoint {
	if len(curve) == 0 {
		return nil
	}
	out := make([]types.FanCurvePoint, len(curve))
	copy(out, curve)
	return out
}

func defaultCurveForUnit(unit string) []types.FanCurvePoint {
	if types.IsRPMSpeedUnit(unit) {
		return types.GetDefaultRPMFanCurve()
	}
	return types.GetDefaultFanCurve()
}

func looksLikePercentCurveInRPMMode(curve []types.FanCurvePoint, unit string) bool {
	if !types.IsRPMSpeedUnit(unit) || len(curve) == 0 {
		return false
	}
	maxSpeed := curve[0].RPM
	for i := 1; i < len(curve); i++ {
		if curve[i].RPM > maxSpeed {
			maxSpeed = curve[i].RPM
		}
	}
	return maxSpeed <= types.FanSpeedMaxPercent
}

func extendCurveRightEdgeForUnit(curve []types.FanCurvePoint, unit string) ([]types.FanCurvePoint, bool) {
	if len(curve) == 0 {
		return nil, false
	}

	defaultCurve := defaultCurveForUnit(unit)
	defaultMaxTemp := defaultCurve[len(defaultCurve)-1].Temperature
	lastPoint := curve[len(curve)-1]
	if lastPoint.Temperature >= defaultMaxTemp {
		return curve, false
	}

	extended := CloneCurve(curve)
	for _, point := range defaultCurve {
		if point.Temperature <= lastPoint.Temperature {
			continue
		}
		extended = append(extended, types.FanCurvePoint{
			Temperature: point.Temperature,
			RPM:         lastPoint.RPM,
		})
	}

	return extended, len(extended) != len(curve)
}

func extendCurveRightEdge(curve []types.FanCurvePoint) ([]types.FanCurvePoint, bool) {
	return extendCurveRightEdgeForUnit(curve, types.FanSpeedUnitPercent)
}

func CloneProfiles(profiles []types.FanCurveProfile) []types.FanCurveProfile {
	if len(profiles) == 0 {
		return nil
	}
	out := make([]types.FanCurveProfile, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, types.FanCurveProfile{
			ID:    p.ID,
			Name:  p.Name,
			Curve: CloneCurve(p.Curve),
		})
	}
	return out
}

func truncateByRunes(input string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(input) <= maxRunes {
		return input
	}
	r := []rune(input)
	return string(r[:maxRunes])
}

func looksCorruptedProfileName(name string) bool {
	n := strings.TrimSpace(name)
	if n == "" {
		return false
	}
	questionLike := 0
	meaningful := 0
	for _, r := range n {
		switch r {
		case '\uFFFD':
			return true
		case ' ', '\t', '\r', '\n', '-', '_', '/', '\\', '(', ')', '[', ']', '（', '）':
			continue
		case '?':
			questionLike++
			meaningful++
		default:
			meaningful++
		}
	}
	return meaningful > 0 && questionLike == meaningful && questionLike >= 2
}

func NormalizeProfileName(name string, fallback string) string {
	n := strings.TrimSpace(name)
	if n == "" || looksCorruptedProfileName(n) {
		n = fallback
	}
	return truncateByRunes(n, 6)
}

func GenerateID() string {
	return fmt.Sprintf("p%x-%x", time.Now().UnixNano(), atomic.AddUint64(&generatedIDCounter, 1))
}

func AppendImportedProfiles(existing, imported []types.FanCurveProfile, importedActiveID string) ([]types.FanCurveProfile, string) {
	merged := CloneProfiles(existing)
	usedIDs := make(map[string]bool, len(existing)+len(imported))
	usedNames := make(map[string]bool, len(existing)+len(imported))
	for _, profile := range merged {
		usedIDs[profile.ID] = true
		usedNames[profile.Name] = true
	}

	newActiveID := ""
	for index, profile := range imported {
		originalID := profile.ID
		profile.ID = GenerateID()
		for usedIDs[profile.ID] {
			profile.ID = GenerateID()
		}
		usedIDs[profile.ID] = true
		profile.Name = uniqueImportedProfileName(profile.Name, fmt.Sprintf("Import%d", index+1), usedNames)
		profile.Curve = CloneCurve(profile.Curve)
		merged = append(merged, profile)
		if originalID == importedActiveID {
			newActiveID = profile.ID
		}
	}
	if newActiveID == "" && len(imported) > 0 {
		newActiveID = merged[len(existing)].ID
	}
	return merged, newActiveID
}

func uniqueImportedProfileName(name, fallback string, used map[string]bool) string {
	base := NormalizeProfileName(name, fallback)
	if !used[base] {
		used[base] = true
		return base
	}
	for suffix := 2; ; suffix++ {
		suffixText := strconv.Itoa(suffix)
		candidate := truncateByRunes(base, 6-utf8.RuneCountInString(suffixText)) + suffixText
		if !used[candidate] {
			used[candidate] = true
			return candidate
		}
	}
}

func FindIndex(profiles []types.FanCurveProfile, profileID string) int {
	for i := range profiles {
		if profiles[i].ID == profileID {
			return i
		}
	}
	return -1
}

func NormalizeConfig(cfg *types.AppConfig) bool {
	return NormalizeConfigForUnit(cfg, types.FanSpeedUnitPercent)
}

func NormalizeConfigForUnit(cfg *types.AppConfig, unit string) bool {
	if cfg == nil {
		return false
	}
	unit = types.NormalizeFanSpeedUnit(unit)

	changed := false
	baseCurve := CloneCurve(cfg.FanCurve)
	if len(baseCurve) == 0 {
		baseCurve = defaultCurveForUnit(unit)
		changed = true
	}
	if looksLikePercentCurveInRPMMode(baseCurve, unit) {
		baseCurve = defaultCurveForUnit(unit)
		changed = true
	}
	if extendedCurve, extended := extendCurveRightEdgeForUnit(baseCurve, unit); extended {
		baseCurve = extendedCurve
		changed = true
	}
	if err := cfgpkg.ValidateFanCurveForUnit(baseCurve, unit); err != nil {
		baseCurve = defaultCurveForUnit(unit)
		changed = true
	}

	if len(cfg.FanCurveProfiles) == 0 {
		cfg.FanCurveProfiles = []types.FanCurveProfile{{
			ID:    "default",
			Name:  "默认",
			Curve: CloneCurve(baseCurve),
		}}
		changed = true
	}

	seenIDs := map[string]bool{}
	normalized := make([]types.FanCurveProfile, 0, len(cfg.FanCurveProfiles))
	for i, p := range cfg.FanCurveProfiles {
		profile := p
		if profile.ID == "" || seenIDs[profile.ID] {
			profile.ID = GenerateID()
			changed = true
		}
		seenIDs[profile.ID] = true

		fallbackName := fmt.Sprintf("方案%d", i+1)
		name := NormalizeProfileName(profile.Name, fallbackName)
		if name != profile.Name {
			profile.Name = name
			changed = true
		}

		if extendedCurve, extended := extendCurveRightEdgeForUnit(profile.Curve, unit); extended {
			profile.Curve = extendedCurve
			changed = true
		}

		if looksLikePercentCurveInRPMMode(profile.Curve, unit) {
			profile.Curve = CloneCurve(baseCurve)
			changed = true
		} else if err := cfgpkg.ValidateFanCurveForUnit(profile.Curve, unit); err != nil {
			profile.Curve = CloneCurve(baseCurve)
			changed = true
		}
		normalized = append(normalized, types.FanCurveProfile{
			ID:    profile.ID,
			Name:  profile.Name,
			Curve: CloneCurve(profile.Curve),
		})
	}

	cfg.FanCurveProfiles = normalized
	if len(cfg.FanCurveProfiles) == 0 {
		cfg.FanCurveProfiles = []types.FanCurveProfile{{
			ID:    "default",
			Name:  "默认",
			Curve: CloneCurve(baseCurve),
		}}
		changed = true
	}

	if FindIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID) < 0 {
		cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[0].ID
		changed = true
	}

	activeIdx := FindIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID)
	if activeIdx < 0 {
		activeIdx = 0
		cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[0].ID
		changed = true
	}
	activeCurve := CloneCurve(cfg.FanCurveProfiles[activeIdx].Curve)
	if len(activeCurve) == 0 {
		activeCurve = defaultCurveForUnit(unit)
		cfg.FanCurveProfiles[activeIdx].Curve = CloneCurve(activeCurve)
		changed = true
	}

	if len(cfg.FanCurve) != len(activeCurve) {
		cfg.FanCurve = CloneCurve(activeCurve)
		changed = true
	} else {
		for i := range cfg.FanCurve {
			if cfg.FanCurve[i] != activeCurve[i] {
				cfg.FanCurve = CloneCurve(activeCurve)
				changed = true
				break
			}
		}
	}

	return changed
}

func Export(activeID string, profiles []types.FanCurveProfile) (string, error) {
	payload := exportPayload{
		V:        1,
		Active:   activeID,
		Profiles: CloneProfiles(profiles),
	}
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

func Import(code string) ([]types.FanCurveProfile, string, error) {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return nil, "", fmt.Errorf("导入字符串不能为空")
	}
	if !strings.HasPrefix(trimmed, exportPrefix) {
		return nil, "", fmt.Errorf("导入字符串格式错误")
	}

	raw := strings.TrimPrefix(trimmed, exportPrefix)
	compressed, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, "", fmt.Errorf("导入字符串解码失败")
	}

	zr, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, "", fmt.Errorf("导入字符串解压失败")
	}
	defer zr.Close()

	plain, err := io.ReadAll(zr)
	if err != nil {
		return nil, "", fmt.Errorf("导入数据读取失败")
	}

	var payload exportPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return nil, "", fmt.Errorf("导入数据格式错误")
	}
	if payload.V != 1 {
		return nil, "", fmt.Errorf("不支持的导入版本")
	}

	return CloneProfiles(payload.Profiles), strings.TrimSpace(payload.Active), nil
}
