package deviceproto

// 本文件是黑鲨两张标定表共同的唯一所有者：
//   BlackSharkCalibration：sp ↔ RPM（0x24 固定形态）
//   BlackSharkCurveCalibration：Field ↔ RPM（0x24 曲线形态）

// BlackSharkRPMSetpoint 是标定曲线上的一个采样点。
type BlackSharkRPMSetpoint struct {
	Setpoint int // 写入配置的目标值（sp，不是 RPM —— 两者单位不同、数值也不同）
	RPM      int // 对应的稳态转速（RPM）
	// Step 标记「从上一行到本行之间存在一个台阶」（RPM 突跳远大于设定值跨度）。
	Step bool
}

// BlackSharkCalibration 是固定形态的 sp → RPM 等稳态标定表（已做等张处理）。
var BlackSharkCalibration = []BlackSharkRPMSetpoint{
	{Setpoint: 800, RPM: 1410},  // 带 0 —— 地板
	{Setpoint: 1200, RPM: 1410}, // 带 0
	{Setpoint: 1600, RPM: 1440}, // 带 0
	{Setpoint: 2000, RPM: 2100}, // 带 0
	{Setpoint: 2400, RPM: 2310}, // 带 120
	{Setpoint: 2800, RPM: 2760}, // 带 0
	{Setpoint: 3200, RPM: 3210}, // 带 150
	{Setpoint: 3600, RPM: 3720}, // 带 0
	{Setpoint: 4000, RPM: 4020}, // 带 210
	{Setpoint: 4400, RPM: 4500}, // 带 150
	{Setpoint: 4800, RPM: 4740}, // 带 210 —— 顶
}

// BlackSharkMinRPM / BlackSharkMaxRPM 是设备可达的转速范围（单位 RPM，唯一所有者）。
// 地板 = 1410、顶 = 4740。
const (
	BlackSharkMinRPM = 1410 // RPM：sp 800/1200 → 1410，判定带宽 0
	BlackSharkMaxRPM = 4740 // RPM：sp 4800 → 4740，带 210
	// BlackSharkRPMSnapRPM 是转速表的最小刻度（RPM）。
	// 设备报回的转速都是 30 的倍数（相邻读数逐个相差 30）。
	BlackSharkRPMSnapRPM = 30
	// BlackSharkMinSetpoint / BlackSharkMaxSetpoint 是本标定表覆盖的设定值(sp)范围，不是设备边界。
	BlackSharkMinSetpoint = 800  // sp
	BlackSharkMaxSetpoint = 4800 // sp
)

// BlackSharkSetpointForRPM 把期望转速换算成要写入设备的设定值 sp（插值）。
func BlackSharkSetpointForRPM(rpm int) int {
	table := BlackSharkCalibration
	if len(table) == 0 {
		return BlackSharkMinSetpoint
	}
	first, last := table[0], table[len(table)-1]
	if rpm <= first.RPM {
		return first.Setpoint
	}
	if rpm >= last.RPM {
		return last.Setpoint
	}
	for i := 1; i < len(table); i++ {
		lo, hi := table[i-1], table[i]
		if rpm > hi.RPM {
			continue
		}
		if hi.Step {
			// 孔洞：中间设备给不出，取更近的一端（等距时取下沿，更保守）
			if rpm-lo.RPM <= hi.RPM-rpm {
				return lo.Setpoint
			}
			return hi.Setpoint
		}
		if hi.RPM == lo.RPM {
			return hi.Setpoint
		}
		span := hi.RPM - lo.RPM
		offset := rpm - lo.RPM
		if offset < 0 {
			offset = 0
		} else if offset > span {
			offset = span
		}
		return lo.Setpoint + (hi.Setpoint-lo.Setpoint)*offset/span
	}
	return last.Setpoint
}

// BlackSharkRPMForSetpoint 是上面的反查：给定 sp，给出表里对应的转速（插值）。
// 用途：把设备回读到的 sp（或出厂固定 sp）换算成界面上应显示的转速。
func BlackSharkRPMForSetpoint(setpoint int) int {
	table := BlackSharkCalibration
	if len(table) == 0 {
		return BlackSharkMinRPM
	}
	first, last := table[0], table[len(table)-1]
	if setpoint <= first.Setpoint {
		return first.RPM
	}
	if setpoint >= last.Setpoint {
		return last.RPM
	}
	for i := 1; i < len(table); i++ {
		lo, hi := table[i-1], table[i]
		if setpoint > hi.Setpoint {
			continue
		}
		if hi.Step {
			// 孔洞段 [lo.sp, hi.sp)：只有正好落在 hi 上才够得到平台；
			// 段内任何更小的 sp 都停不住（中间没有工作点），返回下沿。
			if setpoint >= hi.Setpoint {
				return hi.RPM
			}
			return lo.RPM
		}
		span := hi.Setpoint - lo.Setpoint
		if span <= 0 {
			return hi.RPM
		}
		offset := setpoint - lo.Setpoint
		if offset < 0 {
			offset = 0
		} else if offset > span {
			offset = span
		}
		return lo.RPM + (hi.RPM-lo.RPM)*offset/span
	}
	return last.RPM
}

