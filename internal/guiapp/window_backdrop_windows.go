//go:build windows

package guiapp

import (
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func isWindowsBackdropSupported() bool {
	defer func() { _ = recover() }()
	v := windows.RtlGetVersion()
	if v == nil {
		return false
	}
	return v.BuildNumber >= 22621
}

func isSystemAppDarkMode() bool {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return false
	}
	defer key.Close()

	value, _, err := key.GetIntegerValue("AppsUseLightTheme")
	return err == nil && value == 0
}
