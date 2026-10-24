package deviceproto

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"strings"
	"testing"
	"time"
)

func TestBlackSharkFrameBuildersAndStatusParser(t *testing.T) {
	// LEN = 4 + 载荷 是本仓库的统一约定（4 字节帧 LEN=4），见 docs/blackshark-brb02-access-notes.md。
	if got := BuildBlackSharkGetVersion(); !bytes.Equal(got, []byte{0xA5, 0x04, 0x01, 0xAA}) {
		t.Fatalf("version frame = % X", got)
	}
	// 固定转速 = 官方 apply 仪式三帧（0x24[FF 暂存] → 0x08 00 → 0x24[真实值]），
	// 唯一所有者 BuildBlackSharkFixedSpeedFrames。
	frames, err := BuildBlackSharkFixedSpeedFrames(1, 1600, 0)
	if err != nil || len(frames) != 3 {
		t.Fatalf("fixed speed frames n=%d / err=%v, want three frames", len(frames), err)
	}
	wantStaging := []byte{0xFF, 0x00, 0x01, 0x40, 0x06}
	wantReal := []byte{0x00, 0x00, 0x01, 0x40, 0x06}
	if !bytes.Equal(frames[0].Payload, wantStaging) || frames[0].Cmd != BlackSharkCmdSetSpeed {
		t.Fatalf("staging frame = cmd=0x%02X % X, want 0x24 % X", frames[0].Cmd, frames[0].Payload, wantStaging)
	}
	if !bytes.Equal(frames[1].Payload, []byte{0x00}) || frames[1].Cmd != BlackSharkCmdSetSceneMode {
		t.Fatalf("gate frame = cmd=0x%02X % X, want 0x08 [00]", frames[1].Cmd, frames[1].Payload)
	}
	if !bytes.Equal(frames[2].Payload, wantReal) || frames[2].Cmd != BlackSharkCmdSetSpeed {
		t.Fatalf("real frame = cmd=0x%02X % X, want 0x24 % X", frames[2].Cmd, frames[2].Payload, wantReal)
	}
	if got := BuildBlackSharkFrame(BlackSharkCmdSetSpeed, frames[2].Payload...); !bytes.Equal(got, []byte{0xA5, 0x09, 0x24, 0x00, 0x00, 0x01, 0x40, 0x06, 0x19}) {
		t.Fatalf("speed frame = % X", got)
	}
	// 受控态内的提交帧 = 单帧 0x24，载荷与仪式第三帧相同（节奏化写入的选帧规则见
	// device.Manager.blackSharkFixedSpeedFramesLocked）。
	commit, err := BuildBlackSharkFixedSpeedCommitFrame(1, 1600, 0)
	if err != nil || commit.Cmd != BlackSharkCmdSetSpeed || !bytes.Equal(commit.Payload, wantReal) {
		t.Fatalf("commit frame = cmd=0x%02X % X / err=%v, want 0x24 % X", commit.Cmd, commit.Payload, err, wantReal)
	}
	// 0x22 控制源=01 会话开场帧:官方会话的第一帧,缺了它设备不推状态帧。
	if got := BuildBlackSharkSessionOpen(); !bytes.Equal(got, []byte{0xA5, 0x04, 0x22, 0x01, 0xCC}) {
		t.Fatalf("session open frame = % X, want A5 04 22 01 CC", got)
	}
	frame, ok := ParseBlackSharkFrame([]byte{0xA5, 0x07, 0x06, 0xAC, 0x08, 0x00, 0x66})
	if !ok || !frame.ChecksumOK {
		t.Fatalf("status frame parse = %#v/%v", frame, ok)
	}
	rpm, flag, ok := ParseBlackSharkStatus(frame)
	if !ok || rpm != 2220 || flag != 0 {
		t.Fatalf("status = %d/%d/%v", rpm, flag, ok)
	}
}

