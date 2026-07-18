import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const status = readFileSync(new URL('../src/app/components/DeviceStatus.tsx', import.meta.url), 'utf8');
const shell = readFileSync(new URL('../src/app/components/AppShell.tsx', import.meta.url), 'utf8');
const control = readFileSync(new URL('../src/app/components/ControlPanel.tsx', import.meta.url), 'utf8');
const advanced = readFileSync(new URL('../src/app/components/AdvancedDevicesPanel.tsx', import.meta.url), 'utf8');
const editor = readFileSync(new URL('../src/app/components/devices/DeviceProfileEditorDialog.tsx', import.meta.url), 'utf8');
const store = readFileSync(new URL('../src/app/store/app-store.ts', import.meta.url), 'utf8');

test('clears device identity immediately when the backend reports a disconnect', () => {
  const handler = store.slice(store.indexOf('apiService.onDeviceDisconnected'), store.indexOf('apiService.onDeviceSettingsUpdate'));
  assert.match(handler, /isConnected: false/);
  assert.match(handler, /runtimeDeviceProfile: null/);
  assert.match(handler, /fanData: null/);
  assert.doesNotMatch(handler, /setTimeout/);
  assert.match(store, /const connected = status\?\.connected === true/);
});

test('keeps the existing configured device display while runtime state is disconnected', () => {
  assert.match(status, /const configuredDeviceProfile = useMemo/);
  assert.match(status, /const activeDeviceProfile = runtimeDeviceProfile \|\| configuredDeviceProfile/);
  assert.match(shell, /\|\| \(config as any\)\.deviceTransport/);
  assert.match(shell, /<WifiOff className="h-3\.5 w-3\.5"/);
  assert.doesNotMatch(shell, /\{isConnected && \([\s\S]*?appShell\.status\.smartControl/);
});

test('keeps all three settings sections and the existing device surfaces visible', () => {
  assert.match(control, /const effectiveDeviceProfile = isConnected && runtimeDeviceProfile \? runtimeDeviceProfile : currentDeviceProfile/);
  assert.match(control, /\{ id: 'fan', label: t\('controlPanel\.fan\.sectionTitle'\) \}/);
  assert.match(control, /className="grid grid-cols-3 gap-1 rounded-\[18px\]/);
  assert.match(control, /<div data-theme-card="settings-overview-device"/);
  assert.doesNotMatch(control, /\{isConnected && \([\s\S]*?data-theme-card="settings-overview-device"/);
  assert.match(control, /<DeviceDebugPanel/);
});

test('does not filter device library or profile controls by compatibility state', () => {
  assert.doesNotMatch(advanced, /allowWiFi/);
  assert.match(advanced, /setSupportedProfiles\(Array\.isArray\(supported\) \? supported : \[\]\)/);
  assert.doesNotMatch(advanced, /normalizeTransport\(profile\.transport\) !== 'wifi'/);
  assert.doesNotMatch(editor, /allowWiFi/);
  assert.match(editor, /\{ value: 'wifi', label: t\('advancedDevices\.transport\.wifi'\) \}/);
});
