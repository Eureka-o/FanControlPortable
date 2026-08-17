package guiapp

import "github.com/Eureka-o/FanControlPortable/internal/deviceprofileexec"

func (a *App) ProbeBLEGATT(params BLEGATTProbeParams) (*BLEGATTProbeResult, error) {
	result, err := deviceprofileexec.ProbeBLEGATT(params)
	if err != nil {
		guiLogger.Warnf("probe BLE GATT failed: %v", err)
		return nil, err
	}
	return result, nil
}
