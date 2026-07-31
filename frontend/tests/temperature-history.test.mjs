import assert from 'node:assert/strict';
import test from 'node:test';

import { detectAbruptHistoryPoints } from '../src/app/lib/temperature-history.ts';

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
