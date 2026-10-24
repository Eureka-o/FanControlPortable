# OMEN Realtime Overview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reuse the main settings realtime overview component in the OMEN plugin, with CPU/GPU temperature, power, and fan speed on the left and OMEN mode details on the right.

**Architecture:** Extract the inline settings overview into one presentational host UI component. The settings page and plugin supply different data objects to the same component; backend status and plugin command behavior remain unchanged.

**Tech Stack:** React 19, TypeScript, Tailwind CSS 4, FanControl Plugin Host v1 IIFE, Node test runner, Playwright CLI.

---

### Task 1: Shared Realtime Overview Component

**Files:**
- Create: `frontend/src/app/components/ui/RealtimeOverview.tsx`
- Modify: `frontend/src/app/components/ui/index.tsx`
- Modify: `frontend/src/app/components/ControlPanel.tsx`
- Modify: `frontend/tests/settings-tabs-ui.test.mjs`

- [ ] **Step 1: Write the failing shared-component test**

Load `RealtimeOverview.tsx` in `settings-tabs-ui.test.mjs` and assert that it contains the existing `settings-overview-temperature`, `settings-overview-device`, `settings-overview-model`, and `settings-overview-metrics` theme hooks. Assert that `ControlPanel.tsx` renders `<RealtimeOverview` before the settings tabs instead of owning the panel markup.

- [ ] **Step 2: Run the focused test and confirm RED**

Run: `cd frontend; node --test tests/settings-tabs-ui.test.mjs`

Expected: FAIL because `RealtimeOverview.tsx` and the shared component usage do not exist.

- [ ] **Step 3: Add the minimal presentational component**

Define these public data types and component contract:

```tsx
export interface RealtimeOverviewMetric {
  id: string;
  icon?: React.ReactNode;
  label: string;
  value: React.ReactNode;
}

export interface RealtimeOverviewHardware {
  id: string;
  icon: React.ReactNode;
  label: string;
  model?: string;
  metrics: RealtimeOverviewMetric[];
}

export interface RealtimeOverviewDevice {
  icon: React.ReactNode;
  name: string;
  connected: boolean;
  connectionLabel: string;
  details: Array<{ id: string; label: string; value: React.ReactNode }>;
}

export function RealtimeOverview(props: {
  title: string;
  titleIcon: React.ReactNode;
  hardware: RealtimeOverviewHardware[];
  device: RealtimeOverviewDevice;
}) {
  return (
    <section data-theme-card="settings-overview" className="rounded-2xl border border-border bg-card p-5 shadow-sm">
      <div className="mb-4 flex items-center gap-2">
        {props.titleIcon}
        <h3 className="text-base font-semibold text-foreground">{props.title}</h3>
      </div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-[minmax(0,1.2fr)_minmax(220px,0.8fr)]">
        <div data-theme-card="settings-overview-temperature" className="divide-y divide-border/55 rounded-xl border border-border/70 bg-muted/30 px-4">
          {props.hardware.map((item) => (
            <div key={item.id} className="grid min-w-0 grid-cols-1 gap-3 py-3.5 sm:grid-cols-[minmax(9rem,0.9fr)_minmax(0,1.4fr)] sm:items-center">
              <div className="flex min-w-0 items-center gap-3">
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-background/65 text-muted-foreground shadow-inner shadow-white/15">{item.icon}</div>
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-semibold text-foreground">{item.label}</div>
                  {item.model && <div data-theme-ui="settings-overview-model" title={item.model} className="mt-1 truncate text-[10px] text-muted-foreground">{item.model}</div>}
                </div>
              </div>
              <div data-theme-ui="settings-overview-metrics" className="grid min-w-0 gap-2" style={{ gridTemplateColumns: `repeat(${item.metrics.length}, minmax(0, 1fr))` }}>
                {item.metrics.map((metric) => (
                  <div key={metric.id} className="min-w-0">
                    <div className="flex items-center gap-1 text-[11px] text-muted-foreground">{metric.icon}{metric.label}</div>
                    <div className="mt-1 truncate text-sm font-semibold tabular-nums text-foreground">{metric.value}</div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
        <div data-theme-card="settings-overview-device" className="grid min-h-[10rem] grid-rows-2 divide-y divide-border/55 rounded-xl border border-border/70 bg-muted/30 px-4">
          <div className="flex min-w-0 items-center gap-3 py-3.5">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-background/65 text-muted-foreground shadow-inner shadow-white/15">{props.device.icon}</div>
            <div className="min-w-0 flex-1">
              <div title={props.device.name} className="line-clamp-2 break-words text-sm font-semibold leading-snug text-foreground">{props.device.name}</div>
              <div className="mt-1.5 flex items-center gap-2 text-[11px] text-muted-foreground"><span className={props.device.connected ? 'h-2 w-2 rounded-full bg-emerald-500' : 'h-2 w-2 rounded-full bg-muted-foreground/45'} />{props.device.connectionLabel}</div>
            </div>
          </div>
          <div className="grid items-center gap-4 py-3.5" style={{ gridTemplateColumns: `repeat(${props.device.details.length}, minmax(0, 1fr))` }}>
            {props.device.details.map((detail) => <div key={detail.id} className="min-w-0"><div className="text-[11px] text-muted-foreground">{detail.label}</div><div className="mt-0.5 truncate text-sm font-semibold text-foreground">{detail.value}</div></div>)}
          </div>
        </div>
      </div>
    </section>
  );
}
```

