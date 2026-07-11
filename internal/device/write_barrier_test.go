package device

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestWriteBarrier(t *testing.T) {
	manager := NewManager(nil)
	manager.BlockWrites()
	if !manager.writesBlocked.Load() {
		t.Fatal("writes should be blocked during suspend")
	}
	manager.isConnected = true
	manager.deviceType = types.DeviceTransportWiFi
	if manager.SetTargetSpeed(50, types.FanSpeedUnitPercent) {
		t.Fatal("target write should be rejected during suspend")
	}
	manager.DisconnectSilently()
	if manager.IsConnected() {
		t.Fatal("write barrier must not block device disconnect")
	}
	manager.UnblockWrites()
	if manager.writesBlocked.Load() {
		t.Fatal("writes should resume after a ready connection")
	}
}
