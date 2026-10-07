// 黑鲨 BRB02 的控制面：IPC 分发、四档曲线、LCD 与系统信息推送，以及主机侧灯效驱动与情景规则。
package coreapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"  // 注册 GIF 解码器，供 image.Decode 使用
	_ "image/jpeg" // 注册 JPEG 解码器，供 image.Decode 使用
	_ "image/png"  // 注册 PNG 解码器，供 image.Decode 使用
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/bsaudio"
	"github.com/Eureka-o/FanControlPortable/internal/bspress"
	"github.com/Eureka-o/FanControlPortable/internal/config"
	"github.com/Eureka-o/FanControlPortable/internal/curveprofiles"
	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/scene"
	"github.com/Eureka-o/FanControlPortable/internal/screenimg"
	"github.com/Eureka-o/FanControlPortable/internal/smartcontrol"
	"github.com/Eureka-o/FanControlPortable/internal/temperature"
	"github.com/Eureka-o/FanControlPortable/internal/types"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

// blackSharkLocalOnlyRequests 是不依赖当前设备的黑鲨请求：静态标定表、纯本机配置与本地缓存。
// 未连接设备时面板仍要靠它们渲染，因此不做设备族门控。
var blackSharkLocalOnlyRequests = map[ipc.RequestType]bool{
	ipc.ReqGetBlackSharkFirmwareStatus:    true,
	ipc.ReqGetBlackSharkRgbCache:          true,
	ipc.ReqGetBlackSharkHostEffects:       true,
	ipc.ReqGetBlackSharkManualGearPresets: true,
	ipc.ReqGetBlackSharkCurveTempRange:    true,
	ipc.ReqGetBlackSharkLcdDisplay:        true,
}

func (a *CoreApp) handleBlackSharkIPCRequest(req ipc.Request) (ipc.Response, bool) {
	// 黑鲨命令与飞智共用同一个 HID 句柄，入口先按档案把非黑鲨设备挡在外面：
	// 设备层那些方法只会静默返回零值，前端拿到后无法区分"设备不支持"与"写入失败"。
	if !blackSharkLocalOnlyRequests[req.Type] {
		if a.deviceManager == nil || !a.deviceManager.IsBlackSharkProfileActive() {
			return a.errorResponse("当前设备不是黑鲨 BRB02，无法执行该命令"), true
		}
	}
	switch req.Type {
	case ipc.ReqGetBlackSharkInfo:
		// 会向设备发起多次查询，仅在用户主动刷新时调用。
		return a.dataResponse(a.deviceManager.BlackSharkInfo()), true

	case ipc.ReqCheckBlackSharkFirmwareUpdate:
		// 主动查一次固件版本（设备 0x01 + 官方清单，清单 6s 超时）。
		// 只比对版本，不下载、不刷写；失败只体现在返回的 Error 字段里，不影响设备控制。
		return a.dataResponse(a.CheckBlackSharkFirmwareUpdate()), true

	case ipc.ReqGetBlackSharkFirmwareStatus:
		// 只读缓存：一次设备/网络 IO 都不做。
		return a.dataResponse(a.GetBlackSharkFirmwareStatus()), true

	case ipc.ReqSetBlackSharkLightingEnabled:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		return a.successResponse(a.deviceManager.SetBlackSharkLightingEnabled(params.Enabled)), true

	case ipc.ReqSetBlackSharkLcdScreenEnabled:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		return a.successResponse(a.deviceManager.SetBlackSharkLcdScreenEnabled(params.Enabled)), true

	case ipc.ReqSetBlackSharkOnOffVector:
		var params ipc.BlackSharkOnOffVectorParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		// 0x02 无读回命令，返回 true 仅代表写入成功。
		return a.successResponse(a.deviceManager.SetBlackSharkOnOffVector(
			params.SmartStartStop, params.PowerOnSelfStart)), true

	case ipc.ReqGetBlackSharkRgbLighting:
		// 开关状态 + 全部 8 个模式的参数；会向设备发起多次查询，仅按需调用。
		return a.dataResponse(a.blackSharkRgbLighting()), true

	case ipc.ReqGetBlackSharkRgbCache:
		// 只读本机缓存，一次设备 IO 都不做：打开面板时先显示上次读到的那份，
		// 免得必须先点一次「读取灯效」；要实时值就点它（走 ReqGetBlackSharkRgbLighting）。
		return a.dataResponse(a.blackSharkRgbCached()), true

	case ipc.ReqGetBlackSharkHostEffects:
		// 主机侧驱动的状态（槽位 6 响应 / 槽位 7 音频同步）：纯内存读取、不发查询，
		// 可与灯效页一起刷新。
		return a.dataResponse(a.BlackSharkHostEffects()), true

	case ipc.ReqGetBlackSharkManualGearPresets:
		// 手动挡位面板的转速预设由标定表派生（唯一所有者 = deviceproto 的标定表文件），
		// 纯计算、不碰设备，未连设备也能取，前端不再硬编码这 12 个数。
		return a.dataResponse(deviceproto.BlackSharkManualGearPresets()), true

	case ipc.ReqGetBlackSharkCurveTempRange:
		// 曲线可拖点的温度取值域，所有者同样是 deviceproto；纯计算、不碰设备。
		return a.dataResponse(types.BlackSharkCurveTempRange{
			MinTempC: deviceproto.BlackSharkCurvePointMinTempC,
			MaxTempC: deviceproto.BlackSharkCurvePointMaxTempC,
		}), true

	case ipc.ReqSelectBlackSharkRgbMode:
		var params ipc.SetIntParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		return a.successResponse(a.deviceManager.SelectBlackSharkRgbMode(params.Value)), true

	case ipc.ReqSetBlackSharkRgbModeEffects:
		var params ipc.BlackSharkRgbModeEffectParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		return a.successResponse(a.deviceManager.SetBlackSharkRgbModeEffects(
			params.Index, params.Speed, params.Brightness)), true

	case ipc.ReqSetBlackSharkRgbModeColor:
		var params ipc.BlackSharkRgbColorParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		// 色相 → RGB 只有一处实现（deviceproto，等价于官方的 QColor::setHsv）。
		r, g, b := deviceproto.BlackSharkHueToColor(params.Hue)
		return a.successResponse(a.deviceManager.SetBlackSharkRgbModeColor(
			params.Index, r, g, b, params.StaticColor)), true

	case ipc.ReqSetBlackSharkRgbColorOption:
		var params ipc.BlackSharkRgbColorOptionParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		// 「颜色模式」的第二维（官方 UI 那个 5 项下拉）；越界由 device 层拒绝并记日志。
		return a.successResponse(a.deviceManager.SetBlackSharkRgbModeColorOption(
			params.Index, params.Option)), true

	case ipc.ReqGetBlackSharkLcdDisplay:
		return a.dataResponse(a.blackSharkLcdDisplay()), true

	case ipc.ReqSetBlackSharkLcdDisplay:
		var params ipc.BlackSharkLcdDisplayParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if params.Pos < 0 || params.Pos > types.BlackSharkLcdPosMax {
			return a.errorResponse(fmt.Sprintf(
				"显示位置 %d 目前不可用：只有 0 做过真机标定（0..%d 是官方取值域，"+
					"但 1..3 的每格几何尚无样本，用 0 的几何代填等于猜测）",
				params.Pos, types.BlackSharkLcdPosMax)), true
		}
		items, _ := types.NormalizeBlackSharkLcdItems(params.Items)
		if len(items) != types.BlackSharkLcdItemCount {
			return a.errorResponse(fmt.Sprintf("需要恰好 %d 项显示参数", types.BlackSharkLcdItemCount)), true
		}
		return a.dataResponse(a.setBlackSharkLcdDisplay(params.Pos, items)), true

	case ipc.ReqGetBlackSharkConfigSnapshot:
		snap, ok := a.deviceManager.BlackSharkConfigSnapshot()
		if !ok {
			return a.errorResponse("设备未就绪，无法读取配置快照"), true
		}
		return a.dataResponse(snap), true

	case ipc.ReqRestoreBlackSharkConfig:
		var params ipc.BlackSharkRestoreConfigParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析快照失败: " + err.Error()), true
		}
		restored, issues := a.deviceManager.RestoreBlackSharkConfig(params.Snapshot)
		if restored == nil {
			restored = []string{}
		}
		if issues == nil {
			issues = []string{}
		}
		// 快照还原会把各档位的曲线写回设备（见 RestoreBlackSharkConfig），因此无需再补推承载：
		// 主机不再下发转速，还原本身就是设备侧的唯一写入点。
		return a.dataResponse(types.BlackSharkConfigRestoreResult{Restored: restored, Issues: issues}), true

	case ipc.ReqResetBlackSharkRgb:
		// 官方「灯效页 · 重置」：9 条 0x12 常量（8 槽位 + 把模式 1 设回当前）。
		// 会同时重置未选中的槽位、并让当前模式跳回「彩色流动」，这是官方自身行为。
		return a.successResponse(a.deviceManager.ResetBlackSharkRgbModes()), true

	case ipc.ReqResetBlackSharkCooling:
		// 「一键恢复四档出厂曲线」：四个档位都走「恢复该档出厂曲线」的同一条路
		// （主机侧方案曲线 ← 出厂常量，并把该档的出厂曲线写进设备）。
		// 设备侧保存的是出厂曲线本身，而不是平线承载：形状活在设备里，主机只推温度。
		reset := a.resetAllBlackSharkGearCurves()
		return a.successResponse(reset), true

	default:
		return ipc.Response{}, false
	}
}

// syncBlackSharkTempSource 把本机「温度来源」同步给散热器的温控参照（0x22）。
func (a *CoreApp) syncBlackSharkTempSource(cfg types.AppConfig) {
	if a.deviceManager == nil || !a.deviceManager.IsBlackSharkActive() {
		return
	}
	var source uint8
	switch types.NormalizeTempSource(cfg.TempSource) {
	case types.TempSourceCPU:
		source = 0
	case types.TempSourceGPU:
		source = 1
	default:
		// max 不在这里处理：它要按 CPU/GPU 实时比较（还带迟滞与最小间隔），
		// 由监控循环的 `syncBlackSharkCoolingSourceForMax` 独占负责（同频挂在 0x07 推送旁）。
		a.logDebug("黑鲨温控参照交给监控循环处理：本机温度来源为 %q（max 需按实时温度动态切换）", cfg.TempSource)
		return
	}
	if !a.deviceManager.SetBlackSharkCoolingSource(source) {
		a.logDebug("黑鲨温控参照同步失败（0x22=%d）", source)
		return
	}
	// 与监控循环那路共用同一个去重记录，避免两条路各写一次或互相打架。
	a.blackSharkSourceWritten.Store(int32(source))
	a.blackSharkSourceLastAt.Store(time.Now().UnixNano())
}

// 黑鲨 max（取 CPU/GPU 较高者）的两个待定标参数。
//
// 这两条都是待标定的初值，不是结论：切 0x22 会不会重置设备那个 ≈40s 的曲线应用窗口仍未知，
// 若会，频繁切换会造成转速抖动；官方 UI 里「更高项」只是一句说明文字，也没有可抄的取值。
// 因此宁可少切：迟滞 3℃、最小间隔 30s，标定后再定。
const (
	// blackSharkMaxSwitchMarginC 切换迟滞：对方温度要高出这么多才切参照源。
	blackSharkMaxSwitchMarginC = 3
	// blackSharkMaxSwitchMinInterval 两次切换的最小间隔。
	blackSharkMaxSwitchMinInterval = 30 * time.Second
)

// syncBlackSharkCoolingSourceForMax 维持"设备参照源"与用户选择一致。
//
// 三种用户选择：cpu / gpu 目标固定，与上次写下去的值不同才写（syncBlackSharkTempSource 也做这件事，
// 两条路都以 blackSharkSourceWritten 去重，不会互相打架）；max 取 CPU/GPU 较高者，
// 但带迟滞与最小间隔，避免两温度接近时来回抖。
//
// max 只能由主机实现：设备只有两态（写 2/3 读回 1），而「把大值写进两个温度字段」会污染小屏
// （0xC2 默认三项就是 id0/id1，官方帧也从没把两字段写相等）。唯一不污染显示的做法就是动态切参照源。
func (a *CoreApp) syncBlackSharkCoolingSourceForMax(cfg types.AppConfig, cpuTemp, gpuTemp int) {
	if a == nil || a.deviceManager == nil || !a.deviceManager.IsBlackSharkActive() {
		return
	}
	if !a.autoControlActive() {
		return // 固定 / 手动不读温度 ⇒ 参照源无意义
	}
	cur := a.blackSharkSourceWritten.Load()
	want := int32(-1)
	switch types.NormalizeTempSource(cfg.TempSource) {
	case types.TempSourceCPU:
		want = 0
	case types.TempSourceGPU:
		want = 1
	default: // max（本机默认值）
		if cpuTemp <= 0 && gpuTemp <= 0 {
			return // 两个温度都没读到：不猜，保持现状
		}
		switch {
		case cur < 0: // 首次：直接选更高的那一边（没有迟滞可言）
			if gpuTemp > cpuTemp {
				want = 1
			} else {
				want = 0
			}
		case cur == 0 && gpuTemp >= cpuTemp+blackSharkMaxSwitchMarginC:
			want = 1
		case cur == 1 && cpuTemp >= gpuTemp+blackSharkMaxSwitchMarginC:
			want = 0
		default:
			return // 没越过迟滞：保持现状
		}
		if want != cur {
			// 越过迟滞了，但可能还没到最小间隔 ⇒ 这一拍先不切。
			if last := a.blackSharkSourceLastAt.Load(); last != 0 &&
				time.Since(time.Unix(0, last)) < blackSharkMaxSwitchMinInterval {
				return
			}
		}
	}
	if want < 0 || want == cur {
		return // 没变：一个字节都不发
	}
	if !a.deviceManager.SetBlackSharkCoolingSource(uint8(want)) {
		a.logError("黑鲨参照源同步失败（想要 %d，当前记录 %d）", want, cur)
		return
	}
	a.blackSharkSourceWritten.Store(want)
	a.blackSharkSourceLastAt.Store(time.Now().UnixNano())
	label := "CPU"
	if want == 1 {
		label = "GPU"
	}
	a.logInfo("黑鲨参照源切到 %s（CPU %d℃ / GPU %d℃）", label, cpuTemp, gpuTemp)
}

