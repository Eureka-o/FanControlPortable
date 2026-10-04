//go:build !legacydevice && windows

package device

import (
	"fmt"
	"strings"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

const blackSharkHIDReportLen = 65

func (m *Manager) connectBlackSharkHIDLocked() (bool, map[string]string) {
	dev, err := openHIDDevice(types.BlackSharkHIDVendorID, []uint16{types.BlackSharkBRB02HIDProductID})
	if err != nil {
		m.logWarn("黑鲨 HID 设备连接失败: %v", err)
		m.isConnected = false
		m.deviceType = ""
		m.productID = 0
		m.flyDigiHID = nil
		m.currentFanData.Store(nil)
		return false, nil
	}

	m.flyDigiHID = dev
	m.productID = dev.productID
	m.deviceType = types.DeviceTransportHID
	generation := m.connectionGen.Add(1)
	ready := m.startFlyDigiHIDReaderLocked(dev, generation)
	if !waitForFlyDigiHIDReady(ready, flyDigiHIDReadyTimeout) {
		m.logWarn("黑鲨 HID 已打开，但在 %s 内未收到状态帧", flyDigiHIDReadyTimeout)
		m.closeFlyDigiHIDLocked()
		m.isConnected = false
		m.deviceType = ""
		m.productID = 0
		m.currentFanData.Store(nil)
		return false, nil
	}
	m.isConnected = true
	info := m.blackSharkHIDInfoLocked(dev.path)
	m.logInfo("黑鲨 HID 设备连接成功: %s", info["product"])
	return true, info
}

func (m *Manager) blackSharkHIDInfoLocked(path string) map[string]string {
	return map[string]string{
		"manufacturer": types.BlackSharkBRB02Vendor,
		"product":      types.BlackSharkBRB02DisplayName,
		"serial":       path,
		"model":        types.BlackSharkBRB02DisplayName,
		"transport":    types.DeviceTransportHID,
		"endpoint":     path,
		"productId":    fmt.Sprintf("0x%04X", types.BlackSharkBRB02HIDProductID),
		"profileId":    types.BlackSharkBRB02HIDProfileID,
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
	fanData := parseBlackSharkHIDFanData(raw)
	if fanData == nil || m.connectionGen.Load() != generation {
		return false
	}
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
	if m.writesBlocked.Load() {
		return fmt.Errorf("device writes are blocked during system suspend")
	}
	if m.flyDigiHID == nil {
		return fmt.Errorf("黑鲨 HID 设备未连接")
	}
	report := blackSharkHIDReport(frame)
	m.recordDebugFrame("tx", types.DeviceTransportHID, report)
	return retryDeviceSend("Black Shark HID command", func() error {
		return m.flyDigiHID.WriteReport(report, 800*time.Millisecond)
	})
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
		WorkMode: "自动模式(实时转速)", Transport: types.DeviceTransportHID,
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
		Source:    types.DeviceTransportHID,
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
