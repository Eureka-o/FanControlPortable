'use client';

import React, { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import clsx from 'clsx';
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip as RechartsTooltip,
  XAxis,
  YAxis,
} from 'recharts';

export type SharedFanCurvePoint = {
  temperature: number;
  rpm: number;
};

export type SharedFanCurveEditorLabels = {
  temperatureAxis?: string;
  speedAxis?: string;
  baseCurve?: string;
  learnedCurve?: string;
  currentTemperature?: string;
};

export interface SharedFanCurveEditorProps {
  curve: SharedFanCurvePoint[];
  onCurveChange: (curve: SharedFanCurvePoint[]) => void;
  currentTemp?: number | null;
  learnedCurve?: SharedFanCurvePoint[] | null;
  minTemp?: number;
  maxTemp?: number;
  tempStep?: number;
  minSpeed?: number;
  maxSpeed?: number;
  speedStep?: number;
  speedTicks?: number[];
  speedUnit?: string;
  fallbackCurve?: SharedFanCurvePoint[];
  editable?: boolean;
  className?: string;
  heightClassName?: string;
  chartClassName?: string;
  labels?: SharedFanCurveEditorLabels;
  onInteractionChange?: (interacting: boolean) => void;
}

const DEFAULT_MIN_TEMP = 20;
const DEFAULT_MAX_TEMP = 110;
const DEFAULT_TEMP_STEP = 5;
const DEFAULT_MIN_SPEED = 0;
const DEFAULT_MAX_SPEED = 100;
const DEFAULT_SPEED_STEP = 1;

function formatSpeedValue(value: number) {
  if (!Number.isFinite(value)) {
    return '0';
  }
  const rounded = Math.round(value * 10) / 10;
  return Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1);
}

function buildTemperatureTicks(minTemp: number, maxTemp: number, step: number) {
  const ticks: number[] = [];
  const safeStep = Math.max(1, Math.round(step || DEFAULT_TEMP_STEP));
  for (let temp = minTemp; temp <= maxTemp; temp += safeStep) {
    ticks.push(temp);
  }
  if (ticks[ticks.length - 1] !== maxTemp) {
    ticks.push(maxTemp);
  }
  return ticks;
}

function normalizeSpeedValue(value: number, minSpeed: number, maxSpeed: number, speedStep: number) {
  const safeStep = Math.max(1, Number(speedStep) || DEFAULT_SPEED_STEP);
  const rounded = Math.round(Number(value) / safeStep) * safeStep;
  if (!Number.isFinite(rounded)) {
    return minSpeed;
  }
  return Math.max(minSpeed, Math.min(maxSpeed, Math.round(rounded)));
}

function normalizeCurvePoint(point: SharedFanCurvePoint, minSpeed: number, maxSpeed: number, speedStep: number): SharedFanCurvePoint {
  return {
    temperature: Math.round(point.temperature),
    rpm: normalizeSpeedValue(point.rpm, minSpeed, maxSpeed, speedStep),
  };
}

function interpolateCurveSpeed(
  curve: SharedFanCurvePoint[],
  temperature: number,
  fallbackCurve: SharedFanCurvePoint[],
  minSpeed: number,
  maxSpeed: number,
  speedStep: number,
) {
  if (curve.length === 0) {
    const fallback = fallbackCurve.find((point) => point.temperature === temperature)?.rpm ?? minSpeed;
    return normalizeSpeedValue(fallback, minSpeed, maxSpeed, speedStep);
  }

  if (temperature <= curve[0].temperature) {
    return normalizeSpeedValue(curve[0].rpm, minSpeed, maxSpeed, speedStep);
  }

  const last = curve[curve.length - 1];
  if (temperature >= last.temperature) {
    return normalizeSpeedValue(last.rpm, minSpeed, maxSpeed, speedStep);
  }

  for (let index = 0; index < curve.length - 1; index += 1) {
    const left = curve[index];
    const right = curve[index + 1];
    if (temperature === left.temperature) {
      return normalizeSpeedValue(left.rpm, minSpeed, maxSpeed, speedStep);
    }
    if (temperature > left.temperature && temperature < right.temperature) {
      const ratio = (temperature - left.temperature) / (right.temperature - left.temperature);
      return normalizeSpeedValue(left.rpm + (right.rpm - left.rpm) * ratio, minSpeed, maxSpeed, speedStep);
    }
  }

  return normalizeSpeedValue(last.rpm, minSpeed, maxSpeed, speedStep);
}

