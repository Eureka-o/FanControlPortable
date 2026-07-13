import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const status = readFileSync(new URL('../src/app/components/DeviceStatus.tsx', import.meta.url), 'utf8');
const scan = readFileSync(new URL('../src/app/components/settings/device-connection/DeviceConnectionScanPanel.tsx', import.meta.url), 'utf8');
const connection = readFileSync(new URL('../src/app/components/settings/device-connection/DeviceConnectionPanel.tsx', import.meta.url), 'utf8');
const compatibility = readFileSync(new URL('../src/app/components/settings/device-connection/DeviceCompatibilityPanel.tsx', import.meta.url), 'utf8');
const store = readFileSync(new URL('../src/app/store/app-store.ts', import.meta.url), 'utf8');

test('keeps device capability tags out of the home header', () => {
  assert.doesNotMatch(status, /deviceStatus\.capabilities/);
  assert.doesNotMatch(status, /capabilityLabels/);
});

test('keeps the temperature status card focused on temperature only', () => {
  assert.doesNotMatch(status, /deviceStatus\.telemetryState/);
  assert.match(status, /ShieldCheck/);
});

test('announces asynchronous scan progress without changing the connection flow', () => {
  assert.match(scan, /role="status"/);
  assert.match(scan, /aria-live="polite"/);
});

test('keeps connected device details in the settings overview instead of a second pill', () => {
  assert.doesNotMatch(scan, /currentDeviceName|currentDeviceDetail/);
  assert.doesNotMatch(connection, /currentDeviceSummary/);
});

test('gives compatibility rows more room without changing their controls', () => {
  assert.match(compatibility, /className="space-y-3 pt-4"/);
  assert.match(compatibility, /onWiFiCompatibilityChange/);
  assert.match(compatibility, /onSerialCompatibilityChange/);
});

test('keeps sensor choices when compact updates omit unchanged metadata', () => {
  assert.match(store, /cpuSensors: data\.cpuSensors \?\? current\.temperature\.cpuSensors/);
  assert.match(store, /gpuSensors: data\.gpuSensors \?\? current\.temperature\.gpuSensors/);
  assert.match(store, /gpuDevices: data\.gpuDevices \?\? current\.temperature\.gpuDevices/);
});
