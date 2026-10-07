// 黑鲨 BRB02 的设备写入：冷却与开关、系统信息推送、灯效、配置快照与固件、屏幕与读图。
package device

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/screenimg"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

// ---- 冷却、开关与系统信息推送 ----

// setBlackSharkCurveLocked 用调用方已经拿到的「当前配置」快照完成一次曲线写入 + 回读确认。
//
// 档位由调用方决定；参照源取自 current —— 它与档位来自同一份 `0x25` 响应，
// 所以整条下发路径只需读一次 `0x25`。
// 调用方需持有 m.mutex，并已校验连接。
func (m *Manager) setBlackSharkCurveLocked(
	gear byte,
	points [deviceproto.BlackSharkCurvePointCount]deviceproto.BlackSharkCurvePoint,
	current deviceproto.BlackSharkGearConfig,
) bool {
	source := deviceproto.BlackSharkNormalizeCoolingSource(current.Source)
	payload := deviceproto.EncodeBlackSharkCurvePayload(gear, points, source)
	if err := m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdSetSpeed, payload...); err != nil {
		m.logError("下发黑鲨变频曲线失败: %v", err)
		return false
	}
	confirm, ok := m.blackSharkReadCoolingConfigLocked()
	if !ok {
		m.logError("黑鲨变频曲线写入后回读失败，无法确认生效")
		return false
	}
	if confirm.Form != deviceproto.BlackSharkFormCurve || confirm.Gear != gear || confirm.Curve != points {
		m.logError("黑鲨变频曲线回读不一致: 期望 gear=%d [%s], 实际 gear=%d [%s]",
			gear, deviceproto.BlackSharkCurveString(points),
			confirm.Gear, deviceproto.BlackSharkCurveString(confirm.Curve))
		return false
	}
	m.logInfo("黑鲨变频曲线已生效: gear=%d [%s]", gear, deviceproto.BlackSharkCurveString(points))
	return true
}

// SetBlackSharkCoolingCurve 写入并提交生效一条温度曲线（变频模式）。
func (m *Manager) SetBlackSharkCoolingCurve(gear byte, points [deviceproto.BlackSharkCurvePointCount]deviceproto.BlackSharkCurvePoint) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.blackSharkWriteHandleLocked() == nil {
		return false
	}
	if gear < 1 || gear > deviceproto.BlackSharkGearCount {
		m.logError("黑鲨档位越界: %d（有效 1..%d）", gear, deviceproto.BlackSharkGearCount)
		return false
	}
	// 载荷 byte[0] 是设备侧状态（见 currentConfigSourceLocked 的说明）：
	// 读回来原样写回，不能写死，否则等于顺手改掉一个语义未定的字段；
	// 取不到现值就不写——宁可失败，也不猜一个值发出去。
	current, ok := m.blackSharkReadCoolingConfigLocked()
	if !ok {
		m.logError("黑鲨：0x25 读不回当前配置，放弃本次写入（0x24 首字节取不到现值，" +
			"猜一个会改掉一个语义未定的字段）；请检查设备连接")
		return false
	}
	return m.setBlackSharkCurveLocked(gear, points, current)
}

// activeBlackSharkGearLocked 返回设备当前生效的档位（1..4，来自 0x25 回读的 gear）。
func (m *Manager) activeBlackSharkGearLocked() int {
	cfg, ok := m.blackSharkReadCoolingConfigLocked()
	if !ok {
		return 0
	}
	return int(cfg.Gear)
}

// BlackSharkGearIsActive 报告设备当前生效的档位是不是 gear。
// 变频模式下"保存了某档位方案"不等于"设备正在跑这一档"，调用方据此决定是否切档。
func (m *Manager) BlackSharkGearIsActive(gear int) bool {
	if gear < 1 || gear > deviceproto.BlackSharkGearCount {
		return false
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.blackSharkHandleLocked() == nil {
		return false
	}
	return m.activeBlackSharkGearLocked() == gear
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// setBlackSharkTargetSpeedLocked 设定目标转速。
func (m *Manager) setBlackSharkTargetSpeedLocked(speed types.FanSpeedValue) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.blackSharkWriteHandleLocked() == nil {
		return false
	}
	speed = speed.Normalized()
	if !types.IsRPMSpeedUnit(speed.Unit) {
		m.logWarn("黑鲨 HID 设备只支持 RPM 直控")
		return false
	}
	rpm := types.ClampRPM(speed.Value)
	requested := rpm
	// 上限取所有者给的设备可达区间（不是通用散热器的 DefaultMaxFanRPM）。
	if rpm > deviceproto.BlackSharkMaxRPM {
		rpm = deviceproto.BlackSharkMaxRPM
	}
	// 设备配置里的目标值不是 RPM：先用标定曲线换算成设定值。
	setpoint := deviceproto.BlackSharkSetpointForRPM(rpm)
	if setpoint < deviceproto.BlackSharkMinSetpoint {
		setpoint = deviceproto.BlackSharkMinSetpoint
	}
	if setpoint > deviceproto.BlackSharkMaxSetpoint {
		setpoint = deviceproto.BlackSharkMaxSetpoint
	}
	// 请求值与最终生效值不同就如实报出：必须在这里判，夹紧之后再问 BlackSharkRPMSupported 永远是 true。
	if requested != deviceproto.BlackSharkRPMForSetpoint(setpoint) {
		m.logWarn("黑鲨可达到的转速为 %d~%d RPM，请求 %d RPM 已夹紧到设定值 %d（约 %d RPM）",
			deviceproto.BlackSharkMinRPM, deviceproto.BlackSharkMaxRPM, requested, setpoint,
			deviceproto.BlackSharkRPMForSetpoint(setpoint))
	}

	// 该路径固定写 gear=1；按档位的固定值走 SetBlackSharkFixedSpeedForGear，
	// 官方每档各存一份散热模式。
	return m.setBlackSharkFixedSpeedLocked(1, setpoint)
}

// SetBlackSharkFixedSpeedForGear 把某档位设成固定转速（0x24 的 form=0 形态）。
func (m *Manager) SetBlackSharkFixedSpeedForGear(gear int, rpm int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.blackSharkWriteHandleLocked() == nil {
		return false
	}
	if gear < 1 || gear > types.SceneGearMax {
		m.logError("黑鲨档位越界: %d（应为 1..%d）", gear, types.SceneGearMax)
		return false
	}
	// 切档/手动挡位是"重要写入"：作废还排着队的例行目标，否则例行的旧目标随后会把挡位值盖回曲线态。
	m.CancelPendingSpeedPush()
	setpoint := deviceproto.BlackSharkSetpointForRPM(types.ClampRPM(rpm))
	if setpoint < deviceproto.BlackSharkMinSetpoint {
		setpoint = deviceproto.BlackSharkMinSetpoint
	}
	if setpoint > deviceproto.BlackSharkMaxSetpoint {
		setpoint = deviceproto.BlackSharkMaxSetpoint
	}
	return m.setBlackSharkFixedSpeedLocked(byte(gear), setpoint)
}

// setBlackSharkFixedSpeedLocked 执行固定值写入并回读确认（gear 由调用方给定）。
func (m *Manager) setBlackSharkFixedSpeedLocked(gear byte, setpoint int) bool {
	// 固定值形态：载荷 = [source][form=0][gear][value:u16le]，编码见 deviceproto.EncodeBlackSharkFixedSpeedPayload。
	// byte[0] 是设备侧状态，读回来原样写回（理由同 SetBlackSharkCoolingCurve）；取不到就不写。
	source, ok := m.currentConfigSourceLocked()
	if !ok {
		return false
	}
	payload := deviceproto.EncodeBlackSharkFixedSpeedPayload(gear, uint16(setpoint), source)
	if err := m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdSetSpeed, payload...); err != nil {
		m.logError("下发黑鲨转速失败: %v", err)
		return false
	}
	// 回读确认。只有设备把写入的值存回配置才返回成功——
	// 否则宁可报失败，也不返回「看起来成功」的假绿。
	confirm, ok := m.blackSharkReadCoolingConfigLocked()
	if !ok {
		m.logError("黑鲨转速写入后回读失败，无法确认生效（已下发设定值 %d）", setpoint)
		return false
	}
	// 连 gear 一起比：只比 form/value 的话，写到别的档位也会被判成成功。
	if confirm.Form != deviceproto.BlackSharkFormValue || confirm.Value != uint16(setpoint) ||
		confirm.Gear != gear {
		m.logError("黑鲨固定转速回读不一致: 期望 form=%d gear=%d value=%d, 实际 form=%d gear=%d value=%d",
			deviceproto.BlackSharkFormValue, gear, setpoint, confirm.Form, confirm.Gear, confirm.Value)
		return false
	}
	// target 记录请求的转速，实际转速由 0x06 状态帧独立更新。
	// 本方法只拿到 setpoint（字段值），按标定表换回 RPM 再记录，
	// 避免"字段值"和"请求值"两个口径并存。
	m.storeBlackSharkTargetLocked(deviceproto.BlackSharkRPMForSetpoint(setpoint), "固定转速")
	return true
}

// ---- 状态记录、开关与冷却参照源 ----

// storeBlackSharkStatusLocked 用设备状态帧（0x06 / 0x25）更新当前转速与标志位。
// 这是 CurrentRPM 的唯一来源——必须来自设备回报，不能由目标值推算。
// flag 是状态帧的第 3 字节，语义未定（本机 136/136 都是 0x01），只原样带出、不映射成挡位名。
func (m *Manager) storeBlackSharkStatusLocked(currentRPM uint16, flag byte, command byte) {
	target := currentRPM
	var previous *types.FanData
	if prev := m.currentFanData.Load(); prev != nil && prev.Transport == m.deviceType {
		previous = prev
		// 有已知目标时保留目标值，不用当前转速顶替。
		if prev.TargetRPM != 0 {
			target = prev.TargetRPM
		}
	}
	data := &types.FanData{
		Command:     command,
		Transport:   m.deviceType,
		SpeedUnit:   types.FanSpeedUnitRPM,
		WorkMode:    "实时转速",
		CurrentMode: flag,
		CurrentRPM:  currentRPM,
		TargetRPM:   target,
	}
	if previous != nil {
		data.Status = previous.Status
	}
	m.currentFanData.Store(data)
	if m.onFanDataUpdate != nil {
		go m.onFanDataUpdate(data)
	}
}

// storeBlackSharkTargetLocked 在成功下发转速后更新目标值。
// 只写 TargetRPM，绝不写 CurrentRPM。
func (m *Manager) storeBlackSharkTargetLocked(targetRPM int, workMode string) {
	target := types.ClampRPM(targetRPM)
	data := &types.FanData{
		Command:    deviceproto.BlackSharkCmdSetSpeed,
		Transport:  m.deviceType,
		SpeedUnit:  types.FanSpeedUnitRPM,
		WorkMode:   workMode,
		TargetRPM:  uint16(target),
		CurrentRPM: 0,
	}
	if prev := m.currentFanData.Load(); prev != nil && prev.Transport == m.deviceType {
		data.CurrentRPM = prev.CurrentRPM
		data.Status = prev.Status
		data.CurrentMode = prev.CurrentMode
	}
	m.currentFanData.Store(data)
	if m.onFanDataUpdate != nil {
		go m.onFanDataUpdate(data)
	}
}

// blackSharkReadSwitchLocked 读取单字节开关状态。
func (m *Manager) blackSharkReadSwitchLocked(getCmd byte) (uint8, bool) {
	frame, ok := m.blackSharkQueryLocked(getCmd, nil)
	if !ok {
		return 0, false
	}
	return deviceproto.BlackSharkSingleByteState(frame, getCmd)
}

