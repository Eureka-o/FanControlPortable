//go:build !windows

package guiapp

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const relaunchAfterCoreFlag = "--relaunch-after-core"

func RunRelaunchHelper() bool {
	if len(os.Args) < 2 || os.Args[1] != relaunchAfterCoreFlag {
		return false
	}
	if len(os.Args) < 3 {
		return true
	}
	time.Sleep(500 * time.Millisecond)
	exePath, err := os.Executable()
	if err != nil {
		return true
	}
	cmd := exec.Command(exePath, os.Args[3:]...)
	if err := cmd.Start(); err != nil {
		return true
	}
	return true
}

func ScheduleGUIRestart(monitorOnly bool) error {
	args := []string{relaunchAfterCoreFlag, fmt.Sprint(os.Getpid())}
	for _, arg := range os.Args[1:] {
		if arg == relaunchAfterCoreFlag || arg == "--monitor-only-session" {
			continue
		}
		args = append(args, arg)
	}
	if monitorOnly {
		args = append(args, "--monitor-only-session")
	}
	cmd := exec.Command(os.Args[0], args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	if wailsContext != nil {
		ctx := *wailsContext
		go func() {
			time.Sleep(100 * time.Millisecond)
			runtime.Quit(ctx)
		}()
	}
	return nil
}
