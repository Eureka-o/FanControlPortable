// Wails 绑定：黑鲨 / 情景 / 屏幕三组能力。
package guiapp

import (
	"encoding/json"
	"fmt"
	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/ipc"
	"github.com/Eureka-o/FanControlPortable/internal/screenimg"
	"github.com/Eureka-o/FanControlPortable/internal/types"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"os"
	"time"
)

func (a *App) GetBlackSharkInfo() types.BlackSharkInfo {
	resp, err := a.sendRequest(ipc.ReqGetBlackSharkInfo, nil)
	if err != nil {
		return types.BlackSharkInfo{}
	}
	var info types.BlackSharkInfo
	json.Unmarshal(resp.Data, &info)
	return info
}

// CheckBlackSharkFirmwareUpdate 主动检查一次黑鲨固件版本（设备 CMD 0x01 + 官方版本清单）。
func (a *App) CheckBlackSharkFirmwareUpdate() types.BlackSharkFirmwareStatus {
	resp, err := a.sendRequest(ipc.ReqCheckBlackSharkFirmwareUpdate, nil)
	if err != nil {
		return types.BlackSharkFirmwareStatus{UpdateMethod: "official-tool", Error: err.Error()}
	}
	var status types.BlackSharkFirmwareStatus
	json.Unmarshal(resp.Data, &status)
	return status
}

// GetBlackSharkFirmwareStatus 读取已缓存的固件检查结果，一次设备/网络 IO 都不做。
func (a *App) GetBlackSharkFirmwareStatus() types.BlackSharkFirmwareStatus {
	resp, err := a.sendRequest(ipc.ReqGetBlackSharkFirmwareStatus, nil)
	if err != nil {
		return types.BlackSharkFirmwareStatus{UpdateMethod: "official-tool"}
	}
	var status types.BlackSharkFirmwareStatus
	json.Unmarshal(resp.Data, &status)
	return status
}

// SetBlackSharkLightingEnabled 开关黑鲨 RGB 灯效（0x10 / 0x11，开与关都可复现）。
func (a *App) SetBlackSharkLightingEnabled(enabled bool) bool {
	return a.blackSharkBoolRequest(ipc.ReqSetBlackSharkLightingEnabled, ipc.SetBoolParams{Enabled: enabled})
}

// SetBlackSharkLcdScreenEnabled 开关黑鲨 LCD 小屏（0xC0 / 0xC1，开与关都可复现）。
func (a *App) SetBlackSharkLcdScreenEnabled(enabled bool) bool {
	return a.blackSharkBoolRequest(ipc.ReqSetBlackSharkLcdScreenEnabled, ipc.SetBoolParams{Enabled: enabled})
}

// SetBlackSharkOnOffVector 写入「智能启停 + 通电自启」两路开关（0x02）。
// 两路必须一起给：设备侧 0x02 每次写入的是完整两字节向量，没有单路形式。
func (a *App) SetBlackSharkOnOffVector(smartStartStop bool, powerOnSelfStart bool) bool {
	return a.blackSharkBoolRequest(ipc.ReqSetBlackSharkOnOffVector, ipc.BlackSharkOnOffVectorParams{
		SmartStartStop:   smartStartStop,
		PowerOnSelfStart: powerOnSelfStart,
	})
}

// GetBlackSharkRgbLighting 聚合灯效页信息（开关 + 当前模式 + 全部 8 个模式参数）。
// 会触发 1+1+8 次设备查询，只应在用户主动刷新或展开灯效区时调用。
func (a *App) GetBlackSharkRgbLighting() types.BlackSharkRgbLighting {
	resp, err := a.sendRequest(ipc.ReqGetBlackSharkRgbLighting, nil)
	if err != nil {
		return types.BlackSharkRgbLighting{}
	}
	var out types.BlackSharkRgbLighting
	json.Unmarshal(resp.Data, &out)
	return out
}

