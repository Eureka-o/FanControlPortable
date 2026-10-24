//go:build !legacydevice && windows

package device

import (
	"context"
	"fmt"
	"time"
)

const (
	blackSharkImageFrameDelay  = 7 * time.Millisecond
	blackSharkImagePageDelay   = 40 * time.Millisecond
	blackSharkImageCommitDelay = 70 * time.Millisecond
	blackSharkImageACKTimeout  = 3 * time.Second

	// 官方尾部节奏 (协议 §10): 末数据块 →7ms→ 尾帧 →63.5ms→ 16×0x07 心跳(6ms 间隔)
	// →6ms→ C5 提交。无参数页快照时退化为固定 commit 停顿。
	blackSharkImageFirstBlockDelay = 5 * time.Millisecond
	blackSharkImageTailDelay       = 64 * time.Millisecond
	blackSharkImageHeartbeatGap    = 6 * time.Millisecond
	blackSharkImageTailHeartbeats  = 16

	// blackSharkImageWireLen 是图传会话帧 (C4/A4 块/尾帧/0x07 心跳/C5) 的线上包长。
	// 官方抓包与已验证的第三方实现均为 64 字节 —— 控制帧走 65 字节, 但 OUT 端点按
	// 64 字节包消费块流, 65 字节帧会多出 1 字节包打乱块解析, 设备从首块即回 0x0C。
	blackSharkImageWireLen = 64

	// blackSharkImagePayloadBytes is the 58-byte RGB565 payload of one A4 data
	// block; blackSharkFlashPageSize is the device's 4 KiB flash page.
	blackSharkImagePayloadBytes = 58
	blackSharkFlashPageSize     = 4096
)

// blackSharkImagePagePause reports whether the (index+1)-th data block crosses
// a 4 KiB flash page boundary and needs the extra ~40ms programming pause.
func blackSharkImagePagePause(index int) bool {
	start := index * blackSharkImagePayloadBytes
	return start/blackSharkFlashPageSize != (start+blackSharkImagePayloadBytes)/blackSharkFlashPageSize
}

func waitBlackSharkImagePacing(ctx context.Context, index int) error {
	delay := blackSharkImageFrameDelay
	if blackSharkImagePagePause(index) {
		delay += blackSharkImagePageDelay
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func sleepBlackSharkImage(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func blackSharkUSBImageReport(frame []byte) ([]byte, error) {
	if len(frame) == 0 || len(frame) > blackSharkImageWireLen {
		return nil, fmt.Errorf("黑鲨 USB 图传帧长度无效: %d", len(frame))
	}
	// 图传会话帧零填充到 64 字节 (官方抓包同款); 控制帧仍走 65 字节路径。
	report := make([]byte, blackSharkImageWireLen)
	copy(report, frame)
	return report, nil
}

func sendBlackSharkImageFrames(ctx context.Context, frames [][]byte, send func(index int, frame []byte) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if send == nil {
		return fmt.Errorf("黑鲨 USB 图传发送函数为空")
	}
	if len(frames) == 0 {
		return fmt.Errorf("黑鲨 USB 图传帧为空")
	}
	for index, frame := range frames {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if len(frame) == 0 || len(frame) > blackSharkImageWireLen {
			return fmt.Errorf("黑鲨 USB 图传帧 %d 长度无效: %d", index, len(frame))
		}
		if err := send(index, frame); err != nil {
			return fmt.Errorf("黑鲨 USB 图传帧 %d 发送失败: %w", index, err)
		}
		if err := waitBlackSharkImagePacing(ctx, index); err != nil {
			return err
		}
	}
	return nil
}
