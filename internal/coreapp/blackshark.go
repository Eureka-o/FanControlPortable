// 黑鲨 BRB02 的控制面：IPC 分发、四档曲线、LCD 与系统信息推送，以及主机侧灯效驱动与情景规则。
package coreapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	"github.com/Eureka-o/FanControlPortable/internal/device"
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
		info := a.blackSharkInfoCached()
		if _, _, cached := a.blackSharkStateCached(); !cached {
			a.refreshBlackSharkStateAsync("cold-cache")
		}
		// 固件那一项回落到落盘的那份检查结果：设备侧缓存只在内存里、重启即失，
		// 不回落时冷启动就只剩一个空壳（面板上版本显示 "-"）。
		if strings.TrimSpace(info.Firmware.CurrentVersion) == "" {
			info.Firmware = a.GetBlackSharkFirmwareStatus()
		}
		return a.dataResponse(info), true

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
		ok := a.deviceManager.SetBlackSharkLightingEnabled(params.Enabled)
		a.afterBlackSharkSwitchWrite(ok, func(states *types.BlackSharkSwitchStates) {
			states.LightingEnabled = params.Enabled
			states.LightingKnown = true
		})
		return a.successResponse(ok), true

	case ipc.ReqSetBlackSharkLcdScreenEnabled:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		// 主机侧调速接管期间锁死这只开关：屏保状态是主开关的联动结果，允许手改等于把联动拆掉
		// （设备会在固定转速形态下继续跑屏保，而主开关仍显示"已接管"）。要改先关主开关。
		if a.blackSharkHostControlsSpeed() {
			return a.errorResponse("LCD 屏保已由「Fan control 控制变频转速」接管，请先关闭该开关"), true
		}
		ok := a.deviceManager.SetBlackSharkLcdScreenEnabled(params.Enabled)
		a.afterBlackSharkSwitchWrite(ok, func(states *types.BlackSharkSwitchStates) {
			states.LcdScreenEnabled = params.Enabled
			states.LcdScreenKnown = true
		})
		return a.successResponse(ok), true

	case ipc.ReqSetBlackSharkOnOffVector:
		var params ipc.BlackSharkOnOffVectorParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		// 有读回：写 0x02 之后按 0x03 回读确认（见 device 层的 SetBlackSharkOnOffVector）。
		ok := a.deviceManager.SetBlackSharkOnOffVector(params.SmartStartStop, params.PowerOnSelfStart)
		a.afterBlackSharkSwitchWrite(ok, func(states *types.BlackSharkSwitchStates) {
			states.SmartStartStop = params.SmartStartStop
			states.PowerOnSelfStart = params.PowerOnSelfStart
			states.OnOffVectorKnown = true
		})
		return a.successResponse(ok), true

	case ipc.ReqSetBlackSharkHostVariableSpeed:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		// 本机设置（不改设备开关）：只切变频实现方并让设备按新形态生效。
		if err := a.SetBlackSharkHostVariableSpeed(params.Enabled); err != nil {
			return a.errorResponse("切换变频实现方失败: " + err.Error()), true
		}
		return a.successResponse(true), true

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

	case ipc.ReqSetBlackSharkRgbModeAll:
		var params ipc.BlackSharkRgbModeAllParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		// 色相 → RGB 仍由 deviceproto 唯一实现（等价于官方的 QColor::setHsv），这里只做转发。
		view, ok := a.deviceManager.SetBlackSharkRgbModeAll(
			params.Index, device.BlackSharkRgbModeEdit{
				Speed:       params.Speed,
				Brightness:  params.Brightness,
				Hue:         params.Hue,
				ColorOption: params.ColorOption,
				StaticColor: params.StaticColor,
				HasStatic:   params.HasStatic,
			})
		if !ok {
			return a.errorResponse("黑鲨灯效保存失败（设备未确认受理）"), true
		}
		// 把保存后的权威视图直接回给界面：界面据此**原地更新**，不必再整页重读（省掉 0x11 + 0x13）。
		return a.dataResponse(view), true

	case ipc.ReqGetBlackSharkRgbModeView:
		// 只读指定槽位（0x14，不切换）。读不到就报错，让前端保持"只能选中"的降级形态，
		// 不回落零值 —— 零值会被当成设备现状拿去初始化编辑状态。
		var params ipc.SetIntParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error()), true
		}
		mode, ok := a.deviceManager.BlackSharkRgbModeView(params.Value)
		if !ok {
			return a.errorResponse("读取灯效模式参数失败（设备无响应）"), true
		}
		return a.dataResponse(mode), true

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
		// 主机侧调速下那些曲线写入只是过渡态：下一拍重新下发目标转速（form=0）把设备拉回来
		// （例行下发要等目标变化，可能几分钟）。
		// 快照里也带着屏幕开关（LcdEnabled），还原可能把屏保重新打开 ⇒ 按主开关再压一次。
		if a.blackSharkHostControlsSpeed() {
			if a.autoControlActive() {
				// 快照还原可能把真实曲线写进了设备 ⇒ 作废上次下发记忆，下一拍必须重发。
				a.deviceManager.ResetBlackSharkSpeedWriteState()
				a.forceNextAutoTarget.Store(true)
			}
			if err := a.applyBlackSharkLcdScreensaverForHostSpeed(true); err != nil {
				a.logError("快照还原后同步 LCD 屏保失败: %v", err)
			}
		}
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
	a.rememberBlackSharkCoolingSource(source)
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
	a.rememberBlackSharkCoolingSource(uint8(want))
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

// resetAllBlackSharkGearCurves 把四个档位的方案曲线都恢复成出厂值，并把当前档位的出厂曲线写进设备。
//
// 写 0x24 的档位字段就是"切档"。早期实现逐档写四遍：既会把设备停在最后写入的槽（超频）、
// 又要额外拨回来，四档连续写入还容易撞上设备对控制查询的偶发沉默（0x25 回读超时 ⇒ 界面报
// "操作未生效"）。现在只把四档出厂曲线恢复进本机方案（config），设备侧**只写当前档位一次**：
// 其余三档由切档时的既有下发逻辑（applyBlackSharkGear 从本机方案读该档曲线）自然覆盖，
// 切挡位时前端曲线同样来自 config，行为闭环。
func (a *CoreApp) resetAllBlackSharkGearCurves() bool {
	if a == nil || a.deviceManager == nil || !a.deviceManager.IsBlackSharkActive() {
		return false
	}
	if a.blackSharkHostControlsSpeed() {
		// 主机侧调速下**没有四档曲线**（曲线集换成通用单方案，见 blackSharkHostSpeedCurveSuffix），
		// 所以这里的"重置"语义和飞智那种普通设备一致：只把**当前这条曲线**还原成默认（17 点 RPM），
		// 设备侧一个字节都不写。别去碰四档 —— 那个模式下四档既不存在、也不参与运行。
		return a.resetBlackSharkHostSpeedCurve()
	}
	previous := a.activeBlackSharkGear()
	ok := true
	for gear := 1; gear <= deviceproto.BlackSharkGearCount; gear++ {
		if !a.syncBlackSharkGearProfileCurve(gear) {
			ok = false
		}
	}
	if previous >= 1 && previous <= deviceproto.BlackSharkGearCount {
		if _, err := a.SetActiveFanCurveProfile(types.BlackSharkGearCurveProfileID(previous)); err != nil {
			a.logError("恢复四档出厂曲线后切回原档位失败: %v", err)
			ok = false
		}
	}
	// 设备侧只写当前档位一次：写 previous = 档位不变、曲线换新（0x24 写即切档）。
	switch {
	case previous >= 1 && previous <= deviceproto.BlackSharkGearCount && a.autoControlActive():
		if !a.applyBlackSharkGear(previous) {
			a.logError("恢复四档出厂曲线后把当前档位 %d 写进设备失败", previous)
			ok = false
		}
	case a.autoControlActive():
		// 当前方案不是四档之一（自定义方案/未选中）：设备一个字节都不动，新曲线在切档时生效。
		a.logInfo("恢复四档出厂曲线：当前方案不是四档之一，设备保持原样；切档时新曲线生效")
	default:
		if err := a.applyCurrentGearSetting(); err != nil {
			a.logError("恢复四档出厂曲线后重写当前挡位失败: %v", err)
			ok = false
		}
	}
	return ok
}

