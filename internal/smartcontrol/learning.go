package smartcontrol

import (
	"github.com/TIANLI0/THRM/internal/types"
)

// StableObserver + 稳态学习用到的常量。
const (
	// rpmPerDegree     = 50
	hardOffsetCap    = 60
	stableTempBand   = 2
	stableMinSamples = 6
	stableRPMBand    = 8

	// 冷却效率估计与转速寻优相关常量。
	effHistoryLen    = 6    // 每个温度桶保留的稳态 (转速,温度) 样本数
	minRPMSpanForEff = 8    // 估计冷却效率所需的最小转速跨度
	effFloorPerRPM   = 0.03 // 冷却效率下限，防止步长发散
	effCeilPerRPM    = 1.2  // 冷却效率上限
	defaultEffPerRPM = 0.20 // 无历史时的保守冷却效率
	maxLearnStep     = 8    // 单次学习的最大速度调整
	learnStepDeadRPM = 1    // 小于此调整量则忽略，避免抖动
	minSafetyStep    = 2    // 温度超目标时的最小降温步长
	defaultTargetTmp = 70   // TargetTemp 未配置时的回退目标温度 (°C)

	offsetSmoothPasses         = 2
	offsetSmoothPullLimit      = 3
	offsetSmoothSelfWeight     = 0.7
	offsetSmoothNeighborWeight = 0.15
	offsetSmoothRadius         = 2
	eqConsistencyBand          = 3

	stablePowerAbsBandWatts  = 15.0
	stablePowerRelBand       = 0.15
	powerHistoryAbsBandWatts = 25.0
	powerHistoryRelBand      = 0.25

	rawPercentUnit = "percent-raw"
)

type learningTuning struct {
	hardOffsetCap       int
	stableRPMBand       int
	minRPMSpanForEff    int
	effFloorPerRPM      float64
	effCeilPerRPM       float64
	defaultEffPerRPM    float64
	maxLearnStep        int
	learnStepDeadRPM    int
	minSafetyStep       int
	offsetSmoothPullMax int
}

func learningTuningForUnit(unit string) learningTuning {
	if unit == rawPercentUnit {
		return learningTuning{
			hardOffsetCap:       60,
			stableRPMBand:       8,
			minRPMSpanForEff:    8,
			effFloorPerRPM:      0.03,
			effCeilPerRPM:       1.2,
			defaultEffPerRPM:    0.20,
			maxLearnStep:        8,
			learnStepDeadRPM:    1,
			minSafetyStep:       2,
			offsetSmoothPullMax: 3,
		}
	}
	if types.IsRPMSpeedUnit(unit) {
		return learningTuning{
			hardOffsetCap:       600,
			stableRPMBand:       120,
			minRPMSpanForEff:    80,
			effFloorPerRPM:      0.0008,
			effCeilPerRPM:       0.05,
			defaultEffPerRPM:    0.008,
			maxLearnStep:        80,
			learnStepDeadRPM:    20,
			minSafetyStep:       20,
			offsetSmoothPullMax: 30,
		}
	}
	return learningTuning{
		hardOffsetCap:       600,
		stableRPMBand:       80,
		minRPMSpanForEff:    80,
		effFloorPerRPM:      0.003,
		effCeilPerRPM:       0.12,
		defaultEffPerRPM:    0.020,
		maxLearnStep:        80,
		learnStepDeadRPM:    10,
		minSafetyStep:       20,
		offsetSmoothPullMax: 30,
	}
}

func normalizeLearningUnit(unit string) string {
	if unit == rawPercentUnit {
		return rawPercentUnit
	}
	return types.NormalizeFanSpeedUnit(unit)
}

// EffectivePower keeps the usable CPU/GPU readings for one sample.
// Zero or invalid components are not treated as known idle load.
type EffectivePower struct {
	CPUWatts float64
	GPUWatts float64
	CPUValid bool
	GPUValid bool
}

func normalizeEffectivePower(power EffectivePower) EffectivePower {
	if !power.CPUValid || power.CPUWatts <= 0 {
		power.CPUWatts = 0
		power.CPUValid = false
	}
	if !power.GPUValid || power.GPUWatts <= 0 {
		power.GPUWatts = 0
		power.GPUValid = false
	}
	return power
}

func (power EffectivePower) hasPower() bool {
	return power.CPUValid || power.GPUValid
}

func (power EffectivePower) total() float64 {
	total := 0.0
	if power.CPUValid {
		total += power.CPUWatts
	}
	if power.GPUValid {
		total += power.GPUWatts
	}
	return total
}