func TestBlackSharkChecksumRuleTableAndLenTolerance(t *testing.T) {
	// 表驱动：命令码 → 回帧校验规则（唯一所有者 BlackSharkChecksumRules）。
	if rule, ok := BlackSharkChecksumRuleFor(0x13); !ok || rule != BlackSharkChecksumRuleCRC16 {
		t.Fatalf("0x13 rule = %q/%v", rule, ok)
	}
	if rule, ok := BlackSharkChecksumRuleFor(0x06); !ok || rule != BlackSharkChecksumRuleSum {
		t.Fatalf("0x06 rule = %q/%v", rule, ok)
	}
	if _, ok := BlackSharkChecksumRuleFor(0x7E); ok {
		t.Fatal("unregistered command should report ok=false")
	}

	// 真机回帧 `a5 0c 13 03 b8 0b 50 01 19 00 ff 90`：尾字节 0x90 是 CRC16 低字节（字节和会是 0xF3）。
	reply, ok := ParseBlackSharkFrame([]byte{0xA5, 0x0C, 0x13, 0x03, 0xB8, 0x0B, 0x50, 0x01, 0x19, 0x00, 0xFF, 0x90})
	if !ok || !reply.ChecksumOK {
		t.Fatalf("0x13 reply parse = %#v/%v", reply, ok)
	}
	if reply.ChecksumMatched != BlackSharkChecksumRuleCRC16 || reply.ChecksumExpected != BlackSharkChecksumRuleCRC16 || reply.ChecksumRuleMismatch() {
		t.Fatalf("0x13 reply rules = matched %q / expected %q / mismatch %v",
			reply.ChecksumMatched, reply.ChecksumExpected, reply.ChecksumRuleMismatch())
	}

	// 0x06 主动上报走字节和，且与表一致。
	report, ok := ParseBlackSharkFrame([]byte{0xA5, 0x07, 0x06, 0xAC, 0x08, 0x00, 0x66})
	if !ok || report.ChecksumMatched != BlackSharkChecksumRuleSum || report.ChecksumExpected != BlackSharkChecksumRuleSum {
		t.Fatalf("0x06 rules = ok %v matched %q expected %q", ok, report.ChecksumMatched, report.ChecksumExpected)
	}

	// LEN 容忍：LEN 写「总长−1」的短形态也要接住（官方与 DLL 的历史构造如此，固件也接受）。
	// `A5 03 01 A9`：无载荷查询，LEN=3 而实长 4，尾字节 0xA9 = 前置字节和。
	short, ok := ParseBlackSharkFrame([]byte{0xA5, 0x03, 0x01, 0xA9})
	if !ok || short.Length != 4 || short.Command != 0x01 || len(short.Payload) != 0 {
		t.Fatalf("short form = ok %v length %d command 0x%02X payload % X", ok, short.Length, short.Command, short.Payload)
	}

	// 同一条 9 字节固定转速帧写成短形态：LEN 记 8（= 总长−1），校验按实长 9 字节成立
	// （尾字节 0x18 = 前置字节和）。短读时尾字节落在 0x06，两条规则都不命中
	// （字节和 0x12 / CRC16 0xAD），所以不会被误当成 8 字节帧。
	lenShort := []byte{0xA5, 0x08, 0x24, 0x00, 0x00, 0x01, 0x40, 0x06, 0x18}
	parsed, ok := ParseBlackSharkFrame(lenShort)
	if !ok || parsed.Length != 9 || len(parsed.Payload) != 5 {
		t.Fatalf("LEN-1 fixed frame = ok %v length %d payload %d", ok, parsed.Length, len(parsed.Payload))
	}

	// 19 字节曲线形态同理：LEN 写 18，实长 19、15 字节载荷，校验按 19 字节成立。
	curve := []byte{0xA5, 0x12, 0x24, 0x00, 0x01, 0x01, 0x14, 0xB0, 0x04, 0x28, 0xE8, 0x05,
		0x3C, 0x56, 0x08, 0x50, 0xC4, 0x0A}
	curve = append(curve, BlackSharkChecksum(curve))
	parsed, ok = ParseBlackSharkFrame(curve)
	if !ok || parsed.Length != 19 || len(parsed.Payload) != 15 {
		t.Fatalf("LEN-1 curve frame = ok %v length %d payload %d", ok, parsed.Length, len(parsed.Payload))
	}
}

