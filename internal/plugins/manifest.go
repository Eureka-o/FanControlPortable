package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
)

const (
	SupportedProtocolVersion = 1
	SupportedHostAPIVersion  = 1
)

var (
	pluginIDPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	settingKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
)

// Manifest describes an external plugin package stored under <install-dir>/plugins/<id>/plugin.json.
type Manifest struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Version         string         `json:"version"`
	Platform        string         `json:"platform"`
	MinCoreVersion  string         `json:"minCoreVersion,omitempty"`
	ProtocolVersion int            `json:"protocolVersion"`
	Runtime         string         `json:"runtime,omitempty"` // inproc-dll | sidecar | frontend-only
	UISlot          string         `json:"uiSlot,omitempty"`  // sidebar-page | settings-panel | none
	Backend         string         `json:"backend,omitempty"`
	Frontend        string         `json:"frontend,omitempty"`
	Style           string         `json:"style,omitempty"`
	Icon            string         `json:"icon,omitempty"`
	Author          string         `json:"author,omitempty"`
	Homepage        string         `json:"homepage,omitempty"`
	HostAPIVersion  int            `json:"hostApiVersion,omitempty"`
	Capabilities    []string       `json:"capabilities,omitempty"`
	Description     string         `json:"description,omitempty"`
	Settings        []SettingField `json:"settings,omitempty"`
}

func CurrentPlatform() string {
	return goruntime.GOOS + "-" + goruntime.GOARCH
}

// DiscoverExternal scans plugins/<id>/plugin.json non-recursively and returns validated metadata.
func DiscoverExternal(pluginsDir, currentPlatform, coreVersion string) []PluginInfo {
	infos, _ := DiscoverExternalWithDev(pluginsDir, nil, currentPlatform, coreVersion)
	return infos
}