func effectivePowerContextClose(a, b EffectivePower, absBand, relBand float64) bool {
	a = normalizeEffectivePower(a)
	b = normalizeEffectivePower(b)
	if !a.hasPower() || !b.hasPower() || a.CPUValid != b.CPUValid || a.GPUValid != b.GPUValid {
		return false
	}
	if a.CPUValid && !powerClose(a.CPUWatts, b.CPUWatts, absBand, relBand) {
		return false
	}
	if a.GPUValid && !powerClose(a.GPUWatts, b.GPUWatts, absBand, relBand) {
		return false
	}
	return powerClose(a.total(), b.total(), absBand, relBand)
}

func effectivePowerFromTotal(totalPowerWatts float64, havePower bool) EffectivePower {
	if !havePower || totalPowerWatts <= 0 {
		return EffectivePower{}
	}
	return EffectivePower{CPUWatts: totalPowerWatts, CPUValid: true}
}

// eqPoint 记录一次稳态 (转速, 温度) 平衡点。
type eqPoint struct {
	rpm   int
	temp  int
	power EffectivePower
}

// SteadyResult 是一次稳态观测的结果。
type SteadyResult struct {
	BucketIdx          int     // 命中的曲线点索引；-1 表示无效
	MeanTemp           int     // 稳态平均温度 (°C)
	MeanRPM            int     // 稳态期间的平均下发转速 (RPM)
	MeanPower          float64 // 稳态期间的平均 CPU+GPU 功耗 (W)
	HavePower          bool    // 稳态样本是否具备可用功耗
	LocalEff           float64 // 局部冷却效率 (°C/RPM)，正值
	HaveEff            bool    // 是否成功估计出冷却效率
	QuietLearningReady bool    // 连续两次同桶、同功耗或同无功耗上下文的稳态确认
	Ready              bool    // 是否达到稳态、可触发一次学习
}

// StableObserver 为每个曲线点累积稳态采样，并维护 (转速,温度) 平衡点历史。
type StableObserver struct {
	curveLen         int
	unit             string
	samples          [][]int // 每个温度桶的温度采样
	rpmSamples       [][]int // 与 samples 平行的转速采样
	powerSamples     [][]EffectivePower
	history          [][]eqPoint // 每个温度桶最近的稳态平衡点
	settle           []int       // 每个温度桶进入稳定采样前的延迟计数
	lastTemps        []int       // 最近一次观测温度
	lastRPMs         []int       // 最近一次观测到的实际转速
	lastPowers       []EffectivePower
	seen             []bool // 最近观测是否有效
	powerSeen        []bool
	lastSteadyBucket int
	lastSteadyPower  EffectivePower
	lastSteadySeen   bool
}

// NewStableObserver 创建针对当前曲线长度的观察者。
func NewStableObserver(curveLen int) *StableObserver {
	return NewStableObserverForUnit(curveLen, rawPercentUnit)
}

func NewStableObserverForUnit(curveLen int, unit string) *StableObserver {
	if curveLen <= 0 {
		curveLen = 1
	}
	o := &StableObserver{curveLen: curveLen, unit: normalizeLearningUnit(unit)}
	o.allocBuffers(curveLen)
	return o
}

func (o *StableObserver) SetUnit(unit string) bool {
	if o == nil {
		return false
	}
	next := normalizeLearningUnit(unit)
	if o.unit == next {
		return false
	}
	o.unit = next
	o.Reset()
	return true
}

func (o *StableObserver) allocBuffers(curveLen int) {
	o.samples = make([][]int, curveLen)
	o.rpmSamples = make([][]int, curveLen)
	o.powerSamples = make([][]EffectivePower, curveLen)
	o.history = make([][]eqPoint, curveLen)
	o.settle = make([]int, curveLen)
	o.lastTemps = make([]int, curveLen)
	o.lastRPMs = make([]int, curveLen)
	o.lastPowers = make([]EffectivePower, curveLen)
	o.seen = make([]bool, curveLen)
	o.powerSeen = make([]bool, curveLen)
	o.clearQuietLearningContext()
	for i := range o.samples {
		o.samples[i] = make([]int, 0, 24)
		o.rpmSamples[i] = make([]int, 0, 24)
		o.powerSamples[i] = make([]EffectivePower, 0, 24)
		o.history[i] = make([]eqPoint, 0, effHistoryLen)
	}
}

// Resize 在曲线长度变化时调整内部缓冲。曲线变化会使历史失效，因此一并清空。
func (o *StableObserver) Resize(curveLen int) {
	if curveLen <= 0 {
		curveLen = 1
	}
	if o.curveLen == curveLen {
		o.Reset()
		return
	}
	o.curveLen = curveLen
	o.allocBuffers(curveLen)
}