// ---- 四档：方案曲线 / 切档 / 出厂恢复 / 迁移 ----

var blackSharkGearProfileNames = [deviceproto.BlackSharkGearCount]string{"低噪", "平衡", "强效", "超频"}

// syncBlackSharkGearProfileCurve 把该档位的出厂曲线同步进本机保存的方案里，并报告是否真的写进去了。
//
// 作用域键取当前相连设备的键（activeDeviceCurveScopeKey），不能钉在某个档案上：
// 黑鲨有 BLE 与 USB 两条档案，键不同（形如 hid::… / usb::…）。
// 钉死在某个档案上会写进没人读的桶，表现为"恢复出厂曲线后界面没有变化"而日志仍报成功。
func (a *CoreApp) syncBlackSharkGearProfileCurve(gear int) bool {
	if a == nil || a.configManager == nil {
		return false
	}
	factory, ok := deviceproto.BlackSharkFactoryGearConfig(deviceproto.BlackSharkFormCurve, byte(gear))
	if !ok {
		a.logError("同步出厂曲线失败：没有档位 %d 的出厂常量", gear)
		return false
	}
	key := a.activeDeviceCurveScopeKey(a.configManager.Get())
	if !isBlackSharkCurveScopeKey(key) {
		// 不是黑鲨档案却走到这里：什么都不做，也不谎报成功（宁可让调用方看到 false）。
		a.logError("同步档位 %d 的出厂曲线失败：当前设备作用域键 %q 不是黑鲨档案", gear, key)
		return false
	}
	profileID := types.BlackSharkGearCurveProfileID(gear)
	curve := make([]types.FanCurvePoint, 0, len(factory.Curve))
	for _, point := range factory.Curve {
		curve = append(curve, types.FanCurvePoint{Temperature: int(point.TempC), RPM: int(point.RPM)})
	}
	wrote := false
	if _, err := a.configManager.MutateAndSave(func(cfg *types.AppConfig) {
		state, hasState := cfg.FanCurveProfilesByDevice[key]
		if !hasState {
			return
		}
		idx := curveprofiles.FindIndex(state.Profiles, profileID)
		if idx < 0 {
			return
		}
		state.Profiles[idx].Curve = curveprofiles.CloneCurve(curve)
		state.ActiveID = profileID
		state.FanCurve = curveprofiles.CloneCurve(curve)
		cfg.FanCurveProfilesByDevice[key] = state
		// 曲线页读的是这三个字段 ⇒ 必须一起同步，否则界面还是旧的。
		cfg.FanCurveProfiles = curveprofiles.CloneProfiles(state.Profiles)
		cfg.ActiveFanCurveProfileID = profileID
		cfg.FanCurve = curveprofiles.CloneCurve(curve)
		wrote = true
	}); err != nil {
		a.logError("同步出厂曲线到本机方案失败: %v", err)
		return false
	}
	if !wrote {
		// 键对了但桶里没有这套四档方案（或没有该档）：如实报，别让调用方以为改好了。
		a.logError("同步档位 %d 的出厂曲线失败：作用域键 %q 下没有该档方案（本机状态未播种？）", gear, key)
		return false
	}
	a.logInfo("已把档位 %d 的出厂曲线同步到本机方案（设备与界面现在是同一条）", gear)
	return true
}

// resetAllBlackSharkGearCurves 把四个档位的方案曲线都恢复成出厂值，并把出厂曲线写进设备。
//
// 写 0x24 的档位字段就是"切档"（切档写 0x24 即生效），逐槽写四遍必然停在最后写入的那个槽
// （超频）。所以进循环前先记下当前档位，循环后不仅要把主机侧方案拨回去，还要把**设备**也拨回来 ——
// 只恢复主机侧状态时，设备留在超频上按超频那条曲线走，症状就是"变频下按一次重置直接顶满转速"。
func (a *CoreApp) resetAllBlackSharkGearCurves() bool {
	if a == nil || a.deviceManager == nil || !a.deviceManager.IsBlackSharkActive() {
		return false
	}
	previous := a.activeBlackSharkGear()
	ok := true
	for gear := 1; gear <= deviceproto.BlackSharkGearCount; gear++ {
		if !a.syncBlackSharkGearProfileCurve(gear) {
			ok = false
		}
		if !a.applyBlackSharkGear(gear) {
			ok = false
		}
	}
	if previous >= 1 && previous <= deviceproto.BlackSharkGearCount {
		if _, err := a.SetActiveFanCurveProfile(types.BlackSharkGearCurveProfileID(previous)); err != nil {
			a.logError("恢复四档出厂曲线后切回原档位失败: %v", err)
			ok = false
		}
	}
	// 把设备从"循环最后写下的那个槽"拨回来：变频 = 重写该档曲线；固定 = 重写当前手动挡位。
	switch {
	case a.autoControlActive() && previous >= 1 && previous <= deviceproto.BlackSharkGearCount:
		if !a.applyBlackSharkGear(previous) {
			a.logError("恢复四档出厂曲线后把设备拨回档位 %d 失败", previous)
			ok = false
		}
	case a.autoControlActive():
		// 当前不是四档方案（自定义方案/未选中）：没有档位可拨回，如实说清设备停在超频。
		a.logInfo("恢复四档出厂曲线：当前方案不是四档之一，设备停在最后写入的槽位；请选一次档位以复位")
	default:
		if err := a.applyCurrentGearSetting(); err != nil {
			a.logError("恢复四档出厂曲线后重写当前挡位失败: %v", err)
			ok = false
		}
	}
	return ok
}

// activeBlackSharkGear 返回当前应作用于设备的黑鲨档位（1..4）；不是四档方案时返回 0。
// 真值来源是当前曲线方案 ID（四档方案 ↔ 档位，与「切方案=切档位」同一套映射，单一所有者）。
func (a *CoreApp) activeBlackSharkGear() int {
	if a == nil || a.configManager == nil {
		return 0
	}
	gear, ok := types.BlackSharkGearForCurveProfileID(a.configManager.Get().ActiveFanCurveProfileID)
	if !ok {
		return 0
	}
	return gear
}

// blackSharkDeviceCurvePoints 组装「该档要下发到设备的 4 点」。
//
// 主机把曲线本身写进设备（形状活在设备里），而不是把插值结果当转速下发。
//
// 口径（不要在别处重写）：
//
//	· 曲线来源 = 该档的曲线方案，不是 `cfg.FanCurve`（那只镜像"当前生效"的那一个方案）；
//	· 先补端点得到「插值依据」再烘焙偏移：偏移数组是按依据的点序分配的
//	  （见 `smartcontrol.EffectiveCurveForUnit` 的口径说明），直接喂用户 4 点会让 offsets 整体错位；
//	· 烘焙与取值都走现有所有者：`smartcontrol.EffectiveCurveForUnit` +
//	  `temperature.CalculateTargetRPM`（后者就是主机插值用的同一个求值器）；
//	· 曲线字段按 RPM 直接写（长保持曲线路径 ≈ 恒等，Field ≈ RPM，误差 ≤15）。
func (a *CoreApp) blackSharkDeviceCurvePoints(gear int) ([deviceproto.BlackSharkCurvePointCount]deviceproto.BlackSharkCurvePoint, bool) {
	var out [deviceproto.BlackSharkCurvePointCount]deviceproto.BlackSharkCurvePoint
	if a == nil || a.configManager == nil {
		return out, false
	}
	cfg := a.configManager.Get()
	idx := curveprofiles.FindIndex(cfg.FanCurveProfiles, types.BlackSharkGearCurveProfileID(gear))
	if idx < 0 {
		a.logError("组装下发曲线失败：找不到档位 %d 的曲线方案", gear)
		return out, false
	}
	curve := cfg.FanCurveProfiles[idx].Curve
	if len(curve) != deviceproto.BlackSharkCurvePointCount {
		a.logError("组装下发曲线失败：档位 %d 的方案有 %d 个点（需要 %d 个）",
			gear, len(curve), deviceproto.BlackSharkCurvePointCount)
		return out, false
	}
	points := make([]deviceproto.BlackSharkInterpolationPoint, 0, len(curve))
	for _, p := range curve {
		points = append(points, deviceproto.BlackSharkInterpolationPoint{Temperature: p.Temperature, RPM: p.RPM})
	}
	basis := deviceproto.BlackSharkEffectiveCurveForInterpolation(points)
	interp := make([]types.FanCurvePoint, 0, len(basis))
	for _, p := range basis {
		interp = append(interp, types.FanCurvePoint{Temperature: p.Temperature, RPM: p.RPM})
	}
	effective := smartcontrol.EffectiveCurveForUnit(interp, cfg.SmartControl, a.activeDeviceSpeedUnit(&cfg))
	for i, p := range curve {
		rpm := temperature.CalculateTargetRPM(p.Temperature, effective)
		if rpm <= 0 {
			a.logError("组装下发曲线失败：档位 %d 第 %d 点（%d℃）在有效曲线上取不到值",
				gear, i+1, p.Temperature)
			return out, false
		}
		out[i] = deviceproto.BlackSharkCurvePoint{
			TempC: uint8(p.Temperature),
			RPM:   uint16(deviceproto.ClampBlackSharkCurveRPM(rpm)),
		}
	}
	return out, true
}

// applyBlackSharkGear 把「某一档」施加到设备上：变频写该档曲线，固定写该档固定值。
//
// 切档 = 写该档的 `0x24`（`0x26` 是读、不是选档），因此 device 层原来的
// `ActivateBlackSharkGear`（写平承载/排队）与 `EnsureBlackSharkGearCarrier` 都不再需要；
// "该写哪条曲线"的决策上移到本层（曲线方案与学习偏移都在本层）。
func (a *CoreApp) applyBlackSharkGear(gear int) bool {
	if a == nil || a.deviceManager == nil {
		return false
	}
	if !types.IsBlackSharkDeviceProfileID(a.deviceManager.ActiveProfile().ID) {
		return false // 不是黑鲨：一个字节都不发
	}
	if gear < 1 || gear > deviceproto.BlackSharkGearCount {
		a.logError("施加黑鲨档位失败：档位 %d 越界（有效 1..%d）", gear, deviceproto.BlackSharkGearCount)
		return false
	}
	if !a.autoControlActive() {
		// 固定路径：form=0，写到该档（不再写死 gear=1）。
		return a.applyManualGearForCurveProfile(gear)
	}
	points, ok := a.blackSharkDeviceCurvePoints(gear)
	if !ok {
		return false
	}
	if !a.deviceManager.SetBlackSharkCoolingCurve(byte(gear), points) {
		a.logError("施加黑鲨档位 %d 失败：曲线写入未获回读确认", gear)
		return false
	}
	a.logInfo("黑鲨档位 %d 已施加：设备侧曲线 = %s（含已学偏移，含端点补齐后的插值依据烘焙）",
		gear, deviceproto.BlackSharkCurveString(points))
	return true
}

// reassertBlackSharkInverterGear 在「变频刚刚生效」的时刻把该档曲线写进设备。
//
// 变频下主机不再下发转速（曲线形状写在设备里），所以每个"变频生效"的入口都必须显式补一次
// 曲线写入：开启自动控制（config_control）、温度遥测恢复 / 自动控制刚生效（monitoring）、
// 连接后（system_device 的 postConnectSync）。漏掉任一入口，设备会停在 `form=0`（固定转速）
// 形态，而界面以为已经在变频。
//
// 异步执行：三个调用点分别来自配置提交线程、监控采样线程与连接协程，
// 设备 I/O（0x25 读 + 0x24 写 + 回读）不能占用它们。
func (a *CoreApp) reassertBlackSharkInverterGear(caller string) {
	if a == nil || a.deviceManager == nil {
		return
	}
	if !a.deviceManager.IsBlackSharkActive() || !a.autoControlActive() {
		return
	}
	gear := a.activeBlackSharkGear()
	if gear < 1 {
		return
	}
	a.safeGo("reassertBlackSharkInverterGear", func() {
		if a.applyBlackSharkGear(gear) {
			a.logInfo("变频已生效（%s）：已把档位 %d 的曲线写进设备", caller, gear)
		} else {
			a.logError("变频已生效（%s），但把档位 %d 的曲线写进设备失败", caller, gear)
		}
	})
}

