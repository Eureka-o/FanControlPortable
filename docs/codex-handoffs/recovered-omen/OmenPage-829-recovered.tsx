'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react';
import { Activity, Gauge, Laptop, Power, RefreshCw, RotateCcw, Save, SlidersHorizontal } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useShallow } from 'zustand/react/shallow';
import { apiService } from '../services/api';
import { useAppStore } from '../store/app-store';
import type { FanCurvePoint, OmenFanStatus, PluginInfo } from '../types/app';
import { Badge, Button, Card, Slider, ToggleSwitch } from './ui';

const OMEN_PLUGIN_ID = 'omen-fan';
const MIN_TEMP = 35;
const MAX_TEMP = 95;
const MIN_RPM = 800;
const MAX_RPM = 6000;
const RPM_STEP = 100;
const CHART_WIDTH = 720;
const CHART_HEIGHT = 320;
const CHART_MARGIN = { top: 30, right: 28, bottom: 44, left: 68 };
const TEMP_TICKS = [35, 50, 65, 80, 95];
const RPM_TICKS = [800, 2000, 3200, 4400, 6000];
const DEFAULT_OMEN_MOCK_PORT = 8787;

type OmenSection = 'mode' | 'power' | 'fan';
type OmenMode = 'balanced' | 'performance' | 'quiet' | 'custom';
type OmenPowerProfile = 'quiet' | 'balanced' | 'performance';

