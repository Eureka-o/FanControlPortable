package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	AssetURLPrefix    = "/plugin-assets/"
	maxPluginAssetLen = 8 << 20
)

var allowedPluginAssetExts = map[string]bool{
	".js":    true,
	".mjs":   true,
	".css":   true,
	".svg":   true,
	".png":   true,
	".webp":  true,
	".jpg":   true,
	".jpeg":  true,
	".woff2": true,
	".json":  true,
}

// AssetFile holds bytes and metadata for one served plugin frontend asset.
type AssetFile struct {
	Name    string
	Data    []byte
	ModTime time.Time
}

// ReadPluginAsset safely resolves and reads a relative frontend/style/icon asset for a plugin.
func ReadPluginAsset(pluginsDir string, devDirs []string, pluginID, relPath string) (AssetFile, error) {
	id := strings.TrimSpace(pluginID)
	if !pluginIDPattern.MatchString(id) {
		return AssetFile{}, fmt.Errorf("无效插件 ID: %s", pluginID)
	}
	trimmedRel := strings.TrimSpace(relPath)
	if trimmedRel == "" {
		return AssetFile{}, fmt.Errorf("资源路径为空")
	}

	ext := strings.ToLower(filepath.Ext(trimmedRel))
	if !allowedPluginAssetExts[ext] {
		return AssetFile{}, fmt.Errorf("不支持的插件资源类型: %s", ext)
	}

	pluginDir := resolvePluginAssetRoot(pluginsDir, devDirs, id)
	if pluginDir == "" {
		return AssetFile{}, os.ErrNotExist
	}

	resolvedDir, err := filepath.EvalSymlinks(pluginDir)
	if err != nil {
		return AssetFile{}, err
	}
	if err := validateRelativeEntry(resolvedDir, trimmedRel); err != nil {
		return AssetFile{}, err
	}

	fullPath := filepath.Clean(filepath.Join(resolvedDir, filepath.FromSlash(trimmedRel)))
	info, err := os.Stat(fullPath)
	if err != nil {
		return AssetFile{}, err
	}
	if info.Size() > maxPluginAssetLen {
		return AssetFile{}, fmt.Errorf("插件资源超过大小限制: %d", info.Size())
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return AssetFile{}, err
	}
	return AssetFile{
		Name:    info.Name(),
		Data:    data,
		ModTime: info.ModTime(),
	}, nil
}

func resolvePluginAssetRoot(pluginsDir string, devDirs []string, pluginID string) string {
	if strings.TrimSpace(pluginsDir) != "" {
		candidate := filepath.Join(pluginsDir, pluginID)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			if _, mfErr := os.Stat(filepath.Join(candidate, "plugin.json")); mfErr == nil {
				return candidate
			}
		}
	}

	for _, devDir := range devDirs {
		cleaned := strings.TrimSpace(devDir)
		if cleaned == "" {
			continue
		}
		_, mf, err := InspectPluginDir(cleaned, CurrentPlatform(), "")
		if err == nil && mf.ID == pluginID {
			return cleaned
		}
	}
	return ""
}