// SelectBlackSharkRgbMode 切换当前生效的灯效模式（0x14，写入后用 0x13 回读确认）。
func (a *App) SelectBlackSharkRgbMode(index int) bool {
	return a.blackSharkBoolRequest(ipc.ReqSelectBlackSharkRgbMode, ipc.SetIntParams{Value: index})
}

// GetBlackSharkHostEffects 返回主机侧灯效驱动的状态（槽位 6「响应」/ 槽位 7「音频同步」）。
// 纯内存读取（coreapp 侧不发任何设备查询），可以随灯效页一起刷新。
func (a *App) GetBlackSharkHostEffects() types.BlackSharkHostEffects {
	resp, err := a.sendRequest(ipc.ReqGetBlackSharkHostEffects, nil)
	if err != nil {
		return types.BlackSharkHostEffects{}
	}
	var out types.BlackSharkHostEffects
	json.Unmarshal(resp.Data, &out)
	return out
}

// GetBlackSharkManualGearPresets 返回手动挡位面板需要的全部数值：
// 四档 × 三档转速 + RPM 量程 + 步进。
func (a *App) GetBlackSharkManualGearPresets() deviceproto.BlackSharkManualGearPresetsPayload {
	resp, err := a.sendRequest(ipc.ReqGetBlackSharkManualGearPresets, nil)
	if err != nil {
		return deviceproto.BlackSharkManualGearPresetsPayload{}
	}
	var out deviceproto.BlackSharkManualGearPresetsPayload
	json.Unmarshal(resp.Data, &out)
	return out
}

// GetBlackSharkCurveTempRange 返回曲线那 4 个点能被拖到的温度取值域（℃，含两端）。
// 前端据此限制横向拖动；取值域的所有者在 deviceproto。
func (a *App) GetBlackSharkCurveTempRange() types.BlackSharkCurveTempRange {
	resp, err := a.sendRequest(ipc.ReqGetBlackSharkCurveTempRange, nil)
	if err != nil {
		return types.BlackSharkCurveTempRange{}
	}
	var out types.BlackSharkCurveTempRange
	json.Unmarshal(resp.Data, &out)
	return out
}

// SetBlackSharkRgbModeEffects 修改某灯效模式的速度与亮度（0x12，读-改-写 + 回读校验）。
func (a *App) SetBlackSharkRgbModeEffects(index int, speed int, brightness int) bool {
	return a.blackSharkBoolRequest(ipc.ReqSetBlackSharkRgbModeEffects,
		ipc.BlackSharkRgbModeEffectParams{Index: index, Speed: speed, Brightness: brightness})
}

// SetBlackSharkRgbModeColor 设置某灯效模式的颜色（0x12 的 payload[5..7]）。
// 只收色相（0..360）：色相到 RGB 的换算由 deviceproto 里与官方 DLL 逐字节对齐的表完成。
func (a *App) SetBlackSharkRgbModeColor(index int, hue int, staticColor bool) bool {
	return a.blackSharkBoolRequest(ipc.ReqSetBlackSharkRgbModeColor,
		ipc.BlackSharkRgbColorParams{Index: index, Hue: hue, StaticColor: staticColor})
}

// SetBlackSharkRgbColorOption 设置某灯效模式的颜色下拉选项（`0x12` 的 `payload[0]` 高半字节）。
// 这是「颜色模式」的第二维：SetBlackSharkRgbModeColor 改"具体什么颜色"，本方法改用哪一套配色。
func (a *App) SetBlackSharkRgbColorOption(index int, option int) bool {
	return a.blackSharkBoolRequest(ipc.ReqSetBlackSharkRgbColorOption,
		ipc.BlackSharkRgbColorOptionParams{Index: index, Option: option})
}

// BlackSharkLcdDisplayPayload 屏幕显示参数（选 3 项 + 左右顺序 + pos）。
// Items 是本工具保存的选择，Options 是官方可选项的排列顺序。
type BlackSharkLcdDisplayPayload struct {
	Pos     int   `json:"pos"`
	Items   []int `json:"items"`
	Options []int `json:"options"`
	// DefaultItems 是官方「重置屏幕设置」写入的那一组（id 0/1/7 = CPU温度 / GPU温度 / 时间）。
	DefaultItems []int `json:"defaultItems"`
}

