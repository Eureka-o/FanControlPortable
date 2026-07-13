import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/app/components/ControlPanel.tsx', import.meta.url), 'utf8');

test('keeps three settings sections mounted behind an accessible segmented control', () => {
  assert.match(source, /type SettingsTab = 'device' \| 'fan' \| 'system'/);
  assert.match(source, /data-theme-ui="settings-tabs"/);
  assert.match(source, /role="tablist"/);
  assert.match(source, /id="settings-panel-device"[\s\S]*?hidden=\{activeSettingsTab !== 'device'\}/);
  assert.match(source, /id="settings-panel-fan"[\s\S]*?hidden=\{activeSettingsTab !== 'fan'\}/);
  assert.match(source, /id="settings-panel-system"[\s\S]*?hidden=\{activeSettingsTab !== 'system'\}/);
});

test('keeps overview and device debug outside the switchable panels', () => {
  const overview = source.indexOf('data-theme-card="settings-overview"');
  const firstPanel = source.indexOf('id="settings-panel-device"');
  const lastPanel = source.indexOf('id="settings-panel-system"');
  const debugPanel = source.indexOf('<DeviceDebugPanel');
  assert.ok(overview >= 0 && overview < firstPanel);
  assert.ok(lastPanel >= 0 && lastPanel < debugPanel);
});

test('uses existing telemetry and theme hooks without waking an idle GPU', () => {
  assert.match(source, /temperature\?\.cpuPowerWatts/);
  assert.match(source, /temperature\?\.gpuPowerWatts/);
  assert.match(source, /gpuNotPolled/);
  assert.match(source, /data-theme-card="settings-overview-temperature"/);
  assert.match(source, /data-theme-card="settings-overview-device"/);
  assert.doesNotMatch(source, /data-theme-card="settings-overview-speed"/);
});
