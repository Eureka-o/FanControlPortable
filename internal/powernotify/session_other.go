//go:build !windows

package powernotify

func RegisterSessionEndNotifications(func()) (func(), error) {
	return func() {}, nil
}
