// 黑鲨 BRB02 对外暴露的类型：固件版本 / 设备信息 / 灯效 / 内置档案 / 开关状态 / 情景规则。
package types

import (
	"fmt"
	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"strconv"
	"strings"
)

type BlackSharkFirmwareStatus struct {
	// Supported 表示当前连接的黑鲨设备是否支持固件版本查询。
	Supported bool `json:"supported"`
	// CurrentVersion 设备上报的固件版本，如 "3.0.3"。
	CurrentVersion string `json:"currentVersion,omitempty"`
	// LatestVersion 厂商清单中的最新版本。
	LatestVersion string `json:"latestVersion,omitempty"`
	// UpdateAvailable 是否存在可升级版本。
	UpdateAvailable bool `json:"updateAvailable"`
	// MinVersion 厂商要求的最低可用版本（低于此版本应尽快升级）。
	MinVersion string `json:"minVersion,omitempty"`
	// FirmwareURL 官方固件包地址。仅供展示与人工核对，本工具不会下载或写入设备。
	FirmwareURL string `json:"firmwareUrl,omitempty"`
	// FirmwareMD5 官方固件包 MD5，便于用户核对官方工具下载结果。
	FirmwareMD5 string `json:"firmwareMd5,omitempty"`
	// CheckedAt 最近一次成功检查的时间（RFC3339）。
	CheckedAt string `json:"checkedAt,omitempty"`
	// Error 检查失败时的原因（网络不可达、清单格式异常等）。失败不影响设备控制。
	Error string `json:"error,omitempty"`
	// UpdateMethod 升级方式，恒为 "official-tool"。
	UpdateMethod string `json:"updateMethod,omitempty"`
	// OfficialToolPath 官方工具路径（存在时给出），供前端提供「打开官方工具」入口。
	OfficialToolPath string `json:"officialToolPath,omitempty"`
	// OfficialToolFound 是否在本机找到官方工具。
	OfficialToolFound bool `json:"officialToolFound"`

	// BelowMinVersion 当前版本低于厂商最低要求版本。
	BelowMinVersion bool `json:"belowMinVersion,omitempty"`
}

// BlackSharkGear 一个档位的出厂冷却配置。
type BlackSharkGear struct {
	Gear int `json:"gear"` // 1..4，与官方 UI 的低噪/平衡/强效/超频一致

	// FixedValue 固定值形态的出厂值；FixedKnown 表示是否读取成功。
	FixedValue uint16 `json:"fixedValue"`
	FixedKnown bool   `json:"fixedKnown"`

	// Curve 温度曲线形态的出厂基准（4 点）。
	Curve      []BlackSharkCurvePointView `json:"curve,omitempty"`
	CurveKnown bool                       `json:"curveKnown"`
}

// BlackSharkCurvePointView 曲线点，同时给出字段值与换算后的转速，
// 便于前端直接显示"这个点大约多少转"，而不用自己带标定表。
type BlackSharkCurvePointView struct {
	TempC           uint8  `json:"tempC"`
	FieldRPM        uint16 `json:"fieldRpm"`
	ApproxActualRPM int    `json:"approxActualRpm"`
}

// BlackSharkCurveTempRange 黑鲨曲线那 4 个点能被拖到的温度取值域（℃，含两端）。
// 前端曲线图据此限制横向拖动，避免在界面里另写一份区间。
type BlackSharkCurveTempRange struct {
	MinTempC int `json:"minTempC"`
	MaxTempC int `json:"maxTempC"`
}

// BlackSharkCurveCalibrationEntry 曲线标定表的一行（前端用于限幅与提示）。
type BlackSharkCurveCalibrationEntry struct {
	FieldRPM  int `json:"fieldRpm"`
	ActualRPM int `json:"actualRpm"`
}

// BlackSharkInfo 黑鲨设备的聚合信息，供前端一次取全。
// 分开多次调用会产生多轮设备查询，界面上会出现明显卡顿。
type BlackSharkInfo struct {
	Available bool `json:"available"`

	Firmware BlackSharkFirmwareStatus `json:"firmware"`
	Switches BlackSharkSwitchStates   `json:"switches"`

	Gears []BlackSharkGear `json:"gears,omitempty"`

	// 曲线形态可达到的转速范围，以及标定表（前端用来把
	// "用户想要的转速" 换算/限幅成曲线字段）。
	CurveMinRPM      int                               `json:"curveMinRpm"`
	CurveMaxRPM      int                               `json:"curveMaxRpm"`
	CurveCalibration []BlackSharkCurveCalibrationEntry `json:"curveCalibration,omitempty"`

	// 固定值形态的可达转速范围（与曲线形态不同，不能混用）。
	FixedMinRPM int `json:"fixedMinRpm"`
	FixedMaxRPM int `json:"fixedMaxRpm"`
}

