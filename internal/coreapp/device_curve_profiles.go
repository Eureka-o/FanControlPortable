package coreapp

import (
	"reflect"
	"strings"

	"github.com/TIANLI0/THRM/internal/curveprofiles"
	"github.com/TIANLI0/THRM/internal/types"
)

const deviceCurveScopeSeparator = "::"

func cloneDeviceFanCurveState(state types.DeviceFanCurveProfilesState) types.DeviceFanCurveProfilesState {
	return types.DeviceFanCurveProfilesState{
		Profiles: curveprofiles.CloneProfiles(state.Profiles),
		ActiveID: strings.TrimSpace(state.ActiveID),
		FanCurve: curveprofiles.CloneCurve(state.FanCurve),
	}
}

func cloneDeviceFanCurveStateMap(input map[string]types.DeviceFanCurveProfilesState) map[string]types.DeviceFanCurveProfilesState {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]types.DeviceFanCurveProfilesState, len(input))
	for key, state := range input {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			out[trimmed] = cloneDeviceFanCurveState(state)
		}
	}
	return out
}

func mergeDeviceFanCurveStateMaps(base, overlay map[string]types.DeviceFanCurveProfilesState) map[string]types.DeviceFanCurveProfilesState {
	out := cloneDeviceFanCurveStateMap(base)
	if len(overlay) == 0 {
		return out
	}
	if out == nil {
		out = map[string]types.DeviceFanCurveProfilesState{}
	}
	for key, state := range overlay {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			out[trimmed] = cloneDeviceFanCurveState(state)
		}
	}
	return out
}

func deviceCurveScopeKey(cfg types.AppConfig) string {
	types.NormalizeDeviceProfileConfig(&cfg)
	profile := types.ActiveDeviceProfile(&cfg)
	transport := types.NormalizeDeviceTransport(profile.Transport)
	profileID := strings.TrimSpace(profile.ID)
	if profileID == "" {
		profileID = types.ActiveDeviceProfileIDForTransport(&cfg, transport)
	}
	if profileID == "" {
		profileID = transport
	}
	if profileID == "" {
		return ""
	}
	return transport + deviceCurveScopeSeparator + profileID
}

func deviceCurveScopeKeyForTransportProfileID(transport, profileID string) string {
	transport = types.NormalizeDeviceTransport(transport)
	profileID = strings.TrimSpace(profileID)
	if transport == "" || profileID == "" {
		return ""
	}
	return transport + deviceCurveScopeSeparator + profileID
}

func defaultDeviceFanCurveStateForUnit(unit string) types.DeviceFanCurveProfilesState {
	curve := types.GetDefaultFanCurve()
	if types.IsRPMSpeedUnit(unit) {
		curve = types.GetDefaultRPMFanCurve()
	}
	name := "default"
	defaults := types.GetDefaultConfig(false)
	if len(defaults.FanCurveProfiles) > 0 && strings.TrimSpace(defaults.FanCurveProfiles[0].Name) != "" {
		name = defaults.FanCurveProfiles[0].Name
	}
	return types.DeviceFanCurveProfilesState{
		Profiles: []types.FanCurveProfile{{
			ID:    "default",
			Name:  name,
			Curve: curveprofiles.CloneCurve(curve),
		}},
		ActiveID: "default",
		FanCurve: curveprofiles.CloneCurve(curve),
	}
}

func captureDeviceFanCurveState(cfg types.AppConfig) types.DeviceFanCurveProfilesState {
	profiles := curveprofiles.CloneProfiles(cfg.FanCurveProfiles)
	activeID := strings.TrimSpace(cfg.ActiveFanCurveProfileID)
	curve := curveprofiles.CloneCurve(cfg.FanCurve)
	if idx := curveprofiles.FindIndex(profiles, activeID); idx >= 0 && len(curve) > 0 {
		profiles[idx].Curve = curveprofiles.CloneCurve(curve)
	}
	return types.DeviceFanCurveProfilesState{
		Profiles: profiles,
		ActiveID: activeID,
		FanCurve: curve,
	}
}

func normalizeDeviceFanCurveStateForUnit(cfg types.AppConfig, state types.DeviceFanCurveProfilesState, unit string) types.DeviceFanCurveProfilesState {
	tmp := cfg
	tmp.FanCurveProfiles = curveprofiles.CloneProfiles(state.Profiles)
	tmp.ActiveFanCurveProfileID = strings.TrimSpace(state.ActiveID)
	tmp.FanCurve = curveprofiles.CloneCurve(state.FanCurve)
	if len(tmp.FanCurve) == 0 && len(tmp.FanCurveProfiles) > 0 {
		activeIdx := curveprofiles.FindIndex(tmp.FanCurveProfiles, tmp.ActiveFanCurveProfileID)
		if activeIdx < 0 {
			activeIdx = 0
			tmp.ActiveFanCurveProfileID = tmp.FanCurveProfiles[0].ID
		}
		tmp.FanCurve = curveprofiles.CloneCurve(tmp.FanCurveProfiles[activeIdx].Curve)
	}
	curveprofiles.NormalizeConfigForUnit(&tmp, unit)
	return captureDeviceFanCurveState(tmp)
}

