package coreapp

import (
	"fmt"
	"strings"

	cfgpkg "github.com/Eureka-o/FanControlPortable/internal/config"
	"github.com/Eureka-o/FanControlPortable/internal/curveprofiles"
	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/smartcontrol"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func (a *CoreApp) fanCurveProfilesPayloadFromConfig(cfg types.AppConfig) types.FanCurveProfilesPayload {
	return types.FanCurveProfilesPayload{
		Profiles: curveprofiles.CloneProfiles(cfg.FanCurveProfiles),
		ActiveID: cfg.ActiveFanCurveProfileID,
	}
}

func (a *CoreApp) applyCurveProfilesConfig(cfg types.AppConfig) error {
	unit := a.activeDeviceSpeedUnit(&cfg)
	runtimeDeviceKey := a.activeDeviceCurveScopeKey(cfg)
	// 必须传插值依据（黑鲨四档方案补两端端点 ⇒ 5..6 点）：本函数按曲线长度给 LearnedOffsets
	// 定长，传 cfg.FanCurve（4 点）会与下面 syncSmartControlOffsetsForDeviceKey（按有效曲线定长）
	// 来回翻转长度 ⇒ 静默错位 + 无谓落盘。见 smartControlCurveLen。
	cfg.SmartControl, _ = smartcontrol.NormalizeConfigForUnit(cfg.SmartControl, a.smartControlCurveForUnit(&cfg, unit), cfg.DebugMode, unit)
	syncSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey)
	storeDeviceFanCurveStateForKeyAndUnit(&cfg, runtimeDeviceKey, cfg, unit)
	return a.commitConfigUpdate(cfg, nil)
}

func (a *CoreApp) applyConnectedRuntimeCurveState() (types.AppConfig, bool, error) {
	cfg := a.configManager.Get()
	profile, ok := a.connectedRuntimeDeviceProfile()
	if !ok {
		return cfg, false, nil
	}

	unit := a.activeDeviceSpeedUnit(&cfg)
	runtimeDeviceKey := deviceCurveScopeKeyForProfile(profile)
	if runtimeDeviceKey == "" {
		return cfg, false, nil
	}

	changed := loadDeviceFanCurveStateForProfile(&cfg, profile, unit, runtimeCurveUseCurrentIfMissing(&cfg, unit, true))
	if curveprofiles.NormalizeConfigForUnit(&cfg, unit) {
		changed = true
	}
	// 同 applyCurveProfilesConfig：定位到插值依据，否则 LearnedOffsets 长度会翻转。
	if normalizedSmart, smartChanged := smartcontrol.NormalizeConfigForUnit(cfg.SmartControl, a.smartControlCurveForUnit(&cfg, unit), cfg.DebugMode, unit); smartChanged {
		cfg.SmartControl = normalizedSmart
		changed = true
	}
	if types.NormalizeManualGearRPMForUnit(&cfg, unit) {
		changed = true
	}
	// 必须排在上面那句规范化**之后**：规范化会把缺失/越界的 12 个值补成该单位的默认值，
	// 所以在"删过 config"的新配置上，先跑迁移看到的是空表（迁不动），规范化再把它填成
	// 通用默认值 —— 面板于是显示通用那套，只有点一次「重置」才变成黑鲨派生值。
	// 放在这里，两种出厂形态（空表被补齐后的通用默认集 / 自带的百分比种子）都能落进迁移。
	// 判据里带黑鲨作用域：飞智档按 RPM 走时不能把它的默认 12 个值换成黑鲨那套。
	if isBlackSharkCurveScopeKey(runtimeDeviceKey) && upgradeBlackSharkManualGearRPM(&cfg, unit) {
		changed = true
	}
	if syncSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey) {
		changed = true
	}
	if storeDeviceFanCurveStateForKeyAndUnit(&cfg, runtimeDeviceKey, cfg, unit) {
		changed = true
	}
	if !changed {
		return cfg, false, nil
	}
	if err := a.commitConfigUpdate(cfg, nil); err != nil {
		return cfg, false, err
	}
	return cfg, true, nil
}

func (a *CoreApp) GetFanCurveProfiles() types.FanCurveProfilesPayload {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	unit := a.activeDeviceSpeedUnit(&cfg)
	runtimeDeviceKey := a.activeDeviceCurveScopeKey(cfg)
	changed := a.loadActiveRuntimeDeviceFanCurveState(&cfg, unit, true)
	if curveprofiles.NormalizeConfigForUnit(&cfg, unit) {
		changed = true
	}
	if storeDeviceFanCurveStateForKeyAndUnit(&cfg, runtimeDeviceKey, cfg, unit) {
		changed = true
	}
	if changed {
		if err := a.commitConfigUpdate(cfg, nil); err != nil {
			a.logError("保存温控曲线方案默认配置失败: %v", err)
		}
	}
	return a.fanCurveProfilesPayloadFromConfig(cfg)
}

