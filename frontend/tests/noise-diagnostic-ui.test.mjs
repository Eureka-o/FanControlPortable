import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { buildAxisNoiseRefinementSteps, buildDiagnosticSteps, confirmAxisNoiseSeverity, fanSpeedDisplaySuffix, noiseDiagnosticDeviceKey, NoiseMeter } from '../src/app/lib/noise-diagnostic.ts';

const component = readFileSync(new URL('../src/app/components/NoiseDiagnostic.tsx', import.meta.url), 'utf8');
const axisComponent = readFileSync(new URL('../src/app/components/AxisNoiseScan.tsx', import.meta.url), 'utf8');
const utility = readFileSync(new URL('../src/app/lib/noise-diagnostic.ts', import.meta.url), 'utf8');
const fanCurve = readFileSync(new URL('../src/app/components/FanCurve.tsx', import.meta.url), 'utf8');
const selectComponent = readFileSync(new URL('../src/components/ui/select.tsx', import.meta.url), 'utf8');
const coreDiagnostic = readFileSync(new URL('../../internal/coreapp/noise_diagnostic.go', import.meta.url), 'utf8');

test('noise diagnostic uses device-aware floors and runtime bounds', () => {
  assert.match(utility, /unit === 'percent' \? 5 : isFlyDigi \? 1000/);
  assert.match(utility, /Math\.min\(max, Number\(flyDigiCapability\.maxRpm\)\)/);
  assert.match(component, /setRange\(\{ \.\.\.range, min:/);
  assert.match(component, /setRange\(\{ \.\.\.range, max:/);
});

test('sweep automatically controls targets and waits for actual speed', () => {
  assert.match(component, /apiService\.setNoiseDiagnosticTarget/);
  assert.match(component, /buildDiagnosticSteps/);
  assert.match(component, /NoiseMeter\.open\(selectedMicrophone/);
});

test('cancel aborts microphone work and discards collected points', () => {
  assert.match(component, /abortRef\.current\?\.abort\(\)/);
  assert.match(component, /setPoints\(\[\]\)/);
  assert.match(component, /apiService\.cancelNoiseDiagnostic/);
  assert.match(component, /Stop and discard|noiseDiagnostic\.cancel/);
});

test('noise diagnosis stays separate from axis-noise avoidance', () => {
  assert.doesNotMatch(component, /setSpeedAvoidance|speedAvoidance|axisNoise|axis-noise/);
  assert.match(fanCurve, /<NoiseDiagnostic/);
  assert.match(fanCurve, /<AxisNoiseScan/);
  assert.match(axisComponent, /buildAxisNoiseRefinementSteps/);
  assert.match(axisComponent, /rateCurrent\('none'\)/);
  assert.match(axisComponent, /rateCurrent\('mild'\)/);
  assert.match(axisComponent, /rateCurrent\('obvious'\)/);
});

test('noise diagnosis is a device feature below curve learning', () => {
  const learningIndex = fanCurve.indexOf('data-theme-card="curve-learning"');
  const deviceNoiseIndex = fanCurve.indexOf('data-theme-card="device-noise"');
  assert.ok(learningIndex >= 0);
  assert.ok(deviceNoiseIndex > learningIndex);
  assert.doesNotMatch(fanCurve.slice(learningIndex, deviceNoiseIndex), /setNoiseDiagnosticOpen\(true\)/);
  assert.match(coreDiagnostic, /NoiseDiagnosticsByDevice\[result\.DeviceKey\] = result/);
  assert.match(coreDiagnostic, /AxisNoiseProfilesByDevice\[deviceKey\] = profile/);
});

test('axis-noise refinement stays local and uses device minimum steps', () => {
  assert.deepEqual(
    buildAxisNoiseRefinementSteps({ unit: 'rpm', min: 1000, max: 3600, step: 1 }, 2000, [1000, 1500, 2000, 2500]),
    [1700, 1800, 1900, 2100, 2200, 2300],
  );
  assert.deepEqual(
    buildAxisNoiseRefinementSteps({ unit: 'percent', min: 5, max: 100, step: 1 }, 20, [5, 20, 35]),
    [15, 16, 17, 18, 19, 21, 22, 23, 24, 25],
  );
  assert.equal(noiseDiagnosticDeviceKey({ transport: 'HID', id: 'flydigi.bs3' }), 'hid::flydigi.bs3');
});

test('manual axis-noise ratings require conservative confirmation and remain cancellable', () => {
  assert.equal(confirmAxisNoiseSeverity('obvious', 'obvious'), 'obvious');
  assert.equal(confirmAxisNoiseSeverity('obvious', 'mild'), 'mild');
  assert.equal(confirmAxisNoiseSeverity('mild', 'mild'), 'mild');
  assert.equal(confirmAxisNoiseSeverity('obvious', 'none'), 'none');
  assert.match(axisComponent, /pendingConfirmationRef/);
  assert.match(axisComponent, /cancelRequestedRef/);
  assert.match(axisComponent, /buildAxisNoiseRefinementSteps\(session\.range, current\.actual \|\| requested, stepsRef\.current\)/);
  assert.doesNotMatch(axisComponent, /refinedRef/);
  const stopButton = axisComponent.match(/<Button variant="danger"[^\n]+axisNoise\.stopDiscard[^\n]+<\/Button>/)?.[0] || '';
  assert.ok(stopButton);
  assert.doesNotMatch(stopButton, /disabled=\{busy\}/);
});

test('percent sweep spans the configured range and uses percent display units', () => {
  const steps = buildDiagnosticSteps({ unit: 'percent', min: 5, max: 100, step: 1 });
  assert.equal(steps[0], 5);
  assert.equal(steps.at(-1), 100);
  assert.ok(steps.length >= 5 && steps.length <= 10);
  assert.equal(fanSpeedDisplaySuffix('percent'), '%');
  assert.equal(fanSpeedDisplaySuffix('rpm'), 'RPM');
  assert.doesNotMatch(axisComponent, /unit\.toUpperCase\(\)/);
});

test('microphone selection survives device changes and opens above dialogs', () => {
  assert.match(component, /options\.some\(\(option\) => option\.deviceId === current\)/);
  assert.match(selectComponent, /zIndex: "calc\(var\(--layer-dialog-content\) \+ 1\)"/);
});

test('microphone enumeration survives a permission probe failure', async () => {
  const originalNavigator = globalThis.navigator;
  Object.defineProperty(globalThis, 'navigator', {
    configurable: true,
    value: {
      mediaDevices: {
        enumerateDevices: async () => [{ kind: 'audioinput', deviceId: 'mic-1', label: 'Desk Mic' }],
        getUserMedia: async () => { throw new Error('permission-pending'); },
      },
    },
  });
  try {
    assert.deepEqual(await NoiseMeter.listMicrophones(), [{ deviceId: 'mic-1', label: 'Desk Mic' }]);
  } finally {
    Object.defineProperty(globalThis, 'navigator', { configurable: true, value: originalNavigator });
  }
});
