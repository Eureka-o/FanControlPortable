//go:build windows

package powernotify

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmQueryEndSession = 0x0011
	wmEndSession      = 0x0016
	wmQuit            = 0x0012
	wsPopup           = 0x80000000
)

var (
	user32Session         = windows.NewLazySystemDLL("user32.dll")
	kernel32Session       = windows.NewLazySystemDLL("kernel32.dll")
	procRegisterClassExW  = user32Session.NewProc("RegisterClassExW")
	procCreateWindowExW   = user32Session.NewProc("CreateWindowExW")
	procDefWindowProcW    = user32Session.NewProc("DefWindowProcW")
	procGetMessageW       = user32Session.NewProc("GetMessageW")
	procTranslateMessage  = user32Session.NewProc("TranslateMessage")
	procDispatchMessageW  = user32Session.NewProc("DispatchMessageW")
	procPostThreadMessage = user32Session.NewProc("PostThreadMessageW")
	procDestroyWindow     = user32Session.NewProc("DestroyWindow")
	procUnregisterClassW  = user32Session.NewProc("UnregisterClassW")
	procGetModuleHandleW  = kernel32Session.NewProc("GetModuleHandleW")
)

type sessionWindowClass struct {
	size        uint32
	style       uint32
	windowProc  uintptr
	classExtra  int32
	windowExtra int32
	instance    windows.Handle
	icon        windows.Handle
	cursor      windows.Handle
	background  windows.Handle
	menuName    *uint16
	className   *uint16
	iconSmall   windows.Handle
}

type sessionWindowMessage struct {
	hwnd    windows.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	point   struct{ x, y int32 }
}

type sessionEndNotifier struct {
	threadID atomic.Uint32
	callback uintptr
	stopOnce sync.Once
}

var sessionWindowSequence atomic.Uint64

// RegisterSessionEndNotifications listens for confirmed Windows shutdown/logoff events.
func RegisterSessionEndNotifications(onEnd func()) (func(), error) {
	if onEnd == nil {
		return func() {}, nil
	}

	notifier := &sessionEndNotifier{}
	ready := make(chan error, 1)
	go notifier.run(ready, onEnd)
	if err := <-ready; err != nil {
		return nil, err
	}
	return notifier.stop, nil
}

func (n *sessionEndNotifier) run(ready chan<- error, onEnd func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	n.threadID.Store(windows.GetCurrentThreadId())

	cleanup, err := registerSessionWindow(n, onEnd)
	if err != nil {
		ready <- err
		return
	}
	defer cleanup()
	ready <- nil

	var message sessionWindowMessage
	for {
		result, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func registerSessionWindow(n *sessionEndNotifier, onEnd func()) (func(), error) {
	for _, proc := range []*windows.LazyProc{
		procRegisterClassExW, procCreateWindowExW, procDefWindowProcW,
		procGetMessageW, procTranslateMessage, procDispatchMessageW,
		procPostThreadMessage, procDestroyWindow, procUnregisterClassW,
		procGetModuleHandleW,
	} {
		if err := proc.Find(); err != nil {
			return nil, err
		}
	}

	module, _, callErr := procGetModuleHandleW.Call(0)
	if module == 0 {
		return nil, fmt.Errorf("get module handle for session notification: %w", callErr)
	}
	className, err := windows.UTF16PtrFromString(fmt.Sprintf("FanControlSessionEnd-%d-%d", os.Getpid(), sessionWindowSequence.Add(1)))
	if err != nil {
		return nil, err
	}
	empty, _ := windows.UTF16PtrFromString("")
	n.callback = windows.NewCallback(func(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
		defer func() { _ = recover() }()
		if result, handled := handleSessionEndMessage(message, wParam, onEnd); handled {
			return result
		}
		result, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
		return result
	})

	wndClass := sessionWindowClass{
		size:       uint32(unsafe.Sizeof(sessionWindowClass{})),
		windowProc: n.callback,
		instance:   windows.Handle(module),
		className:  className,
	}
	if atom, _, callErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wndClass))); atom == 0 {
		return nil, fmt.Errorf("register session notification window class: %w", callErr)
	}
	hwnd, _, callErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(empty)),
		wsPopup,
		0, 0, 0, 0,
		0, 0,
		module,
		0,
	)
	if hwnd == 0 {
		procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), module)
		return nil, fmt.Errorf("create session notification window: %w", callErr)
	}
	return func() {
		procDestroyWindow.Call(hwnd)
		procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), module)
	}, nil
}

func (n *sessionEndNotifier) stop() {
	n.stopOnce.Do(func() {
		if threadID := n.threadID.Load(); threadID != 0 {
			procPostThreadMessage.Call(uintptr(threadID), wmQuit, 0, 0)
		}
	})
}

func handleSessionEndMessage(message uint32, wParam uintptr, onEnd func()) (uintptr, bool) {
	switch message {
	case wmQueryEndSession:
		return 1, true
	case wmEndSession:
		if wParam != 0 && onEnd != nil {
			onEnd()
		}
		return 0, true
	default:
		return 0, false
	}
}
