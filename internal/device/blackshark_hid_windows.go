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
	return retryDeviceSendContext(ctx, "Black Shark USB image frame", func() error {
		return m.flyDigiHID.usb.transfer(blackSharkUSBOutEndpoint, report, 800)
	})
}

// blackSharkImageMetadataAckTimeout 等 C4（图传信息帧）应答的上限。
// 官方实录里 C4 → 应答 ≈530ms；这里按"收到应答帧"判定，给足余量。
const blackSharkImageMetadataAckTimeout = 2 * time.Second

// blackSharkImageFirstBlockAfter 是 C4 → 首数据块的**最小**间隔。
//
// 设备的数据窗口在 C4 之后才开。窗口没开就灌数据，设备会把**整轮**都拒掉
// （每块回 `0xC6 状态 0x0C`、块号不推进），并保留旧图不放；而 `0xC5` 回读只是把
// 设备手上那份 11 字节元数据原样回显 ⇒ 只看回读会把"整轮被拒"误判成"写入成功"。
// 真机边界（2095 块整轮，逐档实测）：530ms 全拒；560 / 600 / 650 / 700 / 1000 / 1200 /
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

	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.activeProfile.ID != types.BlackSharkBRB02USBProfileID ||
		m.deviceType != types.DeviceTransportUSB ||
		m.productID != types.BlackSharkBRB02HIDProductID ||
		!m.isConnected || m.flyDigiHID == nil || m.flyDigiHID.usb == nil {
		return fmt.Errorf("黑鲨 USB 设备未连接，图片传输仅支持 libusb 有线连接")
	}
	m.ensureBlackSharkRXLocked()
	// 先排空：上一轮可能还留着一条 C4 应答，别把旧的当成这一轮的握手应答。
	deviceproto.DrainBlackSharkFrames(m.link.blackSharkResp)
	// C4 是握手帧：设备先回一帧 0xC4 才认后续数据块。挂"在等这个 cmd"必须早于发首帧，
	// 否则设备回得比挂 await 快时应答会被当成"无人等"丢掉，这一轮就只能超时。
	m.link.awaitCmd.Store(uint32(deviceproto.BlackSharkCmdImageMetadata))
	defer m.link.awaitCmd.Store(0)
	// 流控应答（每块一条 0xC6）是设备对"这一块收不收"的唯一直接回答，全程收下来计数。
	flow := m.beginBlackSharkImageFlowCapture()
	defer m.endBlackSharkImageFlowCapture()

	sent := 0
	writeFrame := func(frame []byte) error {
		if err := m.writeBlackSharkUSBImageFrameContextLocked(ctx, frame); err != nil {
			return err
		}
		sent++
		if progress != nil && (sent%blackSharkImageProgressEvery == 0 || sent == len(frames)) {
			progress(sent, len(frames))
		}
		return nil
	}

	handshakeAt := time.Now()
	if err := sendBlackSharkImageFrames(ctx, frames, writeFrame, func() error {
		// 等真实应答：缺了它这一轮基本全被拒。超时不中止（那条 530ms 的等待已经过去，
		// 窗口多半已开），只留痕供排查。
		if _, ok := m.blackSharkWaitResponseLocked(deviceproto.BlackSharkCmdImageMetadata, blackSharkImageMetadataAckTimeout); !ok {
			m.logWarn("黑鲨图传：%v 内没等到 C4 信息帧应答，按节拍继续发送", blackSharkImageMetadataAckTimeout)
		}
		// 再补足"首块最早时刻"：窗口是在 C4 之后固定开的，等应答只解决了握手，
		// 应答到得早（或拿到的是上一轮残留的应答）仍会踩在窗口外。
		if wait := blackSharkImageFirstBlockAfter - time.Since(handshakeAt); wait > 0 {
			return waitBlackSharkImageDelay(ctx, wait)
		}
		return nil
	}); err != nil {
		return err
	}
	accepted, rejected := tallyBlackSharkImageFlow(flow)
	m.logInfo("黑鲨图传：2095 个数据块 + 尾帧已发出，设备流控应答 收下 %d / 拒收 %d", accepted, rejected)
	// "一条都没收下"是确定的失败（设备没拿到新图，屏上还是旧的），不能按成功上报；
	// 完全没有应答帧时不下结论 —— 那是我们没收上来的问题，不是设备拒收。
	if accepted == 0 && rejected > 0 {
		return fmt.Errorf("黑鲨图传被设备整轮拒收：%d 块回 0xC6 状态 0x0C、0 块收下（数据窗口未开或 C4 握手未成立）", rejected)
	}
	return nil
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

