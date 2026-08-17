//go:build windows

package autostart

import (
	"os"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestBuildScheduledTaskXMLUsesResidentServiceSettings(t *testing.T) {
	definition := buildScheduledTaskXML(`C:\Program Files\FanControl\FanControl Core.exe`, "S-1-5-21-1-2-3-1001")
	var parsed struct {
		Settings struct {
			DisallowStartIfOnBatteries bool   `xml:"DisallowStartIfOnBatteries"`
			StopIfGoingOnBatteries     bool   `xml:"StopIfGoingOnBatteries"`
			ExecutionTimeLimit         string `xml:"ExecutionTimeLimit"`
			StartWhenAvailable         bool   `xml:"StartWhenAvailable"`
			MultipleInstancesPolicy    string `xml:"MultipleInstancesPolicy"`
			RestartOnFailure           struct {
				Interval string `xml:"Interval"`
				Count    int    `xml:"Count"`
			} `xml:"RestartOnFailure"`
		} `xml:"Settings"`
		Triggers struct {
			LogonTrigger struct {
				Enabled bool   `xml:"Enabled"`
				UserID  string `xml:"UserId"`
			} `xml:"LogonTrigger"`
		} `xml:"Triggers"`
		Principals struct {
			Principal struct {
				LogonType string `xml:"LogonType"`
				RunLevel  string `xml:"RunLevel"`
			} `xml:"Principal"`
		} `xml:"Principals"`
		Actions struct {
			Exec struct {
				Command   string `xml:"Command"`
				Arguments string `xml:"Arguments"`
			} `xml:"Exec"`
		} `xml:"Actions"`
	}
	if err := parseTaskDefinition(definition, &parsed); err != nil {
		t.Fatalf("generated task definition is invalid XML: %v", err)
	}
	if parsed.Settings.DisallowStartIfOnBatteries || parsed.Settings.StopIfGoingOnBatteries {
		t.Fatal("task must keep running on battery power")
	}
	if parsed.Settings.ExecutionTimeLimit != "PT0S" {
		t.Fatalf("ExecutionTimeLimit = %q, want PT0S", parsed.Settings.ExecutionTimeLimit)
	}
	if !parsed.Settings.StartWhenAvailable || parsed.Settings.MultipleInstancesPolicy != "IgnoreNew" {
		t.Fatalf("unexpected task availability settings: %+v", parsed.Settings)
	}
	if parsed.Settings.RestartOnFailure.Interval != "PT1M" || parsed.Settings.RestartOnFailure.Count != 3 {
		t.Fatalf("unexpected restart policy: %+v", parsed.Settings.RestartOnFailure)
	}
	if !parsed.Triggers.LogonTrigger.Enabled || parsed.Triggers.LogonTrigger.UserID != "S-1-5-21-1-2-3-1001" {
		t.Fatalf("unexpected logon trigger: %+v", parsed.Triggers.LogonTrigger)
	}
	if parsed.Principals.Principal.LogonType != "InteractiveToken" || parsed.Principals.Principal.RunLevel != "HighestAvailable" {
		t.Fatalf("unexpected principal: %+v", parsed.Principals.Principal)
	}
	if parsed.Actions.Exec.Command != `C:\Program Files\FanControl\FanControl Core.exe` || parsed.Actions.Exec.Arguments != "--autostart" {
		t.Fatalf("unexpected action: %+v", parsed.Actions.Exec)
	}
	if strings.Contains(definition, "<Delay>") {
		t.Fatal("XML task should not add a logon delay")
	}
}

func TestBuildScheduledTaskXMLHandlesMissingAccountAndEscapesPath(t *testing.T) {
	definition := buildScheduledTaskXML(`C:\Tools & Utils\core.exe`, "")
	if strings.Contains(definition, "<UserId>") || !strings.Contains(definition, "&amp;") {
		t.Fatal("missing account or XML escaping was not handled")
	}
	var parsed struct {
		Command string `xml:"Actions>Exec>Command"`
	}
	if err := parseTaskDefinition(definition, &parsed); err != nil {
		t.Fatalf("generated task definition is invalid XML: %v", err)
	}
	if parsed.Command != `C:\Tools & Utils\core.exe` {
		t.Fatalf("Command = %q", parsed.Command)
	}
}

func TestScheduledTaskNeedsUpgrade(t *testing.T) {
	current := buildScheduledTaskXML(`C:\FanControl\FanControl Core.exe`, "")
	if scheduledTaskNeedsUpgrade(current) {
		t.Fatal("current task definition should not need an upgrade")
	}
	legacy := `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <Settings>
    <DisallowStartIfOnBatteries>true</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>true</StopIfGoingOnBatteries>
    <ExecutionTimeLimit>PT72H</ExecutionTimeLimit>
  </Settings>
</Task>`
	if !scheduledTaskNeedsUpgrade(legacy) {
		t.Fatal("legacy task definition should need an upgrade")
	}
	partial := strings.ReplaceAll(strings.ReplaceAll(legacy,
		"<DisallowStartIfOnBatteries>true<", "<DisallowStartIfOnBatteries>false<"),
		"<StopIfGoingOnBatteries>true<", "<StopIfGoingOnBatteries>false<")
	if !scheduledTaskNeedsUpgrade(partial) {
		t.Fatal("legacy execution limit should need an upgrade")
	}
	if scheduledTaskNeedsUpgrade("not xml") {
		t.Fatal("invalid task XML should not trigger repeated rebuilds")
	}
}

func TestWriteTaskXMLFileUsesUTF16LEAndCleansUp(t *testing.T) {
	path, cleanup, err := writeTaskXMLFile("<Task/>")
	if err != nil {
		t.Fatalf("writeTaskXMLFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temp task XML: %v", err)
	}
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xFE {
		t.Fatalf("missing UTF-16LE BOM: %x", data)
	}
	if got := decodeSchtasksOutput(data); got != "<Task/>" {
		t.Fatalf("decoded task XML = %q", got)
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("cleanup should remove the temporary file")
	}

	source := "<Task><Settings><ExecutionTimeLimit>PT0S</ExecutionTimeLimit></Settings></Task>"
	encoded := []byte{0xFF, 0xFE}
	for _, unit := range utf16.Encode([]rune(source)) {
		encoded = append(encoded, byte(unit), byte(unit>>8))
	}
	if got := decodeSchtasksOutput(encoded); got != source {
		t.Fatalf("UTF-16 output = %q, want %q", got, source)
	}
}