// Reset 清空进行中的采样缓冲，但保留已学到的效率历史。
func (o *StableObserver) Reset() {
	for i := range o.samples {
		o.samples[i] = o.samples[i][:0]
		o.rpmSamples[i] = o.rpmSamples[i][:0]
		o.powerSamples[i] = o.powerSamples[i][:0]
		o.settle[i] = 0
		o.lastTemps[i] = 0
		o.lastRPMs[i] = 0
		o.lastPowers[i] = EffectivePower{}
		o.seen[i] = false
		o.powerSeen[i] = false
	}
	o.clearQuietLearningContext()
}

func (o *StableObserver) clearQuietLearningContext() {
	if o == nil {
		return
	}
	o.lastSteadyBucket = -1
	o.lastSteadyPower = EffectivePower{}
	o.lastSteadySeen = false
}

func stableSampleWindow(cfg types.SmartControlConfig) int {
	window := cfg.LearnWindow
	if window <= 0 {
		window = stableMinSamples
	}
	return clampInt(window, 3, 24)
}

func stableSampleDelay(cfg types.SmartControlConfig) int {
	delay := max(cfg.LearnDelay, 0)
	return clampInt(delay, 0, 8)
}

func stableRPMRangeForUnit(cfg types.SmartControlConfig, unit string) int {
	return max(learningTuningForUnit(unit).stableRPMBand, cfg.MinRPMChange)
}

// CurveLen 返回当前观察者的曲线长度。
func (o *StableObserver) CurveLen() int {
	return o.curveLen
}

// pickBucketIndex 按最近邻选择温度所属的曲线点。
func pickBucketIndex(temp int, curve []types.FanCurvePoint) int {
	if len(curve) == 0 {
		return -1
	}
	if temp <= curve[0].Temperature {
		return 0
	}
	if temp >= curve[len(curve)-1].Temperature {
		return len(curve) - 1
	}
	for i := 0; i < len(curve)-1; i++ {
		if temp >= curve[i].Temperature && temp < curve[i+1].Temperature {
			midpoint := (curve[i].Temperature + curve[i+1].Temperature) / 2
			if temp < midpoint {
				return i
			}
			return i + 1
		}
	}
	return len(curve) - 1
}

// Observe 把一次 (温度, 实际转速) 采样放入对应温度桶。
// 达到稳态时返回平均温度、平均转速及局部冷却效率估计。
func (o *StableObserver) Observe(temp, effectiveRPM int, curve []types.FanCurvePoint, cfg types.SmartControlConfig) SteadyResult {
	return o.ObserveWithPower(temp, effectiveRPM, 0, false, curve, cfg)
}

// ObserveWithPower 把功耗作为稳态判定条件；无功耗时自动退回温度/转速学习。
func (o *StableObserver) ObserveWithPower(temp, effectiveRPM int, totalPowerWatts float64, havePower bool, curve []types.FanCurvePoint, cfg types.SmartControlConfig) SteadyResult {
	return o.ObserveWithEffectivePower(temp, effectiveRPM, effectivePowerFromTotal(totalPowerWatts, havePower), curve, cfg)
}

