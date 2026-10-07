package device

import (
	"fmt"
	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/types"
	"time"
)

const maxDebugFrames = 100

func newDeviceDebugFrame(direction, transport string, raw []byte) types.DeviceDebugFrame {
	copiedRaw := make([]byte, len(raw))
	copy(copiedRaw, raw)

	debugFrame := types.DeviceDebugFrame{
		Direction: direction,
		Transport: transport,
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
		RawHex:    deviceproto.Hex(copiedRaw),
	}

	// 飞智帧优先；不匹配时下面依次尝试黑鲨的 0xA4 数据族与 0xA5 控制族。
	if frameInfo, ok := deviceproto.ParseFrame(copiedRaw); ok {
		debugFrame.FrameHex = deviceproto.Hex(frameInfo.Frame)
		debugFrame.Command = fmt.Sprintf("0x%02X", frameInfo.Command)
		debugFrame.Length = int(frameInfo.Length)
		debugFrame.PayloadHex = deviceproto.Hex(frameInfo.Payload)
		debugFrame.ChecksumOK = frameInfo.ChecksumOK
		debugFrame.Description = deviceproto.CommandDescription(frameInfo.Command)
		decoded := deviceproto.DecodeFrame(frameInfo)
		debugFrame.Decoded = decoded.Summary
		debugFrame.Parsed = decoded
		return debugFrame
	}

	// 黑鲨数据族（`0xA4`）：屏保图像 / 预览动画帧。
	if img, ok := deviceproto.ParseBlackSharkImageFrame(copiedRaw); ok {
		length := imageFrameLength(copiedRaw)
		debugFrame.FrameHex = deviceproto.Hex(copiedRaw[:length])
		debugFrame.Command = fmt.Sprintf("0x%02X", deviceproto.BlackSharkCmdImageBlock)
		debugFrame.Length = length
		debugFrame.PayloadHex = deviceproto.Hex(img.Payload)
		// 数据族的 CK 是字节和，与 0xA5 回帧那套 CRC16 不同。
		debugFrame.ChecksumOK = deviceproto.BlackSharkChecksum(copiedRaw[:length-1]) == copiedRaw[length-1]
		debugFrame.ChecksumRule = string(deviceproto.BlackSharkChecksumRuleSum)
		debugFrame.ChecksumExpected = string(deviceproto.BlackSharkChecksumRuleSum)
		debugFrame.Description = "blackshark image data family (0xA4)"
		debugFrame.Decoded = fmt.Sprintf("数据族帧：序号 %d，标签 0x%02X，载荷 %d 字节",
			img.Sequence, img.Tag, len(img.Payload))
		debugFrame.Parsed = img
		return debugFrame
	}

	// 黑鲨控制族（`0xA5`）。
	if bs, ok := deviceproto.ParseBlackSharkFrame(copiedRaw); ok {
		base := 0
		if len(copiedRaw) >= 1 && copiedRaw[0] != deviceproto.BlackSharkMagic {
			base = 1 // 带 Report ID 前缀
		}
		debugFrame.FrameHex = deviceproto.Hex(copiedRaw[base : base+len(bs.Frame)])
		debugFrame.Command = fmt.Sprintf("0x%02X", bs.Command)
		debugFrame.Length = int(bs.Length)
		debugFrame.PayloadHex = deviceproto.Hex(bs.Payload)
		// CK 规则登记在 deviceproto.BlackSharkChecksumRules（唯一所有者）；解析结果已带出
		// 「登记哪条 / 实际命中哪条」，本文件不再复述规则。
		// TX 侧统一前置字节和；BLE 上的 0x07 用字节和——同命令在不同传输上规则不同，
		// 于是会出现 Expected=crc16 / Matched=sum 的组合，正好由这两个字段暴露出来。
		if direction == "rx" {
			debugFrame.ChecksumOK = bs.ChecksumOK
			debugFrame.ChecksumRule = string(bs.ChecksumMatched)
			debugFrame.ChecksumExpected = string(bs.ChecksumExpected)
		} else {
			body, ck := bs.Frame[:len(bs.Frame)-1], bs.Frame[len(bs.Frame)-1]
			debugFrame.ChecksumOK = deviceproto.BlackSharkChecksum(body) == ck
			debugFrame.ChecksumRule = string(deviceproto.BlackSharkChecksumRuleSum)
			debugFrame.ChecksumExpected = string(deviceproto.BlackSharkChecksumRuleSum)
		}
		debugFrame.Description = deviceproto.BlackSharkCommandDescription(bs.Command)
		debugFrame.Decoded = blackSharkDebugSummary(bs)
		debugFrame.Parsed = bs
		return debugFrame
	}

	debugFrame.Description = "non-protocol data"
	return debugFrame
}

// imageFrameLength 返回 0xA4 数据帧在报文里的实际长度（含可能的 1 字节 Report ID 前缀）。
func imageFrameLength(report []byte) int {
	base := 0
	if len(report) >= 2 && report[0] != deviceproto.BlackSharkCmdImageBlock &&
		report[1] == deviceproto.BlackSharkCmdImageBlock {
		base = 1
	}
	if base >= len(report) {
		return len(report)
	}
	total := int(report[base+1])
	if total < 6 || base+total > len(report) {
		return len(report)
	}
	return base + total
}

// blackSharkDebugSummary 给控制族帧拼一句可读摘要。
func blackSharkDebugSummary(f deviceproto.BlackSharkFrame) string {
	dir := "主机→设备"
	if f.Command == deviceproto.BlackSharkCmdStatusNotify ||
		f.Command == deviceproto.BlackSharkCmdVersion {
		dir = "设备→主机"
	}
	if rpm, ok := deviceproto.BlackSharkStatusRPM(f); ok {
		return fmt.Sprintf("%s · 命令 0x%02X · 载荷 %d 字节 · 转速 %d RPM",
			dir, f.Command, len(f.Payload), rpm)
	}
	return fmt.Sprintf("%s · 命令 0x%02X · 载荷 %d 字节", dir, f.Command, len(f.Payload))
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
