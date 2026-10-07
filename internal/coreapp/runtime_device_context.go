package coreapp

import (
	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/smartcontrol"
	"github.com/Eureka-o/FanControlPortable/internal/types"
	"strings"
)

func deviceCurveScopeKeyForProfile(profile types.DeviceProfile) string {
	transport := types.NormalizeDeviceTransport(profile.Transport)
	profileID := strings.TrimSpace(profile.ID)
	if profileID == "" {
		profileID = strings.TrimSpace(profile.Model)
	}
	if profileID == "" {
		profileID = strings.TrimSpace(profile.DisplayName)
	}
	if profileID == "" {
		profileID = transport
	}
	if profileID == "" {
		return ""
	}
	return transport + deviceCurveScopeSeparator + profileID
}

func (a *CoreApp) connectedRuntimeDeviceProfile() (types.DeviceProfile, bool) {
	if a == nil || a.deviceManager == nil || !a.deviceManager.IsConnected() {
		return types.DeviceProfile{}, false
	}
	profile := types.NormalizeDeviceProfile(a.deviceManager.ActiveProfile(), "")
	if strings.TrimSpace(profile.ID) == "" && strings.TrimSpace(profile.DisplayName) == "" {
		return types.DeviceProfile{}, false
	}
	return profile, true
}

func (a *CoreApp) activeDeviceSpeedUnit(cfg *types.AppConfig) string {
	if profile, ok := a.connectedRuntimeDeviceProfile(); ok {
		if unit := strings.TrimSpace(profile.SpeedUnit); unit != "" {
			return types.NormalizeFanSpeedUnit(unit)
		}
		if unit := strings.TrimSpace(profile.Capabilities.SpeedUnit); unit != "" {
			return types.NormalizeFanSpeedUnit(unit)
		}
	}
	if a != nil && a.deviceManager != nil && a.deviceManager.IsConnected() {
		if fanData := a.deviceManager.GetCurrentFanData(); fanData != nil {
			if unit := strings.TrimSpace(fanData.SpeedUnit); unit != "" {
				return types.NormalizeFanSpeedUnit(unit)
			}
		}
	}
	return types.DeviceProfileSpeedUnit(cfg)
}

func (a *CoreApp) activeDeviceCurveScopeKey(cfg types.AppConfig) string {
	if profile, ok := a.connectedRuntimeDeviceProfile(); ok {
		if key := deviceCurveScopeKeyForProfile(profile); key != "" {
			return key
		}
	}
	return deviceCurveScopeKey(cfg)
}

// blackSharkGearInterpolationFor 是「给定方案 ID + 曲线是否属于黑鲨四档插值模式」的唯一判据。
// 两个条件缺一不可：方案 ID 必须是黑鲨档位、曲线长度必须等于插值点数，否则会误伤其他设备。
func blackSharkGearInterpolationFor(profileID string, curve []types.FanCurvePoint) bool {
	if _, ok := types.BlackSharkGearForCurveProfileID(profileID); !ok {
		return false
	}
	return len(curve) == deviceproto.BlackSharkCurvePointCount
}

// blackSharkGearInterpolationActive 是上面判据在当前生效方案上的取值。
func blackSharkGearInterpolationActive(cfg *types.AppConfig) bool {
	if cfg == nil {
		return false
	}
	return blackSharkGearInterpolationFor(cfg.ActiveFanCurveProfileID, cfg.FanCurve)
}

// smartControlCurveLenFor 返回给定方案的智能控温曲线长度（= 插值依据的长度）。
// 学习偏移数组的长度、稳定观察者的桶数都按这个数分配。
func smartControlCurveLenFor(profileID string, curve []types.FanCurvePoint) int {
	if !blackSharkGearInterpolationFor(profileID, curve) {
		return len(curve)
	}
	points := make([]deviceproto.BlackSharkInterpolationPoint, 0, len(curve))
	for _, point := range curve {
		points = append(points, deviceproto.BlackSharkInterpolationPoint{
			Temperature: point.Temperature,
			RPM:         point.RPM,
		})
	}
	return len(deviceproto.BlackSharkEffectiveCurveForInterpolation(points))
}

// smartControlCurveLen 是上面函数在当前生效方案上的取值。
func smartControlCurveLen(cfg *types.AppConfig) int {
	if cfg == nil {
		return 0
	}
	return smartControlCurveLenFor(cfg.ActiveFanCurveProfileID, cfg.FanCurve)
}

// smartControlCurveForUnit 返回智能控温插值真正该用的那条曲线。
// 它是智能控温插值曲线来源的唯一所有者。
func (a *CoreApp) smartControlCurveForUnit(cfg *types.AppConfig, unit string) []types.FanCurvePoint {
	if cfg == nil {
		return nil
	}
	if !blackSharkGearInterpolationActive(cfg) {
		return smartcontrol.CurveForUnit(cfg.FanCurve, unit)
	}
	points := make([]deviceproto.BlackSharkInterpolationPoint, 0, len(cfg.FanCurve))
	for _, point := range cfg.FanCurve {
		points = append(points, deviceproto.BlackSharkInterpolationPoint{
			Temperature: point.Temperature,
			RPM:         point.RPM,
		})
	}
	effective := deviceproto.BlackSharkEffectiveCurveForInterpolation(points)
	curve := make([]types.FanCurvePoint, 0, len(effective))
	for _, point := range effective {
		curve = append(curve, types.FanCurvePoint{Temperature: point.Temperature, RPM: point.RPM})
	}
	return smartcontrol.CurveForUnit(curve, unit)
}