// setBlackSharkSwitchLocked 下发单字节开关并按 getCmd 回读确认。
// getCmd 必须是真实读回命令，"写后回读确认"是唯一路径。
func (m *Manager) setBlackSharkSwitchLocked(setCmd, getCmd byte, enabled bool, label string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.blackSharkWriteHandleLocked() == nil {
		return false
	}
	if err := m.blackSharkWriteFrameLocked(setCmd, deviceproto.BlackSharkSwitchPayload(enabled)...); err != nil {
		m.logError("黑鲨 %s 下发失败: %v", label, err)
		return false
	}
	state, ok := m.blackSharkReadSwitchLocked(getCmd)
	if !ok {
		m.logError("黑鲨 %s 写入后回读失败，无法确认生效", label)
		return false
	}
	if deviceproto.BlackSharkSwitchEnabled(state) != enabled {
		m.logError("黑鲨 %s 回读不一致: 期望 %v, 实际 0x%02X", label, enabled, state)
		return false
	}
	return true
}

// SetBlackSharkLightingEnabled 开关 RGB 灯效（CMD 0x10 / 读回 0x11）。
func (m *Manager) SetBlackSharkLightingEnabled(enabled bool) bool {
	if !m.usesBlackSharkTransport() {
		return false
	}
	return m.setBlackSharkSwitchLocked(
		deviceproto.BlackSharkCmdRGBEnable,
		deviceproto.BlackSharkCmdRGBStatus,
		enabled, "RGB 灯效开关")
}

// SetBlackSharkLcdScreenEnabled 开关 LCD 小屏（CMD 0xC0 / 读回 0xC1）。
func (m *Manager) SetBlackSharkLcdScreenEnabled(enabled bool) bool {
	if !m.usesBlackSharkTransport() {
		return false
	}
	return m.setBlackSharkSwitchLocked(
		deviceproto.BlackSharkCmdSetLcdScreenSwitch,
		deviceproto.BlackSharkCmdGetLcdScreenSwitch,
		enabled, "LCD 屏开关")
}

// SetBlackSharkCoolingSource 设置温控参照（CMD 0x22 / 读回 0x23）。
// source 经 BlackSharkNormalizeCoolingSource 归一化为 CPU / GPU 两种参照。
func (m *Manager) SetBlackSharkCoolingSource(source uint8) bool {
	if !m.usesBlackSharkTransport() {
		return false
	}
	src := deviceproto.BlackSharkNormalizeCoolingSource(source)
	// 不缓存参照值：写 0x24 时 byte[0] 读 0x25 现取（见 currentConfigSourceLocked），
	// 界面要显示实际参照则走 BlackSharkSwitchStates。
	return m.setBlackSharkSwitchLocked(
		deviceproto.BlackSharkCmdSetCoolingSource,
		deviceproto.BlackSharkCmdGetCoolingSource,
		src == deviceproto.BlackSharkCoolingSourceGPU, "制冷来源")
}

// currentConfigSourceLocked 取本次写 0x24 时 byte[0] 该填的值。
// 返回 false 表示这次写入必须放弃（不许拿猜的值继续写）；调用方需持有 m.mutex。
func (m *Manager) currentConfigSourceLocked() (byte, bool) {
	cfg, ok := m.blackSharkReadCoolingConfigLocked()
	if !ok {
		m.logError("黑鲨：0x25 读不回当前配置，放弃本次写入（0x24 首字节取不到现值，" +
			"猜一个会改掉一个语义未定的字段）；请检查设备连接")
		return 0, false
	}
	return deviceproto.BlackSharkNormalizeCoolingSource(cfg.Source), true
}

// SetBlackSharkOnOffVector 写入「智能启停 + 通电自启」两路开关（CMD 0x02，读回 0x03）。
// 载荷形状与位语义见 deviceproto.BlackSharkCmdSetOnOffVector。
func (m *Manager) SetBlackSharkOnOffVector(smartStartStop, powerOnSelfStart bool) bool {
	if !m.usesBlackSharkTransport() {
		return false
	}
	target := deviceproto.BlackSharkOnOffVector{
		SmartStartStop:   smartStartStop,
		PowerOnSelfStart: powerOnSelfStart,
	}
	payload := deviceproto.BlackSharkOnOffVectorPayload(target)
	// 载荷形状是已知的唯一形状，长度必须是 2；越界说明编码坏了，不能发。
	if len(payload) != 2 {
		m.logError("黑鲨开关向量载荷长度异常: %d（应为 2）", len(payload))
		return false
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.blackSharkWriteHandleLocked() == nil {
		return false
	}
	if err := m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdSetOnOffVector, payload...); err != nil {
		m.logError("黑鲨开关向量下发失败: %v", err)
		return false
	}
	// 记录最近下发值：读回失败时它是唯一的兜底信息，绝不用来冒充设备实况。
	value := target
	m.blackSharkOnOffCmd.Store(&value)

	got, ok := m.blackSharkReadOnOffVectorLocked()
	if !ok {
		m.logError("黑鲨开关向量写入后回读失败（0x03 无响应），无法确认生效")
		return false
	}
	if got != target {
		m.logError("黑鲨开关向量回读不一致: 期望 智能启停=%v 通电自启=%v，实际 智能启停=%v 通电自启=%v",
			smartStartStop, powerOnSelfStart, got.SmartStartStop, got.PowerOnSelfStart)
		return false
	}
	m.logDebug("黑鲨开关向量已生效: 智能启停=%v 通电自启=%v（0x03 回读确认）",
		smartStartStop, powerOnSelfStart)
	return true
}

// blackSharkReadOnOffVectorLocked 读回两路开关向量（CMD 0x03）。调用方需持有 m.mutex。
func (m *Manager) blackSharkReadOnOffVectorLocked() (deviceproto.BlackSharkOnOffVector, bool) {
	frame, ok := m.blackSharkQueryLocked(deviceproto.BlackSharkCmdGetOnOffVector, nil)
	if !ok {
		return deviceproto.BlackSharkOnOffVector{}, false
	}
	return deviceproto.BlackSharkOnOffVectorFromStatus(frame)
}

// BlackSharkSwitchStates 读取黑鲨各开关状态。
// 只有固件提供读回命令的开关才会置 Known=true。
func (m *Manager) BlackSharkSwitchStates() types.BlackSharkSwitchStates {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	states := types.BlackSharkSwitchStates{}
	if m.blackSharkHandleLocked() == nil {
		return states
	}
	states.Available = true

	if state, ok := m.blackSharkReadSwitchLocked(deviceproto.BlackSharkCmdRGBStatus); ok {
		states.LightingEnabled = deviceproto.BlackSharkSwitchEnabled(state)
		states.LightingKnown = true
	}
	if state, ok := m.blackSharkReadSwitchLocked(deviceproto.BlackSharkCmdGetLcdScreenSwitch); ok {
		states.LcdScreenEnabled = deviceproto.BlackSharkSwitchEnabled(state)
		states.LcdScreenKnown = true
	}
	if state, ok := m.blackSharkReadSwitchLocked(deviceproto.BlackSharkCmdGetCoolingSource); ok {
		// 设备实际参照的唯一读取路径（界面显示用它）；写 0x24 的 byte[0] 不从这里取，
		// 而是读 0x25 现值，两者暂不统一。
		states.CoolingSource = state
		states.CoolingSourceKnown = true
	}
	// 0x03 提供读回：优先填设备回报状态；读不到才退回最近下发值，并让 Known 保持 false。
	if vec, ok := m.blackSharkReadOnOffVectorLocked(); ok {
		states.SmartStartStop = vec.SmartStartStop
		states.PowerOnSelfStart = vec.PowerOnSelfStart
		states.OnOffVectorKnown = true
	} else if last := m.blackSharkOnOffCmd.Load(); last != nil {
		states.LastCommandedSmartStartStop = last.SmartStartStop
		states.LastCommandedPowerOnSelfStart = last.PowerOnSelfStart
	}
	// 该固件提供 0x03 读回（闭环验证过），是设备能力而非常量。
	states.OnOffVectorReadbackSupported = true
	return states
}

// ---- 系统信息推送（0x07） ----

// ── 黑鲨 BRB02 的链路与传输 ─────────────────────────────────────────────────
// 链路状态、收帧派发、异步下发 worker、写闸口与运行时判据。

// 黑鲨链路的时间与限额常量；本文件是其唯一 owner。
const (
	blackSharkQueryTimeout = 900 * time.Millisecond
	blackSharkWriteTimeout = 800 * time.Millisecond
)

// blackSharkHandleLocked 报告黑鲨的传输句柄是否已就绪（可用性判据，不是写权限）。
//
// 判据 = 已连接 + 句柄在手 + 当前档案属于黑鲨。句柄即上游的 `flyDigiHIDDevice`：
// `usb != nil` 时走 libusb 批量端点，否则走 HIDAPI。
//
// 档案判据不能省：飞智设备的句柄是同一个字段（`connectFlyDigiHIDLocked` 也写 `m.flyDigiHID`），
// 只看 `m.flyDigiHID != nil` 会把飞智当黑鲨，使黑鲨面板与读图路径在飞智设备上被激活。
//
// 与 blackSharkWriteHandleLocked 的区别是不看 writesBlocked：它服务的是"这个功能现在能不能用"，
// 即灯效/曲线/屏幕/固件的可用性门；写权限由写路径自己那道门管。
func (m *Manager) blackSharkHandleLocked() *flyDigiHIDDevice {
	if !m.isConnected || m.flyDigiHID == nil || !isBlackSharkProfileID(m.activeProfile.ID) {
		return nil
	}
	return m.flyDigiHID
}

// blackSharkWriteHandleLocked 是"现在能不能写、往哪个句柄写"的唯一判据：
// 写闸门 blackSharkWriteGuard 与各处写路径的 guard 都问它。
// 返回 nil = 不可写。判据 = 非悬挂期 + blackSharkHandleLocked（已连接 + 黑鲨档案 + 句柄在手）。
func (m *Manager) blackSharkWriteHandleLocked() *flyDigiHIDDevice {
	if m.writesBlocked.Load() {
		return nil
	}
	return m.blackSharkHandleLocked()
}

// ensureBlackSharkRXLocked 是黑鲨两条接收通道（控制应答 / 图像帧）的 owner：按需创建。
//
// 读协程来自上游的 `readFlyDigiHIDLoop`，它把黑鲨帧交给 `handleBlackSharkHIDRX`。
// 通道若只在连接时机创建，libusb 通路上「0xC5 读信息」与「0xC7/0xC8 读图」会恒失败。
// 连接/断开时机掌握在上游的连接代码里，故改由消费者入口 blackSharkQueryLocked /
// StartBlackSharkScreenImageRead 自行调用本函数。
//
// 通道不随断开清空：读协程按 `connectionGen` 判代次，消费者每次使用前都会排空，
// 陈旧帧不会被当成新应答。
func (m *Manager) ensureBlackSharkRXLocked() {
	if m.link.blackSharkResp == nil {
		m.link.blackSharkResp = make(chan deviceproto.BlackSharkFrame, 32)
	}
	if m.link.blackSharkImgFrames == nil {
		m.link.blackSharkImgFrames = make(chan deviceproto.BlackSharkImageFrame, 64)
	}
}

// blackSharkWriteGuard 是黑鲨写闸门的唯一判据：危险命令黑名单 → 悬挂期禁写 → 可写句柄在手。
// blackSharkWriteFrameLocked 组帧前先过它。
func (m *Manager) blackSharkWriteGuard(cmd byte) error {
	if reason, blocked := deviceproto.BlackSharkIsBlockedCommand(cmd); blocked {
		m.logError("黑鲨命令 0x%02X 已被封锁，拒绝下发: %s", cmd, reason)
		return fmt.Errorf("黑鲨命令 0x%02X 已被封锁（%s）", cmd, reason)
	}
	if m.writesBlocked.Load() {
		return fmt.Errorf("device writes are blocked during system suspend")
	}
	if m.blackSharkWriteHandleLocked() == nil {
		return fmt.Errorf("黑鲨设备未连接")
	}
	return nil
}

