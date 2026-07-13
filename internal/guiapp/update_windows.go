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

func buildUpdateScript(installerPath, exePath, windowTitle, windowBody, windowRestarting string, pid int) string {
	batEsc := func(value string) string {
		value = strings.NewReplacer("^", "^^", "&", "^&", "<", "^<", ">", "^>", "|", "^|").Replace(value)
		value = strings.ReplaceAll(value, "%", "%%")
		return strings.ReplaceAll(value, "!", "")
	}

	var script strings.Builder
	writeLine := func(line string) { script.WriteString(line); script.WriteString("\r\n") }
	writeLine("@echo off")
	writeLine("setlocal enableextensions enabledelayedexpansion")
	writeLine(`set "PATH=%SystemRoot%\System32;%PATH%"`)
	writeLine("chcp 65001>nul")
	writeLine(fmt.Sprintf("title %s", batEsc(windowTitle)))
	writeLine("mode con: cols=72 lines=18>nul")
	writeLine(fmt.Sprintf(`set "INSTALLER_FILE=%s"`, batEsc(installerPath)))
	writeLine(fmt.Sprintf(`set "EXE_PATH=%s"`, batEsc(exePath)))
	writeLine("echo.")
	writeLine("echo    ========================================================")
	writeLine(fmt.Sprintf("echo      %s", batEsc(windowTitle)))
	writeLine("echo    ========================================================")
	writeLine("echo.")
	writeLine(fmt.Sprintf("echo      %s", batEsc(windowBody)))
	writeLine("echo.")
	writeLine(":waitgui")
	writeLine(fmt.Sprintf(`tasklist /FI "PID eq %d" 2>nul | find "%d" >nul`, pid, pid))
	writeLine("if not errorlevel 1 (")
	writeLine("  timeout /t 1 /nobreak >nul")
	writeLine("  goto waitgui")
	writeLine(")")
	writeLine(`start "" /wait "%INSTALLER_FILE%" /S`)
	writeLine(`set "INSTALL_EXIT=!ERRORLEVEL!"`)
	writeLine(`if not "!INSTALL_EXIT!"=="0" goto installfailed`)
	writeLine("echo.")
	writeLine("echo.")
	writeLine(fmt.Sprintf("echo      %s", batEsc(windowRestarting)))
	writeLine("timeout /t 2 /nobreak >nul")
	writeLine(`if "%EXE_PATH%"=="" goto cleanup`)
	writeLine(`set /a restart_wait=0`)
	writeLine(":waitrestart")
	writeLine(`if exist "%EXE_PATH%" goto restart`)
	writeLine(`set /a restart_wait+=1`)
	writeLine(`if !restart_wait! geq 15 goto cleanup`)
	writeLine(`timeout /t 1 /nobreak >nul`)
	writeLine(`goto waitrestart`)
	writeLine(":restart")
	writeLine(`start "" "%EXE_PATH%" --update-complete`)
	writeLine(`goto cleanup`)
	writeLine(":installfailed")
	writeLine("echo.")
	writeLine(`echo      Update installation failed with code !INSTALL_EXIT!.`)
	writeLine("timeout /t 6 /nobreak >nul")
	writeLine(`if not "%EXE_PATH%"=="" if exist "%EXE_PATH%" start "" "%EXE_PATH%"`)
	writeLine(":cleanup")
	writeLine(`del "%INSTALLER_FILE%" >nul 2>&1`)
	writeLine(`del "%~f0" >nul 2>&1`)
	writeLine(`rmdir "%~dp0" >nul 2>&1`)
	writeLine("exit")
	return script.String()
}

func launchUpdateInstaller(installerPath, exePath, windowTitle, windowBody, windowRestarting string) error {
	if _, err := os.Stat(filepath.Dir(installerPath)); err != nil {
		return fmt.Errorf("更新目录不存在: %w", err)
	}

	scriptPath := filepath.Join(filepath.Dir(installerPath), "run-update.bat")
	_ = os.Remove(scriptPath)
	script := buildUpdateScript(installerPath, exePath, windowTitle, windowBody, windowRestarting, os.Getpid())
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
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
