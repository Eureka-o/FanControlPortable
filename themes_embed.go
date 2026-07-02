package main

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/theme"
)

// embeddedThemes 仅内嵌不含自定义字体的基础主题，作为无 themes/ 目录时的安全兜底。
//
// 带字体或大图的高级主题（shinchan、xiaoba-deluxe、dune、cyberpunk2077）由安装包/
// 便携包随 themes/ 目录发布，不内嵌以减少约 2MB 常驻 RSS。
// 字体资产通过 /theme-assets/ URL 路由由磁盘 themes/ 目录直接服务（见 main.go）。
//
// 如需新增高级主题，请将其放入源码 themes/ 目录（发布时随包发布），不必修改此处。
//
//go:embed themes/thrm themes/xiaoba themes/maodie themes/doro
var embeddedThemes embed.FS

// newThemeManager 基于当前可执行文件位置构造主题管理器。
func newThemeManager() *theme.Manager {
	// 内置主题：把 embed 根从 "themes" 下沉，使路径形如 "thrm/theme.json"。
	var builtin fs.FS
	if sub, err := fs.Sub(embeddedThemes, "themes"); err == nil {
		builtin = sub
	}

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

	return theme.NewManager(installThemesDir, legacyThemeDirs, builtin)
}