// BlackSharkRgbMode 一个灯效模式的参数。
type BlackSharkRgbMode struct {
	Index      int    `json:"index"`
	Name       string `json:"name,omitempty"`
	Speed      int    `json:"speed"`
	Brightness int    `json:"brightness"`

	Red   uint8 `json:"red"`
	Green uint8 `json:"green"`
	Blue  uint8 `json:"blue"`

	// Current 表示这是设备当前生效的模式（由 0x13 读回判断）。
	Current bool `json:"current"`

	// NeedsHostData 非空 = 设备有这个模式，但本工具驱动不了它，这里写清原因。
	NeedsHostData string `json:"needsHostData,omitempty"`

	// SpeedRange / BrightnessRange 是官方 UI 上滑块的量程，供前端限幅。
	// Brightness 全模式共用 10..100；Speed 逐模式不同（逐模式区间见 deviceproto.BlackSharkRgbSpeedRange）。
	SpeedRange      ValueRangeView `json:"speedRange"`
	BrightnessRange ValueRangeView `json:"brightnessRange"`

	// SpeedDisabled=true：该模式的速度滑块在官方 UI 里置灰、不可操作（值恒 0）。
	// 只有常亮(4) 与 音频同步(7) 两个；前端据此把速度滑杆 disabled。
	SpeedDisabled bool `json:"speedDisabled"`

	// StaticColor=true：该模式当前吃静态颜色（0x12 的 payload[4] = 0x01，官方 = 选中「单色」）；
	// false 即 0x0A（彩色）。前端开关是本地状态、不随模式同步，直接点「应用颜色」
	// 可能把设备的彩色翻成单色。
	StaticColor bool `json:"staticColor"`

	// ColorOption / ColorOptionName：颜色下拉当前选的是哪一项（0x12 的 payload[0] 高半字节，0..4）。
	// 官方 UI 的「颜色」下拉 = 5 项（彩虹/蓝紫追逐/黄绿/红蓝/橙紫），但它是按页配置的：
	// 只有部分灯效页会出现它（依据见 deviceproto.BlackSharkRgbColorOptionNames 的注释）。
	ColorOption     int    `json:"colorOption"`
	ColorOptionName string `json:"colorOptionName,omitempty"`

	// ColorControls 该灯效页实际显示哪些颜色控件（官方按页 hide/show，8 个灯效页已逐一核对）。
	// 前端据此 gating：设备没有的入口一律不显示（例如彩色循环/常亮/响应/刷新没有「颜色模式」下拉）。
	ColorControls BlackSharkRgbColorControlsView `json:"colorControls"`

	// Known=false 表示该模式的参数没读到（设备无响应），其余字段不可信。
	Known bool `json:"known"`
}

// BlackSharkRgbColorControlsView 一个灯效页显示哪些颜色控件（依据见 deviceproto 同名类型）。
type BlackSharkRgbColorControlsView struct {
	// ColorMode = 有「颜色模式」下拉（5 项），只有彩色流动有。
	ColorMode bool `json:"colorMode"`
	// SingleColor = 有「单色/彩色」开关。
	SingleColor bool `json:"singleColor"`
	// Hue = 色相滑杆的可用性，取值与 deviceproto 的 `BlackSharkHue*` 常量一致：
	// `none`（没有）/ `disabled`（有但置灰）/ `singleColorOnly`（仅「单色」时可用）/ `always`。
	Hue string `json:"hue"`
}

// BlackSharkRgbColorOptionView 颜色下拉的一个选项（序号 + 展示名）。
type BlackSharkRgbColorOptionView struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
}

