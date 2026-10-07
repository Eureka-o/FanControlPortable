//go:build !windows

package scene

import "errors"

// 情景包在非 Windows 平台上的实现：两个入口都明确报告不可用，而不是返回空糊过去。

// GetForegroundProcessName 在非 Windows 平台不可用。
// 返回明确的错误而不是空串：空串会被上层理解成"当前没有前台进程"，从而误撤销情景规则。
// 调用方应把本错误视为"本平台不支持情景"。
func GetForegroundProcessName() (string, error) {
	return "", errors.New("情景功能依赖 Windows 前台窗口 API，当前平台不支持")
}

// ListProcessNames 在非 Windows 平台不可用，返回空列表。
// 界面应据此隐藏"选择进程"入口，而不是显示一个永远为空的列表。
func ListProcessNames() []string { return nil }