// resetBlackSharkHostSpeedCurve 把「主机侧调速」的曲线集还原成通用默认（17 点 RPM）。
//
// 与四档那条重置互斥：主机侧调速下转速由主机逐拍下发（form=0 固定值），
// 重置改的是主机侧 17 点曲线 ⇒ 只改本机、不写设备；rpm 变化由下一拍下发新值。
func (a *CoreApp) resetBlackSharkHostSpeedCurve() bool {
	if a == nil || a.configManager == nil {
		return false
	}
	cfg := a.configManager.Get()
	unit := a.activeDeviceSpeedUnit(&cfg)
	key := a.activeDeviceCurveScopeKey(cfg)
	if key == "" {
		return false
	}
	_, err := a.commitConfigMutation(func(c *types.AppConfig) {
		applyDeviceFanCurveStateForUnit(c, defaultDeviceFanCurveStateForUnit(unit), unit)
		storeDeviceFanCurveStateForKeyAndUnit(c, key, *c, unit)
		c.SmartControl, _ = smartcontrol.NormalizeConfigForUnit(c.SmartControl, a.smartControlCurveInterp(c), c.DebugMode, unit)
		syncSmartControlOffsetsForDeviceKey(c, key)
	}, nil)
	if err != nil {
		a.logError("还原主机侧调速曲线失败: %v", err)
		return false
	}
	a.forceNextAutoTarget.Store(true)
	a.logInfo("主机侧调速：曲线已还原为通用默认曲线，设备侧未写")
	return true
}

// activeBlackSharkGear 返回当前应作用于设备的黑鲨档位（1..4）；不是四档方案时返回 0。
// 真值来源是当前曲线方案 ID（四档方案 ↔ 档位，与「切方案=切档位」同一套映射，单一所有者）。
// 主机侧调速下曲线集是通用单方案（"default"），这里恒返回 0 —— 上游本来就没有档位概念。
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

// blackSharkHostControlsSpeed 报告黑鲨变频此刻由谁实现：true = 主机侧（目标转速以 form=0 固定值发进设备），
// false = 设备侧跟随（真实曲线写进设备）。
//
// 这是「变频实现方」的唯一判据，别在别处再判一次配置字段：分流点有三处
// （setTargetSpeedWithMode 的变频分支、applyBlackSharkGear 的曲线写入、监控循环的曲线目标显示），
// 判据一旦分叉就会出现"主机以为在写下发、设备以为在看曲线"的半套状态。
func (a *CoreApp) blackSharkHostControlsSpeed() bool {
	if a == nil || a.configManager == nil {
		return false
	}
	return a.configManager.Get().BlackSharkHostVariableSpeed
}

// applyBlackSharkLcdScreensaverForHostSpeed 让 LCD 屏保跟随「Fan control 控制变频转速」开关。
//
// 协议里屏保只有一族开关：`0xC0` 写 / `0xC1` 读（设备侧字符串也只有 LcdScreenSwitch；
// `0xC4`/`0xC5` 是屏保图像的内容，不是开关）。所以"关闭屏保" = 把 `0xC0` 写成关。
//
// 语义与开关一一对应，两个方向都幂等：
//   - 开关开：第一次接管时**先记下设备当时的屏保状态**（已有记录就不覆盖），再把屏保关掉；
//   - 开关关：只有当初确实是开着的才恢复；恢复成功后才清记录 —— 设备不在线时**保留记录**，
//     由下次连接补做，否则屏保会被永久留在关闭态。
//
// 没连黑鲨 / 开关关着且无待恢复记录 ⇒ 直接返回，零设备 IO。
func (a *CoreApp) applyBlackSharkLcdScreensaverForHostSpeed(hostSpeed bool) error {
	if a == nil || a.configManager == nil || a.deviceManager == nil {
		return nil
	}
	if !a.deviceManager.IsBlackSharkActive() {
		return nil // 没连黑鲨：没有屏保可动
	}
	record := a.configManager.Get().BlackSharkLcdScreenBeforeHostSpeed
	if !hostSpeed {
		// 退出接管**不读设备**：要不要恢复只看记录。读 0xC1 在 BLE 上经常超时，
		// 一旦被读挡住，本该恢复的屏保就永远回不来了。
		return a.restoreBlackSharkLcdScreensaver(record)
	}

	// 接管方向必须知道"原来是不是开着的"，所以这里要读一次；读不到就退到本机缓存那份，
	// 仍不知道才放弃（宁可不动，也不记一个猜出来的值）。
	current, known := a.deviceManager.BlackSharkLcdScreenEnabled()
	if !known {
		if cached, _, ok := a.blackSharkStateCached(); ok && cached.Switches.LcdScreenKnown {
			current = cached.Switches.LcdScreenEnabled
			known = true
			a.logInfo("0xC1 未应答：接管前屏保状态取自本机缓存（%v）", current)
		}
	}
	if !known {
		return fmt.Errorf("读不回 LCD 屏保状态（0xC1）")
	}

	if record == nil {
		// 记录必须先落盘再关屏：中间崩溃也不会丢掉"当初是开着的"这个事实。
		a.setBlackSharkLcdScreensaverRecord(&current)
	}
	if !current {
		return nil // 已经关着，不必写
	}
	if !a.deviceManager.SetBlackSharkLcdScreenEnabled(false) {
		return fmt.Errorf("关闭 LCD 屏保未被设备确认")
	}
	a.applyVerifiedBlackSharkSwitch(func(states *types.BlackSharkSwitchStates) {
		states.LcdScreenEnabled = false
		states.LcdScreenKnown = true
	})
	a.logInfo("主机侧调速已接管：LCD 屏保已关闭（接管前是开着的）")
	return nil
}