func TestBlackSharkCaptured07FrameIsKeptOpaque(t *testing.T) {
	// This is a captured BLE write from the BRB02 report. 0x07 is deliberately
	// treated as an opaque command here; the report does not establish the
	// meaning of its tagged bytes.
	raw := []byte{0xA5, 0x1A, 0x07, 0x07, 0x00, 0x40, 0x00, 0x01, 0x3B, 0x00, 0x02, 0x1E, 0x00, 0x03, 0x26, 0x00, 0x05, 0x2F, 0x00, 0x06, 0x33, 0x00, 0x07, 0x8F, 0x05, 0x9A}
	frame, ok := ParseBlackSharkFrame(raw)
	if !ok || !frame.ChecksumOK {
		t.Fatalf("captured 0x07 frame parse = %#v/%v", frame, ok)
	}
	if frame.Command != 0x07 || !bytes.Equal(frame.Payload, raw[3:25]) {
		t.Fatalf("captured 0x07 frame fields = command 0x%02X payload % X", frame.Command, frame.Payload)
	}
	// This captured command uses the long-form convention where LEN includes
	// the checksum byte; short commands in this package use the legacy form.
	if got := buildBlackSharkFrameWithLength(0x1A, frame.Command, frame.Payload...); !bytes.Equal(got, raw) {
		t.Fatalf("captured 0x07 frame serialization = % X, want % X", got, raw)
	}
}

func TestBlackSharkHostInfoFrameMatchesOfficialCapture(t *testing.T) {
	// PROTOCOL §5 cap6 首帧 (官方实机 0x07 推送): 7 项 = CPU 温 78 / GPU 温 43 /
	// GPU 功 17 / 未知 0 / 常量 74 / GPU 热点 54 / 时钟 1108 (18:28)。
	entries := []BlackSharkHostInfoEntry{
		{0x00, 78}, {0x01, 43}, {0x02, 17}, {0x03, 0}, {0x05, 74}, {0x06, 54}, {0x07, 1108},
	}
	got := BuildBlackSharkHostInfo(entries)
	want := []byte{
		0xA5, 0x1A, 0x07, 0x07,
		0x00, 0x4E, 0x00,
		0x01, 0x2B, 0x00,
		0x02, 0x11, 0x00,
		0x03, 0x00, 0x00,
		0x05, 0x4A, 0x00,
		0x06, 0x36, 0x00,
		0x07, 0x54, 0x04,
		0x47,
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("host info frame = % X, want % X", got, want)
	}
}

func TestBlackSharkModeNameMatchesFlyDigiConvention(t *testing.T) {
	if got := BlackSharkModeName(0); got != "manual/fixed gear mode" {
		t.Fatalf("manual mode = %q", got)
	}
	if got := BlackSharkModeName(1); got != "auto/realtime RPM mode" {
		t.Fatalf("automatic mode = %q", got)
	}
}

func TestBlackSharkLightingFrame(t *testing.T) {
	frame, err := BuildBlackSharkSetLighting("blackshark_breathing", "medium", 80, 0x19, 0x00, 0xFF)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(frame, []byte{0xA5, 0x0C, 0x12, 0x03, 0xB8, 0x0B, 0x50, 0x01, 0x19, 0x00, 0xFF, 0xF2}) {
		t.Fatalf("lighting frame = % X", frame)
	}
	if _, err := BuildBlackSharkSetLighting("static_multi", "slow", 80, 0x19, 0x00, 0xFF); err != nil {
		t.Fatalf("generic static_multi lighting mode should map to a native mode: %v", err)
	}
}