// blackSharkLink 是黑鲨链路状态的唯一 owner。
//
// 收在这里的东西：两条接收通道、等待序号与 Mission 序号、承载缓存、待切档位、异步下发 Mission。
// 别处一律通过 m.link 引用，对齐官方 CUSBDataSendCenter「一个 owner 管全部 Mission」的形状，
// 以后按官方调整机制（例如把"深度 1 覆盖"换成排队）只改这一处。
// 读协程本身不在本文件：USB 档案用上游的 readFlyDigiHIDLoop，它收帧后投到这里的两条通道。
type blackSharkLink struct {
	// awaitCmd 是当前正在等待应答的 cmd（0 = 没人等）。读协程只投递匹配的帧，
	// 设备主动上报（0x06/0x07 空应答等）一律不占通道。
	awaitCmd atomic.Uint32
	// missionSeq 是宿主侧的 Mission 序号（每次"发一帧等应答"递增一次）。
	// 协议帧里没有序号字段，所以用「宿主序号 + cmd 配对」达到相同目的。
	missionSeq atomic.Uint64

	// 两条接收通道（owner 见 ensureBlackSharkRXLocked）。读协程收帧后投到这里（见 handleBlackSharkHIDRX）：
	// 控制帧（0xA5）进 resp、数据帧（0xA4）进 imgFrames。
	// 通道由消费者按需创建、断开时不清空：读协程按 connectionGen 判代次，消费者每次使用前先排空。
	blackSharkResp      chan deviceproto.BlackSharkFrame
	blackSharkImgFrames chan deviceproto.BlackSharkImageFrame

	// imageFlow 在图传期间收集每块对应的 `0xC6` 流控应答状态字节（nil = 不在图传）。
	// 它是设备对"这一块收不收"的唯一直接回答，也是图传成败的唯一判据；
	// 用 atomic.Pointer 是因为投放方是读协程、装配方是图传调用方，两者不同步。
	imageFlow atomic.Pointer[chan byte]

	// 0x07 主机信息推送的节拍由 device 侧的循环拥有（与官方同形）。
	// hostInfoStop = 当前循环的停止信号（nil = 没在推）；hostInfoInterval = 当前间隔（0 = 停）；
	// hostInfoProvider 每拍取一次要推的条目（条目由 coreapp 采集，device 只管节拍与推送）。
	// 三者都在 m.mutex 下访问。
	hostInfoStop     chan struct{}
	hostInfoInterval time.Duration
	hostInfoProvider func() []deviceproto.BlackSharkSystemInfoEntry

	// 异步下发：最新目标覆盖待发目标（转速是设定值，不需要排队），由唯一 worker 串行执行。
	pushMu      sync.Mutex
	pushPending *speedPushRequest
	pushRunning bool
	// pushEpoch 是"作废标记"：每当有重要写入（安全回退、切档/手动挡位）发生时 +1，
	// 让还排着队的例行目标作废 —— 否则例行的旧目标会把安全值/挡位值盖掉。
	pushEpoch uint64
}

// speedPushRequest 是一次异步下发请求：写入动作由核心层以闭包给出（设备层不知道该走哪条路）。
type speedPushRequest struct {
	write func() bool
	done  func(ok bool)
	// epoch 是提交时的作废代次：worker 真正写之前会再比一次，代次变了就丢弃。
	epoch uint64
}

// blackSharkDroppedFrames 统计"响应通道已满、帧被丢弃"的累计次数。
var blackSharkDroppedFrames atomic.Uint64

// handleBlackSharkHIDRX 是读协程的收帧派发：`0xA4` 数据族走数据通道、`0xA5` 控制族按
// 「正在等的 cmd」入队（官方 Mission 匹配）。返回 true = 这是一条"设备在说话"的有效帧。
func (m *Manager) handleBlackSharkHIDRX(generation uint64, raw []byte, resp chan deviceproto.BlackSharkFrame) bool {
	if m.connectionGen.Load() != generation {
		return false
	}
	m.recordDebugFrame("rx", m.deviceType, raw)

	// `0xA4` 数据族（屏保图像 / 预览动画帧）单独走一条通道：
	// ParseBlackSharkFrame 只认 `0xA5` 控制族，图像帧必须单独解析。
	if img, ok := deviceproto.ParseBlackSharkImageFrame(raw); ok {
		if ch := m.link.blackSharkImgFrames; ch != nil {
			select {
			case ch <- img:
			default:
				// 通道满就丢：预览页有约 52 Hz 的帧流洪峰，这里不能阻塞读协程。
				// 收图方靠序号连续性自己判断丢没丢，不在此限流。
			}
		}
		return true // 它也是"设备在说话"的有效帧（连接就绪判定用）
	}

	frame, ok := deviceproto.ParseBlackSharkFrame(raw)
	if !ok {
		return false
	}
	// 只把"正在等的那一条"入队（官方 Mission 匹配）：设备主动上报（0x06 状态帧、0x07 的空应答等）
	// 没人等，灌进 32 格通道只会把它挤满并丢帧 —— 约每 16 秒丢一帧。
	if resp != nil && m.link.awaitCmd.Load() == uint32(frame.Command) {
		select {
		case resp <- frame:
		default:
			n := blackSharkDroppedFrames.Add(1)
			if n == 1 || n%64 == 0 {
				m.logWarn("黑鲨 HID 响应通道已满，丢弃一帧（累计 %d 帧，cmd=0x%02X）", n, frame.Command)
			}
		}
	}
	// 图传流控（0xC6）：图传期间单独收一份状态字节，供发完统计"收下 / 拒收"。
	if frame.Command == deviceproto.BlackSharkCmdImageDataFlow {
		if ch := m.link.imageFlow.Load(); ch != nil && len(frame.Payload) > 0 {
			select {
			case *ch <- frame.Payload[0]:
			default:
			}
		}
	}
	if rpm, flag, ok := deviceproto.ParseBlackSharkStatus(frame); ok {
		if m.connectionGen.Load() != generation {
			return false
		}
		m.storeBlackSharkStatusLocked(uint16(rpm), flag, frame.Command)
		return true
	}
	return false
}

// CancelPendingSpeedPush 作废队列里尚未下发的速度目标。
// 「重要写入」（安全回退、切档、图传）之前调用，避免排队中的例行目标稍后把它盖掉。
func (m *Manager) CancelPendingSpeedPush() {
	m.link.pushMu.Lock()
	m.link.pushEpoch++
	m.link.pushPending = nil
	m.link.pushMu.Unlock()
}

// speedPushWorker 是异步下发泵：取走挂起的目标写进设备，队列空时退出。
func (l *blackSharkLink) speedPushWorker() {
	for {
		l.pushMu.Lock()
		req := l.pushPending
		l.pushPending = nil
		if req == nil {
			l.pushRunning = false
			l.pushMu.Unlock()
			return
		}
		stale := req.epoch != l.pushEpoch
		l.pushMu.Unlock()

		if stale {
			if req.done != nil {
				req.done(false) // 作废按失败上报，让调用方下一拍强制重来
			}
			continue
		}

		ok := false
		if req.write != nil {
			ok = req.write()
		}
		if req.done != nil {
			req.done(ok)
		}
	}
}

// ---- HID 传输与写闸口 ----

// blackSharkBuildReportLocked 把 A5 帧装成"当前传输真正要写出去的那 65 字节"。
//
// 两条通路的封套不同，不能按一条改另一条。HIDAPI / HIDClass 的首字节是 Report ID，由 HID
// 协议栈吃掉、不会到设备，设备收到的一直是 `帧@0`；libusb 直连中断/批量端点没有 Report ID
// 这一层，线上是什么设备就收到什么，必须自己把帧放到偏移 0。
//
// 两者报文长度都是 65、线上只差 1 字节，写错不会立刻被拒（设备不校验 CK），
// 只会在很后面以"设备不再回 0xC5 / 0x26"暴露。故判据取句柄自己要走的通路，即
// `usb != nil` = libusb（与 `WriteReport` 里那个分支同一个条件），
// 保证"装的报文"与"句柄发的方式"不分叉。
func (m *Manager) blackSharkBuildReportLocked(frame []byte) ([]byte, error) {
	if d := m.blackSharkWriteHandleLocked(); d != nil && d.usb != nil {
		report := make([]byte, deviceproto.BlackSharkReportLength)
		if len(frame) > len(report) {
			return nil, fmt.Errorf("黑鲨帧长度 %d 超过报文长度 %d", len(frame), len(report))
		}
		copy(report, frame)
		return report, nil
	}
	return deviceproto.BuildBlackSharkReport(frame, deviceproto.BlackSharkReportLength)
}

// blackSharkWriteFrameLocked 组帧并写出 65 字节报文（封装按当前传输，见 blackSharkBuildReportLocked）。
// 这是黑鲨唯一的写入闸口；闸门判据见 blackSharkWriteGuard，本函数只负责组帧与发送。
func (m *Manager) blackSharkWriteFrameLocked(cmd byte, payload ...byte) error {
	if err := m.blackSharkWriteGuard(cmd); err != nil {
		return err
	}
	report, err := m.blackSharkBuildReportLocked(deviceproto.BuildBlackSharkFrame(cmd, payload...))
	if err != nil {
		return err
	}
	return retryDeviceSend(fmt.Sprintf("BlackShark HID command 0x%02X", cmd), func() error {
		m.recordDebugFrame("tx", m.deviceType, report)
		return m.blackSharkWriteHandleLocked().WriteReport(report, blackSharkWriteTimeout)
	})
}

// blackSharkQueryLocked 发送查询命令并等待匹配 cmd 的响应帧。
// 调用方需持有 m.mutex（与其它写路径一致，避免报文交错）。
func (m *Manager) blackSharkQueryLocked(cmd byte, payload []byte) (deviceproto.BlackSharkFrame, bool) {
	if m.blackSharkHandleLocked() == nil {
		return deviceproto.BlackSharkFrame{}, false
	}
	// 接收通道按需创建（见 ensureBlackSharkRXLocked），不依赖上游的连接代码预先建好。
	m.ensureBlackSharkRXLocked()
	resp := m.link.blackSharkResp
	deviceproto.DrainBlackSharkFrames(resp)
	report, err := m.blackSharkBuildReportLocked(deviceproto.BuildBlackSharkFrame(cmd, payload...))
	if err != nil {
		return deviceproto.BlackSharkFrame{}, false
	}
	// 先挂"在等这个 cmd"、再写报文：读协程只投递匹配 awaitCmd 的帧
	// （见 handleBlackSharkHIDRX）。若设备回得比挂 await 还快，应答会被当"没人等"丢掉，
	// 这次查询就只剩超时。挂早一点没有代价（超时到时由 blackSharkWaitResponseLocked 清）。
	m.link.awaitCmd.Store(uint32(cmd))
	m.recordDebugFrame("tx", m.deviceType, report)
	if err := m.blackSharkWriteHandleLocked().WriteReport(report, blackSharkWriteTimeout); err != nil {
		m.link.awaitCmd.Store(0)
		m.logWarn("黑鲨查询 0x%02X 发送失败: %v", cmd, err)
		return deviceproto.BlackSharkFrame{}, false
	}
	return m.blackSharkWaitResponseLocked(cmd, blackSharkQueryTimeout)
}

// blackSharkWaitResponseLocked 等待指定 cmd 的响应帧。
// 响应由读协程投递到 m.link.blackSharkResp，因此这里不占用设备句柄，
// 也不会与读循环争用（读协程是句柄的唯一拥有者）。
func (m *Manager) blackSharkWaitResponseLocked(cmd byte, timeout time.Duration) (deviceproto.BlackSharkFrame, bool) {
	resp := m.link.blackSharkResp
	if resp == nil {
		return deviceproto.BlackSharkFrame{}, false
	}
	// 声明"我在等这个 cmd"：读协程只投递匹配的帧（见 handleBlackSharkHIDRX）。
	mission := m.link.missionSeq.Add(1)
	m.link.awaitCmd.Store(uint32(cmd))
	defer m.link.awaitCmd.Store(0)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case frame := <-resp:
			if frame.Command == cmd {
				return frame, true
			}
			// 应答不匹配就报错（对应官方那句「包序号不匹配, cmd, expect, recv」）——
			// 这类帧本不该出现在这里（读协程已按 cmd 过滤），出现即说明设备回了别的命令。
			m.logWarn("黑鲨应答不匹配（mission #%d）：期望 cmd=0x%02X，收到 cmd=0x%02X 载荷 %d 字节",
				mission, cmd, frame.Command, len(frame.Payload))
		case <-timer.C:
			return deviceproto.BlackSharkFrame{}, false
		}
	}
}