// restoreBlackSharkLcdScreensaver 退出接管时按记录恢复屏保；没有记录 ⇒ 什么都不做。
//
// 只依赖记录、不读设备：这是"关闭开关后屏保必须回来"的唯一通路，
// 不能再被一次 0xC1 超时挡掉（读失败 ⇒ 永不恢复，且界面上还会显示成已恢复）。
func (a *CoreApp) restoreBlackSharkLcdScreensaver(record *bool) error {
	if record == nil {
		return nil
	}
	if *record {
		// 写 + 回读确认在设备侧是"ACK 与 0xC1 读回"两条独立判据，BLE/USB 上都可能偶发失败；
		// 重试几次再放弃 —— 一次失败就把屏保留在关闭态是用户可见的功能缺失。
		var lastErr error
		for attempt := 1; attempt <= 3; attempt++ {
			if a.deviceManager.SetBlackSharkLcdScreenEnabled(true) {
				a.applyVerifiedBlackSharkSwitch(func(states *types.BlackSharkSwitchStates) {
					states.LcdScreenEnabled = true
					states.LcdScreenKnown = true
				})
				a.logInfo("已退出主机侧调速：LCD 屏保已恢复（第 %d 次写入生效）", attempt)
				a.setBlackSharkLcdScreensaverRecord(nil)
				return nil
			}
			lastErr = fmt.Errorf("恢复 LCD 屏保未被设备确认（第 %d 次）", attempt)
			time.Sleep(400 * time.Millisecond)
		}
		return lastErr
	}
	a.logInfo("已退出主机侧调速：接管前屏保本来就是关的，不写设备")
	// 只有在恢复动作有了结论之后才清记录：写失败时保留记录，下次连接/切换会再补一次。
	a.setBlackSharkLcdScreensaverRecord(nil)
	return nil
}

// setBlackSharkLcdScreensaverRecord 写入/清空"接管前的屏保状态"记录（nil = 清空）。
func (a *CoreApp) setBlackSharkLcdScreensaverRecord(value *bool) {
	if a == nil || a.configManager == nil {
		return
	}
	var stored *bool
	if value != nil {
		snapshot := *value
		stored = &snapshot
	}
	if _, err := a.commitConfigMutation(func(cfg *types.AppConfig) {
		cfg.BlackSharkLcdScreenBeforeHostSpeed = stored
	}, nil); err != nil {
		a.logError("保存 LCD 屏保接管记录失败: %v", err)
	}
}

// SetBlackSharkHostVariableSpeed 切换黑鲨变频的实现方，并让设备按新形态立刻生效。
//
// 两套实现互斥，切换时各自补一次"生效动作"：
//   - 切到主机侧调速：下一拍按目标转速向设备发 form=0 固定值帧（上游同款，见 device 层 setBlackSharkHIDTargetSpeedLocked）；
//   - 切回设备侧跟随：把该档真实曲线重新写进设备（否则设备停在 form=0 固定转速，而界面以为在变频）。
//
// 另外主开关联动 LCD 屏保（黑鲨固件在固定转速形态下本来也不跑屏保）：开 ⇒ 关屏保并记账，
// 关 ⇒ 按记账恢复。联动失败只记日志、不回滚开关 —— 开关本身是"谁算转速"，不该因为一次
// 设备写失败就翻回去（下一次切换/连接会再补一次）。
func (a *CoreApp) SetBlackSharkHostVariableSpeed(enabled bool) error {
	if a == nil || a.configManager == nil {
		return fmt.Errorf("运行环境未就绪")
	}

	a.mutex.Lock()
	cfg := a.configManager.Get()
	changed := cfg.BlackSharkHostVariableSpeed != enabled
	unit := a.activeDeviceSpeedUnit(&cfg)
	// 先把手上的曲线存回**当前实现方**那一桶，再翻开关、载入另一桶：
	// 主机侧调速用的是通用单方案曲线（默认 17 点），设备侧跟随用的是四档方案（各 4 点）。
	// 不换桶的话，切到上游实现后曲线页仍是那 4 点。
	previousKey := a.activeDeviceCurveScopeKey(cfg)
	if changed && previousKey != "" {
		storeDeviceFanCurveStateForKeyAndUnit(&cfg, previousKey, cfg, unit)
	}
	cfg.BlackSharkHostVariableSpeed = enabled
	if changed {
		if nextKey := a.activeDeviceCurveScopeKey(cfg); nextKey != "" && nextKey != previousKey {
			loadDeviceFanCurveStateForKey(&cfg, nextKey, unit, false)
			// 换曲线集必须同时把学习偏移按新曲线定长，并切到新桶的学习值：
			// 4 点 ↔ 17 点的 LearnedOffsets 长度不同，不归一化会让下标静默错位（曲线页也会抖）。
			cfg.SmartControl, _ = smartcontrol.NormalizeConfigForUnit(
				cfg.SmartControl, a.smartControlCurveInterp(&cfg), cfg.DebugMode, unit)
			syncSmartControlOffsetsForDeviceKey(&cfg, nextKey)
			storeDeviceFanCurveStateForKeyAndUnit(&cfg, nextKey, cfg, unit)
		}
	}
	err := a.commitConfigUpdate(cfg, func(types.AppConfig) {
		if !changed {
			return
		}
		if enabled {
			// 设备侧那两项"曲线跟随"显示状态作废：主机侧调速下它们不再更新，留着会被状态帧沿用。
			if a.deviceManager != nil {
				a.deviceManager.ResetBlackSharkCurveDisplay()
				// 模式刚切 ⇒ 设备里生效的值不再是上次发的，下一拍必须真发一次。
				a.deviceManager.ResetBlackSharkSpeedWriteState()
			}
			a.forceNextAutoTarget.Store(true)
			a.logInfo("黑鲨变频改为 FanControl 控制转速：主机按目标转速向设备发 form=0 固定值")
			return
		}
		a.logInfo("黑鲨变频改回设备侧跟随：曲线写进设备，主机只推温度")
	})
	a.mutex.Unlock()
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if enabled {
		// 开：先关屏保（0xC1 读 + 0xC0 写），不持 a.mutex。下一拍由 forceNextAutoTarget 触发下发。
		if err := a.applyBlackSharkLcdScreensaverForHostSpeed(true); err != nil {
			a.logError("切换到主机侧调速后关闭 LCD 屏保失败: %v", err)
		}
		return nil
	}
	// 关：**先把设备写回"设备侧跟随"形态，再还屏保**。
	// 固件在固定转速形态下不接受"开屏"（0xC0=1 写入后 0xC1 回读仍是 0），
	// 所以这两步必须有先后，且顺序反过来就会"屏保永远回不来"。
	a.safeGo("exitBlackSharkHostSpeed", func() {
		if a.autoControlActive() {
			if gear := a.activeBlackSharkGear(); gear >= 1 {
				if a.applyBlackSharkGear(gear) {
					a.logInfo("已退出主机侧调速：档位 %d 的曲线已写回设备", gear)
				} else {
					a.logError("已退出主机侧调速，但把档位 %d 的曲线写回设备失败", gear)
				}
			}
		}
		// 设备形态切换需要一点点时间，这里留 300 ms 再开屏。
		time.Sleep(300 * time.Millisecond)
		if err := a.applyBlackSharkLcdScreensaverForHostSpeed(false); err != nil {
			a.logError("退出主机侧调速后恢复 LCD 屏保失败: %v", err)
		}
	})
	return nil
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
// 变频有两种实现方（见 SetBlackSharkHostVariableSpeed）：设备侧跟随才写曲线；
// 主机侧调速只把下一拍标成"必须下发"，曲线留在本机参与插值。
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
	if a.blackSharkHostControlsSpeed() {
		// 主机侧调速形态下设备每拍收到的是 form=0 固定值（目标转速，不含形状）：切档不写设备，
		// 下一拍下发时按设备回报的当前档写值；退出主机侧调速时 applyBlackSharkGear 会把该档
		// 真实曲线写回并切档。这里只把下一拍标成"必须下发"。
		a.forceNextAutoTarget.Store(true)
		a.logInfo("档位 %d：当前由 FanControl 控制转速，下一拍按目标转速下发", gear)
		return true
	}
	points, ok := a.blackSharkDeviceCurvePoints(gear)
	if !ok {
		return false
	}
	if !a.deviceManager.SetBlackSharkCoolingCurve(byte(gear), points) {
		a.logError("施加黑鲨档位 %d 失败：曲线写入未获回读确认", gear)
		return false
	}
	// 记账：这一档的曲线已被设备回读确认（SetBlackSharkCoolingCurve 内部做了 0x26 比对），
	// 于是下次进页面不必为它再读一次。
	a.rememberBlackSharkCurveVerified(gear, points)
	a.logInfo("黑鲨档位 %d 已施加：设备侧曲线 = %s（含已学偏移，含端点补齐后的插值依据烘焙）",
		gear, deviceproto.BlackSharkCurveString(points))
	return true
}