// BlackSharkLcdDisplayResult 下发结果。
// Saved 与 Applied 分开报：Saved 是本机配置（确定知道），Applied 是"帧已写出"。
type BlackSharkLcdDisplayResult struct {
	Saved   bool   `json:"saved"`
	Applied bool   `json:"applied"`
	Pos     int    `json:"pos"`
	Items   []int  `json:"items"`
	Error   string `json:"error,omitempty"`
}

// GetBlackSharkLcdDisplay 读取本工具保存的屏幕显示参数，不是设备回读。
func (a *App) GetBlackSharkLcdDisplay() BlackSharkLcdDisplayPayload {
	out := BlackSharkLcdDisplayPayload{
		Items:        append([]int{}, types.DefaultBlackSharkLcdItems()...),
		Options:      deviceproto.BlackSharkLcdSelectableItemIDs(),
		DefaultItems: append([]int{}, types.DefaultBlackSharkLcdItems()...),
	}
	resp, err := a.sendRequest(ipc.ReqGetBlackSharkLcdDisplay, nil)
	if err != nil || resp == nil || !resp.Success {
		return out
	}
	var got BlackSharkLcdDisplayPayload
	if err := json.Unmarshal(resp.Data, &got); err != nil {
		return out
	}
	out.Pos = got.Pos
	if normalized, _ := types.NormalizeBlackSharkLcdItems(got.Items); len(normalized) > 0 {
		out.Items = normalized
	}
	return out
}

// SetBlackSharkLcdDisplay 保存并下发屏幕显示参数（0xC2）：items 是按屏上左右顺序排的项 id。
func (a *App) SetBlackSharkLcdDisplay(pos int, items []int) BlackSharkLcdDisplayResult {
	resp, err := a.sendRequest(ipc.ReqSetBlackSharkLcdDisplay,
		ipc.BlackSharkLcdDisplayParams{Pos: pos, Items: items})
	if err != nil {
		return BlackSharkLcdDisplayResult{Error: err.Error()}
	}
	if resp == nil {
		return BlackSharkLcdDisplayResult{Error: "下发请求无响应"}
	}
	if !resp.Success {
		return BlackSharkLcdDisplayResult{Error: resp.Error}
	}
	var out BlackSharkLcdDisplayResult
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return BlackSharkLcdDisplayResult{Error: "解析下发结果失败: " + err.Error()}
	}
	return out
}

// ResetBlackSharkRgb 官方「灯效页 · 重置」：把 8 个灯效槽位全部恢复为出厂灯效。
func (a *App) ResetBlackSharkRgb() bool {
	return a.blackSharkBoolRequest(ipc.ReqResetBlackSharkRgb, nil)
}

// ResetBlackSharkCooling 官方「散热页 · 重置」：把 4 个档位的冷却配置恢复为出厂值。
//
// 副作用（官方自身行为，界面须如实告知）：重置后设备会停在固定转速形态。
func (a *App) ResetBlackSharkCooling() bool {
	return a.blackSharkBoolRequest(ipc.ReqResetBlackSharkCooling, nil)
}

