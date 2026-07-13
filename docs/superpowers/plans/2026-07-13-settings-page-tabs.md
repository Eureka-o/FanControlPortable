# Settings Page Tabs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the existing settings sections into three persistent tab panels and replace the live overview with a two-column temperature/power and device summary.

**Architecture:** Keep `ControlPanel` as the composition owner. Add one local tab state, leave all three existing settings sections mounted behind `hidden`, and keep the overview, offline notice, and debug panel outside the tab panels. Reuse existing telemetry, device data, translations, and theme hooks; add only two metric labels to each locale.

**Tech Stack:** React 19, TypeScript, Tailwind CSS, i18next, Node test runner, Wails build pipeline.

---

### Task 1: Lock The Layout Contract

**Files:**
- Create: `frontend/tests/settings-tabs-ui.test.mjs`
- Test: `frontend/tests/settings-tabs-ui.test.mjs`

- [x] **Step 1: Write the failing source contract test**

```javascript
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/app/components/ControlPanel.tsx', import.meta.url), 'utf8');

test('keeps three settings sections mounted behind an accessible segmented control', () => {
  assert.match(source, /type SettingsTab = 'device' \| 'fan' \| 'system'/);
  assert.match(source, /data-theme-ui="settings-tabs"/);
  assert.match(source, /role="tablist"/);
  assert.match(source, /id="settings-panel-device"[\s\S]*hidden=/);
  assert.match(source, /id="settings-panel-fan"[\s\S]*hidden=/);
  assert.match(source, /id="settings-panel-system"[\s\S]*hidden=/);
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
```

- [x] **Step 2: Run the test and verify it fails**

Run: `node --test frontend/tests/settings-tabs-ui.test.mjs`

Expected: FAIL because the tab type, tab panels, and new overview do not exist yet.

### Task 2: Implement The Overview And Tabs

**Files:**
- Modify: `frontend/src/app/components/ControlPanel.tsx`
- Modify: `frontend/src/app/locales/zh-CN/translation.json`
- Modify: `frontend/src/app/locales/en-US/translation.json`
- Modify: `frontend/src/app/locales/ja-JP/translation.json`
- Test: `frontend/tests/settings-tabs-ui.test.mjs`

- [x] **Step 1: Add the local tab state and telemetry formatting**

Add the tab type and initialize it inside `ControlPanel`:

```tsx
type SettingsTab = 'device' | 'fan' | 'system';

const [activeSettingsTab, setActiveSettingsTab] = useState<SettingsTab>('device');
```

Format positive telemetry without adding a shared abstraction:

```tsx
const formatTemperature = (value?: number, unavailable = false) => (
  !unavailable && Number.isFinite(value) && Number(value) > 0 ? `${Math.round(Number(value))}\u00b0C` : '--'
);
const formatPowerWatts = (value?: number, unavailable = false) => {
  const watts = Number(value || 0);
  if (unavailable || !Number.isFinite(watts) || watts <= 0) return '--';
  return `${watts < 10 ? Math.round(watts * 10) / 10 : Math.round(watts)} W`;
};
```

- [x] **Step 2: Replace the overview grid**

Keep `data-theme-card="settings-overview"`. Replace its three equal tiles with:

```tsx
<div className="grid grid-cols-1 gap-4 md:grid-cols-[minmax(0,1.2fr)_minmax(220px,0.8fr)]">
  <div data-theme-card="settings-overview-temperature" className="divide-y divide-border/55 rounded-xl border border-border/70 bg-muted/30 px-4">
    {overviewThermals.map(({ id, label, Icon, temperatureValue, powerValue }) => (
      <div key={id} className="flex items-center gap-3 py-4">
        <Icon className="h-5 w-5" />
        <div className="min-w-0 flex-1">{label}</div>
        <span>{temperatureValue}</span>
        <span>{powerValue}</span>
      </div>
    ))}
  </div>
  <div data-theme-card="settings-overview-device" className="rounded-xl border border-border/70 bg-muted/30 p-4">
    <div>{overviewConnectionName}</div>
    <div>{overviewConnectionDetail}</div>
    <div>{overviewFanSpeed ?? '--'}{overviewSpeedLabel}</div>
    <div>{config.autoControl ? t('appShell.status.smartControl') : t('appShell.status.manualMode')}</div>
  </div>
</div>
```

Use `formatTemperature` and `formatPowerWatts`; pass `gpuNotPolled` to both GPU values so an idle dGPU displays `--`.

- [x] **Step 3: Add the segmented control and persistent tab panels**

Use the existing section title translations for tab labels:

```tsx
const settingsTabs = [
  { id: 'device' as const, label: t('controlPanel.device.sectionTitle') },
  { id: 'fan' as const, label: t('controlPanel.fan.sectionTitle') },
  { id: 'system' as const, label: t('controlPanel.system.sectionTitle') },
];
```

Render a `role="tablist"` control with three buttons using `aria-selected` and `aria-controls`. Wrap each existing section in a `role="tabpanel"` container with `hidden={activeSettingsTab !== id}`. Leave the offline notice and `DeviceDebugPanel` after all three panel containers.

- [x] **Step 4: Add the two metric labels to all locales**

Add under `controlPanel.overview`:

```json
"temperatureMetric": "温度",
"powerMetric": "功耗"
```

Use `Temperature` / `Power` for English and `温度` / `電力` for Japanese.

- [x] **Step 5: Run the focused test and TypeScript check**

Run: `node --test frontend/tests/settings-tabs-ui.test.mjs`

Expected: 3 tests pass.

Run: `cd frontend; npx tsc --noEmit`

Expected: exit code 0.

### Task 3: Verify And Package

**Files:**
- Verify: `frontend/src/app/components/ControlPanel.tsx`
- Verify: `frontend/tests/settings-tabs-ui.test.mjs`
- Verify: `themes/dune/theme.css`
- Verify: `themes/shinchan/theme.css`
- Verify: `themes/xiaoba-deluxe/theme.css`

- [x] **Step 1: Run the full frontend test suite**

Run: `node --test frontend/tests/*.test.mjs`

Expected: all tests pass.

- [x] **Step 2: Build the frontend**

Run: `cd frontend; npm run build`

Expected: Next.js production build exits successfully.

- [x] **Step 3: Verify the scoped diff**

Run: `git diff --check`

Expected: no whitespace errors.

Run: `git status --short`

Expected: only the plan, `ControlPanel`, three locale files, and the focused test are changed; existing untracked `Cache/` and `plugins/` remain untouched.

- [x] **Step 4: Build Windows artifacts**

Run: `cmd /c build.bat`

Expected: `build/bin/FanControl-2.5.0-amd64-installer.exe` and `build/bin/FanControl-2.5.0-portable.zip` are produced.

- [x] **Step 5: Commit the implementation**

```powershell
git add -- frontend/src/app/components/ControlPanel.tsx frontend/src/app/locales/zh-CN/translation.json frontend/src/app/locales/en-US/translation.json frontend/src/app/locales/ja-JP/translation.json frontend/tests/settings-tabs-ui.test.mjs docs/superpowers/plans/2026-07-13-settings-page-tabs.md
git commit -m "feat(settings): split control panel into tabs"
```
