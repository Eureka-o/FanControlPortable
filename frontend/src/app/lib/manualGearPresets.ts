import { i18n } from './i18n';
import { FAN_SPEED_UNIT_PERCENT, FAN_SPEED_UNIT_RPM, type FanSpeedUnit } from './fan-speed';
import type { FlyDigiRuntimeCapability } from '../types/app';

interface ManualGearCapabilityCarrier {
  supportsManualGears?: boolean;
  supportsSetSpeed?: boolean;
  supportsCustomSpeed?: boolean;
}

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

export const MANUAL_GEAR_RPM_PRESETS: ManualGearPreset[] = [
  {
    gear: '静音',
    colorClass: 'text-emerald-500',
    borderClass: 'border-emerald-500/50',
    bgClass: 'bg-emerald-500/12',
    levels: [
      { level: '低', rpm: 1300 },
      { level: '中', rpm: 1700 },
      { level: '高', rpm: 1900 },
    ],
  },
  {
    gear: '标准',
    colorClass: 'text-blue-500',
    borderClass: 'border-blue-500/50',
    bgClass: 'bg-blue-500/12',
    levels: [
      { level: '低', rpm: 2100 },
      { level: '中', rpm: 2400 },
      { level: '高', rpm: 2700 },
    ],
  },
  {
    gear: '强劲',
    colorClass: 'text-purple-500',
    borderClass: 'border-purple-500/50',
    bgClass: 'bg-purple-500/12',
    levels: [
      { level: '低', rpm: 2800 },
      { level: '中', rpm: 3000 },
      { level: '高', rpm: 3300 },
    ],
  },
  {
    gear: '超频',
    colorClass: 'text-orange-500',
    borderClass: 'border-orange-500/50',
    bgClass: 'bg-orange-500/12',
    levels: [
      { level: '低', rpm: 3500 },
      { level: '中', rpm: 3700 },
      { level: '高', rpm: 4000 },
    ],
  },
];

// BS1 挡位预设：仅 4 个固定挡位，无子级别。
// 黑鲨派生值：唯一来源是后端（deviceproto 标定表文件）。
export interface CoreBlackSharkGearRpms {
  minRpm: number;
  maxRpm: number;
  gears: { gear: number; levels: number[] }[];
}

let coreBlackSharkRpms: CoreBlackSharkGearRpms | null = null;

/** 幂等：由 `apiService.getBlackSharkManualGearPresets()` 调一次即可。 */
export const setBlackSharkGearRpmsFromCore = (payload?: CoreBlackSharkGearRpms | null): void => {
  coreBlackSharkRpms =
    payload && Array.isArray(payload.gears) && payload.gears.length > 0 ? payload : null;
};

/** 黑鲨可达到的转速区间（来自后端；未加载时为 null，调用方回退到通用量程）。 */
export const getBlackSharkCoreRpmRange = (): { min: number; max: number } | null =>
  coreBlackSharkRpms
    ? { min: coreBlackSharkRpms.minRpm, max: coreBlackSharkRpms.maxRpm }
    : null;

/** 黑鲨四档的展示信息；转速一律来自 `coreBlackSharkRpms`，不要往这里写数值。 */
const BLACKSHARK_GEAR_PRESENTATION: Omit<ManualGearPreset, 'levels'>[] = [
  { gear: '静音', colorClass: 'text-emerald-500', borderClass: 'border-emerald-500/50', bgClass: 'bg-emerald-500/12' },
  { gear: '标准', colorClass: 'text-blue-500', borderClass: 'border-blue-500/50', bgClass: 'bg-blue-500/12' },
  { gear: '强劲', colorClass: 'text-purple-500', borderClass: 'border-purple-500/50', bgClass: 'bg-purple-500/12' },
  { gear: '超频', colorClass: 'text-orange-500', borderClass: 'border-orange-500/50', bgClass: 'bg-orange-500/12' },
];

const BLACKSHARK_LEVEL_NAMES = ['低', '中', '高'];

const blackSharkPresetsFromCore = (): ManualGearPreset[] =>
  BLACKSHARK_GEAR_PRESENTATION.map((presentation, gearIndex) => ({
    ...presentation,
    levels: (coreBlackSharkRpms?.gears[gearIndex]?.levels ?? []).map((rpm, levelIndex) => ({
      level: BLACKSHARK_LEVEL_NAMES[levelIndex] ?? String(levelIndex + 1),
      rpm,
    })),
  }));

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
export const MANUAL_GEAR_LEGACY_RPM_MIN = 800;
export const MANUAL_GEAR_LEGACY_RPM_MAX = 4500;

