package device

import (
	"fmt"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func TestDebugFrameBufferStaysBounded(t *testing.T) {
	var seq uint64
	var records []debugFrameRecord

	for i := 0; i < maxDebugFrames+17; i++ {
		record := newDebugFrameRecord("tx", types.DeviceTransportWiFi, []byte{byte(i)})
		appendBoundedDebugRecord(&seq, &records, record)
	}

	if len(records) != maxDebugFrames {
		t.Fatalf("debug frame count = %d, want %d", len(records), maxDebugFrames)
	}
	frames := materializeDebugFrames(records)
	if frames[0].ID != 18 {
		t.Fatalf("first retained frame ID = %d, want 18", frames[0].ID)
	}
	if frames[len(frames)-1].ID != uint64(maxDebugFrames+17) {
		t.Fatalf("last retained frame ID = %d, want %d", frames[len(frames)-1].ID, maxDebugFrames+17)
	}
}

func TestDebugFrameBufferReusesFullBackingArray(t *testing.T) {
	var seq uint64
	records := make([]debugFrameRecord, 0, maxDebugFrames)
	for i := 0; i < maxDebugFrames; i++ {
		appendBoundedDebugRecord(&seq, &records, newDebugFrameRecord("rx", types.DeviceTransportHID, []byte{byte(i)}))
	}
	firstSlot := &records[0]
	appendBoundedDebugRecord(&seq, &records, newDebugFrameRecord("rx", types.DeviceTransportHID, []byte{0xFF}))
	if &records[0] != firstSlot {
		t.Fatal("full debug buffer allocated a new backing array")
	}
	if records[len(records)-1].id != seq {
		t.Fatalf("last frame ID = %d, want %d", records[len(records)-1].id, seq)
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

// TestDebugFrameRecordCopiesCallerBuffer 验证入环时对调用方缓冲做了拷贝：
// 调用方随后复用缓冲不得影响已登记的帧。
func TestDebugFrameRecordCopiesCallerBuffer(t *testing.T) {
	raw := deviceproto.BuildFrame(0x27, 0x01, 0x02)
	record := newDebugFrameRecord("tx", types.DeviceTransportWiFi, raw)

	raw[0] = 0xFF // 模拟调用方复用缓冲

	frame := record.materialize()
	want := deviceproto.Hex(deviceproto.BuildFrame(0x27, 0x01, 0x02))
	if frame.RawHex != want {
		t.Fatalf("RawHex after caller buffer reuse = %q, want %q", frame.RawHex, want)
	}
}

// expectedEagerDeviceDebugFrame 用 deviceproto 的公开函数在测试内复算"旧的入环即解析"
// 逻辑会得到的结果（与改动前 newDeviceDebugFrame 的分支一一对应）。
func expectedEagerDeviceDebugFrame(t *testing.T, direction, transport string, timestamp time.Time, raw []byte) types.DeviceDebugFrame {
	t.Helper()

	expected := types.DeviceDebugFrame{
		Direction: direction,
		Transport: transport,
		Timestamp: timestamp.Format("2006-01-02 15:04:05.000"),
		RawHex:    deviceproto.Hex(raw),
	}

	if frameInfo, ok := deviceproto.ParseFrame(raw); ok {
		expected.FrameHex = deviceproto.Hex(frameInfo.Frame)
		expected.Command = fmt.Sprintf("0x%02X", frameInfo.Command)
		expected.Length = int(frameInfo.Length)
		expected.PayloadHex = deviceproto.Hex(frameInfo.Payload)
		expected.ChecksumOK = frameInfo.ChecksumOK
		expected.Description = deviceproto.CommandDescription(frameInfo.Command)
		decoded := deviceproto.DecodeFrame(frameInfo)
		expected.Decoded = decoded.Summary
		expected.Parsed = decoded
		return expected
	}

	if img, ok := deviceproto.ParseBlackSharkImageFrame(raw); ok {
		base := 0
		if len(raw) >= 2 && raw[0] != deviceproto.BlackSharkCmdImageBlock && raw[1] == deviceproto.BlackSharkCmdImageBlock {
			base = 1
		}
		length := len(raw)
		if total := int(raw[base+1]); total >= 6 && base+total <= len(raw) {
			length = base + total
		}
		expected.FrameHex = deviceproto.Hex(raw[:length])
		expected.Command = fmt.Sprintf("0x%02X", deviceproto.BlackSharkCmdImageBlock)
		expected.Length = length
		expected.PayloadHex = deviceproto.Hex(img.Payload)
		expected.ChecksumOK = deviceproto.BlackSharkChecksum(raw[:length-1]) == raw[length-1]
		expected.ChecksumRule = string(deviceproto.BlackSharkChecksumRuleSum)
		expected.ChecksumExpected = string(deviceproto.BlackSharkChecksumRuleSum)
		expected.Description = "blackshark image data family (0xA4)"
		expected.Decoded = fmt.Sprintf("数据族帧：序号 %d，标签 0x%02X，载荷 %d 字节", img.Sequence, img.Tag, len(img.Payload))
		expected.Parsed = img
		return expected
	}

	if bs, ok := deviceproto.ParseBlackSharkFrame(raw); ok {
		base := 0
		if len(raw) >= 1 && raw[0] != deviceproto.BlackSharkMagic {
			base = 1
		}
		expected.FrameHex = deviceproto.Hex(raw[base : base+len(bs.Frame)])
		expected.Command = fmt.Sprintf("0x%02X", bs.Command)
		expected.Length = int(bs.Length)
		expected.PayloadHex = deviceproto.Hex(bs.Payload)
		if direction == "rx" {
			expected.ChecksumOK = bs.ChecksumOK
			expected.ChecksumRule = string(bs.ChecksumMatched)
			expected.ChecksumExpected = string(bs.ChecksumExpected)
		} else {
			body, ck := bs.Frame[:len(bs.Frame)-1], bs.Frame[len(bs.Frame)-1]
			expected.ChecksumOK = deviceproto.BlackSharkChecksum(body) == ck
			expected.ChecksumRule = string(deviceproto.BlackSharkChecksumRuleSum)
			expected.ChecksumExpected = string(deviceproto.BlackSharkChecksumRuleSum)
		}
		expected.Description = deviceproto.BlackSharkCommandDescription(bs.Command)
		dir := "主机→设备"
		if bs.Command == deviceproto.BlackSharkCmdStatusNotify || bs.Command == deviceproto.BlackSharkCmdVersion {
			dir = "设备→主机"
		}
		if rpm, ok := deviceproto.BlackSharkStatusRPM(bs); ok {
			expected.Decoded = fmt.Sprintf("%s · 命令 0x%02X · 载荷 %d 字节 · 转速 %d RPM",
				dir, bs.Command, len(bs.Payload), rpm)
		} else {
			expected.Decoded = fmt.Sprintf("%s · 命令 0x%02X · 载荷 %d 字节", dir, bs.Command, len(bs.Payload))
		}
		expected.Parsed = bs
		return expected
	}

	expected.Description = "non-protocol data"
	return expected
}

// buildBlackSharkRXFrame 构造一条"设备→主机"的 0xA5 控制族回帧：CK 用 CRC-16/XMODEM 低字节。
func buildBlackSharkRXFrame(cmd byte, payload ...byte) []byte {
	total := 3 + len(payload) + 1 // A5 + LEN + CMD + payload + CK
	frame := make([]byte, 0, total)
	frame = append(frame, deviceproto.BlackSharkMagic, byte(total), cmd)
	frame = append(frame, payload...)
	frame = append(frame, deviceproto.BlackSharkRXChecksum(frame))
	return frame
}

// buildBlackSharkImageFrame 构造一条 0xA4 数据族帧：CK 是前置字节和。
func buildBlackSharkImageFrame(sequence uint16, tag byte, payload ...byte) []byte {
	total := 5 + len(payload) + 1 // A4 + LEN + seq(2) + tag + payload + CK
	frame := make([]byte, 0, total)
	frame = append(frame, deviceproto.BlackSharkCmdImageBlock, byte(total),
		byte(sequence), byte(sequence>>8), tag)
	frame = append(frame, payload...)
	frame = append(frame, deviceproto.BlackSharkChecksum(frame))
	return frame
}

// TestLazyDebugFrameMaterializationMatchesEagerLogic 记录代表性帧后走读取路径
// （GetDebugFrames / debugFramesAfter），断言物化结果与旧的入环即解析逻辑一致。
func TestLazyDebugFrameMaterializationMatchesEagerLogic(t *testing.T) {
	manager := NewManager(nil)

	flyDigiFrame := deviceproto.BuildFrame(0x27, 0x20, 0x03, 0x40, 0x06, 0x60, 0x09, 0x80, 0x0C)
	unparsable := []byte{0x01, 0x02, 0x03}
	blackSharkTX := deviceproto.BuildBlackSharkFrame(deviceproto.BlackSharkCmdSetSpeed, 0x00, 0x00, 0x01, 0x40)
	blackSharkRX := buildBlackSharkRXFrame(0x13, 0x01, 0x02)
	imageFrame := buildBlackSharkImageFrame(7, 0x02, 0xAA, 0xBB, 0xCC, 0xDD)

	inputs := []struct {
		direction string
		transport string
		raw       []byte
	}{
		{"tx", types.DeviceTransportHID, flyDigiFrame},
		{"rx", types.DeviceTypeHID, unparsable},
		{"tx", types.DeviceTypeHID, blackSharkTX},
		{"rx", types.DeviceTransportUSB, blackSharkRX},
		{"rx", types.DeviceTypeHID, imageFrame},
	}

	before := time.Now()
	for _, input := range inputs {
		manager.recordDebugFrame(input.direction, input.transport, input.raw)
	}
	after := time.Now()

	frames := manager.GetDebugFrames()
	if len(frames) != len(inputs) {
		t.Fatalf("GetDebugFrames returned %d frames, want %d", len(frames), len(inputs))
	}

	timestampLayout := "2006-01-02 15:04:05.000"
	timestampPattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}$`)
	beforeStr := before.Format(timestampLayout)
	afterStr := after.Format(timestampLayout)

	for i, input := range inputs {
		frame := frames[i]
		if frame.ID != uint64(i+1) {
			t.Fatalf("frame %d ID = %d, want %d", i, frame.ID, i+1)
		}
		if !timestampPattern.MatchString(frame.Timestamp) {
			t.Fatalf("frame %d Timestamp = %q, want format %q", i, frame.Timestamp, timestampLayout)
		}
		if frame.Timestamp < beforeStr || frame.Timestamp > afterStr {
			t.Fatalf("frame %d Timestamp = %q outside record window [%q, %q]", i, frame.Timestamp, beforeStr, afterStr)
		}

		expected := expectedEagerDeviceDebugFrame(t, input.direction, input.transport, before, input.raw)
		expected.ID = uint64(i + 1)
		// 时间戳单独已在上面按格式与窗口校验；这里用窗口起点复算期望值，其余字段必须逐字段一致。
		expected.Timestamp = frame.Timestamp
		if !reflect.DeepEqual(frame, expected) {
			t.Fatalf("frame %d materialized = %+v, want eager %+v", i, frame, expected)
		}
	}

	// debugFramesAfter 读取路径：只返回序号大于 seq 的帧。
	afterSeq := manager.debugFramesAfter(3)
	if len(afterSeq) != 2 {
		t.Fatalf("debugFramesAfter(3) returned %d frames, want 2", len(afterSeq))
	}
	if !reflect.DeepEqual(afterSeq, frames[3:]) {
		t.Fatalf("debugFramesAfter(3) = %+v, want %+v", afterSeq, frames[3:])
	}

	// 重复读取（命中物化缓存）必须得到完全一致的结果。
	again := manager.GetDebugFrames()
	if !reflect.DeepEqual(again, frames) {
		t.Fatal("second GetDebugFrames returned different frames than the first read")
	}
}
