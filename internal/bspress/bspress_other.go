//go:build !windows

package bspress

// Hook 在非 Windows 平台上不可用（低层键鼠钩子是 Windows 专有 API）。
type Hook struct{}

// Start 在非 Windows 上总是返回 ErrUnsupported。
func Start(sink Sink) (*Hook, error) {
	return nil, ErrUnsupported
}

// Stop 在非 Windows 上是空操作。
func (h *Hook) Stop() {}