export type ManualGearRpmMap = Record<string, Record<string, number>>;

// 黑鲨 RPM 量程来自后端（1410..4740）；800..4500 是通用回退，直接用会把自定义值静默判为非法。
const manualGearRangeForUnit = (unit: FanSpeedUnit, blackShark = false) => {
  if (unit === FAN_SPEED_UNIT_RPM) {
    const core = blackShark ? getBlackSharkCoreRpmRange() : null;
    return core ?? { min: MANUAL_GEAR_LEGACY_RPM_MIN, max: MANUAL_GEAR_LEGACY_RPM_MAX };
  }
  return { min: MANUAL_GEAR_RPM_MIN, max: MANUAL_GEAR_RPM_MAX };
};

export const getManualGearValueRange = manualGearRangeForUnit;

const isManualGearValueValidForUnit = (value: unknown, unit: FanSpeedUnit, blackShark = false) => {
  if (typeof value !== 'number' || !Number.isFinite(value)) return false;
  const range = manualGearRangeForUnit(unit, blackShark);
  return value >= range.min && value <= range.max;
};

export const getManualGearDefaultPresets = (
  unit: FanSpeedUnit = FAN_SPEED_UNIT_PERCENT,
  // 默认 false：只有显式要求黑鲨时才换那套，其它设备调用点不变。
  blackShark = false,
): ManualGearPreset[] => {
  if (blackShark && unit === FAN_SPEED_UNIT_RPM) {
    // 黑鲨数值全部来自后端标定表；未加载则是空 levels，不编数。
    return blackSharkPresetsFromCore();
  }
  return unit === FAN_SPEED_UNIT_RPM ? MANUAL_GEAR_RPM_PRESETS : MANUAL_GEAR_PRESETS;
};

// 根据用户自定义转速生成有效挡位预设（缺省回退到出厂默认）
export const getEffectiveManualGearPresets = (
  custom?: ManualGearRpmMap | null,
  unit: FanSpeedUnit = FAN_SPEED_UNIT_PERCENT,
  // 重置按钮也经这里生成默认值；必须带上黑鲨标记，否则黑鲨会恢复成飞智默认值。
  blackShark = false,
): ManualGearPreset[] => {
  const defaults = getManualGearDefaultPresets(unit, blackShark);
  if (!custom) return defaults;
  return defaults.map((preset) => ({
    ...preset,
    levels: preset.levels.map((lv) => {
      const value = custom[preset.gear]?.[lv.level];
      return {
        ...lv,
        rpm: isManualGearValueValidForUnit(value, unit, blackShark) ? value : lv.rpm,
      };
    }),
  }));
};

