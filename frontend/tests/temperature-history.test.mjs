import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

import {
  CORE_HISTORY_LIMIT,
  HISTORY_SAMPLE_INTERVAL_MS,
  SESSION_HISTORY_LIMIT,
  SESSION_HISTORY_RETENTION_MS,
  clampHistoryRetentionHours,
  historyRetentionLimit,
  normalizeHistoryPoints,
  resolveHistoryRetentionHours,
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

test('clamps selectable retention to the supported window', () => {
  assert.equal(clampHistoryRetentionHours(0), 1);
  assert.equal(clampHistoryRetentionHours(6.4), 6);
  assert.equal(clampHistoryRetentionHours(999), 24);
  assert.equal(historyRetentionLimit(3), CORE_HISTORY_LIMIT * 3);
});

test('keeps configured retention when history metadata is unavailable', () => {
  assert.equal(resolveHistoryRetentionHours(0, 6), 6);
  assert.equal(resolveHistoryRetentionHours(undefined, 12), 12);
  assert.equal(resolveHistoryRetentionHours(3, 12), 3);
});

test('keeps 5-minute session fallback and renders full retained homepage history at 600 points', () => {
  assert.equal(SESSION_HISTORY_RETENTION_MS, 5 * 60 * 1000);
  assert.equal(SESSION_HISTORY_LIMIT, 60);

  const statusSource = readFileSync(new URL('../src/app/components/DeviceStatus.tsx', import.meta.url), 'utf8');
  const panelStart = statusSource.indexOf('const TemperatureHistoryPanel = memo');
  const panelEnd = statusSource.indexOf('/* ── Main component ── */');
  assert.ok(panelStart >= 0 && panelEnd > panelStart);
  const panelSource = statusSource.slice(panelStart, panelEnd);

  assert.match(panelSource, /downsampleHistoryPoints\(retainedPoints,\s*600\)/);
  assert.doesNotMatch(panelSource, /clipHistoryToRecentWindow/);
  assert.match(panelSource, /fanCurve\.history\.retention\.hours[\s\S]*?count:\s*retentionHours/);
  assert.match(statusSource, /retentionHours=\{historyRetentionHours\}/);
});

test('hydrates and syncs persisted temperatureHistoryEnabled without locking init on error', () => {
  const storeSource = readFileSync(new URL('../src/app/store/app-store.ts', import.meta.url), 'utf8');
  assert.match(storeSource, /initializeApp:[\s\S]*?temperatureHistoryEnabled:\s*typeof appConfig\.temperatureHistoryEnabled === 'boolean'/);
  assert.match(storeSource, /setConfig:[\s\S]*?temperatureHistoryEnabled/);
  assert.match(storeSource, /resyncCore:[\s\S]*?loadTemperatureHistory\(true\)/);

  const loadStart = storeSource.indexOf('loadTemperatureHistory: async');
  const loadCatchStart = storeSource.indexOf('} catch {', loadStart);
  const loadFinallyStart = storeSource.indexOf('} finally {', loadCatchStart);
  assert.ok(loadStart >= 0 && loadCatchStart > loadStart && loadFinallyStart > loadCatchStart);
  const loadCatch = storeSource.slice(loadCatchStart, loadFinallyStart);
  assert.doesNotMatch(loadCatch, /temperatureHistoryEnabled:\s*false/);
  assert.doesNotMatch(loadCatch, /temperatureHistoryInitialized:\s*true/);
});
