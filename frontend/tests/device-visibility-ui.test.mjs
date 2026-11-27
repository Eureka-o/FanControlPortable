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
  assert.match(control, /className="grid grid-cols-4 gap-1 rounded-\[18px\]/);
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
  assert.match(status, /const hasBridgeWarning = isConnected && bridgeOk === false/);
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

test('shows GPU inactive label and dedicated card notice in auto mode across all locales', () => {
  assert.match(status, /const GpuIdleNotice = memo/);
  assert.match(status, /data-theme-ui="gpu-idle-note"/);
  assert.match(status, /const gpuAutoIdle = currentGpuReadMode === 'auto' && gpuReadState === 'notPolled'/);
  assert.match(status, /deviceStatus\.tempGauge\.gpuInactive/);
  assert.match(status, /deviceStatus\.tempGauge\.gpuInactiveNote/);
  assert.match(status, /deviceStatus\.tempGauge\.notReadHintAuto/);
  assert.match(status, /<GpuIdleNotice note=\{gpuAutoIdleNote\} tooltip=\{gpuNotPolledTooltip\} \/>/);
  assert.doesNotMatch(status, /notReadHintNever/);

  for (const locale of ['zh-CN', 'en-US', 'ja-JP']) {
    const messages = JSON.parse(readFileSync(new URL(`../src/app/locales/${locale}/translation.json`, import.meta.url), 'utf8'));
    const tempGauge = messages.deviceStatus?.tempGauge;
    assert.ok(tempGauge?.gpuInactive, `${locale} missing gpuInactive`);
    assert.ok(tempGauge?.gpuInactiveNote, `${locale} missing gpuInactiveNote`);
    assert.ok(tempGauge?.notReadHintAria, `${locale} missing notReadHintAria`);
    assert.ok(tempGauge?.notReadHintAuto, `${locale} missing notReadHintAuto`);
    assert.equal(tempGauge?.notReadHintNever, undefined, `${locale} should not define notReadHintNever`);
  }
});

test('treats sub-threshold CPU power as unavailable instead of rendering 0 W', () => {
  assert.match(status, /const MIN_VALID_POWER_WATTS = 0\.05/);
  assert.match(status, /value < MIN_VALID_POWER_WATTS\)\s*return '-- W'/);
  assert.match(status, /const cpuPowerUnavailable = cpuPowerParts\.value === '--'/);
  assert.match(status, /\{!cpuPowerUnavailable && \(\s*<span className="text-xs font-semibold text-muted-foreground\/80">\s*\{cpuPowerParts\.unit\}\s*<\/span>\s*\)\}/s);
  assert.match(status, /powerAvailable=\{cpuPowerAvailable\}/);
  assert.match(control, /watts < 0\.05\)\s*return '--'/);
});

test('activates sensor catalog only while TemperatureBaselineSection is mounted and supports manual GPU probe without layout shift', () => {
  const baseline = readFileSync(new URL('../src/app/components/settings/TemperatureBaselineSection.tsx', import.meta.url), 'utf8');
  assert.match(baseline, /apiService\.setSensorCatalogActive\(true\)/);
  assert.match(baseline, /apiService\.setSensorCatalogActive\(false\)/);
  assert.match(baseline, /apiService\.probeGpuSensors\(\)/);
  assert.equal((baseline.match(/flex h-8 items-center justify-between gap-2/g) ?? []).length, 2);
  assert.equal((baseline.match(/hintClassName="h-5 truncate leading-5"/g) ?? []).length, 2);
  assert.match(baseline, /className="h-7 shrink-0 px-2\.5 text-xs"/);
  assert.match(baseline, /export function formatFallbackSensorLabel/);
  assert.match(baseline, /formatFallbackSensorLabel\(selectedGpuSensor\)/);
  assert.match(baseline, /const gpuDeviceHint = gpuReadDisabled[\s\S]*?controlPanel\.fan\.gpuDeviceHintDisabled/);
  assert.match(baseline, /const gpuFooterText = gpuReadDisabled[\s\S]*?controlPanel\.fan\.gpuTemperatureDisabled/);

  for (const locale of ['zh-CN', 'en-US', 'ja-JP']) {
    const messages = JSON.parse(readFileSync(new URL(`../src/app/locales/${locale}/translation.json`, import.meta.url), 'utf8'));
    const fan = messages.controlPanel?.fan;
    assert.ok(fan?.probeGpuSensors, `${locale} missing probeGpuSensors`);
    assert.ok(fan?.probeGpuSensorsTooltip, `${locale} missing probeGpuSensorsTooltip`);
    assert.ok(fan?.probingGpuSensors, `${locale} missing probingGpuSensors`);
    assert.ok(fan?.gpuDeviceHintDisabled, `${locale} missing gpuDeviceHintDisabled`);
    assert.ok(fan?.gpuTemperatureDisabled, `${locale} missing gpuTemperatureDisabled`);
    assert.doesNotMatch(fan?.processorDeviceHintDetected ?? '', /TempBridge/);
    assert.doesNotMatch(fan?.processorDeviceHintWaiting ?? '', /TempBridge/);
  }
});

test('restores dark-mode orange accent and keeps thicker top-aligned SemiGauge cards', () => {
  const globalsCss = readFileSync(new URL('../src/app/globals.css', import.meta.url), 'utf8');
  const darkSection = globalsCss.slice(globalsCss.indexOf('.dark {'));
  assert.match(darkSection, /--ring:\s*#ff4b26;/);
  assert.match(darkSection, /--primary:\s*#ff4b26;/);
  assert.match(darkSection, /--chart-primary:\s*#ff4b26;/);
  assert.match(darkSection, /--sidebar-primary:\s*#ff4b26;/);

  const pathStrokes = status.match(/strokeWidth="12"/g) ?? [];
  assert.equal(pathStrokes.length, 2, 'SemiGauge track and progress arcs should use strokeWidth="12"');
  assert.match(status, /r="4\.5"[\s\S]*?strokeWidth="3"/);
  assert.match(status, /<div className="flex flex-1 items-start justify-center pt-3">\s*<TempGaugeDisplay temp=\{cpuDisplayTemp\}/);
  assert.match(status, /<div className="flex flex-1 flex-col items-center justify-between gap-2 pt-3">\s*<TempGaugeDisplay/);
  assert.match(status, /<div className="flex flex-1 items-start justify-center pt-3">\s*<FanSpeedDisplay/);
});
