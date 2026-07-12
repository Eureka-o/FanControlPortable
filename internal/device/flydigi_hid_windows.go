//go:build !legacydevice && windows

package device

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

const (
	hidControlReportLen = 23
	hidLightReportLen   = 65

	flyDigiHIDMaxConsecutiveReadErrors = 5
	flyDigiHIDReadErrorBackoff         = 500 * time.Millisecond
	flyDigiHIDReadyTimeout             = 2 * time.Second
)

var errFlyDigiHIDTimeout = errors.New("hid read timeout")

func (m *Manager) shouldUseLegacyHIDLocked() bool {
	return m.deviceTransport == types.DeviceTransportHID
}

func (m *Manager) connectLegacyHIDLocked() (bool, map[string]string) {
	return m.connectFlyDigiHIDLocked()
}

func (m *Manager) connectFlyDigiHIDLocked() (bool, map[string]string) {
	if !m.shouldUseLegacyHIDLocked() {
		return false, nil
	}

	productIDs := flyDigiHIDProductIDsForProfile(m.activeProfile.ID)
	dev, err := openFlyDigiHIDDevice(productIDs)
	if err != nil {
		m.logWarn("飞智 HID 设备连接失败: %v", err)
		m.isConnected = false
		m.deviceType = ""
		m.productID = 0
		m.flyDigiHID = nil
		m.currentFanData.Store(nil)
		return false, m.flyDigiHIDInfoLocked(0, "")
	}

	m.flyDigiHID = dev
	m.deviceType = types.DeviceTransportHID
	m.productID = dev.productID
	if profile, ok := types.FlyDigiProfileForHIDProductID(dev.productID); ok {
		m.activeProfile = profile
	}
	ready := m.startFlyDigiHIDReaderLocked(dev)
	if !waitForFlyDigiHIDReady(ready, flyDigiHIDReadyTimeout) {
		m.logWarn("FlyDigi HID opened but no valid status frame arrived within %s", flyDigiHIDReadyTimeout)
		m.closeFlyDigiHIDLocked()
		m.isConnected = false
		m.deviceType = ""
		m.productID = 0
		m.currentFanData.Store(nil)
		return false, m.flyDigiHIDInfoLocked(0, "")
	}
	m.isConnected = true

	info := m.flyDigiHIDInfoLocked(dev.productID, dev.path)
	m.logInfo("飞智 HID 设备连接成功: %s", info["product"])
	return true, info
}

func (m *Manager) disconnectFlyDigiHIDLocked() bool {
	if m.deviceType != types.DeviceTransportHID {
		return false
	}
	m.closeFlyDigiHIDLocked()
	m.isConnected = false
	m.deviceType = ""
	m.productID = 0
	m.currentFanData.Store(nil)
	return true
}

func (m *Manager) closeFlyDigiHIDLocked() {
	if m.flyDigiHID == nil {
		return
	}
	m.stopFlyDigiHIDReaderLocked()
	m.flyDigiHID = nil
}

func (m *Manager) startFlyDigiHIDReaderLocked(dev *flyDigiHIDDevice) <-chan struct{} {
	m.stopFlyDigiHIDReaderLocked()
	stop := make(chan struct{})
	done := make(chan struct{})
	ready := make(chan struct{})
	m.flyDigiHIDStop = stop
	m.flyDigiHIDDone = done
	go m.readFlyDigiHIDLoop(dev, stop, done, ready)
	return ready
}

func waitForFlyDigiHIDReady(ready <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ready:
		return true
	case <-timer.C:
		return false
	}
}

func (m *Manager) stopFlyDigiHIDReaderLocked() {
	stop := m.flyDigiHIDStop
	done := m.flyDigiHIDDone
	m.flyDigiHIDStop = nil
	m.flyDigiHIDDone = nil
	if stop == nil {
		return
	}
	close(stop)
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(800 * time.Millisecond):
		m.logWarn("FlyDigi HID reader stop timed out")
	}
}

