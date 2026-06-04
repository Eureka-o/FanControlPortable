//go:build windows

package guiapp

import (
	"os/exec"
	"syscall"
)

func configureCoreCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x08000000,
	}
}