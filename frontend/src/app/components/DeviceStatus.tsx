'use client';

import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import dynamic from 'next/dynamic';
import { m } from 'framer-motion';
import {
  AlertTriangle,
  ArrowUpRight,
  CircleHelp,
  Cpu,
  Download,
  Zap,
  RotateCw,
  Fan,
  Gpu,
  Settings,
  Gauge,
  ShieldCheck,
  Sparkles,
  Wifi,
} from 'lucide-react';
import { types } from '../../../wailsjs/go/models';
import { apiService } from '../services/api';
import { useTemperatureHistory } from '../hooks/useTemperatureHistory';
import { HISTORY_SERIES_ORDER, useHistoryDisplayPreferences } from '../hooks/useHistoryDisplayPreferences';
import { clipHistoryToRecentWindow, downsampleHistoryPoints, type HistorySeriesKey, type TemperatureHistoryPoint } from '../lib/temperature-history';
import {
  clampFanSpeedToRange,
  fanSpeedUnitLabel,
  formatFanSpeedValue,
  getActiveDeviceProfile,
  getFanSpeedRange,
  getFanSpeedTicks,
  getFanSpeedUnit,
  readCurrentFanSpeed,
  readTargetFanSpeed,
} from '../lib/fan-speed';
import { getFlyDigiRuntimeCapability } from '../lib/manualGearPresets';
import { translateWorkModeLabel } from '../lib/work-mode';
import type { DeviceSettings } from '../types/app';
import { applyPowerSpoofToTemperature } from '../lib/power-spoof';
import { selectFanData, selectTemperature, useAppStore } from '../store/app-store';
import { isBridgeStartingTelemetry } from '../store/app-store-logic';
import { useTranslation } from 'react-i18next';
import { ToggleSwitch, Button } from './ui/index';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import clsx from 'clsx';
import { toast } from 'sonner';

const FanCurve = dynamic(() => import('./FanCurve'), { ssr: false, loading: () => null });

interface DeviceStatusProps {
  isConnected: boolean;
  runtimeState: string;
  deviceProductId: string | null;
  deviceModel: string | null;
  deviceSettings: DeviceSettings | null;
  fanData?: types.FanData | null;
  temperature?: types.TemperatureData | null;
  runtimeDeviceProfile?: types.DeviceProfile | null;
	config: types.AppConfig;
	coreServiceError?: string | null;
	monitorOnlyActive?: boolean;
	onConnect: () => void;
	onDisconnect: () => void;
	onEnableMonitorOnly?: () => void;
  onConfigChange: (config: types.AppConfig) => void;
  onOpenCurveEditor: () => void;
  onOpenHistoryDetails: () => void;
  onExportDiagnostics?: () => void;
  diagnosticsExporting?: boolean;
}

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

interface BridgeRuntimeStatus {
  state?: string;
  working?: boolean;
  ownsProcess?: boolean;
  pipeName?: string;
  lastError?: string;
}

const getTempStatus = (temp: number) => {
  if (temp > 85) return { color: 'text-red-500', bg: 'bg-red-500', colorVar: 'var(--status-temperature-hot)', labelKey: 'deviceStatus.tempStatus.overheat' };
  if (temp > 75) return { color: 'text-orange-500', bg: 'bg-orange-500', colorVar: 'var(--status-temperature-warm)', labelKey: 'deviceStatus.tempStatus.high' };
  if (temp > 60) return { color: 'text-primary', bg: 'bg-primary', colorVar: 'var(--primary)', labelKey: 'deviceStatus.tempStatus.normal' };
  return { color: 'text-primary', bg: 'bg-primary', colorVar: 'var(--primary)', labelKey: 'deviceStatus.tempStatus.good' };
};

const getFanSpinDuration = (speed?: number, minSpeed = 0, maxSpeed = 100) => {
  if (!speed || speed <= 0) return 0;
  const speedSpan = Math.max(1, maxSpeed - minSpeed);
  const percent = Math.max(0, Math.min(100, ((speed - minSpeed) / speedSpan) * 100));
  if (percent >= 90) return 0.45;
  if (percent >= 70) return 0.7;
  if (percent >= 40) return 1;
  return 1.35;
};

const FAN_CURVE_PREVIEW_MIN_TEMP = 20;
const FAN_CURVE_PREVIEW_MAX_TEMP = 110;

function normalizePreviewCurvePoints(points: types.FanCurvePoint[]) {
  const unique = points
    .filter((point) => point.temperature >= FAN_CURVE_PREVIEW_MIN_TEMP && point.temperature <= FAN_CURVE_PREVIEW_MAX_TEMP)
    .sort((left, right) => left.temperature - right.temperature)
    .reduce<types.FanCurvePoint[]>((acc, point) => {
      const previous = acc[acc.length - 1];
      if (previous?.temperature === point.temperature) {
        acc[acc.length - 1] = point;
      } else {
        acc.push(point);
      }
      return acc;
    }, []);

  const first = unique[0];
  const last = unique[unique.length - 1];
  if (!first || !last) {
    return unique;
  }

  const preview = [...unique];
  if (first.temperature > FAN_CURVE_PREVIEW_MIN_TEMP) {
    preview.unshift({ temperature: FAN_CURVE_PREVIEW_MIN_TEMP, rpm: first.rpm });
  }
  if (last.temperature < FAN_CURVE_PREVIEW_MAX_TEMP) {
    preview.push({ temperature: FAN_CURVE_PREVIEW_MAX_TEMP, rpm: last.rpm });
  }
  return preview;
}

const MIN_VALID_POWER_WATTS = 0.05;

const formatPowerWatts = (watts?: number | null) => {
  const value = Number(watts || 0);
  if (!Number.isFinite(value) || value < MIN_VALID_POWER_WATTS) return '-- W';
  if (value < 10) {
    const rounded = Math.round(value * 10) / 10;
    return rounded > 0 ? `${rounded} W` : '-- W';
  }
  return `${Math.round(value)} W`;
};

const formatGpuPowerWatts = (watts: number | undefined | null, readState?: string) => {
  if (readState === 'notPolled') return '0 W';
  return formatPowerWatts(watts);
};

const splitPowerParts = (formatted: string) => {
  const spaceIndex = formatted.lastIndexOf(' ');
  if (spaceIndex <= 0) {
    return { value: formatted, unit: 'W' };
  }
  return {
    value: formatted.slice(0, spaceIndex),
    unit: formatted.slice(spaceIndex + 1),
  };
};

const CPU_TEMP_STROKE = 'var(--chart-cpu-temperature)';
const GPU_TEMP_STROKE = 'var(--chart-gpu-temperature)';
const FAN_SPEED_STROKE = 'var(--chart-fan-speed)';
const CPU_POWER_STROKE = 'var(--chart-cpu-power)';
const GPU_POWER_STROKE = 'var(--chart-gpu-power)';
const TOTAL_POWER_STROKE = 'var(--chart-primary)';
const POWER_TREND_WINDOW_MS = 60 * 1000;

const getAvailableTotalPowerWatts = (point: TemperatureHistoryPoint) => {
  const rawCpu = Number(point.cpuPowerWatts || 0);
  const rawGpu = Number(point.gpuPowerWatts || 0);
  const cpu = Number.isFinite(rawCpu) && rawCpu >= MIN_VALID_POWER_WATTS ? rawCpu : 0;
  const gpu = Number.isFinite(rawGpu) && rawGpu >= MIN_VALID_POWER_WATTS ? rawGpu : 0;
  const total = cpu + gpu;
  return total >= MIN_VALID_POWER_WATTS ? Math.round(total * 10) / 10 : 0;
};

type HistoryPathMap = Partial<Record<HistorySeriesKey, string>>;

const AnimatedTemperatureValue = memo(function AnimatedTemperatureValue({ temp, colorClass }: { temp: number | undefined; colorClass: string }) {
  return <span className={clsx('text-[28px] font-bold leading-none tabular-nums tracking-tight', colorClass)}>{temp ?? '--'}</span>;
});

const AnimatedSpeedValue = memo(function AnimatedSpeedValue({ speed }: { speed: number | undefined }) {
  return <span className="text-[28px] font-bold leading-none tabular-nums tracking-tight text-primary">{speed ?? '--'}</span>;
});

interface SemiGaugeProps {
  /** 当前归一化进度 0~1 */
  value: number;
  /** 进度弧颜色，例如 "var(--primary)"、"#f97316" */
  color: string;
  /** 居中区域 — 数值 + 单位 */
  children?: React.ReactNode;
}

export const SemiGauge = memo(function SemiGauge({ value, color, children }: SemiGaugeProps) {
  const r = 84;
  const cx = 100;
  const cy = 100;
  const arc = Math.PI * r;
  const safe = Math.max(0, Math.min(1, Number.isFinite(value) ? value : 0));
  const dashOffset = arc * (1 - safe);

  return (
    <div className="relative w-full max-w-[15.5rem]">
      <svg
        viewBox="0 0 200 116"
        className="semi-gauge-svg block w-full"
        preserveAspectRatio="xMidYMid meet"
        aria-hidden="true"
      >
        <g className="semi-gauge-arcs">
          {/* 背景轨道 */}
          <path
            d={`M ${cx - r} ${cy} A ${r} ${r} 0 0 1 ${cx + r} ${cy}`}
            fill="none"
            stroke="var(--muted)"
            strokeWidth="12"
            strokeLinecap="round"
          />
          {/* 进度弧 */}
          <path
            d={`M ${cx - r} ${cy} A ${r} ${r} 0 0 1 ${cx + r} ${cy}`}
            fill="none"
            stroke={color}
            strokeWidth="12"
            strokeLinecap="round"
            strokeDasharray={arc}
            strokeDashoffset={dashOffset}
            style={{ transition: 'stroke-dashoffset 600ms cubic-bezier(0.22, 1, 0.36, 1)' }}
          />
        </g>
        <g
          className="semi-gauge-tip"
          aria-hidden="true"
          style={{
            transformOrigin: `${cx}px ${cy}px`,
            transform: `rotate(${(safe * 180).toFixed(2)}deg)`,
            opacity: safe > 0.01 ? 1 : 0,
            transition: 'transform 600ms cubic-bezier(0.22, 1, 0.36, 1), opacity 200ms ease',
          }}
        >
          <circle
            cx={cx - r}
            cy={cy}
            r="4.5"
            fill="var(--card)"
            stroke={color}
            strokeWidth="3"
          />
        </g>
      </svg>
      {/* 居中区域 — 数值 + 单位 + 状态标签 全部塞进半圆几何中心略偏下 */}
      <div className="pointer-events-none absolute inset-x-0 top-[68%] -translate-y-1/2 flex flex-col items-center justify-center">
        {children}
      </div>
    </div>
  );
});