// upgradeBlackSharkGearProfiles 一次性迁移：黑鲨键下若还存着通用默认（只有一个「默认」方案），
// 就地升级成四档 = 四个曲线方案。
func upgradeBlackSharkGearProfiles(cfg *types.AppConfig, key, unit string) bool {
	if cfg == nil || !isBlackSharkCurveScopeKey(key) {
		return false
	}
	state, ok := cfg.FanCurveProfilesByDevice[key]
	if !ok || len(state.Profiles) == 0 {
		return false // 没有存量（首次播种走 loadDeviceFanCurveStateForKey）
	}
	if hasBlackSharkGearProfile(state.Profiles) {
		return false // 已迁移过
	}
	if !deviceFanCurveStateLooksDefaultForUnit(*cfg, state, unit) {
		return false // 用户改过 ⇒ 不覆盖
	}
	// 优先继承退役的 HID 档案桶里那套四档方案：
	// 活跃档案从 HID 换成 USB/BLE（作用域键 <transport>::<profileID> 跟着换）后，
	// 老用户改过的档位曲线留在旧桶里、新桶是空的，这里不搬就会"升级即静默重置成出厂值"。
	// 只搬一次：搬进新桶后 hasBlackSharkGearProfile 就为真，本函数不会再进。
	gearState, seeded := upgradeBlackSharkGearProfilesFromLegacy(cfg, key)
	if !seeded {
		gearState, seeded = blackSharkDefaultFanCurveState(key)
	}
	if !seeded {
		return false
	}
	if cfg.FanCurveProfilesByDevice == nil {
		cfg.FanCurveProfilesByDevice = map[string]types.DeviceFanCurveProfilesState{}
	}
	cfg.FanCurveProfilesByDevice[key] = gearState
	return true
}

// upgradeBlackSharkGearProfilesFromLegacy 从退役的 HID 档案那个作用域键下取一套四档方案。
//
// 只在那一桶里确实有四档方案（判据 = 档位方案 ID 前缀）时才认，`key` 自己就是旧键时直接放弃
// （避免自己搬自己）；旧桶不删：既不必要（搬完新桶已有四档方案，迁移不会重跑），
// 也留着当升级前的原始记录。
func upgradeBlackSharkGearProfilesFromLegacy(cfg *types.AppConfig, key string) (types.DeviceFanCurveProfilesState, bool) {
	if cfg == nil {
		return types.DeviceFanCurveProfilesState{}, false
	}
	legacyKey := deviceCurveScopeKeyForProfile(types.BlackSharkFengShenProProfile())
	if legacyKey == "" || legacyKey == key {
		return types.DeviceFanCurveProfilesState{}, false
	}
	legacy, ok := cfg.FanCurveProfilesByDevice[legacyKey]
	if !ok || !hasBlackSharkGearProfile(legacy.Profiles) {
		return types.DeviceFanCurveProfilesState{}, false
	}
	return legacy, true
}

// blackSharkDefaultManualGearRPM 把黑鲨标定表派生的 12 个转速预设转成配置那套 gear→level→rpm 形状。
// 数值来源只有一处：deviceproto.BlackSharkManualGearPresets（前端面板也读它）。
func blackSharkDefaultManualGearRPM() map[string]map[string]int {
	presets := deviceproto.BlackSharkManualGearPresets()
	out := make(map[string]map[string]int, len(presets.Gears))
	for _, gear := range presets.Gears {
		if gear.Gear < 1 || int(gear.Gear) > len(types.ManualGearOrder) {
			continue
		}
		levels := make(map[string]int, len(types.ManualLevelOrder))
		for i, level := range types.ManualLevelOrder {
			if i >= len(gear.Levels) {
				break
			}
			levels[level] = gear.Levels[i]
		}
		out[types.ManualGearOrder[int(gear.Gear)-1]] = levels
	}
	return out
}

// upgradeBlackSharkManualGearRPM 是黑鲨配置的一次性迁移：manualGearRpm 还整份是**没人动过的出厂值**
// 时，换成黑鲨标定表派生的 12 个点。
//
// 为什么要迁：界面上那 12 个值 = 黑鲨派生默认值**再被 cfg.ManualGearRpm 覆盖**，而存量配置里的
// 出厂值全都落在黑鲨量程内、看起来像"用户设的"，于是永久遮蔽黑鲨那套 —— 打开软件显示的是飞智那 12 个值，
// 点一次「重置」把它覆盖成派生值之后才"正常显示"。
//
// 判据复用既有的"是否等于默认集"，但**必须两种形态都认**：
//
//	① 已经是本单位的默认集 —— 配置被规范化过的情形（含"百分比种子被规范化成 RPM 默认集"）；
//	② **逐值**等于出厂那份百分比种子 —— 新装或删掉 config 之后的初始形态。
//	   这里不能借"规范化后是否等于默认集"来判：规范化会把**越界值替换成该单位的默认值**，
//	   于是任何 RPM 量级的表按百分比规范化都会塌成百分比默认集 ⇒ 那样判会把用户设的 RPM 值
//	   当成出厂值迁掉。所以 ② 必须逐值比。
//
// 用户真改过的值不会被误判（改动后的 RPM 表既不等本单位默认集，也不逐值等于百分比种子）。
func upgradeBlackSharkManualGearRPM(cfg *types.AppConfig, unit string) bool {
	if cfg == nil || !types.IsRPMSpeedUnit(unit) {
		return false
	}
	if !manualGearRPMMapLooksDefaultForUnit(cfg.ManualGearRPM, unit) &&
		!manualGearRPMMapIsFactoryPercentSeed(cfg.ManualGearRPM) {
		return false
	}
	next := blackSharkDefaultManualGearRPM()
	if len(next) == 0 {
		return false
	}
	cfg.ManualGearRPM = next
	return true
}

// manualGearRPMMapIsFactoryPercentSeed 判断这 12 个值是否**逐值**等于出厂百分比种子
// （`types.CloneDefaultManualGearRPM`）—— 新装/删过 config 时配置里就是这个形态。
func manualGearRPMMapIsFactoryPercentSeed(input map[string]map[string]int) bool {
	seed := types.CloneDefaultManualGearRPM()
	if len(input) != len(seed) {
		return false
	}
	for gear, levels := range seed {
		got, ok := input[gear]
		if !ok || len(got) != len(levels) {
			return false
		}
		for level, want := range levels {
			if got[level] != want {
				return false
			}
		}
	}
	return true
}

// blackSharkFirmwareCheckInterval 是连接后自动做固件更新检查的最小间隔。
// 这个检查要取厂商清单（走网络，清单请求 6s 超时），不能每次连接都跑；距上次成功检查
// 不足该间隔就跳过，面板照旧显示落盘的那份结果。
const blackSharkFirmwareCheckInterval = 6 * time.Hour

// refreshBlackSharkConnectedSnapshot 连接后取一次"设备侧现状"并落盘：
//
//	· 灯效（0x13 + 各模式参数）读一遍写进 `BlackSharkRgbCache` —— 纯设备本地读，无网络；
//	· 固件更新检查按 `blackSharkFirmwareCheckInterval` 节流跑一次 —— 要走网络。
//
// 两者都落盘，于是下次开软件面板能直接读缓存，不必等用户手点「读取灯效 / 检查更新」。
// 只在黑鲨连接时做；失败只记日志，不影响连接本身。
func (a *CoreApp) refreshBlackSharkConnectedSnapshot() {
	if a == nil || a.deviceManager == nil || !a.deviceManager.IsBlackSharkProfileActive() {
		return
	}
	// 0x07 推送循环必须在**连上的这一刻**起：温度监控通常在核心启动时就跑了（那时还没有设备），
	// 若只在 startTemperatureMonitoring 里设间隔，那次会因为"当下不是黑鲨"被跳过，之后再没人补 ——
	// 设备屏幕就停在上一帧（各值不再变化）。同值短路，重复调用安全。
	if err := a.deviceManager.SetBlackSharkHostInfoRefreshInterval(
		blackSharkSysInfoInterval, a.blackSharkHostInfoPushEntries); err != nil {
		a.logError("连接后启动 0x07 主机信息推送失败: %v", err)
	} else {
		a.logInfo("已启动 0x07 主机信息推送（间隔 %s）", blackSharkSysInfoInterval)
	}
	if lighting := a.blackSharkRgbLighting(); len(lighting.Modes) > 0 {
		a.logInfo("连接后已读取灯效：%d 个模式（已写入本机缓存）", len(lighting.Modes))
	} else {
		a.logInfo("连接后读取灯效失败，面板将回落到上次读到的那份缓存")
	}

	if !a.blackSharkFirmwareCheckDue() {
		return
	}
	status := a.CheckBlackSharkFirmwareUpdate()
	if strings.TrimSpace(status.Error) != "" {
		a.logInfo("连接后固件更新检查未完成: %s", status.Error)
		return
	}
	a.logInfo("连接后固件更新检查完成：当前 %s / 最新 %s（可升级=%v）",
		status.CurrentVersion, status.LatestVersion, status.UpdateAvailable)
}

// blackSharkFirmwareCheckDue 判断距离上次成功的检查是否已经超过节流窗口。
func (a *CoreApp) blackSharkFirmwareCheckDue() bool {
	status := a.GetBlackSharkFirmwareStatus()
	checkedAt := strings.TrimSpace(status.CheckedAt)
	if checkedAt == "" {
		return true
	}
	at, err := time.Parse(time.RFC3339, checkedAt)
	if err != nil {
		return true
	}
	return time.Since(at) >= blackSharkFirmwareCheckInterval
}

// isBlackSharkCurveScopeKey 判断设备曲线作用域键（<transport>::<profileID>）是不是黑鲨的。
func isBlackSharkCurveScopeKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	profileID := key
	if idx := strings.LastIndex(key, deviceCurveScopeSeparator); idx >= 0 {
		profileID = key[idx+len(deviceCurveScopeSeparator):]
	}
	return types.IsBlackSharkDeviceProfileID(profileID)
}

// blackSharkDefaultFanCurveState 为黑鲨构造「四档散热模式 = 四个默认曲线方案」的初始状态。
func blackSharkDefaultFanCurveState(curveScopeKey string) (types.DeviceFanCurveProfilesState, bool) {
	if !isBlackSharkCurveScopeKey(curveScopeKey) {
		return types.DeviceFanCurveProfilesState{}, false
	}

	profiles := make([]types.FanCurveProfile, 0, deviceproto.BlackSharkGearCount)
	for gear := 1; gear <= deviceproto.BlackSharkGearCount; gear++ {
		cfg, ok := deviceproto.BlackSharkFactoryGearConfig(deviceproto.BlackSharkFormCurve, byte(gear))
		if !ok {
			// 常量表里缺这档 ⇒ 不猜、不用别的档顶替，整体放弃播种（调用方会退回通用默认）。
			return types.DeviceFanCurveProfilesState{}, false
		}
		curve := make([]types.FanCurvePoint, 0, len(cfg.Curve))
		for _, point := range cfg.Curve {
			curve = append(curve, types.FanCurvePoint{
				Temperature: int(point.TempC),
				RPM:         int(point.RPM),
			})
		}
		profiles = append(profiles, types.FanCurveProfile{
			ID:    types.BlackSharkGearCurveProfileID(gear),
			Name:  blackSharkGearProfileNames[gear-1],
			Curve: curve,
		})
	}

	return types.DeviceFanCurveProfilesState{
		Profiles: profiles,
		// 初始选中「低噪」= 档位 1。设备此刻真实停在哪个档位由运行时回读（0x25）对齐，
		// 配置加载期不允许做设备 IO，这里只能给一个确定的起点（见 blackSharkGearForProfileID）。
		ActiveID:      profiles[0].ID,
		FanCurve:      curveprofiles.CloneCurve(profiles[0].Curve),
		ManualGearRPM: types.CloneDefaultManualGearRPMForUnit(types.FanSpeedUnitRPM),
	}, true
}

// dispatchGearActivationForCurveProfile 若这个方案是四档方案，就异步激活对应的设备档位。
// 非四档方案直接返回 ⇒ 用户自建的方案一个字节都不发，也就不会影响别的品牌。
func (a *CoreApp) dispatchGearActivationForCurveProfile(profileID string) {
	if a == nil {
		return
	}
	gear, ok := types.BlackSharkGearForCurveProfileID(profileID)
	if !ok {
		return
	}
	a.safeGo("activateDeviceGearForCurveProfile", func() {
		a.activateDeviceGearForCurveProfile(gear)
	})
}

// dispatchGearCurveWriteForCurveProfile 处理「保存四档方案」的落点。
//
// 曲线形状活在设备里，因此保存方案后要把这条新曲线写进设备。
// 只在「这条方案正是设备当前生效档位」时才写：用户可能在编辑别的档位的方案，
// 那种情况下设备侧不需要任何动作（保存它不影响当前运行的那一档）。
// 手动模式下设备跑的是 form=0 固定值，保存曲线方案不写设备（写曲线会把设备拉进曲线模式）。
func (a *CoreApp) dispatchGearCurveWriteForCurveProfile(profileID string, _ []types.FanCurvePoint) {
	if a == nil {
		return
	}
	gear, ok := types.BlackSharkGearForCurveProfileID(profileID)
	if !ok {
		return // 不是四档方案：不碰设备
	}
	a.safeGo("writeGearCurveForGear", func() {
		if a.deviceManager == nil || !types.IsBlackSharkDeviceProfileID(a.deviceManager.ActiveProfile().ID) {
			return // 不是黑鲨：一个字节都不发
		}
		if !a.autoControlActive() {
			a.logInfo("曲线方案 %s 已保存；当前是固定转速模式，不写设备（设备的曲线保持不变）", profileID)
			return
		}
		if !a.deviceManager.BlackSharkGearIsActive(gear) {
			a.logInfo("曲线方案 %s（档位 %d）已保存；设备当前不在这一档，不下发", profileID, gear)
			return
		}
		if a.applyBlackSharkGear(gear) {
			a.logInfo("曲线方案 %s 已保存，并把档位 %d 的新曲线写进了设备", profileID, gear)
		} else {
			a.logError("曲线方案 %s 已保存，但把档位 %d 的曲线写进设备失败", profileID, gear)
		}
	})
}

