//go:build legacydevice || !windows

package device

import (
	"context"
	"fmt"
)

// SendBlackSharkImage is unavailable when the native Windows USB transport is not built.
func (m *Manager) SendBlackSharkImage(ctx context.Context, rgb565 []byte) error {
	return fmt.Errorf("黑鲨屏幕图片传输仅支持 Windows libusb 有线模式")
}

func (m *Manager) SendBlackSharkImageWithProgress(ctx context.Context, rgb565 []byte, progress func(stage string, completed, total int)) error {
	return fmt.Errorf("黑鲨屏幕图片传输仅支持 Windows libusb 有线模式")
}

func (m *Manager) SendBlackSharkHostInfoFrame() error {
	return fmt.Errorf("黑鲨屏幕参数传输仅支持 Windows libusb 有线模式")
}

func (m *Manager) SendBlackSharkWiFiHostInfoFrame() error {
	return fmt.Errorf("黑鲨 WiFi mock 参数传输不可用")
}

func (m *Manager) SetBlackSharkWiFiHostInfoRefreshInterval(intervalMs int) error {
	return fmt.Errorf("黑鲨 WiFi mock 参数刷新不可用")
}
