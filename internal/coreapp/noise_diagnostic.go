package coreapp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

const noiseDiagnosticLeaseDuration = 20 * time.Minute

type noiseDiagnosticLease struct {
	sessionID      string
	deviceKey      string
	rangeConfig    types.NoiseDiagnosticRange
	configRevision uint64
	originalConfig types.AppConfig
	expiresAt      time.Time
	done           chan struct{}
}

func newNoiseDiagnosticSessionID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return fmt.Sprintf("noise-%d", time.Now().UnixNano())
}

func (a *CoreApp) noiseDiagnosticDeviceContext() (types.AppConfig, types.DeviceProfile, types.DeviceCapabilities, string, error) {
	if a == nil || a.deviceManager == nil || a.configManager == nil {
		return types.AppConfig{}, types.DeviceProfile{}, types.DeviceCapabilities{}, "", fmt.Errorf("核心设备服务不可用")
	}
	if !a.deviceManager.IsConnected() {
		return types.AppConfig{}, types.DeviceProfile{}, types.DeviceCapabilities{}, "", fmt.Errorf("设备未连接")
	}
	cfg, _ := a.configManager.GetWithRevision()
	profile := types.NormalizeDeviceProfile(a.deviceManager.ActiveProfile(), "")
	if strings.TrimSpace(profile.ID) == "" && strings.TrimSpace(profile.DisplayName) == "" {
		profile = types.ActiveDeviceProfile(&cfg)
	}
	caps := a.deviceManager.ActiveCapabilities()
	if !caps.SupportsSetSpeed {
		return cfg, profile, caps, "", fmt.Errorf("当前设备不支持风扇转速控制")
	}
	key := a.activeDeviceCurveScopeKey(cfg)
	if strings.TrimSpace(key) == "" {
		return cfg, profile, caps, "", fmt.Errorf("无法识别当前设备")
	}
	return cfg, profile, caps, key, nil
}

// BeginNoiseDiagnostic reserves temporary speed-control ownership. The lease
// intentionally leaves the user's automatic/manual configuration untouched.
func (a *CoreApp) BeginNoiseDiagnostic(request types.NoiseDiagnosticBeginRequest) (types.NoiseDiagnosticSession, error) {
	cfg, profile, caps, deviceKey, err := a.noiseDiagnosticDeviceContext()
	if err != nil {
		return types.NoiseDiagnosticSession{}, err
	}
	if requestedKey := strings.TrimSpace(request.DeviceKey); requestedKey != "" && requestedKey != deviceKey {
		return types.NoiseDiagnosticSession{}, fmt.Errorf("诊断设备已变化")
	}
	allowed, err := types.NoiseDiagnosticRangeForProfile(profile, caps, a.deviceManager.GetCurrentFanData())
	if err != nil {
		return types.NoiseDiagnosticSession{}, err
	}
	rangeConfig, err := types.NormalizeNoiseDiagnosticRange(request.Range, allowed)
	if err != nil {
		return types.NoiseDiagnosticSession{}, err
	}
	_, revision := a.configManager.GetWithRevision()
	lease := &noiseDiagnosticLease{
		sessionID:      newNoiseDiagnosticSessionID(),
		deviceKey:      deviceKey,
		rangeConfig:    rangeConfig,
		configRevision: revision,
		originalConfig: cfg,
		expiresAt:      time.Now().Add(noiseDiagnosticLeaseDuration),
		done:           make(chan struct{}),
	}

	a.noiseDiagnosticMu.Lock()
	if current := a.noiseDiagnosticLease; current != nil && time.Now().Before(current.expiresAt) {
		a.noiseDiagnosticMu.Unlock()
		return types.NoiseDiagnosticSession{}, fmt.Errorf("已有噪声诊断正在进行")
	}
	if current := a.noiseDiagnosticLease; current != nil {
		close(current.done)
	}
	a.noiseDiagnosticLease = lease
	a.noiseDiagnosticMu.Unlock()

	a.safeGo("noise-diagnostic-expiry", func() {
		timer := time.NewTimer(time.Until(lease.expiresAt))
		defer timer.Stop()
		select {
		case <-timer.C:
			_ = a.CancelNoiseDiagnostic(lease.sessionID)
		case <-lease.done:
		}
	})

	return types.NoiseDiagnosticSession{
		SessionID:      lease.sessionID,
		DeviceKey:      lease.deviceKey,
		Range:          lease.rangeConfig,
		ConfigRevision: lease.configRevision,
	}, nil
}

