package autostart

import "testing"

func TestIsInstallAutoStartRequest(t *testing.T) {
	for _, arg := range []string{"--install-autostart", "/install-autostart", "-install-autostart"} {
		if !IsInstallAutoStartRequest([]string{arg}) {
			t.Fatalf("%q was not recognized", arg)
		}
	}
	for _, args := range [][]string{nil, {}, {"--autostart"}, {"--install-autostart-now"}, {"install-autostart"}} {
		if IsInstallAutoStartRequest(args) {
			t.Fatalf("%q was incorrectly recognized", args)
		}
	}
}