func TestBlackSharkRGB565AndCRC(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, BlackSharkImageWidth, BlackSharkImageHeight))
	img.SetRGBA(0, 0, color.RGBA{R: 0xFF, A: 0xFF})
	canvas, err := BlackSharkRGB565(img)
	if err != nil {
		t.Fatal(err)
	}
	if len(canvas) != BlackSharkImageBytes || !bytes.Equal(canvas[:2], []byte{0xF8, 0x00}) {
		t.Fatalf("canvas size/pixel = %d/% X", len(canvas), canvas[:2])
	}
	if got := BlackSharkCRC16XMODEM([]byte("123456789")); got != 0x31C3 {
		t.Fatalf("CRC16/XMODEM = %04X", got)
	}
	if _, err := BlackSharkRGB565(image.NewRGBA(image.Rect(0, 0, 1, 1))); err == nil {
		t.Fatal("expected image dimension error")
	}
}

func TestBlackSharkImageUploadFrames(t *testing.T) {
	canvas := bytes.Repeat([]byte{0x12, 0x34}, BlackSharkImageWidth*BlackSharkImageHeight)
	frames, err := BuildBlackSharkImageUpload(canvas)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2098 || len(frames[0]) != 15 || len(frames[1]) != 64 || len(frames[len(frames)-2]) != 48 || len(frames[len(frames)-1]) != 4 {
		t.Fatalf("frame count/lengths = %d/%d/%d/%d/%d", len(frames), len(frames[0]), len(frames[1]), len(frames[len(frames)-2]), len(frames[len(frames)-1]))
	}
	if !bytes.Equal(frames[0][0:3], []byte{0xA5, 0x0F, 0xC4}) || binary.LittleEndian.Uint32(frames[0][7:11]) != uint32(BlackSharkImageBytes) {
		t.Fatalf("metadata header/size = % X/%d", frames[0][:11], binary.LittleEndian.Uint32(frames[0][7:11]))
	}
	if !bytes.Equal(frames[1][:6], []byte{0xA4, 0x40, 0x30, 0x88, 0xC6, 0x12}) {
		t.Fatalf("first block header = % X", frames[1][:6])
	}
	if !bytes.Equal(frames[2095][:5], []byte{0xA4, 0x40, 0x02, 0x00, 0xC6}) {
		t.Fatalf("last full block header = % X", frames[2095][:6])
	}
	if !bytes.Equal(frames[2096][:5], []byte{0xA4, 0x30, 0x01, 0x00, 0xC6}) {
		t.Fatalf("tail header = % X", frames[2096][:6])
	}
	if !bytes.Equal(frames[len(frames)-1], []byte{0xA5, 0x04, 0xC5, 0x6E}) {
		t.Fatalf("commit frame = % X", frames[len(frames)-1])
	}
	for _, frame := range frames {
		if len(frame) < 4 || int(frame[1]) != len(frame) {
			t.Fatalf("invalid frame length: % X", frame)
		}
		if blackSharkChecksum(frame[:len(frame)-1]) != frame[len(frame)-1] {
			t.Fatalf("invalid frame checksum: % X", frame)
		}
	}
}

func TestBlackSharkImageBuilderRejectsLengths(t *testing.T) {
	if _, err := BuildBlackSharkImageMetadata(nil); err == nil {
		t.Fatal("expected metadata canvas length error")
	}
	if _, err := BuildBlackSharkImageDataBlock(1, make([]byte, 58), false); err == nil {
		t.Fatal("expected block number error")
	}
	if _, err := BuildBlackSharkImageDataBlock(2, make([]byte, 57), false); err == nil {
		t.Fatal("expected data block length error")
	}
	if _, err := BuildBlackSharkImageTail(make([]byte, 41)); err == nil {
		t.Fatal("expected tail length error")
	}
}

// 信息帧前 4 字节是本次下发时刻的 Unix 秒，不再是固定常量。
func TestBlackSharkImageMetadataCarriesUploadTimestamp(t *testing.T) {
	canvas := make([]byte, BlackSharkImageBytes)
	before := time.Now().Unix()
	frame, err := BuildBlackSharkImageMetadata(canvas)
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now().Unix()
	got := int64(binary.LittleEndian.Uint32(frame[3:7]))
	if got < before || got > after {
		t.Fatalf("metadata timestamp = %d, want within [%d,%d]", got, before, after)
	}
}

