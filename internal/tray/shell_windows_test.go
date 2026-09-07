//go:build windows

package tray

import "testing"

func TestSameTrayNotifyStateRequiresStableWindowAndProcess(t *testing.T) {
	if !sameTrayNotifyState(10, 20, 10, 20) {
		t.Fatal("matching notification area state should be stable")
	}
	if sameTrayNotifyState(11, 20, 10, 20) || sameTrayNotifyState(10, 21, 10, 20) || sameTrayNotifyState(0, 0, 0, 0) {
		t.Fatal("changed or missing notification area state should reset settling")
	}
}