// blackSharkBoolRequest 发送一个返回 bool 的黑鲨请求。
// 与项目既有写法一致（见 control_api.go 的 SetSmartStartStop）。
func (a *App) blackSharkBoolRequest(reqType ipc.RequestType, params any) bool {
	resp, err := a.sendRequest(reqType, params)
	if err != nil {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// 配置快照：替代「恢复出厂」。

// ConfigSnapshotExportResult 导出结果。Saved=false 且 Error 为空表示用户取消了。
type ConfigSnapshotExportResult struct {
	Saved bool   `json:"saved"`
	Path  string `json:"path,omitempty"`
	Error string `json:"error,omitempty"`
}

// blackSharkConfigSnapshot 读一份设备配置快照。
// 会向设备发起多次查询（8 个灯效块 + 4 条曲线 + 若干开关），只由「导出快照」按需调用；
// 前端不直接调它，故不导出（导出会生成一个 Promise<T|boolean> 的联合返回，徒增调用方负担）。
func (a *App) blackSharkConfigSnapshot() (types.BlackSharkConfigSnapshot, bool) {
	resp, err := a.sendRequest(ipc.ReqGetBlackSharkConfigSnapshot, nil)
	if err != nil || resp == nil || !resp.Success {
		return types.BlackSharkConfigSnapshot{}, false
	}
	var snap types.BlackSharkConfigSnapshot
	if err := json.Unmarshal(resp.Data, &snap); err != nil {
		return types.BlackSharkConfigSnapshot{}, false
	}
	return snap, true
}

// ExportBlackSharkConfigSnapshot 把当前配置快照导出成一个 JSON 文件（用户选路径）。
func (a *App) ExportBlackSharkConfigSnapshot() ConfigSnapshotExportResult {
	snap, ok := a.blackSharkConfigSnapshot()
	if !ok {
		return ConfigSnapshotExportResult{Error: "设备未就绪，读不到配置快照"}
	}
	if a.ctx == nil {
		return ConfigSnapshotExportResult{Error: "窗口未就绪"}
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出黑鲨配置快照",
		DefaultFilename: "blackshark-config-" + time.Now().Format("20060102-150405") + ".json",
		Filters:         []wailsruntime.FileFilter{{DisplayName: "JSON (*.json)", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return ConfigSnapshotExportResult{} // 用户取消，不算错误
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return ConfigSnapshotExportResult{Error: "序列化失败: " + err.Error()}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return ConfigSnapshotExportResult{Error: "写文件失败: " + err.Error()}
	}
	return ConfigSnapshotExportResult{Saved: true, Path: path}
}

// RestoreBlackSharkConfigFromFile 选一个快照文件并写回设备。
// 写设备前先弹确认框：这是一次会改动多项配置的操作，不能点一下就执行。
func (a *App) RestoreBlackSharkConfigFromFile() types.BlackSharkConfigRestoreResult {
	if a.ctx == nil {
		return types.BlackSharkConfigRestoreResult{Issues: []string{"窗口未就绪"}}
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   "选择黑鲨配置快照",
		Filters: []wailsruntime.FileFilter{{DisplayName: "JSON (*.json)", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return types.BlackSharkConfigRestoreResult{}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return types.BlackSharkConfigRestoreResult{Issues: []string{"读文件失败: " + err.Error()}}
	}
	var snap types.BlackSharkConfigSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return types.BlackSharkConfigRestoreResult{Issues: []string{"解析失败: " + err.Error()}}
	}
	if snap.Version != 1 {
		return types.BlackSharkConfigRestoreResult{
			Issues: []string{fmt.Sprintf("不是本工具导出的快照（version=%d，期望 1）", snap.Version)},
		}
	}

	choice, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "从快照还原黑鲨配置",
		Message:       fmt.Sprintf("将写回：%d 个灯效模式、%d 条档位曲线、以及开关/来源设置。\n该操作会改动设备上的多项配置，确定继续吗？", len(snap.RgbModes), len(snap.GearCurves)),
		Buttons:       []string{"取消", "还原"},
		DefaultButton: "取消",
	})
	if err != nil || choice != "还原" {
		return types.BlackSharkConfigRestoreResult{}
	}

	resp, err := a.sendRequest(ipc.ReqRestoreBlackSharkConfig, ipc.BlackSharkRestoreConfigParams{Snapshot: snap})
	if err != nil || resp == nil || !resp.Success {
		msg := "还原请求失败"
		if resp != nil && resp.Error != "" {
			msg = resp.Error
		}
		return types.BlackSharkConfigRestoreResult{Issues: []string{msg}}
	}
	var out types.BlackSharkConfigRestoreResult
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return types.BlackSharkConfigRestoreResult{Issues: []string{"解析还原结果失败: " + err.Error()}}
	}
	if out.Restored == nil {
		out.Restored = []string{}
	}
	if out.Issues == nil {
		out.Issues = []string{}
	}
	return out
}

// 情景规则。
type SceneRulesPayload struct {
	Rules        []SceneRulePayload `json:"rules"`
	BaselineGear int                `json:"baselineGear"`
}

// SceneRulePayload 单条情景规则（与 types.SceneRule 的 JSON 形态一致）。
// Name 必须原样透传：前端有改名输入框，后端 types.SceneRule.Name 也参与匹配，
// 在桥接层丢掉会让"改名已保存"变成假象。
type SceneRulePayload struct {
	Name    string `json:"name,omitempty"`
	Enabled bool   `json:"enabled"`
	Match   string `json:"match"`
	Gear    int    `json:"gear"`
	RgbMode int    `json:"rgbMode"`
}

// SceneRulesResult 保存结果：Saved 表示已落盘，Issues 是逐字段的校验提示。
type SceneRulesResult struct {
	Saved  bool     `json:"saved"`
	Issues []string `json:"issues"`
}

// GetSceneRules 读取情景规则与基准档位。
func (a *App) GetSceneRules() SceneRulesPayload {
	resp, err := a.sendRequest(ipc.ReqGetSceneRules, nil)
	if err != nil {
		return SceneRulesPayload{Rules: []SceneRulePayload{}}
	}
	var out SceneRulesPayload
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return SceneRulesPayload{Rules: []SceneRulePayload{}}
	}
	if out.Rules == nil {
		out.Rules = []SceneRulePayload{}
	}
	return out
}

// SetSceneRules 保存情景规则与基准档位。
func (a *App) SetSceneRules(payload SceneRulesPayload) SceneRulesResult {
	resp, err := a.sendRequest(ipc.ReqSetSceneRules, payload)
	if err != nil {
		return SceneRulesResult{Issues: []string{err.Error()}}
	}
	var out SceneRulesResult
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return SceneRulesResult{Issues: []string{err.Error()}}
	}
	return out
}

