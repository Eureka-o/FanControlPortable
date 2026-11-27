import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/app/components/FanCurve.tsx', import.meta.url), 'utf8');
// recharts 图表组件/常量已拆进独立分包文件，图表内部实现断言读这份源码。
const chartSource = readFileSync(new URL('../src/app/components/fan-curve-charts.tsx', import.meta.url), 'utf8');
const styles = readFileSync(new URL('../src/app/globals.css', import.meta.url), 'utf8');
const pageSource = readFileSync(new URL('../src/app/page.tsx', import.meta.url), 'utf8');
const historyHookSource = readFileSync(new URL('../src/app/hooks/useTemperatureHistory.ts', import.meta.url), 'utf8');
const storeSource = readFileSync(new URL('../src/app/store/app-store.ts', import.meta.url), 'utf8');

test('keeps profile selection, creation, and management in the curve header', () => {
  assert.match(source, /data-curve-profile-toolbar/);
  assert.match(source, /data-curve-profile-row/);
  assert.match(source, /data-curve-profile-list/);
  assert.match(source, /onWheel=\{handleProfileListWheel\}/);
  assert.match(source, /profileListOverflow\.left/);
  assert.match(source, /profileListOverflow\.right/);
  assert.match(source, /\[&::\-webkit-scrollbar\]:hidden/);
  assert.doesNotMatch(source, /absolute -right-\[6px\] -top-\[1px\] z-10 flex h-\[13px\] w-\[13px\]/);
  assert.doesNotMatch(source, /<X className="h-2 w-2" \/>/);
  assert.match(source, /setCreateProfileDialogOpen\(true\)/);
  assert.match(source, /setManageProfilesDialogOpen\(true\)/);
  assert.doesNotMatch(source, /<FanCurveProfileSelect/);
  assert.doesNotMatch(source, /data-theme-card="curve-profiles"/);
  assert.doesNotMatch(source, /data-theme-card="curve-import-export"/);
});

test('keeps BlackShark four-point axis separate from the FlyDigi curve axis', () => {
  assert.match(source, /const BLACKSHARK_CURVE_AXIS_MIN_TEMP = 10/);
  assert.match(source, /const BLACKSHARK_CURVE_AXIS_MAX_TEMP = 100/);
  assert.match(source, /min: blackSharkGearCurveMode \? BLACKSHARK_CURVE_AXIS_MIN_TEMP : FAN_CURVE_MIN_TEMP/);
  assert.match(source, /max: blackSharkGearCurveMode \? BLACKSHARK_CURVE_AXIS_MAX_TEMP : FAN_CURVE_MAX_TEMP/);
});

test('stacks export and import sections in the profile manager', () => {
  assert.match(source, /data-profile-export-section/);
  assert.match(source, /data-profile-import-section/);
  assert.doesNotMatch(source, /grid-cols-1 gap-3 md:grid-cols-2/);
});

test('keeps profile rename, duplication, and transfer actions unified in a two-tab manager', () => {
  assert.match(source, /DialogContent className="max-w-\[640px\]"/);
  assert.match(source, /manageDialogTab === 'list'/);
  assert.match(source, /data-profile-manage-list/);
  assert.match(source, /data-profile-rename-row/);
  assert.match(source, /sm:grid-cols-\[minmax\(0,1fr\)_auto\]/);
  assert.match(source, /duplicateProfile/);
  assert.match(source, /data-profile-transfer-header className="space-y-1"/);
  assert.match(source, /data-profile-export-actions className="grid grid-cols-1 gap-2 sm:grid-cols-2"/);
  assert.match(source, /rows=\{2\}/);
});

