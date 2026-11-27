'use client';

import React, { useState, useEffect, useCallback, memo, useMemo, useRef } from 'react';
import dynamic from 'next/dynamic';
import { AnimatePresence, m } from 'framer-motion';
import {
  RotateCw,
  Check,
  X,
  History,
  Info,
  Spline,
  Plus,
  Trash2,
  Clipboard,
  Download,
  Sparkles,
  Upload,
  Pencil,
  Radar,
  Settings2,
  GripVertical,
  ArrowUp,
  ArrowDown,
  AudioLines,
  Ear,
  TriangleAlert,
  Copy,
  Undo2,
  Magnet,
  Minus,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Input } from '@/components/ui/input';
import { apiService } from '../services/api';
import { useTemperatureHistory } from '../hooks/useTemperatureHistory';
import { HISTORY_SERIES_ORDER, HISTORY_TIMELINE_EVENT_ORDER, useHistoryDisplayPreferences } from '../hooks/useHistoryDisplayPreferences';
import { useLocale } from '../lib/i18n';
import { getFanSpeedUnit, getFanSpeedRange, getFanSpeedTicks, fanSpeedUnitLabel } from '../lib/fan-speed';
import { downsampleHistoryPoints, HISTORY_RETENTION_HOUR_OPTIONS, type HistorySeriesKey, type TemperatureHistoryPoint } from '../lib/temperature-history';
import {
  cancelCurveEditorSwitch,
  createCurveEditorSession,
  markCurveEditorDirty,
  markCurveEditorSaved,
  requestCurveProfileSwitch,
  requestCurveTabSwitch,
  resolveCurveEditorSwitch,
} from '../lib/curve-editor-session';
import {
  cloneCurvePoints,
  curvesEqual,
  findClosestCurvePointIndex,
  pushCurveUndoSnapshot,
  shiftCurveSpeeds,
  snapCurveSpeedValue,
  syncCurveSpeedAtIndexWithAffected,
  type CurveChartPixelBounds,
} from '../lib/curve-editor-helpers';
import { useAppStore, type CurveFocusTarget } from '../store/app-store';
import { types } from '../../../wailsjs/go/models';
import { ClipboardSetText } from '../../../wailsjs/runtime/runtime';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import NoiseDiagnostic from './NoiseDiagnostic';
import AxisNoiseScan from './AxisNoiseScan';
import {
  ToggleSwitch,
  Button,
  Badge,
  Select,
  Slider,
  NumberInput,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './ui/index';
import clsx from 'clsx';
import {
  getManualGearDefaultPresets,
  getManualGearValueRange,
  getEffectiveManualGearPresets,
  getFlyDigiRuntimeCapability,
  normalizeManualGearRpmMap,
  // 黑鲨派生值由后端给出（标定表是唯一所有者），取到后注入这里。
  setBlackSharkGearRpmsFromCore,
  isManualGearAllowedForFlyDigi,
  getManualGearLabel,
  getManualLevelLabel,
  supportsManualGearsFromCapabilities,
  type ManualGearRpmMap,
} from '../lib/manualGearPresets';
import {
  FAN_CURVE_AXIS_MIN_SPEED,
  TIMELINE_EVENT_COLORS,
  formatPowerValue,
  formatSpeedValue,
} from './fan-curve-chart-shared';
// 仅类型引用（编译期整体擦除）：不会把 recharts 图表 chunk 拉回主包。
import type { HistorySeriesMeta, HistoryStatisticsData, HistoryStatisticsMarker } from './fan-curve-charts';

const FAN_CURVE_MIN_TEMP = 20;
const FAN_CURVE_MAX_TEMP = 110;
const FAN_CURVE_TEMP_STEP = 5;
const DEFAULT_CURVE_LENGTH = ((FAN_CURVE_MAX_TEMP - FAN_CURVE_MIN_TEMP) / FAN_CURVE_TEMP_STEP) + 1;
const RPM_RECOMMENDED_MIN = 1000;
const RPM_HARD_FLOOR_WARN = 800;
const FAN_CURVE_TEMPERATURE_TICKS = Array.from({ length: DEFAULT_CURVE_LENGTH }, (_, i) => FAN_CURVE_MIN_TEMP + i * FAN_CURVE_TEMP_STEP);

// 黑鲨四个可拖点的设备工作范围是 10–100℃；飞智及通用曲线仍使用 20–110℃。
const BLACKSHARK_CURVE_AXIS_MIN_TEMP = 10;
const BLACKSHARK_CURVE_AXIS_MAX_TEMP = 100;
const BLACKSHARK_CURVE_AXIS_TEMPERATURE_TICKS = Array.from(
  { length: (BLACKSHARK_CURVE_AXIS_MAX_TEMP - BLACKSHARK_CURVE_AXIS_MIN_TEMP) / FAN_CURVE_TEMP_STEP + 1 },
  (_, i) => BLACKSHARK_CURVE_AXIS_MIN_TEMP + i * FAN_CURVE_TEMP_STEP,
);

// 纵轴下端（FAN_CURVE_AXIS_MIN_SPEED）挪到了 ./fan-curve-chart-shared：
// `YAxis`（fan-curve-charts）与这里的拖拽 Y 映射必须共用同一个数，不能各写一份。

const DEFAULT_FAN_CURVE: types.FanCurvePoint[] = [
  { temperature: 20, rpm: 0 }, { temperature: 25, rpm: 10 }, { temperature: 30, rpm: 18 }, { temperature: 35, rpm: 24 },
  { temperature: 40, rpm: 30 }, { temperature: 45, rpm: 36 }, { temperature: 50, rpm: 42 }, { temperature: 55, rpm: 48 },
  { temperature: 60, rpm: 55 }, { temperature: 65, rpm: 62 }, { temperature: 70, rpm: 70 }, { temperature: 75, rpm: 78 },
  { temperature: 80, rpm: 86 }, { temperature: 85, rpm: 92 }, { temperature: 90, rpm: 96 }, { temperature: 95, rpm: 100 },
  { temperature: 100, rpm: 100 }, { temperature: 105, rpm: 100 }, { temperature: 110, rpm: 100 },
];
const DEFAULT_RPM_FAN_CURVE: types.FanCurvePoint[] = [
	{ temperature: 20, rpm: 1000 }, { temperature: 25, rpm: 1000 },
  { temperature: 30, rpm: 1000 }, { temperature: 35, rpm: 1200 }, { temperature: 40, rpm: 1400 }, { temperature: 45, rpm: 1600 },
  { temperature: 50, rpm: 1800 }, { temperature: 55, rpm: 2000 }, { temperature: 60, rpm: 2300 }, { temperature: 65, rpm: 2600 },
  { temperature: 70, rpm: 2900 }, { temperature: 75, rpm: 3200 }, { temperature: 80, rpm: 3500 }, { temperature: 85, rpm: 3800 },
  { temperature: 90, rpm: 4000 }, { temperature: 95, rpm: 4000 }, { temperature: 100, rpm: 4000 }, { temperature: 105, rpm: 4000 },
  { temperature: 110, rpm: 4000 },
];
const SMART_CONTROL_TARGET_TEMP_MIN = 45;
const SMART_CONTROL_TARGET_TEMP_MAX = 90;
const FOCUS_SCROLL_STABLE_FRAMES = 8;
const FOCUS_SCROLL_TIMEOUT_MS = 1_200;
const FOCUS_SCROLL_EPSILON_PX = 0.5;
type CurveProfile = { id: string; name: string; curve: types.FanCurvePoint[] };

const LEARNING_BIAS_OPTIONS = [
  { value: 'balanced', labelKey: 'fanCurve.learning.biasOptions.balanced.label', descriptionKey: 'fanCurve.learning.biasOptions.balanced.description' },
  { value: 'cooling', labelKey: 'fanCurve.learning.biasOptions.cooling.label', descriptionKey: 'fanCurve.learning.biasOptions.cooling.description' },
  { value: 'quiet', labelKey: 'fanCurve.learning.biasOptions.quiet.label', descriptionKey: 'fanCurve.learning.biasOptions.quiet.description' },
];

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function normalizeLearningBias(value: unknown): string {
  return LEARNING_BIAS_OPTIONS.some((option) => option.value === value) ? String(value) : 'balanced';
}

function constrainOffsetByLearningBias(offset: number, learningBias: string) {
  if (learningBias === 'cooling' && offset < 0) return 0;
  if (learningBias === 'quiet' && offset > 0) return 0;
  return offset;
}

function normalizeTargetTemp(value: number) {
  return Math.max(SMART_CONTROL_TARGET_TEMP_MIN, Math.min(SMART_CONTROL_TARGET_TEMP_MAX, Math.round(value)));
}

function normalizeSpeedValue(value: number, minSpeed: number, maxSpeed: number) {
  return Math.max(minSpeed, Math.min(maxSpeed, Math.round(value)));
}

function learnedOffsetTicksToPercent(value: number) {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.round(value) / 10;
}

function learnedOffsetForDisplay(value: number, speedUnit: string) {
  return speedUnit === 'percent' ? learnedOffsetTicksToPercent(value) : value;
}

const MIN_VALID_POWER_WATTS = 0.05;

function getAvailableTotalPowerWatts(point: TemperatureHistoryPoint) {
  const rawCpu = Number(point.cpuPowerWatts || 0);
  const rawGpu = Number(point.gpuPowerWatts || 0);
  const cpu = Number.isFinite(rawCpu) && rawCpu >= MIN_VALID_POWER_WATTS ? rawCpu : 0;
  const gpu = Number.isFinite(rawGpu) && rawGpu >= MIN_VALID_POWER_WATTS ? rawGpu : 0;
  const total = cpu + gpu;
  return total >= MIN_VALID_POWER_WATTS ? Math.round(total * 10) / 10 : undefined;
}

// formatSpeedValue / formatPowerValue / HistoryChartPoint（图表数据点类型）挪到了
// ./fan-curve-chart-shared 与 ./fan-curve-charts：图表分包与主包共用同一份实现。

const CPU_TEMP_STROKE = 'var(--chart-cpu-temperature)';
const GPU_TEMP_STROKE = 'var(--chart-gpu-temperature)';
const FAN_SPEED_STROKE = 'var(--chart-fan-speed)';
const CPU_POWER_STROKE = 'var(--chart-cpu-power)';
const GPU_POWER_STROKE = 'var(--chart-gpu-power)';
const TOTAL_POWER_STROKE = 'var(--chart-primary)';

const HISTORY_SERIES_DATA_KEY: Record<HistorySeriesKey, 'cpuTemp' | 'gpuTemp' | 'fanRpm' | 'cpuPowerWatts' | 'gpuPowerWatts' | 'totalPowerWatts'> = {
  cpu: 'cpuTemp',
  gpu: 'gpuTemp',
  fan: 'fanRpm',
  cpuPower: 'cpuPowerWatts',
  gpuPower: 'gpuPowerWatts',
  totalPower: 'totalPowerWatts',
};

const HISTORY_SERIES_AXIS: Record<HistorySeriesKey, 'temp' | 'fan' | 'power'> = {
  cpu: 'temp',
  gpu: 'temp',
  fan: 'fan',
  cpuPower: 'power',
  gpuPower: 'power',
  totalPower: 'power',
};

function normalizeCurvePoint(point: types.FanCurvePoint, minSpeed: number, maxSpeed: number): types.FanCurvePoint {
  return { temperature: Math.round(point.temperature), rpm: normalizeSpeedValue(point.rpm, minSpeed, maxSpeed) };
}

function interpolateCurveSpeed(curve: types.FanCurvePoint[], temperature: number, fallbackCurve: types.FanCurvePoint[], minSpeed: number, maxSpeed: number) {
  if (curve.length === 0) {
    return fallbackCurve.find((point) => point.temperature === temperature)?.rpm ?? minSpeed;
  }

  if (temperature <= curve[0].temperature) {
    return curve[0].rpm;
  }

  const last = curve[curve.length - 1];
  if (temperature >= last.temperature) {
    return last.rpm;
  }

  for (let index = 0; index < curve.length - 1; index += 1) {
    const left = curve[index];
    const right = curve[index + 1];
    if (temperature === left.temperature) {
      return left.rpm;
    }
    if (temperature > left.temperature && temperature < right.temperature) {
      const ratio = (temperature - left.temperature) / (right.temperature - left.temperature);
      return normalizeSpeedValue(left.rpm + (right.rpm - left.rpm) * ratio, minSpeed, maxSpeed);
    }
  }

  return last.rpm;
}

function normalizeFanCurve(curve: types.FanCurvePoint[] | null | undefined, minSpeed = 0, maxSpeed = 100, fallbackCurve = DEFAULT_FAN_CURVE): types.FanCurvePoint[] {
  const source = Array.isArray(curve)
    ? curve
      .map((point) => normalizeCurvePoint(point, minSpeed, maxSpeed))
      .filter((point) => point.temperature >= FAN_CURVE_MIN_TEMP && point.temperature <= FAN_CURVE_MAX_TEMP)
      .sort((left, right) => left.temperature - right.temperature)
    : [];

  const unique = source.reduce<types.FanCurvePoint[]>((points, point) => {
    const previous = points[points.length - 1];
    if (previous && previous.temperature === point.temperature) {
      points[points.length - 1] = point;
    } else {
      points.push(point);
    }
    return points;
  }, []);

  const base = unique.length > 0 ? unique : fallbackCurve;

  // 保留曲线自己的温度格点，不一律重采样到默认网格：黑鲨是设备侧的 4 点曲线，
  // 出厂默认落在 20/40/60/80（见 deviceproto.BlackSharkFactoryGearConfig），但温度可以拖动，格点不固定。
  const usesDefaultGrid =
    base.length === FAN_CURVE_TEMPERATURE_TICKS.length &&
    base.every((point, index) => point.temperature === FAN_CURVE_TEMPERATURE_TICKS[index]);
  if (!usesDefaultGrid) {
    return base.map((point) => ({ temperature: point.temperature, rpm: point.rpm }));
  }

  return FAN_CURVE_TEMPERATURE_TICKS.map((temperature) => ({
    temperature,
    rpm: interpolateCurveSpeed(base, temperature, fallbackCurve, minSpeed, maxSpeed),
  }));
}

function syncCurveSpeedAtIndex(
  curve: types.FanCurvePoint[],
  index: number,
  targetSpeed: number,
  minSpeed: number,
  maxSpeed: number,
) {
  return syncCurveSpeedAtIndexWithAffected(curve, index, targetSpeed, minSpeed, maxSpeed);
}

/**
 * syncCurveTemperatureAtIndex 把第 index 个点横向移到目标温度（℃）。
 *
 * 黑鲨那 4 个点的温度是可拖的：出厂默认落在 20/40/60/80，但设备接受取值域内的任意整数温度
 * （取值域由后端 deviceproto 拥有，经 getBlackSharkCurveTempRange 取回）。
 *
 * 与纵向不同，这里不做"推挤邻居"：温度是横轴上的位置，越界直接夹住即可；
 * 只需保证仍然严格递增（相邻至少差 1 ℃），否则点的先后顺序会翻转、曲线语义就乱了。
 */
function syncCurveTemperatureAtIndex(
  curve: types.FanCurvePoint[],
  index: number,
  targetTemperature: number,
  minTemperature: number,
  maxTemperature: number,
) {
  const currentPoint = curve[index];
  if (!currentPoint) {
    return { curve, changed: false };
  }

  const previous = curve[index - 1];
  const following = curve[index + 1];
  const lowerBound = Math.max(minTemperature, previous ? previous.temperature + 1 : minTemperature);
  const upperBound = Math.min(maxTemperature, following ? following.temperature - 1 : maxTemperature);
  if (lowerBound > upperBound) {
    return { curve, changed: false };
  }

  const normalized = Math.max(lowerBound, Math.min(upperBound, Math.round(targetTemperature)));
  if (currentPoint.temperature === normalized) {
    return { curve, changed: false };
  }

  const nextCurve = [...curve];
  nextCurve[index] = { ...currentPoint, temperature: normalized };
  return { curve: nextCurve, changed: true };
}

interface FanCurveProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  fanData?: types.FanData | null;
  temperature?: types.TemperatureData | null;
  runtimeDeviceProfile?: types.DeviceProfile | null;
  runtimeDeviceCapabilities?: types.DeviceCapabilities | null;
  deviceModel: string | null;
  focusTarget: CurveFocusTarget | null;
  onFocusHandled: () => void;
  historyOnly?: boolean;
}

// formatHistoryTime（时间轴 tick 用）只服务于图表，挪到 ./fan-curve-charts。

function formatHistoryDateTime(timestamp: number, locale: string) {
  return new Date(timestamp).toLocaleTimeString(locale, {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

function formatHistoryDuration(
  startTimestamp: number,
  endTimestamp: number,
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  const durationMs = Math.max(0, endTimestamp - startTimestamp);
  if (durationMs < 60_000) {
    return t('fanCurve.history.duration.ltOneMinute');
  }
  const totalMinutes = Math.round(durationMs / 60_000);
  if (totalMinutes < 60) {
    return t('fanCurve.history.duration.minutes', { count: totalMinutes });
  }
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return minutes > 0
    ? t('fanCurve.history.duration.hoursAndMinutes', { hours, minutes })
    : t('fanCurve.history.duration.hours', { hours });
}

/* ── Shared chart grid geometry observer (independent of telemetry & drags) ── */

type ChartGridGeometry = { left: number; top: number; width: number; height: number };

function useChartGridGeometry(chartRef: React.RefObject<HTMLDivElement | null>): ChartGridGeometry | null {
  const [geom, setGeom] = useState<ChartGridGeometry | null>(null);

  useEffect(() => {
    const root = chartRef.current;
    if (!root) {
      setGeom(null);
      return;
    }

    let rafId = 0;
    let debounceTimer: number | null = null;
    let resizeObserver: ResizeObserver | null = null;
    let mutationObserver: MutationObserver | null = null;

    const measureNow = () => {
      const grid = root.querySelector('.recharts-cartesian-grid');
      const container = root.querySelector('.recharts-responsive-container');
      if (!grid || !container) {
        return false;
      }
      const g = grid.getBoundingClientRect();
      const c = container.getBoundingClientRect();
      if (g.width <= 0 || g.height <= 0) {
        return false;
      }
      const next: ChartGridGeometry = {
        left: g.left - c.left,
        top: g.top - c.top,
        width: g.width,
        height: g.height,
      };
      setGeom((prev) => {
        if (
          prev &&
          Math.abs(prev.left - next.left) <= 0.5 &&
          Math.abs(prev.top - next.top) <= 0.5 &&
          Math.abs(prev.width - next.width) <= 0.5 &&
          Math.abs(prev.height - next.height) <= 0.5
        ) {
          return prev;
        }
        return next;
      });
      return true;
    };

    const scheduleDebouncedMeasure = () => {
      if (debounceTimer !== null) {
        window.clearTimeout(debounceTimer);
      }
      debounceTimer = window.setTimeout(() => {
        debounceTimer = null;
        window.cancelAnimationFrame(rafId);
        rafId = window.requestAnimationFrame(() => {
          measureNow();
        });
      }, 80);
    };

    if (!measureNow()) {
      rafId = window.requestAnimationFrame(() => {
        if (measureNow()) {
          mutationObserver?.disconnect();
          mutationObserver = null;
        }
      });
      if (typeof MutationObserver !== 'undefined') {
        mutationObserver = new MutationObserver(() => {
          if (measureNow()) {
            mutationObserver?.disconnect();
            mutationObserver = null;
          }
        });
        mutationObserver.observe(root, { childList: true, subtree: true });
      }
    }

    window.addEventListener('resize', scheduleDebouncedMeasure);
    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(scheduleDebouncedMeasure);
      resizeObserver.observe(root);
    }

    return () => {
      window.cancelAnimationFrame(rafId);
      if (debounceTimer !== null) {
        window.clearTimeout(debounceTimer);
      }
      window.removeEventListener('resize', scheduleDebouncedMeasure);
      resizeObserver?.disconnect();
      mutationObserver?.disconnect();
    };
  }, [chartRef]);

  return geom;
}

/* ── Temperature indicator overlay (memo, doesn't re-render chart) ── */

const TemperatureIndicator = memo(function TemperatureIndicator({
  temperature,
  chartRef,
  temperatureRange,
  curvePoints,
  speedMax,
}: {
  temperature: number | null;
  chartRef: React.RefObject<HTMLDivElement | null>;
  temperatureRange: { min: number; max: number };
  curvePoints?: Array<{ temperature: number; rpm: number }>;
  speedMax?: number;
}) {
  const { t } = useTranslation();
  const geom = useChartGridGeometry(chartRef);

  const position = useMemo(() => {
    if (!geom || temperature === null) return null;
    const span = Math.max(1, temperatureRange.max - temperatureRange.min);
    const tempPercent = Math.min(1, Math.max(0, (temperature - temperatureRange.min) / span));
    return {
      x: geom.left + tempPercent * geom.width,
      top: geom.top,
      height: geom.height,
    };
  }, [geom, temperature, temperatureRange.max, temperatureRange.min]);

  const operatingY = useMemo(() => {
    if (!position || temperature === null || !curvePoints || curvePoints.length === 0 || !speedMax) {
      return null;
    }
    const sorted = [...curvePoints].sort((a, b) => a.temperature - b.temperature);
    let activeRpm = sorted[0].rpm;
    if (temperature <= sorted[0].temperature) {
      activeRpm = sorted[0].rpm;
    } else if (temperature >= sorted[sorted.length - 1].temperature) {
      activeRpm = sorted[sorted.length - 1].rpm;
    } else {
      for (let i = 0; i < sorted.length - 1; i++) {
        const a = sorted[i];
        const b = sorted[i + 1];
        if (temperature >= a.temperature && temperature <= b.temperature) {
          const span = Math.max(1, b.temperature - a.temperature);
          activeRpm = a.rpm + ((temperature - a.temperature) / span) * (b.rpm - a.rpm);
          break;
        }
      }
    }
    return position.top + (1 - Math.min(1, Math.max(0, activeRpm / Math.max(1, speedMax)))) * position.height;
  }, [curvePoints, position, speedMax, temperature]);

  if (!position || temperature === null) return null;

  return (
    <svg className="absolute inset-0 pointer-events-none overflow-visible" style={{ width: '100%', height: '100%' }}>
      <line x1={position.x} y1={position.top} x2={position.x} y2={position.top + position.height} stroke="var(--chart-temperature-indicator)" strokeWidth={1.75} strokeDasharray="4 4" opacity={0.85} />
      {operatingY !== null && (
        <>
          <circle cx={position.x} cy={operatingY} r={9} fill="var(--chart-temperature-indicator)" opacity={0.2} />
          <circle cx={position.x} cy={operatingY} r={4.5} fill="var(--chart-temperature-indicator)" stroke="var(--card)" strokeWidth={2} />
        </>
      )}
      <rect x={position.x - 48} y={position.top - 24} width={96} height={21} rx={10.5} fill="var(--chart-temperature-indicator)" stroke="var(--card)" strokeWidth={1.5} />
      <text x={position.x} y={position.top - 10} textAnchor="middle" fill="white" fontSize={11} fontWeight={600}>{t('fanCurve.chart.currentTemperature', { temperature })}</text>
    </svg>
  );
});

const LiveTemperatureIndicator = memo(function LiveTemperatureIndicator({
  config,
  temperatureProp,
  chartRef,
  temperatureRange,
  curvePoints,
  speedMax,
}: {
  config: types.AppConfig;
  temperatureProp?: types.TemperatureData | null;
  chartRef: React.RefObject<HTMLDivElement | null>;
  temperatureRange: { min: number; max: number };
  curvePoints: { temperature: number; rpm: number }[];
  speedMax: number;
}) {
  const tempSource = (((config as any).tempSource as string) || 'max') as 'max' | 'cpu' | 'gpu';
  const liveRefTemp = useAppStore((state) => {
    const temp = temperatureProp ?? state.temperature;
    if (!temp) return null;
    const cpu = temp.cpuTemp ?? 0;
    const gpu = temp.gpuTemp ?? 0;
    const max = temp.maxTemp ?? 0;
    if (tempSource === 'cpu') return cpu > 0 ? cpu : (max > 0 ? max : null);
    if (tempSource === 'gpu') return gpu > 0 ? gpu : (max > 0 ? max : null);
    return max > 0 ? max : null;
  });

  return (
    <TemperatureIndicator
      temperature={liveRefTemp}
      chartRef={chartRef}
      temperatureRange={temperatureRange}
      curvePoints={curvePoints}
      speedMax={speedMax}
    />
  );
});

const LiveBlackSharkUsbSupplyWarning = memo(function LiveBlackSharkUsbSupplyWarning({
  visibleDevice,
  fanDataProp,
}: {
  visibleDevice: boolean;
  fanDataProp?: types.FanData | null;
}) {
  const { t } = useTranslation();
  const currentMode = useAppStore((state) => fanDataProp?.currentMode ?? state.fanData?.currentMode ?? 1);
  return (
    <AnimatePresence>
      {visibleDevice && currentMode === 0 && (
        <m.div
          initial={{ opacity: 0, height: 0 }}
          animate={{ opacity: 1, height: 'auto' }}
          exit={{ opacity: 0, height: 0 }}
          className="overflow-hidden"
        >
          <div className="flex items-start gap-3 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-2.5 text-sm text-destructive">
            <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" />
            <p className="flex-1 leading-relaxed">{t('fanCurve.powerSupply.usbWarning')}</p>
          </div>
        </m.div>
      )}
    </AnimatePresence>
  );
});

/* ── 曲线两端的延长线（黑鲨 4 点曲线专用） ── */

// 给黑鲨 4 点曲线补两条延长线：首点 → (10℃, 最低转速)，末点 → (100℃, 最高转速)；端点只展示不可拖。
// 只在「设备侧跟随」模式渲染（blackSharkGearCurveMode）：主机侧调速用的是通用曲线，
// 温度轴到 110℃，右端延长线会从 110℃ 反向折回 100℃ 画错。
const CURVE_EXTENSION_MIN_TEMP = 10;
const CURVE_EXTENSION_MAX_TEMP = 100;

const CurveExtensions = memo(function CurveExtensions({
  chartRef, anchors, temperatureRange, speedMax, floorSpeed,
}: {
  chartRef: React.RefObject<HTMLDivElement | null>;
  anchors: { temperature: number; rpm: number }[];
  temperatureRange: { min: number; max: number };
  speedMax: number;
  floorSpeed: number;
}) {
  const geom = useChartGridGeometry(chartRef);

  if (!geom || anchors.length < 2) return null;

  const tempSpan = Math.max(1, temperatureRange.max - temperatureRange.min);
  const xOf = (temp: number) => geom.left + ((temp - temperatureRange.min) / tempSpan) * geom.width;
  const yOf = (rpm: number) =>
    geom.top + (1 - Math.min(1, Math.max(0, rpm / Math.max(1, speedMax)))) * geom.height;

  const first = anchors[0];
  const last = anchors[anchors.length - 1];
  // 两个端点的坐标：左/右端温度与两端转速都在这里算一次。
  const leftEnd = { temperature: CURVE_EXTENSION_MIN_TEMP, rpm: floorSpeed };
  const rightEnd = { temperature: CURVE_EXTENSION_MAX_TEMP, rpm: speedMax };
  const stroke = 'var(--chart-primary)';
  return (
    <svg className="absolute inset-0 pointer-events-none overflow-visible" style={{ width: '100%', height: '100%' }}>
      <line
        x1={xOf(first.temperature)} y1={yOf(first.rpm)}
        x2={xOf(leftEnd.temperature)} y2={yOf(leftEnd.rpm)}
        stroke={stroke} strokeWidth={2} strokeDasharray="6 4" opacity={0.55}
      />
      <line
        x1={xOf(last.temperature)} y1={yOf(last.rpm)}
        x2={xOf(rightEnd.temperature)} y2={yOf(rightEnd.rpm)}
        stroke={stroke} strokeWidth={2} strokeDasharray="6 4" opacity={0.55}
      />
      {/* 两个端点与其他点一样画成点，但置灰、不可拖（纯展示，无事件）。 */}
      {[leftEnd, rightEnd].map((p) => (
        <circle
          key={p.temperature}
          cx={xOf(p.temperature)} cy={yOf(p.rpm)} r={4}
          fill="var(--chart-tooltip-bg, var(--card))"
          stroke="var(--muted-foreground)" strokeWidth={2} opacity={0.9}
        />
      ))}
    </svg>
  );
});

/* ── Tooltip label helper ── */
const ConfigTooltipLabel = memo(function ConfigTooltipLabel({ label, description }: { label: string; description: string }) {
  const { t } = useTranslation();

  return (
    <span className="inline-flex items-center gap-1">
      <span>{label}</span>
      <Tooltip>
        <TooltipTrigger asChild>
          <button type="button" className="inline-flex cursor-pointer items-center justify-center rounded text-muted-foreground transition-colors hover:text-foreground" aria-label={t('fanCurve.chart.tooltipDescriptionAria', { label })}>
            <Info className="h-3.5 w-3.5" />
          </button>
        </TooltipTrigger>
        <TooltipContent className="max-w-[260px] leading-relaxed">{description}</TooltipContent>
      </Tooltip>
    </span>
  );
});

/* ── Draggable chart point ── */

const DraggablePoint = memo(function DraggablePoint({
  cx, cy, index, temperature, speed, unitSuffix, onDragStart, isActive, isSelected, isMonotonicAffected, allowHorizontal,
}: {
  cx: number; cy: number; index: number; temperature: number; speed: number; unitSuffix: string;
  onDragStart: (index: number, clientX?: number, clientY?: number) => void;
  isActive: boolean; isSelected: boolean; isMonotonicAffected: boolean; allowHorizontal: boolean;
}) {
  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    onDragStart(index, e.clientX, e.clientY);
  }, [index, onDragStart]);
  const handleTouchStart = useCallback((e: React.TouchEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const touch = e.touches[0];
    onDragStart(index, touch?.clientX, touch?.clientY);
  }, [index, onDragStart]);
  // 黑鲨的点可以左右拖，用四向移动光标；其余曲线仍只上下拖，保持纵向拉伸光标。
  const cursor = allowHorizontal ? 'move' : 'ns-resize';

  return (
    <g>
      <circle cx={cx} cy={cy} r={isActive ? 20 : 18} fill="transparent" stroke="transparent" style={{ cursor }} onMouseDown={handleMouseDown} onTouchStart={handleTouchStart} />
      {isMonotonicAffected && !isActive && (
        <circle cx={cx} cy={cy} r={12.5} fill="rgba(245, 158, 11, 0.22)" stroke="#f59e0b" strokeWidth={2} pointerEvents="none" />
      )}
      {isSelected && !isActive && (
        <circle cx={cx} cy={cy} r={11.5} fill="var(--chart-primary)" fillOpacity={0.14} stroke="var(--chart-primary)" strokeWidth={1.75} pointerEvents="none" />
      )}
      {isActive && (
        <circle cx={cx} cy={cy} r={13} fill="var(--chart-primary)" opacity={0.18} pointerEvents="none" />
      )}
      <circle cx={cx} cy={cy} r={isActive ? 7.5 : 5.5} fill={isActive ? 'var(--chart-primary-active)' : 'var(--chart-primary)'} stroke="var(--card)" strokeWidth={2}
        style={{ cursor, transition: 'none' }}
        onMouseDown={handleMouseDown} onTouchStart={handleTouchStart}
      />
      {isActive && (
        <g pointerEvents="none">
          <rect x={cx - 56} y={cy - 37} width={112} height={24} rx={12} fill="var(--chart-primary-active)" stroke="var(--card)" strokeWidth={1.5} />
          <text x={cx} y={cy - 21} textAnchor="middle" fill="white" fontSize={11} fontWeight={600}>{temperature}°C · {formatSpeedValue(speed)}{unitSuffix}</text>
        </g>
      )}
    </g>
  );
});

