import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const status = readFileSync(new URL('../src/app/components/DeviceStatus.tsx', import.meta.url), 'utf8');
const shell = readFileSync(new URL('../src/app/components/AppShell.tsx', import.meta.url), 'utf8');
const page = readFileSync(new URL('../src/app/page.tsx', import.meta.url), 'utf8');
const control = readFileSync(new URL('../src/app/components/ControlPanel.tsx', import.meta.url), 'utf8');
const advanced = readFileSync(new URL('../src/app/components/AdvancedDevicesPanel.tsx', import.meta.url), 'utf8');
const editor = readFileSync(new URL('../src/app/components/devices/DeviceProfileEditorDialog.tsx', import.meta.url), 'utf8');
const store = readFileSync(new URL('../src/app/store/app-store.ts', import.meta.url), 'utf8');
const powerTrendChart = status.slice(status.indexOf('const PowerTrendChart'), status.indexOf('const MonitorInfoCard'));

test('clears device identity immediately when the backend reports a disconnect', () => {
  const handler = store.slice(store.indexOf('apiService.onDeviceDisconnected'), store.indexOf('apiService.onDeviceSettingsUpdate'));
  assert.match(handler, /applyDeviceSnapshotEvent\(state, \{ type: 'disconnected' \}\)/);
  assert.doesNotMatch(handler, /setTimeout/);
  assert.match(store, /applyDeviceSnapshotEvent\(state, \{ type: 'status', status \}\)/);
});

test('keeps the existing configured device display while runtime state is disconnected', () => {
  assert.match(status, /const configuredDeviceProfile = useMemo/);
  assert.match(status, /const activeDeviceProfile = runtimeDeviceProfile \|\| configuredDeviceProfile/);
  assert.match(shell, /\|\|\s*\(config as any\)\.deviceTransport/);
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

test('uses a dedicated two-card monitor-only dashboard without fan controls', () => {
  assert.match(status, /monitorOnlyActive && !isConnected/);
  assert.match(status, /<MonitorInfoCard/);
  assert.match(status, /deviceStatus\.monitorOnly\.cpuInfo/);
  assert.match(status, /deviceStatus\.monitorOnly\.gpuInfo/);
  assert.match(status, /formatPowerWatts\(temperature\?\.cpuPowerWatts\)/);
  assert.match(status, /formatGpuPowerWatts\(temperature\?\.gpuPowerWatts, gpuReadState\)/);
  assert.match(status, /const PowerTrendChart = memo/);
  assert.match(status, /POWER_TREND_WINDOW_MS = 60 \* 1000/);
  assert.match(powerTrendChart, /const recent = clipHistoryToRecentWindow\(points, POWER_TREND_WINDOW_MS\)/);
  assert.match(powerTrendChart, /const xFor = \(timestamp: number\)/);
  assert.match(powerTrendChart, /x: xFor\(point\.timestamp\)/);
  assert.doesNotMatch(powerTrendChart, /downsampleHistoryPoints/);
  assert.match(status, /powerPoints=\{temperatureHistory\}/);
  assert.match(status, /powerSeries="cpu"/);
  assert.match(status, /powerSeries="gpu"/);
  assert.match(status, /grid-cols-\[minmax\(9rem,0\.9fr\)_minmax\(0,1\.1fr\)\]/);
  assert.match(status, /<TempGaugeDisplay temp=\{temperature\} ready=\{ready\} idleLabel=\{idleLabel\} \/>/);
  assert.match(status, /role="img" aria-label=\{`\$\{label\}: \$\{value\}`\}/);
  assert.doesNotMatch(status, /role="progressbar"/);
  assert.match(status, /rounded-lg border border-border\/70 bg-background\/55/);
  assert.match(status, /const hasBridgeWarning = isConnected && temperature\?\.bridgeOk === false/);
  assert.match(status, /if \(!isConnected\) \{[\s\S]*?setActiveCurveProfileName\(''\)/);
  assert.match(status, /data-theme-section="control-protection"/);
  assert.match(status, /\{isConnected && \([\s\S]*?data-theme-section="control-protection"/);
  assert.match(status, /HISTORY_SERIES_ORDER\.filter\(\(series\) => series !== 'fan'\)/);
});

test('merges the curve history into the monitor-only homepage and hides its dock tab', () => {
  assert.match(shell, /monitorOnlyActive\?: boolean/);
  assert.match(shell, /data-monitor-only=\{monitorOnlyActive \? "true" : "false"\}/);
  assert.match(shell, /MAIN_TAB_ITEMS\.filter\(\(tab\) => !\(monitorOnlyActive && tab\.id === "curve"\)\)/);
  assert.match(page, /const effectiveActiveTab = view\.monitorOnlyActive && view\.activeTab === 'curve'/);
  assert.match(page, /activeTab=\{effectiveActiveTab\}/);
  assert.match(page, /const curveContent = \(/);
  assert.doesNotMatch(page, /\{view\.monitorOnlyActive && <div className="mt-3">\{curveContent\}<\/div>\}/);
  assert.match(status, /<TemperatureHistoryPanel/);
  assert.match(status, /onOpen=\{monitorOnlyActive \? undefined : onOpenHistoryDetails\}/);
  assert.match(page, /statusContent=\{statusContent\}/);
  assert.match(shell, /data-monitor-only/);
  assert.match(readFileSync(new URL('../src/app/globals.css', import.meta.url), 'utf8'), /data-monitor-only="true"/);
  assert.match(status, /\{showMonitoringLayout && \(/);
  assert.match(status, /\{!monitorOnlyActive && \(/);
});

test('does not filter device library or profile controls by compatibility state', () => {
  assert.doesNotMatch(advanced, /allowWiFi/);
  assert.match(advanced, /setSupportedProfiles\(Array\.isArray\(supported\) \? supported : \[\]\)/);
  assert.doesNotMatch(advanced, /normalizeTransport\(profile\.transport\) !== 'wifi'/);
  assert.doesNotMatch(editor, /allowWiFi/);
  assert.match(editor, /\{ value: 'wifi', label: t\('advancedDevices\.transport\.wifi'\) \}/);
});