// ObserveWithEffectivePower uses the CPU/GPU readings that are valid for this
// sample. The legacy total-power method remains available for existing callers.
func (o *StableObserver) ObserveWithEffectivePower(temp, effectiveRPM int, power EffectivePower, curve []types.FanCurvePoint, cfg types.SmartControlConfig) SteadyResult {
	idx := pickBucketIndex(temp, curve)
	if idx < 0 || idx >= len(o.samples) {
		return SteadyResult{BucketIdx: -1}
	}
	if o.lastSteadySeen && o.lastSteadyBucket != idx {
		o.clearQuietLearningContext()
	}
	window := stableSampleWindow(cfg)
	delay := stableSampleDelay(cfg)
	rpmBand := stableRPMRangeForUnit(cfg, o.unit)
	power = normalizeEffectivePower(power)
	havePower := power.hasPower()

	if o.seen[idx] {
		tempJump := absInt(temp-o.lastTemps[idx]) > stableTempBand+1
		rpmJump := effectiveRPM > 0 && o.lastRPMs[idx] > 0 && absInt(effectiveRPM-o.lastRPMs[idx]) > rpmBand
		powerJump := havePower != o.powerSeen[idx] ||
			(havePower && o.powerSeen[idx] && !effectivePowerContextClose(power, o.lastPowers[idx], stablePowerAbsBandWatts, stablePowerRelBand))
		if tempJump || rpmJump || powerJump {
			o.samples[idx] = o.samples[idx][:0]
			o.rpmSamples[idx] = o.rpmSamples[idx][:0]
			o.powerSamples[idx] = o.powerSamples[idx][:0]
			o.settle[idx] = 0
			o.clearQuietLearningContext()
		}
	} else {
		o.seen[idx] = true
		o.settle[idx] = 0
	}
	o.lastTemps[idx] = temp
	o.lastRPMs[idx] = effectiveRPM
	o.lastPowers[idx] = power
	o.powerSeen[idx] = havePower

	if o.settle[idx] < delay {
		o.settle[idx]++
		return SteadyResult{BucketIdx: idx}
	}

	o.samples[idx] = append(o.samples[idx], temp)
	o.rpmSamples[idx] = append(o.rpmSamples[idx], effectiveRPM)
	o.powerSamples[idx] = append(o.powerSamples[idx], power)
	if len(o.samples[idx]) > window {
		o.samples[idx] = o.samples[idx][len(o.samples[idx])-window:]
		o.rpmSamples[idx] = o.rpmSamples[idx][len(o.rpmSamples[idx])-window:]
		o.powerSamples[idx] = o.powerSamples[idx][len(o.powerSamples[idx])-window:]
	}

	if len(o.samples[idx]) < window {
		return SteadyResult{BucketIdx: idx}
	}
	minT, maxT, sumT, sumR := o.samples[idx][0], o.samples[idx][0], 0, 0
	minR, maxR := o.rpmSamples[idx][0], o.rpmSamples[idx][0]
	minP, maxP := 0.0, 0.0
	for i, t := range o.samples[idx] {
		if t < minT {
			minT = t
		}
		if t > maxT {
			maxT = t
		}
		rpm := o.rpmSamples[idx][i]
		if rpm < minR {
			minR = rpm
		}
		if rpm > maxR {
			maxR = rpm
		}
		sumT += t
		sumR += rpm
	}
	if maxT-minT > stableTempBand || maxR-minR > rpmBand {
		o.clearQuietLearningContext()
		return SteadyResult{BucketIdx: idx}
	}
	meanPower, steadyHavePower := averageEffectivePower(o.powerSamples[idx])
	meanP := meanPower.total()
	if steadyHavePower {
		for _, samplePower := range o.powerSamples[idx] {
			total := samplePower.total()
			if minP == 0 || total < minP {
				minP = total
			}
			if total > maxP {
				maxP = total
			}
		}
		if maxP-minP > powerBandForMean(meanP, stablePowerAbsBandWatts, stablePowerRelBand) {
			o.clearQuietLearningContext()
			return SteadyResult{BucketIdx: idx}
		}
	}

	meanT := sumT / len(o.samples[idx])
	meanR := sumR / len(o.rpmSamples[idx])
	quietLearningReady := o.lastSteadySeen && o.lastSteadyBucket == idx &&
		((!meanPower.hasPower() && !o.lastSteadyPower.hasPower()) ||
			effectivePowerContextClose(meanPower, o.lastSteadyPower, stablePowerAbsBandWatts, stablePowerRelBand))
	o.lastSteadyBucket = idx
	o.lastSteadyPower = meanPower
	o.lastSteadySeen = true
	o.samples[idx] = o.samples[idx][:0]
	o.rpmSamples[idx] = o.rpmSamples[idx][:0]
	o.powerSamples[idx] = o.powerSamples[idx][:0]
	o.settle[idx] = 0

	o.recordEquilibrium(idx, meanR, meanT, meanPower)
	eff, haveEff := o.localEfficiencyForEffectivePower(idx, meanPower)

	return SteadyResult{
		BucketIdx:          idx,
		MeanTemp:           meanT,
		MeanRPM:            meanR,
		MeanPower:          meanP,
		HavePower:          steadyHavePower,
		LocalEff:           eff,
		HaveEff:            haveEff,
		QuietLearningReady: quietLearningReady,
		Ready:              true,
	}
}

func averageEffectivePower(samples []EffectivePower) (EffectivePower, bool) {
	if len(samples) == 0 {
		return EffectivePower{}, false
	}
	first := normalizeEffectivePower(samples[0])
	if !first.hasPower() {
		return EffectivePower{}, false
	}
	mean := EffectivePower{CPUValid: first.CPUValid, GPUValid: first.GPUValid}
	for _, sample := range samples {
		sample = normalizeEffectivePower(sample)
		if !sample.hasPower() || sample.CPUValid != mean.CPUValid || sample.GPUValid != mean.GPUValid {
			return EffectivePower{}, false
		}
		if mean.CPUValid {
			mean.CPUWatts += sample.CPUWatts
		}
		if mean.GPUValid {
			mean.GPUWatts += sample.GPUWatts
		}
	}
	if mean.CPUValid {
		mean.CPUWatts /= float64(len(samples))
	}
	if mean.GPUValid {
		mean.GPUWatts /= float64(len(samples))
	}
	return mean, true
}