export function DeviceHeroArtwork() {
  return (
    <>
      <div className="theme-fancontrol-only theme-thrm-only glacier-hero-art pointer-events-none absolute inset-y-0 right-0 hidden overflow-hidden md:block" aria-hidden="true">
        <div className="absolute inset-0 bg-gradient-to-r from-card/80 via-card/25 to-transparent" />
        <div className="absolute inset-0 bg-gradient-to-b from-white/20 via-transparent to-card/30" />
      </div>
      <div className="theme-fancontrol-only theme-thrm-only glacier-hero-art-label pointer-events-none absolute top-3 hidden text-[10px] font-semibold uppercase tracking-[0.32em] text-primary/45 md:block" aria-hidden="true">
        AURORA AUX / GLACIER CORE
      </div>
    </>
  );
}

const SpinningFanIcon = memo(function SpinningFanIcon({ duration, className }: { duration: number; className: string }) {
  return (
    <span className={clsx('inline-flex', duration > 0 && 'animate-spin')} style={duration > 0 ? { animationDuration: `${duration}s` } : undefined}>
      <Fan className={className} />
    </span>
  );
});

const MetricHeader = memo(function MetricHeader({
  icon,
  label,
  align = 'center',
}: {
  icon: React.ReactNode;
  label: string;
  align?: 'center' | 'start';
}) {
  return (
    <div className={clsx('mb-2 flex items-center', align === 'start' ? 'justify-start' : 'justify-center')}>
      <div className={clsx('flex min-w-0 max-w-full items-center gap-2 text-[13px] font-medium text-muted-foreground', align === 'start' ? 'justify-start' : 'justify-center')}>
        <span className="metric-header-icon shrink-0 text-primary [&_svg]:stroke-[2.4]">{icon}</span>
        <span className="shrink-0">{label}</span>
      </div>
    </div>
  );
});

const CPU_METRIC_ICON = <Cpu className="h-4 w-4" />;
const GPU_METRIC_ICON = <Gpu className="h-4 w-4" />;
const POWER_TREND_WIDTH = 260;
const POWER_TREND_HEIGHT = 68;
const POWER_TREND_PAD = { left: 4, right: 4, top: 6, bottom: 6 } as const;
const POWER_TREND_PLOT_WIDTH = POWER_TREND_WIDTH - POWER_TREND_PAD.left - POWER_TREND_PAD.right;
const POWER_TREND_PLOT_HEIGHT = POWER_TREND_HEIGHT - POWER_TREND_PAD.top - POWER_TREND_PAD.bottom;

const PowerTrendChart = memo(function PowerTrendChart({
  points,
  series,
  label,
  value,
  available = true,
  unavailableLabel,
}: {
  points: TemperatureHistoryPoint[];
  series: 'cpu' | 'gpu';
  label: string;
  value: string;
  available?: boolean;
  unavailableLabel?: string;
}) {
  const { t } = useTranslation();
  const width = POWER_TREND_WIDTH;
  const height = POWER_TREND_HEIGHT;
  const pad = POWER_TREND_PAD;
  const plotWidth = POWER_TREND_PLOT_WIDTH;
  const plotHeight = POWER_TREND_PLOT_HEIGHT;
  const chart = useMemo(() => {
    if (!available) {
      return { path: '', area: '', current: null, min: 0, max: 0, width, height, pad, plotWidth, plotHeight };
    }

    const recent = clipHistoryToRecentWindow(points, POWER_TREND_WINDOW_MS);
    const values = recent
      .map((point) => Number(series === 'cpu' ? point.cpuPowerWatts || 0 : point.gpuPowerWatts || 0))
      .filter((power) => Number.isFinite(power) && power >= MIN_VALID_POWER_WATTS);
    if (values.length < 2) {
      return { path: '', area: '', current: null, min: 0, max: 0, width, height, pad, plotWidth, plotHeight };
    }

    const min = Math.max(0, Math.floor((Math.min(...values) - 2) / 5) * 5);
    const max = Math.max(min + 10, Math.ceil((Math.max(...values) + 2) / 5) * 5);
    const range = Math.max(1, max - min);
    const firstTimestamp = recent[0]?.timestamp ?? 0;
    const lastTimestamp = recent[recent.length - 1]?.timestamp ?? firstTimestamp;
    const timestampRange = Math.max(1, lastTimestamp - firstTimestamp);
    const xFor = (timestamp: number) => pad.left + ((timestamp - firstTimestamp) / timestampRange) * plotWidth;
    const yFor = (power: number) => pad.top + plotHeight - ((Math.max(min, Math.min(max, power)) - min) / range) * plotHeight;
    const coordinates = recent.map((point) => {
      const power = Number(series === 'cpu' ? point.cpuPowerWatts || 0 : point.gpuPowerWatts || 0);
      return power >= MIN_VALID_POWER_WATTS ? { x: xFor(point.timestamp), y: yFor(power) } : null;
    });
    const segments: string[] = [];
    let segment = '';
    coordinates.forEach((coordinate) => {
      if (!coordinate) {
        if (segment) segments.push(segment.trim());
        segment = '';
        return;
      }
      segment += `${segment ? 'L' : 'M'} ${coordinate.x.toFixed(1)} ${coordinate.y.toFixed(1)} `;
    });
    if (segment) segments.push(segment.trim());
    const path = segments.join(' ');
    const last = coordinates[coordinates.length - 1];
    const first = coordinates.find(Boolean);
    const area = first && last
      ? `${first.x.toFixed(1)},${(pad.top + plotHeight).toFixed(1)} ${path.replace(/[ML] /g, '').trim()} ${last.x.toFixed(1)},${(pad.top + plotHeight).toFixed(1)}`
      : '';
    return { path, area, current: last, min, max, width, height, pad, plotWidth, plotHeight };
  }, [available, height, pad, plotHeight, plotWidth, points, series, width]);

  const strokeColor = series === 'cpu' ? CPU_POWER_STROKE : GPU_POWER_STROKE;

  return (
    <div className="min-w-0">
      <div className="mb-1 flex items-center justify-between gap-2">
        <span className="truncate text-[10px] font-medium text-muted-foreground">{label}</span>
        <span className="shrink-0 text-sm font-semibold leading-none tabular-nums text-foreground">{value}</span>
      </div>
      <div className="h-[68px] overflow-hidden rounded-lg border border-border/60 bg-background/35 px-1.5 py-1">
        {chart.path ? (
          <div className="relative h-full w-full">
            <svg viewBox={`0 0 ${chart.width} ${chart.height}`} className="h-full w-full" preserveAspectRatio="none" role="img" aria-label={`${label}: ${value}`}>
              <defs>
                <linearGradient id={`power-trend-grad-${series}`} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={strokeColor} stopOpacity="0.24" />
                  <stop offset="100%" stopColor={strokeColor} stopOpacity="0.01" />
                </linearGradient>
              </defs>
              <line vectorEffect="non-scaling-stroke" x1={chart.pad.left} y1={chart.pad.top + chart.plotHeight / 2} x2={chart.pad.left + chart.plotWidth} y2={chart.pad.top + chart.plotHeight / 2} stroke="var(--chart-grid)" strokeWidth="1" opacity="0.7" />
              <polygon points={chart.area} fill={`url(#power-trend-grad-${series})`} />
              <path vectorEffect="non-scaling-stroke" d={chart.path} fill="none" stroke={strokeColor} strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
            {chart.current && (
              <span
                aria-hidden="true"
                className="pointer-events-none absolute h-2 w-2 -translate-x-1/2 -translate-y-1/2 rounded-full ring-2 ring-card"
                style={{
                  left: `${(chart.current.x / chart.width) * 100}%`,
                  top: `${(chart.current.y / chart.height) * 100}%`,
                  backgroundColor: strokeColor,
                }}
              />
            )}
          </div>
        ) : (
          <div className="flex h-full items-center justify-center text-[10px] text-muted-foreground">
            {available ? t('deviceStatus.history.waiting') : (unavailableLabel || t('deviceStatus.tempGauge.notRead'))}
          </div>
        )}
      </div>
    </div>
  );
});

const MonitorInfoCard = memo(function MonitorInfoCard({
  icon,
  label,
  themeCard,
  model,
  temperature,
  ready,
  powerLabel,
  power,
  powerPoints,
  powerSeries,
  powerAvailable,
  idleLabel,
  unavailableLabel,
  idleNote,
  idleTooltip,
}: {
  icon: React.ReactNode;
  label: string;
  themeCard?: string;
  model?: string;
  temperature?: number;
  ready: boolean;
  powerLabel: string;
  power: string;
  powerPoints: TemperatureHistoryPoint[];
  powerSeries: 'cpu' | 'gpu';
  powerAvailable?: boolean;
  idleLabel?: string;
  unavailableLabel?: string;
  idleNote?: string;
  idleTooltip?: string;
}) {
  return (
    <div data-theme-card={themeCard} className="glacier-metric-card flex h-full min-h-[230px] flex-col rounded-xl border border-border bg-card px-4 py-3 shadow-sm shadow-black/5">
      <div className="mb-2.5 flex min-h-8 items-start justify-between gap-3">
        <div className="pt-1">
          <MetricHeader icon={icon} label={label} align="start" />
        </div>
        <div className="flex h-8 min-w-0 max-w-[58%] items-center rounded-lg border border-border/70 bg-background/55 px-2.5 text-[11px] text-muted-foreground">
          <span className="truncate">{model || '--'}</span>
        </div>
      </div>
      <div className="grid min-h-0 flex-1 grid-cols-[minmax(9rem,0.9fr)_minmax(0,1.1fr)] items-start gap-3 pt-1">
        <div className="flex min-h-[145px] min-w-0 items-start justify-center pt-1">
          <TempGaugeDisplay temp={temperature} ready={ready} idleLabel={idleLabel} />
        </div>
        <PowerTrendChart
          points={powerPoints}
          series={powerSeries}
          label={powerLabel}
          value={power}
          available={powerAvailable}
          unavailableLabel={unavailableLabel || idleLabel}
        />
      </div>
      {idleNote && idleTooltip && (
        <div className="mt-2">
          <GpuIdleNotice note={idleNote} tooltip={idleTooltip} />
        </div>
      )}
    </div>
  );
});

const HardwareModelLabel = memo(function HardwareModelLabel({
  model,
  fallbackLabel,
}: {
  model: string | undefined;
  fallbackLabel: string;
}) {
  const trimmedModel = model?.trim() || '';
  const text = trimmedModel || fallbackLabel;
  const label = (
    <span className="min-w-0 truncate text-[11px] font-medium text-muted-foreground">
      {text}
    </span>
  );

  if (!trimmedModel) {
    return label;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>{label}</TooltipTrigger>
      <TooltipContent>{trimmedModel}</TooltipContent>
    </Tooltip>
  );
});

/* ── Memo sub-components to avoid parent re-renders ── */

// 温度状态 → 仪表盘弧色（CSS 变量 / 字面色值，避免依赖 Tailwind class）
const getTempArcColor = (temp: number) => {
  if (temp > 85) return 'var(--status-temperature-hot)';
  if (temp > 75) return 'var(--status-temperature-warm)';
  return 'var(--primary)';
};