func (m *Manager) readFlyDigiHIDLoop(dev *flyDigiHIDDevice, stop <-chan struct{}, done chan<- struct{}, ready chan<- struct{}) {
	defer close(done)
	if dev == nil {
		return
	}
	defer func() {
		if err := dev.Close(); err != nil {
			m.logWarn("FlyDigi HID close after reader exit failed: %v", err)
		}
	}()
	if err := dev.SetNonblock(true); err != nil {
		m.logWarn("设置飞智 HID 非阻塞读取失败: %v", err)
	}

	consecutiveErrors := 0
	readySent := false
	for {
		select {
		case <-stop:
			return
		default:
		}

		raw, err := dev.ReadReport(500 * time.Millisecond)
		if err != nil {
			nextErrors, shouldStop := flyDigiHIDReadErrorState(err, consecutiveErrors)
			consecutiveErrors = nextErrors
			if errors.Is(err, errFlyDigiHIDTimeout) {
				continue
			}
			m.logWarn("FlyDigi HID read failed (%d/%d): %v", consecutiveErrors, flyDigiHIDMaxConsecutiveReadErrors, err)
			if shouldStop {
				m.logWarn("FlyDigi HID read failed too many times, triggering reconnect")
				m.handleFlyDigiHIDReaderFailure(dev)
				return
			}
			select {
			case <-stop:
				return
			case <-time.After(flyDigiHIDReadErrorBackoff):
			}
			continue
		}
		consecutiveErrors = 0
		if len(raw) == 0 {
			continue
		}
		if m.handleFlyDigiHIDRX(raw) && !readySent {
			close(ready)
			readySent = true
		}
	}
}

func flyDigiHIDReadErrorState(err error, consecutiveErrors int) (int, bool) {
	if err == nil || errors.Is(err, errFlyDigiHIDTimeout) {
		return 0, false
	}
	next := consecutiveErrors + 1
	return next, next >= flyDigiHIDMaxConsecutiveReadErrors
}

func (m *Manager) handleFlyDigiHIDReaderFailure(dev *flyDigiHIDDevice) {
	if dev == nil {
		return
	}
	var notify func()
	m.mutex.Lock()
	if m.flyDigiHID != dev {
		m.mutex.Unlock()
		return
	}
	if m.isConnected {
		notify = m.onDisconnect
	}
	m.isConnected = false
	m.deviceType = ""
	m.productID = 0
	m.flyDigiHID = nil
	m.flyDigiHIDStop = nil
	m.flyDigiHIDDone = nil
	m.currentFanData.Store(nil)
	m.mutex.Unlock()

	if notify != nil {
		notify()
	}
}

func (m *Manager) handleFlyDigiHIDRX(raw []byte) bool {
	m.recordDebugFrame("rx", types.DeviceTransportHID, raw)
	if fanData := parseFlyDigiFanData(raw); fanData != nil {
		m.currentFanData.Store(fanData)
		if m.onFanDataUpdate != nil {
			go m.onFanDataUpdate(fanData)
		}
		return true
	}
	return false
}

func (m *Manager) flyDigiHIDInfoLocked(productID uint16, path string) map[string]string {
	displayName := m.activeProfileDisplayNameLocked(flyDigiHIDModelName(productID))
	model := strings.TrimSpace(m.activeProfile.Model)
	if productID != 0 && m.activeProfile.ID == types.LegacyRPMProfileID {
		model = flyDigiHIDModelName(productID)
		displayName = "飞智（FlyDigi）" + model
	}
	if model == "" {
		model = flyDigiHIDModelName(productID)
	}
	if model == "" {
		model = displayName
	}
	return map[string]string{
		"manufacturer": "飞智（FlyDigi）",
		"product":      displayName,
		"serial":       path,
		"model":        model,
		"transport":    types.DeviceTransportHID,
		"endpoint":     path,
		"productId":    fmt.Sprintf("0x%04X", productID),
	}
}

func flyDigiHIDModelName(productID uint16) string {
	switch productID {
	case types.FlyDigiBS2ProductID:
		return "BS2"
	case types.FlyDigiBS2PROProductID:
		return "BS2PRO"
	case types.FlyDigiBS3ProductID:
		return "BS3"
	case types.FlyDigiBS3PROProductID:
		return "BS3PRO"
	default:
		return "FlyDigi HID"
	}
}

func flyDigiHIDProductIDsForProfile(profileID string) []uint16 {
	switch strings.TrimSpace(profileID) {
	case types.FlyDigiBS2ProfileID:
		return []uint16{types.FlyDigiBS2ProductID}
	case types.FlyDigiBS2PROProfileID:
		return []uint16{types.FlyDigiBS2PROProductID}
	case types.FlyDigiBS3ProfileID:
		return []uint16{types.FlyDigiBS3ProductID}
	case types.FlyDigiBS3PROProfileID:
		return []uint16{types.FlyDigiBS3PROProductID}
	default:
		return []uint16{
			types.FlyDigiBS2PROProductID,
			types.FlyDigiBS3ProductID,
			types.FlyDigiBS3PROProductID,
			types.FlyDigiBS2ProductID,
		}
	}
}