// BlackSharkRPMSupported 判断转速是否落在设备可达区间内；
// 调用点（device/blackshark_hid.go）在不满足时打一条 warn。
func BlackSharkRPMSupported(rpm int) bool {
	return rpm >= BlackSharkMinRPM && rpm <= BlackSharkMaxRPM
}

// BlackSharkManualGearPresetRPM 是手动挡位面板里一档的低/中/高三档转速（RPM）。
type BlackSharkManualGearPresetRPM struct {
	Gear   byte   `json:"gear"`   // 挡位 1..4
	Levels [3]int `json:"levels"` // 低 / 中 / 高（RPM）
}

// BlackSharkManualGearPresetsPayload 手动挡位面板需要的全部数值（前端唯一来源）。
type BlackSharkManualGearPresetsPayload struct {
	MinRPM int                             `json:"minRpm"` // 可达到的最低转速（RPM）
	MaxRPM int                             `json:"maxRpm"` // 可达到的最高转速（RPM）
	Gears  []BlackSharkManualGearPresetRPM `json:"gears"`
}

// BlackSharkManualGearPresets 生成四档 × 三档的转速预设（前端唯一来源）。
func BlackSharkManualGearPresets() BlackSharkManualGearPresetsPayload {
	out := BlackSharkManualGearPresetsPayload{
		MinRPM: BlackSharkMinRPM,
		MaxRPM: BlackSharkMaxRPM,
		Gears:  make([]BlackSharkManualGearPresetRPM, 0, BlackSharkGearCount),
	}
	if len(BlackSharkCalibration) == 0 {
		return out
	}
	lo := BlackSharkCalibration[0].RPM
	hi := BlackSharkCalibration[len(BlackSharkCalibration)-1].RPM
	total := 3 * int(BlackSharkGearCount) // 12 个点
	step := float64(hi-lo) / float64(total-1)
	for g := byte(1); g <= BlackSharkGearCount; g++ {
		var levels [3]int
		for k := 0; k < 3; k++ {
			raw := float64(lo) + step*float64(3*(int(g)-1)+k)
			levels[k] = snapToQuantum(raw, lo, hi)
		}
		// 保证档内严格递增（量化可能在窄区间把两点压到同一格）
		for k := 1; k < 3; k++ {
			if levels[k] <= levels[k-1] {
				levels[k] = levels[k-1] + BlackSharkRPMSnapRPM
			}
		}
		out.Gears = append(out.Gears, BlackSharkManualGearPresetRPM{Gear: g, Levels: levels})
	}
	return out
}

// snapToQuantum 把 rpm 量化到转速表刻度（`BlackSharkRPMSnapRPM`），并夹在 [lo, hi] 内。
func snapToQuantum(rpm float64, lo, hi int) int {
	q := BlackSharkRPMSnapRPM
	v := int((rpm+float64(q)/2)/float64(q)) * q
	if v < lo {
		v = lo
	}
	if v > hi {
		v = hi
	}
	return v
}

// 第二张表：曲线形态（0x24 form=01）的 Field ↔ RPM。

// BlackSharkCurveCalibrationPoint 曲线标定表的一行。
type BlackSharkCurveCalibrationPoint struct {
	Field int // 写入曲线里的字段值（不是 RPM，也不是固定路径的 sp）
	RPM   int // 对应的稳态转速（RPM）
}

// BlackSharkCurveCalibration 是曲线形态的 Field ↔ RPM 对照表。
// 曲线路径近似恒等（Field ≈ RPM）。
//
// 用途限于界面：`BlackSharkCurveRPMForField` 给 `BlackSharkInfo` 提供 `ApproxActualRPM`，
// `BlackSharkCurveRangeForRPM` 给出可拖拽的转速区间。
// 下发路径按 RPM 直接写字段，不经本表换算。
var BlackSharkCurveCalibration = []BlackSharkCurveCalibrationPoint{
	{1440, 1410}, // 带 0（地板）
	{2000, 1860}, // 带 150
	{3600, 3600}, // 带 120
	{3700, 3720}, // 带 120（读数 3705，量化到刻度 30）
	{4000, 3990}, // 带 120
	{4600, 4530}, // 带 240
}

// BlackSharkCurveRPMForField 是反查：给定曲线字段值，给出对应的转速（插值）。
func BlackSharkCurveRPMForField(field int) int {
	table := BlackSharkCurveCalibration
	if len(table) == 0 {
		return field
	}
	if field <= table[0].Field {
		return table[0].RPM
	}
	last := table[len(table)-1]
	if field >= last.Field {
		return last.RPM
	}
	for i := 1; i < len(table); i++ {
		lo, hi := table[i-1], table[i]
		if field > hi.Field {
			continue
		}
		if hi.Field == lo.Field {
			return hi.RPM
		}
		span := hi.Field - lo.Field
		return lo.RPM + (hi.RPM-lo.RPM)*(field-lo.Field)/span
	}
	return last.RPM
}

// BlackSharkCurveRangeForRPM 返回曲线形态可达到的转速范围。
func BlackSharkCurveRangeForRPM() (minRPM, maxRPM int) {
	if len(BlackSharkCurveCalibration) == 0 {
		return 0, 0
	}
	return BlackSharkCurveCalibration[0].RPM, BlackSharkCurveCalibration[len(BlackSharkCurveCalibration)-1].RPM
}