// blackSharkReadCoolingConfigLocked 读取当前生效的冷却配置（0x25）。
// 形态由 form 字节区分：0 = 固定值（5 字节载荷），1 = 温度曲线（15 字节载荷）。
func (m *Manager) blackSharkReadCoolingConfigLocked() (deviceproto.BlackSharkGearConfig, bool) {
	frame, ok := m.blackSharkQueryLocked(deviceproto.BlackSharkCmdGetStatus, nil)
	if !ok {
		return deviceproto.BlackSharkGearConfig{}, false
	}
	return deviceproto.DecodeBlackSharkGearConfig(frame.Payload)
}

// BlackSharkGearConfig 按 (form, gear) 读取某档位的配置（0x26）。
// form = BlackSharkFormValue / BlackSharkFormCurve；gear = 1..4。
func (m *Manager) BlackSharkGearConfig(form, gear byte) (deviceproto.BlackSharkGearConfig, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.blackSharkHandleLocked() == nil {
		return deviceproto.BlackSharkGearConfig{}, false
	}
	payload := deviceproto.EncodeBlackSharkGearQueryPayload(form, gear)
	frame, ok := m.blackSharkQueryLocked(deviceproto.BlackSharkCmdGetAnyCoolingConfig, payload)
	if !ok {
		return deviceproto.BlackSharkGearConfig{}, false
	}
	return deviceproto.DecodeBlackSharkGearConfig(frame.Payload)
}

// ---- 档案判据 ----

const legacyBlackSharkHIDProfileID = "builtin.blackshark.brb02.hid.rpm"

func isLegacyBlackSharkHIDProfileID(id string) bool {
	return strings.EqualFold(strings.TrimSpace(id), legacyBlackSharkHIDProfileID)
}

// isBlackSharkProfileID 转发到 types 的单一判据，本包不再自己列一份 ID。
// 同一个事实只放在一处：判据分散就难免改一处漏一处。
func isBlackSharkProfileID(id string) bool {
	return types.IsBlackSharkDeviceProfileID(id)
}

// ---- 运行时判定与诊断命令 ----

// usesBlackSharkTransport 报告「当前连接的是不是黑鲨、且传输句柄在手」。
// 供调试命令等不持有 m.mutex 的入口做设备族分发，避免误用飞智的帧格式。
func (m *Manager) usesBlackSharkTransport() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.blackSharkHandleLocked() != nil
}

// IsBlackSharkActive 当前活动设备是不是黑鲨、且它的传输句柄在手。
// 供上层在做"重查询"之前先判断，避免对着别的设备白跑一轮查询。
func (m *Manager) IsBlackSharkActive() bool {
	// 语义 = 句柄在手（不是"档案 ID 对得上"）：调用方里有直接摸 m.link / await 状态机的代码，
	// 没句柄时那些路径只能空跑。只按档案判的场合用 IsBlackSharkProfileActive。
	return m.usesBlackSharkTransport()
}

// IsBlackSharkProfileActive 当前连接的设备是不是任意一条黑鲨档案（HID / BLE / USB）。
// 给"档案级"判据用（UI 展示、面板可用性、图传入口）；凡是要摸 HID 句柄的地方仍用 IsBlackSharkActive。
func (m *Manager) IsBlackSharkProfileActive() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.isConnected && types.IsBlackSharkDeviceProfileID(m.activeProfile.ID)
}

// normalizeBlackSharkDebugInput 把调试面板输入规范化为黑鲨 A5 帧。
func normalizeBlackSharkDebugInput(input string) ([]byte, error) {
	data, err := deviceproto.ParseHex(input)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("raw debug command is empty")
	}
	if data[0] == deviceproto.BlackSharkMagic {
		if _, ok := deviceproto.ParseBlackSharkFrame(data); !ok {
			return nil, fmt.Errorf("invalid blackshark frame")
		}
		return data, nil
	}
	return deviceproto.BuildBlackSharkFrame(data[0], data[1:]...), nil
}

// sendBlackSharkHIDDebugCommand 发送原始黑鲨命令。
// 高危入口：可借此触达未标定字段甚至产线命令。
func (m *Manager) sendBlackSharkHIDDebugCommand(input string, waitMs int) (types.DeviceDebugCommandResult, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return types.DeviceDebugCommandResult{}, fmt.Errorf("raw debug command is empty")
	}
	frame, err := normalizeBlackSharkDebugInput(trimmed)
	if err != nil {
		return types.DeviceDebugCommandResult{}, err
	}

	startSeq := m.currentDebugSeq()

	m.mutex.Lock()
	if m.blackSharkWriteHandleLocked() == nil {
		m.mutex.Unlock()
		return types.DeviceDebugCommandResult{}, fmt.Errorf("blackshark device is not connected")
	}
	// 封装按当前传输装（判据取句柄）⇒ 必须在锁内，见 blackSharkBuildReportLocked。
	report, err := m.blackSharkBuildReportLocked(frame)
	if err != nil {
		m.mutex.Unlock()
		return types.DeviceDebugCommandResult{}, err
	}
	m.recordDebugFrame("tx", m.deviceType, report)
	err = m.blackSharkWriteHandleLocked().WriteReport(report, blackSharkWriteTimeout)
	m.mutex.Unlock()
	if err != nil {
		return types.DeviceDebugCommandResult{}, err
	}

	if waitMs > 0 {
		time.Sleep(time.Duration(waitMs) * time.Millisecond)
	}

	return types.DeviceDebugCommandResult{
		Transport: types.DeviceTransportHID,
		InputHex:  input,
		FrameHex:  deviceproto.Hex(frame),
		RawHex:    deviceproto.Hex(report),
		WaitMs:    waitMs,
		Frames:    m.debugFramesAfter(startSeq),
	}, nil
}

// ── 0x07（主机信息推送）的节拍 ──────────────────────────────────────────────
//
// 与官方同形：coreapp 出一个"取条目"的回调，device 侧用一个可设间隔的循环按节拍推。
// 这样图传这类独占任务可以**暂停/恢复**推送，而不是靠写锁隐式挡住
// （官方 `TransferDeviceImage` 前后就是这么调的）。
//
// 挂点也照官方：连接后启动温度监控时设一次间隔（`coreapp.startTemperatureMonitoring`），
// 图传前后各设一次（暂停 0 / 恢复）。

// SetBlackSharkHostInfoRefreshInterval 设定/停止 0x07 推送的节奏。
//
// interval = 0 停；< 0 报错；与当前值相同则什么都不做（不重启循环 —— 官方在 setter 里也有这条短路，
// 因为启动监控这条路径会被反复调用）。provider 每拍取一次条目。
func (m *Manager) SetBlackSharkHostInfoRefreshInterval(
	interval time.Duration,
	provider func() []deviceproto.BlackSharkSystemInfoEntry,
) error {
	if interval < 0 {
		return fmt.Errorf("黑鲨主机信息推送间隔不能为负: %s", interval)
	}
	if m == nil {
		return fmt.Errorf("设备管理器未就绪")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 停：无论当前是不是黑鲨都要把在跑的循环收掉。
	if interval == 0 {
		m.stopBlackSharkHostInfoRefreshLocked()
		m.link.hostInfoInterval = 0
		m.link.hostInfoProvider = nil
		return nil
	}
	// 同值短路：已在按这个间隔推就什么都不做。
	if m.link.hostInfoStop != nil && m.link.hostInfoInterval == interval {
		m.link.hostInfoProvider = provider
		return nil
	}
	// 判据直接读字段，不调 IsBlackSharkProfileActive()：本函数已持 m.mutex 写锁，
	// 那个方法会再取读锁 ⇒ 自死锁（RWMutex 不可重入）。
	if !m.isConnected || !isBlackSharkProfileID(m.activeProfile.ID) {
		return fmt.Errorf("当前设备不是黑鲨，没有主机信息推送")
	}

	m.stopBlackSharkHostInfoRefreshLocked()
	m.link.hostInfoInterval = interval
	m.link.hostInfoProvider = provider
	stop := make(chan struct{})
	m.link.hostInfoStop = stop
	go m.blackSharkHostInfoRefreshLoop(stop, interval, provider)
	return nil
}

// stopBlackSharkHostInfoRefreshLocked 停掉在跑的推送循环（调用方须持 m.mutex）。
func (m *Manager) stopBlackSharkHostInfoRefreshLocked() {
	if m.link.hostInfoStop == nil {
		return
	}
	close(m.link.hostInfoStop)
	m.link.hostInfoStop = nil
}

// blackSharkHostInfoRefreshLoop 按间隔推 0x07，收到停止信号即退出。
// 条目每拍现取（值要随时间变），取不到就跳过这一拍。
func (m *Manager) blackSharkHostInfoRefreshLoop(
	stop <-chan struct{},
	interval time.Duration,
	provider func() []deviceproto.BlackSharkSystemInfoEntry,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if provider == nil {
				continue
			}
			entries := provider()
			if len(entries) == 0 {
				continue
			}
			if !m.SendBlackSharkHostInfoFrame(entries) {
				// 失败不刷屏：这是周期推送，偶发失败（设备刚断开等）下一次会补上。
				m.logDebug("主机信息推送未成功（设备可能刚断开）")
			}
		}
	}
}

// SendBlackSharkHostInfoFrame 推送一帧主机信息（CMD 0x07）。名与官方同形。
func (m *Manager) SendBlackSharkHostInfoFrame(entries []deviceproto.BlackSharkSystemInfoEntry) bool {
	if !m.usesBlackSharkTransport() {
		return false
	}
	payload := deviceproto.BuildBlackSharkSystemInfoPayload(entries)
	if payload == nil {
		m.logWarn("系统信息推送载荷非法（%d 项），已放弃本次推送", len(entries))
		return false
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.writesBlocked.Load() || m.blackSharkHandleLocked() == nil {
		return false
	}
	if err := m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdNotify, payload...); err != nil {
		m.logWarn("系统信息推送失败: %v", err)
		return false
	}
	return true
}

// IssueAudioSpectrumLevel 推一帧音频同步档位（CMD 0x15，载荷 = 档位 1|2|3）。
// 帧率由调用方控制：官方约 4.9 帧/秒（间隔 0.20 s）。
func (m *Manager) IssueAudioSpectrumLevel(level byte) bool {
	if level < deviceproto.BlackSharkAudioLevelLow || level > deviceproto.BlackSharkAudioLevelHigh {
		return false
	}
	if !m.usesBlackSharkTransport() {
		return false
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.writesBlocked.Load() || m.blackSharkHandleLocked() == nil {
		return false
	}
	if err := m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdIssueAudioSpectrumLevel, level); err != nil {
		// 高频路径：只记 debug，不刷日志（约 5 Hz）。
		m.logDebug("音频同步档位发送失败: %v", err)
		return false
	}
	return true
}

// NotifyMouseKeyPress 推一帧「键鼠按下」事件（CMD 0x16，空载荷）。
// 这是「响应」灯效的唯一驱动：设备只认"到达"，不用载荷内容。
func (m *Manager) NotifyMouseKeyPress() bool {
	if !m.usesBlackSharkTransport() {
		return false
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.writesBlocked.Load() || m.blackSharkHandleLocked() == nil {
		return false
	}
	if err := m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdNotifyMouseKeyPress); err != nil {
		m.logDebug("响应灯效事件发送失败: %v", err)
		return false
	}
	return true
}

// ---- 灯效读写与重置 ----

// BlackSharkRgbModes 读取全部灯效模式的参数（CMD 0x14 <索引>，索引 1..8）。
// 0x14 是纯读，不会切换当前生效的模式。
func (m *Manager) BlackSharkRgbModes() ([]types.BlackSharkRgbMode, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.blackSharkHandleLocked() == nil {
		return nil, false
	}
	modes := make([]types.BlackSharkRgbMode, 0, deviceproto.BlackSharkRgbEffectCount)
	any := false
	for idx := 1; idx <= deviceproto.BlackSharkRgbEffectCount; idx++ {
		mode, ok := m.blackSharkReadRgbModeLocked(idx)
		if ok {
			any = true
		}
		modes = append(modes, mode)
	}
	return modes, any
}

