//go:build windows

package plugins

import (
	"encoding/json"
	"fmt"
	goruntime "runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// InprocRunner loads and invokes an official C ABI v1 DLL inside Core.
// DLL loading only happens during Core startup; mid-session changes require a Core restart.
type InprocRunner struct {
	id      string
	dllPath string

	mutex         sync.Mutex
	dllHandle     windows.Handle
	loaded        bool
	running       bool
	state         string
	lastError     string
	procInit      uintptr
	procOnFanData uintptr
	procInvoke    uintptr
	procShutdown  uintptr
}

func NewInprocRunner(id, dllPath string) *InprocRunner {
	return &InprocRunner{
		id:      id,
		dllPath: dllPath,
		state:   StateDisabled,
	}
}

func (r *InprocRunner) LoadAndStart(configJSON []byte) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.loaded && r.running {
		return nil
	}

	flags := uintptr(windows.LOAD_WITH_ALTERED_SEARCH_PATH)
	h, err := windows.LoadLibraryEx(r.dllPath, 0, flags)
	if err != nil {
		r.state = StateFailed
		r.lastError = fmt.Sprintf("加载进程内插件 DLL 失败: %v", err)
		return fmt.Errorf("%s", r.lastError)
	}

	abiProc, err := windows.GetProcAddress(h, "fc_plugin_abi_version")
	if err != nil {
		_ = windows.FreeLibrary(h)
		r.state = StateIncompatible
		r.lastError = "缺少 fc_plugin_abi_version 导出符号"
		return fmt.Errorf("%s", r.lastError)
	}
	abiVer, _, _ := syscall.SyscallN(abiProc)
	if uint32(abiVer) != InprocABIVersion {
		_ = windows.FreeLibrary(h)
		r.state = StateIncompatible
		r.lastError = fmt.Sprintf("进程内插件 ABI 版本不兼容: %d", abiVer)
		return fmt.Errorf("%s", r.lastError)
	}

	initProc, err := windows.GetProcAddress(h, "fc_plugin_init")
	if err != nil {
		_ = windows.FreeLibrary(h)
		r.state = StateIncompatible
		r.lastError = "缺少 fc_plugin_init 导出符号"
		return fmt.Errorf("%s", r.lastError)
	}

	onFanDataProc, _ := windows.GetProcAddress(h, "fc_plugin_on_fan_data")
	invokeProc, _ := windows.GetProcAddress(h, "fc_plugin_invoke")
	shutdownProc, _ := windows.GetProcAddress(h, "fc_plugin_shutdown")

	cfgBytes := makeCString(string(configJSON))
	if len(configJSON) == 0 {
		cfgBytes = makeCString("{}")
	}
	initRet, _, _ := syscall.SyscallN(initProc, uintptr(unsafe.Pointer(&cfgBytes[0])))
	goruntime.KeepAlive(cfgBytes)
	if int32(initRet) != 0 {
		if shutdownProc != 0 {
			_, _, _ = syscall.SyscallN(shutdownProc)
		}
		_ = windows.FreeLibrary(h)
		r.state = StateFailed
		r.lastError = fmt.Sprintf("fc_plugin_init 返回错误码 %d", int32(initRet))
		return fmt.Errorf("%s", r.lastError)
	}

	r.dllHandle = h
	r.loaded = true
	r.running = true
	r.state = StateReady
	r.lastError = ""
	r.procInit = initProc
	r.procOnFanData = onFanDataProc
	r.procInvoke = invokeProc
	r.procShutdown = shutdownProc
	return nil
}

func (r *InprocRunner) Stop() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.running && r.procShutdown != 0 {
		_, _, _ = syscall.SyscallN(r.procShutdown)
	}
	r.running = false
	if r.state != StateFailed && r.state != StateIncompatible {
		r.state = StateDisabled
	}
	return nil
}

func (r *InprocRunner) OnFanData(rpm uint32, isPercent bool, connected bool) {
	r.mutex.Lock()
	running := r.running
	proc := r.procOnFanData
	r.mutex.Unlock()
	if !running || proc == 0 {
		return
	}
	var pctFlag, connFlag uintptr
	if isPercent {
		pctFlag = 1
	}
	if connected {
		connFlag = 1
	}
	_, _, _ = syscall.SyscallN(proc, uintptr(rpm), pctFlag, connFlag)
}

func (r *InprocRunner) Invoke(method string, payload json.RawMessage) (json.RawMessage, error) {
	r.mutex.Lock()
	running := r.running
	proc := r.procInvoke
	r.mutex.Unlock()

	if !running {
		return nil, fmt.Errorf("进程内插件当前未运行")
	}
	if proc == 0 {
		return nil, fmt.Errorf("该进程内插件未实现 invoke 接口")
	}

	mBytes := makeCString(strings.TrimSpace(method))
	pStr := "{}"
	if len(payload) > 0 {
		pStr = string(payload)
	}
	pBytes := makeCString(pStr)
	outBuf := make([]byte, inprocInvokeBufSize)

	written, _, _ := syscall.SyscallN(
		proc,
		uintptr(unsafe.Pointer(&mBytes[0])),
		uintptr(unsafe.Pointer(&pBytes[0])),
		uintptr(unsafe.Pointer(&outBuf[0])),
		uintptr(len(outBuf)),
	)
	goruntime.KeepAlive(mBytes)
	goruntime.KeepAlive(pBytes)
	goruntime.KeepAlive(outBuf)

	n := int32(written)
	if n < 0 {
		return nil, fmt.Errorf("进程内插件调用失败 (code=%d)", n)
	}
	if int(n) > len(outBuf) {
		n = int32(len(outBuf))
	}
	trimmed := strings.TrimSpace(string(outBuf[:n]))
	if trimmed == "" {
		return json.RawMessage(`{}`), nil
	}
	if !json.Valid([]byte(trimmed)) {
		return nil, fmt.Errorf("进程内插件返回了无效 JSON")
	}
	return json.RawMessage(trimmed), nil
}

func (r *InprocRunner) StateSnapshot() (state string, running bool, loaded bool, lastError string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.state, r.running, r.loaded, r.lastError
}

// ReleaseForTest stops and unloads the DLL handle so temporary test directories can be removed on Windows.
func (r *InprocRunner) ReleaseForTest() {
	if r == nil {
		return
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.running && r.procShutdown != 0 {
		_, _, _ = syscall.SyscallN(r.procShutdown)
	}
	r.running = false
	if r.loaded && r.dllHandle != 0 {
		_ = windows.FreeLibrary(r.dllHandle)
		r.dllHandle = 0
		r.loaded = false
	}
	r.procInit = 0
	r.procOnFanData = 0
	r.procInvoke = 0
	r.procShutdown = 0
	r.state = StateDisabled
}

func makeCString(s string) []byte {
	b := make([]byte, len(s)+1)
	copy(b, s)
	return b
}