func (a *CoreApp) noiseDiagnosticLeaseFor(sessionID string) (*noiseDiagnosticLease, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("缺少诊断会话")
	}
	a.noiseDiagnosticMu.Lock()
	lease := a.noiseDiagnosticLease
	if lease == nil {
		a.noiseDiagnosticMu.Unlock()
		return nil, fmt.Errorf("诊断会话已结束")
	}
	if lease.sessionID != sessionID {
		a.noiseDiagnosticMu.Unlock()
		return nil, fmt.Errorf("诊断会话无效")
	}
	if !time.Now().Before(lease.expiresAt) {
		a.noiseDiagnosticMu.Unlock()
		_ = a.CancelNoiseDiagnostic(sessionID)
		return nil, fmt.Errorf("诊断会话已过期")
	}
	copyLease := *lease
	a.noiseDiagnosticMu.Unlock()
	return &copyLease, nil
}

func (a *CoreApp) noiseDiagnosticLeaseActive() bool {
	a.noiseDiagnosticMu.Lock()
	lease := a.noiseDiagnosticLease
	active := lease != nil && time.Now().Before(lease.expiresAt)
	a.noiseDiagnosticMu.Unlock()
	return active
}

// SetNoiseDiagnosticTarget changes only the temporary target owned by the lease.
func (a *CoreApp) SetNoiseDiagnosticTarget(sessionID string, value int) (types.NoiseDiagnosticTargetResult, error) {
	lease, err := a.noiseDiagnosticLeaseFor(sessionID)
	if err != nil {
		return types.NoiseDiagnosticTargetResult{}, err
	}
	if value < lease.rangeConfig.Min || value > lease.rangeConfig.Max {
		return types.NoiseDiagnosticTargetResult{}, fmt.Errorf("目标转速超出诊断范围")
	}
	_, _, _, currentKey, err := a.noiseDiagnosticDeviceContext()
	if err != nil {
		return types.NoiseDiagnosticTargetResult{}, err
	}
	if currentKey != lease.deviceKey {
		return types.NoiseDiagnosticTargetResult{}, fmt.Errorf("诊断设备已变化")
	}
	unit := types.NormalizeFanSpeedUnit(lease.rangeConfig.Unit)
	deviceValue := value
	if types.IsPercentSpeedUnit(unit) {
		deviceValue = types.PercentToTicks(value)
	}
	if !a.deviceManager.SetTargetSpeed(deviceValue, unit) {
		return types.NoiseDiagnosticTargetResult{}, fmt.Errorf("目标转速下发失败")
	}

	actual := 0
	lastActual := 0
	targetHitCount := 0
	stableCount := 0
	deadline := time.Now().Add(4 * time.Second)
	for {
		if !a.noiseDiagnosticLeaseActive() {
			return types.NoiseDiagnosticTargetResult{}, fmt.Errorf("诊断会话已结束")
		}
		if fanData := a.deviceManager.GetCurrentFanData(); fanData != nil {
			if types.IsPercentSpeedUnit(unit) && fanData.TargetRPM > 0 {
				actual = types.PercentTicksToIntegerPercent(int(fanData.TargetRPM))
				break
			}
			if types.IsRPMSpeedUnit(unit) && fanData.CurrentRPM > 0 {
				actual = int(fanData.CurrentRPM)
				tolerance := max(120, value*6/100)
				if actual >= value-tolerance && actual <= value+tolerance {
					targetHitCount++
				} else {
					targetHitCount = 0
				}
				if lastActual > 0 && actual >= lastActual-30 && actual <= lastActual+30 {
					stableCount++
				} else {
					stableCount = 0
				}
				lastActual = actual
				if targetHitCount >= 2 || stableCount >= 4 {
					break
				}
			}
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if types.IsPercentSpeedUnit(unit) && actual <= 0 {
		// Percentage controllers expose the commanded setpoint rather than a
		// tachometer value; the accepted target is the stable device state here.
		actual = value
	}
	if actual <= 0 || (types.IsRPMSpeedUnit(unit) && targetHitCount < 2 && stableCount < 4) {
		return types.NoiseDiagnosticTargetResult{}, fmt.Errorf("实际转速未能稳定")
	}
	return types.NoiseDiagnosticTargetResult{Requested: value, Actual: actual, Unit: unit}, nil
}

func (a *CoreApp) restoreNoiseDiagnosticState(lease *noiseDiagnosticLease) error {
	if a == nil || a.deviceManager == nil || a.configManager == nil || !a.deviceManager.IsConnected() {
		return nil
	}
	cfg, revision := a.configManager.GetWithRevision()
	if revision == lease.configRevision {
		cfg = lease.originalConfig
	}
	if currentKey := a.activeDeviceCurveScopeKey(cfg); currentKey != lease.deviceKey {
		return nil
	}
	if cfg.AutoControl {
		a.forceNextAutoTarget.Store(true)
		return nil
	}
	unit := a.activeDeviceSpeedUnit(&cfg)
	if cfg.CustomSpeedEnabled {
		speed := types.ClampSpeedForUnit(cfg.CustomSpeedRPM, unit)
		if speed <= 0 || !a.deviceManager.SetTargetSpeed(configSpeedToTargetUnit(speed, unit), unit) {
			return fmt.Errorf("恢复自定义转速失败")
		}
		return nil
	}
	if err := a.applyCurrentGearSetting(); err != nil {
		return fmt.Errorf("恢复手动挡位失败: %w", err)
	}
	return nil
}

func (a *CoreApp) finishNoiseDiagnostic(sessionID, reason string) error {
	if a == nil {
		return nil
	}
	a.noiseDiagnosticMu.Lock()
	lease := a.noiseDiagnosticLease
	if lease == nil {
		a.noiseDiagnosticMu.Unlock()
		return nil
	}
	if strings.TrimSpace(sessionID) != "" && lease.sessionID != sessionID {
		a.noiseDiagnosticMu.Unlock()
		return fmt.Errorf("诊断会话无效")
	}
	a.noiseDiagnosticLease = nil
	close(lease.done)
	a.noiseDiagnosticMu.Unlock()

	if err := a.restoreNoiseDiagnosticState(lease); err != nil {
		a.logError("噪声诊断结束后恢复控制状态失败 (%s): %v", reason, err)
		return err
	}
	return nil
}

func (a *CoreApp) EndNoiseDiagnostic(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("缺少诊断会话")
	}
	return a.finishNoiseDiagnostic(sessionID, "结束")
}

func (a *CoreApp) CancelNoiseDiagnostic(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("缺少诊断会话")
	}
	return a.finishNoiseDiagnostic(sessionID, "取消")
}

