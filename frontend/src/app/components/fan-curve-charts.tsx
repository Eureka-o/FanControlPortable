'use client';

// FanCurve 的三张 recharts 图表（曲线编辑器主图 + 历史温度/转速图 + 历史功率图）全部集中在这里，
// 与主包隔离成独立 chunk：主包（FanCurve.tsx）不再静态依赖 recharts，首帧省掉它的解析与执行。
// 组件与常量从 FanCurve.tsx 原样搬入 —— props、memo 结构、渲染输出都保持不变。
// 唯一的契约调整：renderStatistics 改为返回纯数据描述符（HistoryStatisticsMarker），
// ReferenceLine/ReferenceDot 元素改由本文件创建，主包连这两个 recharts 组件的运行时引用都不需要。

import React, { memo, useCallback, useMemo } from 'react';
import { Area, ComposedChart, Line, XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip, ResponsiveContainer, ReferenceLine, ReferenceArea, ReferenceDot } from 'recharts';
import { useTranslation } from 'react-i18next';
import { useLocale } from '../lib/i18n';
import type { HistorySeriesKey } from '../lib/temperature-history';
import { FAN_CURVE_AXIS_MIN_SPEED, TIMELINE_EVENT_COLORS, formatPowerValue, formatSpeedValue } from './fan-curve-chart-shared';

const CURVE_CHART_MARGIN = { top: 20, right: 30, left: 20, bottom: 20 };
const HISTORY_CHART_MARGIN = { top: 12, right: 16, left: 4, bottom: 8 };
const CHART_GRID_PROPS = { strokeDasharray: '3 3', stroke: 'var(--chart-grid)' };
const CHART_AXIS_LINE = { stroke: 'var(--chart-axis)' };
const CURVE_CHART_TICK_X = { fill: 'var(--chart-tick)', fontSize: 10 };
const CURVE_CHART_TICK_Y = { fill: 'var(--chart-tick)', fontSize: 11 };
const HISTORY_CHART_TICK = { fill: 'var(--chart-tick)', fontSize: 11 };
const CURVE_TOOLTIP_CONTENT_STYLE = { backgroundColor: 'var(--chart-tooltip-bg)', border: '1px solid', borderColor: 'var(--chart-tooltip-border)', borderRadius: '12px', boxShadow: 'var(--chart-tooltip-shadow)', padding: '8px 12px', fontSize: '12px', color: 'var(--chart-tooltip-text)' };
const CURVE_TOOLTIP_LABEL_STYLE = { color: 'var(--chart-tooltip-text)', fontWeight: 600 };
const CURVE_TOOLTIP_ITEM_STYLE = { color: 'var(--chart-tooltip-text)' };

const formatTempAxisTick = (value: number) => `${Number(value)}°C`;
const formatPowerAxisTick = (value: number) => (Number(value) === 0 ? '0 W' : `${formatPowerValue(Number(value))} W`);

function formatHistoryTime(timestamp: number, locale: string) {
  return new Date(timestamp).toLocaleTimeString(locale, {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
  });
}

export type CurveChartDatum = { temperature: number; rpm: number; coupledRpm: number; index: number };

export type HistoryChartPoint = {
  timestamp: number;
  cpuTemp?: number;
  gpuTemp?: number;
  fanRpm?: number;
  cpuPowerWatts?: number;
  gpuPowerWatts?: number;
  totalPowerWatts?: number;
};

export type HistorySeriesMeta = {
  key: HistorySeriesKey;
  label: string;
  color: string;
  dataKey: 'cpuTemp' | 'gpuTemp' | 'fanRpm' | 'cpuPowerWatts' | 'gpuPowerWatts' | 'totalPowerWatts';
  axisId: 'temp' | 'fan' | 'power';
};

export type HistoryStatisticsValue = { min: number; max: number; average: number; minTimestamps: number[]; maxTimestamps: number[] };

export type HistoryStatisticsData = {
  values: Partial<Record<HistorySeriesKey, HistoryStatisticsValue>>;
  thermalSeries: HistorySeriesMeta[];
  powerSeries: HistorySeriesMeta[];
};

export type HistoryTimelineLayoutItem = {
  event: { timestamp: number; type: 'disconnect' | 'reconnect' | 'resume' | 'profile' };
  row: number;
  anchorEnd: boolean;
};

