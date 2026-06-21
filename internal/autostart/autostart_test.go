//go:build windows

package autostart

import "testing"

func TestCommandTargetsFanControlAcceptsCurrentAndLegacyExecutables(t *testing.T) {
	cases := []string{
		`"C:\Program Files\FanControl\FanControl Core.exe" --autostart`,
		`"D:\Apps\FanControl\FanControl.exe" --autostart`,
		`"D:\Apps\FanControl\FanControlPortable Core.exe" --autostart`,
		`"D:\Apps\FanControl\FanControlPortable.exe" --autostart`,
	}

	for _, command := range cases {
		if !commandTargetsFanControl(command) {
			t.Fatalf("expected %q to target FanControl", command)
		}
	}
}

func TestCommandTargetsFanControlRejectsUnrelatedExecutables(t *testing.T) {
	cases := []string{
		`"C:\Program Files\OtherApp\OtherApp.exe" --autostart`,
		`"C:\Tools\OtherApp\FanControl.exe" --autostart`,
		`notepad.exe`,
		``,
	}

	for _, command := range cases {
		if commandTargetsFanControl(command) {
			t.Fatalf("expected %q to be rejected", command)
		}
	}
}
