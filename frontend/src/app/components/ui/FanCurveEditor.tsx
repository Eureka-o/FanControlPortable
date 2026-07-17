'use client';

import React, { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import clsx from 'clsx';
import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip as RechartsTooltip,
  XAxis,
  YAxis,
} from 'recharts';
import {
  updateFanCurvePointSpeed,
  type FanCurveEditorPoint,
} from './fan-curve-editor-logic.mts';

export type { FanCurveEditorPoint } from './fan-curve-editor-logic.mts';

export interface FanCurveEditorProps {
  points: FanCurveEditorPoint[];
  onChange: (points: FanCurveEditorPoint[]) => void;
  minTemperature: number;
  maxTemperature: number;
  minSpeed: number;
  maxSpeed: number;
  temperatureTicks?: number[];
  speedTicks?: number[];
  speedStep?: number;
  speedUnit?: string;
  temperatureAxisLabel?: string;
  speedAxisLabel?: string;
  temperatureLabel?: (temperature: number) => string;
  baseCurveLabel?: string;
  secondaryPoints?: FanCurveEditorPoint[];
  secondaryCurveLabel?: string;
  currentTemperature?: number | null;
  currentTemperatureLabel?: string;
  pointAriaLabel?: (point: FanCurveEditorPoint, index: number) => string;
  enforceMonotonic?: boolean;
  disabled?: boolean;
  compact?: boolean;
  className?: string;
  onInteractionChange?: (interacting: boolean) => void;
}

function formatSpeed(value: number) {
  const rounded = Math.round(value * 10) / 10;
  return Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1);
}

function buildTicks(min: number, max: number) {
  if (max <= min) return [min];
  return Array.from({ length: 6 }, (_, index) => Math.round(min + ((max - min) * index) / 5));
}

function usePrefersReducedMotion() {
  const [reduced, setReduced] = useState(false);

  useEffect(() => {
    const media = window.matchMedia('(prefers-reduced-motion: reduce)');
    const update = () => setReduced(media.matches);
    update();
    media.addEventListener('change', update);
    return () => media.removeEventListener('change', update);
  }, []);

  return reduced;
}

interface CurvePointProps {
  cx: number;
  cy: number;
  index: number;
  point: FanCurveEditorPoint;
  active: boolean;
  disabled: boolean;
  minSpeed: number;
  maxSpeed: number;
  speedUnit: string;
  label: string;
  reducedMotion: boolean;
  onPointerDown: (event: React.PointerEvent<SVGGElement>, index: number) => void;
  onKeyDown: (event: React.KeyboardEvent<SVGGElement>, index: number) => void;
  onFocus: (index: number) => void;
  onBlur: () => void;
}

const CurvePoint = memo(function CurvePoint({
  cx,
  cy,
  index,
  point,
  active,
  disabled,
  minSpeed,
  maxSpeed,
  speedUnit,
  label,
  reducedMotion,
  onPointerDown,
  onKeyDown,
  onFocus,
  onBlur,
}: CurvePointProps) {
  return (
    <g
      role="slider"
      tabIndex={disabled ? -1 : 0}
      aria-label={label}
      aria-valuemin={minSpeed}
      aria-valuemax={maxSpeed}
      aria-valuenow={point.rpm}
      aria-disabled={disabled}
      onPointerDown={(event) => onPointerDown(event, index)}
      onKeyDown={(event) => onKeyDown(event, index)}
      onFocus={() => onFocus(index)}
      onBlur={onBlur}
      style={{ cursor: disabled ? 'not-allowed' : 'ns-resize', outline: 'none', touchAction: 'none' }}
    >
      <circle cx={cx} cy={cy} r={active ? 14 : 10} fill="transparent" stroke="transparent" />
      <circle
        cx={cx}
        cy={cy}
        r={active ? 8 : 6}
        fill={active ? 'var(--chart-primary-active)' : 'var(--chart-primary)'}
        stroke="var(--card)"
        strokeWidth={2}
        style={{
          transition: active || reducedMotion ? 'none' : 'all 0.2s ease',
          filter: active
            ? 'drop-shadow(0 4px 8px var(--chart-primary-glow))'
            : 'drop-shadow(0 2px 4px var(--chart-point-shadow))',
        }}
      />
      {active && (
        <g pointerEvents="none">
          <rect x={cx - 38} y={cy - 35} width={76} height={24} rx={4} fill="var(--chart-primary-active)" opacity={0.96} />
          <text x={cx} y={cy - 19} textAnchor="middle" fill="white" fontSize={12} fontWeight={600}>
            {formatSpeed(point.rpm)}{speedUnit}
          </text>
        </g>
      )}
    </g>
  );
});