const GpuIdleNotice = memo(function GpuIdleNotice({
  note,
  tooltip,
}: {
  note: string;
  tooltip: string;
}) {
  const { t } = useTranslation();
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          data-theme-ui="gpu-idle-note"
          aria-label={t('deviceStatus.tempGauge.notReadHintAria')}
          className="flex w-full cursor-help items-center justify-center gap-1.5 rounded-lg border border-border/60 bg-background/45 px-2.5 py-1 text-[11px] leading-tight text-muted-foreground transition-colors hover:border-border hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
        >
          <CircleHelp className="h-3.5 w-3.5 shrink-0 text-muted-foreground/80" aria-hidden="true" />
          <span className="truncate">{note}</span>
        </button>
      </TooltipTrigger>
      <TooltipContent side="top" className="max-w-[260px] px-3 py-2 text-left leading-relaxed whitespace-pre-line">
        {tooltip}
      </TooltipContent>
    </Tooltip>
  );
});

const TempGaugeDisplay = memo(function TempGaugeDisplay({
  temp,
  ready,
  idleLabel,
}: {
  temp: number | undefined;
  /** 后端首次推送有效温度后置为 true；之前显示占位避免误读 0 °C */
  ready: boolean;
  idleLabel?: string;
}) {
  const { t } = useTranslation();
  const placeholderLabel = idleLabel || t('deviceStatus.tempGauge.loading');

  // 未就绪 → 占位：灰色弧、"--"、"读取中…"，不进入正常状态色
  if (!ready) {
    return (
      <div className="flex w-full max-w-[20rem] flex-col items-center justify-center">
        <SemiGauge value={0} color="var(--muted-foreground)">
          <div className="flex items-baseline gap-0.5 leading-none">
            <span className="text-[28px] font-bold leading-none tabular-nums tracking-tight text-muted-foreground/70">--</span>
            <span className="text-xs font-medium leading-none text-muted-foreground/70">°C</span>
          </div>
          <span className="mt-1.5 inline-flex items-center gap-1.5 text-[11px] leading-none text-muted-foreground">
            {!idleLabel && <span className="inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-muted-foreground/60" />}
            <span>{placeholderLabel}</span>
          </span>
        </SemiGauge>
      </div>
    );
  }

  const status = getTempStatus(temp || 0);
  const ratio = Math.min(1, (temp || 0) / 100);
  const arcColor = getTempArcColor(temp || 0);
  return (
    <div className="flex w-full max-w-[20rem] flex-col items-center justify-center">
      <SemiGauge value={ratio} color={arcColor}>
        <div className="flex items-baseline gap-0.5 leading-none">
          <AnimatedTemperatureValue temp={temp} colorClass={status.color} />
          <span className="text-xs font-medium leading-none text-muted-foreground">°C</span>
        </div>
        <span className="mt-1.5 text-[11px] leading-none text-muted-foreground">{t(status.labelKey)}</span>
      </SemiGauge>
    </div>
  );
});

const FanSpeedDisplay = memo(function FanSpeedDisplay({
  currentSpeed,
  targetSpeed,
  unit,
  minSpeed,
  maxSpeed,
}: {
  currentSpeed: number | undefined;
  targetSpeed: number | undefined;
  unit: string;
  minSpeed: number;
  maxSpeed: number;
}) {
  const { t } = useTranslation();
  const speedSpan = Math.max(1, maxSpeed - minSpeed);
  const ratio = Math.min(1, Math.max(0, ((currentSpeed ?? minSpeed) - minSpeed) / speedSpan));
  const unitLabel = unit || '%';
  const subLabel = t('deviceStatus.fan.targetSummary', { target: targetSpeed ?? '--', unit: unitLabel });

  return (
    <div className="flex w-full max-w-[20rem] flex-col items-center justify-center">
      <SemiGauge value={ratio} color="var(--primary)">
        <div className="flex min-w-[5.25rem] items-baseline justify-center gap-1 leading-none">
          <AnimatedSpeedValue speed={currentSpeed} />
          <span className="text-xs font-medium leading-none text-muted-foreground">{unitLabel}</span>
        </div>
        <span className="mt-1.5 max-w-[11rem] truncate text-[11px] leading-none text-muted-foreground">
          {subLabel}
        </span>
      </SemiGauge>
    </div>
  );
});

const MiniFanCurveChart = memo(function MiniFanCurveChart({
  curve,
  currentTemp,
  minSpeed,
  maxSpeed,
  unitLabel,
  profileName,
  onOpen,
}: {
  curve: types.FanCurvePoint[] | undefined;
  currentTemp: number | undefined;
  minSpeed: number;
  maxSpeed: number;
  unitLabel: string;
  profileName?: string;
  onOpen?: () => void;
}) {
  const { t } = useTranslation();

  const geometry = useMemo(() => {
    const points = Array.isArray(curve)
      ? curve.filter((point) => typeof point.temperature === 'number' && typeof point.rpm === 'number')
      : [];
    const maxPointSpeed = points.reduce((max, point) => Math.max(max, point.rpm), 0);
    const effectivePoints = maxSpeed > 100 && maxPointSpeed <= 100 ? [] : points;
    const fallbackSpeed = (ratio: number) => Math.round(minSpeed + (maxSpeed - minSpeed) * ratio);
    const rawSource = effectivePoints.length > 0 ? effectivePoints : [
      { temperature: 20, rpm: fallbackSpeed(0.2) },
      { temperature: 40, rpm: fallbackSpeed(0.35) },
      { temperature: 60, rpm: fallbackSpeed(0.55) },
      { temperature: 80, rpm: fallbackSpeed(0.75) },
      { temperature: 110, rpm: maxSpeed },
    ];
    const source = normalizePreviewCurvePoints(rawSource);
    // 单遍扫描计算 min/max，避免旧实现 4 次 Math.min/Math.max(...source.map(...)) 重建临时数组。
    let minTemp = 20;
    let maxTemp = 110;
    for (const p of source) {
      if (p.temperature < minTemp) minTemp = p.temperature;
      if (p.temperature > maxTemp) maxTemp = p.temperature;
    }
    const width = 520;
    const height = 196;
    const pad = { left: 44, right: 16, top: 14, bottom: 22 };
    const plotWidth = width - pad.left - pad.right;
    const plotHeight = height - pad.top - pad.bottom;
    const tempRange = Math.max(1, maxTemp - minTemp);
    const speedRange = Math.max(1, maxSpeed - minSpeed);
    const xForTemp = (temp: number) => pad.left + ((temp - minTemp) / tempRange) * plotWidth;
    const yForRpm = (rpm: number) => pad.top + plotHeight - ((Math.max(minSpeed, Math.min(maxSpeed, rpm)) - minSpeed) / speedRange) * plotHeight;
    const linePoints = source
      .map((point) => `${xForTemp(point.temperature).toFixed(1)},${yForRpm(point.rpm).toFixed(1)}`)
      .join(' ');
    const firstPoint = source[0] ?? { temperature: minTemp, rpm: minSpeed };
    const lastPoint = source[source.length - 1] ?? firstPoint;
    const areaStartX = xForTemp(firstPoint.temperature).toFixed(1);
    const areaEndX = xForTemp(lastPoint.temperature).toFixed(1);
    const baselineY = (pad.top + plotHeight).toFixed(1);
    const areaPoints = `${areaStartX},${baselineY} ${linePoints} ${areaEndX},${baselineY}`;
    const yTicks: number[] = getFanSpeedTicks(minSpeed, maxSpeed);
    return { width, height, pad, plotWidth, plotHeight, minTemp, maxTemp, source, xForTemp, yForRpm, linePoints, areaPoints, yTicks };
  }, [curve, maxSpeed, minSpeed]);

  const { width, height, pad, plotWidth, plotHeight, minTemp, maxTemp, source, xForTemp, yForRpm, linePoints, areaPoints, yTicks } = geometry;

  const operatingPoint = useMemo(() => {
    if (typeof currentTemp !== 'number' || currentTemp <= 0 || source.length === 0) {
      return null;
    }
    const clampedTemp = Math.max(minTemp, Math.min(maxTemp, currentTemp));
    let speedAtTemp = source[0].rpm;
    if (clampedTemp <= source[0].temperature) {
      speedAtTemp = source[0].rpm;
    } else if (clampedTemp >= source[source.length - 1].temperature) {
      speedAtTemp = source[source.length - 1].rpm;
    } else {
      for (let i = 0; i < source.length - 1; i++) {
        const a = source[i];
        const b = source[i + 1];
        if (clampedTemp >= a.temperature && clampedTemp <= b.temperature) {
          const span = Math.max(1, b.temperature - a.temperature);
          speedAtTemp = a.rpm + ((clampedTemp - a.temperature) / span) * (b.rpm - a.rpm);
          break;
        }
      }
    }
    return {
      x: Math.max(pad.left, Math.min(pad.left + plotWidth, xForTemp(clampedTemp))),
      y: yForRpm(speedAtTemp),
    };
  }, [currentTemp, maxTemp, minTemp, pad.left, plotWidth, source, xForTemp, yForRpm]);

  return (
    <button
      type="button"
      data-theme-card="fan-curve-preview"
      onClick={onOpen}
      className={clsx(
        'glacier-chart-card group flex h-full w-full flex-col rounded-xl border border-border bg-card px-4 py-3.5 text-left shadow-sm shadow-black/5',
        onOpen && 'cursor-pointer transition-colors hover:border-primary/35 hover:bg-primary/5 hover:shadow-md',
      )}
    >
      <div className="mb-2.5 flex h-5 items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <div className="truncate text-xs font-semibold text-foreground">{t('deviceStatus.chart.fanCurve')}</div>
          {profileName && (
            <span className="max-w-[10rem] truncate rounded-full border border-border/70 bg-background/70 px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
              {profileName}
            </span>
          )}
          <span className="shrink-0 text-[11px] text-muted-foreground/75">{unitLabel}</span>
        </div>
        {onOpen && (
          <span className="inline-flex shrink-0 items-center gap-1 text-[11px] font-medium text-muted-foreground/70 transition-colors duration-150 group-hover:text-primary group-focus-visible:text-primary">
            {t('deviceStatus.chart.openCurve')}
            <ArrowUpRight className="h-3 w-3" />
          </span>
        )}
      </div>
      <div className="glacier-chart-canvas flex min-h-[164px] flex-1 items-center overflow-hidden">
        <svg viewBox={`0 0 ${width} ${height}`} className="h-full w-full" preserveAspectRatio="xMidYMid meet" aria-hidden="true">
          <defs>
            <linearGradient id="mini-fan-curve-area" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="var(--chart-primary)" stopOpacity="0.22" />
              <stop offset="100%" stopColor="var(--chart-primary)" stopOpacity="0.01" />
            </linearGradient>
          </defs>
          {yTicks.map((tick) => {
            const y = yForRpm(tick);
            return (
              <g key={tick}>
                <line x1={pad.left} y1={y} x2={pad.left + plotWidth} y2={y} stroke="var(--chart-grid)" strokeWidth="1" />
                <text x={pad.left - 8} y={y + 4} textAnchor="end" fontSize="10" fill="var(--chart-tick)">{formatFanSpeedValue(tick)}</text>
              </g>
            );
          })}
          <polygon points={areaPoints} fill="url(#mini-fan-curve-area)" />
          <polyline points={linePoints} fill="none" stroke="var(--chart-primary)" strokeWidth="2.75" strokeLinecap="round" strokeLinejoin="round" />
          {operatingPoint !== null && (
            <>
              <line x1={operatingPoint.x} y1={pad.top} x2={operatingPoint.x} y2={pad.top + plotHeight} stroke="var(--chart-temperature-indicator)" strokeWidth="1.5" strokeDasharray="4 4" opacity="0.85" />
              <circle cx={operatingPoint.x} cy={operatingPoint.y} r="6.5" fill="var(--chart-temperature-indicator)" opacity="0.2" />
              <circle cx={operatingPoint.x} cy={operatingPoint.y} r="3.2" fill="var(--chart-temperature-indicator)" stroke="var(--card)" strokeWidth="1.5" />
            </>
          )}
          <text x={pad.left} y={height - 7} fontSize="10" fill="var(--chart-tick)">{minTemp}</text>
          <text x={pad.left + plotWidth} y={height - 7} textAnchor="end" fontSize="10" fill="var(--chart-tick)">{maxTemp} °C</text>
        </svg>
      </div>
    </button>
  );
});