// activateDeviceGearForCurveProfile 把「切换曲线方案」翻译成「切换设备档位」：
// 四档散热模式就是四个曲线方案，切方案即切档位（切档 = 写该档的 `0x24`）。
func (a *CoreApp) activateDeviceGearForCurveProfile(gear int) {
	if a == nil || a.deviceManager == nil {
		return
	}
	if !types.IsBlackSharkDeviceProfileID(a.deviceManager.ActiveProfile().ID) {
		return // 不是黑鲨：一个字节都不发
	}
	if !a.applyBlackSharkGear(gear) {
		// 如实报告：本机「选中哪一档」已经切了，但设备侧可能还停在原档位。
		// 用 logError：这是部分失败（本机配置生效了、设备侧没有），不该按 INFO 吞掉。
		a.logError("切换曲线方案后施加设备档位 %d 失败 —— 本机配置已切换，设备侧可能仍在原档位", gear)
		return
	}
	a.logInfo("曲线方案已切换为设备档位 %d", gear)
}

// applyManualGearForCurveProfile 手动模式下把「切曲线方案」落成「该档的手动固定转速」。
// 与手动挡位面板走同一条路（`SetManualGearRPM` → 黑鲨内部转 `SetBlackSharkFixedSpeedForGear`）。
func (a *CoreApp) applyManualGearForCurveProfile(gear int) bool {
	if gear < 1 || gear > len(types.ManualGearOrder) {
		a.logError("手动模式下切方案失败：档位 %d 越界", gear)
		return false
	}
	if a.configManager == nil || a.deviceManager == nil {
		a.logError("手动模式下切方案失败：运行环境未就绪")
		return false
	}
	name := types.ManualGearOrder[gear-1]
	cfg := a.configManager.Get()
	unit := a.activeDeviceSpeedUnit(&cfg)
	types.NormalizeManualGearRPMForUnit(&cfg, unit)
	level := a.getRememberedManualLevel(name, cfg.ManualLevel)
	rpm := cfg.ResolveGearRPM(name, level)
	if rpm <= 0 {
		a.logError("手动模式下切方案失败：挡位 %s/%s 没有可用转速", name, level)
		return false
	}
	if !a.deviceManager.SetManualGearRPM(name, level, rpm) {
		a.logError("手动模式下切方案失败：挡位 %s 下发固定转速 %d 失败", name, rpm)
		return false
	}
	a.logInfo("手动模式：切方案已按挡位 %s 下发固定转速 %d%s", name, rpm, types.FanSpeedDisplaySuffix(unit))
	return true
}

// ---- LCD 显示三项（0xC2）与磁盘根路径 ----

// blackSharkLcdDisplay 返回本工具保存的显示参数。
func (a *CoreApp) blackSharkLcdDisplay() ipc.BlackSharkLcdDisplayParams {
	cfg := a.configManager.Get()
	items, _ := types.NormalizeBlackSharkLcdItems(cfg.BlackSharkLcdItems)
	// 可选项与「官方默认三项」都从各自的所有者取，不在界面层重抄：
	// Options ← deviceproto.BlackSharkLcdSelectableItems（协议事实）；
	// DefaultItems ← types.DefaultBlackSharkLcdItems()（官方「重置屏幕设置」的效果）。
	return ipc.BlackSharkLcdDisplayParams{
		Pos:          cfg.BlackSharkLcdPos,
		Items:        items,
		Options:      deviceproto.BlackSharkLcdSelectableItemIDs(),
		DefaultItems: types.DefaultBlackSharkLcdItems(),
	}
}

// setBlackSharkLcdDisplay 保存显示参数（本机配置）并下发到设备（0xC2）。
func (a *CoreApp) setBlackSharkLcdDisplay(pos int, items []int) ipc.BlackSharkLcdDisplayResult {
	res := ipc.BlackSharkLcdDisplayResult{Pos: pos, Items: items}
	if _, err := a.configManager.MutateAndSave(func(cfg *types.AppConfig) {
		cfg.BlackSharkLcdItems = items
		cfg.BlackSharkLcdPos = pos
	}); err != nil {
		res.Error = "保存显示参数失败: " + err.Error()
		return res
	}
	res.Saved = true

	if a.deviceManager == nil || !a.deviceManager.IsBlackSharkActive() {
		// 设备不在不算致命：参数已经存下来了，下次连接后再点一次即可。
		res.Error = "设备未就绪，显示参数已保存但未下发"
		return res
	}

	// 0xC2 的条目结构是 id + u16，要连当前值一起发，所以这里现采一次；
	// 与 0x07 共用同一个采集函数，不另写一套。
	a.mutex.RLock()
	temp := a.currentTemp
	a.mutex.RUnlock()
	entries := blackSharkSysInfoPick(a.collectBlackSharkSysInfo(temp, time.Now()), items)

	if !a.deviceManager.SetBlackSharkLcdShowPos(pos, entries) {
		res.Error = "下发失败（参数已保存，可稍后重试）"
		return res
	}
	res.Applied = true
	a.logInfo("黑鲨 LCD 显示参数已下发：pos=%d 项=%v（无回读命令，不能确认生效）", pos, items)
	return res
}

// diskRootPath 返回系统盘根路径：官方取 SystemDrive 盘的用量
// （PublicFunction.dll 的 GetStorageUsage / GetStorageTotalBytes），这里同样以 SystemDrive 为准。
func diskRootPath() string {
	if drive := os.Getenv("SystemDrive"); drive != "" {
		return drive + `\\`
	}
	return `C:////`
}

// ---- 系统信息（0x07）采集与推送 ----

// 系统信息采集：0x07 推送与 0xC2 显示参数共用同一份来源。
const blackSharkSysInfoInterval = time.Second

// collectBlackSharkSysInfo 采集主机指标，返回全部 8 个指标项（id 0..7，顺序即条目 id 升序）。
func (a *CoreApp) collectBlackSharkSysInfo(
	temp types.TemperatureData, now time.Time,
) []deviceproto.BlackSharkSystemInfoEntry {
	cpuLoad := -1
	if pcts, err := cpu.Percent(0, false); err == nil && len(pcts) > 0 {
		cpuLoad = int(pcts[0] + 0.5)
	}
	memLoad := -1
	if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
		memLoad = int(vm.UsedPercent + 0.5)
	}
	diskLoad := -1
	if du, err := disk.Usage(diskRootPath()); err == nil && du != nil {
		diskLoad = int(du.UsedPercent + 0.5)
	}
	// 风扇转速由设备自己知道，这里只是把它回填进 0xC2 的条目
	// （官方不往 0x07 推转速，见 deviceproto.BlackSharkSysInfoPushItems）；读不到就是 0。
	fanRPM := -1
	if a.deviceManager != nil {
		if fd := a.deviceManager.GetCurrentFanData(); fd != nil {
			fanRPM = int(fd.CurrentRPM)
		}
	}

	return []deviceproto.BlackSharkSystemInfoEntry{
		{ID: deviceproto.BlackSharkSysInfoCPUTemp,
			Value: deviceproto.ClampBlackSharkSysInfoValue(temp.CPUTemp)},
		{ID: deviceproto.BlackSharkSysInfoGPUTemp,
			Value: deviceproto.ClampBlackSharkSysInfoValue(temp.GPUTemp)},
		{ID: deviceproto.BlackSharkSysInfoCPULoad,
			Value: deviceproto.ClampBlackSharkSysInfoValue(cpuLoad)},
		// GPU 占用率：没有采集来源，发 0（与官方在无读数时一致）
		{ID: deviceproto.BlackSharkSysInfoGPULoad, Value: 0},
		{ID: deviceproto.BlackSharkSysInfoFanRPM,
			Value: deviceproto.ClampBlackSharkSysInfoValue(fanRPM)},
		{ID: deviceproto.BlackSharkSysInfoDiskUsage,
			Value: deviceproto.ClampBlackSharkSysInfoValue(diskLoad)},
		{ID: deviceproto.BlackSharkSysInfoMemUsage,
			Value: deviceproto.ClampBlackSharkSysInfoValue(memLoad)},
		// 时间：时*60+分（id7=1223 表示 20:23）
		{ID: deviceproto.BlackSharkSysInfoMinuteTime,
			Value: deviceproto.ClampBlackSharkSysInfoValue(now.Hour()*60 + now.Minute())},
	}
}

// blackSharkSysInfoPick 按给定 id 顺序取出条目（缺失的补 0），用于 0xC2 的「选 3 项」。
func blackSharkSysInfoPick(
	all []deviceproto.BlackSharkSystemInfoEntry, ids []int,
) []deviceproto.BlackSharkSystemInfoEntry {
	byID := make(map[byte]uint16, len(all))
	for _, e := range all {
		byID[e.ID] = e.Value
	}
	out := make([]deviceproto.BlackSharkSystemInfoEntry, 0, len(ids))
	for _, id := range ids {
		out = append(out, deviceproto.BlackSharkSystemInfoEntry{ID: byte(id), Value: byID[byte(id)]})
	}
	return out
}

// ── 图传 = 一次"设备独占任务" ────────────────────────────────────────────────
//
// 按上游那套形状实现：coreapp 侧登记/取消（下面这几个函数），device 层只管帧序列与进度上报
// （`device.(*Manager).SendBlackSharkImageWithProgress`）。
//
// 登记表是"当前有没有设备独占任务在飞"的**唯一所有者**：健康检查、运行态、监控循环
// 都读它，不再各自维护判据（原先只有监控循环通过 bulkDeviceOpEndUnix 知道这件事）。

// blackSharkImageTransferTimeout 单次图传的预算：帧流约 18s（每报 7ms、跨 4 KiB 页补 40ms）
// 加上 C4 握手窗口与收尾，留足余量但不允许无限挂起。
const blackSharkImageTransferTimeout = 90 * time.Second

// beginBlackSharkImageTransfer 登记一次图传，返回可被取消的 ctx 与必须 defer 的 done。
// 已有传输在飞时直接报错（一次只允许一个独占任务，避免两条帧流互相踩）。
func (a *CoreApp) beginBlackSharkImageTransfer(parent context.Context) (context.Context, func(), error) {
	if a == nil {
		return nil, func() {}, fmt.Errorf("核心服务未就绪")
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, blackSharkImageTransferTimeout)

	a.blackSharkImageTransferMu.Lock()
	if len(a.blackSharkImageTransfers) > 0 {
		inFlight := len(a.blackSharkImageTransfers)
		a.blackSharkImageTransferMu.Unlock()
		cancel()
		return nil, func() {}, fmt.Errorf("已有 %d 个屏幕图片传输在进行，等它结束再试", inFlight)
	}
	if a.blackSharkImageTransfers == nil {
		a.blackSharkImageTransfers = map[uint64]context.CancelFunc{}
	}
	id := uint64(time.Now().UnixNano())
	a.blackSharkImageTransfers[id] = cancel
	a.blackSharkImageTransferMu.Unlock()

	// 图传期间暂停 0x07 推送（官方 TransferDeviceImage 前后各调一次 setter 做的是同一件事）：
	// 让 2098 帧的图传流是线上唯一的黑鲨流量，不与周期推送交错。
	a.pauseBlackSharkHostInfoForBulkOp()

	done := func() {
		cancel()
		a.blackSharkImageTransferMu.Lock()
		delete(a.blackSharkImageTransfers, id)
		a.blackSharkImageTransferMu.Unlock()
		// 恢复推送必须做：不恢复的话 LCD 参数页会停在最后一帧不动。
		a.resumeBlackSharkHostInfoAfterBulkOp()
		// 结束时刻留痕：监控循环靠它把"这一段的停摆"归因到本机，而不是误判成系统睡眠。
		a.noteBulkDeviceOperation()
		// 独占窗口里温度监控被饿住，智能控温用的是窗口前的温度 ⇒ 让下一拍**强制重估**，
		// 免得变化没越过最小变化阈值时继续沿用窗口前的旧目标（与噪声诊断结束后的处置一致）。
		a.forceNextAutoTarget.Store(true)
		// 同一次停摆还会污染温度 EMA：窗口后第一拍必须重播种，不能把十几秒空洞当一个采样步长。
		a.reseedTempEMA.Store(true)
	}
	return ctx, done, nil
}

// pauseBlackSharkHostInfoForBulkOp / resumeBlackSharkHostInfoAfterBulkOp 是一对：
// 独占任务期间停 0x07 推送，结束后按原来的节奏恢复。失败只记日志 ——
// 推送是周期行为，少推几拍比把独占任务本身搞砸轻。
func (a *CoreApp) pauseBlackSharkHostInfoForBulkOp() {
	if a == nil || a.deviceManager == nil {
		return
	}
	if err := a.deviceManager.SetBlackSharkHostInfoRefreshInterval(0, nil); err != nil {
		a.logDebug("暂停 0x07 推送失败（不影响本次图传）: %v", err)
	}
}

