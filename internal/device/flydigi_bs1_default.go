//go:build !legacydevice

package device

import (
	"github.com/TIANLI0/THRM/internal/deviceproto"
)

func (m *Manager) setFlyDigiBS1PowerOnStart(enabled bool) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.writesBlocked.Load() || !m.isConnected || m.bleExecutor == nil {
		return false
	}
	payload := byte(0x02)
	if enabled {
		payload = 0x01
	}
	if err := m.bleExecutor.WriteRaw(nil, deviceproto.BuildFrame(deviceproto.CmdSetPowerOnStart, payload)); err != nil {
		m.logError("设置飞智 BS1 通电自启失败: %v", err)
		return false
	}
	return true
}
