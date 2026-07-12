//go:build windows

package guiapp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func launchUpdateInstaller(installerPath, exePath, windowTitle, windowBody, windowRestarting string) error {
	if _, err := os.Stat(installerPath); err != nil {
		return fmt.Errorf("安装包不存在: %w", err)
	}

	batEsc := func(value string) string {
		value = strings.NewReplacer("^", "^^", "&", "^&", "<", "^<", ">", "^>", "|", "^|").Replace(value)
		value = strings.ReplaceAll(value, "%", "%%")
		return strings.ReplaceAll(value, "!", "")
	}

	pid := os.Getpid()
	var script strings.Builder
	writeLine := func(line string) { script.WriteString(line); script.WriteString("\r\n") }
	writeLine("@echo off")
	writeLine("setlocal enableextensions enabledelayedexpansion")
	writeLine(`set "PATH=%SystemRoot%\System32;%PATH%"`)
	writeLine("chcp 65001>nul")
	writeLine(fmt.Sprintf("title %s", batEsc(windowTitle)))
	writeLine("mode con: cols=64 lines=16>nul")
	writeLine(`for /f %%a in ('copy /Z "%~f0" nul') do set "CR=%%a"`)
	writeLine("echo.")
	writeLine("echo    ================================================")
	writeLine(fmt.Sprintf("echo      %s", batEsc(windowTitle)))
	writeLine("echo    ================================================")
	writeLine("echo.")
	writeLine(fmt.Sprintf("echo      %s", batEsc(windowBody)))
	writeLine("echo.")
	writeLine(":waitgui")
	writeLine(fmt.Sprintf(`tasklist /FI "PID eq %d" 2>nul | find "%d" >nul`, pid, pid))
	writeLine("if not errorlevel 1 (")
	writeLine("  timeout /t 1 /nobreak >nul")
	writeLine("  goto waitgui")
	writeLine(")")
	writeLine(fmt.Sprintf(`start "" "%s" /S`, installerPath))
	writeLine(`set "frames=|/-\"`)
	writeLine("set /a sec=0")
	writeLine(`set "started="`)
	writeLine(":waitinstall")
	writeLine("set /a sec+=1")
	writeLine("set /a idx=sec %% 4")
	writeLine(`for %%j in (!idx!) do set "spin=!frames:~%%j,1!"`)
	writeLine(fmt.Sprintf(`<nul set /p "=      %s [!spin!] !sec!s   !CR!"`, batEsc(windowBody)))
	writeLine(`set "running="`)
	writeLine(fmt.Sprintf(`tasklist /FI "IMAGENAME eq %s" 2>nul | find /I "%s" >nul && set "running=1"`, updateInstallerName, updateInstallerName))
	writeLine(`if defined running set "started=1"`)
	writeLine("if not defined running if defined started goto done")
	writeLine("if not defined running if !sec! geq 90 goto done")
	writeLine("timeout /t 1 /nobreak >nul")
	writeLine("goto waitinstall")
	writeLine(":done")
	writeLine("echo.")
	writeLine("echo.")
	writeLine(fmt.Sprintf("echo      %s", batEsc(windowRestarting)))
	writeLine("timeout /t 2 /nobreak >nul")
	if exePath != "" {
		writeLine(fmt.Sprintf(`start "" "%s"`, exePath))
	}
	writeLine(`del "%~f0" >nul 2>&1`)
	writeLine("exit")

	scriptPath := filepath.Join(filepath.Dir(installerPath), "run-update.bat")
	_ = os.Remove(scriptPath)
	if err := os.WriteFile(scriptPath, []byte(script.String()), 0o644); err != nil {
		return fmt.Errorf("写入更新脚本失败: %w", err)
	}
	command := exec.Command("cmd", "/d", "/c", "start", "", "cmd", "/d", "/c", scriptPath)
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000 | syscall.CREATE_NEW_PROCESS_GROUP}
	if err := command.Start(); err != nil {
		return fmt.Errorf("启动更新安装程序失败: %w", err)
	}
	if command.Process != nil {
		_ = command.Process.Release()
	}
	return nil
}
