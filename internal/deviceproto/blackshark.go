// Package deviceproto 是设备协议层：通用帧助手（commands.go / protocol.go）与黑鲨 BRB02 的
// 帧编解码、命令表、标定表、出厂常量。
//
// 黑鲨部分是纯逻辑、不做设备 I/O：帧与命令表、0x24/0x25/0x26/0x22 编解码、两张标定表、
// 出厂常量、0x07 与 0xC2 载荷；设备读写归 internal/device，编排归 internal/coreapp。
//
// 黑鲨协议事实见 docs/blackshark-brb02-access-notes.md。
package deviceproto

import (
	"encoding/binary"
	"fmt"
	"image"
	"math"
	"strings"
)

// ---- 帧编解码、命令表、冷却形态与灯效参数常量、系统信息推送 ----
// 协议事实见 docs/blackshark-brb02-access-notes.md。

const (
	// BlackSharkMagic 帧首字节。
	BlackSharkMagic byte = 0xA5
	// BlackSharkReportID HID 报文首字节（Report ID）。
	BlackSharkReportID byte = 0x00
	// BlackSharkReportLength HID 报文总长度（含 Report ID）。
	BlackSharkReportLength = 65
	// BlackSharkMaxFrameLength 协议允许的最大帧长（65 - 1 个 Report ID 字节）。
	BlackSharkMaxFrameLength = BlackSharkReportLength - 1
)

// BlackShark 命令码。取值来自 Brb02CoolerComm.dll 各导出函数的帧构造常量。
const (
	BlackSharkCmdVersion byte = 0x01 // 返回 ASCII 版本号，如 "3.0.3"

	// BlackSharkCmdSetSceneMode 「情景」模式写入（1 字节载荷）。官方只在启动时发 08 00。
	BlackSharkCmdSetSceneMode byte = 0x08

	// BlackSharkCmdSetOnOffVector 写入「智能启停 + 通电自启」两路开关向量。
	BlackSharkCmdSetOnOffVector byte = 0x02

	// BlackSharkCmdGetOnOffVector 读回「智能启停 + 通电自启」两路开关向量。
	BlackSharkCmdGetOnOffVector byte = 0x03

	BlackSharkCmdEnterBootMode byte = 0x05 // 进入 BOOT（固件升级用，勿在正常流程调用）
	BlackSharkCmdStatusNotify  byte = 0x06 // 设备->主机周期状态：[RPM u16le][标志]
	BlackSharkCmdNotify        byte = 0x07 // 主机->设备 ~1Hz 推送 7 组 [id][u16le]
	// BlackSharkCmdRGBEnable 灯效总开关（1 字节载荷）。
	BlackSharkCmdRGBEnable byte = 0x10
	// BlackSharkCmdRGBStatus 读回灯效总开关状态。
	BlackSharkCmdRGBStatus        byte = 0x11
	BlackSharkCmdSetLighting      byte = 0x12 // 官方按「保存」时发它（31 帧 / 22 种载荷）
	BlackSharkCmdGetLighting      byte = 0x13 // 回复 = 8 字节灯效块
	BlackSharkCmdGetAnyRgbEffects byte = 0x14 // 请求载荷 = 1 字节槽位；回复 = 8 字节灯效块
	// BlackSharkCmdSetFanSwitch 风扇开关。已登记、不接入：固件没有对应读回命令，
	// 写完无法确认生效；该状态在 LCD 上本来就能看到，面板里再来一个开关只会误导。
	BlackSharkCmdSetFanSwitch        byte = 0x20
	BlackSharkCmdSetCoolingSource    byte = 0x22
	BlackSharkCmdGetCoolingSource    byte = 0x23
	BlackSharkCmdSetSpeed            byte = 0x24 // 写入冷却配置：[flag][form][gear][data]
	BlackSharkCmdGetStatus           byte = 0x25 // 读取当前冷却配置
	BlackSharkCmdGetAnyCoolingConfig byte = 0x26 // 读取另一份（载荷 [槽位][form]）

	// BlackSharkCmdSetDeviceSwitch 设备总开关。载荷语义未定、官方不解析回帧、本工具暂不发。
	BlackSharkCmdSetDeviceSwitch byte = 0x27

	BlackSharkCmdSetLcdScreenSwitch byte = 0xC0
	BlackSharkCmdGetLcdScreenSwitch byte = 0xC1

	// BlackSharkCmdSetLcdShowPos 设置 LCD 显示内容的位置（官方 coolerSetLcdShowPos）。
	BlackSharkCmdSetLcdShowPos byte = 0xC2

	// BlackSharkCmdImageMetadata 写入屏保图像信息（11 字节载荷）。
	BlackSharkCmdImageMetadata byte = 0xC4

	// BlackSharkCmdImageCommit 屏保图像上传的收尾帧；空载荷请求时读回设备当前持有的图信息（11 字节）。
	BlackSharkCmdImageCommit byte = 0xC5

	// BlackSharkCmdRestoreFactory 恢复出厂。同一命令码官方也用于配置解析，语义由上下文决定。
	BlackSharkCmdRestoreFactory byte = 0xF0

	// BlackSharkCmdImageDataFlow 屏保图像数据段的流控帧。
	BlackSharkCmdImageDataFlow byte = 0xC6

	// BlackSharkCmdGetScreensaverImageData 从设备读回屏保图像数据；起读后设备推一条 0xA4。
	BlackSharkCmdGetScreensaverImageData byte = 0xC7

	// BlackSharkCmdPullNextFrame 「取下一帧」：设备回一条 65 字节的 0xA4 帧数据。
	BlackSharkCmdPullNextFrame byte = 0xC8

	// BlackSharkCmdIssueAudioSpectrumLevel 「音频同步」：主机按帧推主频档位。
	BlackSharkCmdIssueAudioSpectrumLevel byte = 0x15

	// BlackSharkCmdNotifyMouseKeyPress 「响应」灯效：每按一次键/鼠标键推一帧。
	BlackSharkCmdNotifyMouseKeyPress byte = 0x16

	// BlackSharkCmdImageBlock 屏保图像数据族的帧首字节：主机发出的每条数据帧、设备回读的每一帧都以它开头。
	BlackSharkCmdImageBlock byte = 0xA4
)

// BlackSharkDir 命令方向。
type BlackSharkDir string

const (
	BlackSharkDirHostToDevice BlackSharkDir = "h2d"
	BlackSharkDirDeviceToHost BlackSharkDir = "d2h"
	BlackSharkDirBoth         BlackSharkDir = "both"
)

// BlackSharkCommandSpec 一个命令的全部协议事实。
type BlackSharkCommandSpec struct {
	Cmd  byte
	Name string        // 人类可读（优先用官方 mission 名）
	Dir  BlackSharkDir // 方向
	// PayloadLen 典型载荷长度；-1 表示可变（见 Note）。
	PayloadLen int
	// Answers 设备是否会对它回帧。
	Answers bool
	// AppParses 官方 app 的回帧分发表里有它的处理器（机械枚举）。
	AppParses bool
	// Note 调试面板显示用的一行注记，只写这一行放不下的关键限定。详细依据见 docs/blackshark-brb02-access-notes.md。
	Note string
}

