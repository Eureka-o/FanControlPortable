package temperature

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

const (
	DefaultHistoryCapacity              = 3600
	DefaultHistorySampleInterval        = 1 * time.Second
	DefaultHistoryRelativePath          = "telemetry/fancontrol-history.bin"
	LegacyHistoryRelativePath           = "telemetry/fancontrolportable2-history.bin"
	historyBinaryMagic                  = "THST"
	historyBinaryVersionV1       uint16 = 1
	historyBinaryVersionV2       uint16 = 2
	historyBinaryVersion         uint16 = historyBinaryVersionV2
	historyEnabledFlag           uint8  = 1

	dirtyFlushThreshold = 6
	dirtyFlushInterval  = 30 * time.Second
)

type HistoryRecorder struct {
	mutex          sync.RWMutex
	logger         types.Logger
	filePath       string
	enabled        bool
	capacity       int
	sampleInterval time.Duration
	points         []types.TemperatureHistoryPoint
	next           int
	filled         bool
	lastSampleAt   int64

	dirtyCount  int
	lastFlushAt time.Time
	flushMutex  sync.Mutex // 串行化磁盘写入，与 mutex 互不持有
}

func NewHistoryRecorder(filePath string, capacity int, sampleInterval time.Duration, logger types.Logger) *HistoryRecorder {
	if capacity <= 0 {
		capacity = DefaultHistoryCapacity
	}
	if sampleInterval <= 0 {
		sampleInterval = DefaultHistorySampleInterval
	}

	recorder := &HistoryRecorder{
		logger:         logger,
		filePath:       filePath,
		capacity:       capacity,
		sampleInterval: sampleInterval,
		enabled:        true,
		// 惰性分配：起始 len=0，随实际数据增长至 capacity，避免启动即占满 173KB。
		points: make([]types.TemperatureHistoryPoint, 0, capacity),
	}
	recorder.load()
	return recorder
}

func (r *HistoryRecorder) SetEnabled(enabled bool) error {
	r.mutex.Lock()
	r.enabled = enabled
	if !enabled {
		r.clearLocked()
	}
	payload, err := r.serializeLocked()
	r.mutex.Unlock()
	if err != nil {
		return err
	}
	return r.writeFile(payload)
}

func (r *HistoryRecorder) IsEnabled() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.enabled
}

func (r *HistoryRecorder) Flush() error {
	r.mutex.Lock()
	if r.dirtyCount == 0 {
		r.mutex.Unlock()
		return nil
	}
	payload, err := r.serializeLocked()
	r.dirtyCount = 0
	r.lastFlushAt = time.Now()
	r.mutex.Unlock()
	if err != nil {
		return err
	}
	return r.writeFile(payload)
}

func (r *HistoryRecorder) Add(temp types.TemperatureData, fanData *types.FanData) (types.TemperatureHistoryPoint, bool) {
	if temp.CPUTemp <= 0 && temp.GPUTemp <= 0 {
		return types.TemperatureHistoryPoint{}, false
	}

	timestamp := normalizeTimestampMillis(temp.UpdateTime)
	if timestamp <= 0 {
		timestamp = time.Now().UnixMilli()
	}

	fanRPM := 0
	if fanData != nil {
		fanRPM = int(fanData.CurrentRPM)
	}

	gpuTemp := temp.GPUTemp
	gpuPowerWatts := temp.GPUPowerWatts
	if temp.GPUReadState == types.GPUReadStateNotPolled {
		gpuTemp = 0
		gpuPowerWatts = 0
	}

	point := types.TemperatureHistoryPoint{
		Timestamp:     timestamp,
		CPUTemp:       temp.CPUTemp,
		GPUTemp:       gpuTemp,
		FanRPM:        fanRPM,
		CPUPowerWatts: normalizePowerWatts(temp.CPUPowerWatts),
		GPUPowerWatts: normalizePowerWatts(gpuPowerWatts),
	}

	var flushPayload []byte

	r.mutex.Lock()
	if !r.enabled {
		r.mutex.Unlock()
		return types.TemperatureHistoryPoint{}, false
	}
	if r.lastSampleAt > 0 && timestamp-r.lastSampleAt < r.sampleInterval.Milliseconds() {
		r.mutex.Unlock()
		return types.TemperatureHistoryPoint{}, false
	}

	// 未填满前用 append 惰性扩展切片；填满后覆写最旧的槽位（环形语义不变）。
	if !r.filled {
		r.points = append(r.points, point)
		if len(r.points) == r.capacity {
			r.filled = true
			r.next = 0
		} else {
			r.next = len(r.points)
		}
	} else {
		r.points[r.next] = point
		r.next = (r.next + 1) % r.capacity
	}
	r.lastSampleAt = timestamp

	r.dirtyCount++
	now := time.Now()
	if r.dirtyCount >= dirtyFlushThreshold || now.Sub(r.lastFlushAt) >= dirtyFlushInterval {
		if payload, err := r.serializeLocked(); err == nil {
			flushPayload = payload
			r.dirtyCount = 0
			r.lastFlushAt = now
		} else {
			r.logError("序列化温度历史失败: %v", err)
		}
	}
	r.mutex.Unlock()

	if flushPayload != nil {
		if err := r.writeFile(flushPayload); err != nil {
			r.logError("保存温度历史失败: %v", err)
		}
	}
	return point, true
}