// ListSceneProcesses 返回可选作触发者的进程名（去重升序）。
// 平台不支持时返回空列表，界面应据此隐藏「选择进程」入口。
func (a *App) ListSceneProcesses() []string {
	resp, err := a.sendRequest(ipc.ReqListSceneProcesses, nil)
	if err != nil {
		return []string{}
	}
	var out struct {
		Processes []string `json:"processes"`
	}
	if err := json.Unmarshal(resp.Data, &out); err != nil || out.Processes == nil {
		return []string{}
	}
	return out.Processes
}

// 屏幕 / 屏保图像。

// screenUploadTimeout 是"上传屏保图像"这条 IPC 调用的超时。
// 上传的恢复策略是整包重传，重传的最坏耗时上界决定了需要放宽到 200 秒。
const screenUploadTimeout = 200 * time.Second

// screenReadImageTimeout 是"从设备读回屏图"的 IPC 超时。
// 依据：读图 = `0xC7` + 2096 次 `0xC8`（每次一问一答，约 20~40 ms 一帧）。
const screenReadImageTimeout = 180 * time.Second

// ScreenImageReadPayload 从设备读回屏图的结果（data URL 供界面直接显示）。
type ScreenImageReadPayload struct {
	Ok      bool   `json:"ok"`
	DataURL string `json:"dataUrl,omitempty"`
	Bytes   int    `json:"bytes"`
	CRC     uint16 `json:"crc"`
	Error   string `json:"error,omitempty"`
	// FromCache / CachedAtUnix 标识这是上次读到的那一份，不是刚刚读的。
	FromCache    bool  `json:"fromCache,omitempty"`
	CachedAtUnix int64 `json:"cachedAtUnix,omitempty"`
}

