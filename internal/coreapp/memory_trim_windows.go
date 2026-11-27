//go:build windows

package coreapp

import "golang.org/x/sys/windows"

var (
	modKernel32Trim            = windows.NewLazySystemDLL("kernel32.dll")
	procSetProcessWorkingSetSz = modKernel32Trim.NewProc("SetProcessWorkingSetSize")
)

// trimProcessWorkingSet 归还启动期不再访问的冷页，配合 debug.FreeOSMemory 降低后台物理内存占用。
func trimProcessWorkingSet() {
	if procSetProcessWorkingSetSz.Find() != nil {
		return
	}
	sentinel := ^uintptr(0)
	_, _, _ = procSetProcessWorkingSetSz.Call(uintptr(windows.CurrentProcess()), sentinel, sentinel)
}
