package guiapp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/ipc"
)

// GetTemperature 获取当前温度
func (a *App) GetTemperature() TemperatureData {
	resp, err := a.sendRequest(ipc.ReqGetTemperature, nil)
	if err != nil {
		a.mutex.RLock()
		defer a.mutex.RUnlock()
		return a.currentTemp
	}
	var temp TemperatureData
	json.Unmarshal(resp.Data, &temp)
	return temp
}

// GetTemperatureHistory 获取核心服务记录的温度历史。
func (a *App) GetTemperatureHistory() TemperatureHistoryPayload {
	resp, err := a.sendRequest(ipc.ReqGetTemperatureHistory, nil)
	if err != nil || !resp.Success {
		return TemperatureHistoryPayload{}
	}
	var payload TemperatureHistoryPayload
	json.Unmarshal(resp.Data, &payload)
	return payload
}

// SetTemperatureHistoryEnabled 设置后台历史记录开关。
func (a *App) SetTemperatureHistoryEnabled(enabled bool) error {
	resp, err := a.sendRequest(ipc.ReqSetTemperatureHistoryEnabled, ipc.SetBoolParams{Enabled: enabled})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// SetTemperatureHistoryRetentionHours 设置后台历史保留时长(小时)。
func (a *App) SetTemperatureHistoryRetentionHours(hours int) error {
	resp, err := a.sendRequest(ipc.ReqSetTemperatureHistoryRetentionHours, ipc.SetIntParams{Value: hours})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// TestTemperatureReading 测试温度读取
func (a *App) TestTemperatureReading() TemperatureData {
	resp, err := a.sendRequest(ipc.ReqTestTemperatureReading, nil)
	if err != nil {
		return TemperatureData{}
	}
	var temp TemperatureData
	json.Unmarshal(resp.Data, &temp)
	return temp
}

// SetSensorCatalogActive 声明设置页传感器目录是否打开；开启时立即返回全量传感器，关闭或隐藏窗口时释放租约。
func (a *App) SetSensorCatalogActive(enabled bool) TemperatureData {
	if a == nil {
		return TemperatureData{}
	}
	a.sensorCatalogWanted.Store(enabled)
	return a.syncSensorCatalogLease(enabled)
}

func (a *App) syncSensorCatalogLease(enabled bool) TemperatureData {
	if a == nil || a.ipcClient == nil || a.shuttingDown.Load() {
		return TemperatureData{}
	}
	resp, err := a.sendRequest(ipc.ReqSetSensorCatalogActive, ipc.SetBoolParams{Enabled: enabled})
	if !enabled || err != nil || !resp.Success {
		return TemperatureData{}
	}
	var temp TemperatureData
	if err := json.Unmarshal(resp.Data, &temp); err != nil {
		return TemperatureData{}
	}
	if temp.UpdateTime > 0 {
		a.mutex.Lock()
		a.currentTemp = temp
		a.mutex.Unlock()
	}
	return temp
}

// ProbeGpuSensors 在低功耗保护下按需临时读取一次休眠独显的完整传感器列表。
func (a *App) ProbeGpuSensors() TemperatureData {
	if a == nil || a.ipcClient == nil || a.shuttingDown.Load() {
		return TemperatureData{}
	}
	resp, err := a.sendRequestWithTimeout(ipc.ReqProbeGpuSensors, nil, 15*time.Second)
	if err != nil || !resp.Success {
		return TemperatureData{}
	}
	var temp TemperatureData
	if err := json.Unmarshal(resp.Data, &temp); err != nil {
		return TemperatureData{}
	}
	if temp.UpdateTime > 0 {
		a.mutex.Lock()
		a.currentTemp = temp
		a.mutex.Unlock()
	}
	return temp
}