func (a *CoreApp) SetActiveFanCurveProfile(profileID string) (types.FanCurveProfile, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	unit := a.activeDeviceSpeedUnit(&cfg)
	runtimeDeviceKey := a.activeDeviceCurveScopeKey(cfg)
	curveprofiles.NormalizeConfigForUnit(&cfg, unit)
	storeSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey)

	idx := curveprofiles.FindIndex(cfg.FanCurveProfiles, profileID)
	if idx < 0 {
		return types.FanCurveProfile{}, fmt.Errorf("未找到温控曲线方案")
	}

	cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[idx].ID
	cfg.FanCurve = curveprofiles.CloneCurve(cfg.FanCurveProfiles[idx].Curve)
	syncSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey)
	if err := a.applyCurveProfilesConfig(cfg); err != nil {
		return types.FanCurveProfile{}, err
	}
	// 切方案 == 切设备档位（四档散热模式就是四个曲线方案）。
	// 只对四档方案生效，且真正下发前还会再确认设备是黑鲨 ⇒ 不影响别的品牌。
	a.dispatchGearActivationForCurveProfile(cfg.ActiveFanCurveProfileID)
	return types.FanCurveProfile{
		ID:    cfg.FanCurveProfiles[idx].ID,
		Name:  cfg.FanCurveProfiles[idx].Name,
		Curve: curveprofiles.CloneCurve(cfg.FanCurveProfiles[idx].Curve),
	}, nil
}

func (a *CoreApp) CycleFanCurveProfile() (types.FanCurveProfile, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	unit := a.activeDeviceSpeedUnit(&cfg)
	runtimeDeviceKey := a.activeDeviceCurveScopeKey(cfg)
	curveprofiles.NormalizeConfigForUnit(&cfg, unit)
	storeSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey)

	if len(cfg.FanCurveProfiles) == 0 {
		return types.FanCurveProfile{}, fmt.Errorf("暂无可用温控曲线方案")
	}

	idx := max(curveprofiles.FindIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID), 0)
	nextIdx := (idx + 1) % len(cfg.FanCurveProfiles)
	cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[nextIdx].ID
	cfg.FanCurve = curveprofiles.CloneCurve(cfg.FanCurveProfiles[nextIdx].Curve)
	syncSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey)

	if err := a.applyCurveProfilesConfig(cfg); err != nil {
		return types.FanCurveProfile{}, err
	}
	// 同 SetActiveFanCurveProfile：循环切方案也要跟着切设备档位。
	a.dispatchGearActivationForCurveProfile(cfg.ActiveFanCurveProfileID)

	return types.FanCurveProfile{
		ID:    cfg.FanCurveProfiles[nextIdx].ID,
		Name:  cfg.FanCurveProfiles[nextIdx].Name,
		Curve: curveprofiles.CloneCurve(cfg.FanCurveProfiles[nextIdx].Curve),
	}, nil
}