// GetBlackSharkRgbLightingCached 只取本机缓存的灯效状态（上次读到的那份），零设备 IO。
// 面板打开时先显示它，免得"必须先点一次读取灯效"。
func (a *App) GetBlackSharkRgbLightingCached() types.BlackSharkRgbLighting {
	resp, err := a.sendRequest(ipc.ReqGetBlackSharkRgbCache, nil)
	if err != nil || resp == nil || !resp.Success {
		return types.BlackSharkRgbLighting{ColorSettable: true}
	}
	var out types.BlackSharkRgbLighting
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return types.BlackSharkRgbLighting{ColorSettable: true}
	}
	return out
}

// GetScreenImageCached 只取"上次读到的那份"屏图，零设备 IO。
func (a *App) GetScreenImageCached() ScreenImageReadPayload {
	resp, err := a.sendRequest(ipc.ReqReadScreenImageCache, nil)
	if err != nil || resp == nil || !resp.Success {
		return ScreenImageReadPayload{Error: "读取屏图缓存失败"}
	}
	var out ScreenImageReadPayload
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return ScreenImageReadPayload{Error: "解析屏图缓存失败: " + err.Error()}
	}
	return out
}

// ReadScreenImageFromDevice 从设备读回当前屏上那张图（`0xC7` + 反复 `0xC8`，约 2096 帧）。
// 这是"自定义图能不能显示"的正解：设备本来就能读回，不必再靠本机缓存。
func (a *App) ReadScreenImageFromDevice() ScreenImageReadPayload {
	resp, err := a.sendRequestWithTimeout(ipc.ReqReadScreenImage, nil, screenReadImageTimeout)
	if err != nil || resp == nil || !resp.Success {
		return ScreenImageReadPayload{Error: "读回请求失败（超时或设备无响应）"}
	}
	var out ScreenImageReadPayload
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return ScreenImageReadPayload{Error: "解析读回结果失败: " + err.Error()}
	}
	return out
}

// ScreenPresetPayload 一张内置精选。
type ScreenPresetPayload struct {
	OfficialPosition int    `json:"officialPosition"`
	AssetIndex       int    `json:"assetIndex"`
	Name             string `json:"name"`
}

// ScreenPresetListPayload 内置精选列表（按官方 UI 的顺序）。
type ScreenPresetListPayload struct {
	Presets []ScreenPresetPayload `json:"presets"`
}

// ScreenImageInfoPayload 设备当前保存的屏保图像信息。
type ScreenImageInfoPayload struct {
	HasImage  bool   `json:"hasImage"`
	Timestamp uint32 `json:"timestamp"`
	Size      uint32 `json:"size"`
	CRC       uint16 `json:"crc"`

	// 下面是认图的结果：协议没有读图命令，所以拿不到像素，
	// 但把回读的 CRC 与十张内置预设比对就能说清"当前用的是哪一张"。
	PresetMatched          bool   `json:"presetMatched"`
	PresetOfficialPosition int    `json:"presetOfficialPosition,omitempty"` // 1..10，给缩略图接口用
	PresetName             string `json:"presetName,omitempty"`
}

// ScreenUploadResultPayload 一次上传的结果。
type ScreenUploadResultPayload struct {
	Success   bool   `json:"success"`
	Error     string `json:"error"`
	Reports   int    `json:"reports"`
	Short     int    `json:"short"`
	Timestamp uint32 `json:"timestamp"`
	Size      uint32 `json:"size"`
	CRC       uint16 `json:"crc"`
	Verified  bool   `json:"verified"`
}

func (a *App) ListScreenPresets() ScreenPresetListPayload {
	resp, err := a.sendRequest(ipc.ReqListScreenPresets, nil)
	if err != nil || resp == nil || !resp.Success {
		return ScreenPresetListPayload{Presets: []ScreenPresetPayload{}}
	}
	var out ScreenPresetListPayload
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return ScreenPresetListPayload{Presets: []ScreenPresetPayload{}}
	}
	if out.Presets == nil {
		out.Presets = []ScreenPresetPayload{}
	}
	return out
}

