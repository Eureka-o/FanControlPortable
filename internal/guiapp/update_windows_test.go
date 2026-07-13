//go:build windows

package guiapp

import (
	"strings"
	"testing"
)

func TestBuildUpdateScriptWaitsForInstallAndRestartsUpdatedApp(t *testing.T) {
	script := buildUpdateScript(
		`C:\Temp\FanControl-update\FanControl-amd64-installer.exe`,
		`C:\Program Files\FanControl\FanControl.exe`,
		"FanControl is updating",
		"Downloading and installing the new version",
		"Update complete, restarting FanControl",
		1234,
	)

	for _, expected := range []string{
		`start "" /wait "%INSTALLER_FILE%" /S`,
		`if not "!INSTALL_EXIT!"=="0" goto installfailed`,
		`start "" "%EXE_PATH%" --update-complete`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("update script missing %q", expected)
		}
	}
	if strings.Contains(script, ":waitinstall") {
		t.Fatal("update script still polls the installer process instead of waiting for it")
	}
}
