//go:build !windows

package coreapp

import "fmt"

func (a *CoreApp) ReinstallPawnIO() (map[string]any, error) {
	return nil, fmt.Errorf("褰撳墠骞冲彴涓嶆敮鎸?PawnIO 閲嶈")
}

func launchGUI() error {
	return fmt.Errorf("褰撳墠骞冲彴涓嶆敮鎸佸惎鍔?GUI")
}
