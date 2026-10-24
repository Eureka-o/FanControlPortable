# Split History Charts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render temperature/fan and power history as aligned charts whose tooltips show all enabled values from the hovered sample.

**Architecture:** Keep the existing history data and visibility preference hook. Split the current three-axis chart into two Recharts `LineChart` instances with identical margins and axis reservations, and reuse one full-point tooltip callback in both charts.

**Tech Stack:** React 19, TypeScript, Recharts, Tailwind CSS, Node test runner.

---

### Task 1: Add the split-chart regression check

**Files:**
- Modify: `frontend/tests/fan-curve-profile-ui.test.mjs`

- [ ] **Step 1: Write the failing test**

Add a source-level check that requires two named chart regions, a shared tooltip callback based on `payload[0].payload`, matching margins, and the conditional power region:

```javascript
test('splits power history into an aligned conditional chart with a shared full-point tooltip', () => {
  assert.match(source, /data-history-chart="thermal-fan"/);
  assert.match(source, /data-history-chart="power"/);
  assert.match(source, /const renderHistoryTooltip = useCallback/);
  assert.match(source, /const point = payload\?\.\[0\]\?\.payload/);
  assert.match(source, /const showHistoryPowerChart = historyHasPower && \(historySeriesVisibility\.cpuPower \|\| historySeriesVisibility\.gpuPower\)/);
  assert.equal([...source.matchAll(/margin=\{\{ top: 12, right: 16, left: 4, bottom: 8 \}\}/g)].length, 2);
});
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `cd frontend; node --test tests/fan-curve-profile-ui.test.mjs`

Expected: the new test fails because the named split chart regions do not exist.

### Task 2: Split the history charts

**Files:**
- Modify: `frontend/src/app/components/FanCurve.tsx`
- Modify: `frontend/src/app/locales/en-US/translation.json`
- Modify: `frontend/src/app/locales/ja-JP/translation.json`
- Modify: `frontend/src/app/locales/zh-CN/translation.json`
- Test: `frontend/tests/fan-curve-profile-ui.test.mjs`

- [ ] **Step 1: Add the shared tooltip callback and power visibility flag**

Build tooltip rows from the first Recharts payload item's full data point so either chart can show values from both charts:

```tsx
const showHistoryPowerChart = historyHasPower && (historySeriesVisibility.cpuPower || historySeriesVisibility.gpuPower);

const renderHistoryTooltip = useCallback(({ active, label, payload }: any) => {
  const point = payload?.[0]?.payload;
  if (!active || !point) return null;
  const rows = historySeriesMeta.flatMap((series) => {
    if (!historySeriesVisibility[series.key]) return [];
    const numericValue = Number(point[series.dataKey] ?? 0);
    if (!Number.isFinite(numericValue) || numericValue <= 0) return [];
    const value = series.key === 'fan'
      ? `${formatSpeedValue(numericValue)}${speedUnitSuffix}`
      : series.axisId === 'power'
        ? `${formatPowerValue(numericValue)} W`
        : `${numericValue} °C`;
    return [{ ...series, value }];
  });
  if (rows.length === 0) return null;
  return (
    <div className="rounded-lg border px-3 py-2 text-sm shadow-lg" style={{ backgroundColor: 'var(--chart-tooltip-bg)', borderColor: 'var(--chart-tooltip-border)', color: 'var(--chart-tooltip-text)' }}>
      <div className="mb-2 font-semibold">{formatHistoryDateTime(Number(label), locale)}</div>
      <div className="space-y-1.5">
        {rows.map((row) => (
          <div key={row.key} className="flex items-center gap-2">
            <span className="h-2 w-2 rounded-full" style={{ backgroundColor: row.color }} />
            <span>{row.label}：{row.value}</span>
          </div>
        ))}
      </div>
    </div>
  );
}, [historySeriesMeta, historySeriesVisibility, locale, speedUnitSuffix]);
```

- [ ] **Step 2: Replace the single chart with two aligned regions**

Use `data-history-chart="thermal-fan"` for the upper chart and render only non-power lines. Use `data-history-chart="power"` inside `showHistoryPowerChart` and render only visible power lines. Both charts use:

```tsx
margin={{ top: 12, right: 16, left: 4, bottom: 8 }}
```

The upper axes retain fan width `64` on the left and temperature width `44` on the right. The lower chart uses a power axis width `64` on the left plus a right-side axis with `width={44}`, `tick={false}`, `axisLine={false}`, and `tickLine={false}` to reserve the same plot boundary.

- [ ] **Step 3: Add the localized Power Trend label**

Add `fanCurve.history.powerTrend` with these values:

```text
en-US: Power trend
ja-JP: 電力トレンド
zh-CN: 功耗趋势
```

- [ ] **Step 4: Run the focused test and verify GREEN**

Run: `cd frontend; node --test tests/fan-curve-profile-ui.test.mjs`

Expected: all tests in the file pass.

### Task 3: Verify behavior and visual alignment

**Files:**
- Create: `design-qa.md`

- [ ] **Step 1: Run automated checks**

Run:

```powershell
cd frontend
npm test
npx tsc --noEmit
npm run build
```

Expected: all commands exit successfully.

- [ ] **Step 2: Run the desktop-like browser check**

Load the app with Wails runtime mocks and realistic history data. Verify that either chart shows all enabled valid rows, both plot regions share left/right X boundaries, and disabling both power toggles removes the lower region.

- [ ] **Step 3: Complete design QA**

Compare the supplied THRM screenshot and the rendered history detail at a matching desktop viewport. Record the reference path, implementation screenshot path, state, comparison findings, and `final result: passed` in project-root `design-qa.md`.

- [ ] **Step 4: Sync project metadata**

Run `git diff --check`, `codegraph sync`, and `codegraph status`.
