package temperature

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func enableRecorderForTest(t *testing.T, recorder *HistoryRecorder) {
	t.Helper()
	if err := recorder.SetEnabled(true); err != nil {
		t.Fatalf("enable recorder: %v", err)
	}
}

func TestHistoryRecorderDefaultsEnabled(t *testing.T) {
	t.Parallel()

	recorder := NewHistoryRecorder(filepath.Join(t.TempDir(), "history.bin"), 8, 5*time.Second, nil)
	if !recorder.IsEnabled() {
		t.Fatal("expected history recorder to default enabled")
	}
}

func TestHistoryRecorderAddNormalizesSecondTimestamp(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "history.bin")
	recorder := NewHistoryRecorder(filePath, 8, 5*time.Second, nil)
	enableRecorderForTest(t, recorder)

	baseSeconds := int64(1_717_000_000)
	point, recorded := recorder.Add(types.TemperatureData{
		CPUTemp:       61,
		GPUTemp:       58,
		CPUPowerWatts: 25.5,
		GPUPowerWatts: 41.25,
		UpdateTime:    baseSeconds,
	}, &types.FanData{CurrentRPM: 1680})
	if !recorded {
		t.Fatal("expected first history point to be recorded")
	}
	if want := baseSeconds * 1000; point.Timestamp != want {
		t.Fatalf("expected normalized timestamp %d, got %d", want, point.Timestamp)
	}
	if point.CPUPowerWatts != 25.5 || point.GPUPowerWatts != 41.25 {
		t.Fatalf("expected power watts to be preserved, got cpu %.2f gpu %.2f", point.CPUPowerWatts, point.GPUPowerWatts)
	}

	if _, recorded := recorder.Add(types.TemperatureData{
		CPUTemp:    62,
		GPUTemp:    59,
		UpdateTime: baseSeconds + 1,
	}, &types.FanData{CurrentRPM: 1720}); recorded {
		t.Fatal("expected sample inside 5s window to be skipped")
	}

	if _, recorded := recorder.Add(types.TemperatureData{
		CPUTemp:    64,
		GPUTemp:    60,
		UpdateTime: baseSeconds + 5,
	}, &types.FanData{CurrentRPM: 1760}); !recorded {
		t.Fatal("expected sample at 5s boundary to be recorded")
	}
}

func TestHistoryRecorderPersistsBinarySnapshot(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "history.bin")
	recorder := NewHistoryRecorder(filePath, 8, 5*time.Second, nil)
	enableRecorderForTest(t, recorder)
	_, _ = recorder.Add(types.TemperatureData{CPUTemp: 60, GPUTemp: 54, CPUPowerWatts: 22.5, GPUPowerWatts: 35.25, UpdateTime: 1_717_000_000}, &types.FanData{CurrentRPM: 1500})
	_, _ = recorder.Add(types.TemperatureData{CPUTemp: 62, GPUTemp: 55, CPUPowerWatts: 28.75, GPUPowerWatts: 43.5, UpdateTime: 1_717_000_005}, &types.FanData{CurrentRPM: 1550})
	if err := recorder.Flush(); err != nil {
		t.Fatalf("flush binary history: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read binary history: %v", err)
	}
	if !bytes.HasPrefix(data, []byte(historyBinaryMagic)) {
		t.Fatalf("expected binary history to start with %q", historyBinaryMagic)
	}

	reloaded := NewHistoryRecorder(filePath, 8, 5*time.Second, nil)
	snapshot := reloaded.Snapshot()
	if len(snapshot.Points) != 2 {
		t.Fatalf("expected 2 reloaded points, got %d", len(snapshot.Points))
	}
	if snapshot.Points[1].FanRPM != 1550 {
		t.Fatalf("expected fan rpm 1550, got %d", snapshot.Points[1].FanRPM)
	}
	if snapshot.Points[1].CPUPowerWatts != 28.75 || snapshot.Points[1].GPUPowerWatts != 43.5 {
		t.Fatalf("expected power watts to reload, got cpu %.2f gpu %.2f", snapshot.Points[1].CPUPowerWatts, snapshot.Points[1].GPUPowerWatts)
	}
}

func TestHistoryRecorderLoadsV1BinarySnapshot(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "history.bin")
	data := make([]byte, 0, len(historyBinaryMagic)+24+20)
	data = append(data, historyBinaryMagic...)
	data = binary.LittleEndian.AppendUint16(data, historyBinaryVersionV1)
	data = append(data, historyEnabledFlag, 0)
	data = binary.LittleEndian.AppendUint32(data, 5)
	data = binary.LittleEndian.AppendUint32(data, 1)
	data = binary.LittleEndian.AppendUint64(data, uint64(1_717_000_010_000))
	data = binary.LittleEndian.AppendUint64(data, uint64(1_717_000_000_000))
	data = binary.LittleEndian.AppendUint32(data, uint32(int32(61)))
	data = binary.LittleEndian.AppendUint32(data, uint32(int32(54)))
	data = binary.LittleEndian.AppendUint32(data, uint32(int32(1500)))
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("write v1 fixture: %v", err)
	}

	reloaded := NewHistoryRecorder(filePath, 8, 5*time.Second, nil)
	snapshot := reloaded.Snapshot()
	if len(snapshot.Points) != 1 {
		t.Fatalf("expected 1 reloaded v1 point, got %d", len(snapshot.Points))
	}
	point := snapshot.Points[0]
	if point.CPUTemp != 61 || point.GPUTemp != 54 || point.FanRPM != 1500 {
		t.Fatalf("unexpected v1 point: %+v", point)
	}
	if point.CPUPowerWatts != 0 || point.GPUPowerWatts != 0 {
		t.Fatalf("expected v1 power fields to default to 0, got cpu %.2f gpu %.2f", point.CPUPowerWatts, point.GPUPowerWatts)
	}
}

func TestNormalizePowerWattsDropsInvalidValues(t *testing.T) {
	t.Parallel()

	for _, value := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := normalizePowerWatts(value); got != 0 {
			t.Fatalf("expected invalid power %.2f to normalize to 0, got %.2f", value, got)
		}
	}
}
