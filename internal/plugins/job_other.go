//go:build !windows

package plugins

import "os/exec"

type jobObjectHandle struct{}

func createKillOnCloseJob() (*jobObjectHandle, error) {
	return &jobObjectHandle{}, nil
}

func (j *jobObjectHandle) assignProcess(_ *exec.Cmd) error {
	return nil
}

func (j *jobObjectHandle) close() {}

func configureSidecarSysProcAttr(_ *exec.Cmd) {}