func flyDigiHIDProductIDFromPath(path string, productIDs []uint16) (uint16, bool) {
	lower := strings.ToLower(path)
	vendor := fmt.Sprintf("%04x", types.FlyDigiHIDVendorID)
	vendorMatched := strings.Contains(lower, "vid_"+vendor) ||
		strings.Contains(lower, "vid&"+vendor) ||
		strings.Contains(lower, "vid&01"+vendor) ||
		strings.Contains(lower, "dev_vid&"+vendor) ||
		strings.Contains(lower, "dev_vid&01"+vendor)
	if !vendorMatched {
		return 0, false
	}
	for _, id := range productIDs {
		pid := fmt.Sprintf("%04x", id)
		if strings.Contains(lower, "pid_"+pid) || strings.Contains(lower, "pid&"+pid) {
			return id, true
		}
	}
	return 0, false
}

func padFlyDigiHIDReport(report []byte, size int) []byte {
	if len(report) >= size {
		return report
	}
	padded := make([]byte, size)
	copy(padded, report)
	return padded
}

func (m *Manager) writeFlyDigiHIDFrameLocked(cmd byte, payload []byte, reportLen int) error {
	if m.writesBlocked.Load() {
		return fmt.Errorf("device writes are blocked during system suspend")
	}
	if m.flyDigiHID == nil {
		return fmt.Errorf("飞智 HID 设备未连接")
	}
	frame := deviceproto.BuildFrame(cmd, payload...)
	report := deviceproto.BuildReport(frame, reportLen)
	return retryDeviceSend(fmt.Sprintf("FlyDigi HID command 0x%02X", cmd), func() error {
		m.recordDebugFrame("tx", types.DeviceTransportHID, report)
		return m.flyDigiHID.WriteReport(report, 800*time.Millisecond)
	})
}

func (m *Manager) setFlyDigiHIDTargetSpeedLocked(speed types.FanSpeedValue) bool {
	if !m.isConnected || m.flyDigiHID == nil {
		return false
	}
	speed = speed.Normalized()
	if !types.IsRPMSpeedUnit(speed.Unit) {
		m.logWarn("飞智 HID 设备不支持百分比直控")
		return false
	}
	rpm := types.ClampRPM(speed.Value)
	if rpm > types.DefaultMaxFanRPM {
		rpm = types.DefaultMaxFanRPM
	}
	requestedRPM := rpm
	capability := m.flyDigiHIDRuntimeCapabilityLocked()
	if cappedRPM, limited := types.FlyDigiClampRPMForCapability(rpm, capability); limited {
		rpm = cappedRPM
		m.logWarn("飞智 HID 当前供电/挡位上限为 %s %d RPM，自动目标从 %d RPM 限制为 %d RPM", capability.MaxGearLabel, capability.MaxRPM, requestedRPM, rpm)
	}
	if err := m.writeFlyDigiHIDFrameLocked(deviceproto.CmdEnterRealtimeRPM, nil, hidControlReportLen); err != nil {
		m.logError("进入实时转速模式失败: %v", err)
		return false
	}
	time.Sleep(50 * time.Millisecond)

	payload := make([]byte, 2)
	binary.LittleEndian.PutUint16(payload, uint16(rpm))
	if err := m.writeFlyDigiHIDFrameLocked(deviceproto.CmdSetRealtimeRPM, payload, hidControlReportLen); err != nil {
		m.logError("设置飞智 HID 转速失败: %v", err)
		return false
	}
	m.storeFlyDigiHIDFanDataLocked(rpm, "自动模式(实时转速)")
	return true
}

func (m *Manager) flyDigiHIDRuntimeCapabilityLocked() types.FlyDigiRuntimeCapability {
	fanData := m.currentFanData.Load()
	if fanData == nil || fanData.Transport != types.DeviceTransportHID {
		return types.FlyDigiRuntimeCapability{Reason: "missingFanData"}
	}
	if fanData.FlyDigiCapability != nil {
		return *fanData.FlyDigiCapability
	}
	return types.DecodeFlyDigiRuntimeCapability(fanData, nil)
}

func (m *Manager) setFlyDigiHIDManualGearRPM(gear, level string, rpm int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.writesBlocked.Load() || !m.isConnected || m.flyDigiHID == nil {
		return false
	}
	idx, ok := types.GearIndex(gear)
	if !ok {
		m.logError("未知挡位 %s", gear)
		return false
	}
	rpm = types.ClampRPM(rpm)
	capability := m.flyDigiHIDRuntimeCapabilityLocked()
	if !types.FlyDigiIsGearAllowed(gear, capability) {
		m.logWarn("飞智 HID 当前供电/挡位上限为 %s，未解锁 %s 档", capability.MaxGearLabel, gear)
		return false
	}
	requestedRPM := rpm
	if cappedRPM, limited := types.FlyDigiClampRPMForCapability(rpm, capability); limited {
		rpm = cappedRPM
		m.logWarn("飞智 HID 手动挡位 %s %s 请求 %d RPM 超过当前上限 %d RPM，已限制为 %d RPM", gear, level, requestedRPM, capability.MaxRPM, rpm)
	}
	cmd := types.BuildGearRPMCommand(idx, rpm)
	report := deviceproto.BuildReport(cmd, hidControlReportLen)
	if err := retryDeviceSend("FlyDigi HID manual gear", func() error {
		m.recordDebugFrame("tx", types.DeviceTransportHID, report)
		return m.flyDigiHID.WriteReport(report, 800*time.Millisecond)
	}); err != nil {
		m.logError("设置飞智 HID 挡位 %s %s (%d RPM) 失败: %v", gear, level, rpm, err)
		return false
	}
	m.storeFlyDigiHIDFanDataLocked(rpm, "挡位工作模式")
	return true
}

