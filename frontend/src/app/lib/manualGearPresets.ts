import { i18n } from './i18n';

export interface ManualGearPresetLevel {
  level: string;
  rpm: number;
}

export interface ManualGearPreset {
  gear: string;
  colorClass: string;
  borderClass: string;
  bgClass: string;
  levels: ManualGearPresetLevel[];
}

const MANUAL_GEAR_LABEL_KEYS: Record<string, string> = {
  '静音': 'manualGear.gears.quiet',
  '标准': 'manualGear.gears.standard',
  '强劲': 'manualGear.gears.strong',
  '超频': 'manualGear.gears.overclock',
};

const MANUAL_LEVEL_LABEL_KEYS: Record<string, string> = {
  '低': 'manualGear.levels.low',
  '中': 'manualGear.levels.medium',
  '高': 'manualGear.levels.high',
};

export const MANUAL_GEAR_PRESETS: ManualGearPreset[] = [
  {
    gear: '静音',
    colorClass: 'text-emerald-500',
    borderClass: 'border-emerald-500/50',
    bgClass: 'bg-emerald-500/12',
    levels: [
      { level: '低', rpm: 25 },
      { level: '中', rpm: 40 },
      { level: '高', rpm: 55 },
    ],
  },
  {
    gear: '标准',
    colorClass: 'text-blue-500',
    borderClass: 'border-blue-500/50',
    bgClass: 'bg-blue-500/12',
    levels: [
      { level: '低', rpm: 45 },
      { level: '中', rpm: 55 },
      { level: '高', rpm: 65 },
    ],
  },
  {
    gear: '强劲',
    colorClass: 'text-purple-500',
    borderClass: 'border-purple-500/50',
    bgClass: 'bg-purple-500/12',
    levels: [
      { level: '低', rpm: 65 },
      { level: '中', rpm: 75 },
      { level: '高', rpm: 85 },
    ],
  },
  {
    gear: '超频',
    colorClass: 'text-orange-500',
    borderClass: 'border-orange-500/50',
    bgClass: 'bg-orange-500/12',
    levels: [
      { level: '低', rpm: 85 },
      { level: '中', rpm: 95 },
      { level: '高', rpm: 100 },
    ],
  },
];

// BS1 挡位预设（只有4个固定挡位，无子级别）
export const BS1_MANUAL_GEAR_PRESETS: ManualGearPreset[] = [
  {
    gear: '静音',
    colorClass: 'text-emerald-500',
    borderClass: 'border-emerald-500/50',
    bgClass: 'bg-emerald-500/12',
    levels: [{ level: '中', rpm: 40 }],
  },
  {
    gear: '标准',
    colorClass: 'text-blue-500',
    borderClass: 'border-blue-500/50',
    bgClass: 'bg-blue-500/12',
    levels: [{ level: '中', rpm: 55 }],
  },
  {
    gear: '强劲',
    colorClass: 'text-purple-500',
    borderClass: 'border-purple-500/50',
    bgClass: 'bg-purple-500/12',
    levels: [{ level: '中', rpm: 75 }],
  },
  {
    gear: '超频',
    colorClass: 'text-orange-500',
    borderClass: 'border-orange-500/50',
    bgClass: 'bg-orange-500/12',
    levels: [{ level: '中', rpm: 95 }],
  },
];

export const getManualGearHighLevelRpm = (gear?: string | null): number | undefined => {
  if (!gear) return undefined;
  const preset = MANUAL_GEAR_PRESETS.find((item) => item.gear === gear);
  return preset?.levels.find((level) => level.level === '高')?.rpm;
};

// 自定义挡位转速约束（与后端 types.ManualGearMinRPM/MaxRPM 保持一致）
export const MANUAL_GEAR_RPM_MIN = 0;
export const MANUAL_GEAR_RPM_MAX = 100;

export type ManualGearRpmMap = Record<string, Record<string, number>>;

// 根据用户自定义转速生成有效挡位预设（缺省回退到出厂默认）
export const getEffectiveManualGearPresets = (
  custom?: ManualGearRpmMap | null,
): ManualGearPreset[] => {
  if (!custom) return MANUAL_GEAR_PRESETS;
  return MANUAL_GEAR_PRESETS.map((preset) => ({
    ...preset,
    levels: preset.levels.map((lv) => {
      const value = custom[preset.gear]?.[lv.level];
      return {
        ...lv,
        rpm: typeof value === 'number' && value >= MANUAL_GEAR_RPM_MIN ? value : lv.rpm,
      };
    }),
  }));
};

// 校验并补全 12 个自定义转速：限制在 [MIN, MAX] 且按从低到高强制非递减
export const normalizeManualGearRpmMap = (custom?: ManualGearRpmMap | null): ManualGearRpmMap => {
  const out: ManualGearRpmMap = {};
  let prev = 0;
  for (const preset of MANUAL_GEAR_PRESETS) {
    out[preset.gear] = {};
    for (const lv of preset.levels) {
      let v = Math.round(Number(custom?.[preset.gear]?.[lv.level] ?? lv.rpm));
      if (!Number.isFinite(v) || v < MANUAL_GEAR_RPM_MIN || v > MANUAL_GEAR_RPM_MAX) v = lv.rpm;
      if (v < MANUAL_GEAR_RPM_MIN) v = MANUAL_GEAR_RPM_MIN;
      if (v > MANUAL_GEAR_RPM_MAX) v = MANUAL_GEAR_RPM_MAX;
      if (v < prev) v = prev;
      out[preset.gear][lv.level] = v;
      prev = v;
    }
  }
  return out;
};

export const getManualGearLabel = (gear?: string | null): string => {
  if (!gear) return '';
  return i18n.t(MANUAL_GEAR_LABEL_KEYS[gear] || gear);
};

export const getManualLevelLabel = (level?: string | null): string => {
  if (!level) return '';
  return i18n.t(MANUAL_LEVEL_LABEL_KEYS[level] || level);
};

const MAX_GEAR_CODE_TO_RPM: Record<number, number> = {
  // Legacy max-gear codes observed in HID reports. 
  0x2: 65,
  0x3: 65,
  0x4: 85,
  0x6: 100,
  // Compatibility for firmware variants that use full gear codes.
  0xA: 65,
  0xC: 85,
  0xE: 100,
};

export interface ReportedMaxRpmInfo {
  rpm?: number;
  codeHex?: string;
  source: 'gearSettings' | 'maxGearText' | 'unknown';
}

export const getReportedMaxRpm = (
  gearSettings?: number | null,
  maxGearText?: string | null,
): ReportedMaxRpmInfo => {
  if (typeof gearSettings === 'number') {
    const maxGearCode = (gearSettings >> 4) & 0x0f;
    const mapped = MAX_GEAR_CODE_TO_RPM[maxGearCode];
    if (mapped) {
      return { rpm: mapped, source: 'gearSettings' };
    }
    return { codeHex: `0x${maxGearCode.toString(16).toUpperCase()}`, source: 'gearSettings' };
  }

  const textMapped = getManualGearHighLevelRpm(maxGearText);
  if (textMapped) {
    return { rpm: textMapped, source: 'maxGearText' };
  }

  return { source: 'unknown' };
};
