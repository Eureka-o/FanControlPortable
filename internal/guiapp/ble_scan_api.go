package guiapp

import "github.com/Eureka-o/FanControlPortable/internal/deviceprofileexec"

func (a *App) ScanBLEDevices(params BLEScanParams) ([]BLEDeviceInfo, error) {
	devices, err := deviceprofileexec.ScanBLEDevices(params)
	if err != nil {
		guiLogger.Warnf("scan BLE devices failed: %v", err)
		return nil, err
	}
	return devices, nil
}
