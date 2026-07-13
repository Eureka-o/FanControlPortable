'use client';

import React, { useState, useEffect, useCallback, memo, useMemo, useRef } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip, ResponsiveContainer } from 'recharts';
import { AnimatePresence, motion } from 'framer-motion';
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
} from 'lucide-react';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Input } from '@/components/ui/input';
import { apiService } from '../services/api';
import { useTemperatureHistory } from '../hooks/useTemperatureHistory';
import { useHistoryDisplayPreferences } from '../hooks/useHistoryDisplayPreferences';
import { useLocale } from '../lib/i18n';
import { getFanSpeedUnit, getFanSpeedRange, getFanSpeedTicks, fanSpeedUnitLabel } from '../lib/fan-speed';
import { type HistorySeriesKey } from '../lib/temperature-history';
import type { CurveFocusTarget } from '../store/app-store';
import { types } from '../../../wailsjs/go/models';
import { ClipboardSetText } from '../../../wailsjs/runtime/runtime';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
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
  isManualGearAllowedForFlyDigi,
  getManualGearLabel,
  getManualLevelLabel,
  supportsManualGearsFromCapabilities,
  type ManualGearRpmMap,
} from '../lib/manualGearPresets';

const FAN_CURVE_MIN_TEMP = 20;
const FAN_CURVE_MAX_TEMP = 110;
const FAN_CURVE_TEMP_STEP = 5;
const DEFAULT_CURVE_LENGTH = ((FAN_CURVE_MAX_TEMP - FAN_CURVE_MIN_TEMP) / FAN_CURVE_TEMP_STEP) + 1;
const FAN_CURVE_TEMPERATURE_TICKS = Array.from({ length: DEFAULT_CURVE_LENGTH }, (_, i) => FAN_CURVE_MIN_TEMP + i * FAN_CURVE_TEMP_STEP);
const DEFAULT_FAN_CURVE: types.FanCurvePoint[] = [
  { temperature: 20, rpm: 0 }, { temperature: 25, rpm: 10 }, { temperature: 30, rpm: 18 }, { temperature: 35, rpm: 24 },
  { temperature: 40, rpm: 30 }, { temperature: 45, rpm: 36 }, { temperature: 50, rpm: 42 }, { temperature: 55, rpm: 48 },
  { temperature: 60, rpm: 55 }, { temperature: 65, rpm: 62 }, { temperature: 70, rpm: 70 }, { temperature: 75, rpm: 78 },
  { temperature: 80, rpm: 86 }, { temperature: 85, rpm: 92 }, { temperature: 90, rpm: 96 }, { temperature: 95, rpm: 100 },
  { temperature: 100, rpm: 100 }, { temperature: 105, rpm: 100 }, { temperature: 110, rpm: 100 },
];
const DEFAULT_RPM_FAN_CURVE: types.FanCurvePoint[] = [
  { temperature: 30, rpm: 1000 }, { temperature: 35, rpm: 1200 }, { temperature: 40, rpm: 1400 }, { temperature: 45, rpm: 1600 },
  { temperature: 50, rpm: 1800 }, { temperature: 55, rpm: 2000 }, { temperature: 60, rpm: 2300 }, { temperature: 65, rpm: 2600 },
  { temperature: 70, rpm: 2900 }, { temperature: 75, rpm: 3200 }, { temperature: 80, rpm: 3500 }, { temperature: 85, rpm: 3800 },
  { temperature: 90, rpm: 4000 }, { temperature: 95, rpm: 4000 }, { temperature: 100, rpm: 4000 }, { temperature: 105, rpm: 4000 },
  { temperature: 110, rpm: 4000 },
];
const SMART_CONTROL_TARGET_TEMP_MIN = 45;
const SMART_CONTROL_TARGET_TEMP_MAX = 90;
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

function formatSpeedValue(value: number) {
  if (!Number.isFinite(value)) {
    return '0';
  }
  const rounded = Math.round(value * 10) / 10;
  return Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1);
}

function formatPowerValue(value: number) {
  if (!Number.isFinite(value) || value <= 0) {
    return '--';
  }
  const rounded = value < 10 ? Math.round(value * 10) / 10 : Math.round(value);
  return Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1);
}

const CPU_TEMP_STROKE = 'var(--chart-cpu-temperature)';
const GPU_TEMP_STROKE = 'var(--chart-gpu-temperature)';
const FAN_SPEED_STROKE = 'var(--chart-fan-speed)';
const CPU_POWER_STROKE = 'var(--chart-cpu-power)';
const GPU_POWER_STROKE = 'var(--chart-gpu-power)';

const HISTORY_SERIES_DATA_KEY: Record<HistorySeriesKey, 'cpuTemp' | 'gpuTemp' | 'fanRpm' | 'cpuPowerWatts' | 'gpuPowerWatts'> = {
  cpu: 'cpuTemp',
  gpu: 'gpuTemp',
  fan: 'fanRpm',
  cpuPower: 'cpuPowerWatts',
  gpuPower: 'gpuPowerWatts',
};

const HISTORY_SERIES_AXIS: Record<HistorySeriesKey, 'temp' | 'fan' | 'power'> = {
  cpu: 'temp',
  gpu: 'temp',
  fan: 'fan',
  cpuPower: 'power',
  gpuPower: 'power',
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
  const currentPoint = curve[index];
  if (!currentPoint) {
    return { curve, changed: false };
  }

  const normalizedSpeed = normalizeSpeedValue(targetSpeed, minSpeed, maxSpeed);
  const nextCurve = [...curve];
  let changed = false;

  if (currentPoint.rpm !== normalizedSpeed) {
    nextCurve[index] = { ...currentPoint, rpm: normalizedSpeed };
    changed = true;
  }

  for (let left = index - 1; left >= 0; left -= 1) {
    if (nextCurve[left].rpm <= nextCurve[left + 1].rpm) {
      break;
    }

    nextCurve[left] = {
      ...nextCurve[left],
      rpm: nextCurve[left + 1].rpm,
    };
    changed = true;
  }

  for (let right = index + 1; right < nextCurve.length; right += 1) {
    if (nextCurve[right].rpm >= nextCurve[right - 1].rpm) {
      break;
    }

    nextCurve[right] = {
      ...nextCurve[right],
      rpm: nextCurve[right - 1].rpm,
    };
    changed = true;
  }

  return {
    curve: nextCurve,
    changed,
  };
}

interface FanCurveProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  runtimeDeviceProfile?: types.DeviceProfile | null;
  runtimeDeviceCapabilities?: types.DeviceCapabilities | null;
  deviceModel: string | null;
  focusTarget: CurveFocusTarget | null;
  onFocusHandled: () => void;
}

function formatHistoryTime(timestamp: number, locale: string) {
  return new Date(timestamp).toLocaleTimeString(locale, {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
  });
}

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

/* ── Temperature indicator overlay (memo, doesn't re-render chart) ── */