// BlackSharkCurrentRgbMode 读取当前生效的灯效模式（CMD 0x13，空载荷）。
func (m *Manager) BlackSharkCurrentRgbMode() (types.BlackSharkRgbMode, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.blackSharkHandleLocked() == nil {
		return types.BlackSharkRgbMode{}, false
	}
	frame, ok := m.blackSharkQueryLocked(deviceproto.BlackSharkCmdGetLighting, nil)
	if !ok {
		return types.BlackSharkRgbMode{}, false
	}
	mode, ok := m.blackSharkRgbModeFromFrameLocked(frame)
	if ok && mode.Known {
		// 0x13 是权威答案，顺手校准当前槽位缓存（host-effect 循环读的就是它）。
		m.blackSharkActiveRgbSlot.Store(int32(mode.Index))
	}
	return mode, ok
}

// BlackSharkCurrentRgbSlot 返回最近一次确认生效的灯效槽位（1..8）；0 = 未知。
// 纯内存读取、不发设备查询，专供约 5 Hz 的 host-effect 循环使用。
func (m *Manager) BlackSharkCurrentRgbSlot() int {
	return int(m.blackSharkActiveRgbSlot.Load())
}

// blackSharkReadRgbModeLocked 读某个索引的灯效参数。调用方需持有 m.mutex。
func (m *Manager) blackSharkReadRgbModeLocked(idx int) (types.BlackSharkRgbMode, bool) {
	frame, ok := m.blackSharkQueryLocked(deviceproto.BlackSharkCmdGetAnyRgbEffects, []byte{byte(idx)})
	if !ok {
		return types.BlackSharkRgbMode{Index: idx}, false
	}
	return m.blackSharkRgbModeFromFrameLocked(frame)
}

// colorControlsView 把「按页颜色控件」表转成展示结构。
func colorControlsView(slot int) types.BlackSharkRgbColorControlsView {
	c := deviceproto.BlackSharkRgbColorControlsBySlot[slot]
	return types.BlackSharkRgbColorControlsView{
		ColorMode: c.ColorMode, SingleColor: c.SingleColor, Hue: string(c.Hue),
	}
}

// blackSharkRgbModeFromFrameLocked 把 0x13/0x14 的回复帧转成展示结构。
func (m *Manager) blackSharkRgbModeFromFrameLocked(frame deviceproto.BlackSharkFrame) (types.BlackSharkRgbMode, bool) {
	eff, ok := deviceproto.DecodeBlackSharkRgbEffects(frame.Payload)
	if !ok {
		return types.BlackSharkRgbMode{}, false
	}
	r, g, b := eff.ColorRGB()
	// 速度量程随模式各不相同，见 BlackSharkRgbSpeedRange；
	// 未登记的模式退回空区间，由前端据此禁用拖动，不臆造数值。
	// rg.Fixed 表示官方把该模式的速度滑块置灰（如常亮、音频同步）。
	speedRange := types.ValueRangeView{}
	speedDisabled := false
	if rg, ok := deviceproto.BlackSharkRgbSpeedRange[eff.ModeIndex()]; ok {
		speedRange = types.ValueRangeView{Min: rg.Min, Max: rg.Max}
		speedDisabled = rg.Fixed
	}
	// NeedsHostData：设备支持但本工具驱动不了的模式（音频同步 / 响应）在此带出原因，
	// 界面必须显示，否则"点了没反应"会被当成软件故障。
	// 速度直通：payload[1..2] 就是官方 UI 的滑块值本身。
	mode := types.BlackSharkRgbMode{
		Index:         eff.ModeIndex(),
		Name:          deviceproto.BlackSharkRgbEffectNames[eff.ModeIndex()],
		NeedsHostData: deviceproto.BlackSharkRgbEffectNeedsHostData[eff.ModeIndex()],
		Speed:         eff.Speed(),
		Brightness:    eff.Brightness(),
		StaticColor:   eff.StaticColor(),
		// 颜色下拉选项，取自 payload[0] 的高半字节。
		ColorOption:     eff.ColorOption(),
		ColorOptionName: deviceproto.BlackSharkRgbColorOptionNames[eff.ColorOption()],
		// 该页显示哪些颜色控件，见 deviceproto.BlackSharkRgbColorControlsBySlot。
		ColorControls: colorControlsView(eff.ModeIndex()),
		Red:           r, Green: g, Blue: b,
		// 超出量程的真实读数原样展示，由前端按范围决定是否可拖动，不在此截断。
		SpeedRange:      speedRange,
		SpeedDisabled:   speedDisabled,
		BrightnessRange: types.ValueRangeView{Min: 10, Max: 100},
		Known:           true,
	}
	return mode, true
}

// SelectBlackSharkRgbMode 切换当前生效的灯效模式。
// 切换模式 = 用 0x12 写入该模式的整块参数，协议里没有单独的"选模式"命令。
func (m *Manager) SelectBlackSharkRgbMode(idx int) bool {
	if idx < 1 || idx > deviceproto.BlackSharkRgbEffectCount {
		m.logError("黑鲨灯效模式索引越界: %d（有效 1..%d）", idx, deviceproto.BlackSharkRgbEffectCount)
		return false
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.blackSharkWriteHandleLocked() == nil {
		return false
	}
	// 1) 读目标模式的原始块（不自己构造载荷）
	frame, ok := m.blackSharkQueryLocked(deviceproto.BlackSharkCmdGetAnyRgbEffects, []byte{byte(idx)})
	if !ok {
		m.logError("黑鲨灯效模式 %d 参数读取失败，无法切换", idx)
		return false
	}
	target, ok := deviceproto.DecodeBlackSharkRgbEffects(frame.Payload)
	if !ok || target.ModeIndex() != idx {
		m.logError("黑鲨灯效模式 %d 参数异常（解析=%v 读到索引=%d）",
			idx, ok, target.ModeIndex())
		return false
	}
	payload := target.EncodeBlackSharkRgbEffects()

	// 2) 原样写回该块 —— 写 0x12 即"应用并选中"这个模式
	if err := m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdSetLighting, payload...); err != nil {
		m.logError("黑鲨灯效模式切换失败: %v", err)
		return false
	}

	// 3) 回读确认：0x13 必须报出目标索引
	cur, ok := m.blackSharkQueryLocked(deviceproto.BlackSharkCmdGetLighting, nil)
	if !ok {
		m.logError("黑鲨灯效模式切换后回读失败（0x13 无响应），无法确认生效")
		return false
	}
	got, ok := deviceproto.DecodeBlackSharkRgbEffects(cur.Payload)
	if !ok || got.ModeIndex() != idx {
		m.logError("黑鲨灯效模式回读不一致: 期望 %d，实际 %v（ok=%v）", idx, got.ModeIndex(), ok)
		return false
	}
	m.logDebug("黑鲨灯效模式已切到 %d（0x13 回读确认）", idx)
	// 0x13 已确认选中，这是"选模式"的权威路径，更新当前槽位缓存；
	// coreapp 的 host-effect 循环据此决定是否跑键鼠钩子 / 音频采集。
	m.blackSharkActiveRgbSlot.Store(int32(idx))
	return true
}

// rgbEffectsChange 描述一次灯效"读-改-写"要改什么、改完怎么核对。
// mutate 与 verify 成对出现，避免回读校验漏掉本次实际改动的字段。
type rgbEffectsChange struct {
	what   string // 日志用，例如 "速度/亮度"、"颜色"
	mutate func(deviceproto.BlackSharkRgbEffects) deviceproto.BlackSharkRgbEffects
	verify func(got, want deviceproto.BlackSharkRgbEffects) error
}

// setBlackSharkRgbModeChangeLocked 对某个灯效模式做一次"读-改-写 + 回读校验"。
// 调用方必须已持有 m.mutex；三步顺序不能改：读原始块 → 整块写回 → 回读校验。
func (m *Manager) setBlackSharkRgbModeChangeLocked(idx int, change rgbEffectsChange) bool {
	if idx < 1 || idx > deviceproto.BlackSharkRgbEffectCount {
		m.logError("黑鲨灯效模式索引越界: %d（有效 1..%d）", idx, deviceproto.BlackSharkRgbEffectCount)
		return false
	}
	if m.blackSharkWriteHandleLocked() == nil {
		return false
	}

	// 1) 读回原始块（读-改-写的第一步，绝不凭空构造）
	frame, ok := m.blackSharkQueryLocked(deviceproto.BlackSharkCmdGetAnyRgbEffects, []byte{byte(idx)})
	if !ok {
		m.logError("黑鲨灯效模式 %d 参数读取失败，拒绝盲写", idx)
		return false
	}
	current, ok := deviceproto.DecodeBlackSharkRgbEffects(frame.Payload)
	if !ok {
		m.logError("黑鲨灯效模式 %d 参数解析失败（长度 %d），拒绝盲写", idx, len(frame.Payload))
		return false
	}
	target := change.mutate(current)
	payload := target.EncodeBlackSharkRgbEffects()
	if len(payload) != 8 {
		m.logError("黑鲨灯效载荷长度异常: %d（应为 8）", len(payload))
		return false
	}

	// 2) 整块写回
	if err := m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdSetLighting, payload...); err != nil {
		m.logError("黑鲨灯效参数下发失败: %v", err)
		return false
	}

	// 3) 回读校验
	back, ok := m.blackSharkQueryLocked(deviceproto.BlackSharkCmdGetAnyRgbEffects, []byte{byte(idx)})
	if !ok {
		m.logError("黑鲨灯效参数写入后回读失败（0x14 无响应），无法确认生效")
		return false
	}
	got, ok := deviceproto.DecodeBlackSharkRgbEffects(back.Payload)
	if !ok {
		m.logError("黑鲨灯效参数回读解析失败，无法确认生效")
		return false
	}
	if err := change.verify(got, target); err != nil {
		m.logError("黑鲨灯效 %s 回读不一致（模式 %d）: %v", change.what, idx, err)
		return false
	}
	m.logDebug("黑鲨灯效 %d 的 %s 已写入并经回读确认: %+v", idx, change.what, got)
	// 0x12 写哪个槽位即选中哪个槽位（没有单独的"选模式"命令），记下它；
	// coreapp 的 host-effect 循环靠这个判断该不该跑键鼠钩子 / 音频采集。
	m.blackSharkActiveRgbSlot.Store(int32(idx))
	return true
}

// SetBlackSharkRgbModeEffects 修改某个模式的速度与亮度（CMD 0x12，8 字节载荷）。
// 只改这两项，其余字段走读-改-写原样保留。
func (m *Manager) SetBlackSharkRgbModeEffects(idx, speed, brightness int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.setBlackSharkRgbModeChangeLocked(idx, rgbEffectsChange{
		what: "速度/亮度",
		mutate: func(cur deviceproto.BlackSharkRgbEffects) deviceproto.BlackSharkRgbEffects {
			return cur.WithSpeedBrightness(speed, brightness)
		},
		verify: func(got, want deviceproto.BlackSharkRgbEffects) error {
			if got.Speed() != want.Speed() || got.Brightness() != want.Brightness() {
				return fmt.Errorf("期望 speed=%d brightness=%d，实际 speed=%d brightness=%d",
					want.Speed(), want.Brightness(), got.Speed(), got.Brightness())
			}
			return nil
		},
	})
}