const TemperatureHistoryPanel = memo(function TemperatureHistoryPanel({
	  points: retainedPoints,
  enabled,
  source,
  retentionHours,
  minSpeed,
  maxSpeed,
	  visibleSeries,
	  orderedSeries,
	  title,
	  onOpen,
}: {
  points: TemperatureHistoryPoint[];
  enabled: boolean;
  source: 'core' | 'session';
  retentionHours: number;
  minSpeed: number;
  maxSpeed: number;
	  visibleSeries: Record<HistorySeriesKey, boolean>;
	  orderedSeries: HistorySeriesKey[];
	  title?: string;
	  onOpen?: () => void;
}) {
	const { t } = useTranslation();
	const sourceLabel = source === 'core'
		? `${t('deviceStatus.history.source.core')} · ${t('fanCurve.history.retention.hours', { count: retentionHours })}`
		: t('deviceStatus.history.source.session');
	const points = useMemo(
		() => downsampleHistoryPoints(retainedPoints, 600),
		[retainedPoints],
	);
  const chart = useMemo(() => {
    const width = 520;
    const height = 168;
    const pad = { left: 8, right: 8, top: 10, bottom: 10 };
    const plotWidth = width - pad.left - pad.right;
    const plotTop = pad.top;
    const plotHeight = height - pad.top - pad.bottom;
    let minTemp = 35;
    let maxTemp = 80;
    let maxPower = 0;
    const speedRange = Math.max(1, maxSpeed - minSpeed);

    for (const point of points) {
      if (point.cpuTemp > 0) {
        minTemp = Math.min(minTemp, point.cpuTemp);
        maxTemp = Math.max(maxTemp, point.cpuTemp);
      }
      if (point.gpuTemp > 0) {
        minTemp = Math.min(minTemp, point.gpuTemp);
        maxTemp = Math.max(maxTemp, point.gpuTemp);
      }
      const cpuPower = Number(point.cpuPowerWatts || 0);
      if (visibleSeries.cpuPower && Number.isFinite(cpuPower) && cpuPower >= MIN_VALID_POWER_WATTS) {
        maxPower = Math.max(maxPower, cpuPower);
      }
      const gpuPower = Number(point.gpuPowerWatts || 0);
      if (visibleSeries.gpuPower && Number.isFinite(gpuPower) && gpuPower >= MIN_VALID_POWER_WATTS) {
        maxPower = Math.max(maxPower, gpuPower);
      }
      if (visibleSeries.totalPower) {
        maxPower = Math.max(maxPower, getAvailableTotalPowerWatts(point));
      }
    }

    const hasPower = maxPower > 0;
    const minY = Math.max(0, Math.floor((minTemp - 6) / 5) * 5);
    const maxY = Math.min(110, Math.ceil((maxTemp + 6) / 5) * 5);
    const rangeY = Math.max(10, maxY - minY);
    const powerMax = maxPower > 0 ? Math.max(20, Math.ceil((maxPower + 8) / 10) * 10) : 120;
    const minTs = points[0]?.timestamp ?? 0;
    const maxTs = points[points.length - 1]?.timestamp ?? minTs;
    const rangeTs = Math.max(1, maxTs - minTs);
    const xFor = (timestamp: number, index: number) => {
      if (points.length <= 1) return pad.left + plotWidth / 2;
      if (rangeTs <= 1 && points.length > 1) return pad.left + (index / Math.max(1, points.length - 1)) * plotWidth;
      return pad.left + ((timestamp - minTs) / rangeTs) * plotWidth;
    };
    const yForTemp = (temp: number) => plotTop + plotHeight - ((temp - minY) / rangeY) * plotHeight;
    const yForFan = (rpm: number) => plotTop + plotHeight - ((Math.max(minSpeed, Math.min(maxSpeed, rpm)) - minSpeed) / speedRange) * plotHeight;
    const yForPower = (watts: number) => plotTop + plotHeight - (Math.max(0, Math.min(powerMax, watts)) / powerMax) * plotHeight;
    const buildPath = (selector: (point: TemperatureHistoryPoint) => number, projectY: (value: number) => number) => {
      let path = '';
      let started = false;
      points.forEach((point, index) => {
        const value = selector(point);
        if (value <= 0) {
          started = false;
          return;
        }
        path += `${started ? 'L' : 'M'} ${xFor(point.timestamp, index).toFixed(1)} ${projectY(value).toFixed(1)} `;
        started = true;
      });
      return path.trim();
    };

    const paths: HistoryPathMap = {
      cpu: buildPath((point) => point.cpuTemp, yForTemp),
      gpu: buildPath((point) => point.gpuTemp, yForTemp),
      fan: buildPath((point) => Number(point.fanRpm || 0), yForFan),
      cpuPower: buildPath((point) => {
        const power = Number(point.cpuPowerWatts || 0);
        return Number.isFinite(power) && power >= MIN_VALID_POWER_WATTS ? power : 0;
      }, yForPower),
      gpuPower: buildPath((point) => {
        const power = Number(point.gpuPowerWatts || 0);
        return Number.isFinite(power) && power >= MIN_VALID_POWER_WATTS ? power : 0;
      }, yForPower),
      totalPower: buildPath(getAvailableTotalPowerWatts, yForPower),
    };

    return {
      width,
      height,
      pad,
      plotWidth,
      plotTop,
      plotHeight,
      hasPower,
      paths,
      gridLines: [0.2, 0.5, 0.8],
    };
  }, [maxSpeed, minSpeed, points, visibleSeries.cpu, visibleSeries.cpuPower, visibleSeries.gpu, visibleSeries.gpuPower, visibleSeries.totalPower]);
  const { width, height, pad, plotWidth, plotTop, plotHeight, hasPower, paths, gridLines } = chart;
  const legendItems = useMemo(
    () =>
      orderedSeries.flatMap((series) => {
        if (!visibleSeries[series]) return [];
        if ((series === 'cpuPower' || series === 'gpuPower' || series === 'totalPower') && !hasPower) return [];
        if (!paths[series]) return [];
        const color = series === 'cpu'
          ? CPU_TEMP_STROKE
          : series === 'gpu'
            ? GPU_TEMP_STROKE
            : series === 'fan'
              ? FAN_SPEED_STROKE
              : series === 'cpuPower'
                ? CPU_POWER_STROKE
                : series === 'gpuPower'
                  ? GPU_POWER_STROKE
                  : TOTAL_POWER_STROKE;
        const label = series === 'cpu'
          ? t('fanCurve.history.series.cpu')
          : series === 'gpu'
            ? t('fanCurve.history.series.gpu')
            : series === 'fan'
              ? t('fanCurve.history.series.fan')
              : series === 'cpuPower'
                ? t('fanCurve.history.series.cpuPower')
                : series === 'gpuPower'
                  ? t('fanCurve.history.series.gpuPower')
                  : t('fanCurve.history.series.totalPower');
        return [{ key: series, color, label }];
      }),
    [hasPower, orderedSeries, paths, t, visibleSeries],
  );
  const handlePanelKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (!onOpen) return;
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      onOpen();
    }
  };

  return (
    <div
      data-theme-card="temperature-history"
      role={onOpen ? 'button' : undefined}
      tabIndex={onOpen ? 0 : undefined}
      onClick={onOpen}
      onKeyDown={handlePanelKeyDown}
      className={clsx(
        'glacier-chart-card group flex h-full min-h-[200px] w-full flex-col rounded-xl border border-border bg-card px-4 py-3.5 shadow-sm shadow-black/5',
        onOpen && 'cursor-pointer transition-colors hover:border-primary/35 hover:bg-primary/5 hover:shadow-md focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/30',
      )}
    >
      <div className="mb-2.5 flex h-5 items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
	          <div className="truncate text-xs font-semibold text-foreground">{title || t('deviceStatus.history.title')}</div>
          <span className="shrink-0 rounded-full border border-border/70 bg-background/70 px-2 py-0.5 text-[10px] text-muted-foreground">{sourceLabel}</span>
        </div>
        {onOpen && (
          <span className="inline-flex shrink-0 items-center gap-1 text-[11px] font-medium text-muted-foreground/70 transition-colors duration-150 group-hover:text-primary group-focus-visible:text-primary">
            {t('deviceStatus.history.details')}
            <ArrowUpRight className="h-3 w-3" />
          </span>
        )}
      </div>

      <div className="glacier-chart-canvas flex min-h-[124px] flex-1 overflow-hidden rounded-lg bg-muted/25 p-2.5">
        {points.length === 0 ? (
          <div className="flex h-full w-full items-center justify-center text-center text-[11px] leading-relaxed text-muted-foreground">
            {enabled ? t('deviceStatus.history.waiting') : t('deviceStatus.history.disabled')}
          </div>
        ) : points.length < 2 ? (
          <div className="flex h-full w-full items-center justify-center text-center text-[11px] leading-relaxed text-muted-foreground">
            {source === 'core' ? t('deviceStatus.history.singleSampleCore') : t('deviceStatus.history.singleSampleSession')}
          </div>
        ) : (
          <div className="h-full w-full overflow-hidden">
            <svg viewBox={`0 0 ${width} ${height}`} className="h-full w-full" preserveAspectRatio="none" aria-hidden="true">
              {gridLines.map((ratio) => {
                const y = plotTop + plotHeight * ratio;
                return (
                  <g key={ratio}>
                    <line vectorEffect="non-scaling-stroke" x1={pad.left} y1={y} x2={pad.left + plotWidth} y2={y} stroke="var(--chart-grid)" strokeWidth="1" opacity="0.7" />
                  </g>
                );
              })}
              {orderedSeries.map((series) => {
                if (!visibleSeries[series]) {
                  return null;
                }
                if ((series === 'cpuPower' || series === 'gpuPower' || series === 'totalPower') && !hasPower) {
                  return null;
                }

                const path = paths[series];
                if (!path) {
                  return null;
                }

                const stroke = series === 'cpu'
                  ? CPU_TEMP_STROKE
                  : series === 'gpu'
                    ? GPU_TEMP_STROKE
                    : series === 'fan'
                      ? FAN_SPEED_STROKE
                      : series === 'cpuPower'
                        ? CPU_POWER_STROKE
                        : series === 'gpuPower'
                          ? GPU_POWER_STROKE
                          : TOTAL_POWER_STROKE;
                const isPowerSeries = series === 'cpuPower' || series === 'gpuPower' || series === 'totalPower';
                const strokeWidth = series === 'fan' ? '1.8' : isPowerSeries ? '2' : '2.4';
                const opacity = series === 'fan' ? '0.45' : isPowerSeries ? '0.9' : undefined;
                return (
                  <path
                    key={series}
                    vectorEffect="non-scaling-stroke"
                    d={path}
                    fill="none"
                    stroke={stroke}
                    strokeWidth={strokeWidth}
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    opacity={opacity}
                  />
                );
              })}
            </svg>
          </div>
        )}
      </div>
      {points.length >= 2 && legendItems.length > 0 && (
        <div className="mt-2 flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-[11px] leading-none text-muted-foreground/85">
          {legendItems.map((item) => (
            <span key={item.key} className="inline-flex items-center gap-1.5 whitespace-nowrap">
              <span className="h-[3px] w-2.5 shrink-0 rounded-full" style={{ backgroundColor: item.color }} aria-hidden="true" />
              <span>{item.label}</span>
            </span>
          ))}
        </div>
      )}
    </div>
  );
});

