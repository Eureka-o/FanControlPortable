@echo off
setlocal
cd /d "%~dp0"

powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\package_portable.ps1" -CopyToRoot
if errorlevel 1 exit /b 1

endlocal
