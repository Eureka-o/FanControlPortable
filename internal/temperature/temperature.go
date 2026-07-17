// Package temperature 提供温度读取功能
package temperature

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
	"github.com/shirou/gopsutil/v4/sensors"
)

const (
	helperCommandTimeout         = 1200 * time.Millisecond
	gpuVendorCacheTTL            = 30 * time.Second
	bridgeRecoveryTemperatureTTL = 20 * time.Second
)

var (
	execHelperCommand = execCommandHiddenWithTimeout
	readTimeNow       = time.Now
)

type bridgeTemperatureProvider interface {
	GetTemperature(types.TemperatureSelection) types.BridgeTemperatureData
}

// Reader 温度读取器
type Reader struct {
	bridgeManager bridgeTemperatureProvider
	logger        types.Logger

	cacheMutex      sync.RWMutex
	cachedGPUVendor string
	cachedVendorAt  time.Time

	lastGoodBridgeTemp      types.TemperatureData
	lastGoodBridgeSelection types.TemperatureSelection
	lastGoodBridgeAt        time.Time
}

// NewReader 创建新的温度读取器
func NewReader(bridgeManager bridgeTemperatureProvider, logger types.Logger) *Reader {
	return &Reader{
		bridgeManager: bridgeManager,
		logger:        logger,
	}
}

// Read 读取温度
func (r *Reader) Read(selection types.TemperatureSelection) (temp types.TemperatureData) {
	selection = types.NormalizeTemperatureSelection(selection)
	temp = types.TemperatureData{
		UpdateTime:    time.Now().UnixMilli(),
		BridgeOk:      true,
		ControlSource: selection.TempSource,
	}
	defer func() { temp.TelemetryState = telemetryStateFor(temp) }()

	// 优先使用桥接程序读取温度
	bridgeTemp := r.bridgeManager.GetTemperature(selection)
	copyBridgeTemperatureMetadata(&temp, bridgeTemp, selection)
	gpuNotPolled := temp.GPUReadState == types.GPUReadStateNotPolled
	if bridgeTemp.Success {
		if bridgeTemp.CpuTemp == 0 && bridgeTemp.GpuTemp == 0 {
			temp.BridgeOk = false
			temp.BridgeMsg = "桥接程序返回空温度（CPU/GPU 均为 0），已尝试备用读取；可重新初始化温度监控或检查 PawnIO/其它硬件监控工具。"
			r.logger.Warn("桥接程序返回空温度数据，使用备用方法")

			temp.CPUTemp = r.readCPUTemperature()
			if !gpuNotPolled {
				temp.GPUTemp = r.readGPUTemperature()
				if temp.GPUTemp > 0 && temp.GPUReadState == "" {
					temp.GPUReadState = types.GPUReadStateActive
				}
			}
			temp.MaxTemp = max(temp.CPUTemp, temp.GPUTemp)
			temp.ControlTemp = resolveControlTemp(temp.CPUTemp, temp.GPUTemp, selection.TempSource)
			return temp
		}

		temp.BridgeOk = true
		temp.BridgeMsg = ""
		temp.TelemetryFresh = true
		r.storeLastGoodBridgeTemperature(selection, temp)
		return temp
	}

	// 如果桥接程序失败，使用备用方法
	r.logger.Warn("桥接程序读取温度失败: %s, 使用备用方法", bridgeTemp.Error)
	if cached, ok := r.lastGoodBridgeTemperature(selection); ok {
		r.logger.Warn("TempBridge read failed, using last valid temperature: %s", bridgeTemp.Error)
		cached.UpdateTime = time.Now().UnixMilli()
		cached.BridgeOk = true
		cached.BridgeMsg = ""
		cached.TelemetryFresh = false
		return cached
	}

	temp.BridgeOk = false
	temp.BridgeMsg = bridgeTemp.Error
	if strings.TrimSpace(temp.BridgeMsg) == "" {
		temp.BridgeMsg = "CPU/GPU 温度读取失败，可重新初始化温度监控；若 CPU 仍为空，请安装/更新 PawnIO 或关闭其它硬件监控工具。"
	}

	// 读取CPU温度
	temp.CPUTemp = r.readCPUTemperature()

	// 读取GPU温度
	if !gpuNotPolled {
		// GPU fallback may wake a sleeping discrete GPU; keep it behind the bridge state.
		temp.GPUTemp = r.readGPUTemperature()
		if temp.GPUTemp > 0 && temp.GPUReadState == "" {
			temp.GPUReadState = types.GPUReadStateActive
		}
	}

	// 计算最高温度
	temp.MaxTemp = max(temp.CPUTemp, temp.GPUTemp)
	temp.ControlTemp = resolveControlTemp(temp.CPUTemp, temp.GPUTemp, selection.TempSource)

	return temp
}