Keep all theme hooks and responsive behavior inside this component. Export it from `components/ui/index.tsx`.

- [ ] **Step 4: Migrate ControlPanel without changing settings behavior**

Map `overviewThermals` to hardware rows with temperature and power metrics. Map the connected-device block to the same name, connection, fan speed, and control mode values currently rendered inline. Keep `data-theme-card="settings-overview"` above `data-theme-ui="settings-tabs"`.

- [ ] **Step 5: Run the focused test and TypeScript check**

Run: `cd frontend; node --test tests/settings-tabs-ui.test.mjs; npx tsc --noEmit`

Expected: all settings tests pass and TypeScript exits 0.

### Task 2: OMEN Shared Overview Consumer

**Files:**
- Modify: `plugins/omen-fan/src/ui/index.js`
- Modify: `plugins/omen-fan/src/ui/index.css`
- Modify: `frontend/tests/omen-plugin-frontend.test.mjs`
- Modify: `output/playwright/omen-ui-init.js`

- [ ] **Step 1: Write the failing OMEN consumer test**

Assert that the IIFE calls `ui.RealtimeOverview`, supplies `cpuModel`, `gpuModel`, CPU/GPU fan metrics, current mode, and joint-learning details, and no longer renders the old `omen-device-card`, `omen-metrics`, or six independent metric tiles.

- [ ] **Step 2: Run the focused test and confirm RED**

Run: `cd frontend; node --test tests/omen-plugin-frontend.test.mjs`

Expected: FAIL because OMEN still renders its custom device card.

- [ ] **Step 3: Normalize model names and render the shared component**

Add `cpuModel` and `gpuModel` to `normalizeStatus`. Build two hardware rows:

```js
[
  { id: 'cpu', label: 'CPU', model: status.cpuModel, metrics: [temperature, power, fan] },
  { id: 'gpu', label: 'GPU', model: status.gpuModel, metrics: [temperature, power, fan] }
]
```

Build the right summary with the OMEN brand image, device name, connection state, current mode, and joint-learning state. Preserve readback-only updates and capability gates.

- [ ] **Step 4: Remove obsolete plugin-only overview CSS**

Delete the old header, six-tile metric grid, device footer, and their mobile rules. Keep only plugin page, tabs, controls, curve, and device-page styles. Add no replacement card system.

- [ ] **Step 5: Update the browser mock and run focused tests**

Add representative CPU/GPU model strings to `omen-ui-init.js` and run:

`cd frontend; node --test tests/omen-plugin-frontend.test.mjs tests/settings-tabs-ui.test.mjs`

Expected: both test files pass.

### Task 3: Full Verification

**Files:**
- Verify: `frontend/`
- Verify: `output/playwright/`

- [ ] **Step 1: Run all automated checks**

Run:

```powershell
cd frontend
npm test
npx tsc --noEmit
npm run build
```

Expected: zero failed tests, zero type errors, and a successful Next.js production build.

- [ ] **Step 2: Verify real rendering**

Use the existing OMEN Playwright mock and inspect settings plus OMEN at 1440 px, dark mode, and 390 px. Confirm CPU/GPU rows align, the right summary matches the main app, long names remain contained, document width equals viewport width, and the console has no new errors or warnings.

- [ ] **Step 3: Review scoped changes**

Confirm no backend, plugin protocol, packaging, keyboard-lighting, or unrelated settings behavior changed. Keep the local dev preview running for user review.
