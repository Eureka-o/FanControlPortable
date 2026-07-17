//go:build windows

package plugins

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func configureExternalPluginCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