const TemperatureIndicator = memo(function TemperatureIndicator({
  temperature,
  chartRef,
  temperatureRange,
}: {
  temperature: number | null;
  chartRef: React.RefObject<HTMLDivElement | null>;
  temperatureRange: { min: number; max: number };
}) {
  const { t } = useTranslation();
  const [position, setPosition] = useState<{ x: number; top: number; height: number } | null>(null);

  useEffect(() => {
    if (temperature === null || !chartRef.current) { setPosition(null); return; }
    const updatePosition = () => {
      const chartArea = chartRef.current?.querySelector('.recharts-cartesian-grid');
      if (!chartArea) return;
      const rect = chartArea.getBoundingClientRect();
      const containerRect = chartRef.current!.querySelector('.recharts-responsive-container')?.getBoundingClientRect();
      if (!containerRect) return;
      const chartWidth = rect.width;
      const chartLeft = rect.left - containerRect.left;
      const tempPercent = (temperature - temperatureRange.min) / (temperatureRange.max - temperatureRange.min);
      const x = chartLeft + tempPercent * chartWidth;
      setPosition({ x, top: rect.top - containerRect.top, height: rect.height });
    };
    updatePosition();
    window.addEventListener('resize', updatePosition);
    return () => window.removeEventListener('resize', updatePosition);
  }, [temperature, chartRef, temperatureRange]);

  if (!position || temperature === null) return null;

  return (
    <svg className="absolute inset-0 pointer-events-none overflow-visible" style={{ width: '100%', height: '100%' }}>
      <line x1={position.x} y1={position.top} x2={position.x} y2={position.top + position.height} stroke="var(--chart-temperature-indicator)" strokeWidth={2} strokeDasharray="5 5" />
      <rect x={position.x - 45} y={position.top - 22} width={90} height={20} rx={4} fill="var(--chart-temperature-indicator)" />
      <text x={position.x} y={position.top - 8} textAnchor="middle" fill="white" fontSize={11} fontWeight={500}>{t('fanCurve.chart.currentTemperature', { temperature })}</text>
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
  cx, cy, index, speed, unitSuffix, onDragStart, isActive,
}: {
  cx: number; cy: number; index: number; temperature: number; speed: number; unitSuffix: string;
  onDragStart: (index: number) => void; isActive: boolean;
}) {
  const handleMouseDown = useCallback((e: React.MouseEvent) => { e.preventDefault(); e.stopPropagation(); onDragStart(index); }, [index, onDragStart]);
  const handleTouchStart = useCallback((e: React.TouchEvent) => { e.preventDefault(); e.stopPropagation(); onDragStart(index); }, [index, onDragStart]);

  return (
    <g>
      <circle cx={cx} cy={cy} r={isActive ? 14 : 10} fill="transparent" stroke="transparent" style={{ cursor: 'ns-resize' }} onMouseDown={handleMouseDown} onTouchStart={handleTouchStart} />
      <circle cx={cx} cy={cy} r={isActive ? 8 : 6} fill={isActive ? 'var(--chart-primary-active)' : 'var(--chart-primary)'} stroke="var(--card)" strokeWidth={2}
        style={{ cursor: 'ns-resize', transition: isActive ? 'none' : 'all 0.2s ease', filter: isActive ? 'drop-shadow(0 4px 8px var(--chart-primary-glow))' : 'drop-shadow(0 2px 4px var(--chart-point-shadow))' }}
        onMouseDown={handleMouseDown} onTouchStart={handleTouchStart}
      />
      {isActive && (
        <g>
          <rect x={cx - 35} y={cy - 35} width={70} height={24} rx={4} fill="var(--chart-primary-active)" opacity={0.95} />
          <text x={cx} y={cy - 19} textAnchor="middle" fill="white" fontSize={12} fontWeight={600}>{formatSpeedValue(speed)}{unitSuffix}</text>
        </g>
      )}
    </g>
  );
});

/* ═══════════════════════════════════════════════════════════
   ─── Main FanCurve Component ───
   ═══════════════════════════════════════════════════════════ */

