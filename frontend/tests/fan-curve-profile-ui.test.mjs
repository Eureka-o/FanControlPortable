import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/app/components/FanCurve.tsx', import.meta.url), 'utf8');
const styles = readFileSync(new URL('../src/app/globals.css', import.meta.url), 'utf8');
const pageSource = readFileSync(new URL('../src/app/page.tsx', import.meta.url), 'utf8');
const historyHookSource = readFileSync(new URL('../src/app/hooks/useTemperatureHistory.ts', import.meta.url), 'utf8');
const storeSource = readFileSync(new URL('../src/app/store/app-store.ts', import.meta.url), 'utf8');

test('keeps profile selection, creation, and management in the curve header', () => {
  assert.match(source, /data-curve-profile-toolbar/);
  assert.match(source, /data-curve-profile-row/);
  assert.match(source, /data-curve-profile-list/);
  assert.match(source, /px-4 text-center text-xs font-medium/);
  assert.match(source, /\[&::\-webkit-scrollbar\]:hidden/);
  assert.match(source, /group relative flex shrink-0 hover:z-10 focus-within:z-10/);
  assert.match(source, /absolute -right-\[6px\] -top-\[1px\] z-10 flex h-\[13px\] w-\[13px\]/);
  assert.match(source, /opacity-0[^']*group-hover:opacity-100/);
  assert.match(source, /group-hover:border-border group-hover:bg-muted\/65 group-hover:text-foreground/);
  assert.match(source, /border-border bg-card text-muted-foreground[^\"]*hover:border-destructive\/50 hover:text-destructive/);
  assert.match(source, /focus-visible:border-destructive\/50 focus-visible:text-destructive/);
  assert.doesNotMatch(source, /isActive\s*\?\s*'border-destructive\/50 bg-card text-destructive'/);
  assert.doesNotMatch(source, /bg-destructive\/10|opacity-100 hover:bg/);
  assert.doesNotMatch(source, /-ml-px flex h-\[13px\]/);
  assert.match(source, /<X className="h-2 w-2" \/>/);
  assert.match(source, /setCreateProfileDialogOpen\(true\)/);
  assert.match(source, /setManageProfilesDialogOpen\(true\)/);
  assert.doesNotMatch(source, /<FanCurveProfileSelect/);
  assert.doesNotMatch(source, /data-theme-card="curve-profiles"/);
  assert.doesNotMatch(source, /data-theme-card="curve-import-export"/);
});

test('stacks export and import sections in the profile manager', () => {
  assert.match(source, /data-profile-export-section/);
  assert.match(source, /data-profile-import-section/);
  assert.doesNotMatch(source, /grid-cols-1 gap-3 md:grid-cols-2/);
});

test('keeps profile rename and transfer actions compact and aligned', () => {
  assert.match(source, /DialogContent className="max-w-\[620px\]"/);
  assert.match(source, /data-profile-rename-row/);
  assert.match(source, /sm:grid-cols-\[minmax\(0,1fr\)_auto\]/);
  assert.match(source, /data-profile-transfer-header className="space-y-1"/);
  assert.match(source, /data-profile-export-actions className="grid grid-cols-1 gap-2 sm:grid-cols-2"/);
  assert.match(source, /rows=\{2\}/);
});

test('guards profile switches when the current curve has unsaved changes', () => {
  assert.match(source, /if \(hasUnsavedChanges\) \{\s*setPendingProfileId\(id\);\s*setProfileSwitchDialogOpen\(true\);\s*return;/);
  assert.match(source, /confirmProfileSwitch\('save'\)/);
  assert.match(source, /confirmProfileSwitch\('discard'\)/);
});

test('confirms deletion and keeps creating a profile from the current curve', () => {
  assert.match(source, /setDeleteProfileDialogOpen\(true\)/);
  assert.match(source, /setPendingDeleteProfileId\(profile\.id\)/);
  assert.match(source, /group-hover:opacity-100/);
  assert.match(source, /saveFanCurveProfile\('', safeName, localCurve, true\)/);
});

test('exports by clipboard or file and imports pasted, selected, or dropped content', () => {
  assert.match(source, /navigator\.clipboard\.writeText\(code\)/);
  assert.match(source, /apiService\.exportFanCurveProfilesToFile\(\)/);
  assert.doesNotMatch(source, /new Blob\(\[code\]/);
  assert.doesNotMatch(source, /anchor\.download/);
  assert.match(source, /type="file"/);
  assert.match(source, /onDrop=\{handleProfileFileDrop\}/);
  assert.match(source, /await file\.text\(\)/);
  assert.match(source, /value=\{importCode\}/);
});

test('reveals history upward and keeps focused-entry layout stable', () => {
  assert.match(source, /const initialFocusTarget = useRef\(focusTarget\)\.current/);
  assert.match(source, /!isConnected && !initialFocusTarget \? 'cards-delayed'/);
  assert.match(source, /initial=\{initialFocusTarget \? false : \{ opacity: 0, y: 8 \}\}/);
  assert.match(source, /initial=\{initialFocusTarget \? false : \{ opacity: 0, height: 0 \}\}/);
  assert.match(styles, /\[data-page-reveal="cards-reverse"\] > :nth-last-child\(2\)/);
  assert.match(styles, /\[data-page-reveal="cards-reverse"\] > :nth-last-child\(7\)/);
  assert.match(styles, /\[data-page-reveal="cards-delayed"\] > \* \{\s*animation-delay: 0\.09s;/);
  assert.doesNotMatch(source, /pageRevealReady|setPageRevealReady/);
});

test('keeps curve jump aligned until the target layout stabilizes', () => {
  assert.match(source, /const FOCUS_SCROLL_STABLE_FRAMES = 8/);
  assert.match(source, /const FOCUS_SCROLL_TIMEOUT_MS = 1_200/);
  assert.match(source, /if \(!target\) \{\s*onFocusHandled\(\);\s*return;/);
  assert.match(source, /target\.getBoundingClientRect\(\)\.top/);
  assert.match(source, /stableFrameCount >= FOCUS_SCROLL_STABLE_FRAMES/);
  assert.match(source, /window\.requestAnimationFrame\(scrollUntilStable\)/);
});

test('shows only history and never normalizes or saves a curve while disconnected', () => {
  assert.match(source, /\{isConnected && \(\s*<>\s*<motion\.div\s*data-theme-card="curve-header"/s);
  assert.match(source, /<\/>,?\s*\)\}\s*<section ref=\{historyDetailsRef\} data-theme-card="curve-history"/s);
  assert.match(source, /if \(!isConnected && focusTarget !== 'history-details'\) \{\s*onFocusHandled\(\);\s*return;/);
  assert.match(source, /setCurveDraftDirty\(isConnected && hasUnsavedChanges\)/);
  assert.match(source, /if \(!isConnected \|\| isSaving\) return false;/);
  assert.match(source, /if \(!isConnected\) return;\s*if \(\(!isInitialized \|\| !hasUnsavedChanges\)/);
  assert.match(source, /if \(!isConnected\) return;\s*loadCurveProfiles\(\)\.catch/);
});

test('uses normal curve navigation and keeps history ownership outside page mounts', () => {
  assert.match(pageSource, /onOpenCurveEditor=\{\(\) => setActiveTab\('curve'\)\}/);
  assert.match(pageSource, /onOpenHistoryDetails=\{\(\) => openCurveTab\('history-details'\)\}/);
  assert.doesNotMatch(historyHookSource, /apiService|useEffect|useState|useRef|startTransition/);
  assert.match(historyHookSource, /state\.temperatureHistoryPoints/);
  assert.match(historyHookSource, /state\.setTemperatureHistoryEnabled/);
  assert.match(storeSource, /temperatureHistoryInitialized: boolean/);
  assert.match(storeSource, /temperatureHistoryPoints: TemperatureHistoryPoint\[\]/);
  assert.match(storeSource, /if \(!force && \(state\.temperatureHistoryInitialized \|\| state\.temperatureHistoryLoading\)\)/);
  assert.match(storeSource, /apiService\.onTemperatureHistoryUpdate/);
  assert.match(storeSource, /void get\(\)\.loadTemperatureHistory\(\)/);
  assert.match(styles, /\[data-theme-card="curve-history"\] \[data-theme-ui="switch-thumb"\][^}]+transition: none;/s);
});

test('splits power history into an aligned conditional chart with a shared full-point tooltip', () => {
  assert.match(source, /data-history-chart="thermal-fan"/);
  assert.match(source, /data-history-chart="power"/);
  assert.match(source, /const renderHistoryTooltip = useCallback/);
  assert.match(source, /const point = payload\?\.\[0\]\?\.payload/);
  assert.match(source, /const showHistoryPowerChart = historyHasPower && \(historySeriesVisibility\.cpuPower \|\| historySeriesVisibility\.gpuPower \|\| historySeriesVisibility\.totalPower\)/);
  assert.equal([...source.matchAll(/margin=\{\{ top: 12, right: 16, left: 4, bottom: 8 \}\}/g)].length, 2);
});

test('lets the history detail page select one time range for both trend charts', () => {
  assert.match(source, /const \[historyZoomDomain, setHistoryZoomDomain\] = useState/);
  assert.match(source, /const \[historyZoomSelect, setHistoryZoomSelect\] = useState/);
  assert.equal([...source.matchAll(/data=\{smoothedHistoryChartData\}/g)].length, 2);
  assert.equal([...source.matchAll(/syncId="historyTrend"/g)].length, 2);
  assert.equal([...source.matchAll(/onMouseDown=\{handleHistoryZoomMouseDown\}/g)].length, 2);
  assert.equal([...source.matchAll(/onMouseMove=\{handleHistoryZoomMouseMove\}/g)].length, 2);
  assert.equal([...source.matchAll(/<ReferenceArea/g)].length, 2);
  assert.equal([...source.matchAll(/fill="var\(--chart-primary\)"/g)].length, 2);
  assert.match(source, /stroke="var\(--chart-primary-active\)"/);
  assert.match(source, /onDoubleClick=\{resetHistoryZoom\}/);
  assert.match(source, /onClick=\{resetHistoryZoom\}/);
});

test('defers all offscreen history charts until history approaches the viewport', () => {
  assert.match(source, /const \[historyChartsReady, setHistoryChartsReady\] = useState\(false\)/);
  assert.match(source, /new IntersectionObserver/);
  assert.match(source, /observer\.observe\(historyDetailsRef\.current\)/);
  assert.match(source, /historyChartsReady \? \(\s*historyChartData\.length < 2/s);
  assert.doesNotMatch(source, /historyPowerChartReady/);
});

test('provides positive initial dimensions for every responsive chart', () => {
  const responsiveContainers = [...source.matchAll(/<ResponsiveContainer\b[^>]*>/g)].map((match) => match[0]);
  assert.equal(responsiveContainers.length, 3);
  for (const container of responsiveContainers) {
    assert.match(container, /initialDimension=\{\{ width: [1-9]\d*, height: [1-9]\d* \}\}/);
  }
});