func telemetryStateFor(temp types.TemperatureData) string {
	if !temp.BridgeOk || temp.ControlTemp <= 0 {
		return types.TelemetryStateUnavailable
	}
	if temp.TelemetryFresh {
		return types.TelemetryStateFresh
	}
	return types.TelemetryStateDelayed
}

func (r *Reader) storeLastGoodBridgeTemperature(selection types.TemperatureSelection, temp types.TemperatureData) {
	if temp.CPUTemp <= 0 && temp.GPUTemp <= 0 && temp.ControlTemp <= 0 {
		return
	}
	r.cacheMutex.Lock()
	defer r.cacheMutex.Unlock()
	r.lastGoodBridgeTemp = temp
	r.lastGoodBridgeSelection = selection
	r.lastGoodBridgeAt = readTimeNow()
}

func (r *Reader) lastGoodBridgeTemperature(selection types.TemperatureSelection) (types.TemperatureData, bool) {
	r.cacheMutex.RLock()
	defer r.cacheMutex.RUnlock()
	if r.lastGoodBridgeAt.IsZero() || r.lastGoodBridgeSelection != selection {
		return types.TemperatureData{}, false
	}
	if readTimeNow().Sub(r.lastGoodBridgeAt) > bridgeRecoveryTemperatureTTL {
		return types.TemperatureData{}, false
	}
	return r.lastGoodBridgeTemp, true
}

func copyBridgeTemperatureMetadata(temp *types.TemperatureData, bridgeTemp types.BridgeTemperatureData, selection types.TemperatureSelection) {
	if temp == nil {
		return
	}

	temp.CPUTemp = bridgeTemp.CpuTemp
	temp.GPUTemp = bridgeTemp.GpuTemp
	temp.CPUPowerWatts = bridgeTemp.CpuPowerWatts
	temp.GPUPowerWatts = bridgeTemp.GpuPowerWatts
	temp.GPUReadState = normalizeGPUReadState(bridgeTemp.GPUReadState, bridgeTemp.GpuTemp)
	temp.MaxTemp = bridgeTemp.MaxTemp
	temp.ControlTemp = bridgeTemp.ControlTemp
	temp.ControlSource = bridgeTemp.ControlSource
	temp.SelectedGpuDevice = bridgeTemp.SelectedGpuDevice
	if temp.ControlSource == "" {
		temp.ControlSource = selection.TempSource
	}
	if temp.ControlTemp == 0 {
		temp.ControlTemp = resolveControlTemp(temp.CPUTemp, temp.GPUTemp, temp.ControlSource)
	}
	temp.CpuModel = bridgeTemp.CpuModel
	temp.GpuModel = bridgeTemp.GpuModel
	temp.CpuSensors = bridgeTemp.CpuSensors
	temp.GpuSensors = bridgeTemp.GpuSensors
	temp.CpuPowerSensors = bridgeTemp.CpuPowerSensors
	temp.GpuPowerSensors = bridgeTemp.GpuPowerSensors
	temp.GpuDevices = bridgeTemp.GpuDevices
	if bridgeTemp.UpdateTime > 0 {
		temp.UpdateTime = bridgeTemp.UpdateTime
		if temp.UpdateTime < 1_000_000_000_000 {
			temp.UpdateTime *= 1000
		}
	}
}

func normalizeGPUReadState(state string, gpuTemp int) string {
	switch state {
	case types.GPUReadStateActive,
		types.GPUReadStateNotPolled,
		types.GPUReadStateUnavailable,
		types.GPUReadStateError:
		return state
	}
	if gpuTemp > 0 {
		return types.GPUReadStateActive
	}
	return types.GPUReadStateUnavailable
}

func resolveControlTemp(cpuTemp, gpuTemp int, source string) int {
	switch types.NormalizeTempSource(source) {
	case types.TempSourceCPU:
		if cpuTemp <= 0 && gpuTemp > 0 {
			return gpuTemp
		}
		return cpuTemp
	case types.TempSourceGPU:
		if gpuTemp <= 0 && cpuTemp > 0 {
			return cpuTemp
		}
		return gpuTemp
	default:
		return max(cpuTemp, gpuTemp)
	}
}

// readCPUTemperature 读取CPU温度
func (r *Reader) readCPUTemperature() int {
	sensorTemps, err := sensors.SensorsTemperatures()
	if err == nil {
		for _, sensor := range sensorTemps {
			// 查找ACPI ThermalZone TZ00_0或类似的CPU温度传感器
			if strings.Contains(strings.ToLower(sensor.SensorKey), "tz00") ||
				strings.Contains(strings.ToLower(sensor.SensorKey), "cpu") ||
				strings.Contains(strings.ToLower(sensor.SensorKey), "core") {
				return int(sensor.Temperature)
			}
		}
	}

	// 如果传感器方式失败，尝试通过WMI (Windows)
	return r.readWindowsCPUTemp()
}