// BlackSharkHostEffects 是主机侧灯效驱动（槽位 6「响应」/ 槽位 7「音频同步」）的运行状态，
// 这两个灯效要主机持续喂数据才会动。
type BlackSharkHostEffects struct {
	// CurrentSlot = 设备当前生效的灯效槽位（1..8）；0 = 未知或未连接。
	CurrentSlot int `json:"currentSlot"`

	// AudioSyncRunning=true：正在采集系统播放声音并按 ≈4.9 Hz 推 `0x15`。
	AudioSyncRunning bool `json:"audioSyncRunning"`
	// AudioError 非空 = 采集启动失败及原因（用户看到"没反应"时的唯一线索）。
	AudioError string `json:"audioError,omitempty"`
	// AudioFrequencyHz = 最近一次算出的主频（0 = 静音）；AudioLevel = 据此折出的档位（0 = 静音未下发）。
	AudioFrequencyHz float64 `json:"audioFrequencyHz"`
	AudioLevel       int     `json:"audioLevel"`
	// AudioPushCount = 本次运行累计下发的 `0x15` 帧数。
	AudioPushCount uint64 `json:"audioPushCount"`

	// ReactiveRunning=true：已装上全局键鼠低层钩子，按键/鼠标键按下会推 `0x16`。
	ReactiveRunning bool `json:"reactiveRunning"`
	// ReactiveError 非空 = 钩子启动失败及原因。
	ReactiveError string `json:"reactiveError,omitempty"`
	// ReactivePushCount = 本次运行累计下发的 `0x16` 帧数。
	ReactivePushCount uint64 `json:"reactivePushCount"`
}

// ValueRangeView 一个滑块的量程（上下限取自官方 UI）。
type ValueRangeView struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// BlackSharkRgbLighting 灯效页的聚合信息（按需查询，不要放进轮询）。
type BlackSharkRgbLighting struct {
	Available bool `json:"available"`

	// SwitchEnabled / SwitchKnown：0x10 写 / 0x11 读的开关状态。
	SwitchEnabled bool `json:"switchEnabled"`
	SwitchKnown   bool `json:"switchKnown"`

	// Modes 按索引升序（1..Count）。
	Modes []BlackSharkRgbMode `json:"modes,omitempty"`
	Count int                 `json:"count"`

	// CurrentIndex 当前生效的模式索引；0 表示未读到。
	CurrentIndex int `json:"currentIndex"`

	// ColorOptions 颜色下拉的全部选项（序号 + 名称，顺序即官方下拉顺序），从 deviceproto 带出，
	// 前端不再自己维护一份，免得改了一处另一处不跟着变。
	ColorOptions []BlackSharkRgbColorOptionView `json:"colorOptions,omitempty"`

	// ColorSettable 恒为 true：颜色确实由本工具写入（色相 → RGB、单色/彩色、颜色下拉 5 项）。
	ColorSettable bool `json:"colorSettable"`

	// FromCache / CachedAtUnix：这份数据是上次从设备读到的那一份，不是刚刚读的。
	FromCache    bool  `json:"fromCache,omitempty"`
	CachedAtUnix int64 `json:"cachedAtUnix,omitempty"`

	Error string `json:"error,omitempty"`
}

// 内置设备档案。
const (
	// BlackSharkFengShenProProfileID 黑鲨风神 PRO 内置档案 ID。
	BlackSharkFengShenProProfileID = "builtin.blackshark.fengshenpro.hid.rpm"

	// BlackSharkHIDProtocolTemplateID 协议模板标识。
	BlackSharkHIDProtocolTemplateID = "blackshark-hid-a5frame-v1"

	// 厂商 ID 不在这里声明：名字与值都由 `blackshark_profile.go` 提供
	//（`BlackSharkHIDVendorID` = `BlackSharkBRB02HIDVendorID` = 0xE2B7），
	// BLE 与 USB 两条通路也引用它，同一事实在包里只定义一处。

	// BlackSharkHIDMinRPM / BlackSharkHIDMaxRPM 是设备档案用的可达转速范围。
	BlackSharkHIDMinRPM = deviceproto.BlackSharkMinRPM
	BlackSharkHIDMaxRPM = deviceproto.BlackSharkMaxRPM

	blackSharkVendorName = "黑鲨（BlackShark）"
	blackSharkModelName  = "风神PRO"
)

