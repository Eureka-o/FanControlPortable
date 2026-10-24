//go:build !legacydevice && windows

package device

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

const blackSharkHIDReportLen = 65

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
	// 官方会话以 0x22 控制源=01 开场;缺了它设备不会主动推 500ms 状态帧,
	// IN 端点会一直读超时(-7)。
	if err := dev.WriteReport(blackSharkHIDReport(deviceproto.BuildBlackSharkSessionOpen()), 800*time.Millisecond); err != nil {
		m.logWarn("发送黑鲨会话开场帧(0x22)失败: %v", err)
	}
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

const (
	blackSharkUSBWriteTimeout = 800 * time.Millisecond
)

// SendBlackSharkHostInfoFrame sends the latest parameter-page snapshot once.
func (m *Manager) SendBlackSharkHostInfoFrame() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.sendBlackSharkHostInfoFrameLocked()
}

func (m *Manager) sendBlackSharkHostInfoFrameLocked() error {
	if !m.isConnected || m.deviceType != types.DeviceTransportUSB ||
		m.activeProfile.ID != types.BlackSharkBRB02USBProfileID || m.flyDigiHID == nil {
		return fmt.Errorf("黑鲨 USB 设备未连接")
	}
	frame := m.blackSharkHostInfoFrame.Load()
	if frame == nil || len(*frame) == 0 {
		return fmt.Errorf("黑鲨屏幕参数尚未准备好")
	}
	report := blackSharkHIDReport(*frame)
	m.recordDebugFrame("tx", types.DeviceTransportUSB, report)
	return m.flyDigiHID.WriteReport(report, blackSharkUSBWriteTimeout)
}

// SetBlackSharkHostInfoRefreshInterval controls repeated parameter-page sends.
// Zero stops the loop; the loop always reuses the latest frame snapshot.
func (m *Manager) SetBlackSharkHostInfoRefreshInterval(intervalMs int) error {
	intervalMs = types.NormalizeBlackSharkScreenRefreshIntervalMs(intervalMs)
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.isConnected || m.deviceType != types.DeviceTransportUSB ||
		m.activeProfile.ID != types.BlackSharkBRB02USBProfileID || m.flyDigiHID == nil {
		return fmt.Errorf("黑鲨 USB 设备未连接")
	}
	if m.blackSharkHostInfoRefreshStop != nil {
		close(m.blackSharkHostInfoRefreshStop)
		m.blackSharkHostInfoRefreshStop = nil
	}
	if intervalMs == 0 {
		return nil
	}
	stop := make(chan struct{})
	m.blackSharkHostInfoRefreshStop = stop
	go m.blackSharkHostInfoRefreshLoop(time.Duration(intervalMs)*time.Millisecond, stop)
	return nil
}

func (m *Manager) blackSharkHostInfoRefreshLoop(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			m.mutex.Lock()
			select {
			case <-stop:
				m.mutex.Unlock()
				return
			default:
			}
			m.mutex.Unlock()
			err := m.SendBlackSharkHostInfoFrame()
			if err != nil {
				m.logDebug("黑鲨参数页刷新失败: %v", err)
			}
		}
	}
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

func parseBlackSharkHIDFrame(raw []byte) (deviceproto.BlackSharkFrame, bool) {
	if frame, ok := deviceproto.ParseBlackSharkFrame(raw); ok {
		return frame, true
	}
	if len(raw) > 1 {
		return deviceproto.ParseBlackSharkFrame(raw[1:])
	}
	return deviceproto.BlackSharkFrame{}, false
}

func parseBlackSharkHIDFanData(raw []byte) *types.FanData {
	frame, ok := parseBlackSharkHIDFrame(raw)
	if !ok {
		return nil
	}
	rpm, flag, ok := deviceproto.ParseBlackSharkStatus(frame)
	if !ok {
		return nil
	}
	return &types.FanData{
		CurrentRPM:  uint16(rpm),
		TargetRPM:   uint16(rpm),
		CurrentMode: flag,
		WorkMode:    deviceproto.BlackSharkModeName(flag),
		Transport:   types.DeviceTransportHID,
		SpeedUnit:   types.FanSpeedUnitRPM,
		Command:     frame.Command,
	}
}