// BlackSharkCommands 命令表，是命令事实的唯一所有者；加/改命令只改这里。
var BlackSharkCommands = []BlackSharkCommandSpec{
	{0x01, "GetFirmwareVersion", BlackSharkDirHostToDevice, 0, true, true, "响应 ASCII 版本"},
	{0x02, "SetOnOffStatus", BlackSharkDirHostToDevice, 2, true, true, "智能启停 + 通电自启两路向量；应答 a5 05 02 00"},
	{0x03, "GetOnOffStatus", BlackSharkDirHostToDevice, 0, true, true, "读同一个向量"},
	{0x04, "GetConfigSnapshot", BlackSharkDirHostToDevice, 0, true, false, "回帧恒 a5 05 04 00，官方不解析"},
	{0x05, "EnterBootMode", BlackSharkDirHostToDevice, -1, true, true, "进 BOOT，本工具不发"},
	{0x06, "", BlackSharkDirDeviceToHost, 3, false, true, "设备周期上报 [rpm u16le][01]，约 2 Hz"},
	{0x07, "SendBlackSharkHostInfoFrame", BlackSharkDirBoth, 22, true, false, "主机发 7 组 [id][u16le]；设备回 5 字节，约 1 Hz"},
	{0x08, "SetSceneMode", BlackSharkDirHostToDevice, 1, true, false, "官方只在启动发一次，语义未接入"},
	{0x10, "SetRgbLightingSwitch", BlackSharkDirHostToDevice, 1, true, true, ""},
	{0x11, "GetRgbLightingSwitch", BlackSharkDirHostToDevice, 0, true, true, ""},
	{0x12, "SetRgbLightingEffects", BlackSharkDirHostToDevice, 8, true, true, "黄金向量已钉住"},
	{0x13, "GetCurRgbLightingEffects", BlackSharkDirHostToDevice, 0, true, true, ""},
	{0x14, "GetAnyRgbLightingEffects", BlackSharkDirHostToDevice, 1, true, true, ""},
	{0x15, "IssueAudioSpectrumLevel", BlackSharkDirHostToDevice, 1, false, false, "音频同步；主机按帧推档位 1/2/3，约 4.9 Hz"},
	{0x16, "IssueMouseKeyPress", BlackSharkDirHostToDevice, 0, false, false, "响应灯效；每次键鼠按下推一帧"},
	{0x20, "SetFanSwitch", BlackSharkDirHostToDevice, 1, false, false, "无读回命令，只能记最近下发值"},
	{0x22, "SetCoolingSource", BlackSharkDirHostToDevice, 1, true, true, ""},
	{0x23, "GetCoolingSource", BlackSharkDirHostToDevice, 0, true, false, "回帧官方不解析"},
	{0x24, "SetCoolingConfig", BlackSharkDirHostToDevice, -1, true, true, "5（固定）或 15（曲线）；黄金向量已钉住"},
	{0x25, "GetCurCoolingConfig", BlackSharkDirHostToDevice, 0, true, true, ""},
	{0x26, "GetAnyCoolingConfig", BlackSharkDirHostToDevice, 2, true, true, ""},
	// 0x27 载荷语义未定，本工具暂不发（app 侧 IssueSystemSwitch / DLL 侧 coolerSetDeviceSwitch）。
	{0x27, "IssueSystemSwitch", BlackSharkDirHostToDevice, 1, false, false, "载荷语义未定，本工具不发"},
	{0xC0, "SetLcdScreenSwitch", BlackSharkDirHostToDevice, 1, true, true, ""},
	{0xC1, "GetLcdScreenSwitch", BlackSharkDirHostToDevice, 0, true, true, ""},
	{0xC2, "SetLcdShowPos", BlackSharkDirHostToDevice, -1, true, false, "3+4n（条目 4 字节）；会回 ACK，且 ACK 不带参数值"},
	{0xC4, "SetScreensaverImageInfo", BlackSharkDirHostToDevice, 11, true, true, "上传信息帧；数据走 0xA4 族"},
	{0xC5, "GetScreensaverImageInfo", BlackSharkDirHostToDevice, 0, true, true, "响应 11 字节"},
	{0xC6, "SendLongPacket", BlackSharkDirHostToDevice, 0, true, true, "图像数据流控，与每条 0xA4 配对"},
	{0xC7, "GetScreensaverImageData", BlackSharkDirHostToDevice, 0, true, true, "起读屏保图；先答 a5 05 c7 00"},
	{0xC8, "SendLongPacketReply", BlackSharkDirHostToDevice, 1, true, true, "取下一帧（载荷恒 00）；回一条 0xA4"},
	{0xF0, "RestoreFactorySettings", BlackSharkDirHostToDevice, -1, true, true, "恢复出厂，本工具不发"},
}

// BlackSharkCommandByCmd 按命令码查表，找不到返回 false。
func BlackSharkCommandByCmd(cmd byte) (BlackSharkCommandSpec, bool) {
	for _, s := range BlackSharkCommands {
		if s.Cmd == cmd {
			return s, true
		}
	}
	return BlackSharkCommandSpec{}, false
}

// 0xA4 数据族的切分常量。
const (
	// BlackSharkImageFramePayload 一条 0xA4 满帧的载荷长度（64 - 5 头 - 1 CK = 58）。
	BlackSharkImageFramePayload = 58
	// BlackSharkScreenImageBytes 一张屏图的字节数：428×142 RGB565。
	// 与 screenimg.PayloadSize 同值；deviceproto 不 import screenimg（分层），故各自定义，
	// 一致性由 screenimg 侧的测试钉住。
	BlackSharkScreenImageBytes = 428 * 142 * 2
	// BlackSharkScreenImageFrames 一张屏图的报文条数 = ceil(121552/58) = 2096
	//（2095 个 64B 满帧 + 1 个 48B 收尾帧）。设备用它做递减计数器（首帧额外置 bit15）。
	BlackSharkScreenImageFrames = (BlackSharkScreenImageBytes + BlackSharkImageFramePayload - 1) / BlackSharkImageFramePayload

	// BlackSharkImageFlowAccepted 是 0xC6 流控应答里的状态字节：设备收下这一块并推进块号。
	BlackSharkImageFlowAccepted byte = 0x00
	// BlackSharkImageFlowRejected 是 0xC6 流控应答里的状态字节：设备拒收这一块（数据窗口未开）。
	// 它是图传成败的**唯一直接证据** —— 回读（0xC5）只是设备把手上那份元数据原样回显，
	// 整轮被拒也照样"对得上"。
	BlackSharkImageFlowRejected byte = 0x0C
)

// BlackSharkImageFrame 是 0xA4 数据族的一帧（屏保图像数据 / 预览动画帧），
// 布局与上传侧一致（internal/screenimg 的编码器）。
type BlackSharkImageFrame struct {
	Sequence uint16 // [2..3] u16le
	Tag      byte   // [4]
	Payload  []byte // [5..LEN-2]
}

// ParseBlackSharkImageFrame 解析 0xA4 数据帧（报告里可能带 1 字节 report id 前缀）。
func ParseBlackSharkImageFrame(report []byte) (BlackSharkImageFrame, bool) {
	b := report
	// 允许前缀 1 字节 report id（读上来的报文常常带）
	if len(b) >= 2 && b[0] != BlackSharkCmdImageBlock && b[1] == BlackSharkCmdImageBlock {
		b = b[1:]
	}
	if len(b) < 6 || b[0] != BlackSharkCmdImageBlock {
		return BlackSharkImageFrame{}, false
	}
	total := int(b[1])
	if total < 6 || total > len(b) || total > BlackSharkMaxFrameLength {
		return BlackSharkImageFrame{}, false
	}
	return BlackSharkImageFrame{
		Sequence: uint16(b[2]) | uint16(b[3])<<8,
		Tag:      b[4],
		Payload:  append([]byte(nil), b[5:total-1]...),
	}, true
}

// BlackSharkChecksumRule 是「设备→主机回帧」的校验规则。
//
// 两条规则各自覆盖不同命令（见 BlackSharkChecksumRules）。解析时两条都会试，
// 但把「表里登记哪条」与「实际命中哪条」一并带出：两者不一致本身就是诊断信号
// （例如 BLE 传输上的 0x07 用字节和，而同命令在 USB 上走 CRC16）。
type BlackSharkChecksumRule string

const (
	// BlackSharkChecksumRuleSum 前置字节和取低 8 位。
	BlackSharkChecksumRuleSum BlackSharkChecksumRule = "sum"
	// BlackSharkChecksumRuleCRC16 CRC16-CCITT（poly 0x1021、init 0）低字节。
	BlackSharkChecksumRuleCRC16 BlackSharkChecksumRule = "crc16"
	// BlackSharkChecksumRuleUnknown 该命令未登记规则（解析时两条都试）。
	BlackSharkChecksumRuleUnknown BlackSharkChecksumRule = ""
)

// BlackSharkChecksumRules 是「命令码 → 回帧校验规则」的唯一所有者；加/改命令只改这里。
//
// 方向前提：本表只描述「设备→主机」的应答。主机→设备一律字节和（设备不校验）。
// 依据见 docs/blackshark-brb02-access-notes.md §3.2：应答用 CRC16-CCITT 低字节，
// 主动上报的 0x06 用前置字节和；0x01/0x10/0x11/0x13/0xC0 的应答经实算复核后并入。
var BlackSharkChecksumRules = map[byte]BlackSharkChecksumRule{
	0x01: BlackSharkChecksumRuleCRC16,
	0x02: BlackSharkChecksumRuleCRC16,
	0x06: BlackSharkChecksumRuleSum,
	0x07: BlackSharkChecksumRuleCRC16,
	0x08: BlackSharkChecksumRuleCRC16,
	0x10: BlackSharkChecksumRuleCRC16,
	0x11: BlackSharkChecksumRuleCRC16,
	0x13: BlackSharkChecksumRuleCRC16,
	0x14: BlackSharkChecksumRuleCRC16,
	0x24: BlackSharkChecksumRuleCRC16,
	0x25: BlackSharkChecksumRuleCRC16,
	0x26: BlackSharkChecksumRuleCRC16,
	0xC0: BlackSharkChecksumRuleCRC16,
	0xC1: BlackSharkChecksumRuleCRC16,
	0xC4: BlackSharkChecksumRuleCRC16,
	0xC5: BlackSharkChecksumRuleCRC16,
	0xC6: BlackSharkChecksumRuleCRC16,
}

// BlackSharkChecksumRuleFor 查表；未登记返回 (BlackSharkChecksumRuleUnknown, false)。
func BlackSharkChecksumRuleFor(cmd byte) (BlackSharkChecksumRule, bool) {
	rule, ok := BlackSharkChecksumRules[cmd]
	return rule, ok
}

// BlackSharkChecksumMatches 判断「body 的校验字节应当等于 ck」在 rule 下是否成立。
// rule 为 BlackSharkChecksumRuleUnknown 时恒为 false（未登记 ⇒ 没有可断言的规则）。
func BlackSharkChecksumMatches(body []byte, ck byte, rule BlackSharkChecksumRule) bool {
	switch rule {
	case BlackSharkChecksumRuleSum:
		return BlackSharkChecksum(body) == ck
	case BlackSharkChecksumRuleCRC16:
		return BlackSharkRXChecksum(body) == ck
	}
	return false
}