function resolveGpuReadMode(config: types.AppConfig): 'auto' | 'always' | 'never' {
  const configured = ((config as any).gpuReadMode as string) || (((config as any).gpuLowPowerProtection === false) ? 'always' : 'auto');
  return configured === 'always' || configured === 'never' ? configured : 'auto';
}

function resolveReferenceTemp(
  temperature: types.TemperatureData | null,
  config: types.AppConfig,
): number {
  const currentGpuReadMode = resolveGpuReadMode(config);
  const gpuReadState = (((temperature as any)?.gpuReadState as string) || 'unknown');
  const gpuNotPolled = currentGpuReadMode === 'never' || gpuReadState === 'notPolled';
  const source = (((config as any).tempSource as string) || 'max') as 'max' | 'cpu' | 'gpu';
  const cpu = temperature?.cpuTemp ?? 0;
  const gpu = gpuNotPolled ? 0 : (temperature?.gpuTemp ?? 0);
  const max = temperature?.maxTemp ?? 0;
  if (source === 'cpu') return cpu > 0 ? cpu : max;
  if (source === 'gpu') return gpu > 0 ? gpu : max;
  return max;
}

function useResolvedTemperature(config: types.AppConfig, temperatureProp?: types.TemperatureData | null) {
  const storeTemperature = useAppStore(selectTemperature);
  const raw = temperatureProp ?? storeTemperature;
  return useMemo(() => applyPowerSpoofToTemperature(raw, config), [config, raw]);
}

function useLiveCpuGpuMetrics(config: types.AppConfig, temperatureProp?: types.TemperatureData | null) {
  const { t } = useTranslation();
  const temperature = useResolvedTemperature(config, temperatureProp);
  const tempPushed = (temperature?.updateTime ?? 0) > 0;
  const currentGpuReadMode = resolveGpuReadMode(config);
  const gpuReadState = (((temperature as any)?.gpuReadState as string) || 'unknown');
  const gpuAutoIdle = currentGpuReadMode === 'auto' && gpuReadState === 'notPolled';
  const gpuNotPolled = currentGpuReadMode === 'never' || gpuReadState === 'notPolled';
  const gpuIdleLabel = gpuAutoIdle
    ? t('deviceStatus.tempGauge.gpuInactive')
    : gpuNotPolled
      ? t('deviceStatus.tempGauge.notRead')
      : undefined;
  const gpuAutoIdleNote = gpuAutoIdle ? t('deviceStatus.tempGauge.gpuInactiveNote') : undefined;
  const gpuNotPolledTooltip = gpuAutoIdle
    ? t('deviceStatus.tempGauge.notReadHintAuto')
    : undefined;
  const cpuReady = tempPushed && (temperature?.cpuTemp ?? 0) > 0;
  const gpuReady = !gpuNotPolled && tempPushed && (temperature?.gpuTemp ?? 0) > 0;
  const referenceTemp = resolveReferenceTemp(temperature, config);
  const hasTemperatureReading = tempPushed && referenceTemp > 0;
  const cpuFallbackTemp = hasTemperatureReading && !cpuReady && !gpuReady ? referenceTemp : undefined;
  const cpuDisplayTemp = cpuReady ? temperature?.cpuTemp : cpuFallbackTemp;
  const cpuDisplayReady = cpuReady || !!cpuFallbackTemp;
  const cpuMetricLabel = cpuReady || !cpuFallbackTemp
    ? t('deviceStatus.metrics.cpuTemperature')
    : t('deviceStatus.metrics.controlTemperature');

  return {
    temperature,
    gpuReadState,
    gpuNotPolled,
    gpuIdleLabel,
    gpuAutoIdleNote,
    gpuNotPolledTooltip,
    gpuReady,
    cpuDisplayTemp,
    cpuDisplayReady,
    cpuMetricLabel,
  };
}

const LiveHeroModeSummary = memo(function LiveHeroModeSummary({
  config,
  fanDataProp,
  runtimeDeviceProfile,
  fanSpeedRange,
  fanSpeedLabel,
}: {
  config: types.AppConfig;
  fanDataProp?: types.FanData | null;
  runtimeDeviceProfile?: types.DeviceProfile | null;
  fanSpeedRange: { min: number; max: number; step: number; tickScale: number };
  fanSpeedLabel: string;
}) {
  const { t } = useTranslation();
  // autoControl 与非定速模式不展示当前转速，无需订阅 fanData；仅固定转速模式读取回退值。
  const needCurrentSpeedFallback = !config.autoControl && !!config.customSpeedEnabled;
  const storeFanData = useAppStore((state) => (needCurrentSpeedFallback ? state.fanData : null));
  const activeFanData = fanDataProp ?? storeFanData;
  const fanSpeedUnit = getFanSpeedUnit(activeFanData as any, config as any, runtimeDeviceProfile as any);
  const currentFanSpeed = needCurrentSpeedFallback
    ? clampFanSpeedToRange(
        readCurrentFanSpeed(activeFanData, fanSpeedUnit, config as any, runtimeDeviceProfile as any),
        fanSpeedRange,
      )
    : undefined;
  const fixedModeSpeed = clampFanSpeedToRange(config.customSpeedRPM, fanSpeedRange, currentFanSpeed);
  const modeDesc = config.autoControl
    ? t('deviceStatus.mode.smartDescription')
    : config.customSpeedEnabled
      ? t('deviceStatus.mode.fixedDescription', { speed: fixedModeSpeed ?? '--', unit: fanSpeedLabel })
      : t('deviceStatus.mode.manualDescription');

  return (
    <div className="mt-1 space-y-1.5 text-xs text-muted-foreground">
      <div className="flex items-center gap-1.5">
        {config.autoControl ? (
          <Zap className="h-3 w-3 text-primary" />
        ) : (
          <Settings className="h-3 w-3" />
        )}
        <span>{modeDesc}</span>
      </div>
    </div>
  );
});

const LiveBridgeWarningBanner = memo(function LiveBridgeWarningBanner({
  isConnected,
  temperatureProp,
  diagnosticsExporting,
  onExportDiagnostics,
}: {
  isConnected: boolean;
  temperatureProp?: types.TemperatureData | null;
  diagnosticsExporting: boolean;
  onExportDiagnostics?: () => void;
}) {
  const { t } = useTranslation();
  const bridgeOk = useAppStore((state) => (temperatureProp ?? state.temperature)?.bridgeOk ?? true);
  const bridgeMessage = useAppStore((state) => (temperatureProp ?? state.temperature)?.bridgeMessage || '');
  const telemetryFailureStage = useAppStore((state) => (temperatureProp ?? state.temperature)?.telemetryFailureStage || '');
  const isBridgeStarting = isBridgeStartingTelemetry({ bridgeOk, bridgeMessage, telemetryFailureStage });
  const hasBridgeWarning = isConnected && bridgeOk === false && !isBridgeStarting;
  const [bridgeWarningReady, setBridgeWarningReady] = useState(false);
  const [bridgeStatus, setBridgeStatus] = useState<BridgeRuntimeStatus | null>(null);

  useEffect(() => {
    if (!hasBridgeWarning) {
      setBridgeWarningReady(false);
      return;
    }
    const timer = window.setTimeout(() => setBridgeWarningReady(true), 2000);
    return () => window.clearTimeout(timer);
  }, [hasBridgeWarning]);

  useEffect(() => {
    if (!hasBridgeWarning || !bridgeWarningReady) {
      setBridgeStatus(null);
      return;
    }

    let cancelled = false;
    const loadBridgeStatus = async () => {
      try {
        const status = await apiService.getBridgeProgramStatus();
        if (!cancelled) {
          setBridgeStatus((status || null) as BridgeRuntimeStatus | null);
        }
      } catch {
        if (!cancelled) {
          setBridgeStatus(null);
        }
      }
    };

    loadBridgeStatus();
    return () => {
      cancelled = true;
    };
  }, [bridgeWarningReady, hasBridgeWarning]);

  if (!bridgeWarningReady || bridgeStatus?.state === 'starting' || bridgeStatus?.state === 'not_started') {
    return null;
  }

  const bridgeStateLabel = bridgeStatus?.state === 'running_owned'
    ? t('deviceStatus.bridgeState.runningOwned')
    : bridgeStatus?.state === 'attached'
      ? t('deviceStatus.bridgeState.attached')
      : bridgeStatus?.state === 'starting'
        ? t('deviceStatus.bridgeState.starting')
        : bridgeStatus?.state === 'degraded'
          ? t('deviceStatus.bridgeState.degraded')
          : bridgeStatus?.state === 'failed'
            ? t('deviceStatus.bridgeState.failed')
            : bridgeStatus?.state === 'stopping'
              ? t('deviceStatus.bridgeState.stopping')
              : bridgeStatus?.state === 'stopped'
                ? t('deviceStatus.bridgeState.stopped')
                : bridgeStatus?.state === 'not_started'
                  ? t('deviceStatus.bridgeState.notStarted')
                  : '';

  return (
    <m.div
      initial={{ opacity: 0, height: 0 }}
      animate={{ opacity: 1, height: 'auto' }}
      className="overflow-hidden"
    >
      <div className="rounded-xl border border-amber-200 bg-amber-50/70 p-3 text-sm dark:border-amber-800/60 dark:bg-amber-900/20">
        <div className="flex items-start gap-2 text-amber-800 dark:text-amber-200">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
          <div className="flex-1">
            <p>{bridgeMessage || t('deviceStatus.bridgeWarning.default')}</p>
            <ol
              data-device-recovery="temperature"
              className="mt-2 list-decimal space-y-1 pl-5 text-xs text-amber-700/90 dark:text-amber-200/80"
            >
              <li>{t('deviceStatus.bridgeWarning.recovery.checkServices')}</li>
              <li>{t('deviceStatus.bridgeWarning.recovery.releaseCompetingTools')}</li>
              <li>{t('deviceStatus.bridgeWarning.recovery.retryOrExport')}</li>
            </ol>
            {bridgeStatus && (
              <div className="mt-2 space-y-1 text-xs text-amber-700/90 dark:text-amber-200/80">
                {bridgeStateLabel && (
                  <p>
                    {t('deviceStatus.bridgeWarning.stateLine', { state: bridgeStateLabel })}
                    {typeof bridgeStatus.ownsProcess === 'boolean' ? ` · ${bridgeStatus.ownsProcess ? t('deviceStatus.bridgeWarning.ownsProcess') : t('deviceStatus.bridgeWarning.sharedProcess')}` : ''}
                  </p>
                )}
                {bridgeStatus.pipeName && <p>{t('deviceStatus.bridgeWarning.pipeLine', { pipe: bridgeStatus.pipeName })}</p>}
                {bridgeStatus.lastError && bridgeStatus.lastError !== bridgeMessage && <p>{t('deviceStatus.bridgeWarning.diagnosticsLine', { message: bridgeStatus.lastError })}</p>}
              </div>
            )}
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <button
                onClick={async () => {
                  try {
                    await apiService.restartPawnIO();
                  } catch { /* ignore */ }
                }}
                className="inline-flex items-center gap-1.5 rounded-lg border border-amber-300 bg-amber-100 px-3 py-1.5 text-xs font-medium text-amber-900 transition-colors hover:bg-amber-200 dark:border-amber-700 dark:bg-amber-900/40 dark:text-amber-200 dark:hover:bg-amber-800/60"
              >
                <RotateCw className="h-3 w-3" />
                {t('deviceStatus.bridgeWarning.reinitialize')}
              </button>
              {onExportDiagnostics && (
                <button
                  type="button"
                  disabled={diagnosticsExporting}
                  onClick={onExportDiagnostics}
                  className="inline-flex items-center gap-1.5 rounded-lg border border-amber-300 bg-background/60 px-3 py-1.5 text-xs font-medium text-amber-900 transition-colors hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-60 dark:border-amber-700 dark:bg-background/25 dark:text-amber-100 dark:hover:bg-amber-900/40"
                >
                  <Download className="h-3 w-3" />
                  {diagnosticsExporting ? t('appShell.diagnostics.exporting') : t('appShell.diagnostics.export')}
                </button>
              )}
            </div>
          </div>
        </div>
      </div>
    </m.div>
  );
});