// rememberBlackSharkCurveVerified 把"刚经设备回读确认的一条档位曲线"记进 config 缓存。
//
// 只改那一档，零额外设备 IO（SetBlackSharkCoolingCurve 内部已做"写 0x24 → 读 0x26 比对"）。
// 已知缺口：手动模式的固定值路径（applyManualGearForCurveProfile）拿不到档位号，那一项暂不记账，
// 会在下一次冷读时补齐。
func (a *CoreApp) rememberBlackSharkCurveVerified(
	gear int, points [deviceproto.BlackSharkCurvePointCount]deviceproto.BlackSharkCurvePoint,
) {
	if a == nil {
		return
	}
	if gear < 1 || gear > deviceproto.BlackSharkGearCount {
		return
	}
	snapshot, _, ok := a.blackSharkStateCached()
	if !ok {
		return
	}
	curve := make([]types.BlackSharkCurvePointView, 0, len(points))
	for _, pt := range points {
		curve = append(curve, types.BlackSharkCurvePointView{
			TempC:           pt.TempC,
			FieldRPM:        pt.RPM,
			ApproxActualRPM: deviceproto.BlackSharkCurveRPMForField(int(pt.RPM)),
		})
	}
	idx := -1
	for i := range snapshot.Gears {
		if snapshot.Gears[i].Gear == gear {
			idx = i
			break
		}
	}
	if idx < 0 {
		// 缓存里没有这一档（例如那份缓存来自另一台档位不同的设备）：不动它，等冷读补齐。
		return
	}
	if snapshot.Gears[idx].CurveKnown && len(snapshot.Gears[idx].Curve) == len(curve) {
		same := true
		for i := range curve {
			if snapshot.Gears[idx].Curve[i] != curve[i] {
				same = false
				break
			}
		}
		if same {
			return // 没变：不写盘
		}
	}
	snapshot.Gears[idx].Curve = curve
	snapshot.Gears[idx].CurveKnown = true
	a.rememberBlackSharkStateCache(snapshot)
}

