import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const component = readFileSync(new URL('../src/app/components/NoiseDiagnostic.tsx', import.meta.url), 'utf8');
const utility = readFileSync(new URL('../src/app/lib/noise-diagnostic.ts', import.meta.url), 'utf8');
const fanCurve = readFileSync(new URL('../src/app/components/FanCurve.tsx', import.meta.url), 'utf8');
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
});

test('noise diagnosis is a device feature below curve learning', () => {
  const learningIndex = fanCurve.indexOf('data-theme-card="curve-learning"');
  const deviceNoiseIndex = fanCurve.indexOf('data-theme-card="device-noise"');
  assert.ok(learningIndex >= 0);
  assert.ok(deviceNoiseIndex > learningIndex);
  assert.doesNotMatch(fanCurve.slice(learningIndex, deviceNoiseIndex), /setNoiseDiagnosticOpen\(true\)/);
  assert.match(coreDiagnostic, /NoiseDiagnosticsByDevice\[result\.DeviceKey\] = result/);
});