export function normalizeSharedFanCurve(
  curve: SharedFanCurvePoint[] | null | undefined,
  options: {
    minTemp?: number;
    maxTemp?: number;
    tempStep?: number;
    minSpeed?: number;
    maxSpeed?: number;
    speedStep?: number;
    fallbackCurve?: SharedFanCurvePoint[];
  } = {},
) {
  const minTemp = options.minTemp ?? DEFAULT_MIN_TEMP;
  const maxTemp = options.maxTemp ?? DEFAULT_MAX_TEMP;
  const tempStep = options.tempStep ?? DEFAULT_TEMP_STEP;
  const minSpeed = options.minSpeed ?? DEFAULT_MIN_SPEED;
  const maxSpeed = options.maxSpeed ?? DEFAULT_MAX_SPEED;
  const speedStep = options.speedStep ?? DEFAULT_SPEED_STEP;
  const ticks = buildTemperatureTicks(minTemp, maxTemp, tempStep);
  const fallbackCurve = options.fallbackCurve ?? [];
  const source = Array.isArray(curve)
    ? curve
      .map((point) => normalizeCurvePoint(point, minSpeed, maxSpeed, speedStep))
      .filter((point) => point.temperature >= minTemp && point.temperature <= maxTemp)
      .sort((left, right) => left.temperature - right.temperature)
    : [];

  const unique = source.reduce<SharedFanCurvePoint[]>((points, point) => {
    const previous = points[points.length - 1];
    if (previous && previous.temperature === point.temperature) {
      points[points.length - 1] = point;
    } else {
      points.push(point);
    }
    return points;
  }, []);

  const base = unique.length > 0 ? unique : fallbackCurve;
  return ticks.map((temperature) => ({
    temperature,
    rpm: interpolateCurveSpeed(base, temperature, fallbackCurve, minSpeed, maxSpeed, speedStep),
  }));
}

