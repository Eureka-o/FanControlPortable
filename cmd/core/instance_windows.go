//go:build windows

package main

import (
	"errors"
	"runtime"
	"unsafe"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"golang.org/x/sys/windows"
)

var procCreateMutexW = windows.NewLazySystemDLL("kernel32.dll").NewProc("CreateMutexW")

func acquireCoreInstanceLock() (func(), bool, error) {
	name, err := windows.UTF16PtrFromString(appmeta.CoreMutexName)
	if err != nil {
		return nil, false, err
	}

	handleValue, _, callErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	runtime.KeepAlive(name)
	if handleValue == 0 {
		if callErr != windows.ERROR_SUCCESS {
			return nil, false, callErr
		}
		return nil, false, errors.New("CreateMutexW returned null handle")
	}

	handle := windows.Handle(handleValue)
	alreadyRunning := callErr == windows.ERROR_ALREADY_EXISTS
	release := func() {
		_ = windows.CloseHandle(handle)
	}

	if alreadyRunning {
		release()
		return func() {}, true, nil
	}

	return release, false, nil
}
