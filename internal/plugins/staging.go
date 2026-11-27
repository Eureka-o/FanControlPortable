package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

const (
	StagingDirName       = ".staging"
	PendingDeleteDirName = ".pending-delete"
)

type pendingDeleteMarker struct {
	PluginID   string `json:"pluginId"`
	RemoveData bool   `json:"removeData,omitempty"`
}

// StagePluginDir places an unpacked plugin update into <pluginsDir>/.staging/<id>
// so Core can atomically replace locked in-process DLLs on next startup.
func StagePluginDir(pluginsDir, pluginID, sourceDir string) error {
	id := strings.TrimSpace(pluginID)
	if !pluginIDPattern.MatchString(id) {
		return fmt.Errorf("无效的插件 ID: %s", pluginID)
	}
	if strings.TrimSpace(pluginsDir) == "" {
		return fmt.Errorf("插件目录未配置")
	}
	stagingRoot := filepath.Join(pluginsDir, StagingDirName)
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		return fmt.Errorf("创建暂存目录失败: %w", err)
	}

	destDir := filepath.Join(stagingRoot, id)
	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("清理旧暂存目录失败: %w", err)
	}
	if err := os.Rename(sourceDir, destDir); err != nil {
		return fmt.Errorf("移动暂存目录失败: %w", err)
	}
	// Clear any stale pending-delete marker for the same plugin when re-staging an install/update.
	_ = os.Remove(filepath.Join(pluginsDir, PendingDeleteDirName, id+".json"))
	return nil
}

// MarkPluginPendingDelete schedules an in-process plugin directory for deletion on next Core startup.
func MarkPluginPendingDelete(pluginsDir, pluginID string, removeData bool) error {
	id := strings.TrimSpace(pluginID)
	if !pluginIDPattern.MatchString(id) {
		return fmt.Errorf("无效的插件 ID: %s", pluginID)
	}
	if strings.TrimSpace(pluginsDir) == "" {
		return fmt.Errorf("插件目录未配置")
	}
	deleteRoot := filepath.Join(pluginsDir, PendingDeleteDirName)
	if err := os.MkdirAll(deleteRoot, 0o755); err != nil {
		return fmt.Errorf("创建待卸载目录失败: %w", err)
	}
	// Remove any staged update for the plugin that is about to be deleted.
	_ = os.RemoveAll(filepath.Join(pluginsDir, StagingDirName, id))

	markerBytes, err := json.Marshal(pendingDeleteMarker{
		PluginID:   id,
		RemoveData: removeData,
	})
	if err != nil {
		return err
	}
	markerPath := filepath.Join(deleteRoot, id+".json")
	return os.WriteFile(markerPath, markerBytes, 0o644)
}

// HasPendingStaging reports whether a plugin has a staged update or delete waiting for Core restart.
func HasPendingStaging(pluginsDir, pluginID string) bool {
	if strings.TrimSpace(pluginsDir) == "" || !pluginIDPattern.MatchString(pluginID) {
		return false
	}
	if info, err := os.Stat(filepath.Join(pluginsDir, StagingDirName, pluginID)); err == nil && info.IsDir() {
		return true
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, PendingDeleteDirName, pluginID+".json")); err == nil {
		return true
	}
	return false
}

// ApplyPendingStagedOps processes queued deletes and updates inside pluginsDir before any DLL is loaded.
func ApplyPendingStagedOps(pluginsDir, configDir string, logger types.Logger) error {
	if strings.TrimSpace(pluginsDir) == "" {
		return nil
	}

	var firstErr error
	pendingDeleteRoot := filepath.Join(pluginsDir, PendingDeleteDirName)
	if entries, err := os.ReadDir(pendingDeleteRoot); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
				continue
			}
			markerPath := filepath.Join(pendingDeleteRoot, entry.Name())
			raw, readErr := os.ReadFile(markerPath)
			if readErr != nil {
				continue
			}
			var marker pendingDeleteMarker
			if err := json.Unmarshal(raw, &marker); err != nil || !pluginIDPattern.MatchString(marker.PluginID) {
				_ = os.Remove(markerPath)
				continue
			}
			targetDir := filepath.Join(pluginsDir, marker.PluginID)
			if err := os.RemoveAll(targetDir); err != nil {
				if logger != nil {
					logger.Error("应用待卸载插件失败 (%s): %v", marker.PluginID, err)
				}
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if marker.RemoveData && strings.TrimSpace(configDir) != "" {
				if dataDir, err := PluginDataDir(configDir, marker.PluginID); err == nil {
					_ = os.RemoveAll(dataDir)
				}
			}
			_ = os.Remove(markerPath)
			if logger != nil {
				logger.Info("已卸载待重启清理的插件: %s", marker.PluginID)
			}
		}
	}

	stagingRoot := filepath.Join(pluginsDir, StagingDirName)
	if entries, err := os.ReadDir(stagingRoot); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pluginID := entry.Name()
			if !pluginIDPattern.MatchString(pluginID) {
				continue
			}
			stagedDir := filepath.Join(stagingRoot, pluginID)
			targetDir := filepath.Join(pluginsDir, pluginID)
			backupDir := filepath.Join(stagingRoot, "."+pluginID+".bak")
			_ = os.RemoveAll(backupDir)

			hadTarget := false
			if _, statErr := os.Stat(targetDir); statErr == nil {
				hadTarget = true
				if err := os.Rename(targetDir, backupDir); err != nil {
					if logger != nil {
						logger.Error("备份旧插件目录失败 (%s): %v", pluginID, err)
					}
					if firstErr == nil {
						firstErr = err
					}
					continue
				}
			}

			if err := os.Rename(stagedDir, targetDir); err != nil {
				if hadTarget {
					_ = os.Rename(backupDir, targetDir)
				}
				if logger != nil {
					logger.Error("应用暂存插件更新失败 (%s): %v", pluginID, err)
				}
				if firstErr == nil {
					firstErr = err
				}
				continue
			}

			_ = os.RemoveAll(backupDir)
			if logger != nil {
				logger.Info("已应用插件更新: %s", pluginID)
			}
		}
	}

	return firstErr
}
