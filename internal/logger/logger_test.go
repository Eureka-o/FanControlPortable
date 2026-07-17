package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultLogDirPrefersInstallDirWhenWritable(t *testing.T) {
	installDir := t.TempDir()

	got := defaultLogDir(installDir)
	want := filepath.Join(installDir, "logs")

	if got != want {
		t.Fatalf("defaultLogDir() = %q, want %q", got, want)
	}
}

func TestDebugLogWritesOnlyWhileDebugModeEnabled(t *testing.T) {
	installDir := t.TempDir()
	log, err := NewCustomLogger(false, installDir)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	log.Info("normal-only")
	log.SetDebugMode(true)
	log.Info("debug-context")
	log.Debug("debug-detail")
	log.Close()

	path := filepath.Join(installDir, "logs", "debug_"+time.Now().Format("2006-01-02")+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read debug log: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "normal-only") {
		t.Fatal("debug log duplicated info written while debug mode was disabled")
	}
	if !strings.Contains(content, "debug-context") || !strings.Contains(content, "debug-detail") {
		t.Fatalf("debug log missing enabled context: %s", content)
	}
}

func TestDefaultLogDirUsesRelativeLogsWhenInstallDirEmpty(t *testing.T) {
	got := defaultLogDir("")
	want := "logs"

	if got != want {
		t.Fatalf("defaultLogDir() = %q, want %q", got, want)
	}
}
