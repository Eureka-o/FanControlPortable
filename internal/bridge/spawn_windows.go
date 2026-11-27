//go:build windows

package bridge

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func configureCmdSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

type processJob struct {
	handle windows.Handle
}

func attachKillOnCloseJob(cmd *exec.Cmd) (*processJob, error) {
	if cmd == nil || cmd.Process == nil {
		return nil, nil
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	proc, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	defer windows.CloseHandle(proc)
	if err := windows.AssignProcessToJobObject(job, proc); err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	return &processJob{handle: job}, nil
}

func (j *processJob) Close() {
	if j == nil || j.handle == 0 {
		return
	}
	_ = windows.CloseHandle(j.handle)
	j.handle = 0
}
