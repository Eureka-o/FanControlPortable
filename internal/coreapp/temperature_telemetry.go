package coreapp

import (
	"strings"

	"github.com/TIANLI0/THRM/internal/types"
)

func trackBridgeTemperatureStaleness(temp types.TemperatureData, lastUpdate int64, staleCount int) (int64, int, bool) {
	if !temp.BridgeOk || temp.UpdateTime <= 0 {
		return 0, 0, false
	}
	if temp.UpdateTime != lastUpdate {
		return temp.UpdateTime, 0, false
	}
	staleCount++
	return lastUpdate, staleCount, staleCount >= staleBridgeUpdateThreshold
}

func shouldRestartTemperatureBridge(temp types.TemperatureData) bool {
	if temp.BridgeOk {
		return false
	}

	msg := strings.ToLower(strings.TrimSpace(temp.BridgeMsg))
	if msg == "" {
		return true
	}
	if strings.Contains(msg, "[msr-unavailable]") {
		return false
	}

	restartHints := []string{
		"启动桥接程序失败",
		"桥接程序通信失败",
		"桥接程序未连接",
		"连接管道失败",
		"发送命令失败",
		"读取响应失败",
		"等待桥接程序启动超时",
		"未能获取管道名称",
		"pipe",
		"eof",
		"broken",
		"closed",
	}
	for _, hint := range restartHints {
		if strings.Contains(msg, strings.ToLower(hint)) {
			return true
		}
	}

	// 休眠恢复后硬件监控库偶尔会返回全 0 但进程仍能响应，重启桥接可重新初始化底层传感器。
	return temp.CPUTemp == 0 && temp.GPUTemp == 0
}

func compactTemperatureEventPayload(current, previous types.TemperatureData) types.TemperatureData {
	compact := current
	if temperatureSensorsSameIdentity(current.CpuSensors, previous.CpuSensors) {
		compact.CpuSensors = nil
	}
	if temperatureSensorsSameIdentity(current.GpuSensors, previous.GpuSensors) {
		compact.GpuSensors = nil
	}
	if powerSensorsSameIdentity(current.CpuPowerSensors, previous.CpuPowerSensors) {
		compact.CpuPowerSensors = nil
	}
	if powerSensorsSameIdentity(current.GpuPowerSensors, previous.GpuPowerSensors) {
		compact.GpuPowerSensors = nil
	}
	if gpuDevicesSameIdentity(current.GpuDevices, previous.GpuDevices) {
		compact.GpuDevices = nil
	}
	return compact
}

func mergeTemperatureHardwareMetadata(previous, incoming types.TemperatureData) types.TemperatureData {
	if incoming.CpuModel == "" {
		incoming.CpuModel = previous.CpuModel
	}
	if incoming.GpuModel == "" {
		incoming.GpuModel = previous.GpuModel
	}
	if incoming.CpuSensors == nil {
		incoming.CpuSensors = previous.CpuSensors
	}
	if incoming.GpuSensors == nil || (incoming.GPUReadState == types.GPUReadStateNotPolled && len(incoming.GpuSensors) == 0) {
		incoming.GpuSensors = previous.GpuSensors
	}
	if incoming.CpuPowerSensors == nil {
		incoming.CpuPowerSensors = previous.CpuPowerSensors
	}
	if incoming.GpuPowerSensors == nil || (incoming.GPUReadState == types.GPUReadStateNotPolled && len(incoming.GpuPowerSensors) == 0) {
		incoming.GpuPowerSensors = previous.GpuPowerSensors
	}
	if incoming.GpuDevices == nil || (incoming.GPUReadState == types.GPUReadStateNotPolled && len(incoming.GpuDevices) == 0) {
		incoming.GpuDevices = previous.GpuDevices
	}
	return incoming
}

func temperatureSensorsSameIdentity(current, previous []types.TemperatureSensor) bool {
	if (current == nil) != (previous == nil) {
		return false
	}
	if len(current) != len(previous) {
		return false
	}
	for i := range current {
		if current[i].Key != previous[i].Key || current[i].Name != previous[i].Name {
			return false
		}
	}
	return true
}

func powerSensorsSameIdentity(current, previous []types.PowerSensor) bool {
	if (current == nil) != (previous == nil) {
		return false
	}
	if len(current) != len(previous) {
		return false
	}
	for i := range current {
		if current[i].Key != previous[i].Key || current[i].Name != previous[i].Name {
			return false
		}
	}
	return true
}

func gpuDevicesSameIdentity(current, previous []types.TemperatureGPUDevice) bool {
	if (current == nil) != (previous == nil) {
		return false
	}
	if len(current) != len(previous) {
		return false
	}
	for i := range current {
		if current[i].Key != previous[i].Key ||
			current[i].Name != previous[i].Name ||
			current[i].Vendor != previous[i].Vendor ||
			!temperatureSensorsSameIdentity(current[i].Sensors, previous[i].Sensors) ||
			!powerSensorsSameIdentity(current[i].PowerSensors, previous[i].PowerSensors) {
			return false
		}
	}
	return true
}
