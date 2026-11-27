import assert from 'node:assert/strict';
import test from 'node:test';

import {
  CURVE_UNDO_LIMIT,
  findClosestCurvePointIndex,
  pushCurveUndoSnapshot,
  shiftCurveSpeeds,
  snapCurveSpeedValue,
  syncCurveSpeedAtIndexWithAffected,
} from '../src/app/lib/curve-editor-helpers.ts';

test('findClosestCurvePointIndex supports fuzzy clicks and respects plot bounds', () => {
  const curve = [
    { temperature: 20, rpm: 1000 },
    { temperature: 40, rpm: 1500 },
    { temperature: 60, rpm: 2000 },
    { temperature: 80, rpm: 2500 },
    { temperature: 100, rpm: 3000 },
  ];
  const bounds = {
    left: 100,
    right: 500,
    top: 50,
    bottom: 250,
    xMin: 20,
    xMax: 100,
    yMin: 0,
    yMax: 4000,
  };

  // Point 2 (60C, 2000 RPM) sits at (px=300, py=150).
  assert.equal(findClosestCurvePointIndex(curve, 300, 150, bounds), 2);
  assert.equal(findClosestCurvePointIndex(curve, 322, 168, bounds), 2);
  // Same temperature column vertical tolerance up to 58px picks point 2.
  assert.equal(findClosestCurvePointIndex(curve, 310, 102, bounds), 2);
  // Far empty spot in plot box does not accidentally grab a distant node.
  assert.equal(findClosestCurvePointIndex(curve, 120, 60, bounds), null);
  // Click well outside plot box returns null.
  assert.equal(findClosestCurvePointIndex(curve, 20, 150, bounds), null);

  // BlackShark 4-point horizontal mode uses 44px radial threshold.
  assert.equal(findClosestCurvePointIndex(curve, 330, 175, bounds, { allowHorizontal: true }), 2);
  assert.equal(findClosestCurvePointIndex(curve, 310, 102, bounds, { allowHorizontal: true }), null);
});

test('snapCurveSpeedValue snaps by 50/100 RPM or 5/10% and clamps cleanly', () => {
  assert.equal(snapCurveSpeedValue(1483, 800, 4000, 'rpm', true, false), 1500);
  assert.equal(snapCurveSpeedValue(1470, 800, 4000, 'rpm', true, true), 1500);
  assert.equal(snapCurveSpeedValue(1430, 800, 4000, 'rpm', true, true), 1400);
  assert.equal(snapCurveSpeedValue(1483, 800, 4000, 'rpm', false, false), 1483);
  assert.equal(snapCurveSpeedValue(1483, 800, 4000, 'rpm', false, true), 1500);
  // BlackShark range clamping [1410, 4000]
  assert.equal(snapCurveSpeedValue(1412, 1410, 4000, 'rpm', true, false), 1410);

  // Percent mode
  assert.equal(snapCurveSpeedValue(47, 0, 100, 'percent', true, false), 45);
  assert.equal(snapCurveSpeedValue(47, 0, 100, 'percent', true, true), 50);
  assert.equal(snapCurveSpeedValue(47, 0, 100, 'percent', false, false), 47);
  assert.equal(snapCurveSpeedValue(47, 0, 100, 'percent', false, true), 45);
});

test('syncCurveSpeedAtIndexWithAffected tracks monotonic neighbor pushes', () => {
  const base = [
    { temperature: 20, rpm: 1000 },
    { temperature: 30, rpm: 1200 },
    { temperature: 40, rpm: 1500 },
    { temperature: 50, rpm: 1800 },
    { temperature: 60, rpm: 2200 },
  ];

  // Raising index 1 to 1900 should push indices 2 and 3 up.
  const pushedUp = syncCurveSpeedAtIndexWithAffected(base, 1, 1900, 800, 4000);
  assert.equal(pushedUp.changed, true);
  assert.deepEqual(pushedUp.affectedIndices, [2, 3]);
  assert.equal(pushedUp.curve[2].rpm, 1900);
  assert.equal(pushedUp.curve[3].rpm, 1900);

  // Lowering index 3 to 1100 should push indices 1 and 2 down.
  const pushedDown = syncCurveSpeedAtIndexWithAffected(base, 3, 1100, 800, 4000);
  assert.equal(pushedDown.changed, true);
  assert.deepEqual(pushedDown.affectedIndices, [1, 2]);
  assert.equal(pushedDown.curve[1].rpm, 1100);
  assert.equal(pushedDown.curve[2].rpm, 1100);

  // Non-pushing edit yields no affected indices.
  const neutral = syncCurveSpeedAtIndexWithAffected(base, 2, 1600, 800, 4000);
  assert.equal(neutral.changed, true);
  assert.deepEqual(neutral.affectedIndices, []);
});

test('shiftCurveSpeeds shifts all points safely and reports boundary saturation', () => {
  const curve = [
    { temperature: 20, rpm: 820 },
    { temperature: 40, rpm: 2000 },
    { temperature: 60, rpm: 3980 },
  ];

  const up = shiftCurveSpeeds(curve, 50, 800, 4000);
  assert.equal(up.changed, true);
  assert.deepEqual(up.curve.map((p) => p.rpm), [870, 2050, 4000]);

  const clampedFloor = shiftCurveSpeeds(
    [{ temperature: 20, rpm: 800 }, { temperature: 40, rpm: 800 }],
    -50,
    800,
    4000,
  );
  assert.equal(clampedFloor.changed, false);

  const percentShift = shiftCurveSpeeds(
    [{ temperature: 20, rpm: 10 }, { temperature: 40, rpm: 99 }],
    2,
    0,
    100,
  );
  assert.equal(percentShift.changed, true);
  assert.deepEqual(percentShift.curve.map((p) => p.rpm), [12, 100]);
});

test('pushCurveUndoSnapshot clones snapshots, deduplicates, and caps history depth', () => {
  const snapA = [{ temperature: 20, rpm: 1000 }];
  const snapB = [{ temperature: 20, rpm: 1200 }];

  let stack = pushCurveUndoSnapshot([], snapA);
  snapA[0].rpm = 9999; // external mutation must not corrupt stack
  assert.equal(stack[0][0].rpm, 1000);

  // Duplicate snapshot against top of stack should be ignored
  stack = pushCurveUndoSnapshot(stack, [{ temperature: 20, rpm: 1000 }]);
  assert.equal(stack.length, 1);

  stack = pushCurveUndoSnapshot(stack, snapB);
  assert.equal(stack.length, 2);

  let fullStack = [];
  for (let i = 0; i < CURVE_UNDO_LIMIT + 5; i += 1) {
    fullStack = pushCurveUndoSnapshot(fullStack, [{ temperature: 20, rpm: 1000 + i }]);
  }
  assert.equal(fullStack.length, CURVE_UNDO_LIMIT);
  assert.equal(fullStack[0][0].rpm, 1005);
});
