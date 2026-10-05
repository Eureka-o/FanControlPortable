//go:build !windows || legacydevice

package device

import "fmt"

func scanBlackSharkUSBDevice() (string, error) {
	return "", fmt.Errorf("黑鲨 WinUSB 连接仅支持 Windows")
}
