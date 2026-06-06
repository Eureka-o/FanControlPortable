package guiapp

import "github.com/TIANLI0/THRM/internal/deviceprofileexec"

func (a *App) ScanBLEDevices(params BLEScanParams) ([]BLEDeviceInfo, error) {
	devices, err := deviceprofileexec.ScanBLEDevices(params)
	if err != nil {
		guiLogger.Warnf("scan BLE devices failed: %v", err)
		return nil, err
	}
	return devices, nil
}