export const FanCurveEditor = memo(function FanCurveEditor({
  points,
  onChange,
  minTemperature,
  maxTemperature,
  minSpeed,
  maxSpeed,
  temperatureTicks,
  speedTicks,
  speedStep = 1,
  speedUnit = '',
  temperatureAxisLabel = 'Temperature (°C)',
  speedAxisLabel,
  temperatureLabel = (temperature) => `${temperature}°C`,
  baseCurveLabel = 'Base curve',
  secondaryPoints,
  secondaryCurveLabel = 'Secondary curve',
  currentTemperature = null,
  currentTemperatureLabel,
  pointAriaLabel,
  enforceMonotonic = true,
  disabled = false,
  compact = false,
  className,
  onInteractionChange,
}: FanCurveEditorProps) {
  const chartRef = useRef<HTMLDivElement>(null);
  const pointsRef = useRef(points);
  const boundsRef = useRef<{ top: number; bottom: number } | null>(null);
  const dragFrameRef = useRef<number | null>(null);
  const pendingDragYRef = useRef<number | null>(null);
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [focusIndex, setFocusIndex] = useState<number | null>(null);
  const reducedMotion = usePrefersReducedMotion();
  pointsRef.current = points;

  const chartData = useMemo(() => points.map((point, index) => ({
    ...point,
    secondaryRpm: secondaryPoints?.[index]?.temperature === point.temperature
      ? secondaryPoints[index].rpm
      : undefined,
  })), [points, secondaryPoints]);
  const resolvedTemperatureTicks = useMemo(
    () => temperatureTicks ?? buildTicks(minTemperature, maxTemperature),
    [maxTemperature, minTemperature, temperatureTicks],
  );
  const resolvedSpeedTicks = useMemo(
    () => speedTicks ?? buildTicks(minSpeed, maxSpeed),
    [maxSpeed, minSpeed, speedTicks],
  );

  const updateSpeed = useCallback((index: number, value: number) => {
    if (disabled) return;
    const next = updateFanCurvePointSpeed(
      pointsRef.current,
      index,
      value,
      minSpeed,
      maxSpeed,
      speedStep,
      enforceMonotonic,
    );
    if (next === pointsRef.current) return;
    pointsRef.current = next;
    onChange(next);
  }, [disabled, enforceMonotonic, maxSpeed, minSpeed, onChange, speedStep]);

  const endInteraction = useCallback(() => {
    if (dragFrameRef.current !== null) window.cancelAnimationFrame(dragFrameRef.current);
    dragFrameRef.current = null;
    pendingDragYRef.current = null;
    boundsRef.current = null;
    setDragIndex(null);
    onInteractionChange?.(false);
  }, [onInteractionChange]);

  const updateFromPointer = useCallback((clientY: number) => {
    if (dragIndex === null || !boundsRef.current) return;
    const { top, bottom } = boundsRef.current;
    const ratio = Math.max(0, Math.min(1, (bottom - clientY) / Math.max(1, bottom - top)));
    updateSpeed(dragIndex, minSpeed + ratio * (maxSpeed - minSpeed));
  }, [dragIndex, maxSpeed, minSpeed, updateSpeed]);

  const schedulePointerUpdate = useCallback((clientY: number) => {
    pendingDragYRef.current = clientY;
    if (dragFrameRef.current !== null) return;
    dragFrameRef.current = window.requestAnimationFrame(() => {
      dragFrameRef.current = null;
      const pendingY = pendingDragYRef.current;
      pendingDragYRef.current = null;
      if (pendingY !== null) updateFromPointer(pendingY);
    });
  }, [updateFromPointer]);

  useEffect(() => {
    if (dragIndex === null) return;
    const move = (event: PointerEvent) => {
      event.preventDefault();
      schedulePointerUpdate(event.clientY);
    };
    window.addEventListener('pointermove', move, { passive: false });
    window.addEventListener('pointerup', endInteraction);
    window.addEventListener('pointercancel', endInteraction);
    return () => {
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', endInteraction);
      window.removeEventListener('pointercancel', endInteraction);
    };
  }, [dragIndex, endInteraction, schedulePointerUpdate]);

  useEffect(() => () => {
    if (dragFrameRef.current !== null) window.cancelAnimationFrame(dragFrameRef.current);
  }, []);

  const handlePointerDown = useCallback((event: React.PointerEvent<SVGGElement>, index: number) => {
    if (disabled || (event.pointerType === 'mouse' && event.button !== 0)) return;
    event.preventDefault();
    event.stopPropagation();
    event.currentTarget.focus();
    const chartArea = chartRef.current?.querySelector('.recharts-cartesian-grid');
    if (!chartArea) return;
    const bounds = chartArea.getBoundingClientRect();
    boundsRef.current = { top: bounds.top, bottom: bounds.bottom };
    setDragIndex(index);
    onInteractionChange?.(true);
  }, [disabled, onInteractionChange]);

  const handleKeyDown = useCallback((event: React.KeyboardEvent<SVGGElement>, index: number) => {
    if (disabled) return;
    const current = pointsRef.current[index]?.rpm;
    if (current === undefined) return;
    let next: number | null = null;
    if (event.key === 'ArrowUp' || event.key === 'ArrowRight') next = current + speedStep;
    if (event.key === 'ArrowDown' || event.key === 'ArrowLeft') next = current - speedStep;
    if (event.key === 'PageUp') next = current + speedStep * 5;
    if (event.key === 'PageDown') next = current - speedStep * 5;
    if (event.key === 'Home') next = minSpeed;
    if (event.key === 'End') next = maxSpeed;
    if (next === null) return;
    event.preventDefault();
    updateSpeed(index, next);
  }, [disabled, maxSpeed, minSpeed, speedStep, updateSpeed]);

  const CustomDot = useCallback((props: any): React.ReactElement<SVGElement> => {
    const { cx, cy, index, payload } = props;
    if (cx === undefined || cy === undefined || !payload) return <g />;
    const point = points[index] ?? payload;
    return (
      <CurvePoint
        cx={cx}
        cy={cy}
        index={index}
        point={point}
        active={dragIndex === index || focusIndex === index}
        disabled={disabled}
        minSpeed={minSpeed}
        maxSpeed={maxSpeed}
        speedUnit={speedUnit}
        label={pointAriaLabel?.(point, index) ?? `${temperatureLabel(point.temperature)}, ${formatSpeed(point.rpm)}${speedUnit}`}
        reducedMotion={reducedMotion}
        onPointerDown={handlePointerDown}
        onKeyDown={handleKeyDown}
        onFocus={setFocusIndex}
        onBlur={() => setFocusIndex(null)}
      />
    );
  }, [disabled, dragIndex, focusIndex, handleKeyDown, handlePointerDown, maxSpeed, minSpeed, pointAriaLabel, points, reducedMotion, speedUnit, temperatureLabel]);

  return (
    <div
      ref={chartRef}
      data-theme-ui="fan-curve-editor"
      className={clsx(
        'relative rounded-3xl border bg-card p-4 shadow-sm',
        dragIndex !== null ? 'border-primary/30 ring-2 ring-primary/40' : 'border-border/70',
        disabled && 'opacity-70',
        className,
      )}
    >
      <div className={clsx('relative min-h-64', compact ? 'h-64 md:h-72' : 'h-80 md:h-96')}>
        <ResponsiveContainer width="100%" height="100%" minWidth={0} initialDimension={{ width: 1, height: 1 }}>
          <LineChart data={chartData} margin={{ top: 22, right: 30, left: 20, bottom: 20 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--chart-grid)" />
            <XAxis
              dataKey="temperature"
              type="number"
              domain={[minTemperature, maxTemperature]}
              ticks={resolvedTemperatureTicks}
              interval={0}
              minTickGap={0}
              tickLine={false}
              axisLine={{ stroke: 'var(--chart-axis)' }}
              tick={{ fill: 'var(--chart-tick)', fontSize: 10 }}
              label={{ value: temperatureAxisLabel, position: 'insideBottom', offset: -10, fill: 'var(--chart-tick)', fontSize: 12 }}
            />
            <YAxis
              type="number"
              domain={[minSpeed, maxSpeed]}
              ticks={resolvedSpeedTicks}
              tickLine={false}
              axisLine={{ stroke: 'var(--chart-axis)' }}
              tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
              label={{ value: speedAxisLabel ?? `Speed (${speedUnit})`, angle: -90, position: 'insideLeft', fill: 'var(--chart-tick)', fontSize: 12 }}
            />
            <RechartsTooltip
              formatter={(value, name) => [
                `${formatSpeed(Number(value ?? 0))}${speedUnit}`,
                name === 'secondaryRpm' ? secondaryCurveLabel : baseCurveLabel,
              ]}
              labelFormatter={(value) => temperatureLabel(Number(value))}
              contentStyle={{
                backgroundColor: 'var(--chart-tooltip-bg)',
                border: '1px solid var(--chart-tooltip-border)',
                borderRadius: '8px',
                boxShadow: 'var(--chart-tooltip-shadow)',
                color: 'var(--chart-tooltip-text)',
                padding: '8px 12px',
              }}
              labelStyle={{ color: 'var(--chart-tooltip-text)', fontWeight: 600 }}
              itemStyle={{ color: 'var(--chart-tooltip-text)' }}
            />
            {currentTemperature !== null && Number.isFinite(currentTemperature) && (
              <ReferenceLine
                x={currentTemperature}
                stroke="var(--chart-temperature-indicator)"
                strokeWidth={2}
                strokeDasharray="5 5"
                label={{
                  value: currentTemperatureLabel ?? temperatureLabel(currentTemperature),
                  position: 'top',
                  fill: 'var(--chart-temperature-indicator)',
                  fontSize: 11,
                  fontWeight: 600,
                }}
              />
            )}
            <Line type="monotone" dataKey="rpm" stroke="var(--chart-primary)" strokeWidth={3} dot={CustomDot} activeDot={false} isAnimationActive={false} />
            {secondaryPoints && secondaryPoints.length > 0 && (
              <Line type="monotone" dataKey="secondaryRpm" stroke="var(--chart-primary)" strokeWidth={2} strokeDasharray="6 4" dot={false} activeDot={false} isAnimationActive={false} />
            )}
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
});