// BlackSharkFengShenProProfile 返回黑鲨风神 PRO 的内置设备档案。
func BlackSharkFengShenProProfile() DeviceProfile {
	// SpeedRange 的值取自 deviceproto（可达转速区间的所有者），档案里不另写一份数。
	caps := DeviceCapabilities{
		ProfileID:              BlackSharkFengShenProProfileID,
		DisplayName:            blackSharkVendorName + blackSharkModelName,
		Transport:              DeviceTransportHID,
		SpeedUnit:              FanSpeedUnitRPM,
		SpeedRange:             DeviceSpeedRange{Min: BlackSharkHIDMinRPM, Max: BlackSharkHIDMaxRPM, Step: 1},
		SupportsReadState:      true,
		SupportsSetSpeed:       true,
		SupportsManualGears:    false,
		SupportsCustomSpeed:    true,
		SupportsDebugFrames:    false,
		SupportsRawCommands:    false,
		SupportsGearLight:      false,
		SupportsLighting:       false,
		SupportsBrightness:     false,
		SupportsScreen:         false,
		SupportsPowerOnStart:   false,
		SupportsSmartStartStop: false,
	}
	return DeviceProfile{
		ID:           BlackSharkFengShenProProfileID,
		DisplayName:  caps.DisplayName,
		Vendor:       blackSharkVendorName,
		Model:        blackSharkModelName,
		Notes:        blackSharkProfileNotes(),
		BuiltIn:      true,
		Transport:    DeviceTransportHID,
		SpeedUnit:    FanSpeedUnitRPM,
		SpeedRange:   caps.SpeedRange,
		Capabilities: caps,
		// DisplayFeatures 是卡片上如实写的 8 项（读取状态 0x06 / 固定转速 0x24 / 四档档位 0x24/0x26 /
		// 温度曲线 0x24），每项在专属面板里都有入口，协议侧已与官方逐字节对齐。
		DisplayFeatures: blackSharkDisplayFeatures(),
	}
}

// blackSharkDisplayFeatures 黑鲨在设备库卡片上要展示的能力标签（只影响显示）。
func blackSharkDisplayFeatures() []string {
	return []string{
		DeviceDisplayFeatureReadState,
		DeviceDisplayFeatureSetSpeed,
		DeviceDisplayFeatureManualGears,
		DeviceDisplayFeatureCustomSpeed,
		DeviceDisplayFeatureLighting,
		DeviceDisplayFeatureBrightness,
		DeviceDisplayFeaturePowerOnStart,
		DeviceDisplayFeatureSmartStartStop,
	}
}

func blackSharkProfileNotes() string {
	return "Protocol " + BlackSharkHIDProtocolTemplateID +
		" over USB HID (interrupt). VID 0xE2B7 / PID 0x7001, 65-byte reports. " +
		"Frame: A5 LEN CMD PAYLOAD CK, CK = byte-sum. " +
		"Verified on hardware: status polling (0x06), config read (0x25/0x26), " +
		"firmware version (0x01), config write (0x24)."
}

// IsBlackSharkDeviceProfileID 报告某个档案 ID 是不是黑鲨内置档案。
//
// 这是全仓库唯一的黑鲨档案判据，device 包的 `isBlackSharkProfileID` 直接转发到这里，
// 所以新增/删除黑鲨档案只需改这一处。已退役的 HID 档案（BlackSharkFengShenProProfileID）
// 不在其中。
func IsBlackSharkDeviceProfileID(profileID string) bool {
	switch strings.TrimSpace(profileID) {
	case BlackSharkBRB02ProfileID, BlackSharkBRB02USBProfileID:
		return true
	}
	return false
}

// BlackSharkGearCurveProfileID 生成黑鲨「四档散热模式」曲线方案的 ID。
func BlackSharkGearCurveProfileID(gear int) string {
	return fmt.Sprintf("%s.gear%d", BlackSharkFengShenProProfileID, gear)
}

// BlackSharkGearForCurveProfileID 把方案 ID 还原成档位号（1..4）。
// 返回 false 就不要去动设备档位 —— 那说明这是用户自己新建的方案。
func BlackSharkGearForCurveProfileID(profileID string) (int, bool) {
	profileID = strings.TrimSpace(profileID)
	prefix := BlackSharkFengShenProProfileID + ".gear"
	if !strings.HasPrefix(profileID, prefix) {
		return 0, false
	}
	gear, err := strconv.Atoi(strings.TrimSpace(profileID[len(prefix):]))
	if err != nil || gear < 1 || gear > SceneGearMax {
		return 0, false
	}
	return gear, true
}

