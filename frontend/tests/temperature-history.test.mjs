import assert from 'node:assert/strict';
import test from 'node:test';

import {
  CORE_HISTORY_LIMIT,
  HISTORY_SAMPLE_INTERVAL_MS,
  normalizeHistoryPoints,
} from '../src/app/lib/temperature-history.ts';

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
