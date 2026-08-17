//go:build windows

package main

import (
	"fmt"
	"os"

	"github.com/Eureka-o/FanControlPortable/internal/autostart"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

// runInstallAutoStart configures the task and exits without starting CoreApp.
func runInstallAutoStart() int {
	manager := autostart.NewManager(stderrLogger{})
	if err := manager.SetWindowsAutoStart(true); err != nil {
		fmt.Fprintf(os.Stderr, "配置开机自启动失败: %v\n", err)
		return 1
	}
	fmt.Fprintln(os.Stdout, "开机自启动已配置")
	return 0
}

type stderrLogger struct{}

var _ types.Logger = stderrLogger{}

func (stderrLogger) Info(format string, v ...any)  { writeLogLine("INFO", format, v...) }
func (stderrLogger) Warn(format string, v ...any)  { writeLogLine("WARN", format, v...) }
func (stderrLogger) Error(format string, v ...any) { writeLogLine("ERROR", format, v...) }
func (stderrLogger) Debug(string, ...any)          {}
func (stderrLogger) Close()                        {}
func (stderrLogger) CleanOldLogs()                 {}
func (stderrLogger) SetDebugMode(bool)             {}
func (stderrLogger) GetLogDir() string             { return "" }

func writeLogLine(level, format string, v ...any) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", level, fmt.Sprintf(format, v...))
}