test('guards profile switches when the current curve has unsaved changes', () => {
  assert.match(source, /requestCurveProfileSwitch\(editorSession, id, activeProfileId\)/);
  assert.match(source, /resolveCurveEditorSwitch\(editorSession, action, saveSucceeded\)/);
  assert.match(source, /if \(!isConnected \|\| !hasUnsavedChanges\) \{\s*completePendingTabChange\(\);/);
  assert.match(source, /confirmProfileSwitch\('save'\)/);
  assert.match(source, /confirmProfileSwitch\('discard'\)/);
});

test('confirms deletion and keeps creating a profile from the current curve', () => {
  assert.match(source, /data-profile-manage-list/);
  assert.match(source, /setDeleteProfileDialogOpen\(true\)/);
  assert.match(source, /setPendingDeleteProfileId\(profile\.id\)/);
  assert.match(source, /saveFanCurveProfile\('', safeName, localCurve, true\)/);
});

test('exports by clipboard or file and imports pasted, selected, or dropped content', () => {
  assert.match(source, /navigator\.clipboard\.writeText\(code\)/);
  assert.match(source, /apiService\.exportFanCurveProfilesToFile\(selectedExportProfileIds\)/);
  assert.match(source, /apiService\.exportFanCurveProfiles\(selectedExportProfileIds\)/);
  assert.match(source, /data-profile-export-picker/);
  assert.match(source, /fanCurve\.profiles\.selectAll/);
  assert.match(source, /type="checkbox"/);
  assert.match(source, /setSelectedExportProfileIds\(activeProfileId \? \[activeProfileId\] : \[\]\)/);
  assert.doesNotMatch(source, /fanCurve\.importExport\.current/);
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
  assert.match(source, /\{!historyOnly && isConnected && \(\s*<>\s*<m\.div\s*data-theme-card="curve-header"/s);
  assert.match(source, /historyOnly\?: boolean/);
  assert.match(source, /<\/>,?\s*\)\}\s*<section ref=\{historyDetailsRef\} data-theme-card="curve-history"/s);
  assert.match(source, /if \(!isConnected && focusTarget !== 'history-details'\) \{\s*onFocusHandled\(\);\s*return;/);
  assert.match(source, /setCurveDraftDirty\(isConnected && hasUnsavedChanges\)/);
  assert.match(source, /if \(!isConnected \|\| isSaving\) return false;/);
  assert.match(source, /if \(!isConnected\) return;\s*if \(\(!isInitialized \|\| !hasUnsavedChanges\)/);
  // `loadCurveProfiles` 的调用处在**黑鲨**下放开（离线也要取 —— 后端是纯 config 读，
  //   黑鲨要在连接瞬间就画出四条预设）；**非黑鲨保持原语义**（"不影响别的品牌"的收敛结果）。
  //   所以这里的判据是 `!isConnected && !isBlackSharkDevice`。
  assert.match(source, /if \(!isConnected && !isBlackSharkDevice\) return;\s*loadCurveProfiles\(\)\.catch/);
});

test('uses normal curve navigation and keeps history ownership outside page mounts', () => {
  assert.match(pageSource, /onOpenCurveEditor=\{\(\) => setActiveTab\('curve'\)\}/);
  assert.match(pageSource, /onOpenHistoryDetails=\{\(\) => openCurveTab\('history-details'\)\}/);
  assert.doesNotMatch(historyHookSource, /apiService|useEffect|useState|useRef|startTransition/);
  assert.match(historyHookSource, /useAppStore\(selectTemperatureHistoryPoints\)/);
  assert.match(historyHookSource, /state\.setTemperatureHistoryEnabled/);
  assert.match(storeSource, /temperatureHistoryInitialized: boolean/);
  assert.match(storeSource, /temperatureHistoryPoints: TemperatureHistoryPoint\[\]/);
  assert.match(storeSource, /if \(!force && \(state\.temperatureHistoryInitialized \|\| state\.temperatureHistoryLoading\)\)/);
  assert.match(storeSource, /apiService\.onTemperatureHistoryUpdate/);
  assert.match(storeSource, /void get\(\)\.loadTemperatureHistory\(\)/);
  assert.match(styles, /\[data-theme-card="curve-history"\]\s+\[data-theme-ui="switch-thumb"\][^}]+transition: none;/s);
});

test('splits power history into an aligned conditional chart with a shared full-point tooltip', () => {
  assert.match(source, /data-history-chart="thermal-fan"/);
  assert.match(source, /data-history-chart="power"/);
  assert.match(source, /const renderHistoryTooltip = useCallback/);
  assert.match(source, /const point = payload\?\.\[0\]\?\.payload/);
  assert.match(source, /const showHistoryPowerChart = historyHasPower && \(historySeriesVisibility\.cpuPower \|\| historySeriesVisibility\.gpuPower \|\| historySeriesVisibility\.totalPower\)/);
  // 历史图表的 margin 提升为模块级常量（两张图共享同一份，不再逐渲染新建对象）——
  // recharts 拆分后，常量与 margin 用法都在 fan-curve-charts.tsx 里。
  assert.match(chartSource, /const HISTORY_CHART_MARGIN = \{ top: 12, right: 16, left: 4, bottom: 8 \}/);
  assert.equal([...chartSource.matchAll(/margin=\{HISTORY_CHART_MARGIN\}/g)].length, 2);
});

test('lets the history detail page select one time range for both trend charts', () => {
  assert.match(source, /const \[historyZoomDomain, setHistoryZoomDomain\] = useState/);
  assert.match(source, /const \[historyZoomSelect, setHistoryZoomSelect\] = useState/);
  assert.equal([...source.matchAll(/data=\{historyDisplayData\}/g)].length, 2);
  assert.equal([...chartSource.matchAll(/syncId="historyTrend"/g)].length, 2);
  assert.equal([...source.matchAll(/onMouseDown=\{handleHistoryZoomMouseDown\}/g)].length, 2);
  assert.equal([...source.matchAll(/onMouseMove=\{handleHistoryZoomMouseMove\}/g)].length, 2);
  assert.equal([...chartSource.matchAll(/<ReferenceArea/g)].length, 3);
  assert.equal([...chartSource.matchAll(/fill="var\(--chart-primary\)"/g)].length, 2);
  assert.match(chartSource, /stroke="var\(--chart-primary-active\)"/);
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
  const responsiveContainers = [...chartSource.matchAll(/<ResponsiveContainer\b[^>]*>/g)].map((match) => match[0]);
  assert.equal(responsiveContainers.length, 3);
  for (const container of responsiveContainers) {
    assert.match(container, /initialDimension=\{\{ width: [1-9]\d*, height: [1-9]\d* \}\}/);
  }
});

test('integrates curve legend hints, collapsible strategy card, diverging learning bars, and responsive history grid', () => {
  const editorIndex = source.indexOf('data-theme-card="curve-editor"');
  const learningIndex = source.indexOf('data-theme-card="curve-learning"');
  const hintsIndex = source.indexOf('data-theme-ui="curve-hints"');
  assert.ok(editorIndex >= 0 && learningIndex > editorIndex);
  assert.ok(hintsIndex > editorIndex && hintsIndex < learningIndex);
  assert.doesNotMatch(source, /data-theme-card="curve-prediction"/);
  assert.doesNotMatch(source, /fanCurve\.prediction\.badge/);
  assert.match(source, /\{\!\!smartControl\.learning && !blackSharkGearCurveMode && \(/);
  assert.match(source, /data-theme-ui="learning-offset-bars"/);
  assert.match(source, /showValue=\{false\}/);
  assert.match(source, /handleApplyLearnedOffsetsToCurve/);
  assert.match(source, /await apiService\.saveFanCurveProfile\(profileID, profileName, mergedCurve, true\);\s*await apiService\.resetLearnedOffsets\(\)/s);
  assert.match(chartSource, /learningActive/);
  assert.match(chartSource, /comfortBandBounds/);
  assert.doesNotMatch(chartSource, /x=\{comfortBandBounds\.high\}/);
  assert.match(source, /grid-cols-2 gap-3 md:grid-cols-5/);
});

test('places thermal prediction above learning bias tabs, guards apply-learning and low-RPM, and validates manual gears on save', () => {
  const advancedSource = readFileSync(new URL('../src/app/components/AdvancedDevicesPanel.tsx', import.meta.url), 'utf8');
  const learningIndex = source.indexOf('data-theme-card="curve-learning"');
  const predictionIndex = source.indexOf('data-theme-ui="curve-prediction-row"');
  const biasTabsIndex = source.indexOf('data-theme-ui="learning-bias-tabs"');
  assert.ok(learningIndex >= 0 && predictionIndex > learningIndex && biasTabsIndex > predictionIndex);

  assert.match(source, /supportsLowRpmGuidance = speedUnit === 'rpm' && speedRange\.min < RPM_RECOMMENDED_MIN/);
  assert.match(source, /lowRpmDialogOpen/);
  assert.match(source, /applyLearningDialogOpen/);
  assert.match(source, /setApplyLearningDialogOpen\(true\)/);
  assert.match(source, /gearDraftValidation/);
  assert.match(source, /deferClamp/);
  assert.match(advancedSource, /data-theme-card="advanced-gate"/);
  assert.match(advancedSource, /<m\.div[\s\S]*data-theme-card="advanced-gate"/);

  for (const locale of ['zh-CN', 'en-US', 'ja-JP']) {
    const messages = JSON.parse(readFileSync(new URL(`../src/app/locales/${locale}/translation.json`, import.meta.url), 'utf8'));
    const dragHint = messages.fanCurve?.hints?.dragPoint || '';
    assert.ok(dragHint.length > 0);
    assert.doesNotMatch(dragHint, /蓝色|blue|ブルー/i);
    assert.ok(messages.fanCurve?.warning?.lowRpmBody, `${locale} missing lowRpmBody`);
    assert.ok(messages.fanCurve?.learning?.applyConfirmTitle, `${locale} missing applyConfirmTitle`);
    assert.ok(messages.fanCurve?.manualGear?.validationRangeOrder, `${locale} missing validationRangeOrder`);
    assert.ok(messages.fanCurve?.history?.splitTooltip?.title, `${locale} missing splitTooltip.title`);
    assert.ok(messages.fanCurve?.editor?.undo, `${locale} missing editor.undo`);
    assert.ok(messages.fanCurve?.editor?.snapLabel, `${locale} missing editor.snapLabel`);
    assert.ok(messages.fanCurve?.editor?.pointTunerTitle, `${locale} missing editor.pointTunerTitle`);
  }
});

test('splits history hover tooltips by chart and adds fuzzy hit-testing, point tuner, step snap, and undo', () => {
  assert.match(source, /data-theme-ui="history-split-tooltip-toggle"/);
  assert.match(source, /renderThermalHistoryTooltip/);
  assert.match(source, /renderPowerHistoryTooltip/);
  assert.match(source, /data-theme-ui="curve-editor-toolbar"/);
  assert.match(source, /data-theme-ui="curve-snap-toggle"/);
  assert.match(source, /data-theme-ui="curve-undo-button"/);
  assert.match(source, /data-theme-ui="curve-point-tuner"/);
  assert.match(source, /findClosestCurvePointIndex/);
  assert.match(source, /monotonicPushedSet/);
});