func (a *CoreApp) resumeBlackSharkHostInfoAfterBulkOp() {
	if a == nil || a.deviceManager == nil || !a.deviceManager.IsBlackSharkProfileActive() {
		return
	}
	if err := a.deviceManager.SetBlackSharkHostInfoRefreshInterval(
		blackSharkSysInfoInterval, a.blackSharkHostInfoPushEntries); err != nil {
		a.logError("恢复 0x07 推送失败（LCD 参数页可能停在最后一帧）: %v", err)
	}
}

// cancelAllBlackSharkImageTransfers 取消全部在飞的图传。断开、挂起这类"设备要没了"的
// 时刻调用它：传输本来就注定失败，早点结束比让调用方干等十几秒好。
func (a *CoreApp) cancelAllBlackSharkImageTransfers() {
	if a == nil {
		return
	}
	a.blackSharkImageTransferMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(a.blackSharkImageTransfers))
	for id, cancel := range a.blackSharkImageTransfers {
		cancels = append(cancels, cancel)
		delete(a.blackSharkImageTransfers, id)
	}
	a.blackSharkImageTransferMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	if len(cancels) > 0 {
		a.logInfo("已取消 %d 个进行中的屏幕图片传输", len(cancels))
	}
}

// blackSharkImageTransferActive 报告当前是否有图传在飞（健康检查 / 运行态快照用）。
func (a *CoreApp) blackSharkImageTransferActive() bool {
	if a == nil {
		return false
	}
	a.blackSharkImageTransferMu.Lock()
	defer a.blackSharkImageTransferMu.Unlock()
	return len(a.blackSharkImageTransfers) > 0
}

// TransferDeviceImage sends a fixed-size BRB02 RGB565 canvas to a wired Black Shark device.
func (a *CoreApp) TransferDeviceImage(params ipc.TransferDeviceImageParams) error {
	if a.deviceManager == nil || !a.deviceManager.IsConnected() {
		return fmt.Errorf("黑鲨设备未连接")
	}
	profile := a.deviceManager.ActiveProfile()
	if profile.ID != types.BlackSharkBRB02USBProfileID ||
		types.NormalizeDeviceTransport(profile.Transport) != types.DeviceTransportUSB {
		return fmt.Errorf("黑鲨屏幕图片传输仅支持 USB 有线模式")
	}
	if !profile.Capabilities.SupportsScreenImageTransfer {
		return fmt.Errorf("当前设备不支持屏幕图片传输")
	}
	if format := strings.TrimSpace(strings.ToLower(params.Format)); format != "" && format != "rgb565-be" {
		return fmt.Errorf("不支持的图传格式 %q", params.Format)
	}
	if (params.Width != 0 && params.Width != deviceproto.BlackSharkImageWidth) ||
		(params.Height != 0 && params.Height != deviceproto.BlackSharkImageHeight) {
		return fmt.Errorf("黑鲨图传尺寸必须为 %dx%d", deviceproto.BlackSharkImageWidth, deviceproto.BlackSharkImageHeight)
	}
	encoded := strings.TrimSpace(params.DataBase64)
	if encoded == "" {
		return fmt.Errorf("屏幕图片数据为空")
	}
	canvas, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("屏幕图片数据无效: %w", err)
	}
	if len(canvas) != deviceproto.BlackSharkImageBytes {
		return fmt.Errorf("屏幕图片数据必须为 %d 字节 RGB565", deviceproto.BlackSharkImageBytes)
	}
	ctx, done, beginErr := a.beginBlackSharkImageTransfer(a.ctx)
	if beginErr != nil {
		return beginErr
	}
	defer done()
	if err := a.deviceManager.SendBlackSharkImageWithProgress(ctx, canvas, nil); err != nil {
		return fmt.Errorf("黑鲨屏幕图片传输失败: %w", err)
	}
	return nil
}

// blackSharkHostInfoPushEntries 取"这一拍要推的那 7 项"。
//
// 0x07 的节拍由 device 侧的推送循环拥有（`SetBlackSharkHostInfoRefreshInterval`），
// 循环每拍调本函数 —— 所以温度取最近一次采样的结果，而不是把推送挂在采样循环里。
// 推送的是固定 7 项（按 deviceproto.BlackSharkSysInfoPushItems 过滤），
// 不是用户选的那 3 项：官方 0x07 恒 7 条。
func (a *CoreApp) blackSharkHostInfoPushEntries() []deviceproto.BlackSharkSystemInfoEntry {
	if a == nil || a.deviceManager == nil {
		return nil
	}
	a.mutex.RLock()
	temp := a.currentTemp
	a.mutex.RUnlock()
	return blackSharkSysInfoPick(
		a.collectBlackSharkSysInfo(temp, time.Now()), bytesToInts(deviceproto.BlackSharkSysInfoPushItems))
}

// bytesToInts 把 deviceproto 的 byte 列表转成 []int（供 blackSharkSysInfoPick 复用）。
func bytesToInts(in []byte) []int {
	out := make([]int, len(in))
	for i, b := range in {
		out[i] = int(b)
	}
	return out
}

// ---- 主机侧灯效驱动（响应 / 音频同步）与情景规则 ----

const (
	// blackSharkHostFxPollInterval 是推送节拍：204 ms ⇒ 4.90 Hz，与官方 0x15 的
	// 约 4.9 Hz 一致。
	blackSharkHostFxPollInterval = 204 * time.Millisecond

	// blackSharkHostFxRetryInterval 是启动失败后多久再试一次：一次瞬时失败
	// （比如默认音频设备正在切换）就会让功能永久不可用，重试间隔给恢复留窗口。
	blackSharkHostFxRetryInterval = 20 * time.Second

	// blackSharkPressQueueSize 是键鼠事件队列长度：满了就丢，钩子线程绝不能阻塞；
	// 丢的是极短时间内的重复按下，对灯效观感无影响。
	blackSharkPressQueueSize = 64
)

// blackSharkHostFx 是主机侧驱动的运行时状态。零值不可用，用 newBlackSharkHostFx。
//
// 两个驱动的**起停只在轮询协程上发生**（tick 与 shutdown 同源于那一个循环），所以
// ensure*/stop* 之间"先查后建"的写法不需要额外串行化；mu 只用来保护跨协程的读写
// （推送计数、采样值，以及 consumer 协程里的计数累加）。
type blackSharkHostFx struct {
	mu     sync.Mutex
	stop   chan struct{}
	done   chan struct{}
	closed bool

	// 音频同步（槽位 7）
	audio        *bsaudio.Sampler
	audioErr     string
	audioRetryAt time.Time
	audioFreq    float64
	audioLevel   byte
	audioPushes  uint64

	// 响应（槽位 6）
	press        *bspress.Hook
	pressErr     string
	pressRetryAt time.Time
	pressQueue   chan struct{}
	pressStop    chan struct{}
	pressPushes  uint64

	// 上次看到的槽位（用于日志：切换一次只打一行）
	lastSlot int
}

func newBlackSharkHostFx() *blackSharkHostFx {
	return &blackSharkHostFx{stop: make(chan struct{}), done: make(chan struct{})}
}

// startBlackSharkHostFx 起一个约 4.9 Hz 的循环，按设备当前生效的灯效槽位起停两个主机侧驱动。
func (a *CoreApp) startBlackSharkHostFx() {
	if a.btHostFx != nil {
		return
	}
	a.btHostFx = newBlackSharkHostFx()
	fx := a.btHostFx

	a.safeGo("blackSharkHostFxLoop", func() {
		defer close(fx.done)
		ticker := time.NewTicker(blackSharkHostFxPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-fx.stop:
				fx.shutdown(a)
				return
			case <-ticker.C:
				fx.tick(a)
				// 状态由核心推给界面，界面不再轮询：shouldDropEvent 按
				// highFrequencyEventTypes 的阈值节流，这个 204ms 循环实际只推 ≤1 条/秒。
				a.broadcastBlackSharkHostEffects()
			}
		}
	})
	a.logInfo("黑鲨主机侧灯效驱动已启动（轮询 %s；槽位 6=响应 / 7=音频同步）", blackSharkHostFxPollInterval)
}

// stopBlackSharkHostFx 停掉循环并释放两个驱动（幂等；进程退出时调用）。
func (a *CoreApp) stopBlackSharkHostFx() {
	if a.btHostFx == nil {
		return
	}
	a.btHostFx.mu.Lock()
	if !a.btHostFx.closed {
		a.btHostFx.closed = true
		close(a.btHostFx.stop)
	}
	a.btHostFx.mu.Unlock()
	<-a.btHostFx.done
}

// tick 是每个节拍做一次的事：判当前槽位 → 该起的起、该停的停、该推的推。
func (fx *blackSharkHostFx) tick(a *CoreApp) {
	slot := 0
	if a.deviceManager != nil && a.deviceManager.IsBlackSharkActive() {
		slot = a.deviceManager.BlackSharkCurrentRgbSlot()
	}
	if slot != fx.lastSlot {
		if slot == 6 || slot == 7 {
			a.logInfo("黑鲨灯效切到槽位 %d（%s）⇒ 启用对应的主机侧驱动",
				slot, deviceproto.BlackSharkRgbEffectNames[slot])
		} else if fx.lastSlot == 6 || fx.lastSlot == 7 {
			a.logInfo("黑鲨灯效已离开槽位 %d ⇒ 停掉主机侧驱动", fx.lastSlot)
		}
		fx.lastSlot = slot
	}

	switch slot {
	case 7: // 音频同步：要主机推 0x15
		fx.stopPress(a)
		fx.ensureAudio(a)
		fx.pushAudio(a)
	case 6: // 响应：要主机推 0x16（推送发生在消费协程里）
		fx.stopAudio(a)
		fx.ensurePress(a)
	default:
		fx.stopPress(a)
		fx.stopAudio(a)
	}
}

// pushAudio 读一次采样器的主频并（非静音时）下发一帧。
func (fx *blackSharkHostFx) pushAudio(a *CoreApp) {
	fx.mu.Lock()
	s := fx.audio
	fx.mu.Unlock()
	if s == nil || a.deviceManager == nil {
		return
	}
	freq := s.Frequency()
	level := deviceproto.BlackSharkAudioLevelForFrequency(freq)

	fx.mu.Lock()
	fx.audioFreq, fx.audioLevel = freq, level
	fx.mu.Unlock()

	if level == 0 {
		return // 静音（或还没采到）⇒ 与官方一致：不发帧
	}
	if a.deviceManager.IssueAudioSpectrumLevel(level) {
		fx.mu.Lock()
		fx.audioPushes++
		fx.mu.Unlock()
	}
}

// ensureAudio 确保音频采集在跑（成功过就不重复起；失败按 retry 间隔重试）。
func (fx *blackSharkHostFx) ensureAudio(a *CoreApp) {
	fx.mu.Lock()
	if fx.audio != nil {
		fx.mu.Unlock()
		return
	}
	if fx.audioErr != "" && time.Now().Before(fx.audioRetryAt) {
		fx.mu.Unlock()
		return
	}
	fx.mu.Unlock()

	s, err := bsaudio.Start()
	if err != nil {
		fx.mu.Lock()
		fx.audioErr = err.Error()
		fx.audioRetryAt = time.Now().Add(blackSharkHostFxRetryInterval)
		fx.mu.Unlock()
		// 如实报错：用户选了音频同步却看不到反应时，这是他唯一能拿到的线索。
		a.logError("黑鲨「音频同步」：启动系统音频采集失败（%v）—— 该灯效不会有反应；%s 后重试",
			err, blackSharkHostFxRetryInterval)
		return
	}
	fx.mu.Lock()
	fx.audio, fx.audioErr = s, ""
	fx.mu.Unlock()
	a.logInfo("黑鲨「音频同步」：已开始采集系统播放声音（WASAPI loopback，仅在本灯效生效期间）")
}

// stopAudio 停掉音频采集（没在跑就是空操作）。
func (fx *blackSharkHostFx) stopAudio(a *CoreApp) {
	fx.mu.Lock()
	s := fx.audio
	fx.audio = nil
	fx.audioFreq, fx.audioLevel = 0, 0
	fx.mu.Unlock()
	if s == nil {
		return
	}
	s.Stop()
	a.logInfo("黑鲨「音频同步」：已停止音频采集")
}

// ensurePress 确保键鼠钩子在跑。
func (fx *blackSharkHostFx) ensurePress(a *CoreApp) {
	fx.mu.Lock()
	if fx.press != nil {
		fx.mu.Unlock()
		return
	}
	if fx.pressErr != "" && time.Now().Before(fx.pressRetryAt) {
		fx.mu.Unlock()
		return
	}
	fx.mu.Unlock()

	queue := make(chan struct{}, blackSharkPressQueueSize)
	sink := func() {
		// 钩子回调运行在全系统键鼠的必经路径上：这里只允许「丢了就走」。
		select {
		case queue <- struct{}{}:
		default:
		}
	}
	h, err := bspress.Start(sink)
	if err != nil {
		fx.mu.Lock()
		fx.pressErr = err.Error()
		fx.pressRetryAt = time.Now().Add(blackSharkHostFxRetryInterval)
		fx.mu.Unlock()
		a.logError("黑鲨「响应」：装全局键鼠钩子失败（%v）—— 该灯效不会随按键变化；%s 后重试",
			err, blackSharkHostFxRetryInterval)
		return
	}
	pressStop := make(chan struct{})
	fx.mu.Lock()
	fx.press, fx.pressQueue, fx.pressStop, fx.pressErr = h, queue, pressStop, ""
	fx.mu.Unlock()

	// 消费协程：钩子回调只入队，真正的设备写发生在协程里
	// （设备写要拿锁、要走 HID，绝不能放在钩子线程上）。
	a.safeGo("blackSharkPressConsumer", func() {
		for {
			select {
			case <-pressStop:
				return
			case <-queue:
				if a.deviceManager == nil {
					continue
				}
				// 每按下一次发一帧（官方语义），不做合并/限流。
				if a.deviceManager.NotifyMouseKeyPress() {
					fx.mu.Lock()
					fx.pressPushes++
					fx.mu.Unlock()
				}
			}
		}
	})
	a.logInfo("黑鲨「响应」：已装上全局键鼠低层钩子（仅在本灯效生效期间）")
}