// BlackSharkFrame 解析后的协议帧。
type BlackSharkFrame struct {
	ReportID   byte
	Length     byte
	Command    byte
	Payload    []byte
	Checksum   byte
	ChecksumOK bool
	// ChecksumExpected 是表里为该命令登记的规则（未登记为 BlackSharkChecksumRuleUnknown）。
	ChecksumExpected BlackSharkChecksumRule
	// ChecksumMatched 是本次实际命中的规则（都没命中为 BlackSharkChecksumRuleUnknown）。
	ChecksumMatched BlackSharkChecksumRule
	Frame           []byte
}

// ChecksumRuleMismatch 报告「登记规则」与「命中规则」不一致——未登记或未命中时为 false。
// 这不是"帧无效"，只是"与我们的表不符"，值得一条诊断日志。
func (f BlackSharkFrame) ChecksumRuleMismatch() bool {
	return f.ChecksumExpected != BlackSharkChecksumRuleUnknown &&
		f.ChecksumMatched != BlackSharkChecksumRuleUnknown &&
		f.ChecksumExpected != f.ChecksumMatched
}

// BlackSharkChecksum 计算 CK = 所有前置字节之和取低 8 位。
func BlackSharkChecksum(body []byte) byte {
	var sum uint16
	for _, b := range body {
		sum += uint16(b)
	}
	return byte(sum & 0xFF)
}