const FanCurve = memo(function FanCurve({ config, onConfigChange, isConnected, fanData, temperature, runtimeDeviceProfile, runtimeDeviceCapabilities, focusTarget, onFocusHandled }: FanCurveProps) {
  const { t } = useTranslation();
  const { locale } = useLocale();
  const [localCurve, setLocalCurve] = useState<types.FanCurvePoint[]>([]);
  const [curveProfiles, setCurveProfiles] = useState<CurveProfile[]>([]);
  const [activeProfileId, setActiveProfileId] = useState('');
  const [profileNameInput, setProfileNameInput] = useState('');
  const [isProfileNameComposing, setIsProfileNameComposing] = useState(false);
  const [profileOpLoading, setProfileOpLoading] = useState(false);
  const [createProfileDialogOpen, setCreateProfileDialogOpen] = useState(false);
  const [manageProfilesDialogOpen, setManageProfilesDialogOpen] = useState(false);
  const [profileSwitchDialogOpen, setProfileSwitchDialogOpen] = useState(false);
  const [deleteProfileDialogOpen, setDeleteProfileDialogOpen] = useState(false);
  const [pendingProfileId, setPendingProfileId] = useState('');
  const [pendingDeleteProfileId, setPendingDeleteProfileId] = useState('');
  const [newProfileNameInput, setNewProfileNameInput] = useState('');
  const [importCode, setImportCode] = useState('');
  const [isImportDragging, setIsImportDragging] = useState(false);
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);
  const [isInitialized, setIsInitialized] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [learningConfigLoading, setLearningConfigLoading] = useState(false);
  const [learningResetLoading, setLearningResetLoading] = useState(false);
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [isInteracting, setIsInteracting] = useState(false);
  const [historyDisplayDialogOpen, setHistoryDisplayDialogOpen] = useState(false);
  const [draggedHistorySeries, setDraggedHistorySeries] = useState<HistorySeriesKey | null>(null);
  const {
    orderedSeries,
    seriesVisibility: historySeriesVisibility,
    toggleSeriesVisible,
    moveSeries,
    reorderSeries,
    resetPreferences: resetHistoryDisplayPreferences,
  } = useHistoryDisplayPreferences();
  const chartRef = useRef<HTMLDivElement>(null);
  const profileFileInputRef = useRef<HTMLInputElement>(null);
  const curveEditorRef = useRef<HTMLDivElement>(null);
  const historyDetailsRef = useRef<HTMLElement>(null);
  const initialFocusTarget = useRef(focusTarget).current;
  const chartBoundsRef = useRef<{ top: number; bottom: number; left: number; right: number; yMin: number; yMax: number } | null>(null);
  const dragFrameRef = useRef<number | null>(null);
  const pendingDragYRef = useRef<number | null>(null);
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
    if (!runtimeDeviceCapabilities && !fanData?.transport && !fanData?.speedUnit) {
      return null;
    }
    return {
      transport: fanData?.transport || runtimeDeviceCapabilities?.transport,
      speedUnit: fanData?.speedUnit || runtimeDeviceCapabilities?.speedUnit,
      speedRange: runtimeDeviceCapabilities?.speedRange,
      capabilities: runtimeDeviceCapabilities || undefined,
    };
  }, [fanData?.speedUnit, fanData?.transport, isConnected, runtimeDeviceCapabilities, runtimeDeviceProfile]);
  const speedUnit = useMemo(() => getFanSpeedUnit(fanData as any, config as any, runtimeProfileForSpeed as any), [config, fanData, runtimeProfileForSpeed]);
  const speedUnitSuffix = fanSpeedUnitLabel(speedUnit);
  const configuredSpeedRange = useMemo(() => getFanSpeedRange(config as any, speedUnit, runtimeProfileForSpeed as any), [config, runtimeProfileForSpeed, speedUnit]);
  const speedRange = useMemo(() => ({
    min: configuredSpeedRange.min,
    max: configuredSpeedRange.max,
    ticks: getFanSpeedTicks(configuredSpeedRange.min, configuredSpeedRange.max),
  }), [configuredSpeedRange.max, configuredSpeedRange.min]);
  const defaultCurve = speedUnit === 'rpm' ? DEFAULT_RPM_FAN_CURVE : DEFAULT_FAN_CURVE;
  const {
    points: temperatureHistory,
    enabled: temperatureHistoryEnabled,
    saving: temperatureHistorySaving,
    setEnabled: setTemperatureHistoryEnabled,
  } = useTemperatureHistory();

  const activeProfile = useMemo(() => curveProfiles.find((p) => p.id === activeProfileId) ?? null, [curveProfiles, activeProfileId]);
  const pendingDeleteProfile = useMemo(
    () => curveProfiles.find((profile) => profile.id === pendingDeleteProfileId) ?? null,
    [curveProfiles, pendingDeleteProfileId],
  );
  const externalActiveProfileId = ((config as any).activeFanCurveProfileId || '') as string;
  const externalDeviceCurveKey = [
    ((config as any).deviceTransport || '') as string,
    ((config as any).activeDeviceProfileId || '') as string,
    isConnected ? ((runtimeDeviceProfile as any)?.transport || fanData?.transport || '') as string : '',
    isConnected ? ((runtimeDeviceProfile as any)?.id || (runtimeDeviceProfile as any)?.model || fanData?.speedUnit || '') as string : '',
  ].join(':');

  const temperatureRange = useMemo(() => ({
    min: FAN_CURVE_MIN_TEMP,
    max: FAN_CURVE_MAX_TEMP,
    ticks: FAN_CURVE_TEMPERATURE_TICKS,
  }), []);

  const syncConfigFromBackend = useCallback(async () => {
    try {
      const latest = await apiService.getConfig();
      onConfigChange(types.AppConfig.createFrom(latest));
    } catch {
      /* noop */
    }
  }, [onConfigChange]);

  const loadCurveProfiles = useCallback(async () => {
    try {
      const payload = await apiService.getFanCurveProfiles();
      const profiles = Array.isArray(payload?.profiles) ? payload.profiles : [];
      const activeId = payload?.activeId || profiles[0]?.id || '';
      setCurveProfiles(profiles);
      setActiveProfileId(activeId);
      const current = profiles.find((p) => p.id === activeId) ?? profiles[0];
      if (current) {
        setProfileNameInput(current.name || '');
        setLocalCurve(normalizeFanCurve(current.curve, speedRange.min, speedRange.max, defaultCurve));
        setHasUnsavedChanges(false);
      }
    } catch {
      /* noop */
    }
  }, [defaultCurve, speedRange.max, speedRange.min]);

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
    setTargetTempDraft(normalizeTargetTemp((smartControl as any).targetTemp ?? 68));
  }, [smartControl.targetTemp]);

  useEffect(() => {
    if (!focusTarget) {
      return;
    }

    const target = focusTarget === 'history-details' ? historyDetailsRef.current : curveEditorRef.current;
    if (!target) {
      return;
    }

    const frame = window.requestAnimationFrame(() => {
      target.scrollIntoView({ block: 'start' });
      onFocusHandled();
    });

    return () => {
      window.cancelAnimationFrame(frame);
    };
  }, [focusTarget, onFocusHandled]);

  const learnedOffsetSummary = useMemo(() => {
    const sourceCurve = localCurve.length > 0 ? localCurve : (config.fanCurve || []);
    return (smartControl.learnedOffsets || [])
      .map((value, index) => ({
        value: learnedOffsetForDisplay(constrainOffsetByLearningBias(typeof value === 'number' ? value : 0, currentLearningBias), speedUnit),
        index,
      }))
      .filter((item) => item.value !== 0 && item.index < sourceCurve.length)
      .sort((left, right) => Math.abs(right.value) - Math.abs(left.value))
      .slice(0, 4)
      .map((item) => ({
        ...item,
        temperature: sourceCurve[item.index]?.temperature,
      }));
  }, [config.fanCurve, currentLearningBias, localCurve, smartControl.learnedOffsets, speedUnit]);

  const detailHistoryPoints = useMemo(() => temperatureHistory.slice(-720), [temperatureHistory]);

  const historySummary = useMemo(() => {
    const latest = temperatureHistory[temperatureHistory.length - 1] ?? null;
    const first = temperatureHistory[0] ?? null;
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

    for (const point of temperatureHistory) {
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
      if (cpuPower > 0) {
        cpuPowerPeak = Math.max(cpuPowerPeak, cpuPower);
        cpuPowerSum += cpuPower;
        cpuPowerCount += 1;
      }
      const gpuPower = Number(point.gpuPowerWatts || 0);
      if (gpuPower > 0) {
        gpuPowerPeak = Math.max(gpuPowerPeak, gpuPower);
        gpuPowerSum += gpuPower;
        gpuPowerCount += 1;
      }
    }
    const average = (sum: number, count: number) => count > 0 ? Math.round(sum / count) : 0;

    return {
      sampleCount: temperatureHistory.length,
      latest,
      latestLabel: latest ? formatHistoryDateTime(latest.timestamp, locale) : '--',
      durationLabel: first && latest ? formatHistoryDuration(first.timestamp, latest.timestamp, t) : '--',
      cpuPeak,
      cpuAverage: average(cpuSum, cpuCount),
      gpuPeak,
      gpuAverage: average(gpuSum, gpuCount),
      fanPeak,
      fanAverage: average(fanSum, fanCount),
      cpuPowerPeak,
      cpuPowerAverage: average(cpuPowerSum, cpuPowerCount),
      gpuPowerPeak,
      gpuPowerAverage: average(gpuPowerSum, gpuPowerCount),
    };
  }, [locale, speedRange.max, speedRange.min, t, temperatureHistory]);

  const historyChartStats = useMemo(() => {
    let maxPower = 0;
    const data = detailHistoryPoints.map((point) => {
      const cpuPowerWatts = Number(point.cpuPowerWatts || 0);
      const gpuPowerWatts = Number(point.gpuPowerWatts || 0);
      if (historySeriesVisibility.cpuPower) {
        maxPower = Math.max(maxPower, cpuPowerWatts);
      }
      if (historySeriesVisibility.gpuPower) {
        maxPower = Math.max(maxPower, gpuPowerWatts);
      }
      const rawFanRpm = Number(point.fanRpm || 0);
      return {
        ...point,
        cpuTemp: point.cpuTemp > 0 ? point.cpuTemp : undefined,
        gpuTemp: point.gpuTemp > 0 ? point.gpuTemp : undefined,
        fanRpm: rawFanRpm > 0 ? normalizeSpeedValue(rawFanRpm, speedRange.min, speedRange.max) : undefined,
        cpuPowerWatts: cpuPowerWatts > 0 ? Math.round(cpuPowerWatts * 10) / 10 : undefined,
        gpuPowerWatts: gpuPowerWatts > 0 ? Math.round(gpuPowerWatts * 10) / 10 : undefined,
      };
    });

    const powerMax = maxPower > 0 ? Math.max(20, Math.ceil((maxPower + 10) / 10) * 10) : 20;

    return {
      data,
      powerMax,
      hasPower: maxPower > 0,
    };
  }, [detailHistoryPoints, historySeriesVisibility.cpuPower, historySeriesVisibility.gpuPower, speedRange.max, speedRange.min]);
  const historyChartData = historyChartStats.data;
  const historyPowerMax = historyChartStats.powerMax;
  const historyHasPower = historyChartStats.hasPower;

  const historySeriesMeta = useMemo(() => {
    const meta: Record<HistorySeriesKey, { key: HistorySeriesKey; label: string; color: string; dataKey: string; axisId: 'temp' | 'fan' | 'power' }> = {
      cpu: { key: 'cpu', label: t('fanCurve.history.series.cpu'), color: CPU_TEMP_STROKE, dataKey: HISTORY_SERIES_DATA_KEY.cpu, axisId: HISTORY_SERIES_AXIS.cpu },
      gpu: { key: 'gpu', label: t('fanCurve.history.series.gpu'), color: GPU_TEMP_STROKE, dataKey: HISTORY_SERIES_DATA_KEY.gpu, axisId: HISTORY_SERIES_AXIS.gpu },
      fan: { key: 'fan', label: t('fanCurve.history.series.fan'), color: FAN_SPEED_STROKE, dataKey: HISTORY_SERIES_DATA_KEY.fan, axisId: HISTORY_SERIES_AXIS.fan },
      cpuPower: { key: 'cpuPower', label: t('fanCurve.history.series.cpuPower'), color: CPU_POWER_STROKE, dataKey: HISTORY_SERIES_DATA_KEY.cpuPower, axisId: HISTORY_SERIES_AXIS.cpuPower },
      gpuPower: { key: 'gpuPower', label: t('fanCurve.history.series.gpuPower'), color: GPU_POWER_STROKE, dataKey: HISTORY_SERIES_DATA_KEY.gpuPower, axisId: HISTORY_SERIES_AXIS.gpuPower },
    };
    return orderedSeries.map((key) => meta[key]).filter(Boolean);
  }, [orderedSeries, t, locale]);

  const historySeriesByDataKey = useMemo(() => {
    const entries = historySeriesMeta.map((series) => [series.dataKey, series] as const);
    return new Map(entries);
  }, [historySeriesMeta]);

  const historySeriesOrderByDataKey = useMemo(() => {
    const entries = historySeriesMeta.map((series, index) => [series.dataKey, index] as const);
    return new Map(entries);
  }, [historySeriesMeta]);

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

  useEffect(() => {
    if ((!isInitialized || !hasUnsavedChanges) && !isInteracting && config.fanCurve && config.fanCurve.length > 0) {
      setLocalCurve(normalizeFanCurve(config.fanCurve, speedRange.min, speedRange.max, defaultCurve));
      setIsInitialized(true);
    }
  }, [config.fanCurve, defaultCurve, hasUnsavedChanges, isInitialized, isInteracting, speedRange.max, speedRange.min]);

  useEffect(() => {
    loadCurveProfiles().catch(() => {});
  }, [externalActiveProfileId, externalDeviceCurveKey, loadCurveProfiles]);

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

  const hasLearnedOffsets = learnedOffsetSummary.length > 0;
  const showCoupledCurve = config.autoControl && !!smartControl.learning && hasLearnedOffsets;

  /* ── Point update + drag ── */

  const updatePoint = useCallback((index: number, newRpm: number) => {
    let didChange = false;

    setLocalCurve((prev) => {
      const nextState = syncCurveSpeedAtIndex(prev, index, newRpm, speedRange.min, speedRange.max);

      if (!nextState.changed) {
        return prev;
      }

      didChange = true;
      return nextState.curve;
    });

    if (didChange) {
      setHasUnsavedChanges(true);
    }
  }, [speedRange]);

  const handleDragStart = useCallback((index: number) => {
    setDragIndex(index);
    setIsInteracting(true);
    if (chartRef.current) {
      const chartArea = chartRef.current.querySelector('.recharts-cartesian-grid');
      if (chartArea) {
        const rect = chartArea.getBoundingClientRect();
        chartBoundsRef.current = { top: rect.top, bottom: rect.bottom, left: rect.left, right: rect.right, yMin: speedRange.min, yMax: speedRange.max };
      }
    }
  }, [speedRange]);

  const handleDrag = useCallback((clientY: number) => {
    if (dragIndex === null || !chartBoundsRef.current) return;
    const bounds = chartBoundsRef.current;
    const relativeY = Math.max(0, Math.min(1, (bounds.bottom - clientY) / (bounds.bottom - bounds.top)));
    updatePoint(dragIndex, bounds.yMin + relativeY * (bounds.yMax - bounds.yMin));
  }, [dragIndex, updatePoint]);

  const scheduleDrag = useCallback((clientY: number) => {
    pendingDragYRef.current = clientY;
    if (dragFrameRef.current !== null) {
      return;
    }

    dragFrameRef.current = window.requestAnimationFrame(() => {
      dragFrameRef.current = null;
      const nextClientY = pendingDragYRef.current;
      pendingDragYRef.current = null;
      if (nextClientY !== null) {
        handleDrag(nextClientY);
      }
    });
  }, [handleDrag]);

  const handleDragEnd = useCallback(() => {
    if (dragFrameRef.current !== null) {
      window.cancelAnimationFrame(dragFrameRef.current);
      dragFrameRef.current = null;
    }
    pendingDragYRef.current = null;
    setDragIndex(null);
    setTimeout(() => setIsInteracting(false), 100);
  }, []);

  useEffect(() => {
    if (dragIndex === null) return;
    const mm = (e: MouseEvent) => { e.preventDefault(); scheduleDrag(e.clientY); };
    const tm = (e: TouchEvent) => { if (e.touches.length > 0) scheduleDrag(e.touches[0].clientY); };
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
      pendingDragYRef.current = null;
    };
  }, [dragIndex, handleDragEnd, scheduleDrag]);

  /* ── Save / Reset ── */

  const persistCurrentCurve = useCallback(async () => {
    if (isSaving) return;
    try {
      setIsSaving(true);
      const curveToSave = normalizeFanCurve(localCurve, speedRange.min, speedRange.max, defaultCurve);
      const profileID = activeProfileId || (((config as any).activeFanCurveProfileId || '') as string);
      const profileName = activeProfile?.name || t('fanCurve.profiles.currentCurveName');
      await apiService.saveFanCurveProfile(profileID, profileName, curveToSave, true);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      setLocalCurve(curveToSave);
      setHasUnsavedChanges(false);
      return true;
    } catch (e) {
      toast.error(t('fanCurve.toast.saveCurveFailed', { error: getErrorMessage(e) }));
      return false;
    } finally {
      setIsSaving(false);
    }
  }, [activeProfile?.name, activeProfileId, config, isSaving, loadCurveProfiles, localCurve, syncConfigFromBackend, t]);

  const saveCurve = useCallback(async () => {
    await persistCurrentCurve();
  }, [persistCurrentCurve]);

  const getSafeProfileName = useCallback((input: string, fallback: string) => {
    const name = (input || '').trim() || fallback;
    const runes = Array.from(name);
    return runes.slice(0, 6).join('');
  }, []);

  const trimProfileNameToLimit = useCallback((value: string) => {
    return Array.from(value).slice(0, 6).join('');
  }, []);

  const handleProfileNameInputChange = useCallback((value: string, composing: boolean) => {
    if (composing || isProfileNameComposing) {
      setProfileNameInput(value);
      return;
    }
    setProfileNameInput(trimProfileNameToLimit(value));
  }, [isProfileNameComposing, trimProfileNameToLimit]);

  const handleProfileNameCompositionStart = useCallback(() => {
    setIsProfileNameComposing(true);
  }, []);

  const handleProfileNameCompositionEnd = useCallback((value: string) => {
    setIsProfileNameComposing(false);
    setProfileNameInput(trimProfileNameToLimit(value));
  }, [trimProfileNameToLimit]);

  const applyProfileSwitch = useCallback(async (id: string) => {
    try {
      setProfileOpLoading(true);
      await apiService.setActiveFanCurveProfile(id);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      toast.success(t('fanCurve.toast.profileSwitched'));
    } catch (e) {
      toast.error(t('fanCurve.toast.switchFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [loadCurveProfiles, syncConfigFromBackend, t]);

  const switchProfile = useCallback(async (id: string) => {
    if (!id || id === activeProfileId) return;
    if (hasUnsavedChanges) {
      setPendingProfileId(id);
      setProfileSwitchDialogOpen(true);
      return;
    }
    await applyProfileSwitch(id);
  }, [activeProfileId, applyProfileSwitch, hasUnsavedChanges]);

  const confirmProfileSwitch = useCallback(async (action: 'save' | 'discard') => {
    if (!pendingProfileId) return;
    if (action === 'save') {
      const saved = await persistCurrentCurve();
      if (!saved) return;
    }
    const nextProfileId = pendingProfileId;
    setProfileSwitchDialogOpen(false);
    setPendingProfileId('');
    await applyProfileSwitch(nextProfileId);
  }, [applyProfileSwitch, pendingProfileId, persistCurrentCurve]);

  const saveCurrentProfileName = useCallback(async () => {
    const fallbackName = activeProfile?.name || t('fanCurve.profiles.currentCurveName');
    const safeName = getSafeProfileName(profileNameInput, fallbackName);
    try {
      setProfileOpLoading(true);
      const profileCurve = activeProfile?.curve || localCurve;
      await apiService.saveFanCurveProfile(activeProfileId, safeName, profileCurve, false);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      toast.success(t('fanCurve.toast.profileRenamed'));
    } catch (e) {
      toast.error(t('fanCurve.toast.renameFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [activeProfile?.curve, activeProfile?.name, activeProfileId, getSafeProfileName, loadCurveProfiles, localCurve, profileNameInput, syncConfigFromBackend, t]);

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
      setDeleteProfileDialogOpen(false);
      setPendingDeleteProfileId('');
      toast.success(t('fanCurve.toast.profileDeleted'));
    } catch (e) {
      toast.error(t('fanCurve.toast.deleteFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [activeProfileId, loadCurveProfiles, pendingDeleteProfileId, syncConfigFromBackend, t]);

  const exportProfiles = useCallback(async (destination: 'clipboard' | 'file') => {
    try {
      if (hasUnsavedChanges) {
        const ok = await persistCurrentCurve();
        if (!ok) {
          return;
        }
      }
      if (destination === 'file') {
        const savedPath = await apiService.exportFanCurveProfilesToFile();
        if (!savedPath) return;
        toast.success(t('fanCurve.toast.exportFileCreated'));
        return;
      }
      const code = await apiService.exportFanCurveProfiles();
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(code);
      } else {
        await ClipboardSetText(code);
      }
      toast.success(t('fanCurve.toast.exportCopied'));
    } catch (e) {
      toast.error(t('fanCurve.toast.exportFailed', { error: getErrorMessage(e) }));
    }
  }, [hasUnsavedChanges, persistCurrentCurve, t]);

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

  const resetCurve = useCallback(() => {
    setLocalCurve(normalizeFanCurve(defaultCurve, speedRange.min, speedRange.max, defaultCurve));
    setHasUnsavedChanges(true);
  }, [defaultCurve, speedRange.max, speedRange.min]);

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

  /* ── Reference temperature (follows settings 控温温度来源: max/cpu/gpu) ── */
  const referenceTemp = useMemo(() => {
    if (!temperature) return null;
    const source = (((config as any).tempSource as string) || 'max') as 'max' | 'cpu' | 'gpu';
    const cpu = temperature.cpuTemp ?? 0;
    const gpu = temperature.gpuTemp ?? 0;
    const max = temperature.maxTemp ?? 0;
    if (source === 'cpu') return cpu > 0 ? cpu : (max > 0 ? max : null);
    if (source === 'gpu') return gpu > 0 ? gpu : (max > 0 ? max : null);
    return max > 0 ? max : null;
  }, [temperature, config]);

  /* ── Manual gear ── */

  const customGearRpm = useMemo(() => {
    return ((config as any).manualGearRpm ?? null) as ManualGearRpmMap | null;
  }, [config]);

  const manualGearPresets = useMemo(() => {
    return getEffectiveManualGearPresets(customGearRpm, speedUnit);
  }, [customGearRpm, speedUnit]);

  const manualGearDefaultPresets = useMemo(() => {
    return getManualGearDefaultPresets(speedUnit);
  }, [speedUnit]);

  const manualGearValueRange = useMemo(() => {
    return getManualGearValueRange(speedUnit);
  }, [speedUnit]);

  const supportsManualGears = supportsManualGearsFromCapabilities(
    isConnected && runtimeDeviceCapabilities ? runtimeDeviceCapabilities : runtimeDeviceProfile?.capabilities,
  );
  const flyDigiCapability = useMemo(() => getFlyDigiRuntimeCapability(fanData as any), [fanData]);
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

  const [gearEditOpen, setGearEditOpen] = useState(false);
  const [draftGearRpm, setDraftGearRpm] = useState<ManualGearRpmMap>({});
  const [gearRpmSaving, setGearRpmSaving] = useState(false);

  const buildDraftFrom = useCallback((source: ManualGearRpmMap | null): ManualGearRpmMap => {
    const base: ManualGearRpmMap = {};
    getEffectiveManualGearPresets(source, speedUnit).forEach((preset) => {
      base[preset.gear] = {};
      preset.levels.forEach((lv) => {
        base[preset.gear][lv.level] = Math.max(manualGearValueRange.min, Math.min(manualGearValueRange.max, lv.rpm));
      });
    });
    return base;
  }, [manualGearValueRange.max, manualGearValueRange.min, speedUnit]);

  const openGearEditor = useCallback(() => {
    if (!supportsManualGears) {
      toast.error(t('fanCurve.manualGear.unavailable'));
      return;
    }
    setDraftGearRpm(buildDraftFrom(customGearRpm));
    setGearEditOpen(true);
  }, [buildDraftFrom, customGearRpm, supportsManualGears, t]);

  const setDraftRpm = useCallback((gear: string, level: string, value: number) => {
    setDraftGearRpm((prev) => ({
      ...prev,
      [gear]: { ...(prev[gear] ?? {}), [level]: value },
    }));
  }, []);

  const saveGearRpm = useCallback(async () => {
    if (!supportsManualGears) {
      toast.error(t('fanCurve.manualGear.unavailable'));
      return;
    }
    setGearRpmSaving(true);
    try {
      const normalized = normalizeManualGearRpmMap(draftGearRpm, manualGearValueRange.min, manualGearValueRange.max, speedUnit);
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
  }, [config, draftGearRpm, manualGearValueRange.max, manualGearValueRange.min, onConfigChange, speedUnit, supportsManualGears, t]);

  const CustomDot = useCallback((props: any): React.ReactElement<SVGElement> => {
    const { cx, cy, index, payload } = props;
    if (cx === undefined || cy === undefined) return <g />;
    return <DraggablePoint key={`dot-${index}`} cx={cx} cy={cy} index={index} temperature={payload.temperature} speed={payload.rpm} unitSuffix={speedUnitSuffix} onDragStart={handleDragStart} isActive={dragIndex === index} />;
  }, [dragIndex, handleDragStart, speedUnitSuffix]);

  return (
    <div
      data-theme-section="curve-page"
      data-page-reveal={initialFocusTarget === 'history-details' ? 'cards-reverse' : initialFocusTarget ? undefined : 'cards'}
      className="relative space-y-4 px-1 pb-2"
    >
        <motion.div
          data-theme-card="curve-header"
          initial={initialFocusTarget ? false : { opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.2 }}
          className="relative px-1 py-1"
        >
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <Spline className="h-4 w-4 text-primary" />
              <h2 className="text-base font-semibold text-foreground">{t('fanCurve.title')}</h2>
              {hasUnsavedChanges && <Badge variant="warning">{t('fanCurve.badges.unsaved')}</Badge>}
              {isInteracting && <Badge variant="info">{t('fanCurve.badges.editing')}</Badge>}
            </div>

            <div data-curve-profile-row className="flex min-w-0 items-center gap-3">
              <div data-curve-profile-toolbar className="flex min-w-0 flex-1 items-center gap-1.5 rounded-xl border border-border/70 bg-card/70 p-1.5 shadow-sm shadow-black/5">
                <div data-curve-profile-list className="flex min-w-0 flex-1 items-center gap-1.5 overflow-x-auto px-0.5 py-0.5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                  {curveProfiles.map((profile) => {
                    const isActive = profile.id === activeProfileId;
                    return (
                      <div key={profile.id} className="group flex shrink-0 items-start gap-0">
                        <button
                          type="button"
                          onClick={() => void switchProfile(profile.id)}
                          disabled={profileOpLoading}
                          className={clsx(
                            'h-9 cursor-pointer truncate whitespace-nowrap rounded-full border px-4 text-center text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-60',
                            isActive
                              ? 'border-primary/40 bg-primary/10 text-primary'
                              : 'border-border/70 bg-background/55 text-muted-foreground hover:border-border hover:bg-muted/65 hover:text-foreground',
                          )}
                          aria-current={isActive ? 'true' : undefined}
                        >
                          {profile.name}
                        </button>
                        {curveProfiles.length > 1 && (
                          <button
                            type="button"
                            onClick={() => {
                              setPendingDeleteProfileId(profile.id);
                              setDeleteProfileDialogOpen(true);
                            }}
                            disabled={profileOpLoading}
                            className="-ml-px flex h-[13px] w-[13px] cursor-pointer items-center justify-center rounded-full border border-transparent bg-muted/60 text-muted-foreground/80 opacity-0 transition-colors group-hover:opacity-100 group-focus-within:opacity-100 hover:border-destructive/30 hover:bg-destructive/15 hover:text-destructive focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed"
                            aria-label={t('fanCurve.profiles.deleteProfileLabel', { name: profile.name })}
                            title={t('fanCurve.profiles.deleteProfileLabel', { name: profile.name })}
                          >
                            <X className="h-2 w-2" />
                          </button>
                        )}
                      </div>
                    );
                  })}
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
                  onClick={() => setManageProfilesDialogOpen(true)}
                  disabled={profileOpLoading || curveProfiles.length === 0}
                  icon={<Settings2 className="h-3.5 w-3.5" />}
                >
                  {t('fanCurve.profiles.manage')}
                </Button>
              </div>
              <div className="flex shrink-0 items-center gap-3">
                <ToggleSwitch enabled={config.autoControl} onChange={handleAutoControlChange} label={t('fanCurve.actions.smartControl')} size="sm" color="blue" />
                <Button variant="secondary" size="sm" className="rounded-lg" onClick={resetCurve} icon={<RotateCw className="h-3.5 w-3.5" />}>
                  {t('fanCurve.actions.reset')}
                </Button>
                <Button variant="primary" size="sm" className="rounded-lg" onClick={saveCurve} disabled={!hasUnsavedChanges} loading={isSaving} icon={<Check className="h-3.5 w-3.5" />}>
                  {t('common.actions.save')}
                </Button>
              </div>
            </div>
          </div>
        </motion.div>

        <AnimatePresence>
          {!config.autoControl && isConnected && supportsManualGears && (
            <motion.div
              initial={initialFocusTarget ? false : { opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              className="overflow-hidden"
            >
              <div data-theme-card="curve-manual-gears" className="rounded-2xl border border-border/70 bg-card p-4 space-y-4 shadow-sm">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="text-sm font-medium">{t('fanCurve.manualGear.title')}</span>
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-xs text-muted-foreground">{manualGearLimitHint || t('fanCurve.manualGear.defaultSliderHint')}</span>
                    <Button variant="secondary" size="sm" onClick={openGearEditor} icon={<Pencil className="h-3.5 w-3.5" />}>
                      {t('fanCurve.manualGear.customize')}
                    </Button>
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
                  {manualGearPresets.map((preset) => {
                    const isActiveGear = (config.manualGear || '标准') === preset.gear;
                    const rememberedLevel = isActiveGear
                      ? (config.manualLevel || '中')
                      : rememberedManualGearLevels[preset.gear];
                    const activeLevel = preset.levels.find((l) => l.level === rememberedLevel) ?? preset.levels[0];
                    const gearAllowed = isManualGearAllowed(preset.gear);

                    return (
                      <button
                        key={preset.gear}
                        type="button"
                        disabled={!gearAllowed}
                        onClick={() => handleGearCardSelect(preset.gear)}
                        title={!gearAllowed ? t('fanCurve.manualGear.runtimeUnavailable', { gear: getManualGearLabel(preset.gear), limit: manualGearLimitHint }) : undefined}
                        className={clsx(
                          'rounded-xl border px-3 py-2.5 text-left transition-colors',
                          gearAllowed ? 'cursor-pointer' : 'cursor-not-allowed opacity-45',
                          isActiveGear ? `${preset.borderClass} ${preset.bgClass}` : 'border-border/70 bg-background/40 hover:bg-muted/35',
                        )}
                      >
                        <div className={clsx('text-lg font-bold', isActiveGear ? preset.colorClass : 'text-foreground')}>
                          {getManualGearLabel(preset.gear)}
                        </div>
                        <div className={clsx('mt-1 text-base font-semibold tabular-nums', preset.colorClass)}>
                          {activeLevel.rpm}{speedUnitSuffix}
                        </div>
                      </button>
                    );
                  })}
                </div>

                <div className="rounded-xl border border-border/70 bg-background/40 p-3">
                  <div className="relative mb-3 px-2">
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
                                'block h-4 w-4 rounded-full border border-border/80 shadow-[inset_0_1px_0_rgba(255,255,255,0.32)] transition-transform duration-150',
                                point.bgClass,
                                isActivePoint ? `scale-125 ${point.borderClass}` : '',
                              )}
                            />
                          </button>
                        );
                      })}
                    </div>
                  </div>

                  <div className="flex items-start justify-between px-2 text-[11px]">
                    {manualPoints.map((point) => (
                      <span key={`${point.key}-label`} className={clsx('w-6 truncate text-center', point.colorClass)}>
                        {t('fanCurve.manualGear.pointIndex', { index: point.levelIndex + 1 })}
                      </span>
                    ))}
                  </div>
                </div>
              </div>
            </motion.div>
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
                  <div className={clsx('mb-2 text-sm font-semibold', preset.colorClass)}>{getManualGearLabel(preset.gear)}</div>
                  <div className="grid grid-cols-3 gap-2">
                    {preset.levels.map((lv) => (
                      <div key={lv.level} className="space-y-1">
                        <div className="text-[11px] text-muted-foreground">{getManualLevelLabel(lv.level)}</div>
                        <NumberInput
                          value={draftGearRpm[preset.gear]?.[lv.level] ?? lv.rpm}
                          onChange={(value) => setDraftRpm(preset.gear, lv.level, value)}
                          min={manualGearValueRange.min}
                          max={manualGearValueRange.max}
                          step={1}
                          suffix={speedUnitSuffix}
                        />
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
            <DialogFooter>
              <Button variant="secondary" size="sm" onClick={() => setDraftGearRpm(buildDraftFrom(null))} icon={<RotateCw className="h-3.5 w-3.5" />}>
                {t('fanCurve.manualGear.restoreDefault')}
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setGearEditOpen(false)} icon={<X className="h-3.5 w-3.5" />}>
                {t('common.actions.cancel')}
              </Button>
              <Button variant="primary" size="sm" onClick={saveGearRpm} loading={gearRpmSaving} icon={<Check className="h-3.5 w-3.5" />}>
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
            <div className="h-80 md:h-96 relative">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={chartData} margin={{ top: 20, right: 30, left: 20, bottom: 20 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--chart-grid)" />
                  <XAxis dataKey="temperature" type="number" domain={[temperatureRange.min, temperatureRange.max]} ticks={temperatureRange.ticks} interval={0} minTickGap={0} tickLine={false} axisLine={{ stroke: 'var(--chart-axis)' }} tick={{ fill: 'var(--chart-tick)', fontSize: 10 }} label={{ value: t('fanCurve.chart.axes.temperature'), position: 'insideBottom', offset: -10, fill: 'var(--chart-tick)', fontSize: 12 }} />
                  <YAxis type="number" domain={[speedRange.min, speedRange.max]} ticks={speedRange.ticks} tickLine={false} axisLine={{ stroke: 'var(--chart-axis)' }} tick={{ fill: 'var(--chart-tick)', fontSize: 11 }} label={{ value: `速度（${speedUnitSuffix}）`, angle: -90, position: 'insideLeft', fill: 'var(--chart-tick)', fontSize: 12 }} />
                  <RechartsTooltip
                    formatter={(value, name) => {
                      const numericValue = Number(value ?? 0);
                      return name === 'coupledRpm' ? [`${formatSpeedValue(numericValue)}${speedUnitSuffix}`, '学习曲线'] : [`${formatSpeedValue(numericValue)}${speedUnitSuffix}`, '基础曲线'];
                    }}
                    labelFormatter={(v) => t('fanCurve.chart.temperatureLabel', { temperature: v })}
                    contentStyle={{ backgroundColor: 'var(--chart-tooltip-bg)', border: '1px solid', borderColor: 'var(--chart-tooltip-border)', borderRadius: '8px', boxShadow: 'var(--chart-tooltip-shadow)', padding: '8px 12px', color: 'var(--chart-tooltip-text)' }}
                    labelStyle={{ color: 'var(--chart-tooltip-text)', fontWeight: 600 }}
                    itemStyle={{ color: 'var(--chart-tooltip-text)' }}
                  />
                  <Line type="monotone" dataKey="rpm" stroke="var(--chart-primary)" strokeWidth={3} dot={CustomDot} activeDot={false} isAnimationActive={false} />
                  {showCoupledCurve && <Line type="monotone" dataKey="coupledRpm" stroke="var(--chart-primary)" strokeWidth={2} strokeDasharray="6 4" dot={false} activeDot={false} isAnimationActive={false} />}
                </LineChart>
              </ResponsiveContainer>
              <TemperatureIndicator temperature={referenceTemp} chartRef={chartRef} temperatureRange={temperatureRange} />
            </div>
          </div>
        </div>

        <section data-theme-card="curve-prediction" className="rounded-2xl border border-border/70 bg-card p-4 shadow-sm">
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <Radar className="h-4 w-4 text-sky-500" />
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="text-sm font-medium text-foreground">{t('fanCurve.prediction.title')}</div>
                </div>
                <div className="text-xs leading-relaxed text-muted-foreground">{t('fanCurve.prediction.description')}</div>
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <span className="rounded-full border border-amber-500/30 bg-amber-500/10 px-2 py-0.5 text-[11px] font-medium text-amber-700 dark:text-amber-300">
                {t('fanCurve.prediction.badge')}
              </span>
              <ToggleSwitch
                enabled={!!smartControl.temperatureRisePrediction}
                onChange={handleTemperatureRisePredictionToggle}
                loading={learningConfigLoading}
                size="sm"
                color="blue"
                srLabel={t('fanCurve.prediction.toggleAria')}
              />
            </div>
          </div>
        </section>

        <section data-theme-card="curve-learning" className="rounded-2xl border border-border/70 bg-card p-4 shadow-sm">
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <Sparkles className="h-4 w-4 text-amber-500" />
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="text-sm font-medium text-foreground">{t('fanCurve.learning.title')}</div>
                </div>
                <div className="text-xs leading-relaxed text-muted-foreground">根据温度变化微调当前设备速度曲线。</div>
              </div>
            </div>
            <ToggleSwitch
              enabled={!!smartControl.learning}
              onChange={handleLearningToggle}
              loading={learningConfigLoading}
              size="sm"
              color="purple"
              srLabel={t('fanCurve.learning.toggleAria')}
            />
          </div>

          <div className="mt-3 flex flex-col gap-3 rounded-xl border border-border/70 bg-background/45 p-3">
            <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
              <div className="min-w-0">
                <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.learning.biasTitle')}</div>
                <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{currentLearningBiasOption.description}</div>
              </div>
              <Select
                value={currentLearningBias}
                onChange={handleLearningBiasChange}
                options={learningBiasOptions}
                disabled={learningConfigLoading}
                size="sm"
                className="w-full md:w-44"
              />
            </div>

            <div data-theme-ui="learning-target-temp" className="flex flex-col gap-3 rounded-xl border border-border/70 bg-card/55 p-3">
              <div className="min-w-0">
                <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.learning.targetTitle')}</div>
                <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.learning.targetDescription')}</div>
              </div>
              <div className="flex flex-col gap-3 md:flex-row md:items-center">
                <div className="min-w-0 flex-1">
                  <Slider
                    className="learning-target-temp-slider"
                    min={SMART_CONTROL_TARGET_TEMP_MIN}
                    max={SMART_CONTROL_TARGET_TEMP_MAX}
                    step={1}
                    value={targetTempDraft}
                    onChange={handleTargetTempSliderChange}
                    onChangeEnd={handleTargetTempSliderCommit}
                    valueFormatter={(value) => `${value}°C`}
                    disabled={learningConfigLoading}
                  />
                </div>
                <div className="w-full md:w-28">
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
            </div>

            <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div className="min-w-0">
                <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.learning.offsetTitle')}</div>
                <div className="mt-1 text-xs leading-relaxed text-muted-foreground">当前学习曲线相对基础曲线的主要速度修正点。</div>
              </div>
              <Button
                variant="secondary"
                size="sm"
                onClick={handleResetLearnedOffsets}
                loading={learningResetLoading}
                disabled={!hasLearnedOffsets}
                icon={<Sparkles className="h-3.5 w-3.5" />}
              >
                {t('fanCurve.learning.reset')}
              </Button>
            </div>

            {hasLearnedOffsets ? (
              <div className="grid grid-cols-2 gap-2 text-xs text-muted-foreground md:grid-cols-4">
                {learnedOffsetSummary.map((item) => (
                  <div key={item.index} className="rounded-lg border border-border/70 bg-card/70 px-3 py-2 tabular-nums">
                    <span>{item.temperature}°C </span>
                    <span className={clsx('font-semibold', item.value > 0 ? 'text-orange-500' : 'text-blue-500')}>
                      {item.value > 0 ? '+' : ''}{formatSpeedValue(item.value)}{speedUnitSuffix}
                    </span>
                  </div>
                ))}
              </div>
            ) : (
              <div className="rounded-lg border border-dashed border-border/70 bg-card/55 px-3 py-2 text-xs text-muted-foreground">{t('fanCurve.learning.noOffsets')}</div>
            )}
          </div>
        </section>

        <div data-theme-ui="curve-hints" className="flex flex-wrap gap-2">
          <span className="rounded-full border border-border/70 bg-background/60 px-3 py-1 text-[11px] text-muted-foreground backdrop-blur-lg">拖动蓝色点调整速度（{speedUnitSuffix}）</span>
          {showCoupledCurve && <span className="rounded-full border border-border/70 bg-background/60 px-3 py-1 text-[11px] text-muted-foreground backdrop-blur-lg">{t('fanCurve.hints.curveLegend')}</span>}
        </div>

        <section ref={historyDetailsRef} data-theme-card="curve-history" className="rounded-2xl border border-border/70 bg-card p-4 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2">
              <History className="h-4 w-4 text-primary" />
              <div>
                <div className="text-sm font-medium text-foreground">{t('fanCurve.history.detailsTitle')}</div>
                <div className="text-xs text-muted-foreground">查看温度与风扇速度的近期变化。</div>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <ToggleSwitch
                enabled={temperatureHistoryEnabled}
                onChange={setTemperatureHistoryEnabled}
                loading={temperatureHistorySaving}
                label={temperatureHistorySaving ? t('fanCurve.history.saving') : t('fanCurve.history.backgroundRecording')}
                size="sm"
                color="blue"
              />
            </div>
          </div>

          {temperatureHistory.length === 0 ? (
            <div className="rounded-xl border border-dashed border-border/70 bg-background/35 px-4 py-8 text-center text-sm text-muted-foreground">
              {temperatureHistoryEnabled ? t('fanCurve.history.emptyEnabled') : t('fanCurve.history.emptyDisabled')}
            </div>
          ) : (
            <>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-5">
                {[
                  [t('fanCurve.history.summary.cpuPeak'), historySummary.cpuPeak ? `${historySummary.cpuPeak}°C` : '--', historySummary.cpuAverage ? t('fanCurve.history.summary.averageTemperature', { value: historySummary.cpuAverage }) : t('fanCurve.history.summary.noCpuTemperature')],
                  [t('fanCurve.history.summary.gpuPeak'), historySummary.gpuPeak ? `${historySummary.gpuPeak}°C` : '--', historySummary.gpuAverage ? t('fanCurve.history.summary.averageTemperature', { value: historySummary.gpuAverage }) : t('fanCurve.history.summary.noGpuTemperature')],
                  [t('fanCurve.history.summary.fanPeak'), historySummary.fanPeak ? `${formatSpeedValue(historySummary.fanPeak)}${speedUnitSuffix}` : '--', historySummary.fanAverage ? t('fanCurve.history.summary.averageFan', { value: formatSpeedValue(historySummary.fanAverage), unit: speedUnitSuffix }) : t('fanCurve.history.summary.noFanData')],
                  [t('fanCurve.history.summary.cpuPowerPeak'), historySummary.cpuPowerPeak ? `${formatPowerValue(historySummary.cpuPowerPeak)} W` : '-- W', historySummary.cpuPowerAverage ? t('fanCurve.history.summary.averagePower', { value: formatPowerValue(historySummary.cpuPowerAverage) }) : t('fanCurve.history.summary.noPowerData')],
                  [t('fanCurve.history.summary.gpuPowerPeak'), historySummary.gpuPowerPeak ? `${formatPowerValue(historySummary.gpuPowerPeak)} W` : '-- W', historySummary.gpuPowerAverage ? t('fanCurve.history.summary.averagePower', { value: formatPowerValue(historySummary.gpuPowerAverage) }) : t('fanCurve.history.summary.noPowerData')],
                ].map(([label, value, hint]) => (
                  <div key={label} data-theme-card="curve-history-summary" className="rounded-xl border border-border/70 bg-background/35 p-3">
                    <div className="text-[11px] text-muted-foreground">{label}</div>
                    <div className="mt-1 text-sm font-semibold text-foreground">{value}</div>
                    <div className="mt-1 text-[11px] text-muted-foreground">{hint}</div>
                  </div>
                ))}
              </div>

              <div data-theme-card="curve-history-chart" className="rounded-xl border border-border/70 bg-background/35 p-3 space-y-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.history.recentTrend')}</div>
                  <div className="flex min-w-0 flex-wrap items-center gap-2">
                    {historySeriesMeta.map((series) => (
                      <button
                        key={series.key}
                        type="button"
                        onClick={() => toggleSeriesVisible(series.key)}
                        className={clsx(
                          'inline-flex cursor-pointer items-center gap-1.5 rounded-full border px-2.5 py-1 text-[11px] transition-colors',
                          historySeriesVisibility[series.key]
                            ? 'border-border bg-card text-foreground'
                            : 'border-border/60 bg-transparent text-muted-foreground/65',
                        )}
                      >
                        <span className="h-2 w-2 rounded-full" style={{ backgroundColor: series.color }} />
                        {series.label}
                      </button>
                    ))}
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      icon={<Settings2 className="h-3.5 w-3.5" />}
                      className="h-8 rounded-full px-2.5 text-[11px]"
                      onClick={() => setHistoryDisplayDialogOpen(true)}
                    >
                      {t('fanCurve.history.displaySettings')}
                    </Button>
                  </div>
                </div>

                {historyChartData.length < 2 ? (
                  <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">{t('fanCurve.history.waitingMoreSamples')}</div>
                ) : (
                  <div className="h-72">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={historyChartData} margin={{ top: 12, right: 16, left: 4, bottom: 8 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="var(--chart-grid)" />
                        <XAxis
                          dataKey="timestamp"
                          type="number"
                          domain={['dataMin', 'dataMax']}
                          tickFormatter={(value) => formatHistoryTime(Number(value), locale)}
                          tickLine={false}
                          minTickGap={24}
                          axisLine={{ stroke: 'var(--chart-axis)' }}
                          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
                        />
                        <YAxis
                          yAxisId="fan"
                          type="number"
                          domain={[speedRange.min, speedRange.max]}
                          ticks={speedRange.ticks}
                          allowDataOverflow
                          tickFormatter={(value) => `${formatSpeedValue(Number(value))}${speedUnitSuffix}`}
                          tickLine={false}
                          axisLine={{ stroke: 'var(--chart-axis)' }}
                          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
                          width={64}
                        />
                        <YAxis
                          yAxisId="temp"
                          orientation="right"
                          type="number"
                          domain={[temperatureRange.min, temperatureRange.max]}
                          ticks={temperatureRange.ticks.filter((value) => value === temperatureRange.max || (value - temperatureRange.min) % (FAN_CURVE_TEMP_STEP * 4) === 0)}
                          allowDataOverflow
                          tickFormatter={(value) => `${Number(value)}°C`}
                          tickLine={false}
                          axisLine={{ stroke: 'var(--chart-axis)' }}
                          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
                          width={44}
                        />
                        {historyHasPower && <YAxis
                          yAxisId="power"
                          orientation="right"
                          type="number"
                          domain={[0, historyPowerMax]}
                          tickFormatter={(value) => Number(value) === 0 ? '0 W' : `${formatPowerValue(Number(value))} W`}
                          tickLine={false}
                          axisLine={{ stroke: 'var(--chart-axis)' }}
                          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
                          width={54}
                        />}
                        <RechartsTooltip
                          content={({ active, label, payload }) => {
                            if (!active || !Array.isArray(payload) || payload.length === 0) {
                              return null;
                            }

                            const rows = payload
                              .map((item) => {
                                const dataKey = String(item.name ?? item.dataKey ?? '');
                                const series = historySeriesByDataKey.get(dataKey);
                                const numericValue = Number(item.value ?? 0);
                                if (!series || !Number.isFinite(numericValue) || numericValue <= 0) {
                                  return null;
                                }

                                const value = series.key === 'fan'
                                  ? `${formatSpeedValue(numericValue)}${speedUnitSuffix}`
                                  : series.key === 'cpuPower' || series.key === 'gpuPower'
                                    ? `${formatPowerValue(numericValue)} W`
                                    : `${numericValue} °C`;

                                return {
                                  key: series.key,
                                  label: series.label,
                                  value,
                                  color: series.color,
                                  order: historySeriesOrderByDataKey.get(series.dataKey) ?? Number.MAX_SAFE_INTEGER,
                                };
                              })
                              .filter((row): row is { key: HistorySeriesKey; label: string; value: string; color: string; order: number } => row !== null)
                              .sort((left, right) => left.order - right.order);

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
                          }}
                        />
                        {historySeriesMeta.map((series) => {
                          if (!historySeriesVisibility[series.key]) {
                            return null;
                          }
                          if ((series.key === 'cpuPower' || series.key === 'gpuPower') && !historyHasPower) {
                            return null;
                          }
                          return (
                            <Line
                              key={series.key}
                              yAxisId={series.axisId}
                              type="monotone"
                              dataKey={series.dataKey}
                              name={series.dataKey}
                              stroke={series.color}
                              strokeWidth={series.key === 'fan' ? 2 : 2.3}
                              dot={false}
                              activeDot={false}
                              isAnimationActive={false}
                              connectNulls={false}
                            />
                          );
                        })}
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                )}
              </div>
              <Dialog open={historyDisplayDialogOpen} onOpenChange={setHistoryDisplayDialogOpen}>
                <DialogContent data-theme-ui="history-display-dialog" className="max-w-md will-change-auto">
                  <DialogHeader>
                    <DialogTitle>{t('fanCurve.history.displaySettings')}</DialogTitle>
                    <DialogDescription>{t('fanCurve.history.displaySettingsDescription')}</DialogDescription>
                  </DialogHeader>
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

        <Dialog open={createProfileDialogOpen} onOpenChange={setCreateProfileDialogOpen}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>{t('fanCurve.profiles.createTitle')}</DialogTitle>
              <DialogDescription>{t('fanCurve.profiles.createDescription')}</DialogDescription>
            </DialogHeader>
            <Input
              value={newProfileNameInput}
              onChange={(event) => setNewProfileNameInput(trimProfileNameToLimit(event.target.value))}
              onKeyDown={(event) => {
                if (event.key === 'Enter' && !profileOpLoading) void createNewProfile();
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

        <Dialog open={manageProfilesDialogOpen} onOpenChange={setManageProfilesDialogOpen}>
          <DialogContent className="max-w-xl">
            <DialogHeader>
              <DialogTitle>{t('fanCurve.profiles.manageTitle')}</DialogTitle>
              <DialogDescription>{t('fanCurve.profiles.manageDescription')}</DialogDescription>
            </DialogHeader>

            <div className="space-y-5">
              <section className="space-y-2">
                <label className="text-xs font-medium text-muted-foreground" htmlFor="curve-profile-name">
                  {t('fanCurve.profiles.renameLabel')}
                </label>
                <div className="flex flex-wrap items-center gap-2">
                  <Input
                    id="curve-profile-name"
                    value={profileNameInput}
                    onChange={(e) => handleProfileNameInputChange(e.target.value, Boolean((e.nativeEvent as InputEvent).isComposing))}
                    onCompositionStart={handleProfileNameCompositionStart}
                    onCompositionEnd={(e) => handleProfileNameCompositionEnd(e.currentTarget.value)}
                    placeholder={t('fanCurve.profiles.namePlaceholder')}
                    className="h-10 min-w-[220px] flex-1"
                  />
                  <Button variant="secondary" size="sm" onClick={() => void saveCurrentProfileName()} loading={profileOpLoading} icon={<Pencil className="h-3.5 w-3.5" />}>
                    {t('fanCurve.profiles.saveName')}
                  </Button>
                </div>
              </section>

              <section data-profile-transfer className="space-y-3 border-t border-border/70 pt-4">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="text-sm font-medium">{t('fanCurve.importExport.title')}</span>
                  <span className="text-xs text-muted-foreground">{t('fanCurve.importExport.description')}</span>
                </div>
                <div className="space-y-3">
                  <div data-profile-export-section className="flex flex-wrap items-center justify-between gap-4 rounded-xl border border-border/70 bg-background/30 p-4">
                    <div className="min-w-[220px] flex-1">
                      <span className="text-xs font-medium text-muted-foreground">{t('fanCurve.importExport.exportTitle')}</span>
                      <p className="mt-2 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.importExport.exportHint')}</p>
                    </div>
                    <div className="flex shrink-0 flex-wrap gap-2">
                      <Button variant="secondary" size="sm" onClick={() => void exportProfiles('clipboard')} icon={<Clipboard className="h-3.5 w-3.5" />}>
                        {t('fanCurve.importExport.copyCode')}
                      </Button>
                      <Button variant="outline" size="sm" onClick={() => void exportProfiles('file')} icon={<Upload className="h-3.5 w-3.5" />}>
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
                      'space-y-3 rounded-xl border border-border/70 bg-background/30 p-4 transition-colors',
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
                      <Button variant="outline" size="sm" onClick={() => profileFileInputRef.current?.click()} icon={<Download className="h-3.5 w-3.5" />}>
                        {t('fanCurve.importExport.chooseFile')}
                      </Button>
                    </div>
                    <textarea
                      value={importCode}
                      onChange={(e) => setImportCode(e.target.value)}
                      rows={3}
                      className="w-full resize-none rounded-lg border border-border/70 bg-background px-3 py-2 text-xs leading-relaxed"
                      placeholder={t('fanCurve.importExport.importPlaceholder')}
                    />
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <span className="text-[11px] text-muted-foreground">{t('fanCurve.importExport.dropHint')}</span>
                      <Button
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
            </div>
          </DialogContent>
        </Dialog>

        <Dialog
          open={profileSwitchDialogOpen}
          onOpenChange={(open) => {
            setProfileSwitchDialogOpen(open);
            if (!open) setPendingProfileId('');
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
                  setPendingProfileId('');
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

      </div>
  );
});

export default FanCurve;
