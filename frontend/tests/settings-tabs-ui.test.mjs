import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/app/components/ControlPanel.tsx', import.meta.url), 'utf8');
const appShellSource = readFileSync(new URL('../src/app/components/AppShell.tsx', import.meta.url), 'utf8');
const fanCurveSource = readFileSync(new URL('../src/app/components/FanCurve.tsx', import.meta.url), 'utf8');
const fanControlSource = readFileSync(new URL('../src/app/components/settings/FanControlSection.tsx', import.meta.url), 'utf8');
const deviceFeatureSource = readFileSync(new URL('../src/app/components/settings/DeviceFeaturePanel.tsx', import.meta.url), 'utf8');
const systemSettingsSource = readFileSync(new URL('../src/app/components/settings/SystemSettingsSection.tsx', import.meta.url), 'utf8');
const styles = readFileSync(new URL('../src/app/globals.css', import.meta.url), 'utf8');

test('lazily mounts settings sections once behind an accessible segmented control', () => {
  assert.match(source, /type SettingsTab = 'device' \| 'fan' \| 'system'/);
  assert.match(source, /data-theme-ui="settings-tabs"/);
  assert.match(source, /role="tablist"/);
  assert.match(source, /const \[mountedSettingsTabs, setMountedSettingsTabs\]/);
  assert.match(source, /settingsTabs\.filter\(\(\{ id \}\) => mountedSettingsTabs\[id\]\)/);
  assert.match(source, /id=\{`settings-panel-\$\{id\}`\}/);
  assert.match(source, /aria-hidden=\{activeSettingsTab !== id\}/);
  assert.doesNotMatch(source, /\shidden=\{activeSettingsTab !==/);
});

test('gives settings tabs the same active depth and hover language as the sidebar', () => {
  assert.match(source, /data-theme-ui="settings-tab"/);
  assert.match(source, /text-primary/);
  assert.match(source, /text-sidebar-foreground\/62 hover:bg-sidebar-accent hover:text-sidebar-foreground/);
  assert.match(styles, /\[data-theme-ui="sidebar-item"\]\[aria-selected="true"\],\s*\.glacier-shell \[data-theme-ui="settings-tab"\]\[aria-selected="true"\]/);
  assert.match(styles, /\[data-theme-ui="settings-tab"\]:not\(\[aria-selected="true"\]\):hover/);
});

test('uses slightly faster page transitions without changing card reveal timing', () => {
  assert.match(appShellSource, /center:[\s\S]*?duration: 0\.18/);
  assert.match(appShellSource, /exit: \(direction: number\)[\s\S]*?duration: 0\.15/);
  assert.match(styles, /glacier-page-card-fade 0\.285s/);
});

test('uses a slightly faster page-style transition for settings sections', () => {
  assert.match(source, /SETTINGS_PANEL_ENTER_DURATION = 0\.17/);
  assert.match(source, /SETTINGS_PANEL_EXIT_DURATION = 0\.13/);
  assert.match(source, /active:[\s\S]*?opacity: reduceMotion \? 1 : \[0, 1\][\s\S]*?y: reduceMotion \? 0 : \[8, 0\][\s\S]*?duration: reduceMotion \? 0 : SETTINGS_PANEL_ENTER_DURATION/);
  assert.match(source, /inactive:[\s\S]*?opacity: reduceMotion \? 0 : \[1, 0\][\s\S]*?y: reduceMotion \? 0 : \[0, -6\][\s\S]*?duration: reduceMotion \? 0 : SETTINGS_PANEL_EXIT_DURATION/);
  assert.match(source, /initial=\{id === 'device' \? false : \{ opacity: 0, y: reduceMotion \? 0 : 8 \}\}/);
  assert.match(source, /ease: \[0\.22, 1, 0\.36, 1\]/);
  assert.match(source, /data-theme-ui="settings-panels"/);
  assert.match(source, /will-change-\[opacity,transform\]/);
  assert.doesNotMatch(source, /settings-panel-switch|AnimatePresence mode="wait"/);
  assert.doesNotMatch(styles, /glacier-settings-panel-rise|settings-panel-switch/);
});

test('omits curve-owned learning and temperature history controls from settings', () => {
  assert.doesNotMatch(fanControlSource, /controlPanel\.fan\.learningTitle/);
  assert.doesNotMatch(fanControlSource, /controlPanel\.fan\.temperatureHistoryTitle/);
  assert.doesNotMatch(fanControlSource, /getTemperatureHistory\(\)/);
  assert.doesNotMatch(fanControlSource, /setTemperatureHistoryEnabled\(/);
});

test('keeps manual gear controls on the curve page instead of settings', () => {
  assert.doesNotMatch(fanControlSource, /controlPanel\.fan\.manualGearTitle/);
  assert.doesNotMatch(fanControlSource, /handleManualGearApply/);
  assert.match(fanCurveSource, /isConnected && supportsManualGears/);
});

test('keeps curve profile management exclusively on the curve page', () => {
  assert.doesNotMatch(fanControlSource, /FanCurveProfileSelect/);
  assert.doesNotMatch(fanControlSource, /getFanCurveProfiles|setActiveFanCurveProfile/);
  assert.match(fanCurveSource, /setActiveFanCurveProfile/);
});

test('keeps connection and compatibility controls in the device section', () => {
  assert.match(source, /<DeviceFeaturePanel[\s\S]*?<DeviceConnectionSection/);
  assert.match(deviceFeatureSource, /children\?: ReactNode/);
  assert.doesNotMatch(systemSettingsSource, /DeviceConnectionSection/);
});

test('reports user-triggered settings failures', () => {
  assert.match(fanControlSource, /settingsOperationFailed/);
  assert.match(systemSettingsSource, /settingsOperationFailed/);
  assert.match(systemSettingsSource, /toast\.error/);
});

test('keeps the initial settings reveal ordered while section switches stay below the tabs', () => {
  assert.match(source, /data-theme-section="settings-page" data-page-reveal="cards"/);
  const overview = source.indexOf('data-theme-card="settings-overview"');
  const tabs = source.indexOf('data-theme-ui="settings-tabs"');
  const panels = source.indexOf('data-theme-ui="settings-panels"');
  const debugPanel = source.indexOf('<DeviceDebugPanel');
  assert.ok(overview >= 0 && overview < tabs);
  assert.ok(tabs >= 0 && tabs < panels);
  assert.ok(panels >= 0 && panels < debugPanel);
});

test('uses existing telemetry and theme hooks without waking an idle GPU', () => {
  assert.match(source, /temperature\?\.cpuPowerWatts/);
  assert.match(source, /temperature\?\.gpuPowerWatts/);
  assert.match(source, /gpuNotPolled/);
  assert.match(source, /data-theme-card="settings-overview-temperature"/);
  assert.match(source, /data-theme-card="settings-overview-device"/);
  assert.doesNotMatch(source, /data-theme-card="settings-overview-speed"/);
});

test('balances both overview cards and protects long device names', () => {
  assert.match(source, /data-theme-card="settings-overview-temperature" className="grid min-h-\[10rem\] grid-rows-2/);
  assert.match(source, /data-theme-card="settings-overview-device" className="grid min-h-\[10rem\] grid-rows-2/);
  assert.match(source, /title=\{overviewConnectionName\}/);
  assert.match(source, /line-clamp-2 break-words text-sm font-semibold leading-snug/);
  assert.doesNotMatch(source, /className="mt-auto grid grid-cols-2/);
});

test('aligns thermal metrics and keeps hardware model pills at the right edge', () => {
  assert.match(source, /model: temperature\?\.cpuModel\?\.trim\(\) \|\| ''/);
  assert.match(source, /model: temperature\?\.gpuModel\?\.trim\(\) \|\| ''/);
  assert.match(source, /data-theme-ui="settings-overview-model"/);
  assert.match(source, /title=\{model\}/);
  assert.match(source, /border-primary\/20 bg-background\/80/);
  assert.match(source, /shadow-black\/15 backdrop-blur-md/);
  assert.match(source, /data-theme-ui="settings-overview-metrics"/);
  assert.match(source, /grid-cols-\[1rem_2\.25rem_minmax\(3\.25rem,1fr\)_1rem_2\.25rem_minmax\(3\.25rem,1fr\)\]/);
});
