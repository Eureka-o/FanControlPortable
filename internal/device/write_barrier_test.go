package device

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestBlockWritesCancelsInFlightWiFiConnect(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		select {
		case <-requestStarted:
		default:
			close(requestStarted)
		}
		<-request.Context().Done()
	}))
	defer server.Close()

	manager := NewManager(nil)
	manager.Configure(types.DeviceTransportWiFi, server.URL)
	connectDone := make(chan struct{})
	go func() {
		manager.Connect()
		close(connectDone)
	}()

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("WiFi connect request did not start")
	}
	started := time.Now()
	manager.BlockWrites()
	if elapsed := time.Since(started); elapsed > 750*time.Millisecond {
		t.Fatalf("BlockWrites waited %v for in-flight I/O instead of canceling it", elapsed)
	}
	select {
	case <-connectDone:
	case <-time.After(time.Second):
		t.Fatal("canceled WiFi connect did not finish")
	}
}
