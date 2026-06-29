//go:build !windows

package guiapp

func isWindowsBackdropSupported() bool { return false }

func isSystemAppDarkMode() bool { return false }
