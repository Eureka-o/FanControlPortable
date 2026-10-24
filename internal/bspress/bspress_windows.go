//go:build windows

package bspress

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const (
	whKeyboardLL = 13 // WH_KEYBOARD_LL
	whMouseLL    = 14 // WH_MOUSE_LL

	hcAction = 0 // 回调 nCode == HC_ACTION 时才处理

	wmKeyDown    = 0x0100
	wmSysKeyDown = 0x0104

	wmLButtonDown = 0x0201
	wmRButtonDown = 0x0204
	wmMButtonDown = 0x0207
	wmXButtonDown = 0x020B

	wmQuit = 0x0012
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
	procGetCurrentThreadId  = kernel32.NewProc("GetCurrentThreadId")
)

// 回调要转成 C 可调用的函数指针（syscall.NewCallback），只在包初始化时注册一次。
var (
	cbKeyboard = syscall.NewCallback(kbCallback)
	cbMouse    = syscall.NewCallback(msCallback)
)

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

// active 是当前生效的钩子。钩子回调是 C 调用约定、拿不到 this ⇒ 只能走包级句柄。
//
// 同一时刻只应有一个 Hook 生效：本包不阻止第二个实例，但各实例只注销自己的登记（见消息泵退出处），
// 所以即便出现重叠，也不会把对方顶成"钩子还在、却不再触发"。调用方仍须保证 Start/Stop 先后有序。
var active atomic.Pointer[Hook]

// Hook 一对已装上的低层钩子 + 它的消息泵线程。
type Hook struct {
	sink     Sink
	threadID uint32
	done     chan struct{}
	stopOnce sync.Once
	// kb/ms 是钩子句柄（卸钩用）；只在钩子线程内读写。
	kb, ms uintptr
}

// Start 起一个带消息泵的线程并装上键盘/鼠标两个低层钩子。
func Start(sink Sink) (*Hook, error) {
	if sink == nil {
		return nil, fmt.Errorf("bspress: sink 不能为空")
	}
	h := &Hook{sink: sink, done: make(chan struct{})}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		tid, _, _ := procGetCurrentThreadId.Call()
		h.threadID = uint32(tid)

		kb, _, err := procSetWindowsHookExW.Call(whKeyboardLL, cbKeyboard, 0, 0)
		if kb == 0 {
			ready <- fmt.Errorf("bspress: 装键盘钩子失败: %v", err)
			return
		}
		ms, _, err := procSetWindowsHookExW.Call(whMouseLL, cbMouse, 0, 0)
		if ms == 0 {
			procUnhookWindowsHookEx.Call(kb)
			ready <- fmt.Errorf("bspress: 装鼠标钩子失败: %v", err)
			return
		}
		h.kb, h.ms = kb, ms
		active.Store(h)
		ready <- nil

		// 消息泵：低层钩子靠它驱动回调。返回 <= 0（WM_QUIT 或出错）就退出。
		var m msg
		for {
			ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(ret) <= 0 {
				break
			}
		}
		procUnhookWindowsHookEx.Call(h.kb)
		procUnhookWindowsHookEx.Call(h.ms)
		// 只注销自己的登记：全局钩子的回调拿不到 this，包级句柄是唯一能找到 sink 的地方，
		// 若这里无条件置 nil，先起的实例退出时会把后起的实例一并顶掉（钩子还在，却不再触发）。
		active.CompareAndSwap(h, nil)
		close(h.done)
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	return h, nil
}

// Stop 卸钩并让钩子线程退出（幂等）。
func (h *Hook) Stop() {
	if h == nil {
		return
	}
	h.stopOnce.Do(func() {
		// 往那个线程投 WM_QUIT 让 GetMessageW 返回 0 —— 不能用别的线程 ID。
		procPostThreadMessageW.Call(uintptr(h.threadID), wmQuit, 0, 0)
		<-h.done
	})
}

// kbCallback 是键盘低层钩子回调：只在按键按下时触发一次 sink。
func kbCallback(nCode int32, wParam, lParam uintptr) uintptr {
	if nCode == hcAction {
		switch uint32(wParam) {
		case wmKeyDown, wmSysKeyDown:
			fire()
		}
	}
	r, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return r
}

// msCallback 是鼠标低层钩子回调：只在四个按键按下时触发一次 sink。
func msCallback(nCode int32, wParam, lParam uintptr) uintptr {
	if nCode == hcAction {
		switch uint32(wParam) {
		case wmLButtonDown, wmRButtonDown, wmMButtonDown, wmXButtonDown:
			fire()
		}
	}
	r, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return r
}

// fire 调用 sink。回调路径上只做这一件事（sink 约定非阻塞）。
func fire() {
	if h := active.Load(); h != nil {
		h.sink()
	}
}
