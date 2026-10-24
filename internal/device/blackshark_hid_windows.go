//go:build !legacydevice && windows

package device

import (
	"context"
	"fmt"
	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/types"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const blackSharkHIDReportLen = 65

// blackSharkPairedBLEAddress 从 Windows 配对表返回名字匹配的已配对蓝牙设备地址。
//
// BRB02 配对后空闲时不广播，名称扫描扫不到，只能按地址直连（见 ConnectBlackSharkBLEDirect）。
// 配对表在 HKLM\SYSTEM\CurrentControlSet\Services\BTHPORT\Parameters\Devices 下：子键是 12 位
// 十六进制（正序 MAC），值 Name 是设备名。查不到返回 ""，调用方应回落名称扫描，不要当失败。
func blackSharkPairedBLEAddress(nameFilter string) string {
	want := strings.ToLower(strings.TrimSpace(nameFilter))
	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Services\BTHPORT\Parameters\Devices`, registry.READ)
	if err != nil {
		return ""
	}
	defer key.Close()
	subKeys, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return ""
	}
	for _, sub := range subKeys {
		address, ok := bleAddressFromPairedKey(sub)
		if !ok {
			continue
		}
		subKey, err := registry.OpenKey(key, sub, registry.READ)
		if err != nil {
			continue
		}
		raw, _, err := subKey.GetBinaryValue("Name")
		subKey.Close()
		if err != nil || len(raw) == 0 {
			continue
		}
		name := strings.ToLower(decodePairedDeviceName(raw))
		if name == "" {
			continue
		}
		if (want != "" && (name == want || strings.Contains(name, want))) || strings.Contains(name, "brb02") {
			return address
		}
	}
	return ""
}

func (m *Manager) connectBlackSharkUSBLocked() (bool, map[string]string) {
	dev, err := openBlackSharkUSBTransportDevice()
	if err != nil {
		m.logWarn("黑鲨 USB 设备连接失败: %v", err)
		m.isConnected = false
		m.deviceType = ""
		m.productID = 0
		m.flyDigiHID = nil
		m.currentFanData.Store(nil)
		return false, nil
	}
	m.flyDigiHID = dev
	m.productID = dev.productID
	m.deviceType = types.DeviceTransportUSB
	m.activeProfile = types.BlackSharkBRB02USBProfile()
	generation := m.connectionGen.Add(1)
	ready := m.startFlyDigiHIDReaderLocked(dev, generation)
	if !waitForFlyDigiHIDReady(ready, flyDigiHIDReadyTimeout) {
		m.logWarn("黑鲨 USB 已打开，但在 %s 内未收到状态帧", flyDigiHIDReadyTimeout)
		m.closeFlyDigiHIDLocked()
		m.isConnected = false
		m.deviceType = ""
		m.productID = 0
		m.currentFanData.Store(nil)
		return false, nil
	}
	m.isConnected = true
	info := m.blackSharkHIDInfoLocked(dev.path)
	info["transport"] = types.DeviceTransportUSB
	info["profileId"] = types.BlackSharkBRB02USBProfileID
	m.logInfo("黑鲨 USB 设备连接成功: %s", info["product"])
	return true, info
}

func (m *Manager) blackSharkHIDInfoLocked(path string) map[string]string {
	return map[string]string{
		"manufacturer": types.BlackSharkBRB02Vendor,
		"product":      types.BlackSharkBRB02DisplayName,
		"serial":       path,
		"model":        types.BlackSharkBRB02DisplayName,
		"transport":    types.DeviceTransportUSB,
		"endpoint":     path,
		"productId":    fmt.Sprintf("0x%04X", types.BlackSharkBRB02HIDProductID),
		"profileId":    types.BlackSharkBRB02USBProfileID,
	}
}

func blackSharkHIDReport(frame []byte) []byte {
	report := make([]byte, blackSharkHIDReportLen)
	copy(report, frame)
	return report
}

func (m *Manager) writeBlackSharkHIDFrameLocked(frame []byte) error {
	return m.writeBlackSharkHIDFrameContextLocked(context.Background(), frame)
}

func (m *Manager) writeBlackSharkHIDFrameContextLocked(ctx context.Context, frame []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if m.writesBlocked.Load() {
		return fmt.Errorf("device writes are blocked during system suspend")
	}
	// 飞智与黑鲨共用 m.flyDigiHID：只看句柄会把飞智设备当成黑鲨，
	// 故这里的判据必须与 blackSharkHandleLocked 一致（已连接 + 黑鲨档案 + 句柄在手）。
	if !m.isConnected || m.flyDigiHID == nil || !isBlackSharkProfileID(m.activeProfile.ID) {
		return fmt.Errorf("黑鲨 HID 设备未连接")
	}
	if len(frame) == 0 || len(frame) > blackSharkHIDReportLen {
		return fmt.Errorf("黑鲨 HID 帧长度无效: %d", len(frame))
	}
	report := blackSharkHIDReport(frame)
	m.recordDebugFrame("tx", m.deviceType, report)
	return retryDeviceSendContext(ctx, "Black Shark HID command", func() error {
		return m.flyDigiHID.WriteReport(report, 800*time.Millisecond)
	})
}

// blackSharkUSBImageHandleLocked 校验这台机器现在确实可以走 USB 图传，并把端点句柄取出来。
// 调用方必须持 m.mutex（读的都是 m.mutex 保护的字段）。
//
// 把字段读取从逐帧路径里挪走是为了避开 ABBA：查询是持 m.mutex 等 wire，而图传整轮握着 wire。
// 所以入口处一次性快照句柄，循环里只用快照。
func (m *Manager) blackSharkUSBImageHandleLocked() (*blackSharkUSBDevice, error) {
	if !m.isConnected || m.deviceType != types.DeviceTransportUSB ||
		m.activeProfile.ID != types.BlackSharkBRB02USBProfileID ||
		m.productID != types.BlackSharkBRB02HIDProductID || m.flyDigiHID == nil || m.flyDigiHID.usb == nil {
		return nil, fmt.Errorf("黑鲨 USB 设备未连接")
	}
	return m.flyDigiHID.usb, nil
}

// writeBlackSharkUSBImageFrameOn 用已经取好的句柄发一帧，刻意不碰 m.mutex，于是可以在整轮
// 图传里逐帧调用而不与查询形成 ABBA（见 blackSharkUSBImageHandleLocked）。
//
// 中途掉线能安全收尾：断开会把句柄置 0 ⇒ transfer 报 "libusb device is not open"，且 coreapp 侧
// 会调 cancelAllBlackSharkImageTransfers 取消 ctx，不会空转到底。
func (m *Manager) writeBlackSharkUSBImageFrameOn(ctx context.Context, usb *blackSharkUSBDevice, frame []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if m.writesBlocked.Load() {
		return fmt.Errorf("device writes are blocked during system suspend")
	}
	report, err := blackSharkUSBImageReport(frame)
	if err != nil {
		return err
	}
	m.recordDebugFrame("tx", types.DeviceTransportUSB, report)
	return retryDeviceSendContext(ctx, "Black Shark USB image frame", func() error {
		return usb.transfer(blackSharkUSBOutEndpoint, report, 800)
	})
}

// blackSharkImageMetadataAckTimeout 等 C4（图传信息帧）应答的上限。
// 官方实录里 C4 → 应答 ≈530ms；这里按"收到应答帧"判定，给足余量。
const blackSharkImageMetadataAckTimeout = 6 * time.Second

// blackSharkImageFirstBlockAfter 是 C4 → 首数据块的**最小**间隔。
//
// 设备的数据窗口在 C4 之后才开。窗口没开就灌数据，设备会把**整轮**都拒掉
// （每块回 `0xC6 状态 0x0C`、块号不推进），并保留旧图不放；而 `0xC5` 回读只是把
// 设备手上那份 11 字节元数据原样回显 ⇒ 只看回读会把"整轮被拒"误判成"写入成功"。
// 真机边界（2095 块整轮，逐档测过）：530ms 全拒；560 / 600 / 650 / 700 / 1000 / 1200 /
// 1500 / 3000 ms 全收，连测 8 档一致，且整轮没有"窗口随后关闭"的迹象。取 1.2s 留
// ≈2.2 倍余量；代价 0.7s，相对整轮 ~16s 可忽略。
const blackSharkImageFirstBlockAfter = 1200 * time.Millisecond

// blackSharkImageProgressEvery 进度回调的节流粒度（按帧）。一整轮 2098 帧，
// 逐帧回调只会白烧 CPU。
const blackSharkImageProgressEvery = 16

// SendBlackSharkImageWithProgress uploads one complete RGB565 image over the BRB02 USB
// interrupt endpoint. progress 每 blackSharkImageProgressEvery 帧回调一次（末帧必回调），nil = 不回调。
func (m *Manager) SendBlackSharkImageWithProgress(ctx context.Context, rgb565 []byte, progress func(sent, total int)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	frames, err := deviceproto.BuildBlackSharkImageUpload(rgb565)
	if err != nil {
		return err
	}

	// m.mutex 只覆盖"通道选择 + 收帧通道准备"这一小段，不整轮持有。
	//
	// 只覆盖"通道选择 + 收帧通道准备"这一小段：整轮图传（一轮 135 s / 2096 块）若占着 m.mutex，
	// 任何需要它的操作（面板读 / 0x15 推送 / 状态推送）都要等到图传结束。排他性由下面的
	// m.link.wire 负责；所有 m.mutex 保护的字段都在入口一次性读成快照（usbImage / bleSender），
	// 循环里只用快照，既不会读到半个状态，也不需要逐帧取锁。
	m.mutex.Lock()
	// 通道选择：USB 保持原有判据；BLE 走执行器的原始帧写（没有 65 字节报文封套）。
	bleSender := m.blackSharkBLESenderLocked()
	// 逐帧路径不再进 m.mutex，这是收窄的必要配套：blackSharkQueryWithTimeoutLocked 是持
	// m.mutex 等 m.link.wire 的，而本函数整轮握着 wire ⇒ 若循环里逐帧去取 m.mutex 就构成 ABBA：
	// 查询握 mutex 等 wire、图传握 wire 等 mutex。所以 USB 句柄在这里一次性快照，之后只用快照。
	var usbImage *blackSharkUSBDevice
	if bleSender == nil {
		usb, err := m.blackSharkUSBImageHandleLocked()
		if err != nil {
			m.mutex.Unlock()
			return fmt.Errorf("黑鲨设备未连接，图片传输仅支持已连接的 USB 或蓝牙通路")
		}
		usbImage = usb
	}
	m.ensureBlackSharkRXLocked()
	// 原始帧钩子只碰原子量 + 执行器自己的回调位（见其实现），放在这里是为了让它与
	// 其它"准备"一起留在锁内，之后整轮都不再需要 m.mutex。
	if bleSender != nil {
		m.ensureBlackSharkBLERXHook(bleSender)
	}
	m.mutex.Unlock()
	// 整轮图传独占总线：从排空一直到函数收尾，期间不允许任何查询/写插进来。图传因此不再是可与
	// 查询并行的第二条路，而是同一个串行器里的一段长任务（对齐官方 T5：长包就是普通 mission，
	// 共用同一条队列）。
	//
	// 代价是上传期间 1 Hz 推送排不进总线（它 tryAcquire 拿到 busy 就改走不经 wire 的直发，不等）。本函数内部不再调用
	// 任何要 wire 的入口，所以不会自锁。
	m.link.wire.acquire()
	defer m.link.wire.release()
	// 先排空：上一轮可能还留着一条 C4 应答，别把旧的当成这一轮的握手应答。
	deviceproto.DrainBlackSharkFrames(m.link.blackSharkResp)
	// C4 是握手帧：设备先回一帧 0xC4 才认后续数据块。挂"在等这个 cmd"必须早于发首帧，
	// 否则设备回得比挂 await 快时应答会被当成"无人等"丢掉，这一轮就只能超时。
	m.beginBlackSharkExchangeLocked(deviceproto.BlackSharkCmdImageMetadata)
	defer m.endBlackSharkExchangeLocked()
	// 流控应答（每块一条 0xC6）是设备对"这一块收不收"的唯一直接回答。
	// 两条通路都投进 m.link.imageFlow：USB 由读协程投，BLE 由通知回调里的收帧状态机投
	// （所以 BLE 侧必须先挂上原始帧钩子）。
	flow := m.beginBlackSharkImageFlowCapture()
	defer m.endBlackSharkImageFlowCapture()

	sent := 0
	var writeFrame func([]byte) error
	if bleSender != nil {
		m.ensureBlackSharkBLERXHook(bleSender)
		// 图传的 BLE 写不走 bleFrameBackend（那里是"一条命令一次写"的收口），所以链路判死必须
		// 在这里也补一次，否则最该被覆盖的这条路径反而漏掉：写失败后执行器仍 IsConnected() == true，
		// 健康监控永不重连。
		writeFrame = func(frame []byte) error {
			// 用 WriteRawStream 而不是 WriteRaw：块流强制不带响应写。带响应写是给控制命令用的，
			// 块流的流控由设备的 0xC6 逐块回答，让每块再去等一次 ATT 应答会改变整轮耗时基线
			// （见 WriteRawStream 的注释）。
			if err := bleSender.WriteRawStream(ctx, frame); err != nil {
				cmd := byte(0)
				if len(frame) > 2 {
					cmd = frame[2]
				}
				m.reportBlackSharkBLEWriteFailure(cmd, err)
				return err
			}
			return nil
		}
	} else {
		// 用入口处快照的句柄逐帧写，循环里刻意不进 m.mutex（ABBA 见上面收窄注释）。
		writeFrame = func(frame []byte) error { return m.writeBlackSharkUSBImageFrameOn(ctx, usbImage, frame) }
	}
	counted := func(frame []byte) error {
		if err := writeFrame(frame); err != nil {
			return err
		}
		sent++
		if progress != nil && (sent%blackSharkImageProgressEvery == 0 || sent == len(frames)) {
			progress(sent, len(frames))
		}
		return nil
	}

	// 节拍由设备回执驱动：发一块、等一条 0xC6；拒收就退避后重发该块。
	// 计数放在这里而不是发完再数通道 —— 应答会被等待函数消费掉。
	//
	// 计数放在这里而不是发完再数通道（应答会被等待函数消费掉）；拒绝与超时必须分开计 ——
	// 混成一个数就看不出是设备不收，还是回执丢了。
	var accepted, rejected, flowTimeouts int
	// waitFlow 的返回值带上 error：c.Done() 表示主动不发了（取消 / 预算到期），与"设备没回"
	// 完全不同。过去两者都被记成"超时"，把一次主动取消误报成设备不回。
	waitFlow := func(c context.Context, timeout time.Duration) (byte, bool, error) {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case status := <-flow:
			if status == deviceproto.BlackSharkImageFlowAccepted {
				accepted++
			} else {
				rejected++
			}
			return status, true, nil
		case <-timer.C:
			flowTimeouts++
			return 0, false, nil
		case <-c.Done():
			return 0, false, c.Err()
		}
	}

	handshakeAt := time.Now()
	if err := sendBlackSharkImageFrames(ctx, frames, counted, func() error {
		// 等真实应答：缺了它这一轮基本全被拒。超时不中止（那条 530ms 的等待已经过去，
		// 窗口多半已开），只留痕供排查。
		if _, ok := m.blackSharkWaitResponseLocked(deviceproto.BlackSharkCmdImageMetadata, blackSharkImageMetadataAckTimeout); !ok {
			m.logWarn("黑鲨图传：%v 内没等到 C4 信息帧应答，按节拍继续发送", blackSharkImageMetadataAckTimeout)
		}
		// 再补足"首块最早时刻"：窗口是在 C4 之后固定开的，等应答只解决了握手，
		// 应答到得早（或拿到的是上一轮残留的应答）仍会踩在窗口外。这条约束两条通路都成立。
		if wait := blackSharkImageFirstBlockAfter - time.Since(handshakeAt); wait > 0 {
			return waitBlackSharkImageDelay(ctx, wait)
		}
		return nil
	}, waitFlow); err != nil {
		// 闭环失败即失败：中途被取消、或某块重发耗尽都会被 sendBlackSharkImageFrames 报出来。
		// err 必须一起打：曾出现"已发 554 帧，收下 553 / 拒收 0 / 未应答 0"却看不到原因，
		// 只能靠时间戳反推出是总预算到期。
		m.logError("黑鲨图传失败：已发 %d 帧，收下 %d / 拒收 %d / 超时 %d，原因: %v",
			sent, accepted, rejected, flowTimeouts, err)
		return err
	}
	// 收尾：把循环没消费掉的零星应答也计入（末块那条可能刚到）。
	tailAccepted, tailRejected := tallyBlackSharkImageFlow(flow)
	accepted += tailAccepted
	rejected += tailRejected
	m.logInfo("黑鲨图传完成：%d 帧已发出（数据块 %d），设备流控应答 收下 %d / 拒收 %d / 超时重发 %d（含末块）",
		sent, dataFrameCount(frames), accepted, rejected, flowTimeouts)
	// 部分拒收不会静默通过：每个数据块都必须拿到一条 0x00 才继续，拒收会重发、
	// 重发耗尽即整体失败 —— 所以这里不需要再回看统计。（旧判定只看"整轮全拒"，已被这个循环取代。）
	return nil
}

// dataFrameCount 数一条图传序列里有多少个 0xA4 数据块（用于日志）。
func dataFrameCount(frames [][]byte) int {
	n := 0
	for _, frame := range frames {
		if len(frame) > 0 && frame[0] == deviceproto.BlackSharkCmdImageBlock {
			n++
		}
	}
	return n
}

// beginBlackSharkImageFlowCapture 开始收集 0xC6 流控应答（读协程按 read 侧投递，见 handleBlackSharkHIDRX）。
func (m *Manager) beginBlackSharkImageFlowCapture() chan byte {
	// 容量覆盖一整轮（2095 块 + 尾帧），全程不收也不会丢应答 —— 上传期间发送循环占着
	// 设备写锁，没机会边发边数，只能发完再数。
	ch := make(chan byte, deviceproto.BlackSharkScreenImageFrames+64)
	m.link.imageFlow.Store(&ch)
	return ch
}

func (m *Manager) endBlackSharkImageFlowCapture() {
	m.link.imageFlow.Store(nil)
}

// tallyBlackSharkImageFlow 统计流控应答：0x00 = 收下，0x0C = 拒收。
func tallyBlackSharkImageFlow(ch chan byte) (accepted, rejected int) {
	for {
		select {
		case status := <-ch:
			if status == deviceproto.BlackSharkImageFlowAccepted {
				accepted++
			} else {
				rejected++
			}
		default:
			return accepted, rejected
		}
	}
}

// setBlackSharkHIDTargetSpeedLocked 主机侧调速的唯一入口，BLE 与 USB 共用：
// 实际写出经 blackSharkFrameBackendLocked 选后端（BLE 优先、USB 兜底），函数名里的 HID 只余历史含义。
// 调用方需持有 m.mutex（经 SetTargetSpeed 进入）。
func (m *Manager) setBlackSharkHIDTargetSpeedLocked(speed types.FanSpeedValue) bool {
	if !m.isConnected || !m.blackSharkLinkReadyLocked() || !types.IsRPMSpeedUnit(speed.Unit) {
		return false
	}
	rpm := speed.Value
	if rpm < 0 {
		rpm = 0
	}
	if rpm > deviceproto.BlackSharkMaxRPM {
		rpm = deviceproto.BlackSharkMaxRPM
	}
	// form=0 固定值。偏差小于死区就跳过重发：0x24 高频重写会把设备推到其它档的曲线上，
	// 上游 MinRPMChange 的节流挡不住温度噪声的逐拍小偏差；死区取 100，恰在设备判稳窗
	// |实测−目标| < 101 内，写与不写没有可观测差别。
	if last := int(m.blackSharkLastPushedRPM.Load()); last > 0 {
		if last == rpm || absInt(rpm-last) < blackSharkSpeedWriteDeadbandRPM {
			m.logInfo("黑鲨固定转速跳过（死区内）: 目标=%d 上次下发=%d", rpm, last)
			m.storeBlackSharkHostSpeedDisplay(rpm)
			return true
		}
	}
	// 节奏化写入的最小间隔：设备收到目标后按 200ms/步渐变逼近，间隔内的重发只会在
	// 渐变进行中反复改目标，把目标侧摆动直接灌进设备（官方整个会话只发一次仪式，
	// 之后的调速也是低频提交）。死区外但间隔未到时同样跳过。
	if lastWrite := m.blackSharkLastFixedWriteNano.Load(); lastWrite > 0 {
		if elapsed := time.Since(time.Unix(0, lastWrite)); elapsed < blackSharkFixedWriteMinInterval {
			m.logInfo("黑鲨固定转速跳过（写入间隔 %s < %s）: 目标=%d 上次下发=%d",
				elapsed.Round(time.Millisecond), blackSharkFixedWriteMinInterval, rpm, m.blackSharkLastPushedRPM.Load())
			m.storeBlackSharkHostSpeedDisplay(rpm)
			return true
		}
	}
	// 0x24[0] 是设备侧状态字段（冷却参照源），语义未定，必须读回来原样写回、读不到就不写
	// ——上游盲写 0x00 的帧不能照抄。档位取设备当前值：写 0x24 的档位字段就是切档，
	// 写当前档 = 只换值、不动档。
	current, ok := m.blackSharkReadCoolingConfigLocked()
	if !ok || current.Gear < 1 || current.Gear > deviceproto.BlackSharkGearCount {
		m.logError("黑鲨固定值下发：0x25 读不回当前配置（0x24[0] 须原样回写），放弃本次写入")
		return false
	}
	m.logInfo("黑鲨固定转速下发判定: 目标=%d 上次下发=%d 设备配置=(src=%d form=%d gear=%d sp=%d)",
		rpm, m.blackSharkLastPushedRPM.Load(), current.Source, current.Form, current.Gear, current.Value)
	setpoint := deviceproto.BlackSharkSetpointForRPM(rpm)
	frames, err := m.blackSharkFixedSpeedFramesLocked(current.Gear, uint16(setpoint), current.Source)
	if err != nil {
		m.logError("编码黑鲨固定转速失败: %v", err)
		return false
	}
	// 受控态首写 = 官方 apply 仪式（0x24[FF 暂存] → 0x08 00 → 0x24[真实值]）；
	// 之后只发单帧提交。选帧规则与仪式出处见 deviceproto.BuildBlackSharkFixedSpeedFrames。
	if err := m.writeBlackSharkFixedFramesLocked(frames); err != nil {
		m.logError("下发黑鲨固定转速失败: %v", err)
		return false
	}
	// 不回读确认：设备对控制查询偶发沉默数秒，回读会把已生效的写入误判成失败。
	// 生效与否由 0x06 状态帧的实测转速独立可观测。
	m.blackSharkLastPushedRPM.Store(int32(rpm))
	m.markBlackSharkFixedWriteLocked()
	m.storeBlackSharkHostSpeedDisplay(rpm)
	return true
}

// storeBlackSharkHostSpeedDisplay 只更新"显示用"的 FanData：目标 = 本次下发（或未变时沿用）的值，
// 当前转速保留设备 0x06 回报的那个。写入与"值没变、跳过写入"两条路都走它。
func (m *Manager) storeBlackSharkHostSpeedDisplay(rpm int) {
	current := 0
	if previous := m.currentFanData.Load(); previous != nil {
		current = int(previous.CurrentRPM)
	}
	// 固定转速写入 ⇒ 作废曲线目标显示值，界面"目标"应跟着下发值走。
	// 形态文案写进 sticky：否则下一帧状态帧会用上一次"曲线跟随"留下的文案把它盖回去。
	m.clearBlackSharkCurveTargetDisplay()
	m.setBlackSharkDisplayWorkMode("自动模式(实时转速)")
	m.currentFanData.Store(&types.FanData{
		CurrentRPM: uint16(current), TargetRPM: uint16(rpm),
		WorkMode: "自动模式(实时转速)", Transport: m.deviceType,
		SpeedUnit: types.FanSpeedUnitRPM,
	})
}

func (m *Manager) setBlackSharkHIDRGBEnabled(enabled bool) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.isConnected || m.flyDigiHID == nil {
		return false
	}
	return m.writeBlackSharkHIDFrameLocked(deviceproto.BuildBlackSharkSetRGBEnabled(enabled)) == nil
}

func (m *Manager) setBlackSharkHIDLighting(cfg types.LightStripConfig) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.isConnected || m.flyDigiHID == nil {
		return fmt.Errorf("黑鲨 HID 设备未连接")
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Mode), "off") {
		return m.writeBlackSharkHIDFrameLocked(deviceproto.BuildBlackSharkSetRGBEnabled(false))
	}
	color := types.RGBColor{R: 255, G: 255, B: 255}
	if len(cfg.Colors) > 0 {
		color = cfg.Colors[0]
	}
	lighting, err := deviceproto.BuildBlackSharkSetLighting(cfg.Mode, cfg.Speed, cfg.Brightness, color.R, color.G, color.B)
	if err != nil {
		return err
	}
	if err := m.writeBlackSharkHIDFrameLocked(deviceproto.BuildBlackSharkSetRGBEnabled(true)); err != nil {
		return err
	}
	return m.writeBlackSharkHIDFrameLocked(lighting)
}

func (m *Manager) queryBlackSharkHIDDeviceSettings() (types.DeviceSettings, error) {
	if !m.IsBlackSharkProfileActive() {
		return types.DeviceSettings{}, fmt.Errorf("当前设备不是黑鲨 BRB02")
	}
	settings := types.DeviceSettings{
		Available: m.IsConnected(),
		Source:    m.deviceType,
		ReadAt:    time.Now().Format("2006-01-02 15:04:05"),
		Model:     types.BlackSharkBRB02DisplayName,
	}
	if fanData := m.GetCurrentFanData(); fanData != nil {
		settings.WorkMode = fanData.WorkMode
		settings.WorkModeName = fanData.WorkMode
		settings.Status = &types.DeviceStatusRead{ModeName: fanData.WorkMode, CurrentRPM: int(fanData.CurrentRPM), TargetRPM: int(fanData.TargetRPM)}
	}
	if !settings.Available {
		return settings, fmt.Errorf("设备未连接")
	}
	return settings, nil
}

// ── 以下自 blackshark_libusb_windows.go 并入（同 build tag，纯搬移）──

const (
	// libusb 的错误码：-7 = LIBUSB_ERROR_TIMEOUT（空闲窗口内设备没回话，不是故障）。
	libusbErrorTimeout = -7

	// 这两个端点按描述符是**中断（interrupt）**类型；本工具经 libusb 的 bulk API 访问，
	// Windows/WinUSB 对中断管道同样接受（实机应答可收）。参考实现同样写作"libusb 中断传输"。
	blackSharkUSBOutEndpoint = 0x01
	blackSharkUSBInEndpoint  = 0x81
	blackSharkUSBInterface   = 0
)

var (
	// 必须用 NewLazyDLL 而不是 NewLazySystemDLL：后者只搜系统目录，随包放在 exe 旁边的
	// libusb-1.0.dll 会找不到，随后 LazyProc.Call 直接 panic。
	libusbDLL                  = windows.NewLazyDLL("libusb-1.0.dll")
	libusbInitProc             = libusbDLL.NewProc("libusb_init")
	libusbExitProc             = libusbDLL.NewProc("libusb_exit")
	libusbOpenProc             = libusbDLL.NewProc("libusb_open_device_with_vid_pid")
	libusbCloseProc            = libusbDLL.NewProc("libusb_close")
	libusbClaimInterfaceProc   = libusbDLL.NewProc("libusb_claim_interface")
	libusbReleaseInterfaceProc = libusbDLL.NewProc("libusb_release_interface")
	libusbBulkTransferProc     = libusbDLL.NewProc("libusb_bulk_transfer")
	libusbAutoDetachProc       = libusbDLL.NewProc("libusb_set_auto_detach_kernel_driver")
)

type blackSharkUSBDevice struct {
	ctx    uintptr
	handle uintptr
}

func scanBlackSharkUSBDevice() (string, error) {
	dev, err := openBlackSharkUSBDevice()
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("libusb:vid_%04x&pid_%04x", types.BlackSharkHIDVendorID, types.BlackSharkBRB02HIDProductID)
	_ = dev.close()
	return path, nil
}

func openBlackSharkUSBTransportDevice() (*flyDigiHIDDevice, error) {
	usb, err := openBlackSharkUSBDevice()
	if err != nil {
		return nil, err
	}
	return &flyDigiHIDDevice{
		usb:       usb,
		path:      fmt.Sprintf("libusb:vid_%04x&pid_%04x", types.BlackSharkHIDVendorID, types.BlackSharkBRB02HIDProductID),
		vendorID:  types.BlackSharkHIDVendorID,
		productID: types.BlackSharkBRB02HIDProductID,
	}, nil
}

func openBlackSharkUSBDevice() (*blackSharkUSBDevice, error) {
	var ctx uintptr
	if r, _, err := libusbInitProc.Call(uintptr(unsafe.Pointer(&ctx))); int32(r) != 0 {
		return nil, fmt.Errorf("libusb init failed: %v", err)
	}
	if ctx == 0 {
		return nil, fmt.Errorf("libusb init returned nil context")
	}
	handle, _, _ := libusbOpenProc.Call(ctx, uintptr(types.BlackSharkHIDVendorID), uintptr(types.BlackSharkBRB02HIDProductID))
	if handle == 0 {
		libusbExitProc.Call(ctx)
		return nil, fmt.Errorf("黑鲨 USB 设备未找到 (VID 0x%04X PID 0x%04X)", types.BlackSharkHIDVendorID, types.BlackSharkBRB02HIDProductID)
	}
	libusbAutoDetachProc.Call(handle, 1)
	if r, _, _ := libusbClaimInterfaceProc.Call(handle, blackSharkUSBInterface); int32(r) != 0 {
		libusbCloseProc.Call(handle)
		libusbExitProc.Call(ctx)
		return nil, fmt.Errorf("libusb claim interface %d failed: %d", blackSharkUSBInterface, int32(r))
	}
	return &blackSharkUSBDevice{ctx: ctx, handle: handle}, nil
}

func (d *blackSharkUSBDevice) close() error {
	if d == nil {
		return nil
	}
	if d.handle != 0 {
		libusbReleaseInterfaceProc.Call(d.handle, blackSharkUSBInterface)
		libusbCloseProc.Call(d.handle)
		d.handle = 0
	}
	if d.ctx != 0 {
		libusbExitProc.Call(d.ctx)
		d.ctx = 0
	}
	return nil
}

func (d *blackSharkUSBDevice) transfer(endpoint byte, buf []byte, timeout uint32) error {
	if d == nil || d.handle == 0 {
		return fmt.Errorf("libusb device is not open")
	}
	if len(buf) != blackSharkHIDReportLen {
		return fmt.Errorf("黑鲨 USB report length must be %d, got %d", blackSharkHIDReportLen, len(buf))
	}
	var transferred int32
	r, _, _ := libusbBulkTransferProc.Call(
		d.handle,
		uintptr(endpoint),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&transferred)),
		uintptr(timeout),
	)
	if int32(r) != 0 {
		// LIBUSB_ERROR_TIMEOUT(-7)：设备在这个窗口里没说话，是空闲而不是故障。
		// 必须让 errors.Is(err, errFlyDigiHIDTimeout) 能认出它 —— 否则读循环会把每个空闲周期
		// 都记成一次失败（日志刷屏 + 退避 + 连续 5 次就重连），从而错过设备的周期上报（0x06 约 2Hz）。
		if int32(r) == libusbErrorTimeout {
			return fmt.Errorf("%w (libusb endpoint 0x%02X)", errFlyDigiHIDTimeout, endpoint)
		}
		return fmt.Errorf("libusb transfer endpoint 0x%02X failed: %d", endpoint, int32(r))
	}
	if transferred != int32(len(buf)) {
		return fmt.Errorf("libusb short transfer endpoint 0x%02X: %d/%d", endpoint, transferred, len(buf))
	}
	return nil
}

// ── 以下自 blackshark_image_transport.go 并入（同 build tag，纯搬移）──

const (
	// blackSharkImageTailToCommitDelay 是尾帧 → 提交帧（0xC5）之间的总间隔。
	// 依据上游 beta 的上传编排（其 SendBlackSharkImageWithProgress 里可见 0.064s / 0.006s / 3s 等常量）
	// 与官方抓包尾段：尾帧 →63.5ms→ 16×心跳(6ms 间隔) →6ms→ 0xC5 ≈ 165ms。
	// 本工具的序列里没有那 16 条心跳帧，故以等长静默替代，保证写入与提交之间留出同样的时间。
	// 它和"首块最早时刻"同属两条跨通路约束，两条通路都适用。
	blackSharkImageTailToCommitDelay = 165 * time.Millisecond

	// blackSharkImageFlowTimeout 是"发完一块之后等它那条 0xC6"的上限。
	// 闭环节拍约 24 ms/块（BLE）与 7 ms/块（USB）⇒ 2 s 余量足够。
	blackSharkImageFlowTimeout = 2 * time.Second
	// blackSharkImageFirstBlockFlowTimeout 只给首块用：它要等设备把数据窗口打开。
	// BLE 上量到：C4 发出后首块的 0xC6 与 C4 应答都要等约 2 s 才到，正好压住 2 s 窗口被判"未应答"。
	blackSharkImageFirstBlockFlowTimeout = 6 * time.Second

	// blackSharkImageFlowMaxResend 是同一块被 0x0C 拒收后的重发上限。
	blackSharkImageFlowMaxResend = 3

	// blackSharkImageFlowBackoff 是重发前的退避：0x0C 几乎总是"发得太快"，
	// 立刻原样重发只会继续被拒。
	blackSharkImageFlowBackoff = 20 * time.Millisecond
)

func waitBlackSharkImageDelay(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
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

// blackSharkImageFlowWaiter 取一条 0xC6 流控应答的状态字节（0x00 收下 / 0x0C 拒收）。
// 两条通路都从 m.link.imageFlow 里取（USB 读协程与 BLE 通知回调都会往那里投），写成函数是为了
// 测试可以注入假实现。
//
// 第三个返回值是 error：非 nil 表示等待被中止（ctx 取消 / 预算到期），与 ok=false（超时）是两件
// 不同的事 —— 合成一个布尔值会把"主动不发了"写成"设备不回"。
type blackSharkImageFlowWaiter func(ctx context.Context, timeout time.Duration) (byte, bool, error)

// sendBlackSharkImageFrames 顺序发送图传帧，节拍由设备回执驱动。
//
// 每个数据块（0xA4）发完等一条 0xC6：0x00 继续，0x0C 退避后重发该块（上限
// blackSharkImageFlowMaxResend）；信息帧（0xC4）与提交帧（0xC5）不受流控。不用固定时钟 ——
// 那是 USB 上量到的间隔，照抄到 BLE 只会跑在设备前面并收 0x0C。先发再等（设备收不到数据包
// 之前不发 0xC6）。
// 一条 0xC6 属于"刚发的那块"还是"索要下一块"没有定论，所以这里只保证"每块消耗一条应答"这个节拍。
//
// afterHandshake 在第 1 帧（C4 信息帧）发出后调用一次，供调用方等设备对 C4 的应答并补足数据窗口。
func sendBlackSharkImageFrames(
	ctx context.Context,
	frames [][]byte,
	send func([]byte) error,
	afterHandshake func() error,
	waitFlow blackSharkImageFlowWaiter,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if send == nil {
		return fmt.Errorf("黑鲨图传发送函数为空")
	}
	if len(frames) == 0 {
		return fmt.Errorf("黑鲨图传帧为空")
	}
	// 最后一个数据帧的索引：它的应答可能一直不来 —— 设备对第 k 块的应答常由第 k+1 块触发，
	// 总数 1:1 但错开一格。中途等不到才是失败。
	lastDataIndex := -1
	firstDataIndex := -1
	for i, frame := range frames {
		if len(frame) > 0 && frame[0] == deviceproto.BlackSharkCmdImageBlock {
			if firstDataIndex < 0 {
				firstDataIndex = i
			}
			lastDataIndex = i
		}
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
		// 尾段：提交帧（序列最后一帧 0xC5）之前留出官方的尾帧→心跳→提交间隔，让设备把数据写完再提交。
		if index == len(frames)-1 {
			if err := waitBlackSharkImageDelay(ctx, blackSharkImageTailToCommitDelay); err != nil {
				return err
			}
		}
		flowPaced := waitFlow != nil && len(frame) > 0 && frame[0] == deviceproto.BlackSharkCmdImageBlock
		resends := 0
		for {
			if err := send(frame); err != nil {
				return fmt.Errorf("黑鲨图传帧 %d 发送失败: %w", index, err)
			}
			if index == 0 && afterHandshake != nil {
				if err := afterHandshake(); err != nil {
					return err
				}
			}
			if !flowPaced {
				break
			}
			flowTimeout := blackSharkImageFlowTimeout
			if index == firstDataIndex {
				// 首块用独立的长窗：BLE 上这条 0xC6 迟到约 2.0 s，正好压住 2 s 窗口，于是"设备正准备收"
				// 被判成"未应答 1"整轮失败。后续块窗口不变，拉长它会把"真卡住"的整轮耗时放大两千倍。
				flowTimeout = blackSharkImageFirstBlockFlowTimeout
			}
			status, ok, waitErr := waitFlow(ctx, flowTimeout)
			if waitErr != nil {
				// 取消 / 预算到期与"设备不回"是两回事，必须分开报，否则会把一次主动取消写成
				// "第 N 块超时"，把排查带偏。
				return fmt.Errorf("黑鲨图传在第 %d 帧被取消: %w", index, waitErr)
			}
			if !ok {
				if index == lastDataIndex {
					// 末块等不到应答属正常（见上）；不计失败，但留痕。
					break
				}
				// 没等到回执不等于失败：设备偶尔就是会慢过窗口。USB 上一轮 2098 帧里会有一处停 >2 s，
				// 位置随机（990 / 1626 / 1696 / 2049 块），而其余两千多块都是 ~7 ms 就回。
				// 单次漏回执就整轮失败，等于把一次可恢复的抖动判成硬失败，用户看到的正是"上传稳定失败"。
				// 处置与"拒收"同策：重发该块（数据块幂等），重发耗尽才整体失败。
				resends++
				if resends > blackSharkImageFlowMaxResend {
					return fmt.Errorf("黑鲨图传第 %d 块等 0xC6 超时 %v（重发 %d 次仍无回执）",
						index, flowTimeout, resends)
				}
				if err := waitBlackSharkImageDelay(ctx, blackSharkImageFlowBackoff); err != nil {
					return err
				}
				continue
			}
			if status == deviceproto.BlackSharkImageFlowAccepted {
				break
			}
			resends++
			if resends > blackSharkImageFlowMaxResend {
				return fmt.Errorf("黑鲨图传第 %d 块被设备拒收 %d 次（0xC6=0x%02X）",
					index, resends, status)
			}
			if err := waitBlackSharkImageDelay(ctx, blackSharkImageFlowBackoff); err != nil {
				return err
			}
		}
	}
	return nil
}