func (m *Manager) setFlyDigiHIDGearLight(enabled bool) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	payload := byte(0x00)
	if enabled {
		payload = 0x01
	}
	if err := m.writeFlyDigiHIDFrameLocked(deviceproto.CmdGearLight, []byte{payload}, hidControlReportLen); err != nil {
		m.logError("设置飞智 HID 挡位灯失败: %v", err)
		return false
	}
	return true
}

func (m *Manager) setFlyDigiHIDPowerOnStart(enabled bool) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	payload := byte(0x02)
	if enabled {
		payload = 0x01
	}
	if err := m.writeFlyDigiHIDFrameLocked(deviceproto.CmdSetPowerOnStart, []byte{payload}, hidControlReportLen); err != nil {
		m.logError("设置飞智 HID 通电自启失败: %v", err)
		return false
	}
	return true
}

func (m *Manager) setFlyDigiHIDSmartStartStop(mode string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	var payload byte
	switch mode {
	case "off":
		payload = 0x00
	case "immediate":
		payload = 0x01
	case "delayed":
		payload = 0x02
	default:
		return false
	}
	if err := m.writeFlyDigiHIDFrameLocked(deviceproto.CmdSetSmartStartStop, []byte{payload}, hidControlReportLen); err != nil {
		m.logError("设置飞智 HID 智能启停失败: %v", err)
		return false
	}
	return true
}

func (m *Manager) setFlyDigiHIDBrightness(percentage int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if percentage < 0 || percentage > 100 {
		return false
	}
	switch percentage {
	case 0:
		payload := []byte{0x1C, 0x00, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		if err := m.writeFlyDigiHIDFrameLocked(0x47, payload, hidControlReportLen); err != nil {
			m.logError("关闭飞智 HID 亮度失败: %v", err)
			return false
		}
	case 100:
		if err := m.writeFlyDigiHIDFrameLocked(0x43, nil, hidControlReportLen); err != nil {
			m.logError("设置飞智 HID 全亮失败: %v", err)
			return false
		}
	default:
		return false
	}
	return true
}

func (m *Manager) storeFlyDigiHIDFanDataLocked(rpm int, workMode string) {
	rpm = types.ClampRPM(rpm)
	fanData := &types.FanData{
		ReportID:    deviceproto.ReportID,
		MagicSync:   0x5AA5,
		Command:     deviceproto.CmdSetRealtimeRPM,
		CurrentRPM:  0,
		TargetRPM:   uint16(rpm),
		Transport:   types.DeviceTransportHID,
		SpeedUnit:   types.FanSpeedUnitRPM,
		WorkMode:    workMode,
		CurrentMode: 0x01,
	}
	if previous := m.currentFanData.Load(); previous != nil && previous.Transport == types.DeviceTransportHID {
		fanData = &types.FanData{
			ReportID:          previous.ReportID,
			MagicSync:         previous.MagicSync,
			Command:           deviceproto.CmdSetRealtimeRPM,
			Status:            previous.Status,
			GearSettings:      previous.GearSettings,
			CurrentMode:       previous.CurrentMode,
			Reserved1:         previous.Reserved1,
			CurrentRPM:        previous.CurrentRPM,
			TargetRPM:         uint16(rpm),
			MaxGear:           previous.MaxGear,
			SetGear:           previous.SetGear,
			WorkMode:          workMode,
			Transport:         types.DeviceTransportHID,
			SpeedUnit:         types.FanSpeedUnitRPM,
			FlyDigiCapability: previous.FlyDigiCapability,
		}
		if fanData.ReportID == 0 {
			fanData.ReportID = deviceproto.ReportID
		}
		if fanData.MagicSync == 0 {
			fanData.MagicSync = 0x5AA5
		}
	}
	if fanData.FlyDigiCapability == nil {
		capability := types.DecodeFlyDigiRuntimeCapability(fanData, nil)
		fanData.FlyDigiCapability = &capability
	}
	m.currentFanData.Store(fanData)
	if m.onFanDataUpdate != nil {
		go m.onFanDataUpdate(fanData)
	}
}
