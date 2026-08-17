package autostart

// IsInstallAutoStartRequest distinguishes installer setup from normal autostart.
func IsInstallAutoStartRequest(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "--install-autostart", "/install-autostart", "-install-autostart":
			return true
		}
	}
	return false
}