func (r *HistoryRecorder) Snapshot() types.TemperatureHistoryPayload {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return types.TemperatureHistoryPayload{
		Enabled:               r.enabled,
		SampleIntervalSeconds: int(r.sampleInterval / time.Second),
		Points:                r.snapshotPointsLocked(),
	}
}

func normalizeTimestampMillis(timestamp int64) int64 {
	if timestamp <= 0 {
		return 0
	}
	if timestamp < 1_000_000_000_000 {
		return timestamp * 1000
	}
	return timestamp
}

func (r *HistoryRecorder) load() {
	if r.filePath == "" {
		return
	}

	if err := r.loadBinaryFile(r.filePath); err == nil {
		return
	} else if !os.IsNotExist(err) {
		r.logError("解析温度历史文件失败: %v", err)
	}
}

func (r *HistoryRecorder) loadBinaryFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return r.loadBinaryData(data)
}

func (r *HistoryRecorder) loadBinaryData(data []byte) error {
	reader := bytes.NewReader(data)
	magic := make([]byte, len(historyBinaryMagic))
	if _, err := io.ReadFull(reader, magic); err != nil {
		return err
	}
	if string(magic) != historyBinaryMagic {
		return io.ErrUnexpectedEOF
	}

	var version uint16
	if err := binary.Read(reader, binary.LittleEndian, &version); err != nil {
		return err
	}
	if version != historyBinaryVersionV1 && version != historyBinaryVersionV2 {
		return fmt.Errorf("unsupported history version: %d", version)
	}

	var flags uint8
	var reserved uint8
	var sampleIntervalSeconds uint32
	var count uint32
	var updatedAt int64
	if err := binary.Read(reader, binary.LittleEndian, &flags); err != nil {
		return err
	}
	if err := binary.Read(reader, binary.LittleEndian, &reserved); err != nil {
		return err
	}
	if err := binary.Read(reader, binary.LittleEndian, &sampleIntervalSeconds); err != nil {
		return err
	}
	if err := binary.Read(reader, binary.LittleEndian, &count); err != nil {
		return err
	}
	if err := binary.Read(reader, binary.LittleEndian, &updatedAt); err != nil {
		return err
	}

	points := make([]types.TemperatureHistoryPoint, 0, count)
	for i := uint32(0); i < count; i++ {
		var timestamp int64
		var cpuTemp int32
		var gpuTemp int32
		var fanRPM int32
		var cpuPowerWatts float64
		var gpuPowerWatts float64
		if err := binary.Read(reader, binary.LittleEndian, &timestamp); err != nil {
			return err
		}
		if err := binary.Read(reader, binary.LittleEndian, &cpuTemp); err != nil {
			return err
		}
		if err := binary.Read(reader, binary.LittleEndian, &gpuTemp); err != nil {
			return err
		}
		if err := binary.Read(reader, binary.LittleEndian, &fanRPM); err != nil {
			return err
		}
		if version >= historyBinaryVersionV2 {
			if err := binary.Read(reader, binary.LittleEndian, &cpuPowerWatts); err != nil {
				return err
			}
			if err := binary.Read(reader, binary.LittleEndian, &gpuPowerWatts); err != nil {
				return err
			}
		}
		points = append(points, types.TemperatureHistoryPoint{
			Timestamp:     normalizeTimestampMillis(timestamp),
			CPUTemp:       int(cpuTemp),
			GPUTemp:       int(gpuTemp),
			FanRPM:        int(fanRPM),
			CPUPowerWatts: normalizePowerWatts(cpuPowerWatts),
			GPUPowerWatts: normalizePowerWatts(gpuPowerWatts),
		})
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.enabled = flags&historyEnabledFlag != 0
	if sampleIntervalSeconds > 0 {
		r.sampleInterval = time.Duration(sampleIntervalSeconds) * time.Second
	}
	r.applyLoadedPointsLocked(points)
	return nil
}

func (r *HistoryRecorder) applyLoadedPointsLocked(points []types.TemperatureHistoryPoint) {
	if len(points) > r.capacity {
		points = points[len(points)-r.capacity:]
	}
	// 直接复用底层数组：reset 长度后 append，不做全槽清零。
	r.points = r.points[:0]
	r.points = append(r.points, points...)
	if len(r.points) >= r.capacity {
		r.filled = true
		r.next = 0
	} else {
		r.filled = false
		r.next = len(r.points)
	}
	if len(r.points) > 0 {
		r.lastSampleAt = r.points[len(r.points)-1].Timestamp
	} else {
		r.lastSampleAt = 0
	}
}

func (r *HistoryRecorder) snapshotPointsLocked() []types.TemperatureHistoryPoint {
	points := make([]types.TemperatureHistoryPoint, 0, r.pointCountLocked())
	if r.filled {
		points = append(points, r.points[r.next:]...)
		points = append(points, r.points[:r.next]...)
	} else {
		points = append(points, r.points[:r.next]...)
	}
	return points
}

func (r *HistoryRecorder) pointCountLocked() int {
	if r.filled {
		return r.capacity
	}
	return r.next
}

func (r *HistoryRecorder) serializeLocked() ([]byte, error) {
	if r.filePath == "" {
		return nil, nil
	}
	pointCount := r.pointCountLocked()
	var flags uint8
	if r.enabled {
		flags |= historyEnabledFlag
	}
	// header 24B + 每点 36B（v2: timestamp + CPU/GPU温度 + 风扇 + CPU/GPU功耗）
	buf := make([]byte, 0, len(historyBinaryMagic)+24+pointCount*36)
	buf = append(buf, historyBinaryMagic...)
	buf = binary.LittleEndian.AppendUint16(buf, historyBinaryVersion)
	buf = append(buf, flags, 0) // flags + reserved
	buf = binary.LittleEndian.AppendUint32(buf, uint32(r.sampleInterval/time.Second))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(pointCount))
	buf = binary.LittleEndian.AppendUint64(buf, uint64(time.Now().UnixMilli()))
	appendPoint := func(p types.TemperatureHistoryPoint) {
		buf = binary.LittleEndian.AppendUint64(buf, uint64(normalizeTimestampMillis(p.Timestamp)))
		buf = binary.LittleEndian.AppendUint32(buf, uint32(int32(p.CPUTemp)))
		buf = binary.LittleEndian.AppendUint32(buf, uint32(int32(p.GPUTemp)))
		buf = binary.LittleEndian.AppendUint32(buf, uint32(int32(p.FanRPM)))
		buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(normalizePowerWatts(p.CPUPowerWatts)))
		buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(normalizePowerWatts(p.GPUPowerWatts)))
	}
	if r.filled {
		for _, p := range r.points[r.next:] {
			appendPoint(p)
		}
		for _, p := range r.points[:r.next] {
			appendPoint(p)
		}
	} else {
		for _, p := range r.points[:r.next] {
			appendPoint(p)
		}
	}
	return buf, nil
}

func normalizePowerWatts(value float64) float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

// writeFile 在锁外执行磁盘 IO。flushMutex 串行化多次并发 Flush 调用。
func (r *HistoryRecorder) writeFile(payload []byte) error {
	if payload == nil || r.filePath == "" {
		return nil
	}
	r.flushMutex.Lock()
	defer r.flushMutex.Unlock()

	if err := os.MkdirAll(filepath.Dir(r.filePath), 0755); err != nil {
		return err
	}
	tmpPath := r.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0644); err != nil {
		return err
	}
	_ = os.Remove(r.filePath)
	if err := os.Rename(tmpPath, r.filePath); err != nil {
		_ = os.Remove(tmpPath)
		return os.WriteFile(r.filePath, payload, 0644)
	}
	return nil
}

func (r *HistoryRecorder) clearLocked() {
	r.points = r.points[:0]
	r.next = 0
	r.filled = false
	r.lastSampleAt = 0
}

func (r *HistoryRecorder) logError(format string, args ...any) {
	if r.logger != nil {
		r.logger.Error(format, args...)
	}
}
