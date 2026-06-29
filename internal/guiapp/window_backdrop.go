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
	resolvedBlurEnabled = blurEnabledForCurrentSystem(cfg.WindowBlur, cfg.ThemeMode)

	opts := &windows.Options{
		Theme: windowsThemeForLaunch(cfg.ThemeMode),
	}
	if resolvedBlurEnabled {
		opts.WebviewIsTransparent = true
		opts.WindowIsTranslucent = true
		opts.BackdropType = windows.Acrylic
		return opts
	}
	opts.WebviewIsTransparent = false
	opts.WindowIsTranslucent = false
	opts.BackdropType = windows.None
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

func blurEnabledForCurrentSystem(mode string, themeMode string) bool {
	if !isBuiltinLaunchTheme(themeMode) {
		return false
	}
	backdropSupported := isWindowsBackdropSupported()
	switch mode {
	case types.WindowBlurOff:
		return false
	case types.WindowBlurAuto:
		return backdropSupported
	default:
		return backdropSupported
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