// 校验并补全 12 个自定义转速：限制在 [MIN, MAX] 且按从低到高强制非递减
export const normalizeManualGearRpmMap = (
  custom?: ManualGearRpmMap | null,
  minValue = MANUAL_GEAR_RPM_MIN,
  maxValue = MANUAL_GEAR_RPM_MAX,
  unit: FanSpeedUnit = FAN_SPEED_UNIT_PERCENT,
  // 黑鲨：默认值与量程都取后端派生的那套（1410..4740）。
  blackShark = false,
): ManualGearRpmMap => {
  const out: ManualGearRpmMap = {};
  const defaults = getManualGearDefaultPresets(unit, blackShark);
  const manualRange = manualGearRangeForUnit(unit, blackShark);
  const minAllowed = Math.max(minValue, manualRange.min);
  const maxAllowed = Math.max(minAllowed, Math.min(maxValue, manualRange.max));
  let prev = minAllowed;
  for (const preset of defaults) {
    out[preset.gear] = {};
    for (const lv of preset.levels) {
      let v = Math.round(Number(custom?.[preset.gear]?.[lv.level] ?? lv.rpm));
      if (!Number.isFinite(v) || v < minAllowed || v > maxAllowed) v = Math.max(minAllowed, Math.min(maxAllowed, lv.rpm));
      if (v < minAllowed) v = minAllowed;
      if (v > maxAllowed) v = maxAllowed;
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

export const supportsManualGearsFromCapabilities = (
  capabilities?: ManualGearCapabilityCarrier | null,
): boolean => {
  if (!capabilities) {
    return true;
  }
  // Older saved profiles only had set-speed/custom-speed flags. Treat those as
  // manual-gear compatible so legacy WiFi/serial compatibility profiles keep working.
  return Boolean(
    capabilities.supportsManualGears
    || capabilities.supportsSetSpeed
    || capabilities.supportsCustomSpeed,
  );
};

const FLYDIGI_MAX_GEAR_CODE_TO_INDEX: Record<number, number> = {
  0x2: 1,
  0x4: 2,
  0x6: 3,
};

const FLYDIGI_FULL_GEAR_CODE_TO_INDEX: Record<number, number> = {
  0x8: 0,
  0xA: 1,
  0xC: 2,
  0xE: 3,
};

const FLYDIGI_GEAR_INDEX_TO_RPM = [1900, 2700, 3300, 4000];
const FLYDIGI_GEAR_INDEX_TO_LABEL = ['静音', '标准', '强劲', '超频'];

const capabilityMaxRpm = (capability?: FlyDigiRuntimeCapability | null) => {
  const value = Number(capability?.maxRpm || 0);
  return Number.isFinite(value) && value > 0 ? value : undefined;
};

export interface ReportedMaxRpmInfo {
  rpm?: number;
  maxGearIndex?: number;
  maxGearLabel?: string;
  codeHex?: string;
  source: 'runtimeCapability' | 'gearSettings' | 'maxGearText' | 'unknown';
}

export const getReportedMaxRpm = (
  gearSettings?: number | null,
  maxGearText?: string | null,
  runtimeCapability?: FlyDigiRuntimeCapability | null,
): ReportedMaxRpmInfo => {
  const capRpm = capabilityMaxRpm(runtimeCapability);
  if (capRpm) {
    return {
      rpm: capRpm,
      maxGearIndex: runtimeCapability?.maxGearIndex,
      maxGearLabel: runtimeCapability?.maxGearLabel,
      source: 'runtimeCapability',
    };
  }

  if (typeof gearSettings === 'number') {
    const maxGearCode = (gearSettings >> 4) & 0x0f;
    const mappedIndex = FLYDIGI_MAX_GEAR_CODE_TO_INDEX[maxGearCode] ?? FLYDIGI_FULL_GEAR_CODE_TO_INDEX[maxGearCode];
    if (typeof mappedIndex === 'number') {
      return {
        rpm: FLYDIGI_GEAR_INDEX_TO_RPM[mappedIndex],
        maxGearIndex: mappedIndex,
        maxGearLabel: FLYDIGI_GEAR_INDEX_TO_LABEL[mappedIndex],
        source: 'gearSettings',
      };
    }
    return { codeHex: `0x${maxGearCode.toString(16).toUpperCase()}`, source: 'gearSettings' };
  }

  const textIndex = FLYDIGI_GEAR_INDEX_TO_LABEL.indexOf(maxGearText || '');
  if (textIndex >= 0) {
    return {
      rpm: FLYDIGI_GEAR_INDEX_TO_RPM[textIndex],
      maxGearIndex: textIndex,
      maxGearLabel: FLYDIGI_GEAR_INDEX_TO_LABEL[textIndex],
      source: 'maxGearText',
    };
  }

  return { source: 'unknown' };
};

export const getFlyDigiRuntimeCapability = (
  fanData?: { gearSettings?: number | null; maxGear?: string | null; flyDigiCapability?: FlyDigiRuntimeCapability | null } | null,
  deviceSettings?: { flyDigiCapability?: FlyDigiRuntimeCapability | null } | null,
): FlyDigiRuntimeCapability | null => {
  if (fanData?.flyDigiCapability?.available) {
    return fanData.flyDigiCapability;
  }
  if (deviceSettings?.flyDigiCapability?.available) {
    return deviceSettings.flyDigiCapability;
  }
  const reported = getReportedMaxRpm(fanData?.gearSettings, fanData?.maxGear, fanData?.flyDigiCapability || deviceSettings?.flyDigiCapability);
  if (!reported.rpm || typeof reported.maxGearIndex !== 'number') {
    return null;
  }
  return {
    available: true,
    gearSettings: typeof fanData?.gearSettings === 'number' ? fanData.gearSettings : 0,
    maxGearIndex: reported.maxGearIndex,
    maxGearLabel: reported.maxGearLabel,
    maxRpm: reported.rpm,
    source: reported.source,
  };
};

export const isManualGearAllowedForFlyDigi = (
  gear: string,
  capability?: FlyDigiRuntimeCapability | null,
) => {
  if (!capability?.available || typeof capability.maxGearIndex !== 'number') {
    return true;
  }
  const gearIndex = FLYDIGI_GEAR_INDEX_TO_LABEL.indexOf(gear);
  return gearIndex >= 0 && gearIndex <= capability.maxGearIndex;
};