const LiveMonitorOnlyCards = memo(function LiveMonitorOnlyCards({
  config,
  temperatureProp,
}: {
  config: types.AppConfig;
  temperatureProp?: types.TemperatureData | null;
}) {
  const { t } = useTranslation();
  const { points: temperatureHistory } = useTemperatureHistory();
  const {
    temperature,
    gpuReadState,
    gpuNotPolled,
    gpuIdleLabel,
    gpuAutoIdleNote,
    gpuNotPolledTooltip,
    gpuReady,
    cpuDisplayTemp,
    cpuDisplayReady,
  } = useLiveCpuGpuMetrics(config, temperatureProp);

  const cpuPowerFormatted = formatPowerWatts(temperature?.cpuPowerWatts);
  const cpuPowerAvailable = !cpuDisplayReady
    || cpuPowerFormatted !== '-- W'
    || temperatureHistory.some((point) => Number(point.cpuPowerWatts || 0) >= MIN_VALID_POWER_WATTS);

  return (
    <m.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.3, ease: 'easeOut' }}
      className="grid grid-cols-1 items-stretch gap-3 md:grid-cols-2"
    >
      <MonitorInfoCard
        icon={CPU_METRIC_ICON}
        label={t('deviceStatus.monitorOnly.cpuInfo')}
        themeCard="cpu-temperature"
        model={temperature?.cpuModel || t('deviceStatus.device.unknown')}
        temperature={cpuDisplayTemp}
        ready={cpuDisplayReady}
        powerLabel={t('deviceStatus.monitorOnly.power')}
        power={cpuPowerAvailable ? cpuPowerFormatted : '--'}
        powerPoints={temperatureHistory}
        powerSeries="cpu"
        powerAvailable={cpuPowerAvailable}
        unavailableLabel={!cpuPowerAvailable ? t('deviceStatus.tempGauge.notRead') : undefined}
      />
      <MonitorInfoCard
        icon={GPU_METRIC_ICON}
        label={t('deviceStatus.monitorOnly.gpuInfo')}
        themeCard="gpu-temperature"
        model={temperature?.gpuModel || t('deviceStatus.device.unknown')}
        temperature={temperature?.gpuTemp}
        ready={gpuReady}
        powerLabel={t('deviceStatus.monitorOnly.power')}
        power={formatGpuPowerWatts(temperature?.gpuPowerWatts, gpuReadState)}
        powerPoints={temperatureHistory}
        powerSeries="gpu"
        powerAvailable={!gpuNotPolled}
        idleLabel={gpuIdleLabel}
        idleNote={gpuAutoIdleNote}
        idleTooltip={gpuNotPolledTooltip}
      />
    </m.div>
  );
});

const LiveCpuMetricCard = memo(function LiveCpuMetricCard({
  config,
  temperatureProp,
}: {
  config: types.AppConfig;
  temperatureProp?: types.TemperatureData | null;
}) {
  const { t } = useTranslation();
  const {
    temperature,
    cpuDisplayTemp,
    cpuDisplayReady,
    cpuMetricLabel,
  } = useLiveCpuGpuMetrics(config, temperatureProp);
  const cpuPowerParts = splitPowerParts(formatPowerWatts(temperature?.cpuPowerWatts));
  const cpuPowerUnavailable = cpuPowerParts.value === '--';

  return (
    <div data-theme-card="cpu-temperature" className="glacier-metric-card flex h-full min-h-[218px] flex-col rounded-xl border border-border bg-card px-4 pb-3.5 pt-3 shadow-sm shadow-black/5">
      <MetricHeader
        icon={CPU_METRIC_ICON}
        label={cpuMetricLabel}
      />
      <div className="flex flex-1 items-start justify-center pt-3">
        <TempGaugeDisplay temp={cpuDisplayTemp} ready={cpuDisplayReady} />
      </div>
      <div data-theme-card="cpu-power" className="glacier-stat-tile mt-3 flex min-h-[56px] w-full items-center justify-between gap-2.5 rounded-lg border border-border/75 bg-background/55 px-3.5 py-2">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
            <Zap className={clsx('h-3.5 w-3.5 shrink-0', cpuPowerUnavailable ? 'text-muted-foreground/50' : 'text-amber-500 dark:text-amber-400')} />
            <span className="truncate">{t('deviceStatus.stats.cpuPower')}</span>
          </div>
          {temperature?.cpuModel?.trim() && (
            <div className="mt-0.5 flex min-w-0">
              <HardwareModelLabel model={temperature?.cpuModel} fallbackLabel="" />
            </div>
          )}
        </div>
        <div className="flex shrink-0 items-baseline gap-0.5 tabular-nums">
          <span className={clsx('text-[20px] font-bold leading-none tracking-[-0.02em]', cpuPowerUnavailable ? 'text-muted-foreground/65' : 'text-foreground')}>
            {cpuPowerParts.value}
          </span>
          {!cpuPowerUnavailable && (
            <span className="text-xs font-semibold text-muted-foreground/80">
              {cpuPowerParts.unit}
            </span>
          )}
        </div>
      </div>
    </div>
  );
});

const LiveGpuMetricCard = memo(function LiveGpuMetricCard({
  config,
  temperatureProp,
}: {
  config: types.AppConfig;
  temperatureProp?: types.TemperatureData | null;
}) {
  const { t } = useTranslation();
  const {
    temperature,
    gpuReadState,
    gpuNotPolled,
    gpuIdleLabel,
    gpuAutoIdleNote,
    gpuNotPolledTooltip,
    gpuReady,
  } = useLiveCpuGpuMetrics(config, temperatureProp);
  const gpuPowerParts = splitPowerParts(formatGpuPowerWatts(temperature?.gpuPowerWatts, gpuReadState));

  return (
    <div data-theme-card="gpu-temperature" className="glacier-metric-card flex h-full min-h-[218px] flex-col rounded-xl border border-border bg-card px-4 pb-3.5 pt-3 shadow-sm shadow-black/5">
      <MetricHeader
        icon={GPU_METRIC_ICON}
        label={t('deviceStatus.metrics.gpuTemperature')}
      />
      <div className="flex flex-1 flex-col items-center justify-between gap-2 pt-3">
        <TempGaugeDisplay
          temp={temperature?.gpuTemp}
          ready={gpuReady}
          idleLabel={gpuIdleLabel}
        />
        {gpuAutoIdleNote && gpuNotPolledTooltip && (
          <GpuIdleNotice note={gpuAutoIdleNote} tooltip={gpuNotPolledTooltip} />
        )}
      </div>
      <div data-theme-card="gpu-power" className="glacier-stat-tile mt-3 flex min-h-[56px] w-full items-center justify-between gap-2.5 rounded-lg border border-border/75 bg-background/55 px-3.5 py-2">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
            <Zap className={clsx('h-3.5 w-3.5 shrink-0', gpuNotPolled ? 'text-muted-foreground/50' : 'text-violet-500 dark:text-violet-400')} />
            <span className="truncate">{t('deviceStatus.stats.gpuPower')}</span>
          </div>
          {temperature?.gpuModel?.trim() && (
            <div className="mt-0.5 flex min-w-0">
              <HardwareModelLabel model={temperature?.gpuModel} fallbackLabel="" />
            </div>
          )}
        </div>
        <div className="flex shrink-0 items-baseline gap-0.5 tabular-nums">
          <span className={clsx('text-[20px] font-bold leading-none tracking-[-0.02em]', gpuNotPolled || gpuPowerParts.value === '--' ? 'text-muted-foreground/70' : 'text-foreground')}>
            {gpuPowerParts.value}
          </span>
          <span className="text-xs font-semibold text-muted-foreground/75">
            {gpuPowerParts.unit}
          </span>
        </div>
      </div>
    </div>
  );
});