// DiscoverExternalWithDev scans pluginsDir and optional developer plugin directories,
// returning validated PluginInfo records plus parsed manifests indexed by plugin ID.
func DiscoverExternalWithDev(pluginsDir string, devDirs []string, currentPlatform, coreVersion string) ([]PluginInfo, map[string]Manifest) {
	if strings.TrimSpace(currentPlatform) == "" {
		currentPlatform = CurrentPlatform()
	}

	out := make([]PluginInfo, 0)
	manifests := make(map[string]Manifest)
	seen := make(map[string]bool)

	if strings.TrimSpace(pluginsDir) != "" {
		if entries, err := os.ReadDir(pluginsDir); err == nil {
			resolvedRoot, rootErr := filepath.EvalSymlinks(pluginsDir)
			if rootErr != nil {
				resolvedRoot = filepath.Clean(pluginsDir)
			}
			for _, entry := range entries {
				dirName := entry.Name()
				if strings.HasPrefix(dirName, ".") {
					continue
				}
				pluginDir := filepath.Join(pluginsDir, dirName)
				stat, statErr := os.Stat(pluginDir)
				if statErr != nil || !stat.IsDir() {
					continue
				}
				manifestPath := filepath.Join(pluginDir, "plugin.json")
				if _, err := os.Stat(manifestPath); err != nil {
					continue
				}
				info, manifest := inspectExternalPluginDir(resolvedRoot, pluginDir, dirName, manifestPath, currentPlatform, coreVersion, false)
				seen[info.ID] = true
				out = append(out, info)
				if info.Supported && manifest.ID != "" {
					manifests[manifest.ID] = manifest
				}
			}
		}
	}

	for _, devDir := range devDirs {
		cleaned := strings.TrimSpace(devDir)
		if cleaned == "" {
			continue
		}
		absDir, err := filepath.Abs(cleaned)
		if err != nil {
			continue
		}
		stat, statErr := os.Stat(absDir)
		if statErr != nil || !stat.IsDir() {
			continue
		}
		manifestPath := filepath.Join(absDir, "plugin.json")
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}
		dirName := filepath.Base(absDir)
		parentRoot := filepath.Dir(absDir)
		resolvedParent, rootErr := filepath.EvalSymlinks(parentRoot)
		if rootErr != nil {
			resolvedParent = parentRoot
		}
		info, manifest := inspectExternalPluginDir(resolvedParent, absDir, dirName, manifestPath, currentPlatform, coreVersion, true)
		if seen[info.ID] {
			continue
		}
		seen[info.ID] = true
		out = append(out, info)
		if info.Supported && manifest.ID != "" {
			manifests[manifest.ID] = manifest
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, manifests
}

// InspectPluginDir validates an arbitrary unpacked plugin directory using the exact manifest contract.
func InspectPluginDir(pluginDir, currentPlatform, coreVersion string) (PluginInfo, Manifest, error) {
	if strings.TrimSpace(currentPlatform) == "" {
		currentPlatform = CurrentPlatform()
	}
	absDir, err := filepath.Abs(strings.TrimSpace(pluginDir))
	if err != nil {
		return PluginInfo{}, Manifest{}, fmt.Errorf("无效插件目录: %w", err)
	}
	manifestPath := filepath.Join(absDir, "plugin.json")
	if _, err := os.Stat(manifestPath); err != nil {
		return PluginInfo{}, Manifest{}, fmt.Errorf("缺少 plugin.json: %w", err)
	}
	parentRoot := filepath.Dir(absDir)
	resolvedParent, rootErr := filepath.EvalSymlinks(parentRoot)
	if rootErr != nil {
		resolvedParent = parentRoot
	}
	dirName := filepath.Base(absDir)
	info, manifest := inspectExternalPluginDir(resolvedParent, absDir, dirName, manifestPath, currentPlatform, coreVersion, false)
	if !info.Supported {
		return info, manifest, fmt.Errorf("%s", info.Reason)
	}
	return info, manifest, nil
}

// InspectPluginDirLooseID validates an unpacked directory (e.g. a temporary extraction dir whose
// folder name is random) without enforcing directory name == manifest.ID, while still verifying
// paths stay inside pluginDir.
func InspectPluginDirLooseID(pluginDir, currentPlatform, coreVersion string) (PluginInfo, Manifest, error) {
	if strings.TrimSpace(currentPlatform) == "" {
		currentPlatform = CurrentPlatform()
	}
	absDir, err := filepath.Abs(strings.TrimSpace(pluginDir))
	if err != nil {
		return PluginInfo{}, Manifest{}, fmt.Errorf("无效插件目录: %w", err)
	}
	manifestPath := filepath.Join(absDir, "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return PluginInfo{}, Manifest{}, fmt.Errorf("读取 plugin.json 失败: %w", err)
	}
	var peek Manifest
	if err := json.Unmarshal(data, &peek); err != nil {
		return PluginInfo{}, Manifest{}, fmt.Errorf("解析 plugin.json 失败: %w", err)
	}
	expectedDirName := strings.TrimSpace(peek.ID)
	resolvedRoot, rootErr := filepath.EvalSymlinks(filepath.Dir(absDir))
	if rootErr != nil {
		resolvedRoot = filepath.Dir(absDir)
	}
	info, manifest := inspectExternalPluginDir(resolvedRoot, absDir, expectedDirName, manifestPath, currentPlatform, coreVersion, false)
	if !info.Supported {
		return info, manifest, fmt.Errorf("%s", info.Reason)
	}
	return info, manifest, nil
}

func inspectExternalPluginDir(resolvedRoot, pluginDir, dirName, manifestPath, currentPlatform, coreVersion string, devMode bool) (PluginInfo, Manifest) {
	baseInfo := PluginInfo{
		ID:        dirName,
		Name:      dirName,
		Kind:      KindExternal,
		Category:  "extension",
		State:     StateIncompatible,
		Supported: false,
		DevMode:   devMode,
		Path:      pluginDir,
	}

	resolvedDir, err := filepath.EvalSymlinks(pluginDir)
	if err != nil || !isSubPath(resolvedRoot, resolvedDir) {
		baseInfo.Reason = "插件目录越界或符号链接不可用"
		return baseInfo, Manifest{}
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		baseInfo.Reason = fmt.Sprintf("读取 plugin.json 失败: %v", err)
		return baseInfo, Manifest{}
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		baseInfo.Reason = fmt.Sprintf("解析 plugin.json 失败: %v", err)
		return baseInfo, Manifest{}
	}

	manifest.ID = strings.TrimSpace(manifest.ID)
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Platform = strings.TrimSpace(manifest.Platform)
	manifest.MinCoreVersion = strings.TrimSpace(manifest.MinCoreVersion)
	manifest.Runtime = strings.ToLower(strings.TrimSpace(manifest.Runtime))
	manifest.UISlot = strings.ToLower(strings.TrimSpace(manifest.UISlot))
	manifest.Backend = strings.TrimSpace(manifest.Backend)
	manifest.Frontend = strings.TrimSpace(manifest.Frontend)
	manifest.Style = strings.TrimSpace(manifest.Style)
	manifest.Icon = strings.TrimSpace(manifest.Icon)
	manifest.Description = strings.TrimSpace(manifest.Description)

	if manifest.ID != "" {
		baseInfo.ID = manifest.ID
	}
	if manifest.Name != "" {
		baseInfo.Name = manifest.Name
	}
	baseInfo.Version = manifest.Version
	baseInfo.Description = manifest.Description
	baseInfo.Frontend = manifest.Frontend
	baseInfo.Style = manifest.Style
	baseInfo.Icon = manifest.Icon
	if len(manifest.Capabilities) > 0 {
		baseInfo.Capabilities = append([]string(nil), manifest.Capabilities...)
	}

	if !pluginIDPattern.MatchString(manifest.ID) {
		baseInfo.Reason = "无效的插件 ID"
		return baseInfo, manifest
	}
	if dirName != "" && manifest.ID != dirName {
		baseInfo.Reason = fmt.Sprintf("插件目录名 (%s) 与清单 ID (%s) 不一致", dirName, manifest.ID)
		return baseInfo, manifest
	}
	if manifest.Name == "" || manifest.Version == "" {
		baseInfo.Reason = "缺少插件名称或版本号"
		return baseInfo, manifest
	}
	if manifest.Platform != currentPlatform {
		baseInfo.Reason = fmt.Sprintf("平台不匹配（需要 %s，当前 %s）", manifest.Platform, currentPlatform)
		return baseInfo, manifest
	}
	if manifest.ProtocolVersion != SupportedProtocolVersion {
		baseInfo.Reason = fmt.Sprintf("协议版本不兼容（需要 %d）", manifest.ProtocolVersion)
		return baseInfo, manifest
	}
	if manifest.HostAPIVersion != 0 && manifest.HostAPIVersion != SupportedHostAPIVersion {
		baseInfo.Reason = fmt.Sprintf("宿主接口版本不兼容（v%d）", manifest.HostAPIVersion)
		return baseInfo, manifest
	}
	if manifest.MinCoreVersion != "" && !isCoreVersionCompatible(coreVersion, manifest.MinCoreVersion) {
		baseInfo.Reason = fmt.Sprintf("需要核心版本 >= %s", manifest.MinCoreVersion)
		return baseInfo, manifest
	}

	if manifest.Backend == "" && manifest.Frontend == "" {
		baseInfo.Reason = "清单未声明可运行的后端或前端入口"
		return baseInfo, manifest
	}

	resolvedRuntime, err := normalizeAndValidateRuntime(manifest.Runtime, manifest.Backend)
	if err != nil {
		baseInfo.Reason = err.Error()
		return baseInfo, manifest
	}
	manifest.Runtime = resolvedRuntime
	baseInfo.Runtime = resolvedRuntime
	baseInfo.RestartRequired = resolvedRuntime == RuntimeInprocDLL

	resolvedSlot, err := normalizeAndValidateUISlot(manifest.UISlot, manifest.Frontend)
	if err != nil {
		baseInfo.Reason = err.Error()
		return baseInfo, manifest
	}
	manifest.UISlot = resolvedSlot
	baseInfo.UISlot = resolvedSlot

	for _, entryRel := range []string{manifest.Backend, manifest.Frontend, manifest.Style} {
		if entryRel == "" {
			continue
		}
		if err := validateRelativeEntry(resolvedDir, entryRel); err != nil {
			baseInfo.Reason = err.Error()
			return baseInfo, manifest
		}
	}
	if manifest.Icon != "" && isRelativeFileReference(manifest.Icon) {
		if err := validateRelativeEntry(resolvedDir, manifest.Icon); err != nil {
			baseInfo.Reason = err.Error()
			return baseInfo, manifest
		}
	}

	if err := validateSettingsSchema(manifest.Settings); err != nil {
		baseInfo.Reason = err.Error()
		return baseInfo, manifest
	}
	if len(manifest.Settings) > 0 {
		baseInfo.Settings = cloneSettingFields(manifest.Settings)
	}

	baseInfo.Supported = true
	baseInfo.State = StateDiscovered
	return baseInfo, manifest
}

func normalizeAndValidateRuntime(runtimeName, backend string) (string, error) {
	if runtimeName == "" {
		switch {
		case strings.HasSuffix(strings.ToLower(backend), ".dll"):
			return RuntimeInprocDLL, nil
		case backend != "":
			return RuntimeSidecar, nil
		default:
			return RuntimeFrontendOnly, nil
		}
	}

	switch runtimeName {
	case RuntimeInprocDLL:
		if backend == "" || !strings.HasSuffix(strings.ToLower(backend), ".dll") {
			return "", fmt.Errorf("inproc-dll 插件必须声明 .dll 后端入口")
		}
		return RuntimeInprocDLL, nil
	case RuntimeSidecar:
		if backend == "" {
			return "", fmt.Errorf("sidecar 插件必须声明后端可执行入口")
		}
		return RuntimeSidecar, nil
	case RuntimeFrontendOnly:
		if backend != "" {
			return "", fmt.Errorf("frontend-only 插件不能声明后端入口")
		}
		return RuntimeFrontendOnly, nil
	default:
		return "", fmt.Errorf("不支持的插件运行模式: %s", runtimeName)
	}
}

func normalizeAndValidateUISlot(uiSlot, frontend string) (string, error) {
	if uiSlot == "" {
		if frontend != "" {
			return UISlotSettingsPanel, nil
		}
		return UISlotNone, nil
	}
	switch uiSlot {
	case UISlotSidebarPage, UISlotSettingsPanel, UISlotNone:
		return uiSlot, nil
	default:
		return "", fmt.Errorf("不支持的插件界面位置: %s", uiSlot)
	}
}

func isRelativeFileReference(val string) bool {
	if strings.ContainsAny(val, `/\`) {
		return true
	}
	ext := strings.ToLower(filepath.Ext(val))
	return ext != ""
}

func validateSettingsSchema(fields []SettingField) error {
	if len(fields) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(fields))
	for i := range fields {
		key := strings.TrimSpace(fields[i].Key)
		if !settingKeyPattern.MatchString(key) {
			return fmt.Errorf("无效的插件设置项键名: %s", fields[i].Key)
		}
		if seen[key] {
			return fmt.Errorf("重复的插件设置项键名: %s", key)
		}
		seen[key] = true
		if strings.TrimSpace(fields[i].Title) == "" {
			return fmt.Errorf("插件设置项缺少标题: %s", key)
		}
		switch strings.ToLower(strings.TrimSpace(fields[i].Type)) {
		case "boolean", "string", "number", "slider":
		case "select":
			if len(fields[i].Options) == 0 {
				return fmt.Errorf("select 类型设置项缺少选项: %s", key)
			}
		default:
			return fmt.Errorf("不支持的插件设置项类型: %s (%s)", fields[i].Type, key)
		}
	}
	return nil
}

func cloneSettingFields(input []SettingField) []SettingField {
	if len(input) == 0 {
		return nil
	}
	out := make([]SettingField, len(input))
	for i, field := range input {
		if len(field.Options) > 0 {
			field.Options = append([]SettingOption(nil), field.Options...)
		}
		if field.Min != nil {
			v := *field.Min
			field.Min = &v
		}
		if field.Max != nil {
			v := *field.Max
			field.Max = &v
		}
		if field.Step != nil {
			v := *field.Step
			field.Step = &v
		}
		out[i] = field
	}
	return out
}

func validateRelativeEntry(resolvedPluginDir, relPath string) error {
	trimmed := strings.TrimSpace(relPath)
	if filepath.IsAbs(trimmed) || strings.HasPrefix(trimmed, `\`) || strings.HasPrefix(trimmed, `/`) {
		return fmt.Errorf("入口路径不允许为绝对路径: %s", trimmed)
	}
	candidate := filepath.Clean(filepath.Join(resolvedPluginDir, filepath.FromSlash(trimmed)))
	if !isSubPath(resolvedPluginDir, candidate) {
		return fmt.Errorf("入口路径越出插件目录: %s", trimmed)
	}
	resolvedFile, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return fmt.Errorf("缺少入口文件: %s", trimmed)
	}
	if !isSubPath(resolvedPluginDir, resolvedFile) {
		return fmt.Errorf("入口文件符号链接越出插件目录: %s", trimmed)
	}
	info, err := os.Stat(resolvedFile)
	if err != nil || info.IsDir() {
		return fmt.Errorf("无效的入口文件: %s", trimmed)
	}
	return nil
}

func isSubPath(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func isCoreVersionCompatible(current, minimum string) bool {
	curTrimmed := strings.TrimSpace(current)
	if curTrimmed == "" || strings.EqualFold(curTrimmed, "dev") {
		return true
	}
	curParts, curOK := parseSemverTuple(curTrimmed)
	minParts, minOK := parseSemverTuple(minimum)
	if !minOK {
		return false
	}
	if !curOK {
		return true
	}
	for i := 0; i < 3; i++ {
		if curParts[i] > minParts[i] {
			return true
		}
		if curParts[i] < minParts[i] {
			return false
		}
	}
	return true
}

func parseSemverTuple(raw string) ([3]int, bool) {
	var parts [3]int
	cleaned := strings.TrimPrefix(strings.TrimSpace(raw), "v")
	if dash := strings.IndexAny(cleaned, "-+"); dash >= 0 {
		cleaned = cleaned[:dash]
	}
	segs := strings.Split(cleaned, ".")
	if len(segs) < 2 || len(segs) > 3 {
		return parts, false
	}
	for i := 0; i < len(segs); i++ {
		n, err := strconv.Atoi(segs[i])
		if err != nil || n < 0 {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}
