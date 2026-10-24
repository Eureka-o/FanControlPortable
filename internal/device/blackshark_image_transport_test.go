//go:build !legacydevice && windows

package device

import (
	"context"
	"errors"
	"testing"
)

func TestSendBlackSharkImageFramesStopsOnContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	count := 0
	err := sendBlackSharkImageFrames(ctx, [][]byte{{1}, {2}}, func(int, []byte) error {
		count++
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) || count != 1 {
		t.Fatalf("sendBlackSharkImageFrames() = err %v, count %d; want cancellation after one frame", err, count)
	}
}

func TestSendBlackSharkImageFramesRejectsOversizedFrame(t *testing.T) {
	frame := make([]byte, blackSharkImageWireLen+1)
	if err := sendBlackSharkImageFrames(context.Background(), [][]byte{frame}, func(int, []byte) error { return nil }); err == nil {
		t.Fatal("sendBlackSharkImageFrames() accepted oversized frame")
	}
}

func TestBlackSharkUSBImageReportPadsEveryFrameTo64Bytes(t *testing.T) {
	frame := []byte{0xA5, 0x04, 0xC5, 0x6E}
	report, err := blackSharkUSBImageReport(frame)
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != blackSharkImageWireLen {
		t.Fatalf("USB image report length = %d, want %d", len(report), blackSharkImageWireLen)
	}
	for i := range frame {
		if report[i] != frame[i] {
			t.Fatalf("USB image report byte %d = 0x%02X, want 0x%02X", i, report[i], frame[i])
		}
	}
	for i, value := range report[len(frame):] {
		if value != 0 {
			t.Fatalf("USB image report padding byte %d = 0x%02X, want zero", len(frame)+i, value)
		}
	}
}

func TestBlackSharkUSBImageReportRejectsInvalidLength(t *testing.T) {
	for _, frame := range [][]byte{nil, make([]byte, blackSharkHIDReportLen+1)} {
		if _, err := blackSharkUSBImageReport(frame); err == nil {
			t.Fatalf("blackSharkUSBImageReport(len=%d) accepted invalid frame", len(frame))
		}
	}
}

func TestWaitBlackSharkImagePacingHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitBlackSharkImagePacing(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("waitBlackSharkImagePacing() = %v, want cancellation", err)
	}
}

func TestBlackSharkImagePagePauseMatchesFlashPageBoundaries(t *testing.T) {
	// 协议 §7:全图 2095 块共 29 处页边界停顿,第一处在第 71 块(索引 70)之后。
	const blocks = 2095
	pauses := 0
	first := -1
	for index := 0; index < blocks; index++ {
		if blackSharkImagePagePause(index) {
			pauses++
			if first < 0 {
				first = index
			}
		}
	}
	if pauses != 29 {
		t.Fatalf("page pauses = %d, want 29", pauses)
	}
	if first != 70 {
		t.Fatalf("first page pause after block %d, want block 71", first+1)
	}
	if blackSharkImagePagePause(0) {
		t.Fatal("first block must not pause")
	}
}
