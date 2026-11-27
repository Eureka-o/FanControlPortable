package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const settingsFileName = "settings.json"

// PluginDataDir returns the isolated persistent data directory <configDir>/plugins/<pluginID>.
func PluginDataDir(configDir, pluginID string) (string, error) {
	id := strings.TrimSpace(pluginID)
	if !pluginIDPattern.MatchString(id) {
		return "", fmt.Errorf("无效的插件 ID: %s", pluginID)
	}
	base := strings.TrimSpace(configDir)
	if base == "" {
		return "", fmt.Errorf("配置目录为空")
	}
	return filepath.Join(filepath.Clean(base), "plugins", id), nil
}

// LoadPluginSettings loads saved plugin settings overlaid onto manifest defaults.
func LoadPluginSettings(configDir, pluginID string, schema []SettingField) (map[string]any, error) {
	result := make(map[string]any, len(schema))
	for _, field := range schema {
		if field.Key == "" {
			continue
		}
		if field.Default != nil {
			result[field.Key] = field.Default
		}
	}

	dir, err := PluginDataDir(configDir, pluginID)
	if err != nil {
		return result, err
	}
	settingsPath := filepath.Join(dir, settingsFileName)
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return result, fmt.Errorf("读取插件设置失败: %w", err)
	}

	var stored map[string]any
	if err := json.Unmarshal(raw, &stored); err != nil {
		return result, fmt.Errorf("解析插件设置失败: %w", err)
	}
	for k, v := range stored {
		result[k] = v
	}
	return result, nil
}

// SavePluginSettings atomically writes plugin settings to <configDir>/plugins/<id>/settings.json.
func SavePluginSettings(configDir, pluginID string, values map[string]any) error {
	dir, err := PluginDataDir(configDir, pluginID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建插件数据目录失败: %w", err)
	}
	if values == nil {
		values = map[string]any{}
	}
	payload, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化插件设置失败: %w", err)
	}
	payload = append(payload, '\n')

	targetPath := filepath.Join(dir, settingsFileName)
	tempFile, err := os.CreateTemp(dir, ".settings-*.tmp")
	if err != nil {
		return fmt.Errorf("创建插件设置临时文件失败: %w", err)
	}
	tempPath := tempFile.Name()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(payload); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("写入插件设置失败: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("关闭插件设置临时文件失败: %w", err)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		return fmt.Errorf("替换插件设置失败: %w", err)
	}
	cleanupTemp = false
	return nil
}
