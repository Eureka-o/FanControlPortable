package guiapp

import (
	"fmt"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/ipc"
)

// SendBlackSharkScreenParameters sends the current parameter-page snapshot once.
func (a *App) SendBlackSharkScreenParameters() error {
	resp, err := a.sendRequestWithTimeout(ipc.ReqSendBlackSharkScreenParameters, nil, 3*time.Second)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (a *App) SetBlackSharkScreenRefreshInterval(intervalMs int) error {
	resp, err := a.sendRequestWithTimeout(ipc.ReqSetBlackSharkScreenRefresh, ipc.SetBlackSharkScreenRefreshParams{IntervalMs: intervalMs}, 3*time.Second)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

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

// CancelDeviceImage cancels the active Black Shark USB image transfer.
func (a *App) CancelDeviceImage() error {
	resp, err := a.sendRequestWithTimeout(ipc.ReqCancelDeviceImage, nil, 3*time.Second)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}
