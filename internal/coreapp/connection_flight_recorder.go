package coreapp

import (
	"sync"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

const defaultConnectionFlightCapacity = 64

const (
	connectionFlightStageDiscovering  = "discovering"
	connectionFlightStageConnecting   = "connecting"
	connectionFlightStageConnected    = "connected"
	connectionFlightStageReady        = "ready"
	connectionFlightStageDisconnected = "disconnected"
	connectionFlightStageReconnecting = "reconnecting"
	connectionFlightStageSuspended    = "suspended"
	connectionFlightStageError        = "error"
)

type connectionFlightEvent struct {
	Sequence   uint64 `json:"sequence"`
	Timestamp  string `json:"timestamp"`
	Stage      string `json:"stage"`
	Reason     string `json:"reason,omitempty"`
	Transport  string `json:"transport,omitempty"`
	ProfileID  string `json:"profileId,omitempty"`
	Attempt    int    `json:"attempt,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

type connectionFlightSnapshotInput struct {
	State               string
	CanControl          bool
	ReconnectInProgress bool
	Suspended           bool
}

type connectionFlightSnapshot struct {
	State                 string                  `json:"state"`
	CanControl            bool                    `json:"canControl"`
	ReconnectInProgress   bool                    `json:"reconnectInProgress"`
	Suspended             bool                    `json:"suspended"`
	LastSuccessfulReadAt  string                  `json:"lastSuccessfulReadAt,omitempty"`
	LastSuccessfulWriteAt string                  `json:"lastSuccessfulWriteAt,omitempty"`
	Events                []connectionFlightEvent `json:"events"`
}

type connectionFlightRecorder struct {
	mu                    sync.RWMutex
	capacity              int
	now                   func() time.Time
	nextSequence          uint64
	events                []connectionFlightEvent
	lastSuccessfulReadAt  time.Time
	lastSuccessfulWriteAt time.Time
}

func newConnectionFlightRecorder(capacity int, now func() time.Time) *connectionFlightRecorder {
	if capacity <= 0 {
		capacity = defaultConnectionFlightCapacity
	}
	if now == nil {
		now = time.Now
	}
	return &connectionFlightRecorder{
		capacity: capacity,
		now:      now,
		events:   make([]connectionFlightEvent, 0, capacity),
	}
}

func (r *connectionFlightRecorder) record(event connectionFlightEvent) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextSequence++
	event.Sequence = r.nextSequence
	event.Timestamp = r.now().UTC().Format(time.RFC3339Nano)
	if event.DurationMs < 0 {
		event.DurationMs = 0
	}
	if len(r.events) == r.capacity {
		copy(r.events, r.events[1:])
		r.events[len(r.events)-1] = event
		return
	}
	r.events = append(r.events, event)
}

func (r *connectionFlightRecorder) markSuccessfulRead() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.lastSuccessfulReadAt = r.now().UTC()
	r.mu.Unlock()
}

func (r *connectionFlightRecorder) markSuccessfulWrite() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.lastSuccessfulWriteAt = r.now().UTC()
	r.mu.Unlock()
}

// setTargetSpeed 是唯一的「下发目标转速」入口（智能控温 / 安全回退 / 手动 / 自定义都汇到它）。
// 黑鲨的变频/固定分流见 setTargetSpeedWithMode。
func (a *CoreApp) setTargetSpeed(value int, unit string) bool {
	return a.setTargetSpeedWithMode(value, unit, a.autoControlActive())
}

// setTargetSpeedWithMode 见 setTargetSpeed 的说明；blackSharkInverter 表示变频意图。
//
// 黑鲨变频下主机不再下发转速：曲线形状已写进设备（见 applyBlackSharkGear），
// 之后主机只推温度（`0x07`），设备自己按曲线变速。这条分支一个字节都不发；
// 返回 true 表示"按设计处理完毕"，并刻意不 markSuccessfulWrite：这里没有设备写入，
// "连接存活"不该由它刷新（避免假绿）。调用方（监控循环）对黑鲨只把 false 当作失败
// （`autoPushFailed`），因此返回 true 不会误报失败。
func (a *CoreApp) setTargetSpeedWithMode(value int, unit string, blackSharkInverter bool) bool {
	// 黑鲨（RPM 域）：变频/固定两条路都在这里分流 —— 设备层的 `SetTargetSpeed` 只会写 form=0 且 gear 写死 1
	// （`device/blackshark_hid.go` 的 `setBlackSharkTargetSpeedLocked`），所以黑鲨不能走那条通用路。
	if a.deviceManager != nil && types.IsBlackSharkDeviceProfileID(a.deviceManager.ActiveProfile().ID) &&
		types.IsRPMSpeedUnit(unit) {
		if blackSharkInverter {
			// 变频：主机不下发转速 —— 曲线形状已写进设备（见 applyBlackSharkGear），
			// 之后主机只推温度（0x07），设备自己按曲线变速。
			// 返回 true = "按设计处理完毕"；刻意不 markSuccessfulWrite（这里没有设备写入，避免假绿）。
			return true
		}
		// 固定：写到当前档 —— 设备层原本把 gear 写死成 1，与当前档位无关。
		if gear := a.activeBlackSharkGear(); gear >= 1 {
			ok := a.deviceManager.SetBlackSharkFixedSpeedForGear(gear, value)
			if ok {
				a.connectionFlights.markSuccessfulWrite()
			}
			return ok
		}
		// 拿不到当前档（当前方案不是黑鲨四档）⇒ 交回通用路径，避免静默不发。
	}
	ok := a.deviceManager.SetTargetSpeed(value, unit)
	if ok {
		a.connectionFlights.markSuccessfulWrite()
	}
	return ok
}

// autoControlActive 报告智能控温此刻是否生效，即变频意图的判据。
func (a *CoreApp) autoControlActive() bool {
	if a == nil || a.configManager == nil {
		return false
	}
	cfg := a.configManager.Get()
	return cfg.AutoControl && !cfg.CustomSpeedEnabled
}

func (r *connectionFlightRecorder) snapshot(input connectionFlightSnapshotInput) connectionFlightSnapshot {
	if r == nil {
		return connectionFlightSnapshot{
			State:               input.State,
			CanControl:          input.CanControl,
			ReconnectInProgress: input.ReconnectInProgress,
			Suspended:           input.Suspended,
			Events:              []connectionFlightEvent{},
		}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	events := append([]connectionFlightEvent(nil), r.events...)
	return connectionFlightSnapshot{
		State:                 input.State,
		CanControl:            input.CanControl,
		ReconnectInProgress:   input.ReconnectInProgress,
		Suspended:             input.Suspended,
		LastSuccessfulReadAt:  formatConnectionFlightTime(r.lastSuccessfulReadAt),
		LastSuccessfulWriteAt: formatConnectionFlightTime(r.lastSuccessfulWriteAt),
		Events:                events,
	}
}

func formatConnectionFlightTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
