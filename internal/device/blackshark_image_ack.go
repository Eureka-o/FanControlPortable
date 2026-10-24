//go:build !legacydevice

package device

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
)

var errBlackSharkImageACKTimeout = errors.New("等待设备应答超时")

// blackSharkImageACKStream collects device responses (C4 handshake ACK, C6
// block ACKs, C5 commit echo) while an image transfer is in flight. The HID
// reader loop stays the single reader of the IN endpoint and forwards frames
// here, so the 7ms block pacing never competes with a second blocking read.
type blackSharkImageACKStream struct {
	ch chan deviceproto.BlackSharkFrame
}

func newBlackSharkImageACKStream(capacity int) *blackSharkImageACKStream {
	if capacity < 16 {
		capacity = 16
	}
	return &blackSharkImageACKStream{ch: make(chan deviceproto.BlackSharkFrame, capacity)}
}

// forward drops a frame into the stream without ever blocking the reader loop.
func (s *blackSharkImageACKStream) forward(frame deviceproto.BlackSharkFrame) {
	if s == nil || s.ch == nil {
		return
	}
	select {
	case s.ch <- frame:
	default:
	}
}

// rejection drains already-arrived ACKs without blocking and reports the first
// rejected status (e.g. 0x0C when the block pacing was violated).
func (s *blackSharkImageACKStream) rejection() error {
	if s == nil || s.ch == nil {
		return nil
	}
	for {
		select {
		case frame := <-s.ch:
			if status, ok := blackSharkImageACKStatus(frame); ok && status != 0x00 {
				return fmt.Errorf("设备拒绝图传帧 (cmd 0x%02X, status 0x%02X)", frame.Command, status)
			}
		default:
			return nil
		}
	}
}

// waitAccepted waits for a response with one of the given commands whose
// status byte is 0x00, discarding unrelated frames along the way.
func (s *blackSharkImageACKStream) waitAccepted(ctx context.Context, timeout time.Duration, cmds ...byte) error {
	if s == nil || s.ch == nil {
		return fmt.Errorf("图传应答通道未初始化")
	}
	matched := make(map[byte]bool, len(cmds))
	for _, cmd := range cmds {
		matched[cmd] = true
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("%w (期望 cmd 0x%02X)", errBlackSharkImageACKTimeout, cmds[0])
		case frame := <-s.ch:
			if !matched[frame.Command] {
				continue
			}
			status, ok := blackSharkImageACKStatus(frame)
			if !ok {
				continue
			}
			if status != 0x00 {
				return fmt.Errorf("设备返回错误状态 0x%02X (cmd 0x%02X)", status, frame.Command)
			}
			return nil
		}
	}
}

func blackSharkImageACKStatus(frame deviceproto.BlackSharkFrame) (byte, bool) {
	if len(frame.Payload) < 1 {
		return 0, false
	}
	return frame.Payload[0], true
}

// SetBlackSharkHostInfoFrame stores the latest 0x07 host-parameter frame
// built by coreapp (nil clears it). The frame is sent explicitly when the
// parameter page is selected or applied.
func (m *Manager) SetBlackSharkHostInfoFrame(frame []byte) {
	if len(frame) == 0 {
		m.blackSharkHostInfoFrame.Store(nil)
		return
	}
	stored := append([]byte(nil), frame...)
	m.blackSharkHostInfoFrame.Store(&stored)
}

// waitEcho waits for a response with the given command regardless of payload.
// The 0xC5 commit reply is a 15-byte metadata echo rather than a status frame,
// so its first payload byte is not a status code.
func (s *blackSharkImageACKStream) waitEcho(ctx context.Context, timeout time.Duration, cmd byte) error {
	if s == nil || s.ch == nil {
		return fmt.Errorf("图传应答通道未初始化")
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("%w (期望 cmd 0x%02X)", errBlackSharkImageACKTimeout, cmd)
		case frame := <-s.ch:
			if frame.Command == cmd {
				return nil
			}
		}
	}
}