// recordEquilibrium 把一次稳态平衡点写入桶历史（环形保留最近 effHistoryLen 条）。
// 同一转速附近的旧样本会被新样本覆盖，使历史反映最新的热行为。
func (o *StableObserver) recordEquilibrium(idx, rpm, temp int, power EffectivePower) {
	if idx < 0 || idx >= len(o.history) {
		return
	}
	hist := o.history[idx]
	replaced := false
	kept := hist[:0]
	for _, p := range hist {
		sameSpeed := absInt(p.rpm-rpm) < learningTuningForUnit(o.unit).minRPMSpanForEff
		samePower := (!power.hasPower() && !p.power.hasPower()) ||
			(power.hasPower() && p.power.hasPower() && effectivePowerContextClose(power, p.power, powerHistoryAbsBandWatts, powerHistoryRelBand))
		if !replaced && sameSpeed && samePower {
			kept = append(kept, eqPoint{rpm: rpm, temp: temp, power: power})
			replaced = true
			continue
		}
		if !staleEquilibriumForUnit(p, rpm, temp, power, o.unit) {
			kept = append(kept, p)
		}
	}
	if !replaced {
		kept = append(kept, eqPoint{rpm: rpm, temp: temp, power: power})
	}
	if len(kept) > effHistoryLen {
		kept = kept[len(kept)-effHistoryLen:]
	}
	o.history[idx] = kept
}

func staleEquilibriumForUnit(p eqPoint, rpm, temp int, power EffectivePower, unit string) bool {
	if power.hasPower() && (!p.power.hasPower() || !effectivePowerContextClose(power, p.power, powerHistoryAbsBandWatts, powerHistoryRelBand)) {
		return false
	}
	tuning := learningTuningForUnit(unit)
	if p.rpm < rpm {
		if p.temp+eqConsistencyBand < temp {
			return true
		}
		maxDrop := tuning.effCeilPerRPM*float64(rpm-p.rpm) + eqConsistencyBand
		return float64(p.temp-temp) > maxDrop
	}
	if p.rpm > rpm {
		if p.temp > temp+eqConsistencyBand {
			return true
		}
		maxDrop := tuning.effCeilPerRPM*float64(p.rpm-rpm) + eqConsistencyBand
		return float64(temp-p.temp) > maxDrop
	}
	return false
}

// localEfficiency 用历史平衡点回归估计局部冷却效率；无功耗时使用全部历史。
func (o *StableObserver) localEfficiency(idx int) (float64, bool) {
	return o.localEfficiencyForPower(idx, 0, false)
}

func (o *StableObserver) localEfficiencyForPower(idx int, referencePower float64, havePower bool) (float64, bool) {
	return o.localEfficiencyForEffectivePower(idx, effectivePowerFromTotal(referencePower, havePower))
}

func (o *StableObserver) localEfficiencyForEffectivePower(idx int, referencePower EffectivePower) (float64, bool) {
	if idx < 0 || idx >= len(o.history) {
		return 0, false
	}
	hist := o.history[idx]
	if len(hist) < 2 {
		return 0, false
	}
	tuning := learningTuningForUnit(o.unit)
	selected := make([]eqPoint, 0, len(hist))
	for _, p := range hist {
		if !referencePower.hasPower() || (p.power.hasPower() && effectivePowerContextClose(referencePower, p.power, powerHistoryAbsBandWatts, powerHistoryRelBand)) {
			selected = append(selected, p)
		}
	}
	if len(selected) < 2 {
		return 0, false
	}
	minRPM, maxRPM := selected[0].rpm, selected[0].rpm
	sumR, sumT := 0, 0
	for _, p := range selected {
		if p.rpm < minRPM {
			minRPM = p.rpm
		}
		if p.rpm > maxRPM {
			maxRPM = p.rpm
		}
		sumR += p.rpm
		sumT += p.temp
	}
	if maxRPM-minRPM < tuning.minRPMSpanForEff {
		return 0, false
	}
	meanR := float64(sumR) / float64(len(selected))
	meanT := float64(sumT) / float64(len(selected))
	var cov, varR float64
	for _, p := range selected {
		dr := float64(p.rpm) - meanR
		cov += dr * (float64(p.temp) - meanT)
		varR += dr * dr
	}
	if varR <= 0 {
		return 0, false
	}
	eff := -cov / varR
	if eff < tuning.effFloorPerRPM {
		// 冷却几乎无效（甚至负相关）：视为最低效率，让寻优倾向于降转速省噪音。
		eff = tuning.effFloorPerRPM
	}
	if eff > tuning.effCeilPerRPM {
		eff = tuning.effCeilPerRPM
	}
	return eff, true
}

