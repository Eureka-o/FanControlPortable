//go:build windows

package bridge

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestBridgeJobHelperProcess(t *testing.T) {
	if os.Getenv("FANCONTROL_BRIDGE_JOB_HELPER") != "1" {
		return
	}
	time.Sleep(30 * time.Second)
}

func TestAttachKillOnCloseJobTerminatesChildOnClose(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestBridgeJobHelperProcess$")
	cmd.Env = append(os.Environ(), "FANCONTROL_BRIDGE_JOB_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start() error = %v", err)
	}

	job, err := attachKillOnCloseJob(cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("attachKillOnCloseJob() error = %v", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	job.Close()

	select {
	case <-waitDone:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("child process did not terminate after job.Close()")
	}
}