const LiveFanMetricCard = memo(function LiveFanMetricCard({
  isConnected,
  monitorOnlyActive,
  deviceSettings,
  runtimeDeviceProfile,
  config,
  modeTitle,
  modeDisplayTitle,
  fanDataProp,
  temperatureProp,
}: {
  isConnected: boolean;
  monitorOnlyActive: boolean;
  deviceSettings: DeviceSettings | null;
  runtimeDeviceProfile?: types.DeviceProfile | null;
  config: types.AppConfig;
  modeTitle: string;
  modeDisplayTitle: string;
  fanDataProp?: types.FanData | null;
  temperatureProp?: types.TemperatureData | null;
}) {
  const { t } = useTranslation();
  const storeFanData = useAppStore(selectFanData);
  const fanData = fanDataProp ?? storeFanData;
  const maxTemp = useAppStore((state) => (temperatureProp ?? state.temperature)?.maxTemp || 0);
  const fanSpeedUnit = getFanSpeedUnit(fanData as any, config as any, runtimeDeviceProfile as any);
  const fanSpeedLabel = fanSpeedUnitLabel(fanSpeedUnit);
  const fanSpeedRange = useMemo(
    () => getFanSpeedRange(config as any, fanSpeedUnit, runtimeDeviceProfile as any),
    [config, fanSpeedUnit, runtimeDeviceProfile],
  );
  const flyDigiCapability = useMemo(() => getFlyDigiRuntimeCapability(fanData as any, deviceSettings), [deviceSettings, fanData]);
  const displayFanSpeedRange = useMemo(() => {
    const runtimeMax = Number(flyDigiCapability?.maxRpm || 0);
    if (fanSpeedUnit === 'rpm' && Number.isFinite(runtimeMax) && runtimeMax > fanSpeedRange.min) {
      return { ...fanSpeedRange, max: Math.min(fanSpeedRange.max, runtimeMax) };
    }
    return fanSpeedRange;
  }, [fanSpeedRange, fanSpeedUnit, flyDigiCapability]);
  const currentFanSpeed = clampFanSpeedToRange(
    readCurrentFanSpeed(fanData, fanSpeedUnit, config as any, runtimeDeviceProfile as any),
    displayFanSpeedRange,
  );
  const targetFanSpeed = clampFanSpeedToRange(
    readTargetFanSpeed(fanData, fanSpeedUnit, config as any, runtimeDeviceProfile as any),
    displayFanSpeedRange,
  );
  const displayCurrentFanSpeed = monitorOnlyActive && !isConnected ? undefined : currentFanSpeed;
  const displayTargetFanSpeed = monitorOnlyActive && !isConnected ? undefined : targetFanSpeed;
  const fanSpinDuration = getFanSpinDuration(currentFanSpeed, displayFanSpeedRange.min, displayFanSpeedRange.max);
  const fanHeaderIcon = useMemo(
    () => <SpinningFanIcon duration={fanSpinDuration} className="h-4 w-4" />,
    [fanSpinDuration],
  );
  const translatedWorkMode = translateWorkModeLabel(fanData?.workMode, t);
  const distinctWorkModeLabel =
    translatedWorkMode !== '--' &&
    translatedWorkMode !== t('controlPanel.overview.workModes.auto') &&
    translatedWorkMode !== t('controlPanel.overview.workModes.manual') &&
    translatedWorkMode !== modeTitle
      ? translatedWorkMode
      : '';
  const maxTempStatus = getTempStatus(maxTemp);

  return (
    <div data-theme-card="fan-speed" className="glacier-metric-card flex h-full min-h-[218px] flex-col rounded-xl border border-border bg-card px-4 pb-3.5 pt-3 shadow-sm shadow-black/5">
      <MetricHeader
        icon={fanHeaderIcon}
        label={t('deviceStatus.metrics.fanRpm')}
      />
      <div className="flex flex-1 items-start justify-center pt-3">
        <FanSpeedDisplay
          currentSpeed={displayCurrentFanSpeed}
          targetSpeed={displayTargetFanSpeed}
          unit={fanSpeedLabel}
          minSpeed={displayFanSpeedRange.min}
          maxSpeed={displayFanSpeedRange.max}
        />
      </div>
      {isConnected && (
        <div data-theme-section="control-protection" className="mt-3 grid w-full grid-cols-2 gap-2">
          <div
            data-theme-card="control-mode"
            data-control-mode={config.autoControl ? 'auto' : 'manual'}
            className="glacier-stat-tile flex min-h-[56px] min-w-0 flex-col justify-center rounded-lg border border-border/75 bg-background/55 px-3 py-2"
          >
            <div className="mb-1 flex items-center gap-1.5 text-[11px] text-muted-foreground">
              <Sparkles className="h-3 w-3 opacity-80" />
              {t('deviceStatus.stats.controlMode')}
            </div>
            <div className="flex min-w-0 items-center justify-between gap-1.5">
              <div title={modeDisplayTitle} className={clsx('min-w-0 truncate text-sm font-semibold', config.autoControl ? 'text-primary' : 'text-amber-600 dark:text-amber-400')}>
                {modeDisplayTitle}
              </div>
              {distinctWorkModeLabel && (
                <span className="shrink-0 rounded-full border border-border/70 bg-background/70 px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                  {distinctWorkModeLabel}
                </span>
              )}
            </div>
          </div>

          <div data-theme-card="temperature-state" className="glacier-stat-tile flex min-h-[56px] min-w-0 flex-col justify-center rounded-lg border border-border/75 bg-background/55 px-3 py-2">
            <div className="mb-1 flex items-center gap-1.5 text-[11px] text-muted-foreground">
              <ShieldCheck className="h-3 w-3 opacity-80" />
              {t('deviceStatus.stats.tempStatus')}
            </div>
            <div className={clsx('truncate text-sm font-semibold tabular-nums', maxTempStatus.color)}>
              {t(maxTempStatus.labelKey)}
            </div>
          </div>
        </div>
      )}
    </div>
  );
});

const LiveMiniFanCurveCard = memo(function LiveMiniFanCurveCard({
  curve,
  minSpeed,
  maxSpeed,
  unitLabel,
  profileName,
  config,
  temperatureProp,
  onOpen,
}: {
  curve: types.FanCurvePoint[];
  minSpeed: number;
  maxSpeed: number;
  unitLabel: string;
  profileName?: string;
  config: types.AppConfig;
  temperatureProp?: types.TemperatureData | null;
  onOpen: () => void;
}) {
  const referenceTemp = useAppStore((state) => resolveReferenceTemp(temperatureProp ?? state.temperature, config));
  return (
    <MiniFanCurveChart
      curve={curve}
      currentTemp={referenceTemp}
      minSpeed={minSpeed}
      maxSpeed={maxSpeed}
      unitLabel={unitLabel}
      profileName={profileName}
      onOpen={onOpen}
    />
  );
});

const LiveTemperatureHistoryCard = memo(function LiveTemperatureHistoryCard({
  minSpeed,
  maxSpeed,
  visibleSeries,
  orderedSeries,
  title,
  onOpen,
}: {
  minSpeed: number;
  maxSpeed: number;
  visibleSeries: Record<HistorySeriesKey, boolean>;
  orderedSeries: HistorySeriesKey[];
  title?: string;
  onOpen?: () => void;
}) {
  const {
    points: temperatureHistory,
    enabled: temperatureHistoryEnabled,
    source: temperatureHistorySource,
    retentionHours: historyRetentionHours,
  } = useTemperatureHistory();

  return (
    <TemperatureHistoryPanel
      points={temperatureHistory}
      enabled={temperatureHistoryEnabled}
      source={temperatureHistorySource}
      retentionHours={historyRetentionHours}
      minSpeed={minSpeed}
      maxSpeed={maxSpeed}
      visibleSeries={visibleSeries}
      orderedSeries={orderedSeries}
      title={title}
      onOpen={onOpen}
    />
  );
});

/* ── Main component ── */