export type HistoryChartZoomHandlers = {
  onMouseDown: (state: { activeLabel?: string | number } | null) => void;
  onMouseMove: (state: { activeLabel?: string | number } | null) => void;
  onMouseUp: () => void;
  onDoubleClick: () => void;
};

// renderStatistics 返回的统计标线描述符：字段与原先在 FanCurve 里创建的
// ReferenceLine/ReferenceDot JSX 一一对应（key/y/segment/stroke/文案等全部保留）。
export type HistoryStatisticsMarker =
  | {
      kind: 'line';
      key: string;
      yAxisId: 'temp' | 'fan' | 'power';
      y?: number;
      segment?: readonly [{ x: number; y: number }, { x: number; y: number }];
      stroke: string;
      strokeOpacity: number;
      statKey: 'max' | 'min' | 'average';
      labelText: string;
    }
  | {
      kind: 'dot';
      key: string;
      yAxisId: 'temp' | 'fan' | 'power';
      x: number;
      y: number;
      fill: string;
    };

// 把描述符渲染成 recharts 参考元素（props 与搬移前逐字段一致，key 保持同一套）。
function renderStatisticsMarkers(markers: HistoryStatisticsMarker[] | null): React.ReactNode {
  if (!markers) {
    return null;
  }
  return markers.map((marker) => {
    if (marker.kind === 'dot') {
      return (
        <ReferenceDot
          key={marker.key}
          x={marker.x}
          y={marker.y}
          yAxisId={marker.yAxisId}
          r={4.5}
          fill={marker.fill}
          stroke="var(--card)"
          strokeWidth={2}
        />
      );
    }
    return (
      <ReferenceLine
        key={marker.key}
        yAxisId={marker.yAxisId}
        y={marker.y}
        segment={marker.segment}
        stroke={marker.stroke}
        strokeOpacity={marker.strokeOpacity}
        strokeDasharray="5 4"
        strokeWidth={1.5}
        label={({ viewBox }: { viewBox?: { x?: number; y?: number; width?: number } }) => (
          <text
            x={(viewBox?.x ?? 0) + (viewBox?.width ?? 0)}
            y={viewBox?.y ?? 0}
            dx={-8}
            dy={marker.statKey === 'min' ? 14 : -6}
            textAnchor="end"
            fill={marker.stroke}
            stroke="var(--chart-stat-label-halo)"
            strokeWidth={3}
            paintOrder="stroke"
            strokeLinejoin="round"
            fontSize={10}
            fontWeight={600}
          >
            {marker.labelText}
          </text>
        )}
      />
    );
  });
}

/* ── 曲线编辑器主图 ── */

