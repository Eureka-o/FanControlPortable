package coreapp

import (
	"fmt"
	"strings"

	"github.com/TIANLI0/THRM/internal/types"
)

func (a *CoreApp) configureDeviceManager(cfg types.AppConfig) {
	profile := types.ActiveDeviceProfile(&cfg)
	a.deviceManager.ConfigureProfile(profile, cfg.FanControlDeviceIp)
}

func deviceProfileConnectionKey(cfg types.AppConfig) string {
	types.NormalizeDeviceProfileConfig(&cfg)
	profile := types.ActiveDeviceProfile(&cfg)
	profile = types.NormalizeDeviceProfile(profile, cfg.FanControlDeviceIp)

	parts := []string{
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
