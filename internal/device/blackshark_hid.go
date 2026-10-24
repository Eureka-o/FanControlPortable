// 黑鲨 BRB02 的设备写入：冷却与开关、系统信息推送、灯效、配置快照与固件、屏幕与读图。
package device

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
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

	"github.com/Eureka-o/FanControlPortable/internal/deviceprofileexec"
	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/screenimg"
	"github.com/Eureka-o/FanControlPortable/internal/types"

	"tinygo.org/x/bluetooth"
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
	// 写 `0x24` 这个动作本身就是切档，所以只写目标档，绝不逐档写。
	// 参照源按 current 当前值写两份：设备按 (gear, form, src) 存配置，只写一个 src 会让
	// 另一个 src 缺这份曲线（换源后读不到）；次序由 BlackSharkCoolingSourceWriteOrder 定，
	// 当前源放最后一次写，写完设备的生效参照源不变。
	for _, source := range deviceproto.BlackSharkCoolingSourceWriteOrder(current.Source) {
		payload, err := deviceproto.EncodeBlackSharkCurvePayload(gear, points, source)
		if err != nil {
			m.logError("编码黑鲨变频曲线失败: %v", err)
			return false
		}
		if err := m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdSetSpeed, payload...); err != nil {
			m.logError("下发黑鲨变频曲线失败: %v", err)
			return false
		}
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
//
// **写 `0x24` 这个动作本身就是切档**：gear 选档位、form 选形态，设备没有独立的模式开关命令。
// 因此这里只写目标档 —— 逐档写一圈会把设备停在最后写的那个槽上。
func (m *Manager) SetBlackSharkCoolingCurve(gear byte, points [deviceproto.BlackSharkCurvePointCount]deviceproto.BlackSharkCurvePoint) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.blackSharkCanWriteLocked() {
		return false
	}
	if gear < 1 || gear > deviceproto.BlackSharkGearCount {
		m.logError("黑鲨档位越界: %d（有效 1..%d）", gear, deviceproto.BlackSharkGearCount)
		return false
	}
	// 载荷 byte[0] 是参照源（见 currentConfigSourceLocked 的说明），与档位一起决定写设备的哪一份配置；
	// 取不到现值就不写 —— 宁可失败，也不猜一个值发出去。
	current, ok := m.blackSharkReadCoolingConfigLocked()
	if !ok {
		m.logError("黑鲨：0x25 读不回当前配置，放弃本次写入（0x24 首字节取不到现值，" +
			"猜一个会改掉一个语义未定的字段）；请检查设备连接")
		return false
	}
	return m.setBlackSharkCurveLocked(gear, points, current)
}

// ResetBlackSharkSpeedWriteState 作废"上次发出去的目标转速"记忆，让下一次下发真发一帧。
//
// 进入主机侧调速、还原快照、重置曲线等会改动设备曲线/固定值的场景都要调用：
// 这些场景后设备里生效的配置可能不再是上次发的值，"目标没变就跳过"不再成立。
// 同时作废节奏化写入状态：下一次固定值写入重新走完整 apply 仪式（官方行为——
// 仪式是进入受控态的一次性动作，受控态中断后须重来）。
func (m *Manager) ResetBlackSharkSpeedWriteState() {
	if m == nil {
		return
	}
	m.blackSharkLastPushedRPM.Store(0)
	m.blackSharkFixedRitualDone.Store(false)
	m.blackSharkLastFixedWriteNano.Store(0)
}

