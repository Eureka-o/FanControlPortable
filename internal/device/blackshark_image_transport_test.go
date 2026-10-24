//go:build !legacydevice && windows

package device

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
)

// 图传帧序列的测试。节拍由设备回执驱动，所以 sendBlackSharkImageFrames 带一个 waitFlow 参数；
// 下面同时覆盖闭环重发、重发上限、以及"末块可以没有应答 / 中途必须应答"。

func TestSendBlackSharkImageFramesStopsOnContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	count := 0
	err := sendBlackSharkImageFrames(ctx, [][]byte{{1}, {2}}, func([]byte) error {
		count++
		cancel()
		return nil
	}, nil, nil)
	if !errors.Is(err, context.Canceled) || count != 1 {
		t.Fatalf("sendBlackSharkImageFrames() = err %v, count %d; want cancellation after one frame", err, count)
	}
}

func TestSendBlackSharkImageFramesCallsHandshakeOnceAfterFirstFrame(t *testing.T) {
	// afterHandshake 只在该序列的 C4 信息帧之后调一次（序列首帧是开场帧 0x03，故是第 2 帧）；
	// 它返回错误必须中止后续帧。
	sent, handshakes := 0, 0
	frames := [][]byte{{1}, {2}, {3}}
	if err := sendBlackSharkImageFrames(context.Background(), frames, func([]byte) error {
		sent++
		return nil
	}, func() error {
		if sent != 1 {
			t.Fatalf("handshake ran after %d frames, want exactly 1 (C4 必须先发出)", sent)
		}
		handshakes++
		return nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	if handshakes != 1 || sent != 3 {
		t.Fatalf("handshakes=%d sent=%d, want 1/3", handshakes, sent)
	}
}

func TestSendBlackSharkImageFramesAbortsWhenHandshakeFails(t *testing.T) {
	// 拿不到 C4 应答就不能继续灌数据（否则每块都会被设备回 0x0C）。
	sent := 0
	err := sendBlackSharkImageFrames(context.Background(), [][]byte{{1}, {2}, {3}}, func([]byte) error {
		sent++
		return nil
	}, func() error {
		return errors.New("no C4 ack")
	}, nil)
	if err == nil || sent != 1 {
		t.Fatalf("sendBlackSharkImageFrames() = err %v, sent %d; want abort after the first frame", err, sent)
	}
}

func TestSendBlackSharkImageFramesRejectsOversizedFrame(t *testing.T) {
	frame := make([]byte, blackSharkHIDReportLen+1)
	if err := sendBlackSharkImageFrames(context.Background(), [][]byte{frame}, func([]byte) error { return nil }, nil, nil); err == nil {
		t.Fatal("sendBlackSharkImageFrames() accepted oversized frame")
	}
}

func TestWaitBlackSharkImageDelayHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitBlackSharkImageDelay(ctx, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("waitBlackSharkImageDelay() = %v, want cancellation", err)
	}
	if err := waitBlackSharkImageDelay(context.Background(), 0); err != nil {
		t.Fatalf("waitBlackSharkImageDelay(0) = %v, want nil", err)
	}
}

// ---- 闭环节拍与重发 ----

func imageTestBlock(n byte) []byte {
	b := make([]byte, 64)
	b[0] = deviceproto.BlackSharkCmdImageBlock // 0xA4 ⇒ 受流控
	b[1] = 0x40
	b[2] = n
	return b
}

func imageTestFrames(dataBlocks int) [][]byte {
	frames := [][]byte{{0xA5, 0x0F, 0xC4, 0x00}} // C4 信息帧：0xA5 族，不受流控
	for i := 1; i <= dataBlocks; i++ {
		frames = append(frames, imageTestBlock(byte(i)))
	}
	return append(frames, []byte{0xA5, 0x04, 0xC5, 0x6E}) // C5 提交帧
}

func imageTestStatusWaiter(statuses ...byte) (blackSharkImageFlowWaiter, func() int) {
	index := 0
	return func(context.Context, time.Duration) (byte, bool, error) {
		if index >= len(statuses) {
			return 0, false, nil
		}
		status := statuses[index]
		index++
		return status, true, nil
	}, func() int { return index }
}

// TestSendBlackSharkImageFramesResendsAfterMissedAck 是本轮真机缺陷的回归钉子。
//
// 真机上量到过（USB）：一轮 2098 帧里有**一处**停 >2s 不回执，位置随机
// （990 / 1626 / 1696 / 2049 块，全是**中途**块），其余 2000+ 块都是 ~7ms 就回。
// 旧实现单次漏回执就 `return err` 整轮失败，用户侧观感正是"本地图上传稳定失败"。
// 新行为的判据：**中途漏一次回执要重发该块并继续，最终成功**。
//
// 必须用 ≥3 个数据块：末块等不到应答是**设计允许**的（设备对第 k 块的应答常由第 k+1 块触发），
// 所以要把"漏回执"落在中间那一块上才测得到重发。第一次写这条测试时用了 2 块，
// 结果末块走了"容忍"分支、断言不成立 —— 是这个豁免把我自己的测试绊了一下。
func TestSendBlackSharkImageFramesResendsAfterMissedAck(t *testing.T) {
	// 块 1 收下；块 2 第一次等待"超时"（ok=false, err=nil），重发后收下；块 3 收下。
	call := 0
	waiter := func(context.Context, time.Duration) (byte, bool, error) {
		call++
		switch call {
		case 2:
			return 0, false, nil // 漏回执：必须重发而不是整轮失败
		default:
			return deviceproto.BlackSharkImageFlowAccepted, true, nil
		}
	}
	blocksSent := 0
	err := sendBlackSharkImageFrames(context.Background(), imageTestFrames(3), func(frame []byte) error {
		if len(frame) > 0 && frame[0] == deviceproto.BlackSharkCmdImageBlock {
			blocksSent++
		}
		return nil
	}, nil, waiter)
	if err != nil {
		t.Fatalf("中途漏一次回执后应重发并跑完，却返回: %v", err)
	}
	if blocksSent != 4 {
		t.Fatalf("数据块被发了 %d 次，want 4（3 块 + 块 2 重发一次）", blocksSent)
	}
}

// TestSendBlackSharkImageFramesReportsCancellationSeparately 取消必须与"设备不回"分开报。
//
// 两者过去都被写成"第 N 块等 0xC6 超时"，于是"主动不发了"被误报成"设备不回" ——
// 曾有 5 次 USB 失败没法从日志上分清是哪种。
func TestSendBlackSharkImageFramesReportsCancellationSeparately(t *testing.T) {
	cancelled := errors.New("context canceled (test)")
	waiter := func(context.Context, time.Duration) (byte, bool, error) { return 0, false, cancelled }
	err := sendBlackSharkImageFrames(context.Background(), imageTestFrames(2), func([]byte) error { return nil }, nil, waiter)
	if err == nil {
		t.Fatal("等待被中止时应返回错误")
	}
	if !errors.Is(err, cancelled) {
		t.Fatalf("取消原因没被透传，err = %v", err)
	}
	if !strings.Contains(err.Error(), "取消") {
		t.Fatalf("错误文案没说明是「取消」而不是「超时」: %v", err)
	}
}

func TestSendBlackSharkImageFramesResendsRejectedBlock(t *testing.T) {
	// 第 1 块收下、第 2 块先被拒（0x0C）再收下 ⇒ 第 2 块的帧必须被发两次。
	waiter, consumed := imageTestStatusWaiter(
		deviceproto.BlackSharkImageFlowAccepted,
		deviceproto.BlackSharkImageFlowRejected,
		deviceproto.BlackSharkImageFlowAccepted,
	)
	blocksSent := 0
	err := sendBlackSharkImageFrames(context.Background(), imageTestFrames(2), func(frame []byte) error {
		if len(frame) > 0 && frame[0] == deviceproto.BlackSharkCmdImageBlock {
			blocksSent++
		}
		return nil
	}, nil, waiter)
	if err != nil {
		t.Fatalf("sendBlackSharkImageFrames() = %v, want nil (the rejected block must be resent)", err)
	}
	if blocksSent != 3 {
		t.Fatalf("data blocks sent = %d, want 3 (2 blocks, one of them resent once)", blocksSent)
	}
	if got := consumed(); got != 3 {
		t.Fatalf("flow statuses consumed = %d, want 3", got)
	}
}

func TestSendBlackSharkImageFramesFailsAfterResendExhausted(t *testing.T) {
	// 一直拒收 ⇒ 重发到上限后整体失败（重发上限必须真的生效，不许无限重发）。
	alwaysReject := func(context.Context, time.Duration) (byte, bool, error) {
		return deviceproto.BlackSharkImageFlowRejected, true, nil
	}
	blocksSent := 0
	err := sendBlackSharkImageFrames(context.Background(), imageTestFrames(1), func(frame []byte) error {
		if len(frame) > 0 && frame[0] == deviceproto.BlackSharkCmdImageBlock {
			blocksSent++
		}
		return nil
	}, nil, alwaysReject)
	if err == nil {
		t.Fatal("sendBlackSharkImageFrames() succeeded although every block was rejected")
	}
	// 首次发送 + blackSharkImageFlowMaxResend 次重发。
	if want := blackSharkImageFlowMaxResend + 1; blocksSent != want {
		t.Fatalf("data blocks sent = %d, want %d (initial + max resends)", blocksSent, want)
	}
}

// TestSendBlackSharkImageFramesFailsAfterTimeoutResendExhausted 漏回执的重发上限同样必须生效：
// 否则设备真的卡死时会无限重发，整轮永不结束。
//
// 同样必须用 ≥3 块（把"永远漏回执"落在中间那一块上），否则会走到末块的容忍分支。
func TestSendBlackSharkImageFramesFailsAfterTimeoutResendExhausted(t *testing.T) {
	call := 0
	// 块 1 收下（让它推进到块 2），之后永远超时。
	waiter := func(context.Context, time.Duration) (byte, bool, error) {
		call++
		if call == 1 {
			return deviceproto.BlackSharkImageFlowAccepted, true, nil
		}
		return 0, false, nil
	}
	blocksSent := 0
	err := sendBlackSharkImageFrames(context.Background(), imageTestFrames(3), func(frame []byte) error {
		if len(frame) > 0 && frame[0] == deviceproto.BlackSharkCmdImageBlock {
			blocksSent++
		}
		return nil
	}, nil, waiter)
	if err == nil {
		t.Fatal("一直漏回执时必须最终失败，不能无限重发")
	}
	if !strings.Contains(err.Error(), "重发") {
		t.Fatalf("失败原因应说明重发耗尽: %v", err)
	}
	// 块 1 一次 + 块 2（首次 + 上限次重发）。
	if want := 1 + 1 + blackSharkImageFlowMaxResend; blocksSent != want {
		t.Fatalf("data blocks sent = %d, want %d", blocksSent, want)
	}
}

func TestSendBlackSharkImageFramesToleratesUnansweredLastBlock(t *testing.T) {
	// 设备对第 k 块的应答常由第 k+1 块触发（1:1 但错位一格）⇒ 最后一块等不到应答属正常。
	neverAnswers := func(context.Context, time.Duration) (byte, bool, error) { return 0, false, nil }
	if err := sendBlackSharkImageFrames(context.Background(), imageTestFrames(1), func([]byte) error { return nil }, nil, neverAnswers); err != nil {
		t.Fatalf("sendBlackSharkImageFrames() = %v, want nil: the last data block may stay unanswered", err)
	}
	// 但**中途**一直等不到应答仍然失败 —— 重发耗尽是硬失败（不许无限重发）。
	if err := sendBlackSharkImageFrames(context.Background(), imageTestFrames(2), func([]byte) error { return nil }, nil, neverAnswers); err == nil {
		t.Fatal("sendBlackSharkImageFrames() succeeded although a mid-sequence block was never acknowledged")
	}
}

func TestBlackSharkUSBImageReportPadsEveryFrameTo65Bytes(t *testing.T) {
	frame := []byte{0xA5, 0x04, 0xC5, 0x6E}
	report, err := blackSharkUSBImageReport(frame)
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 65 {
		t.Fatalf("USB image report length = %d, want 65", len(report))
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