// readGPUTemperature 读取GPU温度
func (r *Reader) readGPUTemperature() int {
	vendor := r.detectGPUVendor()
	return r.readGPUTempByVendor(vendor)
}

// readWindowsCPUTemp 通过WMI读取Windows CPU温度
func (r *Reader) readWindowsCPUTemp() int {
	output, err := execHelperCommand(helperCommandTimeout, "wmic", "/namespace:\\\\root\\wmi", "PATH", "MSAcpi_ThermalZoneTemperature", "get", "CurrentTemperature", "/value")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Debug("读取Windows CPU温度超时: %v", err)
		} else {
			r.logger.Debug("读取Windows CPU温度失败: %v", err)
		}
		return 0
	}

	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "CurrentTemperature="); ok {
			tempStr := after
			tempStr = strings.TrimSpace(tempStr)
			if tempStr != "" {
				if temp, err := strconv.Atoi(tempStr); err == nil {
					celsius := (temp - 2732) / 10
					if celsius > 0 && celsius < 150 {
						return celsius
					}
				}
			}
		}
	}

	return 0
}

// detectGPUVendor 检测GPU厂商
func (r *Reader) detectGPUVendor() string {
	now := readTimeNow()
	r.cacheMutex.RLock()
	if cached := r.cachedGPUVendor; cached != "" && now.Sub(r.cachedVendorAt) < gpuVendorCacheTTL {
		r.cacheMutex.RUnlock()
		return cached
	}
	r.cacheMutex.RUnlock()

	vendor := "unknown"
	// 尝试NVIDIA
	if _, err := execHelperCommand(helperCommandTimeout, "nvidia-smi", "--version"); err == nil {
		vendor = "nvidia"
	} else if !errors.Is(err, context.DeadlineExceeded) {
		r.logger.Debug("检测GPU厂商失败: %v", err)
	} else {
		r.logger.Debug("检测GPU厂商超时: %v", err)
	}

	r.cacheMutex.Lock()
	r.cachedGPUVendor = vendor
	r.cachedVendorAt = now
	r.cacheMutex.Unlock()

	return vendor
}

// readGPUTempByVendor 根据厂商读取GPU温度
func (r *Reader) readGPUTempByVendor(vendor string) int {
	switch vendor {
	case "nvidia":
		return r.readNvidiaGPUTemp()
	case "amd":
		return 0
	default:
		return 0
	}
}

// readNvidiaGPUTemp 安全读取NVIDIA GPU温度
func (r *Reader) readNvidiaGPUTemp() int {
	output, err := execHelperCommand(helperCommandTimeout, "nvidia-smi", "--query-gpu=temperature.gpu", "--format=csv,noheader,nounits")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Debug("读取NVIDIA GPU温度超时: %v", err)
		} else {
			r.logger.Debug("读取NVIDIA GPU温度失败: %v", err)
		}
		return 0
	}

	tempStr := strings.TrimSpace(string(output))
	lines := strings.Split(tempStr, "\n")

	if len(lines) > 0 && lines[0] != "" {
		if temp, err := strconv.Atoi(lines[0]); err == nil {
			return temp
		}
	}

	return 0
}

// execCommandHiddenWithTimeout 执行命令并隐藏窗口，避免备用读取无限阻塞监控循环。
func execCommandHiddenWithTimeout(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx := context.Background()
	cancel := func() {}
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	output, err := cmd.Output()
	if timeout > 0 && ctx.Err() != nil {
		return output, ctx.Err()
	}
	return output, err
}

// CalculateTargetRPM 根据温度计算目标转速
func CalculateTargetRPM(temperature int, fanCurve []types.FanCurvePoint) int {
	if len(fanCurve) < 2 {
		return 0
	}

	if temperature <= fanCurve[0].Temperature {
		return fanCurve[0].RPM
	}

	lastPoint := fanCurve[len(fanCurve)-1]
	if temperature >= lastPoint.Temperature {
		return lastPoint.RPM
	}

	// 线性插值计算转速
	for i := 0; i < len(fanCurve)-1; i++ {
		p1 := fanCurve[i]
		p2 := fanCurve[i+1]

		if temperature >= p1.Temperature && temperature <= p2.Temperature {
			// 线性插值
			ratio := float64(temperature-p1.Temperature) / float64(p2.Temperature-p1.Temperature)
			rpm := float64(p1.RPM) + ratio*float64(p2.RPM-p1.RPM)
			return int(rpm)
		}
	}

	return 0
}