// stopPress 卸钩并停掉消费协程。
func (fx *blackSharkHostFx) stopPress(a *CoreApp) {
	fx.mu.Lock()
	h := fx.press
	pressStop := fx.pressStop
	fx.press, fx.pressQueue, fx.pressStop = nil, nil, nil
	fx.mu.Unlock()
	if h == nil {
		return
	}
	if pressStop != nil {
		close(pressStop)
	}
	h.Stop()
	a.logInfo("黑鲨「响应」：已卸下全局键鼠钩子")
}

// shutdown 是循环退出时的收尾（两个驱动都停掉）。
func (fx *blackSharkHostFx) shutdown(a *CoreApp) {
	fx.stopPress(a)
	fx.stopAudio(a)
}

// BlackSharkHostEffects 返回主机侧灯效驱动的状态（纯内存读取，不发设备查询）。
// 前端用它显示现在到底在不在推：只说「已实现」而给不出状态，就等于还是假绿。
func (a *CoreApp) BlackSharkHostEffects() types.BlackSharkHostEffects {
	out := types.BlackSharkHostEffects{}
	if a.deviceManager != nil && a.deviceManager.IsBlackSharkActive() {
		out.CurrentSlot = a.deviceManager.BlackSharkCurrentRgbSlot()
	}
	fx := a.btHostFx
	if fx == nil {
		return out
	}
	fx.mu.Lock()
	defer fx.mu.Unlock()
	out.AudioSyncRunning = fx.audio != nil
	out.AudioError = fx.audioErr
	out.AudioFrequencyHz = fx.audioFreq
	out.AudioLevel = int(fx.audioLevel)
	out.AudioPushCount = fx.audioPushes
	out.ReactiveRunning = fx.press != nil
	out.ReactiveError = fx.pressErr
	out.ReactivePushCount = fx.pressPushes
	return out
}

// broadcastBlackSharkHostEffects 把主机侧灯效状态推给界面（见上面循环处的注释）。
func (a *CoreApp) broadcastBlackSharkHostEffects() {
	if a == nil || a.ipcServer == nil {
		return
	}
	a.ipcServer.BroadcastEvent(ipc.EventBlackSharkHostEffects, a.BlackSharkHostEffects())
}

// ---- 情景规则与轮询 ----

// 情景：按前台进程自动施加档位/灯效。
func (a *CoreApp) handleSceneIPCRequest(req ipc.Request) (ipc.Response, bool) {
	switch req.Type {
	case ipc.ReqGetSceneRules:
		rules, baseline := a.GetSceneRules()
		return a.dataResponse(ipc.SceneRulesParams{Rules: rules, BaselineGear: baseline}), true

	case ipc.ReqSetSceneRules:
		var params ipc.SceneRulesParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		issues, err := a.SetSceneRules(params.Rules, params.BaselineGear)
		if err != nil {
			// 把逐字段的校验说明带回前端：只说「保存失败」用户无从下手。
			msg := err.Error()
			if len(issues) > 0 {
				msg += "；" + issues[0]
			}
			return a.errorResponse(msg), true
		}
		return a.dataResponse(map[string]any{"saved": true, "issues": issues}), true

	case ipc.ReqListSceneProcesses:
		return a.dataResponse(map[string]any{"processes": a.ListSceneProcesses()}), true
	}
	return ipc.Response{}, false
}

// 情景判定循环。
const scenePollInterval = time.Second

// startSceneLoop 启动情景循环。
//
// 与健康监控同样的模式：单独 ticker + safeGo + cleanupChan 退出。
func (a *CoreApp) startSceneLoop() {
	a.sceneTicker = time.NewTicker(scenePollInterval)

	a.safeGo("sceneLoop", func() {
		defer a.sceneTicker.Stop()
		for {
			select {
			case <-a.sceneTicker.C:
				if a.stopping.Load() {
					return
				}
				a.tickScene()
			case <-a.cleanupChan:
				a.logInfo("情景循环已停止")
				return
			}
		}
	})

	a.logInfo("情景循环已启动（采样周期 %s）", scenePollInterval)
}

// tickScene 采样一次前台进程并（必要时）施加动作。
func (a *CoreApp) tickScene() {
	cfg := a.configManager.Get()

	// 没有规则：连前台进程都不必读，零开销。
	if len(cfg.SceneRules) == 0 {
		return
	}
	// 情景只对黑鲨有意义：它的「效果」就是黑鲨的档位与灯效模式。
	if a.deviceManager == nil || !a.deviceManager.IsBlackSharkActive() {
		return
	}

	// 配置被改过 -> 引擎里记的规则索引已经不可比，强制重新判定一次。
	if rev := a.configManager.Revision(); rev != a.sceneRevision {
		a.sceneRevision = rev
		a.sceneEngine.Reset()
		a.logDebug("情景：配置已变更（revision=%d），重新判定", rev)
	}

	fg, err := scene.GetForegroundProcessName()
	if err != nil {
		// 读不到前台进程时跳过这一轮，绝不当作「没有匹配」：
		// 否则一次权限抖动就会把情景误撤销，那比晚一秒切换糟得多。
		a.logDebug("情景：本次读不到前台进程（跳过，不撤销）: %v", err)
		return
	}

	act, changed := a.sceneEngine.Update(cfg.SceneRules, fg)
	if !changed {
		return
	}
	a.applyScene(act, cfg)
}

// applyScene 把引擎给出的动作落到设备上。
func (a *CoreApp) applyScene(act scene.Action, cfg types.AppConfig) {
	if a.deviceManager == nil {
		return
	}
	if act.RuleIndex == scene.ActiveBaseline {
		// 回落基准：只有在配置里明确写了基准档位时才动设备；
		// 没写就什么都不做，不去猜测用户原来想要哪一档。
		if cfg.SceneBaselineGear <= 0 {
			a.logInfo("情景：已无匹配规则（前台=%s）；未配置基准档位，保持现状", act.Foreground)
			return
		}
		if !a.applyBlackSharkGear(cfg.SceneBaselineGear) {
			a.logError("情景：回落到基准档位 %d 失败", cfg.SceneBaselineGear)
			return
		}
		a.logInfo("情景：已回落到基准档位 %d（前台=%s）", cfg.SceneBaselineGear, act.Foreground)
		return
	}

	name := act.Foreground
	if act.Gear > 0 {
		if !a.applyBlackSharkGear(act.Gear) {
			// 一条失败不要中断另一条：档位失败时灯效仍然值得施加。
			a.logError("情景「%s」：施加档位 %d 失败", name, act.Gear)
		} else {
			a.logInfo("情景「%s」：已施加档位 %d", name, act.Gear)
		}
	}
	if act.RgbMode > 0 {
		if !a.deviceManager.SelectBlackSharkRgbMode(act.RgbMode) {
			a.logError("情景「%s」：切换灯效模式 %d 失败", name, act.RgbMode)
		} else {
			a.logInfo("情景「%s」：已切换灯效模式 %d", name, act.RgbMode)
		}
	}
}

// GetSceneRules 返回情景规则与基准档位（供界面读取）。
func (a *CoreApp) GetSceneRules() ([]types.SceneRule, int) {
	cfg := a.configManager.Get()
	if cfg.SceneRules == nil {
		return []types.SceneRule{}, cfg.SceneBaselineGear
	}
	return cfg.SceneRules, cfg.SceneBaselineGear
}

// SetSceneRules 保存情景规则与基准档位，返回 (issues, err)。
func (a *CoreApp) SetSceneRules(rules []types.SceneRule, baseline int) ([]string, error) {
	var issues []string
	for i := range rules {
		// 名字只做归一化（去空白/截断），不算「不合法」—— 它不影响匹配语义。
		rules[i].Name = types.TrimSceneRuleName(rules[i].Name)
		for _, msg := range rules[i].SceneRuleIssues() {
			issues = append(issues, fmt.Sprintf("第 %d 条：%s", i+1, msg))
		}
	}
	if baseline < 0 || baseline > types.SceneGearMax {
		issues = append(issues, fmt.Sprintf("基准档位 %d 超出 0..%d", baseline, types.SceneGearMax))
		return issues, fmt.Errorf("参数不合法，未保存")
	}

	if _, err := a.configManager.MutateAndSave(func(cfg *types.AppConfig) {
		cfg.SceneRules = rules
		cfg.SceneBaselineGear = baseline
	}); err != nil {
		return issues, err
	}
	// 规则换了：判定循环会在下一拍发现配置版本号变化并自行重置引擎（见 tickScene）。
	// 这里不要再直接动 sceneEngine —— 它不是并发安全的，而本函数跑在 IPC 线程上。
	a.logInfo("情景规则已保存：%d 条，基准档位 %d，校验提示 %d 条",
		len(rules), baseline, len(issues))
	return issues, nil
}

// ListSceneProcesses 返回可选作触发者的进程名（去重升序）。
func (a *CoreApp) ListSceneProcesses() []string {
	return scene.ListProcessNames()
}

// ── 以下自 blackshark_screen.go 并入（同 build tag，纯搬移）──

// 屏幕：预设 / 裁剪 / 上传 / 读图。

// 屏幕：预设列表 / 缩略图 / 图像上传。
func (a *CoreApp) handleScreenIPCRequest(req ipc.Request) (ipc.Response, bool) {
	switch req.Type {
	case ipc.ReqListScreenPresets:
		list, err := a.ListScreenPresets()
		if err != nil {
			return a.errorResponse("读取内置预设失败: " + err.Error()), true
		}
		return a.dataResponse(map[string]any{"presets": list}), true

	case ipc.ReqScreenPresetThumb:
		var params ipc.ScreenPresetParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		url, err := a.ScreenPresetThumbnail(params.OfficialPosition)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(map[string]any{"dataUrl": url}), true

	case ipc.ReqCancelScreenImageTransfer:
		// 图传是一次设备独占任务：取消 = 让它的 ctx 结束，调用方立刻拿到失败而不是干等十几秒。
		a.cancelAllBlackSharkImageTransfers()
		return a.dataResponse(true), true

	case ipc.ReqUploadScreenPreset:
		var params ipc.ScreenPresetParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		res, err := a.UploadScreenPreset(params.OfficialPosition)
		if err != nil {
			return a.errorResponse("上传屏保图像失败: " + err.Error()), true
		}
		return a.dataResponse(res), true

	case ipc.ReqUploadScreenImage:
		var params ipc.ScreenImagePathParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		res, err := a.UploadScreenImageFile(params.Path, screenCropFromParams(params))
		if err != nil {
			return a.errorResponse("上传本地图片失败: " + err.Error()), true
		}
		return a.dataResponse(res), true

	case ipc.ReqPreviewScreenCrop:
		var params ipc.ScreenImagePathParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		preview, err := a.PreviewScreenCrop(params.Path, screenCropFromParams(params))
		if err != nil {
			return a.errorResponse("生成裁剪预览失败: " + err.Error()), true
		}
		return a.dataResponse(preview), true

	case ipc.ReqListScreenHistory:
		var params ipc.ScreenHistoryQueryParams
		if req.Data != nil {
			if err := json.Unmarshal(req.Data, &params); err != nil {
				return a.errorResponse("解析参数失败: " + err.Error()), true
			}
		}
		list, err := a.ListScreenHistoryImages(params)
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(list), true

	case ipc.ReqUploadScreenHistory:
		var params ipc.ScreenHistoryPathParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		res, err := a.UploadScreenHistoryImage(params.Path)
		if err != nil {
			return a.errorResponse("上传历史图片失败: " + err.Error()), true
		}
		return a.dataResponse(res), true

	case ipc.ReqDeleteScreenHistory:
		var params ipc.ScreenHistoryPathParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		if err := a.DeleteScreenHistoryImage(params.Path); err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(map[string]bool{"deleted": true}), true

	case ipc.ReqGetScreenImageInfo:
		info, err := a.GetScreenImageInfo()
		if err != nil {
			return a.errorResponse(err.Error()), true
		}
		return a.dataResponse(info), true

	case ipc.ReqReadScreenImage:
		// 很慢（约 2096 帧、几十秒）⇒ 只能由用户显式触发。
		return a.dataResponse(a.readScreenImage()), true

	case ipc.ReqReadScreenImageCache:
		// 只取上次读到的那份，一次设备 IO 都不做：
		// 切页签会让组件卸载，没有这条回来就得再等几十秒重读。
		return a.dataResponse(a.cachedScreenImageOrDefault()), true
	}
	return ipc.Response{}, false
}

