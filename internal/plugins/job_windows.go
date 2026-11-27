//go:build windows

package plugins

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type jobObjectHandle struct {
	handle windows.Handle
}

func createKillOnCloseJob() (*jobObjectHandle, error) {
	h, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create job object: %w", err)
	}
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		h,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(h)
		return nil, fmt.Errorf("configure job object: %w", err)
	}
	return &jobObjectHandle{handle: h}, nil
}

func (j *jobObjectHandle) assignProcess(cmd *exec.Cmd) error {
	if j == nil || j.handle == 0 || cmd == nil || cmd.Process == nil {
		return nil
	}
	procHandle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return fmt.Errorf("open child process: %w", err)
	}
	defer windows.CloseHandle(procHandle)
	if err := windows.AssignProcessToJobObject(j.handle, procHandle); err != nil {
		return fmt.Errorf("assign process to job: %w", err)
	}
	return nil
}

func (j *jobObjectHandle) close() {
	if j == nil || j.handle == 0 {
		return
	}
	_ = windows.CloseHandle(j.handle)
	j.handle = 0
}

func configureSidecarSysProcAttr(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
}
