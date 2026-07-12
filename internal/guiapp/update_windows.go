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

func buildUpdateScript(installerPath, progressPath, failedPath, exePath, windowTitle, windowBody, windowRestarting string, pid int) string {
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
	writeLine(`for /f %%a in ('copy /Z "%~f0" nul') do set "CR=%%a"`)
	writeLine(fmt.Sprintf(`set "INSTALLER_FILE=%s"`, batEsc(installerPath)))
	writeLine(fmt.Sprintf(`set "PROGRESS_FILE=%s"`, batEsc(progressPath)))
	writeLine(fmt.Sprintf(`set "FAILED_FILE=%s"`, batEsc(failedPath)))
	writeLine(fmt.Sprintf(`set "EXE_PATH=%s"`, batEsc(exePath)))
	writeLine("echo.")
	writeLine("echo    ========================================================")
	writeLine(fmt.Sprintf("echo      %s", batEsc(windowTitle)))
	writeLine("echo    ========================================================")
	writeLine("echo.")
	writeLine(fmt.Sprintf("echo      %s", batEsc(windowBody)))
	writeLine("echo.")
	writeLine(":waitdownload")
	writeLine(`if exist "%FAILED_FILE%" goto downloadfailed`)
	writeLine(`if exist "%INSTALLER_FILE%" goto downloaddone`)
	writeLine(`set "progress=0"`)
	writeLine(`if exist "%PROGRESS_FILE%" set /p "progress="<"%PROGRESS_FILE%"`)
	writeLine(`<nul set /p "=      Downloading... !progress!%%   !CR!"`)
	writeLine("timeout /t 1 /nobreak >nul")
	writeLine("goto waitdownload")
	writeLine(":downloadfailed")
	writeLine("echo.")
	writeLine("echo.")
	writeLine(`set "failure=Update download failed"`)
	writeLine(`set /p "failure="<"%FAILED_FILE%"`)
	writeLine(`echo      !failure!`)
	writeLine("timeout /t 6 /nobreak >nul")
	writeLine("goto cleanup")
	writeLine(":downloaddone")
	writeLine("echo.")
	writeLine("echo      Download complete. Waiting for FanControl to close...")
	writeLine(":waitgui")
	writeLine(fmt.Sprintf(`tasklist /FI "PID eq %d" 2>nul | find "%d" >nul`, pid, pid))
	writeLine("if not errorlevel 1 (")
	writeLine("  timeout /t 1 /nobreak >nul")
	writeLine("  goto waitgui")
	writeLine(")")
	writeLine(`start "" "%INSTALLER_FILE%" /S`)
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
	writeLine(`if not "%EXE_PATH%"=="" start "" "%EXE_PATH%"`)
	writeLine(":cleanup")
	writeLine(`del "%PROGRESS_FILE%" >nul 2>&1`)
	writeLine(`del "%FAILED_FILE%" >nul 2>&1`)
	writeLine(`del "%~f0" >nul 2>&1`)
	writeLine("exit")
	return script.String()
}

func launchUpdateInstaller(installerPath, progressPath, failedPath, exePath, windowTitle, windowBody, windowRestarting string) error {
	if _, err := os.Stat(filepath.Dir(installerPath)); err != nil {
		return fmt.Errorf("更新目录不存在: %w", err)
	}

	scriptPath := filepath.Join(filepath.Dir(installerPath), "run-update.bat")
	_ = os.Remove(scriptPath)
	script := buildUpdateScript(installerPath, progressPath, failedPath, exePath, windowTitle, windowBody, windowRestarting, os.Getpid())
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
