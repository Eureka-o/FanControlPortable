//go:build legacydevice

package device

import (
	"fmt"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

const (
	hidControlReportLen = 23
	hidLightReportLen   = 65
)

func DebugCommandPresets() []types.DeviceDebugCommandPreset {
	return []types.DeviceDebugCommandPreset{}
}

func (m *Manager) writeHIDFrameLocked(cmd byte, payload []byte, reportLen int) error {
	frame := deviceproto.BuildFrame(cmd, payload...)
	report := deviceproto.BuildReport(frame, reportLen)
	m.recordDebugFrame("tx", types.DeviceTypeHID, report)
	_, err := m.device.Write(report)
	return err
}

func (m *Manager) currentDebugSeq() uint64 {
	m.debugMutex.Lock()
	defer m.debugMutex.Unlock()
	return m.debugSeq
}

func (m *Manager) recordDebugFrame(direction, transport string, raw []byte) uint64 {
	debugFrame := newDeviceDebugFrame(direction, transport, raw)
	m.debugMutex.Lock()
	defer m.debugMutex.Unlock()
	return appendBoundedDebugFrame(&m.debugSeq, &m.debugFrames, debugFrame)
}

func (m *Manager) debugFramesAfter(seq uint64) []types.DeviceDebugFrame {
	m.debugMutex.Lock()
	defer m.debugMutex.Unlock()
	return debugFramesAfterSeq(m.debugFrames, seq)
}

func (m *Manager) GetDebugFrames() []types.DeviceDebugFrame {
	if m.GetDeviceType() == types.DeviceTypeBLE {
		return m.bleManager.GetDebugFrames()
	}
	m.debugMutex.Lock()
	defer m.debugMutex.Unlock()
	return cloneDebugFrames(m.debugFrames)
}

func (m *Manager) SendDebugCommand(input string, waitMs int) (types.DeviceDebugCommandResult, error) {
	if waitMs < 0 {
		waitMs = 0
	}
	if waitMs > 5000 {
		waitMs = 5000
	}

	if m.GetDeviceType() == types.DeviceTypeBLE {
		return m.bleManager.SendDebugCommand(input, waitMs)
	}
	if m.GetDeviceType() == types.DeviceTransportWiFi {
		return types.DeviceDebugCommandResult{
			Transport: types.DeviceTransportWiFi,
			InputHex:  input,
			WaitMs:    waitMs,
			Frames:    m.GetDebugFrames(),
		}, fmt.Errorf("WiFi 控制器不支持原始协议调试命令")
	}

	frame, err := deviceproto.NormalizeDebugInput(input)
	if err != nil {
		return types.DeviceDebugCommandResult{}, err
	}

	startSeq := m.currentDebugSeq()
	m.mutex.Lock()
	if !m.isConnected || m.device == nil {
		m.mutex.Unlock()
		return types.DeviceDebugCommandResult{}, fmt.Errorf("device is not connected")
	}
	report := deviceproto.BuildReport(frame, hidControlReportLen)
	m.recordDebugFrame("tx", types.DeviceTypeHID, report)
	_, err = m.device.Write(report)
	m.mutex.Unlock()
	if err != nil {
		return types.DeviceDebugCommandResult{}, err
	}

	if waitMs > 0 {
		time.Sleep(time.Duration(waitMs) * time.Millisecond)
	}

	return types.DeviceDebugCommandResult{
		Transport: types.DeviceTypeHID,
		InputHex:  input,
		FrameHex:  deviceproto.Hex(frame),
		RawHex:    deviceproto.Hex(report),
		WaitMs:    waitMs,
		Frames:    m.debugFramesAfter(startSeq),
	}, nil
}

func (b *BLEManager) recordDebugFrame(direction, transport string, raw []byte) uint64 {
	debugFrame := newDeviceDebugFrame(direction, transport, raw)
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return appendBoundedDebugFrame(&b.debugSeq, &b.debugFrames, debugFrame)
}

func (b *BLEManager) currentDebugSeq() uint64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.debugSeq
}

func (b *BLEManager) debugFramesAfter(seq uint64) []types.DeviceDebugFrame {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return debugFramesAfterSeq(b.debugFrames, seq)
}

func (b *BLEManager) GetDebugFrames() []types.DeviceDebugFrame {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return cloneDebugFrames(b.debugFrames)
}

func (b *BLEManager) SendDebugCommand(input string, waitMs int) (types.DeviceDebugCommandResult, error) {
	if waitMs < 0 {
		waitMs = 0
	}
	if waitMs > 5000 {
		waitMs = 5000
	}

	frame, err := deviceproto.NormalizeDebugInput(input)
	if err != nil {
		return types.DeviceDebugCommandResult{}, err
	}

	startSeq := b.currentDebugSeq()
	if err := b.WriteCommand(frame); err != nil {
		return types.DeviceDebugCommandResult{}, err
	}
	if waitMs > 0 {
		time.Sleep(time.Duration(waitMs) * time.Millisecond)
	}

	return types.DeviceDebugCommandResult{
		Transport: types.DeviceTypeBLE,
		InputHex:  input,
		FrameHex:  deviceproto.Hex(frame),
		RawHex:    deviceproto.Hex(frame),
		WaitMs:    waitMs,
		Frames:    b.debugFramesAfter(startSeq),
	}, nil
}
