//go:build !legacydevice

package device

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

const flyDigiDeviceQueryWait = 140 * time.Millisecond

var flyDigiDeviceSettingsQueryCommands = []byte{
	deviceproto.CmdQueryGearRPMTable,
	deviceproto.CmdQueryWorkMode,
	deviceproto.CmdRGBStatus,
}

func (m *Manager) queryFlyDigiHIDDeviceSettings() (types.DeviceSettings, error) {
	settings := types.DeviceSettings{
		Available: false,
		Source:    types.DeviceTransportHID,
		ReadAt:    time.Now().Format("2006-01-02 15:04:05"),
		Model:     m.GetModelName(),
	}

	m.mutex.RLock()
	connected := m.isConnected && m.flyDigiHID != nil
	m.mutex.RUnlock()
	if !connected {
		return settings, fmt.Errorf("设备未连接")
	}

	var lastErr error
	for _, cmd := range flyDigiDeviceSettingsQueryCommands {
		frames, err := m.queryFlyDigiHIDCommand(cmd)
		if err != nil {
			lastErr = err
			continue
		}
		settings.RawFrames = append(settings.RawFrames, frames...)
		applyFlyDigiDeviceSettingsFrames(&settings, frames)
	}
	applyFlyDigiCurrentStatus(&settings, m.GetCurrentFanData())
	settings.Available = len(settings.GearRPMTable) > 0 || settings.WorkMode != "" || settings.Status != nil
	return settings, lastErr
}

func (m *Manager) queryFlyDigiHIDCommand(cmd byte) ([]types.DeviceDebugFrame, error) {
	startSeq := m.currentDebugSeq()
	m.mutex.Lock()
	if !m.isConnected || m.flyDigiHID == nil {
		m.mutex.Unlock()
		return nil, fmt.Errorf("设备未连接")
	}
	if err := m.writeFlyDigiHIDFrameLocked(cmd, nil, hidControlReportLen); err != nil {
		m.mutex.Unlock()
		return nil, err
	}
	dev := m.flyDigiHID
	m.mutex.Unlock()

	time.Sleep(flyDigiDeviceQueryWait)
	m.readFlyDigiHIDFrames(dev, 2, 120*time.Millisecond)
	return m.debugFramesAfter(startSeq), nil
}

func (m *Manager) sendFlyDigiHIDDebugCommand(input string, waitMs int) (types.DeviceDebugCommandResult, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return types.DeviceDebugCommandResult{}, fmt.Errorf("raw debug command is empty")
	}

	frame, err := deviceproto.NormalizeDebugInput(trimmed)
	if err != nil {
		return types.DeviceDebugCommandResult{}, err
	}
	report := deviceproto.BuildReport(frame, hidControlReportLen)
	startSeq := m.currentDebugSeq()

	m.mutex.Lock()
	if !m.isConnected || m.flyDigiHID == nil {
		m.mutex.Unlock()
		return types.DeviceDebugCommandResult{}, fmt.Errorf("device is not connected")
	}
	dev := m.flyDigiHID
	m.recordDebugFrame("tx", types.DeviceTransportHID, report)
	err = dev.WriteReport(report, 800*time.Millisecond)
	m.mutex.Unlock()
	if err != nil {
		return types.DeviceDebugCommandResult{}, err
	}

	if waitMs > 0 {
		time.Sleep(time.Duration(waitMs) * time.Millisecond)
	}
	m.readFlyDigiHIDFrames(dev, 4, 80*time.Millisecond)

	return types.DeviceDebugCommandResult{
		Transport: types.DeviceTransportHID,
		InputHex:  input,
		FrameHex:  deviceproto.Hex(frame),
		RawHex:    deviceproto.Hex(report),
		WaitMs:    waitMs,
		Frames:    m.debugFramesAfter(startSeq),
	}, nil
}

func (m *Manager) readFlyDigiHIDFrames(dev *flyDigiHIDDevice, maxFrames int, timeout time.Duration) {
	if dev == nil || maxFrames <= 0 {
		return
	}
	for range maxFrames {
		raw, err := dev.ReadReport(timeout)
		if err != nil {
			return
		}
		if len(raw) == 0 {
			return
		}
		m.recordDebugFrame("rx", types.DeviceTransportHID, raw)
		if fanData := parseFlyDigiFanData(raw); fanData != nil {
			m.currentFanData.Store(fanData)
			if m.onFanDataUpdate != nil {
				go m.onFanDataUpdate(fanData)
			}
		}
	}
}