func (a *CoreApp) cancelNoiseDiagnosticLease(reason string) {
	if err := a.finishNoiseDiagnostic("", reason); err != nil {
		a.logDebug("取消噪声诊断租约失败: %v", err)
	}
}

func (a *CoreApp) SaveNoiseDiagnosticResult(result types.NoiseDiagnosticResult) error {
	result, _ = types.NormalizeNoiseDiagnosticResult(result)
	if strings.TrimSpace(result.DeviceKey) == "" || len(result.Points) == 0 {
		return fmt.Errorf("噪声诊断结果不完整")
	}
	if a == nil || a.configManager == nil || a.deviceManager == nil || !a.deviceManager.IsConnected() {
		return fmt.Errorf("设备未连接")
	}
	_, profile, caps, deviceKey, err := a.noiseDiagnosticDeviceContext()
	if err != nil {
		return err
	}
	if result.DeviceKey != deviceKey {
		return fmt.Errorf("诊断结果设备已变化")
	}
	unit := types.NormalizeFanSpeedUnit(result.Unit)
	allowed, err := types.NoiseDiagnosticRangeForProfile(profile, caps, a.deviceManager.GetCurrentFanData())
	if err != nil || unit != allowed.Unit {
		return fmt.Errorf("诊断结果速度单位与当前设备不匹配")
	}
	result.Unit = unit
	for attempt := 0; attempt < 2; attempt++ {
		_, revision := a.configManager.GetWithRevision()
		updated, _, applied := a.configManager.MutateIfRevision(revision, func(current *types.AppConfig) {
			if current.NoiseDiagnosticsByDevice == nil {
				current.NoiseDiagnosticsByDevice = map[string]types.NoiseDiagnosticResult{}
			}
			current.NoiseDiagnosticsByDevice[result.DeviceKey] = result
		})
		if !applied {
			continue
		}
		if err := a.configManager.Save(); err != nil {
			return err
		}
		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, updated)
		}
		return nil
	}
	return fmt.Errorf("保存噪声诊断结果时配置已变化")
}