// readScreenImage 从设备读回当前屏上那张图，编成 data URL 供界面显示。
// 协议有读图能力（0xC7 起 + 反复 0xC8，约 2096 帧）。
func (a *CoreApp) readScreenImage() ipc.ScreenImageReadResult {
	if a.deviceManager == nil || !a.deviceManager.IsBlackSharkActive() {
		return ipc.ScreenImageReadResult{Error: "当前设备不是黑鲨 BRB02，屏图只对它有意义"}
	}
	raw, ok := a.deviceManager.ReadBlackSharkScreenImage()
	// 读图也是设备独占的批量操作（0xC7 起 + 反复 0xC8，约 2096 帧，几十秒持锁）—— 同上。
	a.noteBulkDeviceOperation()
	if !ok {
		// 读失败 ⇒ 回落到上次读到的那份：读一次要几十秒，重启后不该逼用户再等一次，
		// 但必须说清这是上次的、不是实时的。
		if cached, hasCache := a.cachedScreenImage(); hasCache {
			cached.Error = "本次读回失败，显示的是「上次读到的那份」（不是实时屏图）"
			a.logInfo("屏图读回失败，回落到上次读到的那份（%s）",
				time.Unix(cached.CachedAtUnix, 0).Format("2006-01-02 15:04:05"))
			return cached
		}
		return ipc.ScreenImageReadResult{Error: "从设备读回屏图失败（帧中断或数量不足），详见日志"}
	}
	png, err := screenimg.PNGFromRGB(screenimg.RGB565BEToRGB(raw), screenimg.Width, screenimg.Height)
	if err != nil {
		return ipc.ScreenImageReadResult{Error: "读回来了但编码 PNG 失败: " + err.Error()}
	}
	a.logInfo("已从设备读回屏图：%d 字节，本机 CRC=%04X（无读回校验，字节数已核对）",
		len(raw), screenimg.Crc16Xmodem(raw))
	result := ipc.ScreenImageReadResult{
		Ok:      true,
		DataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
		Bytes:   len(raw),
		CRC:     screenimg.Crc16Xmodem(raw),
	}
	// 读到了 ⇒ 覆盖上次读到的那份。
	a.rememberScreenImageCache(result)
	return result
}

// cachedScreenImageOrDefault 只取上次读到的那份屏图，一次设备 IO 都不做。
func (a *CoreApp) cachedScreenImageOrDefault() ipc.ScreenImageReadResult {
	cached, ok := a.cachedScreenImage()
	if !ok {
		return ipc.ScreenImageReadResult{Ok: false, Error: "本机还没有缓存过屏图，请点「从设备读回」"}
	}
	return cached
}

// 屏图缓存：把上次读到的屏图落盘，供下次直接显示。
const blackSharkScreenCacheFileName = "blackshark-screen-last-read.json"

type blackSharkScreenCacheState struct {
	DataURL string `json:"dataUrl"`
	Bytes   int    `json:"bytes"`
	CRC     uint16 `json:"crc"`
	At      int64  `json:"at"`
}

// screenImageCachePath 返回缓存的落盘路径：优先放在配置文件所在目录
// （cfg.ConfigPath 的目录），取不到才回退到 GetConfigDir()。
func (a *CoreApp) screenImageCachePath() string {
	if a == nil || a.configManager == nil {
		return ""
	}
	dir := ""
	if cfg := a.configManager.Get(); strings.TrimSpace(cfg.ConfigPath) != "" {
		dir = filepath.Dir(strings.TrimSpace(cfg.ConfigPath))
	}
	if strings.TrimSpace(dir) == "" {
		dir = strings.TrimSpace(a.configManager.GetConfigDir())
	}
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, blackSharkScreenCacheFileName)
}

// rememberScreenImageCache 把刚读回来的那张图异步落盘。
// 读屏图路径可能被持 a.mutex 的调用链调到，写文件是 IO，不能占着锁做。
func (a *CoreApp) rememberScreenImageCache(result ipc.ScreenImageReadResult) {
	path := a.screenImageCachePath()
	if path == "" || result.DataURL == "" {
		return
	}
	state := blackSharkScreenCacheState{
		DataURL: result.DataURL,
		Bytes:   result.Bytes,
		CRC:     result.CRC,
		At:      time.Now().Unix(),
	}
	a.safeGo("rememberScreenImageCache", func() {
		if err := a.writeScreenImageCache(state); err != nil {
			a.logDebug("写屏图缓存失败（只是下次要多读一次）: %v", err)
		}
	})
}

// writeScreenImageCache 同步把缓存写进配置目录。
func (a *CoreApp) writeScreenImageCache(state blackSharkScreenCacheState) error {
	path := a.screenImageCachePath()
	if path == "" {
		return fmt.Errorf("拿不到配置目录")
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// cachedScreenImage 取上次读到的那份；读不到或解析不了都返回 false（不猜）。
func (a *CoreApp) cachedScreenImage() (ipc.ScreenImageReadResult, bool) {
	path := a.screenImageCachePath()
	if path == "" {
		return ipc.ScreenImageReadResult{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ipc.ScreenImageReadResult{}, false
	}
	var state blackSharkScreenCacheState
	if err := json.Unmarshal(data, &state); err != nil || strings.TrimSpace(state.DataURL) == "" {
		return ipc.ScreenImageReadResult{}, false
	}
	return ipc.ScreenImageReadResult{
		Ok:           true,
		DataURL:      state.DataURL,
		Bytes:        state.Bytes,
		CRC:          state.CRC,
		FromCache:    true,
		CachedAtUnix: state.At,
	}, true
}

// 预设与上传。
func (a *CoreApp) ListScreenPresets() ([]ipc.ScreenPresetInfo, error) {
	presets, err := screenimg.PresetsInOfficialOrder()
	if err != nil {
		return nil, err
	}
	out := make([]ipc.ScreenPresetInfo, 0, len(presets))
	for i, p := range presets {
		out = append(out, ipc.ScreenPresetInfo{
			OfficialPosition: i + 1,
			AssetIndex:       p.Index,
			Name:             p.Name,
		})
	}
	return out, nil
}

// ScreenPresetThumbnail 返回第 position 张（官方顺序，1..10）的缩略图 data URL。
func (a *CoreApp) ScreenPresetThumbnail(position int) (string, error) {
	presets, err := screenimg.PresetsInOfficialOrder()
	if err != nil {
		return "", err
	}
	if position < 1 || position > len(presets) {
		return "", fmt.Errorf("官方预设位置 %d 超出范围 1..%d", position, len(presets))
	}
	raw, err := screenimg.PresetPNG(presets[position-1].File)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw), nil
}

// UploadScreenPreset 上传第 position 张内置精选（官方顺序，1..10）。
func (a *CoreApp) UploadScreenPreset(position int) (*ipc.ScreenUploadResult, error) {
	presets, err := screenimg.PresetsInOfficialOrder()
	if err != nil {
		return nil, err
	}
	if position < 1 || position > len(presets) {
		return nil, fmt.Errorf("官方预设位置 %d 超出范围 1..%d", position, len(presets))
	}
	_, payload, _, err := screenimg.PresetImage(presets[position-1].Index)
	if err != nil {
		return nil, err
	}
	return a.uploadScreenPayload(payload)
}

// screenCropFromParams 把 IPC 的整数百分比参数换成 screenimg 的裁剪框。
// 换算只在这里做一次：预览与上传都调它，保证看到的就是要传的。
func screenCropFromParams(p ipc.ScreenImagePathParams) screenimg.Crop {
	zoom := float64(p.Zoom) / 100
	if p.Zoom == 0 {
		zoom = screenimg.CropZoomMin
	}
	return screenimg.Crop{
		Zoom:    zoom,
		OffsetX: float64(p.OffsetX) / 100,
		OffsetY: float64(p.OffsetY) / 100,
	}
}

// decodeScreenImage 读入并解码一张本地图片（上传与预览共用）。
func decodeScreenImage(path string) (image.Image, string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, "", fmt.Errorf("未指定图片路径")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("打开图片失败: %w", err)
	}
	defer f.Close()
	img, format, err := image.Decode(f)
	if err != nil {
		return nil, "", fmt.Errorf("解码图片失败（支持 png/jpeg/gif）: %w", err)
	}
	return img, format, nil
}

// PreviewScreenCrop 生成手动裁剪的预览图，不碰设备。
func (a *CoreApp) PreviewScreenCrop(path string, crop screenimg.Crop) (*ipc.ScreenCropPreview, error) {
	img, format, err := decodeScreenImage(path)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	rgb, err := screenimg.CoverToRGBWith(img, screenimg.Width, screenimg.Height, crop)
	if err != nil {
		return nil, err
	}
	pngBytes, err := screenimg.PNGFromRGB(rgb, screenimg.Width, screenimg.Height)
	if err != nil {
		return nil, err
	}
	slackPct := func(srcLen int) int {
		// 归一化后的裁剪窗口在源图上的占比（按与 CoverToRGBWith 相同的换算重算），
		// 只为告诉界面这个方向还能不能动。
		n, _ := crop.Normalize()
		other := float64(screenimg.Height) / float64(b.Dy())
		scale := float64(screenimg.Width) / float64(b.Dx())
		if other > scale {
			scale = other
		}
		scale *= n.Zoom
		var cropLen float64
		if srcLen == b.Dx() {
			cropLen = float64(screenimg.Width) / scale
		} else {
			cropLen = float64(screenimg.Height) / scale
		}
		if cropLen > float64(srcLen) {
			cropLen = float64(srcLen)
		}
		return int(((float64(srcLen) - cropLen) / float64(srcLen)) * 100)
	}
	a.logInfo("屏保图像：裁剪预览 %s（格式 %s，原图 %dx%d，缩放 %d%%，偏移 %d/%d）",
		filepath.Base(path), format, b.Dx(), b.Dy(),
		int(crop.Zoom*100+0.5), int(crop.OffsetX*100+0.5), int(crop.OffsetY*100+0.5))
	return &ipc.ScreenCropPreview{
		DataURL:   "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes),
		Width:     screenimg.Width,
		Height:    screenimg.Height,
		RawWidth:  b.Dx(),
		RawHeight: b.Dy(),
		SlackX:    slackPct(b.Dx()),
		SlackY:    slackPct(b.Dy()),
	}, nil
}

// UploadScreenImageFile 上传一张本地图片（可带手动裁剪框）。
func (a *CoreApp) UploadScreenImageFile(path string, crop screenimg.Crop) (*ipc.ScreenUploadResult, error) {
	img, format, err := decodeScreenImage(path)
	if err != nil {
		return nil, err
	}
	a.logInfo("屏保图像：已读入 %s（格式 %s，%dx%d，缩放 %d%%，偏移 %d/%d）",
		strings.TrimSpace(filepath.Base(path)), format, img.Bounds().Dx(), img.Bounds().Dy(),
		int(crop.Zoom*100+0.5), int(crop.OffsetX*100+0.5), int(crop.OffsetY*100+0.5))
	payload, crc, err := screenimg.EncodeWith(img, crop)
	if err != nil {
		return nil, err
	}
	_ = crc // 编码内部已经算过。信息帧（0xC4）由上行的图传实现自己带，这里只把载荷交给它
	return a.uploadScreenPayload(payload)
}

// ---- 历史图片：本机屏幕图片缓存 ----
//
// 目录 `<安装目录>/screen-images/`（与既有的 `<安装目录>/logs`、`<安装目录>/config` 同级约定），
// 文件名 = 上屏时刻的 Unix 秒，内容恒为 121552 字节（428×142 RGB565 大端）。
//
// **自给自足**：只记我们上传成功过屏的图，不读、也不依赖官方装备箱的任何目录。
// 存的字节就是上传时用的那份，所以再传时既不用解码也不用裁剪，
// 也不会像「从设备读回」那样要几十秒。

// screenHistoryKeep 是本机缓存保留的张数（每张 121552 字节 ≈ 119 KB；30 张 ≈ 3.5 MB）。
// 每成功上屏一次就落一份，所以必须有上限，否则安装目录会一直长大。
const screenHistoryKeep = 30

// screenHistoryCandidate 是缓存目录里的一个候选画布文件。
type screenHistoryCandidate struct {
	path string
	name string
	unix int64
	mod  time.Time
	size int64
}

// screenHistoryDir 返回本机屏幕图片缓存目录。
func screenHistoryDir() string {
	return filepath.Join(config.GetInstallDir(), "screen-images")
}

// screenHistoryCanvasOK 校验一个路径是不是"本机缓存里的画布"：目录内 + `.bin` + 长度正好 PayloadSize。
//
// ★ 目录判定保证**只能删/传我们缓存目录里的东西**，不给任意路径开口子；长度判定挡掉目录里
// 任何非画布的杂项文件。
func screenHistoryCanvasOK(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dir, err := filepath.Abs(screenHistoryDir())
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Dir(abs), dir) {
		return fmt.Errorf("拒绝操作：文件不在本机屏保缓存目录内")
	}
	if !strings.EqualFold(filepath.Ext(abs), ".bin") {
		return fmt.Errorf("拒绝操作：只认官方画布文件（*.bin）")
	}
	st, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if st.Size() != int64(screenimg.PayloadSize) {
		return fmt.Errorf("拒绝操作：画布长度应为 %d，实际 %d", screenimg.PayloadSize, st.Size())
	}
	return nil
}

// readScreenHistoryCanvas 读入一张历史画布（校验后原样返回，不解码）。
func readScreenHistoryCanvas(path string) ([]byte, error) {
	if err := screenHistoryCanvasOK(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取历史画布失败: %w", err)
	}
	if len(data) != screenimg.PayloadSize {
		return nil, fmt.Errorf("历史画布长度应为 %d，实际 %d", screenimg.PayloadSize, len(data))
	}
	return data, nil
}

