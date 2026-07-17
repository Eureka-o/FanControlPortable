//go:build !windows

package plugins

import "os/exec"

func configureExternalPluginCommand(cmd *exec.Cmd) {}
