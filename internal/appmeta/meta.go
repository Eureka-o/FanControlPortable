package appmeta

import (
	"os"
	"path/filepath"
)

const (
	AppName               = "FanControl"
	DisplayName           = "FanControl"
	DeviceModelName       = "Slim压风散热器Pro"
	DeviceTemplateName    = "FanControl WiFi 百分比控制模板"
	ExecutableName        = "FanControl.exe"
	CoreName              = "FanControl Core"
	CoreExecutableName    = "FanControl Core.exe"
	BridgeName            = "FanControl TempBridge"
	BridgeExecutableName  = "FanControl TempBridge.exe"
	LegacyAppName         = "FanControlPortable"
	LegacyExecutableName  = "FanControlPortable.exe"
	LegacyCoreName        = "FanControlPortable Core"
	LegacyCoreExecutable  = "FanControlPortable Core.exe"
	LegacyBridgeName      = "FanControlPortable TempBridge"
	LegacyBridgeExeName   = "FanControlPortable TempBridge.exe"
	IPCPipeName           = "FanControl2-IPC"
	BridgePipeName        = "FanControl2_TempBridge"
	BridgeMutexName       = `Global\FanControl2_TempBridge_Singleton`
	CoreMutexName         = `Local\FanControl2_Core_Singleton`
	PawnIOInstallerName   = "PawnIO_setup.exe"
	ConfigDirName         = ".fancontrol"
	LegacyConfigDirName   = ".fancontrolportable"
	OriginalConfigDirName = ".bs2pro-controller"
	NotificationCacheDir  = "FanControl"
	ProtocolVersion       = "3.0"
	RepositoryURL         = "https://github.com/Eureka-o/FanControlPortable"
	LatestReleaseURL      = RepositoryURL + "/releases/latest"
	GUISingleInstanceID   = "Eureka-o.FanControl.2.GUI"
)

func CoreExecutableCandidates(baseDir string) []string {
	return []string{
		filepath.Join(baseDir, CoreExecutableName),
		filepath.Join(baseDir, LegacyCoreExecutable),
	}
}

func GUIExecutableCandidates(baseDir string) []string {
	return []string{
		filepath.Join(baseDir, ExecutableName),
		filepath.Join(baseDir, LegacyExecutableName),
	}
}

func BridgeExecutableCandidates(baseDir string) []string {
	return []string{
		filepath.Join(baseDir, "bridge", BridgeExecutableName),
		filepath.Join(baseDir, "bridge", LegacyBridgeExeName),
		filepath.Join(baseDir, "..", "bridge", BridgeExecutableName),
		filepath.Join(baseDir, "..", "bridge", LegacyBridgeExeName),
		filepath.Join(baseDir, BridgeExecutableName),
		filepath.Join(baseDir, LegacyBridgeExeName),
	}
}

func PawnIOInstallerPath(baseDir string) string {
	return filepath.Join(baseDir, "drivers", "PawnIO", PawnIOInstallerName)
}

func PawnIOInstallerCandidates(baseDir string) []string {
	return []string{
		PawnIOInstallerPath(baseDir),
		filepath.Join(baseDir, PawnIOInstallerName),
	}
}

func IPCPipeCandidates() []string {
	return []string{IPCPipeName}
}

func BridgePipeCandidates() []string {
	return []string{BridgePipeName}
}

func FirstExistingPath(paths []string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func UserConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ConfigDirName)
}

func LegacyUserConfigDir(homeDir string) string {
	return filepath.Join(homeDir, LegacyConfigDirName)
}

func LegacyUserConfigDirs(homeDir string) []string {
	return []string{
		filepath.Join(homeDir, LegacyConfigDirName),
		filepath.Join(homeDir, OriginalConfigDirName),
	}
}
