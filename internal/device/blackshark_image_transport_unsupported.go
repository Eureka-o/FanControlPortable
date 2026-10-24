//go:build !windows || legacydevice

package device

import (
	"context"
	"fmt"
)

// SendBlackSharkImageWithProgress 在非 Windows / legacydevice 构建下不可用：
// USB 图传实现只存在于 blackshark_hid_windows.go，这里提供同签名 stub 保住构建。
func (m *Manager) SendBlackSharkImageWithProgress(ctx context.Context, rgb565 []byte, progress func(sent, total int)) error {
	return fmt.Errorf("黑鲨图传仅支持 Windows 构建")
}