export default function DeviceStatus({
  isConnected,
  runtimeState,
  deviceModel,
  deviceSettings,
  fanData: fanDataProp,
  temperature: temperatureProp,
  runtimeDeviceProfile,
  config,
  coreServiceError,
  onConnect,
  onDisconnect,
  onConfigChange,
  onOpenCurveEditor: onOpenCurveEditorProp,
  onOpenHistoryDetails: onOpenHistoryDetailsProp,
	onExportDiagnostics,
	diagnosticsExporting = false,
	monitorOnlyActive = false,
	onEnableMonitorOnly,
}: DeviceStatusProps) {
  const { t } = useTranslation();
  const onOpenCurveEditorRef = useRef(onOpenCurveEditorProp);
  onOpenCurveEditorRef.current = onOpenCurveEditorProp;
  const onOpenHistoryDetailsRef = useRef(onOpenHistoryDetailsProp);
  onOpenHistoryDetailsRef.current = onOpenHistoryDetailsProp;
  const onOpenCurveEditor = useCallback(() => {
    onOpenCurveEditorRef.current();
  }, []);
  const onOpenHistoryDetails = useCallback(() => {
    onOpenHistoryDetailsRef.current();
  }, []);
  const [activeCurveProfileName, setActiveCurveProfileName] = useState('');
  const [activeCurveProfileCurve, setActiveCurveProfileCurve] = useState<types.FanCurvePoint[] | null>(null);
  const {
    homeSeriesVisibility,
  } = useHistoryDisplayPreferences();
	const showMonitoringLayout = isConnected || monitorOnlyActive;
  const configuredDeviceProfile = useMemo(() => (getActiveDeviceProfile(config as any) as types.DeviceProfile | undefined) || null, [config]);
  const activeDeviceProfile = runtimeDeviceProfile || configuredDeviceProfile;
  const activeCurveContextKey = [
    isConnected ? 'connected' : 'offline',
    runtimeDeviceProfile?.id || '',
    runtimeDeviceProfile?.transport || '',
    (config as any).deviceTransport || '',
    (config as any).activeDeviceProfileId || '',
  ].join(':');

  useEffect(() => {
    let cancelled = false;

    if (!isConnected) {
      setActiveCurveProfileName('');
      setActiveCurveProfileCurve(null);
      return () => {
        cancelled = true;
      };
    }

    const loadActiveCurveProfile = async () => {
      try {
        const payload = await apiService.getFanCurveProfiles();
        const profiles = Array.isArray(payload?.profiles) ? payload.profiles : [];
        const preferredActiveId = (payload?.activeId || (config as any).activeFanCurveProfileId || profiles[0]?.id || '') as string;
        const activeProfile = profiles.find((p) => p.id === preferredActiveId) ?? profiles[0];
        if (!cancelled) {
          setActiveCurveProfileName(activeProfile?.name || '');
          setActiveCurveProfileCurve(Array.isArray(activeProfile?.curve) ? activeProfile.curve : null);
        }
      } catch {
        if (!cancelled) {
          setActiveCurveProfileName('');
          setActiveCurveProfileCurve(null);
        }
      }
    };

    loadActiveCurveProfile();
    return () => {
      cancelled = true;
    };
  }, [activeCurveContextKey, isConnected, (config as any).activeFanCurveProfileId]);

  const handleAutoControlChange = async (enabled: boolean) => {
    try {
      await apiService.setAutoControl(enabled);
      const latest = await apiService.getConfig();
      onConfigChange(types.AppConfig.createFrom({ ...latest, autoControl: enabled }));
    } catch (err) {
      toast.error(t('controlPanel.fan.autoControlApplyFailed', { error: getErrorMessage(err) }));
      try {
        onConfigChange(types.AppConfig.createFrom(await apiService.getConfig()));
      } catch {
        // Keep the current view if the config refresh also fails.
      }
    }
  };

  const activeDeviceName = activeDeviceProfile?.displayName?.trim() || '';
  const showRuntimeState = runtimeState !== 'disconnected';
  const runtimeStateClass = runtimeState === 'ready'
    ? 'border border-emerald-500/25 bg-emerald-500/12 text-emerald-700 dark:border-emerald-400/25 dark:bg-emerald-500/15 dark:text-emerald-300'
    : runtimeState === 'unavailable'
      ? 'bg-amber-500/10 text-amber-700 dark:text-amber-300'
      : 'bg-primary/10 text-primary';
  const deviceModelName = isConnected
    ? (deviceModel?.trim() || activeDeviceName || t('deviceStatus.device.unknown'))
    : t('deviceStatus.disconnected.title');
  const modeTitle = config.autoControl ? t('deviceStatus.mode.smartControl') : config.customSpeedEnabled ? t('deviceStatus.mode.fixedSpeed') : t('deviceStatus.mode.manualStrategy');
  const storeFanTransport = useAppStore((state) => state.fanData?.transport ?? '');
  const storeFanSpeedUnit = useAppStore((state) => state.fanData?.speedUnit ?? '');
  const fanTransport = fanDataProp?.transport || storeFanTransport;
  const fanSpeedUnitRaw = fanDataProp?.speedUnit || storeFanSpeedUnit;
  const fanSpeedCarrier = useMemo(
    () => (fanTransport || fanSpeedUnitRaw ? { transport: fanTransport, speedUnit: fanSpeedUnitRaw } : null),
    [fanSpeedUnitRaw, fanTransport],
  );
  const fanSpeedUnit = getFanSpeedUnit(fanSpeedCarrier as any, config as any, runtimeDeviceProfile as any);
  const fanSpeedLabel = fanSpeedUnitLabel(fanSpeedUnit);
  const fanSpeedRange = useMemo(() => getFanSpeedRange(config as any, fanSpeedUnit, runtimeDeviceProfile as any), [config, fanSpeedUnit, runtimeDeviceProfile]);
  const modeDisplayTitle = activeCurveProfileName ? t('deviceStatus.mode.withProfile', { mode: modeTitle, profile: activeCurveProfileName }) : modeTitle;

  return (
    <div className="home-dashboard-root flex flex-1 flex-col gap-3">
      {/* ── Device header card ── */}
      <div data-theme-section="hero" data-theme-card="device-hero" className="glacier-hero-card relative flex items-center overflow-hidden rounded-xl border border-border bg-card px-4 py-3.5 shadow-sm shadow-black/5">
        <DeviceHeroArtwork />
        <div className="glacier-hero-content relative z-10 flex w-full flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3">
              <div data-theme-ui="hero-brand-mark" className="flex h-14 w-14 items-center justify-center overflow-hidden rounded-xl bg-primary/10 text-primary">
                <Fan className="h-8 w-8" />
              </div>
            <div>
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-base font-semibold text-foreground">{deviceModelName}</span>
                <span
                  className={clsx(
                    'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-[11px] font-semibold',
                    isConnected
                      ? 'border border-primary/25 bg-primary/10 text-primary'
                      : 'bg-destructive/10 text-destructive',
                  )}
                >
                  {isConnected && <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 ring-2 ring-emerald-500/25" />}
                  {isConnected ? t('deviceStatus.connectStatus.connected') : t('deviceStatus.connectStatus.offline')}
                </span>
                {showRuntimeState && (
                  <span className={clsx('rounded-md px-2 py-0.5 text-[11px] font-semibold', runtimeStateClass)}>
                    {t(`deviceStatus.runtimeState.${runtimeState}`)}
                  </span>
                )}
              </div>
              {isConnected && (
                <LiveHeroModeSummary
                  config={config}
                  fanDataProp={fanDataProp}
                  runtimeDeviceProfile={runtimeDeviceProfile}
                  fanSpeedRange={fanSpeedRange}
                  fanSpeedLabel={fanSpeedLabel}
                />
              )}
              {!isConnected && (
                <p className={clsx('mt-1 text-xs', coreServiceError ? 'text-destructive' : 'text-muted-foreground')}>
                  {coreServiceError
                    ? t('deviceStatus.hero.coreUnavailable')
                    : monitorOnlyActive
                      ? t('deviceStatus.monitorOnly.sensorOnly')
                      : t('deviceStatus.disconnected.description')}
                </p>
              )}
            </div>
          </div>

          <div
            data-theme-ui="hero-actions"
            data-hero-connected={isConnected ? 'true' : 'false'}
            className="glacier-hero-actions flex items-center gap-3"
          >
            {isConnected && (
              <ToggleSwitch
                enabled={config.autoControl}
                onChange={handleAutoControlChange}
                label={t('deviceStatus.actions.smartControl')}
                size="md"
                color="blue"
              />
            )}
            {isConnected && (!monitorOnlyActive || isConnected) && (
              <span className="h-5 w-px bg-border/70" aria-hidden="true" />
            )}
            {(!monitorOnlyActive || isConnected) && (
              <Button
                variant={isConnected ? 'secondary' : 'primary'}
                size="sm"
                onClick={isConnected ? onDisconnect : onConnect}
              >
                {isConnected ? t('deviceStatus.actions.disconnect') : t('deviceStatus.actions.connect')}
              </Button>
            )}
          </div>
        </div>
      </div>

      {/* ── Bridge warning ── */}
      <LiveBridgeWarningBanner
        isConnected={isConnected}
        temperatureProp={temperatureProp}
        diagnosticsExporting={diagnosticsExporting}
        onExportDiagnostics={onExportDiagnostics}
      />

      {/* ── Main Bento Grid ── */}
      {monitorOnlyActive && !isConnected ? (
        <LiveMonitorOnlyCards config={config} temperatureProp={temperatureProp} />
      ) : showMonitoringLayout ? (
        <m.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.25, ease: 'easeOut' }}
          data-theme-section="home-bento"
          className="home-bento-layout"
        >
          {/* Row 1: Equal-Height Telemetry Trio */}
          <div className="home-metrics-grid">
            <LiveCpuMetricCard config={config} temperatureProp={temperatureProp} />
            <LiveGpuMetricCard config={config} temperatureProp={temperatureProp} />
            <LiveFanMetricCard
              isConnected={isConnected}
              monitorOnlyActive={monitorOnlyActive}
              deviceSettings={deviceSettings}
              runtimeDeviceProfile={runtimeDeviceProfile}
              config={config}
              modeTitle={modeTitle}
              modeDisplayTitle={modeDisplayTitle}
              fanDataProp={fanDataProp}
              temperatureProp={temperatureProp}
            />
          </div>

          {/* Row 2: Curve Preview + History Overview */}
          {!monitorOnlyActive && (
            <div className="home-charts-grid">
              <LiveMiniFanCurveCard
                curve={activeCurveProfileCurve || config.fanCurve}
                minSpeed={fanSpeedRange.min}
                maxSpeed={fanSpeedRange.max}
                unitLabel={fanSpeedLabel}
                profileName={activeCurveProfileName}
                config={config}
                temperatureProp={temperatureProp}
                onOpen={onOpenCurveEditor}
              />
              <LiveTemperatureHistoryCard
                minSpeed={fanSpeedRange.min}
                maxSpeed={fanSpeedRange.max}
                visibleSeries={homeSeriesVisibility}
                orderedSeries={monitorOnlyActive ? HISTORY_SERIES_ORDER.filter((series) => series !== 'fan') : HISTORY_SERIES_ORDER}
                title={monitorOnlyActive ? t('deviceStatus.monitorOnly.historyTitle') : undefined}
                onOpen={monitorOnlyActive ? undefined : onOpenHistoryDetails}
              />
            </div>
          )}
        </m.div>
      ) : (
        <m.div
          initial={{ scale: 0.98 }}
          animate={{ scale: 1 }}
          transition={{ duration: 0.3 }}
          data-theme-card="connection-empty"
          className="rounded-xl border border-dashed border-border bg-card p-14 text-center"
        >
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-xl bg-muted">
            <Wifi className="h-7 w-7 text-muted-foreground" />
          </div>
          <h3 className="mb-1.5 text-lg font-semibold">
            {t('deviceStatus.disconnected.title')}
          </h3>
          <p className="mb-4 text-base text-muted-foreground">
            {t('deviceStatus.disconnected.description')}
          </p>
          <ol
            data-device-recovery="connection"
            className="mx-auto mb-5 max-w-xl list-decimal space-y-1.5 pl-5 text-left text-sm text-muted-foreground"
          >
            <li>{t('deviceStatus.recovery.deviceAvailable')}</li>
            <li>{t('deviceStatus.recovery.releaseCompetingTools')}</li>
            <li>{t('deviceStatus.recovery.retryOrExport')}</li>
          </ol>
          <div className="flex flex-wrap items-center justify-center gap-2">
            <Button onClick={onConnect} size="md" icon={<RotateCw className="h-4 w-4" />}>
              {t('deviceStatus.actions.connectDevice')}
            </Button>
            {onExportDiagnostics && (
              <Button
                variant="secondary"
                size="md"
                disabled={diagnosticsExporting}
                onClick={onExportDiagnostics}
                icon={<Download className="h-4 w-4" />}
              >
                {diagnosticsExporting ? t('appShell.diagnostics.exporting') : t('appShell.diagnostics.export')}
              </Button>
            )}
          </div>
        </m.div>
      )}

      {!isConnected && !monitorOnlyActive && onEnableMonitorOnly && (
        <div data-theme-card="monitor-only-entry" className="flex flex-col items-start justify-between gap-3 rounded-xl border border-border bg-card p-4 shadow-sm shadow-black/5 sm:flex-row sm:items-center">
          <div className="min-w-0">
            <h3 className="text-sm font-semibold text-foreground">{t('deviceStatus.monitorOnly.entryTitle')}</h3>
            <p className="mt-1 text-xs text-muted-foreground">{t('deviceStatus.monitorOnly.entryDescription')}</p>
          </div>
          <Button onClick={onEnableMonitorOnly} size="sm" icon={<Gauge className="h-4 w-4" />}>
            {t('deviceStatus.monitorOnly.enable')}
          </Button>
        </div>
      )}

      {showMonitoringLayout && (
        monitorOnlyActive && (
          <m.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ delay: 0.2, duration: 0.3 }}
            className="grid grid-cols-1 items-stretch gap-3 lg:grid-cols-1"
          >
            <FanCurve
              config={config}
              onConfigChange={onConfigChange}
              isConnected={false}
              runtimeDeviceProfile={runtimeDeviceProfile}
              deviceModel={deviceModel}
              focusTarget={null}
              onFocusHandled={() => undefined}
              historyOnly
            />
          </m.div>
        )
      )}
    </div>
  );
}