func TestBlackSharkGearConfigEncode(t *testing.T) {
	fixed, err := EncodeBlackSharkFixedSpeedPayload(3, 0x0B60, BlackSharkCoolingSourceGPU)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fixed, []byte{0x01, 0x00, 0x03, 0x60, 0x0B}) {
		t.Fatalf("fixed payload = % X", fixed)
	}
	if _, err := EncodeBlackSharkCurvePayload(3, [BlackSharkCurvePointCount]BlackSharkCurvePoint{}, BlackSharkCoolingSourceCPU); err != nil {
		t.Fatal(err)
	}
	// 未登记表里的 form 值必须报错，不能回落成固定帧。
	if payload, err := (BlackSharkGearConfig{Form: 0x02, Gear: 1}).Encode(); err == nil {
		t.Fatalf("unknown form should fail, got % X", payload)
	}
}

// 两个参照源各写一遍；当前源放最后一次写，避免写完把设备的生效参照源改掉。
func TestBlackSharkCoolingSourceWriteOrder(t *testing.T) {
	cases := []struct {
		current byte
		want    [BlackSharkCoolingSourceCount]byte
	}{
		{BlackSharkCoolingSourceCPU, [BlackSharkCoolingSourceCount]byte{BlackSharkCoolingSourceGPU, BlackSharkCoolingSourceCPU}},
		{BlackSharkCoolingSourceGPU, [BlackSharkCoolingSourceCount]byte{BlackSharkCoolingSourceCPU, BlackSharkCoolingSourceGPU}},
		// 非法值先被钳到 GPU，次序随之。
		{0x07, [BlackSharkCoolingSourceCount]byte{BlackSharkCoolingSourceCPU, BlackSharkCoolingSourceGPU}},
	}
	for _, c := range cases {
		if got := BlackSharkCoolingSourceWriteOrder(c.current); got != c.want {
			t.Fatalf("write order for %d = %v, want %v", c.current, got, c.want)
		}
	}
}

// 命令表里的官方信号名要与官方 moc 表逐条对上，缺一个都算漏。
func TestBlackSharkCommandsCarryOfficialSignals(t *testing.T) {
	want := map[byte]string{
		0x01: "signal_get_firmware_version",
		0x02: "signal_set_on_off_status",
		0x03: "signal_get_on_off_status",
		0x05: "signal_enter_boot_mode",
		0x06: "signal_firmware_heart_beat",
		0x10: "signal_set_rgb_lighting_switch",
		0x11: "signal_get_rgb_lighting_switch",
		0x12: "signal_set_rgb_lighting_effects",
		0x13: "signal_get_cur_rgb_lighting_effects",
		0x14: "signal_get_any_rgb_lighting_effects",
		0x22: "signal_set_cooling_source",
		0x25: "signal_get_cur_cooling_config",
		0x26: "signal_get_any_cooling_config",
		0xC0: "signal_set_lcd_screen_switch",
		0xC1: "signal_get_lcd_screen_switch",
		0xC4: "signal_set_screensaver_image_info",
		0xC5: "signal_get_screensaver_image_info",
		0xC6: "signal_send_screensaver_image_data",
		0xC7: "signal_get_screensaver_image_data",
		0xC8: "signal_recv_screensaver_image_data",
		0xF0: "signal_restore_factory_settings",
	}
	for cmd, signal := range want {
		spec, ok := BlackSharkCommandByCmd(cmd)
		if !ok {
			t.Fatalf("command 0x%02X missing from table", cmd)
		}
		if spec.OfficialSignal != signal {
			t.Fatalf("command 0x%02X official signal = %q, want %q", cmd, spec.OfficialSignal, signal)
		}
	}
	// 0x24 一条命令对应官方三枚信号（通用 + 固定/变频变体）。
	spec, ok := BlackSharkCommandByCmd(0x24)
	if !ok || !strings.Contains(spec.OfficialSignal, "signal_set_cooling_config") ||
		!strings.Contains(spec.OfficialSignal, "signal_set_cooling_config_fixed") ||
		!strings.Contains(spec.OfficialSignal, "signal_set_cooling_config_inverter") {
		t.Fatalf("command 0x24 official signals = %q", spec.OfficialSignal)
	}
}
