import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import test from 'node:test';

const editorUrl = new URL('../src/app/components/ui/FanCurveEditor.tsx', import.meta.url);
const logicUrl = new URL('../src/app/components/ui/fan-curve-editor-logic.mts', import.meta.url);
const editorSource = existsSync(editorUrl) ? readFileSync(editorUrl, 'utf8') : '';
const uiSource = readFileSync(new URL('../src/app/components/ui/index.tsx', import.meta.url), 'utf8');
const mainCurveSource = readFileSync(new URL('../src/app/components/FanCurve.tsx', import.meta.url), 'utf8');
const omenSource = readFileSync(new URL('../../plugins/omen-fan/src/ui/index.js', import.meta.url), 'utf8');

test('shared curve logic snaps RPM and preserves a non-decreasing curve', async () => {
  assert.equal(existsSync(logicUrl), true, 'shared curve logic should exist');
  const { resampleFanCurve, updateFanCurvePointSpeed } = await import(logicUrl.href);
  const points = [
    { temperature: 40, rpm: 1600 },
    { temperature: 50, rpm: 2000 },
    { temperature: 60, rpm: 2400 },
  ];

  assert.deepEqual(
    updateFanCurvePointSpeed(points, 1, 2737, 1000, 6000, 100, true),
    [
      { temperature: 40, rpm: 1600 },
      { temperature: 50, rpm: 2700 },
      { temperature: 60, rpm: 2700 },
    ],
  );
  assert.deepEqual(
    updateFanCurvePointSpeed(points, 1, 900, 1000, 6000, 100, true),
    [
      { temperature: 40, rpm: 1000 },
      { temperature: 50, rpm: 1000 },
      { temperature: 60, rpm: 2400 },
    ],
  );
  assert.strictEqual(
    updateFanCurvePointSpeed(points, 1, 2000, 1000, 6000, 100, true),
    points,
    'moving to the current value should not dirty a controlled draft',
  );

  assert.deepEqual(
    resampleFanCurve(points, {
      minTemperature: 40,
      maxTemperature: 60,
      temperatureStep: 5,
      minSpeed: 1000,
      maxSpeed: 6000,
      speedStep: 100,
    }),
    [
      { temperature: 40, rpm: 1600 },
      { temperature: 45, rpm: 1800 },
      { temperature: 50, rpm: 2000 },
      { temperature: 55, rpm: 2200 },
      { temperature: 60, rpm: 2400 },
    ],
  );
});

test('shared editor owns chart dragging and accessible keyboard adjustment', () => {
  assert.notEqual(editorSource, '', 'shared FanCurveEditor should exist');
  assert.match(editorSource, /ResponsiveContainer/);
  assert.match(editorSource, /updateFanCurvePointSpeed/);
  assert.match(editorSource, /onPointerDown/);
  assert.match(editorSource, /event\.currentTarget\.focus\(\)/);
  assert.match(editorSource, /onKeyDown/);
  assert.match(editorSource, /role="slider"/);
  assert.match(editorSource, /aria-valuemin/);
  assert.match(editorSource, /prefers-reduced-motion/);
  assert.match(editorSource, /minWidth=\{0\}/);
  assert.match(editorSource, /initialDimension=\{\{ width: 1, height: 1 \}\}/);
});

test('main curve page and OMEN plugin use the same host curve editor', () => {
  assert.match(uiSource, /export \{ FanCurveEditor \}/);
  assert.match(mainCurveSource, /<FanCurveEditor/);
  assert.match(omenSource, /ui\.FanCurveEditor/);
  assert.doesNotMatch(omenSource, /function curveChart/);
  assert.doesNotMatch(omenSource, /h\('svg'/);
});

test('OMEN keeps independent CPU and GPU drafts and master-only activation feedback', () => {
  assert.match(omenSource, /setCurvePoints\(target/);
  assert.match(omenSource, /curveEditor\('cpu'/);
  assert.match(omenSource, /curveEditor\('gpu'/);
  assert.match(omenSource, /isMasterMode \? text\.activeCurve : text\.savedNotActive/);
  assert.match(omenSource, /currentTemperature:\s*target === 'cpu' \? status\.cpuTemp : status\.gpuTemp/);
  assert.match(omenSource, /ui\.resampleFanCurve/);
  assert.match(omenSource, /temperatureStep:\s*5/);
  assert.match(omenSource, /minTemperature:\s*40/);
  assert.match(omenSource, /maxTemperature:\s*100/);
  assert.doesNotMatch(omenSource, /omen-curve-point|function setCurvePoint\(/);
});
