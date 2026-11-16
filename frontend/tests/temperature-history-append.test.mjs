import assert from 'node:assert/strict';
import test from 'node:test';

import {
  HISTORY_SAMPLE_INTERVAL_MS,
  appendHistoryPoint,
  appendSampledHistoryPoint,
  normalizeHistoryPoint,
  trimHistoryPoints,
} from '../src/app/lib/temperature-history.ts';

const START = 1_800_000_000_000;
const SAMPLE_MS = HISTORY_SAMPLE_INTERVAL_MS;

const makePoint = (offsetMs, cpuTemp = 60) => normalizeHistoryPoint({
  timestamp: START + offsetMs,
  cpuTemp,
  gpuTemp: 45,
  fanRpm: 1500,
  cpuPowerWatts: 30,
});

test('appends in-order points under the limit and reuses normalized points', () => {
  const points = [makePoint(0), makePoint(SAMPLE_MS), makePoint(SAMPLE_MS * 2)];
  const appended = appendHistoryPoint(points, makePoint(SAMPLE_MS * 3), {
    retentionMs: 60 * 60 * 1000,
    limit: 720,
  });

  assert.notEqual(appended, points);
  assert.equal(appended.length, 4);
  assert.deepEqual(appended.map((point) => point.timestamp), [
    START,
    START + SAMPLE_MS,
    START + SAMPLE_MS * 2,
    START + SAMPLE_MS * 3,
  ]);
  // stored points are already normalized, so only the new point is normalized:
  // retained points keep their identity instead of being copied per append
  assert.equal(appended[0], points[0]);
  assert.equal(appended[1], points[1]);
  assert.equal(appended[2], points[2]);

  // no-op appends must keep the exact same array reference
  assert.equal(appendHistoryPoint(appended, makePoint(SAMPLE_MS * 3)), appended);
  assert.equal(appendHistoryPoint(appended, null), appended);
  assert.equal(appendHistoryPoint(appended, { timestamp: 0, cpuTemp: 60 }), appended);
  assert.equal(
    appendSampledHistoryPoint(appended, makePoint(SAMPLE_MS * 3 + 1000), { minIntervalMs: SAMPLE_MS }),
    appended,
  );
});

test('replaces the newest point in place when the timestamp matches', () => {
  const points = [makePoint(0), makePoint(SAMPLE_MS)];
  const appended = appendHistoryPoint(points, makePoint(SAMPLE_MS, 70), {
    retentionMs: 60 * 60 * 1000,
    limit: 720,
  });

  assert.notEqual(appended, points);
  assert.equal(appended.length, 2);
  assert.equal(appended[0], points[0]);
  assert.notEqual(appended[1], points[1]);
  assert.equal(appended[1].cpuTemp, 70);
});

test('age cutoff drops points older than the retention window', () => {
  const retentionMs = 60 * 1000;
  const points = [makePoint(0), makePoint(30_000), makePoint(60_000), makePoint(90_000)];
  const appended = appendHistoryPoint(points, makePoint(120_000), { retentionMs, limit: 720 });

  assert.deepEqual(appended.map((point) => point.timestamp), [
    START + 60_000,
    START + 90_000,
    START + 120_000,
  ]);
  assert.deepEqual(
    appended,
    trimHistoryPoints([...points, makePoint(120_000)], retentionMs, 720),
  );
});

test('count cutoff keeps the newest points once the hard limit is reached', () => {
  const points = [makePoint(0), makePoint(SAMPLE_MS), makePoint(SAMPLE_MS * 2)];
  const appended = appendHistoryPoint(points, makePoint(SAMPLE_MS * 3), {
    retentionMs: 60 * 60 * 1000,
    limit: 3,
  });

  assert.equal(appended.length, 3);
  assert.deepEqual(appended.map((point) => point.timestamp), [
    START + SAMPLE_MS,
    START + SAMPLE_MS * 2,
    START + SAMPLE_MS * 3,
  ]);
  assert.deepEqual(
    appended,
    trimHistoryPoints([...points, makePoint(SAMPLE_MS * 3)], 60 * 60 * 1000, 3),
  );
});

test('out-of-order points fall back to the full normalize and sort path', () => {
  const points = [makePoint(60_000), makePoint(120_000), makePoint(180_000)];
  // seconds-based timestamp and string values exercise normalization in the
  // fallback, which like today runs with the default 1h retention and 720 limit
  const staleRaw = { timestamp: (START + 30_000) / 1000, cpuTemp: '65', gpuTemp: 0, fanRpm: 1400 };
  const appended = appendHistoryPoint(points, staleRaw, {
    retentionMs: 5 * 60 * 1000,
    limit: 60,
  });

  assert.deepEqual(appended.map((point) => point.timestamp), [
    START + 30_000,
    START + 60_000,
    START + 120_000,
    START + 180_000,
  ]);
  assert.deepEqual(
    appended,
    trimHistoryPoints([...points, staleRaw], 60 * 60 * 1000, 720),
  );
});