// BlackSharkRXChecksum 计算设备→主机回帧的校验字节：
// CRC16-CCITT(poly 0x1021, init 0x0000) 的低字节（不反转、不异或输出）。
func BlackSharkRXChecksum(body []byte) byte {
	var crc uint16
	for _, b := range body {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return byte(crc & 0xFF)
}

// LCD 显示参数（0xC2）的条目数上界。
const BlackSharkLcdShowPosMaxEntries = 3

// BlackSharkLcdSlotGeometry 是每格的 3 个常量（b1 u16le 与 b3）。
type BlackSharkLcdSlotGeometry struct {
	// A 是每格 3 字节里的前 2 个（u16le）。
	A uint16
	// B 是第 3 个字节。
	B byte
}

// BlackSharkLcdSlotGeometryPos0 是 pos = 0 时三格的几何常量。
var BlackSharkLcdSlotGeometryPos0 = [BlackSharkLcdShowPosMaxEntries]BlackSharkLcdSlotGeometry{
	{A: 10, B: 100},  // 第 1 格
	{A: 143, B: 100}, // 第 2 格
	{A: 276, B: 100}, // 第 3 格
}

// BlackSharkLcdCalibratedPosMax 是已标定的 pos 上界，目前只有 0。
const BlackSharkLcdCalibratedPosMax = 0

// BuildBlackSharkLcdShowPosPayload 构造 0xC2 的载荷：
//
//	[0x00] + [pos u8] + [条数 u8] + N × ( [id u8][A u16le][B u8] )
func BuildBlackSharkLcdShowPosPayload(pos byte, entries []BlackSharkSystemInfoEntry) ([]byte, bool) {
	if int(pos) > BlackSharkLcdCalibratedPosMax {
		return nil, false
	}
	if len(entries) > BlackSharkLcdShowPosMaxEntries {
		return nil, false
	}
	// 设备侧不校验 id（原样透传），只能在这里拦：放一个不在候选表里的 id 过去，
	// 屏上要么显示空位、要么显示别的项，而界面上完全看不出来。宁可拒绝。
	for _, e := range entries {
		if !IsBlackSharkLcdSelectableItem(e.ID) {
			return nil, false
		}
	}
	out := make([]byte, 0, 3+4*len(entries))
	out = append(out, 0x00, pos, byte(len(entries)))
	for i, e := range entries {
		geo := BlackSharkLcdSlotGeometryPos0[i]
		out = append(out, e.ID, byte(geo.A), byte(geo.A>>8), geo.B)
	}
	return out, true
}

// BuildBlackSharkFrame 构造 A5 帧（不含 Report ID 与填充）。
func BuildBlackSharkFrame(cmd byte, payload ...byte) []byte {
	frame := make([]byte, 0, 4+len(payload))
	frame = append(frame, BlackSharkMagic, byte(4+len(payload)), cmd)
	frame = append(frame, payload...)
	frame = append(frame, BlackSharkChecksum(frame))
	return frame
}

// BuildBlackSharkReport 把帧封装成 HIDAPI/HIDClass 形态的报文：ReportID + 帧 + 0x00 填充。
// reportLen <= 0 时使用默认的 65 字节。
//
// 只服务 HIDAPI 一条传输：libusb 直连端点没有 Report ID 这一层，那条路要的是帧@0 加补零到 65。
// 按传输选封套由设备层负责（device.blackSharkBuildReportLocked，判据是句柄是否走 libusb，即 usb != nil），
// 别用本函数去凑 libusb 的形状。两种形态的对照见 docs/blackshark-brb02-access-notes.md §3.2。
func BuildBlackSharkReport(frame []byte, reportLen int) ([]byte, error) {
	if len(frame) == 0 {
		return nil, fmt.Errorf("blackshark frame is empty")
	}
	if reportLen <= 0 {
		reportLen = BlackSharkReportLength
	}
	if reportLen < len(frame)+1 {
		return nil, fmt.Errorf("blackshark report length %d cannot hold frame of %d bytes", reportLen, len(frame))
	}
	report := make([]byte, reportLen)
	report[0] = BlackSharkReportID
	copy(report[1:], frame)
	return report, nil
}

// blackSharkFrameChecksumOK 判断 data[offset:offset+total] 这段帧的校验字节是否正确（两条规则任一命中）。
func blackSharkFrameChecksumOK(data []byte, offset, total int) bool {
	if total < 4 || offset < 0 || offset+total > len(data) {
		return false
	}
	body, ck := data[offset:offset+total-1], data[offset+total-1]
	return BlackSharkChecksum(body) == ck || BlackSharkRXChecksum(body) == ck
}

// ParseBlackSharkFrame 解析报文中的 A5 帧。data 可以带也可以不带 Report ID 前缀。
func ParseBlackSharkFrame(data []byte) (BlackSharkFrame, bool) {
	offset := -1
	reportID := byte(0)
	switch {
	case len(data) >= 1 && data[0] == BlackSharkMagic:
		offset = 0
	case len(data) >= 2 && data[1] == BlackSharkMagic:
		offset = 1
		reportID = data[0]
	default:
		return BlackSharkFrame{}, false
	}
	if len(data) < offset+4 {
		return BlackSharkFrame{}, false
	}
	length := int(data[offset+1])
	// 下界取 3：`A5 03 CMD CK`（无载荷查询，LEN = 总长−1）也是合法短形态，见下面的 LEN 容忍。
	if length < 3 || length > BlackSharkMaxFrameLength {
		return BlackSharkFrame{}, false
	}
	// LEN 的语义是「帧总长」（含 A5/LEN/CMD/CK）。但官方与 DLL 的历史构造里存在 LEN = 总长−1
	// 的短形态（如 `A5 03 01 A9`、`A5 13 24 … C8 3B`），固件也接受。因此解析时两条都试，
	// 判据用校验字节：先按 LEN 取帧，校验不过且后面还有字节时，再按 LEN+1 取一次。
	total := length
	if !blackSharkFrameChecksumOK(data, offset, length) &&
		length+1 <= BlackSharkMaxFrameLength && offset+length+1 <= len(data) &&
		blackSharkFrameChecksumOK(data, offset, length+1) {
		total = length + 1
	}
	if len(data) < offset+total {
		return BlackSharkFrame{}, false
	}
	frame := make([]byte, total)
	copy(frame, data[offset:offset+total])

	payloadLen := total - 4
	payload := make([]byte, payloadLen)
	copy(payload, frame[3:3+payloadLen])

	body, ck := frame[:total-1], frame[total-1]
	expected, _ := BlackSharkChecksumRuleFor(frame[2])
	matched := BlackSharkChecksumRuleUnknown
	switch {
	case BlackSharkChecksumMatches(body, ck, BlackSharkChecksumRuleCRC16):
		matched = BlackSharkChecksumRuleCRC16
	case BlackSharkChecksumMatches(body, ck, BlackSharkChecksumRuleSum):
		matched = BlackSharkChecksumRuleSum
	}

	return BlackSharkFrame{
		ReportID: reportID,
		Length:   byte(total),
		Command:  frame[2],
		Payload:  payload,
		Checksum: ck,
		// CK 不是有效性机制（设备收帧不校验），因此不能用 ChecksumOK 判定帧是否有效。
		// 规则登记在 BlackSharkChecksumRules；这里两条都试（未登记命令也照样试），并把
		// 「登记哪条」与「实际命中哪条」一并带出——两者不一致本身就是诊断信号。
		ChecksumOK:       matched != BlackSharkChecksumRuleUnknown,
		ChecksumExpected: expected,
		ChecksumMatched:  matched,
		Frame:            frame,
	}, true
}

// BlackSharkStatusRPM 从 0x06 状态帧解析当前转速（RPM）。
// 0x06 帧载荷为 [rpm_lo, rpm_hi, 0x01]。
func BlackSharkStatusRPM(frame BlackSharkFrame) (uint16, bool) {
	if frame.Command != BlackSharkCmdStatusNotify || len(frame.Payload) < 2 {
		return 0, false
	}
	return uint16(frame.Payload[0]) | uint16(frame.Payload[1])<<8, true
}

// DrainBlackSharkFrames 清空响应通道并返回丢弃的帧数。
//
// 设备以约 2Hz 主动推送 0x06 状态帧（[RPM u16le][标志]），这些帧会持续进入响应通道。
func DrainBlackSharkFrames(ch chan BlackSharkFrame) int {
	if ch == nil {
		return 0
	}
	dropped := 0
	for {
		select {
		case <-ch:
			dropped++
		default:
			return dropped
		}
	}
}

// BlackSharkRgbEffectCount 灯效模式数量。
const BlackSharkRgbEffectCount = 8

// BlackSharkRgbEffectNames 模式索引 → 官方 UI 名称（8 个槽位）。
var BlackSharkRgbEffectNames = map[int]string{
	1: "彩色流动",
	2: "彩色循环",
	3: "呼吸",
	4: "常亮",
	5: "闪烁",
	6: "响应",
	7: "音频同步",
	8: "刷新",
}

// BlackSharkRgbColorOptionNames 颜色下拉的选项序号 → 名称（payload[0] 的高半字节）。
var BlackSharkRgbColorOptionNames = map[int]string{
	0: "彩虹",
	1: "蓝紫追逐",
	2: "黄绿",
	3: "红蓝",
	4: "橙紫",
}

// BlackSharkRgbColorOptionCount 颜色下拉的项数（= 5）。
const BlackSharkRgbColorOptionCount = 5

// BlackSharkRgbColorOptionOrder 颜色下拉的有序序号（顺序即官方下拉顺序，供 UI 直接渲染）。
var BlackSharkRgbColorOptionOrder = []int{0, 1, 2, 3, 4}

// BlackSharkHueAvailability 色相滑杆在某灯效页的可用性（官方按页配置）。
type BlackSharkHueAvailability string

const (
	// BlackSharkHueNone 该页没有色相控件（刷新）。
	BlackSharkHueNone BlackSharkHueAvailability = "none"
	// BlackSharkHueDisabled 有，但置灰、永久不可操作（彩色流动 / 彩色循环）。
	BlackSharkHueDisabled BlackSharkHueAvailability = "disabled"
	// BlackSharkHueWhenSingleColor 仅当选中「单色」时可用（呼吸 / 闪烁 / 音频同步）。
	BlackSharkHueWhenSingleColor BlackSharkHueAvailability = "singleColorOnly"
	// BlackSharkHueAlways 正常可用（常亮 / 响应）。
	BlackSharkHueAlways BlackSharkHueAvailability = "always"
)

// BlackSharkRgbColorControls 描述某个灯效页实际显示哪些颜色控件（官方按页配置）。
type BlackSharkRgbColorControls struct {
	// ColorMode 有「颜色模式」下拉（5 项，喂 payload[0] 高半字节），只有彩色流动有。
	ColorMode bool
	// SingleColor 有「单色/彩色」下拉（喂 payload[4]）。
	SingleColor bool
	// Hue 色相滑杆的可用性（见 BlackSharkHue* 常量）。
	Hue BlackSharkHueAvailability
}

// BlackSharkRgbColorControlsBySlot 槽位 → 该页的颜色控件与可用性。
var BlackSharkRgbColorControlsBySlot = map[int]BlackSharkRgbColorControls{
	1: {ColorMode: true, Hue: BlackSharkHueDisabled},
	2: {Hue: BlackSharkHueDisabled},
	3: {SingleColor: true, Hue: BlackSharkHueWhenSingleColor},
	4: {Hue: BlackSharkHueAlways},
	5: {SingleColor: true, Hue: BlackSharkHueWhenSingleColor},
	6: {Hue: BlackSharkHueAlways},
	7: {SingleColor: true, Hue: BlackSharkHueWhenSingleColor},
	8: {Hue: BlackSharkHueNone},
}

// BlackSharkRgbEffectNeedsHostData 标记「设备有该模式，但需要主机持续喂数据才能生效」的模式。
var BlackSharkRgbEffectNeedsHostData = map[int]string{
	6: "响应需要主机监听全局鼠标/键盘事件并逐次下发（官方用 WH_KEYBOARD_LL + WH_MOUSE_LL 两个全局低层钩子，帧为 A5 04 16 CK）。本工具已实现：选中该灯效后会自动开始监听，切走即停",
	7: "音频同步需要主机持续推送主频档位（A5 05 15 档位 CK，约 4.9 Hz）。本工具已实现：选中该灯效后会自动开始采集系统播放声音，切走即停；静音时不下发",
}

// BlackSharkRgbEffects 灯效参数（8 字节）。
//
// 载荷映射来自 Brb02CoolerComm.dll 的 coolerSetRgbLightingEffects() 反汇编。
type BlackSharkRgbEffects struct {
	Field0  uint8  // 低 4 位并入 payload[0]；= 模式索引
	Field4  uint8  // 低 4 位并入 payload[0] 高半字节；= 颜色下拉选项号 comboBoxSubColor.currentIndex()
	Value8  uint16 // payload[1..2] 小端；= 速度
	Field12 uint8  // payload[3]；= 亮度
	Field16 uint8  // payload[4]；= 静态颜色类别标志，0x01 = 吃静态色（"单色"）/ 0x0A = 不吃
	Value20 uint32 // payload[5..7] 大端 24 位；= 颜色 R,G,B
}

// ModeIndex 返回模式索引（1..BlackSharkRgbEffectCount）。
func (e BlackSharkRgbEffects) ModeIndex() int { return int(e.Field0) }

// ColorOption 返回颜色下拉的选项号（payload[0] 高半字节，0..4）；
// 这是设备里真正存着的那一项，名字见 BlackSharkRgbColorOptionNames。
func (e BlackSharkRgbEffects) ColorOption() int { return int(e.Field4) }

// Speed 返回速度参数（与官方 UI 的 Speed 滑块同量纲）。
func (e BlackSharkRgbEffects) Speed() int { return int(e.Value8) }

// Brightness 返回亮度（与官方 UI 的 Brightness 滑块同量纲）。
func (e BlackSharkRgbEffects) Brightness() int { return int(e.Field12) }

// ColorRGB 返回颜色（Value20 的大端三字节）。
func (e BlackSharkRgbEffects) ColorRGB() (r, g, b uint8) {
	return uint8((e.Value20 >> 16) & 0xFF), uint8((e.Value20 >> 8) & 0xFF), uint8(e.Value20 & 0xFF)
}

// BlackSharkRgbSpeedBand 描述灯效「速度」滑块的量程，每个模式各不相同。
type BlackSharkRgbSpeedBand struct {
	Min, Max int
	// Fixed=true：官方 UI 里该模式的速度滑块置灰、不可操作（值恒 0），
	// 恰为两个：常亮(4) 与 音频同步(7)。
	Fixed bool
}

var BlackSharkRgbSpeedRange = map[int]BlackSharkRgbSpeedBand{
	1: {Min: 1500, Max: 4000},         // 彩色流动
	2: {Min: 5000, Max: 15000},        // 彩色循环
	3: {Min: 1000, Max: 5000},         // 呼吸
	4: {Min: 0, Max: 99, Fixed: true}, // 常亮，速度滑块不可用
	5: {Min: 450, Max: 1050},          // 闪烁
	6: {Min: 300, Max: 1000},          // 响应
	7: {Min: 0, Max: 99, Fixed: true}, // 音频同步，速度滑块不可用
	8: {Min: 250, Max: 750},           // 刷新
}

// WithSpeedBrightness 返回一份只改速度与亮度的副本。
func (e BlackSharkRgbEffects) WithSpeedBrightness(speed, brightness int) BlackSharkRgbEffects {
	out := e
	out.Value8 = uint16(clampUint16(speed))
	out.Field12 = uint8(clampUint8(brightness))
	return out
}

// WithColor 返回一份只改颜色的副本（其余字段一律原样保留）。
func (e BlackSharkRgbEffects) WithColor(r, g, b uint8) BlackSharkRgbEffects {
	out := e
	out.Value20 = uint32(r)<<16 | uint32(g)<<8 | uint32(b)
	return out
}

// 静态颜色标志（payload[4]）的两个取值。
const (
	BlackSharkStaticColorOn  byte = 0x01
	BlackSharkStaticColorOff byte = 0x0A
)

// StaticColor 返回该模式当前是否吃静态颜色。
func (e BlackSharkRgbEffects) StaticColor() bool { return e.Field16 == BlackSharkStaticColorOn }

// WithStaticColor 返回一份只改静态颜色标志的副本。
//
// 把标志置 0x01 设备才吃静态色。
func (e BlackSharkRgbEffects) WithStaticColor(on bool) BlackSharkRgbEffects {
	out := e
	if on {
		out.Field16 = BlackSharkStaticColorOn
	} else {
		out.Field16 = BlackSharkStaticColorOff
	}
	return out
}

// WithColorOption 返回一份只改"颜色下拉选项号"（payload[0] 高半字节）的副本。
func (e BlackSharkRgbEffects) WithColorOption(idx int) BlackSharkRgbEffects {
	out := e
	if idx < 0 {
		idx = 0
	}
	if idx > 0x0F {
		idx = 0x0F
	}
	out.Field4 = uint8(idx)
	return out
}

func clampUint16(v int) int {
	if v < 0 {
		return 0
	}
	if v > 0xFFFF {
		return 0xFFFF
	}
	return v
}

func clampUint8(v int) int {
	if v < 0 {
		return 0
	}
	if v > 0xFF {
		return 0xFF
	}
	return v
}

// EncodeBlackSharkRgbEffects 生成 8 字节灯效载荷。
func (e BlackSharkRgbEffects) EncodeBlackSharkRgbEffects() []byte {
	out := make([]byte, 8)
	out[0] = (e.Field4 << 4) | (e.Field0 & 0x0F)
	out[1] = byte(e.Value8 & 0xFF)
	out[2] = byte(e.Value8 >> 8)
	out[3] = e.Field12
	out[4] = e.Field16
	out[5] = byte((e.Value20 >> 16) & 0xFF)
	out[6] = byte((e.Value20 >> 8) & 0xFF)
	out[7] = byte(e.Value20 & 0xFF)
	return out
}

// DecodeBlackSharkRgbEffects 从 0x13/0x14 响应载荷还原灯效参数。
func DecodeBlackSharkRgbEffects(payload []byte) (BlackSharkRgbEffects, bool) {
	if len(payload) < 8 {
		return BlackSharkRgbEffects{}, false
	}
	return BlackSharkRgbEffects{
		Field0:  payload[0] & 0x0F,
		Field4:  payload[0] >> 4,
		Value8:  uint16(payload[1]) | uint16(payload[2])<<8,
		Field12: payload[3],
		Field16: payload[4],
		Value20: uint32(payload[5])<<16 | uint32(payload[6])<<8 | uint32(payload[7]),
	}, true
}

// BlackSharkBlockedCommands 绝不允许下发的命令码，是黑鲨危险命令黑名单的唯一定义。
var BlackSharkBlockedCommands = map[byte]string{
	BlackSharkCmdEnterBootMode:  "进入 BOOT 模式（固件升级用；本工具不刷固件）",
	BlackSharkCmdRestoreFactory: "恢复出厂设置（不可逆；本工具不提供该功能）",
	0xF1:                        "产线测试命令（FactoryTestSetLight）",
	0xF2:                        "产线测试命令（FactoryTestSetFan）",
	0xF3:                        "产线测试命令（FactoryTest*）",
	0xF4:                        "产线测试命令（FactoryTestSetMode）",
	0xF5:                        "产线测试命令（FactoryTestKeyPress）",
}

// BlackSharkIsBlockedCommand 判断命令码是否被封锁，返回原因。
func BlackSharkIsBlockedCommand(cmd byte) (string, bool) {
	reason, blocked := BlackSharkBlockedCommands[cmd]
	return reason, blocked
}

// BlackSharkSwitchPayload 把布尔开关转换成单字节载荷。
// 设备侧约定：0x01 = 开，0x00 = 关。
func BlackSharkSwitchPayload(enabled bool) []byte {
	if enabled {
		return []byte{0x01}
	}
	return []byte{0x00}
}

// BlackSharkOnOffVector 0x02 的两路开关状态。
type BlackSharkOnOffVector struct {
	// SmartStartStop 智能启停（官方 UI: checkBoxSmart_start_and_stop）。
	SmartStartStop bool `json:"smartStartStop"`
	// PowerOnSelfStart 通电自启（官方 UI: checkBoxPowerOnSelfStarting）。
	PowerOnSelfStart bool `json:"powerOnSelfStart"`
}

// BlackSharkOnOffVectorPayload 编码 0x02 的 2 字节载荷 [A][B]。
//
// 字节序：A = 智能启停，B = 通电自启，0x01=开 / 0x00=关。
func BlackSharkOnOffVectorPayload(v BlackSharkOnOffVector) []byte {
	return []byte{boolByte(v.SmartStartStop), boolByte(v.PowerOnSelfStart)}
}

// ParseBlackSharkOnOffVectorPayload 解析 0x02 的 2 字节载荷（写方向）。
// 长度不为 2 一律判失败，不做补齐或截断。
func ParseBlackSharkOnOffVectorPayload(payload []byte) (BlackSharkOnOffVector, bool) {
	if len(payload) != 2 {
		return BlackSharkOnOffVector{}, false
	}
	return BlackSharkOnOffVector{
		SmartStartStop:   payload[0] != 0,
		PowerOnSelfStart: payload[1] != 0,
	}, true
}

// BlackSharkOnOffVectorFromStatus 从 0x03 的回复帧解析两路开关向量。
func BlackSharkOnOffVectorFromStatus(frame BlackSharkFrame) (BlackSharkOnOffVector, bool) {
	if frame.Command != BlackSharkCmdGetOnOffVector {
		return BlackSharkOnOffVector{}, false
	}
	return ParseBlackSharkOnOffVectorPayload(frame.Payload)
}

func boolByte(b bool) byte {
	if b {
		return 0x01
	}
	return 0x00
}

// BlackSharkSingleByteState 解析「单字节状态」类响应帧（0x11 / 0xC1 / 0x23）。
func BlackSharkSingleByteState(frame BlackSharkFrame, wantCmd byte) (uint8, bool) {
	if frame.Command != wantCmd || len(frame.Payload) < 1 {
		return 0, false
	}
	return frame.Payload[0], true
}

// BlackSharkSwitchEnabled 把单字节状态换算成布尔（非 0 视为开）。
func BlackSharkSwitchEnabled(state uint8) bool {
	return state != 0
}

// BlackSharkFirmwareVersion 从 0x01 响应解析固件版本字符串。
func BlackSharkFirmwareVersion(frame BlackSharkFrame) (string, bool) {
	if frame.Command != BlackSharkCmdVersion || len(frame.Payload) == 0 {
		return "", false
	}
	return string(frame.Payload), true
}

// BlackSharkCommandDescription 返回命令码的可读说明，从 BlackSharkCommands 表里取。
func BlackSharkCommandDescription(cmd byte) string {
	spec, ok := BlackSharkCommandByCmd(cmd)
	if !ok {
		return fmt.Sprintf("unknown command 0x%02X", cmd)
	}
	name := spec.Name
	if name == "" {
		// 表里有几条（如 0x06 设备→主机状态上报）没有官方 mission 名。
		name = fmt.Sprintf("cmd 0x%02X", spec.Cmd)
	}
	if spec.Note == "" {
		return name
	}
	return name + " — " + spec.Note
}

// ---- 点灯请求构造 ----

const (
	BlackSharkImageWidth      = 428
	BlackSharkImageHeight     = 142
	BlackSharkImageBytes      = BlackSharkImageWidth * BlackSharkImageHeight * 2
	blackSharkImageBlockBytes = 58
)

const (
	BlackSharkModeColorCycle byte = 0x02
	BlackSharkModeBreathing  byte = 0x03
	BlackSharkModeStatic     byte = 0x04
	BlackSharkModeFlashing   byte = 0x05
	BlackSharkModeRefresh    byte = 0x06
	BlackSharkModeResponse   byte = 0x08
	BlackSharkModeColorFlow  byte = 0x11
)

// blackSharkChecksum 转发到唯一实现 BlackSharkChecksum。
func blackSharkChecksum(data []byte) byte { return BlackSharkChecksum(data) }

func buildBlackSharkFrameWithLength(length byte, command byte, params ...byte) []byte {
	body := make([]byte, 0, int(length)+1)
	body = append(body, 0xA5, length, command)
	body = append(body, params...)
	return append(body, blackSharkChecksum(body))
}

func BuildBlackSharkGetVersion() []byte { return BuildBlackSharkFrame(BlackSharkCmdVersion) }

func BuildBlackSharkGetStatus() []byte { return BuildBlackSharkFrame(BlackSharkCmdGetStatus) }

// BuildBlackSharkSetSpeed 构造固定值设定帧（走 BLE / HID 的旧路径）。
// 上限取设备可达转速区间的所有者，不再自带一份数。
func BuildBlackSharkSetSpeed(rpm int) []byte {
	if rpm < 0 {
		rpm = 0
	}
	if rpm > BlackSharkMaxRPM {
		rpm = BlackSharkMaxRPM
	}
	// v3.0.3 write frames use the device's long-form LEN field.
	return buildBlackSharkFrameWithLength(0x09, BlackSharkCmdSetSpeed, 0x00, 0x00, 0x01, byte(rpm), byte(rpm>>8))
}

// BlackSharkModeName uses the same user-facing work-mode labels as FlyDigi.
func BlackSharkModeName(mode byte) string {
	return ModeName(mode)
}

func BuildBlackSharkSetRGBEnabled(enabled bool) []byte {
	value := byte(0)
	if enabled {
		value = 1
	}
	return BuildBlackSharkFrame(BlackSharkCmdRGBEnable, value)
}

func BuildBlackSharkGetLighting() []byte { return BuildBlackSharkFrame(BlackSharkCmdGetLighting) }

func blackSharkLightingMode(mode string) (byte, bool) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "blackshark_color_cycle", "rotation":
		return BlackSharkModeColorCycle, true
	case "blackshark_breathing", "breathing", "smart_temp":
		return BlackSharkModeBreathing, true
	case "blackshark_static", "static_single":
		return BlackSharkModeStatic, true
	case "static_multi":
		// BRB02 has no confirmed multi-zone static mode; use its native
		// color-cycle mode while keeping the common frontend option available.
		return BlackSharkModeColorCycle, true
	case "blackshark_flashing":
		return BlackSharkModeFlashing, true
	case "blackshark_refresh":
		return BlackSharkModeRefresh, true
	case "blackshark_response":
		return BlackSharkModeResponse, true
	case "blackshark_color_flow", "flowing":
		return BlackSharkModeColorFlow, true
	default:
		return 0, false
	}
}