const SECTION_OPTIONS: Array<{ id: OmenSection; label: string }> = [
  { id: 'mode', label: '妯″紡鍒囨崲' },
  { id: 'power', label: '鍔熻€楄皟鏁? },
  { id: 'fan', label: '鑷畾涔夐鎵? },
];

const MODE_OPTIONS: Array<{ id: OmenMode; label: string; description: string }> = [
  { id: 'balanced', label: '鍧囪　', description: '鏃ュ父浣跨敤鐨勬湰鍦伴瑙堟ā寮? },
  { id: 'performance', label: '鎬ц兘', description: '鏇存縺杩涚殑鏈湴棰勮妯″紡' },
  { id: 'quiet', label: '瀹夐潤', description: '浣庡櫔澹板€惧悜鐨勬湰鍦伴瑙堟ā寮? },
  { id: 'custom', label: '鑷畾涔?, description: '杩涘叆鑷畾涔夐鎵囨洸绾胯皟璇? },
];

const POWER_OPTIONS: Array<{ id: OmenPowerProfile; label: string; watts: string }> = [
  { id: 'quiet', label: '瀹夐潤', watts: '35W' },
  { id: 'balanced', label: '鍧囪　', watts: '45W' },
  { id: 'performance', label: '鎬ц兘', watts: '55W' },
];
const OMEN_UNSUPPORTED_SUMMARY = '褰撳墠璁惧鏈€氳繃 OMEN WMI 鏀寔妫€鏌ャ€傚彲缁х画浣跨敤璋冭瘯棰勮锛岃缁嗗師鍥犲凡璁板綍鍒拌瘖鏂棩蹇椼€?;

const DEFAULT_OMEN_CURVE: FanCurvePoint[] = [
  { temperature: 45, rpm: 1400 },
  { temperature: 55, rpm: 1900 },
  { temperature: 65, rpm: 2600 },
  { temperature: 75, rpm: 3600 },
  { temperature: 85, rpm: 4800 },
];

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, Number.isFinite(value) ? value : min));
}

function normalizeBiasPercent(value?: number | null) {
  const bias = Number(value ?? 0);
  const percent = Math.abs(bias) <= 1 ? bias * 100 : bias;
  return Math.round(clamp(percent, -100, 100));
}

function roundRpmToStep(value: number) {
  const rounded = Math.round(clamp(Number(value), MIN_RPM, MAX_RPM) / RPM_STEP) * RPM_STEP;
  return Math.round(clamp(rounded, MIN_RPM, MAX_RPM));
}

function parsePowerWatts(value: string) {
  return Math.round(Number.parseFloat(value.replace(/[^\d.]/g, '')) || 0);
}

function sanitizeCurve(curve?: FanCurvePoint[] | null) {
  const source = Array.isArray(curve) && curve.length > 0 ? curve : DEFAULT_OMEN_CURVE;
  return source
    .map((point) => ({
      temperature: Math.round(clamp(Number(point.temperature), MIN_TEMP, MAX_TEMP)),
      rpm: roundRpmToStep(Number(point.rpm)),
    }))
    .sort((left, right) => left.temperature - right.temperature);
}

function curvesEqual(left: FanCurvePoint[], right: FanCurvePoint[]) {
  if (left.length !== right.length) return false;
  return left.every((point, index) => (
    point.temperature === right[index]?.temperature && point.rpm === right[index]?.rpm
  ));
}

function formatRpm(value?: number) {
  return `${Math.round(Number(value) || 0)} RPM`;
}

function formatTargetRpm(value?: number) {
  return `${roundRpmToStep(Number(value) || MIN_RPM)} RPM`;
}

function formatTemp(value?: number) {
  return `${Math.round(Number(value) || 0)}掳C`;
}

function formatLevel(value?: number) {
  if (value === undefined || value === null) return '-';
  return String(Math.round(value));
}

function formatUpdatedAt(value?: number) {
  if (!value) return '-';
  const timestamp = value > 10_000_000_000 ? value : value * 1000;
  return new Date(timestamp).toLocaleTimeString();
}

function findModeLabel(mode: OmenMode) {
  return MODE_OPTIONS.find((option) => option.id === mode)?.label ?? '鍧囪　';
}

function isOmenMode(value?: string): value is OmenMode {
  return MODE_OPTIONS.some((option) => option.id === value);
}

function isOmenPowerProfile(value?: string): value is OmenPowerProfile {
  return POWER_OPTIONS.some((option) => option.id === value);
}

function calculateTargetRpmForCurve(curve: FanCurvePoint[], temperature?: number) {
  const points = sanitizeCurve(curve);
  const temp = clamp(Number(temperature), MIN_TEMP, MAX_TEMP);
  const first = points[0];
  const last = points[points.length - 1];
  if (!first || !last) return MIN_RPM;
  if (temp <= first.temperature) return roundRpmToStep(first.rpm);
  if (temp >= last.temperature) return roundRpmToStep(last.rpm);

  for (let index = 1; index < points.length; index += 1) {
    const left = points[index - 1];
    const right = points[index];
    if (temp <= right.temperature) {
      const span = Math.max(1, right.temperature - left.temperature);
      const ratio = (temp - left.temperature) / span;
      return roundRpmToStep(left.rpm + (right.rpm - left.rpm) * ratio);
    }
  }

  return roundRpmToStep(last.rpm);
}

function formatFanLevelDetail(level: number | undefined, estimated?: boolean) {
  const levelText = level === undefined || level === null ? '妗ｄ綅 -' : `妗ｄ綅 ${formatLevel(level)}`;
  return estimated ? `${levelText} 路 浼扮畻` : levelText;
}

function sanitizeDiagnosticText(value?: string | null) {
  if (!value) return '';
  const readable = value
    .replace(/\uFFFD/g, ' ')
    .replace(/[^\x20-\x7E]+/g, ' ')
    .replace(/\b(?:脙.|脗.|閿熸枻鎷?+\b/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
  if (!readable) return '';
  return readable.length > 140 ? `${readable.slice(0, 137)}...` : readable;
}

function StatusMetric({ label, value, detail }: { label: string; value: string; detail?: string }) {
  return (
    <div className="min-h-24 rounded-xl border border-border bg-background px-3 py-3">
      <div className="text-xs font-medium text-muted-foreground">{label}</div>
      <div className="mt-2 whitespace-nowrap text-xl font-semibold tracking-normal text-foreground">{value}</div>
      {detail && <div className="mt-1 text-xs text-muted-foreground">{detail}</div>}
    </div>
  );
}

function OmenCurveEditor({
  curve,
  dirty,
  onPointRpmChange,
}: {
  curve: FanCurvePoint[];
  dirty: boolean;
  onPointRpmChange: (index: number, rpm: number) => void;
}) {
  const { t } = useTranslation();
  const svgRef = useRef<SVGSVGElement | null>(null);
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);
  const activePointIndex = activeIndex ?? hoverIndex;
  const chartLeft = CHART_MARGIN.left;
  const chartTop = CHART_MARGIN.top;
  const chartRight = CHART_WIDTH - CHART_MARGIN.right;
  const chartBottom = CHART_HEIGHT - CHART_MARGIN.bottom;
  const chartInnerWidth = chartRight - chartLeft;
  const chartInnerHeight = chartBottom - chartTop;

  const pointToXY = useCallback((point: FanCurvePoint) => {
    const xRatio = (clamp(point.temperature, MIN_TEMP, MAX_TEMP) - MIN_TEMP) / (MAX_TEMP - MIN_TEMP);
    const yRatio = (clamp(point.rpm, MIN_RPM, MAX_RPM) - MIN_RPM) / (MAX_RPM - MIN_RPM);
    return {
      x: chartLeft + xRatio * chartInnerWidth,
      y: chartBottom - yRatio * chartInnerHeight,
    };
  }, [chartBottom, chartInnerHeight, chartInnerWidth, chartLeft]);

  const pointerYToRpm = useCallback((clientY: number) => {
    const rect = svgRef.current?.getBoundingClientRect();
    if (!rect) return MIN_RPM;
    const viewY = ((clientY - rect.top) / rect.height) * CHART_HEIGHT;
    const yRatio = 1 - ((viewY - chartTop) / chartInnerHeight);
    return Math.round(clamp(MIN_RPM + yRatio * (MAX_RPM - MIN_RPM), MIN_RPM, MAX_RPM));
  }, [chartInnerHeight, chartTop]);

  const updateFromPointer = useCallback((index: number, clientY: number) => {
    onPointRpmChange(index, pointerYToRpm(clientY));
  }, [onPointRpmChange, pointerYToRpm]);

  const handlePointPointerDown = useCallback((event: ReactPointerEvent<SVGCircleElement>, index: number) => {
    event.preventDefault();
    event.stopPropagation();
    event.currentTarget.setPointerCapture(event.pointerId);
    setActiveIndex(index);
    updateFromPointer(index, event.clientY);
  }, [updateFromPointer]);

  const handlePointerMove = useCallback((event: ReactPointerEvent<SVGSVGElement>) => {
    if (activeIndex === null) return;
    event.preventDefault();
    updateFromPointer(activeIndex, event.clientY);
  }, [activeIndex, updateFromPointer]);

  const stopDrag = useCallback(() => {
    setActiveIndex(null);
  }, []);

  const points = curve.map((point) => ({ point, ...pointToXY(point) }));
  const polylinePoints = points.map(({ x, y }) => `${x},${y}`).join(' ');
  const activePoint = activePointIndex === null ? null : points[activePointIndex];

  return (
    <div className="px-4 py-4">
      <div className="min-w-0 rounded-xl border border-border bg-background p-3">
        <div className="mb-3 flex min-h-6 items-center justify-between gap-3 text-xs text-muted-foreground">
          <span>{t('omenPage.curve.preview')}</span>
          {dirty && <Badge variant="warning">{t('omenPage.curve.localDraft')}</Badge>}
        </div>
        <svg
          ref={svgRef}
          className="block h-80 w-full touch-none select-none overflow-visible"
          viewBox={`0 0 ${CHART_WIDTH} ${CHART_HEIGHT}`}
          role="img"
          aria-label={t('omenPage.curve.title')}
          onPointerMove={handlePointerMove}
          onPointerUp={stopDrag}
          onPointerCancel={stopDrag}
        >
          <rect x={chartLeft} y={chartTop} width={chartInnerWidth} height={chartInnerHeight} rx={8} fill="var(--muted)" opacity={0.22} />
          {RPM_TICKS.map((tick) => {
            const y = chartBottom - ((tick - MIN_RPM) / (MAX_RPM - MIN_RPM)) * chartInnerHeight;
            return (
              <g key={`rpm-${tick}`}>
                <line x1={chartLeft} x2={chartRight} y1={y} y2={y} stroke="var(--border)" strokeOpacity={0.55} />
                <text x={chartLeft - 10} y={y + 4} textAnchor="end" className="fill-muted-foreground text-[11px]">
                  {tick}
                </text>
              </g>
            );
          })}
          {TEMP_TICKS.map((tick) => {
            const x = chartLeft + ((tick - MIN_TEMP) / (MAX_TEMP - MIN_TEMP)) * chartInnerWidth;
            return (
              <g key={`temp-${tick}`}>
                <line x1={x} x2={x} y1={chartTop} y2={chartBottom} stroke="var(--border)" strokeOpacity={0.35} />
                <text x={x} y={chartBottom + 24} textAnchor="middle" className="fill-muted-foreground text-[11px]">
                  {formatTemp(tick)}
                </text>
              </g>
            );
          })}
          <text x={chartLeft - 12} y={chartTop - 10} textAnchor="end" className="fill-muted-foreground text-[11px]">RPM</text>
          <polyline points={polylinePoints} fill="none" stroke="var(--primary)" strokeWidth={3} strokeLinecap="round" strokeLinejoin="round" />
          {points.map(({ point, x, y }, index) => {
            const isActive = activePointIndex === index;
            return (
              <g
                key={`${point.temperature}-${index}`}
                onMouseEnter={() => setHoverIndex(index)}
                onMouseLeave={() => setHoverIndex((current) => (current === index ? null : current))}
              >
                <circle
                  cx={x}
                  cy={y}
                  r={16}
                  fill="transparent"
                  className="cursor-ns-resize"
                  onPointerDown={(event) => handlePointPointerDown(event, index)}
                />
                <circle
                  cx={x}
                  cy={y}
                  r={isActive ? 7 : 6}
                  fill={isActive ? 'var(--primary)' : 'var(--background)'}
                  stroke="var(--primary)"
                  strokeWidth={isActive ? 3 : 2}
                  className="cursor-ns-resize transition-[r,stroke-width] duration-150"
                  onPointerDown={(event) => handlePointPointerDown(event, index)}
                />
              </g>
            );
          })}
          {activePoint && (
            <g pointerEvents="none">
              <rect
                x={clamp(activePoint.x - 56, chartLeft, chartRight - 112)}
                y={clamp(activePoint.y - 48, chartTop, chartBottom - 34)}
                width={112}
                height={34}
                rx={6}
                fill="var(--primary)"
                opacity={0.96}
              />
              <text
                x={clamp(activePoint.x, chartLeft + 56, chartRight - 56)}
                y={clamp(activePoint.y - 28, chartTop + 20, chartBottom - 14)}
                textAnchor="middle"
                fill="white"
                fontSize={12}
                fontWeight={600}
              >
                {formatTemp(activePoint.point.temperature)} 路 {formatTargetRpm(activePoint.point.rpm)}
              </text>
            </g>
          )}
        </svg>
      </div>
    </div>
  );
}

function OmenUnavailableState({
  title,
  description,
  plugin,
  busy,
  onRefresh,
  onTogglePlugin,
}: {
  title: string;
  description: string;
  plugin?: PluginInfo;
  busy: boolean;
  onRefresh: () => void;
  onTogglePlugin: () => void;
}) {
  const { t } = useTranslation();
  const diagnosticText = sanitizeDiagnosticText(plugin?.lastError);

  return (
    <div className="mx-auto flex min-h-[58vh] max-w-2xl flex-col items-center justify-center px-4 py-10 text-center">
      <div className="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl border border-border bg-muted text-muted-foreground">
        <Laptop className="h-7 w-7" />
      </div>
      <h1 className="text-2xl font-semibold tracking-normal text-foreground">{title}</h1>
      <p className="mt-3 max-w-xl text-sm leading-7 text-muted-foreground">{description}</p>
      {plugin?.lastError && (
        <div className="mt-5 max-h-24 max-w-xl overflow-hidden rounded-xl border border-destructive/25 bg-destructive/5 px-4 py-3 text-left text-xs leading-6 text-destructive">
          <div className="font-medium">
            {OMEN_UNSUPPORTED_SUMMARY}
          </div>
          {diagnosticText && (
            <div className="mt-1 break-words text-destructive/80">
              璇婃柇鎽樿锛歿diagnosticText}
            </div>
          )}
        </div>
      )}
      <div className="mt-6 flex flex-wrap justify-center gap-2">
        {plugin && (
          <Button
            variant={plugin.running ? 'secondary' : 'primary'}
            loading={busy}
            onClick={onTogglePlugin}
            icon={<Power className="h-4 w-4" />}
          >
            {t(plugin.running ? 'omenPage.actions.disablePlugin' : 'omenPage.actions.enablePlugin')}
          </Button>
        )}
        <Button variant="outline" loading={busy} onClick={onRefresh} icon={<RefreshCw className="h-4 w-4" />}>
          {t('omenPage.actions.refresh')}
        </Button>
      </div>
    </div>
  );
}

export default function OmenPage() {
  const { t } = useTranslation();
  const {
    config,
    availablePlugins,
    omenInstalled,
    omenSupported,
    omenFanData,
    omenFanCurve,
    refreshPluginSnapshot,
    refreshOmenSnapshot,
  } = useAppStore(useShallow((state) => ({
    config: state.config,
    availablePlugins: state.availablePlugins,
    omenInstalled: state.omenInstalled,
    omenSupported: state.omenSupported,
    omenFanData: state.omenFanData,
    omenFanCurve: state.omenFanCurve,
    refreshPluginSnapshot: state.refreshPluginSnapshot,
    refreshOmenSnapshot: state.refreshOmenSnapshot,
  })));
  const plugin = availablePlugins.find((item) => item.id === OMEN_PLUGIN_ID);
  const omenConfig = (config as any)?.omen || {};
  const [curveDraft, setCurveDraft] = useState<FanCurvePoint[]>(() => sanitizeCurve(omenFanCurve));
  const [biasDraft, setBiasDraft] = useState(() => normalizeBiasPercent(omenConfig.fanBias));
  const [busyKey, setBusyKey] = useState<string | null>(null);
  const [activeSection, setActiveSection] = useState<OmenSection>('mode');
  const [selectedMode, setSelectedMode] = useState<OmenMode>('balanced');
  const [powerProfile, setPowerProfile] = useState<OmenPowerProfile>('balanced');
  const [mockStatus, setMockStatus] = useState<OmenFanStatus | null>(null);
  const [mockConnected, setMockConnected] = useState(false);
  const mockPort = DEFAULT_OMEN_MOCK_PORT;

  const persistedCurve = useMemo(
    () => sanitizeCurve(omenFanCurve.length > 0 ? omenFanCurve : omenConfig.fanCurve),
    [omenConfig.fanCurve, omenFanCurve],
  );
  const curveDirty = !curvesEqual(curveDraft, persistedCurve);
  const jointLearning = Boolean(omenConfig.jointLearning);
  const displayFanData = mockStatus ?? omenFanData;
  const selectedModeLabel = findModeLabel(selectedMode);
  const dataSourceLabel = mockStatus ? `Mock ${mockPort}` : (omenFanData?.rpmEstimated ? '璋冭瘯棰勮' : '鎻掍欢鏁版嵁');

  useEffect(() => {
    if (!curveDirty) {
      setCurveDraft(persistedCurve);
    }
  }, [curveDirty, persistedCurve]);

  useEffect(() => {
    setBiasDraft(normalizeBiasPercent(omenConfig.fanBias));
  }, [omenConfig.fanBias]);

  useEffect(() => {
    if (mockStatus?.mode && isOmenMode(mockStatus.mode)) {
      setSelectedMode(mockStatus.mode);
    }
  }, [mockStatus?.mode]);

  useEffect(() => {
    if (mockStatus?.powerMode && isOmenPowerProfile(mockStatus.powerMode)) {
      setPowerProfile(mockStatus.powerMode);
    }
  }, [mockStatus?.powerMode]);

  const run = async (key: string, task: () => Promise<void>) => {
    setBusyKey(key);
    try {
      await task();
    } catch (error) {
      toast.error(t('omenPage.toasts.actionFailed', { error: getErrorMessage(error) }));
    } finally {
      setBusyKey(null);
    }
  };

  const refreshMockStatus = useCallback(async () => {
    const health = await apiService.getOmenMockHealth(mockPort);
    if (!health?.ok) {
      setMockConnected(false);
      setMockStatus(null);
      return null;
    }

    const status = await apiService.getOmenMockStatus(mockPort);
    if (!status) {
      setMockConnected(false);
      setMockStatus(null);
      return null;
    }

    setMockConnected(true);
    setMockStatus(status);
    return status;
  }, [mockPort]);

  useEffect(() => {
    void refreshMockStatus();
  }, [refreshMockStatus]);

  const refreshAll = () => run('refresh', async () => {
    await refreshPluginSnapshot();
    if (omenSupported) {
      await refreshOmenSnapshot();
    }
    await refreshMockStatus();
  });

  const togglePlugin = () => run('plugin', async () => {
    if (plugin?.running) {
      await apiService.disablePlugin(OMEN_PLUGIN_ID);
    } else {
      await apiService.enablePlugin(OMEN_PLUGIN_ID);
    }
    await refreshPluginSnapshot();
    if (!plugin?.running && useAppStore.getState().omenSupported) {
      await refreshOmenSnapshot();
    }
    toast.success(t(plugin?.running ? 'omenPage.toasts.pluginDisabled' : 'omenPage.toasts.pluginEnabled'));
  });

  const updateCurvePointRpm = useCallback((index: number, rpm: number) => {
    setCurveDraft((current) => current.map((point, pointIndex) => (
      pointIndex === index
        ? {
          temperature: point.temperature,
          rpm: Math.round(clamp(rpm, MIN_RPM, MAX_RPM)),
        }
        : point
    )));
  }, []);

  const applyCurve = () => run('curve', async () => {
    const nextCurve = sanitizeCurve(curveDraft);
    await apiService.setOmenFanCurve(nextCurve);
    setCurveDraft(nextCurve);
    await refreshOmenSnapshot();
    if (mockConnected) {
      const cpuTarget = calculateTargetRpmForCurve(nextCurve, displayFanData?.cpuTemp);
      const gpuTarget = calculateTargetRpmForCurve(nextCurve, displayFanData?.gpuTemp);
      const status = await apiService.setOmenMockFanTargets(cpuTarget, gpuTarget, mockPort);
      if (status) {
        setMockConnected(true);
        setMockStatus(status);
      }
    }
    toast.success(t('omenPage.toasts.curveApplied'));
  });

  const applyBias = () => run('bias', async () => {
    await apiService.setOmenFanBias(biasDraft);
    await refreshOmenSnapshot();
    toast.success(t('omenPage.toasts.biasApplied'));
  });

  const setJointLearning = (enabled: boolean) => run('jointLearning', async () => {
    await apiService.setOmenJointLearning(enabled);
    await refreshOmenSnapshot();
    toast.success(t(enabled ? 'omenPage.toasts.jointLearningEnabled' : 'omenPage.toasts.jointLearningDisabled'));
  });

  const selectMode = (mode: OmenMode) => run(`mode-${mode}`, async () => {
    setSelectedMode(mode);
    if (mode === 'custom') {
      setActiveSection('fan');
    }
    if (mockConnected) {
      const status = await apiService.setOmenMockMode(mode, mockPort);
      if (status) {
        setMockConnected(true);
        setMockStatus(status);
      }
    }
  });

  const selectPowerProfile = (profile: OmenPowerProfile, watts: string) => run(`power-${profile}`, async () => {
    setPowerProfile(profile);
    if (mockConnected) {
      const status = await apiService.setOmenMockPower(profile, parsePowerWatts(watts), mockPort);
      if (status) {
        setMockConnected(true);
        setMockStatus(status);
      }
    }
  });

  if (!omenInstalled) {
    return (
      <OmenUnavailableState
        title={t('omenPage.states.notInstalled.title')}
        description={t('omenPage.states.notInstalled.description')}
        busy={busyKey !== null}
        onRefresh={refreshAll}
        onTogglePlugin={togglePlugin}
      />
    );
  }

  if (!omenSupported && !mockConnected) {
    return (
      <OmenUnavailableState
        title={t('omenPage.states.notSupported.title')}
        description={OMEN_UNSUPPORTED_SUMMARY}
        plugin={plugin}
        busy={busyKey !== null}
        onRefresh={refreshAll}
        onTogglePlugin={togglePlugin}
      />
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-4 px-4 py-4 lg:px-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-xl font-semibold tracking-normal text-foreground">{t('omenPage.title')}</h1>
            <Badge variant={plugin?.running === false ? 'warning' : 'success'}>
              {t(plugin?.running === false ? 'omenPage.status.stopped' : 'omenPage.status.running')}
            </Badge>
            <Badge variant={mockConnected ? 'success' : 'warning'}>
              Mock {mockPort} {mockConnected ? '宸茶繛鎺? : '绂荤嚎'}
            </Badge>
            {displayFanData?.rpmEstimated && <Badge variant="info">{t('omenPage.status.estimated')}</Badge>}
          </div>
          <p className="mt-1 text-sm leading-6 text-muted-foreground">{t('omenPage.description')}</p>
        </div>
        <div className="flex shrink-0 flex-wrap gap-2">
          <Button variant="outline" loading={busyKey === 'refresh'} onClick={refreshAll} icon={<RefreshCw className="h-4 w-4" />}>
            {t('omenPage.actions.refresh')}
          </Button>
          <Button
            variant={plugin?.running === false ? 'primary' : 'secondary'}
            loading={busyKey === 'plugin'}
            onClick={togglePlugin}
            icon={<Power className="h-4 w-4" />}
          >
            {t(plugin?.running === false ? 'omenPage.actions.enablePlugin' : 'omenPage.actions.disablePlugin')}
          </Button>
        </div>
      </div>

      <div className="flex flex-wrap gap-2 rounded-lg border border-border bg-card p-1">
        {SECTION_OPTIONS.map((section) => {
          const active = activeSection === section.id;
          return (
            <button
              key={section.id}
              type="button"
              className={`min-h-9 cursor-pointer rounded-md px-3 text-sm font-medium transition-colors ${active ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted hover:text-foreground'}`}
              onClick={() => setActiveSection(section.id)}
            >
              {section.label}
            </button>
          );
        })}
      </div>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatusMetric label={t('omenPage.metrics.cpuFan')} value={formatRpm(displayFanData?.cpuRpm)} detail={formatFanLevelDetail(displayFanData?.cpuLevel, displayFanData?.rpmEstimated)} />
        <StatusMetric label={t('omenPage.metrics.gpuFan')} value={formatRpm(displayFanData?.gpuRpm)} detail={formatFanLevelDetail(displayFanData?.gpuLevel, displayFanData?.rpmEstimated)} />
        <StatusMetric label={t('omenPage.metrics.cpuTemp')} value={formatTemp(displayFanData?.cpuTemp)} detail={`妯″紡 ${selectedModeLabel} 路 ${dataSourceLabel}`} />
        <StatusMetric label={t('omenPage.metrics.gpuTemp')} value={formatTemp(displayFanData?.gpuTemp)} detail={t('omenPage.metrics.updatedAt', { time: formatUpdatedAt(displayFanData?.lastUpdated) })} />
      </div>

      {activeSection === 'mode' && (
        <Card padding="md">
          <div className="flex flex-col gap-2 border-b border-border/60 pb-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 className="text-base font-semibold text-foreground">妯″紡鍒囨崲</h2>
              <p className="mt-1 text-sm text-muted-foreground">褰撳墠涓哄墠绔皟璇曢瑙堬紝涓嶅啓鍏ョ‖浠躲€?/p>
            </div>
            <Badge variant="info">璋冭瘯棰勮</Badge>
          </div>
          <div className="mt-4 grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
            {MODE_OPTIONS.map((mode) => {
              const active = selectedMode === mode.id;
              return (
                <button
                  key={mode.id}
                  type="button"
                  disabled={busyKey !== null}
                  className={`min-h-28 cursor-pointer rounded-lg border px-4 py-3 text-left transition-colors ${active ? 'border-primary bg-primary/10 text-foreground' : 'border-border bg-background text-foreground hover:border-primary/40 hover:bg-muted/40'}`}
                  onClick={() => selectMode(mode.id)}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-base font-semibold">{mode.label}</span>
                    {active && <Badge variant="success">宸查€夋嫨</Badge>}
                  </div>
                  <p className="mt-2 text-sm leading-6 text-muted-foreground">{mode.description}</p>
                </button>
              );
            })}
          </div>
        </Card>
      )}

      {activeSection === 'power' && (
        <Card padding="md">
          <div className="flex flex-col gap-2 border-b border-border/60 pb-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 className="text-base font-semibold text-foreground">鍔熻€楄皟鏁?/h2>
              <p className="mt-1 text-sm text-muted-foreground">鏈〉浠呴瑙堟。浣嶏紝寰呯‖浠舵帴鍏ャ€?/p>
            </div>
            <Badge variant="info">璋冭瘯棰勮</Badge>
          </div>
          <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-3">
            {POWER_OPTIONS.map((profile) => {
              const active = powerProfile === profile.id;
              return (
                <button
                  key={profile.id}
                  type="button"
                  disabled={busyKey !== null}
                  className={`min-h-24 cursor-pointer rounded-lg border px-4 py-3 text-left transition-colors ${active ? 'border-primary bg-primary/10 text-foreground' : 'border-border bg-background hover:border-primary/40 hover:bg-muted/40'}`}
                  onClick={() => selectPowerProfile(profile.id, profile.watts)}
                >
                  <div className="text-sm font-medium text-muted-foreground">{profile.label}</div>
                  <div className="mt-2 text-2xl font-semibold tracking-normal text-foreground">{profile.watts}</div>
                </button>
              );
            })}
          </div>
        </Card>
      )}

      {activeSection === 'fan' && selectedMode !== 'custom' && (
        <Card padding="md">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 className="text-base font-semibold text-foreground">鑷畾涔夐鎵?/h2>
              <p className="mt-1 text-sm text-muted-foreground">褰撳墠涓簕selectedModeLabel}妯″紡锛岄鎵囨洸绾夸粎鍦ㄨ嚜瀹氫箟妯″紡涓嬪紑鏀俱€?/p>
            </div>
            <Button
              variant="primary"
              disabled={busyKey !== null}
              onClick={() => selectMode('custom')}
              icon={<SlidersHorizontal className="h-4 w-4" />}
            >
              鍒囨崲鍒拌嚜瀹氫箟
            </Button>
          </div>
        </Card>
      )}

      {activeSection === 'fan' && selectedMode === 'custom' && (
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1fr)_22rem]">
          <Card padding="none" className="overflow-hidden">
            <div className="flex flex-col gap-3 border-b border-border/60 px-4 py-4 sm:flex-row sm:items-start sm:justify-between">
              <div className="flex min-w-0 gap-3">
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                  <SlidersHorizontal className="h-4.5 w-4.5" />
                </div>
                <div className="min-w-0">
                  <h2 className="text-base font-semibold text-foreground">{t('omenPage.curve.title')}</h2>
                  <p className="mt-1 text-sm leading-6 text-muted-foreground">{t('omenPage.curve.description')}</p>
                </div>
              </div>
              <div className="flex shrink-0 flex-wrap gap-2">
                <Button variant="outline" size="sm" disabled={!curveDirty || busyKey !== null} onClick={() => setCurveDraft(persistedCurve)} icon={<RotateCcw className="h-3.5 w-3.5" />}>
                  {t('omenPage.actions.revert')}
                </Button>
                <Button size="sm" disabled={!curveDirty} loading={busyKey === 'curve'} onClick={applyCurve} icon={<Save className="h-3.5 w-3.5" />}>
                  {t('common.actions.apply')}
                </Button>
              </div>
            </div>

            <OmenCurveEditor
              curve={curveDraft}
              dirty={curveDirty}
              onPointRpmChange={updateCurvePointRpm}
            />
          </Card>

          <div className="space-y-4">
            <Card padding="md">
              <div className="flex items-start gap-3">
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                  <Gauge className="h-4.5 w-4.5" />
                </div>
                <div className="min-w-0 flex-1">
                  <h2 className="text-base font-semibold text-foreground">{t('omenPage.bias.title')}</h2>
                  <p className="mt-1 text-sm leading-6 text-muted-foreground">{t('omenPage.bias.description')}</p>
                </div>
              </div>
              <div className="mt-4 space-y-3">
                <Slider
                  value={biasDraft}
                  onChange={setBiasDraft}
                  min={-100}
                  max={100}
                  step={1}
                  label={t('omenPage.bias.label')}
                  valueFormatter={(value) => `${value > 0 ? '+' : ''}${value}`}
                />
                <Button className="w-full" loading={busyKey === 'bias'} onClick={applyBias} icon={<Save className="h-4 w-4" />}>
                  {t('common.actions.apply')}
                </Button>
              </div>
            </Card>

            <Card padding="md">
              <div className="flex items-start gap-3">
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                  <Activity className="h-4.5 w-4.5" />
                </div>
                <div className="min-w-0 flex-1">
                  <h2 className="text-base font-semibold text-foreground">{t('omenPage.jointLearning.title')}</h2>
                  <p className="mt-1 text-sm leading-6 text-muted-foreground">{t('omenPage.jointLearning.description')}</p>
                </div>
                <ToggleSwitch
                  enabled={jointLearning}
                  onChange={setJointLearning}
                  loading={busyKey === 'jointLearning'}
                  size="sm"
                  color="green"
                  srLabel={t('omenPage.jointLearning.title')}
                />
              </div>
            </Card>
          </div>
        </div>
      )}
    </div>
  );
}

