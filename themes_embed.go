package main

import (
	"os"
	"path/filepath"

	"github.com/Eureka-o/FanControlPortable/internal/appmeta"
	"github.com/Eureka-o/FanControlPortable/internal/theme"
)

// newThemeManager 基于当前可执行文件位置构造主题管理器。
//
// 所有官方自定义主题均通过在线主题市场（Eureka-o/FanControl-Plugins）按需安装并可自由删除，
// 不再内嵌至主程序可执行文件中；仅系统内置浅色/深色/跟随系统作为不可删除默认项。
func newThemeManager() *theme.Manager {
	// 安装目录下的 themes（与可执行文件同级）。
	installThemesDir := ""
	if exePath, err := os.Executable(); err == nil {
		installThemesDir = filepath.Join(filepath.Dir(exePath), "themes")
	}

	// 旧版本可能把主题留在用户目录。新版只读取这些位置做一次迁移，
	// 默认写入目标始终是可执行文件同级的 themes 文件夹。
	legacyThemeDirs := []string{}
	if home, err := os.UserHomeDir(); err == nil {
		configDirs := []string{appmeta.UserConfigDir(home)}
		configDirs = append(configDirs, appmeta.LegacyUserConfigDirs(home)...)
		for _, dir := range configDirs {
			legacyThemeDirs = append(legacyThemeDirs, filepath.Join(dir, "themes"))
		}
	}

	return theme.NewManager(installThemesDir, legacyThemeDirs, nil)
}
