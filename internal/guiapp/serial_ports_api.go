package guiapp

import (
	"encoding/json"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/ipc"
)

func (a *App) ListSerialPorts() []SerialPortInfo {
	resp, err := a.sendRequestWithTimeout(ipc.ReqListSerialPorts, nil, 10*time.Second)
	if err != nil || resp == nil || !resp.Success {
		if err != nil {
			guiLogger.Warnf("list serial ports failed: %v", err)
		}
		return []SerialPortInfo{}
	}
	var ports []SerialPortInfo
	if err := json.Unmarshal(resp.Data, &ports); err != nil {
		guiLogger.Warnf("decode serial ports failed: %v", err)
		return []SerialPortInfo{}
	}
	return ports
}
