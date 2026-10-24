package guiapp

import (
	"fmt"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/ipc"
)

// TransferDeviceImage uploads a prepared RGB565 canvas to a connected device.
func (a *App) TransferDeviceImage(params ipc.TransferDeviceImageParams) error {
	resp, err := a.sendRequestWithTimeout(ipc.ReqTransferDeviceImage, params, 95*time.Second)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}
