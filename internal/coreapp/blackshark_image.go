package coreapp

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

// TransferDeviceImage sends a fixed-size BRB02 RGB565 canvas to a wired Black Shark device.
func (a *CoreApp) TransferDeviceImage(params ipc.TransferDeviceImageParams) error {
	if a.deviceManager == nil || !a.deviceManager.IsConnected() {
		return fmt.Errorf("黑鲨设备未连接")
	}
	profile := a.deviceManager.ActiveProfile()
	if profile.ID != types.BlackSharkBRB02USBProfileID ||
		types.NormalizeDeviceTransport(profile.Transport) != types.DeviceTransportUSB {
		return fmt.Errorf("黑鲨屏幕图片传输仅支持 USB 有线模式")
	}
	if !profile.Capabilities.SupportsScreenImageTransfer {
		return fmt.Errorf("当前设备不支持屏幕图片传输")
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
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	if err := a.deviceManager.SendBlackSharkImage(ctx, canvas); err != nil {
		return fmt.Errorf("黑鲨屏幕图片传输失败: %w", err)
	}
	return nil
}