// HasBlackSharkGearCurveProfile 报告一组曲线方案里是否含有「四档方案」；
// curveprofiles 用它决定要不要跳过右端延长（四档方案的曲线另有约束）。
func HasBlackSharkGearCurveProfile(profiles []FanCurveProfile) bool {
	for _, profile := range profiles {
		if _, ok := BlackSharkGearForCurveProfileID(profile.ID); ok {
			return true
		}
	}
	return false
}

// 配置快照（替代「恢复出厂」）。

// BlackSharkConfigSnapshot 一份设备配置快照。
type BlackSharkConfigSnapshot struct {
	Version  int                         `json:"version"`
	TakenAt  string                      `json:"takenAt"`
	RgbModes []BlackSharkRgbModeSnapshot `json:"rgbModes"`
	// CurrentRgbMode 是快照时正在生效的模式号；还原灯效块时写入即切换，
	// 所以最后要把这一项选回去，否则「还原」会把当前模式改掉。
	CurrentRgbMode int                      `json:"currentRgbMode,omitempty"`
	GearCurves     []BlackSharkGearSnapshot `json:"gearCurves"`
	Switches       BlackSharkSwitchSnapshot `json:"switches"`
	// Notes 记录"快照时就取不到、因此不在还原范围内"的项，避免还原后误以为已覆盖全部。
	Notes []string `json:"notes,omitempty"`
}

// BlackSharkRgbModeSnapshot 一个灯效模式的整块参数（原始 hex）。
type BlackSharkRgbModeSnapshot struct {
	Index int    `json:"index"`
	Raw   string `json:"raw"` // 8 字节，小写 hex，无分隔
}

// BlackSharkGearSnapshot 一个档位的冷却配置。
type BlackSharkGearSnapshot struct {
	Gear  int   `json:"gear"`
	Form  int   `json:"form"` // 0=固定值 1=温度曲线
	Value int   `json:"value,omitempty"`
	Temps []int `json:"temps,omitempty"`
	RPM   []int `json:"rpm,omitempty"`
}

// BlackSharkSwitchSnapshot 可读回的开关与来源。
type BlackSharkSwitchSnapshot struct {
	LightingEnabled  bool `json:"lightingEnabled"`
	LcdEnabled       bool `json:"lcdEnabled"`
	CoolingSource    int  `json:"coolingSource"`
	SmartStartStop   bool `json:"smartStartStop"`
	PowerOnSelfStart bool `json:"powerOnSelfStart"`
}

// BlackSharkConfigRestoreResult 还原结果：逐项成败都要报出来，不做静默通过。
type BlackSharkConfigRestoreResult struct {
	Restored []string `json:"restored"`
	Issues   []string `json:"issues"`
}

// BlackSharkSwitchStates 设备的可读回开关状态。
type BlackSharkSwitchStates struct {
	Available bool `json:"available"`

	LightingEnabled bool `json:"lightingEnabled"`
	LightingKnown   bool `json:"lightingKnown"`

	LcdScreenEnabled bool `json:"lcdScreenEnabled"`
	LcdScreenKnown   bool `json:"lcdScreenKnown"`

	CoolingSource      uint8 `json:"coolingSource"`
	CoolingSourceKnown bool  `json:"coolingSourceKnown"`

	// 智能启停 / 通电自启（写 CMD 0x02，读回 CMD 0x03）。
	SmartStartStop   bool `json:"smartStartStop"`
	PowerOnSelfStart bool `json:"powerOnSelfStart"`
	OnOffVectorKnown bool `json:"onOffVectorKnown"`

	// OnOffVectorReadbackSupported 表示固件提供 0x03 读回。当前固件为 true；
	// 保留该字段是为了在不支持读回的固件上仍能如实降级——那种情况下只能回填
	// 下面的最近下发值，且 Known 为 false。
	OnOffVectorReadbackSupported bool `json:"onOffVectorReadbackSupported"`

	// LastCommanded* 仅作为读回失败时的兜底记录，不是设备当前状态。
	LastCommandedSmartStartStop   bool `json:"lastCommandedSmartStartStop"`
	LastCommandedPowerOnSelfStart bool `json:"lastCommandedPowerOnSelfStart"`
}

