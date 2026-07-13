//go:build windows

package guiapp

import "golang.org/x/sys/windows/registry"

func readUpdateSystemProxy() (bool, string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return false, "", nil
	}
	defer key.Close()

	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil {
		return false, "", nil
	}
	if enabled == 0 {
		return false, "", nil
	}
	server, _, err := key.GetStringValue("ProxyServer")
	if err != nil {
		return false, "", err
	}
	return true, server, nil
}
