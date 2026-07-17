package guiapp

import (
	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/types"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

var resolvedBlurEnabled bool
var resolvedLaunchDarkMode bool

// ResolveWindowsOptions returns the Windows backdrop used for this app launch.
// Wails only applies this at window creation time, so changes require restart.
func ResolveWindowsOptions() *windows.Options {
	cfg := resolveLaunchAppearanceConfig()
	resolvedLaunchDarkMode = darkModeForLaunchTheme(cfg.ThemeMode)
	backdrop := resolveWindowBackdrop(cfg.WindowBlur, cfg.ThemeMode, isWindowsBackdropSupported())
	resolvedBlurEnabled = backdrop != windows.None

	opts := &windows.Options{
		Theme:        windowsThemeForLaunch(cfg.ThemeMode),
		BackdropType: backdrop,
	}
	if resolvedBlurEnabled {
		opts.WebviewIsTransparent = true
		opts.WindowIsTranslucent = true
		return opts
	}
	opts.WebviewIsTransparent = false
	opts.WindowIsTranslucent = false
	return opts
}

func WindowBackgroundColour() (r, g, b, a uint8) {
	if resolvedBlurEnabled {
		return 0, 0, 0, 0
	}
	if resolvedLaunchDarkMode {
		return 7, 9, 13, 255
	}
	return 246, 248, 252, 255
}

// WindowBlurEnabled reports the material that was actually enabled at launch.
func (a *App) WindowBlurEnabled() bool {
	return resolvedBlurEnabled
}

func resolveWindowBackdrop(mode string, themeMode string, supported bool) windows.BackdropType {
	if !supported || !isBuiltinLaunchTheme(themeMode) {
		return windows.None
	}
	switch types.NormalizeWindowBlur(mode) {
	case types.WindowBlurMica:
		return windows.Mica
	case types.WindowBlurTabbed:
		return windows.Tabbed
	case types.WindowBlurOff:
		return windows.None
	default:
		return windows.Acrylic
	}
}

func resolveLaunchAppearanceConfig() (cfg types.AppConfig) {
	cfg = types.GetDefaultConfig(false)
	defer func() {
		if recover() != nil {
			cfg = types.GetDefaultConfig(false)
		}
	}()

	manager := config.NewManager(config.GetInstallDir(), nil)
	cfg = manager.Load(false)
	cfg.ThemeMode = types.NormalizeThemeMode(cfg.ThemeMode)
	cfg.WindowBlur = types.NormalizeWindowBlur(cfg.WindowBlur)
	return cfg
}

func isBuiltinLaunchTheme(themeMode string) bool {
	switch types.NormalizeThemeMode(themeMode) {
	case types.ThemeModeSystem, types.ThemeModeLight, types.ThemeModeDark:
		return true
	default:
		return false
	}
}

func darkModeForLaunchTheme(themeMode string) bool {
	switch types.NormalizeThemeMode(themeMode) {
	case types.ThemeModeDark:
		return true
	case types.ThemeModeLight:
		return false
	case types.ThemeModeSystem:
		return isSystemAppDarkMode()
	default:
		return false
	}
}

func windowsThemeForLaunch(themeMode string) windows.Theme {
	switch types.NormalizeThemeMode(themeMode) {
	case types.ThemeModeDark:
		return windows.Dark
	case types.ThemeModeLight:
		return windows.Light
	case types.ThemeModeSystem:
		return windows.SystemDefault
	default:
		return windows.SystemDefault
	}
}