// SceneRule 一个情景规则。
type SceneRule struct {
	// Name 是这个情景槽位的名字（可重命名，官方软件也是这么做的）。
	Name string `json:"name,omitempty"`

	// Enabled 该规则是否启用。未启用的规则一律跳过，不参与匹配。
	Enabled bool `json:"enabled"`

	// Match 前台进程名。大小写不敏感；带不带 `.exe` 都接受
	// （匹配时会先统一去掉扩展名再比较）。
	Match string `json:"match"`

	// Gear 命中后要施加的设备档位（1..4，对应官方 UI 的低噪/平衡/强效/超频）。
	// 0 = 不改变当前档位。
	// 具体可用档位以设备返回的 gears 为准，不要假定 4 档一定都存在。
	Gear int `json:"gear"`

	// RgbMode 命中后要切换的灯效模式（1..8，设备共 8 个模式）。
	// 0 = 不改变当前灯效模式。
	RgbMode int `json:"rgbMode"`
}

// SceneRgbModeCount 灯效模式数量上限（与 deviceproto.BlackSharkRgbEffectCount 一致）。
// 放在 types 里是为了让配置校验不必依赖 deviceproto 包。
const SceneRgbModeCount = 8

// SceneGearMax 档位上限。官方 UI 上是低噪/平衡/强效/超频四档。
const SceneGearMax = 4

// SceneRuleNameMaxLen 槽位名的长度上限。
const SceneRuleNameMaxLen = 24

// TrimSceneRuleName 归一化槽位名（去首尾空白 + 按字符截断）。
func TrimSceneRuleName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	runes := []rune(name)
	if len(runes) > SceneRuleNameMaxLen {
		return string(runes[:SceneRuleNameMaxLen])
	}
	return name
}

// LCD 显示参数（0xC2）：屏上三格显示哪三项、按什么顺序
// （官方对话框原文：「选择先后顺序为屏幕左右顺序」）。
const (
	// BlackSharkLcdItemCount 官方约束：恰好三项（对话框写着「请选择三项参数」）。
	BlackSharkLcdItemCount = 3
	// BlackSharkLcdItemMaxID 条目 id 上界（7 = 时间）。
	BlackSharkLcdItemMaxID = 7
	// BlackSharkLcdPosMax 是允许下发的 pos 上界。
	BlackSharkLcdPosMax = deviceproto.BlackSharkLcdCalibratedPosMax
)

// DefaultBlackSharkLcdItems 是默认显示项：CPU温度 / GPU温度 / 时间（id 0/1/7），与官方默认一致。
func DefaultBlackSharkLcdItems() []int { return []int{0, 1, 7} }

// NormalizeBlackSharkLcdItems 归一化 LCD 显示项：丢掉越界项与重复项，不足三项用默认项补齐。
func NormalizeBlackSharkLcdItems(items []int) ([]int, bool) {
	out := make([]int, 0, BlackSharkLcdItemCount)
	seen := make(map[int]bool, BlackSharkLcdItemCount)
	for _, it := range items {
		if it < 0 || it > BlackSharkLcdItemMaxID || seen[it] {
			continue
		}
		seen[it] = true
		out = append(out, it)
		if len(out) == BlackSharkLcdItemCount {
			break
		}
	}
	changed := len(out) != len(items)
	for _, def := range DefaultBlackSharkLcdItems() {
		if len(out) >= BlackSharkLcdItemCount {
			break
		}
		if seen[def] {
			continue
		}
		seen[def] = true
		out = append(out, def)
		changed = true
	}
	return out, changed
}

// SceneRuleIssues 返回规则中不合法的字段说明（空切片表示合法）。
func (r SceneRule) SceneRuleIssues() []string {
	var issues []string
	if r.Match == "" && r.Enabled {
		issues = append(issues, "已启用但没有填写进程名")
	}
	if r.Gear < 0 || r.Gear > SceneGearMax {
		issues = append(issues, "档位超出 0..4")
	}
	if r.RgbMode < 0 || r.RgbMode > SceneRgbModeCount {
		issues = append(issues, "灯效模式超出 0..8")
	}
	if r.Enabled && r.Gear == 0 && r.RgbMode == 0 {
		issues = append(issues, "已启用但档位与灯效模式都是 0（不改变任何东西），这条规则不会有效果")
	}
	return issues
}