func (a *CoreApp) SaveFanCurveProfile(params ipc.SaveFanCurveProfileParams) (types.FanCurveProfile, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	unit := a.activeDeviceSpeedUnit(&cfg)
	runtimeDeviceKey := a.activeDeviceCurveScopeKey(cfg)
	curveprofiles.NormalizeConfigForUnit(&cfg, unit)
	storeSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey)

	curve := curveprofiles.CloneCurve(params.Curve)
	if err := cfgpkg.ValidateFanCurveForUnit(curve, unit); err != nil {
		return types.FanCurveProfile{}, err
	}

	profileName := curveprofiles.NormalizeProfileName(params.Name, "新曲线")
	profileID := strings.TrimSpace(params.ID)
	idx := curveprofiles.FindIndex(cfg.FanCurveProfiles, profileID)
	if idx < 0 {
		profileID = curveprofiles.GenerateID()
		cfg.FanCurveProfiles = append(cfg.FanCurveProfiles, types.FanCurveProfile{
			ID:    profileID,
			Name:  profileName,
			Curve: curve,
		})
		idx = len(cfg.FanCurveProfiles) - 1
	} else {
		cfg.FanCurveProfiles[idx].Name = profileName
		cfg.FanCurveProfiles[idx].Curve = curve
	}

	if params.SetActive || cfg.ActiveFanCurveProfileID == cfg.FanCurveProfiles[idx].ID {
		cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[idx].ID
		cfg.FanCurve = curveprofiles.CloneCurve(cfg.FanCurveProfiles[idx].Curve)
		syncSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey)
	} else if _, ok := cfg.SmartControl.LearnedOffsetsByProfile[cfg.FanCurveProfiles[idx].ID]; !ok {
		// 预置长度也要走同一个长度所有者：黑鲨四档方案的插值依据会补两端端点（4→5..6 点）。
		// 写 len(curve)（4）的话，这条方案一旦被激活就会被 syncSmartControlOffsetsForDeviceKey
		// 改长，中间的翻转会丢掉已学偏移。用方案 ID + 该方案曲线判，不用 cfg（此刻它还不是活跃方案）。
		cfg.SmartControl.LearnedOffsetsByProfile[cfg.FanCurveProfiles[idx].ID] =
			make([]int, smartControlCurveLenFor(cfg.FanCurveProfiles[idx].ID, curve))
	}

	if err := a.applyCurveProfilesConfig(cfg); err != nil {
		return types.FanCurveProfile{}, err
	}

	updated := cfg.FanCurveProfiles[idx]
	// 四档方案：把这条曲线写进设备对应档位。
	// 只对四档方案 ID 生效，且真正下发前还会再确认设备是黑鲨 ⇒ 不影响别的品牌。
	a.dispatchGearCurveWriteForCurveProfile(updated.ID, updated.Curve)
	return types.FanCurveProfile{ID: updated.ID, Name: updated.Name, Curve: curveprofiles.CloneCurve(updated.Curve)}, nil
}

func (a *CoreApp) DeleteFanCurveProfile(profileID string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	unit := a.activeDeviceSpeedUnit(&cfg)
	runtimeDeviceKey := a.activeDeviceCurveScopeKey(cfg)
	curveprofiles.NormalizeConfigForUnit(&cfg, unit)
	storeSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey)

	if len(cfg.FanCurveProfiles) <= 1 {
		return fmt.Errorf("至少保留一个温控曲线方案")
	}

	idx := curveprofiles.FindIndex(cfg.FanCurveProfiles, profileID)
	if idx < 0 {
		return fmt.Errorf("未找到温控曲线方案")
	}

	cfg.FanCurveProfiles = append(cfg.FanCurveProfiles[:idx], cfg.FanCurveProfiles[idx+1:]...)
	deleteSmartControlOffsetsForProfile(&cfg, profileID)
	if len(cfg.FanCurveProfiles) == 0 {
		return fmt.Errorf("至少保留一个温控曲线方案")
	}

	if cfg.ActiveFanCurveProfileID == profileID {
		nextIdx := idx
		if nextIdx >= len(cfg.FanCurveProfiles) {
			nextIdx = len(cfg.FanCurveProfiles) - 1
		}
		cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[nextIdx].ID
		cfg.FanCurve = curveprofiles.CloneCurve(cfg.FanCurveProfiles[nextIdx].Curve)
		syncSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey)
	}

	return a.applyCurveProfilesConfig(cfg)
}

func (a *CoreApp) ExportFanCurveProfiles(profileIDs []string) (string, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	unit := a.activeDeviceSpeedUnit(&cfg)
	runtimeDeviceKey := a.activeDeviceCurveScopeKey(cfg)
	curveprofiles.NormalizeConfigForUnit(&cfg, unit)
	storeSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey)
	if idx := curveprofiles.FindIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID); idx >= 0 {
		cfg.FanCurveProfiles[idx].Curve = curveprofiles.CloneCurve(cfg.FanCurve)
	}

	return curveprofiles.ExportSelected(cfg.ActiveFanCurveProfileID, cfg.FanCurveProfiles, profileIDs)
}

func (a *CoreApp) ImportFanCurveProfiles(code string) error {
	profiles, activeID, err := curveprofiles.Import(code)
	if err != nil {
		return err
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	unit := a.activeDeviceSpeedUnit(&cfg)
	runtimeDeviceKey := a.activeDeviceCurveScopeKey(cfg)
	curveprofiles.NormalizeConfigForUnit(&cfg, unit)
	storeSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey)

	merged, importedActiveID := curveprofiles.AppendImportedProfiles(cfg.FanCurveProfiles, profiles, activeID)
	if importedActiveID == "" {
		return fmt.Errorf("导入数据中没有可用的曲线方案")
	}
	cfg.FanCurveProfiles = merged
	cfg.ActiveFanCurveProfileID = importedActiveID
	curveprofiles.NormalizeConfigForUnit(&cfg, unit)
	syncSmartControlOffsetsForDeviceKey(&cfg, runtimeDeviceKey)

	return a.applyCurveProfilesConfig(cfg)
}