// SetBlackSharkRgbModeColor 修改某个模式的颜色，并同时设置静态颜色标志。
func (m *Manager) SetBlackSharkRgbModeColor(idx int, r, g, b uint8, staticColor bool) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.setBlackSharkRgbModeChangeLocked(idx, rgbEffectsChange{
		what: "颜色/静态色标志",
		mutate: func(cur deviceproto.BlackSharkRgbEffects) deviceproto.BlackSharkRgbEffects {
			return cur.WithColor(r, g, b).WithStaticColor(staticColor)
		},
		verify: func(got, want deviceproto.BlackSharkRgbEffects) error {
			gr, gg, gb := got.ColorRGB()
			wr, wg, wb := want.ColorRGB()
			if gr != wr || gg != wg || gb != wb {
				return fmt.Errorf("期望 RGB=%02X%02X%02X，实际 RGB=%02X%02X%02X", wr, wg, wb, gr, gg, gb)
			}
			if got.Field16 != want.Field16 {
				return fmt.Errorf("期望静态色标志=0x%02X，实际 0x%02X", want.Field16, got.Field16)
			}
			return nil
		},
	})
}

// SetBlackSharkRgbModeColorOption 修改某个模式的颜色下拉选项（0x12 的 payload[0] 高半字节）。
func (m *Manager) SetBlackSharkRgbModeColorOption(idx, option int) bool {
	if option < 0 || option >= deviceproto.BlackSharkRgbColorOptionCount {
		m.logError("黑鲨颜色选项越界: %d（有效 0..%d）",
			option, deviceproto.BlackSharkRgbColorOptionCount-1)
		return false
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.setBlackSharkRgbModeChangeLocked(idx, rgbEffectsChange{
		what: "颜色模式",
		mutate: func(cur deviceproto.BlackSharkRgbEffects) deviceproto.BlackSharkRgbEffects {
			return cur.WithColorOption(option)
		},
		verify: func(got, want deviceproto.BlackSharkRgbEffects) error {
			if got.ColorOption() != want.ColorOption() {
				return fmt.Errorf("期望颜色选项=%d，实际=%d", want.ColorOption(), got.ColorOption())
			}
			return nil
		},
	})
}

// ---- 灯效重置与读取 ----

// RestoreBlackSharkRgbModeEffects 把某个模式整块写回为给定参数（还原专用）。
// 不能拿 SelectBlackSharkRgbMode 代替：还原需要写回原始块而非设备当前块。
func (m *Manager) RestoreBlackSharkRgbModeEffects(e deviceproto.BlackSharkRgbEffects) bool {
	idx := e.ModeIndex()
	if idx < 1 || idx > deviceproto.BlackSharkRgbEffectCount {
		m.logError("黑鲨灯效模式索引越界: %d（有效 1..%d）", idx, deviceproto.BlackSharkRgbEffectCount)
		return false
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.blackSharkWriteHandleLocked() == nil {
		return false
	}
	payload := e.EncodeBlackSharkRgbEffects()
	if err := m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdSetLighting, payload...); err != nil {
		m.logError("黑鲨灯效还原写入失败: %v", err)
		return false
	}
	back, ok := m.blackSharkQueryLocked(deviceproto.BlackSharkCmdGetAnyRgbEffects, []byte{byte(idx)})
	if !ok {
		m.logError("黑鲨灯效还原后回读失败（0x14 无响应）")
		return false
	}
	got, ok := deviceproto.DecodeBlackSharkRgbEffects(back.Payload)
	if !ok {
		m.logError("黑鲨灯效还原回读解析失败")
		return false
	}
	gr, gg, gb := got.ColorRGB()
	wr, wg, wb := e.ColorRGB()
	if got.ModeIndex() != idx || got.Speed() != e.Speed() || got.Brightness() != e.Brightness() ||
		gr != wr || gg != wg || gb != wb {
		m.logError("黑鲨灯效还原不一致（模式 %d）: 期望 %s，实际 %s",
			idx, deviceproto.Hex(payload), deviceproto.Hex(got.EncodeBlackSharkRgbEffects()))
		return false
	}
	if got.Field4 != e.Field4 || got.Field16 != e.Field16 {
		// 不是错误：这两个字段含义未定，设备可能自行归一化。记下来是为了留证据。
		m.logWarn("黑鲨灯效还原时两个未定字段与旧值不同（设备归一化？）: 期望 Field4=%d Field16=%d，实际 Field4=%d Field16=%d",
			e.Field4, e.Field16, got.Field4, got.Field16)
	}
	return true
}

// ResetBlackSharkRgbModes 把 8 个灯效槽位全部恢复为出厂灯效（= 官方「灯效页 · 重置」）。
func (m *Manager) ResetBlackSharkRgbModes() bool {
	blocks := deviceproto.BlackSharkOfficialResetBlocks
	if len(blocks) == 0 {
		return false
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.blackSharkWriteHandleLocked() == nil {
		return false
	}
	for i, blk := range blocks {
		if err := m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdSetLighting, blk...); err != nil {
			m.logError("黑鲨灯效重置第 %d/%d 条写入失败: %v", i+1, len(blocks), err)
			return false
		}
	}
	// 回读确认：当前生效模式应回到槽 1，且参数等于最后写入的那条。
	// 但"回读不到"不等于"操作没生效"，所以下面允许重试，重试仍失败也不报错。
	var got deviceproto.BlackSharkRgbEffects
	confirmed := false
	for attempt := 0; attempt < 3 && !confirmed; attempt++ {
		if attempt > 0 {
			// 沉降：刚批量写完 9 条之后设备需要一点时间才肯回 0x13。
			time.Sleep(150 * time.Millisecond)
		}
		frame, ok := m.blackSharkQueryLocked(deviceproto.BlackSharkCmdGetLighting, nil)
		if !ok {
			continue
		}
		decoded, ok := deviceproto.DecodeBlackSharkRgbEffects(frame.Payload)
		if !ok {
			continue
		}
		got = decoded
		confirmed = true
	}
	if !confirmed {
		m.logWarn("黑鲨灯效重置：9 条写入已全部下发，但 0x13 回读 3 次均无响应，" +
			"只是无法回读确认、不代表失败（设备侧通常已重置）；不再向上报「操作未生效」")
		// 不猜当前模式：回读不到就不更新槽位缓存。
		return true
	}
	// 期望值取重置后的稳定状态 BlackSharkOfficialResetExpected，
	// 而不是 blocks[len(blocks)-1]——"最后写的那条"只是推断。
	gotHex := got.EncodeBlackSharkRgbEffects()
	if !bytes.Equal(gotHex, deviceproto.BlackSharkOfficialResetExpected) {
		m.logError("黑鲨灯效重置后回读与记录的出厂态不一致: 期望 %s，实际 %s",
			deviceproto.Hex(deviceproto.BlackSharkOfficialResetExpected), deviceproto.Hex(gotHex))
		return false
	}
	m.logInfo("黑鲨灯效已重置为出厂（8 槽位），当前模式 = 槽 %d（%s）",
		got.ModeIndex(), deviceproto.BlackSharkRgbEffectNames[got.ModeIndex()])
	m.blackSharkActiveRgbSlot.Store(int32(got.ModeIndex()))
	return true
}

// ReadBlackSharkRgbModeEffects 只读某个模式的 8 字节参数块（0x14，纯读不切换）。
//
// 标定要用它读回"原始值"以便还原，界面也要用它展示。
func (m *Manager) ReadBlackSharkRgbModeEffects(idx int) (deviceproto.BlackSharkRgbEffects, bool) {
	if idx < 1 || idx > deviceproto.BlackSharkRgbEffectCount {
		return deviceproto.BlackSharkRgbEffects{}, false
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.isConnected || m.blackSharkHandleLocked() == nil {
		return deviceproto.BlackSharkRgbEffects{}, false
	}
	frame, ok := m.blackSharkQueryLocked(deviceproto.BlackSharkCmdGetAnyRgbEffects, []byte{byte(idx)})
	if !ok {
		return deviceproto.BlackSharkRgbEffects{}, false
	}
	return deviceproto.DecodeBlackSharkRgbEffects(frame.Payload)
}

// ---- 配置快照与固件版本 ----

// BlackSharkConfigSnapshot 读一份设备配置快照（灯效块 + 曲线 + 可读回的开关）。
// 这是"恢复出厂"的保守替代：只回到已知的好状态，且完全可逆。
func (m *Manager) BlackSharkConfigSnapshot() (types.BlackSharkConfigSnapshot, bool) {
	if !m.IsBlackSharkActive() {
		return types.BlackSharkConfigSnapshot{}, false
	}
	snap := types.BlackSharkConfigSnapshot{
		Version: 1,
		TakenAt: time.Now().Format(time.RFC3339),
	}
	for i := 1; i <= deviceproto.BlackSharkRgbEffectCount; i++ {
		e, ok := m.ReadBlackSharkRgbModeEffects(i)
		if !ok {
			snap.Notes = append(snap.Notes, fmt.Sprintf("灯效模式 %d 参数读不到，未纳入快照", i))
			continue
		}
		// 存原始字节而不是解释后的字段：还原时整块原样写回，少一次解释错字段的机会。
		snap.RgbModes = append(snap.RgbModes, types.BlackSharkRgbModeSnapshot{
			Index: i,
			Raw:   hex.EncodeToString(e.EncodeBlackSharkRgbEffects()),
		})
	}
	if cur, ok := m.BlackSharkCurrentRgbMode(); ok {
		snap.CurrentRgbMode = cur.Index
	} else {
		snap.Notes = append(snap.Notes, "当前生效的灯效模式读不到，还原后可能停在其他模式")
	}
	for g := 1; g <= deviceproto.BlackSharkGearCount; g++ {
		cfg, ok := m.BlackSharkGearConfig(deviceproto.BlackSharkFormCurve, byte(g))
		if !ok || cfg.Form != deviceproto.BlackSharkFormCurve {
			// 固定值形态的档位不在本项目已实现的写入路径内 —— 如实记为"未纳入"。
			snap.Notes = append(snap.Notes,
				fmt.Sprintf("档位 %d 不是温度曲线形态（或读不到），未纳入快照", g))
			continue
		}
		gs := types.BlackSharkGearSnapshot{Gear: g, Form: int(cfg.Form), Value: int(cfg.Value)}
		for _, p := range cfg.Curve {
			gs.Temps = append(gs.Temps, int(p.TempC))
			gs.RPM = append(gs.RPM, int(p.RPM))
		}
		snap.GearCurves = append(snap.GearCurves, gs)
	}
	st := m.BlackSharkSwitchStates()
	snap.Switches = types.BlackSharkSwitchSnapshot{
		LightingEnabled:  st.LightingEnabled,
		LcdEnabled:       st.LcdScreenEnabled,
		CoolingSource:    int(st.CoolingSource),
		SmartStartStop:   st.SmartStartStop,
		PowerOnSelfStart: st.PowerOnSelfStart,
	}
	for _, miss := range []struct {
		known bool
		what  string
	}{
		{st.LightingKnown, "灯效开关"}, {st.LcdScreenKnown, "屏幕开关"},
		{st.CoolingSourceKnown, "制冷来源"}, {st.OnOffVectorKnown, "智能启停/通电自启"},
	} {
		if !miss.known {
			snap.Notes = append(snap.Notes, miss.what+"读回失败，快照里是兜底值")
		}
	}
	snap.Notes = append(snap.Notes, "风扇开关没有读回命令，不在快照范围内")
	// 屏幕显示参数（0xC2）同理只有 Set 没有 Get，无法纳入快照。
	snap.Notes = append(snap.Notes, "屏幕显示参数（0xC2）没有读回命令，快照不包含它；如需恢复请用屏幕参数区重新下发")
	return snap, true
}

// RestoreBlackSharkConfig 把一份快照写回设备。
func (m *Manager) RestoreBlackSharkConfig(snap types.BlackSharkConfigSnapshot) ([]string, []string) {
	var restored, issues []string

	for _, r := range snap.RgbModes {
		raw, err := hex.DecodeString(strings.TrimSpace(r.Raw))
		if err != nil || len(raw) != 8 {
			issues = append(issues, fmt.Sprintf("灯效模式 %d 的字节无效，已跳过", r.Index))
			continue
		}
		e, ok := deviceproto.DecodeBlackSharkRgbEffects(raw)
		if !ok {
			issues = append(issues, fmt.Sprintf("灯效模式 %d 的字节解析失败，已跳过", r.Index))
			continue
		}
		if e.ModeIndex() != r.Index {
			issues = append(issues, fmt.Sprintf("灯效模式 %d 的快照模式号不符（%d），已跳过", r.Index, e.ModeIndex()))
			continue
		}
		if m.RestoreBlackSharkRgbModeEffects(e) {
			restored = append(restored, fmt.Sprintf("灯效模式 %d", r.Index))
		} else {
			issues = append(issues, fmt.Sprintf("灯效模式 %d 还原失败（回读不一致或设备无响应）", r.Index))
		}
	}
	// 0x12 的副作用是"写入即切为当前模式"，所以最后要把当前模式选回去。
	if snap.CurrentRgbMode > 0 && len(snap.RgbModes) > 0 {
		if m.SelectBlackSharkRgbMode(snap.CurrentRgbMode) {
			restored = append(restored, fmt.Sprintf("当前灯效模式 %d", snap.CurrentRgbMode))
		} else {
			issues = append(issues, fmt.Sprintf("当前灯效模式 %d 选回失败", snap.CurrentRgbMode))
		}
	}

	// 档位曲线不原样写回设备：设备端只允许平曲线承载，
	// 曲线形状只存在于主机侧（它是插值的依据）。
	for _, g := range snap.GearCurves {
		if g.Form != int(deviceproto.BlackSharkFormCurve) {
			issues = append(issues,
				fmt.Sprintf("档位 %d 是固定值形态，本项目只实现了曲线形态的写入，未还原", g.Gear))
			continue
		}
		// 设备侧曲线就是「形状」本身，还原时要把快照里的曲线写回设备，不能只写平承载。
		if len(g.Temps) != deviceproto.BlackSharkCurvePointCount || len(g.RPM) != deviceproto.BlackSharkCurvePointCount {
			issues = append(issues, fmt.Sprintf("档位 %d 的快照曲线点数不是 %d，跳过", g.Gear, deviceproto.BlackSharkCurvePointCount))
			continue
		}
		var points [deviceproto.BlackSharkCurvePointCount]deviceproto.BlackSharkCurvePoint
		for k := range points {
			points[k] = deviceproto.BlackSharkCurvePoint{TempC: uint8(g.Temps[k]), RPM: uint16(g.RPM[k])}
		}
		if !m.SetBlackSharkCoolingCurve(byte(g.Gear), points) {
			issues = append(issues, fmt.Sprintf("档位 %d 的曲线写回失败（回读不一致或无响应）", g.Gear))
			continue
		}
		restored = append(restored, fmt.Sprintf("档位 %d 已写回快照曲线", g.Gear))
	}

	s := snap.Switches
	for _, step := range []struct {
		what string
		fn   func() bool
	}{
		{"灯效开关", func() bool { return m.SetBlackSharkLightingEnabled(s.LightingEnabled) }},
		{"屏幕开关", func() bool { return m.SetBlackSharkLcdScreenEnabled(s.LcdEnabled) }},
		{"温控参照", func() bool { return m.SetBlackSharkCoolingSource(uint8(s.CoolingSource)) }},
		{"智能启停/通电自启", func() bool { return m.SetBlackSharkOnOffVector(s.SmartStartStop, s.PowerOnSelfStart) }},
	} {
		if step.fn() {
			restored = append(restored, step.what)
		} else {
			issues = append(issues, step.what+"还原失败")
		}
	}
	issues = append(issues, "风扇开关没有读回命令、也不在快照内，未参与还原")
	return restored, issues
}

// BlackSharkInfo 聚合黑鲨设备的全部可展示信息，供前端一次取全。
func (m *Manager) BlackSharkInfo() types.BlackSharkInfo {
	info := types.BlackSharkInfo{}
	m.mutex.RLock()
	available := m.blackSharkHandleLocked() != nil
	m.mutex.RUnlock()
	if !available {
		return info
	}
	info.Available = true

	info.Firmware = types.BlackSharkFirmwareStatus{UpdateMethod: "official-tool"}
	if status, ok := m.CachedBlackSharkFirmwareStatus(); ok {
		info.Firmware = status
	}
	info.Switches = m.BlackSharkSwitchStates()

	for gear := 1; gear <= deviceproto.BlackSharkGearCount; gear++ {
		row := types.BlackSharkGear{Gear: gear}
		if cfg, ok := m.BlackSharkGearConfig(deviceproto.BlackSharkFormValue, byte(gear)); ok {
			row.FixedValue = cfg.Value
			row.FixedKnown = true
		}
		if cfg, ok := m.BlackSharkGearConfig(deviceproto.BlackSharkFormCurve, byte(gear)); ok {
			row.CurveKnown = true
			for _, pt := range cfg.Curve {
				row.Curve = append(row.Curve, types.BlackSharkCurvePointView{
					TempC:           pt.TempC,
					FieldRPM:        pt.RPM,
					ApproxActualRPM: deviceproto.BlackSharkCurveRPMForField(int(pt.RPM)),
				})
			}
		}
		info.Gears = append(info.Gears, row)
	}

	info.CurveMinRPM, info.CurveMaxRPM = deviceproto.BlackSharkCurveRangeForRPM()
	for _, c := range deviceproto.BlackSharkCurveCalibration {
		info.CurveCalibration = append(info.CurveCalibration, types.BlackSharkCurveCalibrationEntry{
			FieldRPM: c.Field, ActualRPM: c.RPM,
		})
	}
	info.FixedMinRPM = deviceproto.BlackSharkMinRPM
	info.FixedMaxRPM = deviceproto.BlackSharkMaxRPM
	return info
}

// ---- 固件清单 / 版本 / 状态 ----

// 固件版本检查。
const (
	// blackSharkFirmwareManifestURL 与官方软件同一数据源。
	blackSharkFirmwareManifestURL = "https://blackshark-cn.oss-cn-shanghai.aliyuncs.com/Support/PC_APP/BRB02/Firmware.BRB02.Cooler_LastVersionNo.zip"

	blackSharkFirmwareManifestTimeout = 6 * time.Second
	blackSharkFirmwareCacheTTL        = 6 * time.Hour
	blackSharkFirmwareMaxManifestSize = 512 * 1024
	blackSharkUpdateMethodOfficial    = "official-tool"
)

// blackSharkFirmwareManifest 厂商版本清单（VersionInfo 段）。
type blackSharkFirmwareManifest struct {
	LastVersion    string
	LastVersionURL string
	LastVersionMD5 string
	UserMinVersion string
}

func (mf blackSharkFirmwareManifest) empty() bool {
	return strings.TrimSpace(mf.LastVersion) == ""
}

// officialBlackSharkToolCandidates 官方工具常见安装位置。
func officialBlackSharkToolCandidates() []string {
	return []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "BlackSharkEquipmentBox", "BlackSharkEquipmentBox.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "BlackSharkEquipmentBox", "BlackSharkEquipmentBox.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "BlackSharkEquipmentBox", "BlackSharkEquipmentBox.exe"),
	}
}