func normalizeDeviceFanCurveStateForConfig(cfg types.AppConfig, state types.DeviceFanCurveProfilesState) types.DeviceFanCurveProfilesState {
	return normalizeDeviceFanCurveStateForUnit(cfg, state, types.DeviceProfileSpeedUnit(&cfg))
}

func deviceFanCurveStateLooksDefaultForUnit(cfg types.AppConfig, state types.DeviceFanCurveProfilesState, unit string) bool {
	normalized := normalizeDeviceFanCurveStateForUnit(cfg, state, unit)
	defaultState := normalizeDeviceFanCurveStateForUnit(cfg, defaultDeviceFanCurveStateForUnit(unit), unit)
	return reflect.DeepEqual(normalized.FanCurve, defaultState.FanCurve)
}

func deviceFanCurveStateCandidateKeysForProfile(profile types.DeviceProfile) []string {
	profile = types.NormalizeDeviceProfile(profile, "")
	transport := types.NormalizeDeviceTransport(profile.Transport)
	added := map[string]bool{}
	keys := make([]string, 0, 4)
	add := func(key string) {
		key = strings.TrimSpace(key)
		if key == "" || added[key] {
			return
		}
		added[key] = true
		keys = append(keys, key)
	}

	add(deviceCurveScopeKeyForProfile(profile))
	if id := types.FlyDigiProfileIDForModel(profile.Model); id != "" {
		add(deviceCurveScopeKeyForTransportProfileID(transport, id))
	}
	if types.IsNativeDeviceTransport(transport) {
		add(deviceCurveScopeKeyForTransportProfileID(transport, types.LegacyRPMProfileID))
	}
	return keys
}

func selectDeviceFanCurveStateForProfile(cfg types.AppConfig, profile types.DeviceProfile, unit string) (types.DeviceFanCurveProfilesState, bool) {
	if cfg.FanCurveProfilesByDevice == nil {
		return types.DeviceFanCurveProfilesState{}, false
	}

	var exact types.DeviceFanCurveProfilesState
	hasExact := false
	for i, key := range deviceFanCurveStateCandidateKeysForProfile(profile) {
		state, ok := cfg.FanCurveProfilesByDevice[key]
		if !ok || len(state.Profiles) == 0 {
			continue
		}
		if i == 0 {
			exact = state
			hasExact = true
			if !deviceFanCurveStateLooksDefaultForUnit(cfg, state, unit) {
				return state, true
			}
			continue
		}
		if !deviceFanCurveStateLooksDefaultForUnit(cfg, state, unit) {
			return state, true
		}
	}
	if hasExact {
		return exact, true
	}

	transport := types.NormalizeDeviceTransport(profile.Transport)
	for key, state := range cfg.FanCurveProfilesByDevice {
		if len(state.Profiles) == 0 || !strings.HasPrefix(strings.TrimSpace(key), transport+deviceCurveScopeSeparator) {
			continue
		}
		if !deviceFanCurveStateLooksDefaultForUnit(cfg, state, unit) {
			return state, true
		}
	}
	return types.DeviceFanCurveProfilesState{}, false
}

func storeDeviceFanCurveStateForKeyAndUnit(cfg *types.AppConfig, key string, source types.AppConfig, unit string) bool {
	if cfg == nil || strings.TrimSpace(key) == "" {
		return false
	}
	if cfg.FanCurveProfilesByDevice == nil {
		cfg.FanCurveProfilesByDevice = map[string]types.DeviceFanCurveProfilesState{}
	}
	normalized := normalizeDeviceFanCurveStateForUnit(source, captureDeviceFanCurveState(source), unit)
	key = strings.TrimSpace(key)
	if reflect.DeepEqual(cfg.FanCurveProfilesByDevice[key], normalized) {
		return false
	}
	cfg.FanCurveProfilesByDevice[key] = normalized
	return true
}

func storeDeviceFanCurveStateForKey(cfg *types.AppConfig, key string, source types.AppConfig) bool {
	return storeDeviceFanCurveStateForKeyAndUnit(cfg, key, source, types.DeviceProfileSpeedUnit(&source))
}

func storeActiveDeviceFanCurveState(cfg *types.AppConfig) bool {
	if cfg == nil {
		return false
	}
	return storeDeviceFanCurveStateForKey(cfg, deviceCurveScopeKey(*cfg), *cfg)
}

func applyDeviceFanCurveStateForUnit(cfg *types.AppConfig, state types.DeviceFanCurveProfilesState, unit string) bool {
	if cfg == nil {
		return false
	}
	state = normalizeDeviceFanCurveStateForUnit(*cfg, state, unit)
	changed := false
	if !reflect.DeepEqual(cfg.FanCurveProfiles, state.Profiles) {
		cfg.FanCurveProfiles = curveprofiles.CloneProfiles(state.Profiles)
		changed = true
	}
	if cfg.ActiveFanCurveProfileID != state.ActiveID {
		cfg.ActiveFanCurveProfileID = state.ActiveID
		changed = true
	}
	if !reflect.DeepEqual(cfg.FanCurve, state.FanCurve) {
		cfg.FanCurve = curveprofiles.CloneCurve(state.FanCurve)
		changed = true
	}
	return changed
}

