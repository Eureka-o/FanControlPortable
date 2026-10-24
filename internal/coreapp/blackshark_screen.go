package coreapp

import (
	"fmt"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func (a *CoreApp) SetBlackSharkScreenRefreshInterval(intervalMs int) error {
	if a.deviceManager == nil || !a.deviceManager.IsConnected() {
		return fmt.Errorf("黑鲨设备未连接")
	}
	profile := a.deviceManager.ActiveProfile()
	transport := types.NormalizeDeviceTransport(profile.Transport)
	if profile.ID == types.BlackSharkBRB02WiFiProfileID && transport == types.DeviceTransportWiFi {
		return a.deviceManager.SetBlackSharkWiFiHostInfoRefreshInterval(types.NormalizeBlackSharkScreenRefreshIntervalMs(intervalMs))
	}
	return a.deviceManager.SetBlackSharkHostInfoRefreshInterval(types.NormalizeBlackSharkScreenRefreshIntervalMs(intervalMs))
}

// SendBlackSharkScreenParameters sends the latest selected parameter values once.
func (a *CoreApp) SendBlackSharkScreenParameters() error {
	if a.deviceManager == nil || !a.deviceManager.IsConnected() {
		return fmt.Errorf("黑鲨设备未连接")
	}
	profile := a.deviceManager.ActiveProfile()
	transport := types.NormalizeDeviceTransport(profile.Transport)
	isWiFiMock := profile.ID == types.BlackSharkBRB02WiFiProfileID && transport == types.DeviceTransportWiFi
	if profile.ID != types.BlackSharkBRB02USBProfileID && !isWiFiMock {
		return fmt.Errorf("黑鲨屏幕参数传输仅支持 USB 有线模式或黑鲨 WiFi mock")
	}
	a.mutex.RLock()
	temp := a.currentTemp
	a.mutex.RUnlock()
	cfg := a.configManager.Get()
	a.deviceManager.SetBlackSharkHostInfoFrame(
		deviceproto.BuildBlackSharkHostInfo(blackSharkHostInfoEntries(cfg, temp, time.Now())),
	)
	var err error
	if isWiFiMock {
		err = a.deviceManager.SendBlackSharkWiFiHostInfoFrame()
	} else {
		err = a.deviceManager.SendBlackSharkHostInfoFrame()
	}
	if err != nil {
		return err
	}
	if isWiFiMock {
		return a.deviceManager.SetBlackSharkWiFiHostInfoRefreshInterval(cfg.BlackSharkScreenRefreshIntervalMs)
	}
	return a.SetBlackSharkScreenRefreshInterval(cfg.BlackSharkScreenRefreshIntervalMs)
}

// blackSharkHostInfoEntries 把主机遥测映射成 0x07 参数页推送的 7 项。
// 槽位语义经 Alp 工具箱在实机屏幕上对号确认 (2026-10-03):
// 00=CPU 温度;01=GPU 温度 (屏幕 GPU 槽渲染此项, 不是官方抓包初判的 CPU 功耗);
// 02=GPU 功耗 W;03=未明 (官方实测 0-13 跳动, 恒发 0);05=未明 (官方恒 74, 原样回放);
// 06=GPU 热点温度 (FanControl 无该读数, 回退 GPU 温度);
// 07=当日分钟数 —— 屏幕按 值/60:值%60 渲染为 HH:MM (误发 HHMM 会显示 PM 33:58)。
func blackSharkHostInfoEntries(cfg types.AppConfig, temp types.TemperatureData, now time.Time) []deviceproto.BlackSharkHostInfoEntry {
	cpuTemp := 0
	if temp.CPUTemp > 1 {
		cpuTemp = temp.CPUTemp
	}
	gpuTemp := 0
	if temp.GPUTemp > 1 {
		gpuTemp = temp.GPUTemp
	}
	gpuPower := 0
	if temp.GPUPowerWatts > 1 {
		gpuPower = int(temp.GPUPowerWatts + 0.5)
	}
	selected := make(map[string]bool, len(cfg.BlackSharkScreenItems))
	for _, item := range types.NormalizeBlackSharkScreenItems(cfg.BlackSharkScreenItems) {
		selected[item] = true
	}
	entries := make([]deviceproto.BlackSharkHostInfoEntry, 0, 7)
	if selected[types.BlackSharkScreenItemCPUTemperature] {
		entries = append(entries, deviceproto.BlackSharkHostInfoEntry{ID: 0x00, Value: uint16(cpuTemp)})
	}
	if selected[types.BlackSharkScreenItemGPUTemperature] {
		entries = append(entries, deviceproto.BlackSharkHostInfoEntry{ID: 0x01, Value: uint16(gpuTemp)})
	}
	if selected[types.BlackSharkScreenItemGPUPower] {
		entries = append(entries, deviceproto.BlackSharkHostInfoEntry{ID: 0x02, Value: uint16(gpuPower)})
	}
	entries = append(entries,
		deviceproto.BlackSharkHostInfoEntry{ID: 0x03, Value: 0},
		deviceproto.BlackSharkHostInfoEntry{ID: 0x05, Value: 74},
		deviceproto.BlackSharkHostInfoEntry{ID: 0x06, Value: uint16(gpuTemp)},
	)
	if selected[types.BlackSharkScreenItemTime] {
		entries = append(entries, deviceproto.BlackSharkHostInfoEntry{ID: 0x07, Value: uint16(now.Hour()*60 + now.Minute())})
	}
	return entries
}
