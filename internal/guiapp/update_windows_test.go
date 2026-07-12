//go:build windows

package guiapp

import (
	"strings"
	"testing"
)

func TestBuildUpdateScriptShowsDownloadProgress(t *testing.T) {
	script := buildUpdateScript(
		`C:\Temp\FanControl-update\FanControl-amd64-installer.exe`,
		`C:\Temp\FanControl-update\download.progress`,
		`C:\Temp\FanControl-update\download.failed`,
		`C:\Program Files\FanControl\FanControl.exe`,
		"FanControl is updating",
		"Downloading and installing the new version",
		"Update complete, restarting FanControl",
		1234,
	)

	for _, expected := range []string{
		":waitdownload",
		`set /p "progress="<"%PROGRESS_FILE%"`,
		`if exist "%FAILED_FILE%" goto downloadfailed`,
		`if exist "%INSTALLER_FILE%" goto downloaddone`,
		`start "" "%INSTALLER_FILE%" /S`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("update script missing %q", expected)
		}
	}
}
