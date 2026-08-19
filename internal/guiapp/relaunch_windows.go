//go:build windows

package guiapp

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

const relaunchAfterCoreFlag = "--relaunch-after-core"

// RunRelaunchHelper handles the pre-Wails helper process used for a clean GUI restart.
func RunRelaunchHelper() bool {
	args := os.Args[1:]
	if len(args) < 2 || args[0] != relaunchAfterCoreFlag {
		return false
	}

	parentPID, err := strconv.ParseUint(args[1], 10, 32)
	if err != nil {
		return true
	}
	waitForProcess(uint32(parentPID))
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		if !ipc.CheckCoreServiceRunning() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	exePath, err := os.Executable()
	if err != nil {
		return true
	}
	childArgs := args[2:]
	cmd := exec.Command(exePath, childArgs...)
	configureCoreCommand(cmd)
	if err := cmd.Start(); err != nil {
		return true
	}
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	return true
}

// ScheduleGUIRestart launches a helper before closing this GUI instance. The helper
// waits for the old process and core to exit, then starts a fresh GUI instance.
func ScheduleGUIRestart(monitorOnly bool) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取 GUI 路径失败: %w", err)
	}

	childArgs := make([]string, 0, len(os.Args))
	for _, arg := range os.Args[1:] {
		if arg == relaunchAfterCoreFlag || strings.TrimSpace(arg) == strconv.Itoa(os.Getpid()) || arg == "--monitor-only-session" {
			continue
		}
		childArgs = append(childArgs, arg)
	}
	if monitorOnly {
		childArgs = append(childArgs, "--monitor-only-session")
	}

	helperArgs := []string{relaunchAfterCoreFlag, strconv.Itoa(os.Getpid())}
	helperArgs = append(helperArgs, childArgs...)
	cmd := exec.Command(exePath, helperArgs...)
	configureCoreCommand(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 GUI 重启助手失败: %w", err)
	}
	if cmd.Process != nil {
		_ = cmd.Process.Release()
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

func waitForProcess(pid uint32) {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
	if err != nil {
		return
	}
	defer windows.CloseHandle(handle)
	_, _ = windows.WaitForSingleObject(handle, 10_000)
}