func firstExistingBlackSharkTool() string {
	for _, candidate := range officialBlackSharkToolCandidates() {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// parseBlackSharkFirmwareManifest 解析厂商版本清单。
// 兼容「直接返回 INI 文本」与「返回 zip 包」两种形态。
func parseBlackSharkFirmwareManifest(data []byte) (blackSharkFirmwareManifest, error) {
	if len(data) == 0 {
		return blackSharkFirmwareManifest{}, fmt.Errorf("firmware manifest is empty")
	}
	payload := data
	if len(payload) >= 4 && payload[0] == 'P' && payload[1] == 'K' {
		text, err := extractINITextFromZip(payload)
		if err != nil {
			return blackSharkFirmwareManifest{}, err
		}
		payload = []byte(text)
	}
	manifest := parseBlackSharkVersionINIText(string(payload))
	if manifest.empty() {
		return blackSharkFirmwareManifest{}, fmt.Errorf("firmware manifest has no VersionInfo/LastVersion")
	}
	return manifest, nil
}

func extractINITextFromZip(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("firmware manifest zip is unreadable: %w", err)
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(file.Name), ".ini") {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(io.LimitReader(rc, blackSharkFirmwareMaxManifestSize))
		rc.Close()
		if err != nil {
			continue
		}
		if text := parseBlackSharkVersionINIText(string(content)); !text.empty() {
			return string(content), nil
		}
	}
	return "", fmt.Errorf("firmware manifest zip contains no usable ini")
}

// parseBlackSharkVersionINIText 解析 INI 文本，只取 [VersionInfo] 段。
// 兼容 UTF-8 BOM、CRLF、以及键值两侧空白。
func parseBlackSharkVersionINIText(text string) blackSharkFirmwareManifest {
	text = strings.TrimPrefix(text, "\ufeff")
	var manifest blackSharkFirmwareManifest
	inVersionInfo := false
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inVersionInfo = strings.EqualFold(line, "[VersionInfo]")
			continue
		}
		if !inVersionInfo {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "LastVersion":
			manifest.LastVersion = value
		case "LastVersionURL":
			manifest.LastVersionURL = value
		case "LastVersionMD5":
			manifest.LastVersionMD5 = value
		case "UserMinVersion":
			manifest.UserMinVersion = value
		}
	}
	return manifest
}

// compareBlackSharkVersions 比较点分版本号。
// 返回 -1 / 0 / 1；非数字片段按字符串比较，缺失段按 0 处理。
func compareBlackSharkVersions(a, b string) int {
	aParts := strings.Split(strings.TrimSpace(a), ".")
	bParts := strings.Split(strings.TrimSpace(b), ".")
	length := len(aParts)
	if len(bParts) > length {
		length = len(bParts)
	}
	for i := 0; i < length; i++ {
		// 缺失段按 "0" 处理：这样 "3.0" 与 "3.0.0" 相等，
		// 而不是因为空串 < "0" 被误判为旧版本。
		left, right := "0", "0"
		if i < len(aParts) {
			left = strings.TrimSpace(aParts[i])
		}
		if i < len(bParts) {
			right = strings.TrimSpace(bParts[i])
		}
		if left == "" {
			left = "0"
		}
		if right == "" {
			right = "0"
		}
		leftNum, leftErr := strconv.Atoi(left)
		rightNum, rightErr := strconv.Atoi(right)
		if leftErr == nil && rightErr == nil {
			switch {
			case leftNum < rightNum:
				return -1
			case leftNum > rightNum:
				return 1
			}
			continue
		}
		if left == right {
			continue
		}
		if left < right {
			return -1
		}
		return 1
	}
	return 0
}

// blackSharkDeviceFirmwareVersionLocked 读取设备固件版本（CMD 0x01）。
func (m *Manager) blackSharkDeviceFirmwareVersionLocked() (string, bool) {
	if m.blackSharkHandleLocked() == nil {
		return "", false
	}
	frame, ok := m.blackSharkQueryLocked(deviceproto.BlackSharkCmdVersion, nil)
	if !ok {
		return "", false
	}
	return deviceproto.BlackSharkFirmwareVersion(frame)
}

// BlackSharkFirmwareStatus 返回固件检查结果。
// 优先返回缓存（连接时已在后台刷新），缓存缺失才触发一次网络检查，
// 因此可以安全地在 GetDebugInfo 这类热路径调用。
func (m *Manager) BlackSharkFirmwareStatus(ctx context.Context) types.BlackSharkFirmwareStatus {
	if cached := m.cachedBlackSharkFirmwareStatus(); cached != nil {
		return *cached
	}
	return m.RefreshBlackSharkFirmwareStatus(ctx)
}