func blackSharkLightingSpeed(speed string) uint16 {
	switch strings.ToLower(strings.TrimSpace(speed)) {
	case "fast":
		return 1000
	case "slow":
		return 5000
	default:
		return 3000
	}
}

func BuildBlackSharkSetLighting(modeName, speedName string, brightness int, r, g, b byte) ([]byte, error) {
	mode, ok := blackSharkLightingMode(modeName)
	if !ok {
		return nil, fmt.Errorf("unsupported Black Shark lighting mode %q", modeName)
	}
	if brightness < 0 {
		brightness = 0
	}
	if brightness > 100 {
		brightness = 100
	}
	colorMode := byte(0x01)
	if mode == BlackSharkModeColorCycle || mode == BlackSharkModeColorFlow {
		colorMode = 0x0A
	}
	speed := make([]byte, 2)
	binary.LittleEndian.PutUint16(speed, blackSharkLightingSpeed(speedName))
	return buildBlackSharkFrameWithLength(
		0x0C,
		BlackSharkCmdSetLighting,
		mode, speed[0], speed[1], byte(brightness), colorMode, r, g, b,
	), nil
}

// ---- 状态解析 / RGB565 / CRC16 / 屏保图像上传 ----

func ParseBlackSharkStatus(frame BlackSharkFrame) (currentRPM int, flag byte, ok bool) {
	if frame.Command == BlackSharkCmdStatusNotify && len(frame.Payload) >= 3 {
		return int(binary.LittleEndian.Uint16(frame.Payload[:2])), frame.Payload[2], true
	}
	if frame.Command == BlackSharkCmdGetStatus && len(frame.Payload) >= 5 {
		return int(binary.LittleEndian.Uint16(frame.Payload[3:5])), frame.Payload[2], true
	}
	return 0, 0, false
}

