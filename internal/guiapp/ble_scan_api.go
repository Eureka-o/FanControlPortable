package guiapp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/ipc"
)

func (a *App) ScanBLEDevices(params BLEScanParams) ([]BLEDeviceInfo, error) {
	resp, err := a.sendRequestWithTimeout(ipc.ReqScanBLEDevices, params, 30*time.Second)
	if err != nil {
		guiLogger.Warnf("scan BLE devices failed: %v", err)
		return nil, err
	}
	if resp == nil || !resp.Success {
		errMsg := "scan BLE devices failed"
		if resp != nil && resp.Error != "" {
			errMsg = resp.Error
		}
		return nil, fmt.Errorf("%s", errMsg)
	}
	var devices []BLEDeviceInfo
	if err := json.Unmarshal(resp.Data, &devices); err != nil {
		return nil, err
	}
	return devices, nil
}