export const CurveEditorChart = memo(function CurveEditorChart({
  data,
  temperatureRange,
  speedMax,
  speedTicks,
  speedUnitSuffix,
  showCoupledCurve,
  learningActive = false,
  learningTargetTemp,
  learningComfortLow,
  customDot,
}: {
  data: CurveChartDatum[];
  temperatureRange: { min: number; max: number; ticks: number[] };
  speedMax: number;
  speedTicks: number[];
  speedUnitSuffix: string;
  showCoupledCurve: boolean;
  learningActive?: boolean;
  learningTargetTemp?: number;
  learningComfortLow?: number;
  customDot: (props: any) => React.ReactElement<SVGElement>;
}) {
  const { t } = useTranslation();
  const yAxisLabel = useMemo(() => t('fanCurve.chart.axes.rpm').replace('%', speedUnitSuffix), [speedUnitSuffix, t]);
  const comfortBandBounds = useMemo(() => {
    if (!learningActive || learningTargetTemp === undefined || learningComfortLow === undefined) {
      return null;
    }
    const clampedHigh = Math.min(temperatureRange.max, learningTargetTemp);
    const clampedLow = Math.max(temperatureRange.min, Math.min(clampedHigh, learningComfortLow));
    if (clampedHigh <= clampedLow) {
      return null;
    }
    return { low: clampedLow, high: clampedHigh };
  }, [learningActive, learningComfortLow, learningTargetTemp, temperatureRange.max, temperatureRange.min]);

  const renderTooltipContent = useCallback(({ active, payload }: { active?: boolean; payload?: readonly any[] }) => {
    if (!active || !payload || payload.length === 0) {
      return null;
    }
    const datum = payload[0]?.payload as CurveChartDatum | undefined;
    if (!datum) {
      return null;
    }
    const delta = datum.coupledRpm - datum.rpm;
    const deltaText = delta > 0
      ? `+${formatSpeedValue(delta)}${speedUnitSuffix}`
      : `${formatSpeedValue(delta)}${speedUnitSuffix}`;

    return (
      <div className="rounded-xl border px-3 py-2 text-xs shadow-lg" style={CURVE_TOOLTIP_CONTENT_STYLE}>
        <div className="mb-1.5 font-semibold" style={CURVE_TOOLTIP_LABEL_STYLE}>
          {t('fanCurve.chart.temperatureLabel', { temperature: datum.temperature })}
        </div>
        <div className="space-y-1 tabular-nums">
          <div className="flex items-center justify-between gap-4">
            <span className="inline-flex items-center gap-1.5 text-muted-foreground">
              <span className="h-2 w-2 rounded-full" style={{ backgroundColor: 'var(--chart-primary)' }} />
              {t('fanCurve.chart.series.base')}
            </span>
            <span className="font-semibold">{formatSpeedValue(datum.rpm)}{speedUnitSuffix}</span>
          </div>
          {showCoupledCurve && (
            <>
              <div className="flex items-center justify-between gap-4">
                <span className="inline-flex items-center gap-1.5 text-muted-foreground">
                  <span className="h-2 w-2 rounded-full bg-amber-500" />
                  {t('fanCurve.chart.series.learned')}
                </span>
                <span className="font-semibold">{formatSpeedValue(datum.coupledRpm)}{speedUnitSuffix}</span>
              </div>
              {delta !== 0 && (
                <div className="flex items-center justify-between gap-4 border-t border-border/40 pt-1 text-[11px]">
                  <span className="text-muted-foreground">{t('fanCurve.chart.offsetDelta', { delta: '' }).trim()}</span>
                  <span className={delta > 0 ? 'font-semibold text-amber-500' : 'font-semibold text-sky-500'}>
                    {deltaText}
                  </span>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    );
  }, [showCoupledCurve, speedUnitSuffix, t]);

  return (
    <ResponsiveContainer width="100%" height="100%" debounce={80} initialDimension={{ width: 960, height: 384 }}>
      <ComposedChart data={data} margin={CURVE_CHART_MARGIN}>
        <defs>
          <linearGradient id="curve-editor-area-grad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--chart-primary)" stopOpacity="var(--chart-area-opacity-start)" />
            <stop offset="100%" stopColor="var(--chart-primary)" stopOpacity="var(--chart-area-opacity-end)" />
          </linearGradient>
        </defs>
        <CartesianGrid {...CHART_GRID_PROPS} />
        {comfortBandBounds && (
          <ReferenceArea
            x1={comfortBandBounds.low}
            x2={comfortBandBounds.high}
            fill="#f59e0b"
            fillOpacity={0.08}
            ifOverflow="hidden"
          />
        )}
        <XAxis dataKey="temperature" type="number" domain={[temperatureRange.min, temperatureRange.max]} ticks={temperatureRange.ticks} interval={0} minTickGap={0} tickLine={false} axisLine={CHART_AXIS_LINE} tick={CURVE_CHART_TICK_X} label={{ value: t('fanCurve.chart.axes.temperature'), position: 'insideBottom', offset: -10, fill: 'var(--chart-tick)', fontSize: 12 }} />
        {/* 纵轴从 0 起（与其它图表一致）；只是量程显示，不改数据。 */}
        <YAxis type="number" domain={[FAN_CURVE_AXIS_MIN_SPEED, speedMax]} ticks={speedTicks} tickLine={false} axisLine={CHART_AXIS_LINE} tick={CURVE_CHART_TICK_Y} label={{ value: yAxisLabel, angle: -90, position: 'insideLeft', fill: 'var(--chart-tick)', fontSize: 12 }} />
        <RechartsTooltip content={renderTooltipContent} />
        <Area type="monotone" dataKey="rpm" stroke="none" fill="url(#curve-editor-area-grad)" baseValue={FAN_CURVE_AXIS_MIN_SPEED} tooltipType="none" isAnimationActive={false} />
        <Line type="monotone" dataKey="rpm" stroke="var(--chart-primary)" strokeWidth={3} dot={customDot} activeDot={false} isAnimationActive={false} />
        {showCoupledCurve && <Line type="monotone" dataKey="coupledRpm" stroke="#f59e0b" strokeWidth={2} strokeDasharray="5 4" dot={false} activeDot={false} isAnimationActive={false} />}
      </ComposedChart>
    </ResponsiveContainer>
  );
});

/* ── 历史详情：温度/转速趋势图 ── */

export const HistoryThermalChart = memo(function HistoryThermalChart({
  data,
  timeDomain,
  monitorOnlyActive,
  speedRange,
  temperatureRange,
  temperatureTicks,
  visibleThermalSeries,
  timelineLayout,
  historyTimelineAxisId,
  historyStatistics,
  renderStatistics,
  zoomSelectionBounds,
  tooltipContent,
  speedUnitSuffix,
  onMouseDown,
  onMouseMove,
  onMouseUp,
  onDoubleClick,
}: {
  data: HistoryChartPoint[];
  timeDomain: [number, number];
  monitorOnlyActive: boolean;
  speedRange: { min: number; max: number; ticks: number[] };
  temperatureRange: { min: number; max: number; ticks: number[] };
  temperatureTicks: number[];
  visibleThermalSeries: HistorySeriesMeta[];
  timelineLayout: HistoryTimelineLayoutItem[];
  historyTimelineAxisId: 'temp' | 'fan';
  historyStatistics: HistoryStatisticsData;
  renderStatistics: (series: HistorySeriesMeta | undefined, yAxisId: 'temp' | 'fan' | 'power', showStatisticsForSingleSeries?: boolean) => HistoryStatisticsMarker[] | null;
  zoomSelectionBounds: readonly [number, number] | null;
  tooltipContent: (props: any) => React.ReactNode;
  speedUnitSuffix: string;
} & HistoryChartZoomHandlers) {
  const { t } = useTranslation();
  const { locale } = useLocale();
  const thermalSingleSeries = historyStatistics.thermalSeries.length === 1 ? historyStatistics.thermalSeries[0] : undefined;
  const formatTimeTick = useCallback((value: number) => formatHistoryTime(Number(value), locale), [locale]);
  const formatFanTick = useCallback((value: number) => `${formatSpeedValue(Number(value))}${speedUnitSuffix}`, [speedUnitSuffix]);

  return (
    <ResponsiveContainer width="100%" height="100%" debounce={80} initialDimension={{ width: 960, height: 288 }}>
      <ComposedChart
        data={data}
        syncId="historyTrend"
        margin={HISTORY_CHART_MARGIN}
        onMouseDown={onMouseDown}
        onMouseMove={onMouseMove}
        onMouseUp={onMouseUp}
        onMouseLeave={onMouseUp}
        onDoubleClick={onDoubleClick}
      >
        {thermalSingleSeries && (
          <defs>
            <linearGradient id={`history-thermal-area-${thermalSingleSeries.key}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={thermalSingleSeries.color} stopOpacity="var(--chart-area-opacity-start)" />
              <stop offset="100%" stopColor={thermalSingleSeries.color} stopOpacity="var(--chart-area-opacity-end)" />
            </linearGradient>
          </defs>
        )}
        <CartesianGrid {...CHART_GRID_PROPS} />
        <XAxis
          dataKey="timestamp"
          type="number"
          domain={timeDomain}
          allowDataOverflow
          tickFormatter={formatTimeTick}
          tickLine={false}
          minTickGap={24}
          axisLine={CHART_AXIS_LINE}
          tick={HISTORY_CHART_TICK}
        />
        {!monitorOnlyActive && (
          <YAxis
            yAxisId="fan"
            type="number"
            domain={[speedRange.min, speedRange.max]}
            ticks={speedRange.ticks}
            allowDataOverflow
            tickFormatter={formatFanTick}
            tickLine={false}
            axisLine={CHART_AXIS_LINE}
            tick={HISTORY_CHART_TICK}
            width={64}
          />
        )}
        <YAxis
          yAxisId="temp"
          orientation="right"
          type="number"
          domain={[temperatureRange.min, temperatureRange.max]}
          ticks={temperatureTicks}
          allowDataOverflow
          tickFormatter={formatTempAxisTick}
          tickLine={false}
          axisLine={CHART_AXIS_LINE}
          tick={HISTORY_CHART_TICK}
          width={44}
        />
        <RechartsTooltip content={tooltipContent} />
        {timelineLayout.map(({ event, row, anchorEnd }, index) => {
          const markerColor = TIMELINE_EVENT_COLORS[event.type];
          return (
            <ReferenceLine
              key={`${event.timestamp}-${event.type}-${index}`}
              x={event.timestamp}
              yAxisId={historyTimelineAxisId}
              stroke={markerColor}
              strokeDasharray="4 3"
              label={(labelProps: { viewBox?: { x?: number; y?: number } }) => {
                const x = labelProps.viewBox?.x ?? 0;
                const y = labelProps.viewBox?.y ?? 0;
                return (
                  <text
                    x={x}
                    y={y + 10 + row * 12}
                    dx={anchorEnd ? -5 : 5}
                    textAnchor={anchorEnd ? 'end' : 'start'}
                    fontSize={10}
                    fontWeight={500}
                    fill={markerColor}
                  >
                    {t(`fanCurve.history.events.${event.type}`)}
                  </text>
                );
              }}
            />
          );
        })}
        {thermalSingleSeries && (
          <Area
            yAxisId={thermalSingleSeries.axisId}
            type="monotone"
            dataKey={thermalSingleSeries.dataKey}
            stroke="none"
            fill={`url(#history-thermal-area-${thermalSingleSeries.key})`}
            baseValue="dataMin"
            tooltipType="none"
            isAnimationActive={false}
            connectNulls={false}
          />
        )}
        {visibleThermalSeries.map((series) => (
          <Line
            key={series.key}
            yAxisId={series.axisId}
            type="monotone"
            dataKey={series.dataKey}
            name={series.dataKey}
            stroke={series.color}
            strokeWidth={series.key === 'fan' ? 1.8 : 2}
            dot={false}
            activeDot={false}
            isAnimationActive={false}
            connectNulls={false}
          />
        ))}
        {historyStatistics.thermalSeries.map((series) => {
          const yAxisId = series.axisId === 'fan' ? 'fan' : 'temp';
          return (
            <React.Fragment key={`${series.key}-markers`}>
              {renderStatisticsMarkers(renderStatistics(series, yAxisId, historyStatistics.thermalSeries.length === 1))}
            </React.Fragment>
          );
        })}
        {zoomSelectionBounds && (
          <>
            <ReferenceArea
              yAxisId="temp"
              x1={zoomSelectionBounds[0]}
              x2={zoomSelectionBounds[1]}
              fill="var(--chart-primary)"
              fillOpacity={0.22}
              stroke="var(--chart-primary-active)"
              strokeOpacity={0.95}
              strokeWidth={2}
            />
            <ReferenceLine x={zoomSelectionBounds[0]} yAxisId="temp" stroke="var(--chart-primary-active)" strokeOpacity={0.95} strokeWidth={2} />
            <ReferenceLine x={zoomSelectionBounds[1]} yAxisId="temp" stroke="var(--chart-primary-active)" strokeOpacity={0.95} strokeWidth={2} />
          </>
        )}
      </ComposedChart>
    </ResponsiveContainer>
  );
});

/* ── 历史详情：功率趋势图 ── */

export const HistoryPowerChart = memo(function HistoryPowerChart({
  data,
  timeDomain,
  monitorOnlyActive,
  powerMax,
  visiblePowerSeries,
  historyStatistics,
  renderStatistics,
  zoomSelectionBounds,
  tooltipContent,
  onMouseDown,
  onMouseMove,
  onMouseUp,
  onDoubleClick,
}: {
  data: HistoryChartPoint[];
  timeDomain: [number, number];
  monitorOnlyActive: boolean;
  powerMax: number;
  visiblePowerSeries: HistorySeriesMeta[];
  historyStatistics: HistoryStatisticsData;
  renderStatistics: (series: HistorySeriesMeta | undefined, yAxisId: 'temp' | 'fan' | 'power', showStatisticsForSingleSeries?: boolean) => HistoryStatisticsMarker[] | null;
  zoomSelectionBounds: readonly [number, number] | null;
  tooltipContent: (props: any) => React.ReactNode;
} & HistoryChartZoomHandlers) {
  const { locale } = useLocale();
  const powerSingleSeries = historyStatistics.powerSeries.length === 1 ? historyStatistics.powerSeries[0] : undefined;
  const formatTimeTick = useCallback((value: number) => formatHistoryTime(Number(value), locale), [locale]);

  return (
    <ResponsiveContainer width="100%" height="100%" debounce={80} initialDimension={{ width: 960, height: 192 }}>
      <ComposedChart
        data={data}
        syncId="historyTrend"
        margin={HISTORY_CHART_MARGIN}
        onMouseDown={onMouseDown}
        onMouseMove={onMouseMove}
        onMouseUp={onMouseUp}
        onMouseLeave={onMouseUp}
        onDoubleClick={onDoubleClick}
      >
        {powerSingleSeries && (
          <defs>
            <linearGradient id={`history-power-area-${powerSingleSeries.key}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={powerSingleSeries.color} stopOpacity="var(--chart-area-opacity-start)" />
              <stop offset="100%" stopColor={powerSingleSeries.color} stopOpacity="var(--chart-area-opacity-end)" />
            </linearGradient>
          </defs>
        )}
        <CartesianGrid {...CHART_GRID_PROPS} />
        <XAxis
          dataKey="timestamp"
          type="number"
          domain={timeDomain}
          allowDataOverflow
          tickFormatter={formatTimeTick}
          tickLine={false}
          minTickGap={24}
          axisLine={CHART_AXIS_LINE}
          tick={HISTORY_CHART_TICK}
        />
        {monitorOnlyActive ? (
          <YAxis
            yAxisId="power"
            orientation="right"
            type="number"
            domain={[0, powerMax]}
            tickFormatter={formatPowerAxisTick}
            tickLine={false}
            axisLine={CHART_AXIS_LINE}
            tick={HISTORY_CHART_TICK}
            width={44}
          />
        ) : (
          <>
            <YAxis
              yAxisId="power"
              type="number"
              domain={[0, powerMax]}
              tickFormatter={formatPowerAxisTick}
              tickLine={false}
              axisLine={CHART_AXIS_LINE}
              tick={HISTORY_CHART_TICK}
              width={64}
            />
            <YAxis
              yAxisId="powerSpacer"
              orientation="right"
              type="number"
              domain={[0, powerMax]}
              tick={false}
              tickLine={false}
              axisLine={false}
              width={44}
            />
          </>
        )}
        <RechartsTooltip content={tooltipContent} />
        {powerSingleSeries && (
          <Area
            yAxisId="power"
            type="monotone"
            dataKey={powerSingleSeries.dataKey}
            stroke="none"
            fill={`url(#history-power-area-${powerSingleSeries.key})`}
            baseValue="dataMin"
            tooltipType="none"
            isAnimationActive={false}
            connectNulls={false}
          />
        )}
        {visiblePowerSeries.map((series) => (
          <Line
            key={series.key}
            yAxisId="power"
            type="monotone"
            dataKey={series.dataKey}
            name={series.dataKey}
            stroke={series.color}
            strokeWidth={2}
            dot={false}
            activeDot={false}
            isAnimationActive={false}
            connectNulls={false}
          />
        ))}
        {historyStatistics.powerSeries.map((series) => (
          <React.Fragment key={`${series.key}-markers`}>
            {renderStatisticsMarkers(renderStatistics(series, 'power', historyStatistics.powerSeries.length === 1))}
          </React.Fragment>
        ))}
        {zoomSelectionBounds && (
          <>
            <ReferenceArea
              yAxisId="power"
              x1={zoomSelectionBounds[0]}
              x2={zoomSelectionBounds[1]}
              fill="var(--chart-primary)"
              fillOpacity={0.22}
              stroke="var(--chart-primary-active)"
              strokeOpacity={0.95}
              strokeWidth={2}
            />
            <ReferenceLine x={zoomSelectionBounds[0]} yAxisId="power" stroke="var(--chart-primary-active)" strokeOpacity={0.95} strokeWidth={2} />
            <ReferenceLine x={zoomSelectionBounds[1]} yAxisId="power" stroke="var(--chart-primary-active)" strokeOpacity={0.95} strokeWidth={2} />
          </>
        )}
      </ComposedChart>
    </ResponsiveContainer>
  );
});
