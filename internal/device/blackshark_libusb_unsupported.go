//go:build !windows || legacydevice

package device

import "fmt"

func scanBlackSharkUSBDevice() (string, error) {
	return "", fmt.Errorf("黑鲨 WinUSB 连接仅支持 Windows")
}

// blackSharkPairedBLEAddress 在非 Windows 构建里恒为空：配对表是 Windows 的实现细节，
// 其它平台没有对应物。调用方据此回落到名称扫描。
func blackSharkPairedBLEAddress(string) string { return "" }
