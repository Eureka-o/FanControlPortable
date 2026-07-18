//go:build !windows

package powernotify

// RegisterSuspendResumeNotifications 在非 Windows 平台不做处理。
func RegisterSuspendResumeNotifications(_, _ func()) (func(), error) {
	return func() {}, nil
}

func RegisterHIDInterfaceArrivalNotifications(uint16, []uint16, func(string)) (func(), error) {
	return func() {}, nil
}

func RegisterBluetoothLEInterfaceArrivalNotifications(func(string)) (func(), error) {
	return func() {}, nil
}