// BlackSharkRGB565 converts a 428x142 image to the display's RGB565 big-endian canvas.
func BlackSharkRGB565(src image.Image) ([]byte, error) {
	if src == nil {
		return nil, fmt.Errorf("black shark image is nil")
	}
	bounds := src.Bounds()
	if bounds.Dx() != BlackSharkImageWidth || bounds.Dy() != BlackSharkImageHeight {
		return nil, fmt.Errorf("black shark image must be %dx%d, got %dx%d", BlackSharkImageWidth, BlackSharkImageHeight, bounds.Dx(), bounds.Dy())
	}
	canvas := make([]byte, 0, BlackSharkImageBytes)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := src.At(x, y).RGBA()
			pixel := uint16(r>>11)<<11 | uint16(g>>10)<<5 | uint16(b>>11)
			canvas = append(canvas, byte(pixel>>8), byte(pixel))
		}
	}
	return canvas, nil
}

// BlackSharkCRC16XMODEM returns CRC-16/XMODEM (poly 0x1021, init 0).
func BlackSharkCRC16XMODEM(data []byte) uint16 {
	var crc uint16
	for _, value := range data {
		crc ^= uint16(value) << 8
		for bit := 0; bit < 8; bit++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func validateBlackSharkCanvas(canvas []byte) error {
	if len(canvas) != BlackSharkImageBytes {
		return fmt.Errorf("black shark canvas must contain %d bytes, got %d", BlackSharkImageBytes, len(canvas))
	}
	return nil
}

// BuildBlackSharkImageMetadata builds the C4 image handshake frame.
func BuildBlackSharkImageMetadata(canvas []byte) ([]byte, error) {
	if err := validateBlackSharkCanvas(canvas); err != nil {
		return nil, err
	}
	metadata := []byte{0x18, 0xDE, 0xC0, 0x6A, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(metadata[4:8], uint32(len(canvas)))
	binary.LittleEndian.PutUint16(metadata[8:10], BlackSharkCRC16XMODEM(canvas))
	return buildBlackSharkFrameWithLength(0x0F, BlackSharkCmdImageMetadata, metadata...), nil
}

// BuildBlackSharkImageDataBlock builds one 64-byte A4 data block. Block 2096 is first.
func BuildBlackSharkImageDataBlock(blockNumber int, data []byte, first bool) ([]byte, error) {
	if blockNumber < 2 || blockNumber > 2096 {
		return nil, fmt.Errorf("black shark image block number %d out of range", blockNumber)
	}
	if len(data) != blackSharkImageBlockBytes {
		return nil, fmt.Errorf("black shark image data block must contain %d bytes, got %d", blackSharkImageBlockBytes, len(data))
	}
	high := byte(blockNumber>>8) & 0x0F
	if first {
		high |= 0x80
	}
	params := append([]byte{byte(blockNumber), high, 0xC6}, data...)
	return buildBlackSharkImageDataFrame(0x40, params), nil
}

// BuildBlackSharkImageTail builds the 48-byte A4 tail frame for block 1.
func BuildBlackSharkImageTail(data []byte) ([]byte, error) {
	if len(data) != 42 {
		return nil, fmt.Errorf("black shark image tail must contain 42 bytes, got %d", len(data))
	}
	params := append([]byte{0x01, 0x00, 0xC6}, data...)
	return buildBlackSharkImageDataFrame(0x30, params), nil
}

func buildBlackSharkImageDataFrame(length byte, params []byte) []byte {
	frame := make([]byte, 0, int(length))
	frame = append(frame, BlackSharkCmdImageBlock, length)
	frame = append(frame, params...)
	return append(frame, blackSharkChecksum(frame))
}

// BuildBlackSharkImageCommit builds the C5 image commit frame.
func BuildBlackSharkImageCommit() []byte {
	return []byte{0xA5, 0x04, BlackSharkCmdImageCommit, 0x6E}
}

// BuildBlackSharkImageUpload builds the complete C4, A4, tail, and C5 sequence.
func BuildBlackSharkImageUpload(canvas []byte) ([][]byte, error) {
	if err := validateBlackSharkCanvas(canvas); err != nil {
		return nil, err
	}
	frames := make([][]byte, 0, 2098)
	metadata, err := BuildBlackSharkImageMetadata(canvas)
	if err != nil {
		return nil, err
	}
	frames = append(frames, metadata)
	for block := 2096; block >= 2; block-- {
		start := (2096 - block) * blackSharkImageBlockBytes
		data, err := BuildBlackSharkImageDataBlock(block, canvas[start:start+blackSharkImageBlockBytes], block == 2096)
		if err != nil {
			return nil, err
		}
		frames = append(frames, data)
	}
	tail, err := BuildBlackSharkImageTail(canvas[2095*blackSharkImageBlockBytes:])
	if err != nil {
		return nil, err
	}
	frames = append(frames, tail, BuildBlackSharkImageCommit())
	return frames, nil
}

// ---- 冷却：档位 / 形态 / 曲线 / 参照源 / 出厂常量 ----

// 冷却配置（0x24 写 / 0x25 读当前 / 0x26 读任一）的形态与参照源常量。
const (
	// BlackSharkFormValue 固定值形态（固定转速）。
	BlackSharkFormValue byte = 0x00
	// BlackSharkFormCurve 温度曲线形态（变频模式）。
	BlackSharkFormCurve byte = 0x01

	// BlackSharkCoolingSourceCPU / BlackSharkCoolingSourceGPU 是 0x24 载荷 byte[0] 的候选语义（CPU / GPU 参照）。
	BlackSharkCoolingSourceCPU byte = 0x00
	BlackSharkCoolingSourceGPU byte = 0x01
	// BlackSharkCoolingSourceCount 参照源取值个数；设备对大于 1 的值钳到 GPU。
	BlackSharkCoolingSourceCount = 2

	// BlackSharkGearCount 档位数量；1..4 与官方 UI 的
	// radioButtonLowNoise / Balance / Powerful / Extreme 顺序一致。
	BlackSharkGearCount = 4
	// BlackSharkCurvePointCount 每条温度曲线的点数。
	BlackSharkCurvePointCount = 4
)

// BlackSharkCurvePoint 温度曲线上的一个点。
// 注意：曲线里的 RPM 字段与固定值形态的 sp 不是同一套标度，切勿混用
// （两张对照表见 blackshark_calibration.go）。
type BlackSharkCurvePoint struct {
	TempC uint8
	RPM   uint16
}

// BlackSharkGearConfig 一个档位的冷却配置（两种形态共用）。
type BlackSharkGearConfig struct {
	// Source 是载荷 0x24 的第 1 个字节 = 冷却参照源（0x00 = CPU / 0x01 = GPU）。
	Source byte
	Form   byte
	Gear   byte
	Value  uint16                                          // form == BlackSharkFormValue 时有效
	Curve  [BlackSharkCurvePointCount]BlackSharkCurvePoint // form == BlackSharkFormCurve 时有效
}

// BlackSharkNormalizeCoolingSource 把任意值收敛到合法的参照源（0x00 CPU / 0x01 GPU）。
// 写入 2 / 3 都读回 1，即非 CPU 一律钳到 GPU。
func BlackSharkNormalizeCoolingSource(v byte) byte {
	if v == BlackSharkCoolingSourceCPU {
		return BlackSharkCoolingSourceCPU
	}
	return BlackSharkCoolingSourceGPU
}

// Encode 生成 0x24 的写入载荷。
func (c BlackSharkGearConfig) Encode() []byte {
	src := BlackSharkNormalizeCoolingSource(c.Source)
	if c.Form == BlackSharkFormCurve {
		out := make([]byte, 0, 3+3*BlackSharkCurvePointCount)
		out = append(out, src, c.Form, c.Gear)
		for _, p := range c.Curve {
			out = append(out, p.TempC, byte(p.RPM&0xFF), byte(p.RPM>>8))
		}
		return out
	}
	return []byte{src, c.Form, c.Gear, byte(c.Value & 0xFF), byte(c.Value >> 8)}
}

// EncodeBlackSharkFixedSpeedPayload 生成「固定转速」写入载荷。
// source（参照源）是必填参数，逼每个调用点明确回答用的是 CPU 还是 GPU。
func EncodeBlackSharkFixedSpeedPayload(gear byte, value uint16, source byte) []byte {
	return BlackSharkGearConfig{
		Source: source, Form: BlackSharkFormValue, Gear: gear, Value: value,
	}.Encode()
}

// EncodeBlackSharkCurvePayload 生成「变频曲线」写入载荷；source 同上必填。
func EncodeBlackSharkCurvePayload(gear byte, points [BlackSharkCurvePointCount]BlackSharkCurvePoint, source byte) []byte {
	return BlackSharkGearConfig{
		Source: source, Form: BlackSharkFormCurve, Gear: gear, Curve: points,
	}.Encode()
}

// EncodeBlackSharkGearQueryPayload 生成 0x26 的读取请求载荷。
// 形式为 [form, gear]（请求没有参照源字节，回复才有）。
func EncodeBlackSharkGearQueryPayload(form, gear byte) []byte {
	return []byte{form, gear}
}

// DecodeBlackSharkGearConfig 解析配置载荷（`0x25` 的回复，或 `0x26` 的回复）。
// 支持 5 字节（固定值）与 15 字节（曲线）两种长度。
func DecodeBlackSharkGearConfig(payload []byte) (BlackSharkGearConfig, bool) {
	switch len(payload) {
	case 5:
		return BlackSharkGearConfig{
			Source: payload[0], Form: payload[1], Gear: payload[2],
			Value: uint16(payload[3]) | uint16(payload[4])<<8,
		}, true
	case 15:
		var cfg BlackSharkGearConfig
		cfg.Source = payload[0]
		cfg.Form = payload[1]
		cfg.Gear = payload[2]
		for i := 0; i < BlackSharkCurvePointCount; i++ {
			base := 3 + i*3
			cfg.Curve[i] = BlackSharkCurvePoint{
				TempC: payload[base],
				RPM:   uint16(payload[base+1]) | uint16(payload[base+2])<<8,
			}
		}
		return cfg, true
	default:
		return BlackSharkGearConfig{}, false
	}
}

// BlackSharkCurveString 便于日志/测试的可读输出。
func BlackSharkCurveString(points [BlackSharkCurvePointCount]BlackSharkCurvePoint) string {
	out := ""
	for i, p := range points {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%dC:%d", p.TempC, p.RPM)
	}
	return out
}

// ClampBlackSharkCurveRPM 把目标转速夹进设备可达区间（BlackSharkMinRPM..BlackSharkMaxRPM）。
func ClampBlackSharkCurveRPM(rpm int) int {
	if rpm < BlackSharkMinRPM {
		rpm = BlackSharkMinRPM
	}
	if rpm > BlackSharkMaxRPM {
		rpm = BlackSharkMaxRPM
	}
	q := BlackSharkRPMSnapRPM
	return ((rpm + q/2) / q) * q
}

// BlackSharkCurveEndpointMinTempC / BlackSharkCurveEndpointMaxTempC 是主机侧插值曲线的两个固定端点温度，
// 与前端 CURVE_EXTENSION_MIN_TEMP/MAX_TEMP 同源。
const (
	BlackSharkCurveEndpointMinTempC = 10
	BlackSharkCurveEndpointMaxTempC = 100
)

// BlackSharkCurvePointMinTempC / BlackSharkCurvePointMaxTempC 是黑鲨曲线那 4 个点可以被拖到哪的温度取值域（℃）。
//
// 设备对曲线里的温度字段不做校验（载荷是 [TempC u8][RPM u16le] ×4），所以这是产品约定；
// 出厂曲线落在 20/40/60/80 只是默认值，不是设备限定的格点，四个点可以在取值域内取任意整数温度。
// 与上面两个插值端点同值时也不要合并：端点是主机侧补出来的固定点，这里是用户能拖动的范围，语义不同。
const (
	BlackSharkCurvePointMinTempC = 10
	BlackSharkCurvePointMaxTempC = 100
)

// BlackSharkInterpolationPoint 是主机侧插值曲线的一个点（温度 ℃ + 转速 RPM）。
//
// 不直接用 types.FanCurvePoint：types 已经 import 本包，反向依赖会成环。
type BlackSharkInterpolationPoint struct {
	Temperature int
	RPM         int
}

// BlackSharkEffectiveCurveForInterpolation 把「用户可拖的 4 点」补上两个固定端点，
// 得到主机侧插值真正该用的那条曲线。
func BlackSharkEffectiveCurveForInterpolation(curve []BlackSharkInterpolationPoint) []BlackSharkInterpolationPoint {
	out := make([]BlackSharkInterpolationPoint, 0, len(curve)+2)
	if len(curve) == 0 {
		return out
	}
	if curve[0].Temperature > BlackSharkCurveEndpointMinTempC {
		out = append(out, BlackSharkInterpolationPoint{
			Temperature: BlackSharkCurveEndpointMinTempC,
			RPM:         BlackSharkMinRPM,
		})
	}
	out = append(out, curve...)
	if last := curve[len(curve)-1]; last.Temperature < BlackSharkCurveEndpointMaxTempC {
		out = append(out, BlackSharkInterpolationPoint{
			Temperature: BlackSharkCurveEndpointMaxTempC,
			RPM:         BlackSharkMaxRPM,
		})
	}
	return out
}

// ---- 灯效参数：HSV / 颜色 / 音频电平 ----

// BlackSharkHsvToRGB 复现 Qt QColor::setHsv(h,s,v) 的 RGB 结果（等价于 setHsv(h,255,255)）。
func BlackSharkHsvToRGB(h, s, v int) (r, g, b uint8) {
	h %= 360
	if h < 0 {
		h += 360
	}
	hf := float64(h) / 60.0
	sf := float64(s) / 255.0
	vf := float64(v) / 255.0
	hr := int(math.Floor(hf))
	f := hf - float64(hr)
	p := qRound255(vf * (1 - sf) * 255)
	q := qRound255(vf * (1 - sf*f) * 255)
	t := qRound255(vf * (1 - sf*(1-f)) * 255)
	vm := qRound255(vf * 255)
	var rr, gg, bb int
	switch hr % 6 {
	case 0:
		rr, gg, bb = vm, t, p
	case 1:
		rr, gg, bb = q, vm, p
	case 2:
		rr, gg, bb = p, vm, t
	case 3:
		rr, gg, bb = p, q, vm
	case 4:
		rr, gg, bb = t, p, vm
	default:
		rr, gg, bb = vm, p, q
	}
	return uint8(rr), uint8(gg), uint8(bb)
}

// BlackSharkHueToColor 是官方"单色 + 色相滑块"那条路径的完整换算
// （等价于 setHsv(h,255,255).rgb()），即官方写进 payload[5..7] 的值。
func BlackSharkHueToColor(hue int) (r, g, b uint8) {
	return BlackSharkHsvToRGB(hue, 255, 255)
}

func qRound255(x float64) int {
	if x >= 0 {
		return int(x + 0.5)
	}
	return -int(-x + 0.5)
}

// 帧载荷就是档位（1/2/3 = 低/中/高），不是连续电平。
const (
	BlackSharkAudioLevelLow  byte = 1
	BlackSharkAudioLevelMid  byte = 2
	BlackSharkAudioLevelHigh byte = 3
)

// 分档边界是可调近似：官方那两个阈值尚未拿到。
const (
	BlackSharkAudioHighBandFromHz = 2000.0 // ≥ 2 kHz → 高
	BlackSharkAudioMidBandFromHz  = 400.0  // 400 Hz ~ 2 kHz → 中；< 400 Hz → 低
)

// BlackSharkAudioLevelForFrequency 把主频（Hz；<=0 表示静音）折成档位。
// 返回 0 = 静音，不要发帧（官方在 RMS < 100 时也不发）。
func BlackSharkAudioLevelForFrequency(hz float64) byte {
	switch {
	case hz <= 0:
		return 0
	case hz >= BlackSharkAudioHighBandFromHz:
		return BlackSharkAudioLevelHigh
	case hz >= BlackSharkAudioMidBandFromHz:
		return BlackSharkAudioLevelMid
	default:
		return BlackSharkAudioLevelLow
	}
}

// ---- LCD 显示参数（0xC2）与官方重置、出厂常量 ----

// BlackSharkOfficialResetBlocks 是官方按「重置」时连发的 9 个载荷：8 个模式各写一遍，
// 再重写模式 1（第 9 条）。
var BlackSharkOfficialResetBlocks = [][]byte{
	{0x01, 0xBE, 0x0A, 0x50, 0x0A, 0x00, 0x00, 0x00},
	{0x02, 0x10, 0x27, 0x50, 0x0A, 0x00, 0x00, 0x00},
	{0x03, 0xB8, 0x0B, 0x50, 0x01, 0x00, 0xFF, 0x49},
	{0x04, 0x00, 0x00, 0x50, 0x01, 0x00, 0xFF, 0x49},
	{0x05, 0xEE, 0x02, 0x50, 0x01, 0x00, 0xFF, 0x49},
	{0x06, 0x8A, 0x02, 0x50, 0x01, 0x00, 0xFF, 0x49},
	{0x07, 0x00, 0x00, 0x50, 0x01, 0x00, 0xFF, 0x49},
	{0x08, 0xF4, 0x01, 0x50, 0x0A, 0x00, 0x00, 0x00},
	{0x01, 0xBE, 0x0A, 0x50, 0x0A, 0xFF, 0x00, 0x00}, // 第 9 条：重写模式 1
}

// BlackSharkOfficialResetExpected 是重置之后设备所处的状态。
var BlackSharkOfficialResetExpected = []byte{0x04, 0x00, 0x00, 0x50, 0x01, 0x00, 0xFF, 0x49}

// BlackSharkOfficialCoolingResetBlocks 是官方按「散热页 · 重置」时连发的 9 个 0x24 载荷。
var BlackSharkOfficialCoolingResetBlocks = [][]byte{
	{0x00, 0x00, 0x01, 0xB0, 0x04}, // 档1 固定 = 1200
	{0x00, 0x01, 0x01, 0x14, 0xB0, 0x04, 0x28, 0xE8, 0x05, 0x3C, 0x56, 0x08, 0x50, 0xC4, 0x0A}, // 档1 曲线 1200/1512/2134/2756
	{0x00, 0x00, 0x02, 0x56, 0x08}, // 档2 固定 = 2134
	{0x00, 0x01, 0x02, 0x14, 0xE8, 0x05, 0x28, 0x56, 0x08, 0x3C, 0xC4, 0x0A, 0x50, 0x32, 0x0D}, // 档2 曲线 1512/2134/2756/3378
	{0x00, 0x00, 0x03, 0x60, 0x0B}, // 档3 固定 = 2912
	{0x00, 0x01, 0x03, 0x14, 0x1F, 0x07, 0x28, 0x8D, 0x09, 0x3C, 0x32, 0x0D, 0x50, 0xA0, 0x0F}, // 档3 曲线 1823/2445/3378/4000
	{0x00, 0x00, 0x04, 0xCE, 0x0D}, // 档4 固定 = 3534
	{0x00, 0x01, 0x04, 0x14, 0x56, 0x08, 0x28, 0xFB, 0x0B, 0x3C, 0xA0, 0x0F, 0x50, 0xA0, 0x0F}, // 档4 曲线 2134/3067/4000/4000
	{0x00, 0x00, 0x02, 0x56, 0x08}, // 收尾：切到 档2 + 固定（官方重置后的当前状态）
}

// BlackSharkFactoryGearConfig 从「散热重置」常量里取出某档位的出厂配置。
func BlackSharkFactoryGearConfig(form, gear byte) (BlackSharkGearConfig, bool) {
	for _, blk := range BlackSharkOfficialCoolingResetBlocks {
		cfg, ok := DecodeBlackSharkGearConfig(blk)
		if ok && cfg.Form == form && cfg.Gear == gear {
			return cfg, true
		}
	}
	return BlackSharkGearConfig{}, false
}

// ---- 系统信息推送（0x07） ----

// 系统信息推送（0x07，主机 → 设备，约 1 Hz）的字段 id。
const (
	// BlackSharkSysInfoCPUTemp CPU 温度（°C）。
	BlackSharkSysInfoCPUTemp byte = 0
	// BlackSharkSysInfoGPUTemp GPU 温度（°C）。
	BlackSharkSysInfoGPUTemp byte = 1
	// BlackSharkSysInfoCPULoad CPU 占用率（%）。
	BlackSharkSysInfoCPULoad byte = 2
	// BlackSharkSysInfoGPULoad GPU 占用率（%）。
	BlackSharkSysInfoGPULoad byte = 3
	// BlackSharkSysInfoFanRPM 风扇转速（RPM）。
	BlackSharkSysInfoFanRPM byte = 4
	// BlackSharkSysInfoDiskUsage 磁盘占用率（%）。
	BlackSharkSysInfoDiskUsage byte = 5
	// BlackSharkSysInfoMemUsage 内存占用率（%）。
	BlackSharkSysInfoMemUsage byte = 6
	// BlackSharkSysInfoMinuteTime 时间 = 时*60 + 分（1223 即 20:23）。
	BlackSharkSysInfoMinuteTime byte = 7
)

// BlackSharkSysInfoMaxEntries 是"条数"字节能表达的上限（避免构造出畸形帧）。
const BlackSharkSysInfoMaxEntries = 64

// BlackSharkSysInfoPushItems 是 0x07 周期推送固定的 7 项，顺序即下发顺序。
var BlackSharkSysInfoPushItems = []byte{
	BlackSharkSysInfoCPUTemp,
	BlackSharkSysInfoGPUTemp,
	BlackSharkSysInfoCPULoad,
	BlackSharkSysInfoGPULoad,
	BlackSharkSysInfoDiskUsage,
	BlackSharkSysInfoMemUsage,
	BlackSharkSysInfoMinuteTime,
}

// BlackSharkLcdSelectableItems 是 0xC2（LCD 显示参数）可选的 8 项，顺序即官方 UI 的排列顺序。
var BlackSharkLcdSelectableItems = []byte{
	BlackSharkSysInfoCPUTemp,    // CPU温度
	BlackSharkSysInfoGPUTemp,    // GPU温度
	BlackSharkSysInfoCPULoad,    // CPU负载
	BlackSharkSysInfoGPULoad,    // GPU负载
	BlackSharkSysInfoFanRPM,     // 风扇转速
	BlackSharkSysInfoDiskUsage,  // 磁盘
	BlackSharkSysInfoMemUsage,   // 运存使用率
	BlackSharkSysInfoMinuteTime, // 时间
}

// IsBlackSharkLcdSelectableItem 判断 id 是否是 LCD 可选显示项。
func IsBlackSharkLcdSelectableItem(id byte) bool {
	for _, v := range BlackSharkLcdSelectableItems {
		if v == id {
			return true
		}
	}
	return false
}

// BlackSharkLcdSelectableItemIDs 把可选项以 `[]int` 交出（给界面层用）。
func BlackSharkLcdSelectableItemIDs() []int {
	out := make([]int, len(BlackSharkLcdSelectableItems))
	for i, id := range BlackSharkLcdSelectableItems {
		out[i] = int(id)
	}
	return out
}

// BlackSharkSystemInfoEntry 是推送里的一个字段。
type BlackSharkSystemInfoEntry struct {
	ID    byte
	Value uint16
}

// BuildBlackSharkSystemInfoPayload 构造 `0x07` 的载荷：
//
//	[条数 u8] + N × ( [id u8][数值 u16le] )
func BuildBlackSharkSystemInfoPayload(entries []BlackSharkSystemInfoEntry) []byte {
	if len(entries) == 0 || len(entries) > BlackSharkSysInfoMaxEntries {
		return nil
	}
	out := make([]byte, 0, 1+3*len(entries))
	out = append(out, byte(len(entries)))
	for _, e := range entries {
		out = append(out, e.ID, byte(e.Value), byte(e.Value>>8))
	}
	return out
}

// ClampBlackSharkSysInfoValue 把采集到的数值收进 u16（越界即截断，不 panic、不环绕）。
func ClampBlackSharkSysInfoValue(v int) uint16 {
	if v < 0 {
		return 0
	}
	if v > 0xFFFF {
		return 0xFFFF
	}
	return uint16(v)
}
