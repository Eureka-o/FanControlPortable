@echo off
setlocal enabledelayedexpansion
echo Building FanControl...

if not "%FANCONTROL_BUILD_VERSION%"=="" (
    set "VERSION=%FANCONTROL_BUILD_VERSION%"
    echo Building version from FANCONTROL_BUILD_VERSION: !VERSION!
) else (
    REM Extract version from wails.json
    for /f "tokens=2 delims=:, " %%a in ('findstr /C:"\"productVersion\"" wails.json') do (
        set VERSION=%%a
        set VERSION=!VERSION:"=!
    )
)

if "!VERSION!"=="" (
    echo WARNING: Could not extract version from wails.json, using dev
    set VERSION=dev
) else (
    echo Building version: !VERSION!
)

set "BUILD_BIN=build\bin"
set LDFLAGS=-s -w -X github.com/TIANLI0/THRM/internal/version.BuildVersion=!VERSION! -H=windowsgui
for /f "delims=" %%G in ('go env GOPATH 2^>nul') do set "GOPATH_VALUE=%%G"
if not "!GOPATH_VALUE!"=="" (
    set "PATH=!GOPATH_VALUE!\bin;!PATH!"
)
if exist "C:\Program Files (x86)\NSIS\makensis.exe" (
    set "PATH=C:\Program Files (x86)\NSIS;!PATH!"
) else if exist "C:\Program Files\NSIS\makensis.exe" (
    set "PATH=C:\Program Files\NSIS;!PATH!"
)

if not exist "!BUILD_BIN!" mkdir "!BUILD_BIN!"

echo Cleaning stale root executables...
if exist "THRM.exe" del /q "THRM.exe"
if exist "core.exe" del /q "core.exe"

echo Cleaning stale bridge output...
if exist "!BUILD_BIN!\bridge" rmdir /s /q "!BUILD_BIN!\bridge"

echo Building temperature bridge...
dotnet publish bridge\TempBridge\TempBridge.csproj -c Release --self-contained false -o build\bin\bridge /p:Platform=x64 /p:DebugType=none /p:DebugSymbols=false /p:UseLibreHardwareMonitorProjectReference=false
if errorlevel 1 exit /b 1

REM Build core service first
echo Building core service...
go-winres make --in cmd/core/winres/winres.json --out cmd/core/rsrc
if errorlevel 1 exit /b 1
go build -trimpath -ldflags "!LDFLAGS!" -o "build/bin/FanControl Core.exe" ./cmd/core/
if errorlevel 1 exit /b 1

REM Installer icon is still file-based; system notification icon is embedded in FanControl Core.exe
if not exist "build\windows\icon.ico" (
    echo WARNING: build\windows\icon.ico not found, executable/installer icon may be incorrect
)

REM Build main application with wails
echo Building main application...
wails build -nsis -ldflags "!LDFLAGS!"
if errorlevel 1 exit /b 1

REM Ensure core service is in the bin directory for installer
if exist "build\bin\FanControl Core.exe" (
    echo Core service built successfully
) else (
    echo ERROR: Core service build failed!
    exit /b 1
)

REM Keep build/bin focused on the current build. Old versioned executables are not used by NSIS
REM and make the build output look much larger than the actual distributable payload.
echo Cleaning stale release artifacts...
for %%F in (
    "!BUILD_BIN!\THRM-v*.exe"
    "!BUILD_BIN!\THRM Core.exe"
    "!BUILD_BIN!\FanControlPortable.exe"
    "!BUILD_BIN!\FanControlPortable Core.exe"
    "!BUILD_BIN!\FanControlPortable TempBridge.exe"
    "!BUILD_BIN!\FanControlPortable-amd64-installer.exe"
    "!BUILD_BIN!\FanControlPortable-*-installer.exe"
    "!BUILD_BIN!\FanControlPortable-windows-portable.zip"
    "!BUILD_BIN!\BS2PRO-Controller-v*.exe"
    "!BUILD_BIN!\BS2PRO-Controller-*-installer.exe"
    "!BUILD_BIN!\BS2PRO-Controller-amd64-installer.zip"
    "!BUILD_BIN!\BS2PRO-Controller.exe"
    "!BUILD_BIN!\BS2PRO-Core.exe"
    "!BUILD_BIN!\BS2PRO-Watchdog.exe"
    "!BUILD_BIN!\THRM.exe"
    "!BUILD_BIN!\core.exe"
    "!BUILD_BIN!\*.exe~"
) do (
    if exist "%%~F" del /q "%%~F"
)

if exist "!BUILD_BIN!\FanControl-amd64-installer.exe" (
    copy /y "!BUILD_BIN!\FanControl-amd64-installer.exe" "!BUILD_BIN!\FanControl-!VERSION!-amd64-installer.exe" >nul
    if errorlevel 1 exit /b 1
) else (
    echo ERROR: Installer was not created. Check that NSIS makensis.exe is installed and available.
    exit /b 1
)

echo Build completed successfully with version !VERSION!
endlocal
