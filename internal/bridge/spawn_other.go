//go:build !windows

package bridge

import "os/exec"

func configureCmdSysProcAttr(cmd *exec.Cmd) {
}

type processJob struct{}

func attachKillOnCloseJob(cmd *exec.Cmd) (*processJob, error) {
	return nil, nil
}

func (j *processJob) Close() {}
