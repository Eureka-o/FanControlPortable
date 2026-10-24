package coreapp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

type blackSharkImageTransfer struct {
	cancel context.CancelFunc
}

var blackSharkImageTransfers struct {
	sync.Mutex
	active map[*CoreApp]*blackSharkImageTransfer
}

func beginBlackSharkImageTransfer(a *CoreApp, parent context.Context) (context.Context, func(), error) {
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	transfer := &blackSharkImageTransfer{cancel: cancel}

	blackSharkImageTransfers.Lock()
	if blackSharkImageTransfers.active == nil {
		blackSharkImageTransfers.active = make(map[*CoreApp]*blackSharkImageTransfer)
	}
	if _, ok := blackSharkImageTransfers.active[a]; ok {
		blackSharkImageTransfers.Unlock()
		cancel()
		return nil, nil, fmt.Errorf("黑鲨屏幕图片传输正在进行中")
	}
	blackSharkImageTransfers.active[a] = transfer
	blackSharkImageTransfers.Unlock()

	finish := func() {
		cancel()
		blackSharkImageTransfers.Lock()
		if blackSharkImageTransfers.active[a] == transfer {
			delete(blackSharkImageTransfers.active, a)
		}
		blackSharkImageTransfers.Unlock()
	}
	return ctx, finish, nil
}

func cancelBlackSharkImageTransfer(a *CoreApp) error {
	blackSharkImageTransfers.Lock()
	transfer := blackSharkImageTransfers.active[a]
	blackSharkImageTransfers.Unlock()
	if transfer == nil {
		return fmt.Errorf("没有正在进行的黑鲨屏幕图片传输")
	}
	transfer.cancel()
	return nil
}

// CancelDeviceImage cancels the active Black Shark image transfer.
func (a *CoreApp) CancelDeviceImage() error {
	return cancelBlackSharkImageTransfer(a)
}

// TransferDeviceImage sends a fixed-size BRB02 RGB565 canvas to a wired Black Shark device
// or the explicit local WiFi preview mock.
func (a *CoreApp) TransferDeviceImage(params ipc.TransferDeviceImageParams) error {
	if a.deviceManager == nil || !a.deviceManager.IsConnected() {
		return fmt.Errorf("黑鲨设备未连接")
	}
	profile := a.deviceManager.ActiveProfile()
	transport := types.NormalizeDeviceTransport(profile.Transport)
	isWiFiMock := profile.ID == types.BlackSharkBRB02WiFiProfileID && transport == types.DeviceTransportWiFi
	if profile.ID != types.BlackSharkBRB02USBProfileID && !isWiFiMock {
		return fmt.Errorf("黑鲨屏幕图片传输仅支持 USB 有线模式或黑鲨 WiFi mock")
	}
	if !profile.Capabilities.SupportsScreenImageTransfer {
		return fmt.Errorf("当前设备不支持屏幕图片传输")
	}
	// 图片模式与参数刷新互斥，避免后台参数帧把设备画面切回参数页。
	if isWiFiMock {
		_ = a.deviceManager.SetBlackSharkWiFiHostInfoRefreshInterval(0)
	} else {
		_ = a.deviceManager.SetBlackSharkHostInfoRefreshInterval(0)
	}
	if format := strings.TrimSpace(strings.ToLower(params.Format)); format != "" && format != "rgb565-be" {
		return fmt.Errorf("不支持的图传格式 %q", params.Format)
	}
	if (params.Width != 0 && params.Width != deviceproto.BlackSharkImageWidth) ||
		(params.Height != 0 && params.Height != deviceproto.BlackSharkImageHeight) {
		return fmt.Errorf("黑鲨图传尺寸必须为 %dx%d", deviceproto.BlackSharkImageWidth, deviceproto.BlackSharkImageHeight)
	}
	encoded := strings.TrimSpace(params.DataBase64)
	if encoded == "" {
		return fmt.Errorf("屏幕图片数据为空")
	}
	canvas, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("屏幕图片数据无效: %w", err)
	}
	if len(canvas) != deviceproto.BlackSharkImageBytes {
		return fmt.Errorf("屏幕图片数据必须为 %d 字节 RGB565", deviceproto.BlackSharkImageBytes)
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, finish, err := beginBlackSharkImageTransfer(a, parent)
	if err != nil {
		return err
	}
	defer finish()
	emitProgress := func(active bool, stage string, completed, total, percent int, message string) {
		if a.ipcServer == nil {
			return
		}
		a.ipcServer.BroadcastEvent(ipc.EventDeviceImageTransferProgress, ipc.DeviceImageTransferProgress{
			Active: active, Stage: stage, Completed: completed, Total: total, Percent: percent, Error: message,
		})
	}
	send := func(progress func(stage string, completed, total int)) error {
		if isWiFiMock {
			return a.deviceManager.SendBlackSharkWiFiMockImage(ctx, canvas, progress)
		}
		return a.deviceManager.SendBlackSharkImageWithProgress(ctx, canvas, progress)
	}
	if err := send(func(stage string, completed, blockTotal int) {
		percent := 0
		if blockTotal > 0 {
			percent = completed * 100 / blockTotal
		}
		if stage == "completed" {
			percent = 100
		}
		emitProgress(true, stage, completed, blockTotal, percent, "")
	}); err != nil {
		if errors.Is(err, context.Canceled) {
			emitProgress(false, "canceled", 0, 0, 0, "")
		} else {
			emitProgress(false, "failed", 0, 0, 0, err.Error())
		}
		return fmt.Errorf("黑鲨屏幕图片传输失败: %w", err)
	}
	emitProgress(false, "completed", 1, 1, 100, "")
	return nil
}