// reassertBlackSharkInverterGear 在「变频刚刚生效」的时刻让设备进入变频形态。
//
// 设备侧跟随下变频的输入是曲线（形状写在设备里），所以每个"变频生效"的入口都必须显式补一次
// 曲线写入：开启自动控制（config_control）、温度遥测恢复 / 自动控制刚生效（monitoring）、
// 连接后（system_device 的 postConnectSync）。漏掉任一入口，设备会停在 `form=0`（固定转速）
// 形态，而界面以为已经在变频。
//
// 主机侧调速下没有曲线要写，这里退化成"标记下一拍必须下发"（0x24 固定值形态本身即是生效动作）。
//
// 曲线写入异步执行：三个调用点分别来自配置提交线程、监控采样线程与连接协程，
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
	if a.blackSharkHostControlsSpeed() {
		// 主机侧调速：只要让下一拍**一定**重新下发一次目标转速
		// （设备此刻可能停在旧曲线形态或安全回退写的值上，不能靠"目标没变"跳过）。
		a.deviceManager.ResetBlackSharkSpeedWriteState()
		a.forceNextAutoTarget.Store(true)
		a.logInfo("变频已生效（%s）：当前是 FanControl 控制转速，下一拍重发固定值", caller)
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

// ── 黑鲨状态的"config 优先"读路径 ────────────────────────────────────────────
//
// BlackSharkInfo() 一次要串行 11 条查询，BLE 上单条可到秒级 ⇒ 每次进页面都现读会把界面卡住
// 十几秒，这正是"每次点进设备页都要重新刷新加载"的来源。
//
// 改法对齐官方（docs §13.7：页面初始化"一条查询填满一页"、持续状态靠推送）：
//
//	读：先给 config 里落盘的那份（零设备 IO、立即返回），后台单飞回读校验，只有不同才落盘并推事件。
//	写：下发 → 设备回读确认 → 确认了才记账；没生效就不记账，并把权威快照推给界面让开关弹回。
//
// blackSharkStateCached 取上次读到的那份状态缓存。
func (a *CoreApp) blackSharkStateCached() (types.BlackSharkStateSnapshot, int64, bool) {
	if a == nil || a.configManager == nil {
		return types.BlackSharkStateSnapshot{}, 0, false
	}
	cfg := a.configManager.Get()
	if cfg.BlackSharkStateCache == nil {
		return types.BlackSharkStateSnapshot{}, 0, false
	}
	return *cfg.BlackSharkStateCache, cfg.BlackSharkStateCacheAtUnix, true
}

// blackSharkSnapshotEqual 判断两份状态是否"实质相同"（忽略时间戳）。
func blackSharkSnapshotEqual(a, b types.BlackSharkStateSnapshot) bool {
	if len(a.Gears) != len(b.Gears) {
		return false
	}
	if a.Switches != b.Switches {
		return false
	}
	for i := range a.Gears {
		x, y := a.Gears[i], b.Gears[i]
		if x.Gear != y.Gear ||
			x.FixedValue != y.FixedValue || x.FixedKnown != y.FixedKnown ||
			x.CurveKnown != y.CurveKnown || len(x.Curve) != len(y.Curve) {
			return false
		}
		for j := range x.Curve {
			if x.Curve[j] != y.Curve[j] {
				return false
			}
		}
	}
	return true
}

// rememberBlackSharkStateCache 把刚回读到的状态落盘。返回"是否真的写入了"。
func (a *CoreApp) rememberBlackSharkStateCache(snapshot types.BlackSharkStateSnapshot) bool {
	if a == nil || a.configManager == nil {
		return false
	}
	if prev, _, ok := a.blackSharkStateCached(); ok && blackSharkSnapshotEqual(prev, snapshot) {
		return false
	}
	at := time.Now().Unix()
	if _, err := a.configManager.MutateAndSave(func(cfg *types.AppConfig) {
		copySnapshot := snapshot
		cfg.BlackSharkStateCache = &copySnapshot
		cfg.BlackSharkStateCacheAtUnix = at
	}); err != nil {
		a.logDebug("缓存黑鲨状态失败（只是下次要多读一次）: %v", err)
		return false
	}
	return true
}

// blackSharkInfoCached 组出"给界面的那份"：config 缓存 + 纯计算字段 + 固件缓存。
func (a *CoreApp) blackSharkInfoCached() types.BlackSharkInfo {
	if a == nil || a.deviceManager == nil || !a.deviceManager.IsBlackSharkProfileActive() {
		// 真的不是黑鲨 / 没连上：这里的 available=false 是**实话**，不是"读取失败"。
		return types.BlackSharkInfo{}
	}
	snapshot, at, ok := a.blackSharkStateCached()
	// 有缓存就铺缓存，没有就给一份空快照 —— 两条路都走同一个还原函数，
	// 为的是**量程与标定表永远被补齐**（没有它，首次进页面曲线控件是残的）。
	info := a.deviceManager.BlackSharkInfoFromSnapshot(snapshot)
	info.Available = true
	info.Firmware = a.GetBlackSharkFirmwareStatus()
	info.FromCache = ok
	info.CachedAtUnix = at
	info.Refreshing = a.blackSharkStateRefreshing.Load()
	// 变频实现方是本机设置（不来自设备），跟设备回读无关 ⇒ 这里每份都给，界面才不必再查一次配置。
	info.HostVariableSpeed = a.blackSharkHostControlsSpeed()
	return info
}

// refreshBlackSharkStateAsync 后台向设备回读一次状态并落盘 / 推界面。
func (a *CoreApp) refreshBlackSharkStateAsync(reason string) {
	if a == nil || a.deviceManager == nil {
		return
	}
	if !a.deviceManager.IsBlackSharkProfileActive() {
		return
	}
	if !a.blackSharkStateRefreshInFlight.CompareAndSwap(false, true) {
		return
	}
	a.blackSharkStateRefreshing.Store(true)
	a.safeGo("refreshBlackSharkStateAsync", func() {
		defer a.blackSharkStateRefreshInFlight.Store(false)
		defer a.blackSharkStateRefreshing.Store(false)
		info := a.deviceManager.BlackSharkInfo()
		if !info.Available {
			// 链路不通：**保留**缓存不动。把读不到当成"设备就是空状态"会让面板自己把自己清空。
			a.logDebug("黑鲨状态回读未取到（%s），保留缓存", reason)
			return
		}
		// 单条读失败（Known=false）同样不许覆盖已知值：BLE 上 0xC1 的读回经常超时
		// （写 ACK 已到、读回 2s 内不应答），若把"没读到"当成"关着"落盘，
		// 界面就会显示成与实机相反的状态。
		previous, _, cached := a.blackSharkStateCached()
		switches := info.Switches
		if cached {
			switches = mergeBlackSharkSwitchStatesKnown(previous.Switches, switches)
		}
		gears := info.Gears
		changed := a.rememberBlackSharkStateCache(types.BlackSharkStateSnapshot{
			Switches: switches,
			Gears:    gears,
		})
		info.Switches = switches
		a.publishBlackSharkSnapshot(info, nil, changed, reason)
	})
}

// mergeBlackSharkSwitchStatesKnown 用上次缓存补上"这次没读到"的开关字段。
//
// 判据只看 fresh 的 *Known：fresh 声称读到就以 fresh 为准（值变了也认），否则保留 previous。
// 为什么不能直接把 fresh 落盘：`BlackSharkSwitchStates` 在读失败时给出的是
// `值=false + Known=false`，与"设备确实关着"共用同一份结构；直接落盘等于把读失败
// 记成设备事实，这在 BLE 上每天都会发生（见 setBlackSharkSwitchLocked 的 ACK/读回分离）。
func mergeBlackSharkSwitchStatesKnown(previous, fresh types.BlackSharkSwitchStates) types.BlackSharkSwitchStates {
	out := fresh
	if !fresh.LightingKnown && previous.LightingKnown {
		out.LightingEnabled = previous.LightingEnabled
		out.LightingKnown = true
	}
	if !fresh.LcdScreenKnown && previous.LcdScreenKnown {
		out.LcdScreenEnabled = previous.LcdScreenEnabled
		out.LcdScreenKnown = true
	}
	if !fresh.CoolingSourceKnown && previous.CoolingSourceKnown {
		out.CoolingSource = previous.CoolingSource
		out.CoolingSourceKnown = true
	}
	if !fresh.OnOffVectorKnown && previous.OnOffVectorKnown {
		out.SmartStartStop = previous.SmartStartStop
		out.PowerOnSelfStart = previous.PowerOnSelfStart
		out.OnOffVectorKnown = true
	}
	return out
}

// rememberBlackSharkCoolingSource 把"已回读确认的参照源"记进状态缓存（零额外设备 IO）。
//
// 必须记：参照源会被监控循环改写（用户选 cpu/gpu、或 max 按温度切换），那条路不经过开关页，
// 不记的话缓存会停在旧值 —— 界面显示 GPU 而设备在按 CPU，要点一次刷新才纠正。
// 落盘放后台协程：本函数会被监控循环按拍调到，不能让它等磁盘写。
func (a *CoreApp) rememberBlackSharkCoolingSource(source uint8) {
	if a == nil {
		return
	}
	a.safeGo("rememberBlackSharkCoolingSource", func() {
		a.applyVerifiedBlackSharkSwitch(func(states *types.BlackSharkSwitchStates) {
			states.CoolingSource = source
			states.CoolingSourceKnown = true
		})
	})
}

// applyVerifiedBlackSharkSwitch 把"刚经设备回读确认的一项开关"记进 config 缓存，并推给界面。
func (a *CoreApp) applyVerifiedBlackSharkSwitch(apply func(*types.BlackSharkSwitchStates)) bool {
	if a == nil || apply == nil {
		return false
	}
	snapshot, _, ok := a.blackSharkStateCached()
	if !ok {
		return false
	}
	before := snapshot.Switches
	apply(&snapshot.Switches)
	if snapshot.Switches == before {
		return true // 值没变：不写盘、不推事件
	}
	a.rememberBlackSharkStateCache(snapshot)
	a.publishBlackSharkSwitchState(snapshot.Switches)
	return true
}

// publishBlackSharkSwitchState 把权威开关状态推给界面（写入确认与写入失败回退共用）。
func (a *CoreApp) publishBlackSharkSwitchState(switches types.BlackSharkSwitchStates) {
	if a == nil || a.deviceManager == nil {
		return
	}
	snapshot, _, ok := a.blackSharkStateCached()
	if !ok {
		return
	}
	snapshot.Switches = switches
	info := a.deviceManager.BlackSharkInfoFromSnapshot(snapshot)
	info.Available = true
	info.Firmware = a.GetBlackSharkFirmwareStatus()
	a.publishBlackSharkSnapshot(info, nil, true, "switch-state")
}

func (a *CoreApp) afterBlackSharkSwitchWrite(ok bool, apply func(*types.BlackSharkSwitchStates)) {
	if a == nil || a.deviceManager == nil {
		return
	}
	if ok {
		a.applyVerifiedBlackSharkSwitch(apply)
		return
	}
	// 失败：立刻用缓存里那份把界面拨回去（不等任何回读）。
	snapshot, _, cached := a.blackSharkStateCached()
	if !cached {
		return
	}
	a.publishBlackSharkSwitchState(snapshot.Switches)
}

// blackSharkInitialConfig 是"连接后一次性取到设备侧初始化配置"。
// 返回灯效快照与"是否取到了可用配置"。
func (a *CoreApp) blackSharkInitialConfig() (types.BlackSharkRgbLighting, bool) {
	if a == nil || a.deviceManager == nil || !a.deviceManager.IsBlackSharkProfileActive() {
		return types.BlackSharkRgbLighting{}, false
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
	lighting := a.blackSharkRgbLighting()
	if len(lighting.Modes) > 0 {
		a.logInfo("连接后已读取灯效：%d 个模式（已写入本机缓存）", len(lighting.Modes))
	} else {
		a.logInfo("连接后读取灯效失败，面板将回落到上次读到的那份缓存")
	}

	// 设备版本是本地读，不受厂商清单那道节流约束：连上就重读，否则换了设备或设备升过级，
	// 面板仍显示上一次检查时的版本。
	if status, ok := a.deviceManager.RefreshBlackSharkDeviceVersion(); ok {
		a.logInfo("连接后重读设备固件版本: %s", status.CurrentVersion)
	} else {
		a.logInfo("连接后重读设备固件版本失败")
	}
	return lighting, true
}

// publishBlackSharkSnapshot 把设备侧现状推给界面。
//
// 载荷必须带全，不让界面回读缓存（缓存是异步落盘的，事件到达时回读会拿到旧值）—— 事件是唯一能把
// 新值送到界面的通道。authoritatively = 这就是设备现在的样子，界面可直接覆盖本地乐观状态；
// lighting 传 nil 表示这次不涉及灯效，界面应保留手上那份。
func (a *CoreApp) publishBlackSharkSnapshot(info types.BlackSharkInfo, lighting *types.BlackSharkRgbLighting, authoritatively bool, reason string) {
	if a == nil || a.ipcServer == nil {
		return
	}
	// 来源不同（设备层直出的 / config 缓存组的）都有这一栏缺失的风险：统一在这里补，
	// 否则界面会拿一份 hostVariableSpeed=false 的推送把用户刚打开的开关覆盖回去。
	info.HostVariableSpeed = a.blackSharkHostControlsSpeed()
	a.ipcServer.BroadcastEvent(ipc.EventBlackSharkConnectionSnapshot, map[string]any{
		"rgb":           lighting,
		"info":          info,
		"authoritative": authoritatively,
		"reason":        reason,
	})
}

// refreshBlackSharkConnectedSnapshot 连接后取一次"设备侧现状"：
// 初始化配置（设备本地，见 blackSharkInitialConfig）→ 推界面 →（按节流）厂商清单检查 → 再推一次。
//
// 顺序是刻意的：设备侧本地真值一拿到就先推（面板只在连上的瞬间读过一次缓存，而那次读发生在快照
// 落盘之前），厂商清单要走网络、不能拖住前面那次推送。
func (a *CoreApp) refreshBlackSharkConnectedSnapshot() {
	lighting, ok := a.blackSharkInitialConfig()
	if !ok {
		return
	}
	// 连接这一刻是唯一"必须真读设备"的时机（缓存可能属于上一次连接、甚至另一台设备）；
	// 之后进页面都走 config 优先。所以这里推的是**实读**结果，且标记为权威。
	info := a.blackSharkInfoCached()
	a.publishBlackSharkSnapshot(info, &lighting, true, "connected")

	if a.blackSharkFirmwareCheckDue() {
		status := a.CheckBlackSharkFirmwareUpdate()
		if strings.TrimSpace(status.Error) != "" {
			a.logInfo("连接后固件更新检查未完成: %s", status.Error)
		} else {
			a.logInfo("连接后固件更新检查完成：当前 %s / 最新 %s（可升级=%v）",
				status.CurrentVersion, status.LatestVersion, status.UpdateAvailable)
			// 清单字段（最新版本 / 可升级）到这一步才定下来，再推一次让面板跟上。
			a.publishBlackSharkSnapshot(info, &lighting, true, "connected-firmware")
		}
	}
	if _, _, cached := a.blackSharkStateCached(); !cached {
		a.refreshBlackSharkStateAsync("connected-cold")
	}
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
// 主机侧调速下转速由主机逐拍下发（形状只在主机侧），保存只改本机插值依据 ⇒ 不写设备，
// 下一拍按新曲线下发新值。
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
		if a.blackSharkHostControlsSpeed() {
			// 主机侧调速：新形状只在本机参与插值，设备侧的值由下一拍按新曲线下发。
			a.forceNextAutoTarget.Store(true)
			a.logInfo("曲线方案 %s 已保存；当前由 FanControl 控制转速，下一拍按新曲线下发", profileID)
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
	// 措辞对两种实现方都成立：设备侧跟随下这行就是"设备已切到该档"，主机侧调速下设备档位不参与
	// （上游形态固定写 gear=1），切的是本机插值所用的方案。
	a.logInfo("曲线方案已切换：生效档位 %d", gear)
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

// blackSharkControlTempMaxAge 限定控温基准温度的有效期：智能控温停摆（自动控制关闭、
// 遥测失效、图传把采样循环饿住）超过该时长后，0x07 回落推原始采样，不喂过期值。
const blackSharkControlTempMaxAge = 3 * time.Second

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
// 都读它，判定只有一处（bulkDeviceOpEndUnix），不再各自维护。

// blackSharkImageTransferBudget 取当前通路的图传总预算。
func (a *CoreApp) blackSharkBulkOpInFlight() bool {
	if a == nil {
		return false
	}
	a.blackSharkImageTransferMu.Lock()
	defer a.blackSharkImageTransferMu.Unlock()
	return len(a.blackSharkImageTransfers) > 0
}

func (a *CoreApp) blackSharkImageTransferBudget() (time.Duration, string) {
	transport := types.DeviceTransportUSB
	if a != nil && a.deviceManager != nil {
		if t := a.deviceManager.BlackSharkTransport(); t != "" {
			transport = t
		}
	}
	return types.BlackSharkImageTransferBudget(transport), transport
}

// beginBlackSharkImageTransfer 登记一次图传，返回可被取消的 ctx 与必须 defer 的 done。
// 已有传输在飞时直接报错（一次只允许一个独占任务，避免两条帧流互相踩）。
func (a *CoreApp) beginBlackSharkImageTransfer(parent context.Context) (context.Context, func(), error) {
	if a == nil {
		return nil, func() {}, fmt.Errorf("核心服务未就绪")
	}
	if parent == nil {
		parent = context.Background()
	}
	budget, _ := a.blackSharkImageTransferBudget()
	ctx, cancel := context.WithTimeout(parent, budget)

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
	for _, cancel := range a.blackSharkImageTransfers {
		cancels = append(cancels, cancel)
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

// TransferDeviceImage sends a fixed-size BRB02 RGB565 canvas over the active transport.
//
// 图传两条通路都支持，所以这里只按档案族判定，不要求具体是 USB 档案；走哪条通路由设备层按
// 已连接的通路选择。能力位 SupportsScreenImageTransfer 是第二道门。
func (a *CoreApp) TransferDeviceImage(params ipc.TransferDeviceImageParams) error {
	if a.deviceManager == nil || !a.deviceManager.IsConnected() {
		return fmt.Errorf("黑鲨设备未连接")
	}
	profile := a.deviceManager.ActiveProfile()
	transport := types.NormalizeDeviceTransport(profile.Transport)
	if !types.IsBlackSharkDeviceProfileID(profile.ID) ||
		(transport != types.DeviceTransportUSB && transport != types.DeviceTransportBLE) {
		return fmt.Errorf("当前黑鲨连接方式不支持屏幕图片传输")
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
//
// 温度条目优先取监控循环发布的控温基准温度（`publishBlackSharkControlTemp`）：
// 设备跟随模式下设备按 0x07 里的温度跑自己的曲线，喂原始采样会让设备对主机已经
// 平滑/滤掉的尖峰做出反应，两边输入不一致。max 来源时设备参照源是动态解析出的
// 那一路（blackSharkSourceWritten），平滑后的 max 喂给那一路。
func (a *CoreApp) blackSharkHostInfoPushEntries() []deviceproto.BlackSharkSystemInfoEntry {
	if a == nil || a.deviceManager == nil {
		return nil
	}
	a.mutex.RLock()
	temp := a.currentTemp
	a.mutex.RUnlock()
	if v, src, ok := a.blackSharkSmoothedControlTemp(); ok {
		switch src {
		case 0:
			temp.CPUTemp = v
		case 1:
			temp.GPUTemp = v
		default:
			switch a.blackSharkSourceWritten.Load() {
			case 0:
				temp.CPUTemp = v
			case 1:
				temp.GPUTemp = v
			}
		}
	}
	return blackSharkSysInfoPick(
		a.collectBlackSharkSysInfo(temp, time.Now()), bytesToInts(deviceproto.BlackSharkSysInfoPushItems))
}

// publishBlackSharkControlTemp 由监控循环每拍调用，发布智能控温实际使用的基准温度。
func (a *CoreApp) publishBlackSharkControlTemp(controlTemp int, source string) {
	if a == nil {
		return
	}
	code := int32(2)
	switch types.NormalizeTempSource(source) {
	case types.TempSourceCPU:
		code = 0
	case types.TempSourceGPU:
		code = 1
	}
	a.blackSharkControlTempValue.Store(int32(controlTemp))
	a.blackSharkControlTempSource.Store(code)
	a.blackSharkControlTempAt.Store(time.Now().UnixNano())
}

// blackSharkSmoothedControlTemp 取最近发布的控温基准温度；未发布过、已过期或值非法
// 时返回 false，调用方回落推原始采样。
func (a *CoreApp) blackSharkSmoothedControlTemp() (int, int32, bool) {
	if a == nil {
		return 0, 0, false
	}
	at := a.blackSharkControlTempAt.Load()
	if at == 0 || time.Since(time.Unix(0, at)) > blackSharkControlTempMaxAge {
		return 0, 0, false
	}
	v := a.blackSharkControlTempValue.Load()
	if v <= 0 {
		return 0, 0, false
	}
	return int(v), a.blackSharkControlTempSource.Load(), true
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
	// audioNextLog 是"音频同步采样诊断"下一次可打印的时刻（节流；仅在该灯效生效期间用）。
	// 需要它的原因：所有"灯效不动"的成因在界面上完全一样，只有把主频/RMS/块数
	// 记进日志才能区分"真的没声音"与"采集没出数"。见 logAudioDiagnostics。
	audioNextLog         time.Time
	audioSilent          bool
	audioPausedForBulkOp bool

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
		// 设备独占任务（屏保图传）进行中 ⇒ 停推 0x15。
		if a.blackSharkBulkOpInFlight() {
			fx.setAudioPausedForBulkOp(a, true)
			break
		}
		fx.setAudioPausedForBulkOp(a, false)
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

	// 先落一行诊断（节流 ~5 s），再下发。
	fx.logAudioDiagnostics(a, s, freq, level)
	fx.logAudioSilenceTransition(a, level)

	// 必须每一拍都发，包括静音那一拍（level == 0）。
	if a.deviceManager.IssueAudioSpectrumLevel(level) {
		fx.mu.Lock()
		fx.audioPushes++
		fx.mu.Unlock()
	}
}

// blackSharkAudioDiagInterval 是"音频同步采样诊断"的打印节流间隔。
const blackSharkAudioDiagInterval = 5 * time.Second

// logAudioDiagnostics 节流记一行音频采样现状。
//
// "灯效不跟着声音动"有三种成因，界面上长得一样：灯效根本没生效 / 采集起来但真的没声音 /
// 采集链没出数。原来的日志只有一句"已开始采集系统播放声音"，三种都看不出来。
//
// 判读方法：
//
//	采到块=0          ⇒ ③ 引擎压根没交数据（端点/格式问题）
//	采到块>0 且有声=0 ⇒ ② 真的没在放声音（或音量极低），不是故障
//	有声音但已推=0    ⇒ 采集有数却没下发，问题在下发路径
func (fx *blackSharkHostFx) logAudioDiagnostics(a *CoreApp, s *bsaudio.Sampler, freq float64, level byte) {
	now := time.Now()
	fx.mu.Lock()
	if now.Before(fx.audioNextLog) {
		fx.mu.Unlock()
		return
	}
	fx.audioNextLog = now.Add(blackSharkAudioDiagInterval)
	pushes := fx.audioPushes
	fx.mu.Unlock()

	rms, blocks, active := s.Stats()
	a.logInfo("黑鲨「音频同步」采样：主频=%.1f Hz 档位=%d 最近RMS=%.1f 采到块=%d（其中有声 %d）已推 %d 帧",
		freq, level, rms, blocks, active, pushes)
}

func (fx *blackSharkHostFx) logAudioSilenceTransition(a *CoreApp, level byte) {
	silent := level == deviceproto.BlackSharkAudioLevelSilent

	fx.mu.Lock()
	changed := fx.audioSilent != silent
	fx.audioSilent = silent
	pushes := fx.audioPushes
	fx.mu.Unlock()
	if !changed {
		return
	}
	if silent {
		a.logInfo("黑鲨「音频同步」：采到静音（已推 %d 帧）—— 仍会照发档位 0，让设备回落", pushes)
		return
	}
	a.logInfo("黑鲨「音频同步」：采到声音，档位=%d（已推 %d 帧）", level, pushes)
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
		// 平台不支持（非 Windows）不是瞬态故障：不再安排重试，否则会无限刷错误日志。
		if !errors.Is(err, bsaudio.ErrUnsupported) {
			fx.audioRetryAt = time.Now().Add(blackSharkHostFxRetryInterval)
		}
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

// setAudioPausedForBulkOp 记录/清除"因独占任务停推 0x15"的状态，并在**跳变**时记一条日志。
//
// 为什么要日志：这条抑制的判据在 coreapp 侧（图传登记表），而推送循环在 host-fx 里，
// 出问题时必须能从日志一眼看出"它当时到底有没有生效"。
func (fx *blackSharkHostFx) setAudioPausedForBulkOp(a *CoreApp, paused bool) {
	fx.mu.Lock()
	changed := fx.audioPausedForBulkOp != paused
	fx.audioPausedForBulkOp = paused
	fx.mu.Unlock()
	if !changed {
		return
	}
	if paused {
		a.logInfo("黑鲨「音频同步」：设备独占任务（图传）进行中 ⇒ 暂停 0x15 推送（D4：二者不并发）")
		return
	}
	a.logInfo("黑鲨「音频同步」：独占任务结束 ⇒ 恢复 0x15 推送")
}

// stopAudio 停掉音频采集（没在跑就是空操作）。
func (fx *blackSharkHostFx) stopAudio(a *CoreApp) {
	fx.mu.Lock()
	s := fx.audio
	fx.audio = nil
	fx.audioFreq, fx.audioLevel = 0, 0
	fx.audioNextLog = time.Time{}
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
		// 平台不支持（非 Windows）不是瞬态故障：不再安排重试，否则会无限刷错误日志。
		if !errors.Is(err, bspress.ErrUnsupported) {
			fx.pressRetryAt = time.Now().Add(blackSharkHostFxRetryInterval)
		}
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
// **自给自足**：只记本工具上传成功过屏的图，不读、也不依赖官方装备箱的任何目录。
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
// 目录判定保证**只能删/传本机缓存目录里的东西**，不给任意路径开口子；长度判定挡掉目录里
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

	// 这里**不提前 break**：设备当前那张可能比最新两张更旧，提前退出就找不到它了。
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
		//     多份一模一样的字节（本机样本：46 个文件只有 11 种画布）。不去重就会把两格填成同一张图。
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
// 删的是本工具自己的缓存文件，所以只允许"目录内 + `.bin` + 121552 字节"，其余一律拒绝。
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
// 图传只有一条实现：设备层的 SendBlackSharkImageWithProgress（USB 走 libusb，BLE 走原始帧流）。
func (a *CoreApp) uploadScreenPayload(payload []byte) (*ipc.ScreenUploadResult, error) {
	if a.deviceManager == nil {
		return nil, fmt.Errorf("设备管理器未就绪")
	}
	profile := a.deviceManager.ActiveProfile()
	transport := types.NormalizeDeviceTransport(profile.Transport)
	if !types.IsBlackSharkDeviceProfileID(profile.ID) ||
		(transport != types.DeviceTransportUSB && transport != types.DeviceTransportBLE) {
		return nil, fmt.Errorf("当前黑鲨连接方式不支持屏保图像传输")
	}
	// 登记成一次设备独占任务（上游那套形状）：重复传输会被挡住，断开/挂起时能被取消；
	// 结束时刻由登记表留痕，监控循环据此把这段停摆归因到本机（见 selfInflictedMonitorStall）。
	ctx, done, err := a.beginBlackSharkImageTransfer(a.ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	budget, transport := a.blackSharkImageTransferBudget()
	if err := a.deviceManager.SendBlackSharkImageWithProgress(ctx, payload, a.broadcastScreenImageTransferProgress); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf(
				"屏保图像传输超过时间预算（%s 通路 %v，整轮 %d 帧）：%w。"+
					"设备侧没有拒收也没有超时，是整体耗时超预算 —— 可重试，或改用 USB 连接",
				transport, budget, deviceproto.BlackSharkScreenImageFrames+2, err)
		}
		return nil, err
	}

	reports := (len(payload) + deviceproto.BlackSharkImageFramePayload - 1) /
		deviceproto.BlackSharkImageFramePayload
	short := len(payload) - (reports-1)*deviceproto.BlackSharkImageFramePayload
	// 回读一致性检查（必要不充分，见 verifyBlackSharkScreenImage 的说明）。
	// 权威判据是 SendBlackSharkImageWithProgress 内部的 0xC6 闭环 —— 走到这里说明每个数据块
	// 都已被设备显式收下；这一步只负责抓住"设备手上的元数据不是本次这张图"这种反常。
	info, verified := a.verifyBlackSharkScreenImage(payload)
	if !verified {
		// 不谎报成功：0xC6 说数据被收了，但设备持有的元数据不是本次这张图 ⇒ 按失败上报。
		return nil, fmt.Errorf(
			"屏保图像回读校验未通过：设备持有 size=%d crc=0x%04X，本机写入 size=%d crc=0x%04X"+
				"（0xC6 闭环已确认各块被接收，但设备元数据未反映本次写入）",
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
// 这是必要不充分的一致性校验，不是成功判据：0xC5 只是把设备手上那份（主机经 0xC4 写进去的）元数据
// 原样回显，所以"一致"不构成成功证据。权威判据是 SendBlackSharkImageWithProgress 内部的 0xC6 闭环；
// 这条校验的价值在反向 —— 设备若没接收或没落盘，它手上的元数据就不会是本次这张图。
//
// 判据只用大小 + CRC：`0xC5` 的回显不构成成功证据（见上），再比一个时间戳字段也不会改变这一点 ——
// 回显里的时间戳是设备手上那份的字段，不能当成"本次这张图已落盘"的证明。
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