// ScreenPresetThumbnail 按需取第 position 张的缩略图 data URL（失败返回空串）。
func (a *App) ScreenPresetThumbnail(officialPosition int) string {
	resp, err := a.sendRequest(ipc.ReqScreenPresetThumb,
		ipc.ScreenPresetParams{OfficialPosition: officialPosition})
	if err != nil || resp == nil || !resp.Success {
		return ""
	}
	var out struct {
		DataURL string `json:"dataUrl"`
	}
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return ""
	}
	return out.DataURL
}

// UploadScreenPreset 上传第 position 张内置精选（官方顺序，1..10）。
func (a *App) UploadScreenPreset(officialPosition int) ScreenUploadResultPayload {
	return a.uploadScreen(ipc.ReqUploadScreenPreset,
		ipc.ScreenPresetParams{OfficialPosition: officialPosition})
}

// CancelScreenImageTransfer 取消进行中的屏幕图片传输（图传弹窗上的"取消"）。
// 返回 true 只表示请求已发出：上传调用本身会随即以失败返回。
func (a *App) CancelScreenImageTransfer() bool {
	resp, err := a.sendRequest(ipc.ReqCancelScreenImageTransfer, nil)
	return err == nil && resp != nil && resp.Success
}

// ScreenCropPreviewPayload 手动裁剪的预览图与尺寸信息。
type ScreenCropPreviewPayload struct {
	DataURL   string `json:"dataUrl"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	RawWidth  int    `json:"rawWidth"`
	RawHeight int    `json:"rawHeight"`
	SlackX    int    `json:"slackX"`
	SlackY    int    `json:"slackY"`
}

// UploadScreenImageFile 上传一张本地图片（绝对路径）。
func (a *App) UploadScreenImageFile(path string, zoom, offsetX, offsetY int) ScreenUploadResultPayload {
	return a.uploadScreen(ipc.ReqUploadScreenImage, ipc.ScreenImagePathParams{
		Path: path, Zoom: zoom, OffsetX: offsetX, OffsetY: offsetY,
	})
}

// ScreenHistoryItemPayload 一张历史画布（本机缓存 `<安装目录>/screen-images/` 里的 `*.bin`）。
type ScreenHistoryItemPayload struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	UnixTime int64  `json:"unixTime"`
	Modified string `json:"modified"`
	Size     int64  `json:"size"`
	Thumb    string `json:"thumb,omitempty"`
}

// ScreenHistoryListPayload 历史画布列表；Error 非空表示这一轮没读到（目录不存在 / 没权限）。
type ScreenHistoryListPayload struct {
	Dir   string                     `json:"dir"`
	Items []ScreenHistoryItemPayload `json:"items"`
	// Current 是"设备当前正在用的那张"在本机缓存里的对应项（CRC 相同才给），带缩略图：
	// 界面用它填顶部预览，省掉几十秒的"从设备读回"。对不上就是 null。
	Current *ScreenHistoryItemPayload `json:"current,omitempty"`
	Error   string                    `json:"error,omitempty"`
}

// ListScreenHistoryImages 列出可用的历史画布（排除"设备当前那张"，取最新 limit 张）。
// 纯本机 IO，不打设备；excludeCrc 用界面已经拿到的 `0xC5` CRC（没有就传 0）。
func (a *App) ListScreenHistoryImages(excludeCrc int, limit int) ScreenHistoryListPayload {
	fail := func(msg string) ScreenHistoryListPayload {
		return ScreenHistoryListPayload{Items: []ScreenHistoryItemPayload{}, Error: msg}
	}
	resp, err := a.sendRequest(ipc.ReqListScreenHistory, ipc.ScreenHistoryQueryParams{
		ExcludeCRC: uint16(excludeCrc), Limit: limit,
	})
	if err != nil || resp == nil || !resp.Success {
		return fail("读取历史图片失败")
	}
	var out ScreenHistoryListPayload
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return fail("解析历史图片失败: " + err.Error())
	}
	if out.Items == nil {
		out.Items = []ScreenHistoryItemPayload{}
	}
	return out
}

// UploadScreenHistoryImage 把一张历史画布**原样直传**上屏（`*.bin` 免解码、免裁剪）。
func (a *App) UploadScreenHistoryImage(path string) ScreenUploadResultPayload {
	return a.uploadScreen(ipc.ReqUploadScreenHistory, ipc.ScreenHistoryPathParams{Path: path})
}

// DeleteScreenHistoryImage 删除官方缓存目录里的一张历史画布；true 只表示删除成功。
func (a *App) DeleteScreenHistoryImage(path string) bool {
	resp, err := a.sendRequest(ipc.ReqDeleteScreenHistory, ipc.ScreenHistoryPathParams{Path: path})
	return err == nil && resp != nil && resp.Success
}

// PreviewScreenImageCrop 取一张本地图片在当前裁剪参数下的预览（data URL）。
// 纯本机操作，不需要设备在线 —— 可以先裁好再插散热器。
func (a *App) PreviewScreenImageCrop(path string, zoom, offsetX, offsetY int) ScreenCropPreviewPayload {
	resp, err := a.sendRequest(ipc.ReqPreviewScreenCrop, ipc.ScreenImagePathParams{
		Path: path, Zoom: zoom, OffsetX: offsetX, OffsetY: offsetY,
	})
	if err != nil || resp == nil || !resp.Success {
		return ScreenCropPreviewPayload{}
	}
	var out ScreenCropPreviewPayload
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return ScreenCropPreviewPayload{}
	}
	return out
}

// PickScreenImageFile 打开系统文件选择框，返回选中的路径（取消则返回空串）。
// 用系统对话框而不是手输路径：手输既不好用，也容易受工作目录影响而解析错。
func (a *App) PickScreenImageFile() string {
	if a.ctx == nil {
		return ""
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择屏保图片",
		Filters: []wailsruntime.FileFilter{
			// 只列本工具能解码的格式：Go 标准库有 png/jpeg/gif。
			//   官方还支持 bmp，但标准库没有 bmp 解码器；
			//   与其列上去然后报"解码失败"，不如不列（差异写在文档里）。
			{DisplayName: "图片 (*.png;*.jpg;*.jpeg;*.gif)", Pattern: "*.png;*.jpg;*.jpeg;*.gif"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}

func (a *App) GetScreenImageInfo() ScreenImageInfoPayload {
	resp, err := a.sendRequest(ipc.ReqGetScreenImageInfo, nil)
	if err != nil || resp == nil || !resp.Success {
		return ScreenImageInfoPayload{}
	}
	var out ScreenImageInfoPayload
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return ScreenImageInfoPayload{}
	}
	// 认图：CRC 反查内置预设，唯一所有者在 screenimg.PresetByCRC。
	// 认不出来（不是内置预设）时就如实说不认识，由界面提供「从设备读回」的入口。
	if out.HasImage && out.CRC != 0 {
		if pos, preset, ok := screenimg.PresetByCRC(out.CRC); ok {
			out.PresetMatched = true
			out.PresetOfficialPosition = pos
			out.PresetName = preset.Name
		}
	}
	return out
}

// uploadScreen 是两种上传的公共尾巴：走延长超时的 IPC + 统一错误表达。
func (a *App) uploadScreen(reqType ipc.RequestType, data any) ScreenUploadResultPayload {
	resp, err := a.sendRequestWithTimeout(reqType, data, screenUploadTimeout)
	if err != nil {
		return ScreenUploadResultPayload{Error: "上传请求失败: " + err.Error()}
	}
	if resp == nil {
		return ScreenUploadResultPayload{Error: "上传请求无响应"}
	}
	if !resp.Success {
		return ScreenUploadResultPayload{Error: resp.Error}
	}
	var out ScreenUploadResultPayload
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return ScreenUploadResultPayload{Error: "解析上传结果失败: " + err.Error()}
	}
	out.Success = true
	return out
}
