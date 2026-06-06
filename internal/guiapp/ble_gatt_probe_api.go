package guiapp

import "github.com/TIANLI0/THRM/internal/deviceprofileexec"

func (a *App) ProbeBLEGATT(params BLEGATTProbeParams) (*BLEGATTProbeResult, error) {
	result, err := deviceprofileexec.ProbeBLEGATT(params)
	if err != nil {
		guiLogger.Warnf("probe BLE GATT failed: %v", err)
		return nil, err
	}
	return result, nil
}
