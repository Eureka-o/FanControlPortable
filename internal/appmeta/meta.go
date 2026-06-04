package appmeta

import (
	"os"
	"path/filepath"
)

const (
	AppName              = "FanControlPortable"
	DisplayName          = "FanControlPortable"
	DeviceModelName      = "Slim压风散热器Pro"
	ExecutableName       = "FanControlPortable.exe"
	CoreName             = "FanControlPortable Core"
	CoreExecutableName   = "FanControlPortable Core.exe"
	BridgeName           = "FanControlPortable TempBridge"
	BridgeExecutableName = "FanControlPortable TempBridge.exe"
	IPCPipeName          = "FanControlPortable2-IPC"
	BridgePipeName       = "FanControlPortable2_TempBridge"
	BridgeMutexName      = `Global\FanControlPortable2_TempBridge_Singleton`
	CoreMutexName        = `Local\FanControlPortable2_Core_Singleton`
	PawnIOInstallerName  = "PawnIO_setup.exe"
	ConfigDirName        = ".fancontrolportable"
	LegacyConfigDirName  = ".bs2pro-controller"
	NotificationCacheDir = "FanControlPortable"
	ProtocolVersion      = "3.0"
	RepositoryURL        = "https://github.com/Eureka-o/FanControlPortable"
	LatestReleaseURL     = RepositoryURL + "/releases/latest"
	GUISingleInstanceID  = "Eureka-o.FanControlPortable.2.GUI"
)

func CoreExecutableCandidates(baseDir string) []string {
	return []string{
		filepath.Join(baseDir, CoreExecutableName),
	}
}

func GUIExecutableCandidates(baseDir string) []string {
	return []string{
		filepath.Join(baseDir, ExecutableName),
	}
}

func BridgeExecutableCandidates(baseDir string) []string {
	return []string{
		filepath.Join(baseDir, "bridge", BridgeExecutableName),
		filepath.Join(baseDir, "..", "bridge", BridgeExecutableName),
		filepath.Join(baseDir, BridgeExecutableName),
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
