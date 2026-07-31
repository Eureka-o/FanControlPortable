import assert from 'node:assert/strict';
import test from 'node:test';

import {
  CORE_HISTORY_LIMIT,
  HISTORY_SAMPLE_INTERVAL_MS,
  detectAbruptHistoryPoints,
  normalizeHistoryPoints,
} from '../src/app/lib/temperature-history.ts';

test('detects the strongest local jumps without treating smooth ramps as jumps', () => {
  const points = [10, 11, 12, 30, 13, 14, 25, 25, 25].map((value, index) => ({
    timestamp: index * 1000,
    value,
  }));

  assert.deepEqual(detectAbruptHistoryPoints(points, 3), [
    { timestamp: 3000, value: 30 },
    { timestamp: 6000, value: 25 },
  ]);
  assert.deepEqual(detectAbruptHistoryPoints(points, 3, 1), [
    { timestamp: 3000, value: 30 },
  ]);
});

test('keeps the full rolling hour at the five-second core cadence', () => {
  const start = 1_800_000_000_000;
  const points = Array.from({ length: CORE_HISTORY_LIMIT + 1 }, (_, index) => ({
    timestamp: start + index * HISTORY_SAMPLE_INTERVAL_MS,
    cpuTemp: 60,
    gpuTemp: 0,
    fanRpm: 1500,
  }));

  const normalized = normalizeHistoryPoints(points);
  assert.equal(normalized.length, CORE_HISTORY_LIMIT);
  assert.equal(normalized.at(-1).timestamp - normalized[0].timestamp, 60 * 60 * 1000 - HISTORY_SAMPLE_INTERVAL_MS);
});
