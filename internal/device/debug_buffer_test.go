package device

import (
	"fmt"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestDebugFrameBufferStaysBounded(t *testing.T) {
	var seq uint64
	var frames []types.DeviceDebugFrame

	for i := 0; i < maxDebugFrames+17; i++ {
		frame := newDeviceDebugFrame("tx", types.DeviceTransportWiFi, []byte{byte(i)})
		appendBoundedDebugFrame(&seq, &frames, frame)
	}

	if len(frames) != maxDebugFrames {
		t.Fatalf("debug frame count = %d, want %d", len(frames), maxDebugFrames)
	}
	if frames[0].ID != 18 {
		t.Fatalf("first retained frame ID = %d, want 18", frames[0].ID)
	}
	if frames[len(frames)-1].ID != uint64(maxDebugFrames+17) {
		t.Fatalf("last retained frame ID = %d, want %d", frames[len(frames)-1].ID, maxDebugFrames+17)
	}
}

func TestDebugFrameBufferReusesFullBackingArray(t *testing.T) {
	var seq uint64
	frames := make([]types.DeviceDebugFrame, 0, maxDebugFrames)
	for i := 0; i < maxDebugFrames; i++ {
		appendBoundedDebugFrame(&seq, &frames, newDeviceDebugFrame("rx", types.DeviceTransportHID, []byte{byte(i)}))
	}
	firstSlot := &frames[0]
	appendBoundedDebugFrame(&seq, &frames, newDeviceDebugFrame("rx", types.DeviceTransportHID, []byte{0xFF}))
	if &frames[0] != firstSlot {
		t.Fatal("full debug buffer allocated a new backing array")
	}
	if frames[len(frames)-1].ID != seq {
		t.Fatalf("last frame ID = %d, want %d", frames[len(frames)-1].ID, seq)
	}
}

func TestWiFiDebugAttemptsAreRecordedButBounded(t *testing.T) {
	manager := NewManager(nil)

	for i := 0; i < maxDebugFrames+9; i++ {
		result, err := manager.SendDebugCommand(fmt.Sprintf("%02X", i), 0)
		if err == nil {
			t.Fatal("expected default WiFi raw debug command to remain unsupported")
		}
		if len(result.Frames) != 1 {
			t.Fatalf("attempt %d returned %d frames, want 1", i, len(result.Frames))
		}
	}

	frames := manager.GetDebugFrames()
	if len(frames) != maxDebugFrames {
		t.Fatalf("stored WiFi debug frame count = %d, want %d", len(frames), maxDebugFrames)
	}
	if frames[0].ID != 10 {
		t.Fatalf("first retained WiFi frame ID = %d, want 10", frames[0].ID)
	}
	if frames[len(frames)-1].ID != uint64(maxDebugFrames+9) {
		t.Fatalf("last retained WiFi frame ID = %d, want %d", frames[len(frames)-1].ID, maxDebugFrames+9)
	}
}