// screenHistoryThumb 把一张画布编成 `data:image/png;base64,…` 缩略图（失败返回空串）。
func screenHistoryThumb(canvas []byte) string {
	rgb := screenimg.RGB565BEToRGB(canvas)
	png, err := screenimg.PNGFromRGB(rgb, screenimg.Width, screenimg.Height)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

// screenHistoryItem 由候选文件信息构造列表项（Thumb 由调用方按需补）。
func screenHistoryItem(c screenHistoryCandidate) ipc.ScreenHistoryItem {
	return ipc.ScreenHistoryItem{
		Name: c.name, Path: c.path, UnixTime: c.unix,
		Modified: c.mod.Format("2006-01-02 15:04:05"), Size: c.size,
	}
}

// ListScreenHistoryImages 列出本机缓存里的历史图片：按文件名里的 Unix 秒新→旧，取前 Limit 张（**按 CRC 去重**），
// 并单独把"设备当前正在用的那张"作为 Current 带出去（带缩略图）。**纯本机 IO，一次设备 IO 都不做。**
func (a *CoreApp) ListScreenHistoryImages(q ipc.ScreenHistoryQueryParams) (ipc.ScreenHistoryList, error) {
	dir := screenHistoryDir()
	out := ipc.ScreenHistoryList{Dir: dir, Items: []ipc.ScreenHistoryItem{}}
	limit := q.Limit
	if limit <= 0 {
		limit = 2
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		// 目录不存在是常态（一张都还没传过）⇒ 如实说明，不当失败。
		out.Error = "还没有上传过屏保图片"
		return out, nil
	}

	var cands []screenHistoryCandidate
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".bin") {
			continue
		}
		st, err := e.Info()
		if err != nil || st.Size() != int64(screenimg.PayloadSize) {
			continue
		}
		unix, _ := strconv.ParseInt(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())), 10, 64)
		cands = append(cands, screenHistoryCandidate{
			path: filepath.Join(dir, e.Name()), name: e.Name(),
			unix: unix, mod: st.ModTime(), size: st.Size(),
		})
	}
	// 新→旧：文件名即 Unix 秒（官方写盘规则）；同一秒的用 mtime 兜底。
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].unix != cands[j].unix {
			return cands[i].unix > cands[j].unix
		}
		return cands[i].mod.After(cands[j].mod)
	})

	// ★ 这里**不提前 break**：设备当前那张可能比最新两张更旧，提前退出就找不到它了。
	// 代价是把缓存目录扫完（上限 30 张 × 119 KB，CRC 与读盘都在毫秒级）。
	seen := make(map[uint16]bool, limit)
	for _, c := range cands {
		data, err := os.ReadFile(c.path)
		if err != nil || len(data) != screenimg.PayloadSize {
			continue
		}
		// 判据用的是同一份 CRC-16/XMODEM（对 121552 字节画布算，与设备 `0xC5` 回读的 CRC 同源）：
		//  ① 命中"设备当前那张" ⇒ 不进历史两格，单独作为 Current 带出去（顶部预览用它）；
		//  ② 同一张画布只出现一次 —— 每成功上屏一次就落一份，同一张图反复上传会产生
		//     多份一模一样的字节（本机实测：46 个文件只有 11 种画布）。不去重就会把两格填成同一张图。
		crc := screenimg.Crc16Xmodem(data)
		if q.ExcludeCRC != 0 && crc == q.ExcludeCRC {
			if out.Current == nil {
				it := screenHistoryItem(c)
				it.Thumb = screenHistoryThumb(data)
				out.Current = &it
			}
			continue
		}
		if seen[crc] {
			continue
		}
		seen[crc] = true
		item := screenHistoryItem(c)
		// 缩略图只给"要显示的那几张"生成：一张 428×142 的 PNG 有几十 KB，全列表都带会很重。
		if len(out.Items) < limit {
			item.Thumb = screenHistoryThumb(data)
		}
		out.Items = append(out.Items, item)
	}
	if len(out.Items) == 0 {
		out.Error = "缓存目录里没有可用的图片"
	}
	return out, nil
}

// UploadScreenHistoryImage 把历史画布**原样直传**上屏（`*.bin` 免解码、免裁剪）。
//
// 与 UploadScreenImageFile 分开是有意的：那条会先解码图片、再按裁剪框重采样，
// 喂一张裸画布进去只会解码失败或被裁坏；这里要的是"一个字节都不改地发出去"。
func (a *CoreApp) UploadScreenHistoryImage(path string) (*ipc.ScreenUploadResult, error) {
	canvas, err := readScreenHistoryCanvas(path)
	if err != nil {
		return nil, err
	}
	a.logInfo("屏保图像：历史图直传 %s（%d 字节，免解码）", filepath.Base(path), len(canvas))
	return a.uploadScreenPayload(canvas)
}

// DeleteScreenHistoryImage 删除本机缓存里的一张历史图片。
//
// 删的是我们自己的缓存文件，所以只允许"目录内 + `.bin` + 121552 字节"，其余一律拒绝。
// 删除不可逆（直接 os.Remove），界面上用两步确认按钮兜着。
func (a *CoreApp) DeleteScreenHistoryImage(path string) error {
	if err := screenHistoryCanvasOK(path); err != nil {
		return err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if err := os.Remove(abs); err != nil {
		return fmt.Errorf("删除历史图片失败: %w", err)
	}
	a.logInfo("屏保图像：已删除历史图 %s", filepath.Base(abs))
	return nil
}

// screenHistoryNameFor 给出一个不冲突的 `<Unix秒>.bin` 名字（同一秒内连传两张时顺延 1 秒）。
// 名字必须能被 strconv.ParseInt 解析 —— 列表的排序就靠它。
func screenHistoryNameFor(unix int64, dir string) string {
	for i := int64(0); i < 8; i++ {
		name := fmt.Sprintf("%d.bin", unix+i)
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return name
		}
	}
	return fmt.Sprintf("%d.bin", unix+8)
}

// rememberScreenHistoryCanvas 把一张**已成功上屏并回读通过**的画布存进本机缓存。
//
// 存原始画布而不是原图：格式与上传时用的字节完全一致，所以下次再传可以原样直发，免解码免裁剪。
func (a *CoreApp) rememberScreenHistoryCanvas(canvas []byte) {
	if len(canvas) != screenimg.PayloadSize {
		return
	}
	dir := screenHistoryDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		a.logError("屏保图像缓存目录创建失败: %v", err)
		return
	}
	name := screenHistoryNameFor(time.Now().Unix(), dir)
	if err := os.WriteFile(filepath.Join(dir, name), canvas, 0o644); err != nil {
		a.logError("屏保图像缓存写入失败: %v", err)
		return
	}
	a.logInfo("屏保图像已存入本机缓存: %s", name)
	a.pruneScreenHistory(dir)
}

// pruneScreenHistory 只保留最新的 screenHistoryKeep 张（按文件名里的 Unix 秒）。
func (a *CoreApp) pruneScreenHistory(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type cached struct {
		path string
		name string
		unix int64
	}
	var files []cached
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".bin") {
			continue
		}
		unix, perr := strconv.ParseInt(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())), 10, 64)
		if perr != nil {
			continue
		}
		files = append(files, cached{filepath.Join(dir, e.Name()), e.Name(), unix})
	}
	if len(files) <= screenHistoryKeep {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].unix > files[j].unix })
	for _, old := range files[screenHistoryKeep:] {
		if err := os.Remove(old.path); err == nil {
			a.logInfo("屏保图像缓存已清理旧图: %s", old.name)
		}
	}
}

// GetScreenImageInfo 读回设备当前保存的屏保图像信息（0xC5）。
func (a *CoreApp) GetScreenImageInfo() (ipc.ScreenImageInfo, error) {
	if a.deviceManager == nil || !a.deviceManager.IsBlackSharkProfileActive() {
		return ipc.ScreenImageInfo{}, fmt.Errorf("当前设备不是黑鲨 BRB02，无法读取屏幕图信息")
	}
	info, ok := a.deviceManager.ReadScreenImageInfo()
	if !ok {
		// 读不到不是错误：设备可能从未设过屏保图。用 HasImage=false 表达。
		return ipc.ScreenImageInfo{HasImage: false}, nil
	}
	return ipc.ScreenImageInfo{
		HasImage:  info.Size > 0,
		Timestamp: info.Timestamp,
		Size:      info.Size,
		CRC:       info.CRC,
	}, nil
}

// broadcastScreenImageTransferProgress 把上传进度推给界面（图传弹窗里的进度条）。
// 设备层已按帧节流（每 16 帧一次），这里不再重复节流。
func (a *CoreApp) broadcastScreenImageTransferProgress(sent, total int) {
	if a == nil || a.ipcServer == nil {
		return
	}
	a.ipcServer.BroadcastEvent(ipc.EventScreenImageTransferProgress, map[string]int{
		"sent":  sent,
		"total": total,
	})
}

// uploadScreenPayload 走设备层完成上传，并把结果转成 IPC 载荷。
//
// 图传只有一条实现：设备层的 SendBlackSharkImageWithProgress（USB 走 libusb 中断端点）。
func (a *CoreApp) uploadScreenPayload(payload []byte) (*ipc.ScreenUploadResult, error) {
	if a.deviceManager == nil {
		return nil, fmt.Errorf("设备管理器未就绪")
	}
	// 档案例据（不是 HID 通路判据）：黑鲨现在有 BLE / USB 两条档案。
	if !a.deviceManager.IsBlackSharkProfileActive() {
		return nil, fmt.Errorf("当前设备不是黑鲨 BRB02，屏保图像只对它有意义")
	}
	// 登记成一次设备独占任务（上游那套形状）：重复传输会被挡住，断开/挂起时能被取消；
	// 结束时刻由登记表留痕，监控循环据此把这段停摆归因到本机（见 selfInflictedMonitorStall）。
	ctx, done, err := a.beginBlackSharkImageTransfer(a.ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	if err := a.deviceManager.SendBlackSharkImageWithProgress(ctx, payload, a.broadcastScreenImageTransferProgress); err != nil {
		return nil, err
	}

	reports := (len(payload) + deviceproto.BlackSharkImageFramePayload - 1) /
		deviceproto.BlackSharkImageFramePayload
	short := len(payload) - (reports-1)*deviceproto.BlackSharkImageFramePayload
	// 回读校验：上传序列的最后一帧就是 `0xC5`（上游叫 ImageCommit），但它的应答没人消费，
	// 所以这里单独再问一次设备"你手上那张图是什么"。
	info, verified := a.verifyBlackSharkScreenImage(payload)
	if !verified {
		// 不谎报成功：写确实发出去了，但设备回读不出刚写的那张图 ⇒ 按失败上报
		// （与 screen image verification failed 同一个口径）。
		return nil, fmt.Errorf(
			"屏保图像回读校验未通过：设备持有 size=%d crc=0x%04X，本机写入 size=%d crc=0x%04X"+
				"（图像可能已写到设备上，但无法确认）",
			info.Size, info.CRC, len(payload), deviceproto.BlackSharkCRC16XMODEM(payload))
	}
	a.logInfo("屏保图像上传完成：%d 个报文（末报载荷 %d 字节），回读 size=%d crc=0x%04X 与写入一致",
		reports, short, info.Size, info.CRC)
	// 只有"上屏成功且回读通过"才进本机缓存：失败的半张图留在缓存里没有意义。
	a.rememberScreenHistoryCanvas(payload)
	return &ipc.ScreenUploadResult{
		Reports:   reports,
		Short:     short,
		Timestamp: info.Timestamp,
		Size:      info.Size,
		CRC:       info.CRC,
		Verified:  true,
	}, nil
}

// 屏保上传回读校验的次数与间隔。
//
// 需要重试：上传序列末尾那条 `0xC5` 的应答没人消费，设备也可能还没把新信息落好，
// 一次读回拿到旧值是可能的，重试一两次可避免把"暂时读到旧值"误判成"没写进去"。
const (
	blackSharkScreenVerifyAttempts   = 3
	blackSharkScreenVerifyRetryDelay = 200 * time.Millisecond
)

// verifyBlackSharkScreenImage 读回 `0xC5`，把它与刚写下去的那张图比对。
//
// 判据只用大小 + CRC：新通路的信息帧由传输侧组装（`deviceproto.BuildBlackSharkImageMetadata`），
// 那里前 4 字节是固定常量（`0x6AC0DE18`）而不是本机时间戳，时间戳不可比。
func (a *CoreApp) verifyBlackSharkScreenImage(payload []byte) (screenimg.Info, bool) {
	if a.deviceManager == nil {
		return screenimg.Info{}, false
	}
	wantSize := uint32(len(payload))
	wantCRC := deviceproto.BlackSharkCRC16XMODEM(payload)
	var last screenimg.Info
	for attempt := 1; attempt <= blackSharkScreenVerifyAttempts; attempt++ {
		if attempt > 1 {
			time.Sleep(blackSharkScreenVerifyRetryDelay)
		}
		got, ok := a.deviceManager.ReadScreenImageInfo()
		if !ok {
			a.logInfo("屏保上传回读校验：第 %d/%d 次没读到设备持有的图像信息",
				attempt, blackSharkScreenVerifyAttempts)
			continue
		}
		if got.Size == wantSize && got.CRC == wantCRC && got.Size > 0 {
			return got, true
		}
		last = got
		a.logInfo("屏保上传回读校验：第 %d/%d 次不一致（设备 size=%d crc=0x%04X，本机 size=%d crc=0x%04X）",
			attempt, blackSharkScreenVerifyAttempts, got.Size, got.CRC, wantSize, wantCRC)
	}
	return last, false
}