func powerBandForMean(meanPower, absBand, relBand float64) float64 {
	if meanPower <= 0 {
		return absBand
	}
	relative := meanPower * relBand
	if relative > absBand {
		return relative
	}
	return absBand
}

func powerClose(a, b, absBand, relBand float64) bool {
	if a <= 0 || b <= 0 {
		return false
	}
	mean := (a + b) / 2
	return absFloat(a-b) <= powerBandForMean(mean, absBand, relBand)
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

// alphaFromLearnRate 把 1..10 的 LearnRate 映射成反馈系数。
func alphaFromLearnRate(learnRate int) float64 {
	if learnRate < 1 {
		learnRate = 1
	}
	if learnRate > 10 {
		learnRate = 10
	}
	return 0.025 + float64(learnRate-1)*0.0125
}

// effectiveOffsetCap 取 cfg.MaxLearnOffset 和 hardOffsetCap 的较小值。
func effectiveOffsetCap(cfg types.SmartControlConfig) int {
	return effectiveOffsetCapForUnit(cfg, rawPercentUnit)
}

func effectiveOffsetCapForUnit(cfg types.SmartControlConfig, unit string) int {
	tuning := learningTuningForUnit(unit)
	cap := cfg.MaxLearnOffset
	if cap <= 0 || cap > tuning.hardOffsetCap {
		cap = tuning.hardOffsetCap
	}
	return cap
}

// targetTempCeiling 返回学习寻优使用的目标温度上限。
func targetTempCeiling(cfg types.SmartControlConfig) int {
	if cfg.TargetTemp > 0 {
		return cfg.TargetTemp
	}
	return defaultTargetTmp
}

// comfortBandWidth 返回目标温度下方的舒适带宽度 (°C)。
// 舒适带内不动作，避免无意义的转速抖动；带宽随滞回温差略微放宽。
func comfortBandWidth(cfg types.SmartControlConfig) int {
	band := max(cfg.Hysteresis+3, 3)
	return band
}

// AllowsSteadyOffsetLearning keeps high-temperature safety corrections
// immediate, while requiring a matching steady confirmation before a quiet
// down-adjustment can change learned offsets.
func AllowsSteadyOffsetLearning(steady SteadyResult, cfg types.SmartControlConfig) bool {
	if !steady.Ready {
		return false
	}
	return steady.MeanTemp >= targetTempCeiling(cfg)-comfortBandWidth(cfg) || steady.QuietLearningReady
}

// solveLearnStep 依据稳态温度、目标温度带与冷却效率，求出本次应施加的转速调整 (RPM)。
//
// 策略：
//   - 温度高于目标温度  → 加转速降温，步长 = α·(超出°C)/效率，确保把温度压回目标附近。
//   - 温度处于舒适带内  → 保持不动（这是消除“无脑降温”的关键：温度够低就不再加速）。
//   - 温度低于舒适带    → 主动降转速省噪音，可降幅 = α·(可上升°C)/效率；
//     冷却越低效（效率小），同样的降速带来的升温越小，于是越敢大幅降速。
//
// 冷却效率 eff (°C/RPM) 把“温度误差”换算成“转速需求”，使步长物理合理、收敛快且不易过冲。
func solveLearnStep(steadyTemp int, eff float64, haveEff bool, cfg types.SmartControlConfig) int {
	return solveLearnStepForUnit(steadyTemp, eff, haveEff, cfg, rawPercentUnit)
}

func solveLearnStepForUnit(steadyTemp int, eff float64, haveEff bool, cfg types.SmartControlConfig, unit string) int {
	return solveLearnStepForUnitWithPower(steadyTemp, eff, haveEff, cfg, unit, 0, false)
}

func solveLearnStepForUnitWithPower(steadyTemp int, eff float64, haveEff bool, cfg types.SmartControlConfig, unit string, meanPower float64, havePower bool) int {
	tuning := learningTuningForUnit(unit)
	ceiling := targetTempCeiling(cfg)
	lowTarget := ceiling - comfortBandWidth(cfg)
	alpha := alphaFromLearnRate(cfg.LearnRate)

	if !haveEff || eff < tuning.effFloorPerRPM {
		eff = tuning.defaultEffPerRPM
	}
	if eff > tuning.effCeilPerRPM {
		eff = tuning.effCeilPerRPM
	}

	var step float64
	switch {
	case steadyTemp > ceiling:
		step = alpha * float64(steadyTemp-ceiling) / eff
		if step < float64(tuning.minSafetyStep) {
			step = float64(tuning.minSafetyStep)
		}
	case steadyTemp < lowTarget:
		step = -alpha * float64(lowTarget-steadyTemp) / eff
	default:
		return 0
	}
	if havePower && !haveEff {
		step *= learningPowerGain(step, meanPower)
	}
	if step < 0 {
		step *= 0.5
	}

	if step > float64(tuning.maxLearnStep) {
		step = float64(tuning.maxLearnStep)
	}
	if step < -float64(tuning.maxLearnStep) {
		step = -float64(tuning.maxLearnStep)
	}

	delta := roundFloat(step)
	if steadyTemp <= ceiling && absInt(delta) < tuning.learnStepDeadRPM {
		return 0
	}
	return delta
}

// learningPowerGain 按平均总功耗 (CPU+GPU) 分档，给学习步长一个乘数。
// 阈值针对笔记本工况：CPU ≥ 55W 或 GPU ≥ 80W 通常已经进入"重载"，两者合计 ≥ 90W 就算重载区间；
// 15W 以下多是待机/轻网页，避免为噪音扣转速；15–90W 之间是常见混合负载，走标准增益。
func learningPowerGain(step, meanPower float64) float64 {
	if meanPower <= 0 {
		return 1
	}
	if step > 0 {
		switch {
		case meanPower >= 90:
			return 1.10 // 重载：增速时略激进，压温度优先
		case meanPower <= 15:
			return 0.85 // 轻载：增速时保守，避免风扇噪音
		default:
			return 1
		}
	}
	if step < 0 {
		switch {
		case meanPower >= 90:
			return 0.75 // 重载：降速时保守，防止温度反弹
		case meanPower <= 15:
			return 1.15 // 轻载：降速时激进，降噪
		default:
			return 1
		}
	}
	return 1
}

// LearnSteadyOffset 根据一次稳态观测（温度 + 冷却效率）更新学习偏移。
func LearnSteadyOffset(
	bucketIdx int,
	steadyMeanTemp int,
	localEff float64,
	haveEff bool,
	curve []types.FanCurvePoint,
	prevOffsets []int,
	cfg types.SmartControlConfig,
) ([]int, bool) {
	return LearnSteadyOffsetForUnit(bucketIdx, steadyMeanTemp, localEff, haveEff, curve, prevOffsets, cfg, rawPercentUnit)
}

func LearnSteadyOffsetForUnit(
	bucketIdx int,
	steadyMeanTemp int,
	localEff float64,
	haveEff bool,
	curve []types.FanCurvePoint,
	prevOffsets []int,
	cfg types.SmartControlConfig,
	unit string,
) ([]int, bool) {
	return LearnSteadyOffsetForUnitWithPower(bucketIdx, steadyMeanTemp, 0, false, localEff, haveEff, curve, prevOffsets, cfg, unit)
}

func LearnSteadyOffsetForUnitWithPower(
	bucketIdx int,
	steadyMeanTemp int,
	steadyMeanPower float64,
	havePower bool,
	localEff float64,
	haveEff bool,
	curve []types.FanCurvePoint,
	prevOffsets []int,
	cfg types.SmartControlConfig,
	unit string,
) ([]int, bool) {
	if bucketIdx < 0 || bucketIdx >= len(curve) {
		return prevOffsets, false
	}
	unit = normalizeLearningUnit(unit)

	offsets := make([]int, len(curve))
	for i := range offsets {
		if i < len(prevOffsets) {
			offsets[i] = prevOffsets[i]
		}
	}

	mainDelta := solveLearnStepForUnitWithPower(steadyMeanTemp, localEff, haveEff, cfg, unit, steadyMeanPower, havePower)
	if mainDelta == 0 {
		return offsets, false
	}

	cap := effectiveOffsetCapForUnit(cfg, unit)
	leftMin, rightMax := GetCurveRPMBounds(curve)
	tuning := learningTuningForUnit(unit)

	apply := func(idx, delta int) {
		if idx < 0 || idx >= len(offsets) || delta == 0 {
			return
		}
		offsets[idx] = clampOffsetForPoint(
			offsets[idx]+delta,
			curve[idx].RPM,
			leftMin,
			rightMax,
			cap,
		)
	}
	apply(bucketIdx, mainDelta)
	if biased, updated := constrainOffsetsToLearningBias(offsets, cfg.LearningBias); updated {
		offsets = biased
	}

	smoothOffsetsAroundWithPullLimit(curve, offsets, bucketIdx, cap, leftMin, rightMax, tuning.offsetSmoothPullMax)
	if biased, updated := constrainOffsetsToLearningBias(offsets, cfg.LearningBias); updated {
		offsets = biased
	}
	enforceMonotonicWithOffsets(curve, offsets, cap, leftMin, rightMax)

	changed := false
	for i := range offsets {
		if i >= len(prevOffsets) || offsets[i] != prevOffsets[i] {
			changed = true
			break
		}
	}
	return offsets, changed
}

// roundFloat 四舍五入到最近整数
func roundFloat(v float64) int {
	if v >= 0 {
		return int(v + 0.5)
	}
	return int(v - 0.5)
}

func smoothOffsets(curve []types.FanCurvePoint, offsets []int, cap, leftMin, rightMax int) {
	smoothOffsetsWithPullLimit(curve, offsets, cap, leftMin, rightMax, offsetSmoothPullLimit)
}

func smoothOffsetsWithPullLimit(curve []types.FanCurvePoint, offsets []int, cap, leftMin, rightMax, pullLimit int) {
	limit := min(len(offsets), len(curve))
	if limit < 3 {
		return
	}
	work := make([]int, len(offsets))
	copy(work, offsets)
	for range offsetSmoothPasses {
		copy(work, offsets)
		for i := 1; i < limit-1; i++ {
			target := roundFloat(
				offsetSmoothSelfWeight*float64(offsets[i]) +
					offsetSmoothNeighborWeight*float64(offsets[i-1]) +
					offsetSmoothNeighborWeight*float64(offsets[i+1]),
			)
			pull := target - offsets[i]
			if pull > pullLimit {
				target = offsets[i] + pullLimit
			} else if pull < -pullLimit {
				target = offsets[i] - pullLimit
			}
			work[i] = clampOffsetForPoint(target, curve[i].RPM, leftMin, rightMax, cap)
		}
		copy(offsets, work)
	}
}

func smoothOffsetsAroundWithPullLimit(curve []types.FanCurvePoint, offsets []int, center, cap, leftMin, rightMax, pullLimit int) {
	limit := min(len(offsets), len(curve))
	if limit < 3 {
		return
	}
	lo := max(center-offsetSmoothRadius, 1)
	hi := min(center+offsetSmoothRadius, limit-2)
	if lo > hi {
		return
	}
	work := make([]int, len(offsets))
	copy(work, offsets)
	for range offsetSmoothPasses {
		copy(work, offsets)
		for i := lo; i <= hi; i++ {
			target := roundFloat(
				offsetSmoothSelfWeight*float64(offsets[i]) +
					offsetSmoothNeighborWeight*float64(offsets[i-1]) +
					offsetSmoothNeighborWeight*float64(offsets[i+1]),
			)
			pull := target - offsets[i]
			if pull > pullLimit {
				target = offsets[i] + pullLimit
			} else if pull < -pullLimit {
				target = offsets[i] - pullLimit
			}
			work[i] = clampOffsetForPoint(target, curve[i].RPM, leftMin, rightMax, cap)
		}
		copy(offsets, work)
	}
}

// enforceMonotonicWithOffsets 确保 (RPM_i + Δ_i) 沿 i 非递减；
// 如果某点违反，向上调整 Δ_i 直至单调（仍受 cap 与曲线 RPM 上限约束）。
func enforceMonotonicWithOffsets(curve []types.FanCurvePoint, offsets []int, cap, leftMin, rightMax int) {
	for i := 1; i < len(curve) && i < len(offsets); i++ {
		prevEffective := curve[i-1].RPM + offsets[i-1]
		currEffective := curve[i].RPM + offsets[i]
		if currEffective < prevEffective {
			needed := prevEffective - curve[i].RPM
			offsets[i] = clampOffsetForPoint(needed, curve[i].RPM, leftMin, rightMax, cap)
		}
	}
}

// ResetLearnedState 清空学习相关字段（保留可学习开关本身）。
// 旧字段也清空以保证存档一致。
func ResetLearnedState(cfg types.SmartControlConfig, curve []types.FanCurvePoint) types.SmartControlConfig {
	// rateBucketCount 来自 doc.go (rateBucketMax - rateBucketMin + 1)；
	// 这里仅为保持旧字段长度合法，不再被新算法读取。
	rateLen := rateBucketMax - rateBucketMin + 1
	cfg.LearnedOffsets = make([]int, len(curve))
	cfg.LearnedOffsetsHeat = make([]int, len(curve))
	cfg.LearnedOffsetsCool = make([]int, len(curve))
	cfg.LearnedRateHeat = make([]int, rateLen)
	cfg.LearnedRateCool = make([]int, rateLen)
	return cfg
}
