//go:build !windows

package guiapp

func readUpdateSystemProxy() (bool, string, error) {
	return false, "", nil
}
