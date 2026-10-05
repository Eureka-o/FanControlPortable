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