func applyDeviceFanCurveState(cfg *types.AppConfig, state types.DeviceFanCurveProfilesState) bool {
	return applyDeviceFanCurveStateForUnit(cfg, state, types.DeviceProfileSpeedUnit(cfg))
}

func loadDeviceFanCurveStateForProfile(cfg *types.AppConfig, profile types.DeviceProfile, unit string, useCurrentIfMissing bool) bool {
	if cfg == nil {
		return false
	}
	key := deviceCurveScopeKeyForProfile(profile)
	if key == "" {
		return false
	}
	if cfg.FanCurveProfilesByDevice == nil {
		cfg.FanCurveProfilesByDevice = map[string]types.DeviceFanCurveProfilesState{}
	}

	if state, ok := selectDeviceFanCurveStateForProfile(*cfg, profile, unit); ok {
		changed := applyDeviceFanCurveStateForUnit(cfg, state, unit)
		changed = storeDeviceFanCurveStateForKeyAndUnit(cfg, key, *cfg, unit) || changed
		return changed
	}
	return loadDeviceFanCurveStateForKey(cfg, key, unit, useCurrentIfMissing)
}

func loadDeviceFanCurveStateForKey(cfg *types.AppConfig, key string, unit string, useCurrentIfMissing bool) bool {
	if cfg == nil {
		return false
	}
	if key == "" {
		return false
	}
	if cfg.FanCurveProfilesByDevice == nil {
		cfg.FanCurveProfilesByDevice = map[string]types.DeviceFanCurveProfilesState{}
	}

	if state, ok := cfg.FanCurveProfilesByDevice[key]; ok && len(state.Profiles) > 0 {
		changed := applyDeviceFanCurveStateForUnit(cfg, state, unit)
		changed = storeDeviceFanCurveStateForKeyAndUnit(cfg, key, *cfg, unit) || changed
		return changed
	}

	state := defaultDeviceFanCurveStateForUnit(unit)
	if useCurrentIfMissing && (len(cfg.FanCurveProfiles) > 0 || len(cfg.FanCurve) > 0) {
		state = captureDeviceFanCurveState(*cfg)
	}
	changed := applyDeviceFanCurveStateForUnit(cfg, state, unit)
	changed = storeDeviceFanCurveStateForKeyAndUnit(cfg, key, *cfg, unit) || changed
	return changed
}

func loadActiveDeviceFanCurveState(cfg *types.AppConfig, useCurrentIfMissing bool) bool {
	if cfg == nil {
		return false
	}
	return loadDeviceFanCurveStateForKey(cfg, deviceCurveScopeKey(*cfg), types.DeviceProfileSpeedUnit(cfg), useCurrentIfMissing)
}

func runtimeCurveUseCurrentIfMissing(cfg *types.AppConfig, unit string, requested bool) bool {
	if !requested {
		return false
	}
	return types.NormalizeFanSpeedUnit(types.DeviceProfileSpeedUnit(cfg)) == types.NormalizeFanSpeedUnit(unit)
}

func (a *CoreApp) loadActiveRuntimeDeviceFanCurveState(cfg *types.AppConfig, unit string, useCurrentIfMissing bool) bool {
	if cfg == nil {
		return false
	}
	if profile, ok := a.connectedRuntimeDeviceProfile(); ok {
		return loadDeviceFanCurveStateForProfile(cfg, profile, unit, runtimeCurveUseCurrentIfMissing(cfg, unit, useCurrentIfMissing))
	}
	return loadDeviceFanCurveStateForKey(cfg, a.activeDeviceCurveScopeKey(*cfg), unit, useCurrentIfMissing)
}

func syncDeviceFanCurveStateForStartup(cfg *types.AppConfig) bool {
	return loadActiveDeviceFanCurveState(cfg, true)
}

func prepareDeviceFanCurveStateForUpdate(cfg *types.AppConfig, oldCfg types.AppConfig) bool {
	if cfg == nil {
		return false
	}
	changed := false
	cfg.FanCurveProfilesByDevice = mergeDeviceFanCurveStateMaps(oldCfg.FanCurveProfilesByDevice, cfg.FanCurveProfilesByDevice)
	oldKey := deviceCurveScopeKey(oldCfg)
	newKey := deviceCurveScopeKey(*cfg)
	if oldKey != "" {
		changed = storeDeviceFanCurveStateForKey(cfg, oldKey, oldCfg) || changed
	}
	if newKey != "" && newKey != oldKey {
		changed = loadActiveDeviceFanCurveState(cfg, false) || changed
	}
	return changed
}