/* ═══════════════════════════════════════════════════════════
   ─── Recharts 图表（动态分包，见 fan-curve-charts.tsx） ───
   ═══════════════════════════════════════════════════════════ */

// 图表的 props 只收父级已 memo 的值（数据/量程/可见性/缩放/统计），并把常量对象提升到模块级，
// 这样遥测节拍（约 250ms~1s）触发的父级重渲会在 memo 浅比较处整体跳出，recharts 树只在
// 数据真正变化的刷新周期（约 5s）重算。
//
// recharts 体积大：三张图表组件整体搬进 fan-curve-charts.tsx 单独分包，主包不再解析 recharts。
// 组件仍是首帧后按需拉取；ssr:false（Wails 静态导出无服务端渲染，图表只在客户端挂载），
// 加载完成前 loading 渲染 null，图表区域高度由外层容器撑住，不产生布局抖动。
// chunk 会在首帧后由 requestIdleCallback 预取（见 FanCurve 内的 effect），打开曲线页零等待。
const CurveEditorChart = dynamic(() => import('./fan-curve-charts').then((mod) => mod.CurveEditorChart), { ssr: false, loading: () => null });
const HistoryThermalChart = dynamic(() => import('./fan-curve-charts').then((mod) => mod.HistoryThermalChart), { ssr: false, loading: () => null });
const HistoryPowerChart = dynamic(() => import('./fan-curve-charts').then((mod) => mod.HistoryPowerChart), { ssr: false, loading: () => null });

/* ═══════════════════════════════════════════════════════════
   ─── Main FanCurve Component ───
   ═══════════════════════════════════════════════════════════ */