func parseFlyDigiFanData(data []byte) *types.FanData {
	frame, ok := deviceproto.ParseFrame(data)
	if !ok || frame.Command != deviceproto.CmdStatusNotify || len(frame.Payload) < 7 {
		return nil
	}
	mode := frame.Payload[1]
	currentRPM := binary.LittleEndian.Uint16(frame.Payload[3:5])
	targetRPM := binary.LittleEndian.Uint16(frame.Payload[5:7])
	maxGear, setGear := parseFlyDigiGearSettings(frame.Payload[0])
	return &types.FanData{
		ReportID:     frame.ReportID,
		MagicSync:    0x5AA5,
		Command:      frame.Command,
		Status:       frame.Length,
		GearSettings: frame.Payload[0],
		CurrentMode:  mode,
		CurrentRPM:   currentRPM,
		TargetRPM:    targetRPM,
		MaxGear:      maxGear,
		SetGear:      setGear,
		WorkMode:     deviceproto.ModeName(mode),
		Transport:    types.DeviceTransportHID,
		SpeedUnit:    types.FanSpeedUnitRPM,
	}
}

func parseFlyDigiGearSettings(value byte) (maxGear, selected string) {
	maxCode := (value >> 4) & 0x0F
	selectedCode := value & 0x0F
	switch maxCode {
	case 0x2:
		maxGear = "标准"
	case 0x4:
		maxGear = "强劲"
	case 0x6:
		maxGear = "超频"
	default:
		maxGear = fmt.Sprintf("未知(0x%X)", maxCode)
	}
	switch selectedCode {
	case 0x8:
		selected = "静音"
	case 0xA:
		selected = "标准"
	case 0xC:
		selected = "强劲"
	case 0xE:
		selected = "超频"
	default:
		selected = fmt.Sprintf("未知(0x%X)", selectedCode)
	}
	return maxGear, selected
}

func applyFlyDigiDeviceSettingsFrames(settings *types.DeviceSettings, frames []types.DeviceDebugFrame) {
	for _, debugFrame := range frames {
		if debugFrame.Direction != "rx" || debugFrame.FrameHex == "" {
			continue
		}
		raw, err := deviceproto.ParseHex(debugFrame.FrameHex)
		if err != nil {
			continue
		}
		frame, ok := deviceproto.ParseFrame(raw)
		if !ok || !frame.ChecksumOK {
			continue
		}
		applyFlyDigiDecodedDeviceSetting(settings, deviceproto.DecodeFrame(frame))
	}
}

func applyFlyDigiDecodedDeviceSetting(settings *types.DeviceSettings, decoded deviceproto.DecodedFrame) {
	switch decoded.Type {
	case "gearRpmTable":
		settings.GearRPMTable = make([]types.DeviceGearRPM, 0, len(decoded.GearTable))
		for _, item := range decoded.GearTable {
			settings.GearRPMTable = append(settings.GearRPMTable, types.DeviceGearRPM{
				Gear:  item.Gear,
				Label: item.Label,
				RPM:   item.RPM,
			})
		}
	case "workMode":
		settings.WorkMode = decoded.Mode
		settings.WorkModeName = decoded.ModeName
	case "rgbStatus":
		settings.RGBState = decoded.RGBState
		settings.RGBStateName = decoded.RGBName
	case "statusNotification":
		settings.Status = &types.DeviceStatusRead{
			GearSetting:        decoded.GearSetting,
			MaxGear:            decoded.MaxGear,
			Selected:           decoded.Selected,
			Mode:               decoded.Mode,
			ModeName:           decoded.ModeName,
			SmartStartStop:     decoded.SmartStartStop,
			SmartStartStopName: decoded.SmartStartStopName,
			CurrentRPM:         decoded.CurrentRPM,
			TargetRPM:          decoded.TargetRPM,
		}
	}
}

func applyFlyDigiCurrentStatus(settings *types.DeviceSettings, fanData *types.FanData) {
	if fanData == nil {
		return
	}
	if settings.WorkMode == "" {
		settings.WorkMode = fmt.Sprintf("0x%02X", fanData.CurrentMode)
		settings.WorkModeName = deviceproto.ModeName(fanData.CurrentMode)
	}
	maxGear, selected := deviceproto.DecodeGearSetting(fanData.GearSettings)
	smartCode, smartName := deviceproto.DecodeSmartStartStop(fanData.CurrentMode)
	settings.Status = &types.DeviceStatusRead{
		GearSetting:        fmt.Sprintf("0x%02X", fanData.GearSettings),
		MaxGear:            maxGear,
		Selected:           selected,
		Mode:               fmt.Sprintf("0x%02X", fanData.CurrentMode),
		ModeName:           deviceproto.ModeName(fanData.CurrentMode),
		SmartStartStop:     smartCode,
		SmartStartStopName: smartName,
		CurrentRPM:         int(fanData.CurrentRPM),
		TargetRPM:          int(fanData.TargetRPM),
	}
}
