package coreapp

import (
	"fmt"
	"strings"

	"github.com/TIANLI0/THRM/internal/types"
)

func (a *CoreApp) configureDeviceManager(cfg types.AppConfig) {
	if a == nil || a.deviceManager == nil {
		return
	}
	profile := types.ActiveDeviceProfile(&cfg)
	if strings.TrimSpace(profile.ID) == "" && strings.TrimSpace(profile.Transport) == "" {
		a.deviceManager.ClearProfile()
		return
	}
	a.deviceManager.ConfigureProfile(profile, cfg.FanControlDeviceIp)
}

func (a *CoreApp) reconcileDeviceManagerProfile(cfg types.AppConfig) bool {
	if a == nil || a.deviceManager == nil {
		return false
	}

	a.connectMutex.Lock()
	defer a.connectMutex.Unlock()

	managerConnected := a.deviceManager.IsConnected()
	if managerConnected {
		runtimeProfile := a.deviceManager.ActiveProfile()
		if types.IsNativeDeviceTransport(runtimeProfile.Transport) {
			return false
		}
		if deviceProfileConnectionKeyForProfile(runtimeProfile, "") == deviceProfileConnectionKey(cfg) {
			return false
		}
		a.deviceManager.DisconnectSilently()
	}

	a.configureDeviceManager(cfg)

	a.mutex.Lock()
	wasCoreConnected := a.isConnected
	if managerConnected || wasCoreConnected {
		a.isConnected = false
		a.deviceSettings = nil
	}
	a.mutex.Unlock()
	return managerConnected || wasCoreConnected
}

func deviceProfileConnectionKey(cfg types.AppConfig) string {
	types.NormalizeDeviceProfileConfig(&cfg)
	profile := types.ActiveDeviceProfile(&cfg)
	return deviceProfileConnectionKeyForProfile(profile, cfg.FanControlDeviceIp)
}

func deviceProfileConnectionKeyForProfile(profile types.DeviceProfile, fallbackEndpoint string) string {
	if strings.TrimSpace(profile.ID) == "" && strings.TrimSpace(profile.Transport) == "" {
		return ""
	}
	profile = types.NormalizeDeviceProfile(profile, fallbackEndpoint)

	parts := []string{
		strings.TrimSpace(profile.ID),
		types.NormalizeDeviceTransport(profile.Transport),
		strings.TrimSpace(profile.Connection.Endpoint),
	}
	switch profile.Transport {
	case types.DeviceTransportBLE:
		parts = append(parts,
			strings.TrimSpace(profile.Connection.BLENameFilter),
			strings.TrimSpace(profile.Connection.BLEServiceUUID),
			strings.TrimSpace(profile.Connection.BLEWriteCharacteristic),
			strings.TrimSpace(profile.Connection.BLENotifyCharacteristic),
			fmt.Sprint(profile.Connection.BLEWriteWithResponse),
		)
	case types.DeviceTransportSerial:
		parts = append(parts,
			strings.TrimSpace(profile.Connection.SerialPort),
			fmt.Sprint(profile.Connection.SerialBaudRate),
			fmt.Sprint(profile.Connection.SerialDataBits),
			fmt.Sprint(profile.Connection.SerialStopBits),
			strings.TrimSpace(profile.Connection.SerialParity),
		)
	}
	return strings.Join(parts, "\x1f")
}
