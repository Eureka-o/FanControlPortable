//go:build !windows

package plugins

import (
	"encoding/json"
	"fmt"
)

type InprocRunner struct {
	id      string
	dllPath string
}

func NewInprocRunner(id, dllPath string) *InprocRunner {
	return &InprocRunner{id: id, dllPath: dllPath}
}

func (r *InprocRunner) LoadAndStart(_ []byte) error {
	return fmt.Errorf("当前平台不支持 Windows 进程内 DLL 插件")
}

func (r *InprocRunner) Stop() error {
	return nil
}

func (r *InprocRunner) OnFanData(_ uint32, _ bool, _ bool) {}

func (r *InprocRunner) Invoke(_ string, _ json.RawMessage) (json.RawMessage, error) {
	return nil, fmt.Errorf("当前平台不支持 Windows 进程内 DLL 插件")
}

func (r *InprocRunner) StateSnapshot() (state string, running bool, loaded bool, lastError string) {
	return StateUnsupported, false, false, "当前平台不支持 Windows 进程内 DLL 插件"
}
