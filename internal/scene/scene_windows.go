//go:build windows

package scene

import (
	"fmt"
	"sort"
	"strings"
	"syscall"
	"unsafe"
)

// 本文件是情景包在 Windows 上的全部平台相关实现：
// 读当前前台进程名（后台每秒采样），以及枚举可选进程名（界面「选择进程」）。

var (
	modUser32   = syscall.NewLazyDLL("user32.dll")
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")
	modPSAPI    = syscall.NewLazyDLL("psapi.dll")

	procGetForegroundWindow        = modUser32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessID   = modUser32.NewProc("GetWindowThreadProcessId")
	procOpenProcess                = modKernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = modKernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle                = modKernel32.NewProc("CloseHandle")
	procEnumProcesses              = modPSAPI.NewProc("EnumProcesses")
)

// processQueryLimitedInformation 是 PROCESS_QUERY_LIMITED_INFORMATION（Vista+）：
// 受限权限，对更高完整性级别的进程通常也能拿到路径，且不需要管理员。
// 拿不到时返回错误或空串，调用方按"本次读不到"跳过，不能当成"没有匹配"。
const processQueryLimitedInformation = 0x1000

// maxProcesses 一次最多枚举的进程数。
// 真实桌面上几百个，8192 留足余量；用固定缓冲区避免"先问大小再分配"的两段式调用。
const maxProcesses = 8192

// GetForegroundProcessName 返回当前前台窗口所属进程的可执行文件名（如 "game.exe"）。
// 用 QueryFullProcessImageNameW 而不是遍历进程列表：只需要当前前台那一个，
// 遍历整个进程表既慢又需要更多权限。
// 失败（没有前台窗口、权限不足等）返回错误；调用方视为"本次未知"跳过这一轮，
// 不要把错误当成"没有匹配"——那会让规则在异常时被误撤销。
func GetForegroundProcessName() (string, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return "", fmt.Errorf("没有前台窗口")
	}
	var pid uint32
	procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return "", fmt.Errorf("前台窗口没有关联进程")
	}

	h, _, err := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return "", fmt.Errorf("打开进程 %d 失败: %v", pid, err)
	}
	defer procCloseHandle.Call(h)

	buf := make([]uint16, syscall.MAX_LONG_PATH)
	size := uint32(len(buf))
	ok, _, err := procQueryFullProcessImageNameW.Call(h, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return "", fmt.Errorf("查询进程 %d 路径失败: %v", pid, err)
	}
	full := syscall.UTF16ToString(buf[:size])
	if i := strings.LastIndexAny(full, `\/`); i >= 0 {
		full = full[i+1:]
	}
	if full == "" {
		return "", fmt.Errorf("进程 %d 路径为空", pid)
	}
	return full, nil
}

// ListProcessNames 返回当前所有可读到的进程名（去重、升序、不带路径、去掉 .exe），
// 正是 SceneRule.Match 需要的形态，界面拿到就能直接填。
// 读不到的进程（权限不足、系统进程）静默跳过：界面只需要可选的进程，为它们报错没有意义。
func ListProcessNames() []string {
	pids := make([]uint32, maxProcesses)
	var needed uint32
	ret, _, _ := procEnumProcesses.Call(
		uintptr(unsafe.Pointer(&pids[0])),
		uintptr(maxProcesses*4),
		uintptr(unsafe.Pointer(&needed)),
	)
	if ret == 0 {
		return nil
	}
	count := int(needed) / 4
	if count > maxProcesses {
		count = maxProcesses
	}

	seen := make(map[string]struct{}, count)
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		pid := pids[i]
		if pid == 0 {
			continue
		}
		name := processNameByPID(pid)
		if name == "" {
			continue
		}
		key := NormalizeProcess(name)
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	sort.Slice(out, func(i, j int) bool {
		return NormalizeProcess(out[i]) < NormalizeProcess(out[j])
	})
	return out
}

// processNameByPID 返回可执行文件名；失败返回空串（调用方跳过）。
func processNameByPID(pid uint32) string {
	h, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseHandle.Call(h)

	buf := make([]uint16, syscall.MAX_LONG_PATH)
	size := uint32(len(buf))
	ok, _, _ := procQueryFullProcessImageNameW.Call(h, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 || size == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:size])
}