func (m *Manager) handleBlackSharkHIDRX(generation uint64, raw []byte) bool {
	if m.connectionGen.Load() != generation {
		return false
	}
	if stream := m.blackSharkImageACK.Load(); stream != nil {
		// 图传进行中:读循环是 IN 端点唯一读者,把设备应答转交给图传流程;
		// 常规状态帧在此期间暂停更新,避免打扰 7ms 写节拍。
		if frame, ok := parseBlackSharkHIDFrame(raw); ok {
			stream.forward(frame)
		}
		return true
	}
	fanData := parseBlackSharkHIDFanData(raw)
	if fanData == nil || m.connectionGen.Load() != generation {
		return false
	}
	fanData.Transport = m.deviceType
	m.currentFanData.Store(fanData)
	callback := m.onFanDataUpdate
	if callback != nil {
		go func() {
			if m.connectionGen.Load() == generation {
				callback(fanData)
			}
		}()
	}
	return true
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
	if !m.isConnected || m.flyDigiHID == nil {
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

func (m *Manager) writeBlackSharkUSBImageFrameContextLocked(ctx context.Context, frame []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if m.writesBlocked.Load() {
		return fmt.Errorf("device writes are blocked during system suspend")
	}
	if !m.isConnected || m.deviceType != types.DeviceTransportUSB ||
		m.activeProfile.ID != types.BlackSharkBRB02USBProfileID ||
		m.productID != types.BlackSharkBRB02HIDProductID || m.flyDigiHID == nil || m.flyDigiHID.usb == nil {
		return fmt.Errorf("黑鲨 USB 设备未连接")
	}
	report, err := blackSharkUSBImageReport(frame)
	if err != nil {
		return err
	}
	m.recordDebugFrame("tx", types.DeviceTransportUSB, report)
	// 图传块流不做重试:重发既破坏 7ms 节拍又可能重复块号(协议 §7),失败直接上抛。
	return m.flyDigiHID.usb.transfer(blackSharkUSBOutEndpoint, report, 800)
}

// SendBlackSharkImage uploads one complete RGB565 image over the BRB02 USB bulk endpoint.
// 时序遵循协议 §7:C4 握手 → 等握手 ACK(~530ms)→ 2095 个数据块(7ms/块,
// 跨 4 KiB flash 页边界停 40ms)→ 尾帧 → 停顿 → C5 提交 → 等提交回显。
// 全程由读循环转交设备应答;任一 C6 ACK 状态非 0x00 立即中止。
func (m *Manager) SendBlackSharkImage(ctx context.Context, rgb565 []byte) error {
	return m.SendBlackSharkImageWithProgress(ctx, rgb565, nil)
}

// SendBlackSharkImageWithProgress uploads one RGB565 image and reports coarse
// transfer progress. The callback is deliberately integer-percent based so a
// 7ms block stream cannot flood the IPC event queue.
func (m *Manager) SendBlackSharkImageWithProgress(ctx context.Context, rgb565 []byte, progress func(stage string, completed, total int)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	frames, err := deviceproto.BuildBlackSharkImageUpload(rgb565)
	if err != nil {
		return err
	}
	if len(frames) < 4 {
		return fmt.Errorf("黑鲨图传帧序列不完整: %d 帧", len(frames))
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.activeProfile.ID != types.BlackSharkBRB02USBProfileID ||
		m.deviceType != types.DeviceTransportUSB ||
		m.productID != types.BlackSharkBRB02HIDProductID ||
		!m.isConnected || m.flyDigiHID == nil || m.flyDigiHID.usb == nil {
		return fmt.Errorf("黑鲨 USB 设备未连接，图片传输仅支持 libusb 有线连接")
	}
	dataTotal := len(frames) - 2
	lastPercent := -1
	report := func(stage string, completed int) {
		if progress == nil {
			return
		}
		percent := 0
		if dataTotal > 0 {
			percent = completed * 100 / dataTotal
		}
		if stage == "completed" {
			percent = 100
		}
		if percent == lastPercent && stage != "failed" {
			return
		}
		lastPercent = percent
		progress(stage, completed, dataTotal)
	}
	report("preparing", 0)

	stream := newBlackSharkImageACKStream(len(frames) + 64)
	m.blackSharkImageACK.Store(stream)
	defer m.blackSharkImageACK.Store(nil)

	send := func(seq int, frame []byte) error {
		if err := m.writeBlackSharkUSBImageFrameContextLocked(ctx, frame); err != nil {
			return err
		}
		// 写入后顺手检查已到达的 ACK:设备开始拒绝(0x0C)就立刻中止,不再空跑。
		if err := stream.rejection(); err != nil {
			return fmt.Errorf("第 %d 帧写入后: %w", seq, err)
		}
		return nil
	}

	// C4 握手:设备 ~525ms 后回 ACK,收到 ACK 再发首块(官方节奏)。
	// 无 ACK 时绝不盲发块流(0x0C 级联,协议 §7);重试一次后仍失败则中止。
	if err := send(1, frames[0]); err != nil {
		return fmt.Errorf("黑鲨图传握手帧(0xC4)发送失败: %w", err)
	}
	report("handshake", 0)
	if err := stream.waitAccepted(ctx, blackSharkImageACKTimeout,
		deviceproto.BlackSharkCmdImageMetadata, deviceproto.BlackSharkCmdImageBlockACK); err != nil {
		m.logWarn("黑鲨图传握手未应答,重试一次: %v", err)
		if err := send(1, frames[0]); err != nil {
			return fmt.Errorf("黑鲨图传握手帧(0xC4)重发失败: %w", err)
		}
		if err := stream.waitAccepted(ctx, blackSharkImageACKTimeout,
			deviceproto.BlackSharkCmdImageMetadata, deviceproto.BlackSharkCmdImageBlockACK); err != nil {
			m.logWarn("黑鲨图传握手失败: %v", err)
			return fmt.Errorf("黑鲨图传握手失败: %w", err)
		}
	}

	// ACK→首块:官方/工具箱实测锚定 ACK+4.2ms;本机探针证实 ACK 后立即发首块
	// 会被设备以 0x0C 拒绝(设备需数毫秒完成握手收尾),5ms 锚定后 2095 块全绿。
	if err := sleepBlackSharkImage(ctx, blackSharkImageFirstBlockDelay); err != nil {
		return err
	}

	// frames[0]=C4,frames[1:n-1]=2095 个数据块+尾帧,frames[n-1]=C5。
	if err := sendBlackSharkImageFrames(ctx, frames[1:len(frames)-1], func(index int, frame []byte) error {
		if err := send(index+2, frame); err != nil {
			return err
		}
		report("uploading", index+1)
		return nil
	}); err != nil {
		return err
	}
	// 官方在尾帧与 C5 之间夹带 16 条 0x07 心跳(63.5ms 后开始, 6ms 间隔);
	// 用当前参数页快照复刻该窗口,没有快照则退化为固定停顿。
	if heartbeat := m.blackSharkHostInfoFrame.Load(); heartbeat != nil && len(*heartbeat) > 0 {
		if err := sleepBlackSharkImage(ctx, blackSharkImageTailDelay); err != nil {
			return err
		}
		for i := 0; i < blackSharkImageTailHeartbeats; i++ {
			if err := send(len(frames)+i, *heartbeat); err != nil {
				return fmt.Errorf("黑鲨图传尾部心跳发送失败: %w", err)
			}
			if err := sleepBlackSharkImage(ctx, blackSharkImageHeartbeatGap); err != nil {
				return err
			}
		}
	} else if err := sleepBlackSharkImage(ctx, blackSharkImageCommitDelay); err != nil {
		return err
	}
	if err := send(len(frames)+blackSharkImageTailHeartbeats, frames[len(frames)-1]); err != nil {
		return fmt.Errorf("黑鲨图传提交帧(0xC5)发送失败: %w", err)
	}
	report("committing", dataTotal)
	// C5 的应答是 15B 元数据回显(非状态帧),收到命令字即视为已提交。
	if err := stream.waitEcho(ctx, blackSharkImageACKTimeout, deviceproto.BlackSharkCmdImageCommit); err != nil {
		if errors.Is(err, errBlackSharkImageACKTimeout) {
			m.logWarn("黑鲨图传未收到 0xC5 提交回显,图片可能未显示: %v", err)
			return nil
		}
		return fmt.Errorf("黑鲨图传提交失败: %w", err)
	}
	m.logInfo("黑鲨图传完成: %d 个数据块 + 尾帧已提交", len(frames)-3)
	report("completed", dataTotal)
	return nil
}

func (m *Manager) setBlackSharkHIDTargetSpeedLocked(speed types.FanSpeedValue) bool {
	if !m.isConnected || m.flyDigiHID == nil || !types.IsRPMSpeedUnit(speed.Unit) {
		return false
	}
	rpm := speed.Value
	if rpm < 0 {
		rpm = 0
	}
	if rpm > 4000 {
		rpm = 4000
	}
	if err := m.writeBlackSharkHIDFrameLocked(deviceproto.BuildBlackSharkSetSpeed(rpm)); err != nil {
		m.logError("设置黑鲨 HID 转速失败: %v", err)
		return false
	}
	previous := m.currentFanData.Load()
	current := 0
	if previous != nil {
		current = int(previous.CurrentRPM)
	}
	m.currentFanData.Store(&types.FanData{
		CurrentRPM: uint16(current), TargetRPM: uint16(rpm),
		WorkMode: "自动模式(实时转速)", Transport: m.deviceType,
		SpeedUnit: types.FanSpeedUnitRPM,
	})
	return true
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
