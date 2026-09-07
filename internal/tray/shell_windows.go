//go:build windows

package tray

import (
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32                 = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW           = modUser32.NewProc("FindWindowW")
	procFindWindowExW         = modUser32.NewProc("FindWindowExW")
	procEnumWindows           = modUser32.NewProc("EnumWindows")
	procGetClassNameW         = modUser32.NewProc("GetClassNameW")
	procGetWindowThreadProcID = modUser32.NewProc("GetWindowThreadProcessId")
	procPostMessageW          = modUser32.NewProc("PostMessageW")
	procRegisterWindowMessage = modUser32.NewProc("RegisterWindowMessageW")
)

const (
	systrayWindowClass = "SystrayClass"
	wmClose            = 0x0010
)

// postSystrayClose closes only this process's systray message window.
func postSystrayClose() bool {
	hwnd := findOwnSystrayWindow()
	if hwnd == 0 {
		return false
	}
	ret, _, _ := procPostMessageW.Call(hwnd, wmClose, 0, 0)
	return ret != 0
}

var (
	enumSystrayMu    sync.Mutex
	enumSystrayPID   uint32
	enumSystrayFound uintptr
	enumSystrayOnce  sync.Once
	enumSystrayProc  uintptr
)

func enumSystrayCallback() uintptr {
	enumSystrayOnce.Do(func() {
		enumSystrayProc = windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
			var pid uint32
			procGetWindowThreadProcID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
			if pid != enumSystrayPID || windowClassName(hwnd) != systrayWindowClass {
				return 1
			}
			enumSystrayFound = hwnd
			return 0
		})
	})
	return enumSystrayProc
}

func findOwnSystrayWindow() uintptr {
	enumSystrayMu.Lock()
	defer enumSystrayMu.Unlock()

	enumSystrayPID = windows.GetCurrentProcessId()
	enumSystrayFound = 0
	procEnumWindows.Call(enumSystrayCallback(), 0)
	return enumSystrayFound
}

func windowClassName(hwnd uintptr) string {
	buf := make([]uint16, 257)
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

// findTopWindow 查找指定类名的顶层窗口句柄，未找到返回 0。
func findTopWindow(class string) uintptr {
	classPtr, err := windows.UTF16PtrFromString(class)
	if err != nil {
		return 0
	}
	hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(classPtr)), 0)
	return hwnd
}

// findChildWindow 在父窗口下查找指定类名的子窗口句柄，未找到返回 0。
func findChildWindow(parent uintptr, class string) uintptr {
	classPtr, err := windows.UTF16PtrFromString(class)
	if err != nil {
		return 0
	}
	hwnd, _, _ := procFindWindowExW.Call(parent, 0, uintptr(unsafe.Pointer(classPtr)), 0)
	return hwnd
}

func trayNotifyWindow() uintptr {
	tray := findTopWindow("Shell_TrayWnd")
	if tray == 0 {
		return 0
	}
	return findChildWindow(tray, "TrayNotifyWnd")
}

func trayNotifyState() (uintptr, uint32) {
	hwnd := trayNotifyWindow()
	if hwnd == 0 {
		return 0, 0
	}
	var pid uint32
	procGetWindowThreadProcID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return hwnd, pid
}

// isShellReady 判断 Windows 任务栏外壳及其通知区域是否已就绪。
//
// 仅判断 Shell_TrayWnd 是不够的：开机快速启动时该窗口可能很早创建，但承载
// 通知图标的 TrayNotifyWnd 尚未就绪，此时注册托盘图标可能被静默丢弃。
func isShellReady() bool {
	hwnd, _ := trayNotifyState()
	return hwnd != 0
}

// waitForShellReady 在启动系统托盘前等待外壳就绪。
func waitForShellReady(done <-chan struct{}, timeout time.Duration) bool {
	if isShellReady() {
		return true
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return false
		case <-ticker.C:
			if isShellReady() {
				return true
			}
			if time.Now().After(deadline) {
				return true
			}
		}
	}
}

// waitForTraySettle 在自启动首次注册托盘前等待通知区域稳定。
//
// 即便 isShellReady 已返回 true，开机阶段通知区域仍可能在短时间内被重建。
// 这里要求通知区域连续稳定一小段时间后再返回；超时后仍会继续注册，避免异常环境下永不显示。
func waitForTraySettle(done <-chan struct{}, settle, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	var stableSince time.Time
	var stableTray uintptr
	var stablePID uint32
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			currentTray, currentPID := trayNotifyState()
			if currentTray != 0 {
				if !sameTrayNotifyState(currentTray, currentPID, stableTray, stablePID) {
					stableTray = currentTray
					stablePID = currentPID
					stableSince = time.Now()
				}
				if time.Since(stableSince) >= settle {
					return
				}
			} else {
				stableSince = time.Time{}
				stableTray = 0
				stablePID = 0
			}
			if time.Now().After(deadline) {
				return
			}
		}
	}
}

func sameTrayNotifyState(currentTray uintptr, currentPID uint32, stableTray uintptr, stablePID uint32) bool {
	return currentTray != 0 && currentTray == stableTray && currentPID == stablePID
}

// postTaskbarCreated 向本进程托盘窗口补发 Windows 的 TaskbarCreated 消息。
//
// fyne/systray 内部已监听该消息并会执行 Shell_NotifyIcon(NIM_ADD)。Explorer 重启时
// 系统广播可能早于 TrayNotifyWnd 稳定完成，导致首次 NIM_ADD 被静默丢弃；稳定后补发一次
// 能让托盘库按原生路径重新注册图标，而不需要重启核心服务。
func postTaskbarCreated() bool {
	namePtr, err := windows.UTF16PtrFromString("TaskbarCreated")
	if err != nil {
		return false
	}
	msg, _, _ := procRegisterWindowMessage.Call(uintptr(unsafe.Pointer(namePtr)))
	if msg == 0 {
		return false
	}
	hwnd := findOwnSystrayWindow()
	if hwnd == 0 {
		return false
	}
	ret, _, _ := procPostMessageW.Call(hwnd, msg, 0, 0)
	return ret != 0
}
