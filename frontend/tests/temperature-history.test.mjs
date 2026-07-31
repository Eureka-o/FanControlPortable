import assert from 'node:assert/strict';
import test from 'node:test';

import { detectAbruptHistoryPoints, normalizeHistoryPoints } from '../src/app/lib/temperature-history.ts';

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

test('keeps a full hour when core samples arrive faster than every five seconds', () => {
  const start = 1_800_000_000_000;
  const points = Array.from({ length: 901 }, (_, index) => ({
    timestamp: start + index * 4_000,
    cpuTemp: 60,
    gpuTemp: 0,
    fanRpm: 1500,
  }));

  const normalized = normalizeHistoryPoints(points);
  assert.equal(normalized.length, points.length);
  assert.equal(normalized.at(-1).timestamp - normalized[0].timestamp, 60 * 60 * 1000);
});