export function syncSharedCurveSpeedAtIndex(
  curve: SharedFanCurvePoint[],
  index: number,
  targetSpeed: number,
  minSpeed: number,
  maxSpeed: number,
  speedStep = DEFAULT_SPEED_STEP,
) {
  const currentPoint = curve[index];
  if (!currentPoint) {
    return { curve, changed: false };
  }

  const normalizedSpeed = normalizeSpeedValue(targetSpeed, minSpeed, maxSpeed, speedStep);
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

const SharedTemperatureIndicator = memo(function SharedTemperatureIndicator({
  temperature,
  chartRef,
  minTemp,
  maxTemp,
  label,
}: {
  temperature: number | null;
  chartRef: React.RefObject<HTMLDivElement | null>;
  minTemp: number;
  maxTemp: number;
  label: (temperature: number) => string;
}) {
  const [position, setPosition] = useState<{ x: number; top: number; height: number } | null>(null);

  useEffect(() => {
    if (temperature === null || !chartRef.current) {
      setPosition(null);
      return;
    }

    const updatePosition = () => {
      const chartArea = chartRef.current?.querySelector('.recharts-cartesian-grid');
      if (!chartArea) return;
      const rect = chartArea.getBoundingClientRect();
      const containerRect = chartRef.current?.querySelector('.recharts-responsive-container')?.getBoundingClientRect();
      if (!containerRect) return;
      const chartWidth = rect.width;
      const chartLeft = rect.left - containerRect.left;
      const tempPercent = (temperature - minTemp) / Math.max(1, maxTemp - minTemp);
      const x = chartLeft + tempPercent * chartWidth;
      setPosition({ x, top: rect.top - containerRect.top, height: rect.height });
    };

    updatePosition();
    window.addEventListener('resize', updatePosition);
    return () => window.removeEventListener('resize', updatePosition);
  }, [temperature, chartRef, minTemp, maxTemp]);

  if (!position || temperature === null) return null;

  return (
    <svg className="pointer-events-none absolute inset-0 overflow-visible" style={{ width: '100%', height: '100%' }}>
      <line x1={position.x} y1={position.top} x2={position.x} y2={position.top + position.height} stroke="var(--chart-temperature-indicator)" strokeWidth={2} strokeDasharray="5 5" />
      <rect x={position.x - 45} y={position.top - 22} width={90} height={20} rx={4} fill="var(--chart-temperature-indicator)" />
      <text x={position.x} y={position.top - 8} textAnchor="middle" fill="white" fontSize={11} fontWeight={500}>{label(temperature)}</text>
    </svg>
  );
});

const SharedDraggablePoint = memo(function SharedDraggablePoint({
  cx,
  cy,
  index,
  speed,
  unitSuffix,
  onDragStart,
  isActive,
  editable,
}: {
  cx?: number;
  cy?: number;
  index?: number;
  speed?: number;
  unitSuffix: string;
  onDragStart: (index: number) => void;
  isActive: boolean;
  editable: boolean;
}) {
  const pointIndex = typeof index === 'number' ? index : -1;
  const handleMouseDown = useCallback((event: React.MouseEvent) => {
    if (!editable || pointIndex < 0) return;
    event.preventDefault();
    event.stopPropagation();
    onDragStart(pointIndex);
  }, [editable, onDragStart, pointIndex]);
  const handleTouchStart = useCallback((event: React.TouchEvent) => {
    if (!editable || pointIndex < 0) return;
    event.preventDefault();
    event.stopPropagation();
    onDragStart(pointIndex);
  }, [editable, onDragStart, pointIndex]);

  if (cx === undefined || cy === undefined) {
    return <g />;
  }

  return (
    <g>
      <circle cx={cx} cy={cy} r={isActive ? 14 : 10} fill="transparent" stroke="transparent" style={{ cursor: editable ? 'ns-resize' : 'default' }} onMouseDown={handleMouseDown} onTouchStart={handleTouchStart} />
      <circle
        cx={cx}
        cy={cy}
        r={isActive ? 8 : 6}
        fill={isActive ? 'var(--chart-primary-active)' : 'var(--chart-primary)'}
        stroke="var(--card)"
        strokeWidth={2}
        style={{
          cursor: editable ? 'ns-resize' : 'default',
          transition: isActive ? 'none' : 'all 0.2s ease',
          filter: isActive ? 'drop-shadow(0 4px 8px var(--chart-primary-glow))' : 'drop-shadow(0 2px 4px var(--chart-point-shadow))',
          opacity: editable ? 1 : 0.74,
        }}
        onMouseDown={handleMouseDown}
        onTouchStart={handleTouchStart}
      />
      {isActive && (
        <g>
          <rect x={cx - 38} y={cy - 35} width={76} height={24} rx={4} fill="var(--chart-primary-active)" opacity={0.95} />
          <text x={cx} y={cy - 19} textAnchor="middle" fill="white" fontSize={12} fontWeight={600}>{formatSpeedValue(speed ?? 0)}{unitSuffix}</text>
        </g>
      )}
    </g>
  );
});

export const SharedFanCurveEditor = memo(function SharedFanCurveEditor({
  curve,
  onCurveChange,
  currentTemp = null,
  learnedCurve = null,
  minTemp = DEFAULT_MIN_TEMP,
  maxTemp = DEFAULT_MAX_TEMP,
  tempStep = DEFAULT_TEMP_STEP,
  minSpeed = DEFAULT_MIN_SPEED,
  maxSpeed = DEFAULT_MAX_SPEED,
  speedStep = DEFAULT_SPEED_STEP,
  speedTicks,
  speedUnit = '',
  fallbackCurve,
  editable = true,
  className,
  heightClassName = 'h-80 md:h-96',
  chartClassName,
  labels,
  onInteractionChange,
}: SharedFanCurveEditorProps) {
  const chartRef = useRef<HTMLDivElement>(null);
  const chartBoundsRef = useRef<{ top: number; bottom: number; yMin: number; yMax: number } | null>(null);
  const dragFrameRef = useRef<number | null>(null);
  const pendingDragYRef = useRef<number | null>(null);
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const temperatureTicks = useMemo(() => buildTemperatureTicks(minTemp, maxTemp, tempStep), [minTemp, maxTemp, tempStep]);
  const normalizedCurve = useMemo(() => normalizeSharedFanCurve(curve, {
    minTemp,
    maxTemp,
    tempStep,
    minSpeed,
    maxSpeed,
    speedStep,
    fallbackCurve,
  }), [curve, fallbackCurve, maxSpeed, maxTemp, minSpeed, minTemp, speedStep, tempStep]);
  const normalizedLearnedCurve = useMemo(() => {
    if (!learnedCurve || learnedCurve.length === 0) {
      return null;
    }
    return normalizeSharedFanCurve(learnedCurve, {
      minTemp,
      maxTemp,
      tempStep,
      minSpeed,
      maxSpeed,
      speedStep,
      fallbackCurve: normalizedCurve,
    });
  }, [fallbackCurve, learnedCurve, maxSpeed, maxTemp, minSpeed, minTemp, normalizedCurve, speedStep, tempStep]);
  const chartData = useMemo(() => normalizedCurve.map((point, index) => ({
    ...point,
    learnedRpm: normalizedLearnedCurve?.[index]?.rpm,
    index,
  })), [normalizedCurve, normalizedLearnedCurve]);
  const resolvedSpeedTicks = useMemo(() => {
    if (Array.isArray(speedTicks) && speedTicks.length > 0) {
      return speedTicks;
    }
    const span = Math.max(1, maxSpeed - minSpeed);
    const step = span <= 100 ? 20 : span <= 3000 ? 500 : 1000;
    const ticks: number[] = [];
    for (let value = minSpeed; value <= maxSpeed; value += step) {
      ticks.push(value);
    }
    if (ticks[ticks.length - 1] !== maxSpeed) {
      ticks.push(maxSpeed);
    }
    return ticks;
  }, [maxSpeed, minSpeed, speedTicks]);
  const clampedCurrentTemp = currentTemp === null || currentTemp === undefined
    ? null
    : Math.max(minTemp, Math.min(maxTemp, Math.round(currentTemp)));
  const unitSuffix = speedUnit ? String(speedUnit) : '';
  const speedAxisLabel = labels?.speedAxis ?? (unitSuffix ? `速度（${unitSuffix}）` : '速度');
  const tempAxisLabel = labels?.temperatureAxis ?? '温度';
  const baseCurveLabel = labels?.baseCurve ?? '基础曲线';
  const learnedCurveLabel = labels?.learnedCurve ?? '学习曲线';
  const currentTempLabel = labels?.currentTemperature ?? '当前 {{temperature}}°C';

  const updatePoint = useCallback((index: number, newSpeed: number) => {
    const nextState = syncSharedCurveSpeedAtIndex(normalizedCurve, index, newSpeed, minSpeed, maxSpeed, speedStep);
    if (nextState.changed) {
      onCurveChange(nextState.curve);
    }
  }, [maxSpeed, minSpeed, normalizedCurve, onCurveChange, speedStep]);

  const handleDragStart = useCallback((index: number) => {
    if (!editable) return;
    setDragIndex(index);
    onInteractionChange?.(true);
    if (chartRef.current) {
      const chartArea = chartRef.current.querySelector('.recharts-cartesian-grid');
      if (chartArea) {
        const rect = chartArea.getBoundingClientRect();
        chartBoundsRef.current = { top: rect.top, bottom: rect.bottom, yMin: minSpeed, yMax: maxSpeed };
      }
    }
  }, [editable, maxSpeed, minSpeed, onInteractionChange]);

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
    window.setTimeout(() => onInteractionChange?.(false), 100);
  }, [onInteractionChange]);

  useEffect(() => {
    if (dragIndex === null) return;
    const mm = (event: MouseEvent) => {
      event.preventDefault();
      scheduleDrag(event.clientY);
    };
    const tm = (event: TouchEvent) => {
      if (event.touches.length > 0) {
        scheduleDrag(event.touches[0].clientY);
      }
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
      pendingDragYRef.current = null;
    };
  }, [dragIndex, handleDragEnd, scheduleDrag]);

  const CustomDot = useCallback((props: any): React.ReactElement<SVGElement> => {
    const { cx, cy, index, payload } = props;
    return (
      <SharedDraggablePoint
        cx={cx}
        cy={cy}
        index={index}
        speed={payload?.rpm}
        unitSuffix={unitSuffix}
        onDragStart={handleDragStart}
        isActive={dragIndex === index}
        editable={editable}
      />
    );
  }, [dragIndex, editable, handleDragStart, unitSuffix]);

  return (
    <div data-theme-card="curve-editor" className={className}>
      <div
        ref={chartRef}
        className={clsx(
          'relative rounded-3xl border bg-card p-4 shadow-sm',
          dragIndex !== null ? 'border-primary/30 ring-2 ring-primary/40' : 'border-border/70',
          chartClassName,
        )}
      >
        <div className={clsx('relative', heightClassName)}>
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={chartData} margin={{ top: 20, right: 30, left: 20, bottom: 20 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--chart-grid)" />
              <XAxis
                dataKey="temperature"
                type="number"
                domain={[minTemp, maxTemp]}
                ticks={temperatureTicks}
                interval={0}
                minTickGap={0}
                tickLine={false}
                axisLine={{ stroke: 'var(--chart-axis)' }}
                tick={{ fill: 'var(--chart-tick)', fontSize: 10 }}
                label={{ value: tempAxisLabel, position: 'insideBottom', offset: -10, fill: 'var(--chart-tick)', fontSize: 12 }}
              />
              <YAxis
                type="number"
                domain={[minSpeed, maxSpeed]}
                ticks={resolvedSpeedTicks}
                tickLine={false}
                axisLine={{ stroke: 'var(--chart-axis)' }}
                tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
                label={{ value: speedAxisLabel, angle: -90, position: 'insideLeft', fill: 'var(--chart-tick)', fontSize: 12 }}
              />
              <RechartsTooltip
                formatter={(value, name) => {
                  const numericValue = Number(value ?? 0);
                  return name === 'learnedRpm'
                    ? [`${formatSpeedValue(numericValue)}${unitSuffix}`, learnedCurveLabel]
                    : [`${formatSpeedValue(numericValue)}${unitSuffix}`, baseCurveLabel];
                }}
                labelFormatter={(value) => `${value}°C`}
                contentStyle={{ backgroundColor: 'var(--chart-tooltip-bg)', border: '1px solid', borderColor: 'var(--chart-tooltip-border)', borderRadius: '8px', boxShadow: 'var(--chart-tooltip-shadow)', padding: '8px 12px', color: 'var(--chart-tooltip-text)' }}
                labelStyle={{ color: 'var(--chart-tooltip-text)', fontWeight: 600 }}
                itemStyle={{ color: 'var(--chart-tooltip-text)' }}
              />
              <Line type="monotone" dataKey="rpm" stroke="var(--chart-primary)" strokeWidth={3} dot={CustomDot} activeDot={false} isAnimationActive={false} />
              {normalizedLearnedCurve && <Line type="monotone" dataKey="learnedRpm" stroke="var(--chart-primary)" strokeWidth={2} strokeDasharray="6 4" dot={false} activeDot={false} isAnimationActive={false} />}
            </LineChart>
          </ResponsiveContainer>
          <SharedTemperatureIndicator
            temperature={clampedCurrentTemp}
            chartRef={chartRef}
            minTemp={minTemp}
            maxTemp={maxTemp}
            label={(temperature) => currentTempLabel.replace('{{temperature}}', String(temperature))}
          />
        </div>
      </div>
    </div>
  );
});

