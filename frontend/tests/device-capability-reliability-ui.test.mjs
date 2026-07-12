import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const status = readFileSync(new URL('../src/app/components/DeviceStatus.tsx', import.meta.url), 'utf8');
const page = readFileSync(new URL('../src/app/page.tsx', import.meta.url), 'utf8');
const scan = readFileSync(new URL('../src/app/components/settings/device-connection/DeviceConnectionScanPanel.tsx', import.meta.url), 'utf8');
const store = readFileSync(new URL('../src/app/store/app-store.ts', import.meta.url), 'utf8');

test('shows current device capabilities in the existing device header', () => {
  assert.match(page, /runtimeDeviceCapabilities=\{view\.runtimeDeviceCapabilities\}/);
  assert.match(status, /deviceStatus\.capabilities/);
  assert.match(status, /runtimeDeviceCapabilities/);
});

test('keeps the temperature status card focused on temperature only', () => {
  assert.doesNotMatch(status, /deviceStatus\.telemetryState/);
  assert.match(status, /ShieldCheck/);
});

test('announces asynchronous scan progress without changing the connection flow', () => {
  assert.match(scan, /role="status"/);
  assert.match(scan, /aria-live="polite"/);
});

test('keeps sensor choices when compact updates omit unchanged metadata', () => {
  assert.match(store, /cpuSensors: data\.cpuSensors \?\? current\.temperature\.cpuSensors/);
  assert.match(store, /gpuSensors: data\.gpuSensors \?\? current\.temperature\.gpuSensors/);
  assert.match(store, /gpuDevices: data\.gpuDevices \?\? current\.temperature\.gpuDevices/);
});