// BlackSharkLastPushedTargetRPM 返回最近一次真发进设备的固定转速目标（0 = 未发过/已作废）。
// 原子读，持锁与不持锁上下文都可调用。监控循环的去抖用它与本拍目标作差：
// 比较对象必须是设备实际收到的值，核心层"以为发出去"的值在写入被设备层闸门
// 跳过时并不成立。
func (m *Manager) BlackSharkLastPushedTargetRPM() int {
	if m == nil {
		return 0
	}
	return int(m.blackSharkLastPushedRPM.Load())
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
	if !m.blackSharkLinkReadyLocked() {
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

// SetBlackSharkFixedSpeedForGear 把某档位设成固定转速（0x24 的 form=0 形态）。
//
// 同 SetBlackSharkCoolingCurve：写 `0x24` 即切档，所以只写目标档（gear 由调用方给对）。
func (m *Manager) SetBlackSharkFixedSpeedForGear(gear int, rpm int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.blackSharkCanWriteLocked() {
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

// writeBlackSharkFixedFramesLocked 发出固定转速写入帧序列（帧序由 deviceproto 统一给出，
// apply 仪式为 0x24[FF 暂存] → 0x08 00 → 0x24[真实 src+值]，仪式帧序不得拆开或重排；
// 受控态内的提交序列只有单帧）。
func (m *Manager) writeBlackSharkFixedFramesLocked(seq []deviceproto.BlackSharkSpeedFrame) error {
	if len(seq) > 0 {
		m.logInfo("黑鲨固定转速帧序列: %d 帧 首帧 cmd=0x%02X 载荷=% X", len(seq), seq[0].Cmd, seq[0].Payload)
	}
	for _, f := range seq {
		if err := m.blackSharkWriteFrameLocked(f.Cmd, f.Payload...); err != nil {
			return err
		}
	}
	return nil
}

// blackSharkFixedSpeedFramesLocked 按节奏化规则选固定转速的帧序列：本段受控态还没发过
// apply 仪式就发完整仪式，否则只发提交帧。官方 p3 抓包整个会话只出现一次仪式（用户点保存），
// 持续调速只发提交帧；把一次性仪式当周期动作重放是实测扰动源。
func (m *Manager) blackSharkFixedSpeedFramesLocked(gear byte, setpoint uint16, source byte) ([]deviceproto.BlackSharkSpeedFrame, error) {
	if m.blackSharkFixedRitualDone.Load() {
		commit, err := deviceproto.BuildBlackSharkFixedSpeedCommitFrame(gear, setpoint, source)
		if err != nil {
			return nil, err
		}
		return []deviceproto.BlackSharkSpeedFrame{commit}, nil
	}
	return deviceproto.BuildBlackSharkFixedSpeedFrames(gear, setpoint, source)
}

// markBlackSharkFixedWriteLocked 记录一次实际写出：受控态自此成立（后续写走提交帧），
// 同时刷新最小写入间隔的计时起点。
func (m *Manager) markBlackSharkFixedWriteLocked() {
	m.blackSharkFixedRitualDone.Store(true)
	m.blackSharkLastFixedWriteNano.Store(time.Now().UnixNano())
}

// setBlackSharkFixedSpeedLocked 执行固定值写入并回读确认（gear 由调用方给定）。
func (m *Manager) setBlackSharkFixedSpeedLocked(gear byte, setpoint int) bool {
	// 固定值形态：载荷 = [source][form=0][gear][value:u16le]，编码见 deviceproto.EncodeBlackSharkFixedSpeedPayload。
	// byte[0] 是设备侧状态，读回来原样写回（理由同 SetBlackSharkCoolingCurve）；取不到就不写。
	source, ok := m.currentConfigSourceLocked()
	if !ok {
		return false
	}
	// 写 `0x24` 就是切档 ⇒ 只写目标档。
	// 参照源只写当前那一个：录制里固定形态的两次写是**同一个 src**，
	// 只有曲线形态才见到 CPU/GPU 各写一遍（见 setBlackSharkCurveLocked）。
	frames, err := m.blackSharkFixedSpeedFramesLocked(gear, uint16(setpoint), source)
	if err != nil {
		m.logError("编码黑鲨转速失败: %v", err)
		return false
	}
	if err := m.writeBlackSharkFixedFramesLocked(frames); err != nil {
		m.logError("下发黑鲨转速失败: %v", err)
		return false
	}
	m.markBlackSharkFixedWriteLocked()
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
// flag 是状态帧的第 3 字节 = 供电模式位：0x00 = USB 供电，0x01 = 专用电源；写控速不改变该位。
// 只原样带出、不映射成挡位名。
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
	m.applyBlackSharkCurveTargetDisplay(data)
	m.currentFanData.Store(data)
	if m.onFanDataUpdate != nil {
		go m.onFanDataUpdate(data)
	}
}

// storeBlackSharkTargetLocked 在成功下发转速后更新目标值。
// 只写 TargetRPM，绝不写 CurrentRPM。
func (m *Manager) storeBlackSharkTargetLocked(targetRPM int, workMode string) {
	target := types.ClampRPM(targetRPM)
	// 固定转速写入 ⇒ 曲线目标作废：界面"目标"应显示刚下发的这个值（走 TargetRPM）。
	// 工作模式文案也要记进 sticky，否则后续状态帧会用旧文案把它盖回去（形态已经变了）。
	m.clearBlackSharkCurveTargetDisplay()
	m.setBlackSharkDisplayWorkMode(workMode)
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

// SetBlackSharkCurveTargetDisplay 刷新"曲线跟随"形态下的界面读数：目标转速 + 工作模式文案。
//
// 为什么需要单独的显示字段：变频（曲线跟随）下主机不下发转速（曲线已写进设备、主机只推温度），
// 因此 TargetRPM 会一直停在最后一次固定转速写入上，界面"目标"会是个陈旧值。本方法只写
// CurveTargetRPM / WorkMode 这两个**显示用**字段，**不动 TargetRPM** —— 那个字段是下发节流的输入
// （表示"设备当前持有的目标"），改它会静默改变节流行为。
//
// 值与旧值都相同就直接返回，避免空推一次 IPC（本方法会被监控循环按拍调用）。
func (m *Manager) SetBlackSharkCurveTargetDisplay(rpm int, workMode string) {
	if m == nil {
		return
	}
	// 先记进 Manager 自己的原子字段：状态帧会按帧重建 FanData，只写进那一份会被下一帧清掉。
	if rpm > 0 && rpm <= 65535 {
		m.blackSharkCurveTargetRPM.Store(uint32(rpm))
	}
	m.setBlackSharkDisplayWorkMode(workMode)
	prev := m.currentFanData.Load()
	if prev == nil {
		// 还没收到过任何设备回报：不凭空造一份 FanData（CurrentRPM 必须来自设备回报）。
		// 刚记的值不会丢 —— 下一次落库时由 applyBlackSharkCurveTargetDisplay 补进去。
		return
	}
	next := *prev
	m.applyBlackSharkCurveTargetDisplay(&next)
	if next.CurveTargetRPM == prev.CurveTargetRPM && next.WorkMode == prev.WorkMode {
		return
	}
	m.currentFanData.Store(&next)
	if m.onFanDataUpdate != nil {
		m.onFanDataUpdate(&next)
	}
}

// setBlackSharkDisplayWorkMode 记下"当前形态"的工作模式文案（sticky，供状态帧沿用）。
func (m *Manager) setBlackSharkDisplayWorkMode(workMode string) {
	if m == nil {
		return
	}
	if trimmed := strings.TrimSpace(workMode); trimmed != "" {
		wm := trimmed
		m.blackSharkCurveWorkMode.Store(&wm)
	}
}

// applyBlackSharkCurveTargetDisplay 把上面那两个**主机侧**显示字段补进一份新建的 FanData。
//
// 所有"设备状态 → 界面"的落库点都要调它（状态帧、BLE 通知、连接初始读）：那些载荷里根本没有这两个
// 字段，而 FanData 是每帧重建的 ⇒ 不补就会被清零，界面读数在"曲线目标"与旧值之间反复跳。
// 调用方必须持有 m.mutex（这里无锁读 activeProfile）；非黑鲨档案直接返回，
// 否则切换设备后残留的 sticky 值会污染其它设备的显示字段。
func (m *Manager) applyBlackSharkCurveTargetDisplay(data *types.FanData) {
	if m == nil || data == nil {
		return
	}
	if !isBlackSharkProfileID(m.activeProfile.ID) {
		return
	}
	if rpm := m.blackSharkCurveTargetRPM.Load(); rpm > 0 {
		data.CurveTargetRPM = uint16(rpm)
	}
	if wm := m.blackSharkCurveWorkMode.Load(); wm != nil {
		if trimmed := strings.TrimSpace(*wm); trimmed != "" {
			data.WorkMode = trimmed
		}
	}
}

// clearBlackSharkCurveTargetDisplay 作废曲线目标显示值（切到固定转速形态时用）。
//
// 固定形态下"目标"应当是刚下发的那个固定值（走 TargetRPM）；不清理的话界面会优先显示上一次曲线跟随
// 留下的旧值。工作模式文案不在这里清：调用方紧接着会写"固定转速"。
func (m *Manager) clearBlackSharkCurveTargetDisplay() {
	if m == nil {
		return
	}
	m.blackSharkCurveTargetRPM.Store(0)
}

// ResetBlackSharkCurveDisplay 把"曲线跟随"形态的两项主机侧显示状态一起作废（目标值 + 工作模式文案）。
//
// 只在「变频实现方切换」时调用：从设备侧跟随切到主机侧调速后，主机不再调
// SetBlackSharkCurveTargetDisplay，但那两个 sticky 值仍会被状态帧沿用 ⇒ 界面继续显示一个
// 已经不再生效的"曲线跟随"读数。本方法与 clearBlackSharkCurveTargetDisplay 的分工是
// 前者连文案一起清（切换实现方），后者只清目标值（切固定转速，文案由调用方紧接着写）。
func (m *Manager) ResetBlackSharkCurveDisplay() {
	if m == nil {
		return
	}
	m.blackSharkCurveTargetRPM.Store(0)
	m.blackSharkCurveWorkMode.Store(nil)
}

// setBlackSharkSwitchLocked 下发单字节开关并确认生效。
//
// 写 ACK 只证明设备已受理（写 0x10 就回 0x10），读回才是设备当前真实状态。读回晚于 ACK 到达，
// 在 BLE 的 60 ms/条节拍下容易超出单次窗口 ⇒ ACK 超时不直接判失败，改由读回裁决；
// 读回也拿不到才保持失败语义。读回不下发任何写入，不是重试。
func (m *Manager) setBlackSharkSwitchLocked(setCmd, getCmd byte, enabled bool, label string) bool {
	// m.mutex 只覆盖"校验 + 发出"与"读回取数"，两处等待都在锁外（拆分见 blackSharkExchange）；
	// 总线的排他由 m.link.wire 负责，与 m.mutex 无关。
	m.mutex.Lock()
	canWrite := m.blackSharkCanWriteLocked()
	var ex *blackSharkExchange
	if canWrite {
		// 写 + 等**写 ACK**：把 awaitCmd 挂成 setCmd（而不是 getCmd），
		// 这样设备回的写 ACK 会被读协程投进响应通道，而不是被当"没人等"丢掉。
		ex, _ = m.blackSharkBeginExchangeLocked(
			setCmd, deviceproto.BlackSharkSwitchPayload(enabled), blackSharkQueryTimeout, true)
	}
	m.mutex.Unlock()
	if !canWrite {
		return false
	}
	acked := false
	if ex != nil {
		// ← 等的这 2 s **不持 m.mutex**（Finish 也不再取锁）
		_, acked = m.blackSharkFinishExchangeUnlocked(ex)
	}

	// 读回是权威判据：ACK 超时时设备往往已经受理，直接判失败会让界面把开关拨回去（用户再点一遍）。
	// 读回走 blackSharkReadSwitchPublic，等待同样在锁外。
	state, ok := m.blackSharkReadSwitchPublic(getCmd)

	if !acked {
		// 不再补"读回重查"：那一步在整场日志里从未命中（答案是成批到达的，晚于窗口数秒），
		// 只是给已经落后的设备再压一条请求。只用一次读回裁决。
		verdict := decideBlackSharkSwitchVerification(enabled, state, ok)
		switch verdict {
		case blackSharkSwitchConfirmed:
			// 读回说设备已是目标状态 ⇒ 写入确实生效，只是 ACK 迟到。按成功处理。
			m.logWarn("黑鲨 %s 写 ACK 超时（cmd=0x%02X 等满 %v），但读回 cmd=0x%02X 已确认生效；按成功处理",
				label, setCmd, blackSharkQueryTimeout, getCmd)
			return true
		case blackSharkSwitchMismatch:
			// 读回拿到了、且与目标不符 ⇒ 写入确实没生效（真失败，照实报）。
			m.logError("黑鲨 %s 写 ACK 超时且回读不一致: 期望 %v, 实际 0x%02X（写入未生效）",
				label, enabled, state)
			return false
		}
		// 既没 ACK、读回也没拿到 ⇒ 无法确认，保持失败语义。
		m.logError("黑鲨 %s 写入未获 ACK（cmd=0x%02X 等满 %v）且读回未应答，无法确认设备已受理",
			label, setCmd, blackSharkQueryTimeout)
		return false
	}

	// 写 ACK 已到 ⇒ 设备已受理这条写入。读回只作数值校验。
	verdict := decideBlackSharkSwitchVerification(enabled, state, ok)
	if verdict == blackSharkSwitchUnconfirmedByReadback {
		// 读回拿不到 ≠ 写入失败：写 ACK 已证明设备受理。只记 warn，仍判成功。
		m.logWarn("黑鲨 %s 已获写 ACK（cmd=0x%02X），但读回 cmd=0x%02X 未在 %v 内应答；"+
			"按已受理处理（读回仅作数值校验）", label, setCmd, getCmd, blackSharkQueryTimeout)
		return true
	}
	if verdict == blackSharkSwitchMismatch {
		// 读回拿到了、但和设备受理的值不一致 ⇒ 这才是真异常（写入被接受但未生效）。
		m.logError("黑鲨 %s 回读不一致: 期望 %v, 实际 0x%02X（写 ACK 已到，读回值不符）",
			label, enabled, state)
		return false
	}
	return true
}

// blackSharkSwitchVerdict 是"写 ACK 已到之后，读回给什么结论"的三值枚举。
type blackSharkSwitchVerdict int

const (
	// blackSharkSwitchConfirmed：读回拿到了且与期望一致。
	blackSharkSwitchConfirmed blackSharkSwitchVerdict = iota
	// blackSharkSwitchUnconfirmedByReadback：读回超时/无应答 —— **不算失败**（写 ACK 已到）。
	blackSharkSwitchUnconfirmedByReadback
	// blackSharkSwitchMismatch：读回拿到了但与期望不符 —— 真异常。
	blackSharkSwitchMismatch
)

// decideBlackSharkSwitchVerification 是"写 ACK 已到之后"的纯判定，不碰任何设备/锁。
//
// 抽成纯函数是为了能在不接真机的前提下守住"点两遍才生效"的核心判据（完整路径要真实后端 + BLE）。
func decideBlackSharkSwitchVerification(enabled bool, readbackState uint8, readbackOK bool) blackSharkSwitchVerdict {
	if !readbackOK {
		return blackSharkSwitchUnconfirmedByReadback
	}
	if deviceproto.BlackSharkSwitchEnabled(readbackState) != enabled {
		return blackSharkSwitchMismatch
	}
	return blackSharkSwitchConfirmed
}

// blackSharkWriteAndAwaitAckLocked 写一帧并等同 cmd 写 ACK，窗口取 blackSharkWriteReadTimeout。
//
// 不校验载荷（"回没回"才是受理判据，值由读回负责）。刻意不复用迟到暂存：协议帧不带序号，而判别子
// 只对 0x14 求 ⇒ 0x12 的暂存键与载荷无关，复用陈旧 ACK 会静默丢写。窗口不可退回例行档：
// 0x12 的 ACK 常晚于 2 s 才到，退回会把"设备其实已生效"报成失败。调用方需持有 m.mutex。
func (m *Manager) blackSharkWriteAndAwaitAckLocked(cmd byte, payload ...byte) bool {
	ex, ok := m.blackSharkBeginExchangeLocked(cmd, payload, blackSharkWriteReadTimeout, true)
	if !ok {
		return false
	}
	// 等待在锁外，理由见 blackSharkQueryWithTimeoutLocked。
	m.mutex.Unlock()
	_, ok = m.blackSharkFinishExchangeUnlocked(ex)
	m.mutex.Lock()
	return ok
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

// BlackSharkLcdScreenEnabled 读一次 LCD 屏开关（0xC1），只发这一条查询。
//
// 与 BlackSharkSwitchStates 分开：那个一次要读三族开关，而"接管前先记下屏保是不是开着的"
// 只关心这一位，多读的两族纯属白付延迟（BLE 上每条可到秒级）。
func (m *Manager) BlackSharkLcdScreenEnabled() (bool, bool) {
	if m == nil || !m.usesBlackSharkTransport() {
		return false, false
	}
	state, ok := m.blackSharkReadSwitchPublic(deviceproto.BlackSharkCmdGetLcdScreenSwitch)
	if !ok {
		return false, false
	}
	return deviceproto.BlackSharkSwitchEnabled(state), true
}

// SetBlackSharkCoolingSource 设置温控参照（CMD 0x22 / 读回 0x23）。
// source 经 BlackSharkNormalizeCoolingSource 归一化为 CPU / GPU 两种参照。
//
// 0x23 现在只用于写 0x22 之后的一次性回读校验。不复用 0x25[0]：两者的读回语义是否逐值等价尚未
// 核实，换掉只会引入"写成功却报失败"。
func (m *Manager) SetBlackSharkCoolingSource(source uint8) bool {
	if !m.usesBlackSharkTransport() {
		return false
	}
	src := deviceproto.BlackSharkNormalizeCoolingSource(source)
	// 不缓存参照值：写 0x24 时 byte[0] 读 0x25 现取（见 currentConfigSourceLocked）。
	// 界面要显示的参照源来自 `BlackSharkInfo()`（它从档位配置回复的 byte[0] 取，不再单独读）。
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
	if !m.blackSharkCanWriteLocked() {
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
//
// 不再读 0x23（冷却参照源改由配置回复 byte[0] 填），因此这里只有 3 条设备查询；
// CoolingSourceKnown 由调用方补。
func (m *Manager) BlackSharkSwitchStates() types.BlackSharkSwitchStates {
	states := types.BlackSharkSwitchStates{}

	// 各条查询逐条取放 m.mutex，不整段持锁：整段持锁会把监控 tick 饿死，叠加别的读越过 ~20 s
	// 唤醒阈值 ⇒ 误判系统睡眠 ⇒ 拆连接。
	m.mutex.Lock()
	ready := m.blackSharkLinkReadyLocked()
	m.mutex.Unlock()
	if !ready {
		return states
	}
	states.Available = true

	if state, ok := m.blackSharkReadSwitchPublic(deviceproto.BlackSharkCmdRGBStatus); ok {
		states.LightingEnabled = deviceproto.BlackSharkSwitchEnabled(state)
		states.LightingKnown = true
	}
	if state, ok := m.blackSharkReadSwitchPublic(deviceproto.BlackSharkCmdGetLcdScreenSwitch); ok {
		states.LcdScreenEnabled = deviceproto.BlackSharkSwitchEnabled(state)
		states.LcdScreenKnown = true
	}
	// 冷却参照源不再单独发 0x23：它就在配置回复的第 0 字节里（0x25 / 0x26 同形状），由调用方经
	// applyBlackSharkCoolingSource 顺手填，本函数只负责不读它。
	//
	// 0x03 提供读回：优先填设备回报状态；读不到才退回最近下发值，并让 Known 保持 false。
	m.mutex.Lock()
	vec, vecOK := m.blackSharkReadOnOffVectorLocked()
	last := m.blackSharkOnOffCmd.Load()
	m.mutex.Unlock()
	if vecOK {
		states.SmartStartStop = vec.SmartStartStop
		states.PowerOnSelfStart = vec.PowerOnSelfStart
		states.OnOffVectorKnown = true
	} else if last != nil {
		states.LastCommandedSmartStartStop = last.SmartStartStop
		states.LastCommandedPowerOnSelfStart = last.PowerOnSelfStart
	}
	// 该固件提供 0x03 读回（闭环验证过），是设备能力而非常量。
	states.OnOffVectorReadbackSupported = true
	return states
}

// blackSharkReadSwitchPublic 读一次单字节开关状态，调用方不必持锁
// （锁只覆盖准备与发出，等待在锁外，见 blackSharkQueryPublic）。
func (m *Manager) blackSharkReadSwitchPublic(getCmd byte) (uint8, bool) {
	// 调用方本就不持锁，整段持锁会白挡面板读 / 0x07 推送 / 健康监控。
	frame, ok := m.blackSharkQueryPublic(getCmd, nil, blackSharkQueryTimeout)
	if !ok {
		return 0, false
	}
	return deviceproto.BlackSharkSingleByteState(frame, getCmd)
}

// BlackSharkLightingSwitch 只读「灯效开关」（CMD 0x11），不碰其余开关。
//
// 灯效页只用到 0x11，调 BlackSharkSwitchStates 会白读 0xC1 / 0x03 两条（开关族正是超时最集中的
// 一簇）。返回 known=false 表示设备没回，界面应显示"未知"而不是猜。
func (m *Manager) BlackSharkLightingSwitch() (bool, bool) {
	m.mutex.Lock()
	ready := m.blackSharkLinkReadyLocked()
	m.mutex.Unlock()
	if !ready {
		return false, false
	}
	state, ok := m.blackSharkReadSwitchPublic(deviceproto.BlackSharkCmdRGBStatus)
	if !ok {
		return false, false
	}
	return deviceproto.BlackSharkSwitchEnabled(state), true
}

// applyBlackSharkCoolingSource 用配置回复的第 0 字节填冷却参照源。
//
// 0x25 / 0x26 的应答同形状（都从 payload[0] 取 Source），而调用方本来就已读回其中一条。
// 幂等：已填过就不再覆盖。
func applyBlackSharkCoolingSource(states *types.BlackSharkSwitchStates, cfg deviceproto.BlackSharkGearConfig) bool {
	if states == nil || states.CoolingSourceKnown {
		return false
	}
	states.CoolingSource = deviceproto.BlackSharkNormalizeCoolingSource(cfg.Source)
	states.CoolingSourceKnown = true
	return true
}

// ---- 系统信息推送（0x07） ----

// ── 黑鲨 BRB02 的链路与传输 ─────────────────────────────────────────────────
// 链路状态、收帧派发、异步下发 worker、写闸口与运行时判据。

// 黑鲨链路的时间与限额常量；本文件是其唯一 owner。
const (
	// 加长窗口而不是加重试：设备出站排空速率固定，主机超时后立刻发下一条会让积压单调增长，
	// 之后每条都必然超时。窗口只在设备真的晚回时才被用满。
	blackSharkQueryTimeout = 2000 * time.Millisecond
	blackSharkWriteTimeout = 800 * time.Millisecond
	// blackSharkWriteReadTimeout 是用户触发的写路径里自读/回读的窗口，取 3000 ms。
	// 例行轮询错过只是晚一拍，写路径错过却是用户可见的失败（写被拒 ⇒ 界面按设备真值回滚），
	// 而设备侧的停顿观测到最长 2.5 s。单个窗口只发一帧，比重试少一半帧。
	blackSharkWriteReadTimeout = 3000 * time.Millisecond
	recentCmdWindow            = 1500 * time.Millisecond

	// blackSharkSpeedWriteDeadbandRPM 是主机侧调速重发的最小偏差：目标与上次实际发下的值相差
	// 小于它就不发帧。0x24 对设备是档位/形态级的重下发，高频重写会把设备推到其它档的曲线上，
	// 而上游 MinRPMChange 的节流挡不住温度噪声的逐拍小偏差。取 100：设备判稳窗
	// |实测−目标| < 101，死区内的偏差落在设备自身的稳定容差里。
	blackSharkSpeedWriteDeadbandRPM = 100

	// blackSharkFixedWriteMinInterval 是固定转速两次实际写出的最小间隔。设备收到目标后按
	// 200ms/步渐变逼近（换算器 0x1e170c4 在 200ms 循环里同样受每次限升一级约束），间隔内的
	// 重发只会打断进行中的渐变、把目标侧摆动直接灌进设备。取 5s：足够 10 级以内的渐变走完，
	// 也远大于控制拍的采样周期，不影响目标真正变化的响应。
	blackSharkFixedWriteMinInterval = 5 * time.Second
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

// ---- 通道判据：黑鲨有 USB 与 BLE 两条通路 ----
//
// 两条通路都实现之后，判据必须带上传输维度，否则 BLE 已连而功能仍返回 false。四者刻意不合并：
// blackSharkHandleLocked（USB 句柄）/ blackSharkBLESenderLocked（BLE 执行器 + 黑鲨族档案）/
// blackSharkLinkReadyLocked（读：任一通路）/ blackSharkCanWriteLocked（写：非悬挂期 + 就绪）。

// blackSharkBLESenderLocked 返回当前可用的黑鲨 BLE 发送器；不是黑鲨 BLE 或没连上则 nil。
func (m *Manager) blackSharkBLESenderLocked() *deviceprofileexec.BLEExecutor {
	if m.bleExecutor == nil || !m.bleExecutor.IsConnected() {
		return nil
	}
	if !types.IsBlackSharkDeviceProfileID(m.activeProfile.ID) {
		return nil
	}
	if m.deviceTransport != types.DeviceTransportBLE && m.deviceType != types.DeviceTransportBLE {
		return nil
	}
	return m.bleExecutor
}

// blackSharkLinkReadyLocked 报告当前能不能和黑鲨换帧（读或写）——两条通路任一可用即可。
func (m *Manager) blackSharkLinkReadyLocked() bool {
	if m.blackSharkHandleLocked() != nil {
		return true
	}
	return m.blackSharkBLESenderLocked() != nil
}

// blackSharkCanWriteLocked = 非悬挂期 + 链路就绪。
func (m *Manager) blackSharkCanWriteLocked() bool {
	if m.writesBlocked.Load() {
		return false
	}
	return m.blackSharkLinkReadyLocked()
}

// BlackSharkBluetoothAvailable 报告本机蓝牙适配器此刻是否可用（纯查询：不扫描、不连接）。
//
// 与官方 `coolerIsBluetoothAvailable` 对应：适配器不存在、被禁用或被别的进程独占时返回 false。
// 它只回答"能不能走蓝牙"，不回答"设备在不在"。
func (m *Manager) BlackSharkBluetoothAvailable() bool {
	return bluetooth.DefaultAdapter.Enable() == nil
}

// BlackSharkUSBInterfacePresent 报告 BRB02 的 USB 接口此刻能否枚举到（存在性探针，不建会话）。
//
// 用的是与设备扫描同一个枚举探针（打开后立刻关闭，过程中会短暂占用接口），所以
// **调用方不得持有 m.mutex**，也不要在 USB 会话正在进行时调用。
// 非 Windows 构建恒为 false。
func (m *Manager) BlackSharkUSBInterfacePresent() bool {
	if m == nil {
		return false
	}
	path, err := scanBlackSharkUSBDevice()
	return err == nil && strings.TrimSpace(path) != ""
}

// ConnectBlackSharkBLEDirect 按已配对地址直连黑鲨 BLE，跳过名称扫描。
//
// BRB02 配对后空闲时不广播，名称扫描扫不到（同一次扫描里其它广播者都在），只能按地址连；
// 地址取自 Windows 配对表。取不到或连不上返回 false，调用方回落候选表。
func (m *Manager) ConnectBlackSharkBLEDirect(ctx context.Context) (bool, map[string]string) {
	if !m.BlackSharkBluetoothAvailable() {
		m.logWarn("黑鲨 BLE：本机蓝牙适配器不可用，跳过直连")
		return false, nil
	}
	profile := types.BlackSharkBRB02Profile()
	address := strings.TrimSpace(blackSharkPairedBLEAddress(profile.Connection.BLENameFilter))
	if address == "" {
		return false, nil
	}
	// PrepareRuntimeProfile 会用内置档案的规范值覆盖除 Connection 以外的字段，
	// 而 Connection 是明确保留的（端点属于运行期状态）——所以这样把地址传进去是受支持的用法。
	profile.Connection.Endpoint = address
	connected, info := m.ConnectNativeProfileContext(ctx, profile)
	if connected {
		m.logInfo("黑鲨 BLE 已按配对地址直连: %s", address)
	}
	return connected, info
}

// bleAddressFromPairedKey 把配对表里的 12 位十六进制键转成 AA:BB:CC:DD:EE:FF。
// 该键就是正序 MAC（配对表不翻转字节序）。
func bleAddressFromPairedKey(key string) (string, bool) {
	candidate := strings.TrimSpace(key)
	if len(candidate) != 12 {
		return "", false
	}
	if _, err := hex.DecodeString(candidate); err != nil {
		return "", false
	}
	upper := strings.ToUpper(candidate)
	parts := make([]string, 0, 6)
	for i := 0; i < 12; i += 2 {
		parts = append(parts, upper[i:i+2])
	}
	return strings.Join(parts, ":"), true
}

// decodePairedDeviceName 解配对表里的设备名（多为 UTF-8，偶尔 UTF-16LE）。
func decodePairedDeviceName(raw []byte) string {
	if len(raw) >= 2 && raw[1] == 0x00 {
		runes := make([]rune, 0, len(raw)/2)
		for i := 0; i+1 < len(raw); i += 2 {
			value := uint16(raw[i]) | uint16(raw[i+1])<<8
			if value == 0 {
				break
			}
			runes = append(runes, rune(value))
		}
		return strings.TrimSpace(string(runes))
	}
	return strings.TrimSpace(strings.TrimRight(string(raw), "\x00"))
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
	if !m.blackSharkCanWriteLocked() {
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
// blackSharkWire 是"总线上一次交换"的唯一 owner：按票号 FIFO，一次只放一个进去。
//
// 不用 sync.Mutex：它不保证公平（只有进入饥饿模式后才近似 FIFO，在此之前新来的会插队），
// 而官方是一个任务缓冲、天然先来先服务。
type blackSharkWire struct {
	mu     sync.Mutex
	cond   *sync.Cond
	next   uint64 // 下一个该进入的票号
	issued uint64 // 已发出的票号
	held   bool   // 是否占用中
}

func (w *blackSharkWire) acquire() {
	w.mu.Lock()
	if w.cond == nil {
		w.cond = sync.NewCond(&w.mu)
	}
	ticket := w.issued
	w.issued++
	if w.held || ticket != w.next {
		for w.held || ticket != w.next {
			w.cond.Wait()
		}
	}
	w.held = true
	w.next = ticket + 1
	w.mu.Unlock()
}

// release 交还总线；必须在 acquire 之后调用一次。
func (w *blackSharkWire) release() {
	w.mu.Lock()
	w.held = false
	if w.cond != nil {
		w.cond.Broadcast()
	}
	w.mu.Unlock()
}

// tryAcquire 拿不到立刻返回 false（专供 1 Hz 主机信息推送：不排队，下一拍再补）。
// 有票在排队时也算拿不到 —— 否则就绕过 FIFO 插队了。
func (w *blackSharkWire) tryAcquire() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cond == nil {
		w.cond = sync.NewCond(&w.mu)
	}
	if w.held || w.issued != w.next {
		return false
	}
	w.issued++
	w.next++
	w.held = true
	return true
}

type blackSharkLink struct {
	// awaitCmd 是当前正在等待应答的 cmd（0 = 没人等）。读协程只投递匹配的帧，
	// 设备主动上报（0x06/0x07 空应答等）一律不占通道。
	awaitCmd atomic.Uint32
	// awaitMission 是上面那条交换的序号（第几次交换，单调递增）。
	awaitMission atomic.Uint64
	// wire 是"总线上一次交换"的唯一 owner：排空 → 挂 awaitCmd → 写 → 等应答。
	wire blackSharkWire
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
	// bleRXHookFor 记的是"已经把 BLE 通知帧接进本状态机的那个执行器实例"（nil = 还没接）。
	// 记实例而不是布尔量：断线重连会新建执行器对象，布尔量会让新实例永远接不上。
	// BLE 的通知回调与 USB 读协程是两条独立来源，但都投进同一个 blackSharkResp，
	// 因此 blackSharkWaitResponseLocked 不需要区分通路。
	bleRXHookFor atomic.Pointer[deviceprofileexec.BLEExecutor]

	// 0x07 主机信息推送的节拍由 device 侧的循环拥有（与官方同形）。
	// hostInfoStop = 当前循环的停止信号（nil = 没在推）；hostInfoInterval = 当前间隔（0 = 停）；
	// hostInfoProvider 每拍取一次要推的条目（条目由 coreapp 采集，device 只管节拍与推送）。
	// 三者都在 m.mutex 下访问。
	hostInfoStop     chan struct{}
	hostInfoInterval time.Duration
	hostInfoProvider func() []deviceproto.BlackSharkSystemInfoEntry

	// lateResp 是迟到应答暂存队列及其 owner 锁。
	//
	// 交换收尾会把 awaitCmd 清零，此后同 cmd 的应答若不接住就与"设备根本没回"完全同形。
	// 按 (cmd, 判别子) 留短窗口队列（容量见 blackSharkLateRespQueueCap）：积压排空时同一 cmd 的
	// 应答成批到达（3–8 条），单槽只留得住最后一条。下一次同 cmd 查询先取队列，命中就直接用。
	// 单开一把锁：收帧方不持有 m.mutex，复用会引发数据竞争。
	lateResp struct {
		mu     sync.Mutex
		queues map[blackSharkStashKey]*blackSharkLateQueue
	}
	// rgbBlocks 是"某个灯效模式整块参数（8 字节）"的读缓存（idx → 载荷），由 m.mutex 保护。
	//
	// 有它之后同一模式短时间内再读一次不必再发 0x14（真机出现过"某模式刚成功读过、1.1 秒后同一
	// 模式读超时导致切换失败"）。rgbBlocksGen 换代即整体失效，不跨连接复用。
	rgbBlocks    map[int][]byte
	rgbBlocksGen uint64

	// 异步下发：最新目标覆盖待发目标（转速是设定值，不需要排队），由唯一 worker 串行执行。
	pushMu      sync.Mutex
	pushPending *speedPushRequest
	pushRunning bool // pushEpoch 是"作废标记"：每当有重要写入（安全回退、切档/手动挡位）发生时 +1，
	// 让还排着队的例行目标作废 —— 否则例行的旧目标会把安全值/挡位值盖掉。
	pushEpoch uint64
}

// blackSharkStashKey 是迟到应答暂存队列的键：cmd + 请求判别子。
//
// 只按 cmd 分键会把"槽 A 的答案"交给"问槽 B"的查询（带参读的应答不含请求回显）。判别子必须能
// 从应答自身推出来（0x14 的槽位 = payload[0] 低 4 位），推不出来的命令不能进白名单。
// 无参命令恒为 0。
type blackSharkStashKey struct {
	cmd byte
	sub byte
}

// blackSharkStashSubForRequest 求"本次请求"的判别子；无判别子（无参命令）返回 0。
// 必须与 blackSharkStashSubForResponse 的口径**逐值一致**，否则两边对不上就是白暂存。
func blackSharkStashSubForRequest(cmd byte, payload []byte) (byte, bool) {
	if cmd == deviceproto.BlackSharkCmdGetAnyRgbEffects {
		if len(payload) == 0 {
			return 0, false // 0x14 必须带槽位；不带就不知道问的是哪个槽
		}
		return payload[0], true
	}
	return 0, true
}

// blackSharkStashSubForResponse 求"这条应答"的判别子 —— 口径与请求侧一致。
//
// `0x14`：槽位从**应答自身**取（`ModeIndex()` = `payload[0]` 低 4 位）。
// 解析不出槽位时返回 false ⇒ **宁可不暂存也不猜**（猜错就是跨槽误配，比丢掉更糟）。
func blackSharkStashSubForResponse(frame deviceproto.BlackSharkFrame) (byte, bool) {
	if frame.Command == deviceproto.BlackSharkCmdGetAnyRgbEffects {
		eff, ok := deviceproto.DecodeBlackSharkRgbEffects(frame.Payload)
		if !ok {
			return 0, false
		}
		return byte(eff.ModeIndex()), true
	}
	return 0, true
}

// blackSharkLateQueueCap 是每个 (cmd, 判别子) 的迟到应答队列容量。
//
// 取 8：设备排空积压时同一 cmd 的应答成批到达，观测到的最大批是一轮 8 个槽位的 0x14 答案；
// 而单次查询一次只消费一条。
const blackSharkLateQueueCap = 8

// blackSharkLateStash 是暂存里的一条：帧 + 它到达的时刻。
//
// 过期由条目自身判定，所以交换收尾不必清表 —— 清表会让面板连发（0x11 → 0xC1 → 0x03）时上一条
// 的应答刚入队就被下一条的收尾抹掉。
type blackSharkLateStash struct {
	frame deviceproto.BlackSharkFrame
	at    time.Time
}

// blackSharkLateQueue 是**单一 (cmd, 判别子)** 的迟到应答 FIFO（切片，按到达顺序）。
// 超过容量时丢**最旧**的一条：新应答比旧应答更可能对应用户当下的操作。
type blackSharkLateQueue struct {
	frames []blackSharkLateStash
}

// push 追加一条（超过容量则丢最旧的）。
func (q *blackSharkLateQueue) push(frame deviceproto.BlackSharkFrame, at time.Time) {
	if len(q.frames) >= blackSharkLateQueueCap {
		copy(q.frames, q.frames[1:])
		q.frames = q.frames[:len(q.frames)-1]
	}
	q.frames = append(q.frames, blackSharkLateStash{frame: frame, at: at})
}

// pop 先丢掉**自身已过期**的条目（到达时刻早于 now-recentCmdWindow），再取最旧的一条。
// 空则返回 false。过期策略从"全局 until"移到条目自身 ⇒ 与整表清空解耦。
func (q *blackSharkLateQueue) pop(now time.Time) (deviceproto.BlackSharkFrame, bool) {
	cutoff := now.Add(-recentCmdWindow)
	keep := q.frames[:0]
	for _, s := range q.frames {
		if s.at.After(cutoff) {
			keep = append(keep, s)
		}
	}
	q.frames = keep
	if len(q.frames) == 0 {
		return deviceproto.BlackSharkFrame{}, false
	}
	frame := q.frames[0].frame
	copy(q.frames, q.frames[1:])
	q.frames = q.frames[:len(q.frames)-1]
	return frame, true
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
	return m.blackSharkHandleRXLocked(generation, raw, resp)
}

// ensureBlackSharkBLERXHook 把 BLE 通知帧接进黑鲨的收帧状态机（幂等）。
//
// 收帧状态机原本只由 USB 读协程喂，不接 BLE 通知则 BLE 上所有查询只能超时。
// 注册是懒的（第一次 BLE 收发时），不必改上游连接代码。
func (m *Manager) ensureBlackSharkBLERXHook(sender *deviceprofileexec.BLEExecutor) {
	if sender == nil {
		return
	}
	if m.link.bleRXHookFor.Load() == sender {
		return
	}
	sender.SetRawFrameCallback(func(raw []byte) {
		m.blackSharkHandleRXLocked(m.connectionGen.Load(), raw, m.link.blackSharkResp)
	})
	m.link.bleRXHookFor.Store(sender)
}

// blackSharkTransportLabel 返回当前黑鲨传输的日志标签（"BLE" / "USB"）。
//
// 收帧与查询两条通路共用同一份实现，日志必须自带通路才分得清。BLE 分支文案不可改
// （`黑鲨 BLE 应答帧：…` / `黑鲨 BLE 查询超时…`），既有 grep 与历史日志依赖它。
func (m *Manager) blackSharkTransportLabel() string {
	if m.deviceTransport == types.DeviceTransportBLE || m.deviceType == types.DeviceTransportBLE {
		return "BLE"
	}
	return "USB"
}

// blackSharkHandleRXLocked 是黑鲨收帧的**唯一实现**，USB 读协程与 BLE 通知回调共用。
//
// 代次校验由调用方负责。只碰原子量、通道与自带锁的成员，因此持锁与不持锁两种上下文都能调用。
func (m *Manager) blackSharkHandleRXLocked(generation uint64, raw []byte, resp chan deviceproto.BlackSharkFrame) bool {
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
	// 非 0x06（设备 2 Hz 主动上报）的帧都是应答，量少，逐条记。
	// 价值：BLE 上"写得出、读不回"时，"设备根本没回"与"回了但 awaitCmd 对不上被丢"都表现为
	// 查询超时，这条日志是唯一能分开两者的观测点。按实际传输打标，见 blackSharkTransportLabel。
	if frame.Command != deviceproto.BlackSharkCmdStatusNotify {
		payloadHex := ""
		if len(frame.Payload) > 0 && len(frame.Payload) <= 16 {
			payloadHex = " 内容=" + deviceproto.Hex(frame.Payload)
		}
		m.logInfo("黑鲨 %s 应答帧：cmd=0x%02X 载荷=%d 字节%s，当前等待=0x%02X（mission #%d）",
			m.blackSharkTransportLabel(),
			frame.Command, len(frame.Payload), payloadHex, byte(m.link.awaitCmd.Load()), m.link.awaitMission.Load())
	}
	// 不匹配也尝试暂存：设备可能把某条应答推到"下一条命令已占住 awaitCmd"之后才发，旧条件
	// （只投递匹配项、只在 awaitCmd==0 时暂存）会让它两边都进不去而被静默丢弃。
	// 设备主动推送（0x06 / 0x07）必须排除，否则会刷满 8 格容量、挤掉真正的应答。
	if resp != nil && m.link.awaitCmd.Load() == uint32(frame.Command) {
		select {
		case resp <- frame:
		default:
			n := blackSharkDroppedFrames.Add(1)
			if n == 1 || n%64 == 0 {
				m.logWarn("黑鲨 HID 响应通道已满，丢弃一帧（累计 %d 帧，cmd=0x%02X）", n, frame.Command)
			}
		}
	} else if blackSharkIsQueryResponse(frame.Command) && m.stashLateBlackSharkResponse(frame) {
		// 迟到 / 错位应答：按 cmd 存入对应队列，供下一次同 cmd 查询取用。
		m.logInfo("黑鲨迟到应答已暂存：cmd=0x%02X 载荷=%d 字节（距上次超时 < %v，将由下一次**同 cmd 同判别子**的查询取用）",
			frame.Command, len(frame.Payload), recentCmdWindow)
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
	// 只有 `0x06`（设备主动上报的状态）才是状态帧。
	//
	// ★ 0x25 的应答是**冷却配置**（5 B 固定 / 15 B 曲线：`[src][form][gear][value]`），
	// ParseBlackSharkStatus 却把它的 payload[3:5] 当转速读 —— 那正好是**设定值 sp**。
	// 只要读一次 0x25（面板回读、档位查询、主机侧调速写后校验都会读），界面"当前转速"
	// 就会被顶成 sp，与真实 0x06 值交替出现 ⇒ 用户看到"两个数值来回跳、设备本身没变化"。
	if frame.Command != deviceproto.BlackSharkCmdStatusNotify {
		return false
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

// blackSharkIsQueryResponse 报告某个 cmd 是不是"主动发、设备会回同 cmd"的无参数查询应答。
// 用途：收帧时决定这条不匹配帧该不该进迟到暂存队列。
//
// 带参查询只有在判别子能从应答自身推出来时才能进白名单（0x14 可以，0x26 的 gear/form 与请求
// 参数是否逐值等价尚未核实，仍排除）：否则错位的应答会被下一次读另一个槽位的查询取走，
// 静默返回错误数据，比超时更糟。
// 用白名单而不是黑名单：黑名单会随协议扩张漏掉新推送，而漏掉一条推送就会刷满队列。
// 排除在外的：0x06 / 0x07 / 0x16 / 0xC6（图传流控走 imageFlow 专用通道）。
func blackSharkIsQueryResponse(cmd byte) bool {
	switch cmd {
	case deviceproto.BlackSharkCmdVersion,
		deviceproto.BlackSharkCmdGetOnOffVector,
		deviceproto.BlackSharkCmdRGBEnable,
		deviceproto.BlackSharkCmdRGBStatus,
		deviceproto.BlackSharkCmdSetLighting,
		deviceproto.BlackSharkCmdGetLighting,
		// 0x14 的判别子（槽位）能从应答自身可靠推出 ⇒ 跨槽误配不可能发生。
		deviceproto.BlackSharkCmdGetAnyRgbEffects,
		deviceproto.BlackSharkCmdSetFanSwitch,
		deviceproto.BlackSharkCmdSetCoolingSource,
		deviceproto.BlackSharkCmdGetCoolingSource,
		deviceproto.BlackSharkCmdSetSpeed,
		deviceproto.BlackSharkCmdGetStatus,
		deviceproto.BlackSharkCmdSetLcdScreenSwitch,
		deviceproto.BlackSharkCmdGetLcdScreenSwitch,
		deviceproto.BlackSharkCmdSetLcdShowPos,
		deviceproto.BlackSharkCmdImageMetadata,
		deviceproto.BlackSharkCmdImageCommit:
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
	return m.blackSharkBuildReportForHandle(m.blackSharkWriteHandleLocked(), frame)
}

func (m *Manager) blackSharkBuildReportForHandle(handle *flyDigiHIDDevice, frame []byte) ([]byte, error) {
	if handle != nil && handle.usb != nil {
		report := make([]byte, deviceproto.BlackSharkReportLength)
		if len(frame) > len(report) {
			return nil, fmt.Errorf("黑鲨帧长度 %d 超过报文长度 %d", len(frame), len(report))
		}
		copy(report, frame)
		return report, nil
	}
	return deviceproto.BuildBlackSharkReport(frame, deviceproto.BlackSharkReportLength)
}

// blackSharkFrameBackend 表示"把一帧写到设备"。
type blackSharkFrameBackend interface {
	transport() string
	// prepare 是发送前的一次性准备（BLE 侧要把通知帧接进收帧状态机；幂等）。USB 侧为空实现。
	prepare()
	send(cmd byte, frame []byte) error
}

// bleFrameBackend 走 BLE 特征：帧原样写出，没有 USB 那层 65 字节报文封套。
// 一次写即可、不分片：协商后的单帧容量远大于本模块最长的一帧（0x24 曲线 19 字节）。
// 用哪条特征、带不带响应写由执行器决定（控制帧走 WriteRaw、块流走 WriteRawStream）。
type bleFrameBackend struct {
	manager *Manager
	sender  *deviceprofileexec.BLEExecutor
}

func (b bleFrameBackend) transport() string { return types.DeviceTransportBLE }

func (b bleFrameBackend) prepare() {

	// 把通知帧接进黑鲨的收帧状态机（幂等）：BLE 的应答与数据帧都走通知通道，
	// 不接则"发一帧等一条"永远超时。挂在发送路径上，读图（0xC7/0xC8 拉 0xA4 帧）也被覆盖到。
	b.manager.ensureBlackSharkBLERXHook(b.sender)
}

func (b bleFrameBackend) send(cmd byte, frame []byte) error {
	b.manager.recordDebugFrame("tx", types.DeviceTransportBLE, frame)
	err := b.sender.WriteRaw(context.Background(), frame)
	if err != nil {
		b.manager.reportBlackSharkBLEWriteFailure(cmd, err)
		return fmt.Errorf("黑鲨 BLE 命令 0x%02X 发送失败: %w", cmd, err)
	}
	return nil
}

// reportBlackSharkBLEWriteFailure 把"BLE 写失败"上报成链路丢失。
//
// 必须异步：调用点可能正持有 m.mutex（图传整轮都持有），而上报路径要取它。代次校验交给上报路径
// 自己的 expectedGeneration 参数。
func (m *Manager) reportBlackSharkBLEWriteFailure(cmd byte, err error) {
	if m == nil {
		return
	}
	generation := m.connectionGen.Load()
	m.logError("黑鲨 BLE 写失败（cmd=0x%02X）：判定链路已断并触发重连: %v", cmd, err)
	go m.onBLEConnectionLost(generation)
}

// usbFrameBackend 走 HID/libusb：按句柄的通路装封套（见 blackSharkBuildReportForHandle），写报文并重试。
type usbFrameBackend struct {
	manager *Manager
	handle  *flyDigiHIDDevice
}

func (b usbFrameBackend) transport() string { return types.DeviceTransportUSB }

func (b usbFrameBackend) prepare() {}

func (b usbFrameBackend) send(cmd byte, frame []byte) error {
	report, err := b.manager.blackSharkBuildReportForHandle(b.handle, frame)
	if err != nil {
		return err
	}
	return retryDeviceSend(fmt.Sprintf("BlackShark HID command 0x%02X", cmd), func() error {
		b.manager.recordDebugFrame("tx", types.DeviceTransportUSB, report)
		return b.handle.WriteReport(report, blackSharkWriteTimeout)
	})
}

// blackSharkFrameBackendLocked 选当前该用的后端：BLE 优先，否则 USB；两者都不在返回 nil
// （顺带修掉了改造前 USB 分支对着 nil 句柄调方法的问题）。
func (m *Manager) blackSharkFrameBackendLocked() blackSharkFrameBackend {
	if sender := m.blackSharkBLESenderLocked(); sender != nil {
		return bleFrameBackend{manager: m, sender: sender}
	}
	if handle := m.blackSharkWriteHandleLocked(); handle != nil {
		return usbFrameBackend{manager: m, handle: handle}
	}
	return nil
}

// blackSharkWriteFrameLocked 组帧并写出（封装按当前传输选，见 blackSharkBuildReportForHandle），
// 是黑鲨唯一的写入闸口；闸门判据见 blackSharkWriteGuard，本函数只负责组帧与发送。
//
// 取 wire 是为了让写与查询共用同一次交换语义（见 blackSharkWire）；不取的话图传这类长任务占着
// 总线时周期写仍会插进去，总线次序就不再等价于官方的单队列。
func (m *Manager) blackSharkWriteFrameLocked(cmd byte, payload ...byte) error {
	m.link.wire.acquire()
	defer m.link.wire.release()
	return m.blackSharkWriteFrameOnWireLocked(cmd, payload...)
}

// blackSharkWriteFrameOnWireLocked 与 blackSharkWriteFrameLocked 同义，但要求调用方已持有 wireMu，
// 供"写一帧 + 等它的应答"必须原子成对的场合（被别的交换插进来就会丢掉应答）。
func (m *Manager) blackSharkWriteFrameOnWireLocked(cmd byte, payload ...byte) error {
	if err := m.blackSharkWriteGuard(cmd); err != nil {
		return err
	}
	backend := m.blackSharkFrameBackendLocked()
	if backend == nil {
		return fmt.Errorf("黑鲨设备不可写，命令 0x%02X 未发出", cmd)
	}
	backend.prepare()
	return backend.send(cmd, deviceproto.BuildBlackSharkFrame(cmd, payload...))
}

// errBlackSharkWireBusy 表示"总线被别的交换占着，而调用方要求不等"。
var errBlackSharkWireBusy = errors.New("black shark wire is busy")

// blackSharkTryWriteFrameLocked 与 blackSharkWriteFrameLocked 同义，但不肯等总线：
// 拿不到 wire 立刻返回 errBlackSharkWireBusy。
//
// 专供 1 Hz 推送：图传会占着总线十几秒，排进去会让它在持有 m.mutex 的情况下等十几秒，
// 把连接与设置页一起堵住。调用方拿到 errBlackSharkWireBusy 后改走不经 wire 的直发。
func (m *Manager) blackSharkTryWriteFrameLocked(cmd byte, payload ...byte) error {
	if !m.link.wire.tryAcquire() {
		return errBlackSharkWireBusy
	}
	defer m.link.wire.release()
	return m.blackSharkWriteFrameOnWireLocked(cmd, payload...)
}

// blackSharkQueryLocked 发送查询命令并等待匹配 cmd 的响应帧。
// 调用方需持有 m.mutex（与其它写路径一致，避免报文交错）。
func (m *Manager) blackSharkQueryLocked(cmd byte, payload []byte) (deviceproto.BlackSharkFrame, bool) {
	return m.blackSharkQueryWithTimeoutLocked(cmd, payload, blackSharkQueryTimeout)
}

// blackSharkQueryWithTimeoutLocked 与 blackSharkQueryLocked 同义，但可以指定等待窗口。
//
// 与 blackSharkQueryLocked 的差别只有窗口长度：设备侧的停顿与"谁在等"无关，但代价不同 ——
// 例行轮询错过只是晚一拍，用户点了一下就等结果的写路径错过是可见的失败。
// 单窗口而不是多试几次：只发一帧，且应答落在窗口内就直接投递。调用方需持有 m.mutex。
//
// 超时不补等：宽限期从未命中（两天 413 次超时 0 次），迟到的应答靠在下一次同 cmd 查询开始时取暂存。
func (m *Manager) blackSharkQueryWithTimeoutLocked(cmd byte, payload []byte, timeout time.Duration) (deviceproto.BlackSharkFrame, bool) {
	ex, ok := m.blackSharkBeginExchangeLocked(cmd, payload, timeout, false)
	if !ok {
		return deviceproto.BlackSharkFrame{}, false
	}
	// 等待在锁外：1 Hz 的 0x07 推送入口要取同一把锁，占着它等 2~3 s 会把设备唯一的主机输入整段饿掉。
	// 让出锁不会让别的交换插进来 —— 交换的排他由 m.link.wire 负责，m.mutex 只保护状态；
	// Finish 先放 wire 再由这里重新取锁，锁序仍是 m.mutex → m.link.wire。
	m.mutex.Unlock()
	frame, ok := m.blackSharkFinishExchangeUnlocked(ex)
	m.mutex.Lock()
	return frame, ok
}

// blackSharkExchange 是一次"发一帧、等一条同 cmd 应答"的交换，被拆成两半：
//
//	blackSharkBeginExchangeLocked    —— 需要 m.mutex：校验 / 排空 / 查迟到暂存 / 选后端 / 组帧 / 发出
//	blackSharkFinishExchangeUnlocked —— 不需要 m.mutex：等应答 / 记账 / 收尾 / 释放总线
//
// 拆开是为了把等待移出 m.mutex：否则 1 Hz 推送、面板读、状态推送、健康监控都被排队到一次交换结束。
// 锁序仍统一为 m.mutex → m.link.wire（Finish 不持 m.mutex 就释放 wire，不会形成反向边）。
type blackSharkExchange struct {
	resp    chan deviceproto.BlackSharkFrame
	cmd     byte
	timeout time.Duration
	// hit 非 nil = 已在迟到暂存里命中，无需发送与等待（此路径**未持有** wire）。
	hit *deviceproto.BlackSharkFrame
}

// blackSharkBeginExchangeLocked 需要调用方持 m.mutex。
// 返回 (ex, true) 时 wire **已被本函数持有**，必须由 blackSharkFinishExchangeUnlocked 释放。
// writePath=true 走写路径（过写闸门、且**不查**迟到暂存）；false 走查询路径。
func (m *Manager) blackSharkBeginExchangeLocked(cmd byte, payload []byte, timeout time.Duration, writePath bool) (*blackSharkExchange, bool) {
	if writePath {
		// 写闸门（危险命令黑名单 / 悬挂期禁写）必须在**碰总线之前**过。
		if err := m.blackSharkWriteGuard(cmd); err != nil {
			return nil, false
		}
	}
	if !m.blackSharkLinkReadyLocked() {
		return nil, false
	}
	// 接收通道按需创建（见 ensureBlackSharkRXLocked），不依赖上游的连接代码预先建好。
	m.ensureBlackSharkRXLocked()
	// 从排空到"声明这次交换"整段独占：awaitCmd 只有一个槽，并发交换会互相吃掉对方的应答。
	m.link.wire.acquire()
	resp := m.link.blackSharkResp
	deviceproto.DrainBlackSharkFrames(resp)
	// 迟到暂存只认同 cmd；写路径刻意不查 —— 协议帧不带序号，旧 ACK 证明不了它响应的是哪一次写。
	if !writePath {
		if late, ok := m.takeLateBlackSharkResponse(cmd, payload); ok {
			m.logInfo("黑鲨查询 0x%02X 命中迟到应答暂存（跳过重发）", cmd)
			m.link.wire.release() // 本轮没用到总线
			return &blackSharkExchange{hit: &late}, true
		}
	}
	frame := deviceproto.BuildBlackSharkFrame(cmd, payload...)
	// 两条通路收敛到同一个后端选择 + 同一个等待：帧怎么装、往哪条特征上写，都在后端里；
	// 应答则无论走哪条通路都投进同一个 blackSharkResp（见 handleBlackSharkHIDRX / BLE 收帧钩子）。
	backend := m.blackSharkFrameBackendLocked()
	if backend == nil {
		m.link.wire.release()
		return nil, false
	}
	backend.prepare()
	// 先声明这次交换（序号 + 在等哪个 cmd）、再写：读协程只投递匹配 awaitCmd 的帧，
	// 若设备回得比挂 await 还快，应答会被当"没人等"丢掉。
	mission := m.beginBlackSharkExchangeLocked(cmd)
	if err := backend.send(cmd, frame); err != nil {
		if writePath {
			m.logWarn("黑鲨写 ACK 路径下发 0x%02X 失败: %v", cmd, err)
		} else {
			m.logWarn("黑鲨查询 0x%02X 发送失败（mission #%d）: %v", cmd, mission, err)
		}
		m.endBlackSharkExchangeLocked()
		m.link.wire.release()
		return nil, false
	}
	return &blackSharkExchange{resp: resp, cmd: cmd, timeout: timeout}, true
}

// blackSharkFinishExchangeUnlocked **不需要 m.mutex**：等应答、记账、收尾、释放总线。
func (m *Manager) blackSharkFinishExchangeUnlocked(ex *blackSharkExchange) (deviceproto.BlackSharkFrame, bool) {
	if ex == nil {
		return deviceproto.BlackSharkFrame{}, false
	}
	if ex.hit != nil {
		// 命中暂存：Begin 里已经释放过 wire，这里什么都不用做。
		return *ex.hit, true
	}
	res, ok := m.blackSharkWaitResponseLocked(ex.cmd, ex.timeout)
	m.endBlackSharkExchangeLocked()
	m.link.wire.release()
	return res, ok
}

// blackSharkQueryPublic 是 blackSharkQueryLocked 的**自取锁**版本：
// 锁只覆盖"准备 / 发出"，**等待在锁外**。供本就不持锁的公开入口使用。
func (m *Manager) blackSharkQueryPublic(cmd byte, payload []byte, timeout time.Duration) (deviceproto.BlackSharkFrame, bool) {
	m.mutex.Lock()
	ex, ok := m.blackSharkBeginExchangeLocked(cmd, payload, timeout, false)
	m.mutex.Unlock()
	if !ok {
		return deviceproto.BlackSharkFrame{}, false
	}
	return m.blackSharkFinishExchangeUnlocked(ex)
}

// 锁内不做任何等待：曾经按积压 sleep 让设备追赶的实现在锁内等待，把多查询接口的持锁时间放大到
// 约 20 s，触发 health-monitor 误判系统睡眠并拆连接。积压只能靠整段让出锁来缓解。
//
// beginBlackSharkExchangeLocked 必须在写之前调用（设备可能回得比挂 await 还快），
// 调用方随后必须配对调用 endBlackSharkExchangeLocked。
func (m *Manager) beginBlackSharkExchangeLocked(cmd byte) uint64 {
	mission := m.link.missionSeq.Add(1)
	m.link.awaitCmd.Store(uint32(cmd))
	m.link.awaitMission.Store(mission)
	return mission
}

// endBlackSharkExchangeLocked 收尾：清掉"在等谁"，但**保留**一个短窗口，让随后到达的
// 同 cmd 应答能被暂存（而不是被当"没人等"丢弃）——见 lateResp 的注释与问题 ②③ 的处置。
func (m *Manager) endBlackSharkExchangeLocked() {
	// 不再清空暂存表：面板读连发时，下一条的收尾会把上一条刚存下的应答抹掉。过期完全由条目自身的
	// 到达时刻管（见 blackSharkLateStash）。
	m.link.awaitMission.Store(0)
	m.link.awaitCmd.Store(0)
}

// takeLateBlackSharkResponse 尝试取走"迟到应答暂存队列"里属于**本次请求**的最旧一帧。
//
// 键 = (cmd, 本次请求的判别子)，见 blackSharkStashKey：带参命令只有槽位相符才命中。
// 命中即出队（一条迟到应答只回答一次查询）。
func (m *Manager) takeLateBlackSharkResponse(cmd byte, payload []byte) (deviceproto.BlackSharkFrame, bool) {
	sub, subOK := blackSharkStashSubForRequest(cmd, payload)
	if !subOK {
		return deviceproto.BlackSharkFrame{}, false
	}
	key := blackSharkStashKey{cmd: cmd, sub: sub}

	m.link.lateResp.mu.Lock()
	defer m.link.lateResp.mu.Unlock()
	// 过期由**条目自身**的到达时刻判定（见 blackSharkLateQueue.pop）：这里不再看全局 until。
	q := m.link.lateResp.queues[key]
	if q == nil {
		return deviceproto.BlackSharkFrame{}, false
	}
	frame, ok := q.pop(time.Now())
	if !ok {
		delete(m.link.lateResp.queues, key)
		return deviceproto.BlackSharkFrame{}, false
	}
	if len(q.frames) == 0 {
		delete(m.link.lateResp.queues, key)
	}
	return frame, true
}

// stashLateBlackSharkResponse 把一条迟到应答追加进它自己 (cmd, 判别子) 的队列。
// 由收帧路径在"当前没人等任何应答"时调用；返回 true 表示确实被留下（供收帧路径记日志）。
//
// 入队一律接受（过期在取用时按条目自身时刻判定）；判别子推不出来时不暂存 —— 宁可丢，
// 也不能让它以后被别的请求取走。
func (m *Manager) stashLateBlackSharkResponse(frame deviceproto.BlackSharkFrame) bool {
	sub, subOK := blackSharkStashSubForResponse(frame)
	if !subOK {
		return false
	}
	key := blackSharkStashKey{cmd: frame.Command, sub: sub}

	m.link.lateResp.mu.Lock()
	defer m.link.lateResp.mu.Unlock()
	if m.link.lateResp.queues == nil {
		m.link.lateResp.queues = make(map[blackSharkStashKey]*blackSharkLateQueue)
	}
	q := m.link.lateResp.queues[key]
	if q == nil {
		q = &blackSharkLateQueue{}
		m.link.lateResp.queues[key] = q
	}
	q.push(frame, time.Now())
	return true
}

// blackSharkWaitResponseLocked 等待指定 cmd 的响应帧。
// 响应由读协程投递到 m.link.blackSharkResp，因此这里不占用设备句柄，
// 也不会与读循环争用（读协程是句柄的唯一拥有者）。
func (m *Manager) blackSharkWaitResponseLocked(cmd byte, timeout time.Duration) (deviceproto.BlackSharkFrame, bool) {
	resp := m.link.blackSharkResp
	if resp == nil {
		return deviceproto.BlackSharkFrame{}, false
	}
	// 序号与"在等谁"都由调用方（beginBlackSharkExchangeLocked）声明并清除 ——
	// 必须在写之前就挂好，所以不能在这里才开始。这里只读它，把日志串成一条线。
	mission := m.link.awaitMission.Load()
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
			// 超时时记下等了多久：设备回得比超时更慢时，应答会在 awaitCmd 移走后才到，看起来与
			// "设备没回"完全一样。按实际传输打标（见 blackSharkTransportLabel）。
			m.logInfo("黑鲨 %s 查询超时（mission #%d）：cmd=0x%02X 等满 %v",
				m.blackSharkTransportLabel(), mission, cmd, timeout)
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
	if !m.blackSharkLinkReadyLocked() {
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

// usesBlackSharkTransport 报告「当前连接的是不是黑鲨、且链路就绪（USB 句柄在手，或 BLE 执行器已连）」。
// 供调试命令等不持有 m.mutex 的入口做设备族分发，避免误用飞智的帧格式。
func (m *Manager) usesBlackSharkTransport() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.blackSharkLinkReadyLocked()
}

// IsBlackSharkActive 当前活动设备是不是黑鲨、且能和它换帧。
// 供上层在做"重查询"之前先判断，避免对着别的设备白跑一轮查询。
func (m *Manager) IsBlackSharkActive() bool {
	// 语义 = 能和黑鲨换帧（不是"档案 ID 对得上"）：调用方里有直接摸 m.link / await 状态机的代码，
	// 链路不通时那些路径只能空跑。只按档案判的场合用 IsBlackSharkProfileActive。
	return m.usesBlackSharkTransport()
}

// IsBlackSharkProfileActive 当前连接的设备是不是任意一条黑鲨档案（HID / BLE / USB）。
// 给"档案级"判据用（UI 展示、面板可用性、图传入口）；凡是要摸 HID 句柄的地方仍用 IsBlackSharkActive。
func (m *Manager) IsBlackSharkProfileActive() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.isConnected && types.IsBlackSharkDeviceProfileID(m.activeProfile.ID)
}

// BlackSharkTransport 报告当前黑鲨链路走的是哪条通路（types.DeviceTransportUSB / ...BLE）；
// 不是黑鲨或未连接时返回空串。图传的总预算必须按通路取：USB 约 7~40 ms/帧、BLE 约 160 ms/帧，
// 用一个数管两边会把慢的那边判错。
func (m *Manager) BlackSharkTransport() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if !m.isConnected || !isBlackSharkProfileID(m.activeProfile.ID) {
		return ""
	}
	return m.deviceType
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
	if !m.blackSharkCanWriteLocked() {
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
	if !m.blackSharkCanWriteLocked() {
		return false
	}
	err := m.blackSharkTryWriteFrameLocked(deviceproto.BlackSharkCmdNotify, payload...)
	if errors.Is(err, errBlackSharkWireBusy) {
		// 不经 wire 直接发：0x07 推送不等应答、不挂 awaitCmd、不排空，本来就不需要这把锁；
		// 走 tryAcquire 反而会被正在等应答的交换逐拍跳过，把设备唯一的周期性输入饿掉。
		// 仍是 1 Hz 一拍一条，不增加链路负担。
		err = m.blackSharkWriteFrameOnWireLocked(deviceproto.BlackSharkCmdNotify, payload...)
	}
	if err != nil {
		m.logWarn("系统信息推送失败: %v", err)
		return false
	}
	return true
}

// IssueAudioSpectrumLevel 推一帧音频同步档位（CMD 0x15，载荷 = 档位 1|2|3）。
// 帧率由调用方控制：官方约 4.9 帧/秒（间隔 0.20 s）。
func (m *Manager) IssueAudioSpectrumLevel(level byte) bool {
	// 允许 0（静音）：官方是**持续流**，静音时发 0 —— 不发就会让设备永远停在最后一档
	//   （见 deviceproto.BlackSharkAudioLevelSilent 的注释与真机证据）。
	if level > deviceproto.BlackSharkAudioLevelHigh {
		return false
	}
	if !m.usesBlackSharkTransport() {
		return false
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.blackSharkCanWriteLocked() {
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
	if !m.blackSharkCanWriteLocked() {
		return false
	}
	if err := m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdNotifyMouseKeyPress); err != nil {
		m.logDebug("响应灯效事件发送失败: %v", err)
		return false
	}
	return true
}

// ---- 灯效读写与重置 ----

// BlackSharkRgbModeCatalog 构造 8 个灯效槽位的目录，零设备 IO。
//
// 只发 1 条 0x13 读当前槽，不再预取 8 条 0x14 —— 官方 getInitConfig 的线上形态就是"一条查询
// 填满整页"（槽位明细在子页按需用 0x14 读）。
// 不预取不会写坏任何字节：两条写路径都是"自读自写"，从不依赖前端传来的模式列表。
// 返回里只有 current.Index 那项 Known=true。
func (m *Manager) BlackSharkRgbModeCatalog(current types.BlackSharkRgbMode, currentKnown bool) []types.BlackSharkRgbMode {
	modes := make([]types.BlackSharkRgbMode, 0, deviceproto.BlackSharkRgbEffectCount)
	for idx := 1; idx <= deviceproto.BlackSharkRgbEffectCount; idx++ {
		if currentKnown && current.Index == idx {
			modes = append(modes, current)
			continue
		}
		var mode types.BlackSharkRgbMode
		blackSharkRgbModeStaticMeta(&mode, idx)
		// Known 保持 false：这一槽的参数值还没读过（名称/量程等静态项已经填好）。
		modes = append(modes, mode)
	}
	return modes
}

// BlackSharkRgbModeCatalogAll 取全部 8 槽的参数视图，供"把每种灯效状态都落进 config"用。
//
// 与 BlackSharkRgbModeCatalog 的差别：后者只把**当前槽**填成真值、其余留骨架（对齐官方"一条 0x13
// 填满一页"）。这里把其余槽各读一遍（0x14，纯读不切换）⇒ 缓存与界面都有 8 槽真值，界面不必再
// "点一个没读过的槽补读一条"。槽位读不到就保留骨架（Known=false），不拿零值冒充设备现状。
func (m *Manager) BlackSharkRgbModeCatalogAll(current types.BlackSharkRgbMode, currentKnown bool) []types.BlackSharkRgbMode {
	catalog := m.BlackSharkRgbModeCatalog(current, currentKnown)
	for i := range catalog {
		if catalog[i].Known {
			continue
		}
		if view, ok := m.BlackSharkRgbModeView(catalog[i].Index); ok {
			catalog[i] = view
		}
	}
	return catalog
}

// BlackSharkCurrentRgbMode 读取当前生效的灯效模式（CMD 0x13，空载荷）。
func (m *Manager) BlackSharkCurrentRgbMode() (types.BlackSharkRgbMode, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.blackSharkLinkReadyLocked() {
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

// BlackSharkRgbModeView 只读**指定槽位**的参数并转成展示结构（CMD 0x14，纯读不切换）。
//
// 与 BlackSharkCurrentRgbMode 的区别：那条读的是"当前生效槽"（0x13），这条读的是指定槽。
// 供灯效页"选中还没读过的槽时按需读"用 —— 拿不到真值就不能放开编辑：色相与单色标志由调用方
// 按这个模式的值初始化，而它们的初值来自上一个模式，直接放开等于把别的模式的颜色写进去。
//
// 顺带把读到的块写进按槽缓存（blackSharkQueryRgbBlockLocked 内部完成），后续保存的读-改-写可复用它。
func (m *Manager) BlackSharkRgbModeView(idx int) (types.BlackSharkRgbMode, bool) {
	if idx < 1 || idx > deviceproto.BlackSharkRgbEffectCount {
		return types.BlackSharkRgbMode{}, false
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.blackSharkLinkReadyLocked() {
		return types.BlackSharkRgbMode{}, false
	}
	frame, ok := m.blackSharkQueryRgbBlockLocked(idx)
	if !ok {
		return types.BlackSharkRgbMode{}, false
	}
	mode, ok := m.blackSharkRgbModeFromFrameLocked(frame)
	// 应答错位必须当失败：把别的槽的值当成这个槽的，界面就会拿错值去初始化编辑状态。
	if !ok || !mode.Known || mode.Index != idx {
		return types.BlackSharkRgbMode{}, false
	}
	return mode, true
}

// colorControlsView 把「按页颜色控件」表转成展示结构。
func colorControlsView(slot int) types.BlackSharkRgbColorControlsView {
	c := deviceproto.BlackSharkRgbColorControlsBySlot[slot]
	return types.BlackSharkRgbColorControlsView{
		ColorMode: c.ColorMode, SingleColor: c.SingleColor, Hue: string(c.Hue),
	}
}

// blackSharkRgbModeStaticMeta 按槽位号填与设备取值无关的静态字段（依据全来自 deviceproto 协议表），
// 供 blackSharkRgbModeFromFrameLocked 与 BlackSharkRgbModeCatalog 共用。
func blackSharkRgbModeStaticMeta(mode *types.BlackSharkRgbMode, idx int) {
	mode.Index = idx
	mode.Name = deviceproto.BlackSharkRgbEffectNames[idx]
	mode.NeedsHostData = deviceproto.BlackSharkRgbEffectNeedsHostData[idx]
	// 未登记的模式退回空区间，由前端据此禁用拖动，不臆造数值；rg.Fixed 表示官方把该模式的速度滑块
	// 置灰（如常亮、音频同步）。
	mode.SpeedRange = types.ValueRangeView{}
	mode.SpeedDisabled = false
	if rg, ok := deviceproto.BlackSharkRgbSpeedRange[idx]; ok {
		mode.SpeedRange = types.ValueRangeView{Min: rg.Min, Max: rg.Max}
		mode.SpeedDisabled = rg.Fixed
	}
	mode.BrightnessRange = types.ValueRangeView{Min: 10, Max: 100}
	// 该页显示哪些颜色控件，见 deviceproto.BlackSharkRgbColorControlsBySlot。
	mode.ColorControls = colorControlsView(idx)
}

// blackSharkRgbModeFromFrameLocked 把 0x13/0x14 的回复帧转成展示结构。
func (m *Manager) blackSharkRgbModeFromFrameLocked(frame deviceproto.BlackSharkFrame) (types.BlackSharkRgbMode, bool) {
	eff, ok := deviceproto.DecodeBlackSharkRgbEffects(frame.Payload)
	if !ok {
		return types.BlackSharkRgbMode{}, false
	}
	r, g, b := eff.ColorRGB()
	var mode types.BlackSharkRgbMode
	// 静态部分（名称/量程/控件可用性）与目录骨架共用同一份构造。
	blackSharkRgbModeStaticMeta(&mode, eff.ModeIndex())
	// 动态部分：只有这些来自设备答复。
	// 速度直通：payload[1..2] 就是官方 UI 的滑块值本身。
	// 超出量程的真实读数原样展示，由前端按范围决定是否可拖动，不在此截断。
	mode.Speed = eff.Speed()
	mode.Brightness = eff.Brightness()
	mode.StaticColor = eff.StaticColor()
	// 颜色下拉选项，取自 payload[0] 的高半字节。
	mode.ColorOption = eff.ColorOption()
	mode.ColorOptionName = deviceproto.BlackSharkRgbColorOptionNames[eff.ColorOption()]
	mode.Red, mode.Green, mode.Blue = r, g, b
	// Hue：把设备存的 RGB 反推成色相（**唯一实现**在 deviceproto，与正向 BlackSharkHueToColor 互逆）。
	// 前端色相滑块用它初始化，从而"滑杆反映设备颜色"且"只切单色/彩色不会连颜色一起改"。
	mode.Hue = deviceproto.BlackSharkRGBToHue(r, g, b)
	mode.Known = true
	return mode, true
}

// blackSharkQueryRgbBlockLocked 读某个槽位的原始灯效块（CMD 0x14），用加长的单窗口。
//
// 用 blackSharkWriteReadTimeout 的单个加长窗口：这条自读错过一次就是用户可见的失败
// （0x14 读超时 ⇒ 写被拒 ⇒ 界面按设备真值把开关拨回去）。只发一帧，不重发。
// 调用方需持有 m.mutex。
func (m *Manager) blackSharkQueryRgbBlockLocked(idx int) (deviceproto.BlackSharkFrame, bool) {
	payload := []byte{byte(idx)}
	frame, ok := m.blackSharkQueryWithTimeoutLocked(
		deviceproto.BlackSharkCmdGetAnyRgbEffects, payload, blackSharkWriteReadTimeout)
	if ok {
		m.rememberRgbBlockLocked(idx, frame.Payload)
		return frame, true
	}
	// 读失败不中止，回落缓存里同一模式的那份块（对齐官方"超时非致命 + 从内存写"）。
	// 安全前提：缓存键是模式索引，且只在同一代连接内有效（见 rememberRgbBlockLocked）；
	// 拿不到缓存就照实失败，也绝不重发。
	if cached, hit := m.cachedRgbBlockLocked(idx); hit {
		m.logWarn("黑鲨灯效模式 %d 读超时，回落缓存里的同一模式块（%d 字节；零新增帧、零新增等待）",
			idx, len(cached))
		// 用**组帧再解析**的方式构造，保证 Length/Payload/原始字节都与真帧一致
		//（下游只取 Payload，但别留半个形状的帧给以后的调用方）。
		if f, pok := deviceproto.ParseBlackSharkFrame(
			deviceproto.BuildBlackSharkFrame(deviceproto.BlackSharkCmdGetAnyRgbEffects, cached...)); pok {
			return f, true
		}
	}
	return deviceproto.BlackSharkFrame{}, false
}

// rememberRgbBlockLocked 记下某个模式刚读到的整块参数（调用方须持 m.mutex）。
// 连接换代时整体作废：缓存绝不跨连接复用。
func (m *Manager) rememberRgbBlockLocked(idx int, payload []byte) {
	if len(payload) == 0 {
		return
	}
	gen := m.connectionGen.Load()
	if m.link.rgbBlocks == nil || m.link.rgbBlocksGen != gen {
		m.link.rgbBlocks = make(map[int][]byte)
		m.link.rgbBlocksGen = gen
	}
	m.link.rgbBlocks[idx] = append([]byte(nil), payload...)
}

// cachedRgbBlockLocked 取缓存副本；换代（或没缓存）一律返回 false（调用方须持 m.mutex）。
func (m *Manager) cachedRgbBlockLocked(idx int) ([]byte, bool) {
	if m.link.rgbBlocks == nil || m.link.rgbBlocksGen != m.connectionGen.Load() {
		return nil, false
	}
	payload, ok := m.link.rgbBlocks[idx]
	if !ok || len(payload) == 0 {
		return nil, false
	}
	return append([]byte(nil), payload...), true
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
	if !m.blackSharkCanWriteLocked() {
		return false
	}
	// 1) 读目标模式的原始块（不自己构造载荷）
	frame, ok := m.blackSharkQueryRgbBlockLocked(idx)
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

	// 2) 原样写回该块 —— 写 0x12 即"应用并选中"这个模式。只写这一条、只等它自己的写 ACK，
	// 不做回读校验（此前的回读几乎没有收益，却把失败面放大到三分之一的命令量）。
	if acked := m.blackSharkWriteAndAwaitAckLocked(deviceproto.BlackSharkCmdSetLighting, payload...); !acked {
		m.logError("黑鲨灯效模式 %d 切换未获写 ACK（cmd=0x12），无法确认设备已受理", idx)
		return false
	}
	// 刚写进去的 8 字节就是该模式当前的块 ⇒ 顺手更新读缓存（**零成本**）：同一模式再点选时
	// 连那次 0x14 读都能省掉（缓存命中优先，见 blackSharkQueryRgbBlockLocked）。
	m.rememberRgbBlockLocked(idx, payload)
	// 写 ACK 已到 = 设备已受理并选中，这是"选模式"的权威路径；
	// coreapp 的 host-effect 循环据此决定是否跑键鼠钩子 / 音频采集。
	m.blackSharkActiveRgbSlot.Store(int32(idx))
	return true
}

// BlackSharkRgbModeEdit 是「灯效页一次保存」要写进某个模式的字段集合。
//
// 负值 = 本次不改这一项（与 durationFromMillis 的负数哨兵同一套约定）：页面没有对应控件的字段
// 必须留空，让读-改-写把设备当前值原样带出去。
type BlackSharkRgbModeEdit struct {
	Speed       int // <0 = 不改
	Brightness  int // <0 = 不改
	Hue         int // <0 = 不改（0..359，语义同色相滑杆）
	ColorOption int // <0 = 不改
	StaticColor bool
	HasStatic   bool // false ⇒ 该页没有单色/彩色开关，别动这个字节
}

// SetBlackSharkRgbModeAll 一次写完某个模式的整块参数，并顺带选中它（0x12 写哪个槽就选中哪个槽）。
//
// 三条刻意取舍：
//   - 只走一遍读-改-写。界面改一项灯效最多涉及同一个 8 字节块里的多个字段，分开写就要各走一遍。
//   - 用**等写 ACK** 的写而不是直接写：界面要给出可信的"保存成功"。
//   - ACK 没等到**不直接判失败** —— 由回读裁决，与开关写路径同一条判据（ACK 迟到 ≠ 没生效）。
//
// 返回保存后的权威视图，供调用方原地更新、省掉"保存后整页重读"。Index=0 即失败；
// Known=false 表示只拿到写 ACK、没拿到回读 ⇒ 此时只有 Index 可用，字段值不能用。
func (m *Manager) SetBlackSharkRgbModeAll(idx int, edit BlackSharkRgbModeEdit) (types.BlackSharkRgbMode, bool) {
	if idx < 1 || idx > deviceproto.BlackSharkRgbEffectCount {
		m.logError("黑鲨灯效模式索引越界: %d（有效 1..%d）", idx, deviceproto.BlackSharkRgbEffectCount)
		return types.BlackSharkRgbMode{}, false
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.blackSharkCanWriteLocked() {
		return types.BlackSharkRgbMode{}, false
	}

	// 读回原始块（读-改-写的第一步，绝不凭空构造）：没改的字段由它原样带出，
	// 所以前端不必"知道设备当前那个字节是什么"。
	frame, ok := m.blackSharkQueryRgbBlockLocked(idx)
	if !ok {
		m.logError("黑鲨灯效模式 %d 参数读取失败，拒绝盲写", idx)
		return types.BlackSharkRgbMode{}, false
	}
	current, ok := deviceproto.DecodeBlackSharkRgbEffects(frame.Payload)
	if !ok {
		m.logError("黑鲨灯效模式 %d 参数解析失败（长度 %d），拒绝盲写", idx, len(frame.Payload))
		return types.BlackSharkRgbMode{}, false
	}
	if current.ModeIndex() != idx {
		m.logError("黑鲨灯效模式 %d 参数读取错位（应答 ModeIndex=%d），拒绝写入以免改到别的槽位",
			idx, current.ModeIndex())
		return types.BlackSharkRgbMode{}, false
	}

	target := current
	if edit.Speed >= 0 || edit.Brightness >= 0 {
		speed, brightness := target.Speed(), target.Brightness()
		if edit.Speed >= 0 {
			speed = edit.Speed
		}
		if edit.Brightness >= 0 {
			brightness = edit.Brightness
		}
		target = target.WithSpeedBrightness(speed, brightness)
	}
	if edit.Hue >= 0 {
		r, g, b := deviceproto.BlackSharkHueToColor(edit.Hue)
		target = target.WithColor(r, g, b)
	}
	if edit.HasStatic {
		target = target.WithStaticColor(edit.StaticColor)
	}
	if edit.ColorOption >= 0 {
		target = target.WithColorOption(edit.ColorOption)
	}

	payload := target.EncodeBlackSharkRgbEffects()
	if len(payload) != 8 {
		m.logError("黑鲨灯效载荷长度异常: %d（应为 8）", len(payload))
		return types.BlackSharkRgbMode{}, false
	}

	acked := m.blackSharkWriteAndAwaitAckLocked(deviceproto.BlackSharkCmdSetLighting, payload...)
	// ACK 未获不等于失败（设备可能只是慢于窗口，见 blackSharkWriteReadTimeout）：由回读裁决。
	back, ok := m.blackSharkQueryRgbBlockLocked(idx)
	if !ok {
		if !acked {
			m.logError("黑鲨灯效模式 %d：写 ACK 与回读都没拿到，无法确认设备已受理", idx)
			return types.BlackSharkRgbMode{}, false
		}
		m.logWarn("黑鲨灯效模式 %d 已获写 ACK，但回读未应答；按已受理处理（与开关写路径同判据）", idx)
		m.blackSharkActiveRgbSlot.Store(int32(idx))
		// 没有回读就没有权威字段值：只把槽位给出去，字段一律留零（Known=false）。
		return types.BlackSharkRgbMode{Index: idx}, true
	}
	got, ok := deviceproto.DecodeBlackSharkRgbEffects(back.Payload)
	if !ok {
		m.logError("黑鲨灯效模式 %d 保存后回读解析失败，无法确认生效", idx)
		return types.BlackSharkRgbMode{}, false
	}
	// 回读必须答的就是本次写的那个槽：0x14 是按槽寻址的，错位说明设备答的是别的槽。
	// 纯切换（edit 全为 -1）时 verifyBlackSharkRgbModeEdit 一条都不比 ⇒ 这条是**唯一的写后校验**。
	if got.ModeIndex() != idx {
		m.logError("黑鲨灯效模式 %d 保存后回读错位（应答 ModeIndex=%d），判为未生效",
			idx, got.ModeIndex())
		return types.BlackSharkRgbMode{}, false
	}
	if err := verifyBlackSharkRgbModeEdit(got, target, edit); err != nil {
		m.logError("黑鲨灯效模式 %d 保存后回读不一致: %v", idx, err)
		return types.BlackSharkRgbMode{}, false
	}
	m.blackSharkActiveRgbSlot.Store(int32(idx))
	view, ok := m.blackSharkRgbModeFromFrameLocked(back)
	if !ok || !view.Known {
		// 解码已成功 ⇒ 理论到不了；宁可让前端退回"整页重读"，也不给半个视图。
		return types.BlackSharkRgbMode{Index: idx}, true
	}
	view.Current = true
	return view, true
}

// verifyBlackSharkRgbModeEdit 只核对本次**改过**的字段。
// 没改的字段读回来可能与发出去的不同（设备会规范化量程外的值），一并比对会把成功误报成失败。
func verifyBlackSharkRgbModeEdit(got, want deviceproto.BlackSharkRgbEffects, edit BlackSharkRgbModeEdit) error {
	if edit.Speed >= 0 && got.Speed() != want.Speed() {
		return fmt.Errorf("期望 speed=%d，实际 %d", want.Speed(), got.Speed())
	}
	if edit.Brightness >= 0 && got.Brightness() != want.Brightness() {
		return fmt.Errorf("期望 brightness=%d，实际 %d", want.Brightness(), got.Brightness())
	}
	if edit.Hue >= 0 {
		gr, gg, gb := got.ColorRGB()
		wr, wg, wb := want.ColorRGB()
		if gr != wr || gg != wg || gb != wb {
			return fmt.Errorf("期望 RGB=%02X%02X%02X，实际 %02X%02X%02X", wr, wg, wb, gr, gg, gb)
		}
	}
	if edit.HasStatic && got.Field16 != want.Field16 {
		return fmt.Errorf("期望静态色标志=0x%02X，实际 0x%02X", want.Field16, got.Field16)
	}
	if edit.ColorOption >= 0 && got.ColorOption() != want.ColorOption() {
		return fmt.Errorf("期望颜色选项=%d，实际=%d", want.ColorOption(), got.ColorOption())
	}
	return nil
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
	if !m.blackSharkCanWriteLocked() {
		return false
	}
	payload := e.EncodeBlackSharkRgbEffects()
	if err := m.blackSharkWriteFrameLocked(deviceproto.BlackSharkCmdSetLighting, payload...); err != nil {
		m.logError("黑鲨灯效还原写入失败: %v", err)
		return false
	}
	back, ok := m.blackSharkQueryRgbBlockLocked(idx)
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
	if !m.blackSharkCanWriteLocked() {
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
	if !m.isConnected || !m.blackSharkLinkReadyLocked() {
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
	// gearSourceCfg 承接档位循环里顺手取到的第一个配置回复（`0x23` 已不再单独读，
	// 参照源改由它的 byte[0] 提供）。循环结束后并进真正的开关状态里。
	var gearSourceCfg deviceproto.BlackSharkGearConfig
	var gearSourceOK bool
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
		// 参照源顺手取自配置回复 byte[0]（见 applyBlackSharkCoolingSource）：
		//   `BlackSharkSwitchStates` 已不单独读 0x23，这里补上，零额外查询。
		if !gearSourceOK {
			gearSourceCfg, gearSourceOK = cfg, true
		}
		snap.GearCurves = append(snap.GearCurves, gs)
	}
	st := m.BlackSharkSwitchStates()
	if gearSourceOK {
		applyBlackSharkCoolingSource(&st, gearSourceCfg)
	}
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

	// 档位曲线不原样写回设备：主机侧调速时转速由主机逐拍下发，
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
	// profileActive 是档案级（与 IPC 入口同一判据）：若这里反过来判"未连接"，界面会显示
	// "未连接黑鲨设备"而设备其实连着。linkReady 是链路级，决定下面各字段能不能读。
	profileActive := m.isConnected && isBlackSharkProfileID(m.activeProfile.ID)
	linkReady := m.blackSharkLinkReadyLocked()
	if profileActive && !linkReady {
		// 诊断：连接正常但读判据不成立时，把每条判据的取值一次打全，
		// 否则只能看到界面一句"未连接黑鲨设备"，无从判断是哪一条不成立。
		m.logInfo("黑鲨链路判据不成立：profile=%q type=%q transport=%q bleExecNil=%v bleExecConnected=%v",
			m.activeProfile.ID, m.deviceType, m.deviceTransport,
			m.bleExecutor == nil, m.bleExecutor != nil && m.bleExecutor.IsConnected())
	}
	m.mutex.RUnlock()
	if !profileActive {
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
			// 参照源顺手取自配置回复 byte[0]（0x26 与 0x25 同形状）⇒ 零额外查询；
			//   这也是 `BlackSharkSwitchStates` 不再单独读 0x23 之后它的来源。
			applyBlackSharkCoolingSource(&info.Switches, cfg)
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
			applyBlackSharkCoolingSource(&info.Switches, cfg)
		}
		info.Gears = append(info.Gears, row)
	}

	fillBlackSharkDerivedInfo(&info)
	return info
}

// fillBlackSharkDerivedInfo 填 BlackSharkInfo 里纯计算出来的字段：曲线量程、曲线标定表、
// 固定值量程。
//
// 抽出来是因为 Info 有两条生产路径（向设备实读、从落盘快照还原），两边必须共用同一段填充代码，
// 否则标定表一改只改到一边。
func fillBlackSharkDerivedInfo(info *types.BlackSharkInfo) {
	info.CurveMinRPM, info.CurveMaxRPM = deviceproto.BlackSharkCurveRangeForRPM()
	if len(info.CurveCalibration) == 0 {
		for _, c := range deviceproto.BlackSharkCurveCalibration {
			info.CurveCalibration = append(info.CurveCalibration, types.BlackSharkCurveCalibrationEntry{
				FieldRPM: c.Field, ActualRPM: c.RPM,
			})
		}
	}
	info.FixedMinRPM = deviceproto.BlackSharkMinRPM
	info.FixedMaxRPM = deviceproto.BlackSharkMaxRPM
}

// BlackSharkInfoFromSnapshot 用落盘快照还原 Info，不做任何设备 IO（BlackSharkInfo 要串行 11 条
// 查询、BLE 上单条可到秒级）。Available / Firmware / 缓存元信息由调用方负责。
func (m *Manager) BlackSharkInfoFromSnapshot(snapshot types.BlackSharkStateSnapshot) types.BlackSharkInfo {
	info := types.BlackSharkInfo{
		Switches: snapshot.Switches,
		Gears:    snapshot.Gears,
	}
	fillBlackSharkDerivedInfo(&info)
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
	if !m.blackSharkLinkReadyLocked() {
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
	isBlackShark := m.blackSharkLinkReadyLocked()
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

// RefreshBlackSharkDeviceVersion 只重读设备侧版本号（0x01）并合并进缓存 —— 它不该和厂商清单的
// 网络检查一起被节流，否则换设备或升级后面板仍显示旧版本。
func (m *Manager) RefreshBlackSharkDeviceVersion() (types.BlackSharkFirmwareStatus, bool) {
	m.mutex.Lock()
	version, ok := m.blackSharkDeviceFirmwareVersionLocked()
	m.mutex.Unlock()
	if !ok {
		return types.BlackSharkFirmwareStatus{}, false
	}

	status := types.BlackSharkFirmwareStatus{}
	if cached := m.cachedBlackSharkFirmwareStatus(); cached != nil {
		status = *cached
	} else {
		toolPath := firstExistingBlackSharkTool()
		status.UpdateMethod = blackSharkUpdateMethodOfficial
		status.OfficialToolPath = toolPath
		status.OfficialToolFound = toolPath != ""
	}
	status.Supported = true
	status.CurrentVersion = strings.TrimSpace(strings.Trim(version, "\x00"))
	if status.CurrentVersion != "" && status.LatestVersion != "" {
		status.UpdateAvailable = compareBlackSharkVersions(status.CurrentVersion, status.LatestVersion) < 0
	}
	if status.CurrentVersion != "" && status.MinVersion != "" {
		status.BelowMinVersion = compareBlackSharkVersions(status.CurrentVersion, status.MinVersion) < 0
	}
	m.storeBlackSharkFirmwareStatus(status)
	return status, true
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
	if !m.blackSharkCanWriteLocked() {
		return false
	}
	// 写与"等 ACK"必须成对独占总线：0xC2 的应答靠 awaitCmd 配对，被别的交换插进来就会丢掉。
	// 排空也放进段内，避免拿到上一轮的残留。
	m.link.wire.acquire()
	defer m.link.wire.release()
	deviceproto.DrainBlackSharkFrames(m.link.blackSharkResp)
	mission := m.beginBlackSharkExchangeLocked(deviceproto.BlackSharkCmdSetLcdShowPos)
	defer m.endBlackSharkExchangeLocked()
	if err := m.blackSharkWriteFrameOnWireLocked(deviceproto.BlackSharkCmdSetLcdShowPos, payload...); err != nil {
		m.logError("黑鲨 LCD 显示参数下发失败（mission #%d）: %v", mission, err)
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
	if !m.blackSharkCanWriteLocked() {
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
	ready := m.blackSharkCanWriteLocked()
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
