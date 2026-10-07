// Package bspress 提供 Windows 全局低层键鼠钩子 —— 黑鲨「响应」灯效的驱动。
package bspress

import "errors"

// ErrUnsupported 表示当前平台不支持（本包的实现只在 Windows 上有意义）。
var ErrUnsupported = errors.New("bspress: 仅 Windows 支持")

// Sink 每次「按下」被调用一次。
type Sink func()
