import test from 'node:test';
import assert from 'node:assert/strict';

import {
  LatestRequestGate,
  appendTimelineEvent,
  cancelPendingTabChange,
  completePendingTabChange,
  isBridgeStartingTelemetry,
  requestTabChange,
  resolveBridgeWarning,
} from '../src/app/store/app-store-logic.ts';

test('keeps the curve page mounted while a dirty draft awaits navigation confirmation', () => {
  const state = requestTabChange({
    activeTab: 'curve',
    curveDraftDirty: true,
    pendingTab: null,
  }, 'control');

  assert.deepEqual(state, {
    activeTab: 'curve',
    curveDraftDirty: true,
    pendingTab: 'control',
  });
});

test('completes or cancels a pending curve navigation without losing state silently', () => {
  const pending = {
    activeTab: 'curve',
    curveDraftDirty: true,
    pendingTab: 'about',
  };

  assert.deepEqual(completePendingTabChange(pending), {
    activeTab: 'about',
    curveDraftDirty: false,
    pendingTab: null,
  });
  assert.deepEqual(cancelPendingTabChange(pending), {
    activeTab: 'curve',
    curveDraftDirty: true,
    pendingTab: null,
  });
});

test('only the latest device context request may commit', () => {
  const gate = new LatestRequestGate();
  const first = gate.begin();
  const second = gate.begin();

  assert.equal(gate.isCurrent(first), false);
  assert.equal(gate.isCurrent(second), true);
  gate.invalidate();
  assert.equal(gate.isCurrent(second), false);
});

test('keeps recent timeline events while deduplicating repeated backend notifications', () => {
  const first = appendTimelineEvent([], { timestamp: 1_000, type: 'disconnect' });
  assert.deepEqual(first, [{ timestamp: 1_000, type: 'disconnect' }]);
  assert.equal(appendTimelineEvent(first, { timestamp: 2_000, type: 'disconnect' }), first);

  const reconnect = appendTimelineEvent(first, { timestamp: 2_100, type: 'reconnect' });
  assert.deepEqual(reconnect.at(-1), { timestamp: 2_100, type: 'reconnect' });

  const many = Array.from({ length: 105 }, (_, index) => ({ timestamp: index * 3_000, type: 'resume' }));
  assert.equal(many.reduce(appendTimelineEvent, []).length, 100);
});

test('suppresses transient startup bridge warm-up warnings while keeping real errors', () => {
  assert.equal(resolveBridgeWarning({ bridgeOk: true, bridgeMessage: 'ignored' }, 'fallback'), null);

  const byStage = { bridgeOk: false, telemetryFailureStage: 'starting', bridgeMessage: '桥接程序正在启动中' };
  assert.equal(isBridgeStartingTelemetry(byStage), true);
  assert.equal(resolveBridgeWarning(byStage, 'fallback'), null);

  const byMessage = { bridgeOk: false, bridgeMessage: '桥接程序正在启动中' };
  assert.equal(isBridgeStartingTelemetry(byMessage), true);
  assert.equal(resolveBridgeWarning(byMessage, 'fallback'), null);

  const realTimeout = {
    bridgeOk: false,
    telemetryFailureStage: 'timeout',
    bridgeMessage: '读取 GetTemperature 响应超时',
  };
  assert.equal(isBridgeStartingTelemetry(realTimeout), false);
  assert.equal(resolveBridgeWarning(realTimeout, 'fallback'), '读取 GetTemperature 响应超时');
  assert.equal(resolveBridgeWarning({ bridgeOk: false }, 'fallback'), 'fallback');
});