func (m *Manager) setBlackSharkHIDTargetSpeedLocked(speed types.FanSpeedValue) bool {
	if !m.isConnected || m.flyDigiHID == nil || !types.IsRPMSpeedUnit(speed.Unit) {
		return false
	}
	rpm := speed.Value
	if rpm < 0 {
		rpm = 0
	}
	if rpm > deviceproto.BlackSharkMaxRPM {
		rpm = deviceproto.BlackSharkMaxRPM
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

	// 这两个端点按描述符是**中断（interrupt）**类型；我们经 libusb 的 bulk API 访问，
	// Windows/WinUSB 对中断管道同样接受（实测应答可收）。参考实现同样写作"libusb 中断传输"。
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
	blackSharkImageFrameDelay = 7 * time.Millisecond
	blackSharkImagePageDelay  = 40 * time.Millisecond

	// blackSharkImageTailToCommitDelay 是尾帧 → 提交帧（0xC5）之间的总间隔。
	// 依据上游 beta 的上传编排（其 SendBlackSharkImageWithProgress 里可见 0.064s / 0.006s / 3s 等常量）
	// 与官方抓包尾段：尾帧 →63.5ms→ 16×心跳(6ms 间隔) →6ms→ 0xC5 ≈ 165ms。
	// 我们的序列里没有那 16 条心跳帧，故以等长静默替代，保证写入与提交之间留出同样的时间。
	blackSharkImageTailToCommitDelay = 165 * time.Millisecond
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

func waitBlackSharkImagePacing(ctx context.Context, index int) error {
	if index == 0 {
		return nil
	}
	delay := blackSharkImageFrameDelay
	// A4 payloads carry 58 sequential image bytes. Pause when one crosses a 4 KiB flash page.
	if index <= 2095 {
		start := (index - 1) * 58
		if start/4096 != (start+58)/4096 {
			delay += blackSharkImagePageDelay
		}
	}
	timer := time.NewTimer(delay)
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

// sendBlackSharkImageFrames 顺序发送图传帧，并按节拍等待。
//
// afterHandshake 在第 1 帧（C4 信息帧）发出后调用一次，供调用方等设备对 C4 的应答。
// 这一步不能省：设备要先完成 C4 握手才开数据窗口，未完成的块一律被拒并回
// `0xC6 状态 0x0C`（不等应答时整轮 2036/2038 个 0x0C，且与节拍无关 —— 每块 50ms
// 照样 0x0C；等应答后 2096/2096 全 0x00）。设备拒收后不会提交新图，屏上仍是旧图，
// 而 `0xC5` 回读只是把主机写进去的那 11 字节元数据原样回显 ⇒ 不修这里会"上传成功但屏幕不变"。
func sendBlackSharkImageFrames(ctx context.Context, frames [][]byte, send func([]byte) error, afterHandshake func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if send == nil {
		return fmt.Errorf("黑鲨 USB 图传发送函数为空")
	}
	if len(frames) == 0 {
		return fmt.Errorf("黑鲨 USB 图传帧为空")
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
		if err := send(frame); err != nil {
			return fmt.Errorf("黑鲨 USB 图传帧 %d 发送失败: %w", index, err)
		}
		if index == 0 && afterHandshake != nil {
			if err := afterHandshake(); err != nil {
				return err
			}
		}
		if err := waitBlackSharkImagePacing(ctx, index); err != nil {
			return err
		}
	}
	return ctx.Err()
}
