//go:build !legacydevice && windows

package device

import (
	"context"
	"fmt"
	"time"
)

const (
	blackSharkImageFrameDelay = 7 * time.Millisecond
	blackSharkImagePageDelay  = 40 * time.Millisecond
)

func waitBlackSharkImagePacing(ctx context.Context, index int) error {
	if index == 0 {
		return nil
	}
	delay := blackSharkImageFrameDelay
	// A4 payloads carry 58 sequential image bytes. Pause when one crosses a 4 KiB flash page.
	if index <= 2095 {
		start := (index - 1) * 58
		if start/4096 != (start+58)/4096 {
			delay += blackSharkImagePageDelay
		}
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

func blackSharkUSBImageReport(frame []byte) ([]byte, error) {
	if len(frame) == 0 || len(frame) > blackSharkHIDReportLen {
		return nil, fmt.Errorf("黑鲨 USB 图传帧长度无效: %d", len(frame))
	}
	report := make([]byte, blackSharkHIDReportLen)
	copy(report, frame)
	return report, nil
}

func sendBlackSharkImageFrames(ctx context.Context, frames [][]byte, send func([]byte) error) error {
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
		if len(frame) == 0 || len(frame) > blackSharkHIDReportLen {
			return fmt.Errorf("黑鲨 USB 图传帧 %d 长度无效: %d", index, len(frame))
		}
		if err := send(frame); err != nil {
			return fmt.Errorf("黑鲨 USB 图传帧 %d 发送失败: %w", index, err)
		}
		if err := waitBlackSharkImagePacing(ctx, index); err != nil {
			return err
		}
	}
	return ctx.Err()
}
