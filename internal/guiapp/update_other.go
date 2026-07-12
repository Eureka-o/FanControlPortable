//go:build !windows

package guiapp

import "fmt"

func launchUpdateInstaller(_, _, _, _, _, _, _ string) error {
	return fmt.Errorf("当前平台暂不支持自动安装更新，请手动下载安装包")
}
