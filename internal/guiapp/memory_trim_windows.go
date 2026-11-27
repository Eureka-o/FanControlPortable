//go:build windows

package guiapp

import "golang.org/x/sys/windows"

var (
	modKernel32Trim            = windows.NewLazySystemDLL("kernel32.dll")
	procSetProcessWorkingSetSz = modKernel32Trim.NewProc("SetProcessWorkingSetSize")
)

// trimProcessWorkingSet returns cold pages to the OS when the GUI is hidden or idle.
func trimProcessWorkingSet() {
	if procSetProcessWorkingSetSz.Find() != nil {
		return
	}
	sentinel := ^uintptr(0)
	_, _, _ = procSetProcessWorkingSetSz.Call(uintptr(windows.CurrentProcess()), sentinel, sentinel)
}