const FanCurve = memo(function FanCurve({ config, onConfigChange, isConnected, fanData, temperature: temperatureProp, runtimeDeviceProfile, runtimeDeviceCapabilities, deviceModel, focusTarget, onFocusHandled, historyOnly = false }: FanCurveProps) {
  const storeFanTransport = useAppStore((state) => state.fanData?.transport ?? '');
  const storeFanSpeedUnit = useAppStore((state) => state.fanData?.speedUnit ?? '');
  const storeFlyDigiCapability = useAppStore((state) => state.fanData?.flyDigiCapability ?? null);
  const fanTransport = fanData?.transport || storeFanTransport;
  const fanSpeedUnit = fanData?.speedUnit || storeFanSpeedUnit;
  const fanCapabilitiesMeta = useMemo(
    () => (fanTransport || fanSpeedUnit
      ? { transport: fanTransport, speedUnit: fanSpeedUnit }
      : null),
    [fanSpeedUnit, fanTransport],
  );
  // 是否黑鲨设备：用运行时档案判断（原生 BLE/HID 下配置里的 activeDeviceProfileId 会被清空）。
  const isBlackSharkDevice = ((runtimeDeviceProfile as { id?: string } | null | undefined)?.id ?? '')
    .startsWith('builtin.blackshark');

  // 黑鲨曲线那 4 个点能被拖到的温度取值域，同样由后端拥有（纯计算，未连设备也能取）。
  const [blackSharkCurveTempRange, setBlackSharkCurveTempRange] = useState<{ minTempC: number; maxTempC: number } | null>(null);

  // 「设备侧四档曲线」模式：黑鲨 + 未被主机侧调速接管。
  // 主机侧调速下曲线集换成通用单方案（默认 17 点，见后端 blackSharkHostSpeedCurveSuffix），
  // 四档那套"4 点可横拖、温度轴受设备约束"的交互一律不适用。
  const blackSharkGearCurveMode = isBlackSharkDevice && !config.blackSharkHostVariableSpeed;

  // 黑鲨手动挡位的转速/量程/步进由后端按标定表派生；这里只负责取一次并注入。
  const [blackSharkRpmsRev, setBlackSharkRpmsRev] = useState(0);
  useEffect(() => {
    if (!isBlackSharkDevice) return;
    if (!blackSharkGearCurveMode) {
      // 主机侧调速：曲线是通用的 17 点，不需要设备侧那套温度取值域。
      setBlackSharkCurveTempRange(null);
      setBlackSharkGearRpmsFromCore(null);
      setBlackSharkRpmsRev((v) => v + 1);
      return;
    }
    let cancelled = false;
    void (async () => {
      try {
        const payload = await apiService.getBlackSharkManualGearPresets();
        if (cancelled) return;
        setBlackSharkGearRpmsFromCore(payload);
      } catch {
        if (cancelled) return;
        setBlackSharkGearRpmsFromCore(null);
      }
      try {
        const range = await apiService.getBlackSharkCurveTempRange();
        if (!cancelled && typeof range?.minTempC === 'number' && typeof range?.maxTempC === 'number') {
          setBlackSharkCurveTempRange({ minTempC: range.minTempC, maxTempC: range.maxTempC });
        }
      } catch {
        // 取不到就只允许纵向拖动：宁可退化成原来的行为，也不猜一个区间。
        if (!cancelled) setBlackSharkCurveTempRange(null);
      }
      if (!cancelled) setBlackSharkRpmsRev((v) => v + 1);
    })();
    return () => {
      cancelled = true;
    };
  }, [blackSharkGearCurveMode, isBlackSharkDevice, isConnected]);

  const { t } = useTranslation();
  const { locale } = useLocale();
  const [localCurve, setLocalCurve] = useState<types.FanCurvePoint[]>([]);
  const [curveProfiles, setCurveProfiles] = useState<CurveProfile[]>([]);
  const [activeProfileId, setActiveProfileId] = useState('');
  const [profileOpLoading, setProfileOpLoading] = useState(false);
  const [createProfileDialogOpen, setCreateProfileDialogOpen] = useState(false);
  const [manageProfilesDialogOpen, setManageProfilesDialogOpen] = useState(false);
  const [manageDialogTab, setManageDialogTab] = useState<'list' | 'transfer'>('list');
  const [editingProfileId, setEditingProfileId] = useState('');
  const [editingProfileName, setEditingProfileName] = useState('');
  const [isEditingProfileComposing, setIsEditingProfileComposing] = useState(false);
  const [selectedExportProfileIds, setSelectedExportProfileIds] = useState<string[]>([]);
  const [profileSwitchDialogOpen, setProfileSwitchDialogOpen] = useState(false);
  const [deleteProfileDialogOpen, setDeleteProfileDialogOpen] = useState(false);
  const [pendingDeleteProfileId, setPendingDeleteProfileId] = useState('');
  const [newProfileNameInput, setNewProfileNameInput] = useState('');
  const [isNewProfileComposing, setIsNewProfileComposing] = useState(false);
  const [importCode, setImportCode] = useState('');
  const [isImportDragging, setIsImportDragging] = useState(false);
  const [profileListOverflow, setProfileListOverflow] = useState({ left: false, right: false });
  const [editorSession, setEditorSession] = useState(createCurveEditorSession);
  const hasUnsavedChanges = editorSession.dirty;
  const pendingProfileId = editorSession.pendingProfileId;
  const pendingTab = useAppStore((state) => state.pendingTab);
  const monitorOnlyActive = useAppStore((state) => state.monitorOnlyActive);
  const timelineEvents = useAppStore((state) => state.timelineEvents);
  const setCurveDraftDirty = useAppStore((state) => state.setCurveDraftDirty);
  const completePendingTabChange = useAppStore((state) => state.completePendingTabChange);
  const cancelPendingTabChange = useAppStore((state) => state.cancelPendingTabChange);
  const [isInitialized, setIsInitialized] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [learningConfigLoading, setLearningConfigLoading] = useState(false);
  const [learningApplyLoading, setLearningApplyLoading] = useState(false);
  const [learningResetLoading, setLearningResetLoading] = useState(false);
  const [applyLearningDialogOpen, setApplyLearningDialogOpen] = useState(false);
  const [lowRpmDialogOpen, setLowRpmDialogOpen] = useState(false);
  const [gearEditOpen, setGearEditOpen] = useState(false);
  const [noiseDiagnosticOpen, setNoiseDiagnosticOpen] = useState(false);
  const [axisNoiseScanOpen, setAxisNoiseScanOpen] = useState(false);
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [selectedPointIndex, setSelectedPointIndex] = useState<number | null>(null);
  const [pointSpeedDraft, setPointSpeedDraft] = useState('');
  const [pointTempDraft, setPointTempDraft] = useState('');
  const [stepSnapEnabled, setStepSnapEnabled] = useState(true);
  const [curveUndoStack, setCurveUndoStack] = useState<types.FanCurvePoint[][]>([]);
  const [monotonicPushedIndices, setMonotonicPushedIndices] = useState<number[]>([]);
  const [isInteracting, setIsInteracting] = useState(false);
  const [historyDisplayDialogOpen, setHistoryDisplayDialogOpen] = useState(false);
  const [historyChartsReady, setHistoryChartsReady] = useState(false);
  const [historyZoomDomain, setHistoryZoomDomain] = useState<[number, number] | null>(null);
  const [historyZoomSelect, setHistoryZoomSelect] = useState<{ start: number; end: number } | null>(null);
  const [draggedHistorySeries, setDraggedHistorySeries] = useState<HistorySeriesKey | null>(null);
  const {
    orderedSeries,
    seriesVisibility: historySeriesVisibility,
    homeSeriesVisibility,
    toggleSeriesVisible,
    toggleHomeSeriesVisible,
    moveSeries,
    reorderSeries,
    resetPreferences: resetHistoryDisplayPreferences,
    showStatistics: historyShowStatistics,
    setShowStatistics: setHistoryShowStatistics,
    splitTooltipByChart: historySplitTooltipByChart,
    setSplitTooltipByChart: setHistorySplitTooltipByChart,
    showTimelineEvents: historyShowTimelineEvents,
    setShowTimelineEvents: setHistoryShowTimelineEvents,
    timelineEventVisibility: historyTimelineEventVisibility,
    toggleTimelineEventVisible,
  } = useHistoryDisplayPreferences();
  const chartRef = useRef<HTMLDivElement>(null);
  const profileFileInputRef = useRef<HTMLInputElement>(null);
  const profileListRef = useRef<HTMLDivElement>(null);
  const curveEditorRef = useRef<HTMLDivElement>(null);
  const historyDetailsRef = useRef<HTMLElement>(null);
  const initialFocusTarget = useRef(focusTarget).current;
  const chartBoundsRef = useRef<CurveChartPixelBounds | null>(null);
  const dragFrameRef = useRef<number | null>(null);
  const pendingDragRef = useRef<{ clientX: number; clientY: number; shiftKey: boolean } | null>(null);
  const dragStartOriginRef = useRef<{ x: number; y: number; moved: boolean } | null>(null);
  const dragStartCurveRef = useRef<types.FanCurvePoint[] | null>(null);
  const monotonicHighlightTimerRef = useRef<number | null>(null);
  const localCurveRef = useRef<types.FanCurvePoint[]>([]);
  const curveUndoStackRef = useRef<types.FanCurvePoint[][]>([]);
  const curveSavedBaselineRef = useRef<types.FanCurvePoint[] | null>(null);
  const lowRpmWarnedLevelRef = useRef<'none' | 'below1000' | 'below800'>('none');
  const pendingLowRpmSaveRef = useRef(false);
  const historySeriesItemRefs = useRef<Partial<Record<HistorySeriesKey, HTMLDivElement>>>({});
  const historySeriesDragRef = useRef<{ key: HistorySeriesKey; target?: HistorySeriesKey; placement?: 'before' | 'after' } | null>(null);
  const historySeriesDragCleanupRef = useRef<(() => void) | null>(null);

  const runtimeProfileForSpeed = useMemo(() => {
    if (!isConnected) {
      return null;
    }
    if (runtimeDeviceProfile) {
      return runtimeDeviceProfile;
    }
    if (!runtimeDeviceCapabilities && !fanTransport && !fanSpeedUnit) {
      return null;
    }
    return {
      transport: fanTransport || runtimeDeviceCapabilities?.transport,
      speedUnit: fanSpeedUnit || runtimeDeviceCapabilities?.speedUnit,
      speedRange: runtimeDeviceCapabilities?.speedRange,
      capabilities: runtimeDeviceCapabilities || undefined,
    };
  }, [fanSpeedUnit, fanTransport, isConnected, runtimeDeviceCapabilities, runtimeDeviceProfile]);
  const speedUnit = useMemo(() => getFanSpeedUnit(fanCapabilitiesMeta as any, config as any, runtimeProfileForSpeed as any), [config, fanCapabilitiesMeta, runtimeProfileForSpeed]);
  const speedUnitSuffix = fanSpeedUnitLabel(speedUnit);
  const configuredSpeedRange = useMemo(() => getFanSpeedRange(config as any, speedUnit, runtimeProfileForSpeed as any), [config, runtimeProfileForSpeed, speedUnit]);
  const speedRange = useMemo(() => ({
    min: configuredSpeedRange.min,
    max: configuredSpeedRange.max,
    ticks: getFanSpeedTicks(configuredSpeedRange.min, configuredSpeedRange.max),
  }), [configuredSpeedRange.max, configuredSpeedRange.min]);
  const supportsLowRpmGuidance = speedUnit === 'rpm' && speedRange.min < RPM_RECOMMENDED_MIN;
  const hasLowRpmPoints = useMemo(
    () => supportsLowRpmGuidance && localCurve.some((point) => point.rpm < RPM_RECOMMENDED_MIN),
    [localCurve, supportsLowRpmGuidance],
  );
  // 纵轴从 0 起后不能再用 getFanSpeedTicks(min,max)（从 min=1440 起算）：会出现 0..1440 无刻度、顶部间距不均。
  const zeroBasedSpeedTicks = useMemo(() => {
    const max = speedRange.max;
    if (max <= 0) return [0];
    if (speedUnit === 'rpm' && !blackSharkGearCurveMode) {
      const ticks: number[] = [];
      for (let value = 0; value <= max; value += 1000) ticks.push(value);
      if (ticks[ticks.length - 1] !== max) ticks.push(max);
      return ticks;
    }
    const candidates = [100, 200, 250, 300, 400, 500, 600, 800, 1000, 1200, 1500, 2000, 2400, 2500, 3000];
    const step = candidates.find((c) => max % c === 0 && max / c >= 3 && max / c <= 6)
      ?? Math.max(1, Math.round(max / 4));
    const ticks: number[] = [];
    for (let v = 0; v <= max; v += step) ticks.push(v);
    if (ticks[ticks.length - 1] !== max) ticks.push(max);
    return ticks;
  }, [blackSharkGearCurveMode, speedRange.max, speedUnit]);

  const defaultCurve = speedUnit === 'rpm' ? DEFAULT_RPM_FAN_CURVE : DEFAULT_FAN_CURVE;
  const {
    points: temperatureHistory,
    enabled: temperatureHistoryEnabled,
	    saving: temperatureHistorySaving,
	    setEnabled: setTemperatureHistoryEnabled,
	    retentionHours: historyRetentionHours,
	    setRetentionHours: setHistoryRetentionHours,
	  } = useTemperatureHistory();

  useEffect(() => {
    setHistoryZoomSelect(null);
    setHistoryZoomDomain(null);
  }, [isConnected, temperatureHistoryEnabled]);

  const activeProfile = useMemo(() => curveProfiles.find((p) => p.id === activeProfileId) ?? null, [curveProfiles, activeProfileId]);
  const pendingDeleteProfile = useMemo(
    () => curveProfiles.find((profile) => profile.id === pendingDeleteProfileId) ?? null,
    [curveProfiles, pendingDeleteProfileId],
  );
  const externalActiveProfileId = ((config as any).activeFanCurveProfileId || '') as string;
  const externalDeviceCurveKey = [
    ((config as any).deviceTransport || '') as string,
    ((config as any).activeDeviceProfileId || '') as string,
    isConnected ? ((runtimeDeviceProfile as any)?.transport || fanTransport || '') as string : '',
    isConnected ? ((runtimeDeviceProfile as any)?.id || (runtimeDeviceProfile as any)?.model || fanSpeedUnit || '') as string : '',
  ].join(':');

  const temperatureRange = useMemo(() => ({
    // 黑鲨设备侧曲线保留完整 0℃ 轴域；飞智及通用曲线从 20℃ 起，避免出现无意义的空白段。
    min: blackSharkGearCurveMode ? BLACKSHARK_CURVE_AXIS_MIN_TEMP : FAN_CURVE_MIN_TEMP,
    max: blackSharkGearCurveMode ? BLACKSHARK_CURVE_AXIS_MAX_TEMP : FAN_CURVE_MAX_TEMP,
    ticks: blackSharkGearCurveMode ? BLACKSHARK_CURVE_AXIS_TEMPERATURE_TICKS : FAN_CURVE_TEMPERATURE_TICKS,
  }), [blackSharkGearCurveMode]);

  const syncConfigFromBackend = useCallback(async () => {
    try {
      const latest = await apiService.getConfig();
      onConfigChange(types.AppConfig.createFrom(latest));
    } catch {
      /* noop */
    }
  }, [onConfigChange]);

  // latestConfigRef 让回调能读到**最新**的 config，而不必把 config.fanCurve 写进依赖数组。
  const latestConfigRef = useRef(config);
  useEffect(() => {
    if (!isBlackSharkDevice) return;
    latestConfigRef.current = config;
  }, [config, isBlackSharkDevice]);

  const loadCurveProfiles = useCallback(async () => {
    if (!isConnected && !isBlackSharkDevice) return;
    try {
      const payload = await apiService.getFanCurveProfiles();
      const profiles = Array.isArray(payload?.profiles) ? payload.profiles : [];
      const activeId = payload?.activeId || profiles[0]?.id || '';
      const current = profiles.find((p) => p.id === activeId) ?? profiles[0];
      setCurveProfiles(profiles);
      setActiveProfileId(activeId);

      if (!isBlackSharkDevice) {
        if (current) {
          const normalized = normalizeFanCurve(current.curve, speedRange.min, speedRange.max, defaultCurve);
          curveSavedBaselineRef.current = cloneCurvePoints(normalized);
          setLocalCurve(normalized);
          setEditorSession((session) => markCurveEditorSaved(session, true));
        }
        return;
      }

      const ipcCurve = current?.curve && current.curve.length > 0 ? current.curve : null;
      const latest = latestConfigRef.current;
      const cacheCurve = latest.fanCurve && latest.fanCurve.length > 0 ? latest.fanCurve : null;
      const curveToRender = ipcCurve ?? cacheCurve ?? defaultCurve;
      const normalized = normalizeFanCurve(curveToRender, speedRange.min, speedRange.max, defaultCurve);
      curveSavedBaselineRef.current = cloneCurvePoints(normalized);
      setLocalCurve(normalized);
      setEditorSession((session) => markCurveEditorSaved(session, true));
    } catch {
      /* noop */
    }
  }, [defaultCurve, isBlackSharkDevice, isConnected, speedRange.max, speedRange.min]);

  const curveSpeedBounds = useMemo(() => {
    const source = localCurve.length > 0 ? localCurve : (config.fanCurve ?? []);
    if (source.length === 0) {
      return { min: speedRange.min, max: speedRange.max };
    }
    let minCurveSpeed = source[0].rpm;
    let maxCurveSpeed = source[0].rpm;
    for (let i = 1; i < source.length; i++) {
      const speed = source[i].rpm;
      if (speed < minCurveSpeed) minCurveSpeed = speed;
      if (speed > maxCurveSpeed) maxCurveSpeed = speed;
    }
    return { min: minCurveSpeed, max: maxCurveSpeed };
  }, [config.fanCurve, localCurve, speedRange.max, speedRange.min]);

  /* ── Smart control state ── */

  const smartControl = useMemo(() => {
    const curveLength = localCurve.length || config.fanCurve?.length || DEFAULT_CURVE_LENGTH;
    const defaultOffsets = Array.from({ length: curveLength }, () => 0);
    const defaultRateOffsets = Array.from({ length: 7 }, () => 0);
    const existing = config.smartControl;
    const normalizeOffsets = (source?: number[]) => Array.isArray(source) ? [...source.slice(0, curveLength), ...defaultOffsets].slice(0, curveLength) : defaultOffsets;
    const normalizeRateOffsets = (source?: number[]) => Array.isArray(source) ? [...source.slice(0, 7), ...defaultRateOffsets].slice(0, 7) : defaultRateOffsets;

    if (!existing) {
      return { enabled: true, learning: true, learningBias: 'balanced', filterTransientSpike: true, temperatureRisePrediction: true, temperatureRisePredictionMaxBoost: 60, targetTemp: 68, aggressiveness: 5, hysteresis: 2, minRpmChange: 2, rampUpLimit: 8, rampDownLimit: 6, learnRate: 3, learnWindow: 8, learnDelay: 3, overheatWeight: 8, rpmDeltaWeight: 5, noiseWeight: 4, trendGain: 5, maxLearnOffset: 20, learnedOffsets: defaultOffsets, learnedOffsetsHeat: defaultOffsets, learnedOffsetsCool: defaultOffsets, learnedRateHeat: defaultRateOffsets, learnedRateCool: defaultRateOffsets };
    }

    return {
      ...existing,
      learning: existing.learning ?? true,
      learningBias: normalizeLearningBias((existing as any).learningBias),
      filterTransientSpike: existing.filterTransientSpike ?? true,
      temperatureRisePrediction: (existing as any).temperatureRisePrediction ?? true,
      temperatureRisePredictionMaxBoost: (existing as any).temperatureRisePredictionMaxBoost ?? 60,
      targetTemp: normalizeTargetTemp(existing.targetTemp ?? 68),
      hysteresis: Math.max(1, existing.hysteresis ?? 2),
      learnWindow: existing.learnWindow ?? 8, learnDelay: existing.learnDelay ?? 3,
      overheatWeight: existing.overheatWeight ?? 8, rpmDeltaWeight: existing.rpmDeltaWeight ?? 5,
      noiseWeight: existing.noiseWeight ?? 4, trendGain: existing.trendGain ?? 5,
      learnedOffsets: normalizeOffsets(existing.learnedOffsets),
      learnedOffsetsHeat: normalizeOffsets(existing.learnedOffsetsHeat),
      learnedOffsetsCool: normalizeOffsets(existing.learnedOffsetsCool),
      learnedRateHeat: normalizeRateOffsets(existing.learnedRateHeat),
      learnedRateCool: normalizeRateOffsets(existing.learnedRateCool),
    };
  }, [config.fanCurve, config.smartControl, localCurve.length]);

  const learningBiasOptions = useMemo(
    () => LEARNING_BIAS_OPTIONS.map((option) => ({
      value: option.value,
      label: t(option.labelKey),
      description: t(option.descriptionKey),
    })),
    [t, locale],
  );

  const currentLearningBias = normalizeLearningBias((smartControl as any).learningBias);
  const currentLearningBiasOption = learningBiasOptions.find((option) => option.value === currentLearningBias) ?? learningBiasOptions[0];
  const [targetTempDraft, setTargetTempDraft] = useState(() => normalizeTargetTemp((config.smartControl as any)?.targetTemp ?? 68));

  useEffect(() => {
    setCurveDraftDirty(isConnected && hasUnsavedChanges);
  }, [hasUnsavedChanges, isConnected, setCurveDraftDirty]);

  useEffect(() => {
    if (pendingTab) {
      if (!isConnected || !hasUnsavedChanges) {
        completePendingTabChange();
        return;
      }
      setEditorSession((session) => requestCurveTabSwitch(session).session);
      setProfileSwitchDialogOpen(true);
    }
  }, [completePendingTabChange, hasUnsavedChanges, isConnected, pendingTab]);

  useEffect(() => {
    localCurveRef.current = localCurve;
  }, [localCurve]);

  useEffect(() => {
    curveUndoStackRef.current = curveUndoStack;
  }, [curveUndoStack]);

  useEffect(() => () => {
    if (monotonicHighlightTimerRef.current !== null) {
      window.clearTimeout(monotonicHighlightTimerRef.current);
    }
  }, []);

  const triggerMonotonicHighlight = useCallback((affectedIndices: number[]) => {
    if (affectedIndices.length === 0) return;
    setMonotonicPushedIndices(affectedIndices);
    if (monotonicHighlightTimerRef.current !== null) {
      window.clearTimeout(monotonicHighlightTimerRef.current);
    }
    monotonicHighlightTimerRef.current = window.setTimeout(() => {
      monotonicHighlightTimerRef.current = null;
      setMonotonicPushedIndices([]);
    }, 900);
  }, []);

  const pushUndoSnapshot = useCallback((snapshot: types.FanCurvePoint[]) => {
    setCurveUndoStack((prev) => pushCurveUndoSnapshot(prev, snapshot));
  }, []);

  const clearEditorHistory = useCallback(() => {
    setCurveUndoStack([]);
    setSelectedPointIndex(null);
    setMonotonicPushedIndices([]);
  }, []);

  useEffect(() => {
    if (!supportsLowRpmGuidance) {
      lowRpmWarnedLevelRef.current = 'none';
      pendingLowRpmSaveRef.current = false;
      setLowRpmDialogOpen(false);
    }
  }, [supportsLowRpmGuidance]);

  useEffect(() => {
    if (isConnected) return;
    setEditorSession(createCurveEditorSession());
    setIsInteracting(false);
    setCreateProfileDialogOpen(false);
    setManageProfilesDialogOpen(false);
    setProfileSwitchDialogOpen(false);
    setDeleteProfileDialogOpen(false);
    setApplyLearningDialogOpen(false);
    setLowRpmDialogOpen(false);
    lowRpmWarnedLevelRef.current = 'none';
    pendingLowRpmSaveRef.current = false;
    setGearEditOpen(false);
    setPendingDeleteProfileId('');
    setSelectedPointIndex(null);
    setCurveUndoStack([]);
    setMonotonicPushedIndices([]);
  }, [isConnected]);

  useEffect(() => {
    setTargetTempDraft(normalizeTargetTemp((smartControl as any).targetTemp ?? 68));
  }, [smartControl.targetTemp]);

  useEffect(() => {
    if (!focusTarget) {
      return;
    }
    if (!isConnected && focusTarget !== 'history-details') {
      onFocusHandled();
      return;
    }

    const target = focusTarget === 'history-details' ? historyDetailsRef.current : curveEditorRef.current;
    if (!target) {
      onFocusHandled();
      return;
    }

    const startedAt = performance.now();
    let frame = 0;
    let lastTop: number | null = null;
    let stableFrameCount = 0;

    const scrollUntilStable = (timestamp: number) => {
      target.scrollIntoView({ block: 'start' });
      const nextTop = target.getBoundingClientRect().top;
      stableFrameCount = lastTop !== null && Math.abs(nextTop - lastTop) <= FOCUS_SCROLL_EPSILON_PX
        ? stableFrameCount + 1
        : 0;
      lastTop = nextTop;

      if (stableFrameCount >= FOCUS_SCROLL_STABLE_FRAMES || timestamp - startedAt >= FOCUS_SCROLL_TIMEOUT_MS) {
        onFocusHandled();
        return;
      }

      frame = window.requestAnimationFrame(scrollUntilStable);
    };

    frame = window.requestAnimationFrame(scrollUntilStable);

    return () => {
      window.cancelAnimationFrame(frame);
    };
  }, [focusTarget, isConnected, onFocusHandled]);

  useEffect(() => {
    if (historyChartsReady) return;
    if (typeof IntersectionObserver === 'undefined') {
      setHistoryChartsReady(true);
      return;
    }
    if (!historyDetailsRef.current) return;

    const observer = new IntersectionObserver(([entry]) => {
      if (!entry?.isIntersecting) return;
      setHistoryChartsReady(true);
      observer.disconnect();
    }, { rootMargin: '360px 0px' });
    observer.observe(historyDetailsRef.current);
    return () => observer.disconnect();
  }, [historyChartsReady]);

  // 首帧后趁空闲预取 recharts 图表分包（fan-curve-charts），用户打开曲线页/历史图前把 chunk 拉热，
  // dynamic 挂载零等待；失败静默吞掉（各图表的 loading 兜底为 null，不影响页面其余部分）。
  useEffect(() => {
    const prefetchFanCurveCharts = () => {
      void import('./fan-curve-charts').catch(() => {});
    };
    if (typeof window.requestIdleCallback === 'function') {
      const idleId = window.requestIdleCallback(prefetchFanCurveCharts, { timeout: 1500 });
      return () => window.cancelIdleCallback(idleId);
    }
    const timer = window.setTimeout(prefetchFanCurveCharts, 1500);
    return () => window.clearTimeout(timer);
  }, []);

  const learningComfortBand = useMemo(() => {
    const low = Math.max(temperatureRange.min, targetTempDraft - Math.max((smartControl.hysteresis ?? 2) + 3, 3));
    const high = Math.min(temperatureRange.max, targetTempDraft);
    return { low: Math.min(low, high), high };
  }, [smartControl.hysteresis, targetTempDraft, temperatureRange.max, temperatureRange.min]);

  const learnedOffsetPoints = useMemo(() => {
    const sourceCurve = localCurve.length > 0 ? localCurve : (config.fanCurve || []);
    const offsets = smartControl.learnedOffsets || [];
    return sourceCurve.map((point, index) => {
      const delta = learnedOffsetForDisplay(constrainOffsetByLearningBias(typeof offsets[index] === 'number' ? offsets[index] : 0, currentLearningBias), speedUnit);
      const learned = Math.max(curveSpeedBounds.min, Math.min(curveSpeedBounds.max, point.rpm + delta));
      return {
        index,
        temperature: point.temperature,
        base: point.rpm,
        learned,
        delta,
      };
    });
  }, [config.fanCurve, currentLearningBias, curveSpeedBounds.max, curveSpeedBounds.min, localCurve, smartControl.learnedOffsets, speedUnit]);

  const learnedOffsetStats = useMemo(() => {
    const nonZero = learnedOffsetPoints.filter((item) => item.delta !== 0);
    let maxItem = nonZero[0] ?? null;
    for (const item of nonZero) {
      if (maxItem && Math.abs(item.delta) > Math.abs(maxItem.delta)) {
        maxItem = item;
      }
    }
    const maxAbs = maxItem ? Math.abs(maxItem.delta) : 0;
    const scaleMax = Math.max(speedUnit === 'percent' ? 12 : 200, maxAbs);
    return {
      count: nonZero.length,
      maxItem,
      scaleMax,
    };
  }, [learnedOffsetPoints, speedUnit]);

  const detailHistoryPoints = useMemo(
    () => historyChartsReady ? temperatureHistory : [],
    [historyChartsReady, temperatureHistory],
  );

  // 摘要统计同样改在降采样切片上计算（峰值/均值随抽样略有偏移，已确认可接受）；
  // 与原先语义一致：摘要不看缩放窗口、也不等图表就绪门槛。
  const historySummaryPoints = useMemo(
    () => downsampleHistoryPoints(temperatureHistory, 600),
    [temperatureHistory],
  );

  const historySummary = useMemo(() => {
    const latest = historySummaryPoints[historySummaryPoints.length - 1] ?? null;
    const first = historySummaryPoints[0] ?? null;
    let cpuPeak = 0;
    let gpuPeak = 0;
    let fanPeak = 0;
    let cpuPowerPeak = 0;
    let gpuPowerPeak = 0;
    let cpuSum = 0;
    let gpuSum = 0;
    let fanSum = 0;
    let cpuPowerSum = 0;
    let gpuPowerSum = 0;
    let cpuCount = 0;
    let gpuCount = 0;
    let fanCount = 0;
    let cpuPowerCount = 0;
    let gpuPowerCount = 0;

    for (const point of historySummaryPoints) {
      if (point.cpuTemp > 0) {
        cpuPeak = Math.max(cpuPeak, point.cpuTemp);
        cpuSum += point.cpuTemp;
        cpuCount += 1;
      }
      if (point.gpuTemp > 0) {
        gpuPeak = Math.max(gpuPeak, point.gpuTemp);
        gpuSum += point.gpuTemp;
        gpuCount += 1;
      }
      const rawFan = Number(point.fanRpm || 0);
      if (rawFan > 0) {
        const fan = normalizeSpeedValue(rawFan, speedRange.min, speedRange.max);
        fanPeak = Math.max(fanPeak, fan);
        fanSum += fan;
        fanCount += 1;
      }
      const cpuPower = Number(point.cpuPowerWatts || 0);
      if (Number.isFinite(cpuPower) && cpuPower >= MIN_VALID_POWER_WATTS) {
        cpuPowerPeak = Math.max(cpuPowerPeak, cpuPower);
        cpuPowerSum += cpuPower;
        cpuPowerCount += 1;
      }
      const gpuPower = Number(point.gpuPowerWatts || 0);
      if (Number.isFinite(gpuPower) && gpuPower >= MIN_VALID_POWER_WATTS) {
        gpuPowerPeak = Math.max(gpuPowerPeak, gpuPower);
        gpuPowerSum += gpuPower;
        gpuPowerCount += 1;
      }
    }
    const average = (sum: number, count: number) => count > 0 ? Math.round(sum / count) : 0;

    return {
      sampleCount: historySummaryPoints.length,
      latest,
      latestLabel: latest ? formatHistoryDateTime(latest.timestamp, locale) : '--',
      durationLabel: first && latest ? formatHistoryDuration(first.timestamp, latest.timestamp, t) : '--',
      cpuPeak: Number(formatSpeedValue(cpuPeak)),
      cpuAverage: average(cpuSum, cpuCount),
      gpuPeak: Number(formatSpeedValue(gpuPeak)),
      gpuAverage: average(gpuSum, gpuCount),
      fanPeak,
      fanAverage: average(fanSum, fanCount),
      cpuPowerPeak,
      cpuPowerAverage: average(cpuPowerSum, cpuPowerCount),
      gpuPowerPeak,
      gpuPowerAverage: average(gpuPowerSum, gpuPowerCount),
    };
  }, [historySummaryPoints, locale, speedRange.max, speedRange.min, t]);

  // 缩放窗口先作用在原始点（时间戳与取值映射无关），再降采样，最后才做逐点取值映射与统计，
  // 让每个刷新周期（约 5s 一点）的重活都落在 ≤601 点的显示切片上；与首页详图保持一致，显著减轻 SVG DOM 与重绘开销。
  const zoomedHistoryPoints = useMemo(() => {
    if (!historyZoomDomain) {
      return detailHistoryPoints;
    }
    const [from, to] = historyZoomDomain;
    const selected = detailHistoryPoints.filter((point) => point.timestamp >= from && point.timestamp <= to);
    return selected.length >= 2 ? selected : detailHistoryPoints;
  }, [detailHistoryPoints, historyZoomDomain]);
  const historyDisplayPoints = useMemo(
    () => downsampleHistoryPoints(zoomedHistoryPoints, 600),
    [zoomedHistoryPoints],
  );

  const historyChartStats = useMemo(() => {
    let maxPower = 0;
    const data = historyDisplayPoints.map((point) => {
      const rawCpuPowerWatts = Number(point.cpuPowerWatts || 0);
      const rawGpuPowerWatts = Number(point.gpuPowerWatts || 0);
      const cpuPowerWatts = Number.isFinite(rawCpuPowerWatts) && rawCpuPowerWatts >= MIN_VALID_POWER_WATTS ? rawCpuPowerWatts : 0;
      const gpuPowerWatts = Number.isFinite(rawGpuPowerWatts) && rawGpuPowerWatts >= MIN_VALID_POWER_WATTS ? rawGpuPowerWatts : 0;
      const totalPowerWatts = getAvailableTotalPowerWatts(point);
      if (historySeriesVisibility.cpuPower) {
        maxPower = Math.max(maxPower, cpuPowerWatts);
      }
      if (historySeriesVisibility.gpuPower) {
        maxPower = Math.max(maxPower, gpuPowerWatts);
      }
      if (historySeriesVisibility.totalPower) {
        maxPower = Math.max(maxPower, totalPowerWatts || 0);
      }
      const rawFanRpm = Number(point.fanRpm || 0);
      return {
        ...point,
        cpuTemp: point.cpuTemp > 0 ? point.cpuTemp : undefined,
        gpuTemp: point.gpuTemp > 0 ? point.gpuTemp : undefined,
        fanRpm: rawFanRpm > 0 ? normalizeSpeedValue(rawFanRpm, speedRange.min, speedRange.max) : undefined,
        cpuPowerWatts: cpuPowerWatts >= MIN_VALID_POWER_WATTS ? Math.round(cpuPowerWatts * 10) / 10 : undefined,
        gpuPowerWatts: gpuPowerWatts >= MIN_VALID_POWER_WATTS ? Math.round(gpuPowerWatts * 10) / 10 : undefined,
        totalPowerWatts,
      };
    });

    const powerMax = maxPower > 0 ? Math.max(20, Math.ceil((maxPower + 10) / 10) * 10) : 20;

    return {
      data,
      powerMax,
      hasPower: maxPower > 0,
    };
  }, [historyDisplayPoints, historySeriesVisibility.cpuPower, historySeriesVisibility.gpuPower, historySeriesVisibility.totalPower, speedRange.max, speedRange.min]);
  // 降采样提前后，“缩放窗口的图表数据 / 图表显示数据 / 统计输入”收敛为同一份切片。
  const zoomedHistoryChartData = historyChartStats.data;
  const historyDisplayData = zoomedHistoryChartData;
  const historyChartData = zoomedHistoryChartData;
  const historyPowerMax = historyChartStats.powerMax;
  const historyHasPower = historyChartStats.hasPower;
  const historySeriesMeta = useMemo<HistorySeriesMeta[]>(() => {
    const meta: Record<HistorySeriesKey, HistorySeriesMeta> = {
      cpu: { key: 'cpu', label: t('fanCurve.history.series.cpu'), color: CPU_TEMP_STROKE, dataKey: HISTORY_SERIES_DATA_KEY.cpu, axisId: HISTORY_SERIES_AXIS.cpu },
      gpu: { key: 'gpu', label: t('fanCurve.history.series.gpu'), color: GPU_TEMP_STROKE, dataKey: HISTORY_SERIES_DATA_KEY.gpu, axisId: HISTORY_SERIES_AXIS.gpu },
      fan: { key: 'fan', label: t('fanCurve.history.series.fan'), color: FAN_SPEED_STROKE, dataKey: HISTORY_SERIES_DATA_KEY.fan, axisId: HISTORY_SERIES_AXIS.fan },
      cpuPower: { key: 'cpuPower', label: t('fanCurve.history.series.cpuPower'), color: CPU_POWER_STROKE, dataKey: HISTORY_SERIES_DATA_KEY.cpuPower, axisId: HISTORY_SERIES_AXIS.cpuPower },
      gpuPower: { key: 'gpuPower', label: t('fanCurve.history.series.gpuPower'), color: GPU_POWER_STROKE, dataKey: HISTORY_SERIES_DATA_KEY.gpuPower, axisId: HISTORY_SERIES_AXIS.gpuPower },
      totalPower: { key: 'totalPower', label: t('fanCurve.history.series.totalPower'), color: TOTAL_POWER_STROKE, dataKey: HISTORY_SERIES_DATA_KEY.totalPower, axisId: HISTORY_SERIES_AXIS.totalPower },
    };
    return orderedSeries
      .filter((key) => !(monitorOnlyActive && key === 'fan'))
      .map((key) => meta[key])
      .filter(Boolean);
  }, [locale, monitorOnlyActive, orderedSeries, t]);
  // 图表子组件只收“当前可见”的系列数组：可见性切换才变化，遥测节拍保持引用稳定。
  const visibleThermalSeries = useMemo(
    () => historySeriesMeta.filter((series) => series.axisId !== 'power' && historySeriesVisibility[series.key]),
    [historySeriesMeta, historySeriesVisibility],
  );
  const visiblePowerSeries = useMemo(
    () => historySeriesMeta.filter((series) => series.axisId === 'power' && historySeriesVisibility[series.key]),
    [historySeriesMeta, historySeriesVisibility],
  );
  const historyStatistics = useMemo<HistoryStatisticsData>(() => {
    const values: Partial<Record<HistorySeriesKey, { min: number; max: number; average: number; minTimestamps: number[]; maxTimestamps: number[] }>> = {};
    const seriesWithData = historySeriesMeta.filter((series) => {
      if (!historySeriesVisibility[series.key]) {
        return false;
      }
      let count = 0;
      let sum = 0;
      let min = Number.POSITIVE_INFINITY;
      let max = Number.NEGATIVE_INFINITY;
      let minTimestamps: number[] = [];
      let maxTimestamps: number[] = [];
      for (const point of zoomedHistoryChartData) {
        const value = Number(point[series.dataKey] ?? 0);
        if (!Number.isFinite(value) || value <= 0) {
          continue;
        }
        count += 1;
        sum += value;
        if (value < min) {
          min = value;
          minTimestamps = [point.timestamp];
        } else if (value === min) {
          minTimestamps.push(point.timestamp);
        }
        if (value > max) {
          max = value;
          maxTimestamps = [point.timestamp];
        } else if (value === max) {
          maxTimestamps.push(point.timestamp);
        }
      }
      if (count === 0) {
        return false;
      }
      values[series.key] = {
        min,
        max,
        average: Math.round((sum / count) * 10) / 10,
        minTimestamps,
        maxTimestamps,
      };
      return true;
    });

    return {
      values,
      thermalSeries: seriesWithData.filter((series) => series.axisId !== 'power'),
      powerSeries: seriesWithData.filter((series) => series.axisId === 'power'),
    };
  }, [historySeriesMeta, historySeriesVisibility, zoomedHistoryChartData]);
  const historyRightTimestamp = zoomedHistoryChartData[zoomedHistoryChartData.length - 1]?.timestamp ?? 0;
  // 统计标线（max/min/average）改为返回纯数据描述符：ReferenceLine/ReferenceDot 元素由
  // fan-curve-charts 创建，主包因此不需要任何 recharts 运行时引用。
  // 判定门槛、取值、格式化、key 规则与原先完全一致；label 文案在这里算好随描述符传下去。
  const renderHistoryStatistics = useCallback((series: HistorySeriesMeta | undefined, yAxisId: 'temp' | 'fan' | 'power', showStatisticsForSingleSeries = true): HistoryStatisticsMarker[] | null => {
    if (!historyShowStatistics || !showStatisticsForSingleSeries || !series) {
      return null;
    }
    const statistics = historyStatistics.values[series.key];
    if (!statistics) {
      return null;
    }
    const formatValue = (value: number) => series.axisId === 'power'
      ? formatPowerValue(value)
      : formatSpeedValue(value);
    const entries = [
      { key: 'max', value: statistics.max, timestamps: statistics.maxTimestamps, color: 'var(--chart-stat-max)', opacity: 0.9 },
      { key: 'min', value: statistics.min, timestamps: statistics.minTimestamps, color: 'var(--chart-stat-min)', opacity: 0.72 },
      { key: 'average', value: statistics.average, timestamps: [], color: 'var(--chart-stat-average)', opacity: 0.82 },
    ] as const;
    return entries.flatMap(({ key, value, timestamps, color, opacity }) => {
      const leftmostTimestamp = timestamps[0] ?? 0;
      const emphasizedTimestamps = timestamps.slice(0, 1);
      return [
        {
          kind: 'line',
          key: `${series.key}-${key}`,
          yAxisId,
          y: key === 'average' ? value : undefined,
          segment: key === 'average' ? undefined : [
            { x: leftmostTimestamp, y: value },
            { x: historyRightTimestamp, y: value },
          ],
          stroke: color,
          strokeOpacity: opacity,
          statKey: key,
          labelText: `${t(`fanCurve.history.statistics.${key}`)} ${formatValue(value)}`,
        } satisfies HistoryStatisticsMarker,
        ...emphasizedTimestamps.map((timestamp) => ({
          kind: 'dot',
          key: `${series.key}-${key}-point-${timestamp}`,
          yAxisId,
          x: timestamp,
          y: value,
          fill: color,
        } satisfies HistoryStatisticsMarker)),
      ];
    });
  }, [historyRightTimestamp, historyShowStatistics, historyStatistics.values, t]);
  const handleHistoryZoomMouseDown = useCallback((state: { activeLabel?: string | number } | null) => {
    if (state?.activeLabel == null) {
      return;
    }
    const timestamp = Number(state.activeLabel);
    if (!Number.isFinite(timestamp)) {
      return;
    }
    setHistoryZoomSelect({ start: timestamp, end: timestamp });
  }, []);
  const handleHistoryZoomMouseMove = useCallback((state: { activeLabel?: string | number } | null) => {
    if (state?.activeLabel == null) {
      return;
    }
    const timestamp = Number(state.activeLabel);
    if (!Number.isFinite(timestamp)) {
      return;
    }
    setHistoryZoomSelect((current) => current ? { ...current, end: timestamp } : current);
  }, []);
  const handleHistoryZoomMouseUp = useCallback(() => {
    setHistoryZoomSelect((selection) => {
      if (selection) {
        const from = Math.min(selection.start, selection.end);
        const to = Math.max(selection.start, selection.end);
        // 采样数按原始全窗数据计（与显示切片无关），保证小范围缩放的判定不变。
        const sampleCount = detailHistoryPoints.filter((point) => point.timestamp >= from && point.timestamp <= to).length;
        if (sampleCount >= 2) {
          setHistoryZoomDomain([from, to]);
        }
      }
      return null;
    });
  }, [detailHistoryPoints]);
  const resetHistoryZoom = useCallback(() => {
    setHistoryZoomSelect(null);
    setHistoryZoomDomain(null);
  }, []);
  const historyZoomSelectionBounds = useMemo(
    () => (historyZoomSelect && historyZoomSelect.start !== historyZoomSelect.end
      ? [Math.min(historyZoomSelect.start, historyZoomSelect.end), Math.max(historyZoomSelect.start, historyZoomSelect.end)] as const
      : null),
    [historyZoomSelect],
  );
  // 温度副轴只显示每 4 格一个刻度；useMemo 稳住数组引用，避免子组件 memo 失效。
  const historyTempTicks = useMemo(
    () => temperatureRange.ticks.filter((value) => value === temperatureRange.max || (value - temperatureRange.min) % (FAN_CURVE_TEMP_STEP * 4) === 0),
    [temperatureRange],
  );
  const visibleTimelineEvents = useMemo(() => {
    const firstTimestamp = zoomedHistoryChartData[0]?.timestamp ?? 0;
    const lastTimestamp = zoomedHistoryChartData[zoomedHistoryChartData.length - 1]?.timestamp ?? firstTimestamp;
    if (!historyShowTimelineEvents || !firstTimestamp) return [];
    return timelineEvents.filter((event) => (
      !(monitorOnlyActive && (event.type === 'disconnect' || event.type === 'reconnect'))
      &&
      historyTimelineEventVisibility[event.type]
      && event.timestamp >= firstTimestamp
      && event.timestamp <= lastTimestamp
    )).slice(-12);
  }, [historyShowTimelineEvents, historyTimelineEventVisibility, monitorOnlyActive, timelineEvents, zoomedHistoryChartData]);
  const historyTimelineAxisId = monitorOnlyActive ? 'temp' : 'fan';
  const historyTimeDomain = useMemo<[number, number]>(() => {
    const firstTimestamp = zoomedHistoryChartData[0]?.timestamp ?? 0;
    const lastSampleTimestamp = zoomedHistoryChartData[zoomedHistoryChartData.length - 1]?.timestamp ?? firstTimestamp;
    const lastEventTimestamp = visibleTimelineEvents[visibleTimelineEvents.length - 1]?.timestamp ?? firstTimestamp;
    return [firstTimestamp, Math.max(firstTimestamp + 1, lastSampleTimestamp, lastEventTimestamp)];
  }, [visibleTimelineEvents, zoomedHistoryChartData]);
  const timelineEventLayout = useMemo(() => {
    const [firstTimestamp, lastTimestamp] = historyTimeDomain;
    const range = Math.max(1, lastTimestamp - firstTimestamp);
    const minGap = range * 0.07;
    const rowLastTimestamps: number[] = [];
    return visibleTimelineEvents.map((event) => {
      let row = rowLastTimestamps.findIndex((timestamp) => event.timestamp - timestamp >= minGap);
      if (row === -1) {
        row = rowLastTimestamps.length < 4
          ? rowLastTimestamps.length
          : rowLastTimestamps.reduce((best, timestamp, index) => (timestamp < rowLastTimestamps[best] ? index : best), 0);
      }
      rowLastTimestamps[row] = event.timestamp;
      return { event, row, anchorEnd: (event.timestamp - firstTimestamp) / range > 0.55 };
    });
  }, [historyTimeDomain, visibleTimelineEvents]);

  const showHistoryPowerChart = historyHasPower && (historySeriesVisibility.cpuPower || historySeriesVisibility.gpuPower || historySeriesVisibility.totalPower);

  const renderHistoryTooltip = useCallback(({ active, label, payload }: any, chartScope?: 'thermal' | 'power') => {
    const point = payload?.[0]?.payload;
    if (!active || !point) {
      return null;
    }

    const rows = historySeriesMeta.flatMap((series) => {
      if (!historySeriesVisibility[series.key]) {
        return [];
      }
      if (historySplitTooltipByChart && chartScope) {
        if (chartScope === 'thermal' && series.axisId === 'power') {
          return [];
        }
        if (chartScope === 'power' && series.axisId !== 'power') {
          return [];
        }
      }
      const numericValue = Number(formatSpeedValue(Number(point[series.dataKey] ?? 0)));
      if (!Number.isFinite(numericValue) || numericValue <= 0) {
        return [];
      }
      const value = series.key === 'fan'
        ? `${formatSpeedValue(numericValue)}${speedUnitSuffix}`
        : series.axisId === 'power'
          ? `${formatPowerValue(numericValue)} W`
          : `${numericValue} °C`;
      return [{ key: series.key, label: series.label, value, color: series.color }];
    });

    if (rows.length === 0) {
      return null;
    }

    return (
      <div
        className="rounded-lg border px-3 py-2 text-sm shadow-lg"
        style={{
          backgroundColor: 'var(--chart-tooltip-bg)',
          borderColor: 'var(--chart-tooltip-border)',
          boxShadow: 'var(--chart-tooltip-shadow)',
          color: 'var(--chart-tooltip-text)',
        }}
      >
        <div className="mb-2 font-semibold">{formatHistoryDateTime(Number(label), locale)}</div>
        <div className="space-y-1.5">
          {rows.map((row) => (
            <div key={row.key} className="flex items-center gap-2">
              <span className="h-2 w-2 rounded-full" style={{ backgroundColor: row.color }} />
              <span>{row.label}：{row.value}</span>
            </div>
          ))}
        </div>
      </div>
    );
  }, [historySeriesMeta, historySeriesVisibility, historySplitTooltipByChart, locale, speedUnitSuffix]);

  const renderThermalHistoryTooltip = useCallback(
    (props: any) => renderHistoryTooltip(props, 'thermal'),
    [renderHistoryTooltip],
  );
  const renderPowerHistoryTooltip = useCallback(
    (props: any) => renderHistoryTooltip(props, 'power'),
    [renderHistoryTooltip],
  );

  const handleHistorySeriesPointerDown = useCallback((event: React.PointerEvent<HTMLButtonElement>, key: HistorySeriesKey) => {
    if (event.pointerType === 'mouse' && event.button !== 0) {
      return;
    }

    historySeriesDragCleanupRef.current?.();
    event.preventDefault();
    event.stopPropagation();
    setDraggedHistorySeries(key);
    historySeriesDragRef.current = { key };

    // Avoid native HTML drag previews in WebView2; they repaint translucent shadows and feel jumpy.
    const handlePointerMove = (moveEvent: PointerEvent) => {
      const dragState = historySeriesDragRef.current;
      if (!dragState) {
        return;
      }

      for (const series of historySeriesMeta) {
        if (series.key === dragState.key) {
          continue;
        }

        const element = historySeriesItemRefs.current[series.key];
        if (!element) {
          continue;
        }

        const rect = element.getBoundingClientRect();
        if (moveEvent.clientY < rect.top || moveEvent.clientY > rect.bottom) {
          continue;
        }

        const placement = moveEvent.clientY > rect.top + rect.height / 2 ? 'after' : 'before';
        if (dragState.target === series.key && dragState.placement === placement) {
          return;
        }

        historySeriesDragRef.current = { key: dragState.key, target: series.key, placement };
        reorderSeries(dragState.key, series.key, placement);
        return;
      }
    };

    const cleanup = () => {
      window.removeEventListener('pointermove', handlePointerMove);
      window.removeEventListener('pointerup', handlePointerEnd);
      window.removeEventListener('pointercancel', handlePointerEnd);
      if (historySeriesDragCleanupRef.current === cleanup) {
        historySeriesDragCleanupRef.current = null;
      }
    };

    const handlePointerEnd = () => {
      historySeriesDragRef.current = null;
      setDraggedHistorySeries(null);
      cleanup();
    };

    window.addEventListener('pointermove', handlePointerMove);
    window.addEventListener('pointerup', handlePointerEnd);
    window.addEventListener('pointercancel', handlePointerEnd);
    historySeriesDragCleanupRef.current = cleanup;
  }, [historySeriesMeta, reorderSeries]);

  useEffect(() => () => historySeriesDragCleanupRef.current?.(), []);

  /* ── Init ── */

  // 黑鲨专享：不等连接就先用 config 里的曲线把页面画出来（非黑鲨影响审计后拆分）。
  useEffect(() => {
    if (!isBlackSharkDevice) return;
    if (localCurve.length > 0 || isInteracting) return; // 已经有曲线/正在拖动：不覆盖
    if (!config.fanCurve || config.fanCurve.length === 0) return;
    const normalized = normalizeFanCurve(config.fanCurve, speedRange.min, speedRange.max, defaultCurve);
    curveSavedBaselineRef.current = cloneCurvePoints(normalized);
    setLocalCurve(normalized);
    setIsInitialized(true);
  }, [config.fanCurve, defaultCurve, isBlackSharkDevice, isInteracting, localCurve.length, speedRange.max, speedRange.min]);

  useEffect(() => {
    if (!isConnected) return;
    if ((!isInitialized || !hasUnsavedChanges) && !isInteracting && config.fanCurve && config.fanCurve.length > 0) {
      const normalized = normalizeFanCurve(config.fanCurve, speedRange.min, speedRange.max, defaultCurve);
      curveSavedBaselineRef.current = cloneCurvePoints(normalized);
      setLocalCurve(normalized);
      setIsInitialized(true);
    }
  }, [config.fanCurve, defaultCurve, hasUnsavedChanges, isConnected, isInitialized, isInteracting, speedRange.max, speedRange.min]);

  useEffect(() => {
    clearEditorHistory();
    if (!isConnected && !isBlackSharkDevice) return;
    loadCurveProfiles().catch(() => {});
  }, [clearEditorHistory, externalActiveProfileId, externalDeviceCurveKey, isBlackSharkDevice, isConnected, loadCurveProfiles]);

  /* ── Chart data ── */

  const chartData = useMemo(() => {
    const offsets = smartControl.learnedOffsets || [];
    return localCurve.map((point, index) => {
      const offset = learnedOffsetForDisplay(constrainOffsetByLearningBias(offsets[index] ?? 0, currentLearningBias), speedUnit);
      return {
        temperature: point.temperature,
        rpm: point.rpm,
        coupledRpm: Math.max(curveSpeedBounds.min, Math.min(curveSpeedBounds.max, point.rpm + offset)),
        index,
      };
    });
  }, [curveSpeedBounds.max, curveSpeedBounds.min, currentLearningBias, localCurve, smartControl.learnedOffsets, speedUnit]);

  const hasLearnedOffsets = learnedOffsetStats.count > 0;
  const learningActive = config.autoControl && !!smartControl.learning && !blackSharkGearCurveMode;
  const showCoupledCurve = learningActive && hasLearnedOffsets;

  const updateProfileListOverflow = useCallback(() => {
    const el = profileListRef.current;
    if (!el) return;
    const left = el.scrollLeft > 4;
    const right = el.scrollLeft + el.clientWidth < el.scrollWidth - 4;
    setProfileListOverflow((prev) => (prev.left === left && prev.right === right ? prev : { left, right }));
  }, []);

  useEffect(() => {
    updateProfileListOverflow();
    window.addEventListener('resize', updateProfileListOverflow);
    return () => window.removeEventListener('resize', updateProfileListOverflow);
  }, [curveProfiles.length, updateProfileListOverflow]);

  const handleProfileListWheel = useCallback((event: React.WheelEvent<HTMLDivElement>) => {
    const el = profileListRef.current;
    if (!el || el.scrollWidth <= el.clientWidth) return;
    if (Math.abs(event.deltaY) > Math.abs(event.deltaX)) {
      el.scrollLeft += event.deltaY;
      event.preventDefault();
    }
  }, []);

  /* ── Point update + drag ── */

  const monotonicPushedSet = useMemo(() => new Set(monotonicPushedIndices), [monotonicPushedIndices]);
  const pointFineStep = speedUnit === 'rpm' ? 50 : 1;
  const pointCoarseStep = speedUnit === 'rpm' ? 100 : 5;
  const globalShiftStep = speedUnit === 'rpm' ? 50 : 2;
  const selectedCurvePoint = selectedPointIndex !== null ? localCurve[selectedPointIndex] ?? null : null;
  const canShiftCurveDown = useMemo(() => localCurve.some((point) => point.rpm > speedRange.min), [localCurve, speedRange.min]);
  const canShiftCurveUp = useMemo(() => localCurve.some((point) => point.rpm < speedRange.max), [localCurve, speedRange.max]);

  useEffect(() => {
    if (selectedPointIndex !== null && selectedPointIndex >= localCurve.length) {
      setSelectedPointIndex(null);
      return;
    }
    if (selectedCurvePoint) {
      setPointSpeedDraft(String(selectedCurvePoint.rpm));
      setPointTempDraft(String(selectedCurvePoint.temperature));
    } else {
      setPointSpeedDraft('');
      setPointTempDraft('');
    }
  }, [localCurve.length, selectedCurvePoint, selectedPointIndex]);

  const measureChartBounds = useCallback((): CurveChartPixelBounds | null => {
    const chartArea = chartRef.current?.querySelector('.recharts-cartesian-grid');
    if (!chartArea) {
      return null;
    }
    const rect = chartArea.getBoundingClientRect();
    return {
      top: rect.top,
      bottom: rect.bottom,
      left: rect.left,
      right: rect.right,
      // 纵轴域用坐标轴的下端（0），不是 speedRange.min —— 两者不一致时点会跳。
      yMin: FAN_CURVE_AXIS_MIN_SPEED,
      yMax: speedRange.max,
      xMin: temperatureRange.min,
      xMax: temperatureRange.max,
    };
  }, [speedRange.max, temperatureRange.max, temperatureRange.min]);

  const updatePoint = useCallback((index: number, newRpm: number) => {
    let didChange = false;
    let affected: number[] = [];

    setLocalCurve((prev) => {
      const nextState = syncCurveSpeedAtIndex(prev, index, newRpm, speedRange.min, speedRange.max);

      if (!nextState.changed) {
        return prev;
      }

      didChange = true;
      affected = nextState.affectedIndices;
      return nextState.curve;
    });

    if (didChange) {
      setEditorSession(markCurveEditorDirty);
      if (affected.length > 0) {
        triggerMonotonicHighlight(affected);
      }
    }
  }, [speedRange, triggerMonotonicHighlight]);

  // 黑鲨那 4 个点的温度可以左右拖（取值域来自后端）；取不到范围时退化为只允许纵向拖动。
  // 主机侧调速下曲线是通用 17 点，不套用这套设备侧约束（见 blackSharkGearCurveMode）。
  const canDragPointHorizontally = blackSharkGearCurveMode && blackSharkCurveTempRange !== null;

  const updatePointTemperature = useCallback((index: number, targetTemperature: number) => {
    if (!blackSharkCurveTempRange) return;
    let didChange = false;

    setLocalCurve((prev) => {
      const nextState = syncCurveTemperatureAtIndex(
        prev,
        index,
        targetTemperature,
        blackSharkCurveTempRange.minTempC,
        blackSharkCurveTempRange.maxTempC,
      );

      if (!nextState.changed) {
        return prev;
      }

      didChange = true;
      return nextState.curve;
    });

    if (didChange) {
      setEditorSession(markCurveEditorDirty);
    }
  }, [blackSharkCurveTempRange]);

  const handleDragStart = useCallback((index: number, clientX?: number, clientY?: number) => {
    setSelectedPointIndex(index);
    setDragIndex(index);
    setIsInteracting(true);
    chartBoundsRef.current = measureChartBounds();
    dragStartCurveRef.current = cloneCurvePoints(localCurveRef.current);
    dragStartOriginRef.current = clientX !== undefined && clientY !== undefined
      ? { x: clientX, y: clientY, moved: false }
      : null;
  }, [measureChartBounds]);

  const handleChartMouseDown = useCallback((event: React.MouseEvent<HTMLDivElement>) => {
    if (event.button !== 0) {
      return;
    }
    const target = event.target as HTMLElement | null;
    if (target?.closest('button, input, [data-curve-overlay-interactive]')) {
      return;
    }
    const bounds = measureChartBounds();
    if (!bounds) {
      return;
    }
    const hitIndex = findClosestCurvePointIndex(
      localCurveRef.current,
      event.clientX,
      event.clientY,
      bounds,
      { allowHorizontal: canDragPointHorizontally },
    );
    if (hitIndex !== null) {
      event.preventDefault();
      handleDragStart(hitIndex, event.clientX, event.clientY);
    } else {
      setSelectedPointIndex(null);
    }
  }, [canDragPointHorizontally, handleDragStart, measureChartBounds]);

  const handleChartTouchStart = useCallback((event: React.TouchEvent<HTMLDivElement>) => {
    if (event.touches.length === 0) {
      return;
    }
    const target = event.target as HTMLElement | null;
    if (target?.closest('button, input, [data-curve-overlay-interactive]')) {
      return;
    }
    const touch = event.touches[0];
    const bounds = measureChartBounds();
    if (!bounds) {
      return;
    }
    const hitIndex = findClosestCurvePointIndex(
      localCurveRef.current,
      touch.clientX,
      touch.clientY,
      bounds,
      { allowHorizontal: canDragPointHorizontally },
    );
    if (hitIndex !== null) {
      event.preventDefault();
      handleDragStart(hitIndex, touch.clientX, touch.clientY);
    } else {
      setSelectedPointIndex(null);
    }
  }, [canDragPointHorizontally, handleDragStart, measureChartBounds]);

  const handleDrag = useCallback((clientX: number, clientY: number, shiftKey = false) => {
    if (dragIndex === null || !chartBoundsRef.current) return;
    const origin = dragStartOriginRef.current;
    if (origin && !origin.moved) {
      if (Math.hypot(clientX - origin.x, clientY - origin.y) < 3) {
        return;
      }
      origin.moved = true;
    }

    const bounds = chartBoundsRef.current;
    const relativeY = Math.max(0, Math.min(1, (bounds.bottom - clientY) / (bounds.bottom - bounds.top)));
    const rawSpeed = bounds.yMin + relativeY * (bounds.yMax - bounds.yMin);
    const snappedSpeed = snapCurveSpeedValue(rawSpeed, speedRange.min, speedRange.max, speedUnit, stepSnapEnabled, shiftKey);
    updatePoint(dragIndex, snappedSpeed);

    if (!canDragPointHorizontally || bounds.right <= bounds.left) return;
    const relativeX = Math.max(0, Math.min(1, (clientX - bounds.left) / (bounds.right - bounds.left)));
    updatePointTemperature(dragIndex, bounds.xMin + relativeX * (bounds.xMax - bounds.xMin));
  }, [canDragPointHorizontally, dragIndex, speedRange.max, speedRange.min, speedUnit, stepSnapEnabled, updatePoint, updatePointTemperature]);

  const scheduleDrag = useCallback((clientX: number, clientY: number, shiftKey = false) => {
    pendingDragRef.current = { clientX, clientY, shiftKey };
    if (dragFrameRef.current !== null) {
      return;
    }

    dragFrameRef.current = window.requestAnimationFrame(() => {
      dragFrameRef.current = null;
      const pending = pendingDragRef.current;
      pendingDragRef.current = null;
      if (pending !== null) {
        handleDrag(pending.clientX, pending.clientY, pending.shiftKey);
      }
    });
  }, [handleDrag]);

  const checkLowRpmThreshold = useCallback((curve: types.FanCurvePoint[], fromSave = false) => {
    if (!supportsLowRpmGuidance || curve.length === 0) {
      return false;
    }
    let minPointSpeed = Infinity;
    for (const point of curve) {
      if (point.rpm < minPointSpeed) {
        minPointSpeed = point.rpm;
      }
    }
    if (minPointSpeed >= RPM_RECOMMENDED_MIN) {
      lowRpmWarnedLevelRef.current = 'none';
      return false;
    }
    const level: 'below1000' | 'below800' = minPointSpeed < RPM_HARD_FLOOR_WARN ? 'below800' : 'below1000';
    const shouldPrompt = level === 'below800'
      ? lowRpmWarnedLevelRef.current !== 'below800'
      : lowRpmWarnedLevelRef.current === 'none';
    if (!shouldPrompt) {
      return false;
    }
    lowRpmWarnedLevelRef.current = level;
    pendingLowRpmSaveRef.current = fromSave;
    setLowRpmDialogOpen(true);
    return true;
  }, [supportsLowRpmGuidance]);

  const handleDragEnd = useCallback(() => {
    if (dragFrameRef.current !== null) {
      window.cancelAnimationFrame(dragFrameRef.current);
      dragFrameRef.current = null;
    }
    pendingDragRef.current = null;
    const beforeDrag = dragStartCurveRef.current;
    dragStartCurveRef.current = null;
    dragStartOriginRef.current = null;
    setDragIndex(null);
    setTimeout(() => setIsInteracting(false), 100);
    if (beforeDrag && !curvesEqual(beforeDrag, localCurveRef.current)) {
      pushUndoSnapshot(beforeDrag);
      checkLowRpmThreshold(localCurveRef.current, false);
    }
  }, [checkLowRpmThreshold, pushUndoSnapshot]);

  const handleGlobalShiftCurve = useCallback((delta: number) => {
    const current = localCurveRef.current;
    const nextState = shiftCurveSpeeds(current, delta, speedRange.min, speedRange.max);
    if (!nextState.changed) {
      return;
    }
    pushUndoSnapshot(current);
    setLocalCurve(nextState.curve);
    setEditorSession(markCurveEditorDirty);
    if (delta < 0) {
      checkLowRpmThreshold(nextState.curve, false);
    }
  }, [checkLowRpmThreshold, pushUndoSnapshot, speedRange.max, speedRange.min]);

  const applySelectedPointSpeed = useCallback((targetSpeed: number) => {
    if (selectedPointIndex === null) {
      return;
    }
    const current = localCurveRef.current;
    const point = current[selectedPointIndex];
    if (!point) {
      return;
    }
    const nextState = syncCurveSpeedAtIndex(current, selectedPointIndex, targetSpeed, speedRange.min, speedRange.max);
    if (!nextState.changed) {
      setPointSpeedDraft(String(point.rpm));
      return;
    }
    pushUndoSnapshot(current);
    setLocalCurve(nextState.curve);
    setEditorSession(markCurveEditorDirty);
    if (nextState.affectedIndices.length > 0) {
      triggerMonotonicHighlight(nextState.affectedIndices);
    }
    checkLowRpmThreshold(nextState.curve, false);
  }, [checkLowRpmThreshold, pushUndoSnapshot, selectedPointIndex, speedRange.max, speedRange.min, triggerMonotonicHighlight]);

  const applySelectedPointTemperature = useCallback((targetTemp: number) => {
    if (selectedPointIndex === null || !blackSharkCurveTempRange) {
      return;
    }
    const current = localCurveRef.current;
    const point = current[selectedPointIndex];
    if (!point) {
      return;
    }
    const nextState = syncCurveTemperatureAtIndex(
      current,
      selectedPointIndex,
      targetTemp,
      blackSharkCurveTempRange.minTempC,
      blackSharkCurveTempRange.maxTempC,
    );
    if (!nextState.changed) {
      setPointTempDraft(String(point.temperature));
      return;
    }
    pushUndoSnapshot(current);
    setLocalCurve(nextState.curve);
    setEditorSession(markCurveEditorDirty);
  }, [blackSharkCurveTempRange, pushUndoSnapshot, selectedPointIndex]);

  const handleUndoCurve = useCallback(() => {
    const stack = curveUndoStackRef.current;
    if (stack.length === 0) {
      return;
    }
    const restored = cloneCurvePoints(stack[stack.length - 1]);
    setCurveUndoStack(stack.slice(0, -1));
    setMonotonicPushedIndices([]);
    setLocalCurve(restored);
    const baseline = curveSavedBaselineRef.current;
    if (baseline && curvesEqual(restored, baseline)) {
      setEditorSession((session) => markCurveEditorSaved(session, true));
    } else {
      setEditorSession(markCurveEditorDirty);
    }
  }, []);

  useEffect(() => {
    if (dragIndex === null) return;
    const mm = (e: MouseEvent) => { e.preventDefault(); scheduleDrag(e.clientX, e.clientY, e.shiftKey); };
    const tm = (e: TouchEvent) => {
      if (e.touches.length > 0) scheduleDrag(e.touches[0].clientX, e.touches[0].clientY, false);
    };
    const end = () => handleDragEnd();
    document.addEventListener('mousemove', mm);
    document.addEventListener('mouseup', end);
    document.addEventListener('touchmove', tm, { passive: false });
    document.addEventListener('touchend', end);
    return () => {
      document.removeEventListener('mousemove', mm);
      document.removeEventListener('mouseup', end);
      document.removeEventListener('touchmove', tm);
      document.removeEventListener('touchend', end);
      if (dragFrameRef.current !== null) {
        window.cancelAnimationFrame(dragFrameRef.current);
        dragFrameRef.current = null;
      }
      pendingDragRef.current = null;
    };
  }, [dragIndex, handleDragEnd, scheduleDrag]);

  useEffect(() => {
    if (historyOnly || !isConnected) {
      return;
    }
    const anyDialogOpen =
      createProfileDialogOpen ||
      manageProfilesDialogOpen ||
      gearEditOpen ||
      historyDisplayDialogOpen ||
      applyLearningDialogOpen ||
      lowRpmDialogOpen ||
      profileSwitchDialogOpen ||
      deleteProfileDialogOpen ||
      noiseDiagnosticOpen ||
      axisNoiseScanOpen;
    if (anyDialogOpen) {
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (
        target &&
        (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.tagName === 'SELECT' || target.isContentEditable)
      ) {
        return;
      }

      if ((event.ctrlKey || event.metaKey) && !event.shiftKey && event.key.toLowerCase() === 'z') {
        if (curveUndoStackRef.current.length > 0) {
          event.preventDefault();
          handleUndoCurve();
        }
        return;
      }

      if (selectedPointIndex === null) {
        return;
      }
      const point = localCurveRef.current[selectedPointIndex];
      if (!point) {
        return;
      }

      if (event.key === 'ArrowUp') {
        event.preventDefault();
        applySelectedPointSpeed(point.rpm + (event.shiftKey ? pointCoarseStep : pointFineStep));
      } else if (event.key === 'ArrowDown') {
        event.preventDefault();
        applySelectedPointSpeed(point.rpm - (event.shiftKey ? pointCoarseStep : pointFineStep));
      } else if (event.key === 'ArrowLeft') {
        event.preventDefault();
        setSelectedPointIndex((prev) => (prev === null ? null : Math.max(0, prev - 1)));
      } else if (event.key === 'ArrowRight') {
        event.preventDefault();
        setSelectedPointIndex((prev) => (prev === null ? null : Math.min(localCurveRef.current.length - 1, prev + 1)));
      } else if (event.key === 'Escape') {
        event.preventDefault();
        setSelectedPointIndex(null);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [
    applyLearningDialogOpen,
    applySelectedPointSpeed,
    axisNoiseScanOpen,
    createProfileDialogOpen,
    deleteProfileDialogOpen,
    gearEditOpen,
    handleUndoCurve,
    historyDisplayDialogOpen,
    historyOnly,
    isConnected,
    lowRpmDialogOpen,
    manageProfilesDialogOpen,
    noiseDiagnosticOpen,
    pointCoarseStep,
    pointFineStep,
    profileSwitchDialogOpen,
    selectedPointIndex,
  ]);

  /* ── Save / Reset ── */

  const persistCurrentCurve = useCallback(async (overrideCurve?: types.FanCurvePoint[]) => {
    if (!isConnected || isSaving) return false;
    try {
      setIsSaving(true);
      const sourceCurve = overrideCurve ?? localCurve;
      const curveToSave = normalizeFanCurve(sourceCurve, speedRange.min, speedRange.max, defaultCurve);
      const profileID = activeProfileId || (((config as any).activeFanCurveProfileId || '') as string);
      const profileName = activeProfile?.name || t('fanCurve.profiles.currentCurveName');
      await apiService.saveFanCurveProfile(profileID, profileName, curveToSave, true);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      curveSavedBaselineRef.current = cloneCurvePoints(curveToSave);
      setLocalCurve(curveToSave);
      setEditorSession((session) => markCurveEditorSaved(session, true));
      setCurveUndoStack([]);
      return true;
    } catch (e) {
      toast.error(t('fanCurve.toast.saveCurveFailed', { error: getErrorMessage(e) }));
      return false;
    } finally {
      setIsSaving(false);
    }
  }, [activeProfile?.name, activeProfileId, config, defaultCurve, isConnected, isSaving, loadCurveProfiles, localCurve, speedRange.max, speedRange.min, syncConfigFromBackend, t]);

  const saveCurve = useCallback(async () => {
    if (checkLowRpmThreshold(localCurve, true)) {
      return;
    }
    await persistCurrentCurve();
  }, [checkLowRpmThreshold, localCurve, persistCurrentCurve]);

  const raiseCurveToRecommendedRpm = useCallback(async () => {
    const floor = Math.min(speedRange.max, RPM_RECOMMENDED_MIN);
    const adjusted = normalizeFanCurve(
      localCurve.map((point) => ({
        ...point,
        rpm: Math.max(floor, point.rpm),
      })),
      speedRange.min,
      speedRange.max,
      defaultCurve,
    );
    pushUndoSnapshot(localCurve);
    lowRpmWarnedLevelRef.current = 'none';
    setLocalCurve(adjusted);
    setEditorSession(markCurveEditorDirty);
    setLowRpmDialogOpen(false);
    if (pendingLowRpmSaveRef.current) {
      pendingLowRpmSaveRef.current = false;
      await persistCurrentCurve(adjusted);
    }
  }, [defaultCurve, localCurve, persistCurrentCurve, pushUndoSnapshot, speedRange.max, speedRange.min]);

  const confirmLowRpmKeep = useCallback(async () => {
    setLowRpmDialogOpen(false);
    if (pendingLowRpmSaveRef.current) {
      pendingLowRpmSaveRef.current = false;
      await persistCurrentCurve();
    }
  }, [persistCurrentCurve]);

  const getSafeProfileName = useCallback((input: string, fallback: string) => {
    const name = (input || '').trim() || fallback;
    const runes = Array.from(name);
    return runes.slice(0, 6).join('');
  }, []);

  const trimProfileNameToLimit = useCallback((value: string) => {
    return Array.from(value).slice(0, 6).join('');
  }, []);

  const handleNewProfileInputChange = useCallback((value: string, composing: boolean) => {
    if (composing || isNewProfileComposing) {
      setNewProfileNameInput(value);
      return;
    }
    setNewProfileNameInput(trimProfileNameToLimit(value));
  }, [isNewProfileComposing, trimProfileNameToLimit]);

  const handleEditingProfileInputChange = useCallback((value: string, composing: boolean) => {
    if (composing || isEditingProfileComposing) {
      setEditingProfileName(value);
      return;
    }
    setEditingProfileName(trimProfileNameToLimit(value));
  }, [isEditingProfileComposing, trimProfileNameToLimit]);

  const startEditingProfile = useCallback((profile: CurveProfile) => {
    setEditingProfileId(profile.id);
    setEditingProfileName(trimProfileNameToLimit(profile.name || ''));
    setIsEditingProfileComposing(false);
  }, [trimProfileNameToLimit]);

  const saveEditingProfileName = useCallback(async (profile: CurveProfile) => {
    const safeName = getSafeProfileName(editingProfileName, profile.name);
    if (safeName === profile.name) {
      setEditingProfileId('');
      return;
    }
    try {
      setProfileOpLoading(true);
      await apiService.saveFanCurveProfile(profile.id, safeName, profile.curve || localCurve, false);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      setEditingProfileId('');
      toast.success(t('fanCurve.toast.profileRenamed'));
    } catch (e) {
      toast.error(t('fanCurve.toast.renameFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [editingProfileName, getSafeProfileName, loadCurveProfiles, localCurve, syncConfigFromBackend, t]);

  const duplicateProfile = useCallback(async (profile: CurveProfile) => {
    const suffix = t('fanCurve.profiles.copySuffix');
    const suffixRunes = Array.from(suffix);
    const baseRunes = Array.from((profile.name || '').trim() || t('fanCurve.profiles.currentCurveName'));
    const maxBaseLen = Math.max(1, 6 - suffixRunes.length);
    const candidate = getSafeProfileName(`${baseRunes.slice(0, maxBaseLen).join('')}${suffix}`, suffix);
    try {
      setProfileOpLoading(true);
      await apiService.saveFanCurveProfile('', candidate, profile.curve || localCurve, false);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      toast.success(t('fanCurve.toast.profileDuplicated'));
    } catch (e) {
      toast.error(t('fanCurve.toast.duplicateFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [getSafeProfileName, loadCurveProfiles, localCurve, syncConfigFromBackend, t]);

  const formatProfileSummary = useCallback((curve?: types.FanCurvePoint[]) => {
    if (!Array.isArray(curve) || curve.length === 0) return '--';
    const sorted = [...curve].sort((a, b) => a.temperature - b.temperature);
    const minTemp = sorted[0]?.temperature ?? 0;
    const maxTemp = sorted[sorted.length - 1]?.temperature ?? 0;
    const minSpeed = Math.min(...sorted.map((p) => p.rpm));
    const maxSpeed = Math.max(...sorted.map((p) => p.rpm));
    return `${minTemp}–${maxTemp}°C · ${formatSpeedValue(minSpeed)}–${formatSpeedValue(maxSpeed)}${speedUnitSuffix}`;
  }, [speedUnitSuffix]);

  const applyProfileSwitch = useCallback(async (id: string) => {
    try {
      setProfileOpLoading(true);
      await apiService.setActiveFanCurveProfile(id);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      clearEditorHistory();
      toast.success(t('fanCurve.toast.profileSwitched'));
    } catch (e) {
      toast.error(t('fanCurve.toast.switchFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [clearEditorHistory, loadCurveProfiles, syncConfigFromBackend, t]);

  const switchProfile = useCallback(async (id: string) => {
    const decision = requestCurveProfileSwitch(editorSession, id, activeProfileId);
    setEditorSession(decision.session);
    if (decision.requiresConfirmation) {
      setProfileSwitchDialogOpen(true);
      return;
    }
    if (decision.nextProfileId) {
      await applyProfileSwitch(decision.nextProfileId);
    }
  }, [activeProfileId, applyProfileSwitch, editorSession]);

  const confirmProfileSwitch = useCallback(async (action: 'save' | 'discard') => {
    if (!pendingProfileId && !pendingTab) return;
    let saveSucceeded = true;
    if (action === 'save') {
      saveSucceeded = await persistCurrentCurve();
    }
    const decision = resolveCurveEditorSwitch(editorSession, action, saveSucceeded);
    if (action === 'save' && !saveSucceeded) return;
    if (action === 'discard') {
      clearEditorHistory();
    }
    setEditorSession(decision.session);
    setProfileSwitchDialogOpen(false);
    if (decision.completeTab) {
      completePendingTabChange();
      return;
    }
    if (decision.nextProfileId) {
      await applyProfileSwitch(decision.nextProfileId);
    }
  }, [applyProfileSwitch, clearEditorHistory, completePendingTabChange, editorSession, pendingProfileId, pendingTab, persistCurrentCurve]);

  const createNewProfile = useCallback(async () => {
    const newProfileName = t('fanCurve.profiles.newCurveName');
    const safeName = getSafeProfileName(newProfileNameInput, newProfileName);
    try {
      setProfileOpLoading(true);
      await apiService.saveFanCurveProfile('', safeName, localCurve, true);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      setNewProfileNameInput('');
      setCreateProfileDialogOpen(false);
      toast.success(t('fanCurve.toast.profileSavedAsNew'));
    } catch (e) {
      toast.error(t('fanCurve.toast.saveAsFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [getSafeProfileName, loadCurveProfiles, localCurve, newProfileNameInput, syncConfigFromBackend, t]);

  const removeProfile = useCallback(async () => {
    if (!pendingDeleteProfileId) return;
    const deletingActiveProfile = pendingDeleteProfileId === activeProfileId;
    try {
      setProfileOpLoading(true);
      await apiService.deleteFanCurveProfile(pendingDeleteProfileId);
      if (deletingActiveProfile) {
        await loadCurveProfiles();
      } else {
        setCurveProfiles((profiles) => profiles.filter((profile) => profile.id !== pendingDeleteProfileId));
      }
      await syncConfigFromBackend();
      setSelectedExportProfileIds((ids) => ids.filter((id) => id !== pendingDeleteProfileId));
      if (editingProfileId === pendingDeleteProfileId) {
        setEditingProfileId('');
      }
      setDeleteProfileDialogOpen(false);
      setPendingDeleteProfileId('');
      toast.success(t('fanCurve.toast.profileDeleted'));
    } catch (e) {
      toast.error(t('fanCurve.toast.deleteFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [activeProfileId, editingProfileId, loadCurveProfiles, pendingDeleteProfileId, syncConfigFromBackend, t]);

  const exportProfiles = useCallback(async (destination: 'clipboard' | 'file') => {
    try {
      if (selectedExportProfileIds.length === 0) {
        toast.error(t('fanCurve.importExport.selectAtLeastOne'));
        return;
      }
      if (hasUnsavedChanges) {
        const ok = await persistCurrentCurve();
        if (!ok) {
          return;
        }
      }
      if (destination === 'file') {
        const savedPath = await apiService.exportFanCurveProfilesToFile(selectedExportProfileIds);
        if (!savedPath) return;
        toast.success(t('fanCurve.toast.exportFileCreated'));
        return;
      }
      const code = await apiService.exportFanCurveProfiles(selectedExportProfileIds);
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(code);
      } else {
        await ClipboardSetText(code);
      }
      toast.success(t('fanCurve.toast.exportCopied'));
    } catch (e) {
      toast.error(t('fanCurve.toast.exportFailed', { error: getErrorMessage(e) }));
    }
  }, [hasUnsavedChanges, persistCurrentCurve, selectedExportProfileIds, t]);

  const loadProfileImportFile = useCallback(async (file?: File) => {
    if (!file) return;
    try {
      const code = (await file.text()).trim();
      if (!code) throw new Error(t('fanCurve.importExport.emptyFile'));
      setImportCode(code);
      toast.success(t('fanCurve.toast.importFileLoaded', { name: file.name }));
    } catch (error) {
      toast.error(t('fanCurve.toast.importFileFailed', { error: getErrorMessage(error) }));
    }
  }, [t]);

  const handleProfileFileDrop = useCallback((event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    setIsImportDragging(false);
    void loadProfileImportFile(event.dataTransfer.files?.[0]);
  }, [loadProfileImportFile]);

  const importProfiles = useCallback(async () => {
    const code = importCode.trim();
    if (!code) {
      toast.error(t('fanCurve.toast.importMissingCode'));
      return;
    }
    if (hasUnsavedChanges) {
      const saved = await persistCurrentCurve();
      if (!saved) return;
    }
    try {
      setProfileOpLoading(true);
      await apiService.importFanCurveProfiles(code);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      setImportCode('');
      toast.success(t('fanCurve.toast.importSucceeded'));
    } catch (e) {
      toast.error(t('fanCurve.toast.importFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [hasUnsavedChanges, importCode, loadCurveProfiles, persistCurrentCurve, syncConfigFromBackend, t]);

  const resetCurve = useCallback(async () => {
    // 黑鲨手动模式：重置为黑鲨默认值并下发一次。
    if (!config.autoControl && isBlackSharkDevice) {
      // 内联判断能力位：supportsManualGears memo 声明在 resetCurve 之后，引用会踩 TDZ。
      const manualGearOk = supportsManualGearsFromCapabilities(
        isConnected && runtimeDeviceCapabilities ? runtimeDeviceCapabilities : runtimeDeviceProfile?.capabilities,
      );
      if (!manualGearOk) {
        toast.error(t('fanCurve.manualGear.unavailable'));
        return;
      }
      try {
        // 默认值来源与卡片一致（buildDraftFrom(null) 会带黑鲨标记）；内联取默认值与量程以免引用后声明的 memo。
        const range = getManualGearValueRange(speedUnit, isBlackSharkDevice);
        const defaults: ManualGearRpmMap = {};
        getEffectiveManualGearPresets(null, speedUnit, isBlackSharkDevice).forEach((preset) => {
          defaults[preset.gear] = {};
          preset.levels.forEach((lv) => { defaults[preset.gear][lv.level] = lv.rpm; });
        });
        const normalized = normalizeManualGearRpmMap(defaults, range.min, range.max, speedUnit, isBlackSharkDevice);
        // 重置覆盖转速值、挡位(标准)与档级(中)三项，并下发一次。
        const next = types.AppConfig.createFrom({
          ...config,
          manualGearRpm: normalized,
          manualGear: '标准',
          manualLevel: '中',
        });
        await apiService.updateConfig(next);
        const ok = await apiService.setManualGear('标准', '中');
        if (!ok) {
          throw new Error(t('fanCurve.manualGear.unavailable'));
        }
        onConfigChange(types.AppConfig.createFrom(await apiService.getConfig()));
        toast.success(t('fanCurve.manualGear.rpmSaved'));
      } catch (err) {
        toast.error(t('fanCurve.manualGear.rpmSaveFailed', { error: getErrorMessage(err) }));
      }
      return;
    }

    pushUndoSnapshot(localCurveRef.current);
    setLocalCurve(normalizeFanCurve(defaultCurve, speedRange.min, speedRange.max, defaultCurve));
    setEditorSession(markCurveEditorDirty);
  }, [config, defaultCurve, isBlackSharkDevice, isConnected, onConfigChange, pushUndoSnapshot, runtimeDeviceCapabilities, runtimeDeviceProfile, speedRange.max, speedRange.min, speedUnit, t]);

  // 一键把四个档位的曲线恢复成出厂值。
  const resetCoolingAll = useCallback(async () => {
    try {
      if (!(await apiService.resetBlackSharkCooling())) {
        toast.error(t('controlPanel.blackShark.actionFailed'));
        return;
      }
      // 四个方案曲线都换了，必须重拉方案，否则图表（读的是 localCurve）不会变。
      await loadCurveProfiles();
      toast.success(t('fanCurve.actions.coolingResetOk'));
    } catch (err) {
      toast.error(t('controlPanel.blackShark.actionFailed') + ' ' + getErrorMessage(err));
    }
  }, [loadCurveProfiles, t]);

  // 「重置」按钮：与官方那颗一致 —— 点一次直接执行。
  // 黑鲨按屏幕走（开变频 → 四档恢复出厂曲线；关变频 → 重置手动挡位转速），其它设备沿用原本的"重置曲线"。
  // 主机侧调速模式下没有四档曲线（曲线集是通用单方案 17 点），只重置当前这条曲线。
  const handleResetButton = useCallback(() => {
    if (isBlackSharkDevice && isConnected && config.autoControl && !config.blackSharkHostVariableSpeed) {
      void resetCoolingAll();
      return;
    }
    void resetCurve();
  }, [config.autoControl, config.blackSharkHostVariableSpeed, isConnected, isBlackSharkDevice, resetCoolingAll, resetCurve]);

  /* ── Auto control / smart control handlers ── */

  const handleAutoControlChange = useCallback(async (enabled: boolean) => {
    try {
      await apiService.setAutoControl(enabled);
      const latest = await apiService.getConfig();
      onConfigChange(types.AppConfig.createFrom({ ...latest, autoControl: enabled }));
    } catch (err) {
      toast.error(t('controlPanel.fan.autoControlApplyFailed', { error: getErrorMessage(err) }));
      try {
        onConfigChange(types.AppConfig.createFrom(await apiService.getConfig()));
      } catch {
        /* noop */
      }
    }
  }, [onConfigChange, t]);

  const updateSmartControlConfig = useCallback(async (patch: Partial<types.SmartControlConfig> & { learningBias?: string }) => {
    setLearningConfigLoading(true);
    try {
      const nextSmartControl = types.SmartControlConfig.createFrom({ ...smartControl, ...patch });
      const nextConfig = types.AppConfig.createFrom({ ...config, smartControl: nextSmartControl });
      await apiService.updateConfig(nextConfig);
      onConfigChange(nextConfig);
    } catch (err) {
      toast.error(t('fanCurve.toast.saveLearningFailed'), { description: getErrorMessage(err) });
    } finally {
      setLearningConfigLoading(false);
    }
  }, [config, onConfigChange, smartControl, t]);

  const handleLearningToggle = useCallback((enabled: boolean) => {
    void updateSmartControlConfig({ learning: enabled });
  }, [updateSmartControlConfig]);

  const handleTemperatureRisePredictionToggle = useCallback((enabled: boolean) => {
    void updateSmartControlConfig({ temperatureRisePrediction: enabled } as Partial<types.SmartControlConfig>);
  }, [updateSmartControlConfig]);

  const handleLearningBiasChange = useCallback((value: string) => {
    void updateSmartControlConfig({ learningBias: normalizeLearningBias(value) });
  }, [updateSmartControlConfig]);

  const commitTargetTemp = useCallback((value: number) => {
    const normalized = normalizeTargetTemp(value);
    setTargetTempDraft(normalized);
    if (normalized === normalizeTargetTemp((smartControl as any).targetTemp ?? 68)) {
      return;
    }
    void updateSmartControlConfig({ targetTemp: normalized });
  }, [smartControl.targetTemp, updateSmartControlConfig]);

  const handleTargetTempSliderChange = useCallback((value: number) => {
    setTargetTempDraft(normalizeTargetTemp(value));
  }, []);

  const handleTargetTempSliderCommit = useCallback(() => {
    commitTargetTemp(targetTempDraft);
  }, [commitTargetTemp, targetTempDraft]);

  const handleTargetTempInputChange = useCallback((value: number) => {
    setTargetTempDraft(normalizeTargetTemp(value));
  }, []);

  const handleTargetTempInputBlur = useCallback(() => {
    commitTargetTemp(targetTempDraft);
  }, [commitTargetTemp, targetTempDraft]);

  const handleApplyLearnedOffsetsToCurve = useCallback(async () => {
    if (!isConnected || !hasLearnedOffsets || blackSharkGearCurveMode || learningApplyLoading) return;
    const offsets = smartControl.learnedOffsets || [];
    const rawMerged = localCurve.map((point, index) => {
      const delta = learnedOffsetForDisplay(
        constrainOffsetByLearningBias(typeof offsets[index] === 'number' ? offsets[index] : 0, currentLearningBias),
        speedUnit,
      );
      return {
        temperature: point.temperature,
        rpm: Math.max(speedRange.min, Math.min(speedRange.max, point.rpm + delta)),
      };
    });
    const mergedCurve = normalizeFanCurve(rawMerged, speedRange.min, speedRange.max, defaultCurve);
    const profileID = activeProfileId || (((config as any).activeFanCurveProfileId || '') as string);
    const profileName = activeProfile?.name || t('fanCurve.profiles.currentCurveName');

    setLearningApplyLoading(true);
    try {
      await apiService.saveFanCurveProfile(profileID, profileName, mergedCurve, true);
      await apiService.resetLearnedOffsets();
      await loadCurveProfiles();
      await syncConfigFromBackend();
      curveSavedBaselineRef.current = cloneCurvePoints(mergedCurve);
      setLocalCurve(mergedCurve);
      setEditorSession((session) => markCurveEditorSaved(session, true));
      setCurveUndoStack([]);
      setApplyLearningDialogOpen(false);
      toast.success(t('fanCurve.toast.learningApplied'));
    } catch (err) {
      toast.error(t('fanCurve.toast.learningApplyFailed'), { description: getErrorMessage(err) });
    } finally {
      setLearningApplyLoading(false);
    }
  }, [activeProfile?.name, activeProfileId, blackSharkGearCurveMode, config, currentLearningBias, defaultCurve, hasLearnedOffsets, isConnected, learningApplyLoading, loadCurveProfiles, localCurve, smartControl.learnedOffsets, speedRange.max, speedRange.min, speedUnit, syncConfigFromBackend, t]);

  const handleResetLearnedOffsets = useCallback(async () => {
    setLearningResetLoading(true);
    try {
      await apiService.resetLearnedOffsets();
      await syncConfigFromBackend();
      toast.success(t('fanCurve.toast.learningReset'), { description: t('fanCurve.toast.learningResetDescription'), duration: 2400 });
    } catch (err) {
      toast.error(t('fanCurve.toast.resetFailed'), { description: getErrorMessage(err) });
    } finally {
      setLearningResetLoading(false);
    }
  }, [syncConfigFromBackend, t]);

  /* ── Manual gear ── */

  const customGearRpm = useMemo(() => {
    return ((config as any).manualGearRpm ?? null) as ManualGearRpmMap | null;
  }, [config]);

  const manualGearPresets = useMemo(() => {
    // 必须带黑鲨标记：否则这张卡片会显示飞智那套默认值（静音1700/标准2400/强劲3000/超频3700）。
    return getEffectiveManualGearPresets(customGearRpm, speedUnit, isBlackSharkDevice);
  }, [blackSharkRpmsRev, customGearRpm, isBlackSharkDevice, speedUnit]);

  const manualGearDefaultPresets = useMemo(() => {
    // 黑鲨用自己那套默认值：后端 BlackSharkManualGearPresets() 从可选项量程派生（1410..4000 等分 12 点 → 四档 × 三档级），前端不硬编码数值。
    return getManualGearDefaultPresets(speedUnit, isBlackSharkDevice);
  }, [blackSharkRpmsRev, isBlackSharkDevice, speedUnit]);

  const manualGearValueRange = useMemo(() => {
    // 黑鲨量程来自后端（deviceproto.BlackSharkMinRPM/MaxRPM = 1410..4000），不是通用的 800..4500。
    return getManualGearValueRange(speedUnit, isBlackSharkDevice);
  }, [blackSharkRpmsRev, isBlackSharkDevice, speedUnit]);

  const supportsManualGears = supportsManualGearsFromCapabilities(
    isConnected && runtimeDeviceCapabilities ? runtimeDeviceCapabilities : runtimeDeviceProfile?.capabilities,
  );
  const flyDigiCapabilitySource = useMemo(
    () => (fanData ? fanData : (storeFlyDigiCapability ? { flyDigiCapability: storeFlyDigiCapability } : null)),
    [fanData, storeFlyDigiCapability],
  );
  const flyDigiCapability = useMemo(() => getFlyDigiRuntimeCapability(flyDigiCapabilitySource as any), [flyDigiCapabilitySource]);
  const isManualGearAllowed = useCallback((gear: string) => (
    supportsManualGears && isManualGearAllowedForFlyDigi(gear, flyDigiCapability)
  ), [flyDigiCapability, supportsManualGears]);
  const manualGearLimitHint = flyDigiCapability?.available && flyDigiCapability.maxGearLabel && flyDigiCapability.maxRpm
    ? t('fanCurve.manualGear.runtimeLimit', { gear: getManualGearLabel(flyDigiCapability.maxGearLabel), rpm: flyDigiCapability.maxRpm })
    : '';

  const manualPoints = useMemo(() => {
    return manualGearPresets.flatMap((preset, gearIndex) => preset.levels.map((item, levelIndex) => ({
      key: `${preset.gear}-${item.level}`,
      gear: preset.gear,
      level: item.level,
      rpm: item.rpm,
      gearIndex,
      levelIndex,
      colorClass: preset.colorClass,
      borderClass: preset.borderClass,
      bgClass: preset.bgClass,
    })));
  }, [manualGearPresets]);

  const selectedManualPointIndex = useMemo(() => {
    const selected = manualPoints.findIndex((p) => p.gear === (config.manualGear || '标准') && p.level === (config.manualLevel || '中'));
    return selected >= 0 ? selected : 4;
  }, [config.manualGear, config.manualLevel, manualPoints]);

  const rememberedManualGearLevels = useMemo(() => {
    return ((config as any).manualGearLevels ?? {}) as Record<string, string>;
  }, [config]);

  const applyManualGearPreset = useCallback(async (gear: string, level: string) => {
    if (!supportsManualGears) {
      toast.error(t('fanCurve.manualGear.unavailable'));
      return;
    }
    if (!isManualGearAllowed(gear)) {
      toast.error(t('fanCurve.manualGear.runtimeUnavailable', { gear: getManualGearLabel(gear), limit: manualGearLimitHint }));
      return;
    }
    try {
      const ok = await apiService.setManualGear(gear, level);
      if (!ok) {
        throw new Error(t('fanCurve.manualGear.unavailable'));
      }
      const latest = await apiService.getConfig();
      onConfigChange(types.AppConfig.createFrom(latest));
    } catch (err) {
      toast.error(t('fanCurve.manualGear.applyFailed', { error: getErrorMessage(err) }));
    }
  }, [isManualGearAllowed, manualGearLimitHint, onConfigChange, supportsManualGears, t]);

  const handleManualPointSelect = useCallback(async (index: number) => {
    const selected = manualPoints[index];
    if (!selected) return;
    await applyManualGearPreset(selected.gear, selected.level);
  }, [applyManualGearPreset, manualPoints]);

  const handleGearCardSelect = useCallback(async (gear: string) => {
    const rememberedLevel = rememberedManualGearLevels[gear];
    const nextLevel = rememberedLevel === '低' || rememberedLevel === '中' || rememberedLevel === '高'
      ? rememberedLevel
      : (config.manualLevel || '中');
    await applyManualGearPreset(gear, nextLevel);
  }, [applyManualGearPreset, config.manualLevel, rememberedManualGearLevels]);

  /* ── Manual gear speed editor ── */

  const [draftGearRpm, setDraftGearRpm] = useState<ManualGearRpmMap>({});
  const [draftGearText, setDraftGearText] = useState<Record<string, Record<string, string>>>({});
  const [gearRpmSaving, setGearRpmSaving] = useState(false);

  const buildDraftTextFrom = useCallback((map: ManualGearRpmMap) => {
    const textMap: Record<string, Record<string, string>> = {};
    for (const [gear, levels] of Object.entries(map)) {
      textMap[gear] = {};
      for (const [level, rpm] of Object.entries(levels || {})) {
        textMap[gear][level] = String(rpm);
      }
    }
    return textMap;
  }, []);

  const buildDraftFrom = useCallback((source: ManualGearRpmMap | null): ManualGearRpmMap => {
    const base: ManualGearRpmMap = {};
    // 必须带黑鲨标记：否则「重置」会恢复成飞智那套默认值。
    getEffectiveManualGearPresets(source, speedUnit, isBlackSharkDevice).forEach((preset) => {
      base[preset.gear] = {};
      preset.levels.forEach((lv) => {
        base[preset.gear][lv.level] = Math.max(manualGearValueRange.min, Math.min(manualGearValueRange.max, lv.rpm));
      });
    });
    return base;
  }, [blackSharkRpmsRev, isBlackSharkDevice, manualGearValueRange.max, manualGearValueRange.min, speedUnit]);

  const openGearEditor = useCallback(() => {
    if (!supportsManualGears) {
      toast.error(t('fanCurve.manualGear.unavailable'));
      return;
    }
    const nextDraft = buildDraftFrom(customGearRpm);
    setDraftGearRpm(nextDraft);
    setDraftGearText(buildDraftTextFrom(nextDraft));
    setGearEditOpen(true);
  }, [buildDraftFrom, buildDraftTextFrom, customGearRpm, supportsManualGears, t]);

  const setDraftRpmText = useCallback((gear: string, level: string, raw: string) => {
    setDraftGearText((prev) => ({
      ...prev,
      [gear]: { ...(prev[gear] ?? {}), [level]: raw },
    }));
    const trimmed = raw.trim();
    if (trimmed === '') return;
    const parsed = Number(trimmed);
    if (!Number.isFinite(parsed)) return;
    setDraftGearRpm((prev) => ({
      ...prev,
      [gear]: { ...(prev[gear] ?? {}), [level]: Math.round(parsed) },
    }));
  }, []);

  const gearDraftValidation = useMemo(() => {
    const invalidKeys = new Set<string>();
    const parsedMap: ManualGearRpmMap = {};
    let prev = manualGearValueRange.min;

    for (const preset of manualGearDefaultPresets) {
      parsedMap[preset.gear] = {};
      for (const lv of preset.levels) {
        const key = `${preset.gear}:${lv.level}`;
        const rawText = (draftGearText[preset.gear]?.[lv.level] ?? String(draftGearRpm[preset.gear]?.[lv.level] ?? lv.rpm)).trim();
        const parsed = rawText === '' ? NaN : Number(rawText);
        if (!Number.isFinite(parsed) || !Number.isInteger(parsed)) {
          invalidKeys.add(key);
          continue;
        }
        if (parsed < manualGearValueRange.min || parsed > manualGearValueRange.max) {
          invalidKeys.add(key);
        } else if (parsed < prev) {
          invalidKeys.add(key);
        } else {
          prev = parsed;
        }
        parsedMap[preset.gear][lv.level] = parsed;
      }
    }

    return {
      valid: invalidKeys.size === 0,
      invalidKeys,
      parsedMap,
    };
  }, [draftGearRpm, draftGearText, manualGearDefaultPresets, manualGearValueRange.max, manualGearValueRange.min]);

  const saveGearRpm = useCallback(async () => {
    if (!supportsManualGears) {
      toast.error(t('fanCurve.manualGear.unavailable'));
      return;
    }
    if (!gearDraftValidation.valid) {
      toast.error(t('fanCurve.manualGear.validationRangeOrder', {
        min: `${formatSpeedValue(manualGearValueRange.min)}${speedUnitSuffix}`,
        max: `${formatSpeedValue(manualGearValueRange.max)}${speedUnitSuffix}`,
      }));
      return;
    }
    setGearRpmSaving(true);
    try {
      const normalized = normalizeManualGearRpmMap(gearDraftValidation.parsedMap, manualGearValueRange.min, manualGearValueRange.max, speedUnit, isBlackSharkDevice);
      const next = types.AppConfig.createFrom({ ...config, manualGearRpm: normalized });
      await apiService.updateConfig(next);
      const ok = await apiService.setManualGear(next.manualGear || '标准', next.manualLevel || '中');
      if (!ok) {
        throw new Error(t('fanCurve.manualGear.unavailable'));
      }
      const latest = await apiService.getConfig();
      onConfigChange(types.AppConfig.createFrom(latest));
      setGearEditOpen(false);
      toast.success(t('fanCurve.manualGear.rpmSaved'));
    } catch (err) {
      toast.error(t('fanCurve.manualGear.rpmSaveFailed', { error: getErrorMessage(err) }));
    } finally {
      setGearRpmSaving(false);
    }
  }, [config, gearDraftValidation, isBlackSharkDevice, manualGearValueRange.max, manualGearValueRange.min, onConfigChange, speedUnit, speedUnitSuffix, supportsManualGears, t]);

  const CustomDot = useCallback((props: any): React.ReactElement<SVGElement> => {
    const { cx, cy, index, payload } = props;
    if (cx === undefined || cy === undefined) return <g />;
    return (
      <DraggablePoint
        key={`dot-${index}`}
        cx={cx}
        cy={cy}
        index={index}
        temperature={payload.temperature}
        speed={payload.rpm}
        unitSuffix={speedUnitSuffix}
        onDragStart={handleDragStart}
        isActive={dragIndex === index}
        isSelected={selectedPointIndex === index}
        isMonotonicAffected={monotonicPushedSet.has(index)}
        allowHorizontal={canDragPointHorizontally}
      />
    );
  }, [canDragPointHorizontally, dragIndex, handleDragStart, monotonicPushedSet, selectedPointIndex, speedUnitSuffix]);

  return (
    <div
      data-theme-section="curve-page"
      data-page-reveal={initialFocusTarget === 'history-details'
        ? 'cards-reverse'
        : (!isConnected && !initialFocusTarget ? 'cards-delayed' : (!initialFocusTarget ? 'cards' : undefined))}
      className={clsx('relative space-y-4', !historyOnly && 'px-1 pb-2')}
    >
      {!historyOnly && isConnected && (
        <>
        <m.div
          data-theme-card="curve-header"
          initial={initialFocusTarget ? false : { opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.2 }}
          className="relative px-1 py-1"
        >
          <div className="space-y-2.5">
            <div className="flex flex-wrap items-center justify-between gap-2.5">
              <div className="flex min-w-0 items-center gap-2.5">
                <div data-theme-ui="setting-section-icon" className="flex h-7 w-7 shrink-0 items-center justify-center rounded-[9px] bg-muted/70 text-primary">
                  <Spline className="h-4 w-4 shrink-0" />
                </div>
                <h2 className="text-base font-semibold tracking-[-0.015em] text-foreground">{t('fanCurve.title')}</h2>
                {hasUnsavedChanges && <Badge variant="warning">{t('fanCurve.badges.unsaved')}</Badge>}
                {isInteracting && <Badge variant="info">{t('fanCurve.badges.editing')}</Badge>}
              </div>
              <div className="flex shrink-0 flex-wrap items-center gap-2.5">
                <ToggleSwitch enabled={config.autoControl} onChange={handleAutoControlChange} label={t('fanCurve.actions.smartControl')} size="sm" color="blue" />
                {/* 「重置」：与官方那颗一致 —— 文案「重置」、点一次就执行、不做二次确认。
                    黑鲨按屏幕走（开变频 → 四档恢复出厂曲线；关变频 → 重置手动挡位转速），
                    其它设备仍是重置曲线。 */}
                <Button variant="secondary" size="sm" className="rounded-lg" onClick={handleResetButton} icon={<RotateCw className="h-3.5 w-3.5" />}>
                  {t('fanCurve.actions.reset')}
                </Button>
                <Button variant="primary" size="sm" className="rounded-lg" onClick={saveCurve} disabled={!hasUnsavedChanges} loading={isSaving} icon={<Check className="h-3.5 w-3.5" />}>
                  {t('common.actions.save')}
                </Button>
              </div>
            </div>

            <div data-curve-profile-row className="flex min-w-0 items-center gap-2.5">
              <div data-curve-profile-toolbar className="flex min-w-0 flex-1 items-center gap-1.5 rounded-2xl border border-border/70 bg-card/85 p-1.5 shadow-sm shadow-black/5">
                <div className="relative min-w-0 flex-1">
                  <div
                    ref={profileListRef}
                    data-curve-profile-list
                    onScroll={updateProfileListOverflow}
                    onWheel={handleProfileListWheel}
                    className="flex min-w-0 items-center gap-1.5 overflow-x-auto px-0.5 py-0.5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
                  >
                    {curveProfiles.map((profile) => {
                      const isActive = profile.id === activeProfileId;
                      return (
                        <button
                          key={profile.id}
                          type="button"
                          onClick={() => void switchProfile(profile.id)}
                          disabled={profileOpLoading}
                          className={clsx(
                            'h-8 shrink-0 cursor-pointer truncate whitespace-nowrap rounded-full border px-3.5 text-center text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-60',
                            isActive
                              ? 'border-primary/40 bg-primary/10 text-primary shadow-xs'
                              : 'border-border/70 bg-background/55 text-muted-foreground hover:border-border hover:bg-muted/65 hover:text-foreground',
                          )}
                          aria-current={isActive ? 'true' : undefined}
                        >
                          {profile.name}
                        </button>
                      );
                    })}
                  </div>
                  {profileListOverflow.left && (
                    <div aria-hidden="true" className="pointer-events-none absolute inset-y-0 left-0 w-5 bg-gradient-to-r from-card/90 to-transparent" />
                  )}
                  {profileListOverflow.right && (
                    <div aria-hidden="true" className="pointer-events-none absolute inset-y-0 right-0 w-5 bg-gradient-to-l from-card/90 to-transparent" />
                  )}
                </div>
                <Button
                  variant="secondary"
                  size="sm"
                  className="shrink-0 rounded-lg"
                  onClick={() => setCreateProfileDialogOpen(true)}
                  disabled={profileOpLoading}
                  icon={<Plus className="h-3.5 w-3.5" />}
                >
                  {t('fanCurve.profiles.add')}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  className="shrink-0 rounded-lg"
                  onClick={() => {
                    setSelectedExportProfileIds(activeProfileId ? [activeProfileId] : []);
                    setEditingProfileId('');
                    setManageDialogTab('list');
                    setManageProfilesDialogOpen(true);
                  }}
                  disabled={profileOpLoading || curveProfiles.length === 0}
                  icon={<Settings2 className="h-3.5 w-3.5" />}
                >
                  {t('fanCurve.profiles.manage')}
                </Button>
              </div>
            </div>
          </div>
        </m.div>

        {/* 供电状态提示：0x06 状态帧第 3 字节 = 供电模式位（0x00 = USB 供电，0x01 = 专用电源）。
            USB 供电下设备转速被供电能力封顶（官方 App 同款红字），只提示、不参与任何控制逻辑。
            取不到数据时按专用电源处理，避免连接瞬间先闪一条误报。 */}
        <LiveBlackSharkUsbSupplyWarning visibleDevice={isBlackSharkDevice} fanDataProp={fanData} />

        {/* 手动挡位面板直接接入黑鲨：0x24 写 form=value，档位映射 = 飞智 静音/标准/强劲/超频 ↔ 黑鲨 1/2/3/4。 */}

        <AnimatePresence>
          {!config.autoControl && isConnected && supportsManualGears && (
            <m.div
              initial={initialFocusTarget ? false : { opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              className="overflow-hidden"
            >
              <div data-theme-card="curve-manual-gears" className="rounded-2xl border border-border/70 bg-card p-3.5 space-y-3 shadow-sm">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="text-sm font-medium">{t('fanCurve.manualGear.title')}</span>
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-xs text-muted-foreground">{manualGearLimitHint || t('fanCurve.manualGear.defaultSliderHint')}</span>
                    <Button variant="secondary" size="sm" onClick={openGearEditor} icon={<Pencil className="h-3.5 w-3.5" />}>
                      {t('fanCurve.manualGear.customize')}
                    </Button>
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-2.5 md:grid-cols-4">
                  {manualGearPresets.map((preset) => {
                    const isActiveGear = (config.manualGear || '标准') === preset.gear;
                    const rememberedLevel = isActiveGear
                      ? (config.manualLevel || '中')
                      : rememberedManualGearLevels[preset.gear];
                    // 黑鲨那套转速由后端标定表派生；后端还没给到时 levels 会是空数组（见 lib/manualGearPresets），
                    // 此时必须显示 "--" 并禁用卡片，不能让 activeLevel.rpm 直接把整个页签干崩。
                    const activeLevel = preset.levels.find((l) => l.level === rememberedLevel) ?? preset.levels[0];
                    const gearAllowed = isManualGearAllowed(preset.gear) && preset.levels.length > 0;

                    return (
                      <button
                        key={preset.gear}
                        type="button"
                        disabled={!gearAllowed}
                        onClick={() => handleGearCardSelect(preset.gear)}
                        title={!gearAllowed ? t('fanCurve.manualGear.runtimeUnavailable', { gear: getManualGearLabel(preset.gear), limit: manualGearLimitHint }) : undefined}
                        className={clsx(
                          'rounded-2xl border px-3.5 py-2.5 text-left transition-colors',
                          gearAllowed ? 'cursor-pointer' : 'cursor-not-allowed opacity-45',
                          isActiveGear ? `${preset.borderClass} ${preset.bgClass}` : 'border-border/70 bg-background/60 hover:bg-muted/45',
                        )}
                      >
                        <div className={clsx('text-lg font-bold', isActiveGear ? preset.colorClass : 'text-foreground')}>
                          {getManualGearLabel(preset.gear)}
                        </div>
                        <div className={clsx('mt-0.5 text-base font-semibold tabular-nums', isActiveGear ? preset.colorClass : 'text-muted-foreground')}>
                          {activeLevel ? `${activeLevel.rpm}${speedUnitSuffix}` : '--'}
                        </div>
                      </button>
                    );
                  })}
                </div>

                <div data-theme-ui="setting-subcard" className="rounded-2xl border border-border/70 bg-background/40 p-2.5">
                  <div className="relative mb-2 px-2">
                    <div className="absolute left-2 right-2 top-1/2 h-1 -translate-y-1/2 rounded-full bg-muted" />
                    <div className="relative flex items-center justify-between">
                      {manualPoints.map((point, index) => {
                        const isActivePoint = selectedManualPointIndex === index;
                        const isPassed = index < selectedManualPointIndex;
                        const gearAllowed = isManualGearAllowed(point.gear);

                        return (
                          <button
                            key={point.key}
                            type="button"
                            disabled={!gearAllowed}
                            onClick={() => handleManualPointSelect(index)}
                            className={clsx('flex h-6 w-6 shrink-0 items-center justify-center', gearAllowed ? 'cursor-pointer' : 'cursor-not-allowed opacity-45')}
                            title={gearAllowed
                              ? t('fanCurve.manualGear.pointTooltip', { gear: getManualGearLabel(point.gear), level: getManualLevelLabel(point.level), speed: `${formatSpeedValue(point.rpm)}${speedUnitSuffix}` })
                              : t('fanCurve.manualGear.runtimeUnavailable', { gear: getManualGearLabel(point.gear), limit: manualGearLimitHint })}
                          >
                            <span
                              data-theme-ui="manual-gear-dot"
                              className={clsx(
                                'block h-4 w-4 rounded-full border shadow-[inset_0_1px_0_rgba(255,255,255,0.32)] transition-transform duration-150',
                                isActivePoint
                                  ? 'scale-125 border-primary bg-primary'
                                  : isPassed
                                    ? 'border-primary/45 bg-primary/35'
                                    : 'border-border bg-muted-foreground/25',
                              )}
                            />
                          </button>
                        );
                      })}
                    </div>
                  </div>

                  <div className="flex items-start justify-between px-2 text-[11px]">
                    {manualPoints.map((point, index) => (
                      <span
                        key={`${point.key}-label`}
                        className={clsx(
                          'w-6 truncate text-center',
                          selectedManualPointIndex === index ? 'font-semibold text-primary' : 'text-muted-foreground',
                        )}
                      >
                        {t('fanCurve.manualGear.pointIndex', { index: point.levelIndex + 1 })}
                      </span>
                    ))}
                  </div>
                </div>
              </div>
            </m.div>
          )}
        </AnimatePresence>

        {supportsManualGears && <Dialog open={gearEditOpen} onOpenChange={setGearEditOpen}>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <DialogTitle>{t('fanCurve.manualGear.editTitle')}</DialogTitle>
              <DialogDescription>{t('fanCurve.manualGear.editHint', { max: `${formatSpeedValue(manualGearValueRange.max)}${speedUnitSuffix}` })}</DialogDescription>
            </DialogHeader>
            <div className="space-y-3">
              {manualGearDefaultPresets.map((preset) => (
                <div key={preset.gear} className="rounded-xl border border-border/70 bg-background/40 p-3">
                  <div className="mb-2 text-sm font-semibold text-foreground">{getManualGearLabel(preset.gear)}</div>
                  <div className="grid grid-cols-3 gap-2">
                    {preset.levels.map((lv) => {
                      const key = `${preset.gear}:${lv.level}`;
                      const fallbackValue = draftGearRpm[preset.gear]?.[lv.level] ?? lv.rpm;
                      const textValue = draftGearText[preset.gear]?.[lv.level] ?? String(fallbackValue);
                      return (
                        <div key={lv.level} className="space-y-1">
                          <div className="text-[11px] text-muted-foreground">{getManualLevelLabel(lv.level)}</div>
                          <NumberInput
                            deferClamp
                            invalid={gearDraftValidation.invalidKeys.has(key)}
                            value={fallbackValue}
                            textValue={textValue}
                            onTextChange={(raw) => setDraftRpmText(preset.gear, lv.level, raw)}
                            onChange={() => {}}
                            min={manualGearValueRange.min}
                            max={manualGearValueRange.max}
                            step={1}
                            suffix={speedUnitSuffix}
                          />
                        </div>
                      );
                    })}
                  </div>
                </div>
              ))}
              {!gearDraftValidation.valid && (
                <div className="rounded-lg border border-destructive/35 bg-destructive/10 px-3 py-2 text-xs text-destructive">
                  {t('fanCurve.manualGear.validationRangeOrder', {
                    min: `${formatSpeedValue(manualGearValueRange.min)}${speedUnitSuffix}`,
                    max: `${formatSpeedValue(manualGearValueRange.max)}${speedUnitSuffix}`,
                  })}
                </div>
              )}
            </div>
            <DialogFooter>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => {
                  const defaults = buildDraftFrom(null);
                  setDraftGearRpm(defaults);
                  setDraftGearText(buildDraftTextFrom(defaults));
                }}
                icon={<RotateCw className="h-3.5 w-3.5" />}
              >
                {t('fanCurve.manualGear.restoreDefault')}
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setGearEditOpen(false)} icon={<X className="h-3.5 w-3.5" />}>
                {t('common.actions.cancel')}
              </Button>
              <Button variant="primary" size="sm" onClick={saveGearRpm} loading={gearRpmSaving} disabled={!gearDraftValidation.valid} icon={<Check className="h-3.5 w-3.5" />}>
                {t('common.actions.save')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>}

        <div ref={curveEditorRef} data-theme-card="curve-editor">
          <div
            ref={chartRef}
            className={clsx('relative rounded-3xl border bg-card p-4 shadow-sm', dragIndex !== null ? 'ring-2 ring-primary/40 border-primary/30' : 'border-border/70')}
          >
            <div data-theme-ui="curve-editor-toolbar" className="mb-3 flex flex-wrap items-center justify-between gap-2 border-b border-border/50 pb-2.5">
              <div className="flex flex-wrap items-center gap-1.5">
                <span className="text-xs font-medium text-muted-foreground">{t('fanCurve.editor.globalShift')}</span>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-7 rounded-lg px-2 text-xs tabular-nums"
                  disabled={!canShiftCurveDown}
                  onClick={() => handleGlobalShiftCurve(-globalShiftStep)}
                  aria-label={t('fanCurve.editor.shiftDownAria', { step: `${globalShiftStep}${speedUnitSuffix}` })}
                  icon={<Minus className="h-3 w-3" />}
                >
                  {globalShiftStep}{speedUnitSuffix}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-7 rounded-lg px-2 text-xs tabular-nums"
                  disabled={!canShiftCurveUp}
                  onClick={() => handleGlobalShiftCurve(globalShiftStep)}
                  aria-label={t('fanCurve.editor.shiftUpAria', { step: `${globalShiftStep}${speedUnitSuffix}` })}
                  icon={<Plus className="h-3 w-3" />}
                >
                  {globalShiftStep}{speedUnitSuffix}
                </Button>
                {monotonicPushedIndices.length > 0 && (
                  <span data-theme-ui="curve-monotonic-badge" className="inline-flex items-center gap-1 rounded-full bg-amber-500/15 px-2 py-0.5 text-[11px] font-medium text-amber-600 dark:text-amber-300">
                    {t('fanCurve.editor.monotonicSynced')}
                  </span>
                )}
              </div>
              <div className="flex flex-wrap items-center gap-1.5">
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type="button"
                      data-theme-ui="curve-snap-toggle"
                      variant={stepSnapEnabled ? 'secondary' : 'outline'}
                      size="sm"
                      aria-pressed={stepSnapEnabled}
                      className={clsx('h-7 rounded-lg px-2.5 text-xs', stepSnapEnabled && 'border-primary/40 bg-primary/10 text-primary')}
                      onClick={() => setStepSnapEnabled((prev) => !prev)}
                      icon={<Magnet className="h-3.5 w-3.5" />}
                    >
                      {t('fanCurve.editor.snapLabel')}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    {!stepSnapEnabled
                      ? t('fanCurve.editor.snapOffTooltip')
                      : (speedUnit === 'rpm' ? t('fanCurve.editor.snapTooltipRpm') : t('fanCurve.editor.snapTooltipPercent'))}
                  </TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type="button"
                      data-theme-ui="curve-undo-button"
                      variant="outline"
                      size="sm"
                      className="h-7 rounded-lg px-2.5 text-xs"
                      disabled={curveUndoStack.length === 0}
                      onClick={handleUndoCurve}
                      icon={<Undo2 className="h-3.5 w-3.5" />}
                    >
                      {t('fanCurve.editor.undo')}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t('fanCurve.editor.undoTooltip')}</TooltipContent>
                </Tooltip>
              </div>
            </div>
            <div
              className="h-80 md:h-96 relative"
              onMouseDown={handleChartMouseDown}
              onTouchStart={handleChartTouchStart}
            >
              <CurveEditorChart
                data={chartData}
                temperatureRange={temperatureRange}
                speedMax={speedRange.max}
                speedTicks={zeroBasedSpeedTicks}
                speedUnitSuffix={speedUnitSuffix}
                showCoupledCurve={showCoupledCurve}
                learningActive={learningActive}
                learningTargetTemp={targetTempDraft}
                learningComfortLow={learningComfortBand.low}
                customDot={CustomDot}
              />
              {blackSharkGearCurveMode && chartData.length >= 2 && (
                <CurveExtensions
                  chartRef={chartRef}
                  anchors={chartData}
                  temperatureRange={temperatureRange}
                  speedMax={speedRange.max}
                  floorSpeed={manualGearValueRange.min}
                />
              )}
              <LiveTemperatureIndicator config={config} temperatureProp={temperatureProp} chartRef={chartRef} temperatureRange={temperatureRange} curvePoints={chartData} speedMax={speedRange.max} />
            </div>
            <AnimatePresence>
              {selectedCurvePoint && selectedPointIndex !== null && (
                <m.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  transition={{ duration: 0.16 }}
                  className="overflow-hidden"
                >
                  <div
                    data-theme-ui="curve-point-tuner"
                    className="mt-2.5 flex flex-wrap items-center justify-between gap-2 rounded-xl border border-primary/25 bg-primary/5 px-3 py-2"
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-xs font-medium text-foreground">{t('fanCurve.editor.pointTunerTitle')}</span>
                      <div className="inline-flex items-center gap-1 rounded-lg border border-border/70 bg-background/60 p-0.5">
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          className="h-6 w-6 px-0"
                          disabled={selectedPointIndex <= 0}
                          onClick={() => setSelectedPointIndex((prev) => (prev === null ? null : Math.max(0, prev - 1)))}
                          aria-label={t('fanCurve.editor.prevPointAria')}
                          icon={<ChevronLeft className="h-3.5 w-3.5" />}
                        />
                        {canDragPointHorizontally && blackSharkCurveTempRange ? (
                          <div
                            className="w-24"
                            onKeyDown={(event) => {
                              if (event.key !== 'Enter') return;
                              const parsed = Number(pointTempDraft.trim());
                              if (Number.isFinite(parsed)) {
                                applySelectedPointTemperature(parsed);
                              } else {
                                setPointTempDraft(String(selectedCurvePoint.temperature));
                              }
                            }}
                          >
                            <NumberInput
                              deferClamp
                              value={selectedCurvePoint.temperature}
                              textValue={pointTempDraft}
                              onTextChange={setPointTempDraft}
                              onChange={() => {}}
                              onBlur={() => {
                                const parsed = Number(pointTempDraft.trim());
                                if (Number.isFinite(parsed)) {
                                  applySelectedPointTemperature(parsed);
                                } else {
                                  setPointTempDraft(String(selectedCurvePoint.temperature));
                                }
                              }}
                              min={blackSharkCurveTempRange.minTempC}
                              max={blackSharkCurveTempRange.maxTempC}
                              step={1}
                              suffix="°C"
                            />
                          </div>
                        ) : (
                          <span className="px-2 text-xs font-semibold tabular-nums text-foreground">
                            {selectedCurvePoint.temperature}°C
                          </span>
                        )}
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          className="h-6 w-6 px-0"
                          disabled={selectedPointIndex >= localCurve.length - 1}
                          onClick={() => setSelectedPointIndex((prev) => (prev === null ? null : Math.min(localCurve.length - 1, prev + 1)))}
                          aria-label={t('fanCurve.editor.nextPointAria')}
                          icon={<ChevronRight className="h-3.5 w-3.5" />}
                        />
                      </div>
                    </div>

                    <div className="flex flex-wrap items-center gap-1.5">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="h-8 rounded-lg px-2 text-xs tabular-nums"
                        disabled={selectedCurvePoint.rpm <= speedRange.min}
                        onClick={() => applySelectedPointSpeed(selectedCurvePoint.rpm - pointFineStep)}
                        aria-label={t('fanCurve.editor.decreaseSpeedAria', { step: `${pointFineStep}${speedUnitSuffix}` })}
                        icon={<Minus className="h-3 w-3" />}
                      >
                        {pointFineStep}
                      </Button>
                      <div
                        className="w-32"
                        onKeyDown={(event) => {
                          if (event.key !== 'Enter') return;
                          const parsed = Number(pointSpeedDraft.trim());
                          if (Number.isFinite(parsed)) {
                            applySelectedPointSpeed(Math.round(parsed));
                          } else {
                            setPointSpeedDraft(String(selectedCurvePoint.rpm));
                          }
                        }}
                      >
                        <NumberInput
                          deferClamp
                          value={selectedCurvePoint.rpm}
                          textValue={pointSpeedDraft}
                          onTextChange={setPointSpeedDraft}
                          onChange={() => {}}
                          onBlur={() => {
                            const parsed = Number(pointSpeedDraft.trim());
                            if (Number.isFinite(parsed)) {
                              applySelectedPointSpeed(Math.round(parsed));
                            } else {
                              setPointSpeedDraft(String(selectedCurvePoint.rpm));
                            }
                          }}
                          min={speedRange.min}
                          max={speedRange.max}
                          step={pointFineStep}
                          suffix={speedUnitSuffix}
                        />
                      </div>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="h-8 rounded-lg px-2 text-xs tabular-nums"
                        disabled={selectedCurvePoint.rpm >= speedRange.max}
                        onClick={() => applySelectedPointSpeed(selectedCurvePoint.rpm + pointFineStep)}
                        aria-label={t('fanCurve.editor.increaseSpeedAria', { step: `${pointFineStep}${speedUnitSuffix}` })}
                        icon={<Plus className="h-3 w-3" />}
                      >
                        {pointFineStep}
                      </Button>
                      <span className="hidden text-[11px] text-muted-foreground xl:inline">
                        {t('fanCurve.editor.pointTunerHint', { step: `${pointFineStep}${speedUnitSuffix}` })}
                      </span>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="h-7 w-7 px-0"
                        onClick={() => setSelectedPointIndex(null)}
                        aria-label={t('fanCurve.editor.closeTunerAria')}
                        icon={<X className="h-3.5 w-3.5" />}
                      />
                    </div>
                  </div>
                </m.div>
              )}
            </AnimatePresence>
            <div data-theme-ui="curve-hints" className="mt-2.5 flex flex-wrap items-center justify-between gap-2 border-t border-border/50 pt-2.5 text-[11px] text-muted-foreground">
              <div className="flex flex-wrap items-center gap-3">
                <span className="inline-flex items-center gap-1.5">
                  <span className="h-0.5 w-4 rounded-full" style={{ backgroundColor: 'var(--chart-primary)' }} />
                  {t('fanCurve.chart.series.base')}
                </span>
                {showCoupledCurve && (
                  <span className="inline-flex items-center gap-1.5">
                    <span className="h-0.5 w-4 border-t-2 border-dashed border-amber-500" />
                    {t('fanCurve.chart.series.learned')}
                  </span>
                )}
                {learningActive && (
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-amber-500/10 px-2 py-0.5 text-amber-700 dark:text-amber-300">
                    <span className="h-2 w-2 rounded-xs bg-amber-500/55" />
                    {t('fanCurve.chart.comfortBand', { low: learningComfortBand.low, high: learningComfortBand.high })}
                  </span>
                )}
              </div>
              <div className="flex items-center gap-1.5">
                <span>{t('fanCurve.hints.dragPoint')} ({speedUnitSuffix})</span>
                {supportsLowRpmGuidance && (
                  <button
                    type="button"
                    onClick={() => {
                      pendingLowRpmSaveRef.current = false;
                      setLowRpmDialogOpen(true);
                    }}
                    className="inline-flex h-5 w-5 cursor-pointer items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted/70 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/30"
                    aria-label={t('fanCurve.warning.lowRpmHintButton')}
                    title={t('fanCurve.warning.lowRpmHintButton')}
                  >
                    <Info className="h-3.5 w-3.5" />
                  </button>
                )}
              </div>
            </div>
          </div>
        </div>

        <section data-theme-card="curve-learning" className="rounded-2xl border border-border/70 bg-card p-5 shadow-sm">
          <div className="flex items-start justify-between gap-4">
            <div className="flex min-w-0 items-center gap-3">
              <div data-theme-ui="setting-row-icon" data-icon-tone="blue" className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[11px] bg-muted/70 text-muted-foreground">
                <Sparkles className="h-4 w-4" />
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-[15px] font-semibold text-foreground">{t('fanCurve.learning.title')}</span>
                  {!smartControl.learning && (
                    <span className="rounded-full border border-border/70 bg-muted/50 px-2 py-0.5 text-[10px] text-muted-foreground">
                      {t('fanCurve.learning.paused')}
                    </span>
                  )}
                </div>
                <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground">
                  {blackSharkGearCurveMode ? t('fanCurve.learning.hardwareCurveNote') : t('fanCurve.learning.description')}
                </div>
              </div>
            </div>
            <ToggleSwitch
              enabled={!!smartControl.learning}
              onChange={handleLearningToggle}
              loading={learningConfigLoading}
              size="sm"
              color="blue"
              srLabel={t('fanCurve.learning.toggleAria')}
            />
          </div>

          {!isBlackSharkDevice && (
            <div data-theme-ui="curve-prediction-row" className="mt-3.5 flex items-center justify-between gap-4 rounded-2xl border border-border/65 bg-muted/35 px-4 py-3">
              <div className="flex min-w-0 items-center gap-2.5">
                <Radar className="h-4 w-4 shrink-0 text-sky-500 dark:text-sky-400" />
                <div className="min-w-0">
                  <div className="text-[13px] font-semibold text-foreground">{t('fanCurve.prediction.title')}</div>
                  <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.prediction.description')}</div>
                </div>
              </div>
              <ToggleSwitch
                enabled={!!smartControl.temperatureRisePrediction}
                onChange={handleTemperatureRisePredictionToggle}
                loading={learningConfigLoading}
                size="sm"
                color="blue"
                srLabel={t('fanCurve.prediction.toggleAria')}
              />
            </div>
          )}

          {!!smartControl.learning && !blackSharkGearCurveMode && (
            <div className="mt-3.5 divide-y divide-border/50 border-t border-border/50">
              <div className="py-3.5">
                <div data-theme-ui="setting-subcard" className="flex flex-col gap-3 rounded-2xl border border-border/65 bg-muted/30 px-4 py-3 lg:flex-row lg:items-center lg:justify-between">
                  <div className="min-w-0">
                    <div className="text-[13px] font-semibold text-foreground">{t('fanCurve.learning.biasTitle')}</div>
                    <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground">{currentLearningBiasOption.description}</div>
                  </div>
                  <div
                    role="radiogroup"
                    aria-label={t('fanCurve.learning.biasTitle')}
                    data-theme-ui="learning-bias-tabs"
                    className="grid shrink-0 grid-cols-3 gap-1 rounded-xl border border-border/65 bg-muted/65 p-1"
                  >
                    {learningBiasOptions.map((option) => {
                      const active = currentLearningBias === option.value;
                      return (
                        <button
                          key={option.value}
                          type="button"
                          role="radio"
                          aria-checked={active}
                          disabled={learningConfigLoading}
                          onClick={() => handleLearningBiasChange(option.value)}
                          className={clsx(
                            'h-8 cursor-pointer whitespace-nowrap rounded-lg border px-3 text-xs transition-colors disabled:cursor-not-allowed disabled:opacity-60',
                            active
                              ? 'border-border/60 bg-card font-semibold text-foreground shadow-xs'
                              : 'border-transparent text-muted-foreground hover:text-foreground',
                          )}
                        >
                          {option.label}
                        </button>
                      );
                    })}
                  </div>
                </div>
              </div>

              <div data-theme-ui="learning-target-temp" className="space-y-3 rounded-xl px-1 py-3.5">
                <div className="flex flex-col gap-2.5 sm:flex-row sm:items-center sm:justify-between">
                  <div className="min-w-0">
                    <div className="text-[13px] font-semibold text-foreground">{t('fanCurve.learning.targetTitle')}</div>
                    <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.learning.targetDescription')}</div>
                  </div>
                  <div className="w-full shrink-0 sm:w-28">
                    <NumberInput
                      className="learning-target-temp-input"
                      value={targetTempDraft}
                      onChange={handleTargetTempInputChange}
                      onBlur={handleTargetTempInputBlur}
                      min={SMART_CONTROL_TARGET_TEMP_MIN}
                      max={SMART_CONTROL_TARGET_TEMP_MAX}
                      step={1}
                      suffix="°C"
                      disabled={learningConfigLoading}
                    />
                  </div>
                </div>
                <Slider
                  className="learning-target-temp-slider"
                  min={SMART_CONTROL_TARGET_TEMP_MIN}
                  max={SMART_CONTROL_TARGET_TEMP_MAX}
                  step={1}
                  value={targetTempDraft}
                  onChange={handleTargetTempSliderChange}
                  onChangeEnd={handleTargetTempSliderCommit}
                  showValue={false}
                  disabled={learningConfigLoading}
                />
              </div>

              <div className="space-y-3 pt-3.5">
                <div className="flex flex-wrap items-center justify-between gap-2.5">
                  <div className="min-w-0">
                    <div className="text-[13px] font-semibold text-foreground">{t('fanCurve.learning.offsetTitle')}</div>
                    <div className="mt-0.5 text-xs text-muted-foreground">
                      {hasLearnedOffsets && learnedOffsetStats.maxItem
                        ? t('fanCurve.learning.adjustedSummary', {
                            count: learnedOffsetStats.count,
                            max: `${learnedOffsetStats.maxItem.delta > 0 ? '+' : ''}${formatSpeedValue(learnedOffsetStats.maxItem.delta)}${speedUnitSuffix}`,
                          })
                        : t('fanCurve.learning.offsetDescription')}
                    </div>
                  </div>
                  <div className="flex shrink-0 flex-wrap items-center gap-2">
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => setApplyLearningDialogOpen(true)}
                      loading={learningApplyLoading}
                      disabled={!hasLearnedOffsets || learningResetLoading}
                      icon={<Check className="h-3.5 w-3.5" />}
                    >
                      {t('fanCurve.learning.applyToCurve')}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleResetLearnedOffsets}
                      loading={learningResetLoading}
                      disabled={!hasLearnedOffsets || learningApplyLoading}
                      icon={<RotateCw className="h-3.5 w-3.5" />}
                    >
                      {t('fanCurve.learning.reset')}
                    </Button>
                  </div>
                </div>

                {hasLearnedOffsets ? (
                  <div data-theme-ui="learning-offset-bars" className="grid grid-cols-2 gap-2.5 sm:grid-cols-3 md:grid-cols-5">
                    {learnedOffsetPoints
                      .filter((item) => item.delta !== 0)
                      .map((item) => {
                        const widthPercent = Math.max(12, Math.min(100, Math.round((Math.abs(item.delta) / learnedOffsetStats.scaleMax) * 100)));
                        const deltaText = `${item.delta > 0 ? '+' : ''}${formatSpeedValue(item.delta)}${speedUnitSuffix}`;
                        const tooltip = t('fanCurve.learning.barTooltip', {
                          temp: item.temperature,
                          base: `${formatSpeedValue(item.base)}${speedUnitSuffix}`,
                          learned: `${formatSpeedValue(item.learned)}${speedUnitSuffix}`,
                          delta: deltaText,
                        });
                        return (
                          <div
                            key={`${item.index}-${item.temperature}`}
                            title={tooltip}
                            className="rounded-xl border border-border/60 bg-background/45 px-3 py-2 tabular-nums"
                          >
                            <div className="flex items-baseline justify-between gap-2">
                              <span className="text-[11px] font-medium text-muted-foreground">{item.temperature}°C</span>
                              <span className={clsx('text-xs font-semibold', item.delta > 0 ? 'text-primary' : 'text-muted-foreground')}>
                                {deltaText}
                              </span>
                            </div>
                            <div className="mt-1.5 h-[3px] w-full overflow-hidden rounded-full bg-muted/70">
                              <div
                                className={clsx('h-full rounded-full', item.delta > 0 ? 'bg-primary/75' : 'bg-amber-500/65')}
                                style={{ width: `${widthPercent}%` }}
                              />
                            </div>
                          </div>
                        );
                      })}
                  </div>
                ) : (
                  <div className="rounded-lg border border-dashed border-border/70 bg-background/30 px-3 py-2 text-xs text-muted-foreground">
                    {t('fanCurve.learning.noOffsets')}
                  </div>
                )}
              </div>
            </div>
          )}
        </section>

        <section data-theme-card="device-noise" className="rounded-2xl border border-border/70 bg-card p-4 shadow-sm">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="flex min-w-0 items-center gap-3">
              <div data-theme-ui="setting-row-icon" className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-muted/70 text-muted-foreground shadow-inner shadow-white/20">
                <AudioLines className="h-4 w-4 text-primary" />
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="text-sm font-medium text-foreground">{t('axisNoise.toolsTitle')}</div>
                  <Badge variant="success">{deviceModel || t('noiseDiagnostic.connected')}</Badge>
                </div>
                <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground">{t('axisNoise.toolsDescription')}</div>
              </div>
            </div>
            <div className="flex shrink-0 flex-wrap gap-2">
              <Button variant="secondary" size="sm" onClick={() => setNoiseDiagnosticOpen(true)} disabled={!isConnected} icon={<AudioLines className="h-3.5 w-3.5" />}>
                {t('noiseDiagnostic.open')}
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setAxisNoiseScanOpen(true)} disabled={!isConnected} icon={<Ear className="h-3.5 w-3.5" />}>
                {t('axisNoise.open')}
              </Button>
            </div>
          </div>
        </section>
        </>
      )}

        <section ref={historyDetailsRef} data-theme-card="curve-history" className="rounded-2xl border border-border/70 bg-card p-4 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2">
              <History className="h-4 w-4 text-primary" />
              <div>
                <div className="text-sm font-medium text-foreground">
                  {monitorOnlyActive ? t('deviceStatus.monitorOnly.historyTitle') : t('fanCurve.history.detailsTitle')}
                </div>
                <div className="text-xs text-muted-foreground">
                  {monitorOnlyActive ? t('deviceStatus.monitorOnly.historyDescription') : t('fanCurve.history.detailsDescription')}
                </div>
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2.5">
              <div className="flex items-center gap-2">
                <span className="text-xs text-muted-foreground">{t('fanCurve.history.retention.label')}</span>
                <Select
                  value={historyRetentionHours}
                  onChange={(value) => { void setHistoryRetentionHours(Number(value)); }}
                  disabled={temperatureHistorySaving || !temperatureHistoryEnabled}
                  size="sm"
                  className="min-w-28"
                  triggerClassName="h-8 rounded-lg px-3 text-xs"
                  options={HISTORY_RETENTION_HOUR_OPTIONS.map((hours) => ({
                    value: hours,
                    label: t('fanCurve.history.retention.hours', { count: hours }),
                  }))}
                />
              </div>
              <ToggleSwitch
                enabled={temperatureHistoryEnabled}
                onChange={setTemperatureHistoryEnabled}
                loading={temperatureHistorySaving}
                label={temperatureHistorySaving ? t('fanCurve.history.saving') : t('fanCurve.history.backgroundRecording')}
                size="sm"
                color="blue"
              />
              {temperatureHistory.length > 0 && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  icon={<Settings2 className="h-3.5 w-3.5" />}
                  className="h-8 rounded-lg px-2.5 text-xs"
                  onClick={() => setHistoryDisplayDialogOpen(true)}
                >
                  {t('fanCurve.history.displaySettings')}
                </Button>
              )}
            </div>
          </div>

          {temperatureHistory.length === 0 ? (
            <div className="rounded-xl border border-dashed border-border/70 bg-background/35 px-4 py-8 text-center text-sm text-muted-foreground">
              {temperatureHistoryEnabled ? t('fanCurve.history.emptyEnabled') : t('fanCurve.history.emptyDisabled')}
            </div>
          ) : (
            <>
              <div className={clsx('grid grid-cols-2 gap-3', monitorOnlyActive ? 'md:grid-cols-4' : 'md:grid-cols-5')}>
                {[
                  [t('fanCurve.history.summary.cpuPeak'), historySummary.cpuPeak ? `${historySummary.cpuPeak}°C` : '--', historySummary.cpuAverage ? t('fanCurve.history.summary.averageTemperature', { value: historySummary.cpuAverage }) : t('fanCurve.history.summary.noCpuTemperature')],
                  [t('fanCurve.history.summary.gpuPeak'), historySummary.gpuPeak ? `${historySummary.gpuPeak}°C` : '--', historySummary.gpuAverage ? t('fanCurve.history.summary.averageTemperature', { value: historySummary.gpuAverage }) : t('fanCurve.history.summary.noGpuTemperature')],
                  ...(!monitorOnlyActive
                    ? [[t('fanCurve.history.summary.fanPeak'), historySummary.fanPeak ? `${formatSpeedValue(historySummary.fanPeak)}${speedUnitSuffix}` : '--', historySummary.fanAverage ? t('fanCurve.history.summary.averageFan', { value: formatSpeedValue(historySummary.fanAverage), unit: speedUnitSuffix }) : t('fanCurve.history.summary.noFanData')]]
                    : []),
                  [t('fanCurve.history.summary.cpuPowerPeak'), historySummary.cpuPowerPeak ? `${formatPowerValue(historySummary.cpuPowerPeak)} W` : '-- W', historySummary.cpuPowerAverage ? t('fanCurve.history.summary.averagePower', { value: formatPowerValue(historySummary.cpuPowerAverage) }) : t('fanCurve.history.summary.noPowerData')],
                  [t('fanCurve.history.summary.gpuPowerPeak'), historySummary.gpuPowerPeak ? `${formatPowerValue(historySummary.gpuPowerPeak)} W` : '-- W', historySummary.gpuPowerAverage ? t('fanCurve.history.summary.averagePower', { value: formatPowerValue(historySummary.gpuPowerAverage) }) : t('fanCurve.history.summary.noPowerData')],
                ].map(([label, value, hint]) => (
                  <div key={label} data-theme-card="curve-history-summary" className="min-w-0 rounded-xl border border-border/70 bg-background/35 p-3">
                    <div className="truncate text-[11px] text-muted-foreground">{label}</div>
                    <div className="mt-1 truncate font-mono text-sm font-semibold tabular-nums text-foreground">{value}</div>
                    <div className="mt-1 truncate text-[11px] text-muted-foreground">{hint}</div>
                  </div>
                ))}
              </div>

              <div data-theme-card="curve-history-chart" className="rounded-xl border border-border/70 bg-background/35 p-3 space-y-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
	                  <div className="flex min-w-0 flex-wrap items-center gap-2">
	                    <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.history.recentTrend')}</div>
                    {historyZoomSelectionBounds ? (
                      <span className="rounded-full border border-primary/45 bg-primary/12 px-2 py-0.5 text-[11px] font-medium text-primary">
                        {t('fanCurve.history.zoomSelecting', {
                          from: formatHistoryDateTime(historyZoomSelectionBounds[0], locale),
                          to: formatHistoryDateTime(historyZoomSelectionBounds[1], locale),
                        })}
                      </span>
                    ) : historyZoomDomain ? (
                      <button
                        type="button"
                        onClick={resetHistoryZoom}
                        className="cursor-pointer rounded-full border border-border/70 bg-card px-2 py-0.5 text-[11px] text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/25"
                      >
                        {t('fanCurve.history.resetZoom')}
                      </button>
                    ) : (
	                      <span className="text-[11px] text-muted-foreground/60">{t('fanCurve.history.zoomHint')}</span>
	                    )}
                  </div>
                  <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                    {historySeriesMeta.map((series) => (
                      <button
                        key={series.key}
                        type="button"
                        onClick={() => toggleSeriesVisible(series.key)}
                        className={clsx(
                          'inline-flex cursor-pointer items-center gap-1.5 rounded-full border px-2 py-0.5 text-[11px] transition-colors',
                          historySeriesVisibility[series.key]
                            ? 'border-border bg-card text-foreground'
                            : 'border-border/60 bg-transparent text-muted-foreground/65',
                        )}
                      >
                        <span className="h-2 w-2 rounded-full" style={{ backgroundColor: series.color }} />
                        {series.label}
                      </button>
                    ))}
                  </div>
                </div>

                {historyChartsReady ? (
                  historyChartData.length < 2 ? (
                    <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">{t('fanCurve.history.waitingMoreSamples')}</div>
                  ) : (
                    <>
                    <div data-history-chart="thermal-fan" className="h-72 cursor-crosshair select-none">
                      <HistoryThermalChart
                        data={historyDisplayData}
                        timeDomain={historyTimeDomain}
                        monitorOnlyActive={monitorOnlyActive}
                        speedRange={speedRange}
                        temperatureRange={temperatureRange}
                        temperatureTicks={historyTempTicks}
                        visibleThermalSeries={visibleThermalSeries}
                        timelineLayout={timelineEventLayout}
                        historyTimelineAxisId={historyTimelineAxisId}
                        historyStatistics={historyStatistics}
                        renderStatistics={renderHistoryStatistics}
                        zoomSelectionBounds={historyZoomSelectionBounds}
                        tooltipContent={renderThermalHistoryTooltip}
                        speedUnitSuffix={speedUnitSuffix}
                        onMouseDown={handleHistoryZoomMouseDown}
                        onMouseMove={handleHistoryZoomMouseMove}
                        onMouseUp={handleHistoryZoomMouseUp}
                        onDoubleClick={resetHistoryZoom}
                      />
                    </div>

                    {showHistoryPowerChart && (
                      <div data-history-chart="power" className="border-t border-border/60 pt-3">
                        <div className="mb-2 text-xs font-medium text-muted-foreground">{t('fanCurve.history.powerTrend')}</div>
                        <div className="h-48 cursor-crosshair select-none">
                          <HistoryPowerChart
                            data={historyDisplayData}
                            timeDomain={historyTimeDomain}
                            monitorOnlyActive={monitorOnlyActive}
                            powerMax={historyPowerMax}
                            visiblePowerSeries={visiblePowerSeries}
                            historyStatistics={historyStatistics}
                            renderStatistics={renderHistoryStatistics}
                            zoomSelectionBounds={historyZoomSelectionBounds}
                            tooltipContent={renderPowerHistoryTooltip}
                            onMouseDown={handleHistoryZoomMouseDown}
                            onMouseMove={handleHistoryZoomMouseMove}
                            onMouseUp={handleHistoryZoomMouseUp}
                            onDoubleClick={resetHistoryZoom}
                          />
                        </div>
                      </div>
                    )}
                    </>
                  )
                ) : null}
              </div>
              <Dialog open={historyDisplayDialogOpen} onOpenChange={setHistoryDisplayDialogOpen}>
                <DialogContent data-theme-ui="history-display-dialog" className="max-w-4xl will-change-auto">
                  <DialogHeader>
                    <DialogTitle>{t('fanCurve.history.displaySettings')}</DialogTitle>
                    <DialogDescription>{t('fanCurve.history.displaySettingsDescription')}</DialogDescription>
                  </DialogHeader>
                  <div className="grid gap-5 lg:grid-cols-2">
                    <div className="space-y-3">
                      <div>
                        <div className="text-sm font-semibold text-foreground">{t('fanCurve.history.detailDisplayTitle')}</div>
                        <div className="mt-1 text-xs text-muted-foreground">{t('fanCurve.history.detailDisplayDescription')}</div>
                      </div>
                      <div className="flex items-center justify-between gap-3 rounded-xl border border-border/70 bg-background/45 px-3 py-2">
                        <div className="min-w-0">
                          <div className="text-sm font-medium text-foreground">{t('fanCurve.history.statistics.enabled')}</div>
                          <div className="text-[11px] text-muted-foreground">{t('fanCurve.history.statistics.description')}</div>
                        </div>
                        <ToggleSwitch
                          enabled={historyShowStatistics}
                          onChange={() => setHistoryShowStatistics(!historyShowStatistics)}
                          size="sm"
                          color="blue"
                          srLabel={t('fanCurve.history.statistics.enabled')}
                        />
                      </div>
                      <div
                        data-theme-ui="history-split-tooltip-toggle"
                        className="flex items-center justify-between gap-3 rounded-xl border border-border/70 bg-background/45 px-3 py-2"
                      >
                        <div className="min-w-0">
                          <div className="text-sm font-medium text-foreground">{t('fanCurve.history.splitTooltip.title')}</div>
                          <div className="text-[11px] text-muted-foreground">{t('fanCurve.history.splitTooltip.description')}</div>
                        </div>
                        <ToggleSwitch
                          enabled={historySplitTooltipByChart}
                          onChange={() => setHistorySplitTooltipByChart(!historySplitTooltipByChart)}
                          size="sm"
                          color="blue"
                          srLabel={t('fanCurve.history.splitTooltip.title')}
                        />
                      </div>
                      <div className="space-y-2">
                        {historySeriesMeta.map((series, index) => (
                          <div
                            key={series.key}
                            ref={(element) => {
                              if (element) {
                                historySeriesItemRefs.current[series.key] = element;
                              } else {
                                delete historySeriesItemRefs.current[series.key];
                              }
                            }}
                            data-theme-ui="history-display-row"
                            className={clsx(
                              'flex select-none items-center gap-2 rounded-xl border border-border/70 bg-background/45 px-3 py-2 transition-[background-color,border-color] duration-150',
                              draggedHistorySeries === series.key && 'border-primary/45 bg-primary/10',
                            )}
                          >
                            <button
                              type="button"
                              className={clsx(
                                'inline-flex h-7 w-6 shrink-0 touch-none items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/25',
                                draggedHistorySeries === series.key && 'cursor-grabbing bg-primary/10 text-primary',
                              )}
                              onPointerDown={(event) => handleHistorySeriesPointerDown(event, series.key)}
                              aria-label={series.label}
                            >
                              <GripVertical className="h-4 w-4" />
                            </button>
                            <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: series.color }} />
                            <div className="min-w-0 flex-1 text-sm font-medium text-foreground">{series.label}</div>
                            <div className="flex shrink-0 items-center gap-1">
                              <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                icon={<ArrowUp className="h-3.5 w-3.5" />}
                                className="h-7 w-7 px-0"
                                disabled={index === 0}
                                aria-label={t('fanCurve.history.moveUp')}
                                onClick={() => moveSeries(series.key, -1)}
                              />
                              <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                icon={<ArrowDown className="h-3.5 w-3.5" />}
                                className="h-7 w-7 px-0"
                                disabled={index === historySeriesMeta.length - 1}
                                aria-label={t('fanCurve.history.moveDown')}
                                onClick={() => moveSeries(series.key, 1)}
                              />
                            </div>
                            <ToggleSwitch
                              enabled={historySeriesVisibility[series.key]}
                              onChange={() => toggleSeriesVisible(series.key)}
                              size="sm"
                              color="blue"
                              srLabel={series.label}
                            />
                          </div>
                        ))}
                      </div>
                    </div>
                    <div className="space-y-3 border-t border-border/70 pt-5 lg:border-l lg:border-t-0 lg:pl-5 lg:pt-0">
                      <div>
                        <div className="text-sm font-semibold text-foreground">{t('fanCurve.history.homeDisplayTitle')}</div>
                        <div className="mt-1 text-xs text-muted-foreground">{t('fanCurve.history.homeDisplayDescription')}</div>
                      </div>
                      <div className="space-y-2">
                        {HISTORY_SERIES_ORDER.map((seriesKey) => historySeriesMeta.find((series) => series.key === seriesKey)).filter(Boolean).map((series) => series && (
                          <div
                            key={series.key}
                            data-theme-ui="home-history-display-row"
                            className="flex items-center gap-3 rounded-xl border border-border/70 bg-background/45 px-3 py-2"
                          >
                            <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: series.color }} />
                            <div className="min-w-0 flex-1 text-sm font-medium text-foreground">{series.label}</div>
                            <ToggleSwitch
                              enabled={homeSeriesVisibility[series.key]}
                              onChange={() => toggleHomeSeriesVisible(series.key)}
                              size="sm"
                              color="blue"
                              srLabel={series.label}
                            />
                          </div>
                        ))}
                      </div>
                      <div data-history-timeline-settings className="space-y-2 border-t border-border/70 pt-3">
                        <div className="flex items-center justify-between gap-3">
                          <div className="min-w-0">
                            <div className="text-sm font-medium text-foreground">{t('fanCurve.history.timelineEvents.title')}</div>
                            <div className="text-[11px] text-muted-foreground">{t('fanCurve.history.timelineEvents.description')}</div>
                          </div>
                          <ToggleSwitch
                            enabled={historyShowTimelineEvents}
                            onChange={() => setHistoryShowTimelineEvents(!historyShowTimelineEvents)}
                            size="sm"
                            color="blue"
                            srLabel={t('fanCurve.history.timelineEvents.title')}
                          />
                        </div>
                        <div className="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
                          {HISTORY_TIMELINE_EVENT_ORDER.map((eventType) => (
                            <div key={eventType} className="flex items-center gap-2 rounded-lg bg-background/35 px-2.5 py-1.5">
                              <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: TIMELINE_EVENT_COLORS[eventType] }} />
                              <span className="min-w-0 flex-1 truncate text-xs text-foreground">{t(`fanCurve.history.events.${eventType}`)}</span>
                              <ToggleSwitch
                                enabled={historyTimelineEventVisibility[eventType]}
                                onChange={() => toggleTimelineEventVisible(eventType)}
                                size="sm"
                                color="blue"
                                srLabel={t(`fanCurve.history.events.${eventType}`)}
                              />
                            </div>
                          ))}
                        </div>
                      </div>
                    </div>
                  </div>
                  <DialogFooter>
                    <Button type="button" variant="outline" onClick={resetHistoryDisplayPreferences}>
                      {t('fanCurve.history.resetDisplaySettings')}
                    </Button>
                    <Button type="button" onClick={() => setHistoryDisplayDialogOpen(false)}>
                      {t('common.actions.close')}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </>
          )}
        </section>

      {isConnected && (
        <>
        <Dialog open={createProfileDialogOpen} onOpenChange={(open) => {
          setCreateProfileDialogOpen(open);
          if (!open) setIsNewProfileComposing(false);
        }}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>{t('fanCurve.profiles.createTitle')}</DialogTitle>
              <DialogDescription>{t('fanCurve.profiles.createDescription')}</DialogDescription>
            </DialogHeader>
            <Input
              value={newProfileNameInput}
              onChange={(event) => handleNewProfileInputChange(event.target.value, Boolean((event.nativeEvent as InputEvent).isComposing))}
              onCompositionStart={() => setIsNewProfileComposing(true)}
              onCompositionEnd={(event) => {
                setIsNewProfileComposing(false);
                setNewProfileNameInput(trimProfileNameToLimit(event.currentTarget.value));
              }}
              onKeyDown={(event) => {
                if (event.key === 'Enter' && !isNewProfileComposing && !profileOpLoading) void createNewProfile();
              }}
              placeholder={t('fanCurve.profiles.newNamePlaceholder')}
              className="h-10"
              autoFocus
            />
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setCreateProfileDialogOpen(false)} disabled={profileOpLoading}>
                {t('common.actions.cancel')}
              </Button>
              <Button type="button" onClick={() => void createNewProfile()} loading={profileOpLoading} icon={<Plus className="h-3.5 w-3.5" />}>
                {t('fanCurve.profiles.createAction')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Dialog open={manageProfilesDialogOpen} onOpenChange={(open) => {
          setManageProfilesDialogOpen(open);
          if (!open) {
            setEditingProfileId('');
            setIsEditingProfileComposing(false);
          }
        }}>
          <DialogContent className="max-w-[640px]">
            <DialogHeader>
              <DialogTitle>{t('fanCurve.profiles.manageTitle')}</DialogTitle>
              <DialogDescription>{t('fanCurve.profiles.manageDescription')}</DialogDescription>
            </DialogHeader>

            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-1 rounded-xl border border-border/70 bg-muted/40 p-1">
                <button
                  type="button"
                  onClick={() => setManageDialogTab('list')}
                  className={clsx(
                    'h-8 cursor-pointer rounded-lg px-3 text-xs font-medium transition-colors',
                    manageDialogTab === 'list'
                      ? 'bg-card text-foreground shadow-xs'
                      : 'text-muted-foreground hover:text-foreground',
                  )}
                >
                  {t('fanCurve.profiles.tabList')}
                </button>
                <button
                  type="button"
                  onClick={() => setManageDialogTab('transfer')}
                  className={clsx(
                    'h-8 cursor-pointer rounded-lg px-3 text-xs font-medium transition-colors',
                    manageDialogTab === 'transfer'
                      ? 'bg-card text-foreground shadow-xs'
                      : 'text-muted-foreground hover:text-foreground',
                  )}
                >
                  {t('fanCurve.profiles.tabTransfer')}
                </button>
              </div>

              {manageDialogTab === 'list' ? (
                <section className="space-y-2.5">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-xs text-muted-foreground">{t('fanCurve.profiles.countBadge', { count: curveProfiles.length })}</span>
                    <Button
                      type="button"
                      variant="secondary"
                      size="sm"
                      onClick={() => setCreateProfileDialogOpen(true)}
                      disabled={profileOpLoading}
                      icon={<Plus className="h-3.5 w-3.5" />}
                    >
                      {t('fanCurve.profiles.add')}
                    </Button>
                  </div>
                  <div data-profile-manage-list className="max-h-[340px] space-y-2 overflow-y-auto pr-0.5">
                    {curveProfiles.map((profile) => {
                      const isActive = profile.id === activeProfileId;
                      const isEditing = editingProfileId === profile.id;
                      return (
                        <div
                          key={profile.id}
                          className={clsx(
                            'rounded-xl border px-3 py-2.5 transition-colors',
                            isActive ? 'border-primary/40 bg-primary/5' : 'border-border/70 bg-background/35',
                          )}
                        >
                          {isEditing ? (
                            <div data-profile-rename-row className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]">
                              <Input
                                value={editingProfileName}
                                onChange={(e) => handleEditingProfileInputChange(e.target.value, Boolean((e.nativeEvent as InputEvent).isComposing))}
                                onCompositionStart={() => setIsEditingProfileComposing(true)}
                                onCompositionEnd={(e) => {
                                  setIsEditingProfileComposing(false);
                                  setEditingProfileName(trimProfileNameToLimit(e.currentTarget.value));
                                }}
                                onKeyDown={(e) => {
                                  if (e.key === 'Enter' && !isEditingProfileComposing && !profileOpLoading) {
                                    void saveEditingProfileName(profile);
                                  } else if (e.key === 'Escape') {
                                    setEditingProfileId('');
                                  }
                                }}
                                placeholder={t('fanCurve.profiles.namePlaceholder')}
                                className="h-9 w-full"
                                autoFocus
                              />
                              <div className="flex shrink-0 items-center justify-end gap-1.5">
                                <Button
                                  type="button"
                                  variant="primary"
                                  size="sm"
                                  className="h-9"
                                  onClick={() => void saveEditingProfileName(profile)}
                                  loading={profileOpLoading}
                                  icon={<Check className="h-3.5 w-3.5" />}
                                >
                                  {t('common.actions.save')}
                                </Button>
                                <Button
                                  type="button"
                                  variant="outline"
                                  size="sm"
                                  className="h-9"
                                  onClick={() => setEditingProfileId('')}
                                  disabled={profileOpLoading}
                                  icon={<X className="h-3.5 w-3.5" />}
                                />
                              </div>
                            </div>
                          ) : (
                            <div className="flex flex-wrap items-center justify-between gap-2">
                              <div className="min-w-0 flex-1">
                                <div className="flex items-center gap-2">
                                  <span className="truncate text-sm font-medium text-foreground">{profile.name}</span>
                                  {isActive && <Badge variant="info">{t('fanCurve.profiles.activeBadge')}</Badge>}
                                </div>
                                <div className="mt-0.5 truncate text-[11px] tabular-nums text-muted-foreground">
                                  {formatProfileSummary(profile.curve)}
                                </div>
                              </div>
                              <div className="flex shrink-0 items-center gap-1">
                                {!isActive && (
                                  <Button
                                    type="button"
                                    variant="secondary"
                                    size="sm"
                                    className="h-8 px-2.5 text-xs"
                                    disabled={profileOpLoading}
                                    onClick={() => void switchProfile(profile.id)}
                                  >
                                    {t('fanCurve.profiles.activate')}
                                  </Button>
                                )}
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="sm"
                                  className="h-8 w-8 px-0"
                                  disabled={profileOpLoading}
                                  onClick={() => startEditingProfile(profile)}
                                  aria-label={t('fanCurve.profiles.rename')}
                                  title={t('fanCurve.profiles.rename')}
                                  icon={<Pencil className="h-3.5 w-3.5" />}
                                />
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="sm"
                                  className="h-8 w-8 px-0"
                                  disabled={profileOpLoading}
                                  onClick={() => void duplicateProfile(profile)}
                                  aria-label={t('fanCurve.profiles.duplicate')}
                                  title={t('fanCurve.profiles.duplicate')}
                                  icon={<Copy className="h-3.5 w-3.5" />}
                                />
                                {curveProfiles.length > 1 && (
                                  <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    className="h-8 w-8 px-0 text-muted-foreground hover:text-destructive"
                                    disabled={profileOpLoading}
                                    onClick={() => {
                                      setPendingDeleteProfileId(profile.id);
                                      setDeleteProfileDialogOpen(true);
                                    }}
                                    aria-label={t('fanCurve.profiles.deleteProfileLabel', { name: profile.name })}
                                    title={t('fanCurve.profiles.deleteProfileLabel', { name: profile.name })}
                                    icon={<Trash2 className="h-3.5 w-3.5" />}
                                  />
                                )}
                              </div>
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </section>
              ) : (
                <section data-profile-transfer className="space-y-3">
                  <div data-profile-transfer-header className="space-y-1">
                    <div className="text-sm font-medium">{t('fanCurve.importExport.title')}</div>
                    <div className="text-xs leading-relaxed text-muted-foreground">{t('fanCurve.importExport.description')}</div>
                  </div>
                  <div className="space-y-3">
                    <div data-profile-export-section className="space-y-3 rounded-xl border border-border/70 bg-background/30 p-3.5">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <div>
                          <span className="text-xs font-medium text-muted-foreground">
                            {t('fanCurve.importExport.exportTitle')} · {t('fanCurve.importExport.selectedCount', { count: selectedExportProfileIds.length })}
                          </span>
                          <p className="mt-0.5 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.importExport.exportHint')}</p>
                        </div>
                        <div className="flex shrink-0 items-center gap-1.5">
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            className="h-7 px-2 text-xs"
                            onClick={() => setSelectedExportProfileIds(
                              selectedExportProfileIds.length === curveProfiles.length
                                ? (activeProfileId ? [activeProfileId] : [])
                                : curveProfiles.map((profile) => profile.id),
                            )}
                          >
                            {selectedExportProfileIds.length === curveProfiles.length
                              ? t('fanCurve.profiles.selectCurrentOnly')
                              : t('fanCurve.profiles.selectAll')}
                          </Button>
                        </div>
                      </div>
                      <div id="curve-profile-export-picker" data-profile-export-picker className="grid max-h-36 grid-cols-2 gap-1.5 overflow-y-auto rounded-lg border border-border/70 bg-background/60 p-1.5 sm:grid-cols-3">
                        {curveProfiles.map((profile) => {
                          const checked = selectedExportProfileIds.includes(profile.id);
                          return (
                            <label
                              key={profile.id}
                              className={clsx(
                                'flex cursor-pointer items-center gap-2 rounded-lg px-2.5 py-1.5 text-sm transition-colors',
                                checked ? 'bg-primary/10 text-foreground' : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground',
                              )}
                            >
                              <input
                                type="checkbox"
                                checked={checked}
                                onChange={() => setSelectedExportProfileIds((ids) => checked ? ids.filter((id) => id !== profile.id) : [...ids, profile.id])}
                                className="h-3.5 w-3.5 shrink-0 accent-primary"
                              />
                              <span className="min-w-0 truncate">{profile.name}</span>
                            </label>
                          );
                        })}
                      </div>
                      <div data-profile-export-actions className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                        <Button className="w-full" variant="secondary" size="sm" onClick={() => void exportProfiles('clipboard')} disabled={selectedExportProfileIds.length === 0} icon={<Clipboard className="h-3.5 w-3.5" />}>
                          {t('fanCurve.importExport.copyCode')}
                        </Button>
                        <Button className="w-full" variant="outline" size="sm" onClick={() => void exportProfiles('file')} disabled={selectedExportProfileIds.length === 0} icon={<Upload className="h-3.5 w-3.5" />}>
                          {t('fanCurve.importExport.exportFile')}
                        </Button>
                      </div>
                    </div>

                    <div
                      data-profile-import-section
                      onDragEnter={(event) => { event.preventDefault(); setIsImportDragging(true); }}
                      onDragOver={(event) => { event.preventDefault(); setIsImportDragging(true); }}
                      onDragLeave={() => setIsImportDragging(false)}
                      onDrop={handleProfileFileDrop}
                      className={clsx(
                        'space-y-2.5 rounded-xl border border-border/70 bg-background/30 p-3.5 transition-colors',
                        isImportDragging && 'border-primary bg-primary/5',
                      )}
                    >
                      <input
                        ref={profileFileInputRef}
                        type="file"
                        accept=".fcurve,.fancontrolcurve,.b2curve,.txt,text/plain"
                        className="hidden"
                        onChange={(event) => {
                          void loadProfileImportFile(event.target.files?.[0]);
                          event.target.value = '';
                        }}
                      />
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <span className="text-xs font-medium text-muted-foreground">{t('fanCurve.importExport.importTitle')}</span>
                        <Button className="shrink-0" variant="outline" size="sm" onClick={() => profileFileInputRef.current?.click()} icon={<Download className="h-3.5 w-3.5" />}>
                          {t('fanCurve.importExport.chooseFile')}
                        </Button>
                      </div>
                      <textarea
                        value={importCode}
                        onChange={(e) => setImportCode(e.target.value)}
                        rows={2}
                        className="min-h-16 w-full resize-none rounded-lg border border-border/70 bg-background px-3 py-2 text-xs leading-relaxed"
                        placeholder={t('fanCurve.importExport.importPlaceholder')}
                      />
                      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                        <span className="text-[11px] text-muted-foreground">{t('fanCurve.importExport.dropHint')}</span>
                        <Button
                          className="w-full sm:w-auto"
                          variant="secondary"
                          size="sm"
                          onClick={() => {
                            if (!importCode.trim()) {
                              toast.error(t('fanCurve.toast.importMissingCode'));
                              return;
                            }
                            void importProfiles();
                          }}
                          loading={profileOpLoading}
                          icon={<Download className="h-3.5 w-3.5" />}
                        >
                          {t('fanCurve.importExport.importAction')}
                        </Button>
                      </div>
                    </div>
                  </div>
                </section>
              )}
            </div>
          </DialogContent>
        </Dialog>

        <Dialog
          open={profileSwitchDialogOpen}
          onOpenChange={(open) => {
            setProfileSwitchDialogOpen(open);
            if (!open) {
              setEditorSession(cancelCurveEditorSwitch);
              cancelPendingTabChange();
            }
          }}
        >
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>{t('fanCurve.profiles.unsavedSwitchTitle')}</DialogTitle>
              <DialogDescription>{t('fanCurve.profiles.unsavedSwitchDescription')}</DialogDescription>
            </DialogHeader>
            <DialogFooter className="sm:justify-between">
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setProfileSwitchDialogOpen(false);
                  setEditorSession(cancelCurveEditorSwitch);
                  cancelPendingTabChange();
                }}
                disabled={isSaving || profileOpLoading}
              >
                {t('common.actions.cancel')}
              </Button>
              <div className="flex flex-wrap justify-end gap-2">
                <Button type="button" variant="secondary" onClick={() => void confirmProfileSwitch('discard')} disabled={isSaving || profileOpLoading}>
                  {t('fanCurve.profiles.discardAndSwitch')}
                </Button>
                <Button type="button" onClick={() => void confirmProfileSwitch('save')} loading={isSaving || profileOpLoading} icon={<Check className="h-3.5 w-3.5" />}>
                  {t('fanCurve.profiles.saveAndSwitch')}
                </Button>
              </div>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Dialog
          open={deleteProfileDialogOpen}
          onOpenChange={(open) => {
            setDeleteProfileDialogOpen(open);
            if (!open) setPendingDeleteProfileId('');
          }}
        >
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>{t('fanCurve.profiles.deleteConfirmTitle')}</DialogTitle>
              <DialogDescription>
                {t(
                  hasUnsavedChanges && pendingDeleteProfileId === activeProfileId
                    ? 'fanCurve.profiles.deleteUnsavedDescription'
                    : 'fanCurve.profiles.deleteDescription',
                  { name: pendingDeleteProfile?.name || '' },
                )}
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDeleteProfileDialogOpen(false)} disabled={profileOpLoading}>
                {t('common.actions.cancel')}
              </Button>
              <Button type="button" variant="danger" onClick={() => void removeProfile()} loading={profileOpLoading} icon={<Trash2 className="h-3.5 w-3.5" />}>
                {t('common.actions.delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Dialog open={applyLearningDialogOpen} onOpenChange={setApplyLearningDialogOpen}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>{t('fanCurve.learning.applyConfirmTitle')}</DialogTitle>
              <DialogDescription>
                {t('fanCurve.learning.applyConfirmDescription', { count: learnedOffsetStats.count })}
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setApplyLearningDialogOpen(false)} disabled={learningApplyLoading}>
                {t('common.actions.cancel')}
              </Button>
              <Button type="button" variant="primary" onClick={() => void handleApplyLearnedOffsetsToCurve()} loading={learningApplyLoading} icon={<Check className="h-3.5 w-3.5" />}>
                {t('fanCurve.learning.applyConfirmAction')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Dialog
          open={lowRpmDialogOpen}
          onOpenChange={(open) => {
            setLowRpmDialogOpen(open);
            if (!open) pendingLowRpmSaveRef.current = false;
          }}
        >
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <TriangleAlert className="h-4 w-4 shrink-0 text-amber-500" />
                <span>{t('fanCurve.warning.lowRpmTitle')}</span>
              </DialogTitle>
              <DialogDescription className="leading-relaxed">
                {t('fanCurve.warning.lowRpmBody')}
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button type="button" variant={hasLowRpmPoints ? 'outline' : 'primary'} onClick={() => void confirmLowRpmKeep()} disabled={isSaving}>
                {t('fanCurve.warning.confirm')}
              </Button>
              {hasLowRpmPoints && (
                <Button type="button" variant="primary" onClick={() => void raiseCurveToRecommendedRpm()} loading={isSaving} icon={<Check className="h-3.5 w-3.5" />}>
                  {t('fanCurve.warning.lowRpmAdjustAction')}
                </Button>
              )}
            </DialogFooter>
          </DialogContent>
        </Dialog>
        </>
      )}

      <NoiseDiagnostic
        open={noiseDiagnosticOpen}
        onOpenChange={setNoiseDiagnosticOpen}
        config={config}
        onConfigChange={onConfigChange}
        isConnected={isConnected}
        fanData={fanData}
        runtimeDeviceProfile={runtimeDeviceProfile}
        runtimeDeviceCapabilities={runtimeDeviceCapabilities}
        deviceModel={deviceModel}
      />

      <AxisNoiseScan
        open={axisNoiseScanOpen}
        onOpenChange={setAxisNoiseScanOpen}
        config={config}
        onConfigChange={onConfigChange}
        isConnected={isConnected}
        fanData={fanData}
        runtimeDeviceProfile={runtimeDeviceProfile}
        runtimeDeviceCapabilities={runtimeDeviceCapabilities}
        deviceModel={deviceModel}
      />

      </div>
  );
});

export default FanCurve;

