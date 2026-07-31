import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const statusSource = readFileSync(new URL('../src/app/components/DeviceStatus.tsx', import.meta.url), 'utf8');
const curveSource = readFileSync(new URL('../src/app/components/FanCurve.tsx', import.meta.url), 'utf8');
const preferencesSource = readFileSync(new URL('../src/app/hooks/useHistoryDisplayPreferences.ts', import.meta.url), 'utf8');
const globalStyles = readFileSync(new URL('../src/app/globals.css', import.meta.url), 'utf8');

const seriesVariables = {
  CPU_TEMP_STROKE: '--chart-cpu-temperature',
  GPU_TEMP_STROKE: '--chart-gpu-temperature',
  FAN_SPEED_STROKE: '--chart-fan-speed',
  CPU_POWER_STROKE: '--chart-cpu-power',
  GPU_POWER_STROKE: '--chart-gpu-power',
};

test('uses the same theme variables for history thumbnail and detail charts', () => {
  for (const [constant, variable] of Object.entries(seriesVariables)) {
    const declaration = new RegExp(`const ${constant} = 'var\\(${variable}\\)'`);
    assert.match(statusSource, declaration);
    assert.match(curveSource, declaration);
  }

  assert.match(statusSource, /series === 'cpu'\s*\? CPU_TEMP_STROKE/);
  assert.match(statusSource, /series === 'gpu'\s*\? GPU_TEMP_STROKE/);
  assert.match(statusSource, /series === 'fan'\s*\? FAN_SPEED_STROKE/);
  assert.doesNotMatch(statusSource, /FAN_TREND_STROKE|series === 'cpu'\s*\? 'var\(--chart-primary\)'|series === 'gpu'\s*\? 'var\(--chart-temperature-indicator\)'/);
});

test('supports total power and single-series statistics without hard-coded stat colors', () => {
  assert.match(curveSource, /totalPower:\s*'totalPowerWatts'/);
  assert.match(curveSource, /historySeriesVisibility\.totalPower/);
  assert.match(curveSource, /ReferenceDot/);
  assert.match(curveSource, /stroke=\{color\}/);
  assert.match(curveSource, /var\(--chart-stat-max\)/);
  assert.match(curveSource, /var\(--chart-stat-min\)/);
  assert.match(curveSource, /var\(--chart-stat-average\)/);
  assert.match(curveSource, /showStatistics/);
  assert.match(curveSource, /segment=\{key === 'average' \? undefined/);
  assert.match(curveSource, /minTimestamps: number\[\]/);
  assert.match(curveSource, /maxTimestamps: number\[\]/);
  assert.match(curveSource, /const leftmostTimestamp = timestamps\[0\] \?\? 0/);
  assert.match(curveSource, /const emphasizedTimestamps = timestamps\.slice\(0, 1\)/);
  assert.match(curveSource, /\{ x: leftmostTimestamp, y: value \}/);
  assert.match(curveSource, /\{ x: historyRightTimestamp, y: value \}/);
  assert.match(curveSource, /textAnchor="end"/);
  assert.match(curveSource, /stroke="var\(--chart-stat-label-halo\)"/);
  assert.match(curveSource, /<ComposedChart/);
  assert.match(curveSource, /baseValue="dataMin"/);
  assert.match(curveSource, /var\(--chart-area-opacity-start\)/);
  assert.match(curveSource, /var\(--chart-area-opacity-end\)/);
  assert.match(curveSource, /const formatValue = \(value: number\) => series\.axisId === 'power'/);
  assert.match(curveSource, /cpuPeak: Number\(formatSpeedValue\(cpuPeak\)\)/);
  assert.match(curveSource, /gpuPeak: Number\(formatSpeedValue\(gpuPeak\)\)/);
  assert.match(curveSource, /const numericValue = Number\(formatSpeedValue\(Number\(point\[series\.dataKey\]/);
  assert.match(globalStyles, /--chart-stat-label-halo:/);
  assert.match(globalStyles, /--chart-area-opacity-start:/);
  assert.match(globalStyles, /--chart-area-opacity-end:/);
});

test('keeps independent home chart visibility and supports total power in the thumbnail', () => {
  assert.match(preferencesSource, /homeVisible: HistorySeriesVisibility/);
  assert.match(preferencesSource, /const homeVisibilityInput = input\?\.homeVisible/);
  assert.match(preferencesSource, /homeSeriesVisibility: preferences\.homeVisible/);
  assert.match(preferencesSource, /const preferencesRef = useRef\(preferences\)/);
  assert.match(preferencesSource, /updater\(preferencesRef\.current\)/);
  assert.doesNotMatch(preferencesSource, /setPreferences\(\(current\) => \{[\s\S]*writeHistoryDisplayPreferences/);
  assert.match(curveSource, /homeDisplayTitle/);
  assert.match(curveSource, /toggleHomeSeriesVisible/);
  assert.match(statusSource, /visibleSeries=\{homeSeriesVisibility\}/);
  assert.match(statusSource, /orderedSeries=\{HISTORY_SERIES_ORDER\}/);
  assert.match(statusSource, /totalPower: buildPath\(getAvailableTotalPowerWatts, yForPower\)/);
  assert.match(statusSource, /const TOTAL_POWER_STROKE = 'var\(--chart-primary\)'/);
});

test('keeps raw history values and relies on monotone rendering for light smoothing', () => {
  assert.doesNotMatch(curveSource, /smoothHistoryChartData/);
  assert.equal((curveSource.match(/data=\{zoomedHistoryChartData\}/g) || []).length, 2);
  assert.equal((curveSource.match(/type="monotone"/g) || []).length >= 4, true);
  assert.match(curveSource, /for \(const point of zoomedHistoryChartData\)/);
  assert.match(curveSource, /detectAbruptHistoryPoints/);
  assert.match(curveSource, /minimumDelta,\s*1,/);
  assert.match(curveSource, /historyAbruptPoints/);
  assert.match(curveSource, /historyStatistics\.thermalSeries\.map/);
  assert.match(curveSource, /historyStatistics\.powerSeries\.map/);
});
