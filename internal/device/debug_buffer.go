package device

import (
	"fmt"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

const maxDebugFrames = 100

func newDeviceDebugFrame(direction, transport string, raw []byte) types.DeviceDebugFrame {
	copiedRaw := make([]byte, len(raw))
	copy(copiedRaw, raw)

	frameInfo, ok := deviceproto.ParseFrame(copiedRaw)
	debugFrame := types.DeviceDebugFrame{
		Direction: direction,
		Transport: transport,
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
		RawHex:    deviceproto.Hex(copiedRaw),
	}
	if ok {
		debugFrame.FrameHex = deviceproto.Hex(frameInfo.Frame)
		debugFrame.Command = fmt.Sprintf("0x%02X", frameInfo.Command)
		debugFrame.Length = int(frameInfo.Length)
		debugFrame.PayloadHex = deviceproto.Hex(frameInfo.Payload)
		debugFrame.ChecksumOK = frameInfo.ChecksumOK
		debugFrame.Description = deviceproto.CommandDescription(frameInfo.Command)
		decoded := deviceproto.DecodeFrame(frameInfo)
		debugFrame.Decoded = decoded.Summary
		debugFrame.Parsed = decoded
	} else {
		debugFrame.Description = "non-protocol data"
	}
	return debugFrame
}

func appendBoundedDebugFrame(seq *uint64, frames *[]types.DeviceDebugFrame, frame types.DeviceDebugFrame) uint64 {
	*seq = *seq + 1
	frame.ID = *seq
	if len(*frames) < maxDebugFrames {
		*frames = append(*frames, frame)
	} else {
		copy(*frames, (*frames)[1:])
		(*frames)[maxDebugFrames-1] = frame
	}
	return frame.ID
}

func cloneDebugFrames(frames []types.DeviceDebugFrame) []types.DeviceDebugFrame {
	copied := make([]types.DeviceDebugFrame, len(frames))
	copy(copied, frames)
	return copied
}

func debugFramesAfterSeq(frames []types.DeviceDebugFrame, seq uint64) []types.DeviceDebugFrame {
	filtered := make([]types.DeviceDebugFrame, 0, len(frames))
	for _, frame := range frames {
		if frame.ID > seq {
			filtered = append(filtered, frame)
		}
	}
	return filtered
}
