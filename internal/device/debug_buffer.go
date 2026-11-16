package device

import (
	"fmt"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

const maxDebugFrames = 100

// debugFrameRecord 是环形缓冲里落盘的"原料"帧：方向、传输、入环时刻和原始字节副本。
//
// Why: recordDebugFrame 在每一条收发的设备帧上都会被调用（黑鲨 2Hz 状态帧、BLE 通知、
// FlyDigi HID 收发，图传期间还有 ~52Hz 的 4KB 图像块），而解析结果只有调试面板（或
// 设置回读）真正来读的时候才有人消费。旧的实现入环就做 hex 编码、ParseFrame/DecodeFrame、
// ParseBlackSharkImageFrame 和 Sprintf，等于让常驻收帧路径永远替调试面板买单。
// 现在入环只存一份 raw 副本（调用方缓冲会被复用，必须拷贝），解析全部推迟到读取时，
// 由 materialize 完成；物化结果缓存在记录里，反复轮询不会重复解析。
// 缓存字段的读写全部发生在环自身的锁内，无需额外同步。
type debugFrameRecord struct {
	id        uint64
	direction string
	transport string
	timestamp time.Time // 入环时刻；materialize 时按原格式排版成 Timestamp 字符串
	raw       []byte    // 入环时拷贝的原始字节，此后只读
	cached    *types.DeviceDebugFrame
}

// newDebugFrameRecord 拷贝原始字节并打上入环时刻。除一次 memcpy 外无任何解析开销，
// 因此收帧路径可以在不持锁的情况下调用它（与旧实现把解析放在锁外的结构一致）。
func newDebugFrameRecord(direction, transport string, raw []byte) debugFrameRecord {
	copiedRaw := make([]byte, len(raw))
	copy(copiedRaw, raw)
	return debugFrameRecord{
		direction: direction,
		transport: transport,
		timestamp: time.Now(),
		raw:       copiedRaw,
	}
}

// materialize 在读取路径上把原料记录物化成对外的 types.DeviceDebugFrame。
// 解析逻辑与旧的"入环即解析"完全一致（见 parseDeviceDebugFrame），结果逐字段相同；
// Timestamp 用入环时刻的时间戳按同一格式排版。
func (r *debugFrameRecord) materialize() types.DeviceDebugFrame {
	if r.cached != nil {
		return *r.cached
	}
	frame := parseDeviceDebugFrame(r.direction, r.transport, r.timestamp, r.raw)
	frame.ID = r.id
	r.cached = &frame
	return frame
}

// parseDeviceDebugFrame 是原 newDeviceDebugFrame 的主体：hex 编码 + 协议解析 + 摘要。
// raw 必须是记录私有的副本（调用方缓冲会被复用），这里直接原地解析、不再复制。
func parseDeviceDebugFrame(direction, transport string, timestamp time.Time, raw []byte) types.DeviceDebugFrame {
	debugFrame := types.DeviceDebugFrame{
		Direction: direction,
		Transport: transport,
		Timestamp: timestamp.Format("2006-01-02 15:04:05.000"),
		RawHex:    deviceproto.Hex(raw),
	}

	// 飞智帧优先；不匹配时下面依次尝试黑鲨的 0xA4 数据族与 0xA5 控制族。
	if frameInfo, ok := deviceproto.ParseFrame(raw); ok {
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
	if img, ok := deviceproto.ParseBlackSharkImageFrame(raw); ok {
		length := imageFrameLength(raw)
		debugFrame.FrameHex = deviceproto.Hex(raw[:length])
		debugFrame.Command = fmt.Sprintf("0x%02X", deviceproto.BlackSharkCmdImageBlock)
		debugFrame.Length = length
		debugFrame.PayloadHex = deviceproto.Hex(img.Payload)
		// 数据族的 CK 是字节和，与 0xA5 回帧那套 CRC16 不同。
		debugFrame.ChecksumOK = deviceproto.BlackSharkChecksum(raw[:length-1]) == raw[length-1]
		debugFrame.ChecksumRule = string(deviceproto.BlackSharkChecksumRuleSum)
		debugFrame.ChecksumExpected = string(deviceproto.BlackSharkChecksumRuleSum)
		debugFrame.Description = "blackshark image data family (0xA4)"
		debugFrame.Decoded = fmt.Sprintf("数据族帧：序号 %d，标签 0x%02X，载荷 %d 字节",
			img.Sequence, img.Tag, len(img.Payload))
		debugFrame.Parsed = img
		return debugFrame
	}

	// 黑鲨控制族（`0xA5`）。
	if bs, ok := deviceproto.ParseBlackSharkFrame(raw); ok {
		base := 0
		if len(raw) >= 1 && raw[0] != deviceproto.BlackSharkMagic {
			base = 1 // 带 Report ID 前缀
		}
		debugFrame.FrameHex = deviceproto.Hex(raw[base : base+len(bs.Frame)])
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

// appendBoundedDebugRecord 登记一条已拷贝好的原料帧并分配序号，超过上限时覆盖最旧的一条。
func appendBoundedDebugRecord(seq *uint64, records *[]debugFrameRecord, record debugFrameRecord) uint64 {
	*seq = *seq + 1
	record.id = *seq
	if len(*records) < maxDebugFrames {
		*records = append(*records, record)
	} else {
		copy(*records, (*records)[1:])
		(*records)[maxDebugFrames-1] = record
	}
	return record.id
}

// materializeDebugFrames 物化整个环（按入环顺序），等价于旧的 cloneDebugFrames 读取语义。
func materializeDebugFrames(records []debugFrameRecord) []types.DeviceDebugFrame {
	frames := make([]types.DeviceDebugFrame, len(records))
	for i := range records {
		frames[i] = records[i].materialize()
	}
	return frames
}

// materializeDebugFramesAfter 物化环中序号大于 seq 的部分，等价于旧的 debugFramesAfterSeq。
func materializeDebugFramesAfter(records []debugFrameRecord, seq uint64) []types.DeviceDebugFrame {
	frames := make([]types.DeviceDebugFrame, 0, len(records))
	for i := range records {
		if records[i].id > seq {
			frames = append(frames, records[i].materialize())
		}
	}
	return frames
}