// RefreshBlackSharkFirmwareStatus 强制重新检查一次固件版本并更新缓存。
func (m *Manager) RefreshBlackSharkFirmwareStatus(ctx context.Context) types.BlackSharkFirmwareStatus {
	toolPath := firstExistingBlackSharkTool()
	status := types.BlackSharkFirmwareStatus{
		UpdateMethod:      blackSharkUpdateMethodOfficial,
		OfficialToolPath:  toolPath,
		OfficialToolFound: toolPath != "",
	}

	m.mutex.Lock()
	isBlackShark := m.blackSharkHandleLocked() != nil
	if isBlackShark {
		if version, ok := m.blackSharkDeviceFirmwareVersionLocked(); ok {
			status.Supported = true
			status.CurrentVersion = strings.TrimSpace(strings.Trim(version, "\x00"))
		}
	}
	m.mutex.Unlock()

	if !isBlackShark {
		m.storeBlackSharkFirmwareStatus(status)
		return status
	}
	status.Supported = true

	manifest, ok := m.loadBlackSharkFirmwareManifest(ctx)
	if !ok {
		status.Error = "无法获取官方固件版本清单（网络不可达或清单格式变化）"
		m.storeBlackSharkFirmwareStatus(status)
		return status
	}

	status.LatestVersion = manifest.LastVersion
	status.MinVersion = manifest.UserMinVersion
	status.FirmwareURL = manifest.LastVersionURL
	status.FirmwareMD5 = manifest.LastVersionMD5
	status.CheckedAt = m.blackSharkManifestCheckedAt()

	if status.CurrentVersion != "" {
		status.UpdateAvailable = compareBlackSharkVersions(status.CurrentVersion, manifest.LastVersion) < 0
		if manifest.UserMinVersion != "" {
			status.BelowMinVersion = compareBlackSharkVersions(status.CurrentVersion, manifest.UserMinVersion) < 0
		}
	}
	m.storeBlackSharkFirmwareStatus(status)
	return status
}

// refreshBlackSharkFirmwareStatusAsync 在设备连接后于后台刷新一次，
// 避免把网络与设备查询放进连接路径的关键路径里。
func (m *Manager) refreshBlackSharkFirmwareStatusAsync() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.logWarn("黑鲨固件版本检查异常: %v", r)
			}
		}()
		m.RefreshBlackSharkFirmwareStatus(context.Background())
	}()
}

// CachedBlackSharkFirmwareStatus 读取缓存的固件检查结果，
// 不触发网络请求或设备查询。供 GetDebugInfo 等热路径使用。
func (m *Manager) CachedBlackSharkFirmwareStatus() (types.BlackSharkFirmwareStatus, bool) {
	cached := m.cachedBlackSharkFirmwareStatus()
	if cached == nil {
		return types.BlackSharkFirmwareStatus{}, false
	}
	return *cached, true
}

func (m *Manager) cachedBlackSharkFirmwareStatus() *types.BlackSharkFirmwareStatus {
	return m.blackSharkFirmware.Load()
}

func (m *Manager) storeBlackSharkFirmwareStatus(status types.BlackSharkFirmwareStatus) {
	snapshot := status
	m.blackSharkFirmware.Store(&snapshot)
}

func (m *Manager) loadBlackSharkFirmwareManifest(ctx context.Context) (blackSharkFirmwareManifest, bool) {
	if manifest, ok := m.cachedBlackSharkManifest(); ok {
		return manifest, true
	}
	if ctx == nil {
		ctx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(ctx, blackSharkFirmwareManifestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, blackSharkFirmwareManifestURL, nil)
	if err != nil {
		return blackSharkFirmwareManifest{}, false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return blackSharkFirmwareManifest{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		m.logWarn("黑鲨固件清单请求返回 HTTP %d", resp.StatusCode)
		return blackSharkFirmwareManifest{}, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, blackSharkFirmwareMaxManifestSize))
	if err != nil {
		return blackSharkFirmwareManifest{}, false
	}
	manifest, err := parseBlackSharkFirmwareManifest(body)
	if err != nil {
		m.logWarn("黑鲨固件清单解析失败: %v", err)
		return blackSharkFirmwareManifest{}, false
	}
	m.storeBlackSharkManifest(manifest)
	return manifest, true
}

// 版本清单缓存。用原子指针，避免为了读一个缓存去抢 m.mutex。
type blackSharkManifestCache struct {
	manifest blackSharkFirmwareManifest
	at       time.Time
}

func (m *Manager) cachedBlackSharkManifest() (blackSharkFirmwareManifest, bool) {
	cached := m.blackSharkManifest.Load()
	if cached == nil || cached.manifest.empty() {
		return blackSharkFirmwareManifest{}, false
	}
	if time.Since(cached.at) > blackSharkFirmwareCacheTTL {
		return blackSharkFirmwareManifest{}, false
	}
	return cached.manifest, true
}

func (m *Manager) storeBlackSharkManifest(manifest blackSharkFirmwareManifest) {
	m.blackSharkManifest.Store(&blackSharkManifestCache{manifest: manifest, at: time.Now()})
}

func (m *Manager) blackSharkManifestCheckedAt() string {
	cached := m.blackSharkManifest.Load()
	if cached == nil || cached.at.IsZero() {
		return ""
	}
	return cached.at.UTC().Format(time.RFC3339)
}

// ---- 屏幕显示位置与读图 ----

// SetBlackSharkLcdShowPos 设置 LCD 显示哪几项参数、按什么顺序（CMD 0xC2）。
// 载荷 [0x00][pos][条数][条数 × (id u8 + 数值 u16le)]，顺序即屏幕左右顺序。
func (m *Manager) SetBlackSharkLcdShowPos(pos int, entries []deviceproto.BlackSharkSystemInfoEntry) bool {
	payload, ok := deviceproto.BuildBlackSharkLcdShowPosPayload(uint8(pos), entries)
	if !ok {
		m.logError("黑鲨 LCD 显示参数被拒: pos=%d 条目数=%d（pos 只标定到 %d；"+
			"条目 1..%d 项）—— 0xC2 是 4 字节/条，pos>0 的几何还没有样本，不发猜的帧",
			pos, len(entries), deviceproto.BlackSharkLcdCalibratedPosMax,
			deviceproto.BlackSharkLcdShowPosMaxEntries)
		return false
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.blackSharkWriteHandleLocked() == nil {
		return false
	}
	if err := m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdSetLcdShowPos, payload...); err != nil {
		m.logError("黑鲨 LCD 显示参数下发失败: %v", err)
		return false
	}
	// 等设备的 ACK：0xC2 没有值读回，ACK 是唯一确认手段。
	if _, ok := m.blackSharkWaitResponseLocked(deviceproto.BlackSharkCmdSetLcdShowPos, blackSharkQueryTimeout); !ok {
		m.logError("黑鲨 LCD 显示参数：已下发但未收到设备应答（0xC2 无值读回，ACK 是唯一确认手段），"+
			"无法确认设备已收下。pos=%d 条目=%d", pos, len(entries))
		return false
	}
	m.logDebug("黑鲨 LCD 显示参数已下发且设备已应答（ACK 不含参数值，只证明收下、不证明屏上已按此显示）: pos=%d 条目=%d",
		pos, len(entries))
	return true
}

// ReadScreenImageInfo 读回设备当前保存的屏保图像信息（CMD 0xC5）。
// 设备把主机写入的 11 字节原样存下、原样返回，所以比对这 11 字节即可。
func (m *Manager) ReadScreenImageInfo() (screenimg.Info, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	frame, ok := m.blackSharkQueryLocked(deviceproto.BlackSharkCmdImageCommit, nil)
	if !ok {
		return screenimg.Info{}, false
	}
	info, err := screenimg.ParseInfo(frame.Payload)
	if err != nil {
		m.logWarn("屏保图像信息解析失败: %v", err)
		return screenimg.Info{}, false
	}
	return info, true
}

// StartBlackSharkScreenImageRead 请求设备开始送当前屏图的第一帧（CMD 0xC7，空载荷）。
func (m *Manager) StartBlackSharkScreenImageRead() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.blackSharkWriteHandleLocked() == nil {
		return false
	}
	// 读图也要用 0xA4 那条通道 ⇒ 同样按需创建（见 ensureBlackSharkRXLocked）。
	m.ensureBlackSharkRXLocked()
	if err := m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdGetScreensaverImageData); err != nil {
		m.logError("黑鲨读图请求（0xC7）发送失败: %v", err)
		return false
	}
	m.logDebug("黑鲨读图请求已发出（0xC7）：设备随即送第 1 条 0xA4 数据帧")
	return true
}

// PullBlackSharkScreenFrame 取下一条 0xA4 数据帧（CMD 0xC8）。
// 一条 0xC8 对应一条 0xA4，1:1。
func (m *Manager) PullBlackSharkScreenFrame(timeout time.Duration) (deviceproto.BlackSharkImageFrame, bool) {
	ch := m.link.blackSharkImgFrames
	if ch == nil {
		return deviceproto.BlackSharkImageFrame{}, false
	}
	m.mutex.Lock()
	ready := m.blackSharkWriteHandleLocked() != nil
	var err error
	if ready {
		err = m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdPullNextFrame, 0x00)
	}
	m.mutex.Unlock()
	if !ready || err != nil {
		return deviceproto.BlackSharkImageFrame{}, false
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case img := <-ch:
		return img, true
	case <-timer.C:
		return deviceproto.BlackSharkImageFrame{}, false
	}
}

// drainBlackSharkImageFrames 非阻塞地丢掉 `blackSharkImgFrames` 里积压的 0xA4 帧（清空为止）。
// 与 deviceproto.DrainBlackSharkFrames 对应，只是那条管的是 0xA5 控制帧。
func drainBlackSharkImageFrames(ch chan deviceproto.BlackSharkImageFrame) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// ReadBlackSharkScreenImage 从设备读回当前屏上的那张图（0xC7 起 + 反复 0xC8）。
func (m *Manager) ReadBlackSharkScreenImage() ([]byte, bool) {
	if !m.StartBlackSharkScreenImageRead() {
		return nil, false
	}
	ch := m.link.blackSharkImgFrames
	if ch == nil {
		return nil, false
	}
	// 通道不随断开清空（见 ensureBlackSharkRXLocked）⇒ 开始读之前先排空，
	// 免得把上一次连接残留的 0xA4 帧当成这次的第一帧。
	drainBlackSharkImageFrames(ch)
	out := make([]byte, 0, deviceproto.BlackSharkScreenImageBytes)
	// 第 1 帧由 0xC7 直接推来，不需要先发 0xC8。
	first := true
	// prevRemaining = 上一帧的"剩余帧数"，用于校验序号连续性（-1 = 还没收到过）。
	prevRemaining := -1
	for i := 0; i < deviceproto.BlackSharkScreenImageFrames+8; i++ {
		var img deviceproto.BlackSharkImageFrame
		var ok bool
		if first {
			timer := time.NewTimer(blackSharkQueryTimeout)
			select {
			case img, ok = <-ch:
			case <-timer.C:
			}
			timer.Stop()
			first = false
		} else {
			img, ok = m.PullBlackSharkScreenFrame(blackSharkQueryTimeout)
		}
		if !ok {
			m.logError("读屏图中断：第 %d 帧没有到达（已收 %d 字节）", i+1, len(out))
			return nil, false
		}
		out = append(out, img.Payload...)
		// 计数器低 15 位 = 剩余帧数；1 = 最后一帧（收尾帧 LEN=48）。
		remain := int(img.Sequence & 0x7FFF)
		if prevRemaining >= 0 && remain != prevRemaining-1 {
			// 序号不连续 = 中间丢帧或重帧（对应官方那句「包序号不匹配, cmd, expect, recv」）。
			// 先只上报不拦截：等证据攒够再决定是否直接判失败重读。
			m.logWarn("屏保读图序号不连续：期望剩余 %d 帧，收到 %d（第 %d 帧，已收 %d 字节）",
				prevRemaining-1, remain, i+1, len(out))
		}
		prevRemaining = remain
		if remain <= 1 {
			break
		}
	}
	if len(out) < deviceproto.BlackSharkScreenImageBytes {
		m.logError("读屏图不完整：只拿到 %d 字节（应为 %d）",
			len(out), deviceproto.BlackSharkScreenImageBytes)
		return nil, false
	}
	m.logInfo("已从设备读回当前屏图：%d 字节（%d 帧）",
		len(out), (len(out)+deviceproto.BlackSharkImageFramePayload-1)/deviceproto.BlackSharkImageFramePayload)
	return out[:deviceproto.BlackSharkScreenImageBytes], true
}
