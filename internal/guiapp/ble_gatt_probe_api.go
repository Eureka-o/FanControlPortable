package guiapp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/ipc"
)

func (a *App) ProbeBLEGATT(params BLEGATTProbeParams) (*BLEGATTProbeResult, error) {
	resp, err := a.sendRequestWithTimeout(ipc.ReqProbeBLEGATT, params, 30*time.Second)
	if err != nil {
		guiLogger.Warnf("probe BLE GATT failed: %v", err)
		return nil, err
	}
	if resp == nil || !resp.Success {
		errMsg := "probe BLE GATT failed"
		if resp != nil && resp.Error != "" {
			errMsg = resp.Error
		}
		return nil, fmt.Errorf("%s", errMsg)
	}
	var result BLEGATTProbeResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
