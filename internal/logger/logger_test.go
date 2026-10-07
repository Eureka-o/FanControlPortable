package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

func TestDebugLogWritesOnlyDebugLevelWhileEnabled(t *testing.T) {
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
	if strings.Contains(content, "debug-context") {
		t.Fatal("debug log duplicated info written while debug mode was enabled")
	}
	if !strings.Contains(content, "debug-detail") {
		t.Fatalf("debug log missing debug entry: %s", content)
	}
}

func TestDefaultLogDirUsesRelativeLogsWhenInstallDirEmpty(t *testing.T) {
	got := defaultLogDir("")
	want := "logs"

	if got != want {
		t.Fatalf("defaultLogDir() = %q, want %q", got, want)
	}
}

// GetDirectSugar 的 caller 必须落在真正写日志的那一行上。
func TestDirectSugarAttributesCallerToTheCallSite(t *testing.T) {
	installDir := t.TempDir()
	log, err := NewPrefixedLogger(false, installDir, "guitest")
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	sugar := log.GetDirectSugar()
	probeLine := writeCallerProbe(sugar) // 真正的调用点在另一层函数里
	log.Close()

	path := filepath.Join(installDir, "logs", "guitest_"+time.Now().Format("2006-01-02")+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read gui log: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "caller-probe") {
		t.Fatalf("日志里没有探针内容: %s", content)
	}
	want := fmt.Sprintf("logger_test.go:%d", probeLine)
	if !strings.Contains(content, want) {
		t.Fatalf("caller 没有指到写日志的那一行（应含 %s，错一帧就会指到调用方那一行）: %s",
			want, content)
	}
}

// writeCallerProbe 故意把日志调用放在另一层函数里，并返回"写日志的那一行"行号。
//
// 返回的是 runtime.Caller(0) 的下一行 —— 也就是下面那句 Infof 所在的行。
func writeCallerProbe(sugar interface{ Infof(string, ...any) }) int {
	_, _, line, _ := runtime.Caller(0)
	sugar.Infof("caller-probe")
	return line + 1
}
