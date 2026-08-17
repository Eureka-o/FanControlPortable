//go:build windows

package coreapp

import (
	"strings"
	"testing"
)

func TestBuildPawnIOReinstallScript(t *testing.T) {
	script := buildPawnIOReinstallScript(`C:\FanControl\drivers\PawnIO\PawnIO_setup.exe`, `C:\FanControl\FanControl.exe`, 4242)
	for _, want := range []string{
		`PID eq 4242`,
		`-uninstall -silent`,
		`PawnIO is not registered; continuing with installation.`,
		`-install -silent`,
		`start "" "%GUI_FILE%"`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("repair script missing %q:\n%s", want, script)
		}
	}
}
